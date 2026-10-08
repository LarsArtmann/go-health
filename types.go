package health

import "time"

// Status is the roll-up health status of a check or the overall response.
type Status string

const (
	// StatusPass means the service or system is healthy.
	StatusPass Status = "pass"
	// StatusFail means the service or system is unhealthy.
	StatusFail Status = "fail"
	// StatusWarn means the service is degraded but functional.
	// Used for non-critical service failures in readiness responses.
	StatusWarn Status = "warn"
	// StatusOff means the check's dependency is deliberately not
	// configured: an intentional absence, not a failure (an optional
	// analytics database, a disabled integration). Produced only by the
	// [Off] sentinel error; the roll-up ([Response.Status]) never reports
	// it — off is visibility, never a verdict. See
	// docs/configured-off-design.md.
	StatusOff Status = "off"
)

// Rank orders statuses by severity so merges can pick the worst: a lower
// rank is worse (fail 0, warn 1, pass 2, off 2). Unknown values rank as
// pass: the Status type is validated at the boundaries, and an unknown must
// not fail a merge ([aggregate.Aggregate], [federation namespace]). Merge
// sites compare `got.Rank() < worst.Rank()` to keep the more severe one.
func (s Status) Rank() int {
	switch s {
	case StatusFail:
		return 0
	case StatusWarn:
		return 1
	case StatusPass, StatusOff:
		return 2
	default:
		return 2
	}
}

// Check is the per-service health result.
type Check struct {
	// Status is the health status of this individual check.
	Status Status `json:"status"`
	// Error contains the failure message when Status is not pass. For an
	// off check (StatusOff) it carries the configuration detail — usually
	// the enable recipe — instead of a failure.
	Error string `json:"error,omitempty"`
	// Since is when this check entered its current status, as observed by
	// the probe: the time of the first batch that reported the current
	// value, carried forward unchanged while the status holds. It is
	// stamped on every evaluation path (background refresh, live requests,
	// startup probes) and resets when the status changes, when a check
	// reappears after being absent, or when the process restarts. Zero —
	// and omitted from JSON via omitzero — when unknown (checks the probe
	// never built, e.g. liveness's empty set or Healthz's synthetic
	// startup entry). See docs/check-metadata-design.md.
	Since time.Time `json:"since,omitzero"`
	// DurationNanos is how long the most recent execution of this check
	// took, in nanoseconds, as reported by its executor. Zero — and omitted
	// from JSON via omitzero — when the executor does not report timing: the
	// raw samber/do injector path cannot measure per-check duration, so only
	// detailed sources ([NewWithDetailedCheck], [DetailedHealthRecorder])
	// populate it. Nanoseconds (not milliseconds) so sub-millisecond checks
	// — the common case — survive losslessly; a Go time.Duration is not used
	// here because encoding/json/v2 has no default (or tag-format)
	// representation for it, which would break consumer re-marshaling.
	DurationNanos int64 `json:"duration_ns,omitzero"`
}

// OffError marks a check as deliberately not configured — an intentional
// absence, not a failure. Return it from any check executor (a
// [CheckFunc], a [NewWithHealthCheck] batch, or a samber/do
// HealthcheckerWithContext service) to render the check with [StatusOff]
// and its detail text:
//
//	health.Off("not configured: set database.url / CV_DATABASE_URL to enable analytics")
//
// An off check never fails readiness, never blocks the startup latch, and
// never warns the roll-up — but the row stays visible on every health
// surface, carrying the enable recipe in the Error field. Detail should
// state what is (not) configured and how to enable it. Wrap-free by
// design: wrapping is preserved via errors.As, but a lost sentinel degrades
// to a plain warn/fail, so return the sentinel unwrapped.
type OffError struct {
	// Detail explains what is not configured and how to enable it.
	Detail string
}

// Error implements error so [Off] results flow the plain-error health-check
// channel (map[string]error, samber/do HealthcheckerWithContext). The
// message is the Detail verbatim — the status field already says "off".
func (e *OffError) Error() string {
	return e.Detail
}

// Off returns an [*OffError] marking the check as deliberately not
// configured. See [OffError].
func Off(detail string) *OffError {
	return &OffError{Detail: detail}
}

// CheckDetail is the executor's raw report for one service check: the outcome
// plus execution metadata only the executor can know. It is the metadata-rich
// input accepted by [NewWithDetailedCheck] and [DetailedHealthRecorder].
// The probe still owns classification: Err is graded against the critical set
// exactly like a plain map[string]error result (an [*OffError] grades to
// [StatusOff] regardless of criticality), and Since is probe-observed,
// so a detail cannot influence Status, Error text, or Since.
type CheckDetail struct {
	// Err is the check outcome: nil means the service is healthy. Non-nil
	// errors are graded (critical → fail, non-critical → warn) and their
	// message becomes the check's Error field.
	Err error
	// Duration is how long this execution of the check took. Zero means
	// unknown and is omitted from the wire response.
	Duration time.Duration
}

// Response is the aggregate health-check response served by all probe handlers.
type Response struct {
	// Status is the overall roll-up: fail if any critical service is down
	// or the probe is shutting down, warn if only non-critical services
	// are degraded, pass when all services are healthy.
	Status Status `json:"status"`
	// Version is the application version, if configured.
	Version string `json:"version,omitempty"`
	// InstanceID identifies this replica when multiple instances serve behind
	// one load balancer, if configured via [WithInstanceID].
	InstanceID string `json:"instance_id,omitempty"`
	// Uptime is human-readable duration since boot.
	Uptime string `json:"uptime,omitempty"`
	// ShuttingDown is true when the probe has been marked for shutdown.
	// Readiness returns 503 when this is set; liveness stays 200.
	ShuttingDown bool `json:"shutting_down,omitempty"`
	// TotalLatencyMs is the wall-clock time spent running the health-check
	// batch. Populated by readiness and startup evaluations; always zero
	// for liveness (which performs no dependency checks).
	TotalLatencyMs int64 `json:"total_latency_ms,omitempty"`
	// Timestamp is when the evaluation completed (server time, RFC 3339 in
	// JSON). Zero — and omitted from JSON via omitzero — until the first
	// evaluation. Live-mode throttling uses it to judge cache freshness.
	Timestamp time.Time `json:"timestamp,omitzero"`
	// Checks maps each service name to its individual result.
	Checks map[string]Check `json:"checks"`
}
