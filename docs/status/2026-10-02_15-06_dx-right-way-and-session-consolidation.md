# Status Report — DX "Right Way" Analysis (Phase 2) & Session Consolidation

**Date:** 2026-10-02 15:06 CEST
**Session:** read-only analysis, two phases. Phase 1 (wire format vs paperless-ngx + consumer survey) is documented in
[`2026-10-02_13-48_wire-format-vs-paperless-and-consumer-survey.md`](2026-10-02_13-48_wire-format-vs-paperless-and-consumer-survey.md)
— this report supersedes it as the session snapshot and adds Phase 2 (DX).
**Trigger (Phase 2):** "How can we make using it the 'right way' easier?"

---

## TL;DR

- Phase 2 asked how to make correct go-health usage easier. Answer delivered in chat: **7 ranked proposals**, 5 evidence-grounded friction points, and one headline finding.
- **Headline finding (code-grounded):** critical service names are loose strings (`probe.go:136`). A typo means, for readiness, the service is _silently non-critical_ (`classifier.go:40`); for startup, a critical name that never appears in results **blocks the latch forever** (`classifier.go:62-68`) with no error and no log. `Validate()` covers only timeout/refresh (`probe.go:438-452`) — and `Start()` already runs an initial batch (`probe.go:491`), so it could catch this for free.
- **Second finding (ecosystem leverage):** 16 of 30 consumers arrive indirectly through just two bridges — cqrs-htmx (14) and go-appkit (2). Making those bridges default-correct is higher leverage than any core API change.
- **Third finding:** CV's `SystemResources` and file-and-image-renamer's `CheckDiskSpace` are the same disk/memory need implemented twice, privately, drifting.
- Nothing was implemented; no code changed. Two consecutive reports now sit unharvested.

---

## Phase 2 evidence trail

| # | Action                                                                                                | Evidence                                                                                                                                                                                                                                                                          |
| - | ----------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | Loaded `how-to-golang` (matched: Go DI/architecture); principles: type safety first, errors-as-values | SKILL.md only; references not loaded                                                                                                                                                                                                                                              |
| 2 | Read the full Option surface                                                                          | `probe.go:121-258` (`WithVersion`, `WithInstanceID`, `WithCriticalServices`, `WithEvaluationHook`, `WithLiveThrottle`, `WithShutdownGracePeriod`, `WithNowFunc`, `WithAllowedMethods`, `WithHealthRecorder`, `WithRefreshInterval`, `WithTimeout`, `WithBootTime`, `WithGETOnly`) |
| 3 | Read `classifier.go` in full — proved name-matching semantics                                         | `classify` `classifier.go:31-45`; `evaluateStartup` `classifier.go:62-68` (`!found` → latch never sets)                                                                                                                                                                           |
| 4 | Read `Validate` + `Start`                                                                             | `probe.go:438-452` (only timeout/refresh); `probe.go:465-498` (initial `refreshCache` at `:491`)                                                                                                                                                                                  |
| 5 | Read the documented quick start                                                                       | `README.md` Quick Start block                                                                                                                                                                                                                                                     |
| 6 | Delivered 7 ranked DX proposals + 5 friction points                                                   | chat                                                                                                                                                                                                                                                                              |

---

## a) FULLY DONE

