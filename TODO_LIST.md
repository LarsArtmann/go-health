# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.
>
> Lifecycle: a completed TODO is deleted (it now lives in CHANGELOG). Done
> items never stay here, and there is no "Previously Completed" section.
>
> **Re-verified 2026-10-27 against the released `v0.8.1` tag.** The drift-alarm
> gate (`.#docs-check`) mechanizes the README/AGENTS/CHANGELOG/FEATURES sync;
> branch protection was verified at CONFIG level 2026-10-09 (5 named checks +
> linear history + admin bypass). Executed history lives in CHANGELOG.md and the
> dated reports under `docs/status/` — this file only carries work that is still
> open. The actionable plan behind most rows below is
> [docs/planning/2026-10-27_21-12_wave2-adoption-closing-the-consumption-gap.md](docs/planning/2026-10-27_21-12_wave2-adoption-closing-the-consumption-gap.md).

## Status legend

| Status      | Meaning                                                 |
| ----------- | ------------------------------------------------------- |
| TODO        | Not started. Needs doing.                               |
| IN_PROGRESS | Actively being worked on.                               |
| BLOCKED     | Cannot proceed, external dependency or decision needed. |

## The consumption gap (highest impact — the wave is shipped, unused)

| Task                                                                                                                                              | Status  | Impact | Effort | Evidence / next action                                                                                                             |
| ------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | ------ | ------ | ---------------------------------------------------------------------------------------------------------------------------------- |
| **Re-pin both bridges to v0.8.1** (`go-appkit/health`, `cqrs-htmx/health`) before any tag is minted                                               | BLOCKED | High   | 30min  | Both pin v0.8.0, whose own CI run was red; v0.8.1 is the green re-cut. Owner push authority (plan M01).                            |
| **Push `go-appkit/health` v0.3.0** — the tag exists locally only, so `go get` cannot resolve it                                                   | BLOCKED | High   | 30min  | `git tag -l` shows it; proxy does not. A local tag nobody can resolve is worse than no tag (plan M02).                             |
| **Tag `cqrs-htmx/health`** (next `health/v4.x`) — the module has no tag at all; ~20 of 36 consumers reach the wave through it; README PR #37 open | BLOCKED | High   | 100min | Latest module tag is `health/v4.13.0`, predating the wave. Largest single lever on the board (plan M03).                           |
| Compile 2 bridges + 5 consumers against the release; publish the evidence table                                                                   | BLOCKED | High   | 100min | Turns "shipped" into adoption evidence (plan M04).                                                                                 |
| Adopt `CachedResponseFresh`/`EvaluateAndCache` in DiscordSync, dnsblockd, cv, go-taskqueue; delete the four hand-rolled cache adapters            | BLOCKED | High   | 3h     | Needs a resolvable bridge first; the primitives have **zero** adopters today ([docs/adoption-matrix.md](docs/adoption-matrix.md)). |
| go-taskqueue → `NewChecks`, dnsblockd → `CacheAge`+`Response.Clone`, company-master-data → `checks.Disk`                                          | BLOCKED | Med    | 4h     | Five-repo adoption wave; each deletes a hand-rolled adapter (plan M06-M09, M19).                                                   |

## Next breaking window — staged renames (A22 decision table)

Harvested 2026-10-11 from docs/naming-integrity.md; they ride the next minor that
already carries a breaking change.

| Task                                                                                                                                                 | Status  | Impact | Effort | Evidence                                                                                                                                                                                                                                         |
| ---------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `WithCriticalServices(...string)` → `(...ServiceName)` clean cut + `WithCriticalChecks` deprecated alias (A22 order step 1–2, ServiceName cut first) | BLOCKED | Med    | 2h     | Owner go/no-go on typed identity still reserved (2026-10-11 report §b1; servicename-design.md is DESIGN-ONLY) — then docs/servicename-design.md fleet inventory + `tools/servicename-scan.sh`; 8 constructor repos + 2 bridges break, mechanical |
| `SanitizeResponse` → `CoerceValidUTF8` alias + deprecate (A22 step 3)                                                                                | TODO    | Low    | 30min  | dashboard-only break (`dashboard.go`, 1 prod + 1 test file)                                                                                                                                                                                      |
| `Check.Since` → `StatusSince` Go-field clean cut, JSON `since` frozen (A22 step 4)                                                                   | TODO    | Low    | 30min  | dashboard-only break (`status.go:311,391` + 3 test files); pair with docs/check-metadata-design.md doc pass                                                                                                                                      |
| `Probe`/`Prober`/`Source`/`Remote` lexicon reconciliation                                                                                            | BLOCKED | Low    | 1d     | A22: deferred past the window — highest coupling, lowest lie-factor; revisit only with docs/vocabulary-reconciliation.md enforcement tooling                                                                                                     |

## Repo hardening — decisions recorded 2026-10-27

| Task                                                                              | Status | Impact | Effort | Evidence                                                                                                                     |
| --------------------------------------------------------------------------------- | ------ | ------ | ------ | ---------------------------------------------------------------------------------------------------------------------------- |
| Fix or disable the stale golangci LSP integration (lying diagnostics, 5 sessions) | TODO   | Low    | 30min  | `docs/adr/`-adjacent discipline note in AGENTS Gotchas; the CLI gates are the truth source. Owner call on disable-vs-repair. |

Three decision records landed 2026-10-27 and are no longer open questions:
[ADR-009](docs/adr/ADR-009-shutdown-is-one-way.md) (shutdown is one-way),
[ADR-010](docs/adr/ADR-010-freshness-is-a-primitive.md) (no `WithStaleAfter`),
and [docs/accessor-read-cost-design.md](docs/accessor-read-cost-design.md)
(two read passes, measured, no merged accessor).

