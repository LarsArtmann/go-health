package aggregate_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	health "github.com/larsartmann/go-health"
	"github.com/larsartmann/go-health/aggregate"
)

// TestRegisterRoutes_SkipsEmptyPatterns pins the selective-mount contract on
// the aggregate surface, mirroring the root probe.
func TestRegisterRoutes_SkipsEmptyPatterns(t *testing.T) {
	t.Parallel()

	agg, err := aggregate.New(aggregate.Source{
		Name:  "api",
		Probe: health.NewChecks(nil),
	})
	if err != nil {
		t.Fatalf("aggregate.New: %v", err)
	}

	mux := http.NewServeMux()
	agg.RegisterRoutes(mux, health.Routes{Readiness: "/readyz"})

	cases := map[string]int{
		"/readyz":   http.StatusOK,
		"/healthz":  http.StatusNotFound,
		"/startupz": http.StatusNotFound,
	}

	for path, want := range cases {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))

		if rec.Code != want {
			t.Errorf("GET %s: want %d, got %d", path, want, rec.Code)
		}
	}
}

// TestRegisterRoutes_MountsVersionEndpoint pins the Routes.Version slot on
// the aggregate surface: the mount serves {"version":""} because the
// aggregate carries no build identity, and DefaultRoutes() leaves /version
// unmounted.
func TestRegisterRoutes_MountsVersionEndpoint(t *testing.T) {
	t.Parallel()

	agg, err := aggregate.New(aggregate.Source{
		Name:  "api",
		Probe: health.NewChecks(nil),
	})
	if err != nil {
		t.Fatalf("aggregate.New: %v", err)
	}

	mux := http.NewServeMux()
	agg.RegisterRoutes(mux, health.Routes{Version: "GET /version"})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/version", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /version: want 200, got %d", rec.Code)
	}

	if body := rec.Body.String(); body != `{"version":""}` {
		t.Errorf("GET /version body: want %q, got %q", `{"version":""}`, body)
	}

	defaults := http.NewServeMux()
	agg.RegisterRoutes(defaults, health.DefaultRoutes())

	rec = httptest.NewRecorder()
	defaults.ServeHTTP(
		rec,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/version", nil),
	)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /version with DefaultRoutes: want 404 (not mounted), got %d", rec.Code)
	}
}
