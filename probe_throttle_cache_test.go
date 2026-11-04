package health_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
)

// --- Throttle × Start-cache interaction (README "Flood-safe live mode") ---.

// awaitLoopBatches polls until the background refresh loop has stored at
// least minBatches batches. Fails the test on timeout.
func awaitLoopBatches(t *testing.T, batches *atomic.Int64, minBatches int64) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for {
		if batches.Load() >= minBatches {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("refresh loop stored no cache entry after 2s (batches=%d)", batches.Load())
		}

		time.Sleep(5 * time.Millisecond)
	}
}

// TestWithLiveThrottle_StartCacheServesWithoutBatches backs the README
// paragraph on the WithLiveThrottle × Start interaction: once Start's
// background loop has populated the cache, requests are served from it and
// trigger zero evaluation batches. The loop is cancelled (and given longer
// than one tick to fully stop) before the request phase, so the batch count
// is exact rather than statistical.
func TestWithLiveThrottle_StartCacheServesWithoutBatches(t *testing.T) {
	t.Parallel()

	const (
		loopInterval = 50 * time.Millisecond
		window       = time.Second
	)

	var batches atomic.Int64

	epoch := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		batches.Add(1)

		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithRefreshInterval(loopInterval),
		health.WithLiveThrottle(window),
		health.WithBootTime(epoch),
		health.WithNowFunc(clock.Now),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}

	awaitLoopBatches(t, &batches, 1)

	cancel()
	time.Sleep(3 * loopInterval) // any in-flight tick has landed by now

	before := batches.Load()

	handler := probe.ReadinessHandler()

	assertFrozenWindow(
		t,
		handler,
		"/readyz",
		requestReadiness(t, handler, "/readyz"),
		25,
	)

	if after := batches.Load(); after != before {
		t.Errorf(
			"requests against the loop-refreshed cache ran %d extra batches; want 0",
			after-before,
		)
	}
}

// TestWithLiveThrottle_StaleCacheTriggersExactlyOneEvaluation completes the
// Start-cache interaction contract: a cached result older than the throttle
// window does not get served — exactly one live evaluation runs, and the
// refreshed result is then served without further batches.
func TestWithLiveThrottle_StaleCacheTriggersExactlyOneEvaluation(t *testing.T) {
	t.Parallel()

	const (
		loopInterval = 50 * time.Millisecond
		window       = time.Second
	)

	var batches atomic.Int64

	epoch := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		batches.Add(1)

		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithRefreshInterval(loopInterval),
		health.WithLiveThrottle(window),
		health.WithBootTime(epoch),
		health.WithNowFunc(clock.Now),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}

	awaitLoopBatches(t, &batches, 1)

	cancel()
	time.Sleep(3 * loopInterval)

	clock.Advance(window + time.Millisecond) // cached.Timestamp is now stale

	handler := probe.ReadinessHandler()

	requestReadiness(t, handler, "/readyz")

	if got := batches.Load(); got != 2 {
		t.Fatalf("stale-cache request batches: want exactly 2 (1 loop + 1 live), got %d", got)
	}

	// The freshly evaluated result is inside the window again.
	assertFrozenWindow(
		t,
		handler,
		"/readyz",
		requestReadiness(t, handler, "/readyz"),
		10,
	)

	if got := batches.Load(); got != 2 {
		t.Errorf("post-refresh requests ran %d total batches; want still 2", got)
	}
}
