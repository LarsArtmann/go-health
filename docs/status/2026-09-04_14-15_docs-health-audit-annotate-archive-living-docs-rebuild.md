# Status Report — 2026-09-04 14:15 — Docs-Health Audit: Living-Docs Rebuild, ~200 Annotations, 1 Archive

> 2026-09-04 14:15 CEST · Point-in-time snapshot of the docs-health run triggered by
> "View ALL \*\*/2026-0\* files! Execute the docs-health SKILL!" — a full AUDIT
> (VERIFY + HARVEST + BUILD + ANNOTATE + ARCHIVE) over all 10 `**/2026-0*` files,
> the six living docs, and DOMAIN_LANGUAGE.md. Predecessor:
> [13-17 release report](2026-09-04_13-17_v011-release-deprecation-and-first-green-ci.md).
>
> **Format note:** the `status-report` skill's canonical output is a styled HTML
> dashboard; the user explicitly requested `.md` — honored per the skill's
> override rule.

## TL;DR

All six living docs were verified against code and brought to truth: three
Critical accuracy bugs fixed (version split brain `v0.0.2`/`v0.1.0`/`v0.1.1`
across AGENTS/README, FEATURES claiming CI "never executed" against four green
runs, FEATURES claiming security scans "not wired into CI"), a dead
Go Report Card badge removed (the service has been **sunset** — a genuinely new
drift class: badge rot), TODO_LIST rebuilt open-only (DONE rows and the
"Resolved this cycle" trophy section deleted), CHANGELOG put into canonical
category order with the link block moved to the bottom, DOMAIN_LANGUAGE
line-refs refreshed (+Aggregate, +Deprecation terms), `doc.go` given its
missing programmatic-API/observability sections, and the aggregate `InstanceID`
merge rule documented. HARVEST pulled the freshest report's open items into
TODO_LIST (20 bounded rows) and ROADMAP (Theme 7). ~200 inline `done at <hash>`
verdicts across all six historical files; the **09-03 audit report is fully
resolved and archived** (`git mv` → `docs/status/archived/`); the Pareto master
plan is stamped **COMPLETED**. Gates all green. Working tree clean (daemon
committed in 9 commits during the run).

---

## a) FULLY DONE

