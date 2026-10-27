# Federation per-remote roll-up: `Prober.SourceStatuses` (issue #6)

**Date:** 2026-10-27 · **Status:** IMPLEMENTED (additive, unreleased)

## Problem

`aggregate.Aggregate` gained `SourceStatuses()` in v0.6.0 so a view can ask
"is source `api` healthy overall?" without refolding `api/*` keys. The merge
unification (v0.7.0) left federation the odd one out.

Measured cost (go-health#6, dashboard v0.7.0 commit `1473fc2`): a remote
that reports all-pass rows while its document carries `shutting_down: true`
contributes its rows unchanged and forces the merged roll-up to fail. A
per-remote card folded from rows therefore renders green under a red page,
and no accessor exists to correct it. Unreachable remotes are already honest
through the synthetic `name/reachable` fail row; the gap is precisely the
drain case (and any future per-source state that rides the roll-up without a
row).

## Decision

Add `(*Prober).SourceStatuses() map[string]health.Status`, mirroring
`aggregate.Aggregate.SourceStatuses` exactly in signature and semantics, so
the dashboard's optional `SourceStatuser` capability
(`SourceStatuses() map[string]health.Status`) covers both merge surfaces.

| Case | Reported status |
| --- | --- |
| Fetch succeeded, remote healthy | the remote document's `status` |
| Fetch succeeded, remote `shutting_down` | `fail` (mirrors its contribution to the merged fold) |
| Fetch failed (transport, non-200, oversized/undecodable, missing status) | `fail` (mirrors its `name/reachable` row) |
| Hub marked via `MarkShuttingDown` | **no change** — the drain is the hub's, not any remote's |

## Cost, stated plainly

Federation is merge-on-read: `SourceStatuses` performs the same one-parallel-
fetch-per-remote pass as `CachedResponse`. Calling both costs two passes.
That is the same contract the package doc already states for reads, so no new
caching layer is introduced (a cache would break the "freshness is live"
property that makes a dark remote visible immediately).

## Startup latches are not advanced

`merge` advances each remote's one-way startup latch on a successful fetch;
`SourceStatuses` deliberately does not. Asking a dashboard question must not
satisfy an unrelated startup probe (pinned by
`TestSourceStatuses_DoesNotAdvanceStartupLatch`).

## Implementation note

The parallel fetch loop moved into `fetchAll(ctx)`, shared by
`cachedResponse` and `SourceStatuses`. It was already carrying the remote's
state pointer in each result (`fetchedRemote`) so no cross-slice index
invariant exists; the extraction preserves that property and keeps latch
movement in `merge`, its only legitimate owner.

## Tests

`federation/federation_source_status_test.go`:
per-remote verdicts (healthy/degraded/draining/dark), merged-worst-of
agreement, no latch movement, and hub-drain isolation.