## Federation drain follow-through (issue #5)

| Task                                                                                                                                                                                 | Status | Impact | Effort | Evidence                                                                                                                                                                                                      |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------ | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Wire `fed.MarkShuttingDown()` into go-health-dashboard's hub SIGTERM path + end-to-end verify (held SSE client, poll `/readyz` → 503 through the drain window, `/healthz` stays 200) | TODO   | High   | M      | Draft ready in [docs/drafts/dashboard-family-handoff.md](docs/drafts/dashboard-family-handoff.md) draft 2; the go-health side is released (v0.8.0) and the hub's comment still says the method does not exist |

## Dashboard-family asks (drafts ready, posting is the owner's act)

| Task                                                                            | Status | Impact | Effort | Evidence                                                                                                                                                 |
| ------------------------------------------------------------------------------- | ------ | ------ | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| dashboard#14 — `Shutdown` does not cancel `Start`'s ctx (goroutine outlives it) | DRAFT  | Med    | 5min   | [docs/drafts/dashboard-family-handoff.md](docs/drafts/dashboard-family-handoff.md) draft 1, verified against `dashboard.go:243-258` + `pusher.go:92-108` |
| hub SIGTERM → drain readiness                                                   | DRAFT  | High   | 5min   | Same file, draft 2.                                                                                                                                      |

## Owner Actions (artifacts ready, publishing is yours)

| Task                                                                | Status | Impact | Effort | Evidence                                                                                                                                                                                                                                                    |
| ------------------------------------------------------------------- | ------ | ------ | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Publish the v0.1.1/v0.1.2 announcement                              | TODO   | Low    | 15min  | Draft + channels checklist ready in `docs/announcements/2026-09-04_v0.1.1-v0.1.2.md`.                                                                                                                                                                       |
| Post samber/do#318 comment: per-service duration on `HealthOutcome` | TODO   | Medium | 5min   | Draft + verification notes + checklist ready in `docs/announcements/2026-09-22_samber-do-issue-318-duration-comment.md`. Citations re-verified against samber/do master 2026-10-09; #318 still has 0 comments. Posting is the owner act (third-party repo). |
| Post the issue-#5 close comment                                     | TODO   | Medium | 5min   | Draft ready in `docs/drafts/issue5-close-comment.md` (implementation + gates evidence, closing as completed); posting is the owner act.                                                                                                                     |
| Post the issue-#7 annotation (four findings, all addressed)         | DRAFT  | Low    | 10min  | Draft at `docs/drafts/issue7-annotation.md` — F1/F2/F3 fixed on master, F4 docs landed, so the annotation is final.                                                                                                                                         |

## High Impact (owner decisions — blocked)

| Task                                                           | Status  | Impact | Effort | Evidence                                                                                                                                                                                                                               |
| -------------------------------------------------------------- | ------- | ------ | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Does BuildFlow full mode REPLACE the flake gates as canonical? | BLOCKED | Medium | S      | Open owner question since 2026-10-08. Until answered: `.#gates` canonical, BuildFlow an additional surface — full-mode reds arbitrated against gates (5aaf336 license-check precedent).                                                |
| Coverage floor value + whether CI may red on it                | DECIDED | Medium | S      | 2026-10-27: floor is 99.5% total / 98% per package, enforced by `.#coverage-gate` in CI and in `.#gates` (the script also carries `--report-only` for adoption on a red baseline). Raise only after a deliberate coverage improvement. |

## Blocked — upstream / cross-repo decisions

| Task                                                                                                                                      | Status  | Impact | Effort | Evidence                                                                                                                                                                                                                                                                                                                                                              |
| ----------------------------------------------------------------------------------------------------------------------------------------- | ------- | ------ | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Settle the BuildFlow profile binary, then restore full-mode exit 0                                                                        | BLOCKED | Medium | 1h     | Binary is the `chore/linter-v050-guard` branch install `5aaf336` (go 1.27.0 closure); the morning-verified `200ab99` (79/79 green) was silently replaced. BuildFlow HEAD `aa9e2e4` did not build (concurrent session's WIP). Re-run full mode once a buildable binary lands, then retire `tools/bin` wrappers + `GOTOOLCHAIN` per `.buildflow.yml`'s drop conditions. |
| File upstream BuildFlow issue: full-mode spawn env ignores `GOTOOLCHAIN=auto` for license-check (+ misleading `buildflow upgrade` output) | BLOCKED | Medium | 1h     | drain-report §f13/D3; needs `verify-before-filing` at source level. BuildFlow has zero open issues — the gap lives only in `.buildflow.yml` comments.                                                                                                                                                                                                                 |
| samber-do-auditlog: implement `DetailedHealthRecorder` (first implementor)                                                                | BLOCKED | Medium | 1h     | Owner decision. The premise was false (auditlog times builds + shutdowns, not health checks); implementing it reverses ADR-004's dependency-free decoupling. Pattern proven by go-health-dashboard's `timedScreenshotRecorder`.                                                                                                                                       |
| Normalize `tools/doanalyzerv2` go directive (1.27.1 dep floor vs repo `1.27`)                                                             | BLOCKED | Low    | 30min  | Root floor is the RELEASED samber-linter v0.4.0 (`go 1.27.1`); local checkout already declares `go 1.27`. Releasing it, then `go get` in branching-flow, then re-lowering both directives is the fix. `go mod tidy` re-bumps (empirical 2026-10-08), so the directive is not a local lever.                                                                           |
