package health_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	do "github.com/samber/do/v2"

	health "github.com/larsartmann/go-health"
)

// --- P25: Status / Alive / Ready accessors ---.

func TestAccessors_StatusAliveReady(t *testing.T) {
	t.Parallel()

	injector := do.New()
	t.Cleanup(func() { injector.Shutdown() })

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	if !probe.Alive() {
		t.Error("Alive: want true before shutdown")
	}

	if probe.Status() != health.StatusPass {
		t.Errorf(
			"Status before any evaluation: want pass (cached fallback), got %s",
			probe.Status(),
		)
	}

	if !probe.Ready() {
		t.Error("Ready: want true while pass")
	}

	probe.Shutdown()

	if probe.Alive() {
		t.Error("Alive: want false after Shutdown")
	}

	if probe.Ready() {
		t.Error("Ready: want false after Shutdown")
	}

	if probe.Status() != health.StatusFail {
		t.Errorf("Status after Shutdown: want fail, got %s", probe.Status())
	}
}

// --- P26: AwaitReady + Healthz ---.

func TestAwaitReady_ReturnsImmediatelyWhenReady(t *testing.T) {
	t.Parallel()

	injector := do.New()
	t.Cleanup(func() { injector.Shutdown() })

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	if err := probe.AwaitReady(ctx); err != nil {
		t.Errorf("AwaitReady on healthy probe: %v", err)
	}
}

func TestAwaitReady_TimesOutWhenFailing(t *testing.T) {
	t.Parallel()

	injector := do.New()
	t.Cleanup(func() { injector.Shutdown() })

	provideUnhealthy(injector, "db", "down")
	invoke[*unhealthyService](t, injector, "db")

	probe := health.New(injector, health.WithCriticalServices("db"))
	probe.Shutdown() // one-way fail: AwaitReady must never succeed

	ctx, cancel := context.WithTimeout(t.Context(), 120*time.Millisecond)
	defer cancel()

	if err := probe.AwaitReady(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("AwaitReady on failing probe: want DeadlineExceeded, got %v", err)
	}
}

func TestHealthz_CombinesStartupAndReadiness(t *testing.T) {
	t.Parallel()

	injector := do.New()
	t.Cleanup(func() { injector.Shutdown() })

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	handler := probe.Healthz()
	routes := health.DefaultRoutes()

	// Booting: latch unset → 503 with a startup check entry.
	rec := doRequest(t, handler, routes.Readiness)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("booting: want 503, got %d", rec.Code)
	}

	if body := rec.Body.String(); !strings.Contains(body, "startup latch not set") {
		t.Errorf("booting body missing startup entry: %s", body)
	}

	// After one successful startup-handler batch the latch is set to 200.
	// (Evaluate alone does not latch; only the startup path does.)
	rec = doRequest(t, probe.StartupHandler(), routes.Startup)
	if rec.Code != http.StatusOK {
		t.Errorf("startup after first batch: want 200, got %d", rec.Code)
	}

	rec = doRequest(t, handler, routes.Readiness)
	if rec.Code != http.StatusOK {
		t.Errorf("after latch: want 200, got %d", rec.Code)
	}

	// Shutdown → 503 again.
	probe.Shutdown()

	rec = doRequest(t, handler, routes.Readiness)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("after shutdown: want 503, got %d", rec.Code)
	}
}

// --- P27: NewWithHealthCheck ---.

func TestNewWithHealthCheck_FullSurface(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64

	fn := func(context.Context) map[string]error {
		calls.Add(1)

		return map[string]error{"custom": nil}
	}

	probe := health.NewWithHealthCheck(fn,
		health.WithCriticalServices("custom"),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusPass {
		t.Errorf("status: want pass, got %s", resp.Status)
	}

	if calls.Load() != 1 {
		t.Errorf("health-check func calls: want 1, got %d", calls.Load())
	}

	// Validate-on-Start works without any injector.
	if err := probe.Start(t.Context()); err != nil {
		t.Errorf("Start: %v", err)
	}

	probe.Shutdown()
}

func TestNewWithHealthCheck_PanicRecovered(t *testing.T) {
	t.Parallel()

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		panic("custom fn exploded")
	})

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusFail {
		t.Errorf("status: want fail (fail-closed panic), got %s", resp.Status)
	}
}

