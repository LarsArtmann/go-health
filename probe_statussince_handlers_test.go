package health_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
)

// TestHandlers_SinceZeroOnSyntheticAndEmptyChecks pins the zero-StatusSince
// surfaces: liveness (empty checks), the never-started cached view, and the
// Healthz synthetic startup entry never claim a transition time.
func TestHandlers_SinceZeroOnSyntheticAndEmptyChecks(t *testing.T) {
	t.Parallel()

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil}
	}, health.WithRefreshInterval(0))

	rec := doRequest(t, probe.LivenessHandler(), "/healthz")
	resp := decodeResponse(t, rec)

	if len(resp.Checks) != 0 {
		t.Fatalf("liveness checks: want empty, got %d", len(resp.Checks))
	}

	cached := probe.CachedResponse()
	if len(cached.Checks) != 0 {
		t.Fatalf("never-started cached checks: want empty, got %d", len(cached.Checks))
	}

	// Healthz before any startup evaluation injects the synthetic startup
	// check; it must not carry a StatusSince.
	rec = doRequest(t, probe.Healthz(), "/healthz")
	startup := decodeResponse(t, rec).Checks["startup"]

	if !startup.StatusSince.IsZero() {
		t.Errorf("synthetic startup check StatusSince: want zero, got %v", startup.StatusSince)
	}
}

// TestStartupHandler_SinceStampedOnStartupEvaluations verifies the startup
// path participates in transition tracking: its batches stamp StatusSince, and a
// later readiness evaluation carries the stamp forward.
func TestStartupHandler_SinceStampedOnStartupEvaluations(t *testing.T) {
	t.Parallel()

	epoch := time.Date(2026, 9, 15, 14, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil}
	},
		health.WithCriticalServices("db"),
		health.WithNowFunc(clock.Now),
		health.WithRefreshInterval(0),
	)

	rec := doRequest(t, probe.StartupHandler(), "/startupz")
	if rec.Code != 200 {
		t.Fatalf("startup status: want 200, got %d", rec.Code)
	}

	resp := decodeResponse(t, rec)
	if got := resp.Checks["db"].StatusSince; !got.Equal(epoch) {
		t.Fatalf("startup-evaluated check StatusSince: want %v, got %v", epoch, got)
	}

	clock.Advance(9 * time.Minute)

	if got := probe.Evaluate(context.Background()).Checks["db"].StatusSince; !got.Equal(epoch) {
		t.Errorf(
			"readiness must carry StatusSince stamped by the startup batch: want %v, got %v",
			epoch,
			got,
		)
	}
}

// TestEvaluate_ConcurrentBatchesAreRaceFree hammers concurrent evaluations
// (the overlapping refresh/startup/live scenario) so the race detector can
// prove transitionTracker stamping is safe. Out-of-order completions may
// attribute a transition to either batch's clock; both stamps must be one of
// the injected clocks, never garbage.
func TestEvaluate_ConcurrentBatchesAreRaceFree(t *testing.T) {
	t.Parallel()

	epoch := time.Date(2026, 9, 15, 14, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	failing := &atomic.Bool{}

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		if failing.Load() {
			return map[string]error{"db": errUnhealthy}
		}

		return map[string]error{"db": nil}
	},
		health.WithCriticalServices("db"),
		health.WithNowFunc(clock.Now),
		health.WithRefreshInterval(0),
	)

	var wg sync.WaitGroup

	for i := range 32 {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			if i%8 == 0 {
				failing.Store(!failing.Load()) // flap the source, concurrently
			}

			probe.Evaluate(context.Background())
		}(i)
	}

	wg.Wait()

	resp := probe.Evaluate(context.Background())
	if got := resp.Checks["db"].StatusSince; !got.After(epoch.Add(-time.Second)) {
		t.Errorf("stamped StatusSince should be an observation clock, got %v", got)
	}
}

// TestEvaluate_TrackerDisabledLeavesSinceZero pins the benchmark-only escape
// hatch behind [Probe.SetTrackerDisabledForTest]: with stamping off every
// check keeps the zero time, which is how the A/B benchmark isolates the
// tracker's marginal cost from the rest of an evaluation batch.
func TestEvaluate_TrackerDisabledLeavesSinceZero(t *testing.T) {
	t.Parallel()

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil}
	}, health.WithRefreshInterval(0))

	probe.SetTrackerDisabledForTest(true)

	if got := probe.Evaluate(context.Background()).Checks["db"].StatusSince; !got.IsZero() {
		t.Errorf("StatusSince = %v, want the zero time while the tracker is disabled", got)
	}
}
