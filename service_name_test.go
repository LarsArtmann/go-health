package health_test

import (
	"testing"

	health "github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// TestWithCriticalService_UsesDosInferredName pins the typed option against
// the name samber/do actually registered: the check key in the response is
// the same string DoServiceName reports, and marking that name critical is
// enough to fail readiness. If either side ever drifts, this test fails
// instead of a consumer's startup latch hanging at run time.
func TestWithCriticalService_UsesDosInferredName(t *testing.T) {
	t.Parallel()

	injector := do.New()
	do.ProvideValue(injector, &unhealthyService{reason: "database down"})

	t.Cleanup(func() { injector.Shutdown() })

	probe := health.New(injector, health.WithCriticalService[*unhealthyService]())

	resp := probe.Evaluate(t.Context())

	if resp.Status != health.StatusFail {
		t.Errorf("critical service via typed option: status = %q, want fail", resp.Status)
	}

	name := health.DoServiceName[*unhealthyService]()

	check, ok := resp.Checks[name]
	if !ok {
		t.Fatalf("check %q missing from response; got keys of %v", name, resp.Checks)
	}

	if check.Status != health.StatusFail {
		t.Errorf("check %q: status = %q, want fail", name, check.Status)
	}
}

// TestWithCriticalService_ComposesWithNamedCriticalSet pins that the typed
// option appends to the set instead of replacing it: both the typed service
// and a literal name stay critical in one probe.
func TestWithCriticalService_ComposesWithNamedCriticalSet(t *testing.T) {
	t.Parallel()

	injector := do.New()
	do.ProvideValue(injector, &healthyService{})
	do.ProvideValue(injector, &unhealthyService{reason: "cache down"})

	t.Cleanup(func() { injector.Shutdown() })

	probe := health.New(injector,
		health.WithCriticalService[*healthyService](),
		health.WithCriticalServices("named-service"),
	)

	if err := probe.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	resp := probe.Evaluate(t.Context())

	// The non-critical unhealthy service stays warn-tier while the typed
	// critical service passes.
	if resp.Status != health.StatusWarn {
		t.Errorf("status = %q, want warn", resp.Status)
	}
}
