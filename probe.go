package health

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/samber/do/v2"
)

// HealthRecorder wraps health-check execution so that external systems (e.g.
// an audit-log plugin) can observe every check batch. Any type with a method
// matching this signature can be wired via [WithHealthRecorder].
//
// [github.com/larsartmann/samber-do-auditlog].Plugin satisfies this interface
// implicitly — pass it directly when you want audit-log integration.
type HealthRecorder interface {
	RecordHealthCheckWithContext(ctx context.Context, injector do.Injector) map[string]error
}

const (
	// defaultTimeout is the per-request deadline for health-check batches.
	defaultTimeout = 5 * time.Second
	// defaultRefreshInterval is how often the background loop refreshes
	// the cached readiness response. Set to zero via WithRefreshInterval(0)
	// to evaluate live on every request instead.
	defaultRefreshInterval = 1 * time.Second
	// uptimeResolution is the granularity of the human-readable uptime string.
	uptimeResolution = time.Second
)

// healthCheckFunc runs a single health-check batch and returns the per-service
// results. It is resolved once at construction time (see [New]) so the Probe
// holds the resolved capability rather than the injector container itself.
type healthCheckFunc func(ctx context.Context) map[string]error

// Probe orchestrates health checks against a samber/do v2 injector and exposes
// three distinct HTTP endpoints: liveness, readiness, and startup.
//
// The health-check capability is resolved once at construction time (see
// [New]) so the Probe holds a resolved function value, never the injector
// itself. Probe classifies registered services into critical and non-critical:
// only critical service failures cause readiness to return 503; non-critical
// failures are surfaced as individual check entries but do not affect the
// HTTP status code.
//
// Probe is safe for concurrent use by multiple goroutines.
type Probe struct {
	healthCheck healthCheckFunc
	critical    map[string]struct{}

	shuttingDown  atomic.Bool
	startupPassed atomic.Bool

	bootTime time.Time
	version  string
	getOnly  bool

	latest          atomic.Pointer[Response]
	refreshInterval time.Duration
	timeout         time.Duration

	evalHook      func(Response)
	liveThrottle  time.Duration
	shutdownGrace time.Duration
	lastEvalNano  atomic.Int64

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// config holds construction-only configuration. It is populated by [Option]
// functions and consumed by [New]; the resulting [Probe] never carries a
// reference to it.
type config struct {
	version         string
	critical        map[string]struct{}
	recorder        HealthRecorder
	bootTime        time.Time
	refreshInterval time.Duration
	timeout         time.Duration
	getOnly         bool
	evalHook        func(Response)
	liveThrottle    time.Duration
	shutdownGrace   time.Duration
}

// Option configures a [Probe]. Use the With* functions to create options.
type Option func(*config)

// WithVersion sets the application version included in health responses.
func WithVersion(v string) Option {
	return func(c *config) { c.version = v }
}

// WithCriticalServices marks the named services as critical: if any of them
// fails its health check, readiness returns 503. Services not listed here are
// non-critical; their failures appear in the response body but do not change
// the HTTP status code.
func WithCriticalServices(names ...string) Option {
	return func(c *config) {
		for _, name := range names {
			c.critical[name] = struct{}{}
		}
	}
}

// WithEvaluationHook registers a callback invoked synchronously after every
// [Probe.Evaluate] with the fully classified response. Use it to feed metrics
// or alerting without polling. The hook must be fast and must not block: it
// runs on the evaluation path (background loop and live requests). The hook
// receives the response by value and must not retain or mutate its Checks map.
func WithEvaluationHook(fn func(Response)) Option {
	return func(c *config) { c.evalHook = fn }
}

// WithLiveThrottle caps how often live (cache-miss) evaluations may run:
// within the window after an evaluation, readiness and combined handlers
// serve that stored result instead of re-running the batch. Without it,
// live mode runs one full batch per request — a DoS amplifier when the
// health endpoint is exposed. It has no effect when a background cache is
// active ([WithRefreshInterval] > 0 with [Probe.Start] called).
func WithLiveThrottle(d time.Duration) Option {
	return func(c *config) { c.liveThrottle = d }
}

// WithShutdownGracePeriod makes [Probe.Shutdown] two-phase automatically:
// after marking the probe as draining (readiness 503) it keeps the refresh
// loop running for d so dashboards and load balancers observe fresh 503s,
// then stops the loop. Shutdown blocks for the grace window. Zero (default)
// stops the loop immediately; [Probe.MarkShuttingDown] remains the manual
// two-phase path.
func WithShutdownGracePeriod(d time.Duration) Option {
	return func(c *config) { c.shutdownGrace = d }
}

// WithHealthRecorder wires a [HealthRecorder] so that every health-check batch
// is observable by an external system. When nil (the default), checks run
// against the raw injector without any recording layer.
//
// Pass an [auditlog.Plugin] directly when you want audit-log integration:
//
//	probe := health.New(injector, health.WithHealthRecorder(plugin))
func WithHealthRecorder(r HealthRecorder) Option {
	return func(c *config) { c.recorder = r }
}

// WithRefreshInterval sets the background cache refresh cadence. When greater
// than zero, [Probe.Start] launches a goroutine that re-evaluates health checks
// on this interval and readiness handlers serve the cached result. When zero,
// readiness handlers evaluate live on every request (no background goroutine).
//
// Use caching (the default) when kubelet or load-balancer polling could
// overwhelm downstream dependencies. Use live evaluation for low-traffic or
// development scenarios.
func WithRefreshInterval(d time.Duration) Option {
	return func(c *config) { c.refreshInterval = d }
}

// WithTimeout sets the batch-level context deadline shared across ALL services
// in a single health-check evaluation. All checks run concurrently against the
// same deadline — a slow dependency silently steals time from every other check.
//
// For per-service isolation, configure samber/do's native option at injector
// creation time:
//
//	injector := do.NewWithOpts(do.WithHealthCheckTimeout(2 * time.Second))
//
// This library does not override that setting; it only controls the outer
// batch deadline.
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// WithBootTime overrides the boot timestamp used to compute uptime. Defaults
// to the time [New] was called.
func WithBootTime(t time.Time) Option {
	return func(c *config) { c.bootTime = t }
}

// WithGETOnly wraps all handlers so they reject non-GET requests with 405
// Method Not Allowed. Kubernetes probes always use GET; enabling this surfaces
// misconfigurations (e.g. a load balancer sending HEAD or POST) early.
func WithGETOnly() Option {
	return func(c *config) { c.getOnly = true }
}

// guard wraps a handler with GET-only enforcement when WithGETOnly is active.
func (p *Probe) guard(handler http.HandlerFunc) http.HandlerFunc {
	if !p.getOnly {
		return handler
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "health probes only accept GET", http.StatusMethodNotAllowed)

			return
		}

		handler(w, r)
	}
}

