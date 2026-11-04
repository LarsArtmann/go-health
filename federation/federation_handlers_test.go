package federation_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// TestLivenessHandler_FetchFreeAlwaysOK pins the cascade-safety contract:
// liveness is a static pass that never touches a remote — even one that
// refuses connections.
func TestLivenessHandler_FetchFreeAlwaysOK(t *testing.T) {
	t.Parallel()

	fed := mustNew(t, []federation.Remote{{Name: "ghost", URL: darkUpstream()}})

	rec := httptest.NewRecorder()
	fed.LivenessHandler()(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("liveness: want 200, got %d", rec.Code)
	}

	var got health.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("liveness body: %v", err)
	}

	if got.Status != health.StatusPass || len(got.Checks) != 0 {
		t.Errorf("liveness: want pass with empty checks, got %+v", got)
	}
}

// TestReadinessHandler_ServesMergedVerdict walks readiness both ways and
// proves the remote request carries Accept: application/json (the
// upstream rejects the read without it).
func TestReadinessHandler_ServesMergedVerdict(t *testing.T) {
	t.Parallel()

	fed := mustNew(t, []federation.Remote{{Name: "a", URL: healthyUpstream(t).server.URL}})

	rec := httptest.NewRecorder()
	fed.ReadinessHandler()(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"readiness over healthy remote: want 200, got %d (%s)",
			rec.Code,
			rec.Body.String(),
		)
	}

	dark := mustNew(t, []federation.Remote{{Name: "b", URL: darkUpstream()}})

	rec = httptest.NewRecorder()
	dark.ReadinessHandler()(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness over dark remote: want 503, got %d", rec.Code)
	}
}

// TestStartupHandler_ProgressesViaOwnFetches pins the boot contract: the
// startup handler's own fetches move the latches, so kubelet polling
// alone drives startup to completion.
func TestStartupHandler_ProgressesViaOwnFetches(t *testing.T) {
	t.Parallel()

	fed := mustNew(t, []federation.Remote{{Name: "a", URL: healthyUpstream(t).server.URL}})

	rec := httptest.NewRecorder()
	fed.StartupHandler()(rec, httptest.NewRequest(http.MethodGet, "/startupz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"startup after successful fetch: want 200, got %d (%s)",
			rec.Code,
			rec.Body.String(),
		)
	}

	if !fed.StartupComplete() {
		t.Fatal("startup handler must latch the answered remote")
	}
}

// TestStartupHandler_NamesUnansweredRemotes pins the 503 shape: one
// failing check per remote without a successful fetch, answered remotes
// absent from the report.
func TestStartupHandler_NamesUnansweredRemotes(t *testing.T) {
	t.Parallel()

	fed := mustNew(t, []federation.Remote{
		{Name: "dark", URL: darkUpstream()},
		{Name: "lit", URL: healthyUpstream(t).server.URL},
	})

	rec := httptest.NewRecorder()
	fed.StartupHandler()(rec, httptest.NewRequest(http.MethodGet, "/startupz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("startup with a dark remote: want 503, got %d", rec.Code)
	}

	var got health.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("startup body: %v", err)
	}

	dark, ok := got.Checks["dark"]
	if !ok || dark.Status != health.StatusFail {
		t.Errorf("startup must name the dark remote as failing, got %v", got.Checks)
	}

	if _, ok := got.Checks["lit"]; ok {
		t.Errorf("startup must not flag the answered remote, got %v", got.Checks)
	}
}

// TestRegisterRoutes_WiresAllThree smoke-tests the route registration
// surface end to end over a real mux.
func TestRegisterRoutes_WiresAllThree(t *testing.T) {
	t.Parallel()

	upstream := newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"pass","checks":{}}`))
	})

	fed := mustNew(t, []federation.Remote{{Name: "a", URL: upstream.server.URL}})

	mux := http.NewServeMux()
	fed.RegisterRoutes(mux, health.DefaultRoutes())

	for _, path := range []string{"/healthz", "/readyz", "/startupz"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: want 200, got %d", path, rec.Code)
		}
	}
}
