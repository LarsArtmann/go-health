# Status Report — Merge Unification, WithGETOnly Removal, BuildFlow Green

**Date:** 2026-10-11 18:25 CEST
**Repo:** `github.com/larsartmann/go-health` @ `master` (`5b008ac`, 14 daemon commits since session start `eeeb157`; 27 files, +845/−393)
**Session scope:** Execute the TODO_LIST's actionable rows: v0.6-staging merge unification, `WithGETOnly` removal, stale-row cleanup, buildflow full-mode restoration. Gate verification at every step. No other research performed beyond what the work required.

---

## Headline numbers

| Metric                       | Value                                                                                                                                                                                                   |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| TODO rows executed & deleted | 3 of 3 actionable (merge unification, WithGETOnly, stale hook-panic row) + buildflow row                                                                                                                |
| New package                  | `internal/merge` (Source + Responses, pure fold)                                                                                                                                                        |
| Removed exported API         | `WithGETOnly()` (deprecated since v0.1.1)                                                                                                                                                               |
| Latent test flakes fixed     | 2 (both surfaced by buildflow's race-stress fan-out)                                                                                                                                                    |
| Gates                        | `.#gates` exit 0 · `nix flake check` exit 0 · `buildflow --build-mode full` exit 0 (79/79) · docs-check in sync with v0.6.1                                                                             |
| Buildflow runs this session  | 6 (3 failed on real signal: format, AGENTS.md line cap, ticker flake — each fixed)                                                                                                                      |
| Merge fuzz campaign          | 4.7M execs, 0 failures                                                                                                                                                                                  |
| Docs touched                 | 13 files (README, FEATURES, AGENTS, CHANGELOG, INDEX, TODO_LIST, ROADMAP, CONTRIBUTING, DOMAIN_LANGUAGE, openapi.yaml, deprecation-policy, naming-integrity, adoption-matrix, merge-unification-design) |

---

## a) FULLY DONE

1. **Merge unification (`internal/merge`)** — the v0.6-staging TODO row, executed per docs/merge-unification-design.md: new `internal/merge` package (`Source{Name, Response, FetchErr}` + `Responses(sources)`), `aggregate.CachedResponse` ported to a one-load-per-source mapping, `federation.merge` ported with latch advancement kept federation-side. Evidence: all pre-existing aggregate/federation tests passed unchanged and served as the port's golden-check oracle; `.#test-race` green; `.#gates` exit 0. Files: `internal/merge/merge.go` (new), `aggregate/aggregate.go`, `federation/federation.go`.
2. **One fold semantics decision resolved and documented** — rank ties are now tie-stable (never replace the accumulator), which preserves aggregate's exact historical behavior, makes the fold order-independent (pinned by `TestResponses_OrderIndependent`), and closes a real federation edge: a remote document with top-level `status: "off"`/unknown could previously leak into the merged roll-up via last-same-rank-wins. Recorded in CHANGELOG §Unreleased, the design doc (now IMPLEMENTED), and AGENTS.md.
3. **Fuzz/property suite collapse** — the duplicated merge-invariant assertions collapsed into `FuzzResponsesMergeInvariants` over the primitive (arbitrary Source pairs, independent fold oracle); aggregate's fuzz thinned to the mapping pin (merged output must equal `merge.Responses` over the mapped views, same corpus-safe signature); federation keeps its fetch-path fuzz (decode/refusal semantics, not merge semantics). Deleted with the port: `statusRank` + `TestStatusRank` (rank ordering now pinned through the primitive's tests). Evidence: 4.7M-exec fuzz campaign clean, suite green.
4. **`WithGETOnly` removed** — function deleted from probe.go, the three `TestGETOnly_*` pin tests deleted, `FuzzHandlerInput` seed switched to `WithAllowedMethods()` (signature unchanged, corpus safe), and the 405-body-message pin folded into `TestWithAllowedMethods_EmptyActsAsGETOnly` so no observable contract coverage was lost. Evidence: zero `WithGETOnly` references in Go code (`rg` exit 1), full suite green, `.#lint` green.
5. **Deprecation policy reconciled before removal** — docs/deprecation-policy.md said v0.x symbols are never removed, directly conflicting with the staged A22 decision; amended with an explicit zero-production-adopter removal path (adoption matrix clean + decision-table record + CHANGELOG migration snippet), with `WithGETOnly` as the first application. ROADMAP's "no earlier than v1.0" burn-down line updated to reflect the executed removal. The policy change was made _before_ the code cut so docs and code never disagreed.
6. **KeyHolderAI consumer fix (local)** — `cmd/keyholderai/health_probe_http_test.go` switched from `WithGETOnly()` to `WithAllowedMethods()`; verified compiling against its pinned go-health v0.5.1 (`go vet` exit 0). Daemon-committed in that repo (`0a1c5117`), unpushed per the fleet's push-authority hold.
7. **Two latent test flakes fixed** — (1) `TestCachedResponse_NeverStartedSource` hardcoded `TotalLatencyMs == 0` while the primed source's own evaluation latency can legitimately read 1ms under full-suite parallelism; now asserts the real property (merged total == the real source's cached batch, ghost contributes nothing). (2) `TestRefreshLoop_TickerFiresBeforeShutdown` slept a fixed 50ms then required two calls; under the fan-out the 5ms ticker starved past the window; now polls with a 5s deadline. Both CHANGELOG'd as test-only. Both fail on mechanism analysis independent of the merge port (the port propagates latency identically — old code took max over the same sources), though isolated reproduction on the pre-change commit under simulated load did not fire; the reliable reproducer was buildflow itself.
8. **`buildflow --build-mode full` restored to exit 0** — the gate red since 2026-10-08. Six runs this session: run 1 failed treefmt (self-inflicted, unformatted deletion), runs 2/3 failed on test-race (flake #2, then flake #1 context), run 4/5 failed the findings gate (AGENTS.md over the 220-line cap — my additions), run 6: **exit 0, 79/79 steps** on rebuilt binary `200ab99` + `GOTOOLCHAIN=auto`. The four 2026-10-09 stragglers (license-check, govalid-generate, root+tools variants) now pass.
9. **Budget re-review (the "don't let them ratchet" duty)** — with full mode green: go-auto-upgrade 7/8, branching-flow 7/9, art-dupl 63/80 — all within ceiling with headroom, every finding in a documented accepted class; the `.buildflow.yml` rationale comments (re-verified 2026-10-11 marker) still accurate. No budget change needed. Recorded in AGENTS.md.
10. **Docs sync across 13 files** — CHANGELOG §Unreleased gained Changed (merge unification + tie-rule delta), Removed (WithGETOnly + migration snippet), Fixed (two flake fixes); README's method-guard sections rewritten; FEATURES options count 13→12; AGENTS.md gained the `internal/merge` sub-package paragraph and updated buildflow/method-set bullets while staying under the 220-line preflight cap (219); design doc status → IMPLEMENTED; naming-integrity + adoption-matrix rows marked EXECUTED; INDEX, ROADMAP, CONTRIBUTING, DOMAIN_LANGUAGE, openapi.yaml all de-referenced the removed symbol. `nix run .#docs-check` green.
11. **TODO_LIST rebuilt** — three rows deleted per lifecycle, A22 staged renames harvested into a new "Next breaking window" section (ServiceName row correctly BLOCKED on owner go/no-go per the 2026-10-11 report §b1), header stamped.
12. **LSP hygiene** — the golangci LSP panel carried a provably false `reflect` compile error on merge_test.go for the entire session (CLI build/test green throughout — the documented stale-panel trap); restarted `golangci_lint_ls` per decision A18 at session end.

## b) PARTIALLY DONE

1. **AGENTS.md line budget** — 219/220 after compressing my additions. Works (gate green), but headroom is zero: the next session that adds a line must remove one, and the counting is fragile — `wc -l` said 220 while the buildflow structure-linter demanded ≤220 and read 221; I never root-caused the off-by-one, I just went to 219. Blocker: none. Effort to finish: S.
2. **Buildflow green state** — exit 0, but with caveats kept honest: 155 findings remain (all documented accepted classes within budgets), 9 "tools unavailable" health-check warnings (all JS/TS/Python tools — jest/vitest/knip/madge/publint/svelte-check/vue-tsc/c8/interrogate — inapplicable to a Go-only repo), and `GOTOOLCHAIN=auto` was exported in every green run, so it is unproven whether the rebuilt binary still needs it. Effort to finish: S.
3. **Federation tie-rule delta coverage** — the semantics change (off/unknown remote roll-ups can no longer leak into the merged status) is implemented and CHANGELOG'd, but no dedicated unit test pins the exact delta scenario (remote `status:"off"` + passing remote → merged pass); the existing merge table's off row is check-level, and the fuzz doesn't assert exact statuses. The property holds by construction (tie-stability is unit-pinned), but the federation-facing regression pin is missing. Effort: S.
4. **Session commit hygiene** — all 14 commits are daemon heuristic commits ("chore: auto-commit N file(s)"); a semantic session (new package + API removal + flake fixes) landed as an unreadable log. The harness forbids manual commits without explicit request, so this is the known tradeoff, not an error — but the log does not tell the story. Effort to fix: owner-harness decision.
5. **A22 harvest fidelity** — the harvested rename rows carry effort estimates taken from the A22 table's shape (e.g. "2h" for the ServiceName cut), which are planning guesses, not resourced estimates. Acceptable for staging; do not treat as commitments.

## c) NOT STARTED

All pre-existing, visible from this session's vantage; none touched (owner-blocked or separate-window work):

1. **v0.7.0 release cut** — Unreleased now carries a breaking removal (`WithGETOnly`) and a behavior delta (tie rule); cutting the tag is a distinct go-release-skill flow. Not started.
2. **ServiceName typed identity + `WithCriticalChecks` alias** — owner go/no-go reserved; design + fleet inventory + scanner ready in docs/servicename-design.md. BLOCKED.
3. **`SanitizeResponse` → `CoerceValidUTF8` alias** and **`Check.Since` → `StatusSince` clean cut** — A22 steps 3–4, dashboard-only breaks. TODO rows, not started.
4. **golangci LSP integration fix-or-disable** — standing TODO, owner call; I only restarted the client this session.
5. **Version-skew CI script** — BLOCKED on owner push authority (would observe stale pins and run red day one).
6. **Coverage-threshold CI job (fail < 97%)** — BLOCKED on owner policy call.
7. **Owner actions** — v0.1.1/v0.1.2 announcement publish; samber/do#318 comment post. Drafts ready, publishing is the owner act.
8. **samber-do-auditlog `DetailedHealthRecorder`** — BLOCKED on owner decision (ADR-004 dependency-direction tension, documented).
9. **`tools/doanalyzerv2` go directive normalization** — BLOCKED on samber-linter release; the go-version-auto-configure warning persisted even in this session's green buildflow run.
10. **gosec nix pin removal** — waiting on nixpkgs shipping gosec that reads export-data v5 (securego/gosec #1772).
11. **Fleet verification sweeps** for the next breaking window (11 repos per servicename doc, const-decl checks, PMA scan) — not started.
12. **Federation `CachedResponse`/merge benchmark** — carried from the 2026-10-11 04:27 report §f.6; untouched.
13. **go-branded-id claims source-verification** — conditional on the typed-identity decision advancing; untouched.

## d) TOTALLY FUCKED UP

**Nothing at fuckup severity.** All gates green end-of-session, no regressions shipped, no data loss, working tree clean except the final TODO_LIST edit (daemon pending). The merge port's oracle discipline (pre-existing tests never modified except the two flaky assertions) means the refactor is provably behavior-preserving where it claimed to be.

Nearest misses worth naming honestly:

1. **AGENTS.md line-cap failure shipped to the gate twice.** My additions silently crossed the 220-line cap; the structure-linter error only surfaced via buildflow's findings gate (runs 4 and 5, exit 69), not through any check of my own. I then discovered the linter's line count (221) disagrees with `wc -l` (220) and never root-caused the off-by-one — I just retreated to 219. A documented invariant (the cap exists since 2026-10-08) was violated by the session that knew about it. Severity: process, not product. Mitigation: none yet — see (e) 1 and (f) 3–4.
2. **Format-before-verify slip.** Run 1 of buildflow failed treefmt on the `probe_test.go` deletion's double blank line — a self-inflicted, zero-value failure loop (~2 min + diagnosis). `nix fmt` after every structural edit would have prevented it. Severity: wasted cycle.
3. **A meaningless verification run.** My first flake-reproduction worktree was checked out at "HEAD" — but the auto-commit daemon had already committed my in-progress changes, so "HEAD" was my own code, not the pre-change baseline. The worktree, checkout, and test cycle answered nothing. Caught on the second look (git log before assert), but the log check should have been first. Severity: wasted cycle + a moment of false confidence.
4. **Three failed scripted edits to merge_test.go.** The funlen fix went through python heredoc surgery that (a) generated a malformed struct literal, (b) dropped the package header, (c) left an unused closure — each caught by the compiler, none landed broken, but the edit tool with exact matching was the right instrument and I reached for a heredoc first. Severity: wasted cycles.
5. **The two flakes existed on master through at least one release window** (the ticker test predates v0.6.0; the latency assertion shipped with the never-started pin in the v0.6.0 cycle) and neither CI nor local `.#test-race` ever caught them — only buildflow's heavier fan-out (race + shuffle + concurrent nix derivations) did. The repo's flake-detection coverage was thinner than its gate list implied. Now fixed; the detection gap lesson is in (e) 8.

## e) WHAT WE SHOULD IMPROVE

1. **Format before every verify sweep.** `nix fmt` is 30ms; a treefmt failure in a full-mode run costs minutes. Make it a reflex after any deletion/restructure, not a post-hoc fix.
2. **Check `git log` before any HEAD-based reproduction.** The daemon commits continuously; "HEAD" is a moving target that can include your own uncommitted work within seconds. Worktree reproductions must pin an explicit pre-session commit.
3. **Prefer the edit tools over scripted rewrites.** Python heredoc surgery on code files produced three malformed intermediate states. For line-boundary work, `view` + `edit` with exact matches is slower per keystroke and faster end-to-end.
4. **Root-cause the AGENTS.md line-count off-by-one** (linter 221 vs wc 220 at identical bytes) and record it — otherwise the cap stays a guess-and-retreat game. Candidate cause: the linter counts a virtual EOF line or CRLF-normalized lines.
5. **Restore AGENTS.md headroom** by moving the `internal/merge` paragraph's semantics detail into docs/merge-unification-design.md and leaving a two-line pointer; 219/220 means the next doc-worthy lesson has no home.
6. **Pin the federation tie-rule delta directly**: one test feeding a remote document `{"status":"off"}` plus a passing remote, asserting merged pass. The property is currently guaranteed only transitively via tie-stability unit tests.
7. **Dedicated flake-reproducer knowledge**: simulated local load failed to reproduce either flake; buildflow full mode reproduced both within two runs. Record as a gotcha: "the reliable reproducer for load-flakes in this repo is buildflow full mode, not artisanal load loops."
8. **Silence inapplicable tool noise**: the 9 "unavailable tools" warnings are all JS/TS/Python. If `.buildflow.yml` supports per-tool skips (it supports `skip_steps`), configure the Go-only profile so health-check warnings carry signal again.
9. **Commit-story coherence** (owner/harness decision): a semantic session lands as 14 heuristic daemon commits; if the harness ever allows it, per-concern commits (package / removal / flakes / docs) would make bisect and revert tractable. Until then this is a known, accepted tradeoff.
10. **CHANGELOG heading convention**: test-only fixes currently live under `### Fixed` alongside behavior fixes (precedent: v0.6.0 put test-surface under Added). A dedicated `### Tests`/`Internal` heading would make release-note triage cheaper. Minor, decide once.

## f) NEXT TASKS (40 items; brainstorm, not commitments — HARVEST routes them)

**Tier 1 — direct follow-ups from this session (high impact, low effort)**

| #  | Task                                                                                                                                                                         | Impact   | Effort | Category      |
| -- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- | ------ | ------------- |
| 1  | Cut the next release (v0.7.0): CHANGELOG cut, full gates, tag, proxy/pkg.go.dev verification — Unreleased carries a breaking removal that consumers must be able to pin past | Critical | M      | Release       |
| 2  | Draft v0.7.0 release notes: WithGETOnly migration snippet + tie-rule behavior callout (pair with #1)                                                                         | High     | S      | Documentation |
| 3  | Root-cause the AGENTS.md 220-cap line-count off-by-one (linter 221 vs wc 220); record as gotcha                                                                              | Medium   | S      | Quality       |
| 4  | Restore AGENTS.md headroom: move `internal/merge` semantics detail to the design doc, leave a 2-line pointer                                                                 | Medium   | S      | Cleanup       |
| 5  | Test whether rebuilt buildflow `200ab99` still needs `GOTOOLCHAIN=auto` (run full mode without the export); simplify gotchas if not                                          | High     | S      | Quality       |
| 6  | Add the federation tie-rule delta pin test (remote `{"status":"off"}` + passing remote → merged pass)                                                                        | High     | S      | Bug           |
| 7  | Verify go-health-dashboard against the tie-rule change — it renders merged Status, which could previously be `"off"`; confirm rendering path is safe                         | High     | S      | Bug           |
| 8  | Execute the golangci LSP owner decision: fix or disable `golangci_lint_ls` (it lied for a full session again)                                                                | High     | S      | Quality       |
| 9  | Investigate `.buildflow.yml` per-tool skip config to silence the 9 JS/TS/Python unavailable-tool warnings                                                                    | Low      | S      | Quality       |
| 10 | HARVEST this report's section (f) into TODO_LIST.md/ROADMAP.md (docs-health)                                                                                                 | Medium   | S      | Documentation |
| 11 | Record the flake-reproducer gotcha ("buildflow full mode is the reproducer, not local load loops") in AGENTS.md — needs a line freed first (#4)                              | Low      | S      | Documentation |
| 12 | After v0.7.0 tag: bump KeyHolderAI to it — REQUIRED together, its test fix is local and `WithGETOnly` no longer exists post-tag                                              | High     | S      | Cleanup       |
| 13 | Fleet bump train post-v0.7.0, dashboard first (deepest aggregate/federation consumer)                                                                                        | High     | L      | Cleanup       |

**Tier 2 — next breaking window (staged, harvested into TODO_LIST)**

| #  | Task                                                                                                    | Impact   | Effort | Category      |
| -- | ------------------------------------------------------------------------------------------------------- | -------- | ------ | ------------- |
| 14 | Owner go/no-go on ServiceName typed identity (blocks 15–18)                                             | Critical | S      | Decision      |
| 15 | Execute ServiceName clean cut + `WithCriticalChecks` deprecated alias (inventory + scanner ready)       | High     | L      | Feature       |
| 16 | `SanitizeResponse` → `CoerceValidUTF8` alias + deprecate (dashboard-only)                               | Medium   | S      | Feature       |
| 17 | `Check.Since` → `StatusSince` clean cut, JSON `since` frozen                                            | Medium   | S      | Feature       |
| 18 | Post-rewrite fleet verification (11 repos, Zlota44/dnsblockd const-decls, KeyHolderAI literal-vs-typed) | High     | L      | Quality       |
| 19 | PMA scan on next contact (no local checkout)                                                            | Low      | S      | Quality       |
| 20 | Correct adoption-matrix over-counts (go-daemon rows are NOT consumers)                                  | Low      | S      | Documentation |
| 21 | Re-baseline branching-flow budget after typing lands (finding mix changes)                              | Low      | S      | Quality       |

**Tier 3 — owner actions (artifacts ready or blocked on decision)**

| #  | Task                                                                            | Impact | Effort | Category |
| -- | ------------------------------------------------------------------------------- | ------ | ------ | -------- |
| 22 | Publish the v0.1.1/v0.1.2 announcement (draft + channels checklist ready)       | Low    | S      | Release  |
| 23 | Post samber/do#318 comment (draft ready, citations re-verified 2026-10-09)      | Medium | S      | Release  |
| 24 | Decide coverage-threshold CI job (fail < 97%?)                                  | Medium | S      | Decision |
| 25 | Grant/decline consumer push authority (gates version-skew CI + the fleet train) | High   | S      | Decision |
| 26 | Decide samber-do-auditlog `DetailedHealthRecorder` (ADR-004 tension documented) | Medium | M      | Decision |
| 27 | Version-skew CI script (fleet go.mod pins vs latest tag) — after #25            | Medium | M      | Quality  |

**Tier 4 — quality surfaces & tooling**

| #  | Task                                                                                                                                                           | Impact | Effort | Category      |
| -- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------ | ------------- |
| 28 | Federation `CachedResponse`/merge benchmark (carried from 2026-10-11 04:27 §f.6)                                                                               | Medium | S      | Quality       |
| 29 | Watch securego/gosec #1772; drop the gosec nix pin when nixpkgs ships export-data-v5 reader                                                                    | Low    | S      | Quality       |
| 30 | Normalize `tools/doanalyzerv2` go directive when samber-linter releases (then `go get` branching-flow, re-lower directives)                                    | Low    | M      | Cleanup       |
| 31 | Drop `tools/bin` wrappers + `tool_paths` once #30 lands (conditional chain)                                                                                    | Low    | S      | Cleanup       |
| 32 | Verify the 42 govulncheck stdlib warnings (GO-2026-66xx class) are the documented accepted toolchain-track findings, note the clearing condition               | Medium | S      | Quality       |
| 33 | Source-verify go-branded-id v0.5.1 claims iff #14 advances                                                                                                     | Low    | S      | Quality       |
| 34 | Vocabulary reconciliation (Probe/Prober/Source/Remote lexicon) — revisit only with enforcement tooling (A22 deferral stands)                                   | Low    | L      | Cleanup       |
| 35 | Tidy merge-unification-design.md's stale "Schedule with E1" migration line now that it is IMPLEMENTED                                                          | Low    | S      | Documentation |
| 36 | Confirm `internal/merge` is intentionally absent from FEATURES.md (internal packages are) — one-line doc-policy note                                           | Low    | S      | Documentation |
| 37 | Consider a godoc `Example` for `internal/merge.Responses` (package has none)                                                                                   | Low    | S      | Documentation |
| 38 | Decide the CHANGELOG `### Tests`/`Internal` heading convention (see e.10)                                                                                      | Low    | S      | Documentation |
| 39 | Add "format before gates" to the personal workflow gotchas (see e.1)                                                                                           | Low    | S      | Documentation |
| 40 | Re-run `buildflow --build-mode full` weekly cadence check — it caught 2 flakes CI never did; consider adding full mode to the pre-release checklist explicitly | Medium | S      | Quality       |

## g) QUESTIONS (up to 3 — not answerable from the repo)

1. **Release policy: cut v0.7.0 now, or wait for the A22 renames?** Unreleased carries one breaking removal (`WithGETOnly`) and one behavior delta (federation tie rule). Tagging now means consumers take one breaking bump; waiting means the ServiceName/alias renames ride the same window but delays the fix train. Which do you want — and if "wait," is there a target date?
2. **golangci LSP: fix or disable?** The panel delivered a provably false compile error for the entire session while every CLI gate stayed green (third documented incident). Disabling `golangci_lint_ls` in the Crush config removes the false-signal source but loses in-editor lint; the CLI gates remain canonical either way. Your call — I cannot determine your editor workflow needs.
3. **Consumer push authority: granted, and for which repos?** The fleet's fixes (KeyHolderAI's `WithAllowedMethods` switch today; the earlier consumer commits from the pareto-v2 train) sit local-unpushed pending your answer. After a v0.7.0 tag, KeyHolderAI's fix becomes _required_ on its next bump (the symbol no longer exists), so this answer gates the whole fleet train and the version-skew CI row.

---

_Arte in Aeternum_