| # | Task | Evidence |
| - | ---- | -------- |
| 1 | **All 10 `**/2026-0*` files viewed in full** before any edit (3 already-archived + 6 status reports + 1 planning file) | this session |
| 2 | **Full docs-health skill loaded**: SKILL.md + doc-ownership, harvest-guide, verify-checklist, health-report-format, annotation-placement, resolving-items, build-guide, agents-quality-guide | `~/.config/crush/skills/docs-health/` |
| 3 | **VERIFY against code**: 13 options counted (`grep '^func With'`), all 8 benchmarks enumerated, CI runs verified via `gh run list` (4× success incl. the v0.1.1 tag run), dependabot covers gomod+actions, `aggregate.go` merge code read (InstanceID dropped), `.gitignore` covers `*.out`, flake apps enumerated (fuzz exists), pkg.go.dev + goreportcard fetched live | this session |
| 4 | **New external finding: goreportcard.com is sunset** — the README badge pointed at a dead service | fetch of goreportcard.com |
| 5 | **README**: stability line v0.1.0 → v0.1.1; dead badge removed; new **Metrics** and **Middleware** sections; 2 new troubleshooting entries ("405 with Allow header", "pkg.go.dev shows old docs"); Project Docs lists all 10 design notes + openapi.yaml + ADRs | `1d0e1ea`, `e366fcc` |
| 6 | **AGENTS.md**: status line v0.1.1; `nix run .#fuzz` added to commands; doanalyzerv2/consumer-verification paragraphs de-dated to current truth (endurance test); Project Documentation table +7 design notes; benchmark list +2 (`StartupHandler_Contention`, `CachedResponse_ParallelReads`) | `e366fcc` |
| 7 | **FEATURES.md**: CI row corrected to "green on master and v0.1.1 tag"; security row corrected to "enforced by CI jobs"; consumer row moved to the status vocabulary with updated note | `e366fcc` |
| 8 | **TODO_LIST rebuilt open-only**: 3 struck DONE rows deleted, "Resolved this cycle" section deleted, 20 verified-open rows across High/Medium/Low, new "Blocked on user decisions" table; every row re-verified against code | `9219ebc` |
| 9 | **ROADMAP**: new Theme 7 "Release & Ecosystem Strategy" (v0.2.0 scoping, v1.0 criteria, release automation, erraudit/doanalyzerv2-in-CI-if-public, Go patch tracking); new raw ideas: cache-aware `AwaitReady` poll, aggregate `Healthz` parity, `Probe.Snapshot()` accessor | this session |
| 10 | **CHANGELOG**: `[0.1.1]` reordered to Keep-a-Changelog canon (Added → Changed → Deprecated → Fixed); compare/release link definitions moved from mid-file to the bottom; content otherwise untouched (append-only respected) | `e366fcc` |
| 11 | **DOMAIN_LANGUAGE.md**: ~19 stale `file:line` refs re-verified and refreshed; **Aggregate** and **Deprecation** terms added | `9219ebc` |
| 12 | **`doc.go` refreshed** (+43 doc lines): Programmatic Health API (`Status`/`Alive`/`Ready`/`AwaitReady`/`Healthz`/`NewWithHealthCheck`/do-conformance), Observability (hook), Live Evaluation and Throttling, Method-Set Guard, Instance Identity — closes 13-17 f5; `go build` + full tests green after | `1d0e1ea` |
| 13 | **`aggregate/aggregate.go` doc comment** now states Version, Uptime, **and InstanceID** are per-process scalars that do not survive a merge (verified against the merge code) — closes 13-17 f23 | `1d0e1ea` |
| 14 | **HARVEST**: 13-17's 50-item NEXT list swept — 12 verified done in code (dropped), 17 routed into TODO_LIST, 5 strategic items graduated to ROADMAP Theme 7, moot items recorded as moot | this session |
| 15 | **Planning file stamped COMPLETED** with pointers to both execution reports | this session |
| 16 | **11-13 marathon report closed out**: stale "working tree is currently red" TL;DR claim inline-corrected; all 50 f-items resolved inline; end-of-file Completion section added | this session |
| 17 | **09-12 report**: 37 routed items topped up with `done at <hash>` or explicit `Won't implement` + design-note citations | this session |
| 18 | **19-01 report**: 41 routed items resolved (incl. its still-open consumer question) | this session |
| 19 | **19-11 report**: 43 routed items resolved (c-section rows, f-list, improvement items) | this session |
| 20 | **09-03 audit report fully resolved and ARCHIVED**: b-table gaps, c-section, 24 f-table verdicts via the skill's `annotate-rows.py` (dry-run first, shape-verified), g-questions resolved; `git mv` → `docs/status/archived/` | this session |
| 21 | **13-17 release report annotated** where THIS run closed items: f1–f8 Now items, b-partials 1/3/6, c-items 3/4/8, f14/35–40/42 (22 verdicts; genuinely-open items deliberately left unmarked) | this session |
| 22 | **Skill-mandated tooling used with mandatory dry-run** (`annotate-rows.py`) — the exact anti-pattern the 09-03 report confessed (d1: hand-rolled annotations) was consciously avoided after one early slip (see d3) | this session |
| 23 | **Gates all green**: `nix run .#test` ok (root + aggregate) · `.#lint` 0 issues · `nix fmt` 0 changed · `nix flake check` pass · all 16 files linked from README/AGENTS verified to exist | this session |
| 24 | **TODO_LIST Low row "top up hash citations in 19-01/19-11" closed by the sweep itself** — removed from TODO_LIST during the rebuild | `9219ebc` |
| 25 | **pkg.go.dev re-checked**: v0.1.1 page still propagating (404) while the proxy resolves the version — re-verified rather than assumed, and honestly kept open in TODO_LIST | fetch of pkg.go.dev |

## b) PARTIALLY DONE

1. **13-17 report annotation** — only the items this run actually closed were
   marked (22 of ~50 f-items). The rest are genuinely open; per ANNOTATE
   discipline absence IS the open signal. Full resolution of that file's list
   happens as the backlog executes, not in one pass.
2. **User decisions collected but not resolved** — the 13-17 g-questions
   (branch protection, `WithGETOnly` removal timeline, dashboard consumer bump)
   plus SA1019 policy and optional CI jobs are routed into TODO_LIST's
   "Blocked on user decisions" table; none have answers yet.
