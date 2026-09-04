package health_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	do "github.com/samber/do/v2"
)

// newStressProbe builds a started probe with a fast background refresh, so
// the refresh loop, evaluations, and shutdown interleave under load.
func newStressProbe(t *testing.T) (*health.Probe, context.Context) {
	t.Helper()

	injector := do.New()

	provideHealthy(injector, "db")

	invoke[*healthyService](t, injector, "db")

	t.Cleanup(func() { injector.Shutdown() })

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(5*time.Millisecond),
		health.WithTimeout(time.Second),
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}

	return probe, ctx
}

// TestStress_ConcurrentStartAndShutdown hammers Start, Shutdown, and
// MarkShuttingDown from many goroutines while readers evaluate health and
// consume cached responses. Run under -race it verifies the lifecycle's
// mutex/atomic discipline: no data races, no panics, no goroutine leaks from
// double-started refresh loops. It guards a real regression: Start's
// WaitGroup.Add and Shutdown's WaitGroup.Wait must stay serialized under the
// probe mutex, or WaitGroup panics with "reused before previous Wait".
func TestStress_ConcurrentStartAndShutdown(t *testing.T) {
	t.Parallel()

	probe, baseCtx := newStressProbe(t)

	const (
		writers = 8
		rounds  = 50
		readers = 4
	)

	var wg sync.WaitGroup

	for range writers {
		wg.Go(func() {
			for range rounds {
				ctx, cancel := context.WithCancel(baseCtx)

				_ = probe.Start(ctx) // may be a no-op when already started
				probe.MarkShuttingDown()
				probe.Shutdown()

				cancel()
			}
		})
	}

	for range readers {
		wg.Go(func() {
			for range rounds {
				resp := probe.Evaluate(context.Background())
				_ = resp.Status

				cached := probe.CachedResponse()
				_ = cached.Status
			}
		})
	}

	wg.Wait()

	probe.Shutdown()
}

// TestStress_MarkThenShutdown_TwoPhase drains via MarkShuttingDown first and
// stops the loop later, with a reader asserting the drain invariant the whole
// time: once marked, cached responses must report ShuttingDown with fail.
func TestStress_MarkThenShutdown_TwoPhase(t *testing.T) {
	t.Parallel()

	probe, _ := newStressProbe(t)

	stop := make(chan struct{})

	var readers sync.WaitGroup

	readers.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}

			cached := probe.CachedResponse()

			if cached.ShuttingDown && cached.Status != health.StatusFail {
				t.Errorf("cached response while shutting down: want fail, got %s", cached.Status)

				return
			}
		}
	})

	const phases = 25

	for range phases {
		probe.MarkShuttingDown()

		if cached := probe.CachedResponse(); !cached.ShuttingDown {
			t.Fatalf("after MarkShuttingDown, ShuttingDown must be true")
		}

		probe.Shutdown()
	}

	close(stop)
	readers.Wait()
}

// TestStart_AfterShutdown_RestartsLoopButStaysDown pins the restart contract:
// Start after Shutdown re-validates and may re-arm the loop, but the probe
// remains in the shutting-down state — readiness stays 503, liveness stays
// 200. There is intentionally no API to un-shutdown a probe.
func TestStart_AfterShutdown_RestartsLoopButStaysDown(t *testing.T) {
	t.Parallel()

	probe, _ := newStressProbe(t)
	probe.Shutdown()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("Start after Shutdown: %v", err)
	}

	cached := probe.CachedResponse()
	if !cached.ShuttingDown {
		t.Error("ShuttingDown: want true after Shutdown, got false")
	}

	if cached.Status != health.StatusFail {
		t.Errorf("cached status after Shutdown: want fail, got %s", cached.Status)
	}

	liveness := probe.LivenessHandler()
	rec := doRequest(t, liveness, health.DefaultRoutes().Liveness)

	if rec.Code != http.StatusOK {
		t.Errorf("liveness after restart: want 200, got %d", rec.Code)
	}

	readiness := probe.ReadinessHandler()
	rec = doRequest(t, readiness, health.DefaultRoutes().Readiness)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("readiness after restart: want 503, got %d", rec.Code)
	}
}

// flappableService models a dependency whose health changes between requests.
type flappableService struct {
	up atomic.Bool
}

var errStillBooting = errors.New("still booting")

func (s *flappableService) HealthCheck(_ context.Context) error {
	if s.up.Load() {
		return nil
	}

	return errStillBooting
}

// The startup latch is one-way in the public API. ResetStartupLatchForTest
// (test builds only) clears it so the full latch lifecycle can be exercised
// on a single Probe: latch, re-evaluate after reset, re-latch.
func TestStartupLatch_ResetForTest_ReEvaluatesAndRelatches(t *testing.T) {
	t.Parallel()

	injector := do.New()

	svc := &flappableService{}
	svc.up.Store(true)

	do.ProvideNamed(injector, "db", func(_ do.Injector) (*flappableService, error) {
		return svc, nil
	})
	invoke[*flappableService](t, injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	startup := probe.StartupHandler()
	path := health.DefaultRoutes().Startup

	w := doRequest(t, startup, path)
	if w.Code != http.StatusOK {
		t.Fatalf("healthy startup: want 200, got %d", w.Code)
	}

	if !probe.StartupComplete() {
		t.Fatal("latch should be set after all critical services pass")
	}

	svc.up.Store(false)
	probe.ResetStartupLatchForTest()

	w = doRequest(t, startup, path)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("after reset with failing service: want 503, got %d", w.Code)
	}

	if probe.StartupComplete() {
		t.Error("latch should be clear after reset with failing service")
	}

	svc.up.Store(true)

	w = doRequest(t, startup, path)
	if w.Code != http.StatusOK {
		t.Fatalf("after recovery: want 200, got %d", w.Code)
	}

	if !probe.StartupComplete() {
		t.Error("latch should be set again after successful re-evaluation")
	}
}
