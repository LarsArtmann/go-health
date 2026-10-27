package federation_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// healthzCall drives the combined handler once and returns the status code
// plus the decoded body.
func healthzCall(t *testing.T, prober *federation.Prober) (int, health.Response) {
	t.Helper()

	rec := httptest.NewRecorder()
	prober.Healthz()(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	var resp health.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode healthz body %q: %v", rec.Body.String(), err)
	}

	return rec.Code, resp
}

func TestHealthz_AllRemotesHealthyIs200(t *testing.T) {
	t.Parallel()

	upstream := healthyUpstream(t)
	fed := mustNew(t, []federation.Remote{{Name: "api", URL: upstream.server.URL}})

	code, resp := healthzCall(t, fed)

	if code != http.StatusOK {
		t.Fatalf("healthz: want 200, got %d (%+v)", code, resp)
	}

	if resp.Status != health.StatusPass {
		t.Errorf("status = %q, want pass", resp.Status)
	}
}

func TestHealthz_WarnStays200(t *testing.T) {
	t.Parallel()

	upstream := serveDocument(t, health.Response{
		Status: health.StatusWarn,
		Checks: map[string]health.Check{"db": {Status: health.StatusWarn, Error: "slow"}},
	})
	fed := mustNew(t, []federation.Remote{{Name: "api", URL: upstream.server.URL}})

	code, resp := healthzCall(t, fed)

	if code != http.StatusOK {
		t.Fatalf("warn healthz: want 200, got %d (%+v)", code, resp)
	}

	if resp.Status != health.StatusWarn {
		t.Errorf("status = %q, want warn", resp.Status)
	}
}

func TestHealthz_DarkRemoteIs503WithReachableRow(t *testing.T) {
	t.Parallel()

	fed := mustNew(t, []federation.Remote{{Name: "api", URL: darkUpstream()}})

	code, resp := healthzCall(t, fed)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("dark healthz: want 503, got %d (%+v)", code, resp)
	}

	row, ok := resp.Checks["api/reachable"]
	if !ok {
		t.Fatalf("dark remote: missing api/reachable row; got %v", resp.Checks)
	}

	if row.Status != health.StatusFail {
		t.Errorf("api/reachable status = %q, want fail", row.Status)
	}

	// The reachable row already explains the failure; the synthetic startup
	// row must not overwrite the real cause.
	if _, ok := resp.Checks["startup"]; ok {
		t.Error("dark remote: startup row added over an already-failing roll-up")
	}
}

func TestHealthz_RemoteRollUpFailIs503(t *testing.T) {
	t.Parallel()

	upstream := serveDocument(t, health.Response{
		Status: health.StatusFail,
		Checks: map[string]health.Check{"db": {Status: health.StatusFail, Error: "down"}},
	})
	fed := mustNew(t, []federation.Remote{{Name: "api", URL: upstream.server.URL}})

	code, resp := healthzCall(t, fed)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("failing remote: want 503, got %d (%+v)", code, resp)
	}

	if resp.Status != health.StatusFail {
		t.Errorf("status = %q, want fail", resp.Status)
	}
}

func TestHealthz_DrainingRemoteIs503(t *testing.T) {
	t.Parallel()

	upstream := serveDocument(t, health.Response{
		Status:       health.StatusPass,
		ShuttingDown: true,
		Checks:       map[string]health.Check{},
	})
	fed := mustNew(t, []federation.Remote{{Name: "api", URL: upstream.server.URL}})

	code, resp := healthzCall(t, fed)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("draining remote: want 503, got %d (%+v)", code, resp)
	}

	if resp.Status != health.StatusFail {
		t.Errorf("status = %q, want fail", resp.Status)
	}
}

func TestHealthz_HubDrainIs503(t *testing.T) {
	t.Parallel()

	upstream := healthyUpstream(t)
	fed := mustNew(t, []federation.Remote{{Name: "api", URL: upstream.server.URL}})

	fed.MarkShuttingDown()

	code, resp := healthzCall(t, fed)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("draining hub: want 503, got %d (%+v)", code, resp)
	}

	if !resp.ShuttingDown {
		t.Error("draining hub: body is missing shutting_down")
	}
}
