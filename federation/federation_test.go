package federation_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// dashboardProber is the consumer-side interface go-health-dashboard
// defines (dashboard.Prober). go-health cannot import the dashboard —
// the dependency points the other way — so the conformance is asserted
// structurally: if this compiles, a *federation.Prober renders in the
// dashboard like any local probe.
type dashboardProber interface {
	CachedResponse() health.Response
	RefreshInterval() time.Duration
	LivenessHandler() http.HandlerFunc
	ReadinessHandler() http.HandlerFunc
	StartupHandler() http.HandlerFunc
}

var _ dashboardProber = (*federation.Prober)(nil)

// errQueueRefused is the canned non-critical failure for probe-backed
// upstreams (static, so the classification is errors.Is-able).
var errQueueRefused = errors.New("connection refused")

// --- Test helpers ---.

// jsonRemote stands up one upstream HTTP server for the test's lifetime.
type jsonRemote struct {
	server *httptest.Server
}

func newRemote(t *testing.T, handler http.HandlerFunc) jsonRemote {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return jsonRemote{server: server}
}

func serveDocument(t *testing.T, doc health.Response) jsonRemote {
	t.Helper()

	payload, err := json.Marshal(doc, json.Deterministic(true))
	if err != nil {
		t.Fatalf("marshal test document: %v", err)
	}

	return newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	})
}

// healthyUpstream serves a valid pass document and rejects requests
// without the Accept header federation must send — the content
// negotiation contract with dashboard-hosted remotes.
func healthyUpstream(t *testing.T) jsonRemote {
	t.Helper()

	return newRemote(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			w.WriteHeader(http.StatusNotAcceptable)

			return
		}

		_, _ = w.Write([]byte(`{"status":"pass","checks":{}}`))
	})
}

func mustNew(
	t *testing.T,
	remotes []federation.Remote,
	opts ...federation.Option,
) *federation.Prober {
	t.Helper()

	prober, err := federation.New(remotes, opts...)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	return prober
}

// darkUpstream is an address with nothing listening: connection refused.
func darkUpstream() string {
	return "http://127.0.0.1:1/readyz"
}

// TestNew_Validation walks the construction contract: every invalid
// configuration is rejected with the documented sentinel and a cause
// naming the offending value; the valid baseline constructs.
func TestNew_Validation(t *testing.T) {
	t.Parallel()

	good := federation.Remote{Name: "ok", URL: "http://localhost:9101/readyz"}

	tests := []struct {
		name    string
		remotes []federation.Remote
		opts    []federation.Option
		wantErr error
	}{
		{
			name:    "no remotes",
			remotes: nil,
			wantErr: federation.ErrNoRemotes,
		},
		{
			name:    "empty name",
			remotes: []federation.Remote{{Name: "", URL: "http://x/readyz"}},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name:    "name contains slash",
			remotes: []federation.Remote{{Name: "a/b", URL: "http://x/readyz"}},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name:    "empty URL",
			remotes: []federation.Remote{{Name: "nas", URL: ""}},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name: "duplicate name",
			remotes: []federation.Remote{
				{Name: "nas", URL: "http://nas/readyz"},
				{Name: "nas", URL: "http://other/readyz"},
			},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name:    "unparseable URL",
			remotes: []federation.Remote{{Name: "nas", URL: "http://[::1"}},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name:    "non-http scheme",
			remotes: []federation.Remote{{Name: "nas", URL: "ftp://nas/readyz"}},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name:    "hostless URL",
			remotes: []federation.Remote{{Name: "nas", URL: "http:///readyz"}},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name:    "zero timeout",
			remotes: []federation.Remote{good},
			opts:    []federation.Option{federation.WithTimeout(0)},
			wantErr: federation.ErrInvalidTimeout,
		},
		{
			name:    "negative timeout",
			remotes: []federation.Remote{good},
			opts:    []federation.Option{federation.WithTimeout(-time.Second)},
			wantErr: federation.ErrInvalidTimeout,
		},
		{
			name:    "valid baseline",
			remotes: []federation.Remote{good},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prober, err := federation.New(tt.remotes, tt.opts...)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("New: unexpected error: %v", err)
				}

				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("New: want %v, got %v", tt.wantErr, err)
			}

			if prober != nil {
				t.Errorf("New: want nil prober on error, got %+v", prober)
			}
		})
	}
}

