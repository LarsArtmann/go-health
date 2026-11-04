package health_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- RegisterRoutes tests ---.

func TestRegisterRoutes_AllThreeHandlersRegistered(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector,
		health.WithRefreshInterval(0),
		health.WithVersion("test"),
	)

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
// conflicts with "GET /{slug}"). The version endpoint is mounted by default —
// a binary serving health routes answers "which build am I talking to?"
// without extra wiring.
func TestDefaultRoutes_AreMethodQualified(t *testing.T) {
	t.Parallel()

	want := health.Routes{
		Liveness:  "GET /healthz",
		Readiness: "GET /readyz",
		Startup:   "GET /startupz",
		Version:   "GET /version",
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
	probe := health.New(injector,
		health.WithRefreshInterval(0),
		health.WithVersion("test"),
	)

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
// stamp, non-GET requests get 405, and DefaultRoutes includes the mount.
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

	if w.Code != http.StatusOK {
		t.Errorf("GET /version with DefaultRoutes: want 200 (mounted by default), got %d", w.Code)
	}
}

// TestRegisterRoutes_PanicsOnUnstampedVersionMount pins the hard-fail
// contract: an unstamped probe must not mount the version endpoint, because
// the endpoint would answer {"version":""} and lie about the build. The
// panic fires at registration (boot time, before the first request) and
// names both remedies.
func TestRegisterRoutes_PanicsOnUnstampedVersionMount(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("RegisterRoutes with Version set on an unstamped probe: want panic, got none")
		}

		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value: want string, got %T", r)
		}

		for _, want := range []string{"WithVersion", "Routes.Version"} {
			if !strings.Contains(msg, want) {
				t.Errorf("panic message missing %q: %s", want, msg)
			}
		}
	}()

	probe.RegisterRoutes(http.NewServeMux(), health.DefaultRoutes())
}

// TestRegisterRoutes_EmptyVersionStaysUnmounted pins the escape hatch: an
// explicit empty Routes.Version keeps /version unmounted, so a consumer that
// wires its own version handler (or serves none) never double-registers.
func TestRegisterRoutes_EmptyVersionStaysUnmounted(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector,
		health.WithRefreshInterval(0),
		health.WithVersion("v1.2.3"),
	)

	mux := http.NewServeMux()
	probe.RegisterRoutes(mux, health.Routes{})

	w := httptest.NewRecorder()

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/version", nil)
	if err != nil {
		t.Fatal(err)
	}

	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("GET /version with empty Routes.Version: want 404 (not mounted), got %d", w.Code)
	}
}
