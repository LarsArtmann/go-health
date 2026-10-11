package federation_test

import (
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

			return
		}

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
