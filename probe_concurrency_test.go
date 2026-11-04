package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Concurrency tests ---.

func TestReadiness_ConcurrentAccess_AllSucceed(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector, health.WithCriticalServices("db"))
	mustStart(t, probe, t.Context())

	defer probe.Shutdown()

	handler := probe.ReadinessHandler()

	const goroutines = 1000

	var wg sync.WaitGroup

	wg.Add(goroutines)

	var failures atomic.Int64

	for range goroutines {
		go func() {
			defer wg.Done()

			w := httptest.NewRecorder()

			r, err := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				"/readyz",
				nil,
			)
			if err != nil {
				failures.Add(1)

				return
			}

			handler(w, r)

			if w.Code != http.StatusOK {
				failures.Add(1)
			}
		}()
	}

	wg.Wait()

	if f := failures.Load(); f > 0 {
		t.Errorf("concurrent readiness: %d/%d requests failed", f, goroutines)
	}
}

func TestEvaluate_ConcurrentAccess_NoRace(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	provideUnhealthy(injector, "metrics", "exporter down")
	invoke[*healthyService](t, injector, "db")
	invoke[*unhealthyService](t, injector, "metrics")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	const goroutines = 100

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			_ = probe.Evaluate(context.Background())
		}()
	}

	wg.Wait()
}

func TestShutdown_Idempotent(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector, health.WithCriticalServices("db"))
	mustStart(t, probe, t.Context())

	probe.Shutdown()
	probe.Shutdown() // must not panic or hang

	w := doRequest(t, probe.ReadinessHandler(), "/readyz")
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("post-shutdown readiness: want 503, got %d", w.Code)
	}
}

func TestShutdown_WithoutStart_DoesNotPanic(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithCriticalServices("db"))

	// Must not panic or hang when Start was never called.
	probe.Shutdown()

	w := doRequest(t, probe.ReadinessHandler(), "/readyz")
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("readiness after Shutdown-without-Start: want 503, got %d", w.Code)
	}
}
