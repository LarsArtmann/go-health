package health_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	do "github.com/samber/do/v2"
)

// --- P25: Status / Alive / Ready accessors ---.

func TestAccessors_StatusAliveReady(t *testing.T) {
	t.Parallel()

	injector := do.New()

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	t.Cleanup(func() { injector.Shutdown() })

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	if !probe.Alive() {
		t.Error("Alive: want true before shutdown")
	}

	if probe.Status() != health.StatusPass {
		t.Errorf(
			"Status before any evaluation: want pass (cached fallback), got %s",
			probe.Status(),
		)
	}

	if !probe.Ready() {
		t.Error("Ready: want true while pass")
	}

	probe.Shutdown()

	if probe.Alive() {
		t.Error("Alive: want false after Shutdown")
	}

	if probe.Ready() {
		t.Error("Ready: want false after Shutdown")
	}

	if probe.Status() != health.StatusFail {
		t.Errorf("Status after Shutdown: want fail, got %s", probe.Status())
	}
}

// --- P26: AwaitReady + Healthz ---.

func TestAwaitReady_ReturnsImmediatelyWhenReady(t *testing.T) {
	t.Parallel()

	injector := do.New()

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	t.Cleanup(func() { injector.Shutdown() })

	probe := health.New(injector)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	if err := probe.AwaitReady(ctx); err != nil {
		t.Errorf("AwaitReady on healthy probe: %v", err)
	}
}

func TestAwaitReady_TimesOutWhenFailing(t *testing.T) {
	t.Parallel()

	injector := do.New()

	provideUnhealthy(injector, "db", "down")
	invoke[*unhealthyService](t, injector, "db")

	t.Cleanup(func() { injector.Shutdown() })

	probe := health.New(injector, health.WithCriticalServices("db"))
	probe.Shutdown() // one-way fail: AwaitReady must never succeed

	ctx, cancel := context.WithTimeout(t.Context(), 120*time.Millisecond)
	defer cancel()

	if err := probe.AwaitReady(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("AwaitReady on failing probe: want DeadlineExceeded, got %v", err)
	}
}

func TestHealthz_CombinesStartupAndReadiness(t *testing.T) {
	t.Parallel()

	injector := do.New()

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	t.Cleanup(func() { injector.Shutdown() })

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	handler := probe.Healthz()
	routes := health.DefaultRoutes()

	// Booting: latch unset → 503 with a startup check entry.
	rec := doRequest(t, handler, routes.Readiness)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("booting: want 503, got %d", rec.Code)
	}

	if body := rec.Body.String(); !strings.Contains(body, "startup latch not set") {
		t.Errorf("booting body missing startup entry: %s", body)
	}

	// After one successful startup-handler batch the latch is set to 200.
	// (Evaluate alone does not latch; only the startup path does.)
	rec = doRequest(t, probe.StartupHandler(), routes.Startup)
	if rec.Code != http.StatusOK {
		t.Errorf("startup after first batch: want 200, got %d", rec.Code)
	}

	rec = doRequest(t, handler, routes.Readiness)
	if rec.Code != http.StatusOK {
		t.Errorf("after latch: want 200, got %d", rec.Code)
	}

	// Shutdown → 503 again.
	probe.Shutdown()

	rec = doRequest(t, handler, routes.Readiness)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("after shutdown: want 503, got %d", rec.Code)
	}
}

// --- P27: NewWithHealthCheck ---.

func TestNewWithHealthCheck_FullSurface(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64

	fn := func(context.Context) map[string]error {
		calls.Add(1)

		return map[string]error{"custom": nil}
	}

	probe := health.NewWithHealthCheck(fn,
		health.WithCriticalServices("custom"),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusPass {
		t.Errorf("status: want pass, got %s", resp.Status)
	}

	if calls.Load() != 1 {
		t.Errorf("health-check func calls: want 1, got %d", calls.Load())
	}

	// Validate-on-Start works without any injector.
	if err := probe.Start(t.Context()); err != nil {
		t.Errorf("Start: %v", err)
	}

	probe.Shutdown()
}

func TestNewWithHealthCheck_PanicRecovered(t *testing.T) {
	t.Parallel()

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		panic("custom fn exploded")
	})

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusFail {
		t.Errorf("status: want fail (fail-closed panic), got %s", resp.Status)
	}
}

