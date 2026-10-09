# Design: unified merge for `aggregate` and `federation`

**Date:** 2026-10-02 · **Status:** DESIGN — implementation-ready sketch (2026-10-09 re-grounding below); v0.6 window vehicle. No code changes yet. See TODO_LIST "v0.6 window — staging".

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

## Proposed core API

**Placement (resolved 2026-10-09):** an `internal/merge` subpackage. The
original "unexported in the root package" option cannot work:
`aggregate` and `federation` are separate packages and cannot import
unexported root symbols — only an `internal/` package (or an exported root
API, rejected as premature surface) is importable by both while staying
closed to consumers. The flat-library linter excludes already cover
subpackage roots (`aggregate/`, `federation/`, `checks/`); `internal/merge`
joins them.

```go
// internal/merge — the shared worst-of fold for both merge-on-read packages.
//
// A Source is one already-fetched (or cache-loaded) contribution. Callers
// own acquisition: aggregate loads atomics, federation performs parallel
// HTTP fetches. The primitive is pure — no locks, no goroutines, no state.
type Source struct {
	Name     string
	Response health.Response // final per-source view; TotalLatencyMs as served
	FetchErr string          // non-empty → synthetic "<name>/reachable" FAIL check; Response otherwise ignored
}

func Responses(sources []Source) health.Response
```

- `aggregate.CachedResponse` maps its atomic loads onto `merge.Source`
  (FetchErr always empty) and returns `merge.Responses(...)`.
- `federation.(*Prober).merge` maps fetch outcomes (fetch error → FetchErr
  carrying the cause) — but keeps the per-remote latch advancement
  (`state.latched.Store(true)` on successful fetch) federation-side,
  before calling the primitive. Latches are stateful outputs of a
  successful fetch, not merge inputs; `Started` is therefore absent from
  `merge.Source` (correcting the original sketch, which modeled it as an
  input).
- Property/fuzz suites duplicated across both packages
  (`FuzzAggregateMergeInvariants`, federation fuzz) collapse onto one suite
  over `merge.Responses`, plus thin per-package tests for the mapping.
- `Aggregate.SourceStatuses` (added post-v0.5.1) reuses the same worst-of
  and shutdown-overlay rules per source but does not namespace checks or
  drop scalars; it stays aggregate-side. If federation later grows the
  same accessor, fold both onto a per-source variant of the primitive
  rather than re-implementing the overlay a third time.

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
