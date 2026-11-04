package aggregate_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	health "github.com/larsartmann/go-health"
)

// TestAggregateHandlers_MirrorMergedStatus asserts the handler status codes
// are a pure function of the merged roll-up: readiness 503 iff fail, startup
// 200 iff every latch is set, healthz 200 iff every latch is set and the
// merged roll-up is not fail.
func TestAggregateHandlers_MirrorMergedStatus(t *testing.T) {
	t.Parallel()

	fixtures := sourceFixtures(t)

	for _, keys := range orderedSelections() {
		t.Run(joinKeys(keys), func(t *testing.T) {
			t.Parallel()

			agg := mustAggregate(t, sourcesFor(fixtures, keys)...)

			readyRec := httptest.NewRecorder()
			agg.ReadinessHandler()(readyRec, fuzzRequest(t))

			wantReady := http.StatusOK
			if agg.CachedResponse().Status == health.StatusFail {
				wantReady = http.StatusServiceUnavailable
			}

			if readyRec.Code != wantReady {
				t.Errorf("readiness code = %d, want %d", readyRec.Code, wantReady)
			}

			startupRec := httptest.NewRecorder()
			agg.StartupHandler()(startupRec, fuzzRequest(t))

			wantStartup := http.StatusServiceUnavailable
			if agg.StartupComplete() {
				wantStartup = http.StatusOK
			}

			if startupRec.Code != wantStartup {
				t.Errorf("startup code = %d, want %d", startupRec.Code, wantStartup)
			}

			healthRec := httptest.NewRecorder()
			agg.Healthz()(healthRec, fuzzRequest(t))

			wantHealth := http.StatusOK
			if !agg.StartupComplete() || agg.CachedResponse().Status == health.StatusFail {
				wantHealth = http.StatusServiceUnavailable
			}

			if healthRec.Code != wantHealth {
				t.Errorf("healthz code = %d, want %d", healthRec.Code, wantHealth)
			}
		})
	}
}
