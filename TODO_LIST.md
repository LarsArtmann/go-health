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
> Fuzz (weekly long) dispatched (run 35756511889; scheduled runs already
> green) — run completed successfully 2026-09-22 17:03 UTC. Shipped same day: `Aggregate.Healthz()` (design accepted,
> implemented) and the OpenAPI ↔ golden lockstep check
> (`nix run .#openapi-lockstep` + `checks.openapi-lockstep` under
> `nix flake check`). nolint_filter warning accepted + documented
> (upstream has no suppression gate; bare `//nolint` would over-suppress).
> New task surfaced: implement `Aggregate.Healthz()` per
> `docs/aggregate-healthz-design.md` (v0.3.0 candidate) — DONE, see
> CHANGELOG.
>
> Harvested 2026-09-22 (cont.): `BenchmarkEvaluate` before/after tracker
> measured (A/B seam: +~355 ns/+47%, +528 B, +2 allocs per `Evaluate`;
> evaluate/tracker rows re-baselined on go1.27.1 in FEATURES.md — see
> CHANGELOG `[Unreleased]`). All five dashboard rows (`09-15` §f2–f5, §f24)
> verified DONE in `go-health-dashboard` and deleted per lifecycle: it
> renders `since` ("since 14:02:05 UTC (17m)", `status.go`
> `rowMetadataTexts`), `duration_ns` adaptively with absent-when-unknown
> (`formatCheckDuration`), powers a status-change timeline
> (`history.go`), collapses healthy groups (`applyCollapsePolicy` +
> `TestCollapse_*`), and pins the rendering with a golden test +
> `timedScreenshotRecorder` — focused suite green 2026-09-22.

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
| Post samber/do#318 comment: per-service duration on `HealthOutcome` | TODO   | Medium | 5min   | Draft + verification notes + checklist ready in `docs/announcements/2026-09-22_samber-do-issue-318-duration-comment.md` (gates passed, voice-checked; filing is an owner call). |

## Blocked — upstream / cross-repo decisions

| Task                                                                        | Status  | Impact | Effort | Evidence |
| --------------------------------------------------------------------------- | ------- | ------ | ------ | -------- |
| samber-do-auditlog: implement `DetailedHealthRecorder` (first implementor)  | BLOCKED | Medium | 1h     | Owner decision required (verified 2026-09-22): the row's premise was false — auditlog times builds + shutdowns, NOT health checks. Implementing the interface requires importing go-health (`health.CheckDetail` return type), reversing the deliberate post-extraction "dependency-free both ways" decoupling (ADR-004), and it would silently switch go-health consumers from do's pooled batch to a hand-rolled fan-out (do's default semantics match, but `HealthCheckParallelism`/`HealthCheckTimeout` configs would be bypassed). The pattern is proven: go-health-dashboard's `timedScreenshotRecorder` implements the interface via `do.HealthCheckNamedWithContext` (`di_lifecycle.go:217`). |

## Open — unblocked (any session can pick these up)

Nothing open. All former rows are done (see CHANGELOG + harvest notes above),
owner-gated (Owner Actions), or decision-blocked (section above).
