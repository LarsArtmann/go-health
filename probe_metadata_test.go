package health_test

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Check.Since (probe-observed status transitions) ---.

// TestEvaluate_SinceStampedOnFirstObservation pins the first-observation
// semantics: a check's Since is the clock of the first batch that reported
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

	if got := resp.Checks["db"].Since; !got.Equal(epoch) {
		t.Errorf("first observation Since: want %v, got %v", epoch, got)
	}
}

// TestEvaluate_SinceCarriedWhileStatusUnchanged verifies the carry-forward:
// once a status holds across batches, Since keeps pointing at the batch that
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

	if got := resp.Checks["db"].Since; !got.Equal(epoch) {
		t.Errorf("carried Since: want %v, got %v", epoch, got)
	}
}

// TestEvaluate_SinceRestartsOnStatusChange covers both transition
// directions: pass→fail moves Since to the failing batch's clock, and the
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

	if got := probe.Evaluate(context.Background()).Checks["db"].Since; !got.Equal(epoch) {
		t.Fatalf("initial Since: want %v, got %v", epoch, got)
	}

	failingAt := epoch.Add(2 * time.Hour)

	clock.Advance(2 * time.Hour)
	healthy = false

	if got := probe.Evaluate(context.Background()).Checks["db"].Since; !got.Equal(failingAt) {
		t.Fatalf("Since after pass→fail: want %v, got %v", failingAt, got)
	}

	clock.Advance(30 * time.Minute)

	if got := probe.Evaluate(context.Background()).Checks["db"].Since; !got.Equal(failingAt) {
		t.Fatalf("Since while still failing: want %v, got %v", failingAt, got)
	}

	clock.Advance(5 * time.Minute)
	healthy = true

	recoveredAt := failingAt.Add(35 * time.Minute)

	if got := probe.Evaluate(context.Background()).Checks["db"].Since; !got.Equal(recoveredAt) {
		t.Fatalf("Since after fail→pass: want %v, got %v", recoveredAt, got)
	}
}

// TestEvaluate_SinceRestartsWhenCheckReappears pins the pruning rule: a check
// absent from a batch is forgotten, so when it returns its Since restarts —
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
	if got := resp.Checks["cache"].Since; !got.Equal(want) {
		t.Errorf("reappearing check Since: want %v (restart, gap unobserved), got %v", want, got)
	}
}

// TestEvaluate_SinceTrackedAcrossErrorTextChanges pins that Since follows the
// STATUS, not the error message: a changing failure reason with the same
// graded status keeps the original Since.
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

	if got := probe.Evaluate(context.Background()).Checks["db"].Since; !got.Equal(epoch) {
		t.Errorf("Since drifted without a status change: want %v, got %v", epoch, got)
	}

	if resp.Checks["db"].Status != health.StatusWarn {
		t.Errorf("db status: want warn (non-critical), got %s", resp.Checks["db"].Status)
	}
}

// TestHandlers_SinceZeroOnSyntheticAndEmptyChecks pins the zero-Since
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
	// check; it must not carry a Since.
	rec = doRequest(t, probe.Healthz(), "/healthz")
	startup := decodeResponse(t, rec).Checks["startup"]

	if !startup.Since.IsZero() {
		t.Errorf("synthetic startup check Since: want zero, got %v", startup.Since)
	}
}

