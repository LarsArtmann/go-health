# Status Report — Pareto v2 Execution: v0.5.1 Shipped, Hook-Panic Fixed, CI Un-Reddened (Mid-Execution Snapshot)

**Date:** 2026-10-09 16:53 CEST · **Session:** full Pareto-plan-v2 execution under the owner's blanket "get the whole list done" directive · **Trigger of this report:** explicit mid-execution status demand with self-critique, while work was in flight

---

## a) WHAT WAS DONE

### Released: v0.5.1 (the 1% tier's centerpiece)

- **CHANGELOG cut** `[Unreleased]` → `[v0.5.1] - 2026-10-09` with compare links; README stability line + AGENTS status line synced; docs-check green against the new tag.
- **Tag** `v0.5.1` (annotated) on `23db7fe`, pushed with master; **module proxy hash-verified** (`23db7fee3b7eb8cc7f8952a87fe182923a1b4bd0`, exact tag commit); `@latest` had not flipped yet at check time.
- **GitHub Release** live: not prerelease, not draft, Latest; curated notes staged from the changelog.
- Contents: `health.Off` configured-off checks, hook-panic recovery (below), `checks.Disk` G115 guard, failed-`Start` disarm fix.

### Code: the hook-panic hole closed (A04+A05, rode the release)

- **Design note** (panic-recovery-design.md addendum): the hook is an *observation* surface — it runs after evidence collection completes — so fail-closed (the batch-surface rule) would assert unhealthiness the probe knows to be false. Decision: **recover + non-critical `evaluation-hook` warn row** (new sentinel `ErrPanicDuringEvaluationHook`) + **defensive Checks clone** so a hook can never corrupt the served/cached response. The "recovered panics never warn" rule is scope-amended to the data-collection surface.
- **Implementation** (probe.go): `invokeEvaluationHook` with the clone, recover, synthetic row, `Rank`-based roll-up raise (fail never lowered). **Five tests** green incl. loop-survival under a permanently panicking hook and mutation-isolation; race clean; CHANGELOG/AGENTS/doc.go synced.

### CI un-reddened: two supply-chain fires the gates caught (A02)

