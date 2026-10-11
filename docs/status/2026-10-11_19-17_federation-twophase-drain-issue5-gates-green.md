# Status Report — 2026-10-11 19:17 CEST — Federation Two-Phase Drain (Issue #5)

| | |
| --- | --- |
| **Session scope** | Implement [LarsArtmann/go-health#5](https://github.com/LarsArtmann/go-health/issues/5) — `federation.Prober` had no way to flip readiness to 503 during graceful shutdown; a hub behind an LB answered 200 "ready" all the way through its drain window. |
| **Session window** | ~19:00–19:15 CEST, 2026-10-11 |
| **Base → head** | `0a8a4d4` → `95ef76b`+ (auto-commit daemon interleaved 4 commits during the session; concurrent session(s) also landed release-prep artifacts) |
| **Verification truth** | `nix run .#gates` → **exit 0, all gates green** (run twice: after implementation, and again at final state after doc edits) |
| **Canonical format note** | `.md` at explicit path demand (user instruction); status-report skill's canonical format is styled HTML — this file is the documented override, not a new default. |

---

## a) FULLY DONE

Each item verifiably complete with evidence.

| # | What | Evidence | Scope |
| - | ---- | -------- | ----- |
| A1 | **`federation.Prober.MarkShuttingDown()` + drain overlay** — one-way `shuttingDown atomic.Bool` on `Prober`; overlay in `cachedResponse()` sets `ShuttingDown: true` + `Status: fail` on every merged read, so readiness answers 503 while the merge-on-read fetches keep running (fresh 503 bodies — the issue's preferred option). Single overlay point reaches exactly the right surfaces: `CachedResponse`/`ReadinessHandler` see it; `StartupHandler` discards the response; `LivenessHandler` never fetches. | All tests green; `nix run .#gates` exit 0 | `federation/federation.go` (+48/-7): package doc, `Prober` struct, `cachedResponse`, `MarkShuttingDown`, `CachedResponse`/`ReadinessHandler` doc comments |
| A2 | **Parity design honored** — exact `health.Probe` contract: no `Shutdown()` counterpart (the prober runs no background loop; YAGNI — phase two is the host draining its own HTTP server), no unmark, no getter (observable via `CachedResponse().ShuttingDown`), wire format unchanged (`shutting_down` field pre-exists in the schema). | Design rationale in `docs/federation-design.md` §"Two-phase drain" | API surface +1 method only |
| A3 | **Test suite for the drain** — 4 unit tests + 1 race stress test: drain flips 503 with `shutting_down: true` and a *provably fresh* body (remote document mutates mid-drain, the new check row appears → fetches continued); programmatic `CachedResponse` parity (no reachable row — remote fetched, not refused); liveness/startup blast-radius pin (200/pass, no flag on liveness; startup latches normally); concurrent mark/read stress under `-race` asserting "once ShuttingDown observed, status is fail". | `nix run .#test` and `.#test-race` green across all 5 packages | `federation/federation_test.go` (+184) |
| A4 | **Godoc example** `ExampleProber_MarkShuttingDown` — mirrors the root package's two-phase example; verified passing via `go test -run` (the `nix run` grep pipeline swallowed output; direct run shows `--- PASS`). | `=== RUN ExampleProber_MarkShuttingDown / --- PASS` | `federation/example_test.go` (+46) |
| A5 | **Documentation suite updated** — package doc, design doc (new §"Two-phase drain" + handler bullet updates + API listing), CHANGELOG `[Unreleased]` Added entry, FEATURES federation row, README federation section, AGENTS.md federation paragraph, openapi.yaml info description (drain is overlay on the same wire format). | All 7 files carry `MarkShuttingDown` mentions; `docs-check` gate green post-edit | `federation.go`, `docs/federation-design.md`, `CHANGELOG.md`, `FEATURES.md`, `README.md`, `AGENTS.md`, `docs/openapi.yaml` |
| A6 | **Full gate sweep green at final state** — `nix run .#gates` (test-race, vet, lint, vulncheck, gosec, fuzz, docs-check, openapi-lockstep, flake check) exit 0, re-run *after* the last doc edits. | `GATES_EXIT=0`, "all gates green" | repo-wide |
| A7 | **BuildFlow `go-structure-linter` red → green** — my AGENTS.md edit breached the 220-line cap (219→222/223, an ERROR-severity finding that fails the BuildFlow findings gate). Caught during BuildFlow full mode, fixed by compressing the drain mention into the existing line budget; file now 219 lines; structure-linter exit 0. | `wc -l AGENTS.md` = 219; `buildflow -s go-structure-linter` → exit 0 | `AGENTS.md` |
| A8 | **license-check full-mode red proven pre-existing, not mine** — worktree replay at pre-change commit `0a8a4d4`: single-step `license-check` exit 0; single-step from current master also exit 0; only full-mode fan-out fails (documented env gap, see D1). | `/tmp/go-health-precheck` replay, `PRECHECK_EXIT=0` | diagnosis only, no code |
| A9 | **BuildFlow binary forensics + AGENTS.md gotcha amended** — the profile resolves to the `chore/linter-v050-guard` branch install (`5aaf336`, 1.27.0 tool closure), not the `200ab99` binary verified green this morning; BuildFlow HEAD (`aa9e2e4`) is unbuildable (broken WIP from a concurrent session: `toolsdk.Fact` undefined). The stale "full mode exits 0 end to end" claim is now caveated in the BuildFlow gotcha (line budget preserved). | `buildflow doctor`, `nix build` failure log, `AGENTS.md` BuildFlow bullet | `AGENTS.md` |
| A10 | **`buildflow upgrade` side effect investigated and cleared** — it emitted contradictory output ("does not match any input of this flake" + "Updated only the 'buildflow' flake input in .."); checked `/home/lars/projects`: no `flake.nix`/`flake.lock` exists at the parent → nothing was mutated, binary unchanged (`5aaf336`). | `ls /home/lars/projects/`, `buildflow --version` | hygiene check |

## b) PARTIALLY DONE

| # | What | Works now | Remaining open | Blocker | Effort |
| - | ---- | --------- | -------------- | ------- | ------ |
| B1 | **BuildFlow as a quality surface** | Everything except full-mode `license-check`: structure-linter green, budgets hold (go-auto-upgrade 7/8, branching-flow 7/9, art-dupl 63/80), single-step `license-check` passes, `--fix` ran clean on everything else | Full-mode `license-check` (root + `tools/doanalyzerv2`) exits 1 on `branching-flow requires go >= 1.27.1 (running go 1.27.0)` | Installed binary is the branch install `5aaf336` whose tool closure runs go 1.27.0; the shell `GOTOOLCHAIN=auto` export fixes 7/8 skew-red steps but full-mode fan-out spawn env for license-check is a **known BuildFlow upstream gap** (documented in `.buildflow.yml` comments). BuildFlow HEAD can't be rebuilt (broken WIP) and replacing the shared profile install isn't mine to do unilaterally | M (once BuildFlow HEAD builds) |
| B2 | **Issue #5 disposition** | Implementation complete and verified on master | No comment on the issue, no `Closes #5` cross-reference in the CHANGELOG entry, issue still OPEN — deliberately left for owner instruction (Crush harness: never commit/push/close without explicit ask) | none — awaiting instruction | S |
| B3 | **Release readiness** | CHANGELOG `[Unreleased]` entry written; all four docs-check sync invariants hold | No tag; `docs/status/2026-10-11_00-51_v070-release-prep-staged-tag-gates-green.md` (concurrent session) suggests a v0.7.0 tag was being staged today — whether the drain rides that tag or cuts v0.8.0 is undecided (see Q1) | owner decision | S |
| B4 | **HARVEST of section (f)** | This report's next-task list is written and ranked | `TODO_LIST.md`/`ROADMAP.md` not yet updated from it (status-report skill: run docs-health HARVEST if the session continues) | awaiting instructions per user's "THEN WAIT" | S |

## c) NOT STARTED

Planned/adjacent work this session surfaced but deliberately did not start (scoped to what the session touched — no unrelated research):

| # | What | Why not started | Still wanted? |
| - | ---- | --------------- | ------------- |
| C1 | Wire `fed.MarkShuttingDown()` into go-health-dashboard's hub shutdown (SIGTERM handler) — the *actual* LB fix for the live hub the issue measured | Different repo (`go-health-dashboard`); go-health only ships the primitive | Yes — this is the payoff of the feature |
| C2 | Fleet rollout note: 15 direct + 16 indirect consumers (per `docs/consumer-verification.md`) can't call the new API until they bump | Out of session scope; doc task post-tag | Yes, post-release |
| C3 | Upstream BuildFlow issue for the full-mode license-check spawn-env gap (BuildFlow has **zero** open issues; the gap lives only in `.buildflow.yml` comments) | `verify-before-filing` gate + BuildFlow repo is mid-WIP by another session | Yes |
| C4 | `nix run .#ci-emulation` on current master | Not part of the session's verification loop; gates green without it | Yes, cheap |
| C5 | `nix run .#coverage` delta check on the drain path; `nix run .#fuzz-long` weekly budget incl. overlay path | Unit + race + stress already pin the behavior | Nice-to-have |
| C6 | Aggregate drain-parity decision (aggregate has no own mark; drain flows through sources' probes — issue scoped it out, design doc records why) | Owner decision, ROADMAP fuel | Question, not task |
| C7 | `draining` overlay fuzz seeds (fuzz harness never marks the prober, so the overlay branch is unit-pinned but not fuzz-covered) | Low marginal value over the deterministic tests | Nice-to-have |

## d) TOTALLY FUCKED UP

Radical honesty. Ranked by severity.

| # | What's broken | Severity | Root cause | Mitigation |
| - | ------------- | -------- | ---------- | ---------- |
| D1 | **BuildFlow full mode is red repo-wide** (`license-check`, root + tools) while the canonical flake gates are green — the "additional quality surface" silently contradicts the canonical one, and any session that trusts full-mode exit codes will chase a ghost | Blocks nobody who follows AGENTS.md's "`.#gates` remains canonical"; wastes session time (cost me ~5 diagnostic steps) | Profile binary silently regressed: this morning's verified-green build was `200ab99`; the profile now resolves to the `chore/linter-v050-guard` branch install `5aaf336` whose go-licenses closure runs go 1.27.0 — below branching-flow's `go 1.27.1` floor. Compounding: BuildFlow HEAD `aa9e2e4` **does not build** (`tools/providers/sdk_registry.go:294: undefined: toolsdk.Fact` — concurrent session's WIP), so "just rebuild it" is unavailable | Documented: AGENTS.md BuildFlow gotcha amended (arbitrate full-mode reds against `.#gates`); `.buildflow.yml` already names license-check as an upstream task. Workaround: single-step `license-check` passes; shell `GOTOOLCHAIN=auto` covers the other 7 formerly-skew-red steps |
| D2 | **My AGENTS.md edit briefly tripped a quality gate** — I added 4 lines to a file whose 220-line cap is *stated in the file itself* ("moved out of this file 2026-10-08 for headroom under the 220-line preflight cap") and enforced as ERROR severity by go-structure-linter, without counting lines | Was gate-blocking for ~10 minutes within this session; fixed same session (219/220, exit 0) | I read the cap mention and didn't act on it; the canonical flake gates don't enforce the cap, so my first green `.#gates` run was true *and* incomplete | Fixed by compressing the drain mention; lesson in (e) E2/E3 |
| D3 | **`buildflow upgrade` output lies by omission** — it warned "does not match any input of this flake" then claimed "Updated only the 'buildflow' flake input in .." for a parent directory that has no flake at all; binary unchanged | Cosmetic but misleading tool behavior; unverified-mutation risk for future callers | Likely the parent dir *used to* be (or is expected to be) a flake checkout | Investigated during session: no files at `/home/lars/projects` mutated. Candidate upstream report alongside C3 |

Nothing in the shipped feature is fucked up: the drain path is green on build, vet, lint, race, full suite, fuzz, and both lockstep/docs gates at final state.

## e) WHAT WE SHOULD IMPROVE

Process and tooling lessons from this session, each with a concrete fix:

| # | Pattern | Impact | Suggested fix |
| - | ------- | ------ | ------------- |
| E1 | **I did forensics before reading the config that documented the answer.** `.buildflow.yml`'s comment block states verbatim: full-mode fan-out ignores `tool_paths`, the shell export fixes 7/8 skew-red steps, "license-check/govalid-generate remain BuildFlow upstream tasks" — I found this only *after* the worktree replay, single-step replay, doctor, and upgrade attempt | ~4 wasted steps (~3 min + nix build attempt) | Standing rule: when a BuildFlow step fails, grep `.buildflow.yml` comments *first*; they are the repo's tooling incident log |
| E2 | **Line-budget-blind edits to AGENTS.md.** The cap is documented in the file, enforced by a BuildFlow ERROR finding, and NOT enforced by the canonical gates — so gates-green gave false comfort | One fix cycle; would have shipped red on the BuildFlow surface | Personal rule: `wc -l AGENTS.md` before and after every edit to it |
| E3 | **The AGENTS.md 220-line cap has no canonical-gate enforcement** — only BuildFlow catches it, and D1 shows BuildFlow's surface can be red for environmental reasons | Any session can push AGENTS.md over the cap while `.#gates` stays green | Add a `wc -l` check to the flake (`nix run .#docs-check` or a new check) so the canonical sweep enforces it |
| E4 | **Issue-driven work left the issue untouched.** Implemented, tested, documented — but no comment, no `Closes #5`, no evidence link. The close-the-loop happened only in the repo, not on the tracker | Owner must manually connect fix ↔ issue | For issue-driven tasks: propose the issue comment + `Closes #N` CHANGELOG line as part of the work, gated on owner instruction when the harness forbids pushes |
| E5 | **AGENTS.md and `.buildflow.yml` now both describe the toolchain skew** (I amended the former; the latter already had it) — two docs, drift risk | Split brain: the yaml says "remain upstream tasks", AGENTS.md now says "re-reddened on 5aaf336" | Keep mechanism detail in `.buildflow.yml` (it's the config), keep only the session-facing rule in AGENTS.md; consider trimming the AGENTS.md addition once upstream fixes it |
| E6 | **Overlay path is unit-pinned, not fuzz-covered** — `FuzzCachedResponse_ArbitraryRemoteBodies` never marks the prober draining | Low: deterministic tests pin the property; but the fuzz suite is the repo's stated property-safety net | Add a mark-once-then-read phase to the federation fuzz harness, or a dedicated tiny fuzz target |
| E7 | **Concurrent-session interleaving was noticed late.** The daemon landed 4 commits mid-session and another session's release-prep artifacts (FEATURES.md +22 lines, two status docs) share the diff range — I only reconstructed authorship at review time | Mental-model drift; risk of misattributing changes | Mid-task `git log --oneline` glance when the daemon is active (AGENTS.md warns about it; I under-used the warning) |

## f) TOP 40 NEXT TASKS (ranked by impact)

HARVEST input — route to `TODO_LIST.md` (actionable, bounded) vs `ROADMAP.md` (ideas/decisions). Impact / Effort (S<30min, M 30min–2h, L>2h) / Category.

| # | Task | Impact | Effort | Category |
| - | ---- | ------ | ------ | -------- |
| 1 | Decide release vehicle for the drain (ride staged v0.7.0 vs cut v0.8.0) — blocks every downstream item | Critical | S | Release |
| 2 | Tag + release per decision in #1; verify pkg.go.dev renders the new godoc | Critical | S | Release |
| 3 | Wire `fed.MarkShuttingDown()` into go-health-dashboard hub SIGTERM (the live-hub fix the issue measured) | Critical | M | Feature |
| 4 | End-to-end repro of the issue's measurement: SIGTERM with SSE client held, poll `/readyz` through the drain → now 503 (closes the loop on the filed evidence) | High | M | Verification |
| 5 | Comment on issue #5 with implementation + verification evidence; close or link per owner preference | High | S | Documentation |
| 6 | Settle the BuildFlow profile binary (rebuild from local checkout once HEAD builds, or bless the branch install) — clears D1 for every future session | High | M | Tooling |
| 7 | Add AGENTS.md 220-line cap to the flake checks so `.#gates` enforces it (E3) | High | S | Tooling |
| 8 | Re-run `buildflow --fix --build-mode=full` to exit 0 after #6 lands (full-mode parity with gates restored) | High | S | Quality |
| 9 | Add `Closes #5` cross-reference to the CHANGELOG entry before the tag cuts | Medium | S | Documentation |
| 10 | HARVEST this report's list into `TODO_LIST.md`/`ROADMAP.md` | Medium | S | Process |
| 11 | Update `docs/consumer-verification.md`: federation `Prober` surface grew a method; note dashboard's consumer-side interface still compiles (structural assertion test green) and what bump consumers need | Medium | S | Documentation |
| 12 | Annotate `docs/status/2026-10-11_18-25_merge-unification-withgetonly-removal-buildflow-green.md`: its "buildflow green" claim predates the 5aaf336 license-check red (docs-health ANNOTATE mode) | Medium | S | Documentation |
| 13 | File upstream BuildFlow issue: full-mode spawn env ignores `GOTOOLCHAIN=auto` for license-check (+ the misleading `buildflow upgrade` output, D3) — verify-before-filing first | Medium | M | Bug (upstream) |
| 14 | Refresh `.buildflow.yml` tool_paths comment with the 5aaf336 closure finding (config comments are the incident log — keep current) | Medium | S | Documentation |
| 15 | Run `nix run .#ci-emulation` on current master (go-free PATH emulation; not run this session) | Medium | S | Quality |
| 16 | Run `nix run .#coverage`; eyeball the drain-path coverage delta | Medium | S | Quality |
| 17 | Review the concurrent session's FEATURES.md edits (+22 lines) and release-prep status doc for consistency with my CHANGELOG entry | Medium | S | Review |
| 18 | Add mark-then-read phase to the federation fuzz harness (E6) | Medium | M | Quality |
| 19 | Add `TestProber_MarkShuttingDown` coverage for `RegisterRoutes`-mounted handlers through a real mux (drain through the wired surface, not just direct handler calls) | Medium | S | Quality |
| 20 | Decide aggregate parity: own drain flag vs sources-only (design doc records the current stance; make it an explicit ADR or ROADMAP line) | Medium | S | Decision |
| 21 | Benchmarks: add `ReadinessHandler_Draining` to complete the handler benchmark matrix (overlay is one atomic load — pin it) | Low | S | Quality |
| 22 | Mirror a one-line drain pointer into `aggregate`'s package doc (cross-surface discoverability: "federation's drain is the one sources cannot give you") | Low | S | Documentation |
| 23 | docs/federation-design.md Non-goals: add explicit "no unmark" line (currently only in the drain section) | Low | S | Documentation |
| 24 | Record the overlay boundary decision in `docs/merge-unification-design.md`: hub-side drain is federation-side, deliberately NOT a merge input (primitive stays pure) | Low | S | Documentation |
| 25 | Weekly `nix run .#fuzz-long` budget incl. the drain path | Low | L | Quality |
| 26 | Post-release: `go get github.com/larsartmann/go-health@latest` smoke test (go-release checklist item) | Low | S | Release |
| 27 | Check README's top-level feature list/table for a federation row needing the drain mention (section updated; top matter not re-checked) | Low | S | Documentation |
| 28 | openapi.yaml: consider a 503-with-`shutting_down` example response for the federation description (schema already covers the shape) | Low | S | Documentation |
| 29 | Assert `MarkShuttingDown` availability on the dashboard-side `dashboardProber` structural interface *if/when* the dashboard adopts it (today deliberately not added — the consumer interface is the dashboard's) | Low | S | Quality |
| 30 | Trim the AGENTS.md BuildFlow gotcha amendment once upstream fixes the full-mode spawn env (E5 anti-drift) | Low | S | Cleanup |
| 31 | Standing TODO (pre-existing, re-surfaced): fix or disable the golangci LSP integration (AGENTS.md documents it lying across sessions) | Medium | M | Tooling |
| 32 | Standing TODO (pre-existing): directive-row BLOCKED item — go-version-auto-configure warning clears only when post-v0.4.0 samber-linter propagates through branching-flow; re-check after next samber-linter release | Medium | S | Tooling |
| 33 | Standing TODO (pre-existing): decide whether BuildFlow REPLACES the flake gates as canonical (owner question from 2026-10-08, still open) | Medium | S | Decision |
| 34 | Delete `tools/bin` wrappers + the `.buildflow.yml` `tool_paths` block once the graph no longer requires a newer patch than BuildFlow's toolchain (yaml's own retirement condition) | Low | S | Cleanup |
| 35 | Retire the `.buildflow.yml` `GOTOOLCHAIN: auto` env block when nothing in the graph exceeds the running toolchain (yaml's own revisit note) | Low | S | Cleanup |
| 36 | Consider `WithEvaluationHook` parity for federation (hook surface is root-only; noticed while studying handler surfaces) | Low | S | Feature |
| 37 | Document the drain-window operational interplay: hub drain keeps fetching at the dashboard's push cadence × N remotes — bound the LB drain window accordingly | Low | S | Documentation |
| 38 | Post-release: re-run the 4-docs docs-check + openapi-lockstep against the new tag (mechanical, part of the release checklist) | Medium | S | Release |
| 39 | Sweep for stale `v0.5.1`/`v0.6.0` line references in docs that the drain feature makes misleading (issue cites `probe.go:645 v0.5.1`; docs should cite current line anchors) | Low | S | Documentation |
| 40 | ROADMAP: "hub drain orchestration" — a tiny helper composing MarkShuttingDown + grace sleep + server.Shutdown for hub operators (mirrors `WithShutdownGracePeriod`'s automation at the root) | Low | S | Feature |

## g) QUESTIONS (cannot answer myself)

**Q1 — Release vehicle:** The concurrent session staged a v0.7.0 release-prep today (`docs/status/2026-10-11_00-51_v070-release-prep-staged-tag-gates-green.md`, plus its 18-25 follow-up). Does the federation drain ride into that already-staged v0.7.0, or cut as v0.8.0 after it? I can't see the staged tag's final state or whether it was cut; this decides whether my `[Unreleased]` CHANGELOG entry stays, merges down into a v0.7.0 section, or becomes v0.8.0.

**Q2 — BuildFlow binary:** Is the `chore/linter-v050-guard` branch install (`5aaf336`) on `~/.nix-profile` deliberate and staying? I deliberately did not replace the shared profile (another session owns that branch work, and HEAD `aa9e2e4` doesn't build). If the branch install is staying, full-mode `license-check` stays red until the branch or the dependency graph moves — want me to file the upstream BuildFlow issue (task #13) or wait for that session to land?

**Q3 — Issue #5 close-out:** Close #5 now with an evidence comment (implementation summary + gate results + the fresh-body proof), or keep it open until the tagged release ships? Related: the issue body has a `MANUALLY REVIEWED by @LarsArtmann` checkbox from the filing workflow — should fix-applied issues get the same review checkbox ritual, or is that filing-only?

---

*Report by Crush (GLM-5.3) — session-scoped per instruction: no research beyond what this session touched. Point-in-time snapshot; section (f) is HARVEST input pending instruction.*

💘 Generated with Crush