// --- P32: do.HealthcheckerWithContext + shutdown adapter ---.

func TestProbe_HealthCheck_Conformance(t *testing.T) {
	t.Parallel()

	injector := do.New()

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	t.Cleanup(func() { injector.Shutdown() })

	probe := health.New(injector, health.WithCriticalServices("db"))

	if err := probe.HealthCheck(t.Context()); err != nil {
		t.Errorf("HealthCheck while pass: %v", err)
	}

	probe.Shutdown()

	err := probe.HealthCheck(t.Context())
	if !errors.Is(err, health.ErrProbeUnhealthy) {
		t.Errorf("HealthCheck after Shutdown: want ErrProbeUnhealthy wrap, got %v", err)
	}
}

func TestProbe_AsShutdowner(t *testing.T) {
	t.Parallel()

	probe, _ := newStressProbe(t)

	shutdowner := probe.AsShutdowner()

	if err := shutdowner.Shutdown(); err != nil {
		t.Errorf("ProbeShutdowner.Shutdown: %v", err)
	}

	if probe.Alive() {
		t.Error("Alive after adapter Shutdown: want false")
	}
}

// --- P29: evaluation hook ---.

func TestWithEvaluationHook_InvokedPerEvaluation(t *testing.T) {
	t.Parallel()

	var (
		hookCalls atomic.Int64

		lastStatus atomic.Value
	)

	injector := do.New()

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	t.Cleanup(func() { injector.Shutdown() })

	probe := health.New(injector,
		health.WithEvaluationHook(func(resp health.Response) {
			hookCalls.Add(1)
			lastStatus.Store(resp.Status)
		}),
	)

	_ = probe.Evaluate(context.Background())
	_ = probe.Evaluate(context.Background())

	if calls := hookCalls.Load(); calls != 2 {
		t.Errorf("hook calls: want 2, got %d", calls)
	}

	if got := lastStatus.Load(); got != health.StatusPass {
		t.Errorf("last hook status: want pass, got %v", got)
	}
}

// --- P30: live throttle ---.

func TestWithLiveThrottle_CoalescesLiveEvaluations(t *testing.T) {
	t.Parallel()

	var batches atomic.Int64

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		batches.Add(1)

		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithLiveThrottle(50*time.Millisecond),
	)

	handler := probe.ReadinessHandler()
	routes := health.DefaultRoutes()

	var wg sync.WaitGroup

	for range 20 {
		wg.Go(func() {
			rec := doRequest(t, handler, routes.Readiness)
			if rec.Code != http.StatusOK {
				t.Errorf("status: want 200, got %d", rec.Code)
			}
		})
	}

	wg.Wait()

	if calls := batches.Load(); calls > 5 {
		t.Errorf(
			"20 requests within the throttle window ran %d batches; want coalescing (<=5)",
			calls,
		)
	}

	// After the window elapses a fresh batch runs.
	time.Sleep(60 * time.Millisecond)

	before := batches.Load()
	rec := doRequest(t, handler, routes.Readiness)

	if rec.Code != http.StatusOK {
		t.Errorf("post-window status: want 200, got %d", rec.Code)
	}

	if after := batches.Load(); after != before+1 {
		t.Errorf("post-window batches: want %d, got %d", before+1, after)
	}
}

// mutableClock is a controllable clock for determinism tests. Evaluate stamps
// Response.Timestamp from it and the live throttle compares freshness against
// it, so tests advance it explicitly instead of sleeping.
type mutableClock struct {
	nowNs atomic.Int64
}

