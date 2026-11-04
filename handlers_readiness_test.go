package health_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Readiness tests ---.

func TestReadiness_AllHealthy_Returns200(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	provideHealthy(injector, "cache")
	invoke[*healthyService](t, injector, "db")
	invoke[*healthyService](t, injector, "cache")

	probe := health.New(injector,
		health.WithCriticalServices("db", "cache"),
		health.WithRefreshInterval(0),
	)

	w := doRequest(t, probe.ReadinessHandler(), "/readyz")

	if w.Code != http.StatusOK {
		t.Fatalf("readiness status: want 200, got %d", w.Code)
	}

	resp := decodeResponse(t, w)
	if resp.Status != health.StatusPass {
		t.Errorf("readiness status field: want pass, got %s", resp.Status)
	}

	if len(resp.Checks) != 2 {
		t.Errorf("checks count: want 2, got %d", len(resp.Checks))
	}
}

func TestReadiness_CriticalFailure_Returns503(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "cache")
	provideUnhealthy(injector, "db", "connection refused")
	invoke[*healthyService](t, injector, "cache")
	invoke[*unhealthyService](t, injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	w := doRequest(t, probe.ReadinessHandler(), "/readyz")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status: want 503, got %d", w.Code)
	}

	resp := decodeResponse(t, w)
	if resp.Status != health.StatusFail {
		t.Errorf("readiness status field: want fail, got %s", resp.Status)
	}

	dbCheck, ok := resp.Checks["db"]
	if !ok {
		t.Fatal("db check missing from response")
	}

	if dbCheck.Status != health.StatusFail {
		t.Errorf("db check status: want fail, got %s", dbCheck.Status)
	}

	if dbCheck.Error != "service unhealthy: connection refused" {
		t.Errorf(
			"db check error: want 'service unhealthy: connection refused', got %q",
			dbCheck.Error,
		)
	}
}

func TestReadiness_NonCriticalFailure_Returns200(t *testing.T) {
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

	w := doRequest(t, probe.ReadinessHandler(), "/readyz")

	if w.Code != http.StatusOK {
		t.Fatalf("readiness status: want 200 (non-critical failure), got %d", w.Code)
	}

	resp := decodeResponse(t, w)
	if resp.Status != health.StatusWarn {
		t.Errorf("readiness status field: want warn (non-critical failure), got %s", resp.Status)
	}

	metricsCheck, ok := resp.Checks["metrics"]
	if !ok {
		t.Fatal("metrics check missing from response")
	}

	if metricsCheck.Status != health.StatusWarn {
		t.Errorf(
			"metrics check status: want warn (non-critical failure), got %s",
			metricsCheck.Status,
		)
	}
}

func TestReadiness_ShuttingDown_Returns503(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	probe.Shutdown()

	w := doRequest(t, probe.ReadinessHandler(), "/readyz")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status during shutdown: want 503, got %d", w.Code)
	}

	resp := decodeResponse(t, w)
	if !resp.ShuttingDown {
		t.Error("response should show shutting_down=true")
	}
}

func TestReadiness_MarkShuttingDown_DoesNotStopBackgroundLoop(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector, health.WithCriticalServices("db"))

	mustStart(t, probe, t.Context())
	probe.MarkShuttingDown()

	// Give the loop time to prove it is still running.
	time.Sleep(50 * time.Millisecond)

	cached := probe.Evaluate(context.Background())
	if !cached.ShuttingDown {
		t.Error("Evaluate should reflect shutting down state")
	}

	probe.Shutdown()
}

func TestReadiness_NoServices_Returns200(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	w := doRequest(t, probe.ReadinessHandler(), "/readyz")

	if w.Code != http.StatusOK {
		t.Fatalf("readiness with no services: want 200, got %d", w.Code)
	}
}

// --- Readiness cache tests ---.

func TestReadiness_CachedMode_ServesFromCache(t *testing.T) {
	t.Parallel()

	injector := do.New()
	svc := provideCounting(injector, "db")
	invoke[*countingService](t, injector, "db")

	probe := health.New(injector,
		health.WithRefreshInterval(0), // live mode for Evaluate
	)

	// Manually populate cache via Start.
	mustStart(t, probe, t.Context())
	defer probe.Shutdown()

	// Wait for initial evaluation.
	time.Sleep(50 * time.Millisecond)

	initialCalls := svc.calls.Load()

	// Hit readiness 10 times — should not increase call count.
	for range 10 {
		w := doRequest(t, probe.ReadinessHandler(), "/readyz")
		if w.Code != http.StatusOK {
			t.Fatalf("readiness status: want 200, got %d", w.Code)
		}
	}

	if calls := svc.calls.Load(); calls != initialCalls {
		t.Errorf(
			"cached readiness should not call HealthCheck, initial=%d, after=%d",
			initialCalls,
			calls,
		)
	}
}
