# ADR-005: Aggregate source names reject "/"

> Status: ACCEPTED · Date: 2026-09-18 · Deciders: maintainer
> Promotes [docs/aggregate-source-name-design.md](../aggregate-source-name-design.md)
> into the formal ADR series. The decision itself dates to 2026-09-04 and
> shipped in v0.1.3; this record exists so the ADR set is the single index of
> accepted architecture decisions.

## Context

`aggregate.CachedResponse` namespaces every check key as `"source/check"`.
Two invariants are claimed for that format, and both depend on source names
never containing `/`:

1. **Collision-free keys** — two sources can never produce the same key.
2. **Stable grouping axis** — everything before the *first* `/` is the source
   name, giving dashboards a way to attribute checks.

Before the strict rule, an aliasing hazard was silent and produced a wrong
health answer:

```go
// Both produce the merged key "api/db/pool":
aggregate.New(
    aggregate.Source{Name: "api/db", Probe: probeA}, // check "pool"    → "api/db/pool"
    aggregate.Source{Name: "api",   Probe: probeB},  // check "db/pool" → "api/db/pool"
)
// Last write wins: one source's state disappears, and a consumer splitting
// on the first "/" attributes both to a source "api" that does not exist.
```

## Decision

`aggregate.New` rejects any `Source.Name` that is empty, duplicated, contains
`/`, or has a nil `Probe`, wrapping `ErrInvalidSource` with the offending
value. Check names stay lenient: `"pool/primary"` under source `"api"` merges
to `"api/pool/primary"` and violates neither invariant. Banning slashes in
check names would reject real samber/do service names for zero correctness
gain.

One rule (`srcName` unique and slash-free) is sufficient: merged keys are
`"srcName/..."`, so one source's keys can never land in another's namespace,
and the first `/` always separates the true source name from the check.

## Consequences

- The package doc's two invariants become provable at construction instead of
  aspirational at read time.
- A source name that used `/` now fails `New` at startup — fail-fast, before
  the first request. Rename `api/db` → `api.db`; the merged keys change
  accordingly, which is the point: the old keys were ambiguous.
- No wire-format change: `"source/check"` was always the format.

## Alternatives rejected

- **Document the hazard, validate nothing.** Zero behavior change, but keeps
  the "collision-free" claim false and leaves a silent wrong-health failure
  mode — the worst kind — for every consumer to defend against.
- **A dedicated `ErrInvalidSourceName` sentinel.** `ErrInvalidSource` already
  covers every per-source construction fault (nil, empty, duplicate, slash);
  a new sentinel adds API surface with no distinct remediation path (the fix
  is always "correct the `Source` config").
- **Escape or encode `/` in names.** Changes the key format for everyone and
  makes the grouping axis harder to read for a problem construction can
  simply refuse.

## Enforcement

- Unit: `TestNew_SlashNameContract`, `TestNew_RejectsInvalidSources`
  (`name_containing_slash`).
- Property: `aggregate_property_test.go` asserts the merged checks map is the
  namespaced union of the sources' checks — disjoint and complete — for every
  source topology it enumerates.
- Fuzz: `FuzzAggregateMergeInvariants` skips slash names in
  `buildFuzzSources`; they are out of contract by construction.
