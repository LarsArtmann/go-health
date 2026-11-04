package health

import "errors"

// ErrInvalidTimeout is returned by [Probe.Validate] when the configured timeout
// is zero or negative.
var ErrInvalidTimeout = errors.New("health: timeout must be positive")

// ErrInvalidRefreshInterval is returned by [Probe.Validate] when the configured
// refresh interval is negative.
var ErrInvalidRefreshInterval = errors.New("health: refresh interval must not be negative")

// ErrUnknownCriticalService is returned by [Probe.Start] when a name passed to
// [WithCriticalServices] never appears in the first health-check batch: no
// check produced a result under that name, so it can never influence
// readiness classification and the startup latch can never set. Match with
// errors.Is. See docs/start-validation-design.md.
var ErrUnknownCriticalService = errors.New("health: unknown critical service")

// ErrPanicDuringHealthCheck is wrapped into the synthetic "health-check" error
// when the health-check batch panics and the panic is recovered. Match with
// errors.Is to distinguish a recovered panic from an ordinary service failure:
// a recovered panic rolls the response up to fail, never to warn.
var ErrPanicDuringHealthCheck = errors.New("health: panic during health check")

// ErrPanicDuringEvaluationHook is wrapped into the synthetic
// "evaluation-hook" check error when a WithEvaluationHook callback panics and
// the panic is recovered. Match with errors.Is to detect observer failure
// specifically. Unlike a batch panic, a hook panic never fails the roll-up:
// the evaluation completed before the hook ran, so the response is degraded
// to warn at most (a visible "evaluation-hook" row), never to fail. See
// docs/panic-recovery-design.md, "the evaluation-hook surface".
var ErrPanicDuringEvaluationHook = errors.New("health: panic during evaluation hook")

// ErrRecorderNotApplicable is returned by [Probe.Validate] (and therefore
// [Probe.Start]) when [WithHealthRecorder] was passed to a function-path
// constructor — [NewWithHealthCheck], [NewWithDetailedCheck], or [NewChecks].
// Those constructors take an explicit batch function that owns execution, so
// the recorder would be accepted and silently ignored: no callbacks, no
// traces, no error. Rejecting at Start turns that dead wiring into a visible
// misconfiguration. Use [New] with a samber/do injector when recorder
// callbacks are required.
var ErrRecorderNotApplicable = errors.New(
	"health: WithHealthRecorder is not applicable to function-path constructors " +
		"(NewWithHealthCheck, NewWithDetailedCheck, NewChecks); use New(injector, ...) instead",
)
