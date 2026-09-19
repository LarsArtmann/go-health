package health_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
)

var (
	errConnRefused = errors.New("connection refused")
	errDBDown      = errors.New("down")
)

func TestNewChecks_ReportsNamedPassAndFail(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(map[string]health.CheckFunc{
		"ok-check":  func(_ context.Context) error { return nil },
		"bad-check": func(_ context.Context) error { return errConnRefused },
	})

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusWarn {
		t.Errorf("non-critical failure should degrade to warn, got %q", resp.Status)
	}

	bad, ok := resp.Checks["bad-check"]

	if !ok {
		t.Fatalf("expected a 'bad-check' entry, got checks %v", keysOf(resp.Checks))
	}

	if bad.Status != health.StatusWarn || !strings.Contains(bad.Error, "connection refused") {
		t.Errorf("expected warn + error message on 'bad-check', got %+v", bad)
	}

	if okCheck := resp.Checks["ok-check"]; okCheck.Status != health.StatusPass {
		t.Errorf("expected 'ok-check' to pass, got %+v", okCheck)
	}
}

func TestNewChecks_CriticalFailureFailsReadiness(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(map[string]health.CheckFunc{
		"db": func(_ context.Context) error { return errDBDown },
	}, health.WithCriticalServices("db"))

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusFail {
		t.Errorf("critical failure should fail the batch, got %q", resp.Status)
	}
}

func TestNewChecks_RunsConcurrently(t *testing.T) {
	t.Parallel()

	a, b := make(chan struct{}), make(chan struct{})

	// Each check only proceeds after the other has started: the batch can
	// only complete when execution is concurrent (a serial runner would
	// deadlock and the watchdog below would fail the test).
	probe := health.NewChecks(map[string]health.CheckFunc{
		"a": func(_ context.Context) error {
			close(a)
			<-b

			return nil
		},
		"b": func(_ context.Context) error {
			close(b)
			<-a

			return nil
		},
	})

	done := make(chan health.Response, 1)

	go func() { done <- probe.Evaluate(context.Background()) }()

	select {
	case resp := <-done:
		if resp.Status != health.StatusPass {
			t.Errorf("expected pass, got %q (%v)", resp.Status, resp.Checks)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("batch did not complete: checks are not running concurrently")
	}
}

func TestNewChecks_RecoversPanickingCheck(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(map[string]health.CheckFunc{
		"boom": func(_ context.Context) error { panic("exploded") },
		"calm": func(_ context.Context) error { return nil },
	}, health.WithCriticalServices("boom"))

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusFail {
		t.Errorf("a panicked critical check must fail the batch, got %q", resp.Status)
	}

	boom := resp.Checks["boom"]

	if !strings.Contains(boom.Error, "boom") || !strings.Contains(boom.Error, "exploded") {
		t.Errorf("expected the panic error to name the check and value, got %q", boom.Error)
	}

	if calm := resp.Checks["calm"]; calm.Status != health.StatusPass {
		t.Errorf("siblings must be unaffected by a panicking check, got %+v", calm)
	}
}

func TestNewChecks_NilCheckFailsClosed(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(map[string]health.CheckFunc{
		"unwired": nil,
	})

	resp := probe.Evaluate(context.Background())

	if got := resp.Checks["unwired"]; !strings.Contains(got.Error, "nil") {
		t.Errorf("expected a fail-closed nil-check error, got %+v", got)
	}
}

func TestNewChecks_HungCheckFailsClosedAtDeadline(t *testing.T) {
	t.Parallel()

	hung := make(chan struct{})
	defer close(hung) // release the abandoned check goroutine

	probe := health.NewChecks(map[string]health.CheckFunc{
		"wedged": func(_ context.Context) error {
			<-hung

			return nil
		},
		"quick":  func(_ context.Context) error { return nil },
	})

	// Evaluate honors the caller's deadline (handlers and Start apply
	// p.timeout themselves; direct callers bound their own context).
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	resp := probe.Evaluate(ctx)
	elapsed := time.Since(start)

	wedged := resp.Checks["wedged"]

	if !strings.Contains(wedged.Error, "wedged") || !strings.Contains(wedged.Error, "deadline") {
		t.Errorf("expected a fail-closed deadline error naming the check, got %q", wedged.Error)
	}

	if resp.Status != health.StatusWarn {
		t.Errorf("expected the batch to degrade (non-critical wedged check), got %q", resp.Status)
	}

	if elapsed > 2*time.Second {
		t.Errorf("probe must answer at the batch deadline, took %s", elapsed)
	}
}

func TestNewChecks_DurationPopulated(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(map[string]health.CheckFunc{
		"slowish": func(_ context.Context) error {
			time.Sleep(5 * time.Millisecond)

			return nil
		},
	})

	resp := probe.Evaluate(context.Background())

	if resp.Checks["slowish"].DurationNanos <= 0 {
		t.Errorf("expected executor-reported duration, got %+v", resp.Checks["slowish"])
	}
}

func TestNewChecks_EmptyMapPasses(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(map[string]health.CheckFunc{})

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusPass || len(resp.Checks) != 0 {
		t.Errorf("an empty check set should pass with no entries, got %q %v", resp.Status, resp.Checks)
	}
}

func ExampleNewChecks() {
	probe := health.NewChecks(map[string]health.CheckFunc{
		"database": func(_ context.Context) error { return nil },
		"cache":    func(_ context.Context) error { return nil },
	})

	resp := probe.Evaluate(context.Background())

	names := make([]string, 0, len(resp.Checks))
	for name := range resp.Checks {
		names = append(names, name)
	}

	slices.Sort(names)

	fmt.Println(resp.Status, names)
	// Output: pass [cache database]
}

func keysOf(checks map[string]health.Check) []string {
	names := make([]string, 0, len(checks))
	for name := range checks {
		names = append(names, name)
	}

	return names
}
