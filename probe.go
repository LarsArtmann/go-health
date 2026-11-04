package health

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
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

// DetailedHealthRecorder is an optional interface a [HealthRecorder] can
// implement to report per-check execution metadata. When the configured
// recorder implements it, the probe uses the detailed batch and each
// [Check].Duration reflects the recorder's measurement; otherwise Duration
// stays zero (unknown) and the recorder is used via its plain method.
// Implementations must be safe for concurrent use, like HealthRecorder itself.
type DetailedHealthRecorder interface {
	RecordDetailedHealthCheckWithContext(
		ctx context.Context,
		injector do.Injector,
	) map[string]CheckDetail
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
// raw reports (outcome plus executor-known metadata such as Duration). It is
// resolved once at construction time (see [New]) so the Probe holds the
// resolved capability rather than the injector container itself.
type healthCheckFunc func(ctx context.Context) map[string]CheckDetail

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
	rollups     classifier
	transitions transitionTracker

	shuttingDown  atomic.Bool
	startupPassed atomic.Bool

	// lifecycleStarted records that a Start call completed successfully, in
	// any mode (cached or live). Shutdown consults it to decide whether the
	// WithShutdownGracePeriod window is honored: a probe that never started
	// has no lifecycle to drain, so the window would be dead time.
	lifecycleStarted atomic.Bool

	bootTime time.Time
	version  string

	latest          atomic.Pointer[Response]
	refreshInterval time.Duration
	timeout         time.Duration

	evalHook       func(Response)
	liveThrottle   time.Duration
	shutdownGrace  time.Duration
	nowFunc        func() time.Time
	instanceID     InstanceID
	allowedMethods map[string]struct{}

	// recorderDropped records that a WithHealthRecorder option was passed
	// to a function-path constructor, where the explicit function owns
	// batch execution and the recorder cannot run. Validate and Start
	// reject that mis-wiring instead of letting it silently capture
	// nothing.
	recorderDropped bool

	// throttleMu serializes throttled live evaluations so a request flood
	// produces one batch per window, not one per request.
	throttleMu sync.Mutex

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// guard wraps a handler with method enforcement when a method set is
// configured via [WithAllowedMethods]. The accepted set is
// GET plus any explicitly allowed methods; the Allow response header names
// the full set.
func (p *Probe) guard(handler http.HandlerFunc) http.HandlerFunc {
	allowed := p.allowedMethods

	if allowed == nil {
		return handler
	}

	allowList := make([]string, 0, len(allowed))
	for m := range allowed {
		allowList = append(allowList, m)
	}

	sort.Strings(allowList)

	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := allowed[r.Method]; !ok {
			w.Header().Set("Allow", strings.Join(allowList, ", "))
			http.Error(w, "health probes only accept GET", http.StatusMethodNotAllowed)

			return
		}

		handler(w, r)
	}
}

// now returns the configured clock. Tests inject a fixed one via WithNowFunc.
func (p *Probe) now() time.Time {
	if p.nowFunc != nil {
		return p.nowFunc()
	}

	return time.Now()
}

// uptime renders the rounded time since boot.
func (p *Probe) uptime() string {
	return p.now().Sub(p.bootTime).Round(uptimeResolution).String()
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

// buildStandaloneConfig resolves options for a probe whose health-check
// batches are produced by an explicit function ([NewWithHealthCheck],
// [NewWithDetailedCheck], [NewChecks]). Such a function owns batch
// execution, so [WithHealthRecorder] has no effect: it is cleared here,
// once, instead of at every standalone constructor. The drop is recorded so
// [Probe.Validate] (and therefore [Probe.Start]) can reject the mis-wiring
// loudly instead of silently capturing nothing.
func buildStandaloneConfig(opts []Option) config {
	cfg := buildConfig(opts)
	cfg.recorderDropped = cfg.recorder != nil
	cfg.recorder = nil

	return cfg
}

// assemble wires a resolved health-check capability and configuration into
// a Probe. Both constructors funnel through here.
func assemble(healthCheck healthCheckFunc, cfg config) *Probe {
	return &Probe{
		healthCheck:     healthCheck,
		rollups:         classifier{critical: cfg.critical},
		bootTime:        cfg.bootTime,
		version:         cfg.version,
		refreshInterval: cfg.refreshInterval,
		timeout:         cfg.timeout,
		evalHook:        cfg.evalHook,
		liveThrottle:    cfg.liveThrottle,
		shutdownGrace:   cfg.shutdownGrace,
		nowFunc:         cfg.nowFunc,
		instanceID:      cfg.instanceID,
		allowedMethods:  cfg.allowedMethods,
		recorderDropped: cfg.recorderDropped,
	}
}

// resolveHealthCheck captures the health-check capability at construction
// time. A recorder that also satisfies [DetailedHealthRecorder] delegates
// through its metadata-rich method so per-check durations survive; a plain
// recorder delegates through its own method; without a recorder, checks call
// the injector's HealthCheckWithContext directly. Plain map[string]error
// results are adapted to [CheckDetail] with zero Duration.
func resolveHealthCheck(recorder HealthRecorder, injector do.Injector) healthCheckFunc {
	switch r := recorder.(type) {
	case nil:
		return adaptPlainChecks(injector.HealthCheckWithContext)
	case DetailedHealthRecorder:
		return func(ctx context.Context) map[string]CheckDetail {
			return r.RecordDetailedHealthCheckWithContext(ctx, injector)
		}
	default:
		return adaptPlainChecks(func(ctx context.Context) map[string]error {
			return r.RecordHealthCheckWithContext(ctx, injector)
		})
	}
}

// adaptPlainChecks lifts a map[string]error batch into the CheckDetail seam.
// Zero Duration: a plain executor cannot know per-check timing.
func adaptPlainChecks(fn func(ctx context.Context) map[string]error) healthCheckFunc {
	return func(ctx context.Context) map[string]CheckDetail {
		return detailOf(fn(ctx))
	}
}

// detailOf converts plain per-service errors into [CheckDetail] reports.
func detailOf(results map[string]error) map[string]CheckDetail {
	details := make(map[string]CheckDetail, len(results))

	for name, err := range results {
		details[name] = CheckDetail{Err: err}
	}

	return details
}

// errorsOf projects a detail batch back onto the plain error view the
// classifier consumes.
func errorsOf(details map[string]CheckDetail) map[string]error {
	results := make(map[string]error, len(details))

	for name, detail := range details {
		results[name] = detail.Err
	}

	return results
}

// Validate checks that the Probe configuration is internally consistent.
// Returns nil when the configuration is safe to use.
//
// This catches the common mistakes that cause runtime problems:
//
//   - Timeout <= 0 creates an already-expired context, so every health check
//     fails immediately with "context deadline exceeded".
//   - RefreshInterval < 0 is treated the same as 0 (live evaluation) by Start,
//     but callers likely intended a positive interval.
//   - A recorder configured on a function-path constructor can never run
//     ([ErrRecorderNotApplicable]); Start fails instead of silently dropping
//     the observer.
func (p *Probe) Validate() error {
	if p.recorderDropped {
		return ErrRecorderNotApplicable
	}

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

// StartupComplete returns true once all critical services have passed their
// health checks at least once during a startup evaluation. After this returns
// true it always returns true (the latch is one-way).
func (p *Probe) StartupComplete() bool {
	return p.startupPassed.Load()
}
