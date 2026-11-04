package aggregate_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	aggregate "github.com/larsartmann/go-health/aggregate"
)

// --- CachedResponse merging ---.

// TestCachedResponse_NeverStartedSource pins the zero-cache merge path: a
// source probe that was never started (or whose checks were never invoked)
// contributes an empty cached view — no checks, no latency, status folded as
// healthy. That is the documented false-confidence sharp edge (AGENTS.md:
// invoke critical services at boot): readiness reports pass until the
// source's own first evaluation populates its cache. The startup latch is
// the honest signal — it stays false, so startup keeps serving 503.
func TestCachedResponse_NeverStartedSource(t *testing.T) {
	t.Parallel()

	neverStarted := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"svc": nil}
	}, health.WithRefreshInterval(0))

	started := newPrimedSource(t, func(context.Context) map[string]error {
		return map[string]error{"svc": nil}
	}, "pod-1")

	agg, err := aggregate.New(
		aggregate.Source{Name: "ghost", Probe: neverStarted},
		aggregate.Source{Name: "real", Probe: started},
	)
	if err != nil {
		t.Fatalf("aggregate.New: %v", err)
	}

	merged := agg.CachedResponse()

	if len(merged.Checks) != 1 {
		t.Fatalf("merged check count = %d, want 1 (only the started source)", len(merged.Checks))
	}

	if _, ok := merged.Checks["real/svc"]; !ok {
		t.Fatalf("merged checks = %v, want only \"real/svc\"", merged.Checks)
	}

	if merged.Status != health.StatusPass {
		t.Errorf("merged status = %q, want pass (never-started folds as healthy)", merged.Status)
	}

	// The ghost contributes no latency: the merged total is exactly the real
	// source's own cached batch, not a hardcoded zero — the priming
	// evaluation's own time.Since reading can legitimately be 1ms on a loaded
	// machine (observed flaking under full-suite parallelism), and the
	// property under test is "the ghost adds nothing", not "the batch is
	// instant".
	if merged.ShuttingDown || merged.TotalLatencyMs != started.CachedResponse().TotalLatencyMs ||
		!merged.InstanceID.IsZero() || merged.Version != "" {
		t.Errorf("unexpected merged scalars: %+v", merged)
	}

	if agg.StartupComplete() {
		t.Error("StartupComplete = true, want false (never-started latch is unset)")
	}

	startupRec := httptest.NewRecorder()
	agg.StartupHandler()(startupRec, fuzzRequest(t))

	if startupRec.Code != http.StatusServiceUnavailable {
		t.Errorf("startup status = %d, want 503 while any latch is unset", startupRec.Code)
	}

	readyRec := httptest.NewRecorder()
	agg.ReadinessHandler()(readyRec, fuzzRequest(t))

	if readyRec.Code != http.StatusOK {
		t.Errorf(
			"readiness status = %d, want 200 (pass fold; the documented sharp edge)",
			readyRec.Code,
		)
	}
}

func TestCachedResponse_WorstOfStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		one  bool // first source critical+unhealthy (fail)
		two  bool // second source non-critical+unhealthy (warn)
		// third source is always healthy (pass)
		want health.Status
	}{
		{name: "all pass", one: false, two: false, want: health.StatusPass},
		{name: "warn beats pass", one: false, two: true, want: health.StatusWarn},
		{name: "fail beats pass", one: true, two: false, want: health.StatusFail},
		{name: "fail beats warn", one: true, two: true, want: health.StatusFail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agg := mustAggregate(t,
				aggregate.Source{Name: "one", Probe: newStartedProbe(t, true, tt.one)},
				aggregate.Source{Name: "two", Probe: newStartedProbe(t, false, tt.two)},
				aggregate.Source{Name: "three", Probe: newStartedProbe(t, false, false)},
			)

			resp := agg.CachedResponse()

			if resp.Status != tt.want {
				t.Fatalf("Status = %q, want %q", resp.Status, tt.want)
			}

			if resp.ShuttingDown {
				t.Fatal("ShuttingDown = true, want false")
			}

			if resp.TotalLatencyMs < 0 {
				t.Fatalf("TotalLatencyMs = %d, want >= 0", resp.TotalLatencyMs)
			}

			// Every source contributes its namespaced dependency check.
			for _, key := range []string{"one/dependency", "two/dependency", "three/dependency"} {
				if _, ok := resp.Checks[key]; !ok {
					t.Fatalf("Checks missing namespaced key %q; got %v", key, resp.Checks)
				}
			}
		})
	}
}

