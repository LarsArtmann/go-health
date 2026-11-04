package health_test

import (
	"context"
	"strings"
	"testing"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- HealthRecorder integration tests ---.

func TestWithHealthRecorder_DelegatesChecks(t *testing.T) {
	t.Parallel()

	injector := do.New()
	recorder := &mockRecorder{result: map[string]error{"db": nil}}

	probe := health.New(injector,
		health.WithHealthRecorder(recorder),
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusPass {
		t.Errorf("status: want pass, got %s", resp.Status)
	}

	if calls := recorder.calls.Load(); calls != 1 {
		t.Errorf("recorder should be called once, got %d", calls)
	}
}

func TestWithHealthRecorder_NilRecorder_FallsBackToInjector(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector,
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusPass {
		t.Errorf("status: want pass, got %s", resp.Status)
	}

	if len(resp.Checks) != 1 {
		t.Errorf("checks count: want 1, got %d", len(resp.Checks))
	}
}

func TestWithHealthRecorder_PanicRecovered_DoesNotCrash(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(),
		health.WithHealthRecorder(panicRecorder{}),
		health.WithRefreshInterval(0),
	)

	// Evaluate must not panic — the recorder panic is recovered and reported
	// as a synthetic error.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Evaluate panicked with misbehaving recorder: %v", r)
		}
	}()

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusFail {
		t.Errorf(
			"status: want fail (panics fail closed, see docs/panic-recovery-design.md), got %s",
			resp.Status,
		)
	}

	panicCheck, ok := resp.Checks["health-check"]
	if !ok {
		t.Fatal("expected 'health-check' entry for recovered panic")
	}

	if panicCheck.Status != health.StatusFail {
		t.Errorf("panic check status: want fail, got %s", panicCheck.Status)
	}

	if panicCheck.Error == "" {
		t.Error("panic check error should contain the panic message")
	}

	if !strings.Contains(panicCheck.Error, "recorder exploded") {
		t.Errorf("panic check error should contain panic message, got %q", panicCheck.Error)
	}
}

// Service panics on the injector path are deliberately NOT tested here:
// samber/do runs each service health check in its own goroutine
// (raceWithTimeout), so a panicking service crashes the process before any
// probe-side recover can run. Only the recorder path is recoverable. See
// docs/panic-recovery-design.md.
