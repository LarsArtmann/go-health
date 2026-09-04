# Per-Source Roll-up Visibility — design note

> Decided: 2026-09-04 · Status: DEFERRED to v0.2.0 (additive accessor candidate) · Resolves: open idea "aggregate observability / per-source roll-up"

## Problem

The merged view answers "is the fleet healthy?" (`status` roll-up) and
"which check failed?" (`source/check` keys), but not the middle question a
dashboard always asks next: **"is source `api` healthy overall?"** Today a
consumer must fold the merged checks by the prefix before the first `/` —
reimplementing the worst-of fold the package already owns.

## Options considered

1. **Labels in keys** (`api[status=warn]/db`) — rejected: mutates the check
   key format. The wire format is frozen; every consumer, golden file, and
   grouping rule would change for a cosmetic gain.
2. **Per-source pseudo-checks** (synthetic `source/api` check entry) —
   rejected: injects phantom checks into the served body. Dashboards and
   alerting would see N extra entries that are not real checks; it is also a
   wire-format change (golden file + changelog callout required).
3. **Programmatic map accessor** — chosen shape: an additive method, e.g.

   ```go
   // SourceStatuses folds each source's cached view to its roll-up status.
   // One atomic load per source, no evaluation, no wire impact.
   func (a *Aggregate) SourceStatuses() map[string]health.Status
   ```

   Zero wire change, composable (CLI can print a table, a dashboard can
   badge per source), and the fold logic stays owned by the package.

## Why deferred

The accessor is purely additive and could ship in a patch, but nothing
currently demands it — the only known consumer (go-health-dashboard) folds
prefixes itself today. Shipping it as a v0.2.0 headline (together with the
`errors.Join` construction errors) keeps each release purposeful. If a
second consumer appears earlier, promote it.

## Contract check

Does not conflict with the three-probe model or the merge-on-read design:
it is a read-side convenience over state that already exists per source, so
it introduces no locks, no goroutines, and no freshness semantics of its
own.
