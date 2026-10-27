package federation

import (
	"net/http"

	"github.com/larsartmann/go-health"
)

// Healthz answers the federation's single-endpoint question: "should traffic
// be routed here?" It exists for deployments behind an external load balancer
// rather than a kubelet's three probes, mirroring [health.Probe.Healthz] and
// [aggregate.Aggregate.Healthz].
//
// The handler returns 503 while any per-remote startup latch is unset, the
// merged roll-up is fail (which already includes an unreachable remote's
// synthetic "name/reachable" row and any remote reporting shutting_down), or
// the hub itself is draining; 200 otherwise, with the merged body.
// Non-critical degradation (warn) stays 200.
//
// Like the aggregate's handler it is standalone: [Prober.RegisterRoutes] keeps
// wiring the three kubelet probes, and [health.DefaultRoutes] already claims
// /healthz for liveness. It uses the same read path as
// [Prober.ReadinessHandler], so one parallel fetch across the remotes happens
// per request; the fetch caps and polling guidance in the package doc apply
// unchanged.
func (p *Prober) Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := p.cachedResponse(r.Context())

		if !p.StartupComplete() && resp.Status != health.StatusFail {
			// Defensive: the merge always allocates a Checks map, but the
			// overlay must never panic if a future read path changes that.
			if resp.Checks == nil {
				resp.Checks = map[string]health.Check{}
			}

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
