package health_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	do "github.com/samber/do/v2"

	health "github.com/larsartmann/go-health"
)

// newStressProbe builds a started probe with a fast background refresh, so
// the refresh loop, evaluations, and shutdown interleave under load.
func newStressProbe(t *testing.T) (*health.Probe, context.Context) {
	t.Helper()

	injector := do.New()
	t.Cleanup(func() { injector.Shutdown() })

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

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
// double-started refresh loops.
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
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range rounds {
				ctx, cancel := context.WithCancel(baseCtx)

				_ = probe.Start(ctx) // may be a no-op when already started
				probe.MarkShuttingDown()
				probe.Shutdown()

				cancel()
			}
		}()
	}

	for range readers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range rounds {
				resp := probe.Evaluate(context.Background())
				_ = resp.Status

				cached := probe.CachedResponse()
				_ = cached.Status
			}
		}()
	}

	wg.Wait()

	probe.Shutdown()
}

// TestStress_MarkThenShutdown_TwoPhase drains via MarkShuttingDown first and
// stops the loop later, with readers asserting the drain invariant the whole
// time: once marked, cached responses must report ShuttingDown with fail.
func TestStress_MarkThenShutdown_TwoPhase(t *testing.T) {
	t.Parallel()

	probe, _ := newStressProbe(t)

	stop := make(chan struct{})

	var readers sync.WaitGroup

	readers.Add(1)

	go func() {
		defer readers.Done()

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
	}()

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

	ctx, cancel := context.WithCancel(context.Background())
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
