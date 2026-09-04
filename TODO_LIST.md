# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.

## Status legend

| Status      | Meaning                                                     |
| ----------- | ----------------------------------------------------------- |
| TODO        | Not started. Needs doing.                                   |
| IN_PROGRESS | Actively being worked on.                                   |
| BLOCKED     | Cannot proceed, external dependency or decision needed.     |
| DONE        | Completed. Remove from this list and log in `CHANGELOG.md`. |

## High Impact

| Task                                  | Status  | Impact | Effort | Evidence                                                                                                                                                          |
| ------------------------------------- | ------- | ------ | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |

## Medium Impact

| Task                                                            | Status | Impact | Effort | Evidence                                                                                                                                                     |
| --------------------------------------------------------------- | ------ | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Add `go.mod` toolchain directive                                | TODO   | Med    | 5min   | `go.mod:3` says `go 1.26.7` but has no `toolchain` line pinning the exact toolchain for reproducibility.                                                      |
| Add `meta.description` to all Nix apps                          | TODO   | Med    | 15min  | Apps in `flake.nix` carry no `meta.description`; `nix flake check` emits warnings for every app.                                                              |
| Scope v0.0.3 release                                            | TODO   | Med    | 15min  | json/v2 migration + toolchain bump sit unreleased in `[Unreleased]`. Decide content, tag, release. `CHANGELOG.md`                                              |

## Low Impact

| Task                                                              | Status | Impact | Effort | Evidence                                                                                                    |
| ----------------------------------------------------------------- | ------ | ------ | ------ | ----------------------------------------------------------------------------------------------------------- |
| Add fuzz tests for JSON marshaling edge cases                     | TODO   | Low    | 1h     | No fuzz tests exist. `handlers.go:165`                                                                       |
| Add benchmark: startup handler under contention                   | TODO   | Low    | 20min  | Only the unlatched variant exists. `probe_test.go:1422`                                                      |
| Verify README Quick Start compiles (as an `Example` function)      | TODO   | Low    | 15min  | README example drifted from `ExampleNew` once already. `README.md:53`, `example_test.go:23`                   |
| Re-evaluate the two `//nolint` suppressions in panic recovery      | TODO   | Low    | 30min  | `err113` could use a static sentinel; `nonamedreturns` could be refactored away. Suppressed with justification at `probe.go:382`. |
| Add ADRs for key decisions                                        | TODO   | Low    | 1h     | Stdlib errors, no-logging, and three-state classify decisions live only in AGENTS.md prose. No `docs/adr/`.   |
| Add renovate.json or dependabot.yml                               | TODO   | Low    | 15min  | No automated dependency updates. Only one direct dependency, so value is modest.                              |
| Verify `nix flake check --all-systems`                            | TODO   | Low    | 10min  | Flake declares all systems via `systems` input but checks have only been run on x86_64-linux. `flake.nix`      |
| Top up `done at <hash>` citations in reports `19-01`/`19-11`       | TODO   | Low    | 10min  | ~12 verdicts cite dates instead of hashes; hashes now exist. `docs/status/2026-08-07_19-01*`, `_19-11*`       |
| Curate TODO_LIST Low section                                       | TODO   | Low    | 15min  | Cap at ~8 actionable rows; move infra-noise (renovate, templates) to ROADMAP until scheduled.                   |
| Full read-through verification of all 6 annotated reports          | TODO   | Low    | 30min  | Annotation verification relied on grep sweeps; one whitespace-equivalent edit only indirectly verified.         |
