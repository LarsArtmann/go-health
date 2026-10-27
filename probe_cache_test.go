package health_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
)

// errDatabaseDown is the canned dependency failure used by the publish tests.
var errDatabaseDown = errors.New("database down")

// fixedClock is a manually advanced clock for age assertions.
type fixedClock struct {
	now atomic.Int64
}

func newFixedClock(start time.Time) *fixedClock {
	clock := &fixedClock{}
	clock.now.Store(start.UnixNano())

	return clock
}

func (c *fixedClock) Now() time.Time { return time.Unix(0, c.now.Load()).UTC() }

func (c *fixedClock) Advance(d time.Duration) { c.now.Add(int64(d)) }

func TestCacheAge_NoSnapshotReportsFalse(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(map[string]health.CheckFunc{
		"database": func(context.Context) error { return nil },
	})

	age, ok := probe.CacheAge()
	if ok {
		t.Fatalf("CacheAge = %s, true before any evaluation; want false", age)
	}

	if resp, fresh := probe.CachedResponseFresh(time.Second); fresh {
		t.Errorf("CachedResponseFresh = %+v, true with no snapshot; want false", resp)
	}
}

func TestCacheAge_TracksPublishedEvaluation(t *testing.T) {
	t.Parallel()

	clock := newFixedClock(time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC))

	probe := health.NewChecks(
		map[string]health.CheckFunc{"database": func(context.Context) error { return nil }},
		health.WithRefreshInterval(0),
		health.WithNowFunc(clock.Now),
	)

	probe.EvaluateAndCache(context.Background())

	age, ok := probe.CacheAge()
	if !ok || age != 0 {
		t.Fatalf("CacheAge = %s, %v immediately after EvaluateAndCache; want 0, true", age, ok)
	}

	clock.Advance(2 * time.Second)

	age, ok = probe.CacheAge()
	if !ok || age != 2*time.Second {
		t.Fatalf("CacheAge = %s, %v after advancing 2s; want 2s, true", age, ok)
	}

	if _, fresh := probe.CachedResponseFresh(time.Second); fresh {
		t.Error("a 2s-old snapshot must not be fresh within a 1s window")
	}

	resp, fresh := probe.CachedResponseFresh(3 * time.Second)
	if !fresh {
		t.Fatal("a 2s-old snapshot must be fresh within a 3s window")
	}

	if resp.Status != health.StatusPass {
		t.Errorf("fresh snapshot status = %q, want pass", resp.Status)
	}

	if _, fresh := probe.CachedResponseFresh(0); fresh {
		t.Error("a non-positive maxAge must never report fresh")
	}

	if _, fresh := probe.CachedResponseFresh(-time.Second); fresh {
		t.Error("a negative maxAge must never report fresh")
	}
}

func TestCacheAge_ZeroTimestampReportsFalse(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(
		map[string]health.CheckFunc{"database": func(context.Context) error { return nil }},
		health.WithRefreshInterval(0),
		health.WithNowFunc(func() time.Time { return time.Time{} }),
	)

	probe.EvaluateAndCache(context.Background())

	if age, ok := probe.CacheAge(); ok {
		t.Errorf("CacheAge = %s, true with a zero evaluation timestamp; want false", age)
	}
}

func TestCacheAge_BackwardsClockNeverNegative(t *testing.T) {
	t.Parallel()

	clock := newFixedClock(time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC))

	probe := health.NewChecks(
		map[string]health.CheckFunc{"database": func(context.Context) error { return nil }},
		health.WithRefreshInterval(0),
		health.WithNowFunc(clock.Now),
	)

	probe.EvaluateAndCache(context.Background())
	clock.Advance(-5 * time.Second)

	age, ok := probe.CacheAge()
	if !ok || age != 0 {
		t.Errorf("CacheAge = %s, %v after a backwards step; want 0, true", age, ok)
	}
}

func TestCachedResponseFresh_OverlaysShutdown(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(
		map[string]health.CheckFunc{"database": func(context.Context) error { return nil }},
		health.WithRefreshInterval(0),
	)

	probe.EvaluateAndCache(context.Background())
	probe.MarkShuttingDown()

	resp, fresh := probe.CachedResponseFresh(time.Minute)
	if !fresh {
		t.Fatal("shutdown must not make a fresh snapshot stale")
	}

	if !resp.ShuttingDown || resp.Status != health.StatusFail {
		t.Errorf("fresh snapshot = shutting_down=%v status=%q, want true/fail",
			resp.ShuttingDown, resp.Status)
	}
}

