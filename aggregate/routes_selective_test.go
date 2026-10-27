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