// New creates a [Probe] wired to the given injector.
//
// The injector must be the root container created via [do.NewWithOpts]. The
// health-check capability (and, when configured, the [HealthRecorder]) is
// resolved here at construction time, so the returned Probe holds only the
// resolved function — never the container itself.
func New(injector do.Injector, opts ...Option) *Probe {
	cfg := buildConfig(opts)

	return assemble(resolveHealthCheck(cfg.recorder, injector), cfg)
}

// buildConfig applies options onto the default construction-time config.
func buildConfig(opts []Option) config {
	cfg := config{
		critical:        make(map[string]struct{}),
		bootTime:        time.Now(),
		timeout:         defaultTimeout,
		refreshInterval: defaultRefreshInterval,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

// assemble wires a resolved health-check capability and configuration into
// a Probe. Both constructors funnel through here.
func assemble(healthCheck healthCheckFunc, cfg config) *Probe {
	return &Probe{
		healthCheck:     healthCheck,
		critical:        cfg.critical,
		bootTime:        cfg.bootTime,
		version:         cfg.version,
		getOnly:         cfg.getOnly,
		refreshInterval: cfg.refreshInterval,
		timeout:         cfg.timeout,
		evalHook:        cfg.evalHook,
		liveThrottle:    cfg.liveThrottle,
		shutdownGrace:   cfg.shutdownGrace,
	}
}

// resolveHealthCheck captures the health-check capability at construction
// time. When a recorder is configured, checks delegate through it; otherwise
// they call the injector's HealthCheckWithContext directly.
func resolveHealthCheck(recorder HealthRecorder, injector do.Injector) healthCheckFunc {
	if recorder != nil {
		return func(ctx context.Context) map[string]error {
			return recorder.RecordHealthCheckWithContext(ctx, injector)
		}
	}

	return injector.HealthCheckWithContext
}

// ErrInvalidTimeout is returned by [Probe.Validate] when the configured timeout
// is zero or negative.
var ErrInvalidTimeout = errors.New("health: timeout must be positive")

// ErrInvalidRefreshInterval is returned by [Probe.Validate] when the configured
// refresh interval is negative.
var ErrInvalidRefreshInterval = errors.New("health: refresh interval must not be negative")

// ErrPanicDuringHealthCheck is wrapped into the synthetic "health-check" error
// when the health-check batch panics and the panic is recovered. Match with
// errors.Is to distinguish a recovered panic from an ordinary service failure:
// a recovered panic rolls the response up to fail, never to warn.
var ErrPanicDuringHealthCheck = errors.New("health: panic during health check")

// Validate checks that the Probe configuration is internally consistent.
// Returns nil when the configuration is safe to use.
//
// This catches the two common mistakes that cause runtime problems:
//
//   - Timeout <= 0 creates an already-expired context, so every health check
//     fails immediately with "context deadline exceeded".
//   - RefreshInterval < 0 is treated the same as 0 (live evaluation) by Start,
//     but callers likely intended a positive interval.
func (p *Probe) Validate() error {
	if p.timeout <= 0 {
		return fmt.Errorf("%w: got %s (configure via WithTimeout)", ErrInvalidTimeout, p.timeout)
	}

	if p.refreshInterval < 0 {
		return fmt.Errorf(
			"%w: got %s (use WithRefreshInterval(0) for live mode or a positive duration)",
			ErrInvalidRefreshInterval,
			p.refreshInterval,
		)
	}

	return nil
}

// Start validates the Probe configuration and, if valid, launches the
// background cache refresh loop (when RefreshInterval > 0) and performs an
// immediate evaluation so the cache is populated before the first request
// arrives. Calling Start more than once is a no-op.
//
// Returns [ErrInvalidTimeout] or [ErrInvalidRefreshInterval] if the
// configuration is unusable. Call [Probe.Validate] separately to check
// configuration before starting.
//
// The provided ctx controls the lifetime of the background goroutine. Call
// [Probe.Shutdown] to stop the loop and mark the probe as shutting down.
func (p *Probe) Start(ctx context.Context) error {
	if err := p.Validate(); err != nil {
		return err
	}

	p.mu.Lock()

	if p.cancel != nil {
		p.mu.Unlock()

		return nil
	}

	runCtx := ctx

	if p.refreshInterval > 0 {
		runCtx, p.cancel = context.WithCancel(ctx)

		// Register with the WaitGroup in the same critical section that
		// publishes cancel: Shutdown swaps cancel under the lock before it
		// waits, so a concurrent Start can never slip an Add past a Wait.
		p.wg.Add(1)
	}

	p.mu.Unlock()

	p.refreshCache(ctx)

	if p.refreshInterval > 0 {
		go p.refreshLoop(runCtx)
	}

	return nil
}

// refreshLoop runs the periodic cache refresh until the start context is cancelled.
func (p *Probe) refreshLoop(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.refreshCache(ctx)
		}
	}
}

