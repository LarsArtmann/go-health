# Design: per-check severity override (informational checks)

**Date:** 2026-11-04 · **Status:** PROPOSED — demand-gated, rides the next minor with an additive wire change (no implementation shipped yet)
**Related:** [docs/classification-2.0-design.md](classification-2.0-design.md) (weights rejected), [docs/configured-off-design.md](configured-off-design.md) (off = intentional absence), [docs/start-validation-design.md](start-validation-design.md) (batch-based name validation), [docs/merge-unification-design.md](merge-unification-design.md) (the shared worst-of fold)

## Problem

The classify model is two-tier: a check is critical (failure ⇒ roll-up
`fail`, readiness 503) or non-critical (failure ⇒ `warn`, readiness 200).
PMA (projects-management-automation) and go-taskqueue both carry a class of
dependency that fits neither tier: **configured, evaluated, allowed to fail
without moving anything but the row itself** — a webhook relay, an optional
analytics sink, a background sync partner. Today their choices are:

- register the check **critical** → a blip pages someone (503) — too loud;
- register it **non-critical** → every prolonged outage pins the roll-up at
  `warn` forever, training operators to ignore the warn state entirely —
  this is the ask behind the 2026-10-27 fleet-needs sweep (plan M22.2);
- **hide the check** (return `nil` on soft failure) → the row disappears;
  the failure loses its only witness;
- mark it `Off(detail)` → renders pass-tier and visible, but lies about the
  domain: `Off` is specified as _intentionally unconfigured_, and the whole
  configured-off rollout (federation consumers refuse off documents from
  producers that predate it) rests on that contract.

`ROADMAP.md`'s `WithTransitionHook` idea is a different item: it fires on
status _changes_ for alerting; it does not change what the roll-up computes.

## The distinction this note turns on

`Off` answers "is this dependency supposed to exist?" — **no**. The fleet
ask is "does this dependency's failure own a slice of the verdict?" — **no,
but it exists and must stay visible**. Those are different questions; the
first already has a sentinel, the second has nothing. Overloading `Off`
with the second would corrupt the first (every `off` row would become
ambiguous between "not configured" and "configured but soft"), and the
federation producer/consumer rollout order depends on `off` meaning exactly
one thing.

## Proposal: a third contribution tier, not a fourth Status

The roll-up enum stays frozen at three values (docs/starting-status-design.md
stands). What changes is which check rows **contribute** to the fold:

```go
// WithInformational marks named checks as informational: they are
// evaluated, rendered with their true verdict, and excluded from the
// roll-up fold and the startup latch — the tier a configured-but-soft
// dependency needs. Mutually exclusive with the critical set.
func WithInformational(names ...string) Option
```

- **Evaluation:** unchanged. The check still runs in every batch; a failure
  is still a failure.
- **Roll-up:** an informational row is excluded from `classify`'s worst-of
  fold — the same structural treatment `OffError` rows already get
  (classifier.go), so the fold's shape does not change, only its input set.
- **Rendering:** the row keeps its true verdict (`status: "fail"` with the
  real `error`), and gains one additive field `"severity":"informational"`
  (`omitzero`; absent for normal rows). The field is what keeps the row
  honest: a reader seeing `pass` next to a `fail` row can tell the row was
  deliberately excluded, not that a bug dropped it. Dashboards that never
  learn the field see today's behavior plus one extra key.
- **Startup latch:** informational checks neither satisfy nor block the
  latch (mirroring off: `evaluateStartup` skips them). A dependency too soft
  to fail readiness is too soft to gate boot.
- **Panic exception:** a recovered panic (batch-level or per-check) grades
  `fail` and contributes to the roll-up **even for informational checks** —
  the fail-closed panic rule (issue #7 finding 1) is a statement about
  process health, not dependency criticality, and must not be overridable by
  configuration.
- **Since/Duration:** unaffected; informational rows are normal rows in
  every other respect.

### Why not the general `WithCheckSeverity(name, tier)`?

A three-tier `tier` parameter (critical / non-critical / informational)
would subsume `WithCriticalServices`. It is deliberately not proposed: the
critical tier already has a validated, documented, fleet-wide spelling
(`WithCriticalServices` + `WithCriticalService[T]` + `DoServiceName[T]` plus
`ErrUnknownCriticalService`), and a second spelling for the same tier is
drift surface, not flexibility. One new option covering the one tier that
does not exist; the tier model stays binary-plus-exclusions.

## Alternatives rejected

1. **Weights** (classification-2.0 §1) — rejected 2026-09-04 and still
   rejected: this proposal adds no score function and no thresholds; the
   roll-up remains worst-of-contributing-tiers.
2. **Consumer-side composition** — wrap the check fn and return
   `health.Off("…: " + err.Error())` on failure. Works today with zero API,
   and is the right _stopgap_, but it (a) lies with the `Off` contract, (b)
   muddies the configured-off rollout order, and (c) cannot demote a check
   the _option surface_ declares critical. The design exists so consumers
   can stop doing this.
3. **Post-classify hook rewriting** — `WithEvaluationHook` receives a
   defensive copy and cannot change the served response (by design,
   2026-10-09); "fix it in the hook" would need a mutable-response seam,
   which is a larger and riskier surface than one fold exclusion.
4. **Per-check HTTP-status mapping** — the kubelet consumes status codes,
   not per-check JSON; any knob must act on the roll-up to be visible at
   every consumer boundary.

## Wire and compatibility

- `"severity":"informational"` is additive; `encoding/json` and
  `encoding/json/v2` both render it, `omitzero` keeps it off normal rows.
  The golden wire-format test gets one new fixture; `docs/openapi.yaml`
  gains the optional field (the lockstep gate covers it).
- `internal/merge` gains one rule: informational rows stay excluded from the
  fold after a merge (a merge of excluded rows must not resurrect them as
  contributors). `FuzzResponsesMergeInvariants` and the aggregate/federation
  mapping harnesses get the corresponding invariant; the aggregate merge is
  the trickier half because aggregate sources are separate probes whose
  _own_ roll-ups already absorbed the exclusion — the merged body must not
  re-fail because a namespaced informational row looks worse than the
  source's verdict.
- **Rollout order (federation lesson applied):** ship the field in a minor;
  _consumers that render checks_ (dashboard, hub) learn to expect it before
  any producer turns a check informational. Old consumers ignore the field
  and are merely surprised by the row/roll-up pairing — acceptable because
  the roll-up they compute is unchanged on their side.

## Validation surface

`Start()` extends `validateCriticalNames`'s batch-based check: a name in
both the critical set and the informational set is a boot-time error (the
tiers are mutually exclusive by definition), and — same batch-based logic as
critical names — an informational name that never ran is _not_ an error
(informational never influences the latch, so a phantom name is inert by
construction; documenting that difference is part of the API).

## Testing plan (when implemented)

- `classifier` table extension: informational rows excluded from fold,
  latch, and upgrade path; panic rows still fail (the exception).
- Golden-file fixture with one informational row.
- `Start` validation: tier conflict error; phantom informational name inert.
- Merge invariants: excluded rows stay excluded across root/aggregate
  federation merges; fuzz harnesses extended in the same commit as the
  invariant change.

## Decision request

Owner call, gated on a named adopter moving first (PMA or go-taskqueue
migrating off their stopgap within a release of the option shipping). The
API is one option, one wire field, one fold rule — sized to the ask, and
shaped so that declining it costs only the stopgap's continued use.
