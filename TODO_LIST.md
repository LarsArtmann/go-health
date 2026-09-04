# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.

> Curated 2026-09-04 (docs-health run, post-v0.1.1): the Pareto master plan
> (P1–P39) is complete and released; every item below was re-verified open
> against code on this date.
>
> Executed 2026-09-04 (same day): all 18 TODO items (High/Medium/Low) were
> completed and verified — pkg.go.dev v0.1.1 render (with `Deprecated`
> section), fake-clock throttle determinism test, `getOnly` collapse, coverage
> enumeration (now 99.7% / aggregate 100%), persisted doanalyzerv2 runner,
> CI-emulation + Option-checklist in CONTRIBUTING, SECURITY.md, PR template,
> aggregate fuzz target + instance_id fuzz seeds (found and fixed the
> `instance_id` UTF-8 sanitization bug), JSON round-trip property test, guard
> benchmark, godoc examples, README compatibility matrix, deprecation policy,
> openapi CI validation, and the `WithLiveThrottle` × cache documentation.
> Completed items are recorded in CHANGELOG `[Unreleased]`, not here.

## Status legend

| Status      | Meaning                                                 |
| ----------- | ------------------------------------------------------- |
| TODO        | Not started. Needs doing.                               |
| IN_PROGRESS | Actively being worked on.                               |
| BLOCKED     | Cannot proceed, external dependency or decision needed. |

## High Impact

| Task                                                              | Status  | Impact | Effort | Evidence                                                                                    |
| ----------------------------------------------------------------- | ------- | ------ | ------ | ------------------------------------------------------------------------------------------- |
| Bump `go-health-dashboard` to v0.1.1 (drop its replace directive) | BLOCKED | High   | 30min  | First real downstream consumer bump; needs owner sign-off on that repo's cadence. 13-17 g3   |
| Branch protection on `master`: require the 5 CI checks + linear history | BLOCKED | High | 10min | Needs owner/admin repo settings. `.github/workflows/ci.yml` header (now five jobs); 13-17 f9 |

## Blocked on user decisions

| Decision                                                                                | Why it blocks                                                                             |
| --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| SA1019 policy for in-repo use of deprecated symbols (pin-tests vs nolint)               | 4 tests intentionally pin the deprecated `WithGETOnly`; deprecation policy now documents the pin-test stance — confirm or override. 13-17 f12 |
| Optional CI jobs: coverage threshold (fail < 97%?) and dependabot auto-merge rules      | Policy calls, not defaults. CONTRIBUTING now states the 99.7% baseline; a CI threshold remains a policy call. 13-17 f16/f17 |
| v0.2.0 scoping, v1.0 criteria draft, release automation evaluation, v0.1.1 announcement | Strategic; tracked in ROADMAP until refined. 13-17 f43–f46                                  |
