# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.
>
> Lifecycle: a completed TODO is deleted (it now lives in CHANGELOG). Done
> items never stay here, and there is no "Previously Completed" section.
>
> Re-verified 2026-10-08 (docs-health audit) against the v0.5.0 tag
> (released 2026-10-05). The former release train shipped as **v0.5.0**
> (not v0.4.2) and its cut/tag row was deleted; the fresh-user sim passed
> against the released tag the same day (proxy resolve, no replace, batteries
>
> - critical-name validation + VersionHandler exercised). The "v0.5 window"
>   staging section is now the v0.6 window — none of its five items shipped in
>   v0.5.0. Newly harvested from the two 2026-10-04 reports: the drift-alarm
>   gate, the archive sweep, and the FEATURES benchmark re-verify. Owner gates:
>   G1 (release timing) is moot; G2 (upstream filing authority) and G3 (CV bump
>   authority) stand.

## Status legend

| Status      | Meaning                                                 |
| ----------- | ------------------------------------------------------- |
| TODO        | Not started. Needs doing.                               |
| IN_PROGRESS | Actively being worked on.                               |
| BLOCKED     | Cannot proceed, external dependency or decision needed. |

## Release follow-through — the 1% (v0.5.0 shipped 2026-10-05, verification did not)

| Task                                                                                                        | Status | Impact | Effort | Evidence                                                                                      |
| ----------------------------------------------------------------------------------------------------------- | ------ | ------ | ------ | --------------------------------------------------------------------------------------------- |
| go-health-dashboard full suite against released v0.5.0                                                      | TODO   | High   | 40min  | The one deep aggregate+federation consumer; no verification recorded since the 2026-10-05 tag |
| v0.5.0 announcement draft (boot-contract `ErrUnknownCriticalService` + checks batteries + `VersionHandler`) | TODO   | Med    | 30min  | CHANGELOG `[0.5.0]`; the release shipped with no announcement; publish is an owner call       |

## Fleet proof & leverage — the 4%

| Task                                                                                                                    | Status | Impact | Effort | Evidence                                                                                      |
| ----------------------------------------------------------------------------------------------------------------------- | ------ | ------ | ------ | --------------------------------------------------------------------------------------------- |
| File go-appkit/health + cqrs-htmx/health upstream issues from drafts                                                    | TODO   | High   | 40min  | Drafts ready in `docs/announcements/2026-10-02_*.md`; filing is owner (G2)                    |
| Run consumer test suites against the v0.5.0 tag: fir, KeyHolderAI, DiscordSync, go-taskqueue, webphone, nsfw-classifier | TODO   | Medium | 45min  | 2026-10-03 train: all 8 direct apps BUILD-OK against then-unreleased master; rerun on the tag |
| Bump CV to go-health v0.5.x + `go 1.27` floor (currently v0.1.3 + `go 1.26.7`; double-stale vs v0.5.0)                  | TODO   | Medium | 45min  | 2026-10-03 version-skew table: every other consumer pins v0.4.1 (G3)                          |

## v0.6 window — staging (20%)

| Task                                                                                                                         | Status | Impact   | Effort | Evidence                                                                                             |
| ---------------------------------------------------------------------------------------------------------------------------- | ------ | -------- | ------ | ---------------------------------------------------------------------------------------------------- |
| ServiceName call-site inventory: fleet list → mechanical rewrite script + verification diff                                  | TODO   | High     | 60min  | docs/servicename-design.md (vehicle label there still says v0.5 — update when landing); plan R6      |
| Finalize rename staging: `SanitizeResponse`→`CoerceValidUTF8`, `Since`→`StatusSince`, `WithCriticalChecks` decision table    | TODO   | Med      | 40min  | docs/naming-integrity.md; plan R7                                                                    |
| Aggregate-validation integration test: a probe with an unknown critical name inside a source (validation composes)           | TODO   | Med-High | 30min  | plan R15; verified 2026-10-08: no `ErrUnknownCriticalService` reference in aggregate/ or federation/ |
| `mergeResponses` port prep: extract primitive sketch + corpus fixture; port both property suites when the window opens       | TODO   | Med      | 50min  | docs/merge-unification-design.md; plan R11                                                           |
| Federation validation semantics doc: remote names never hit `ErrUnknownCriticalService` (fetch-side is a different universe) | TODO   | Low-Med  | 25min  | plan R21                                                                                             |

