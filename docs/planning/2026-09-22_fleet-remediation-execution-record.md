# Fleet Remediation — Execution Record (2026-09-22)

**Parent plan**: [fleet-dependency-currency-remediation](2026-09-18_13-54_fleet-dependency-currency-remediation.md) · **Outcome**: plan executed to ~100% of the defined result.

## What shipped

| Area                        | Outcome                                                                                                                                                                                                                                                                                   |
| --------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Compile hazards             | Both fixed: library-policy test migrated to `KeyedRateLimiterMiddleware` (`a9c643a`); nsfw-classifier swapped to `etag.New` + v1.2.0 (`2f91fe9`)                                                                                                                                          |
| Pre-v1.0 waves (11 modules) | All on httputil ≥ v1.2.0; go-etag lifted to v0.3.1+ fleet-wide via MVS                                                                                                                                                                                                                    |
| v1.1.1 sweep (22 modules)   | 19 committed green; plugmarket/server + RedditParse carry verified pre-existing test failures; auto-deduplicate blocked by a pre-existing unfinished refactor (adapters reference a removed `Duplicate` API) — its json/v2 import restoration committed separately (`0a74adb`, `36b6cb7`) |
| Standup-Killer              | Unblocked by go-github-kit v0.3.1 (parallel session; proxy-verified) and bumped (07c7e97)                                                                                                                                                                                                 |
| Releases                    | go-etag v0.4.0 (parallel session), go-github-kit v0.3.1 (parallel session), httputil v1.3.0 + new `etagmetrics/v0.1.0` sub-module — all four tags verified on the module proxy                                                                                                            |
| C10 compression             | Identity `AbsentEncoding` default adopted fleet-wide; decision record: [compression review](2026-09-22_compression-absent-encoding-review.md)                                                                                                                                             |
| C14/C15                     | `etagmetrics` sub-module (Attach/Counters/HitRatio/Snapshot, hook-chaining, overflow contract pinned by test, benchmarks); CHANGELOG [1.3.0] + backfilled [1.1.1] docs-only note                                                                                                          |
| C16 Metrics adoption        | blog (`d815e12`) + crush-daily (httputil.Metrics over the existing Prometheus registry, legacy counter preserved); adoption snippet already canonical in `docs/integrations/prometheus-metrics.md`                                                                                        |
| C17 docs                    | Both audit reports carry actioned-status banners linking here                                                                                                                                                                                                                             |

## Final verification (M69)

- Fleet grep: **85 modules current** on httputil v1.2.0/v1.3.0, **0 genuine laggards** (one grep-format false positive: sales-landing-page/cloud-run pins v1.2.0 on a single-line require).
- go-etag: **0 modules below v0.3.1**.
- Both audits' stale-module tables re-derived to zero; both compile hazards reproduce as fixed.

## Discovered & left for follow-up (not regressions)

1. **auto-deduplicate**: unfinished architecture refactor leaves adapters/services uncompilable (stripped imports, references to a moved `Duplicate` API). Needs a proper repair pass.
2. **games/SEC**: pre-existing `GET /docs` route collision with go-cqrs-lite's docserver (51/65 integration specs) — needs a product decision on which route wins.
3. **crush-daily**: 3 pre-existing CLI dispatch test failures (env-dependent `crush projects` exec), verified identical pre/post upgrade.
4. **accountability-system**: BDD suite half-implemented (57 undefined steps) + 97 standing lint findings; host-toolchain hooks fail independent of dependency state.
