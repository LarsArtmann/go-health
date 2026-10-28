package aggregate_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	aggregate "github.com/larsartmann/go-health/aggregate"
	merge "github.com/larsartmann/go-health/internal/merge"
)

// fuzzRequest builds a plain GET request for handler smoke assertions.
func fuzzRequest(t *testing.T) *http.Request {
	t.Helper()

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/healthz",
		nil,
	)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	return request
}

// fuzzSourceSpec controls one fuzz-built source's cached health view.
type fuzzSourceSpec struct {
	checkName  string
	critical   bool
	failing    bool
	instanceID string
	// refreshInterval > 0 arms a background refresh loop (cache mode); the
	// interval is chosen far beyond an iteration's lifetime so the cached
	// snapshot stays at the boot state and merge assertions stay exact.
	refreshInterval time.Duration
	// liveThrottle > 0 puts a live-mode source on the throttled handler path.
	// The aggregate itself reads caches, never source handlers, so this pins
	// construction coexistence: aggregate invariants must not depend on the
	// source's freshness mode.
	liveThrottle time.Duration
}

// newFuzzSource builds a started probe whose cached response has a controlled
// status: pass when the service is healthy, fail when it fails critically,
// warn when it fails non-critically.
func newFuzzSource(t *testing.T, spec fuzzSourceSpec) *health.Probe {
	t.Helper()

	results := func(context.Context) map[string]error {
		if spec.failing {
			return map[string]error{spec.checkName: errUnhealthy}
		}

		return map[string]error{spec.checkName: nil}
	}

	options := []health.Option{
		health.WithRefreshInterval(spec.refreshInterval),
		health.WithInstanceID(health.NewInstanceID(spec.instanceID)),
	}

	if spec.liveThrottle > 0 {
		options = append(options, health.WithLiveThrottle(spec.liveThrottle))
	}

	if spec.critical {
		options = append(options, health.WithCriticalServices(spec.checkName))
	}

	probe := health.NewWithHealthCheck(results, options...)

	if spec.refreshInterval > 0 {
		t.Cleanup(func() { probe.Shutdown() })
	}

	if err := probe.Start(context.Background()); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}

	return probe
}

// FuzzAggregateMergeInvariants fuzzes source topology through the merge-on-read
// path. The fold semantics (worst-of status, shutdown overlay, namespacing,
// scalar drop, latency max) live in the shared [merge.Responses] primitive and
// its own suite; this harness pins the aggregate-specific surface under any
// input: the mapping (one Source per probe, carrying the probe's cached view
// verbatim, no fetch errors) — asserted bit-for-bit against the primitive —
// the startup AND, and the three handlers' documented status codes. Merge
// results and handler codes never depend on the sources' freshness modes.
//
// Source names containing "/" are out of contract: aggregate.New rejects
// them (they would blur the "source/check" grouping axis and could alias
// another source's namespace), so this harness skips them and construction
// rejections stay covered by unit tests. The never-started zero-cache source
// path is likewise unit-pinned (TestCachedResponse_NeverStartedSource)
// rather than widening this fuzz signature, which would invalidate the
// accumulated corpus in testdata/fuzz.
func FuzzAggregateMergeInvariants(f *testing.F) {
	f.Add("edge-a", "svc", "edge-b", "svc", true, false, true, true, "pod-1")
	f.Add("api", "db", "worker", "db", true, true, false, true, "")
	f.Add("s1", "x/y", "s2", "x/y", false, true, true, false, "i-0abc")
	f.Add("a", "", "b", "", false, false, false, false, "replica-7")
	// Golden-fixture seed: mirrors testdata/readiness_response.golden
	// (non-critical "cache" failing with "connection refused" over a passing
	// "db", instance pod-7f9c) plus a cache-mode source name (even length) so
	// the corpus always carries the shipped wire shape in both freshness
	// modes.
	f.Add("golden", "cache", "golden-alt", "db", false, true, false, false, "pod-7f9c")

	f.Fuzz(func(
		t *testing.T,
		nameOne, checkNameOne, nameTwo, checkNameTwo string,
		critOne, failOne, critTwo, failTwo bool,
		instanceID string,
	) {
		probeOne, probeTwo := buildFuzzSources(t, fuzzSourceSpec{
			checkName: checkNameOne, critical: critOne, failing: failOne, instanceID: instanceID,
		}, nameOne, fuzzSourceSpec{
			checkName: checkNameTwo, critical: critTwo, failing: failTwo, instanceID: instanceID,
		}, nameTwo)

		agg, err := aggregate.New(
			aggregate.Source{Name: nameOne, Probe: probeOne},
			aggregate.Source{Name: nameTwo, Probe: probeTwo},
		)
		if err != nil {
			t.Fatalf("aggregate.New: %v", err)
		}

		cachedViews := map[string]health.Response{
			nameOne: probeOne.CachedResponse(),
			nameTwo: probeTwo.CachedResponse(),
		}

		merged := agg.CachedResponse()

		mapped := []merge.Source{
			{Name: nameOne, Response: cachedViews[nameOne]},
			{Name: nameTwo, Response: cachedViews[nameTwo]},
		}

		if want := merge.Responses(mapped); !reflect.DeepEqual(merged, want) {
			t.Fatalf(
				"merged response drifts from the shared fold:\nwant %+v\ngot  %+v",
				want,
				merged,
			)
		}

		wantStartup := probeOne.StartupComplete() && probeTwo.StartupComplete()
		if agg.StartupComplete() != wantStartup {
			t.Fatalf("startup AND: want %v, got %v", wantStartup, agg.StartupComplete())
		}

		assertAggregateHandlers(t, agg, merged.Status, wantStartup)
	})
}