## Hardening — the tail (100%)

| Task                                                                                                                                                                             | Status | Impact  | Effort | Evidence                                                                                                                                                                    |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `health/checks` fuzz target + benchmarks + coverage-gap close                                                                                                                    | TODO   | Med     | 80min  | plan R12/R13 (package tested, not fuzzed/benched); verified 2026-10-08: zero fuzz/bench funcs in checks/                                                                    |
| Version-skew CI script: fleet `go.mod` pins vs latest tag, fail-on-drift                                                                                                         | TODO   | Med     | 45min  | plan R14 (CV class of skew caught by machine)                                                                                                                               |
| Drift-alarm gate: `.#docs-check` flake app + `.#gates` step (README stability line == latest tag, CHANGELOG `[Unreleased]` base + complete link block, ADR range == `docs/adr/`) | TODO   | Med     | 60min  | Both 2026-10-04 reports rank it first; the v0.5.0 release skipped CONTRIBUTING checklist items 1/3 (README + AGENTS lines stale for 3 days, caught by the 2026-10-08 audit) |
| Verify FEATURES benchmark rows against a fresh `-count=3` run; label single-run rows                                                                                             | TODO   | Low-Med | 45min  | 2026-10-04 audit §c5: table structure verified, numbers never re-run                                                                                                        |
| Archive resolved status reports: 2026-09-15_08-55 (route §f → ROADMAP Theme 2 first), 2026-09-18_09-46, the four 2026-09-22 reports; decide the three 2026-09-04 anchors         | TODO   | Low     | 2h     | 2026-10-04 audit §b2/§c1; annotation tooling + manifest pattern ready                                                                                                       |
| DOMAIN_LANGUAGE + AGENTS symbol-ref anti-rot: replace bare line numbers (`probe.go:54`, `di.go:442`) with symbol names                                                           | TODO   | Low     | 30min  | plan R20 (line refs rot every edit); verified 2026-10-08: bare refs still present in both files                                                                             |

## Owner Actions (artifacts ready, publishing is yours)

| Task                                                                | Status | Impact | Effort | Evidence                                                                                                                                                                        |
| ------------------------------------------------------------------- | ------ | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Publish the v0.1.1/v0.1.2 announcement                              | TODO   | Low    | 15min  | Draft + channels checklist ready in `docs/announcements/2026-09-04_v0.1.1-v0.1.2.md`.                                                                                           |
| Post samber/do#318 comment: per-service duration on `HealthOutcome` | TODO   | Medium | 5min   | Draft + verification notes + checklist ready in `docs/announcements/2026-09-22_samber-do-issue-318-duration-comment.md` (gates passed, voice-checked; filing is an owner call). |

## High Impact (owner decisions — blocked)

| Task                                    | Status  | Impact | Effort | Evidence                                                                                                                                            |
| --------------------------------------- | ------- | ------ | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| Coverage-threshold CI job (fail < 97%)? | BLOCKED | Medium | 20min  | Policy call (decision G3 follow-up). CONTRIBUTING states the 99.7% baseline; a red-failing threshold job is a maintainer preference, not a default. |

## Blocked — upstream / cross-repo decisions

| Task                                                                       | Status  | Impact | Effort | Evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| -------------------------------------------------------------------------- | ------- | ------ | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| samber-do-auditlog: implement `DetailedHealthRecorder` (first implementor) | BLOCKED | Medium | 1h     | Owner decision required (verified 2026-09-22): the row's premise was false — auditlog times builds + shutdowns, NOT health checks. Implementing the interface requires importing go-health (`health.CheckDetail` return type), reversing the deliberate post-extraction "dependency-free both ways" decoupling (ADR-004), and it would silently switch go-health consumers from do's pooled batch to a hand-rolled fan-out. The pattern is proven: go-health-dashboard's `timedScreenshotRecorder` implements the interface via `do.HealthCheckNamedWithContext` (`di_lifecycle.go:217`). |
