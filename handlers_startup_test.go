package health_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Startup tests ---.

func TestStartup_LatchesOnceAllCriticalPass(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	// First call: all critical healthy -> 200, latches.
	w1 := doRequest(t, probe.StartupHandler(), "/startupz")
	if w1.Code != http.StatusOK {
		t.Fatalf("first startup: want 200, got %d", w1.Code)
	}

	if !probe.StartupComplete() {
		t.Error("StartupComplete should be true after all critical pass")
	}

	// Second call: latched -> still 200.
	w2 := doRequest(t, probe.StartupHandler(), "/startupz")
	if w2.Code != http.StatusOK {
		t.Fatalf("latched startup: want 200, got %d", w2.Code)
	}
}

func TestStartup_NeverInvokedService_AppearsHealthyInSamberDo(t *testing.T) {
	t.Parallel()

	// Documents samber/do v2.1.0 behavior: never-invoked services appear in
	// HealthCheckWithContext results with nil error (no actual HealthCheck call
	// is made). The startup probe therefore treats them as healthy. For the
	// startup probe to be meaningful, eagerly invoke critical services at boot
	// so their HealthCheck methods are actually exercised.
	injector := do.New()
	provideHealthy(injector, "db") // registered, NOT invoked

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	w := doRequest(t, probe.StartupHandler(), "/startupz")

	if w.Code != http.StatusOK {
		t.Fatalf(
			"startup with never-invoked service: want 200 (samber/do reports healthy), got %d",
			w.Code,
		)
	}

	if !probe.StartupComplete() {
		t.Error(
			"StartupComplete should be true (samber/do v2.1.0 reports never-invoked as healthy)",
		)
	}
}

func TestStartup_CriticalServiceUnhealthy_Returns503(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideUnhealthy(injector, "db", "starting up")
	invoke[*unhealthyService](t, injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	w := doRequest(t, probe.StartupHandler(), "/startupz")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("startup with unhealthy critical: want 503, got %d", w.Code)
	}
}

// TestStartup_CriticalNameMissingFromBatch_DoesNotLatch covers the case
// Start's batch validation cannot: a recorder answers with whatever it likes,
// so a configured critical name can be absent from a later batch. Treating
// absence as healthy would latch startup on a dependency that was never
// checked — the exact false confidence the critical set exists to prevent.
func TestStartup_CriticalNameMissingFromBatch_DoesNotLatch(t *testing.T) {
	t.Parallel()

	probe := health.NewWithRecorder(
		health.HealthRecorderFunc(
			func(context.Context, do.Injector) map[string]error {
				return map[string]error{"cache": nil}
			},
		),
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	w := doRequest(t, probe.StartupHandler(), "/startupz")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("startup with a missing critical name: want 503, got %d", w.Code)
	}

	if probe.StartupComplete() {
		t.Error("StartupComplete should stay false while a critical name never appears")
	}
}

func TestStartup_NoCriticalServices_ImmediatelyPasses(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	w := doRequest(t, probe.StartupHandler(), "/startupz")

	if w.Code != http.StatusOK {
		t.Fatalf("startup with no critical services: want 200, got %d", w.Code)
	}

	if !probe.StartupComplete() {
		t.Error("StartupComplete should be true when there are no critical services")
	}
}

func TestStartup_SlowCriticalService_TimesOut_DoesNotLatch(t *testing.T) {
	t.Parallel()

	injector := do.New()
	do.ProvideNamed(injector, "slow", func(_ do.Injector) (*slowService, error) {
		return &slowService{delay: 200 * time.Millisecond}, nil
	})
	invoke[*slowService](t, injector, "slow")

	// 1ms timeout — the slow service cannot complete in time.
	probe := health.New(injector,
		health.WithCriticalServices("slow"),
		health.WithTimeout(1*time.Millisecond),
		health.WithRefreshInterval(0),
	)

	w := doRequest(t, probe.StartupHandler(), "/startupz")

	// The startup probe must not latch when critical services time out.
	if probe.StartupComplete() {
		t.Error("StartupComplete should be false when critical service timed out")
	}

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("startup with timed-out critical service: want 503, got %d", w.Code)
	}
}
