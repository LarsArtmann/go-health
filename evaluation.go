package health

import (
	"context"
	"fmt"
	"maps"
	"time"
)

// Evaluate runs a full health-check batch against the injector and returns
// the aggregate response. This is the core evaluation logic used by the
// readiness and startup handlers, exposed publicly for testing and custom
// handler scenarios.
//
// The context should carry a deadline; [Probe.Start] applies [Probe.timeout]
// automatically. When a [HealthRecorder] is configured, checks are delegated
// to it instead of the raw injector.
//
// Evaluate does NOT publish its result: the served snapshot
// ([Probe.CachedResponse], readiness, dashboards) keeps the previous
// evaluation until the background loop or an explicit
// [Probe.EvaluateAndCache] replaces it. Use Evaluate for one-off probes and
// tests; use EvaluateAndCache when the result must become what the next
// request sees.
func (p *Probe) Evaluate(ctx context.Context) Response {
	start := time.Now()

	details := p.runHealthChecks(ctx)
	results := errorsOf(details)

	resp := Response{
		Version:        p.version,
		InstanceID:     p.instanceID,
		Uptime:         p.uptime(),
		ShuttingDown:   p.shuttingDown.Load(),
		Checks:         p.buildChecks(details),
		TotalLatencyMs: time.Since(start).Milliseconds(),
	}

	resp.Status = p.classify(results, resp.ShuttingDown)
	resp.Timestamp = p.now()

	if p.evalHook != nil {
		if hookErr := invokeEvaluationHook(p.evalHook, resp); hookErr != nil {
			resp.Checks["evaluation-hook"] = Check{
				Status: StatusWarn,
				Error:  hookErr.Error(),
			}
			if resp.Status.Rank() > StatusWarn.Rank() {
				resp.Status = StatusWarn
			}
		}
	}

	return resp
}

// EvaluateAndCache runs one full health-check batch bounded by
// [WithTimeout], publishes the result as the served snapshot, and returns
// it. It is the on-demand sibling of the background refresh tick: after the
// call, [Probe.CachedResponse], readiness, and every dashboard read observe
// this evaluation.
//
// Use it when a consumer's fallback path must leave the cache coherent —
// the common shape is "serve the cached response while it is fresh, run
// EvaluateAndCache when it is not". [Probe.Evaluate] deliberately does not
// publish, so a fallback built on it would answer correctly once and then
// keep serving the older snapshot.
//
// The caller's context may carry an earlier deadline; the batch deadline is
// the earlier of the two. Publishing races a concurrent refresh tick the
// same way two ticks would: both responses are truthful evaluations and the
// last store wins.
func (p *Probe) EvaluateAndCache(ctx context.Context) Response {
	evalCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	resp := p.Evaluate(evalCtx)
	p.latest.Store(&resp)

	return resp
}

// invokeEvaluationHook calls fn with a defensive copy of resp: the struct by
// value and a fresh clone of the Checks map, so a misbehaving hook can
// neither corrupt the response the probe serves and caches nor observe later
// mutations of it. A panicking hook is recovered and reported as an error
// wrapping [ErrPanicDuringEvaluationHook]; the caller degrades the response,
// never the process. See docs/panic-recovery-design.md.
func invokeEvaluationHook(fn func(Response), resp Response) (hookErr error) {
	hookView := resp

	hookView.Checks = make(map[string]Check, len(resp.Checks))
	maps.Copy(hookView.Checks, resp.Checks)

	invoked := func() bool {
		fn(hookView)

		return true
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				hookErr = fmt.Errorf("%w: %v", ErrPanicDuringEvaluationHook, r)
			}
		}()

		_ = invoked()
	}()

	return hookErr
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
func (p *Probe) runHealthChecks(ctx context.Context) map[string]CheckDetail {
	return recoverHealthChecks(func() map[string]CheckDetail { return p.healthCheck(ctx) })
}

// recoverHealthChecks runs fn and converts a panic into a synthetic result
// map. The inner closure confines the deferred recover to one frame whose
// result is captured in an outer variable, so no named return is needed.
func recoverHealthChecks(fn func() map[string]CheckDetail) map[string]CheckDetail {
	var details map[string]CheckDetail

	func() {
		defer func() {
			if r := recover(); r != nil {
				details = map[string]CheckDetail{
					"health-check": {Err: fmt.Errorf("%w: %v", ErrPanicDuringHealthCheck, r)},
				}
			}
		}()

		details = fn()
	}()

	return details
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
	return p.rollups.classify(results, shuttingDown)
}

// evaluateStartup checks whether all critical services are present and healthy
// in the results map. Returns true if the startup latch should be set.
func (p *Probe) evaluateStartup(results map[string]error) bool {
	return p.rollups.evaluateStartup(results)
}

// buildChecks converts the raw per-service reports into typed Check entries.
// A nil error means the service passed; a non-nil error populates the Error
// field. Failures on critical services are marked StatusFail; failures on
// non-critical services are marked StatusWarn to distinguish "degraded but
// functional" from "take this pod out of rotation"; an [*OffError] marks the
// check StatusOff with its Detail text in Error ("deliberately not
// configured" — visible, never a verdict). Executor-reported
// durations are carried through, and every check is stamped with StatusSince —
// when the probe first observed its current status — via the transition
// tracker.
func (p *Probe) buildChecks(details map[string]CheckDetail) map[string]Check {
	checks := make(map[string]Check, len(details))

	for name, detail := range details {
		check := Check{Status: StatusPass, DurationNanos: detail.Duration.Nanoseconds()}

		if detail.Err != nil {
			check.Status = p.rollups.grades(name, detail.Err)
			check.Error = detail.Err.Error()
		}

		checks[name] = check
	}

	p.transitions.stamp(checks, p.now())

	return checks
}
