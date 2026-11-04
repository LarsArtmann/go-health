package health_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
)

// --- Check.StatusSince (probe-observed status transitions) ---.

// TestEvaluate_SinceStampedOnFirstObservation pins the first-observation
// semantics: a check's StatusSince is the clock of the first batch that reported
// its current status, not a per-batch timestamp.
func TestEvaluate_SinceStampedOnFirstObservation(t *testing.T) {
	t.Parallel()

	epoch := time.Date(2026, 9, 15, 14, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil}
	},
		health.WithNowFunc(clock.Now),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if got := resp.Checks["db"].StatusSince; !got.Equal(epoch) {
		t.Errorf("first observation StatusSince: want %v, got %v", epoch, got)
	}
}

// TestEvaluate_SinceCarriedWhileStatusUnchanged verifies the carry-forward:
// once a status holds across batches, StatusSince keeps pointing at the batch that
// started it, no matter how far the clock advances.
func TestEvaluate_SinceCarriedWhileStatusUnchanged(t *testing.T) {
	t.Parallel()

	epoch := time.Date(2026, 9, 15, 14, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil}
	},
		health.WithNowFunc(clock.Now),
		health.WithRefreshInterval(0),
	)

	probe.Evaluate(context.Background())

	for range 3 {
		clock.Advance(17 * time.Minute)
		probe.Evaluate(context.Background())
	}

	resp := probe.Evaluate(context.Background())

	if got := resp.Checks["db"].StatusSince; !got.Equal(epoch) {
		t.Errorf("carried StatusSince: want %v, got %v", epoch, got)
	}
}

// TestEvaluate_SinceRestartsOnStatusChange covers both transition
// directions: pass→fail moves StatusSince to the failing batch's clock, and the
// later fail→pass recovery moves it again.
func TestEvaluate_SinceRestartsOnStatusChange(t *testing.T) {
	t.Parallel()

	epoch := time.Date(2026, 9, 15, 14, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	healthy := true

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		if healthy {
			return map[string]error{"db": nil}
		}

		return map[string]error{"db": errUnhealthy}
	},
		health.WithCriticalServices("db"),
		health.WithNowFunc(clock.Now),
		health.WithRefreshInterval(0),
	)

	if got := probe.Evaluate(context.Background()).Checks["db"].StatusSince; !got.Equal(epoch) {
		t.Fatalf("initial StatusSince: want %v, got %v", epoch, got)
	}

	failingAt := epoch.Add(2 * time.Hour)

	clock.Advance(2 * time.Hour)

	healthy = false

	if got := probe.Evaluate(context.Background()).Checks["db"].StatusSince; !got.Equal(failingAt) {
		t.Fatalf("StatusSince after pass→fail: want %v, got %v", failingAt, got)
	}

	clock.Advance(30 * time.Minute)

	if got := probe.Evaluate(context.Background()).Checks["db"].StatusSince; !got.Equal(failingAt) {
		t.Fatalf("StatusSince while still failing: want %v, got %v", failingAt, got)
	}

	clock.Advance(5 * time.Minute)

	healthy = true

	recoveredAt := failingAt.Add(35 * time.Minute)

	if got := probe.Evaluate(context.Background()).Checks["db"].StatusSince; !got.Equal(
		recoveredAt,
	) {
		t.Fatalf("StatusSince after fail→pass: want %v, got %v", recoveredAt, got)
	}
}

// TestEvaluate_SinceRestartsWhenCheckReappears pins the pruning rule: a check
// absent from a batch is forgotten, so when it returns its StatusSince restarts —
// the gap was unobserved, and carrying the old stamp would lie.
func TestEvaluate_SinceRestartsWhenCheckReappears(t *testing.T) {
	t.Parallel()

	epoch := time.Date(2026, 9, 15, 14, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	includeCache := true

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		if includeCache {
			return map[string]error{"db": nil, "cache": nil}
		}

		return map[string]error{"db": nil}
	},
		health.WithNowFunc(clock.Now),
		health.WithRefreshInterval(0),
	)

	probe.Evaluate(context.Background())

	clock.Advance(time.Hour)

	includeCache = false

	probe.Evaluate(context.Background())

	clock.Advance(time.Hour)

	includeCache = true

	resp := probe.Evaluate(context.Background())

	want := epoch.Add(2 * time.Hour)
	if got := resp.Checks["cache"].StatusSince; !got.Equal(want) {
		t.Errorf(
			"reappearing check StatusSince: want %v (restart, gap unobserved), got %v",
			want,
			got,
		)
	}
}

// TestEvaluate_SinceTrackedAcrossErrorTextChanges pins that StatusSince follows the
// STATUS, not the error message: a changing failure reason with the same
// graded status keeps the original StatusSince.
func TestEvaluate_SinceTrackedAcrossErrorTextChanges(t *testing.T) {
	t.Parallel()

	epoch := time.Date(2026, 9, 15, 14, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	reason := "connection refused"

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": fmt.Errorf("%w: %s", errUnhealthy, reason)}
	}, health.WithNowFunc(clock.Now), health.WithRefreshInterval(0))

	resp := probe.Evaluate(context.Background())

	clock.Advance(10 * time.Minute)

	reason = "dns lookup failed"

	if got := probe.Evaluate(context.Background()).Checks["db"].StatusSince; !got.Equal(epoch) {
		t.Errorf("StatusSince drifted without a status change: want %v, got %v", epoch, got)
	}

	if resp.Checks["db"].Status != health.StatusWarn {
		t.Errorf("db status: want warn (non-critical), got %s", resp.Checks["db"].Status)
	}
}
