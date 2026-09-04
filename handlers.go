package health

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"strings"
	"time"
)

// Routes configures the URL paths for [Probe.RegisterRoutes].
type Routes struct {
	Liveness  string
	Readiness string
	Startup   string
}

// DefaultRoutes returns the conventional Kubernetes health-probe paths.
func DefaultRoutes() Routes {
	return Routes{
		Liveness:  "/healthz",
		Readiness: "/readyz",
		Startup:   "/startupz",
	}
}

// LivenessHandler returns an http.HandlerFunc that answers the liveness
// question: "Is the process alive and not deadlocked?"
//
// Liveness performs zero dependency checks and returns in microseconds. It
// always returns 200 with status "pass". This prevents restart cascades
// caused by downstream dependency blips.
func (p *Probe) LivenessHandler() http.HandlerFunc {
	return p.guard(func(w http.ResponseWriter, _ *http.Request) {
		resp := Response{
			Status:  StatusPass,
			Uptime:  p.uptime(),
			Version: p.version,
			Checks:  map[string]Check{},
		}

		writeResponse(w, http.StatusOK, resp)
	})
}

// ReadinessHandler returns an http.HandlerFunc that answers the readiness
// question: "Can this instance serve traffic right now?"
//
// Readiness runs a full health-check batch against the injector, classifies
// results into critical and non-critical, and returns:
//
//   - 200 when all critical services pass (non-critical failures appear as
//     individual check entries but do not change the status code).
//   - 503 when any critical service fails or the probe is shutting down.
//
// When a background cache is active (RefreshInterval > 0 and [Probe.Start]
// called), the handler serves the cached result for O(1) response time. When
// no cache is available, it evaluates live with a timeout-bounded context.
func (p *Probe) ReadinessHandler() http.HandlerFunc {
	return p.guard(func(w http.ResponseWriter, r *http.Request) {
		resp := p.readinessResponse(r.Context())

		// Always overlay the live shutdown flag so a stale cached response
		// (evaluated before Shutdown was called) still produces 503.
		if p.shuttingDown.Load() {
			resp.ShuttingDown = true
			resp.Status = StatusFail
		}

		code := http.StatusOK
		if resp.Status == StatusFail {
			code = http.StatusServiceUnavailable
		}

		writeResponse(w, code, resp)
	})
}

// StartupHandler returns an http.HandlerFunc that answers the startup
// question: "Is the application done booting?"
//
// Startup evaluates critical services on every request until all of them are
// present and healthy. Once that condition is met, the latch flips and all
// subsequent calls return 200 immediately without re-checking. This allows
// Kubernetes to use a generous failureThreshold for slow-booting applications
// without affecting liveness or readiness sensitivity.
func (p *Probe) StartupHandler() http.HandlerFunc {
	return p.guard(func(w http.ResponseWriter, r *http.Request) {
		if p.startupPassed.Load() {
			resp := Response{
				Status:  StatusPass,
				Uptime:  p.uptime(),
				Version: p.version,
				Checks:  map[string]Check{},
			}

			writeResponse(w, http.StatusOK, resp)

			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), p.timeout)
		defer cancel()

		results := p.runHealthChecks(ctx)

		if p.evaluateStartup(results) {
			p.startupPassed.Store(true)
		}

		resp := p.buildStartupResponse(results)

		code := http.StatusServiceUnavailable
		if p.startupPassed.Load() {
			code = http.StatusOK
		}

		writeResponse(w, code, resp)
	})
}

// RegisterRoutes registers all three probe handlers on the given mux using
// the provided routes. Pass [DefaultRoutes] for conventional Kubernetes paths.
func (p *Probe) RegisterRoutes(mux *http.ServeMux, routes Routes) {
	mux.HandleFunc(routes.Liveness, p.LivenessHandler())
	mux.HandleFunc(routes.Readiness, p.ReadinessHandler())
	mux.HandleFunc(routes.Startup, p.StartupHandler())
}

