package health_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Lifecycle tests ---.

func TestStart_PerformsImmediateEvaluation(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector, health.WithCriticalServices("db"))

	mustStart(t, probe, t.Context())

	// Cache should be populated immediately, before any tick.
	w := doRequest(t, probe.ReadinessHandler(), "/readyz")
	if w.Code != http.StatusOK {
		t.Errorf("readiness after Start: want 200, got %d", w.Code)
	}

	probe.Shutdown()
}

func TestStart_InvalidTimeout_ReturnsError(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithTimeout(0))

	err := probe.Start(t.Context())
	if !errors.Is(err, health.ErrInvalidTimeout) {
		t.Fatalf("Start with zero timeout: want ErrInvalidTimeout, got %v", err)
	}
}

func TestStart_InvalidRefreshInterval_ReturnsError(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithRefreshInterval(-1))

	err := probe.Start(t.Context())
	if !errors.Is(err, health.ErrInvalidRefreshInterval) {
		t.Fatalf(
			"Start with negative refresh interval: want ErrInvalidRefreshInterval, got %v",
			err,
		)
	}
}

func TestRefreshLoop_TickerFiresBeforeShutdown(t *testing.T) {
	t.Parallel()

	injector := do.New()
	svc := provideCounting(injector, "db")
	invoke[*countingService](t, injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(5*time.Millisecond),
	)

	mustStart(t, probe, t.Context())

	// Poll for the initial evaluation plus at least one ticker-driven
	// refresh instead of sleeping a fixed window: under a loaded scheduler
	// even a 5ms ticker can starve past any fixed wait (observed under the
	// race-stress fan-out), and the property under test is "the loop fires",
	// not "it fires within 50ms".
	deadline := time.Now().Add(5 * time.Second)
	for svc.calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	probe.Shutdown()

	// The initial evaluation + at least one ticker-driven refresh should
	// have called HealthCheck more than once.
	if calls := svc.calls.Load(); calls < 2 {
		t.Errorf("expected at least 2 health-check calls (initial + ticker), got %d", calls)
	}
}

func TestShutdown_StopsBackgroundLoop(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(10*time.Millisecond))

	mustStart(t, probe, t.Context())
	probe.Shutdown()

	// Should not panic or hang.
	w := doRequest(t, probe.ReadinessHandler(), "/readyz")
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("readiness after Shutdown: want 503, got %d", w.Code)
	}
}

func TestStart_CalledTwice_IsNoOp(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(10*time.Millisecond))

	mustStart(t, probe, t.Context())
	mustStart(t, probe, t.Context()) // should not panic or start a second loop
	probe.Shutdown()
}

func TestReadiness_BeforeStart_EvaluatesLive(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	// Default refresh interval > 0, but Start not called -> no cache -> live eval.
	probe := health.New(injector, health.WithCriticalServices("db"))

	w := doRequest(t, probe.ReadinessHandler(), "/readyz")
	if w.Code != http.StatusOK {
		t.Fatalf("readiness before Start (live fallback): want 200, got %d", w.Code)
	}
}
