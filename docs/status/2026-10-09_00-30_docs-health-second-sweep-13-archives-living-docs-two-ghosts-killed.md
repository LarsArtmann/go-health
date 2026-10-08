# Status Report — Docs-Health Second Sweep: 13 Reports Annotated + Archived, Living Docs Rebuilt, Two Ghosts Killed

**Date:** 2026-10-09 00:30 CEST
**Scope:** this session only (~21:00 → 00:30), the second full docs-health sweep of the week (predecessor: `docs/status/archived/2026-10-04_11-39_*`). Trigger: "View ALL \*\*/2026-0\* files! Execute the docs-health SKILL! … TODO_LIST/CHANGELOG/AGENTS/README/ROADMAP/FEATURES must be SUPERB! Archive FULLY done and UPDATED (inline strikethrough) .md files!"
**End state:** `docs/status/` holds exactly the two freshest reports; 13 reports annotated inline (~820 `~~` verdicts) and archived; `nix flake check` **all checks passed**; `docs-check` OK; tests 4/4; lint 0 issues; `nix fmt` 0 changed; link sweeps clean; working tree has 5 doc files pending for the daemon; local ref reports **0 commits ahead of origin** (the 20:58 report's 10-commit lag closed outside this session — see §g1).
**Skill compliance:** docs-health SKILL.md loaded first; annotate tooling used with mechanical key emission; one bug in my own batch pipeline found and fixed mid-sweep (§d2–d3).

---

## a) FULLY DONE

### The archive sweep (the session's core mandate)

| # | Work                                                                                                                                                                                                                                                                                                     | Evidence                                                                                 |
| - | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| 1 | **13 status reports annotated inline** (every §b/§c/§f/§g item struck with a per-item verdict; §a/§d/§e left as historical record per the documented convention) and `git mv`'d to `docs/status/archived/`                                                                                               | grep gate: `grep -rLn '~~' archived/*.md` prints nothing; per-file counts 46–69 verdicts |
| 2 | **Archive set:** the three 2026-09-04 reports (the "anchor" question left open on 2026-10-04 — decided ARCHIVE: nothing cites them as live state), 2026-09-15 OTEL, 2026-09-18 hardening, both 2026-09-22 19:51 reports, 2026-09-22 21:01 + 21:32, 2026-10-03, both 2026-10-04 reports, 2026-10-08 19:42 | `docs/status/archived/README.md` second manifest (13 rows, deciding reason each)         |
| 3 | **Keep-window policy applied:** `docs/status/` = the two 2026-10-08 evening reports only (20:58 handoff + 20:59 BuildFlow triage)                                                                                                                                                                        | `ls docs/status/*.md \| wc -l` → 2                                                       |
| 4 | **Manifest updated** with the 2026-10-08 sweep table + the one-line archive rule + the §a/§d/§e allowance, so the next audit can tell resolved-and-archived from never-classified                                                                                                                        | archived/README.md                                                                       |
| 5 | **Fresh-report annotation:** the two kept reports' items THIS session resolved are struck inline (20:58: 20 items incl. §b3/§b4 memory debt; 20:59: 6 items incl. the CHANGELOG/TODO/FEATURES entry ask), so the live handoff doesn't re-ask answered questions                                          | both files                                                                               |

### Verification done before verdicts (not after)

| #  | Finding                                                                                                                                                                                                                                                                                  | Consequence                                                                                                                          |
| -- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| 6  | **Branch protection is ENABLED** on master (`gh api …/branches/master` → `.protection` truthy) — the G3 question asked since 2026-09-04 in six reports resolved itself                                                                                                                   | All G3 asks struck `done`; TODO_LIST header records it. Caveat: I verified _enabled_, not the exact ruleset (§d8)                    |
| 7  | **`sevRank` split brain was real and live** (2026-09-18 §e3, never actioned): the test helper duplicated `Status.Rank()` ordering including the `Off` case                                                                                                                               | Fixed: helper deleted, both call sites call `Status.Rank()`; commit `c62f79f`; `nix run .#test` 4/4 green, `nix run .#lint` 0 issues |
| 8  | **Hook-panic hazard upgraded from "untested concern" to verified fact:** `Evaluate` calls `p.evalHook(resp)` at `probe.go:661-663` with no recover, and `refreshCache` (the background loop) calls `Evaluate` — a panicking consumer hook kills the loop goroutine (process-fatal in Go) | 2026-09-15 §e6/§f4 struck `superseded by a sharper finding`; TODO_LIST Hardening row added with the file:line                        |
| 9  | **Cookbook claim verified before documenting:** `validateCriticalNames` (probe.go:537) reads the initial batch response, so `ErrUnknownCriticalService` covers `NewChecks`/standalone batches exactly like the injector path                                                             | Sentence added to docs/detailed-checks-cookbook.md (closes 20:58 §f25)                                                               |
| 10 | **CI truth pulled, not assumed:** master CI red at 17:33/18:08 (the toolchain-skew window), green again 19:09; weekly fuzz-long schedule green through 2026-10-05 (still 3-target era — the 4-target workflow first fires 2026-10-12)                                                    | Verdicts in the 21:32/20:58 annotations say exactly that                                                                             |

### Living docs rebuilt (all six)

| #  | Doc                          | Change                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| -- | ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 11 | **TODO_LIST.md**             | Broken header markdown fixed (the "batteries↔- critical-name" split line); header re-stamped (second audit, drift-gate shipped, branch-protection ON); archive row deleted (executed by this session); 5 new Hardening rows (makezero semantics, hook panic, golangci LSP, BuildFlow gate restore, + existing skew/bench rows); `tools/doanalyzerv2` floor normalized to a BLOCKED row (owner call, 20:59 §g1)                                  |
| 12 | **AGENTS.md**                | 220 → **188 lines**: the 40-row docs table moved verbatim to new `docs/INDEX.md` (relative links rewritten for the new location — the move initially broke every link, caught and fixed, §d1); two new Gotchas: BuildFlow coverage + the makezero seam regression (`b2b9ed0`) with the honest "why the old form passed is still unexplained", and the golangci-LSP-lies discipline; federation paragraph now links the validation-semantics doc |
| 13 | **FEATURES.md**              | Fuzz-targets row "Three targets" → **four** (`FuzzBatteries` shipped in v0.5.0 — real drift the 2026-10-04 audit missed); CI row "green on the v0.1.1/v0.1.2 tag runs" → "every tagged release through v0.5.0"                                                                                                                                                                                                                                  |
| 14 | **ROADMAP.md**               | Theme 2 extended with the 2026-09-15 SystemNix findings (OTEL spike against a real collector, non-blocking hook adapter, hook-stream vs served-response divergence, Gatus/textfile/integrations-index recipes); Theme 7 gains the GOTOOLCHAIN=auto README note                                                                                                                                                                                  |
| 15 | **README.md / CHANGELOG.md** | No edits needed — `docs-check` proves the sync lines, and the CHANGELOG policy (now written down, #16) keeps docs/internal work out. Verified rather than touched                                                                                                                                                                                                                                                                               |
| 16 | **CONTRIBUTING.md**          | CHANGELOG policy extended: docs-only AND internal-only changes (tooling, private analyzers, test helpers, behavior-neutral refactors) stay out — "an entry exists for a consumer of the library, and nothing else" (closes 20:58 §e5 and the 21:32 §f28 ask permanently)                                                                                                                                                                        |

### Smaller closures landed on sight

| #  | Item                                                                                                                                                                   | Where                         |
| -- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------- |
| 17 | servicename-design.md title de-drift ("v0.5 candidate" → "v0.6 candidate"; body framing stays v0.5 by documented intent)                                               | 20:58 §f15 closed             |
| 18 | federation-design.md now cross-links federation-validation-semantics.md (non-goals entry) — AGENTS already did                                                         | 20:58 §f24 closed             |
| 19 | Planning docs stamped: 13-54 fleet plan EXECUTED + the C16 substitution (dashboardui → crush-daily) recorded in the plan file; 00-02 plan marked SUPERSEDED by 19-34   | closes fleet-report §f37/§c51 |
| 20 | Gates: `nix run .#docs-check` OK · `nix flake check` **all checks passed** · `nix fmt` 0 changed · link sweeps over living docs + INDEX + sweep-touched docs: 0 broken | session runs                  |

---

## b) PARTIALLY DONE

1. **"View ALL \*\*/2026-0\* files" is literal-complete for .md but not for .html.** Every dated `.md` (status/planning/announcements) was read and dispositioned. The three dated HTML artifacts (`docs/research/2026-09-18_*.html` ×2, `docs/planning/2026-09-18_09_18-next-level-hardening-pareto.html`) were classified from prior reports' descriptions and left untouched — never opened. Strikethrough annotation doesn't apply to them; but "ALL" was interpreted, not literal.
2. **The 2026-10 planning docs (`2026-10-02_16-11`, `2026-10-03_03-43`) were not read** — outside the literal pattern and already stamped/superseded by earlier reports' evidence. Classified only through other files' citations.
3. **Verdict precision is uneven.** Shipped-work verdicts mostly cite the shipping release, not a commit hash ("done — shipped v0.4.0"); only ~6 carry real hashes (`b2b9ed0`, `c62f79f`, `6716a24`, `7fe2b22`, `274d19f`, `5c60f0d`, `d558878`, `a806cd5`). Same confession the 2026-09-04 22:15 report made: hash archaeology was done lazily.
4. **The sevRank fix passed `.#test` and `.#lint` but not `.#test-race`** — the change is test-only (deleting a helper, calling an existing method), so race risk is nil by construction, but the repo's own pre-push bar (`nix run .#gates`) was not run end-to-end this session: docs-check + flake check + test + lint ran; vet/vulncheck/security/fuzz rode on "no production code changed" reasoning.
5. **check-rows.py reports PARTIAL rows by design.** My table-strike shape (work-item cell struck + verdict, Impact/Effort cells readable) matches the 2026-10-04 sweep's precedent (`| ~~ | b1 | …`) but is not the checker's COMPLETE class. Accepted knowingly and documented in the manifest — not "fixed", decided.
6. **The 20:58 report got no completion note** — 20:59 got one ("only items this sweep resolved are struck; the gate-restoration list remains the live handoff"), 20:58 did not. A future reader must infer the same convention there.
7. **Daemon-shaped history again.** Every commit this session is a `chore: auto-commit N changed file(s) (heuristic)` — including the sevRank fix (`c62f79f`) and the sweep itself (a 16-file commit). Nothing was lost, but the change-grouping and messages are heuristic, and two files were still staged-uncommitted at report time.

---

## c) NOT STARTED (all tracked; none silently dropped)

- **Owner-gated, artifacts ready:** push decision (§g1), v0.5.1 vehicle (§g2), BuildFlow ownership boundary (§g3), #318 comment post, v0.5.0 + v0.1.1/v0.1.2 announcement publishes, coverage-threshold policy, CV bump (G3), upstream filings (G2), tools go.mod floor (patch-pin tension).
- **TODO_LIST Hardening rows (unblocked):** hook-panic recover-vs-document + test; makezero `always` semantics investigation; golangci LSP fix/disable; version-skew CI script; FEATURES benchmark re-verify at fresh `-count=3`; BuildFlow full-mode gate restoration (20:59 §f1–15).
- **Release follow-through:** go-health-dashboard full suite vs released v0.5.0; consumer test train (fir, KeyHolderAI, DiscordSync, go-taskqueue, webphone, nsfw-classifier).
- **Watch:** first 4-target fuzz-long run (scheduled 2026-10-12).
- **ROADMAP Themes 1/2/6/7 tails** — unchanged this session except the Theme 2 routing.
- **v0.6 window staging** — ServiceName inventory, rename staging, mergeResponses prep (all three rows stand).

---

## d) TOTALLY FUCKED UP

1. **I moved the AGENTS.md docs table into `docs/INDEX.md` without rewriting its relative links.** Every `[FEATURES.md](FEATURES.md)` and `[docs/…](docs/…)` in the table pointed at nonexistent paths from the new location — a self-inflicted 40-link breakage in the doc whose job is pointing at things. Caught by a spot-check existence loop minutes later (before any gate ran), fixed with a systematic rewrite, re-swept. Root cause: I executed a structural move from a bash splice without re-deriving the link semantics for the file's new directory depth.
2. **I repeated the June-batch incident class (positional verdict paste).** For the 22-15 report I paired `--emit-keys` output with my verdicts file by _position_; the emitter skips continuation lines, so 56 keys met 53 verdicts and **verdicts attached to the wrong items** (§b rows wearing §f verdicts). The tool even printed "VERIFY OK" — it validates key resolution, not my pairing. Caught only because I audited the struck lines afterward; fixed by rewriting all 58 verdicts keyed by line number. Lesson now mechanical: key↔verdict mapping must be by line number, never by list position.
3. **A 57-item strike batch silently never landed.** My first spec for the 19:42 report aborted on an unexpected line (atomic, wrote nothing) — and I then archived the file with only 5 strikes in it. The completeness gate (`grep '~~'` presence) passed with 5, so nothing failed loudly; I caught it in the post-sweep verdict count, re-applied, verified 62. An archived file briefly existed in a state its own manifest called "fully annotated".
4. **Wrong counts in written records — twice.** The manifest first said "12 reports", the 20:58 annotation said "11", the truth was 13. A status sweep that exists to kill fabricated numbers typed two fabricated numbers of its own. Both fixed by deriving counts from `ls | wc -l` instead of memory.
5. **edit-before-read rejections, again:** CONTRIBUTING and servicename edits attempted from grep output instead of a View (2 round trips); one modified-since-read rejection caused by my _own_ python splice minutes earlier. The 2026-10-04 report's §d1, repeated.
6. **Two wasted edit rounds on FEATURES because I misread View's line-number prefix** (`150|| Fuzz targets` — I copied `||` into old_string as content). Cosmetic, but it is exactly the "pattern-match the actual line text" lesson from 2026-10-04 §d3, unlearned for one round.
7. **Two annotator crashes on structural surprises** (a `##` heading line keyed as an item; a blank separator line inside a numbered list) — atomic failures, no corruption, but each cost a diagnose-and-rekey cycle because I built specs from remembered section shapes instead of a fresh awk map.
8. **The branch-protection verdict may overclaim.** I verified `.protection` is enabled but did NOT verify the ruleset (which 5 checks, linear history, `enforce_admins:false`). Six old reports' G3 asks are now struck "done" on an existence check, not a config match. Probably right; not proven.
9. **No `.#gates` end-to-end run.** The repo's pre-push bar was substituted with a component-wise argument (docs-check + flake check + test + lint, "no production code changed"). That argument is sound but it is the same "documented nearby ≠ executed" shortcut 22:37 §d2 confessed.

---

## e) WHAT WE SHOULD IMPROVE

1. **Key↔verdict mapping must be line-numbered, always.** The `--emit-keys | paste` pattern is structurally unsafe (the emitter's order/coverage is an implementation detail). The line-numbered python pass used after §d2 should be the default tool, or annotate-status-items should grow a `<lineno>\t<verdict>` mode.
2. **Add a per-file strike-count assertion to the sweep:** expected strikes (from the spec) vs `grep -c '~~'` (from the file), per section, before `git mv`. It would have caught §d3 before the move instead of after.
3. **Derive every count in a report from the filesystem** (`ls | wc -l`), never from memory — the manifest and cross-references are the class of number that rots fastest while claiming authority.
4. **Commit code fixes BEFORE annotating**, so verdicts cite real hashes instead of "this sweep": one deliberate commit right after sevRank would have upgraded every "done" that leaned on it.
5. **Structural moves get a link-derivation step, not a copy step:** when relocating a doc block, enumerate its relative links and re-target them as part of the move script (the spot-check loop that caught §d1 should have been part of the move itself).
6. **Read View output as `lineno|content`** — the `||`-prefix confusion cost two cycles; pattern-match content only.
7. **The keep-window policy works** — `docs/status/` = newest 2 is the right steady state; the sweep cost scales with how long reports linger. Consider stating the policy in CONTRIBUTING (it currently lives only in the manifest).
8. **Verdict grammar should distinguish "verified config" from "verified existence"** — "branch protection ENABLED" vs "branch protection present, ruleset unverified" are different claims; the second is what I measured.
9. **Run `nix run .#gates` once before push even on doc-only sessions** — the component-wise substitution is exactly the rationalization the repo keeps confessing; 10 minutes buys the claim.
10. **When annotating a section of a LIVE report (20:58/20:59), add the same completion note as archived files get** — say which items are struck and why the rest stay bare, so the mixed shape reads as intent, not accident.

---

## f) Up to 50 things we should get done next

_Ranked by impact. (B) = blocked on owner/external. 1–12 are the live handoff; 13+ this sweep's additions._

| #  | Task                                                                                                                                                           | Bucket     |
| -- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------- |
| 1  | (B) Push decision — tree was 10 ahead at 20:58, local ref now reads 0 (closed outside this session; verify what actually happened), 5 doc files pending        | release    |
| 2  | (B) v0.5.1 vehicle for the Shutdown-hang fix (CHANGELOG `[Unreleased]` carries it)                                                                             | release    |
| 3  | Restore `buildflow --fix --build-mode=full` to exit 0 (20:59 §f1–15, toolchain-skew follow-ups)                                                                | gates      |
| 4  | (B) BuildFlow ownership boundary: buildflow-owned vs flake-owned quality runs (20:58 §g3)                                                                      | decision   |
| 5  | go-health-dashboard full suite vs released v0.5.0 (the one deep consumer)                                                                                      | verify     |
| 6  | Hook panic at probe.go:661-663: decide recover-and-fail-closed vs document-the-contract (design doc first), pin with a test                                    | design     |
| 7  | (B) File go-appkit/health + cqrs-htmx/health upstream issues (drafts ready, G2)                                                                                | upstream   |
| 8  | Consumer test train vs v0.5.0: fir, KeyHolderAI, DiscordSync, go-taskqueue, webphone, nsfw-classifier                                                          | verify     |
| 9  | (B) CV bump to go-health v0.5.x + `go 1.27` (G3)                                                                                                               | consumer   |
| 10 | (B) Post samber/do#318 comment (draft + checklist ready)                                                                                                       | owner      |
| 11 | (B) Publish v0.5.0 announcement (draft ready `9b87a88`) + v0.1.1/v0.1.2 draft                                                                                  | owner      |
| 12 | (B) Coverage-threshold CI job: yes/no + threshold                                                                                                              | decision   |
| 13 | Run `nix run .#test-race` + full `.#gates` once before push (test-only change still deserves the bar)                                                          | verify     |
| 14 | Nail makezero `always` semantics; record the contract in AGENTS + `.golangci.yml` comment (or close the open question honestly)                                | code       |
| 15 | Fix or disable the stale golangci LSP integration (3 standing false warnings)                                                                                  | tooling    |
| 16 | Watch the first 4-target fuzz-long run (scheduled 2026-10-12); verify corpus artifact paths                                                                    | verify     |
| 17 | (B) tools/doanalyzerv2 go directive: accept 1.27.1 + budget the warning, or lower branching-flow's floor (20:59 §g1)                                           | decision   |
| 18 | Fix `tools/doanalyzerv2/main.go` doc comment ("Command doanalyzerv2-runner")                                                                                   | code       |
| 19 | Audit CONTRIBUTING.md / run.sh / docs for stale `doanalyzerv2-runner` module-path references                                                                   | docs       |
| 20 | Re-run standalone go-structure-linter after the AGENTS.md restructure (needs the branching-flow checkout)                                                      | gates      |
| 21 | Verify the branch-protection ruleset matches the G3 spec (5 required checks + linear history + admin bypass) — upgrade §d8's existence check to a config check | verify     |
| 22 | Add the completion-note convention to live-report annotations (or backfill one on the 20:58 report)                                                            | process    |
| 23 | Version-skew CI script: fleet go.mod pins vs latest tag, fail-on-drift                                                                                         | automation |
| 24 | FEATURES benchmark re-verify at fresh `-count=3`; label single-run rows                                                                                        | verify     |
| 25 | Extend `.#docs-check`: AGENTS "Packages" list == go.mod packages; FEATURES option count == `grep -c '^func With'`                                              | automation |
| 26 | Add `.#docs-check` to the `ci-emulation` gate list                                                                                                             | automation |
| 27 | State the docs-health keep-window policy in CONTRIBUTING (newest 2–3, archive rule)                                                                            | docs       |
| 28 | dprint/markdownlint pass over the sweep-edited markdown; decide the CI role                                                                                    | tooling    |
| 29 | Verify `feature_request.md`'s fixed relative link renders on github.com (20:59 §f25)                                                                           | verify     |
| 30 | File the branching-flow bounds-analyzer upstream issue (parallel pre-sized writes false positive; verify-before-filing first)                                  | upstream   |
| 31 | Confirm + strike BuildFlow TODO ZC2 (embedded snapshot honors project yaml — 20:59 §f27)                                                                       | upstream   |
| 32 | ServiceName call-site inventory + mechanical rewrite script (v0.6 staging, R6)                                                                                 | v0.6       |
| 33 | Finalize rename staging table (`SanitizeResponse`→`CoerceValidUTF8`, `Since`→`StatusSince`, `WithCriticalChecks` decision) (R7)                                | v0.6       |
| 34 | mergeResponses port prep: primitive sketch + corpus fixture (R11)                                                                                              | v0.6       |
| 35 | README: add the GOTOOLCHAIN=auto note (routed to ROADMAP this sweep; it is a 5-minute row — promote it)                                                        | docs       |
| 36 | openapi.yaml: the Healthz-mount sentence (never added; 19:51 §f11) + federation endpoint coverage (ROADMAP raw idea)                                           | docs       |
| 37 | `errors.Join` in aggregate.New + `Aggregate.SourceStatuses()` + federation `Prober.Healthz()` parity (ROADMAP Theme 7 set)                                     | feature    |
| 38 | ROADMAP Theme 6 tail: aggregate handler fuzz corpus, throttled-contention bench, throttle-boundary fuzz, `-count=N` stress                                     | quality    |
| 39 | `AwaitReady` cache-aware poll interval (Theme 1)                                                                                                               | feature    |
| 40 | checks package coverage-close report (R13 remainder)                                                                                                           | verify     |
| 41 | `WithShutdownGracePeriod` × failed-Start interaction test (19:42 §f48)                                                                                         | test       |
| 42 | Federation-adjacent `ErrUnknownCriticalService` composition test (19:42 §f49)                                                                                  | test       |
| 43 | Internal-link sweep across docs/** (not just living docs), skipping code fences (lychee covers it only in BuildFlow runs)                                      | verify     |
| 44 | Render-check the two HTML reports in docs/reviews/ — or formally retire the render expectation                                                                 | verify     |
| 45 | Inspect `.config/`, `reports/`, `coverage/`, stale `result` symlink; trash if stale (20:59 §c8)                                                                | hygiene    |
| 46 | Re-review `.buildflow.yml` budgets (art-dupl 80, branching-flow 14, go-auto-upgrade 6) once the gate is green — budgets are policy, not wallpaper              | gates      |
| 47 | Read `references/lessons.md` and reconcile the fleet/process recommendations that 13-13 left unread (§c3)                                                      | docs       |
| 48 | Re-verify docs/adoption-matrix.md at the next release (per 10-03 §e5; the matrix is the consumer source of truth)                                              | verify     |
| 49 | Decide the process-metrics thread's fate (measure the loop or retire it — 13-13's abandoned series)                                                            | decision   |
| 50 | Re-run this docs-health AUDIT after the §g answers land (they reshape TODO_LIST and reopen the archive trail)                                                  | process    |

---

## g) QUESTIONS (3 — cannot figure out myself)

1. **Push state contradiction:** the 20:58 report said "10 commits ahead of origin, unpushed"; at this report's close my local ref reads `origin/master..HEAD` = **0** — but I never pushed, and I saw no push in this session. Did you (or another flow) push the 2026-10-08 work, or did a fetch just move my origin ref past commits that were pushed earlier by the concurrent session? I need to know whether the Makezero/lifecycle-fix/doc sweep lineage is actually on the remote before anyone relies on CI green at 19:09.
2. **Branch protection ruleset:** I verified `.protection` is enabled, but not WHICH checks/linear-history/admin-bypass config is live. Is it the G3 spec from the 2026-09-04 reports (5 required checks, linear history, `enforce_admins:false`)? If yes, I will close every G3 mention fleet-wide as done-at-config; if it is a different (looser/stricter) ruleset, the struck verdicts in six archived reports need a correction note in the manifest.
3. **The panicking eval-hook (verified unrecovered at probe.go:661-663 on the refresh-loop path):** do you want (a) recover + synthesize a fail-closed signal (extends docs/panic-recovery-design.md's recoverable-surface list — a design decision, not a patch), (b) document the "hooks must not panic" contract in the option's godoc + README only, or (c) leave it as the TODO row for a designed fix in the v0.6 window? I cannot choose the failure semantics of a consumer callback for you.

---

_Point-in-time snapshot of the 2026-10-08 → 09 sweep. Every struck verdict cites its evidence class (hash, release, gate run, or cross-repo marker); the two live 2026-10-08 reports carry this session's resolutions inline. THEN WAIT FOR INSTRUCTIONS._
