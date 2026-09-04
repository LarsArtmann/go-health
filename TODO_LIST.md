# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.

> Curated 2026-09-04 (docs-health run, post-v0.1.1): the Pareto master plan
> (P1–P39) is complete and released; every item below was re-verified open
> against code on this date.

## Status legend

| Status      | Meaning                                                     |
| ----------- | ----------------------------------------------------------- |
| TODO        | Not started. Needs doing.                                   |
| IN_PROGRESS | Actively being worked on.                                   |
| BLOCKED     | Cannot proceed, external dependency or decision needed.     |

## High Impact

| Task                                                           | Status  | Impact | Effort | Evidence                                                                                    |
| -------------------------------------------------------------- | ------- | ------ | ------ | ------------------------------------------------------------------------------------------- |
| Verify pkg.go.dev renders v0.1.1 (incl. `Deprecated` section)   | TODO    | High   | 5min   | Still 404 at ~1h post-tag (propagation lag); proxy already resolves v0.1.1. `docs/status/2026-09-04_13-17*` f1 |
| Bump `go-health-dashboard` to v0.1.1 (drop its replace directive) | BLOCKED | High | 30min  | First real downstream consumer bump; needs owner sign-off on that repo's cadence. 13-17 g3   |
| Branch protection on `master`: require the 4 CI checks + linear history | BLOCKED | High | 10min | Needs owner/admin repo settings. `.github/workflows/ci.yml` header; 13-17 f9            |

## Medium Impact

| Task                                                          | Status | Impact | Effort | Evidence                                                                                       |
| ------------------------------------------------------------- | ------ | ------ | ------ | ----------------------------------------------------------------------------------------------- |
| Fake-clock determinism test for the live throttle (`WithLiveThrottle` freshness seam) | TODO | Med | 30min | B100 covered uptime/Timestamp only; throttle freshness has no dedicated test. `handlers.go` (`throttledLiveResponse`) |
| Collapse internal `getOnly bool` into the `WithAllowedMethods` method set | TODO | Med | 30min | Guard reads two flags; behavior-preserving simplification. `probe.go:242` (`guard`), `config`     |
| Enumerate the uncovered ~1.5% coverage functions; cover or document each | TODO | Med | 30min | 98.5% is recorded but the gap was never itemized. `nix run .#coverage`                          |
| Persist a repeatable doanalyzerv2 runner (tools/ script or CONTRIBUTING one-liner) | TODO | Med | 30min | Two sessions rebuilt it ad-hoc in /tmp. AGENTS.md "doanalyzerv2" note                            |
| CONTRIBUTING: sanitized-PATH "CI emulation" step in the release checklist | TODO | Med | 15min | First CI run failed on a host-env leak local gates masked. `docs/status/2026-09-04_13-17*` d1   |
| Add SECURITY.md (vuln disclosure path for a public module)     | TODO   | Med    | 15min  | Absent. 13-17 f13                                                                               |
| Add PR template (issue templates exist)                        | TODO   | Med    | 15min  | `.github/ISSUE_TEMPLATE/` exists; `.github/PULL_REQUEST_TEMPLATE.md` does not. 13-17 f15        |
| Fuzz the `aggregate` package (merge-on-read invariants); add `instance_id`-populated fuzz seeds | TODO | Med | 45min | Fuzzing covers the root package only. `handlers_fuzz_test.go`; 13-17 f21/f25                     |
| Godoc examples: `ExampleNewWithHealthCheck`, `ExampleProbe_Healthz`, `ExampleWithEvaluationHook`, `ExampleProbe_AwaitReady` | TODO | Med | 45min | Discoverability of the programmatic API. 13-17 f26; 11-13 #38                                    |

## Low Impact

| Task                                                             | Status | Impact | Effort | Evidence                                                                              |
| ---------------------------------------------------------------- | ------ | ------ | ------ | --------------------------------------------------------------------------------------- |
| Examples: custom `HealthRecorder`, two-phase shutdown, live-vs-cached | TODO | Low  | 60min  | Patterns live only in tests/docs; promote to `example_test.go`. ROADMAP Theme 1            |
| README compatibility matrix (tested Go × samber/do versions)     | TODO   | Low    | 15min  | 13-17 f28                                                                                |
| Deprecation policy doc (how long deprecated symbols live; v1.0 promises) | TODO | Low | 30min | `WithGETOnly` removal timeline currently lives only in godoc. 13-17 f29                     |
| Benchmark: method-set guard overhead (allowed vs unlisted vs no-guard) | TODO | Low | 30min | 13-17 f22                                                                               |
| CONTRIBUTING: "adding a new Option" checklist                    | TODO   | Low    | 15min  | 11-13 f41                                                                                |
| Property-based JSON round-trip test (unmarshal → marshal identity) | TODO | Low  | 30min  | Complements the golden snapshot. 13-17 f48                                               |
| Optional CI: validate `docs/openapi.yaml` (spectral/redocly) so it can't drift | TODO | Low | 30min | Spec is kept in lockstep only by convention today. 13-17 f30                             |
| Document `WithLiveThrottle` × `Start`-populated cache interaction | TODO  | Low    | 15min  | Throttle only matters on cache miss; worth one paragraph. 11-13 f43                       |

## Blocked on user decisions

| Decision                                                          | Why it blocks                                                                          |
| ----------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| SA1019 policy for in-repo use of deprecated symbols (pin-tests vs nolint) | 4 tests intentionally pin the deprecated `WithGETOnly`; no recorded lint policy. 13-17 f12 |
| Optional CI jobs: coverage threshold (fail < 97%?) and dependabot auto-merge rules | Policy calls, not defaults. 13-17 f16/f17                                              |
| v0.2.0 scoping, v1.0 criteria draft, release automation evaluation, v0.1.1 announcement | Strategic; tracked in ROADMAP until refined. 13-17 f43–f46                        |
