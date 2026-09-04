# errors.Join for aggregate.New — design note

> Decided: 2026-09-04 · Status: DEFERRED to v0.2.0 (spike verified) · Resolves: open idea "better construction errors"

## Problem

`aggregate.New` fails fast: it returns on the **first** invalid source. An
aggregate over ten sources with three problems costs three fix-run cycles
before construction succeeds. `errors.Join` (go1.20+) could report all
problems at once.

## Spike results (verified 2026-09-04)

Joining wrapped sentinels —
`errors.Join(fmt.Errorf("%w: ...", ErrInvalidSource, ...), ...)`:

1. **Shape:** the message is the per-error messages joined with `\n` — each
   line self-contained, prefixed by the sentinel text (`aggregate: invalid
   source: ...`). Multi-line, log-friendly, readable in a startup failure.
2. **errors.Is:** `errors.Is(joined, ErrInvalidSource)` → true. Every
   existing caller matching the sentinel keeps working; only literal
   `err.Error() ==` matching (which nothing should do) would notice.
3. **Cost:** stdlib, zero allocations beyond the joined tree; no new
   dependency (the project is deliberately stdlib-errors — see ADR-001).

## Decision: defer to v0.2.0

The change is small, but it is a **behavior change**, not a refactor:

- Construction stops at the first invalid source today; after, it returns
  everything. Better UX, but a different failure mode that a consumer could
  (legally) observe.
- Error *messages* become multi-line. Any caller formatting construction
  errors into single-line logs/panic messages would see wrapping change.

Both belong in a minor release with a changelog callout, not a patch-window
drive-by. v0.2.0 candidate:

> `aggregate.New` validates all sources and returns a joined error listing
> every problem (one line each), instead of failing on the first. `errors.Is`
> against `ErrInvalidSource` / `ErrNoSources` is unchanged.
