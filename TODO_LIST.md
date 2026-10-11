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
>
> 2026-10-11 (staging execution): the merge-unification row and the
> `WithGETOnly` row are executed and deleted per lifecycle —
> `internal/merge` extracted (docs/merge-unification-design.md, now
> IMPLEMENTED), the symbol removed under the deprecation policy's
> zero-production-adopter path, KeyHolderAI's test call site fixed locally.
> The stale hook-panic hardening row is deleted: the fix shipped in v0.5.1
> (CHANGELOG) — the row predates the header's own re-verification stamp.
> Remaining A22 staged renames harvested into their own section below.
> The buildflow full-mode row is also executed and deleted: gate verified
> exit 0 on 2026-10-11 (rebuilt binary 200ab99, `GOTOOLCHAIN=auto`), budgets
> re-reviewed and confirmed (7/9, 63/80, 7/8).
>
> Cut as v0.7.0 the same day (merge unification + `WithGETOnly` removal);
> the A22 rename section below is the successor breaking window.
>
> 2026-10-11 (evening, drain follow-through): the federation-drain report
> (`docs/status/2026-10-11_19-17_*`) §f is harvested — its engineering and
> doc items executed same session (mux-mounted drain test, draining
> benchmark with FEATURES baseline, fuzz overlay twin, AGENTS.md line-cap
> gate in `.#docs-check`/flake check, consumer-verification/merge-doc/
> federation-doc/`.buildflow.yml` updates, 18-25 report annotated); the
> drain itself stays `[Unreleased]` (v0.7.0 shipped without it — the next
> release carries it). Survivors routed below; CHANGELOG §Unreleased has
> the test-surface entry.

## Status legend

| Status      | Meaning                                                 |
| ----------- | ------------------------------------------------------- |
| TODO        | Not started. Needs doing.                               |
| IN_PROGRESS | Actively being worked on.                               |
| BLOCKED     | Cannot proceed, external dependency or decision needed. |

Both v0.6-staging rows (merge unification, `WithGETOnly` removal) were
executed 2026-10-11 and deleted per lifecycle — they now live in the
CHANGELOG's v0.7.0 section. The v0.6 window itself closed with v0.6.0
without the staged renames; they are harvested below for the next breaking
window.

## Next breaking window — staged renames (A22 decision table)

Harvested 2026-10-11 from docs/naming-integrity.md §"v0.6 staged-rename
decision table" (the window these were staged for closed without them; they
ride the next minor that already carries a breaking change). Order and
mechanics per the A22 table.

| Task                                                                                                                                                 | Status  | Impact | Effort | Evidence                                                                                                                                                                                                                                         |
| ---------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `WithCriticalServices(...string)` → `(...ServiceName)` clean cut + `WithCriticalChecks` deprecated alias (A22 order step 1–2, ServiceName cut first) | BLOCKED | Med    | 2h     | Owner go/no-go on typed identity still reserved (2026-10-11 report §b1; servicename-design.md is DESIGN-ONLY) — then docs/servicename-design.md fleet inventory + `tools/servicename-scan.sh`; 8 constructor repos + 2 bridges break, mechanical |
| `SanitizeResponse` → `CoerceValidUTF8` alias + deprecate (A22 step 3)                                                                                | TODO    | Low    | 30min  | dashboard-only break (`dashboard.go`, 1 prod + 1 test file)                                                                                                                                                                                      |
| `Check.Since` → `StatusSince` Go-field clean cut, JSON `since` frozen (A22 step 4)                                                                   | TODO    | Low    | 30min  | dashboard-only break (`status.go:311,391` + 3 test files); pair with docs/check-metadata-design.md doc pass                                                                                                                                      |
| `Probe`/`Prober`/`Source`/`Remote` lexicon reconciliation                                                                                            | BLOCKED | Low    | 1d     | A22: deferred past the window — highest coupling, lowest lie-factor; revisit only with docs/vocabulary-reconciliation.md enforcement tooling                                                                                                     |

## Hardening — the tail (100%)

