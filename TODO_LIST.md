# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.
>
> Lifecycle: a completed TODO is deleted (it now lives in CHANGELOG). Done
> items never stay here, and there is no "Previously Completed" section.
>
> Re-verified 2026-10-09 against the released **v0.5.1** tag (cut
> 2026-10-09: configured-off checks, hook-panic recovery, Disk G115
> guard, failed-Start disarm fix; proxy hash-matched, GitHub Release
> Latest). The drift-alarm gate (`.#docs-check`) mechanizes the
> README/AGENTS/CHANGELOG/FEATURES sync this header used to narrate.
> Branch protection verified at CONFIG level 2026-10-09 (5 named checks +
> linear history + admin bypass). The v0.6 staging window (ServiceName,
> renames, merge unification) is unaffected by v0.5.1.
>
> 2026-10-09 (pareto-v2 execution): the release follow-through and
> fleet-proof sections are executed and deleted per lifecycle — dashboard,
> KeyHolderAI, DiscordSync, dnsblockd, webphone, go-taskqueue, Zlota44, and
> CV all verified green on v0.5.1 (CV: go 1.27 floor, nix build green);
> evidence in `docs/status/2026-10-09_21-57_*.md`. Consumer commits are
> local-unpushed pending the owner's push-authority answer.

## Status legend

| Status      | Meaning                                                 |
| ----------- | ------------------------------------------------------- |
| TODO        | Not started. Needs doing.                               |
| IN_PROGRESS | Actively being worked on.                               |
| BLOCKED     | Cannot proceed, external dependency or decision needed. |

## v0.6 window — staging (20%)

| Task                                                                                                                             | Status | Impact | Effort | Evidence                                                                                                          |
| -------------------------------------------------------------------------------------------------------------------------------- | ------ | ------ | ------ | ----------------------------------------------------------------------------------------------------------------- |
| Port `mergeResponses` (aggregate + federation) onto the `internal/merge` primitive; collapse the duplicated fuzz property suites   | TODO   | Med    | 50min  | Sketch landed 2026-10-09: docs/merge-unification-design.md (placement `internal/merge`, `Started` is federation-side) |
| Remove `WithGETOnly` (deprecated since v0.1.1): zero production adopters — KeyHolderAI usage is tests only (adoption matrix 2026-10-09); run the deprecation-policy checklist | TODO   | Low    | 20min  | docs/naming-integrity.md staged-rename table; docs/adoption-matrix.md re-verification                              |

_Staging completed 2026-10-09 and deleted per lifecycle: ServiceName call-site
inventory + scanner + verification plan (A21 → docs/servicename-design.md
§"Fleet inventory", `tools/servicename-scan.sh`); rename decision table
(A22 → docs/naming-integrity.md §"v0.6 staged-rename decision table"). Both
execute only at v0.6 window-open._

## Hardening — the tail (100%)

| Task                                                                                                                                                                               | Status | Impact  | Effort | Evidence                                                                    |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------- | ------ | --------------------------------------------------------------------------- |
| Version-skew CI script: fleet `go.mod` pins vs latest tag, fail-on-drift                                                                                                           | BLOCKED | Med     | 45min  | plan R14; gated on owner push authority (2026-10-09 report §next 3): consumer bumps are local-unpushed, so the workflow would observe stale pins and run red on day one |
| Panicking `WithEvaluationHook` callback is unrecovered on the refresh-loop path (probe.go:661-663): decide recover-vs-document per docs/panic-recovery-design.md + pin with a test | TODO    | Med     | 45min  | Verified unrecovered 2026-10-08 (docs-health audit); flagged 2026-09-15 §e6 |
| Fix or disable the stale golangci LSP integration (3 standing false warnings; discipline note now in AGENTS Gotchas)                                                               | TODO    | Low     | 30min  | 2026-10-08 20:58 §f19; reconfirmed lying 2026-10-09 (report §e7); owner call |
| Restore `buildflow --fix --build-mode=full` to exit 0 (toolchain-skew follow-ups, binary rebuild) and re-review the `.buildflow.yml` budgets once green                            | TODO   | Med     | 1h     | 2026-10-08 20:59 §b1–§b4/§e7; gate last verified red                        |

## Owner Actions (artifacts ready, publishing is yours)

| Task                                                                | Status | Impact | Effort | Evidence                                                                                                                                                                                                                                                                                                                                                               |
| ------------------------------------------------------------------- | ------ | ------ | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Publish the v0.1.1/v0.1.2 announcement                              | TODO   | Low    | 15min  | Draft + channels checklist ready in `docs/announcements/2026-09-04_v0.1.1-v0.1.2.md`.                                                                                                                                                                                                                                                                                  |
| Post samber/do#318 comment: per-service duration on `HealthOutcome` | TODO   | Medium | 5min   | Draft + verification notes + checklist ready in `docs/announcements/2026-09-22_samber-do-issue-318-duration-comment.md` (gates passed, voice-checked). **2026-10-09: citations re-verified against samber/do master** (batch machinery moved to `queueServiceHealthcheck`, root_scope.go:213); #318 still has 0 comments. Posting is the owner act (third-party repo). |

## High Impact (owner decisions — blocked)

| Task                                    | Status  | Impact | Effort | Evidence                                                                                                                                            |
| --------------------------------------- | ------- | ------ | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| Coverage-threshold CI job (fail < 97%)? | BLOCKED | Medium | 20min  | Policy call (decision G3 follow-up). CONTRIBUTING states the 99.7% baseline; a red-failing threshold job is a maintainer preference, not a default. |

## Blocked — upstream / cross-repo decisions

| Task                                                                                                              | Status  | Impact | Effort | Evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| ----------------------------------------------------------------------------------------------------------------- | ------- | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| samber-do-auditlog: implement `DetailedHealthRecorder` (first implementor)                                        | BLOCKED | Medium | 1h     | Owner decision required (verified 2026-09-22): the row's premise was false — auditlog times builds + shutdowns, NOT health checks. Implementing the interface requires importing go-health (`health.CheckDetail` return type), reversing the deliberate post-extraction "dependency-free both ways" decoupling (ADR-004), and it would silently switch go-health consumers from do's pooled batch to a hand-rolled fan-out. The pattern is proven: go-health-dashboard's `timedScreenshotRecorder` implements the interface via `do.HealthCheckNamedWithContext` (`di_lifecycle.go:217`).                                                                                                                                                                                                                                            |
| Normalize `tools/doanalyzerv2` go directive (1.27.1 dep floor vs repo `1.27` + go-version-auto-configure warning) | BLOCKED | Low    | 30min  | Root floor is the RELEASED samber-linter v0.4.0 (`go 1.27.1`); its local checkout already declares `go 1.27` — releasing it (tag+push), then `go get` in branching-flow, then re-lowering both directives is the fix. Empirical 2026-10-08 00:39: `go mod tidy` re-bumps any hand-lowering ("switching to go1.27.2"), so the directive is not a local lever — re-confirmed 2026-10-09: the directive flapped `1.27.1`→`1.27`→`1.27.1` across two daemon commits (954ae91/c1363c5) as toolchains rewrote it. Runtime levers instead (verified green, full mode exit 0): `.buildflow.yml` `tool_paths → tools/bin` wrapper scripts (go 1.27.1 / auto-switching govalid — the load-bearing layer; the stale BuildFlow binary ec8d2d3 resolves generators to a 1.27.0 go and drops config env for them in full mode), plus `env: GOTOOLCHAIN: auto` and `~/.local/bin/govalid` as backstops. Drop when samber-linter releases or BuildFlow rebuilds with a ≥1.27.1 generator toolchain. |
