package aggregate_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	health "github.com/larsartmann/go-health"
	aggregate "github.com/larsartmann/go-health/aggregate"
)

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

// latchSource flips one source's startup latch via a single startup request —
// the same evaluation path a real kubelet poll drives.
func latchSource(probe *health.Probe) {
	probe.StartupHandler()(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/startupz", nil),
	)
}

// TestHealthzHandler_SingleEndpointTable pins the aggregate's single-endpoint
// contract: 503 while any source is unlatched, failing, or draining; 200
// otherwise. warn stays 200, and the synthetic "startup" check appears only
// when the latch is the reason for the 503 (mirroring health.Probe.Healthz).
func TestHealthzHandler_SingleEndpointTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		critical       bool
		unhealthy      bool
		latched        bool
		shutdown       bool
		wantCode       int
		wantStartupKey bool
	}{
		{
			name:     "all sources latched and healthy is 200",
			latched:  true,
			wantCode: http.StatusOK,
		},
		{
			name:           "one unlatched source is 503 with synthetic startup check",
			wantCode:       http.StatusServiceUnavailable,
			wantStartupKey: true,
		},
		{
			name:      "non-critical degradation (warn) stays 200",
			unhealthy: true,
			latched:   true,
			wantCode:  http.StatusOK,
		},
		{
			name:      "critical failure (fail) is 503 without startup key",
			critical:  true,
			unhealthy: true,
			wantCode:  http.StatusServiceUnavailable,
		},
		{
			name:     "shutting-down source is 503 without startup key",
			shutdown: true,
			wantCode: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			failing := newStartedProbe(t, tt.critical, tt.unhealthy)
			healthy := newStartedProbe(t, false, false)

			if tt.latched {
				latchSource(failing)
				latchSource(healthy)
			}

			if tt.shutdown {
				failing.Shutdown()
			}

			agg := mustAggregate(t,
				aggregate.Source{Name: "api", Probe: failing},
				aggregate.Source{Name: "web", Probe: healthy},
			)

			rec := httptest.NewRecorder()
			agg.Healthz().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

			if rec.Code != tt.wantCode {
				t.Fatalf("healthz code = %d, want %d", rec.Code, tt.wantCode)
			}

			body := decodeResponse(t, rec)

			checks, ok := body["checks"].(map[string]any)
			if !ok {
				t.Fatalf("healthz body missing checks map; got %v", body)
			}

			if _, has := checks["startup"]; has != tt.wantStartupKey {
				t.Fatalf(
					"startup synthetic check present = %v, want %v; checks %v",
					has,
					tt.wantStartupKey,
					checks,
				)
			}

			// The merged view stays the body: both sources' namespaced checks.
			for _, key := range []string{"api/dependency", "web/dependency"} {
				if _, has := checks[key]; !has {
					t.Fatalf("healthz checks missing merged key %q; got %v", key, checks)
				}
			}
		})
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