| Task                                                                                                                 | Status  | Impact | Effort | Evidence                                                                                                                                                                |
| -------------------------------------------------------------------------------------------------------------------- | ------- | ------ | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Version-skew CI script: fleet `go.mod` pins vs latest tag, fail-on-drift                                             | BLOCKED | Med    | 45min  | plan R14; gated on owner push authority (2026-10-09 report §next 3): consumer bumps are local-unpushed, so the workflow would observe stale pins and run red on day one |
| Fix or disable the stale golangci LSP integration (3 standing false warnings; discipline note now in AGENTS Gotchas) | TODO    | Low    | 30min  | 2026-10-08 20:58 §f19; reconfirmed lying 2026-10-09 (report §e7); owner call                                                                                            |

## Federation drain follow-through (issue #5)

| Task                                                                                                                                                                                                                                          | Status | Impact | Effort | Evidence                                                                                                                                                              |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------ | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Wire `fed.MarkShuttingDown()` into go-health-dashboard's hub SIGTERM path + end-to-end verify (held SSE client, poll `/readyz` → 503 through the drain window; assert availability on the dashboard's `dashboardProber` surface once adopted) | TODO   | High   | M      | drain-report §f3/f4/f29 (the payoff of the feature; dashboard repo, its own session); go-health side is `[Unreleased]`-complete: tests, benchmark, fuzz overlay, docs |
| Pre-tag coverage baseline sweep (before/after delta, not just absolutes)                                                                                                                                                                      | TODO   | Low    | S      | 20:39 report B1: drain-path absolutes captured (98.6% total, overlay 100%) but no pre-change baseline was measured first                                              |
| Re-measure `BenchmarkReadinessHandler_Draining` with `-benchmem -count=3`; unify FEATURES row provenance (median ns + single-run allocs mixed)                                                                                                | TODO   | Low    | S      | 20:39 report B2; FEATURES Performance row, `federation/federation_benchmark_test.go`                                                                                  |
| Multi-remote draining benchmark (1/2/4/8 remotes), mirroring the aggregate scaling rows                                                                                                                                                       | TODO   | Low    | S      | 20:39 report f15; complements the single-remote draining baseline                                                                                                     |

## Owner Actions (artifacts ready, publishing is yours)

| Task                                                                | Status | Impact | Effort | Evidence                                                                                                                                                                                                                                                                                                                                                               |
| ------------------------------------------------------------------- | ------ | ------ | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Publish the v0.1.1/v0.1.2 announcement                              | TODO   | Low    | 15min  | Draft + channels checklist ready in `docs/announcements/2026-09-04_v0.1.1-v0.1.2.md`.                                                                                                                                                                                                                                                                                  |
| Post samber/do#318 comment: per-service duration on `HealthOutcome` | TODO   | Medium | 5min   | Draft + verification notes + checklist ready in `docs/announcements/2026-09-22_samber-do-issue-318-duration-comment.md` (gates passed, voice-checked). **2026-10-09: citations re-verified against samber/do master** (batch machinery moved to `queueServiceHealthcheck`, root_scope.go:213); #318 still has 0 comments. Posting is the owner act (third-party repo). |
| Post the issue-#5 close comment                                     | TODO   | Medium | 5min   | Draft ready in `docs/drafts/issue5-close-comment.md` (implementation + gates evidence, closing as completed); posting is the owner act.                                                                                                                                                                                                                                |
| Cut the next release carrying the federation drain (v0.8.0)         | TODO   | High   | 30min  | CHANGELOG `[Unreleased]` entry ready (drain + test-surface hardening, `issue #5` xref); manual tag flow per ROADMAP; post-release checklist incl. docs-check vs the new tag + `go get @latest` smoke. The drain did NOT ride v0.7.0.                                                                                                                                   |

## High Impact (owner decisions — blocked)

