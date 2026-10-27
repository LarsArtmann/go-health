# Cache freshness primitives: `CacheAge`, `CachedResponseFresh`, `EvaluateAndCache`

**Date:** 2026-10-27 · **Status:** IMPLEMENTED (additive, unreleased)

## Problem

Five fleet repos hand-roll the same policy around `Probe.CachedResponse`
(evidence: `docs/research/2026-10-27_fleet-consumer-needs.md` §N1/N2):

- DiscordSync `cmd/discordsync/health_readyz_cache.go` — age-aware adapter
  over the probe cache with a live-sweep fallback.
- dnsblockd `internal/server/health.go` — zero-timestamp fallback plus a
  stale-degradation overlay.
- cv `internal/health/handlers.go` — empty-cache fallback, then a projected
  copy.
- go-taskqueue `internal/webui/health.go` — a complete replacement `Prober`
  that re-evaluates on read.

They agree on the mechanics and disagree only on the staleness bound. The
library could not express either half:

1. `CachedResponse()` cannot distinguish "fresh pass" from "never evaluated"
   without inspecting `Timestamp` (an internal convention, not a contract).
2. `Evaluate()` deliberately does not publish, so any fallback built on it
   answers correctly once and then keeps serving the older snapshot.

## Decision

Three additive members; no policy, no new option, no default change.

| Member                                               | Contract                                                                                                                                                      |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Probe.CacheAge() (time.Duration, bool)`             | Age of the published snapshot on the probe's injected clock; `false` when none exists (or the clock stamped the zero time). Never evaluates, never allocates. |
| `Probe.CachedResponseFresh(maxAge) (Response, bool)` | The cached view (with the live shutdown overlay) when `0 < age <= maxAge`; otherwise zero + `false`. Non-positive `maxAge` never reports fresh.               |
| `Probe.EvaluateAndCache(ctx) Response`               | One batch bounded by `WithTimeout` (or the caller's earlier deadline), published to the served snapshot, returned.                                            |

The intended consumer shape:

```go
resp, ok := probe.CachedResponseFresh(2 * probe.RefreshInterval())
if !ok {
    resp = probe.EvaluateAndCache(ctx)
}
```

## Why not a `WithStaleAfter` / `WithStalenessGuard` option

`docs/synthetic-check-overlay.md` already rejected a policy option: the
bounds differ per consumer (dnsblockd 15s against a 1s refresh + 5s timeout;
DiscordSync 2× its interval), and the degradation verdict is domain-specific
(dnsblockd rewrites a real check to fail; cv folds a synthetic row with its
own off→warn rule). A primitive composes; a policy guesses. The overlay
design's "no API change" verdict is now superseded only on the _mechanics_
half — nothing here decides what staleness means.

## Semantics pinned

- The age uses the same injected clock that stamps `Response.Timestamp`, so
  `WithNowFunc` gives deterministic tests (no sleeps).
- A backwards clock step clamps to zero rather than reporting a negative age.
- The shutdown overlay still applies to a fresh snapshot: freshness and
  drain are independent axes.
- Publishing in `EvaluateAndCache` races a refresh tick the same way two
  ticks would — both are truthful evaluations and the last store wins; the
  transition tracker already serializes `Since` stamping.

## Tests

`probe_cache_test.go`: no-snapshot, published-age with a fixed clock, stale
vs fresh windows, non-positive window, zero timestamp, backwards clock,
shutdown overlay, publish-vs-not-publish parity, and `Response.Clone`
independence.
