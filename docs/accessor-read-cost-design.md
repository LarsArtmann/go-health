# Accessor read cost: `CachedResponse` and `SourceStatuses` are two passes

**Date:** 2026-10-27 · **Status:** DECIDED (documented cost, no merged accessor)
**Milestone:** T23 · **Code:** `federation.SourceStatuses`,
`aggregate.Aggregate.SourceStatuses`, `internal/merge`
**Benchmark:** `BenchmarkMergedReadAndStatuses` (`federation/federation_benchmark_test.go`)

## The question

Both merge-on-read packages expose two read accessors:

| Accessor           | Answers                                            | Also does                                        |
| ------------------ | -------------------------------------------------- | ------------------------------------------------ |
| `CachedResponse()` | the merged document (roll-up + namespaced checks)  | advances per-remote/per-source startup latches   |
| `SourceStatuses()` | one roll-up status per source, no namespacing      | deliberately does **not** move any latch         |

A dashboard asks both questions on one page, which means it calls both — and
each performs its own read pass. In `federation` a pass is one HTTP fetch per
remote; in `aggregate` it is one atomic load per source. The godoc has said
"calling both costs two passes" since v0.8.0; this record says how much that is
and why it stays.

## Measurement

`BenchmarkMergedReadAndStatuses`, 4 remotes over loopback `httptest` upstreams,
`-benchmem -count=3`, 2026-10-27:

| Shape           | ns/op (3 runs)             | B/op (3 runs)          | allocs/op |
| --------------- | -------------------------- | ---------------------- | --------- |
| `merged_only`   | 660 k, 1006 k, 1048 k      | 34230, 34481, 34834    | 342–344   |
| `statuses_only` | 501 k, 520 k, 753 k        | 33572, 34663, 36759    | 341–348   |
| `both`          | 806 k, 881 k, 1053 k       | 72021, 72589, 73763    | 692–696   |

Two facts matter more than the ns column (this run shared a heavily loaded host,
so wall time is an upper bound and only comparable within the run):

1. **Bytes and allocations scale exactly 2×** — 34 kB/343 allocs for either
   alone, 72 kB/693 for both. The second pass is not free; it is a second
   identical pass.
2. **The cost is the fetch, not the fold.** `statuses_only` allocates the same
   as `merged_only` despite building a map instead of a merged document, so the
   per-source HTTP exchange dominates both. On loopback each remote costs
   roughly 125 kB/op of *work* per 4-remote pass; a real remote costs a network
   round trip.

## Options considered

| Option                                                                | Verdict                                                                                                                                                                                                                                              |
| --------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| One combined accessor, e.g. `Snapshot() (Response, map[string]Status)` | **Rejected.** It would make the latch asymmetry (`SourceStatuses` must not advance latches) a mode flag on one API, and it adds a new public type for a consumer population of one. Merging two passes into one also couples the two folds, which are separate today on purpose. |
| Memoize both reads behind a short window                              | **Rejected.** A cache of a cache: `aggregate` has no clock of its own and `federation` would start serving one request's remote answers to the next — a freshness lie in exchange for a 2× saving on a page a human loads once per refresh.       |
| Make `SourceStatuses` derive from the merged document                 | **Rejected for `federation`.** Deriving a remote's roll-up from `name/*` keys cannot distinguish "remote unreachable" from "remote reported fail" without the fetch outcome, and it re-folds keys the accessor exists to avoid. Works for callers, not for the package. |
| Leave both, document the cost, provide the one-pass workaround        | **Chosen.**                                                                                                                                                                                                                                            |

## Decision

Keep both accessors as separate passes. Record the cost here and at both call
sites, and point callers who need both at the cheaper single-pass shape:

```go
resp := fed.CachedResponse()          // one pass; also advances latches
// per-remote verdicts without a second fetch: the merged document already
// namespaces every remote's checks, so a roll-up can be folded from
// "name/" keys when the consumer only needs a coarse label.
```

## Revisit conditions

Add a combined accessor only when **both** hold:

1. a consumer demonstrates that the second pass is material (not loopback), and
2. the combined API can express the latch asymmetry without a boolean mode.

Until then the second pass is a documented, measured, caller-controlled cost —
not a defect, and not a place for a silent cache.
