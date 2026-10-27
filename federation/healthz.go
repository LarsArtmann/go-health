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
		resp := applyStartupLatchOverlay(p.cachedResponse(r.Context()), p.StartupComplete())

		code := http.StatusOK
		if resp.Status == health.StatusFail {
			code = http.StatusServiceUnavailable
		}

		writeResponse(w, code, resp)
	}
}

// applyStartupLatchOverlay adds the synthetic failing "startup" row while the
// startup latch is open, mirroring [health.Probe.Healthz] and
// [aggregate.Aggregate.Healthz]. It never overwrites a fail the merge already
// produced: a reachable row or a remote's own failing check explains the real
// cause, and replacing that explanation with "startup latch not set" would be
// a lie about why the hub is out of rotation.
//
// Given federation's read path the latch is provably closed only while the
// merged status is already fail — every successful fetch flips its remote's
// latch before the merge runs, and a failed fetch contributes a reachable fail
// row — so this branch is unreachable through the public API today. It stays
// because it is the aggregate's contract, because it is one atomic load, and
// because an unreachable-contract change here would otherwise surface as a
// silent 200 from a hub that never finished starting. The internal test pins
// the behaviour directly.
func applyStartupLatchOverlay(resp health.Response, startupComplete bool) health.Response {
	if startupComplete || resp.Status == health.StatusFail {
		return resp
	}

	// Defensive: the merge always allocates a Checks map, but the overlay
	// must never panic if a future read path changes that.
	if resp.Checks == nil {
		resp.Checks = map[string]health.Check{}
	}

	resp.Checks["startup"] = health.Check{
		Status: health.StatusFail,
		Error:  "startup latch not set",
	}
	resp.Status = health.StatusFail

	return resp
}
