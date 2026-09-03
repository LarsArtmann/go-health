package aggregate_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	aggregate "github.com/larsartmann/go-health/aggregate"
	health "github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Test service types ---.

type healthyService struct{}

var _ do.HealthcheckerWithContext = (*healthyService)(nil)

func (healthyService) HealthCheck(_ context.Context) error { return nil }

var errUnhealthy = errors.New("service unhealthy")

type unhealthyService struct{ reason string }

var _ do.HealthcheckerWithContext = (*unhealthyService)(nil)

func (u *unhealthyService) HealthCheck(_ context.Context) error {
	return fmt.Errorf("%w: %s", errUnhealthy, u.reason)
}

// --- Test helpers ---.

func newStartedProbe(t *testing.T, critical bool, unhealthy bool) *health.Probe {
	t.Helper()

	injector := do.New()

	if unhealthy {
		do.ProvideNamed(injector, "dependency", func(_ do.Injector) (*unhealthyService, error) {
			return &unhealthyService{reason: "connection refused"}, nil
		})

		// samber/do only health-checks instantiated services: without the
		// eager invoke, the dependency is invisible and the probe stays pass.
		do.MustInvokeNamed[*unhealthyService](injector, "dependency")
	} else {
		do.ProvideNamed(injector, "dependency", func(_ do.Injector) (*healthyService, error) {
			return &healthyService{}, nil
		})
		do.MustInvokeNamed[*healthyService](injector, "dependency")
	}

	opts := []health.Option{health.WithRefreshInterval(0)}
	if critical {
		opts = append(opts, health.WithCriticalServices("dependency"))
	}

	probe := health.New(injector, opts...)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		injector.Shutdown()
	})

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}

	return probe
}

func mustAggregate(t *testing.T, sources ...aggregate.Source) *aggregate.Aggregate {
	t.Helper()

	agg, err := aggregate.New(sources...)
	if err != nil {
		t.Fatalf("aggregate.New: %v", err)
	}

	return agg
}

// decodeResponse unmarshals a handler body into a map for status-code and
// shape assertions without depending on struct equality.
func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	return body
}

// --- Construction ---.

func TestNew_RejectsInvalidSources(t *testing.T) {
	t.Parallel()

	healthy := newStartedProbe(t, false, false)

	tests := []struct {
		name    string
		sources []aggregate.Source
		wantErr error
	}{
		{
			name:    "no sources",
			sources: nil,
			wantErr: aggregate.ErrNoSources,
		},
		{
			name:    "nil probe",
			sources: []aggregate.Source{{Name: "api", Probe: nil}},
			wantErr: aggregate.ErrInvalidSource,
		},
		{
			name:    "empty name",
			sources: []aggregate.Source{{Name: "", Probe: healthy}},
			wantErr: aggregate.ErrInvalidSource,
		},
		{
			name: "duplicate name",
			sources: []aggregate.Source{
				{Name: "api", Probe: healthy},
				{Name: "api", Probe: healthy},
			},
			wantErr: aggregate.ErrInvalidSource,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := aggregate.New(tt.sources...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("aggregate.New error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNew_RefreshIntervalIsSlowestSource(t *testing.T) {
	t.Parallel()

	fast := health.New(do.New(), health.WithRefreshInterval(1*1e9)) // 1s
	slow := health.New(do.New(), health.WithRefreshInterval(5*1e9)) // 5s
	live := health.New(do.New(), health.WithRefreshInterval(0))

	agg := mustAggregate(t,
		aggregate.Source{Name: "fast", Probe: fast},
		aggregate.Source{Name: "slow", Probe: slow},
		aggregate.Source{Name: "live", Probe: live},
	)

	if got := agg.RefreshInterval(); got != 5*1e9 {
		t.Fatalf("RefreshInterval = %s, want 5s (slowest source)", got)
	}
}

// --- CachedResponse merging ---.

func TestCachedResponse_WorstOfStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		one  bool // first source critical+unhealthy (fail)
		two  bool // second source non-critical+unhealthy (warn)
		// third source is always healthy (pass)
		want health.Status
	}{
		{name: "all pass", one: false, two: false, want: health.StatusPass},
		{name: "warn beats pass", one: false, two: true, want: health.StatusWarn},
		{name: "fail beats pass", one: true, two: false, want: health.StatusFail},
		{name: "fail beats warn", one: true, two: true, want: health.StatusFail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agg := mustAggregate(t,
				aggregate.Source{Name: "one", Probe: newStartedProbe(t, true, tt.one)},
				aggregate.Source{Name: "two", Probe: newStartedProbe(t, false, tt.two)},
				aggregate.Source{Name: "three", Probe: newStartedProbe(t, false, false)},
			)

			resp := agg.CachedResponse()

			if resp.Status != tt.want {
				t.Fatalf("Status = %q, want %q", resp.Status, tt.want)
			}

			if resp.ShuttingDown {
				t.Fatal("ShuttingDown = true, want false")
			}

			if resp.TotalLatencyMs < 0 {
				t.Fatalf("TotalLatencyMs = %d, want >= 0", resp.TotalLatencyMs)
			}

			// Every source contributes its namespaced dependency check.
			for _, key := range []string{"one/dependency", "two/dependency", "three/dependency"} {
				if _, ok := resp.Checks[key]; !ok {
					t.Fatalf("Checks missing namespaced key %q; got %v", key, resp.Checks)
				}
			}
		})
	}
}

func TestCachedResponse_ShutdownForcesFail(t *testing.T) {
	t.Parallel()

	shuttingDown := newStartedProbe(t, false, false)
	shuttingDown.Shutdown()

	agg := mustAggregate(t,
		aggregate.Source{Name: "down", Probe: shuttingDown},
		aggregate.Source{Name: "up", Probe: newStartedProbe(t, false, false)},
	)

	resp := agg.CachedResponse()

	if !resp.ShuttingDown {
		t.Fatal("ShuttingDown = false, want true")
	}

	if resp.Status != health.StatusFail {
		t.Fatalf("Status = %q, want fail (shutdown mirrors Probe.classify)", resp.Status)
	}

	// The healthy source's checks survive the merge.
	if _, ok := resp.Checks["up/dependency"]; !ok {
		t.Fatalf("healthy source checks lost in merge; got %v", resp.Checks)
	}
}

// --- Handlers ---.

func TestLivenessHandler_AlwaysPass(t *testing.T) {
	t.Parallel()

	failing := newStartedProbe(t, true, true)
	agg := mustAggregate(t, aggregate.Source{Name: "api", Probe: failing})

	rec := httptest.NewRecorder()
	agg.LivenessHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("liveness code = %d, want 200 (liveness never checks dependencies)", rec.Code)
	}

	body := decodeResponse(t, rec)
	if body["status"] != string(health.StatusPass) {
		t.Fatalf("liveness status = %v, want pass", body["status"])
	}
}

