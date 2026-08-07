package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Test service types ---.

type healthyService struct{}

var _ do.HealthcheckerWithContext = (*healthyService)(nil)

func (healthyService) HealthCheck(_ context.Context) error { return nil }

var errUnhealthy = errors.New("service unhealthy")

type unhealthyService struct{ reason string }

var _ do.HealthcheckerWithContext = (*unhealthyService)(nil)

func (u *unhealthyService) HealthCheck(_ context.Context) error {
	return fmt.Errorf("%w: %s", errUnhealthy, u.reason)
}

type slowService struct {
	delay time.Duration
}

var _ do.HealthcheckerWithContext = (*slowService)(nil)

func (s *slowService) HealthCheck(ctx context.Context) error {
	select {
	case <-time.After(s.delay):
		return nil
	case <-ctx.Done():
		return fmt.Errorf("health check interrupted: %w", ctx.Err())
	}
}

type countingService struct {
	calls atomic.Int64
}

var _ do.HealthcheckerWithContext = (*countingService)(nil)

func (c *countingService) HealthCheck(_ context.Context) error {
	c.calls.Add(1)

	return nil
}

// --- Mock HealthRecorder ---.

type mockRecorder struct {
	calls  atomic.Int64
	result map[string]error
}

func (m *mockRecorder) RecordHealthCheckWithContext(
	_ context.Context,
	_ do.Injector,
) map[string]error {
	m.calls.Add(1)

	return m.result
}

// panicRecorder simulates a misbehaving recorder that panics during health checks.
type panicRecorder struct{}

func (panicRecorder) RecordHealthCheckWithContext(
	_ context.Context,
	_ do.Injector,
) map[string]error {
	panic("recorder exploded")
}

// --- Test helpers ---.

func provideHealthy(i do.Injector, name string) {
	do.ProvideNamed(i, name, func(_ do.Injector) (*healthyService, error) {
		return &healthyService{}, nil
	})
}

func provideUnhealthy(i do.Injector, name, reason string) {
	do.ProvideNamed(i, name, func(_ do.Injector) (*unhealthyService, error) {
		return &unhealthyService{reason: reason}, nil
	})
}

func provideCounting(i do.Injector, name string) *countingService {
	svc := &countingService{}

	do.ProvideNamed(i, name, func(_ do.Injector) (*countingService, error) {
		return svc, nil
	})

	return svc
}

func mustStart(t *testing.T, probe *health.Probe, ctx context.Context) {
	t.Helper()

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}
}

func invoke[T any](t *testing.T, i do.Injector, name string) T {
	t.Helper()

	return do.MustInvokeNamed[T](i, name)
}

func doRequest(t *testing.T, handler http.HandlerFunc, target string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}

	handler(w, r)

	return w
}

func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) health.Response {
	t.Helper()

	var resp health.Response

	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	return resp
}

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

// --- Validate tests ---.

func TestValidate_DefaultConfig_IsValid(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New())

	if err := probe.Validate(); err != nil {
		t.Errorf("default config: want nil error, got %v", err)
	}
}

func TestValidate_ZeroTimeout_ReturnsError(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithTimeout(0))

	err := probe.Validate()
	if !errors.Is(err, health.ErrInvalidTimeout) {
		t.Errorf("zero timeout: want ErrInvalidTimeout, got %v", err)
	}

	if msg := err.Error(); !strings.Contains(msg, "0s") || !strings.Contains(msg, "WithTimeout") {
		t.Errorf(
			"zero timeout: error should include the offending value and remediation, got %q",
			msg,
		)
	}
}

func TestValidate_NegativeTimeout_ReturnsError(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithTimeout(-1*time.Second))

	err := probe.Validate()
	if !errors.Is(err, health.ErrInvalidTimeout) {
		t.Errorf("negative timeout: want ErrInvalidTimeout, got %v", err)
	}

	if msg := err.Error(); !strings.Contains(msg, "-1s") || !strings.Contains(msg, "WithTimeout") {
		t.Errorf(
			"negative timeout: error should include the offending value and remediation, got %q",
			msg,
		)
	}
}

func TestValidate_NegativeRefreshInterval_ReturnsError(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithRefreshInterval(-1))

	err := probe.Validate()
	if !errors.Is(err, health.ErrInvalidRefreshInterval) {
		t.Errorf("negative refresh interval: want ErrInvalidRefreshInterval, got %v", err)
	}

	if msg := err.Error(); !strings.Contains(msg, "-1ns") ||
		!strings.Contains(msg, "WithRefreshInterval") {
		t.Errorf(
			"negative refresh interval: error should include the offending value and remediation, got %q",
			msg,
		)
	}
}