3. **Health-score findings table** — computed inline in the conversation with
   visible math (pre-fix 3.0/7.8 → post-fix 10/10) but the per-file finding
   list lives in the conversation only; this report carries the summary, not a
   full duplicate.
4. **Hash attribution granularity** — items executed inside the 13-17 session
   landed via daemon heuristic commits; where a precise per-item hash would
   have required git archaeology, verdicts cite the identifiable commits
   (`5a79775`, `d20f481`, `ea5dda0`, `eb67f1c`, …) or "covered by 13-17 §a".
   Grammar-compliant (the `done (covered by X)` variant) but less precise than
   ideal.
5. **This report vs the skill's HTML dashboard** — the skill's canonical output
   is styled HTML; user explicitly requested `.md`, so the stat-cards/badge
   treatment was not applied. Flagged here per the skill's override rule.

## c) NOT STARTED

All deliberate scope exclusions for this docs-only run (user: "DO NOT RESEARCH
OTHER STUFF UNRELATED"). Everything below is already tracked in TODO_LIST /
ROADMAP with evidence:

- Code/tests: fake-clock throttle determinism test; `getOnly bool` collapse
  into the method set; coverage-gap enumeration (~1.5%); aggregate fuzz
  targets + `instance_id` seeds; godoc examples (`NewWithHealthCheck`,
  `Healthz`, `WithEvaluationHook`, `AwaitReady`); method-set-guard benchmark;
  JSON round-trip property test; the three promoted examples (custom recorder,
  two-phase shutdown, live-vs-cached); aggregate coverage gaps.
- Infra/repo: persisted doanalyzerv2 runner; SECURITY.md; PR template;
  sanitized-PATH release-checklist step in CONTRIBUTING; "adding a new Option"
  checklist; openapi.yaml CI validation; README compat matrix; coverage-threshold
  and dependabot-auto-merge jobs (optional, policy).
- User decisions: branch protection; dashboard bump to v0.1.1; SA1019 policy;
  deprecation removal timeline; v0.2.0 scoping; v1.0 criteria; release
  automation; v0.1.1 announcement.
- External: pkg.go.dev v0.1.1 render verification (propagation lag — already
  >1h; re-check later).
- HTML status dashboard variant (superseded by explicit `.md` request).

## d) TOTALLY FUCKED UP

| # | Issue | Impact |
| - | ----- | ------ |
| 1 | **First 09-12 multiedit: 12 of 13 edits applied, 1 silently failed** (the 27-line f-list block). Never identified the exact byte difference — individual lines all matched, the whole block didn't (likely an invisible whitespace/line-wrap artifact from the formatter). Rescued by switching to a strict exact-line replacer with `count == 1` + read-back verification. | One wasted round trip; a lesson that whole-block matching is fragile on formatter-managed files. |
| 2 | **My read-back assertion was logically wrong**: it required `old not in file` after replacement, but strikethrough *embeds* the old text (`~~old~~ done …`), so a successful batch aborted with "READBACK FAIL". The write had already happened; I re-ran the batch idempotently. A check meant to catch corruption flagged success as failure. | One wasted cycle; brief double-application risk (neutralized by the idempotent re-run). |
| 3 | **Repeated the 09-03 d1 anti-pattern once**: for the 09-03 Medium rows I started hand-rolling annotation logic in a python heredoc (with a broken placeholder loop) instead of going straight to `annotate-rows.py`. Caught it mid-script, aborted before any write, dry-ran the mandated script, and it handled the file perfectly. One session after the exact mistake was documented in the very file I was annotating. | One aborted attempt; zero corruption; the skill's "do not hand-roll" rule is now proven, not just believed. |
| 4 | **Edited AGENTS.md from context-memory**: first multiedit rejected with "read the file first" because I hadn't Viewed it this session (the content came from the project-context snapshot). Mechanical rule violation. | One round trip. |
| 5 | **Two failed edits on the 13-17 report**: (a) modified-since-read rejection — `annotate-rows.py` had rewritten the file between my read and my edit; (b) a guessed line-wrap in f-items 1–8 that didn't match the formatter's reflow. Both fixed by re-reading exact bytes first. | Two round trips. |
| 6 | **README Metrics snippet uses undefined helpers** (`healthGauge`, `checkGauge`, `statusValue`). It is deliberately illustrative and links the compiled `prometheus_example_test.go`, but strict readers may try to copy it verbatim. | Cosmetic-to-minor; flag for a possible tightening. |
| 7 | **Scope creep temptation at the fresh-report boundary**: the 13-17 f-list contains ~30 still-open items that would each be "quick"; annotating them as done would have been fabrication. Discipline held (absence = open), but the temptation itself shows why HARVEST and ANNOTATE must stay separate directions. | None — noted as a guard for future runs. |

## e) WHAT WE SHOULD IMPROVE

1. **Tool precedence for strike-through batches on formatter-managed files**:
   `annotate-rows.py` / `annotate-prose.py` → exact-line python with `count==1`
   + new-text-only read-back → `multiedit` last. Whole-block `old_string`
   matching silently fails on re-wrapped tables and lists.
2. **Idempotency-first scripts**: check `new in file` before applying, skip
   already-applied replacements, and assert only `new in file` afterward when
   the new text embeds the old. The read-back check must model strikethrough
   semantics.
3. **Two-phase annotation is now proven practice**, not just the 09-03
   recommendation: living-doc edits land via daemon commits → annotate citing
   the real hashes (`1d0e1ea`/`e366fcc`/`9219ebc` cited in 13-17 within the
   same session). Keep this ordering for every future docs run.
4. **Temporal-pollution detection should include verification stamps**:
   "doanalyzerv2 verified (2026-09-04)" and "Consumer verification (2026-09-04)"
   survived the previous audit because the grep targets "audited/added/FIXED
   20…" but not "verified (date)". Add `verified? \(?20\d\d` to the sweep.
5. **Badge/external-service rot is a new drift class**: goreportcard's sunset
   made a README badge lie while every repo-internal fact stayed true. VERIFY
   should probe external claims (badges, pkg.go.dev, proxy) with real fetches —
   this run caught it exactly that way.
6. **Routings are pointers and can dangle**: "→ TODO_LIST"/"→ ROADMAP" markers
   in old reports kept pointing at items that later shipped. When an item
   ships, sweep *both* directions: the target file AND the historical routing.
7. **doc.go belongs on the release version/API-sync checklist** — it was the
   release session's own f5 and was missed by that session's sweep; only this
   run closed it. The checklist in 13-17 e3 should name doc.go explicitly.
8. **Use the annotate scripts FIRST for table-shaped reports** — they are
   atomic, refuse already-annotated rows, and shape-verify on write; this run
   proved it on the 09-03 f-table after wasting cycles elsewhere.
9. **Trophy decay regrows between curations**: TODO_LIST was rebuilt clean on
   09-03 and had 3 DONE rows + a trophy section again one day later (the
   release session's closings). Curation must be part of every session-ending
   report, not a periodic special task.

## f) Up to 50 Things We Should Get Done Next (priority-ordered)

> Items 1–20 are the harvested TODO_LIST (verified open, evidence cited there).
> Items 21–30 surfaced during this run and are report-new. Items 31–50 are
> strategic/ROADMAP fuel and hygiene; most need user input or are polish.

**Now — release hygiene & external truth**

1. Verify pkg.go.dev renders v0.1.1 (incl. `Deprecated` section) — still 404 at 14:00 CEST.
2. Owner decision: branch protection on `master` (4 CI checks + linear history).
3. Bump `go-health-dashboard` to v0.1.1 (drop its replace directive).
4. Owner decision: `WithGETOnly` removal timeline / deprecation policy doc.
5. Owner decision: SA1019 policy for in-repo pinning of deprecated symbols.
6. Fake-clock determinism test for the live-throttle freshness seam.
7. Persist a repeatable doanalyzerv2 runner (tools/ + CONTRIBUTING one-liner).
8. CONTRIBUTING: add the sanitized-PATH "CI emulation" step to the release checklist.
9. SECURITY.md (disclosure path for the public module).
10. PR template (issue templates exist; PRs don't).
11. Collapse internal `getOnly bool` into the `WithAllowedMethods` method set.
12. Enumerate the uncovered ~1.5% coverage; cover or document each.
13. Fuzz the `aggregate` package; add `instance_id`-populated fuzz seeds.
14. Godoc examples: `ExampleNewWithHealthCheck`, `ExampleProbe_Healthz`, `ExampleWithEvaluationHook`, `ExampleProbe_AwaitReady`.

**Docs & guides (this run's leftovers)**

15. Tighten the README Metrics snippet (define or pseudonymize the helpers).
16. CONTRIBUTING: document the docs-health two-phase annotation workflow and the annotate scripts.
17. CONTRIBUTING/AGENTS: extend the version/API-sync checklist with `doc.go` + a badge-liveness check.
18. AGENTS gotcha: "external badges can rot (goreportcard sunset)" — record once, prevent re-adds.
19. Document `WithLiveThrottle` × `Start`-populated cache interaction (throttle matters only on cache miss).
20. Aggregate: fill coverage gaps (error paths in `New`, partial startup latch).

**Quality & polish**

21. Examples: custom `HealthRecorder`, two-phase shutdown, live-vs-cached → `example_test.go`.
22. README compatibility matrix (tested Go × samber/do versions).
23. Benchmark: method-set guard overhead (allowed / unlisted / no-guard).
24. CONTRIBUTING: "adding a new Option" checklist.
25. Property-based JSON round-trip test (unmarshal → marshal identity).
26. Optional CI: validate `docs/openapi.yaml` (spectral/redocly) against drift.
27. Optional CI: coverage-threshold job (fail < 97%?) — policy decision.
28. Optional: dependabot auto-merge rules for patch bumps.
29. Aggregate `Healthz` parity decision (single worst-of-N combined endpoint?).
30. `AwaitReady` cache-aware poll interval (respect refresh interval vs fixed 50ms).

**Strategic (ROADMAP Theme 7; needs product input)**

31. v0.2.0 scoping: feature-driven vs date-based cadence.
32. v1.0 criteria draft (API freeze list, stability promises).
33. Deprecation burn-down plan (starts with `WithGETOnly`).
34. Release automation: actions-based tag→release (GoReleaser judged overkill).
35. Promote erraudit/doanalyzerv2 into CI if they ever become public.
36. Track Go 1.26.x patch releases in the flake input (dependabot may cover).
37. v0.1.1 announcement channel (user-owned marketing).
38. `Probe.Snapshot()`-style accessor for structured-logging consumers (only with a concrete need).
39. OpenTelemetry spans on Evaluate/checks (via the hook seam).
40. `Response.TotalLatencyMs` as `float64` (revisit only with consumer need).

**Hygiene & habit**

41. Add "verified (date)" stamps to the temporal-pollution grep in future VERIFY passes.
42. Add external-claim probing (badge/pkg.go.dev/proxy fetches) to every docs-health VERIFY.
43. Sweep routing markers (`→ TODO_LIST/ROADMAP`) in the same pass as TODO_LIST rebuilds.
44. Keep session-ending TODO_LIST curation mandatory (trophy decay regrows in one day).
45. Prefer `annotate-rows.py`/`annotate-prose.py` before any hand-rolled annotation tooling.
46. Re-check the gopls stdversion warnings after the next toolchain bump (AGENTS gotcha current as of today).
47. Re-run consumer verification after every future API batch (dashboard compile via replace).
48. Keep `erraudit ./... --type-aware` in the personal gate checklist (documented; not CI).
49. Watch the first dependabot PRs (config verified; behavior not yet observed).
50. Celebrate: the docs are now truthful end-to-end — six living docs, ten historical files, zero known inaccuracies at 14:15 CEST.

## g) Questions I Cannot Answer Myself

1. **Branch protection**: shall the 4 CI checks + linear history be required on
   `master`? It needs owner/admin repo settings either way — shall it be
   configured now, or left until the repo has external contributors?
2. **Dashboard consumer bump**: shall `go-health-dashboard` be upgraded to
   v0.1.1 (drop its replace directive) as the first real downstream consumer
   release now, or is that repo's cadence handled separately?
3. **Deprecation removal timeline**: the godoc currently promises "no removal
   in the v0.x line" for `WithGETOnly` — is that the final answer, or should a
   deprecation policy pin a concrete version (v0.2? v1.0?) for removal?

---

*Report ends. Awaiting instructions.*
