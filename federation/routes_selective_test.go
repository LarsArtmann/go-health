package federation_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// TestRegisterRoutes_SkipsEmptyPatterns pins the selective-mount contract on
// the federation surface, mirroring the root probe and the aggregate.
func TestRegisterRoutes_SkipsEmptyPatterns(t *testing.T) {
	t.Parallel()

	upstream := healthyUpstream(t)
	fed := mustNew(t, []federation.Remote{{Name: "api", URL: upstream.server.URL}})

	mux := http.NewServeMux()
	fed.RegisterRoutes(mux, health.Routes{Readiness: "/readyz"})

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
// the federation surface: the mount serves {"version":""} because the hub
// carries no build identity, and DefaultRoutes() leaves /version unmounted.
func TestRegisterRoutes_MountsVersionEndpoint(t *testing.T) {
	t.Parallel()

	upstream := healthyUpstream(t)
	fed := mustNew(t, []federation.Remote{{Name: "api", URL: upstream.server.URL}})

	mux := http.NewServeMux()
	fed.RegisterRoutes(mux, health.Routes{Version: "GET /version"})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/version", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /version: want 200, got %d", rec.Code)
	}

	if body := rec.Body.String(); body != `{"version":""}` {
		t.Errorf("GET /version body: want %q, got %q", `{"version":""}`, body)
	}

	defaults := http.NewServeMux()
	fed.RegisterRoutes(defaults, health.DefaultRoutes())

	rec = httptest.NewRecorder()
	defaults.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/version", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /version with DefaultRoutes: want 404 (not mounted), got %d", rec.Code)
	}
}
