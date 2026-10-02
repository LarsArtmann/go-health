# Synthetic check overlays: the escape hatch (dnsblockd pattern)

**Date:** 2026-10-02 · **Status:** DOCUMENTED AS INTENDED — no API change.

## The bypass

dnsblockd (`internal/server/health.go`, `degradeStaleHealth`) constructs
`health.Check` values **by hand** and injects them into a cloned
`CachedResponse` before serving: when the cached response is older than 15s
(a wedged background evaluation), the endpoint flips to fail with a
synthetic `database` check carrying `Since: now` and `DurationNanos: 0`.

This bypasses the probe-observed `transitionTracker` (the synthetic `Since`
is caller-observed, not probe-observed) and writes into a map the probe
considers immutable. It is nevertheless **correct composition**, because:

1. The overlay expresses a fact the probe genuinely cannot know — that the
   evaluation pipeline itself is wedged. No probe-side mechanism can
   report its own staleness honestly (the reporter is the broken part).
2. The caller clones the Checks map first (`maps.Copy`) — the shallow copy
   from `CachedResponse` aliases the live cached map, and handlers marshal
   it concurrently. **The clone is the load-bearing line**; writing the
   returned response's map directly is a concurrent-map-write crash.
3. `DurationNanos: 0` is honest (no execution ran) and omitted from JSON.

## Rules for anyone composing overlays

1. **Clone before write** — `CachedResponse()` returns a shallow copy; its
   `Checks` map aliases probe state.
2. **Own your `Since`** — a synthetic check's `Since` is the moment you
   observed the condition, not the probe's transition time; do not fake
   tracker-consistent values.
3. **Fail loudly, name plainly** — surface the synthetic verdict under a
   real check name with a descriptive error, so monitors see the specific
   wedge (dnsblockd reuses `database`; a dedicated
   `evaluation-stale`-style name composes better with
   `ErrUnknownCriticalService` expectations if listed critical).
4. **Serve, don't store** — overlays belong in the serving path (handler
   composition), never written back into the probe's cache.

## Library verdict

No API change: the pattern needs no support the library lacks, and a
`WithStalenessGuard` option would guess at policy (per-consumer staleness
bounds differ: dnsblockd's 15s is tuned to its 1s refresh + 5s timeout).
The cookbook (docs/system-checks-cookbook.md) is the discoverability
surface for this document.
