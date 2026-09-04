# Status Report — 2026-09-04 22:15 CEST — Evening Docs-Health Run: ~175 Verdicts, 7 Archives, ROADMAP Re-Audit

> Point-in-time snapshot of the docs-health run triggered by "View ALL \*\*/2026-0\* files!
> Execute the docs-health SKILL!", continued by the "ROADMAP.md = SUPERB??" challenge and
> closed with this report. Predecessor:
> [21-31 Pareto-v2 execution report](2026-09-04_21-31_pareto-plan-v2-full-execution-v012-released.md)
> (left untouched by design — its open items are the live backlog).

## TL;DR

Full AUDIT (VERIFY + HARVEST + BUILD + ANNOTATE + ARCHIVE) over all 15 `**/2026-0*`
files and the six living docs. ~175 inline verdicts landed across five historical
reports; **7 fully-dispositioned reports archived** (`git mv` → `docs/status/archived/`,
`docs/status/` now holds only the fresh 21-31 report). Living docs fixed: version split
brain (README/AGENTS said v0.1.1; v0.1.2 is released), stale FEATURES rows (consumer,
CI, ADR titles), TODO_LIST trophy section deleted + re-curated with the 21-31 report's
open items harvested in, CONTRIBUTING gained the release/API-sync checklist (open since
the 13-17 session), the 19-34 plan stamped EXECUTED. The first pass missed three ROADMAP
defects — including one my own harvest edit created — caught only when the owner asked
"ROADMAP.md = SUPERB??"; all six ROADMAP findings fixed. Gates green (test, lint 0, vet,
fmt 0 changed, flake check). Health scores post-fix: **Accuracy 10/10, Fitness 10/10**
(pre-fix 7.0 / 8.5, math in the inline report of this session).

---

## a) FULLY DONE

