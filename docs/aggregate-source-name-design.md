# Aggregate Source-Name Contract — "/" rejected, check names lenient — design note

> Decided: 2026-09-04 · Status: ADOPTED (strict source names, lenient check names) · Resolves: open fuzz follow-up "slash-name aliasing hazard"

## Problem

`aggregate.CachedResponse` namespaces every check as `"source/check"`
(aggregate.go). The package doc claims two invariants that fall out of that
format:

1. **Collision-free keys** — two sources can never produce the same check key.
2. **Stable grouping axis** — the part before the *first* `/` is the source
   name, giving dashboards a way to group merged checks.

Both claims are only true if source names never contain `/`. The original
release accepted any name, leaving a silent aliasing hazard:

```go
// Both produce the merged key "api/db/pool":
aggregate.New(
    aggregate.Source{Name: "api/db", Probe: probeA}, // check "pool"  → "api/db/pool"
    aggregate.Source{Name: "api",   Probe: probeB},  // check "db/pool" → "api/db/pool"
)
// Last write wins. One source's health state silently disappears, and a
// consumer splitting on the first "/" attributes both to a source "api"
// that does not exist.
```

## Options

1. **Strict: reject `/` in `Source.Name` at construction** — makes both
   documented invariants provable instead of aspirational. Cost: a source
   name that worked before now fails `New` with a wrapped
   `ErrInvalidSource`.
2. **Lenient: document the hazard, validate nothing** — zero behavior
   change. Cost: the doc's "collision-free" claim stays false, the failure
   mode is a silent wrong health answer (the worst kind), and every consumer
   must defend itself.

## Decision: strict on source names, lenient on check names

`aggregate.New` now rejects source names containing `/` with
`fmt.Errorf("%w: source name %q must not contain '/' ...", ErrInvalidSource, name)`
(TestNew_SlashNameContract). That single rule is sufficient to restore both
invariants:

- **Collision-free:** merged keys are `"srcName/..."` with unique, slash-free
  `srcName`, so one source's keys can never land in another source's
  namespace. Within a source, check names come from a map and are unique.
- **Grouping axis:** the first `/` in a merged key always separates the true
  source name from the check.

**Check names remain lenient** — `"pool/primary"` under source `"api"` merges
to `"api/pool/primary"` and violates neither invariant. Banning slashes there
would break real check names (samber/do service names, connection strings)
for zero correctness gain.

## Why not a dedicated sentinel?

`ErrInvalidSource` already covers every per-source construction fault (nil
probe, empty, duplicate, slash). A separate `ErrInvalidSourceName` would add
API surface without a distinct remediation path — the fix in all cases is
"correct the `Source` config", and the wrapped message names the offending
value.

## Migration note

Any consumer that (deliberately or accidentally) used `/` inside a source
name now gets an error from `aggregate.New` at startup — fail-fast, before
the first request. Rename the source (e.g. `api/db` → `api.db`); the merged
keys change accordingly, which is the point: the old keys were ambiguous.

## Enforcement

- Unit: `TestNew_SlashNameContract` (rejection + lenient-check-name pin),
  `TestNew_RejectsInvalidSources/name_containing_slash`.
- Fuzz: `FuzzAggregateMergeInvariants` skips slash names in
  `buildFuzzSources` — they are out of contract by construction now; the
  merge invariants themselves are fuzzed under valid topologies only.
