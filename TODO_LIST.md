# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.

> Re-curated 2026-09-04 (evening, post-v0.1.2 docs-health run): completed rows
> deleted per lifecycle (they live in CHANGELOG), the "Resolved" traceability
> section removed, and the open items from the freshest status report
> (`docs/status/2026-09-04_21-31_pareto-plan-v2-full-execution-v012-released.md`
> §b/§f) harvested in. v0.1.2 is released; the aggregate slash-name contract
> sits unreleased in CHANGELOG `[Unreleased]` awaiting its release vehicle.

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
| Release vehicle for the aggregate slash-name contract            | BLOCKED | High   | 5min   | Owner veto window (21-31 §g1): the strict `aggregate.New` source-name rejection sits unreleased in CHANGELOG `[Unreleased]` — ship as v0.1.3 now, or let it ride in v0.2.0 with `errors.Join` + `SourceStatuses()`.             |

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

| Task                                                                         | Status | Impact | Effort | Evidence                                                                                                                                                              |
| ---------------------------------------------------------------------------- | ------ | ------ | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Trigger `Fuzz (weekly long)` once via `workflow_dispatch`                    | TODO   | Medium | 5min   | The weekly fuzz YAML has never executed on GitHub; a typo would only surface Monday 04:17 UTC. `21-31` §b. `.github/workflows/fuzz-long.yml` has `workflow_dispatch`. |
| Verify pkg.go.dev renders v0.1.2 (aggregate + examples visible)              | TODO   | Medium | 10min  | Proxy verified; the page itself was not fetched post-release. `21-31` §b.                                                                                             |
| ADR-005: promote the slash-name design note into the ADR series              | TODO   | Medium | 30min  | `docs/aggregate-source-name-design.md` is a design note; ADR-001..004 are the formal series. `21-31` §f13.                                                            |
| OpenAPI: state explicitly that aggregate endpoints are covered or scoped out | TODO   | Medium | 30min  | Same response shapes as the root probes, but `docs/openapi.yaml` is silent on `aggregate`. `21-31` §f12.                                                              |
| Aggregate property tests: merge idempotence + source-order commutativity     | TODO   | Medium | 45min  | Two reads of `CachedResponse` identical; merge result independent of source order. `21-31` §f16–17. `aggregate/aggregate.go`                                          |
| OpenAPI ↔ golden-file lockstep check in CI (currently a manual eyeball)      | TODO   | Low    | 45min  | The golden test and `docs/openapi.yaml` can drift silently. `21-31` §f23.                                                                                             |
| README: short "which probe should I hit?" decision table for newcomers       | TODO   | Low    | 30min  | Three-probe choice is explained in prose only. `21-31` §f25.                                                                                                          |
