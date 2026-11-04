package federation_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// TestCachedResponse_EndToEndWireContract is the federation contract
// test: a real go-health probe (injector-free constructor) runs with a
// background refresh loop and serves its readiness handler over HTTP,
// and a federated view over it reports the probe's checks namespaced,
// statuses preserved, and the probe-observed StatusSince intact.
func TestCachedResponse_EndToEndWireContract(t *testing.T) {
	t.Parallel()

	probe := health.NewWithHealthCheck(
		func(_ context.Context) map[string]error {
			return map[string]error{
				"postgres": nil,
				"queue":    errQueueRefused,
			}
		},
		health.WithRefreshInterval(5*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())

	t.Cleanup(func() {
		cancel()
	})

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("probe start: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for probe.CachedResponse().Status == "" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	upstream := newRemote(t, probe.ReadinessHandler())

	fed := mustNew(t, []federation.Remote{
		{Name: "core", URL: upstream.server.URL},
	})

	got := fed.CachedResponse()
	before := probe.CachedResponse()

	if want := health.StatusWarn; got.Status != want {
		t.Errorf("overall status: want %q, got %q", want, got.Status)
	}

	postgres, ok := got.Checks["core/postgres"]
	if !ok {
		t.Fatalf("want namespaced check core/postgres, got checks %v", got.Checks)
	}

	if postgres.Status != health.StatusPass {
		t.Errorf("core/postgres: want pass, got %q", postgres.Status)
	}

	queue := got.Checks["core/queue"]
	if queue.Status != health.StatusWarn || !strings.Contains(queue.Error, "connection refused") {
		t.Errorf("core/queue: want warn with cause, got %+v", queue)
	}

	if !postgres.StatusSince.Equal(before.Checks["postgres"].StatusSince) ||
		postgres.StatusSince.IsZero() {
		t.Errorf(
			"StatusSince must survive federation verbatim: remote %v, federated %v",
			before.Checks["postgres"].StatusSince,
			postgres.StatusSince,
		)
	}
}

// TestCachedResponse_Merge walks the merge table: worst-of across
// remotes, per-check namespacing, scalar non-survival, and every
// reachable-failure mode (transport, non-200, undecodable body, missing
// status, invalid per-check status) surfacing as one synthetic fail
// check with the cause.
func TestCachedResponse_Merge(t *testing.T) {
	t.Parallel()

	passDoc := health.Response{
		Status: health.StatusPass,
		Checks: map[string]health.Check{"cache": {Status: health.StatusPass}},
	}
	warnDoc := health.Response{
		Status:  health.StatusWarn,
		Version: "1.2.3",
		Uptime:  "4h",
		Checks:  map[string]health.Check{"cache": {Status: health.StatusWarn, Error: "degraded"}},
	}
	failDoc := health.Response{
		Status:         health.StatusFail,
		ShuttingDown:   true,
		TotalLatencyMs: 42,
		Checks:         map[string]health.Check{"db": {Status: health.StatusFail, Error: "down"}},
	}

	tests := []struct {
		name          string
		remotes       []federation.Remote
		servers       []jsonRemote
		wantStatus    health.Status
		wantShutDown  bool
		wantLatencyMs int64
		wantChecks    map[string]health.Check
	}{
		{
			name:       "single healthy remote",
			remotes:    []federation.Remote{{Name: "a", URL: ""}},
			servers:    []jsonRemote{serveDocument(t, passDoc)},
			wantStatus: health.StatusPass,
			wantChecks: map[string]health.Check{"a/cache": {Status: health.StatusPass}},
		},
		{
			name: "worst-of warn beats pass",
			remotes: []federation.Remote{
				{Name: "a", URL: ""},
				{Name: "b", URL: ""},
			},
			servers:    []jsonRemote{serveDocument(t, passDoc), serveDocument(t, warnDoc)},
			wantStatus: health.StatusWarn,
			wantChecks: map[string]health.Check{
				"a/cache": {Status: health.StatusPass},
				"b/cache": {Status: health.StatusWarn, Error: "degraded"},
			},
		},
		{
			name:    "off check is accepted and stays pass-tier",
			remotes: []federation.Remote{{Name: "a", URL: ""}},
			servers: []jsonRemote{serveDocument(t, health.Response{
				Status: health.StatusPass,
				Checks: map[string]health.Check{
					"db": {
						Status: health.StatusOff,
						Error:  "not configured: set database.url to enable",
					},
				},
			})},
			wantStatus: health.StatusPass,
			wantChecks: map[string]health.Check{
				"a/db": {
					Status: health.StatusOff,
					Error:  "not configured: set database.url to enable",
				},
			},
		},
		{
			name: "shutting-down remote forces fail",
			remotes: []federation.Remote{
				{Name: "a", URL: ""},
				{Name: "b", URL: ""},
			},
			servers:       []jsonRemote{serveDocument(t, passDoc), serveDocument(t, failDoc)},
			wantStatus:    health.StatusFail,
			wantShutDown:  true,
			wantLatencyMs: 42,
			wantChecks: map[string]health.Check{
				"a/cache": {Status: health.StatusPass},
				"b/db":    {Status: health.StatusFail, Error: "down"},
			},
		},
		{
			name:       "unreachable remote yields synthetic fail check",
			remotes:    []federation.Remote{{Name: "ghost", URL: ""}},
			servers:    []jsonRemote{newRemote(t, http.NotFoundHandler().ServeHTTP)},
			wantStatus: health.StatusFail,
			wantChecks: map[string]health.Check{
				"ghost/reachable": {Status: health.StatusFail},
			},
		},
		{
			name:    "undecodable body yields synthetic fail check",
			remotes: []federation.Remote{{Name: "weird", URL: ""}},
			servers: []jsonRemote{newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("<html>not json</html>"))
			})},
			wantStatus: health.StatusFail,
			wantChecks: map[string]health.Check{
				"weird/reachable": {Status: health.StatusFail},
			},
		},
		{
			name:    "status-less document yields synthetic fail check",
			remotes: []federation.Remote{{Name: "wrong", URL: ""}},
			servers: []jsonRemote{newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"checks":{}}`))
			})},
			wantStatus: health.StatusFail,
			wantChecks: map[string]health.Check{
				"wrong/reachable": {Status: health.StatusFail},
			},
		},
		{
			name:    "invalid per-check status is refused whole",
			remotes: []federation.Remote{{Name: "liar", URL: ""}},
			servers: []jsonRemote{newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"status":"pass","checks":{"db":{"status":"sorta"}}}`))
			})},
			wantStatus: health.StatusFail,
			wantChecks: map[string]health.Check{
				"liar/reachable": {Status: health.StatusFail},
			},
		},
		{
			// SourceStatuses reports the document's own roll-up verbatim and
			// Status.Rank scores an unknown value pass-tier, so an unvalidated
			// roll-up would render as a healthy source card.
			name:    "invalid roll-up status is refused whole",
			remotes: []federation.Remote{{Name: "liar", URL: ""}},
			servers: []jsonRemote{newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"status":"healthy","checks":{}}`))
			})},
			wantStatus: health.StatusFail,
			wantChecks: map[string]health.Check{
				"liar/reachable": {Status: health.StatusFail},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			remotes := make([]federation.Remote, 0, len(tt.remotes))
			for i := range tt.remotes {
				remotes = append(remotes, federation.Remote{
					Name: tt.remotes[i].Name,
					URL:  tt.servers[i].server.URL,
				})
			}

			assertMerged(t, mustNew(t, remotes).CachedResponse(), mergedExpectation{
				status:       tt.wantStatus,
				shuttingDown: tt.wantShutDown,
				latencyMs:    tt.wantLatencyMs,
				checks:       tt.wantChecks,
			})
		})
	}
}

// mergedExpectation is the assertion payload for assertMerged.
type mergedExpectation struct {
	status       health.Status
	shuttingDown bool
	latencyMs    int64
	checks       map[string]health.Check
}

// assertMerged compares a merged response against the expectation:
// exact status/shutdown/latency, every wanted check present with the
// right status (substring error match), and no extra checks.
func assertMerged(t *testing.T, got health.Response, want mergedExpectation) {
	t.Helper()

	if got.Status != want.status {
		t.Errorf("status: want %q, got %q", want.status, got.Status)
	}

	if got.ShuttingDown != want.shuttingDown {
		t.Errorf("shutting down: want %v, got %v", want.shuttingDown, got.ShuttingDown)
	}

	if got.TotalLatencyMs != want.latencyMs {
		t.Errorf("latency: want %d, got %d", want.latencyMs, got.TotalLatencyMs)
	}

	for key, wantCheck := range want.checks {
		gotCheck, ok := got.Checks[key]
		if !ok {
			t.Errorf("missing check %q in %v", key, got.Checks)

			continue
		}

		if gotCheck.Status != wantCheck.Status {
			t.Errorf("check %q: want status %q, got %q", key, wantCheck.Status, gotCheck.Status)
		}

		if wantCheck.Error != "" && !strings.Contains(gotCheck.Error, wantCheck.Error) {
			t.Errorf(
				"check %q: error must contain %q, got %q",
				key,
				wantCheck.Error,
				gotCheck.Error,
			)
		}
	}

	if len(got.Checks) != len(want.checks) {
		t.Errorf("check count: want %d, got %d (%v)", len(want.checks), len(got.Checks), got.Checks)
	}
}
