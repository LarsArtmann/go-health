package federation_test

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// TestCachedResponse_ScalarsDoNotSurviveMerge pins the merge rule that
// per-process scalars (Version, Uptime, InstanceID, Timestamp) are zeroed
// in the merged view: they would lie about the hub, not the remotes.
func TestCachedResponse_ScalarsDoNotSurviveMerge(t *testing.T) {
	t.Parallel()

	doc := health.Response{
		Status:         health.StatusPass,
		Version:        "9.9.9",
		InstanceID:     health.NewInstanceID("replica-1"),
		Uptime:         "1h",
		Timestamp:      time.Now(),
		TotalLatencyMs: 7,
		Checks:         map[string]health.Check{"db": {Status: health.StatusPass}},
	}

	upstream := serveDocument(t, doc)
	fed := mustNew(t, []federation.Remote{{Name: "a", URL: upstream.server.URL}})

	got := fed.CachedResponse()

	if got.Version != "" || !got.InstanceID.IsZero() || got.Uptime != "" ||
		!got.Timestamp.IsZero() {
		t.Errorf(
			"per-process scalars must not survive the merge: got version=%q instance=%q uptime=%q timestamp=%v",
			got.Version,
			got.InstanceID.Get(),
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
	if !dbCheck.StatusSince.Equal(wantSince) {
		t.Errorf("since: want %v, got %v", wantSince, dbCheck.StatusSince)
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