// refreshCache evaluates health checks with a timeout-bounded context and
// stores the result atomically for cached handlers.
func (p *Probe) refreshCache(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	resp := p.Evaluate(ctx)
	p.latest.Store(&resp)
}

// Shutdown marks the probe as shutting down and stops the background refresh
// loop if one is running. After Shutdown:
//
//   - Liveness continues to return 200 (the process is still alive).
//   - Readiness returns 503 so load balancers drain traffic.
//   - Startup returns its latched value (200 if it had previously passed).
func (p *Probe) Shutdown() {
	p.shuttingDown.Store(true)

	// Add (in Start) and Wait are serialized under p.mu on purpose: the
	// WaitGroup contract forbids an Add from a zero counter running
	// concurrently with a Wait, which concurrent Start/Shutdown callers
	// would otherwise trigger ("WaitGroup is reused before previous Wait
	// has returned"). refreshLoop does not take p.mu, so waiting under the
	// lock cannot deadlock the loop we are waiting for.
	p.mu.Lock()
	cancel := p.cancel
	p.cancel = nil

	if cancel != nil {
		cancel()
		p.wg.Wait()
	}

	p.mu.Unlock()
}

// MarkShuttingDown flips the shutdown flag without stopping the background
// loop. Use this for a two-phase graceful shutdown: mark first so load
// balancers start draining, then call [Probe.Shutdown] after a grace period
// to stop the refresh loop.
func (p *Probe) MarkShuttingDown() {
	p.shuttingDown.Store(true)
}

// Evaluate runs a full health-check batch against the injector and returns
// the aggregate response. This is the core evaluation logic used by the
// readiness and startup handlers, exposed publicly for testing and custom
// handler scenarios.
//
// The context should carry a deadline; [Probe.Start] applies [Probe.timeout]
// automatically. When a [HealthRecorder] is configured, checks are delegated
// to it instead of the raw injector.
func (p *Probe) Evaluate(ctx context.Context) Response {
	start := time.Now()

	results := p.runHealthChecks(ctx)

	resp := Response{
		Version:        p.version,
		Uptime:         time.Since(p.bootTime).Round(uptimeResolution).String(),
		ShuttingDown:   p.shuttingDown.Load(),
		Checks:         p.buildChecks(results),
		TotalLatencyMs: time.Since(start).Milliseconds(),
	}

	resp.Status = p.classify(results, resp.ShuttingDown)

	return resp
}

