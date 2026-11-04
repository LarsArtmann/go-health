package health_test

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Evaluate tests ---.

func TestEvaluate_ReturnsCorrectClassification(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	provideUnhealthy(injector, "cache", "redis down")
	invoke[*healthyService](t, injector, "db")
	invoke[*unhealthyService](t, injector, "cache")

	probe := health.New(injector, health.WithCriticalServices("db"))

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusWarn {
		t.Errorf("status: want warn (db healthy, cache non-critical), got %s", resp.Status)
	}

	if len(resp.Checks) != 2 {
		t.Fatalf("checks count: want 2, got %d", len(resp.Checks))
	}

	if resp.Checks["db"].Status != health.StatusPass {
		t.Error("db should pass")
	}

	if resp.Checks["cache"].Status != health.StatusWarn {
		t.Error("cache should warn (non-critical failure)")
	}
}

func TestEvaluate_CriticalFailure_ReturnsFail(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideUnhealthy(injector, "db", "unreachable")
	invoke[*unhealthyService](t, injector, "db")

	probe := health.New(injector, health.WithCriticalServices("db"))

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusFail {
		t.Errorf("status: want fail (critical db down), got %s", resp.Status)
	}
}

func TestEvaluate_IncludesUptimeAndVersion(t *testing.T) {
	t.Parallel()

	injector := do.New()
	boot := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	probe := health.New(injector, health.WithVersion("v2.0.0"), health.WithBootTime(boot))

	resp := probe.Evaluate(context.Background())

	if resp.Version != "v2.0.0" {
		t.Errorf("version: want v2.0.0, got %s", resp.Version)
	}

	if resp.Uptime == "" {
		t.Error("uptime should not be empty")
	}
}

func TestEvaluate_TotalLatencyRecorded(t *testing.T) {
	t.Parallel()

	injector := do.New()
	do.ProvideNamed(injector, "slow", func(_ do.Injector) (*slowService, error) {
		return &slowService{delay: 50 * time.Millisecond}, nil
	})
	invoke[*slowService](t, injector, "slow")

	probe := health.New(injector, health.WithRefreshInterval(0))

	resp := probe.Evaluate(context.Background())

	if resp.TotalLatencyMs < 1 {
		t.Errorf("total latency should be > 0, got %dms", resp.TotalLatencyMs)
	}
}

func TestEvaluate_MixedFailures_CriticalFailNonCriticalWarn(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideUnhealthy(injector, "db", "unreachable")
	provideUnhealthy(injector, "metrics", "exporter down")
	invoke[*unhealthyService](t, injector, "db")
	invoke[*unhealthyService](t, injector, "metrics")

	probe := health.New(injector, health.WithCriticalServices("db"))

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusFail {
		t.Errorf("roll-up status: want fail (critical db down), got %s", resp.Status)
	}

	if resp.Checks["db"].Status != health.StatusFail {
		t.Errorf("db check: want fail (critical), got %s", resp.Checks["db"].Status)
	}

	if resp.Checks["metrics"].Status != health.StatusWarn {
		t.Errorf("metrics check: want warn (non-critical), got %s", resp.Checks["metrics"].Status)
	}

	if resp.Checks["metrics"].Error != "service unhealthy: exporter down" {
		t.Errorf("metrics error: want message, got %q", resp.Checks["metrics"].Error)
	}
}

func TestEvaluate_AllNonCriticalFailures_RollupIsWarn(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideUnhealthy(injector, "metrics", "exporter down")
	provideUnhealthy(injector, "feature-flags", "service unavailable")
	invoke[*unhealthyService](t, injector, "metrics")
	invoke[*unhealthyService](t, injector, "feature-flags")
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector, health.WithCriticalServices("db"))

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusWarn {
		t.Errorf("roll-up status: want warn (only non-critical failures), got %s", resp.Status)
	}

	for _, name := range []string{"metrics", "feature-flags"} {
		if resp.Checks[name].Status != health.StatusWarn {
			t.Errorf("%s check: want warn, got %s", name, resp.Checks[name].Status)
		}
	}

	if resp.Checks["db"].Status != health.StatusPass {
		t.Errorf("db check: want pass, got %s", resp.Checks["db"].Status)
	}
}
