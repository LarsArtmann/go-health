# Design: unified merge for `aggregate` and `federation`

**Date:** 2026-10-02 · **Status:** DESIGN ONLY — re-venued 2026-10-08 to the v0.6 window (v0.5.0 shipped without it; body retains its original v0.5 framing). No code changes now. See TODO_LIST "v0.6 window — staging".

## Problem (P9)

`aggregate.CachedResponse` (merge-on-read across in-process probes) and
`federation.CachedResponse` (merge-on-read across HTTP remotes) implement
the same merge semantics twice: worst-of status via `Status.Rank`,
`"source/check"` namespacing, shutdown overlay, max-latency, scalars dropped.
Duplicated rules drift independently — a wire-contract fix must be made
twice and can silently diverge (the classic split brain).

## Shared invariants (extracted from both implementations)

1. One source = one `health.Response` read (atomic / one HTTP fetch).
2. Roll-up = worst source status (`Rank`: fail > warn > pass).
3. Check names namespace as `<source>/<check>`; collision-free by the
   no-slash source-name contract (ADR-005).
4. Shutdown overlays: any source draining → merged fail; source startup
   latches AND.
5. Scalars (`Version`, `Uptime`, `InstanceID`, `Timestamp`) never survive;
   `TotalLatencyMs` = max across sources.
6. Any source appearing unhealthy/unreachable surfaces as a synthetic
   `<source>/reachable` FAIL check — never silent, never a status override
   (federation's wire rule).

## Proposed core API (internal, unexported)

```go
// internal merge primitive used by both packages
type mergeSource struct {
    Name      string
    Response  health.Response
    Reachable bool          // federation: fetch failed → synthetic check
    Draining  bool
    Started   bool          // startup latch
}

func mergeResponses(sources []mergeSource) health.Response
```

- `aggregate` maps its sources' cached loads onto `mergeSource` (always
  Reachable, Draining from probe state, Started from latch).
- `federation` maps fetch outcomes onto it (fetch error → Reachable=false
  with the cause carried as the synthetic check error).
- Property tests currently duplicated across both packages
  (`FuzzAggregateMergeInvariants`, federation fuzz) collapse onto one suite
  over `mergeResponses`, plus thin per-package tests for the mapping.

## Migration plan

1. Extract `mergeResponses` (unexported) into the root package with an
   `export_test` seam or an `internal/` subpackage; port aggregate first
   (simpler), then federation.
2. Golden-check: for a fixed corpus of source responses, the merged output
   of old and new code must be byte-identical (both packages' existing
   golden/property tests serve as the oracle).
3. No public API change; no wire change. Shippable inside v0.5.x without a
   major bump — schedule with E1 to share the release train.

## Risks

- Federation's per-remote latch movement on _successful fetch_ is
  stateful and stays federation-side; only the pure merge is unified.
- Latency accounting differs subtly (aggregate: max of cached;
  federation: max including fetch time) — `mergeSource` must carry the
  final `TotalLatencyMs` per source, not raw reads.