// TestCachedResponse_ChecksAliasThePublishedSnapshot pins the ownership
// contract in both directions, because both halves are load-bearing:
//
//   - the returned Checks map aliases the published snapshot (the lock-free
//     read must not allocate), so a write through it is visible to the next
//     reader — the hazard dnsblockd and cv each crashed on;
//   - Clone() breaks that alias, so the sanctioned overlay pattern is safe.
//
// A refactor that quietly starts deep-copying CachedResponse passes every
// other test in this file while regressing the zero-allocation guarantee, so
// this test is the tripwire for it.
func TestCachedResponse_ChecksAliasThePublishedSnapshot(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(
		map[string]health.CheckFunc{"database": func(context.Context) error { return nil }},
		health.WithRefreshInterval(0),
	)
	t.Cleanup(probe.Shutdown)

	if err := probe.Start(t.Context()); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	mutated := probe.CachedResponse()
	mutated.Checks["overlay"] = health.Check{Status: health.StatusWarn}

	// No background loop is running (live mode), so this write is the only
	// mutator and the read below cannot race it.
	if _, ok := probe.CachedResponse().Checks["overlay"]; !ok {
		t.Fatal("write through CachedResponse().Checks was not visible; " +
			"CachedResponse stopped aliasing the published snapshot, " +
			"which silently changes the documented zero-allocation contract")
	}

	cloned := probe.CachedResponse().Clone()
	cloned.Checks["cloned-only"] = health.Check{Status: health.StatusWarn}

	if _, ok := probe.CachedResponse().Checks["cloned-only"]; ok {
		t.Error("write through Clone().Checks leaked into the served response")
	}
}

func TestEvaluateAndCache_PublishesButEvaluateDoesNot(t *testing.T) {
	t.Parallel()

	var failing atomic.Bool

	probe := health.NewChecks(
		map[string]health.CheckFunc{
			"database": func(context.Context) error {
				if failing.Load() {
					return errDatabaseDown
				}

				return nil
			},
		},
		health.WithRefreshInterval(0),
	)
	t.Cleanup(probe.Shutdown)

	failing.Store(true)

	published := probe.EvaluateAndCache(context.Background())

	if published.Status != health.StatusWarn {
		t.Fatalf("published status = %q, want warn", published.Status)
	}

	if cached := probe.CachedResponse(); cached.Status != health.StatusWarn {
		t.Fatalf("CachedResponse status = %q, want the published warn", cached.Status)
	}

	failing.Store(false)

	if evaluated := probe.Evaluate(context.Background()); evaluated.Status != health.StatusPass {
		t.Fatalf("Evaluate status = %q, want pass", evaluated.Status)
	}

	if cached := probe.CachedResponse(); cached.Status != health.StatusWarn {
		t.Errorf("CachedResponse status = %q after a bare Evaluate; want the older warn "+
			"(Evaluate must not publish)", cached.Status)
	}

	if published := probe.EvaluateAndCache(context.Background()); published.Status != health.StatusPass {
		t.Fatalf("second EvaluateAndCache status = %q, want pass", published.Status)
	}

	if cached := probe.CachedResponse(); cached.Status != health.StatusPass {
		t.Errorf("CachedResponse status = %q after EvaluateAndCache; want pass", cached.Status)
	}
}

func TestResponse_CloneIsIndependent(t *testing.T) {
	t.Parallel()

	original := health.Response{
		Status: health.StatusWarn,
		Checks: map[string]health.Check{
			"database": {Status: health.StatusPass},
		},
	}

	clone := original.Clone()
	clone.Checks["database"] = health.Check{Status: health.StatusFail}
	clone.Checks["extra"] = health.Check{Status: health.StatusFail}
	clone.Status = health.StatusFail

	if got := original.Checks["database"].Status; got != health.StatusPass {
		t.Errorf("original check mutated through the clone: %q", got)
	}

	if _, ok := original.Checks["extra"]; ok {
		t.Error("original gained a check added to the clone")
	}

	if original.Status != health.StatusWarn {
		t.Errorf("original status = %q after cloning; want warn", original.Status)
	}

	var empty health.Response

	if cloned := empty.Clone(); cloned.Checks != nil {
		t.Errorf("nil Checks cloned into %v, want nil", cloned.Checks)
	}
}
