// Package merge implements the shared worst-of fold behind go-health's
// merge-on-read surfaces: [aggregate.Aggregate] (in-process probes) and
// [federation.Prober] (HTTP remotes) both reduce N per-source health
// responses into one [health.Response]. This package is the single home for
// those semantics so a wire-contract fix can never be made in one place and
// silently missed in the other.
//
// The primitive is pure — no locks, no goroutines, no state. Callers own
// acquisition: aggregate loads one atomic cache per source, federation
// performs one parallel HTTP fetch per remote, and both hand the
// already-fetched contributions to [Responses]. Federation-side state (per
// remote startup latches) also stays federation-side: a latch is an output
// of a successful fetch, not a merge input.
//
// Design: docs/merge-unification-design.md.
package merge

import (
	health "github.com/larsartmann/go-health"
)

// Source is one contribution to a merged response: an already-fetched (or
// cache-loaded) per-source view. An empty FetchErr marks a successful read.
type Source struct {
	// Name namespaces this source's checks as "name/check". Callers enforce
	// the non-empty, unique, slash-free contract at construction; Responses
	// relies on it for collision-free keying.
	Name string
	// Response is the source's final per-source view, with TotalLatencyMs as
	// served. Ignored entirely when FetchErr is non-empty.
	Response health.Response
	// FetchErr carries the cause of an unsuccessful acquisition (the
	// federation unreachable-remote case; aggregate reads trusted in-process
	// probes and never sets it). Non-empty: the source contributes one
	// synthetic "name/reachable" fail check and its Response is ignored, so
	// a dark remote is visible instead of silently frozen.
	FetchErr string
}

// Responses folds the sources into one merged response. The overall status
// is the worst source status by [health.Status.Rank]; rank ties (off, any
// unknown status) never replace the accumulated severity, so the merged
// roll-up stays within the frozen pass/warn/fail enum regardless of source
// order. Any source shutting down forces the merged fail. Every check key is
// namespaced "source/check" and carries the source's own check value,
// including Since and DurationNanos. TotalLatencyMs is the maximum across
// successful sources. Per-process scalars (Version, Uptime, InstanceID,
// Timestamp) never survive: they are per-process and would lie in a merged
// view. Folding zero sources yields the pass response with no checks; the
// aggregate and federation constructors reject empty source sets themselves.
func Responses(sources []Source) health.Response {
	checks := make(map[string]health.Check)
	status := health.StatusPass
	shuttingDown := false

	var maxLatency int64

	for _, src := range sources {
		if src.FetchErr != "" {
			checks[src.Name+"/reachable"] = health.Check{
				Status: health.StatusFail,
				Error:  "fetch: " + src.FetchErr,
			}
			status = worst(status, health.StatusFail)

			continue
		}

		if src.Response.ShuttingDown {
			shuttingDown = true
		}

		if src.Response.TotalLatencyMs > maxLatency {
			maxLatency = src.Response.TotalLatencyMs
		}

		status = worst(status, src.Response.Status)

		for name, check := range src.Response.Checks {
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

// worst returns the more severe of two statuses by [health.Status.Rank]:
// lower rank wins. Ties keep the accumulated status, so off and unknown
// statuses (which rank pass-tier) can never demote a pass accumulator nor
// flatten a warn or fail one — the property that keeps the fold
// order-independent and the merged roll-up inside the frozen
// pass/warn/fail enum.
func worst(a, b health.Status) health.Status {
	if a.Rank() <= b.Rank() {
		return a
	}

	return b
}