- **8 stdlib advisories** (GO-2026-6603..6617, net/http/http2/textproto/crypto-tls via federation's HTTP client) fixed only in go 1.27.2 had **CI red on every push since 00:14** — nixpkgs' `go_1_27` was still 1.27.1. Fixed with an `overrideAttrs` 1.27.2 pin in flake.nix (hash-prefetched; drop-condition documented); vulncheck now clean.
- **gosec 2.29.0 cannot decode go 1.27.2's export data v5** (39 import errors, exit 1, 0 findings). Upstream fixed it only today (securego/gosec#1772, unreleased). Interim: security app runs gosec under the patch-prior toolchain (`pkgs.go_1_27`), with the unpin condition in a flake comment.
- Full gates + ci-emulation green on the exact push tree; second push rode the release commit.

### Fleet proof + leverage (4% tier, partial)

- **A12/A13 FILED:** [go-appkit#25](https://github.com/LarsArtmann/go-appkit/issues/25) + [cqrs-htmx#31](https://github.com/LarsArtmann/cqrs-htmx/issues/31) — drafts re-verified against live bridge source first, voice-checked 0 FAIL/0 WARN, filed via gh, verified OPEN; drafts + bridge-golden-path annotated; filed bodies preserved in docs/announcements/.
- **A14 staged:** #318 citations re-verified against samber/do master (batch machinery moved to `queueServiceHealthcheck` root_scope.go:213 — draft updated with current numbers); #318 has 0 comments. Posting stays an owner act (third-party repo).
- **A15/A16 staged:** both announcement checklists confirmed complete; publishing is the owner's.

### Standing-drift killers (20% tier, partial)

- **A17 makezero contract SETTLED** (closed 20:58 §b1's open question): empirically pinned via probe programs, a historical-tree worktree repro, and the analyzer source — `always: true` flags **every identifier-bound two-arg `make` with non-literal-zero length regardless of usage**; **composite-literal makes are invisible** (that, not usage, is why the old `startup: make(...)` passed); blessed forms + the `max-same-issues: 3` diagnostic trap. Recorded in AGENTS + a `.golangci.yml` comment.
- **A19 fleet-skew detector** (B061/B062 + the red half of B063): `tools/fleet-repos.txt` (15 consumers) + `tools/fleet-skew.sh` (authenticated gh api — private repos 404 on raw fetch), reading each repo's live default branch. **Current verdict: 9 repos at v0.5.0, 5 minor-stale (CV v0.1.3; Zlota44, dnsblockd, DiscordSync, KeyHolderAI v0.4.1).** Deliberate-skew failure path proven (fake `v0.9.0` → exit 1). CI workflow not yet wired.
- **A21 ServiceName inventory** in servicename-design.md: 14 local repos inventoried with constructor + critical-name form; scanner `tools/servicename-scan.sh` + fleet dry-run; verification plan. **Load-bearing finding: untyped literals/consts survive the `ServiceName` change** — the real break set is 6 sites in 6 repos (nsfw-classifier typed getter; fir/DiscordSync/CV/library-policy `[]string` spreads), not "every call site".
- **A22 rename decision table** in naming-integrity.md: alias-vs-clean-cut principle per row, consumer impact source-verified (`SanitizeResponse` dashboard-only; `Check.Since` dashboard-only; `WithGETOnly` a KeyHolderAI **test file only** — lighter than the adoption matrix claimed; lexicon deferred past v0.6). TODO staging rows deleted per lifecycle.
- **A24 OpenAPI completeness:** mountable combined `Healthz` documented (`/healthz-combined`, mount-path-is-host's-choice note, synthetic `startup`-row semantics for root + aggregate), federation paragraph added to info.description (source/check namespacing, `<name>/reachable` synthetic fail), redocly valid, lockstep green, spec version 0.5.1.

### Misc

- TODO_LIST re-verified header rewritten for v0.5.1; filings row deleted; train row refreshed with real pins; #318 row refreshed; v0.6 staging rows replaced by a staging-complete note. FEATURES CI row through v0.5.1.
- A01: branch protection verified at **config** level (5 named checks + linear history + `enforce_admins:false` + strict) — the sweep report's §d8 existence-check caveat upgraded with a dated verification note; the G3 spec matches live exactly.

---

## b) PARTIALLY DONE / IN FLIGHT (the honest state)

- **A06** — release shipped, but **pkg.go.dev render + `@latest` flip + CI-green-on-tag verification not done** (B021 second half, B008 completion).
- **A07 BuildFlow** — B024 green (govalid-generate exit 0, skew cleared); the BuildFlow rebuild (B026) was dispatched to a background shell **and never collected**; B025 full run, B027 content verification, B028 AGENTS update not done.
- **A19** — script + table + red-path done; **CI workflow file not written** (deliberately sequenced after the consumer trains so its first run is green).
- **A32** — the grace × failed-Start **fix is written but UNVERIFIED**: `probe.go` carries an uncommitted edit (skip the grace sleep when no loop is armed — peek under `p.mu` before sleeping, re-arm-during-drain semantics preserved) with **no test, no build, no CHANGELOG yet**. The daemon has not committed it at report time (` M probe.go`).
- **Background jobs never collected:** CI verdicts on `4476cd0` (Security/Test were in_progress at last look) and on the release commit; the BuildFlow nix build. Three dispatches, zero reads.

---

## c) NOT STARTED

A08 (budget re-review), A09 (dashboard suite vs v0.5.1), A10 (consumer train — note: the plan's train list was stale; the REAL minor-stale set is KeyHolderAI, DiscordSync, Zlota44, dnsblockd + CV), A11 (CV bump, G3), A18 (golangci LSP — fresh evidence accrued, see §d), A20 (benchmarks `-count=3`), A23 (mergeResponses prep), A25 (errors.Join), A26 (SourceStatuses), A27 (federation Healthz note), A28/A29 (fuzz/bench batches), A30 (AwaitReady note), A31 (checks coverage), A33 (federation validation test), A34 (docs hygiene + B114 "(CV)" rename), A35 (parking + polish: B102–B111, B113), the final gates+push, the plan's EXECUTED stamp.

---

## d) WHAT WE FORGOT / FUCKED UP (critique, no anesthesia)

1. **An unverified code edit is sitting in the tree** (A32). The status demand interrupted mid-task — but the "test immediately after each modification" rule doesn't care about interruptions. First action on resume: build+test+lint it or revert it. No exceptions.
2. **Three background jobs dispatched, zero collected.** The CI watch on 4476cd0, the CI watch on the release commit, and the BuildFlow rebuild all finished (or failed) unseen. Dispatching and forgetting is the exact "documented nearby ≠ executed" anti-pattern the 00:30 sweep confessed.
3. **Verify-before-claiming miss:** I wrote "zero false literals" into servicename-design.md *before* reading the scanner's dry-run output — which contained a false positive (go-appkit `opts ...health.Option` spread). Caught and corrected minutes later, but the claim was typed first. The rule is: output first, prose second.
4. **Wrong first implementation of fleet-skew:** raw.githubusercontent fetches (10 false "no pin" WARNs — private repos 404). I even briefly treated the garbage table's exit 0 as a pass. Second implementation (gh api, live default-branch resolution) is the one that works. Private-by-default must be the fleet tooling assumption from now on.
5. **Release-commit message inaccuracy:** the daemon raced my `git add` and captured CHANGELOG.md into its own commit; my "Release v0.5.1" commit stat shows only README/AGENTS while its message claims the CHANGELOG cut. The tagged tree is correct (verified via `git show v0.5.1:CHANGELOG.md`) — but the message lies about its own contents. Add files immediately after editing, or `git add` in the same breath.
6. **wsl_v5 whack-a-mole (3 lint rounds):** I invented a closure idiom instead of matching `recoverHealthChecks`' assignment shape from the start; the surviving `_ = invoked()` is a wart future readers will puzzle over.
7. **The docs-check mid-train failure was discoverable upfront:** the compare-base chicken-and-egg (the tag must exist for `[Unreleased]`'s link to validate) is visible in flake.nix:135-161. Reading the gate's source before the release train would have saved a failed gates run and a triage cycle.
8. **SemVer convention deviation, self-authorized:** v0.5.1 ships an **Added** section (Off) — the repo's own pattern (v0.2.0, v0.4.0 additive → minor) says that's v0.6.0 material; the plan says v0.5.1; 0.x semver permits either. I followed the plan's letter. Unratified — question 1 below.
9. **The golangci LSP panel lied all session** (stale typecheck on evaluation_hook_test.go:97 from the first minute; the file compiled and passed race tests throughout). I arbitrated correctly by real gates but never captured the evidence for A18 — now noted here.

---

## e) WHAT WE SHOULD IMPROVE

- **Collect before you context-switch:** every background dispatch gets a `job_output` read before starting new work — a dispatch without a collection is unfinished work wearing a busy badge.
- **Output before prose:** doc claims quoting tool runs get written after reading the run, every time.
- **Read the gate source before orchestrating around it** (release trains, mid-train expected failures).
- **Match repo idioms before inventing shapes** (recover patterns, assignment-vs-expression under wsl_v5).
- **Private-first fleet tooling:** authenticated API by default; raw fetches only for known-public repos.
- **`git add` in the same breath as the edit** when a commit is planned — the daemon will otherwise split the logical change across commits and lying messages.

---

## f) NEXT 50

1. Build + test + lint the A32 grace fix (or revert); CHANGELOG `[Unreleased]` entry; commit.
2. Collect the three lost background jobs: CI on `4476cd0`, CI on `23db7fe`/tag, BuildFlow build.
3. Finish B026: install the rebuilt BuildFlow (`nix profile install`), `buildflow doctor` stops warning.
4. B025: `buildflow --fix --build-mode=full` → exit 0, findings gate read.
5. B027: `buildflow -s "golangci-lint [tools/doanalyzerv2]"` + govulncheck both modules, content-read, 0 findings.
6. B028: AGENTS BuildFlow gotcha updated (1.27.2 resolution + makezero seam note).
7. A08: budget re-review (art-dupl 80 / branching-flow 14 / go-auto-upgrade 6) post-green.
8. B021 finish: pkg.go.dev v0.5.1 render poll; proxy `@latest` flip check.
9. B008 finish: CI green on tag `v0.5.1` recorded.
10. A09: dashboard — baseline suite on v0.5.0 pin, bump to v0.5.1, full suite incl. browser/screenshot tests, cross-repo handoff note.
11. A10 train (real stale set): KeyHolderAI v0.4.1→v0.5.1, focused tests.
12. DiscordSync v0.4.1→v0.5.1 + critical-name guard test.
13. Zlota44 v0.4.1→v0.5.1 + tests.
14. dnsblockd v0.4.1→v0.5.1 + dashboard-overlay test.
15. Patch-lag bumps to v0.5.1: fir, go-taskqueue, webphone, nsfw-classifier, library-policy (fleet-skew green target).
16. A11: CV → go-health v0.5.1 + `go 1.27` floor, build + health suite, flake verify, push-decision note (G3).
17. A19 finish: `.github/workflows/fleet-skew.yml` (push-to-master + weekly + dispatch); green run vs v0.5.1.
18. A18: fix or disable the golangci LSP integration; record the decision (evidence: this session's stale panel).
19. A20: benchmarks `-count=3` (root, aggregate, checks) → medians + spread → FEATURES table update; label single-run rows.
20. A23: `mergeResponses` primitive sketch vs both merge sites; corpus fixture from aggregate golden + federation samples; port plan; design-doc + TODO flip.
21. A25: `errors.Join` in `aggregate.New` per design + multi-error construction tests + CHANGELOG + doc status flip.
22. A26: `SourceStatuses()` + property test against per-source roll-ups + README aggregate section + FEATURES row + CHANGELOG.
23. A27: federation `Healthz()` parity design note (503 conditions + latch semantics) + ROADMAP cross-link; NO implementation.
24. A28: golden-fixture inputs → aggregate fuzz seed corpus; `-count=N` race-stress CI step (zero-flake condition).
25. A29: throttled live-path contention benchmark (+FEATURES row); throttle-window boundary fuzz (fake clock); handler fuzz × throttle/cache modes.
26. A30: AwaitReady cache-aware poll-interval mini design note (implementation stays demand-gated).
27. A31: `go test -cover ./checks` report; close top gaps; re-measure.
28. A33: federation-adjacent `ErrUnknownCriticalService` composition test (mirror of the aggregate one).
29. B114: start-validation-design.md "(CV)" → "conditional sets" rename (collision with the CV repo).
30. B100: link sweep over docs/** (fences skipped); triage hits.
31. B101: inspect `.config/`, `reports/`, `coverage/`, stale `result` symlink; trash stale.
32. B102: BLOCKED rows' evidence refresh (doanalyzerv2 floor under the 1.27.2 toolchain; auditlog ADR-004; coverage threshold) — no implementation.
33. B103: dprint/markdownlint pass over session-edited markdown; CI-role decision.
34. B104: references/lessons.md reconcile (fleet/process recommendations, no duplicates).
35. B105: process-metrics thread fate (measure or retire) recorded.
36. B106: standalone go-structure-linter re-run after the AGENTS restructure.
37. B107: `feature_request.md` blob render check on github.com.
38. B108: doanalyzerv2 main.go doc comment fix + stale module-path reference sweep.
39. B109: README GOTOOLCHAIN=auto note + compatibility table re-check.
40. B110: CONTRIBUTING keep-window policy + completion-note convention for live reports.
41. B111: backfill the completion note on the 20:58 report (makezero answer, grace fix, hook fix).
42. B112: fuzz-long 4-target watch (calendar 2026-10-12) + corpus-artifact verify.
43. Final sweep: docs-check + full gates + ci-emulation on the final tree; push master.
44. Stamp plan v2 EXECUTED with per-task annotations (this report + the next one feed it).
45. Draft the v0.5.1 announcement checklist (extend or sibling the 2026-10-08 v0.5.0 one).
46. gosec unpin: when gosec ≥ #1772 reaches nixpkgs, restore `goPkg` in the security app (flake comment tracks it).
47. go override drop: when nixpkgs `go_1_27` ≥ 1.27.2, remove the overrideAttrs pin (flake comment tracks it).
48. Adoption-matrix corrections: `WithGETOnly` row (test-only usage), go-daemon/project-discovery-daemon row (not consumers — own same-named option), PMA/doadapter naming.
49. B113: post-answer docs-health re-audit once the owner answers §g.
50. Next status report after the tail completes.

---

## g) QUESTIONS FOR THE OWNER (cannot be answered by me)

1. **Version-number ratification (G4 aftermath):** v0.5.1 shipped an Added section (`health.Off`) per the plan's letter, but the repo's own changelog pattern (v0.2.0, v0.4.0: additive → minor) would have called it v0.6.0 — and "v0.6" is the number the breaking-change staging docs reserve. Keep v0.5.1 as shipped (no action), and going forward: is additive→minor a hard rule, or does the plan's naming win when they conflict?
2. **Publishing acts (A14/A15/A16):** the samber/do#318 comment is fully staged and citation-current, and both announcements have complete channel checklists. Under the blanket directive I filed your own-repo issues (go-appkit#25, cqrs-htmx#31) but stopped at third-party/social voice: the #318 comment and the announcement posts. Post them now under the directive, or do you review and post yourself?
3. **Cross-repo push authority:** the consumer trains (A10/A11: KeyHolderAI, DiscordSync, Zlota44, dnsblockd, CV, then the patch-lag six) will produce green bumps as local commits. May I push those repos' masters when their suites pass, or leave every consumer commit unpushed for your review?

---

**Resume point:** item 1 of §f (verify or revert the A32 fix), then the three lost collections (item 2). The full plan state: 14 of 35 Track-A tasks complete, 4 partial, 17 not started; the 1% tier is done except A07's tail; v0.5.1 is live on the proxy.