// buildFuzzSources constructs the two fuzz probes, skipping the iteration for
// inputs aggregate.New rejects: empty, duplicate, or slash-containing source
// names (construction rejections are covered by unit tests instead).
func buildFuzzSources(
	t *testing.T,
	specOne fuzzSourceSpec, nameOne string,
	specTwo fuzzSourceSpec, nameTwo string,
) (*health.Probe, *health.Probe) {
	t.Helper()

	if nameOne == "" || nameTwo == "" || nameOne == nameTwo ||
		strings.Contains(nameOne, "/") || strings.Contains(nameTwo, "/") {
		t.Skip("invalid or colliding source names are covered by unit tests")
	}

	// Freshness modes are derived from name parity, not explicit fuzz
	// parameters, so the accumulated corpus signature in testdata/fuzz stays
	// loadable: source one alternates live/cache mode (cache mode arms a
	// background refresh loop whose interval far outlives the iteration, so
	// the boot snapshot stays the merged input), source two alternates
	// plain/throttled live mode. The pinned invariant: merge results and
	// handler status codes never depend on the sources' freshness modes.
	if len(nameOne)%2 == 0 {
		specOne.refreshInterval = time.Hour
	}

	if len(nameTwo)%2 == 0 {
		specTwo.liveThrottle = time.Millisecond
	}

	return newFuzzSource(t, specOne), newFuzzSource(t, specTwo)
}

// assertAggregateHandlers verifies the three handlers answer with their
// documented status codes for the merged state.
func assertAggregateHandlers(
	t *testing.T,
	agg *aggregate.Aggregate,
	wantStatus health.Status,
	wantStartup bool,
) {
	t.Helper()

	cases := []struct {
		name    string
		handler http.HandlerFunc
		want    int
	}{
		{name: "liveness", handler: agg.LivenessHandler(), want: http.StatusOK},
	}

	if wantStatus == health.StatusFail {
		cases = append(cases, struct {
			name    string
			handler http.HandlerFunc
			want    int
		}{name: "readiness", handler: agg.ReadinessHandler(), want: http.StatusServiceUnavailable})
	} else {
		cases = append(cases, struct {
			name    string
			handler http.HandlerFunc
			want    int
		}{name: "readiness", handler: agg.ReadinessHandler(), want: http.StatusOK})
	}

	if wantStartup {
		cases = append(cases, struct {
			name    string
			handler http.HandlerFunc
			want    int
		}{name: "startup", handler: agg.StartupHandler(), want: http.StatusOK})
	} else {
		cases = append(cases, struct {
			name    string
			handler http.HandlerFunc
			want    int
		}{name: "startup", handler: agg.StartupHandler(), want: http.StatusServiceUnavailable})
	}

	for _, testCase := range cases {
		recorder := httptest.NewRecorder()
		testCase.handler(recorder, fuzzRequest(t))

		if recorder.Code != testCase.want {
			t.Errorf("%s handler: want %d, got %d", testCase.name, testCase.want, recorder.Code)
		}
	}
}
