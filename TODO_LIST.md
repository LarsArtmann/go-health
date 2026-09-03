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
| Run `doanalyzerv2` to verify DO-6 fix | BLOCKED | High   | 15min  | DO-6 anti-pattern fix is architecturally sound but unverified by the analyzer. Not in nixpkgs; `go install` blocked by sandbox. `probe.go:52` (no injector field) |

## Medium Impact

| Task                                                            | Status | Impact | Effort | Evidence                                                                                                                                                     |
| --------------------------------------------------------------- | ------ | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Create migration guide: `WithPlugin` to `WithHealthRecorder`    | TODO   | Med    | 20min  | Consumers coming from `samber-do-auditlog` need to know the API changed. No guide exists.                                                                     |
| Add `go.mod` toolchain directive                                | TODO   | Med    | 5min   | `go.mod:3` says `go 1.26.7` but has no `toolchain` line pinning the exact toolchain for reproducibility.                                                      |
| Decide: should panic recovery treat critical services as fail?  | TODO   | Med    | 1h     | A panic anywhere in the batch surfaces as one synthetic `health-check` error, which `classify` maps to `warn` even if a critical service panicked. `probe.go:382` |
| Add `meta.description` to all Nix apps                          | TODO   | Med    | 15min  | Apps in `flake.nix` carry no `meta.description`; `nix flake check` emits warnings for every app.                                                              |
| Scope v0.0.3 release                                            | TODO   | Med    | 15min  | json/v2 migration + toolchain bump sit unreleased in `[Unreleased]`. Decide content, tag, release. `CHANGELOG.md`                                              |

## Low Impact

| Task                                                              | Status | Impact | Effort | Evidence                                                                                                    |
| ----------------------------------------------------------------- | ------ | ------ | ------ | ----------------------------------------------------------------------------------------------------------- |
| Add property-based test for `classify`                            | TODO   | Low    | 1h     | No property test covering pass/warn/fail across all possible result maps + critical sets. `probe.go:401`     |
| Add stress tests for lifecycle interleaving                       | TODO   | Low    | 30min  | Concurrent `Start()`+`Shutdown()` and `MarkShuttingDown()`+`Shutdown()` interleavings untested. Existing concurrency tests cover read paths only. `probe.go:260` |
| Add test: `Start()` called after `Shutdown()` (restart scenario)  | TODO   | Low    | 15min  | `TestShutdown_Idempotent` and `TestShutdown_WithoutStart_DoesNotPanic` exist; restart-after-shutdown does not. `probe_test.go:1299` |
| Add snapshot test for readiness JSON response shape               | TODO   | Low    | 30min  | No golden-file test for full JSON response structure. `handlers.go:165`                                      |
| Add fuzz tests for JSON marshaling edge cases                     | TODO   | Low    | 1h     | No fuzz tests exist. `handlers.go:165`                                                                       |
| Add benchmark: startup handler under contention                   | TODO   | Low    | 20min  | Only the unlatched variant exists. `probe_test.go:1422`                                                      |
| Verify README Quick Start compiles (as an `Example` function)      | TODO   | Low    | 15min  | README example drifted from `ExampleNew` once already. `README.md:53`, `example_test.go:23`                   |
| Improve `writeResponse` marshal-error message                      | TODO   | Low    | 10min  | Message is opaque (no underlying error). Branch is defensive-only today — `Response` has marshal-safe types. `handlers.go:165` |
| Verify samber-do-auditlog still compiles against this module        | TODO   | Low    | 20min  | Public API preserved through the DO-6 refactor but no consumer-compile verification ever ran. `probe.go:21`    |
| Re-evaluate the two `//nolint` suppressions in panic recovery      | TODO   | Low    | 30min  | `err113` could use a static sentinel; `nonamedreturns` could be refactored away. Suppressed with justification at `probe.go:382`. |
| Add ADRs for key decisions                                        | TODO   | Low    | 1h     | Stdlib errors, no-logging, and three-state classify decisions live only in AGENTS.md prose. No `docs/adr/`.   |
| Document `docs/status/` convention + testing strategy in CONTRIBUTING | TODO | Low   | 20min  | Report naming pattern and testing philosophy are tribal knowledge. `CONTRIBUTING.md`                          |
| Add renovate.json or dependabot.yml                               | TODO   | Low    | 15min  | No automated dependency updates. Only one direct dependency, so value is modest.                              |
| Add `.github/ISSUE_TEMPLATE/`                                     | TODO   | Low    | 15min  | No bug report / feature request templates.                                                                   |
| Verify `nix flake check --all-systems`                            | TODO   | Low    | 10min  | Flake declares all systems via `systems` input but checks have only been run on x86_64-linux. `flake.nix`      |
| Top up `done at <hash>` citations in reports `19-01`/`19-11`       | TODO   | Low    | 10min  | ~12 verdicts cite dates instead of hashes; hashes now exist. `docs/status/2026-08-07_19-01*`, `_19-11*`       |
| Add `erraudit ./... --type-aware` to the verification gate         | TODO   | Low    | 10min  | Canonical invocation documented in AGENTS.md but never part of the gate.                                        |
| Re-run coverage and record the current %                           | TODO   | Low    | 10min  | Last recorded figure (98.7%) predates the json/v2 migration. `nix run .#coverage`                               |
| Curate TODO_LIST Low section                                       | TODO   | Low    | 15min  | Cap at ~8 actionable rows; move infra-noise (renovate, templates) to ROADMAP until scheduled.                   |
| Full read-through verification of all 6 annotated reports          | TODO   | Low    | 30min  | Annotation verification relied on grep sweeps; one whitespace-equivalent edit only indirectly verified.         |
