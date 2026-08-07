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

| Task                                                         | Status  | Impact | Effort | Evidence                                                                                                                                                          |
| ------------------------------------------------------------ | ------- | ------ | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Verify `flake.nix` builds (`nix develop`, `nix flake check`) | TODO    | High   | 30min  | Written but never executed. May have hash mismatches or config issues. `flake.nix`                                                                                |
| Set up GitHub Actions CI pipeline                            | TODO    | High   | 1h     | No `.github/workflows/`. CI should run: `go test -race`, `go vet`, `golangci-lint`, `govulncheck`, `gosec`                                                        |
| Run `doanalyzerv2` to verify DO-6 fix                        | BLOCKED | High   | 15min  | DO-6 anti-pattern fix is architecturally sound but unverified by the analyzer. Not in nixpkgs; `go install` blocked by sandbox. `probe.go:52` (no injector field) |
| Decide: keep or revert `Start()` returning `error`           | BLOCKED | High   | 5min   | Breaking API change made without user confirmation. Library is ALPHA so likely fine, but user should decide. `probe.go:257`                                       |

## Medium Impact

| Task                                                           | Status | Impact | Effort | Evidence                                                                                                                                                   |
| -------------------------------------------------------------- | ------ | ------ | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Curate `.golangci.yml` for this project                        | TODO   | Med    | 30min  | Cargo-culted from `go-sse`. 40+ linters may include ones inappropriate for a 4-file library. `.golangci.yml`                                               |
| Expand `CONTRIBUTING.md` with real dev setup                   | TODO   | Med    | 30min  | Currently a 22-line stub. Should cover Nix setup, test/lint commands, code conventions, PR process. `CONTRIBUTING.md`                                      |
| Create migration guide: `WithPlugin` to `WithHealthRecorder`   | TODO   | Med    | 20min  | Consumers coming from `samber-do-auditlog` need to know the API changed. No guide exists.                                                                  |
| Add `go.mod` toolchain directive                               | TODO   | Med    | 5min   | Go 1.26.5 in use but no `toolchain` line for reproducibility. `go.mod:3`                                                                                   |
| Decide: should panic recovery treat critical services as fail? | TODO   | Med    | 1h     | Panics produce `StatusWarn` regardless of service criticality. A panicking critical service arguably should produce `StatusFail` (503). `probe.go:377-384` |

## Low Impact

| Task                                                    | Status | Impact | Effort | Evidence                                                                                                 |
| ------------------------------------------------------- | ------ | ------ | ------ | -------------------------------------------------------------------------------------------------------- |
| Add property-based test for `classify`                  | TODO   | Low    | 1h     | No property test covering pass/warn/fail across all possible result maps + critical sets. `probe.go:395` |
| Add stress test for concurrent `Start()` + `Shutdown()` | TODO   | Low    | 30min  | Mutex protects `cancel` but interleaving not stress-tested. `probe.go:262-276`                           |
| Add snapshot test for readiness JSON response shape     | TODO   | Low    | 30min  | No golden-file test for full JSON response structure.                                                    |
| Add fuzz tests for JSON marshaling edge cases           | TODO   | Low    | 1h     | No fuzz tests exist. `handlers.go:165` (`json.Marshal`)                                                  |
| Add release/tagging workflow                            | TODO   | Low    | 30min  | No semver tags, no release process defined.                                                              |