func newMutableClock(t time.Time) *mutableClock {
	c := &mutableClock{}
	c.nowNs.Store(t.UnixNano())

	return c
}

func (c *mutableClock) Now() time.Time { return time.Unix(0, c.nowNs.Load()) }

func (c *mutableClock) Advance(d time.Duration) { c.nowNs.Add(int64(d)) }

// requestReadiness performs one GET against the readiness handler and
// returns the response body, failing the test on a non-200 status.
func requestReadiness(t *testing.T, handler http.HandlerFunc, path string) string {
	t.Helper()

	rec := doRequest(t, handler, path)
	if rec.Code != http.StatusOK {
		t.Fatalf("readiness request: want 200, got %d", rec.Code)
	}

	return rec.Body.String()
}

func TestWithLiveThrottle_FakeClockFreshnessDeterministic(t *testing.T) {
	t.Parallel()

	const window = time.Second

	var batches atomic.Int64

	epoch := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		batches.Add(1)

		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithRefreshInterval(0),
		health.WithLiveThrottle(window),
		health.WithBootTime(epoch),
		health.WithNowFunc(clock.Now),
	)

	handler := probe.ReadinessHandler()
	routes := health.DefaultRoutes()

	first := requestReadiness(t, handler, routes.Readiness)

	if got := batches.Load(); got != 1 {
		t.Fatalf("first request batches: want 1, got %d", got)
	}

	assertTimestampOf(t, first, epoch)

	// With the clock frozen, any number of requests must serve the stored
	// result byte-identically: one batch total, one response.
	assertFrozenWindow(t, handler, routes.Readiness, first, 25)

	if got := batches.Load(); got != 1 {
		t.Errorf("25 frozen-clock requests ran %d batches; want exactly 1", got)
	}

	// Crossing the freshness boundary (now - Timestamp >= window) must run
	// exactly one new batch; the new result is then frozen the same way.
	clock.Advance(window + time.Millisecond)

	second := requestReadiness(t, handler, routes.Readiness)

	if got := batches.Load(); got != 2 {
		t.Fatalf("post-window batches: want 2, got %d", got)
	}

	if second == first {
		t.Error("post-window body equals pre-window body; freshness marker did not advance")
	}

	assertFrozenWindow(t, handler, routes.Readiness, second, 10)

	if got := batches.Load(); got != 2 {
		t.Errorf("post-window re-frozen requests ran %d batches; want still 2", got)
	}
}

// assertTimestampOf verifies the response's timestamp equals the expected
// instant, proving Evaluate stamps from the injected clock.
func assertTimestampOf(t *testing.T, body string, want time.Time) {
	t.Helper()

	var decoded struct {
		Timestamp time.Time `json:"timestamp"`
	}

	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !decoded.Timestamp.Equal(want) {
		t.Errorf(
			"response timestamp not stamped from fake clock: want %v, got %v",
			want,
			decoded.Timestamp,
		)
	}
}

// assertFrozenWindow fires concurrent requests with the clock frozen and
// verifies every one serves wantBody byte-identically.
func assertFrozenWindow(
	t *testing.T,
	handler http.HandlerFunc,
	path, wantBody string,
	requests int,
) {
	t.Helper()

	var wg sync.WaitGroup

	for range requests {
		wg.Go(func() {
			if body := requestReadiness(t, handler, path); body != wantBody {
				t.Errorf(
					"frozen-clock body drift within window:\n want %s\n got  %s",
					wantBody,
					body,
				)
			}
		})
	}

	wg.Wait()
}

// --- Throttle × Start-cache interaction (README "Flood-safe live mode") ---.

// awaitLoopBatches polls until the background refresh loop has stored at
// least minBatches batches, then returns the count. Fails the test on timeout.
func awaitLoopBatches(t *testing.T, batches *atomic.Int64, minBatches int64) int64 {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for {
		if got := batches.Load(); got >= minBatches {
			return got
		}

		if time.Now().After(deadline) {
			t.Fatalf("refresh loop stored no cache entry after 2s (batches=%d)", batches.Load())
		}

		time.Sleep(5 * time.Millisecond)
	}
}

