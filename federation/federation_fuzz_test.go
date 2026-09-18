package federation_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// FuzzCachedResponse_ArbitraryRemoteBodies feeds arbitrary remote bodies
// through the fetch-and-merge path. Invariants that must hold for ANY
// input: no panic, a non-empty overall status, and no synthesized check
// other than the reachable fail row with a failing status — the hub must
// never render a remote's garbage as a healthy check.
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

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, body string) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		prober, err := federation.New(
			[]federation.Remote{{Name: "fuzz", URL: server.URL}},
			federation.WithTimeout(time.Second),
		)
		if err != nil {
			t.Fatalf("New: %v", err)
		}

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
				t.Fatalf("a refused remote must contribute only the reachable row, got %v", got.Checks)
			}

			return
		}

		for name, check := range got.Checks {
			if len(name) < 5 || name[:5] != "fuzz/" {
				t.Fatalf("accepted remote checks must be namespaced, got %q", name)
			}

			switch check.Status {
			case health.StatusPass, health.StatusWarn, health.StatusFail:
			default:
				t.Fatalf("accepted remote check %q has invalid status %q", name, check.Status)
			}
		}
	})
}