// readinessResponse returns the cached response when available, or evaluates
// live with a timeout-bounded context. With [WithLiveThrottle] set, live
// evaluations coalesce: within the throttle window the stored result of the
// previous evaluation is served, so request floods cannot amplify into batch
// floods against the dependencies.
func (p *Probe) readinessResponse(ctx context.Context) Response {
	if p.liveThrottle > 0 {
		return p.throttledLiveResponse(ctx)
	}

	if cached := p.latest.Load(); cached != nil {
		return *cached
	}

	evalCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	return p.Evaluate(evalCtx)
}

// throttledLiveResponse serializes live evaluations and reuses the stored
// result while it is younger than the throttle window. The stored response's
// Timestamp is the freshness marker; Evaluate stamps it.
func (p *Probe) throttledLiveResponse(ctx context.Context) Response {
	p.throttleMu.Lock()
	defer p.throttleMu.Unlock()

	if cached := p.latest.Load(); cached != nil && time.Since(cached.Timestamp) < p.liveThrottle {
		return *cached
	}

	evalCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	resp := p.Evaluate(evalCtx)
	p.latest.Store(&resp)

	return resp
}

// buildStartupResponse assembles the response for a startup probe evaluation.
func (p *Probe) buildStartupResponse(results map[string]error) Response {
	resp := Response{
		Version:      p.version,
		Uptime:       p.uptime(),
		Checks:       p.buildChecks(results),
		ShuttingDown: p.shuttingDown.Load(),
	}

	if p.startupPassed.Load() {
		resp.Status = StatusPass
	} else {
		resp.Status = StatusFail
	}

	return resp
}

// marshalResponse is the single serialization seam for health responses.
// It is a package variable only so tests can force the marshal-error branch;
// production code must never swap it.
//
//nolint:gochecknoglobals // deliberate test seam; see handlers_internal_test.go
var marshalResponse = func(resp Response) ([]byte, error) {
	return json.Marshal(resp, json.Deterministic(true))
}

// SanitizeResponse coerces every string field to valid UTF-8, replacing
// invalid bytes with U+FFFD. encoding/json/v2 refuses to marshal invalid
// UTF-8 (v1 replaced it silently), so without this a dependency emitting a
// malformed error string could turn a health endpoint into a 500. Applying
// it at the write seam keeps the served bytes valid under both v1 and v2
// semantics for any input.
func SanitizeResponse(resp Response) Response {
	resp.Status = Status(strings.ToValidUTF8(string(resp.Status), "\uFFFD"))
	resp.Version = strings.ToValidUTF8(resp.Version, "\uFFFD")
	resp.Uptime = strings.ToValidUTF8(resp.Uptime, "\uFFFD")

	if resp.Checks != nil {
		checks := make(map[string]Check, len(resp.Checks))

		for name, check := range resp.Checks {
			check.Status = Status(strings.ToValidUTF8(string(check.Status), "\uFFFD"))
			check.Error = strings.ToValidUTF8(check.Error, "\uFFFD")

			checks[strings.ToValidUTF8(name, "\uFFFD")] = check
		}

		resp.Checks = checks
	}

	return resp
}

// writeResponse serialises the health response as JSON with the given status code.
func writeResponse(w http.ResponseWriter, code int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	payload, err := marshalResponse(SanitizeResponse(resp))
	if err != nil {
		// Defensive: SanitizeResponse removes the one known marshal failure
		// mode (invalid UTF-8). This branch guards against future fields
		// that might introduce marshal errors, and includes the underlying
		// cause so a regression is debuggable.
		http.Error(
			w,
			"health: failed to encode response: "+err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	w.WriteHeader(code)

	// The status line is already committed so a write failure (client
	// disconnect, broken pipe) is genuinely unrecoverable. Silently
	// swallow — a library must not make logging decisions for the host.
	_, _ = w.Write(
		payload,
	) //nolint:erraudit // intentional: status already committed; a library must not log client disconnects
}
