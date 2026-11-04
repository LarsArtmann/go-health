package aggregate

import (
	"encoding/json/v2"
	"net/http"

	health "github.com/larsartmann/go-health"
)

// LivenessHandler answers the aggregate's liveness question: "is this
// process alive?" Liveness performs zero dependency checks by design — the
// aggregate's process being alive says nothing about its sources, and
// restarting over a dependency blip would cause a restart cascade. Always
// 200 with an empty checks map, mirroring [health.Probe.LivenessHandler].
func (a *Aggregate) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeResponse(w, http.StatusOK, health.Response{
			Status: health.StatusPass,
			Checks: map[string]health.Check{},
		})
	}
}

// ReadinessHandler answers the aggregate's readiness question: "can this
// instance serve traffic?" Serves the merged cached state: 503 when the
// overall status is fail (which includes any source shutting down), 200
// otherwise. Non-critical degradations (warn) stay 200, matching
// [health.Probe.ReadinessHandler]: a degraded instance can still serve.
func (a *Aggregate) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		resp := a.CachedResponse()

		code := http.StatusOK
		if resp.Status == health.StatusFail {
			code = http.StatusServiceUnavailable
		}

		writeResponse(w, code, resp)
	}
}

// StartupHandler answers the aggregate's startup question: "is every source
// done booting?" Returns 503 with one failing check per incomplete source
// until all startup latches are set, then 200 with an empty checks map.
func (a *Aggregate) StartupHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if a.StartupComplete() {
			writeResponse(w, http.StatusOK, health.Response{
				Status: health.StatusPass,
				Checks: map[string]health.Check{},
			})

			return
		}

		checks := make(map[string]health.Check)

		for _, src := range a.sources {
			if !src.Probe.StartupComplete() {
				checks[src.Name] = health.Check{
					Status: health.StatusFail,
					Error:  "startup incomplete",
				}
			}
		}

		writeResponse(w, http.StatusServiceUnavailable, health.Response{
			Status: health.StatusFail,
			Checks: checks,
		})
	}
}

// Healthz answers the aggregate's single-endpoint question: "should traffic
// be routed here?" — for deployments behind an external load balancer rather
// than a kubelet's three probes. It is the worst of the same three conditions
// [health.Probe.Healthz] evaluates, applied across all sources: 503 while any
// source has an unset startup latch, the merged roll-up is fail (which
// includes any source shutting down), or the aggregate is draining; 200
// otherwise, with the merged body. Non-critical degradation (warn) stays 200.
// Until every latch is set, the body carries one synthetic failing "startup"
// check, mirroring [health.Probe.Healthz]. Like the root probe, this handler
// is standalone: [Aggregate.RegisterRoutes] keeps wiring the three kubelet
// probes, and [health.DefaultRoutes] already claims /healthz for liveness.
func (a *Aggregate) Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		resp := a.CachedResponse()

		if !a.StartupComplete() && resp.Status != health.StatusFail {
			resp.Checks["startup"] = health.Check{
				Status: health.StatusFail,
				Error:  "startup latch not set",
			}
			resp.Status = health.StatusFail
		}

		code := http.StatusOK
		if resp.Status == health.StatusFail {
			code = http.StatusServiceUnavailable
		}

		writeResponse(w, code, resp)
	}
}

// RegisterRoutes registers the aggregate probe handlers on the given mux
// using the provided routes. Pass [health.DefaultRoutes] for the conventional
// Kubernetes paths. A route whose pattern is empty is skipped, so an
// aggregate that only exposes readiness mounts only that handler. The
// Routes.Version field is ignored here: the aggregate carries no build
// identity, so it cannot serve a truthful /version — wire
// [health.VersionHandler] yourself if you stamp the hosting process.
func (a *Aggregate) RegisterRoutes(mux *http.ServeMux, routes health.Routes) {
	registerRoute(mux, routes.Liveness, a.LivenessHandler())
	registerRoute(mux, routes.Readiness, a.ReadinessHandler())
	registerRoute(mux, routes.Startup, a.StartupHandler())
}

// registerRoute mounts handler at pattern unless the pattern is empty.
func registerRoute(mux *http.ServeMux, pattern string, handler http.HandlerFunc) {
	if pattern == "" {
		return
	}

	mux.HandleFunc(pattern, handler)
}

// marshalResponse is the single serialization seam for aggregate responses.
// It is a package variable only so tests can force the marshal-error branch;
// production code must never swap it.
//
//nolint:gochecknoglobals // deliberate test seam; mirrors the root package's seam
var marshalResponse = func(resp health.Response) ([]byte, error) {
	return json.Marshal(health.CoerceValidUTF8(resp), json.Deterministic(true))
}

// writeResponse serialises the health response as JSON with the given status
// code. Same wire format as the root package's handlers: deterministic key
// order, no caching.
func writeResponse(w http.ResponseWriter, code int, resp health.Response) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	payload, err := marshalResponse(resp)
	if err != nil {
		// Defensive: Response only contains basic types so json.Marshal
		// cannot fail today. Mirrors the root package's guard, including
		// the underlying cause in the body.
		http.Error(
			w,
			"aggregate: failed to encode response: "+err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	w.WriteHeader(code)
	_, _ = w.Write(
		payload,
	) //nolint:erraudit // intentional: status already committed; a library must not log client disconnects
}
