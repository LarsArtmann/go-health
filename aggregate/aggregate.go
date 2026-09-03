// Package aggregate merges multiple in-process [health.Probe] instances into
// a single health surface: one [health.Response], one set of kubelet probe
// handlers, one thing to point a dashboard or monitoring stack at.
//
// The intended topology is one process that embeds several independently
// configured probes — one per logical service or tenant — and needs to expose
// their combined state as if it were a single go-health deployment. Merging
// happens on read: every [Aggregate.CachedResponse] call performs one atomic
// [health.Probe.CachedResponse] load per source, so the aggregate adds no
// locks, no goroutines, and no staleness of its own. Freshness is bounded by
// the slowest source's refresh interval.
//
// Sources must have unique non-empty names. Every check is namespaced as
// "source/check", which keeps keys collision-free and gives consumers a
// stable grouping axis (the part before the first "/").
package aggregate

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"time"

	health "github.com/larsartmann/go-health"
)

// ErrNoSources is returned by [New] when called without any sources. An
// aggregate of nothing has no meaningful state to serve.
var ErrNoSources = errors.New("aggregate: at least one source is required")

// ErrInvalidSource is returned by [New] for a source with an empty name, a
// nil probe, or a name that collides with an earlier source. Names become
// check-key prefixes, so duplicates would silently overwrite check entries.
var ErrInvalidSource = errors.New("aggregate: invalid source")

// Source is one named probe contributing to an [Aggregate]. The name becomes
// the prefix of every check key the probe contributes ("name/check") and the
// key under which its startup state is reported.
type Source struct {
	// Name namespaces this source's checks. Must be non-empty and unique
	// across all sources of an aggregate.
	Name string
	// Probe is the in-process probe whose cached state is merged.
	Probe *health.Probe
}

// Aggregate presents N probes as a single go-health-compatible surface. It is
// passive (no Start, no goroutines): handlers and accessors read the sources'
// atomic caches on demand. Aggregate is safe for concurrent use.
type Aggregate struct {
	sources         []Source
	refreshInterval time.Duration
}

// New creates an [Aggregate] over the given sources. Construction validates
// the invariants that reads would otherwise have to enforce silently: at
// least one source, and unique non-empty names with non-nil probes.
func New(sources ...Source) (*Aggregate, error) {
	if len(sources) == 0 {
		return nil, ErrNoSources
	}

	seen := make(map[string]struct{}, len(sources))
	maxInterval := time.Duration(0)

	for _, src := range sources {
		switch {
		case src.Probe == nil:
			return nil, fmt.Errorf("%w: source %q has a nil Probe", ErrInvalidSource, src.Name)
		case src.Name == "":
			return nil, fmt.Errorf("%w: source name must not be empty", ErrInvalidSource)
		}

		if _, dup := seen[src.Name]; dup {
			return nil, fmt.Errorf("%w: duplicate source name %q", ErrInvalidSource, src.Name)
		}

		seen[src.Name] = struct{}{}

		if d := src.Probe.RefreshInterval(); d > maxInterval {
			maxInterval = d
		}
	}

	return &Aggregate{sources: sources, refreshInterval: maxInterval}, nil
}

// CachedResponse returns the merged health state of all sources. The overall
// status is the worst of the source statuses (fail beats warn beats pass);
// checks are namespaced "source/check"; a shutting-down source forces the
// aggregate to report ShuttingDown with overall fail, mirroring
// [health.Probe] semantics. TotalLatencyMs is the slowest source's batch.
// Version and Uptime are zero: they are per-process scalars and do not
// survive a merge.
func (a *Aggregate) CachedResponse() health.Response {
	checks := make(map[string]health.Check)
	status := health.StatusPass
	shuttingDown := false

	var maxLatency int64

	for _, src := range a.sources {
		resp := src.Probe.CachedResponse()

		if resp.ShuttingDown {
			shuttingDown = true
		}

		if resp.TotalLatencyMs > maxLatency {
			maxLatency = resp.TotalLatencyMs
		}

		if statusRank(resp.Status) < statusRank(status) {
			status = resp.Status
		}

		for name, check := range resp.Checks {
			checks[src.Name+"/"+name] = check
		}
	}

	if shuttingDown {
		status = health.StatusFail
	}

	return health.Response{
		Status:         status,
		ShuttingDown:   shuttingDown,
		Checks:         checks,
		TotalLatencyMs: maxLatency,
	}
}

// RefreshInterval returns the slowest source's refresh interval: the merged
// view can never be fresher than that. Zero means at least one source
// evaluates live and no source uses a background cache.
func (a *Aggregate) RefreshInterval() time.Duration {
	return a.refreshInterval
}

// StartupComplete reports whether every source has flipped its startup latch.
// It is the AND of the sources' [health.Probe.StartupComplete] values and is
// one-way per source: once all latches are set, it stays true.
func (a *Aggregate) StartupComplete() bool {
	for _, src := range a.sources {
		if !src.Probe.StartupComplete() {
			return false
		}
	}

	return true
}

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

// RegisterRoutes registers all three aggregate probe handlers on the given
// mux using the provided routes. Pass [health.DefaultRoutes] for the
// conventional Kubernetes paths.
func (a *Aggregate) RegisterRoutes(mux *http.ServeMux, routes health.Routes) {
	mux.HandleFunc(routes.Liveness, a.LivenessHandler())
	mux.HandleFunc(routes.Readiness, a.ReadinessHandler())
	mux.HandleFunc(routes.Startup, a.StartupHandler())
}

// statusRank orders statuses by severity so merges can pick the worst:
// lower rank wins. Unknown values rank as healthy — the Status type is
// validated at the boundaries and unknowns must not fail an aggregate.
func statusRank(s health.Status) int {
	switch s {
	case health.StatusFail:
		return 0
	case health.StatusWarn:
		return 1
	case health.StatusPass:
		return 2
	default:
		return 2
	}
}

// writeResponse serialises the health response as JSON with the given status
// code. Same wire format as the root package's handlers: deterministic key
// order, no caching.
func writeResponse(w http.ResponseWriter, code int, resp health.Response) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	payload, err := json.Marshal(resp, json.Deterministic(true))
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
	_, _ = w.Write(payload)
}
