# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.

> Curated 2026-09-04 after the Pareto master-plan execution (P1–P39):
> the 39-task plan is complete; remaining items below are user decisions,
> first-push follow-ups, and cosmetics.

## Status legend

| Status      | Meaning                                                     |
| ----------- | ----------------------------------------------------------- |
| TODO        | Not started. Needs doing.                                   |
| IN_PROGRESS | Actively being worked on.                                   |
| BLOCKED     | Cannot proceed, external dependency or decision needed.     |
| DONE        | Completed. Remove from this list and log in `CHANGELOG.md`. |

## High Impact

| Task                                                           | Status  | Impact | Effort | Evidence                                                                                        |
| -------------------------------------------------------------- | ------- | ------ | ------ | ----------------------------------------------------------------------------------------------- |
| ~~Push master; cut v0.1.1; sign off API names~~                | DONE    | High   | —      | Pushed 2026-09-04 (CI green on master and tag); v0.1.1 tagged, released, proxy-verified (`go get` + consumer run); API names approved; `WithGETOnly` deprecated in godoc. |

## Medium Impact

| Task                                                 | Status | Impact | Effort | Evidence                                                                                     |
| ---------------------------------------------------- | ------ | ------ | ------ | ---------------------------------------------------------------------------------------------- |
| ~~Re-verify dashboard consumer~~                      | DONE   | Med    | —      | Compiles + tests pass against HEAD via replace (2026-09-04). |

## Low Impact

| Task                                                         | Status | Impact | Effort | Evidence                                                                              |
| ------------------------------------------------------------ | ------ | ------ | ------ | --------------------------------------------------------------------------------------- |
| Top up `done at <hash>` citations in reports `19-01`/`19-11` | TODO   | Low    | 10min  | ~12 verdicts cite dates instead of hashes; hashes now exist. `docs/status/2026-08-07_19-01*`, `_19-11*` |
| Add examples: custom recorder, two-phase shutdown, live vs cached | TODO | Low | 30min | Examples exist for Quick Start, Prometheus, middleware; these three patterns live only in tests/docs. ROADMAP Theme 1. |

## Resolved this cycle (kept for traceability, delete at next curation)

- ~~go.mod toolchain directive~~ — `go mod tidy` drops a `toolchain` line equal
  to the `go` directive (verified: added, tidy removed it, build green). The
  hermetic pin lives in `flake.nix`; nothing to do for consumers.
- ~~`meta.description` on Nix apps~~ — done in flake; `nix flake check` passes
  warning-free.
- ~~Fuzz tests, contention benchmark, README-as-Example, `//nolint` removals,
  dependabot, issue templates, CI workflow, ADRs, CONTRIBUTING, migration
  guide, coverage re-run, erraudit gate, doanalyzerv2 baseline, full
  read-through of annotated reports, TODO curation~~ — all completed in the
  2026-09-04 Pareto execution; see `CHANGELOG.md` `[Unreleased]` and
  `docs/status/2026-09-04_11-13_pareto-execution-marathon-and-api-batch.md`.
