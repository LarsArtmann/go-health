package health_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- RegisterRoutes tests ---.

func TestRegisterRoutes_AllThreeHandlersRegistered(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	mux := http.NewServeMux()
	probe.RegisterRoutes(mux, health.DefaultRoutes())

	for _, path := range []string{"/healthz", "/readyz", "/startupz"} {
		w := httptest.NewRecorder()

		r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		if err != nil {
			t.Fatal(err)
		}

		mux.ServeHTTP(w, r)

		if w.Code == http.StatusNotFound {
			t.Errorf("route %s not registered", path)
		}
	}
}

func TestRegisterRoutes_CustomPaths(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	mux := http.NewServeMux()
	probe.RegisterRoutes(mux, health.Routes{
		Liveness:  "/live",
		Readiness: "/ready",
		Startup:   "/started",
	})

	w := httptest.NewRecorder()

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/live", nil)
	if err != nil {
		t.Fatal(err)
	}

	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("custom liveness route: want 200, got %d", w.Code)
	}
}
