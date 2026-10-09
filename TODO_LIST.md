# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.
>
> Lifecycle: a completed TODO is deleted (it now lives in CHANGELOG). Done
> items never stay here, and there is no "Previously Completed" section.
>
> Re-verified 2026-10-08 (second docs-health audit) against the v0.5.0 tag
> (released 2026-10-05). The former release train shipped as **v0.5.0**
> (not v0.4.2) and its cut/tag row was deleted; the fresh-user sim passed
> against the released tag the same day (proxy resolve, no replace,
> batteries + critical-name validation + VersionHandler exercised). The
> "v0.5 window" staging section is now the v0.6 window — none of its five
> items shipped in v0.5.0. The drift-alarm gate (`.#docs-check`) shipped
> 2026-10-08 and now mechanizes the README/AGENTS/CHANGELOG/FEATURES sync
> this header used to narrate. Owner gates: G2 (upstream filing authority)
> and G3 (CV bump authority) stand; branch protection is ON (verified via
> `gh api …/branches/master` 2026-10-08), so the old G3 branch-protection
> asks are closed.

## Status legend

| Status      | Meaning                                                 |
| ----------- | ------------------------------------------------------- |
| TODO        | Not started. Needs doing.                               |
| IN_PROGRESS | Actively being worked on.                               |
| BLOCKED     | Cannot proceed, external dependency or decision needed. |

## Release follow-through — the 1% (v0.5.0 shipped 2026-10-05, verification did not)

| Task                                                   | Status | Impact | Effort | Evidence                                                                                      |
| ------------------------------------------------------ | ------ | ------ | ------ | --------------------------------------------------------------------------------------------- |
| go-health-dashboard full suite against released v0.5.x | TODO   | High   | 40min  | The one deep aggregate+federation consumer; no verification recorded since the 2026-10-05 tag |

## Fleet proof & leverage — the 4%

