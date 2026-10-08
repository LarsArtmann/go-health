# Federation validation semantics: remote names never hit `ErrUnknownCriticalService`

**Date:** 2026-10-08 · **Status:** DECIDED — clarification of scope, no mechanism change
**Related:** [docs/start-validation-design.md](start-validation-design.md), [docs/configured-off-design.md](configured-off-design.md), [docs/federation-design.md](federation-design.md)

## Why this doc exists

v0.5.0 shipped `Start()`-time critical-name validation: a `WithCriticalServices`
name that never appears in the initial evaluation batch fails `Start()` with
`ErrUnknownCriticalService`. A reader who knows that rule may ask whether a
federation `Remote{Name, URL}` name gets the same treatment — could a typo'd
remote name silently block `StartupComplete` the way a typo'd critical service
silently blocks the startup latch?

The answer is no, by design. Verified 2026-10-08: no production code in
`aggregate/` or `federation/` references `ErrUnknownCriticalService` — the
sentinel lives only in `probe.go`. The one match outside it,
`aggregate/aggregate_validation_test.go`, asserts that a misconfigured _source
probe's_ own `Start()` fails inside `aggregate.New` composition: the error
propagates from the inner probe, not from aggregate. (See
`aggregate_validation_test.go`, the 2026-10-08 integration test.)

## The two validation universes

|                   | Root probe (`health.Probe`)                        | Federation (`federation.Prober`)                                                           |
| ----------------- | -------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| Identity input    | `WithCriticalServices` names                       | `Remote{Name, URL}`                                                                        |
| Validated against | keys of the initial evaluation batch               | nothing — remote names are labels, never matched                                           |
| Enforcement point | `Start()` hard error (`ErrUnknownCriticalService`) | construction (`validateRemotes`) + per-fetch wire decode (`decodeDocument`)                |
| Typo failure mode | silent: latch never sets, pod never ready          | loud: mislabeled row at worst; wrong URL → synthetic `name/reachable` FAIL → readiness 503 |

## Why federation needs no name validation

Three structural reasons.

1. **The silent failure mode does not exist on the fetch side.** Root-probe
   validation exists because a typo'd critical name silently downgrades
   fail→warn and blocks the latch forever (the DiscordSync 2026-08-16 class,
   docs/start-validation-design.md). A federation remote name is never matched
   against anything: it only namespaces merged check keys (`"name/check"`) and
   labels the shutdown overlay. A typo'd name yields a mislabeled row —
   cosmetic, not a hang.

2. **The federation-analogous mistake is already loud by construction.** The
   dangerous typo on the fetch side is the URL. An unreachable, non-200,
   oversized, or undecodable remote contributes one synthetic
   `name/reachable` FAIL check (in `merge`, federation/federation.go) with the
   cause in `Error` — readiness 503, never a silent freeze, never a status
   override.

3. **There is no batch for names to be validated against.** `Start()`'s
   validation universe is the initial evaluation batch; federation has no
   `Start()` and no batch. Its method set is `CachedResponse`,
   `RefreshInterval`, `StartupComplete`, the three handlers, and
   `RegisterRoutes`. Critical/non-critical classification already happened
   inside each remote process; the merge only takes worst-of via
   `Status.Rank`.

## What federation does validate

Two layers, both about untrusted input rather than identity.

**Construction — `validateRemotes` (federation/federation.go).** The same
source-name contract as aggregate: empty set (`ErrNoRemotes`), empty names,
slash-containing names, and duplicate names are rejected; plus URL sanity —
absolute http(s) with a non-empty host (`ErrInvalidRemote` with the offending
value). This is the federation counterpart of `Probe.Validate`: enforcement
at construction, before any fetch.

**Per fetch — `decodeDocument` (federation/federation.go).** Wire input is
untrusted. A document missing `status`, or carrying a check status outside
pass/warn/fail/off, is refused whole — the fetch converts to one synthetic
`name/reachable` FAIL carrying the cause. The refusal is fail-closed by
design: an unknown status decoded naively would render healthy downstream.
`off` is accepted because it is a known status with pass-tier
`Status.Rank` — it can never fail or warn the merged roll-up, only annotate a
row (the contract is stated in the `decodeDocument` doc comment).

## Configured-off rollout note

`off` acceptance is version-gated in the wild: an OLD federation consumer
pairing with a NEW producer that emits `off` rows refuses those documents
whole, surfacing `name/reachable` FAIL instead. The rollout order stays:
producers ship `off` last — see docs/configured-off-design.md. Federation in
v0.5.0+ accepts `off`.

## Verdict

No mechanism changes. The root probe validates _identity against its check
universe_ because identity typos there are silent; federation validates
_construction parameters and wire documents_ because its identity typos are
cosmetic and its real hazards (URL, payload) already fail loud.
