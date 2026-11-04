package health

import "time"

// CachedResponse returns the last background-refreshed health Response. When
// the background cache is active ([Probe.Start] called with a non-zero
// [WithRefreshInterval]), this reads the atomic p.latest pointer — lock-free,
// zero dependency calls. When no cache exists (live mode or before the first
// refresh), it returns a zero-value Response with StatusPass.
//
// The live shuttingDown flag is overlaid on the cached value so a stale cached
// response (evaluated before [Probe.Shutdown] was called) still reflects the
// current shutdown state.
//
// The returned Response is a read-only view: its Checks map aliases the
// published snapshot on purpose (the lock-free read must not allocate). Use
// [Response.Clone] before projecting extra rows onto it, and use
// [Probe.CacheAge] to distinguish "fresh pass" from "never evaluated" — the
// zero-value response above is indistinguishable from a real all-pass batch
// by status alone.
func (p *Probe) CachedResponse() Response {
	if cached := p.latest.Load(); cached != nil {
		resp := *cached

		if p.shuttingDown.Load() {
			resp.ShuttingDown = true
			resp.Status = StatusFail
		}

		return resp
	}

	resp := Response{Status: StatusPass, Checks: map[string]Check{}}

	if p.shuttingDown.Load() {
		resp.ShuttingDown = true
		resp.Status = StatusFail
	}

	return resp
}

// CacheAge reports how long ago the served snapshot was evaluated, and
// whether a snapshot exists at all. It never runs checks and never allocates.
//
// The boolean separates the two states [Probe.CachedResponse] cannot: false
// means no evaluation has been published yet (never started, or live mode
// with no request so far), so the synthesized pass response must not be
// trusted as evidence. A zero evaluation timestamp (a custom
// [WithNowFunc] returning the zero time) also reports false.
//
// Age comes from the same injected clock the probe stamps responses with, so
// a fixed test clock yields a deterministic age. Use it with
// [Probe.CachedResponseFresh] for the serve-cache-or-evaluate policy that
// consumers otherwise hand-roll around the raw timestamp. See
// docs/cache-freshness-design.md; why this stays a primitive rather than a
// staleness option is docs/adr/ADR-010-freshness-is-a-primitive.md.
func (p *Probe) CacheAge() (time.Duration, bool) {
	cached := p.latest.Load()
	if cached == nil || cached.Timestamp.IsZero() {
		return 0, false
	}

	// A backwards clock step must not report a negative age; the snapshot
	// is simply as fresh as it can be.
	return max(p.now().Sub(cached.Timestamp), 0), true
}

// CachedResponseFresh returns the served snapshot when it was evaluated at
// most maxAge ago. When no snapshot exists, it is older than maxAge, or
// maxAge is not positive, it returns the zero Response and false.
//
// This is the read-side half of the "serve cache while fresh, otherwise
// evaluate" policy:
//
//	if resp, ok := probe.CachedResponseFresh(2 * probe.RefreshInterval()); ok {
//	    serve(resp)
//	} else {
//	    serve(probe.EvaluateAndCache(ctx))
//	}
//
// The returned response carries the live shutdown overlay, exactly like
// [Probe.CachedResponse]. A non-positive maxAge never reports fresh: a
// zero-length freshness window would otherwise bless a snapshot taken an
// arbitrary time ago.
func (p *Probe) CachedResponseFresh(maxAge time.Duration) (Response, bool) {
	if maxAge <= 0 {
		return Response{}, false
	}

	age, ok := p.CacheAge()
	if !ok || age > maxAge {
		return Response{}, false
	}

	return p.CachedResponse(), true
}

// RefreshInterval returns the configured background cache refresh interval.
// Returns zero when the probe is in live evaluation mode.
func (p *Probe) RefreshInterval() time.Duration {
	return p.refreshInterval
}