// TestStartupHandler_SinceStampedOnStartupEvaluations verifies the startup
// path participates in transition tracking: its batches stamp Since, and a
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
	if got := resp.Checks["db"].Since; !got.Equal(epoch) {
		t.Fatalf("startup-evaluated check Since: want %v, got %v", epoch, got)
	}

	clock.Advance(9 * time.Minute)

	if got := probe.Evaluate(context.Background()).Checks["db"].Since; !got.Equal(epoch) {
		t.Errorf(
			"readiness must carry Since stamped by the startup batch: want %v, got %v",
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
	if got := resp.Checks["db"].Since; !got.After(epoch.Add(-time.Second)) {
		t.Errorf("stamped Since should be an observation clock, got %v", got)
	}
}

// --- Check.Duration (executor-reported) ---.

// TestNewWithDetailedCheck_DurationCarriedAndStatusGraded pins the detailed
// seam: durations flow into the response untouched while classification
// stays with the probe (critical failure stays fail, message becomes Error).
func TestNewWithDetailedCheck_DurationCarriedAndStatusGraded(t *testing.T) {
	t.Parallel()

	probe := health.NewWithDetailedCheck(func(context.Context) map[string]health.CheckDetail {
		return map[string]health.CheckDetail{
			"db":     {Err: errUnhealthy, Duration: 1500 * time.Microsecond},
			"cache":  {Duration: 400 * time.Microsecond},
			"search": {Err: errUnhealthy, Duration: 250 * time.Millisecond},
		}
	},
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusFail {
		t.Fatalf("roll-up: want fail (critical db failed), got %s", resp.Status)
	}

	dbCheck := resp.Checks["db"]
	if dbCheck.Status != health.StatusFail || dbCheck.Error != errUnhealthy.Error() {
		t.Errorf("db check: want fail with error, got %+v", dbCheck)
	}

	if dbCheck.DurationNanos != (1500 * time.Microsecond).Nanoseconds() {
		t.Errorf("db duration: want 1500µs, got %v", time.Duration(dbCheck.DurationNanos))
	}

	wantCache := (400 * time.Microsecond).Nanoseconds()
	if got := resp.Checks["cache"].DurationNanos; got != wantCache {
		t.Errorf("cache duration: want 400µs, got %v", time.Duration(got))
	}

	wantSearch := (250 * time.Millisecond).Nanoseconds()
	if got := resp.Checks["search"].DurationNanos; got != wantSearch {
		t.Errorf("search duration: want 250ms, got %v", time.Duration(got))
	}
}

// TestNew_DurationZeroOnPlainInjectorPath pins the honest-absence contract:
// samber/do's batch API cannot measure per-check timing, so the raw injector
// path reports zero (unknown), never a fabricated number.
func TestNew_DurationZeroOnPlainInjectorPath(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector, health.WithRefreshInterval(0))

	resp := probe.Evaluate(context.Background())

	if got := resp.Checks["db"].DurationNanos; got != 0 {
		t.Errorf("injector-path duration: want 0 (unknown), got %v", got)
	}
}

// mockDetailedRecorder implements both the plain and detailed recorder
// methods so the optional-interface upgrade path can be exercised.
type mockDetailedRecorder struct {
	mockRecorder

	details map[string]health.CheckDetail
}

func (m *mockDetailedRecorder) RecordDetailedHealthCheckWithContext(
	context.Context,
	do.Injector,
) map[string]health.CheckDetail {
	return m.details
}

// TestWithHealthRecorder_DetailedRecorderDurationsFlow pins the optional
// recorder upgrade: a recorder that also implements DetailedHealthRecorder
// has its per-check durations carried into the response, with no changes to
// how it is wired.
func TestWithHealthRecorder_DetailedRecorderDurationsFlow(t *testing.T) {
	t.Parallel()

	recorder := &mockDetailedRecorder{
		details: map[string]health.CheckDetail{
			"db": {Duration: 3 * time.Millisecond},
		},
	}

	probe := health.New(do.New(),
		health.WithHealthRecorder(recorder),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	wantDuration := (3 * time.Millisecond).Nanoseconds()
	if got := resp.Checks["db"].DurationNanos; got != wantDuration {
		t.Errorf("detailed recorder duration: want 3ms, got %v", got)
	}

	if got := resp.Checks["db"].Status; got != health.StatusPass {
		t.Errorf("nil Err must grade pass, got %s", got)
	}
}

// TestWithHealthRecorder_PlainRecorderDurationsStayZero pins that a plain
// recorder (no detailed method) keeps the old behavior: durations unknown.
func TestWithHealthRecorder_PlainRecorderDurationsStayZero(t *testing.T) {
	t.Parallel()

	recorder := &mockRecorder{result: map[string]error{"db": nil}}

	probe := health.New(do.New(),
		health.WithHealthRecorder(recorder),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if got := resp.Checks["db"].DurationNanos; got != 0 {
		t.Errorf("plain recorder duration: want 0 (unknown), got %v", got)
	}
}

// --- Wire format (omitzero absence) ---.

// TestCheck_JSONOmitZero pins the issue #2 open question: under jsonv2,
// omitzero gives strict absence for both fields — zero Since and zero
// duration disappear instead of marshaling as "0001-01-01T00:00:00Z" / 0.
// Duration is a plain int64 (nanoseconds) because jsonv2 cannot marshal
// time.Duration at all without per-call options (verified:
// go.dev/issue/71631 — no valid tag format exists).
func TestCheck_JSONOmitZero(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(map[string]health.Check{
		"absent": {Status: health.StatusPass},
	}, json.Deterministic(true))
	if err != nil {
		t.Fatalf("marshal zero-metadata check: %v", err)
	}

	if strings.Contains(string(payload), "since") || strings.Contains(string(payload), "duration") {
		t.Errorf("zero metadata must be omitted, got %s", payload)
	}

	stamped := time.Date(2026, 9, 15, 14, 2, 0, 0, time.UTC)

	payload, err = json.Marshal(health.Check{
		Status:        health.StatusFail,
		Error:         "down",
		Since:         stamped,
		DurationNanos: (1500 * time.Microsecond).Nanoseconds(),
	}, json.Deterministic(true))
	if err != nil {
		t.Fatalf("marshal populated check: %v", err)
	}

	want := `{"status":"fail","error":"down","since":"2026-09-15T14:02:00Z","duration_ns":1500000}`
	if string(payload) != want {
		t.Errorf("populated check wire format:\nwant: %s\ngot:  %s", want, payload)
	}
}
