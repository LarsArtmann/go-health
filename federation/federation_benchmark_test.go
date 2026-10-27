package federation_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/larsartmann/go-health/federation"
)

// BenchmarkReadinessHandler_Draining completes the handler benchmark
// matrix with the drain overlay: after MarkShuttingDown, readiness still
// performs the merge-on-read fetch (fresh 503 bodies are the feature), so
// this measures the drained path end to end — one parallel fetch per
// remote plus the overlay itself, which is a single atomic load. Compare
// against an undrained readiness run to see the overlay is not the cost.
func BenchmarkReadinessHandler_Draining(b *testing.B) {
	upstream := healthyUpstream(b)

	fed := mustNew(b, []federation.Remote{{Name: "a", URL: upstream.server.URL}})

	fed.MarkShuttingDown()

	handler := fed.ReadinessHandler()

	r, err := http.NewRequest(http.MethodGet, "/readyz", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()

		for pb.Next() {
			w.Body.Reset()
			handler(w, r)
		}
	})
}

// BenchmarkSourceStatuses measures the per-remote accessor end to end: the
// same one-parallel-fetch pass the merged read performs, folded into a map
// instead of a merged document. Compare against a readiness run to see that
// the per-remote fold adds no fetch cost of its own — the interesting cases
// are one, two, four and eight remotes, because the fan-out is what scales.
func BenchmarkSourceStatuses(b *testing.B) {
	for _, remotes := range []int{1, 2, 4, 8} {
		b.Run(remoteCountName(remotes), func(b *testing.B) {
			fed := multiRemoteProber(b, remotes)

			b.ResetTimer()

			for b.Loop() {
				fed.SourceStatuses()
			}
		})
	}
}

// BenchmarkHealthz covers the three single-endpoint verdicts: a healthy hub
// (200 with a merged body), a degraded hub (warn is still 200), and a drained
// hub (503). All three fetch per request, so the numbers track readiness plus
// the overlay, which is a single atomic load.
func BenchmarkHealthz(b *testing.B) {
	cases := []struct {
		name  string
		doc   string
		drain bool
	}{
		{name: "healthy", doc: `{"status":"pass","checks":{}}`},
		{name: "warn", doc: `{"status":"warn","checks":{"q":{"status":"warn"}}}`},
		{name: "draining", doc: `{"status":"pass","checks":{}}`, drain: true},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			upstream := newRemote(b, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.doc))
			})

			fed := mustNew(b, []federation.Remote{{Name: "a", URL: upstream.server.URL}})
			if tc.drain {
				fed.MarkShuttingDown()
			}

			handler := fed.Healthz()

			r, err := http.NewRequest(http.MethodGet, "/healthz", nil)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				w := httptest.NewRecorder()

				for pb.Next() {
					w.Body.Reset()
					handler(w, r)
				}
			})
		})
	}
}

// multiRemoteProber builds a prober over n independent healthy upstreams,
// named a, b, c, ... so the merge sees distinct sources without a shared
// server serializing the fan-out.
func multiRemoteProber(b *testing.B, n int) *federation.Prober {
	b.Helper()

	remotes := make([]federation.Remote, 0, n)

	for i := range n {
		upstream := healthyUpstream(b)
		remotes = append(remotes, federation.Remote{
			Name: string(rune('a' + i)),
			URL:  upstream.server.URL,
		})
	}

	return mustNew(b, remotes)
}

// remoteCountName names the fan-out subtests by remote count, so a benchmark
// run reads as the scaling table it is.
func remoteCountName(n int) string {
	return strconv.Itoa(n) + "_remotes"
}
