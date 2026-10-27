package federation_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// FuzzCachedResponse_ArbitraryRemoteBodies feeds arbitrary remote bodies
// through the fetch-and-merge path. Invariants that must hold for ANY
// input: no panic, a non-empty overall status, and no synthesized check
// other than the reachable fail row with a failing status — the hub must
// never render a remote's garbage as a healthy check. One shared server
// backs the whole campaign: a listener per iteration exhausts ports. A
// second prober, marked draining once at setup, runs the same bodies
// through the drain overlay: whatever the merge produced, the drained
// read must answer fail with shutting_down set.
func FuzzCachedResponse_ArbitraryRemoteBodies(f *testing.F) {
	seeds := []string{
		`{"status":"pass","checks":{}}`,
		`{"status":"fail","checks":{"db":{"status":"fail","error":"down"}}}`,
		`{"status":"pass","checks":{"x":{"status":""}}}`,
		`{"status":"warn","checks":{"q":{"status":"warn","since":"2026-09-18T08:00:00Z","duration_ns":42}}}`,
		`not json`,
		``,
		`{"status":"pass","checks":{"a":null}}`,
	}

	var bodyMu sync.Mutex

	body := []byte(seeds[0])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		bodyMu.Lock()
		payload := body
		bodyMu.Unlock()

		_, _ = w.Write(payload)
	}))
	defer server.Close()

	prober, err := federation.New(
		[]federation.Remote{{Name: "fuzz", URL: server.URL}},
		federation.WithTimeout(time.Second),
	)
	if err != nil {
		f.Fatalf("New: %v", err)
	}

	// Draining twin: same remote, marked once before the campaign. Every
	// read below must carry the overlay regardless of what the body did to
	// the merge — the mark beats any remote document.
	draining, err := federation.New(
		[]federation.Remote{{Name: "fuzz", URL: server.URL}},
		federation.WithTimeout(time.Second),
	)
	if err != nil {
		f.Fatalf("New draining twin: %v", err)
	}

	draining.MarkShuttingDown()

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, next string) {
		bodyMu.Lock()
		body = []byte(next)
		bodyMu.Unlock()

		got := prober.CachedResponse()

		if got.Status == "" {
			t.Fatal("merged status must never be empty")
		}

		reachable, refused := got.Checks["fuzz/reachable"]
		if refused {
			if reachable.Status != health.StatusFail {
				t.Fatalf("synthetic reachable check must fail, got %q", reachable.Status)
			}

			if len(got.Checks) != 1 {
				t.Fatalf(
					"a refused remote must contribute only the reachable row, got %v",
					got.Checks,
				)
			}
		} else {
			assertAcceptedChecksValid(t, got)
		}

		drained := draining.CachedResponse()
		if !drained.ShuttingDown || drained.Status != health.StatusFail {
			t.Fatalf(
				"drained read must overlay fail+shutting_down over any merge result, got status %q, shutting_down %t",
				drained.Status,
				drained.ShuttingDown,
			)
		}
	})
}

// assertAcceptedChecksValid pins the invariants for checks that came from a
// decoded (not refused) remote body: every key namespaced under the remote,
// every status inside the frozen enum. The hub must never render a remote's
// garbage as a healthy check.
func assertAcceptedChecksValid(t *testing.T, got health.Response) {
	t.Helper()

	for name, check := range got.Checks {
		if !strings.HasPrefix(name, "fuzz/") {
			t.Fatalf("accepted remote checks must be namespaced, got %q", name)
		}

		switch check.Status {
		case health.StatusPass, health.StatusWarn, health.StatusFail, health.StatusOff:
		default:
			t.Fatalf("accepted remote check %q has invalid status %q", name, check.Status)
		}
	}
}

