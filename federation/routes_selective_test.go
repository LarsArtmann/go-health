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
