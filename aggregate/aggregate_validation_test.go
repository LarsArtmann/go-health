package aggregate_test

import (
	"context"
	"errors"
	"testing"

	health "github.com/larsartmann/go-health"
	"github.com/larsartmann/go-health/aggregate"
)

// A source probe whose WithCriticalServices name never runs in a batch fails
// its own Start (the boot contract is per-probe, not per-aggregate), and the
// aggregate stays constructible while that source contributes never-started
// emptiness and keeps the aggregate startup latch open. The loud failure must
// surface at the source's Start — never as a silent pass inside the merge.
func TestSourceCriticalValidationComposesIntoAggregate(t *testing.T) {
	t.Parallel()

	healthy := health.NewChecks(map[string]health.CheckFunc{
		"ok": func(context.Context) error { return nil },
	})
	misconfigured := health.NewChecks(map[string]health.CheckFunc{
		"ok": func(context.Context) error { return nil },
	}, health.WithCriticalServices("typoed-critical"))

	err := misconfigured.Start(context.Background())
	if !errors.Is(err, health.ErrUnknownCriticalService) {
		t.Fatalf("misconfigured source Start() = %v, want ErrUnknownCriticalService", err)
	}

	misconfigured.Shutdown()

	if err := healthy.Start(context.Background()); err != nil {
		t.Fatalf("healthy source Start() = %v", err)
	}

	t.Cleanup(healthy.Shutdown)

	agg, err := aggregate.New(
		aggregate.Source{Name: "healthy", Probe: healthy},
		aggregate.Source{Name: "misconfigured", Probe: misconfigured},
	)
	if err != nil {
		t.Fatalf("aggregate.New() = %v, want nil (validation is per-probe)", err)
	}

	resp := agg.CachedResponse()
	if _, ok := resp.Checks["healthy/ok"]; !ok {
		t.Fatalf("aggregate response missing healthy/ok, got %v", resp.Checks)
	}

	if len(resp.Checks) != 1 {
		t.Fatalf(
			"aggregate checks = %v, want only healthy/ok (a never-started source contributes emptiness)",
			resp.Checks,
		)
	}

	if agg.StartupComplete() {
		t.Fatal("aggregate StartupComplete() = true, want false while a source never latched")
	}
}