// FuzzSourceStatuses_Invariants drives the per-remote accessor with arbitrary
// remote documents. The accessor answers a different question than the merged
// view — "how is each remote overall?" — so its invariants are about the
// per-remote map, not the roll-up: exactly one entry keyed by the remote name,
// never an empty or unrecognized status, deterministic across two reads of the
// same body, and fail whenever the merged view refused the remote at all. The
// hub's own drain must not enter the map: the drained twin runs the same body
// and must produce an identical map while its merged roll-up reads fail.
//
// The merged roll-up is deliberately NOT asserted against the per-remote map:
// the merge derives a status from the checks while SourceStatuses reports the
// document's own claim, and a body may legally disagree with itself.
func FuzzSourceStatuses_Invariants(f *testing.F) {
	seeds := []string{
		`{"status":"pass","checks":{}}`,
		`{"status":"fail","checks":{"db":{"status":"fail","error":"down"}}}`,
		`{"status":"warn","checks":{"q":{"status":"warn"}}}`,
		`{"status":"off","checks":{"analytics":{"status":"off"}}}`,
		`{"status":"pass","checks":{"db":{"status":"fail"}}}`,
		`{"status":"pass","shutting_down":true,"checks":{}}`,
		`{"status":"bogus","checks":{}}`,
		`not json`,
		``,
	}

	var bodyMu sync.Mutex

	body := []byte(seeds[0])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		bodyMu.Lock()
		payload := body
		bodyMu.Unlock()

		_, _ = w.Write(payload)
	}))
	defer server.Close()

	remotes := []federation.Remote{{Name: "fuzz", URL: server.URL}}

	prober, err := federation.New(remotes, federation.WithTimeout(time.Second))
	if err != nil {
		f.Fatalf("New: %v", err)
	}

	draining, err := federation.New(remotes, federation.WithTimeout(time.Second))
	if err != nil {
		f.Fatalf("New draining twin: %v", err)
	}

	draining.MarkShuttingDown()

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, next string) {
		bodyMu.Lock()
		body = []byte(next)
		bodyMu.Unlock()

		statuses := prober.SourceStatuses()

		if len(statuses) != 1 {
			t.Fatalf("statuses = %v, want exactly one entry for the single remote", statuses)
		}

		status, ok := statuses["fuzz"]
		if !ok {
			t.Fatalf("statuses = %v, want the key %q", statuses, "fuzz")
		}

		switch status {
		case health.StatusPass, health.StatusWarn, health.StatusFail, health.StatusOff:
		default:
			t.Fatalf("per-remote status %q is not a known status", status)
		}

		again := prober.SourceStatuses()
		if len(again) != len(statuses) || again["fuzz"] != status {
			t.Fatalf("SourceStatuses is not deterministic: %v then %v", statuses, again)
		}

		if _, refused := prober.CachedResponse().Checks["fuzz/reachable"]; refused &&
			status != health.StatusFail {
			t.Fatalf("refused remote reports %q, want fail", status)
		}

		if drained := draining.SourceStatuses(); len(drained) != 1 || drained["fuzz"] != status {
			t.Fatalf("hub drain rewrote the per-remote map: %v vs %v", drained, statuses)
		}
	})
}

// FuzzHealthz_CodeMatchesBody drives the single-endpoint handler with arbitrary
// remote documents. The status code must be a pure function of the body the
// handler actually wrote: 503 exactly when that body's roll-up is fail, 200 for
// pass and warn, and never any other code. Reading both off the same response
// is the point — a future overlay that changed one without the other fails
// here, and so does a served body that disagrees with its own transport status.
//
// A hub marked draining must answer 503 for every body, because the drain
// overlay is a property of this hub rather than of any remote document.
func FuzzHealthz_CodeMatchesBody(f *testing.F) {
	seeds := []string{
		`{"status":"pass","checks":{}}`,
		`{"status":"warn","checks":{"q":{"status":"warn"}}}`,
		`{"status":"fail","checks":{"db":{"status":"fail","error":"down"}}}`,
		`{"status":"off","checks":{"analytics":{"status":"off"}}}`,
		`{"status":"pass","shutting_down":true,"checks":{}}`,
		`{"status":"bogus","checks":{}}`,
		`not json`,
		``,
	}

	var bodyMu sync.Mutex

	body := []byte(seeds[0])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		bodyMu.Lock()
		payload := body
		bodyMu.Unlock()

		_, _ = w.Write(payload)
	}))
	defer server.Close()

	remotes := []federation.Remote{{Name: "fuzz", URL: server.URL}}

	prober, err := federation.New(remotes, federation.WithTimeout(time.Second))
	if err != nil {
		f.Fatalf("New: %v", err)
	}

	draining, err := federation.New(remotes, federation.WithTimeout(time.Second))
	if err != nil {
		f.Fatalf("New draining twin: %v", err)
	}

	draining.MarkShuttingDown()

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, next string) {
		bodyMu.Lock()
		body = []byte(next)
		bodyMu.Unlock()

		assertHealthzCodeMatchesBody(t, prober.Healthz())

		got := serveHealthz(t, draining.Healthz())
		if got.code != http.StatusServiceUnavailable {
			t.Fatalf("drained hub answered %d, want 503", got.code)
		}

		if got.status != health.StatusFail {
			t.Fatalf("drained hub body status = %q, want fail", got.status)
		}
	})
}

// healthzOutcome is one served single-endpoint answer, reduced to the two
// facts the invariant relates: the transport code and the body's roll-up.
type healthzOutcome struct {
	code   int
	status health.Status
}

// serveHealthz runs the handler once and decodes what it wrote. It fails the
// test on a body that is not a go-health response, so callers only ever reason
// about a well-formed answer.
func serveHealthz(t *testing.T, handler http.HandlerFunc) healthzOutcome {
	t.Helper()

	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	var decoded health.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("healthz body is not a health response: %v (body %q)", err, recorder.Body.String())
	}

	if decoded.Status == "" {
		t.Fatalf("healthz body has no status (body %q)", recorder.Body.String())
	}

	return healthzOutcome{code: recorder.Code, status: decoded.Status}
}

// assertHealthzCodeMatchesBody pins the pure-function contract: the code
// follows the served roll-up, and nothing else.
func assertHealthzCodeMatchesBody(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	got := serveHealthz(t, handler)

	switch got.code {
	case http.StatusOK:
		if got.status == health.StatusFail {
			t.Fatalf(
				"healthz answered 200 with a fail body (status %q)",
				got.status,
			)
		}
	case http.StatusServiceUnavailable:
		if got.status != health.StatusFail {
			t.Fatalf(
				"healthz answered 503 with status %q, want fail",
				got.status,
			)
		}
	default:
		t.Fatalf("healthz answered %d, want 200 or 503", got.code)
	}
}