func TestValidate_ZeroRefreshInterval_IsValid(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithRefreshInterval(0))

	if err := probe.Validate(); err != nil {
		t.Errorf("zero refresh interval (live mode): want nil error, got %v", err)
	}
}

// --- RegisterRoutes tests ---.

func TestRegisterRoutes_AllThreeHandlersRegistered(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	mux := http.NewServeMux()
	probe.RegisterRoutes(mux, health.DefaultRoutes())

	for _, path := range []string{"/healthz", "/readyz", "/startupz"} {
		w := httptest.NewRecorder()

		r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		if err != nil {
			t.Fatal(err)
		}

		mux.ServeHTTP(w, r)

		if w.Code == http.StatusNotFound {
			t.Errorf("route %s not registered", path)
		}
	}
}

func TestRegisterRoutes_CustomPaths(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	mux := http.NewServeMux()
	probe.RegisterRoutes(mux, health.Routes{
		Liveness:  "/live",
		Readiness: "/ready",
		Startup:   "/started",
	})

	w := httptest.NewRecorder()

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/live", nil)
	if err != nil {
		t.Fatal(err)
	}

	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("custom liveness route: want 200, got %d", w.Code)
	}
}

// --- Content-Type and format tests ---.

func TestResponse_ContentTypeIsJSON(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector)

	w := doRequest(t, probe.LivenessHandler(), "/healthz")

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("content-type: want application/json, got %s", ct)
	}
}

func TestResponse_NoCacheHeader(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector)

	w := doRequest(t, probe.LivenessHandler(), "/healthz")

	cc := w.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("cache-control: want no-cache, got %s", cc)
	}
}

func TestReadiness_JSONChecksAreSortedAlphabetically(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideUnhealthy(injector, "zebra", "down")
	provideHealthy(injector, "alpha")
	provideUnhealthy(injector, "mongo", "slow")
	invoke[*unhealthyService](t, injector, "zebra")
	invoke[*healthyService](t, injector, "alpha")
	invoke[*unhealthyService](t, injector, "mongo")

	probe := health.New(injector, health.WithRefreshInterval(0))

	// Verify alphabetical ordering: "alpha" must appear before "mongo" before "zebra"
	// in the raw JSON body. Go's json.Marshal sorts map[string]K keys, so this is
	// guaranteed by the standard library — the test locks the property in.
	body := doRequest(t, probe.ReadinessHandler(), "/readyz").Body.String()

	indices := make([]int, 0, 3)

	for _, name := range []string{"alpha", "mongo", "zebra"} {
		idx := strings.Index(body, `"`+name+`"`)
		if idx < 0 {
			t.Fatalf("service %q missing from JSON output: %s", name, body)
		}

		indices = append(indices, idx)
	}

	if indices[0] >= indices[1] || indices[1] >= indices[2] {
		t.Errorf(
			"checks are not alphabetically sorted in JSON: positions alpha=%d, mongo=%d, zebra=%d",
			indices[0],
			indices[1],
			indices[2],
		)
	}
}

// --- HealthRecorder integration tests ---.

func TestWithHealthRecorder_DelegatesChecks(t *testing.T) {
	t.Parallel()

	injector := do.New()
	recorder := &mockRecorder{result: map[string]error{"db": nil}}

	probe := health.New(injector,
		health.WithHealthRecorder(recorder),
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusPass {
		t.Errorf("status: want pass, got %s", resp.Status)
	}

	if calls := recorder.calls.Load(); calls != 1 {
		t.Errorf("recorder should be called once, got %d", calls)
	}
}

func TestWithHealthRecorder_NilRecorder_FallsBackToInjector(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector,
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusPass {
		t.Errorf("status: want pass, got %s", resp.Status)
	}

	if len(resp.Checks) != 1 {
		t.Errorf("checks count: want 1, got %d", len(resp.Checks))
	}
}

func TestWithHealthRecorder_PanicRecovered_DoesNotCrash(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(),
		health.WithHealthRecorder(panicRecorder{}),
		health.WithRefreshInterval(0),
	)

	// Evaluate must not panic — the recorder panic is recovered and reported
	// as a synthetic error.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Evaluate panicked with misbehaving recorder: %v", r)
		}
	}()

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusWarn {
		t.Errorf("status: want warn (non-critical panic), got %s", resp.Status)
	}

	panicCheck, ok := resp.Checks["health-check"]
	if !ok {
		t.Fatal("expected 'health-check' entry for recovered panic")
	}

	if panicCheck.Error == "" {
		t.Error("panic check error should contain the panic message")
	}

	if !strings.Contains(panicCheck.Error, "recorder exploded") {
		t.Errorf("panic check error should contain panic message, got %q", panicCheck.Error)
	}
}

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

	// Wait long enough for at least 3 ticker cycles.
	time.Sleep(50 * time.Millisecond)

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