| Task                                                           | Status  | Impact | Effort | Evidence                                                                                                                                                                                                             |
| -------------------------------------------------------------- | ------- | ------ | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Coverage-threshold CI job (fail < 97%)?                        | BLOCKED | Medium | 20min  | Policy call (decision G3 follow-up). CONTRIBUTING states the 99.7% baseline; a red-failing threshold job is a maintainer preference, not a default.                                                                  |
| Does BuildFlow full mode REPLACE the flake gates as canonical? | BLOCKED | Medium | S      | Open owner question since 2026-10-08 (AGENTS.md BuildFlow gotcha). Until answered: `.#gates` canonical, BuildFlow an additional surface — full-mode reds arbitrated against gates (5aaf336 license-check precedent). |

## Blocked — upstream / cross-repo decisions

| Task                                                                                                                                      | Status  | Impact | Effort | Evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| ----------------------------------------------------------------------------------------------------------------------------------------- | ------- | ------ | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Settle the BuildFlow profile binary, then restore full-mode exit 0                                                                        | BLOCKED | Medium | 1h     | Binary is the `chore/linter-v050-guard` branch install `5aaf336` (go 1.27.0 closure); the morning-verified `200ab99` (79/79 green) was silently replaced. BuildFlow HEAD `aa9e2e4` does not build (`toolsdk.Fact` undefined, concurrent session's WIP) and the shared profile is not session-replaceable. Once a buildable binary lands: re-run `buildflow --fix --build-mode full` to exit 0 (single-step license-check already green), then retire `tools/bin` wrappers + `GOTOOLCHAIN` env per `.buildflow.yml`'s documented drop-conditions (drain-report §f6/f8/f34/f35 folded here). Mechanism record: `.buildflow.yml` tool_paths comment (2026-10-11 evening).                                                                                                                                                                                                                                                                                                                                                                                                                 |
| File upstream BuildFlow issue: full-mode spawn env ignores `GOTOOLCHAIN=auto` for license-check (+ misleading `buildflow upgrade` output) | BLOCKED | Medium | 1h     | drain-report §f13/D3; gated on BuildFlow repo leaving mid-WIP (HEAD unbuildable) + `verify-before-filing` at source level; BuildFlow has zero open issues today — the gap lives only in `.buildflow.yml` comments.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| samber-do-auditlog: implement `DetailedHealthRecorder` (first implementor)                                                                | BLOCKED | Medium | 1h     | Owner decision required (verified 2026-09-22): the row's premise was false — auditlog times builds + shutdowns, NOT health checks. Implementing the interface requires importing go-health (`health.CheckDetail` return type), reversing the deliberate post-extraction "dependency-free both ways" decoupling (ADR-004), and it would silently switch go-health consumers from do's pooled batch to a hand-rolled fan-out. The pattern is proven: go-health-dashboard's `timedScreenshotRecorder` implements the interface via `do.HealthCheckNamedWithContext` (`di_lifecycle.go:217`).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| Normalize `tools/doanalyzerv2` go directive (1.27.1 dep floor vs repo `1.27` + go-version-auto-configure warning)                         | BLOCKED | Low    | 30min  | Root floor is the RELEASED samber-linter v0.4.0 (`go 1.27.1`); its local checkout already declares `go 1.27` — releasing it (tag+push), then `go get` in branching-flow, then re-lowering both directives is the fix. Empirical 2026-10-08 00:39: `go mod tidy` re-bumps any hand-lowering ("switching to go1.27.2"), so the directive is not a local lever — re-confirmed 2026-10-09: the directive flapped `1.27.1`→`1.27`→`1.27.1` across two daemon commits (954ae91/c1363c5) as toolchains rewrote it. Runtime levers instead (verified green, full mode exit 0): `.buildflow.yml` `tool_paths → tools/bin` wrapper scripts (go 1.27.1 / auto-switching govalid — the load-bearing layer), plus `env: GOTOOLCHAIN: auto` and `~/.local/bin/govalid` as backstops. **2026-10-11 update:** BuildFlow rebuilt (200ab99) and full mode verified exit 0 (79/79) with `GOTOOLCHAIN=auto` — but the go-version-auto-configure warning on `go 1.27.1` persisted in that green run, so the wrapper drop-condition (samber-linter release) is still open. Drop when samber-linter releases. |