// --- P32: do.HealthcheckerWithContext + shutdown adapter ---.

func TestProbe_HealthCheck_Conformance(t *testing.T) {
	t.Parallel()

	injector := do.New()
	t.Cleanup(func() { injector.Shutdown() })

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector, health.WithCriticalServices("db"))

	if err := probe.HealthCheck(t.Context()); err != nil {
		t.Errorf("HealthCheck while pass: %v", err)
	}

	probe.Shutdown()

	err := probe.HealthCheck(t.Context())
	if !errors.Is(err, health.ErrProbeUnhealthy) {
		t.Errorf("HealthCheck after Shutdown: want ErrProbeUnhealthy wrap, got %v", err)
	}
}

func TestProbe_AsShutdowner(t *testing.T) {
	t.Parallel()

	probe, _ := newStressProbe(t)

	shutdowner := probe.AsShutdowner()

	if err := shutdowner.Shutdown(); err != nil {
		t.Errorf("ProbeShutdowner.Shutdown: %v", err)
	}

	if probe.Alive() {
		t.Error("Alive after adapter Shutdown: want false")
	}
}

// --- P29: evaluation hook ---.

func TestWithEvaluationHook_InvokedPerEvaluation(t *testing.T) {
	t.Parallel()

	var (
		hookCalls atomic.Int64

		lastStatus atomic.Value
	)

	injector := do.New()
	t.Cleanup(func() { injector.Shutdown() })

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector,
		health.WithEvaluationHook(func(resp health.Response) {
			hookCalls.Add(1)
			lastStatus.Store(resp.Status)
		}),
	)

	_ = probe.Evaluate(context.Background())
	_ = probe.Evaluate(context.Background())

	if calls := hookCalls.Load(); calls != 2 {
		t.Errorf("hook calls: want 2, got %d", calls)
	}

	if got := lastStatus.Load(); got != health.StatusPass {
		t.Errorf("last hook status: want pass, got %v", got)
	}
}

// --- P30: live throttle ---.

func TestWithLiveThrottle_CoalescesLiveEvaluations(t *testing.T) {
	t.Parallel()

	var batches atomic.Int64

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		batches.Add(1)

		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithLiveThrottle(50*time.Millisecond),
	)

	handler := probe.ReadinessHandler()
	routes := health.DefaultRoutes()

	var wg sync.WaitGroup

	for range 20 {
		wg.Go(func() {
			rec := doRequest(t, handler, routes.Readiness)
			if rec.Code != http.StatusOK {
				t.Errorf("status: want 200, got %d", rec.Code)
			}
		})
	}

	wg.Wait()

	if calls := batches.Load(); calls > 5 {
		t.Errorf(
			"20 requests within the throttle window ran %d batches; want coalescing (<=5)",
			calls,
		)
	}

	// After the window elapses a fresh batch runs.
	time.Sleep(60 * time.Millisecond)

	before := batches.Load()
	rec := doRequest(t, handler, routes.Readiness)

	if rec.Code != http.StatusOK {
		t.Errorf("post-window status: want 200, got %d", rec.Code)
	}

	if after := batches.Load(); after != before+1 {
		t.Errorf("post-window batches: want %d, got %d", before+1, after)
	}
}

// --- P31: shutdown grace period ---.

func TestWithShutdownGracePeriod_BlocksButStops(t *testing.T) {
	t.Parallel()

	const grace = 120 * time.Millisecond

	// Apply the option by building a second probe with the same injector.
	injector := do.New()
	t.Cleanup(func() { injector.Shutdown() })

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	graceProbe := health.New(injector,
		health.WithRefreshInterval(5*time.Millisecond),
		health.WithShutdownGracePeriod(grace),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := graceProbe.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	done := make(chan struct{})

	go func() {
		graceProbe.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Shutdown with grace period returned before the window elapsed")
	case <-time.After(grace / 2):
		// expected: still draining
	}

	<-done

	if graceProbe.Alive() {
		t.Error("Alive after graceful Shutdown: want false")
	}
}
