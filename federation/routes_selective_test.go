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

// TestRegisterRoutes_IgnoresVersionField pins the identity-honest contract
// on the federation surface: the hub carries no build identity, so it never
// mounts the version endpoint — not even when Routes.Version is set —
// because an unstamped /version would answer {"version":""} and lie about
// the build. Wire health.VersionHandler yourself if you stamp the process.
func TestRegisterRoutes_IgnoresVersionField(t *testing.T) {
	t.Parallel()

	upstream := healthyUpstream(t)
	fed := mustNew(t, []federation.Remote{{Name: "api", URL: upstream.server.URL}})

	mux := http.NewServeMux()
	fed.RegisterRoutes(mux, health.Routes{Version: "GET /version"})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/version", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf(
			"GET /version with Routes.Version set: want 404 (hub has no build identity, field ignored), got %d",
			rec.Code,
		)
	}

	defaults := http.NewServeMux()
	fed.RegisterRoutes(defaults, health.DefaultRoutes())

	rec = httptest.NewRecorder()
	defaults.ServeHTTP(
		rec,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/version", nil),
	)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /version with DefaultRoutes: want 404 (not mounted), got %d", rec.Code)
	}
}