func TestCachedResponse_ShutdownForcesFail(t *testing.T) {
	t.Parallel()

	shuttingDown := newStartedProbe(t, false, false)
	shuttingDown.Shutdown()

	agg := mustAggregate(t,
		aggregate.Source{Name: "down", Probe: shuttingDown},
		aggregate.Source{Name: "up", Probe: newStartedProbe(t, false, false)},
	)

	resp := agg.CachedResponse()

	if !resp.ShuttingDown {
		t.Fatal("ShuttingDown = false, want true")
	}

	if resp.Status != health.StatusFail {
		t.Fatalf("Status = %q, want fail (shutdown mirrors Probe.classify)", resp.Status)
	}

	// The healthy source's checks survive the merge.
	if _, ok := resp.Checks["up/dependency"]; !ok {
		t.Fatalf("healthy source checks lost in merge; got %v", resp.Checks)
	}
}

func TestCachedResponse_TotalLatencyMsIsSlowestSource(t *testing.T) {
	t.Parallel()

	// A check that takes measurably long stamps its source's cached response
	// with a non-zero TotalLatencyMs; the merge must propagate the max.
	slow := health.NewWithHealthCheck(func(_ context.Context) map[string]error {
		time.Sleep(2 * time.Millisecond)

		return map[string]error{"db": nil}
	}, health.WithRefreshInterval(0))

	if err := slow.Start(context.Background()); err != nil {
		t.Fatalf("slow.Start: %v", err)
	}

	slowCached := slow.CachedResponse()
	if slowCached.TotalLatencyMs <= 0 {
		t.Fatalf("test setup: slow source latency = %dms, want > 0", slowCached.TotalLatencyMs)
	}

	agg := mustAggregate(t,
		aggregate.Source{Name: "slow", Probe: slow},
		aggregate.Source{Name: "fast", Probe: newStartedProbe(t, false, false)},
	)

	// The merge must propagate the slowest source: merged latency is the max
	// over sources, so it is at least the slow source's stamp. Exact equality
	// would flake under load — the "fast" source's own evaluation jitter can
	// exceed the slow source's sleep and legitimately become the max.
	if got := agg.CachedResponse().TotalLatencyMs; got < slowCached.TotalLatencyMs {
		t.Fatalf(
			"merged latency = %dms, want >= slowest source's %dms",
			got,
			slowCached.TotalLatencyMs,
		)
	}
}

// TestCachedResponse_CheckMetadataSurvivesMerge pins that per-check metadata
// added in go-health issue #2 survives the namespaced merge: StatusSince and
// DurationNanos belong to the Check value each source contributed and must
// arrive in the merged view unchanged.
func TestCachedResponse_CheckMetadataSurvivesMerge(t *testing.T) {
	t.Parallel()

	epoch := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	// Detailed source reporting a per-check duration; the fixed clock makes
	// the transition-tracker's StatusSince deterministic. Priming via one throttled
	// readiness request stores the evaluation the merge reads.
	detailed := health.NewWithDetailedCheck(func(context.Context) map[string]health.CheckDetail {
		return map[string]health.CheckDetail{
			"db": {Duration: 4 * time.Millisecond},
		}
	},
		health.WithRefreshInterval(0),
		health.WithLiveThrottle(time.Hour),
		health.WithNowFunc(func() time.Time { return epoch }),
	)

	rec := httptest.NewRecorder()
	detailed.ReadinessHandler()(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	agg := mustAggregate(t,
		aggregate.Source{Name: "api", Probe: detailed},
		aggregate.Source{Name: "web", Probe: newStartedProbe(t, false, false)},
	)

	merged := agg.CachedResponse().Checks["api/db"]

	if !merged.StatusSince.Equal(epoch) {
		t.Errorf("merged StatusSince: want %v, got %v", epoch, merged.StatusSince)
	}

	if want := (4 * time.Millisecond).Nanoseconds(); merged.DurationNanos != want {
		t.Errorf("merged DurationNanos: want %d, got %d", want, merged.DurationNanos)
	}
}
