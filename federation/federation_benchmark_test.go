package federation_test

import (
	"net/http"
	"net/http/httptest"
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