// --- GET-only enforcement tests ---.

func TestGETOnly_RejectsNonGET(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")
	probe := health.New(injector,
		health.WithGETOnly(),
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	handlers := map[string]http.HandlerFunc{
		"/healthz":  probe.LivenessHandler(),
		"/readyz":   probe.ReadinessHandler(),
		"/startupz": probe.StartupHandler(),
	}

	for path, handler := range handlers {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodHead} {
			w := httptest.NewRecorder()

			r, err := http.NewRequestWithContext(t.Context(), method, path, nil)
			if err != nil {
				t.Fatal(err)
			}

			handler(w, r)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: want 405, got %d", method, path, w.Code)
			}

			if allow := w.Header().Get("Allow"); allow != "GET" {
				t.Errorf("%s %s Allow header: want GET, got %s", method, path, allow)
			}
		}
	}
}

func TestGETOnly_AllowsGET(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")
	probe := health.New(injector,
		health.WithGETOnly(),
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	for path, handler := range map[string]http.HandlerFunc{
		"/healthz":  probe.LivenessHandler(),
		"/readyz":   probe.ReadinessHandler(),
		"/startupz": probe.StartupHandler(),
	} {
		w := doRequest(t, handler, path)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s with GETOnly: want 200, got %d", path, w.Code)
		}
	}
}

func TestGETOnly_405BodyContainsMessage(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector,
		health.WithGETOnly(),
		health.WithRefreshInterval(0),
	)

	w := httptest.NewRecorder()

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	probe.LivenessHandler()(w, r)

	if !strings.Contains(w.Body.String(), "health probes only accept GET") {
		t.Errorf("405 body should contain actionable message, got %q", w.Body.String())
	}
}

func TestDefault_AllowsNonGETWithoutGuard(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	w := httptest.NewRecorder()

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	probe.LivenessHandler()(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("POST /healthz without GETOnly: want 200 (no guard), got %d", w.Code)
	}
}

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

// --- Benchmarks ---.

func BenchmarkLivenessHandler(b *testing.B) {
	injector := do.New()
	probe := health.New(injector, health.WithVersion("1.0.0"))
	handler := probe.LivenessHandler()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler(w, r)
	}
}

func BenchmarkReadinessHandler_CacheHit(b *testing.B) {
	injector := do.New()
	provideHealthy(injector, "db")
	do.MustInvokeNamed[*healthyService](injector, "db")

	probe := health.New(injector, health.WithCriticalServices("db"))

	if err := probe.Start(context.Background()); err != nil {
		b.Fatal(err)
	}
	defer probe.Shutdown()

	handler := probe.ReadinessHandler()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler(w, r)
	}
}

func BenchmarkReadinessHandler_LiveEval(b *testing.B) {
	injector := do.New()
	provideHealthy(injector, "db")
	do.MustInvokeNamed[*healthyService](injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)
	handler := probe.ReadinessHandler()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler(w, r)
	}
}

func BenchmarkEvaluate(b *testing.B) {
	injector := do.New()
	provideHealthy(injector, "db")
	provideHealthy(injector, "cache")
	do.MustInvokeNamed[*healthyService](injector, "db")
	do.MustInvokeNamed[*healthyService](injector, "cache")

	probe := health.New(injector, health.WithCriticalServices("db", "cache"))

	ctx := context.Background()

	b.ResetTimer()

	for b.Loop() {
		_ = probe.Evaluate(ctx)
	}
}

func BenchmarkStartupHandler_Unlatched(b *testing.B) {
	injector := do.New()
	provideHealthy(injector, "db")
	do.MustInvokeNamed[*healthyService](injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	handler := probe.StartupHandler()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/startupz", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler(w, r)
	}
}

func BenchmarkReadinessHandler_RecorderPath(b *testing.B) {
	recorder := &mockRecorder{result: map[string]error{"db": nil}}

	probe := health.New(do.New(),
		health.WithHealthRecorder(recorder),
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	handler := probe.ReadinessHandler()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler(w, r)
	}
}