| #  | Work                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Evidence                                                                |
| -- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| 1  | **All 15 `**/2026-0*` files viewed** (8 active reports, 3 archived reports spot-verified, 2 planning docs, 1 announcement, 1 archived audit) before any edit                                                                                                                                                                                                                                                                                                                        | this session                                                            |
| 2  | **Skill + references loaded first** (SKILL.md, verify-checklist, harvest-guide, resolving-items, health-report-format) and annotate scripts used with mandatory dry-runs                                                                                                                                                                                                                                                                                                            | `~/.config/crush/skills/docs-health/`                                   |
| 3  | **Code state verified before doc edits**: git log + tags (v0.0.1…v0.1.2), `nix run .#test` both packages ok, `.#lint` 0 issues, `.#vet` ok, `nix fmt` 0 changed, `nix flake check` pass                                                                                                                                                                                                                                                                                             | session tool output                                                     |
| 4  | **Version split brain fixed** — README:11 + AGENTS.md:5 "v0.1.1 alpha" → **v0.1.2** (v0.1.2 tagged at `4b3ca9c`)                                                                                                                                                                                                                                                                                                                                                                    | daemon commit `c4bff75`                                                 |
| 5  | **FEATURES.md accuracy fixes** — consumer row (dashboard on released v0.1.2, no replace; was "bump pending"), CI row (green through the v0.1.2 tag run), ADR row titles corrected against the real `docs/adr/` files (001 stdlib errors, 002 zero logging, 003 three-state, 004 recorder decoupling)                                                                                                                                                                                | daemon commit `c4bff75`                                                 |
| 6  | **TODO_LIST rebuilt open-only** — forbidden "Resolved 2026-09-04 (kept for traceability…)" trophy section deleted; harvested from 21-31 §b/§f: 3 owner decisions (branch protection G3, coverage threshold, **new**: slash-contract release vehicle v0.1.3-vs-v0.2.0), 1 owner action (announcement), 7 unblocked rows (fuzz-long trigger, pkg.go.dev v0.1.2 render, ADR-005, OpenAPI aggregate scope, aggregate property tests, spec↔golden lockstep, README probe-decision table) | daemon commit `c4bff75`; TODO_LIST.md                                   |
| 7  | **ROADMAP harvested** — 10 routed raw ideas placed in Themes 2/5/6/7 (aggregate handler benchmarks, fuzz-seed feeding, throttled-contention benchmark, throttle-boundary fuzz, aggregate-handler-fuzz × throttle/cache, `-count=N` stress, ETag rejection note, dependabot flake-input coverage, non-nix CI matrix + arm64, fuzz-budget raise, gopls suppression)                                                                                                                   | daemon commit `1e4ea6e`, `9fdba12`                                      |
| 8  | **CONTRIBUTING: "Release / API-Sync Checklist" added** — README stability line, badges/Project Docs, AGENTS status line + docs table, `doc.go`, CHANGELOG links, openapi.yaml + golden tests. Closes 13-17 §e3 + 14-15 §e7/f17 (open across three sessions)                                                                                                                                                                                                                         | daemon commit `1e4ea6e`                                                 |
| 9  | **19-34 plan stamped EXECUTED** — header now points at the 21-31 report (closes that report's §f15)                                                                                                                                                                                                                                                                                                                                                                                 | daemon commit `1e4ea6e`                                                 |
| 10 | **~175 inline verdicts**: 11-13 (10 f/g items + completion addendum), 13-17 (47: b/c/e/f/g incl. 34 f-items + 5 hand-edited tops), 14-15 (60: all 50 f-items, 3 g-questions, 2 b-items, 5 c-bullet blocks), 19-25 (57: 6 b-rows, 6 c-rows, all 34 f-rows, 8 e-items, 3 g-questions), 19-01 (consumer question resolved)                                                                                                                                                             | `annotate-rows.py` / `annotate-prose.py`, dry-run first, shape-verified |
| 11 | **7 reports archived** via `git mv` → `docs/status/archived/`: `09-12`, `19-01`, `19-11` (08-07), `11-13`, `13-17`, `14-15`, `19-25` (09-04) — every item now done/routed/won't-do                                                                                                                                                                                                                                                                                                  | daemon commit `9fdba12`                                                 |
| 12 | **Dangling links repaired** — `docs/planning/00-02` relative links retargeted to `archived/` after the moves                                                                                                                                                                                                                                                                                                                                                                        | daemon commit `95663d0`                                                 |
| 13 | **ROADMAP re-audit (owner challenge) — 6 defects fixed**: Healthz-parity split brain (listed twice); false "Each … carries a written design" claim (Healthz parity has none); dependabot near-duplicate **created by my own harvest edit** (merged); "SHIPPED in v0.1.0" theme headers contradicted by live raw ideas; Theme 5 section order; stale header stamp                                                                                                                    | daemon commit `95663d0`                                                 |
| 14 | **Inline health report produced** with visible math (Accuracy 7.0→10, Fitness 8.5→10) and honest unverified list (pkg.go.dev v0.1.2 render, today's CI runs — not fetched)                                                                                                                                                                                                                                                                                                          | conversation only, per skill                                            |
| 15 | **Living-docs link check**: 0 broken relative links across the six living docs + touched files; `nix fmt` clean after all edits                                                                                                                                                                                                                                                                                                                                                     | session script                                                          |

## b) PARTIALLY DONE

1. **Verdict completeness on two 08-07 stragglers** — `09-12` §e4 (startup-latch
   context cancellation) carries its own mitigation evidence but no grammar-compliant
   verdict marker; `14-15` §b3–b5 are meta-notes left unmarked (judgment call: not work
   items). A strict "every numbered item carries a verdict" reading flags both.
2. **TODO_LIST evidence quality** — the 7 unblocked rows cite report sections and file
   paths, but not every row carries a fresh `file:line` verified this session (e.g. the
   property-test row cites `aggregate/aggregate.go` without a line).
3. **External claims trusted from reports, not re-fetched** — "pkg.go.dev v0.1.1
   rendered" (19-25 §a1) and CI green-ness were taken from report evidence, not
   re-verified live this session; the v0.1.2 render check remains a TODO_LIST row.
4. **21-31 report deliberately untouched** — freshest snapshot, feeds TODO_LIST; but a
   future run should re-check its §b items once the owner decisions land.
5. **This report** — written at session end, not at the moment of the last doc edit;
   the daemon committed the working tree before the report existed (see d4).

## c) NOT STARTED (all tracked, none silently dropped)

- **Owner decisions (TODO_LIST, blocked):** branch protection (G3, ready-to-run
  command); coverage-threshold job (fail < 97%?); release vehicle for the
  slash-name contract (v0.1.3 now vs ride in v0.2.0).
- **Owner action:** publish the v0.1.1/v0.1.2 announcement (draft + checklist ready).
- **Unblocked TODO_LIST rows:** fuzz-long `workflow_dispatch` trigger; pkg.go.dev
  v0.1.2 render; ADR-005; OpenAPI aggregate scope statement; aggregate property tests
  (idempotence + commutativity); OpenAPI↔golden lockstep CI check; README
  "which probe should I hit?" table.
- **ROADMAP raw ideas:** everything listed in Themes 1/2/5/6/7 (incl. v0.2.0
  candidates `errors.Join`, `SourceStatuses()`, aggregate `Healthz` parity — design
  note to be written first).
- **Report hygiene backlog (this session's f-list below):** verdict top-ups (b1),
  machine-readable report front-matter, intra-file consistency check as an explicit
  VERIFY step.

## d) TOTALLY FUCKED UP

1. **The ROADMAP first pass shipped with a split brain and a false universal claim —
   and one defect was my own creation.** I flagged "no TODO↔ROADMAP duplication" as a
   check but never scanned _intra-file_ duplication: "Aggregate `Healthz` parity" sat
   in both Theme 1 and Theme 7 through the entire audit. Worse, the dependabot
   near-duplicate (Theme 7) was _introduced by my own harvest edit_ landing next to
   the pre-existing "Track Go 1.26.x patches (dependabot may cover)" line. The owner's
   one-word challenge ("SUPERB??") caught what the full audit missed. Cost: one
   re-audit cycle; near-miss on shipping a "superb" label over a defective file.
2. **Stale context snapshot nearly caused blind edits.** The session-start AGENTS.md
   context was hours behind the repo (the 21-31 session had landed gotchas/rows I
   couldn't see). I caught it via grep before editing AGENTS.md content — but I
   _viewed_ FEATURES.md and planned edits against the snapshot before confirming;
   the habit of trusting context-cache over disk survived longer than it should.
3. **Two wasted round trips on edit mechanics** — a "read the file first" rejection
   (19-34 plan: edited from grep knowledge, not View) and a modified-since-read
   rejection on 13-17 (my own annotate script rewrote the file between read and
   multiedit — the exact daemon-style race this repo keeps documenting, this time
   self-inflicted).
4. **Daemon collision confusion cost a debug cycle.** The auto-daemon committed my
   living-doc edits in two batches mid-run (`c4bff75`, `1e4ea6e`), so `git status`
   showed RM/rename states I initially misread as lost work; I re-verified by
   inspecting the daemon commits. Nothing was lost — but I ran status twice before
   trusting it. The session-ending curation discipline (commit fast or pause the
   daemon around release/doc ops) remains unpracticed.
5. **Naive tooling, known limitations** — the link checker is a regex (false positive
   on `do.MustInvokeNamed[*Database](injector, …)` in a Go fence) and the
   unmarked-item sweeps grep for shapes rather than parsing markdown; both worked
   only because I manually adjudicated every hit. A parser-based check would not
   have needed the adjudication.
6. **Hash archaeology done lazily** — I collected the four meaningful non-daemon
   hashes (`4b3ca9c`, `46c3f5e`, `5a79775`, `e366fcc`…) up front but still wrote many
   verdicts as "covered by the 21-31 report §a…" where a per-item daemon-commit hash
   would have required git log archaeology I skipped. Grammar-compliant, less precise
   than ideal — the same tradeoff 14-15 §b4 already confessed.

## e) WHAT WE SHOULD IMPROVE

1. **Intra-file consistency must be an explicit VERIFY step.** Cross-file checks are
   in the skill checklist; same-fact-twice-in-one-file (the Healthz parity split
   brain) is not — add it, or every audit remains one challenge away from a miss.
2. **Universal claims ("each", "all", "every") get item-by-item verification.** The
   false "Each carries a written design" survived because the claim _read_ right.
3. **Re-audit a living doc immediately after editing it** — my own ROUTING edits
   created a near-duplicate; a 30-second self-diff review would have caught it before
   the owner did.
4. **Collect hashes before the annotation pass**, not during: one `git log` sweep
   with per-file stats would upgrade "covered by §…" verdicts to `done at <hash>`.
5. **Parser-based doc checks** (markdown AST for links + list items) replace regex
   sweeps — kills both false positives and silent negatives.
6. **Write the status report before the last doc edit**, or commit-intent-stamp the
   tree first, so daemon batch boundaries align with the report's evidence.
7. **TODO_LIST evidence column: `file:line` per row**, refreshed at curation time —
   "§report" citations rot the moment the next session rewrites the report list.
8. **Keep `annotate-*.py` dry-run + shape-verify as mandatory** — this run again
   proved it (one early spec mismatch caught in dry-run, zero corruption).

## f) Up to 50 Things We Should Get Done Next (priority-ordered)

> 1–11 are the live TODO_LIST (verified this session); 12–27 are ROADMAP raw ideas
> now in their themes; 28–50 are report/tooling hygiene and older leftovers.

1. Owner: execute branch protection (G3) — ready-to-run command in TODO_LIST.
2. Owner: release vehicle for the slash-name contract (v0.1.3 now vs v0.2.0).
3. Owner: coverage-threshold job decision (fail < 97%?).
4. Owner: publish the announcement (channels + checklist ready).
5. Trigger `Fuzz (weekly long)` once via `workflow_dispatch` (5 min).
6. Verify pkg.go.dev renders v0.1.2 (aggregate + examples visible).
7. ADR-005: promote `docs/aggregate-source-name-design.md` into the ADR series.
8. OpenAPI: state explicitly that aggregate endpoints are covered or scoped out.
9. Aggregate property tests: merge idempotence + source-order commutativity.
10. OpenAPI ↔ golden-file lockstep check in CI.
11. README "which probe should I hit?" decision table.
12. v0.2.0: `errors.Join` in `aggregate.New` (design + spike ready).
13. v0.2.0: `Aggregate.SourceStatuses()` (design ready).
14. v0.2.0: aggregate `Healthz` parity — write the design note first.
15. Aggregate handler (HTTP-path) benchmarks complementing the merge benchmark.
16. Feed golden-fixture inputs into the aggregate fuzz seed corpus.
17. Benchmark the throttled live path under contention.
18. Fuzz the throttle-window boundary under concurrency with a fake clock.
19. Combine aggregate handler fuzz with throttle/cache modes.
20. `-count=N` race-suite stress in CI if flakiness stays at zero.
21. Go 1.27 floor bump (directive + drop GOEXPERIMENT) when 1.26 support drops.
22. Promote erraudit/doanalyzerv2 to CI if either becomes public.
23. Dependabot/Renovate for flake inputs + pinned action SHAs (subsumes Go patch
    tracking); auto-merge rules as separate policy call.
24. Non-nix CI matrix job (plain `go test`) for OS/arch honesty.
25. arm64 native runner evaluation if QEMU stays too slow.
26. Raise per-push fuzz budget above 10s/target if CI cost allows.
27. gopls stdversion warning suppression (editor noise only).
28. Top up grammar-compliant verdicts on `09-12` §e4 and (optionally) `14-15` §b3–b5.
29. Add machine-readable front-matter to future status reports (21-31 §f48).
30. Harden TODO_LIST evidence column to fresh `file:line` per row.
31. Add an intra-file duplication check to the docs-health VERIFY routine.
32. Replace regex link/list sweeps with a markdown parser check.
33. Post-v0.1.3: re-run the consumer verification matrix (dashboard vs new release).
34. ETag/`If-None-Match` rejection note (Theme 5) before someone asks.
35. `AwaitReady` cache-aware poll interval (needs a use case).
36. OTel spans spike via `WithEvaluationHook` (Theme 2).
37. `TotalLatencyMs` as `float64` (wire-format change — gated on consumer need).
38. `Probe.Snapshot()` accessor (only with a concrete consumer).
39. `HealthRecorder` signature revisit at v1.0 (ADR-004).
40. Review `WithGETOnly` pin-tests at v1.0 deprecation burn-down.
41. Dependabot auto-merge rules decision (separate from #23's coverage).
42. README benchmark-table excerpt for the "fast by design" story (21-31 §f49).
43. Example: custom `HealthRecorder` combined with the aggregate (21-31 §f26).
44. Example: live-vs-cached mode side-by-side (21-31 §f27).
45. Shellcheck `tools/doanalyzerv2/run.sh` if shellcheck joins treefmt (21-31 §f47).
46. Consider `omitzero` migration for always-emitted scalar fields (v0.2.0+ wire
    decision; needs changelog callout) (21-31 §f45).
47. Evaluate auto-generated GitHub release notes vs hand-curated excerpt (21-31 §f40).
48. Add a small index/README for `docs/status/archived/` (navigation + archive rule).
49. Pause-daemon (or commit-fast) practice around release/doc ops (19-25 §e5).
50. Re-run this docs-health AUDIT after the owner answers land (G3/vehicle/coverage
    change TODO_LIST shape and unblock the archive trail again).

## g) QUESTIONS ONLY YOU CAN ANSWER (3)

1. **Release vehicle (G2 follow-up):** the aggregate slash-name contract sits
   unreleased in CHANGELOG `[Unreleased]`. Cut **v0.1.3 now** (small, contract-only),
   or let it ride in **v0.2.0** with `errors.Join` + `SourceStatuses()`? Your veto
   window closes when consumers adopt the stricter validation.
2. **Branch protection (G3):** shall I run the ready-to-run command (5 required checks
   - linear history, `enforce_admins:false` keeps your bypass)? It changes push
     semantics for non-admins on `master` — the only reason it hasn't been flipped.
3. **Coverage-threshold job:** do you want CI to fail below 97% statement coverage
   (baseline is 99.7%)? It's been a blocked policy row for four sessions; a yes is a
   20-minute CI job, a no lets me delete the row permanently.

---

_Report ends. Awaiting instructions._
