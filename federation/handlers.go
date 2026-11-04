package federation

import (
	"encoding/json/v2"
	"net/http"

	health "github.com/larsartmann/go-health"
)

// LivenessHandler answers the federation's liveness question: "is this
// process alive?" Liveness performs zero remote fetches by design — the
// hub's process being alive says nothing about its remotes, and
// restarting the hub over a remote blip would cause a restart cascade.
// Always 200 with an empty checks map, mirroring [health.Probe].
func (p *Prober) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeResponse(w, http.StatusOK, health.Response{
			Status: health.StatusPass,
			Checks: map[string]health.Check{},
		})
	}
}

// ReadinessHandler answers the federation's readiness question: "can
// this hub serve traffic?" Serves the merged view: 503 when the overall
// status is fail (which includes any unreachable or shutting-down
// remote, and the hub itself once [Prober.MarkShuttingDown] flipped the
// drain), 200 otherwise. A dark remote takes the hub out of rotation —
// deliberate: the hub's whole answer is its remotes.
func (p *Prober) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := p.cachedResponse(r.Context())

		code := http.StatusOK
		if resp.Status == health.StatusFail {
			code = http.StatusServiceUnavailable
		}

		writeResponse(w, code, resp)
	}
}

// StartupHandler answers the federation's startup question: "has every
// remote answered successfully at least once?" The fetch itself moves
// the latches, so repeated kubelet polls make progress. Returns 503
// with one failing check per not-yet-latched remote until all latches
// are set, then 200 with an empty checks map.
func (p *Prober) StartupHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = p.cachedResponse(r.Context())

		if p.StartupComplete() {
			writeResponse(w, http.StatusOK, health.Response{
				Status: health.StatusPass,
				Checks: map[string]health.Check{},
			})

			return
		}

		checks := make(map[string]health.Check)

		for i := range p.states {
			if !p.states[i].latched.Load() {
				checks[p.states[i].remote.Name] = health.Check{
					Status: health.StatusFail,
					Error:  "no successful fetch yet",
				}
			}
		}

		writeResponse(w, http.StatusServiceUnavailable, health.Response{
			Status: health.StatusFail,
			Checks: checks,
		})
	}
}

// RegisterRoutes registers the federation probe handlers on the given mux
// using the provided routes. Pass [health.DefaultRoutes] for the conventional
// Kubernetes paths. A route whose pattern is empty is skipped, so a hub that
// only exposes readiness mounts only that handler. A non-empty Routes.Version
// mounts [health.VersionHandler]; the hub carries no build identity, so the
// mount answers {"version":""} — leave the field empty and wire the handler
// yourself if the hosting process has a stamp.
func (p *Prober) RegisterRoutes(mux *http.ServeMux, routes health.Routes) {
	registerRoute(mux, routes.Liveness, p.LivenessHandler())
	registerRoute(mux, routes.Readiness, p.ReadinessHandler())
	registerRoute(mux, routes.Startup, p.StartupHandler())
	registerRoute(mux, routes.Version, health.VersionHandler(""))
}

// registerRoute mounts handler at pattern unless the pattern is empty.
func registerRoute(mux *http.ServeMux, pattern string, handler http.HandlerFunc) {
	if pattern == "" {
		return
	}

	mux.HandleFunc(pattern, handler)
}

// marshalResponse is the single serialization seam for federation
// responses. It is a package variable only so tests can force the
// marshal-error branch; production code must never swap it.
//
//nolint:gochecknoglobals // deliberate test seam; mirrors the aggregate's seam
var marshalResponse = func(resp health.Response) ([]byte, error) {
	return json.Marshal(health.CoerceValidUTF8(resp), json.Deterministic(true))
}

// writeResponse serialises the health response as JSON with the given
// status code. Same wire format as the root package's handlers:
// deterministic key order, no caching.
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
			"federation: failed to encode response: "+err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	w.WriteHeader(code)
	_, _ = w.Write(
		payload,
	) //nolint:erraudit // intentional: status already committed; a library must not log client disconnects
}
