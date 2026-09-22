package aggregate_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	aggregate "github.com/larsartmann/go-health/aggregate"
)

// This file holds property tests for the aggregate merge. The merge is a pure
// function of the sources' cached views, so it must be idempotent,
// order-independent, and a namespaced union whose roll-up is the worst source
// status. Table-driven examples pin specific shapes; these properties pin the
// algebra every shape must satisfy.

// maxSourcesPerAggregate bounds the source combinations the property tests
// enumerate. Three distinct states already exercise every collision and
// ordering the merge can see.
const maxSourcesPerAggregate = 3

// sourceKeys is the deterministic iteration order for fixtures.
var sourceKeys = []string{"pass", "warn", "fail", "shutdown", "ghost"}

// newStateProbe builds a primed probe in a chosen cached state. It primes via
// a readiness request under a one-hour live throttle (the same pattern as
// newPrimedSource) so CachedResponse reads a populated atomic cache. The
// golden clock keeps Since deterministic across fixtures.
func newStateProbe(
	t *testing.T,
	results map[string]error,
	critical ...string,
) *health.Probe {
	t.Helper()

	opts := []health.Option{
		health.WithRefreshInterval(0),
		health.WithLiveThrottle(time.Hour),
		health.WithNowFunc(goldenClock),
	}

	if len(critical) > 0 {
		opts = append(opts, health.WithCriticalServices(critical...))
	}

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return results
	}, opts...)

	if err := probe.Start(context.Background()); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}

	rec := httptest.NewRecorder()
	probe.ReadinessHandler()(rec, fuzzRequest(t))

	return probe
}

// sourceFixtures returns one probe per cached state, built once per test so
// every ordering below reuses the same probe instances (isolating the merge's
// order sensitivity from any construction-time noise).
func sourceFixtures(t *testing.T) map[string]*health.Probe {
	t.Helper()

	shutting := newStateProbe(t, map[string]error{"db": nil}, "db")
	shutting.Shutdown()

	neverStarted := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil}
	}, health.WithRefreshInterval(0))

	return map[string]*health.Probe{
		"pass": newStateProbe(t, map[string]error{"db": nil}, "db"),
		"warn": newStateProbe(t, map[string]error{
			"db":    nil,
			"cache": errUnhealthy,
		}, "db"),
		"fail":     newStateProbe(t, map[string]error{"db": errUnhealthy}, "db"),
		"shutdown": shutting,
		"ghost":    neverStarted,
	}
}

// orderedSelections returns every ordered selection of up to
// maxSourcesPerAggregate distinct keys from sourceKeys, in a deterministic
// order.
func orderedSelections() [][]string {
	var out [][]string

	var walk func(current []string)

	walk = func(current []string) {
		if len(current) > 0 {
			out = append(out, slices.Clone(current))
		}

		if len(current) == maxSourcesPerAggregate {
			return
		}

		for _, key := range sourceKeys {
			if slices.Contains(current, key) {
				continue
			}

			walk(append(current, key))
		}
	}

	walk(nil)

	return out
}

// sourcesFor turns an ordered key list into aggregate sources.
func sourcesFor(fixtures map[string]*health.Probe, keys []string) []aggregate.Source {
	sources := make([]aggregate.Source, 0, len(keys))
	for _, key := range keys {
		sources = append(sources, aggregate.Source{Name: key, Probe: fixtures[key]})
	}

	return sources
}

// expectedWorstOf recomputes the documented merge from the sources' own cached
// views, independently of the implementation under test.
func expectedWorstOf(sources []aggregate.Source) (health.Status, bool, int64) {
	status := health.StatusPass
	shutting := false

	var maxLatency int64

	for _, src := range sources {
		resp := src.Probe.CachedResponse()

		if resp.ShuttingDown {
			shutting = true
		}

		if resp.TotalLatencyMs > maxLatency {
			maxLatency = resp.TotalLatencyMs
		}

		if sevRank(resp.Status) < sevRank(status) {
			status = resp.Status
		}
	}

	if shutting {
		status = health.StatusFail
	}

	return status, shutting, maxLatency
}

// TestAggregateMerge_MergeIdempotent asserts repeated reads of one aggregate
// return identical responses: the merge has no internal state to advance.
func TestAggregateMerge_MergeIdempotent(t *testing.T) {
	t.Parallel()

	fixtures := sourceFixtures(t)

	for _, keys := range orderedSelections() {
		t.Run(joinKeys(keys), func(t *testing.T) {
			t.Parallel()

			agg := mustAggregate(t, sourcesFor(fixtures, keys)...)

			first := agg.CachedResponse()
			second := agg.CachedResponse()

			if !reflect.DeepEqual(first, second) {
				t.Errorf("merge not idempotent:\nfirst:  %+v\nsecond: %+v", first, second)
			}
		})
	}
}

