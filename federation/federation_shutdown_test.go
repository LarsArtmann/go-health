package federation_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// TestMarkShuttingDown_ReadinessDrainsWithFreshBody pins the two-phase
// drain contract: after the mark, readiness answers 503 with
// shutting_down set even over healthy remotes, and the drained body is
// still fetched live — a remote document that changes mid-drain shows up
// in the drained response, so a dashboard keeps seeing truth while the
// load balancer pulls the hub.
func TestMarkShuttingDown_ReadinessDrainsWithFreshBody(t *testing.T) {
	t.Parallel()

	var lit atomic.Bool

	lit.Store(true)

	upstream := newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
		if lit.Load() {
			_, _ = w.Write([]byte(`{"status":"pass","checks":{}}`))

			return
		}

		_, _ = w.Write(
			[]byte(`{"status":"pass","checks":{"disk":{"status":"off","error":"enable recipe"}}}`),
		)
	})

	fed := mustNew(t, []federation.Remote{{Name: "api", URL: upstream.server.URL}})

	rec := httptest.NewRecorder()
	fed.ReadinessHandler()(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("readiness before the mark: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	fed.MarkShuttingDown()

	rec = httptest.NewRecorder()
	fed.ReadinessHandler()(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness while draining: want 503, got %d (%s)", rec.Code, rec.Body.String())
	}

	var got health.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("drained body: %v", err)
	}

	if !got.ShuttingDown {
		t.Errorf("drained body must set shutting_down, got %+v", got)
	}

	if got.Status != health.StatusFail {
		t.Errorf("drained status: want fail, got %q", got.Status)
	}

	if _, unreachable := got.Checks["api/reachable"]; unreachable {
		t.Errorf("draining must not sever the remote: got reachable row %v", got.Checks)
	}

	lit.Store(false)

	rec = httptest.NewRecorder()
	fed.ReadinessHandler()(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("drained body after remote change: %v", err)
	}

	if disk, ok := got.Checks["api/disk"]; !ok || disk.Status != health.StatusOff {
		t.Errorf("drained body must stay fetched live, want fresh api/disk row, got %v", got.Checks)
	}
}

// TestMarkShuttingDown_CachedResponseOverlays pins the programmatic
// parity: the dashboard-facing read reports the drain too — fail with
// shutting_down set — while the merge still carries the fetched remote
// view (no reachable row: the remote was fetched, not refused).
func TestMarkShuttingDown_CachedResponseOverlays(t *testing.T) {
	t.Parallel()

	fed := mustNew(t, []federation.Remote{{Name: "a", URL: healthyUpstream(t).server.URL}})

	before := fed.CachedResponse()
	if before.Status != health.StatusPass || before.ShuttingDown {
		t.Fatalf("before the mark: want unmarked pass, got %+v", before)
	}

	fed.MarkShuttingDown()

	after := fed.CachedResponse()
	if !after.ShuttingDown || after.Status != health.StatusFail {
		t.Fatalf("after the mark: want shutting-down fail, got %+v", after)
	}

	if _, refused := after.Checks["a/reachable"]; refused {
		t.Errorf("draining must not sever the remote, got %v", after.Checks)
	}
}

// TestMarkShuttingDown_LivenessAndStartupUnaffected pins the drain's
// blast radius: liveness stays a static fetch-free 200 without the
// shutting_down flag, and startup latches on its own fetches exactly as
// if the hub were not draining — the same contract [health.Probe] gives.
func TestMarkShuttingDown_LivenessAndStartupUnaffected(t *testing.T) {
	t.Parallel()

	fed := mustNew(t, []federation.Remote{{Name: "a", URL: healthyUpstream(t).server.URL}})
	fed.MarkShuttingDown()

	rec := httptest.NewRecorder()
	fed.LivenessHandler()(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("liveness while draining: want 200, got %d", rec.Code)
	}

	var liveness health.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &liveness); err != nil {
		t.Fatalf("liveness body: %v", err)
	}

	if liveness.ShuttingDown || liveness.Status != health.StatusPass {
		t.Errorf("liveness must not carry the drain, got %+v", liveness)
	}

	rec = httptest.NewRecorder()
	fed.StartupHandler()(rec, httptest.NewRequest(http.MethodGet, "/startupz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("startup while draining: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	if !fed.StartupComplete() {
		t.Fatal("startup handler must latch the answered remote even while draining")
	}
}

// TestMarkShuttingDown_DrainsThroughRegisterRoutes observes the drain
// through the wired surface a hub actually serves: the same handlers
// mounted on a real mux via RegisterRoutes. Only readiness flips to 503
// with the shutting_down body; the mux-mounted liveness and startup
// routes answer as if nothing happened.
func TestMarkShuttingDown_DrainsThroughRegisterRoutes(t *testing.T) {
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
			t.Fatalf(
				"GET %s before the mark: want 200, got %d (%s)",
				path,
				rec.Code,
				rec.Body.String(),
			)
		}
	}

	fed.MarkShuttingDown()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz while draining: want 503, got %d (%s)", rec.Code, rec.Body.String())
	}

	var got health.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("drained body: %v", err)
	}

	if !got.ShuttingDown || got.Status != health.StatusFail {
		t.Errorf("drained body: want shutting-down fail, got %+v", got)
	}

	for _, path := range []string{"/healthz", "/startupz"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != http.StatusOK {
			t.Errorf(
				"GET %s while draining: want 200, got %d (%s)",
				path,
				rec.Code,
				rec.Body.String(),
			)
		}
	}
}

// TestStress_MarkShuttingDown_ConcurrentReads hammers the mark from many
// goroutines while readers assert the drain invariant the whole time.
// Run under -race it verifies the flag's atomic discipline: no data
// races, and once any reader observes ShuttingDown the status is fail —
// the overlay travels with the flag, never behind it.
func TestStress_MarkShuttingDown_ConcurrentReads(t *testing.T) {
	t.Parallel()

	fed := mustNew(t, []federation.Remote{{Name: "a", URL: healthyUpstream(t).server.URL}})

	stop := make(chan struct{})

	var readers sync.WaitGroup

	readers.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}

			got := fed.CachedResponse()

			if got.ShuttingDown && got.Status != health.StatusFail {
				t.Errorf("drained cached response: want fail, got %s", got.Status)

				return
			}
		}
	})

	const phases = 25

	for range phases {
		fed.MarkShuttingDown()

		if got := fed.CachedResponse(); !got.ShuttingDown {
			t.Fatalf("after MarkShuttingDown, ShuttingDown must be true")
		}
	}

	close(stop)
	readers.Wait()
}