// runHealthChecks invokes the health-check function resolved at construction
// time. The recorder-versus-injector decision was already made in [New], so
// this is a single function call.
//
// A panic from the health-check function (e.g. a misbehaving recorder or a
// service with a nil-pointer dereference) is recovered and reported as a
// synthetic "health-check" error wrapping [ErrPanicDuringHealthCheck], so it
// never crashes the process or the HTTP handler. classify maps a recovered
// panic to fail: the interrupted batch leaves every not-yet-checked service
// unverified, and a panic that hit a critical service must not degrade to a
// 200 warn. See docs/panic-recovery-design.md for the full rationale.
func (p *Probe) runHealthChecks(ctx context.Context) map[string]error {
	return recoverHealthChecks(func() map[string]error { return p.healthCheck(ctx) })
}

// recoverHealthChecks runs fn and converts a panic into a synthetic result
// map. The inner closure confines the deferred recover to one frame whose
// result is captured in an outer variable, so no named return is needed.
func recoverHealthChecks(fn func() map[string]error) map[string]error {
	var results map[string]error

	func() {
		defer func() {
			if r := recover(); r != nil {
				results = map[string]error{
					"health-check": fmt.Errorf("%w: %v", ErrPanicDuringHealthCheck, r),
				}
			}
		}()

		results = fn()
	}()

	return results
}

// classify computes the roll-up status from health-check results:
//
//   - StatusFail when shutting down, any critical service failed, or the
//     batch panicked (recovered panic: results are untrustworthy — see
//     docs/panic-recovery-design.md).
//   - StatusWarn when no critical service failed but at least one
//     non-critical service failed (degraded but still serving traffic).
//   - StatusPass when every checked service is healthy.
func (p *Probe) classify(results map[string]error, shuttingDown bool) Status {
	if shuttingDown {
		return StatusFail
	}

	hasWarning := false

	for name, err := range results {
		if err == nil {
			continue
		}

		if errors.Is(err, ErrPanicDuringHealthCheck) {
			return StatusFail
		}

		if _, critical := p.critical[name]; critical {
			return StatusFail
		}

		hasWarning = true
	}

	if hasWarning {
		return StatusWarn
	}

	return StatusPass
}

// StartupComplete returns true once all critical services have passed their
// health checks at least once during a startup evaluation. After this returns
// true it always returns true (the latch is one-way).
func (p *Probe) StartupComplete() bool {
	return p.startupPassed.Load()
}

// CachedResponse returns the last background-refreshed health Response. When
// the background cache is active ([Probe.Start] called with a non-zero
// [WithRefreshInterval]), this reads the atomic p.latest pointer — lock-free,
// zero dependency calls. When no cache exists (live mode or before the first
// refresh), it returns a zero-value Response with StatusPass.
//
// The live shuttingDown flag is overlaid on the cached value so a stale cached
// response (evaluated before [Probe.Shutdown] was called) still reflects the
// current shutdown state.
func (p *Probe) CachedResponse() Response {
	if cached := p.latest.Load(); cached != nil {
		resp := *cached

		if p.shuttingDown.Load() {
			resp.ShuttingDown = true
			resp.Status = StatusFail
		}

		return resp
	}

	resp := Response{Status: StatusPass, Checks: map[string]Check{}}

	if p.shuttingDown.Load() {
		resp.ShuttingDown = true
		resp.Status = StatusFail
	}

	return resp
}

// RefreshInterval returns the configured background cache refresh interval.
// Returns zero when the probe is in live evaluation mode.
func (p *Probe) RefreshInterval() time.Duration {
	return p.refreshInterval
}

// evaluateStartup checks whether all critical services are present and healthy
// in the results map. Returns true if the startup latch should be set.
func (p *Probe) evaluateStartup(results map[string]error) bool {
	if len(p.critical) == 0 {
		return true
	}

	for name := range p.critical {
		err, found := results[name]

		if !found || err != nil {
			return false
		}
	}

	return true
}

// buildChecks converts the raw map[string]error from samber/do into typed
// Check entries. A nil error means the service passed; a non-nil error
// populates the Error field. Failures on critical services are marked
// StatusFail; failures on non-critical services are marked StatusWarn to
// distinguish "degraded but functional" from "take this pod out of rotation".
func (p *Probe) buildChecks(results map[string]error) map[string]Check {
	checks := make(map[string]Check, len(results))

	for name, err := range results {
		check := Check{Status: StatusPass}

		if err != nil {
			if _, critical := p.critical[name]; critical {
				check.Status = StatusFail
			} else {
				check.Status = StatusWarn
			}

			if errors.Is(err, ErrPanicDuringHealthCheck) {
				check.Status = StatusFail
			}

			check.Error = err.Error()
		}

		checks[name] = check
	}

	return checks
}