// TestWithLiveThrottle_StartCacheServesWithoutBatches backs the README
// paragraph on the WithLiveThrottle × Start interaction: once Start's
// background loop has populated the cache, requests are served from it and
// trigger zero evaluation batches. The loop is cancelled (and given longer
// than one tick to fully stop) before the request phase, so the batch count
// is exact rather than statistical.
func TestWithLiveThrottle_StartCacheServesWithoutBatches(t *testing.T) {
	t.Parallel()

	const (
		loopInterval = 50 * time.Millisecond
		window       = time.Second
	)

	var batches atomic.Int64

	epoch := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		batches.Add(1)

		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithRefreshInterval(loopInterval),
		health.WithLiveThrottle(window),
		health.WithBootTime(epoch),
		health.WithNowFunc(clock.Now),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}

	awaitLoopBatches(t, &batches, 1)

	cancel()
	time.Sleep(3 * loopInterval) // any in-flight tick has landed by now

	before := batches.Load()

	handler := probe.ReadinessHandler()
	routes := health.DefaultRoutes()

	assertFrozenWindow(
		t,
		handler,
		routes.Readiness,
		requestReadiness(t, handler, routes.Readiness),
		25,
	)

	if after := batches.Load(); after != before {
		t.Errorf(
			"requests against the loop-refreshed cache ran %d extra batches; want 0",
			after-before,
		)
	}
}

// TestWithLiveThrottle_StaleCacheTriggersExactlyOneEvaluation completes the
// Start-cache interaction contract: a cached result older than the throttle
// window does not get served — exactly one live evaluation runs, and the
// refreshed result is then served without further batches.
func TestWithLiveThrottle_StaleCacheTriggersExactlyOneEvaluation(t *testing.T) {
	t.Parallel()

	const (
		loopInterval = 50 * time.Millisecond
		window       = time.Second
	)

	var batches atomic.Int64

	epoch := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		batches.Add(1)

		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithRefreshInterval(loopInterval),
		health.WithLiveThrottle(window),
		health.WithBootTime(epoch),
		health.WithNowFunc(clock.Now),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}

	awaitLoopBatches(t, &batches, 1)

	cancel()
	time.Sleep(3 * loopInterval)

	clock.Advance(window + time.Millisecond) // cached.Timestamp is now stale

	handler := probe.ReadinessHandler()
	routes := health.DefaultRoutes()

	requestReadiness(t, handler, routes.Readiness)

	if got := batches.Load(); got != 2 {
		t.Fatalf("stale-cache request batches: want exactly 2 (1 loop + 1 live), got %d", got)
	}

	// The freshly evaluated result is inside the window again.
	assertFrozenWindow(
		t,
		handler,
		routes.Readiness,
		requestReadiness(t, handler, routes.Readiness),
		10,
	)

	if got := batches.Load(); got != 2 {
		t.Errorf("post-refresh requests ran %d total batches; want still 2", got)
	}
}

// --- P31: shutdown grace period ---.

func TestWithShutdownGracePeriod_BlocksButStops(t *testing.T) {
	t.Parallel()

	const grace = 120 * time.Millisecond

	// Apply the option by building a second probe with the same injector.
	injector := do.New()

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	t.Cleanup(func() { injector.Shutdown() })

	graceProbe := health.New(injector,
		health.WithRefreshInterval(5*time.Millisecond),
		health.WithShutdownGracePeriod(grace),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := graceProbe.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	done := make(chan struct{})

	go func() {
		graceProbe.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Shutdown with grace period returned before the window elapsed")
	case <-time.After(grace / 2):
		// expected: still draining
	}

	<-done

	if graceProbe.Alive() {
		t.Error("Alive after graceful Shutdown: want false")
	}
}
