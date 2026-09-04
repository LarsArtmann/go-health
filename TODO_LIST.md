# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.

> Curated 2026-09-04 (docs-health run, post-v0.1.1); re-harvested 2026-09-04
> (evening) after the Pareto master plan v2 executed end-to-end: v0.1.2 cut
> and released (instance_id fix), all 5 CI jobs green including the new
> OpenAPI job, aggregate slash-name contract locked (strict source names),
> dashboard bumped to v0.1.2 and synced, `.#gates` / `.#ci-emulation` /
> `.#fuzz-long` flake apps landed, aggregate golden + merge benchmark +
> throttle×Start tests added. Completed work lives in CHANGELOG, not here.

## Status legend

| Status      | Meaning                                                 |
| ----------- | ------------------------------------------------------- |
| TODO        | Not started. Needs doing.                               |
| IN_PROGRESS | Actively being worked on.                               |
| BLOCKED     | Cannot proceed, external dependency or decision needed. |

## High Impact

| Task                                                                 | Status  | Impact | Effort | Evidence                                                                                                                                                                                                                               |
| -------------------------------------------------------------------- | ------- | ------ | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Enable branch protection on `master` (5 checks + linear history)     | BLOCKED | High   | 10min  | Needs owner/admin repo settings (decision G3). Ready-to-run command below. ⚠️ required status checks block direct pushes for non-admins; `enforce_admins: false` keeps your admin bypass. See `.github/workflows/ci.yml` header.       |
| Coverage-threshold CI job (fail < 97%)?                              | BLOCKED | Medium | 20min  | Policy call (decision G3 follow-up). CONTRIBUTING states the 99.7% baseline; a red-failing threshold job is a maintainer preference, not a default.                                                                                    |

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

| Task                                                                          | Status  | Impact | Effort | Evidence                                                                                                             |
| ----------------------------------------------------------------------------- | ------- | ------ | ------ | -------------------------------------------------------------------------------------------------------------------- |
| Publish the v0.1.1/v0.1.2 announcement                                        | TODO    | Low    | 15min  | Draft + channels checklist ready in `docs/announcements/2026-09-04_v0.1.1-v0.1.2.md`.                                |

## Resolved 2026-09-04 (kept for traceability, remove freely)

- ~~Bump `go-health-dashboard` to v0.1.2~~ — done: requires released v0.1.2 directly (no replace), build + tests green, synced to origin.
- ~~SA1019 policy (pin-tests vs nolint)~~ — confirmed (G5): pin-tests stay, accepted findings are the documented stance (`docs/deprecation-policy.md`).
- ~~v0.2.0 scoping / v1.0 criteria / release automation / announcement draft~~ — done: ROADMAP Theme 7 now carries candidates, criteria, and the GoReleaser-vs-manual decision (manual adopted).
