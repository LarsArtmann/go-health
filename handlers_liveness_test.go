package health_test

import (
	"net/http"
	"testing"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Liveness tests ---.

func TestLiveness_AlwaysReturns200(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector)

	w := doRequest(t, probe.LivenessHandler(), "/healthz")

	if w.Code != http.StatusOK {
		t.Fatalf("liveness status: want 200, got %d", w.Code)
	}

	resp := decodeResponse(t, w)
	if resp.Status != health.StatusPass {
		t.Errorf("liveness status field: want pass, got %s", resp.Status)
	}
}

func TestLiveness_PerformsNoDependencyChecks(t *testing.T) {
	t.Parallel()

	injector := do.New()
	svc := provideCounting(injector, "db")
	invoke[*countingService](t, injector, "db")

	probe := health.New(injector)
	w := doRequest(t, probe.LivenessHandler(), "/healthz")

	if w.Code != http.StatusOK {
		t.Fatalf("liveness status: want 200, got %d", w.Code)
	}

	if calls := svc.calls.Load(); calls != 0 {
		t.Errorf(
			"liveness should not check dependencies, but HealthCheck was called %d times",
			calls,
		)
	}
}

func TestLiveness_ContainsVersionAndUptime(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithVersion("v1.2.3"))

	w := doRequest(t, probe.LivenessHandler(), "/healthz")
	resp := decodeResponse(t, w)

	if resp.Version != "v1.2.3" {
		t.Errorf("version: want v1.2.3, got %s", resp.Version)
	}

	if resp.Uptime == "" {
		t.Error("uptime should not be empty")
	}
}
