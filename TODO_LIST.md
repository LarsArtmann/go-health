# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.

> Re-curated 2026-09-04 22:30 (post-v0.1.3 release): the slash-name contract
> shipped as v0.1.3 (tagged, released, proxy-verified; dashboard bumped);
> completed rows
> deleted per lifecycle (they live in CHANGELOG), the "Resolved" traceability
> section removed, and the open items from the freshest status report
> (`docs/status/2026-09-04_21-31_pareto-plan-v2-full-execution-v012-released.md`
> §b/§f) harvested in. v0.1.2 is released; the aggregate slash-name contract
> sits unreleased in CHANGELOG `[Unreleased]` awaiting its release vehicle.
>
> Harvested 2026-09-16: issue #2 (per-check Since/DurationNanos) closed
> after verification (gates green, fuzz re-seeded, e2e omitzero test added,
> analyzer 0 findings, dashboard consumer build verified). Remaining
> follow-ups from `docs/status/2026-09-15_06-56_issue-2-per-check-metadata-session.md`
> §f routed into the tables below. Release vehicle decided same day:
> shipped as v0.2.0 (owner decision, tag + proxy-verify per go-release skill).
>
> Note 2026-09-18 (hardening pass): aggregate merge property tests,
> aggregate HTTP / Evaluate-scaling / tracker-stamp benchmarks, ADR-005, the
> OpenAPI aggregate coverage, the README "which probe?" decision table, and
> the `ExampleNewWithDetailedCheck` label fix shipped (see CHANGELOG
> `[Unreleased]`).
>
> Harvested 2026-09-22: the resolved 2026-09-18 rows (ADR-005, OpenAPI
> aggregate coverage, aggregate merge property tests, README decision
> table, `ExampleNewWithDetailedCheck` determinism) verified in code and
> deleted per lifecycle (they live in CHANGELOG). pkg.go.dev render
> verification passed the same day (v0.2.0 root + aggregate; v0.3.0
> root + aggregate + federation all render metadata fields and examples;
> proxy @latest = v0.3.0). Fixed stale CHANGELOG version-link block.
> New task surfaced: implement `Aggregate.Healthz()` per
> `docs/aggregate-healthz-design.md` (v0.3.0 candidate).

## Status legend

| Status      | Meaning                                                 |
| ----------- | ------------------------------------------------------- |
| TODO        | Not started. Needs doing.                               |
| IN_PROGRESS | Actively being worked on.                               |
| BLOCKED     | Cannot proceed, external dependency or decision needed. |

## High Impact (owner decisions — blocked)

| Task                                                             | Status  | Impact | Effort | Evidence                                                                                                                                                                                                                        |
| ---------------------------------------------------------------- | ------- | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Enable branch protection on `master` (5 checks + linear history) | BLOCKED | High   | 10min  | Needs owner/admin repo settings (decision G3). Ready-to-run command below. ⚠️ required status checks block direct pushes for non-admins; `enforce_admins: false` keeps your admin bypass. See `.github/workflows/ci.yml` header. |
| Coverage-threshold CI job (fail < 97%)?                          | BLOCKED | Medium | 20min  | Policy call (decision G3 follow-up). CONTRIBUTING states the 99.7% baseline; a red-failing threshold job is a maintainer preference, not a default.                                                                             |

### Ready-to-run: branch protection (G3)

```bash
gh api -X PUT repos/LarsArtmann/go-health/branches/master/protection --input - <<'JSON'
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "Test (race)",
      "Vet + Lint",
      "Security (govulncheck + gosec)",
      "Flake + Formatting",
      "OpenAPI spec"
    ]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": null,
  "restrictions": null,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false
}
JSON
```

Check names are the exact `name:` fields CI reports. To revert:
`gh api -X DELETE repos/LarsArtmann/go-health/branches/master/protection`.

## Owner Actions (artifacts ready, publishing is yours)