1. **Complete Option-surface inventory** (`probe.go:121-258`) — every config knob catalogued, including the ones not previously audited (`WithBootTime`, `WithLiveThrottle`, `WithShutdownGracePeriod`).
2. **Proved the critical-name semantics by reading the classifier** — three distinct match sites (`classify`, `evaluateStartup`, `grades`) and their divergent failure modes.
3. **Established that `Start()` is the natural guard point** — it already evaluates before returning (`probe.go:491`), so no extra evaluation cost to validate critical names.
4. **Captured the currently-documented golden path** (README quick start: `New` + `WithCriticalServices` + `WithVersion` + `Start` + `RegisterRoutes`).
5. **Delivered the DX answer** — 5 friction points from fleet evidence, 7 proposals with an effort/leverage table, an explicit ship order (#1 → #3 → #5 → #2), and the bridge-leverage insight (16/30 via go-appkit + cqrs-htmx).
6. **Phase 1 carried fully** (see the linked report): wire-format inventory, paperless-ngx `/api/status/` comparison, 4 consumer implementation patterns.

## b) PARTIALLY DONE

1. **The headline footgun is inferred, not reproduced** — code-reading conclusion, no executed test. A ~20-line repro would have hardened it to fact.
2. **samber/do injector API not verified** — I wrote "do exposes `injector.ListProvidedServices()`/similar" without confirming the exact method. Hedged, but still unverified.
3. **Proposal effort/leverage ratings are judgment**, not measured (no LOC estimate, no compat matrix).
4. **Backward-compat impact of a `Start()` error not assessed** — env-specific deployments (a critical name legitimately absent in some environments) could break.
5. **Prior-decision check skipped** — did not search `docs/` for a previously rejected "batteries/checks subpackage" before proposing one (the repo carries 10+ rejected-design docs; real risk of re-proposing a settled decision).
6. **`how-to-golang` references not loaded** — `key-patterns.md` (DI) and `banned-libraries.md` were offered by the skill and plausibly relevant to a DI/DX answer.
7. **CV/fir checks read only at signature level** — the "batteries" proposal lacks threshold/severity semantics.

## c) NOT STARTED

1. **Any implementation** — exploration mode; no `ErrUnknownCriticalService`, no validation, no `health/checks`, no README changes.
2. **Writing Phase 1 + Phase 2 findings to repo docs** — AGENTS.md inventory/pattern update, genre-comparison doc, DX write-up: all still absent.
3. **Harvesting either report's section (f)** into `TODO_LIST.md` / `ROADMAP.md`.
4. **A repro test for the typo footgun.**
5. **Adoption audit of the remaining features** — `WithInstanceID`, `NewChecks`, `NewWithDetailedCheck`/`DetailedHealthRecorder`, `WithEvaluationHook`, method guard, `WithLiveThrottle`, `WithShutdownGracePeriod`, `Healthz`, `AwaitReady`, `MarkShuttingDown`.
6. **Compat analysis of the 30 consumers** for any `Validate()`/`Start()` behavior change.
7. **A fresh-user integration simulation** (copy the README quick start, run it) to measure the golden-path gap empirically.

## d) TOTALLY FUCKED UP

Nothing destructive (read-only session). Honest failures:

1. **Unverified external-API assertion** — "do exposes `injector.ListProvidedServices()`/similar" was stated as if checked. It wasn't. This is exactly the verify-external-claims failure mode.
2. **Headline finding shipped without proof** — the strongest claim of the session (typo → startup never latches) is a code-read inference. Cheap to prove, not proven.
3. **Proposed a new API surface (`health/checks`) without checking whether it was previously rejected** — the repo's own docs contain 10+ rejected designs; I read none of them before proposing.
4. **Two full reports, zero harvests** — this is the second consecutive snapshot; the improvement knowledge keeps living in chat + timestamped files. Memory/doc protocol drift in real time.
5. **"Fix the bridges is highest leverage" rests on consumer-count inference**, not on opening the bridges to confirm their criticality defaults are actually wrong.

## e) WHAT WE SHOULD IMPROVE

1. **Reproduce before asserting** — convert the footgun into a failing test first; then the design note writes itself.
2. **Verify the dependency's API before proposing to use it** — check samber/do's enumeration surface.
3. **Search `docs/` for prior decisions before proposing new surface** — the ADR/rejected-design corpus exists precisely for this.
4. **Ship compat analysis with any `Start()`/`Validate()` change** — enumerate the 30 consumers' `WithCriticalServices` calls.
5. **Harvest promptly** — a report that is never harvested is an expensive diary entry.
6. **Cap proposals at one proven change**, then widen — the 7-item list risks list-driven scope creep (the very trap AGENTS.md warns about).

## f) Top 50 things we should get done next

_Brainstorm, not commitment list. Carries Phase 1 items forward; the two reports together feed one HARVEST._

| #  | Task                                                                                                                                              | Bucket   |
| -- | ------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| 1  | Reproduce the typo footgun as a failing test (unknown critical name → startup latch never sets)                                                   | test     |
| 2  | Verify the exact samber/do injector enumeration method for service-name validation                                                                | verify   |
| 3  | Search `docs/` (ADRs + rejected-designs) for any prior "checks subpackage"/batteries decision                                                     | verify   |
| 4  | Draft the design note for `ErrUnknownCriticalService` + `Start()`-time validation                                                                 | design   |
| 5  | Compat analysis: does a `Start()` error break any of the 30 consumers (env-specific names)?                                                       | verify   |
| 6  | Fleet grep of every `WithCriticalServices` call to inventory critical-name usage                                                                  | survey   |
| 7  | Decide gating posture: hard error vs eval-hook diagnostic vs dev-mode strictness                                                                  | decision |
| 8  | Prototype `health/checks` (Disk/Memory/HTTP/Database), zero new deps                                                                              | code     |
| 9  | Read CV `SystemResources` + fir `CheckDiskSpace` fully; pin threshold/severity semantics                                                          | verify   |
| 10 | Decide batteries ownership: core subpackage vs separate module vs go-appkit/health                                                                | decision |
| 11 | Inspect go-appkit/health criticality defaults — does the bridge derive critical correctly?                                                        | verify   |
| 12 | Inspect cqrs-htmx/health criticality defaults (are projection checks critical?)                                                                   | verify   |
| 13 | Design the "bridges are the golden path" story (ecosystem, 16/30 consumers)                                                                       | design   |
| 14 | README decision table: which constructor for which need (durations, non-injector, etc.)                                                           | docs     |
| 15 | Evaluate `health.Setup`/`MustNew` convenience (or reject with rationale)                                                                          | design   |
| 16 | Evaluate `health.InvokeCritical` helper vs documentation-only eager invocation                                                                    | design   |
| 17 | Consolidate version stamping: `WithVersion` + `VersionHandler` + ldflags one recipe                                                               | docs     |
| 18 | Evaluate `WithVersionFromBuildInfo` (or reject)                                                                                                   | design   |
| 19 | Publish the full Option reference (all knobs incl. BootTime/LiveThrottle/ShutdownGrace)                                                           | docs     |
| 20 | Audit `WithInstanceID` adoption across 30 consumers                                                                                               | survey   |
| 21 | Audit `NewChecks` adoption                                                                                                                        | survey   |
| 22 | Audit `NewWithDetailedCheck`/`DetailedHealthRecorder` adoption                                                                                    | survey   |
| 23 | Audit `WithEvaluationHook` adoption                                                                                                               | survey   |
| 24 | Audit `WithAllowedMethods`/`WithGETOnly` adoption (deprecation-removal input)                                                                     | survey   |
| 25 | Audit `WithLiveThrottle` adoption                                                                                                                 | survey   |
| 26 | Audit `WithShutdownGracePeriod` adoption                                                                                                          | survey   |
| 27 | Audit aggregate `Healthz` adoption                                                                                                                | survey   |
| 28 | Audit `AwaitReady`/`MarkShuttingDown` adoption                                                                                                    | survey   |
| 29 | Audit `WithNowFunc` usage — confirm tests-only, catch prod misuse                                                                                 | survey   |
| 30 | Ghost-feature review: zero-adoption features → integrate, document-as-intended, or retire                                                         | review   |
| 31 | Split-brain fix: critical-name identity (string vs typed `ServiceName`)                                                                           | design   |
| 32 | Split-brain fix: version identity (WithVersion/VersionHandler/ldflags/VCS)                                                                        | design   |
| 33 | Write the DX golden-path doc (or extend README) from this analysis                                                                                | docs     |
| 34 | HARVEST both reports' (f) into TODO_LIST.md / ROADMAP.md                                                                                          | docs     |
| 35 | Update AGENTS.md consumer inventory: 14 direct consumers (add nsfw-classifier, webphone, go-taskqueue, projects-management-automation, cqrs-htmx) | docs     |
| 36 | Update AGENTS.md with the 4 consumer patterns + DX friction points                                                                                | docs     |
| 37 | Genre-comparison doc / ADR (probe vs status page, paperless-ngx case study)                                                                       | docs     |
| 38 | Alias-safe re-verification of aggregate/federation non-adoption                                                                                   | verify   |
| 39 | Resolve go-taskqueue's go-health usage (empty grep)                                                                                               | verify   |
| 40 | Re-verify go-health-dashboard federation usage in dashboard source                                                                                | verify   |
| 41 | Run consumer test suites for the 8 direct app consumers                                                                                           | verify   |
| 42 | Version-skew audit (cqrs-htmx v4.7.0–v4.13.0; go-appkit v0.5.1/v0.7.0) vs v0.4.1                                                                  | verify   |
| 43 | Record the paperless-ngx checkout pin used for the comparison                                                                                     | verify   |
| 44 | Security note: unauthenticated probe endpoints vs staff-only status page (info-leak threat model)                                                 | docs     |
| 45 | Evaluate a project skill/template that wires go-health correctly by default (agent-skill lever)                                                   | design   |
| 46 | Evaluate a vet/analyzer for critical-name mismatches (doanalyzerv2 infra exists)                                                                  | design   |
| 47 | Define acceptance criteria for the `Start()`-validation change                                                                                    | design   |
| 48 | Decide whether `Validate()` grows or a new method owns critical-name checking                                                                     | decision |
| 49 | Simulate a fresh user (copy README quick start, run) to measure the golden-path gap                                                               | verify   |
| 50 | Carry the 3 open questions (below) to a decision                                                                                                  | decision |

## g) Three questions I cannot figure out myself

1. **Gating posture for the critical-name guard:** hard error at `Start()` (fail boot loud, but can break deployments where a critical service legitimately absent in some environment), or non-fatal diagnostic (eval hook / log, safe but missable)? Which do you want?
2. **Batteries ownership:** core `health/checks` subpackage (zero-dep, stays single-module), a separate module/repo (`go-health-contrib`), or go-appkit/health (where two bridges already live)? This decides the 30-consumer adoption story.
3. **First action:** prove the footgun with a repro test, draft the design note, or persist/harvest the two reports' findings into AGENTS.md + TODO_LIST first?

---

_Auto-commit daemon will pick this file up; no manual commit (harness rule)._

**NEXT STEP AFTER THIS REPORT:** docs-health → HARVEST (both reports) — awaiting instructions.