// TestAggregateMerge_MergeCommutative asserts the merged response does not
// depend on source order: the same probes in any order produce a bit-for-bit
// equal response.
func TestAggregateMerge_MergeCommutative(t *testing.T) {
	t.Parallel()

	fixtures := sourceFixtures(t)

	for _, keys := range orderedSelections() {
		t.Run(joinKeys(keys), func(t *testing.T) {
			t.Parallel()

			forward := mustAggregate(t, sourcesFor(fixtures, keys)...).CachedResponse()

			reversed := slices.Clone(keys)
			slices.Reverse(reversed)

			backward := mustAggregate(t, sourcesFor(fixtures, reversed)...).CachedResponse()

			if !reflect.DeepEqual(forward, backward) {
				t.Errorf("merge order-dependent for %v:\nforward:  %+v\nbackward: %+v",
					keys, forward, backward)
			}
		})
	}
}

// TestAggregateMerge_MergeIsNamespacedUnion asserts the merged checks map is
// exactly the union of every source's checks under its "name/" prefix:
// disjoint (no collisions) and complete (nothing dropped or invented).
func TestAggregateMerge_MergeIsNamespacedUnion(t *testing.T) {
	t.Parallel()

	fixtures := sourceFixtures(t)

	for _, keys := range orderedSelections() {
		t.Run(joinKeys(keys), func(t *testing.T) {
			t.Parallel()

			sources := sourcesFor(fixtures, keys)
			merged := mustAggregate(t, sources...).CachedResponse()

			want := make(map[string]health.Check)

			for _, src := range sources {
				for name, check := range src.Probe.CachedResponse().Checks {
					want[src.Name+"/"+name] = check
				}
			}

			if !reflect.DeepEqual(merged.Checks, want) {
				t.Errorf("merged checks are not the namespaced union:\nwant: %+v\ngot:  %+v",
					want, merged.Checks)
			}
		})
	}
}

// TestAggregateMerge_RollupMatchesWorstOf asserts the merged status, shutdown
// flag, and latency independently match the documented worst-of fold over the
// sources' cached views.
func TestAggregateMerge_RollupMatchesWorstOf(t *testing.T) {
	t.Parallel()

	fixtures := sourceFixtures(t)

	for _, keys := range orderedSelections() {
		t.Run(joinKeys(keys), func(t *testing.T) {
			t.Parallel()

			sources := sourcesFor(fixtures, keys)
			merged := mustAggregate(t, sources...).CachedResponse()

			wantStatus, wantShutting, wantLatency := expectedWorstOf(sources)

			if merged.Status != wantStatus {
				t.Errorf("status = %q, want %q", merged.Status, wantStatus)
			}

			if merged.ShuttingDown != wantShutting {
				t.Errorf("shutting_down = %v, want %v", merged.ShuttingDown, wantShutting)
			}

			if merged.TotalLatencyMs != wantLatency {
				t.Errorf("total_latency_ms = %d, want %d", merged.TotalLatencyMs, wantLatency)
			}
		})
	}
}

// TestAggregateMerge_ShutdownIsAbsorbing asserts a single shutting-down source
// forces the whole aggregate to fail and report shutting_down, regardless of
// the other sources' health.
func TestAggregateMerge_ShutdownIsAbsorbing(t *testing.T) {
	t.Parallel()

	fixtures := sourceFixtures(t)

	for _, other := range sourceKeys {
		if other == "shutdown" {
			continue
		}

		t.Run(other, func(t *testing.T) {
			t.Parallel()

			agg := mustAggregate(t,
				aggregate.Source{Name: other, Probe: fixtures[other]},
				aggregate.Source{Name: "shutdown", Probe: fixtures["shutdown"]},
			)

			merged := agg.CachedResponse()

			if merged.Status != health.StatusFail {
				t.Errorf("status = %q, want fail (shutdown absorbs %q)", merged.Status, other)
			}

			if !merged.ShuttingDown {
				t.Error("shutting_down = false, want true")
			}

			if agg.StartupComplete() {
				t.Error("StartupComplete = true, want false: a shutting-down source never latches")
			}
		})
	}
}

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

// joinKeys renders a source key list as a stable subtest name.
func joinKeys(keys []string) string {
	return strings.Join(keys, "+")
}
