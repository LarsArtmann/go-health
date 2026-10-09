package health_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
)

// Panicking-hook semantics per docs/panic-recovery-design.md, "the
// evaluation-hook surface": recover on every path, a visible non-critical
// "evaluation-hook" warn row, roll-up degraded to warn at most, and the
// served response provably pristine (defensive Checks clone).

var errHookTestBoom = errors.New("db down")

func TestWithEvaluationHook_PanicRecovered_DegradesToWarn(t *testing.T) {
	t.Parallel()

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithEvaluationHook(func(health.Response) {
			panic("hook exploded")
		}),
	)

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusWarn {
		t.Errorf("Status: want warn (degraded, not failed), got %q", resp.Status)
	}

	row, ok := resp.Checks["evaluation-hook"]
	if !ok {
		t.Fatalf("Checks missing synthetic evaluation-hook row; got %v", resp.Checks)
	}

	if row.Status != health.StatusWarn {
		t.Errorf("evaluation-hook status: want warn, got %q", row.Status)
	}

	if !strings.Contains(row.Error, "health: panic during evaluation hook") ||
		!strings.Contains(row.Error, "hook exploded") {
		t.Errorf("evaluation-hook error should carry sentinel and panic value, got %q", row.Error)
	}

	if row.Since != (time.Time{}) {
		t.Errorf("synthetic row must never carry Since, got %v", row.Since)
	}

	if row.DurationNanos != 0 {
		t.Errorf("synthetic row must never carry DurationNanos, got %d", row.DurationNanos)
	}

	if svc := resp.Checks["svc"]; svc.Status != health.StatusPass {
		t.Errorf("svc status: want pass (evidence was complete), got %q", svc.Status)
	}
}

func TestWithEvaluationHook_PanicNeverLowersFail(t *testing.T) {
	t.Parallel()

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": errHookTestBoom}
	},
		health.WithCriticalServices("db"),
		health.WithEvaluationHook(func(health.Response) {
			panic("observer also broken")
		}),
	)

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusFail {
		t.Errorf("Status: want fail (critical failure must not degrade to warn), got %q", resp.Status)
	}

	if _, ok := resp.Checks["evaluation-hook"]; !ok {
		t.Error("Checks missing synthetic evaluation-hook row alongside the fail")
	}
}

func TestWithEvaluationHook_HookSeesPreDegradationResponse(t *testing.T) {
	t.Parallel()

	var sawRow atomic.Bool
	var sawStatus atomic.Value

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithEvaluationHook(func(resp health.Response) {
			_, hasRow := resp.Checks["evaluation-hook"]
			sawRow.Store(hasRow)
			sawStatus.Store(resp.Status)
			panic("after observing")
		}),
	)

	got := probe.Evaluate(context.Background())

	if sawRow.Load() {
		t.Error("hook must not see its own synthetic row (it observes the evaluation, not the degradation)")
	}

	if s := sawStatus.Load(); s != health.StatusPass {
		t.Errorf("hook should see the true classified status pass, got %v", s)
	}

	if got.Status != health.StatusWarn {
		t.Errorf("served Status: want warn, got %q", got.Status)
	}
}

func TestWithEvaluationHook_CannotMutateServedResponse(t *testing.T) {
	t.Parallel()

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithEvaluationHook(func(resp health.Response) {
			resp.Checks["injected"] = health.Check{Status: health.StatusFail, Error: "hook wrote into the map"}
			resp.Checks["svc"] = health.Check{Status: health.StatusFail}
			resp.Status = health.StatusFail
		}),
	)

	got := probe.Evaluate(context.Background())

	if _, ok := got.Checks["injected"]; ok {
		t.Error("hook-injected check leaked into the served response")
	}

	if svc := got.Checks["svc"]; svc.Status != health.StatusPass {
		t.Errorf("svc status corrupted by hook mutation: got %q, want pass", svc.Status)
	}

	if got.Status != health.StatusPass {
		t.Errorf("Status corrupted by hook mutation: got %q, want pass", got.Status)
	}
}

func TestWithEvaluationHook_PanicOnRefreshLoop_DoesNotKillLoop(t *testing.T) {
	t.Parallel()

	var hookCalls atomic.Int64

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithRefreshInterval(5*time.Millisecond),
		health.WithEvaluationHook(func(health.Response) {
			hookCalls.Add(1)
			panic("loop-path panic")
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(probe.Shutdown)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if hookCalls.Load() >= 3 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	if calls := hookCalls.Load(); calls < 3 {
		t.Fatalf("refresh loop died after a hook panic: only %d hook calls (want >= 3)", calls)
	}

	cached := probe.CachedResponse()
	if cached.Status != health.StatusWarn {
		t.Errorf("CachedResponse status: want warn, got %q", cached.Status)
	}

	if _, ok := cached.Checks["evaluation-hook"]; !ok {
		t.Error("cached response missing the evaluation-hook row")
	}
}