| Task                                   | Status | Impact | Effort | Evidence                                                                              |
| -------------------------------------- | ------ | ------ | ------ | ------------------------------------------------------------------------------------- |
| Publish the v0.1.1/v0.1.2 announcement | TODO   | Low    | 15min  | Draft + channels checklist ready in `docs/announcements/2026-09-04_v0.1.1-v0.1.2.md`. |

## Open — unblocked (any session can pick these up)

| Task                                                                                    | Status | Impact | Effort | Evidence                                                                                                                                                              |
| --------------------------------------------------------------------------------------- | ------ | ------ | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Trigger `Fuzz (weekly long)` once via `workflow_dispatch`                               | DONE   | Medium | 5min   | Dispatched 2026-09-22 (run 35756511889); scheduled runs 09-14 + 09-21 already green, YAML proven on GitHub.                                                            |
| Verify pkg.go.dev renders v0.2.0 (metadata fields + examples + aggregate visible)       | DONE   | Medium | 10min  | Verified 2026-09-22: v0.2.0 root+aggregate and v0.3.0 root+aggregate+federation all render; proxy @latest = v0.3.0 (released 2026-09-19).                     |
| OpenAPI ↔ golden-file lockstep check in CI (currently a manual eyeball)                 | TODO   | Low    | 45min  | The golden test and `docs/openapi.yaml` can drift silently. `21-31` §f23.                                                                                             |
| Dashboard: render "failing since HH:MM (Nm)" from `check.since`                         | TODO   | High   | 2h     | Wire fields shipped (unreleased); dashboard renders placeholders. Own repo: `go-health-dashboard`. `09-15` §f2.                                                       |
| Dashboard: status-changes timeline from `since` (kill sampling-clock guess)             | TODO   | High   | 4h     | Same wire source. `09-15` §f3.                                                                                                                                        |
| Dashboard: render `duration_ns` adaptively (µs/ms), hide when absent                    | TODO   | Medium | 1h     | `09-15` §f4.                                                                                                                                                          |
| Dashboard: "stable for Xh" collapse summaries for healthy groups                        | TODO   | Medium | 2h     | `09-15` §f5.                                                                                                                                                          |
| `BenchmarkEvaluate` before/after tracker + FEATURES.md delta; re-baseline existing rows | TODO   | Medium | 1h     | Since stamping adds one mutex'd batch pass; unmeasured. `09-15` §f8–9.                                                                                                |
| File samber/do upstream: richer batch results (per-service timing)                      | TODO   | Medium | 30min  | Would let the injector path populate `duration_ns` (currently zero). `09-15` §f11.                                                                                    |
| samber-do-auditlog: implement `DetailedHealthRecorder` (first implementor)              | TODO   | Medium | 1h     | It already times checks internally. `09-15` §f12.                                                                                                                     |
| Dashboard integration test pinning `since`/`duration_ns` rendering                      | TODO   | Low    | 45min  | After dashboard adopts. `09-15` §f24.                                                                                                                                 |
| Detailed-checks cookbook (self-timing + `NewWithDetailedCheck` composition)             | TODO   | Low    | 1h     | README or docs/. `09-15` §f22–23.                                                                                                                                     |
| Prose review: `middleware_example_test.go` / `prometheus_example_test.go` wire examples | TODO   | Low    | 20min  | They pass; docs-only sweep. `09-15` §f27.                                                                                                                             |
| golangci `nolint_filter` warning for `//nolint:erraudit` (unknown linter)               | TODO   | Low    | 15min  | erraudit is a standalone tool honoring `//nolint`; golangci doesn't know it (warning, exit 0). Accept or find a suppression path. Found 2026-09-16.                   |
| ADR: unify latency units (`total_latency_ms` vs `duration_ns`) in v0.3                  | TODO   | Low    | 30min  | Dual units forever or one breaking unification while alpha. `09-15` §f25.                                                                                             |