// TestCachedResponse_EndToEndWireContract is the federation contract
// test: a real go-health probe (injector-free constructor) runs with a
// background refresh loop and serves its readiness handler over HTTP,
// and a federated view over it reports the probe's checks namespaced,
// statuses preserved, and the probe-observed Since intact.
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

	if !postgres.Since.Equal(before.Checks["postgres"].Since) || postgres.Since.IsZero() {
		t.Errorf(
			"Since must survive federation verbatim: remote %v, federated %v",
			before.Checks["postgres"].Since,
			postgres.Since,
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

// TestCachedResponse_ScalarsDoNotSurviveMerge pins the merge rule that
// per-process scalars (Version, Uptime, InstanceID, Timestamp) are zeroed
// in the merged view: they would lie about the hub, not the remotes.
func TestCachedResponse_ScalarsDoNotSurviveMerge(t *testing.T) {
	t.Parallel()

	doc := health.Response{
		Status:         health.StatusPass,
		Version:        "9.9.9",
		InstanceID:     "replica-1",
		Uptime:         "1h",
		Timestamp:      time.Now(),
		TotalLatencyMs: 7,
		Checks:         map[string]health.Check{"db": {Status: health.StatusPass}},
	}

	upstream := serveDocument(t, doc)
	fed := mustNew(t, []federation.Remote{{Name: "a", URL: upstream.server.URL}})

	got := fed.CachedResponse()

	if got.Version != "" || got.InstanceID != "" || got.Uptime != "" || !got.Timestamp.IsZero() {
		t.Errorf(
			"per-process scalars must not survive the merge: got version=%q instance=%q uptime=%q timestamp=%v",
			got.Version,
			got.InstanceID,
			got.Uptime,
			got.Timestamp,
		)
	}

	if got.TotalLatencyMs != 7 {
		t.Errorf("TotalLatencyMs rides the merge: want 7, got %d", got.TotalLatencyMs)
	}
}

// TestCachedResponse_WireFieldsDecode pins the JSON decode of the
// optional per-check metadata: since (RFC3339) and duration_ns (int64)
// arrive from the wire verbatim — the fields v0.2.0 added.
func TestCachedResponse_WireFieldsDecode(t *testing.T) {
	t.Parallel()

	const body = `{"status":"pass","checks":{"db":{"status":"pass",` +
		`"since":"2026-09-18T08:00:00Z","duration_ns":1500}}}`

	upstream := newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	})

	fed := mustNew(t, []federation.Remote{{Name: "nas", URL: upstream.server.URL}})

	got := fed.CachedResponse()

	dbCheck, ok := got.Checks["nas/db"]
	if !ok {
		t.Fatalf("missing nas/db in %v", got.Checks)
	}

	wantSince := time.Date(2026, 9, 18, 8, 0, 0, 0, time.UTC)
	if !dbCheck.Since.Equal(wantSince) {
		t.Errorf("since: want %v, got %v", wantSince, dbCheck.Since)
	}

	if dbCheck.DurationNanos != 1500 {
		t.Errorf("duration_ns: want 1500, got %d", dbCheck.DurationNanos)
	}
}

// TestCachedResponse_FetchTimeout bounds the cost of a hung remote: with
// a per-fetch deadline shorter than the upstream's delay, the read fails
// with the deadline cause instead of blocking forever.
func TestCachedResponse_FetchTimeout(t *testing.T) {
	t.Parallel()

	slow := newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(`{"status":"pass","checks":{}}`))
	})

	fed := mustNew(t,
		[]federation.Remote{{Name: "slow", URL: slow.server.URL}},
		federation.WithTimeout(50*time.Millisecond),
	)

	got := fed.CachedResponse()

	if got.Status != health.StatusFail {
		t.Fatalf("hung remote: want overall fail, got %q", got.Status)
	}

	reachable := got.Checks["slow/reachable"]
	if reachable.Status != health.StatusFail ||
		!strings.Contains(reachable.Error, "context deadline exceeded") {
		t.Errorf("reachable check must carry the deadline cause, got %+v", reachable)
	}
}

// TestCachedResponse_ConcurrentReads exercises concurrent CachedResponse
// calls against shared upstreams — the race detector (gates run -race)
// is the assertion; the counting handler additionally proves every read
// fetched every remote.
func TestCachedResponse_ConcurrentReads(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64

	upstream := newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"status":"pass","checks":{"db":{"status":"pass"}}}`))
	})

	fed := mustNew(t, []federation.Remote{{Name: "a", URL: upstream.server.URL}})

	const readers = 8

	var wg sync.WaitGroup

	for range readers {
		wg.Go(func() {
			if got := fed.CachedResponse(); got.Status != health.StatusPass {
				t.Errorf("concurrent read: want pass, got %q", got.Status)
			}
		})
	}

	wg.Wait()

	if got := requests.Load(); got != readers {
		t.Errorf("fetch-per-read contract: want %d upstream requests, got %d", readers, got)
	}
}

// TestRefreshInterval_IsZero pins the live-fetch contract: federation has
// no cadence of its own, so consumers set their own read cadence.
func TestRefreshInterval_IsZero(t *testing.T) {
	t.Parallel()

	fed := mustNew(t, []federation.Remote{{Name: "a", URL: "http://localhost:9101/readyz"}})

	if got := fed.RefreshInterval(); got != 0 {
		t.Errorf("RefreshInterval: want 0, got %v", got)
	}
}

// TestStartupComplete_LatchesOnFirstSuccess walks the one-way startup
// latch: a remote that has never answered keeps startup incomplete; one
// successful fetch latches it permanently — including across later
// failures.
func TestStartupComplete_LatchesOnFirstSuccess(t *testing.T) {
	t.Parallel()

	var healthy atomic.Bool

	healthy.Store(false)

	flaky := newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
		if !healthy.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		_, _ = w.Write([]byte(`{"status":"pass","checks":{}}`))
	})

	fed := mustNew(t, []federation.Remote{{Name: "flaky", URL: flaky.server.URL}})

	_ = fed.CachedResponse()

	if fed.StartupComplete() {
		t.Fatal("a remote that has never answered must keep startup incomplete")
	}

	healthy.Store(true)

	_ = fed.CachedResponse()

	if !fed.StartupComplete() {
		t.Fatal("a successful fetch must latch startup complete")
	}

	healthy.Store(false)

	_ = fed.CachedResponse()

	if !fed.StartupComplete() {
		t.Fatal("startup latch is one-way: a later failure must not un-latch")
	}
}

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
