package health

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ErrProbeUnhealthy is returned by [Probe.HealthCheck] when the probe's
// cached roll-up status is fail. Match with errors.Is.
var ErrProbeUnhealthy = errors.New("health: probe reports fail")

// HealthCheckFunc runs one health-check batch and returns the per-service
// results. It is the injector-free equivalent of the capability [New]
// resolves from a samber/do injector or [HealthRecorder]: pass one to
// [NewWithHealthCheck] to run a probe without any injector at all.
type HealthCheckFunc func(ctx context.Context) map[string]error

// NewWithHealthCheck creates a [Probe] whose health-check batches are
// produced by the given function, with no samber/do injector involved. Use it
// for injectors other than samber/do, for composed or external checks, or for
// tests that need full control over the batch.
//
// The function must be safe for concurrent use: batch evaluations can run
// from the background refresh loop and from live handler requests. A panic
// inside the function is recovered and reported as a fail-closed synthetic
// error, exactly like the recorder path.
//
// If the options include [WithHealthRecorder], it has no effect here: the
// explicit function already owns batch execution. All other options apply
// normally.
func NewWithHealthCheck(fn HealthCheckFunc, opts ...Option) *Probe {
	cfg := buildConfig(opts)
	cfg.recorder = nil

	return assemble(func(ctx context.Context) map[string]error { return fn(ctx) }, cfg)
}

// Status returns the cached roll-up status: pass, warn, or fail. It performs
// no dependency checks and is safe to call at high frequency (e.g. from
// dashboards or middleware). Before the first evaluation (probe never
// started, live mode) it reports pass.
func (p *Probe) Status() Status {
	return p.CachedResponse().Status
}

// Alive reports whether the probe considers this instance alive. It mirrors
// the liveness handler's semantics: true from construction until Shutdown
// marks the probe as draining. It never consults dependencies.
func (p *Probe) Alive() bool {
	return !p.shuttingDown.Load()
}

// Ready reports whether the instance can serve traffic according to the
// cached readiness view: not shutting down and roll-up status not fail.
// Equivalent to [Probe.Status] != fail. It performs no dependency checks.
func (p *Probe) Ready() bool {
	return p.Status() != StatusFail
}

// AwaitReady blocks until [Probe.Ready] reports true or ctx is done, polling
// the cached view every 50ms. It returns ctx.Err() on cancellation or
// timeout. AwaitReady never triggers dependency checks itself: with a
// background cache it observes refreshes; in live mode it observes the
// requests of others. AwaitReady after Shutdown never becomes ready —
// shutdown is one-way.
func (p *Probe) AwaitReady(ctx context.Context) error {
	const pollInterval = 50 * time.Millisecond

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		if p.Ready() {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("health: await ready: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// HealthCheck implements do.HealthcheckerWithContext, letting the probe
// itself be registered as a service in the very injector it monitors. The
// verdict comes from the cached roll-up view — no dependency calls, so it
// composes safely with samber/do's health-check fan-out. It wraps
// [ErrProbeUnhealthy] on fail (including shutdown).
func (p *Probe) HealthCheck(_ context.Context) error {
	if p.CachedResponse().Status == StatusFail {
		return fmt.Errorf("%w: shutting_down=%v", ErrProbeUnhealthy, p.shuttingDown.Load())
	}

	return nil
}

// ProbeShutdowner adapts a [Probe] to do.ShutdownerWithError. The adapter
// exists because the Probe's own Shutdown returns nothing, which cannot
// change without breaking the public API.
type ProbeShutdowner struct {
	// Probe is the probe to shut down.
	Probe *Probe
}

// Shutdown drains the wrapped probe and always returns nil: Probe.Shutdown
// reports problems by state (readiness 503), not by error.
func (s ProbeShutdowner) Shutdown() error {
	s.Probe.Shutdown()

	return nil
}

// AsShutdowner returns a do.ShutdownerWithError view of the probe for
// registration as a samber/do shutdown service.
func (p *Probe) AsShutdowner() ProbeShutdowner {
	return ProbeShutdowner{Probe: p}
}

// Healthz returns a combined-traffic handler: one endpoint answering "should
// traffic be routed here?". It returns 503 while the startup latch is unset
// (booting), when the readiness roll-up is fail, or when shutting down; 200
// with the full response body otherwise. Use it for deployments that expose
// a single health endpoint (external load balancers, composition layers);
// native Kubernetes configurations should keep the three-probe split.
func (p *Probe) Healthz() http.HandlerFunc {
	return p.guard(func(w http.ResponseWriter, r *http.Request) {
		resp := p.readinessResponse(r.Context())

		if p.shuttingDown.Load() {
			resp.ShuttingDown = true
			resp.Status = StatusFail
		}

		if !p.startupPassed.Load() && resp.Status != StatusFail {
			if resp.Checks == nil {
				resp.Checks = map[string]Check{}
			}

			resp.Checks["startup"] = Check{Status: StatusFail, Error: "startup latch not set"}
			resp.Status = StatusFail
		}

		code := http.StatusOK
		if resp.Status == StatusFail {
			code = http.StatusServiceUnavailable
		}

		writeResponse(w, code, resp)
	})
}
