package health_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	health "github.com/larsartmann/go-health"
)

// TestRegisterRoutes_SkipsEmptyPatterns pins the selective-mount contract: a
// zero Routes field means "do not serve this probe", not "panic on an empty
// ServeMux pattern". Consumers that expose only readiness (or only liveness)
// pass the subset they serve.
func TestRegisterRoutes_SkipsEmptyPatterns(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(nil)

	mux := http.NewServeMux()
	probe.RegisterRoutes(mux, health.Routes{Readiness: "/readyz"})

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