| Task                                                    | Status | Impact | Effort | Evidence                                                                                     |
| ------------------------------------------------------- | ------ | ------ | ------ | -------------------------------------------------------------------------------------------- |
| Run consumer test suites against the latest tag: fir, KeyHolderAI, DiscordSync, go-taskqueue, webphone, nsfw-classifier | TODO   | Medium | 45min  | Pins 2026-10-09: fir/go-taskqueue/webphone/nsfw-classifier on v0.5.0; KeyHolderAI/DiscordSync on v0.4.1; rerun on v0.5.1 |
| Bump CV to go-health v0.5.x + `go 1.27` floor (currently v0.1.3 + `go 1.26.7`; double-stale vs v0.5.0)                  | TODO   | Medium | 45min  | CV = [github.com/LarsArtmann/CV](https://github.com/LarsArtmann/CV) (enterprise CV/resume generator). 2026-10-03 version-skew table: every other consumer pins v0.4.1 (G3) |

## v0.6 window — staging (20%)

| Task                                                      | Status | Impact | Effort | Evidence                                                                       |
| --------------------------------------------------------- | ------ | ------ | ------ | ------------------------------------------------------------------------------ |
| `mergeResponses` port prep: extract primitive sketch + corpus fixture; port both property suites when the window opens    | TODO   | Med    | 50min  | docs/merge-unification-design.md; plan R11                                     |

_Staging completed 2026-10-09 and deleted per lifecycle: ServiceName call-site
inventory + scanner + verification plan (A21 → docs/servicename-design.md
§"Fleet inventory", `tools/servicename-scan.sh`); rename decision table
(A22 → docs/naming-integrity.md §"v0.6 staged-rename decision table"). Both
execute only at v0.6 window-open._

## Hardening — the tail (100%)

| Task                                                                                                                                                                               | Status | Impact  | Effort | Evidence                                                                    |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------- | ------ | --------------------------------------------------------------------------- |
| Version-skew CI script: fleet `go.mod` pins vs latest tag, fail-on-drift                                                                                                           | TODO   | Med     | 45min  | plan R14 (CV class of skew caught by machine)                               |
| Verify FEATURES benchmark rows against a fresh `-count=3` run; label single-run rows                                                                                               | TODO   | Low-Med | 45min  | 2026-10-04 audit §c5: table structure verified, numbers never re-run        |
| Nail `makezero: always` semantics (why the pre-refactor pre-sized slice passed lint) and record the contract in AGENTS + `.golangci.yml` comment                                   | TODO   | Low-Med | 30min  | 2026-10-08 20:58 §b1/§f4 — fix shipped (`b2b9ed0`), understanding not       |
| Panicking `WithEvaluationHook` callback is unrecovered on the refresh-loop path (probe.go:661-663): decide recover-vs-document per docs/panic-recovery-design.md + pin with a test | TODO   | Med     | 45min  | Verified unrecovered 2026-10-08 (docs-health audit); flagged 2026-09-15 §e6 |
| Fix or disable the stale golangci LSP integration (3 standing false warnings; discipline note now in AGENTS Gotchas)                                                               | TODO   | Low     | 30min  | 2026-10-08 20:58 §f19; AGENTS Gotcha of record                              |
| Restore `buildflow --fix --build-mode=full` to exit 0 (toolchain-skew follow-ups, binary rebuild) and re-review the `.buildflow.yml` budgets once green                            | TODO   | Med     | 1h     | 2026-10-08 20:59 §b1–§b4/§e7; gate last verified red                        |

## Owner Actions (artifacts ready, publishing is yours)

| Task                                                                | Status | Impact | Effort | Evidence                                                                                                                                                                        |
| ------------------------------------------------------------------- | ------ | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Publish the v0.1.1/v0.1.2 announcement                              | TODO   | Low    | 15min  | Draft + channels checklist ready in `docs/announcements/2026-09-04_v0.1.1-v0.1.2.md`.                                                                                           |
| Post samber/do#318 comment: per-service duration on `HealthOutcome` | TODO   | Medium | 5min   | Draft + verification notes + checklist ready in `docs/announcements/2026-09-22_samber-do-issue-318-duration-comment.md` (gates passed, voice-checked). **2026-10-09: citations re-verified against samber/do master** (batch machinery moved to `queueServiceHealthcheck`, root_scope.go:213); #318 still has 0 comments. Posting is the owner act (third-party repo). |

## High Impact (owner decisions — blocked)

| Task                                    | Status  | Impact | Effort | Evidence                                                                                                                                            |
| --------------------------------------- | ------- | ------ | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| Coverage-threshold CI job (fail < 97%)? | BLOCKED | Medium | 20min  | Policy call (decision G3 follow-up). CONTRIBUTING states the 99.7% baseline; a red-failing threshold job is a maintainer preference, not a default. |

## Blocked — upstream / cross-repo decisions

| Task                                                                                                              | Status  | Impact | Effort | Evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| ----------------------------------------------------------------------------------------------------------------- | ------- | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| samber-do-auditlog: implement `DetailedHealthRecorder` (first implementor)                                        | BLOCKED | Medium | 1h     | Owner decision required (verified 2026-09-22): the row's premise was false — auditlog times builds + shutdowns, NOT health checks. Implementing the interface requires importing go-health (`health.CheckDetail` return type), reversing the deliberate post-extraction "dependency-free both ways" decoupling (ADR-004), and it would silently switch go-health consumers from do's pooled batch to a hand-rolled fan-out. The pattern is proven: go-health-dashboard's `timedScreenshotRecorder` implements the interface via `do.HealthCheckNamedWithContext` (`di_lifecycle.go:217`).                                                                                                                                                                                                                                            |
| Normalize `tools/doanalyzerv2` go directive (1.27.1 dep floor vs repo `1.27` + go-version-auto-configure warning) | BLOCKED | Low    | 30min  | Root floor is the RELEASED samber-linter v0.4.0 (`go 1.27.1`); its local checkout already declares `go 1.27` — releasing it (tag+push), then `go get` in branching-flow, then re-lowering both directives is the fix. Empirical 2026-10-08 00:39: `go mod tidy` re-bumps any hand-lowering ("switching to go1.27.2"), so the directive is not a local lever. Runtime levers instead (verified green, full mode exit 0): `.buildflow.yml` `tool_paths → tools/bin` wrapper scripts (go 1.27.1 / auto-switching govalid — the load-bearing layer; the stale BuildFlow binary ec8d2d3 resolves generators to a 1.27.0 go and drops config env for them in full mode), plus `env: GOTOOLCHAIN: auto` and `~/.local/bin/govalid` as backstops. Drop when samber-linter releases or BuildFlow rebuilds with a ≥1.27.1 generator toolchain. |
