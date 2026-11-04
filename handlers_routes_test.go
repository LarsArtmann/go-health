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

// TestDefaultRoutes_AreMethodQualified pins the default patterns: kubelet
// probes are always GET, and the method prefix is what keeps registration
// from panicking against common page-router shapes (a bare "/healthz"
// conflicts with "GET /{slug}").
func TestDefaultRoutes_AreMethodQualified(t *testing.T) {
	t.Parallel()

	want := health.Routes{
		Liveness:  "GET /healthz",
		Readiness: "GET /readyz",
		Startup:   "GET /startupz",
	}

	if got := health.DefaultRoutes(); got != want {
		t.Errorf("DefaultRoutes: want %+v, got %+v", want, got)
	}
}

// TestRegisterRoutes_DefaultsCoexistWithCatchAllPage pins the failure that
// motivated the method-qualified defaults: a mux that already carries a
// catch-all page route ("GET /{slug}") panicked at registration when the
// probe mounted bare "/healthz", because the bare pattern matches more
// methods but a less specific path. With the defaults method-scoped, both
// shapes register and each keeps its own requests.
func TestRegisterRoutes_DefaultsCoexistWithCatchAllPage(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{slug}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	probe.RegisterRoutes(mux, health.DefaultRoutes())

	for _, path := range []string{"/healthz", "/readyz", "/startupz", "/about"} {
		w := httptest.NewRecorder()

		r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		if err != nil {
			t.Fatal(err)
		}

		mux.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("GET %s after coexistence: want 200, got %d", path, w.Code)
		}
	}
}

// TestRegisterRoutes_MountsVersionEndpoint pins the Routes.Version slot: a
// non-empty pattern mounts VersionHandler serving the probe's WithVersion
// stamp; the empty DefaultRoutes value keeps /version unmounted (opt-in, so
// consumers that wire their own version handler never double-register).
func TestRegisterRoutes_MountsVersionEndpoint(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector,
		health.WithRefreshInterval(0),
		health.WithVersion("v1.2.3"),
	)

	mux := http.NewServeMux()
	probe.RegisterRoutes(mux, health.Routes{Version: "GET /version"})

	w := httptest.NewRecorder()

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/version", nil)
	if err != nil {
		t.Fatal(err)
	}

	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /version: want 200, got %d", w.Code)
	}

	if body := w.Body.String(); body != `{"version":"v1.2.3"}` {
		t.Errorf("GET /version body: want %q, got %q", `{"version":"v1.2.3"}`, body)
	}

	w = httptest.NewRecorder()

	r, err = http.NewRequestWithContext(t.Context(), http.MethodPost, "/version", nil)
	if err != nil {
		t.Fatal(err)
	}

	mux.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /version: want 405, got %d", w.Code)
	}

	defaults := http.NewServeMux()
	probe.RegisterRoutes(defaults, health.DefaultRoutes())

	w = httptest.NewRecorder()

	r, err = http.NewRequestWithContext(t.Context(), http.MethodGet, "/version", nil)
	if err != nil {
		t.Fatal(err)
	}

	defaults.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("GET /version with DefaultRoutes: want 404 (not mounted), got %d", w.Code)
	}
}
