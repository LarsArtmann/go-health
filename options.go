package health

import (
	"net/http"
	"time"
)

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
	evalHook        func(Response)
	liveThrottle    time.Duration
	shutdownGrace   time.Duration
	nowFunc         func() time.Time
	instanceID      InstanceID
	allowedMethods  map[string]struct{}
	recorderDropped bool
}

// Option configures a [Probe]. Use the With* functions to create options.
type Option func(*config)

// WithVersion sets the application version included in health responses.
func WithVersion(v string) Option {
	return func(c *config) { c.version = v }
}

// WithInstanceID sets a replica identifier included in health responses.
// Use it when several instances serve behind one load balancer and logs or
// dashboards must attribute a response to the pod that produced it. Wrap a
// raw string with [NewInstanceID]; the zero [InstanceID] leaves instance_id
// unset (omitted from JSON).
func WithInstanceID(id InstanceID) Option {
	return func(c *config) { c.instanceID = id }
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
// receives a copy of the response with its own Checks map, so it can neither
// corrupt the served response nor rely on aliasing it. A hook that panics is
// recovered: the evaluation proceeds and the response carries a synthetic
// non-critical "evaluation-hook" check (warn), wrapping
// [ErrPanicDuringEvaluationHook]. See docs/panic-recovery-design.md.
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
// then stops the loop. Shutdown blocks for the grace window whenever the
// probe's lifecycle started successfully (cached or live mode). Zero
// (default) stops the loop immediately; [Probe.MarkShuttingDown] remains the
// manual two-phase path.
func WithShutdownGracePeriod(d time.Duration) Option {
	return func(c *config) { c.shutdownGrace = d }
}

// WithNowFunc overrides the clock used for uptime and evaluation timestamps.
// Defaults to time.Now. Useful for deterministic tests; production code
// should leave it unset.
func WithNowFunc(fn func() time.Time) Option {
	return func(c *config) { c.nowFunc = fn }
}

// WithAllowedMethods wraps all handlers with method enforcement: the listed
// methods are accepted (plus GET, always), anything else gets 405 with an
// Allow header. Use it when infrastructure probes with HEAD or OPTIONS must
// pass. Calling it with no arguments seeds the set with GET alone.
func WithAllowedMethods(methods ...string) Option {
	return func(cfg *config) {
		if cfg.allowedMethods == nil {
			cfg.allowedMethods = map[string]struct{}{http.MethodGet: {}}
		}

		for _, m := range methods {
			cfg.allowedMethods[m] = struct{}{}
		}
	}
}

// WithHealthRecorder wires a [HealthRecorder] so that every health-check batch
// is observable by an external system. When nil (the default), checks run
// against the raw injector without any recording layer.
//
// Pass an [auditlog.Plugin] directly when you want audit-log integration:
//
//	probe := health.New(injector, health.WithHealthRecorder(plugin))
//
// The option only applies to the injector path ([New]). The function-path
// constructors ([NewWithHealthCheck], [NewWithDetailedCheck], [NewChecks])
// own batch execution themselves, so they cannot run a recorder: passing it
// there makes [Probe.Validate] (and therefore [Probe.Start]) fail with
// [ErrRecorderNotApplicable] instead of silently capturing nothing. Use
// [ChainRecorders] to observe one batch through several recorders.
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
