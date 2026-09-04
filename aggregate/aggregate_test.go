package aggregate_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	aggregate "github.com/larsartmann/go-health/aggregate"
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
		{
			name:    "name containing slash",
			sources: []aggregate.Source{{Name: "team/api", Probe: healthy}},
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

// TestNew_SlashNameContract pins both halves of the source-name contract:
// source names with "/" are rejected because the name becomes the check-key
// prefix ("name/check") and a slash would blur the grouping axis and could
// alias another source's namespace; check names with "/" are accepted and
// everything before the first slash stays the source name.
// See docs/aggregate-source-name-design.md.
func TestNew_SlashNameContract(t *testing.T) {
	t.Parallel()

	t.Run("rejects slash in source name", func(t *testing.T) {
		t.Parallel()

		_, err := aggregate.New(aggregate.Source{
			Name:  "team/api",
			Probe: newStartedProbe(t, false, false),
		})
		if !errors.Is(err, aggregate.ErrInvalidSource) {
			t.Fatalf("aggregate.New error = %v, want %v", err, aggregate.ErrInvalidSource)
		}
	})

	t.Run("accepts slash in check name, grouping axis intact", func(t *testing.T) {
		t.Parallel()

		results := func(context.Context) map[string]error {
			return map[string]error{"pool/primary": nil}
		}

		probe := health.NewWithHealthCheck(results, health.WithRefreshInterval(0))

		if err := probe.Start(context.Background()); err != nil {
			t.Fatalf("probe.Start: %v", err)
		}

		agg, err := aggregate.New(aggregate.Source{Name: "api", Probe: probe})
		if err != nil {
			t.Fatalf("aggregate.New: %v", err)
		}

		merged := agg.CachedResponse()

		if len(merged.Checks) != 1 {
			t.Fatalf("merged check count = %d, want 1", len(merged.Checks))
		}

		if _, ok := merged.Checks["api/pool/primary"]; !ok {
			t.Fatalf("merged checks = %v, want key \"api/pool/primary\"", merged.Checks)
		}
	})
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

// TestCachedResponse_NeverStartedSource pins the zero-cache merge path: a
// source probe that was never started (or whose checks were never invoked)
// contributes an empty cached view — no checks, no latency, status folded as
// healthy. That is the documented false-confidence sharp edge (AGENTS.md:
// invoke critical services at boot): readiness reports pass until the
// source's own first evaluation populates its cache. The startup latch is
// the honest signal — it stays false, so startup keeps serving 503.
func TestCachedResponse_NeverStartedSource(t *testing.T) {
	t.Parallel()

	neverStarted := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"svc": nil}
	}, health.WithRefreshInterval(0))

	started := newPrimedSource(t, func(context.Context) map[string]error {
		return map[string]error{"svc": nil}
	}, "pod-1")

	agg, err := aggregate.New(
		aggregate.Source{Name: "ghost", Probe: neverStarted},
		aggregate.Source{Name: "real", Probe: started},
	)
	if err != nil {
		t.Fatalf("aggregate.New: %v", err)
	}

	merged := agg.CachedResponse()

	if len(merged.Checks) != 1 {
		t.Fatalf("merged check count = %d, want 1 (only the started source)", len(merged.Checks))
	}

	if _, ok := merged.Checks["real/svc"]; !ok {
		t.Fatalf("merged checks = %v, want only \"real/svc\"", merged.Checks)
	}

	if merged.Status != health.StatusPass {
		t.Errorf("merged status = %q, want pass (never-started folds as healthy)", merged.Status)
	}

	if merged.ShuttingDown || merged.TotalLatencyMs != 0 ||
		merged.InstanceID != "" || merged.Version != "" {
		t.Errorf("unexpected merged scalars: %+v", merged)
	}

	if agg.StartupComplete() {
		t.Error("StartupComplete = true, want false (never-started latch is unset)")
	}

	startupRec := httptest.NewRecorder()
	agg.StartupHandler()(startupRec, fuzzRequest(t))

	if startupRec.Code != http.StatusServiceUnavailable {
		t.Errorf("startup status = %d, want 503 while any latch is unset", startupRec.Code)
	}

	readyRec := httptest.NewRecorder()
	agg.ReadinessHandler()(readyRec, fuzzRequest(t))

	if readyRec.Code != http.StatusOK {
		t.Errorf(
			"readiness status = %d, want 200 (pass fold; the documented sharp edge)",
			readyRec.Code,
		)
	}
}

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

func TestCachedResponse_TotalLatencyMsIsSlowestSource(t *testing.T) {
	t.Parallel()

	// A check that takes measurably long stamps its source's cached response
	// with a non-zero TotalLatencyMs; the merge must propagate the max.
	slow := health.NewWithHealthCheck(func(_ context.Context) map[string]error {
		time.Sleep(2 * time.Millisecond)

		return map[string]error{"db": nil}
	}, health.WithRefreshInterval(0))

	if err := slow.Start(context.Background()); err != nil {
		t.Fatalf("slow.Start: %v", err)
	}

	slowCached := slow.CachedResponse()
	if slowCached.TotalLatencyMs <= 0 {
		t.Fatalf("test setup: slow source latency = %dms, want > 0", slowCached.TotalLatencyMs)
	}

	agg := mustAggregate(t,
		aggregate.Source{Name: "slow", Probe: slow},
		aggregate.Source{Name: "fast", Probe: newStartedProbe(t, false, false)},
	)

	if got := agg.CachedResponse().TotalLatencyMs; got != slowCached.TotalLatencyMs {
		t.Fatalf(
			"merged latency = %dms, want slowest source's %dms",
			got,
			slowCached.TotalLatencyMs,
		)
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
		name      string
		critical  bool
		unhealthy bool
		shutdown  bool
		wantCode  int
	}{
		{name: "healthy is 200", wantCode: http.StatusOK},
		{name: "non-critical failure (warn) stays 200", unhealthy: true, wantCode: http.StatusOK},
		{
			name:      "critical failure (fail) is 503",
			critical:  true,
			unhealthy: true,
			wantCode:  http.StatusServiceUnavailable,
		},
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
			agg.ReadinessHandler().
				ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

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
		probe.StartupHandler().
			ServeHTTP(probeRec, httptest.NewRequest(http.MethodGet, "/startupz", nil))
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
