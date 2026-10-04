# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.
>
> Lifecycle: a completed TODO is deleted (it now lives in CHANGELOG). Done
> items never stay here, and there is no "Previously Completed" section.
>
> Harvested 2026-10-03 (docs-health run) from
> `docs/status/2026-10-03_03-11_pareto-plan-executed-and-verified.md` §f and
> `docs/planning/2026-10-03_03-43_release-and-v05-window-pareto-plan.md`. The
> plan encodes three owner gates — G1 (v0.4.2 release timing), G2 (upstream
> filing authority), G3 (CV bump authority); recommended defaults let execution
> proceed. The two 2026-10-02 reports' §f lists were re-audited item-by-item and
> routed (all headline items shipped; the tail lives in ROADMAP or below).

## Status legend

| Status      | Meaning                                                 |
| ----------- | ------------------------------------------------------- |
| TODO        | Not started. Needs doing.                               |
| IN_PROGRESS | Actively being worked on.                               |
| BLOCKED     | Cannot proceed, external dependency or decision needed. |

## Release — the 1% (ship the unreleased value)

| Task                                                                                                              | Status | Impact | Effort | Evidence                                                                                              |
| ----------------------------------------------------------------------------------------------------------------- | ------ | ------ | ------ | ----------------------------------------------------------------------------------------------------- |
| Cut v0.4.2: pre-tag `.#gates`, CHANGELOG link-up, dashboard build on the tag, tag + push, proxy/pkg.go.dev verify | TODO   | High   | 90min  | CHANGELOG `[Unreleased]` holds `health/checks`, `ErrUnknownCriticalService`, `VersionHandler`; plan R1 |
| go-health-dashboard full suite against released v0.4.2                                                            | TODO   | High   | 40min  | The one deep aggregate+federation consumer; plan R5                                                   |
| v0.4.2 announcement draft (validation is a boot-contract change consumers must know)                              | TODO   | Med    | 30min  | plan R18; publish is an owner call                                                                    |
| Fresh-user sim against released v0.4.2 (no replace directive)                                                     | TODO   | Med    | 25min  | plan R19                                                                                              |

## Fleet proof & leverage — the 4%

| Task                                                                                                   | Status | Impact | Effort | Evidence                                                                  |
| ------------------------------------------------------------------------------------------------------ | ------ | ------ | ------ | ------------------------------------------------------------------------- |
| File go-appkit/health + cqrs-htmx/health upstream issues from drafts                                   | TODO   | High   | 40min  | Drafts ready in `docs/announcements/2026-10-02_*.md`; filing is owner (G2) |
| Run consumer test suites: fir, KeyHolderAI, DiscordSync, go-taskqueue, webphone, nsfw-classifier       | TODO   | Medium | 45min  | 2026-10-03 train: all 8 direct apps BUILD-OK; CV + dnsblockd suites green |
| Bump CV to go-health v0.4.x + `go 1.27` floor (currently v0.1.3 + `go 1.26.7`; the one breaking skew)  | TODO   | Medium | 45min  | 2026-10-03 version-skew table: every other consumer pins v0.4.1 (G3)      |

## v0.5 window — staging (20%)

| Task                                                                                                                     | Status | Impact   | Effort | Evidence                                                                     |
| ------------------------------------------------------------------------------------------------------------------------ | ------ | -------- | ------ | ---------------------------------------------------------------------------- |
| ServiceName call-site inventory: fleet list → mechanical rewrite script + verification diff                              | TODO   | High     | 60min  | docs/servicename-design.md; plan R6                                          |
| Finalize rename staging: `SanitizeResponse`→`CoerceValidUTF8`, `Since`→`StatusSince`, `WithCriticalChecks` decision table | TODO   | Med      | 40min  | docs/naming-integrity.md; plan R7                                            |
| Aggregate-validation integration test: a probe with an unknown critical name inside a source (validation composes)       | TODO   | Med-High | 30min  | plan R15                                                                     |
| `mergeResponses` port prep: extract primitive sketch + corpus fixture; port both property suites when the window opens   | TODO   | Med      | 50min  | docs/merge-unification-design.md; plan R11                                   |
| Federation validation semantics doc: remote names never hit `ErrUnknownCriticalService` (fetch-side is a different universe) | TODO   | Low-Med  | 25min  | plan R21                                                                     |

## Hardening — the tail (100%)

| Task                                                                              | Status | Impact | Effort | Evidence                       |
| --------------------------------------------------------------------------------- | ------ | ------ | ------ | ------------------------------ |
| `health/checks` fuzz target + benchmarks + coverage-gap close                     | TODO   | Med    | 80min  | plan R12/R13 (package tested, not fuzzed/benched) |
| Version-skew CI script: fleet `go.mod` pins vs latest tag, fail-on-drift          | TODO   | Med    | 45min  | plan R14 (CV class of skew caught by machine) |
| DOMAIN_LANGUAGE symbol-ref anti-rot: replace bare line numbers with symbol names  | TODO   | Low    | 30min  | plan R20 (line refs rot every edit) |

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

| Task                                                                       | Status  | Impact | Effort | Evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| -------------------------------------------------------------------------- | ------- | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| samber-do-auditlog: implement `DetailedHealthRecorder` (first implementor) | BLOCKED | Medium | 1h     | Owner decision required (verified 2026-09-22): the row's premise was false — auditlog times builds + shutdowns, NOT health checks. Implementing the interface requires importing go-health (`health.CheckDetail` return type), reversing the deliberate post-extraction "dependency-free both ways" decoupling (ADR-004), and it would silently switch go-health consumers from do's pooled batch to a hand-rolled fan-out. The pattern is proven: go-health-dashboard's `timedScreenshotRecorder` implements the interface via `do.HealthCheckNamedWithContext` (`di_lifecycle.go:217`). |