func TestReadinessHandler_StatusCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		critical bool
		unhealthy bool
		shutdown bool
		wantCode int
	}{
		{name: "healthy is 200", wantCode: http.StatusOK},
		{name: "non-critical failure (warn) stays 200", unhealthy: true, wantCode: http.StatusOK},
		{name: "critical failure (fail) is 503", critical: true, unhealthy: true, wantCode: http.StatusServiceUnavailable},
		{name: "shutdown is 503", shutdown: true, wantCode: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			probe := newStartedProbe(t, tt.critical, tt.unhealthy)
			if tt.shutdown {
				probe.Shutdown()
			}

			agg := mustAggregate(t, aggregate.Source{Name: "api", Probe: probe})

			rec := httptest.NewRecorder()
			agg.ReadinessHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

			if rec.Code != tt.wantCode {
				t.Fatalf("readiness code = %d, want %d", rec.Code, tt.wantCode)
			}
		})
	}
}

func TestStartupHandler_LatchesWhenAllSourcesComplete(t *testing.T) {
	t.Parallel()

	first := newStartedProbe(t, false, false)
	second := newStartedProbe(t, false, false)
	agg := mustAggregate(t,
		aggregate.Source{Name: "first", Probe: first},
		aggregate.Source{Name: "second", Probe: second},
	)

	rec := httptest.NewRecorder()
	agg.StartupHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/startupz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("startup code before evaluation = %d, want 503", rec.Code)
	}

	body := decodeResponse(t, rec)
	for _, name := range []string{"first", "second"} {
		if _, ok := body["checks"].(map[string]any)[name]; !ok {
			t.Fatalf("startup checks missing incomplete source %q; got %v", name, body["checks"])
		}
	}

	// Evaluating each source flips its latch (no critical services configured,
	// so the first evaluation passes immediately).
	for _, probe := range []*health.Probe{first, second} {
		probeRec := httptest.NewRecorder()
		probe.StartupHandler().ServeHTTP(probeRec, httptest.NewRequest(http.MethodGet, "/startupz", nil))
	}

	rec = httptest.NewRecorder()
	agg.StartupHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/startupz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("startup code after latches = %d, want 200", rec.Code)
	}
}

func TestRegisterRoutes_WiresAllThreeProbes(t *testing.T) {
	t.Parallel()

	agg := mustAggregate(t,
		aggregate.Source{Name: "api", Probe: newStartedProbe(t, false, false)},
	)

	mux := http.NewServeMux()
	agg.RegisterRoutes(mux, health.DefaultRoutes())

	tests := []struct {
		path string
		want int
	}{
		{path: "/healthz", want: http.StatusOK},
		{path: "/readyz", want: http.StatusOK},
		{path: "/startupz", want: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

		if rec.Code != tt.want {
			t.Fatalf("GET %s code = %d, want %d", tt.path, rec.Code, tt.want)
		}

		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("GET %s content-type = %q, want application/json", tt.path, ct)
		}
	}
}
