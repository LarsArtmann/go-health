# Configured-Off Checks (`Off` / `StatusOff`) — design note

> Decided: 2026-10-08 · Status: ACCEPTED · Extends the Status vocabulary at
> the CHECK level only; `Response.Status` stays three-state (ADR-003,
> [starting-status-design.md](starting-status-design.md))

## Problem

Fleet services carry optional dependencies whose absence is a deliberate
configuration, not a fault — an optional analytics database, a disabled chat
provider. Two real cases shaped this design:

1. **CV** synthesizes an explicit `database` row _outside_ the probe: the
   projection layer injects a hand-built `warn` check with the detail
   "intentional absence — analytics features disabled" and rolls it into the
   overall verdict. The row is honest but the vocabulary is wrong — nothing
   is degraded — and the fleet hub (health.home.lan) therefore reports the
   service `warn` forever, training operators to ignore the aggregate.
2. **DiscordSync** went the other way: an unconfigured Turso sync handle is
   never registered, so the health surface is silently green and the
   capability gap is visible only by noticing a missing dashboard card.

The library had no way to say "deliberately not configured": checks are
graded pass/warn/fail from errors, and the Status enum was frozen.

## Relationship to the "starting" rejection

[starting-status-design.md](starting-status-design.md) rejected a fourth
Status value (`starting`) and froze the enum. That rejection was about a
_response-level transient boot state_; `off` differs on every argument:

| `starting` argument                                 | Why it does not reject `off`                                                                                                                                                                                         |
| --------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Kubernetes never reads the body                     | Same — which is exactly why off must ride a 200. It does.                                                                                                                                                            |
| Nonstandard vocabulary; every consumer needs a case | Applies, but the cost is one enum case at _check_ level in consumers that render rows (the hub already has an unknown-status fallback), and the alternative (CV's prose-warn) costs more: it corrupts the aggregate. |
| Roll-up ambiguity (`starting` vs `fail` on /readyz) | Does not apply: off is a per-CHECK annotation; `Response.Status` remains three-state, so there is exactly one roll-up vocabulary.                                                                                    |
| "A status that can regress is a mood"               | Off is config-stable per process, not transient.                                                                                                                                                                     |
| No do-conformance path                              | Does not apply: off rides the existing error channel — `HealthCheck(ctx) error` returning `Off(...)` is fully expressible.                                                                                           |

The frozen-enum consequence is narrowed accordingly: the _roll-up_ enum
(pass/warn/fail) is still frozen and `Response.Status` will never report
`off`. The check-level vocabulary grows by one value.

## Semantics

**Production.** `Off(detail string) *OffError` is a sentinel error. Any
executor channel can return it: a `CheckFunc` ([NewChecks]), a
`NewWithHealthCheck` batch, a `DetailedHealthRecorder` path, or a samber/do
`HealthcheckerWithContext` service. The detail should state what is not
configured and how to enable it (the enable recipe).

**Grading.** `grades` maps an `OffError` to `StatusOff` with the detail text
in `Check.Error`, regardless of criticality. Wrapping is preserved through
`errors.As`; a lost sentinel degrades to plain warn/fail grading.

**Verdict exclusion.**

- `classify` skips off results entirely: an all-off instance rolls up
  `pass`. Off is visibility, never a verdict.
- `evaluateStartup` counts an off critical service as satisfied — there is
  no runtime dependency to wait for, and blocking the startup latch on a
  deliberate absence would be the restart-cascade antipattern. A critical
  check reporting off is a configuration smell; surface it via
  [WithEvaluationHook], not the verdict.
- `Status.Rank()` ranks off with pass (tier 2), so aggregate and federation
  worst-of merges never let an off row warn or fail the merged roll-up.

**Wire.** `Check.Status` gains `"off"`; `Check.Error` carries the detail
(its doc already scoped it to "when Status is not pass"). `Response.Status`
enum is unchanged. docs/openapi.yaml documents the check-level value.

**Federation trust boundary.** `decodeDocument` accepts `off` — it is now a
known status with pass-tier rank, so it cannot masquerade a broken remote as
anything worse than a visible annotation. Unknown statuses are still refused
whole.

## Alternatives rejected

- **Warn + boolean field** (`{status:"warn", configured_off:true}`):
  zero wire-enum change, but the roll-up stays warn — CV's aggregate noise,
  the exact problem being solved, survives.
- **Pass + boolean field**: fixes the roll-up but renders green; the detail
  would not surface in any existing row renderer (they print `error` only
  for non-pass). Visibility regresses below CV's current synthesis.
- **Fourth response-level status**: the starting-status rejection, verbatim.

## Rollout / compatibility

`federation` consumers older than this change **refuse an `off` document
whole** (synthetic `name/reachable` fail). Producers must therefore ship
strictly after every federation consumer on their path:

1. go-health: this change ([Unreleased] → next minor).
2. go-health-dashboard: bump its go-health requirement, render `off` rows
   (grey, detail from `error`), release.
3. SystemNix: deploy the hub carrying that dashboard bump.
4. Only then may a producer (CV first) return `Off(...)` from its check and
   delete its projection-layer synthesis.

Until step 4, nothing on the wire changes for anyone.

## Consequences

- The library can express intentional absence natively; CV's synthesized
  row and DiscordSync's omission both become instances of one primitive.
- The three-probe HTTP contract is unchanged: liveness never sees checks;
  readiness answers 200 for pass/warn/off and 503 only for fail; the
  startup latch treats off as satisfied.
- Consumers rendering check rows need one new case; every other consumer is
  unaffected (additive wire value, pass-tier rank).
