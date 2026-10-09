# Status Report — Pareto v2 Resume: A32 Refined + Verified, BuildFlow Skew Pinned Upstream, Fleet Trains Rolling

**Date:** 2026-10-09 21:09 CEST · **Session:** continuation of the Pareto-plan-v2 execution under the owner's resume directive ("execute and verify one step at a time, repeat until done"), interrupted by this status demand mid-train · **Scope of this report:** this session only (resumed from the 16:53 pause point: unverified A32 edit, three lost background jobs, A07/A06 tails, consumer trains, doc-tail sweep).

---

## a) FULLY DONE (this session)

### A32 closed — and the fix was WRONG before this session reviewed it

- The daemon had committed the prior session's unverified grace fix (`6a3cff2`). Race suite + lint green on it — but critical review found a **design defect**: keying the skip on `p.cancel != nil` also skipped the grace window in **live mode** (Start succeeded, `WithRefreshInterval(0)` — no loop armed), silently breaking the option's documented contract ("Shutdown blocks for the grace window") for a case the fix's own rationale never claimed to cover ("a probe that never served").
- **Refined:** new `lifecycleStarted atomic.Bool`, set on every successful `Start`; `Shutdown` honors the window for any started probe (cached OR live — the block is the caller's drain window even without a loop) and skips only never-started / failed-validation probes (dead time).
- **Test:** `TestShutdown_GraceWindowSkippedWhenLifecycleNeverStarted` — three subtests (never-started returns fast, failed-Start returns fast, live mode STILL blocks ≥ window). Race-clean.
- CHANGELOG `[Unreleased]` Fixed entry; commit `903ebcf` (+ the daemon's `cfbc878` carrying the code files — see §d1).

### The three lost background jobs — all collected

- **Job 102 (CI on `4476cd0`):** Security/Vet+Lint/Flake/OpenAPI success; Test(race) _cancelled_ (superseded by the release push) — acceptable.
- **CI on the release commit `23db7fe`:** **all five checks success** → **B008 complete: CI green on the v0.5.1 tag.**
- **Job 107 (BuildFlow build):** completed — but from a `-dirty` source tree (the dirty bits were daemon-committed afterwards).

### A06 tail — release propagation proven

- **B021:** module proxy `@latest` resolves **v0.5.1**; version list carries it. **pkg.go.dev renders v0.5.1** (published Oct 9, 2026) with the full README — and the render exposed that the README compat table was stale (see B109 below).

### B026 — BuildFlow rebuilt, installed, doctor green

- Old profile entry was `812c6be` (older than the summary's `ec8d2d3` claim — that number was stale). Rebuilt from the now-clean master → `1b99ae2`; `nix profile` upgrade-by-name failed (matcher quirks), fixed with remove + reinstall; `buildflow doctor`: **binary-freshness ✓** ("current, built at HEAD 1b99ae2"). Doctor's 30 "failed" checks are missing optional tools for unused ecosystems (bandit/cargo-\*/eslint/…) — not blockers.

### B027, B028 — A07's verifiable halves

- **B027:** `golangci-lint [tools/doanalyzerv2]` exit 0; `govulncheck` exit 0 (both modules).
- **B028:** AGENTS "Go-directive patch floors" gotcha rewritten with the honest new state (binary rebuilt; full-mode env loss persists; working invocation; drop-conditions) — replacing the now-false "verified green in full mode" and "stale binary is the hazard" claims.

### A08 — budget re-review, no ratchet

- **art-dupl: 63 findings** (60/63 in `*_test.go` — the documented test-similarity class; 3 tiny adapter sites in aggregate/federation) — budget 80 holds with headroom.
- **branching-flow: 9 findings**, every one in a documented class, PLUS one new entry the yaml comment didn't cover (`probe.go:69` "21 fields (threshold 15)" god-struct) — annotated into the `.buildflow.yml` comment as the same single-package design decision.
- **go-auto-upgrade: 7 ≤ 8.** All three budgets re-verified with sampled evidence, comments updated.

### Doc/tooling tail — eleven items closed

| Item | Result                                                                                                                                                        |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| B114 | "(CV)" → "conditional sets" in start-validation-design.md                                                                                                     |
| B109 | README compat table 1.27.1→1.27.2 + GOTOOLCHAIN=auto note + GO-2026-6603..6617 rationale                                                                      |
| B107 | feature_request.md blob verified on github.com (content matches local, frontmatter valid)                                                                     |
| B111 | Completion note backfilled on the 20:58 report (b1 settled, g1/g2 resolved, g3 re-asked)                                                                      |
| B110 | CONTRIBUTING: completion-note convention + keep-window policy written                                                                                         |
| B100 | lychee sweep: 7 breaks, ALL one archived report whose links were never rebased on the move to `archived/`; fixed → **0 errors** (2 redirects left cosmetic)   |
| B101 | Trashed stale `coverage/coverage.out`, `reports/coverage.out`, `result` symlink; kept PMA-tracked `.config/metadata.yaml`                                     |
| B103 | dprint check clean over session-edited markdown; CI role stays with treefmt                                                                                   |
| B104 | lessons.md reconciled — no duplicates; ONE new cross-project lesson (dispatch-requires-collection) committed at the crush-config source (`ad1bcba`, unpushed) |
| B105 | Process-metrics thread: **RETIRED** (no living trace; the 13:13 series' own annotations retired it)                                                           |
| B106 | go-structure-linter: 0 findings                                                                                                                               |
| B108 | `doanalyzerv2-runner` → `doanalyzerv2` rename in main.go + run.sh; build+vet green                                                                            |

### Fleet trains — four repos done, all green (NONE pushed — §g1)

- **A09 dashboard:** baseline on v0.5.0 green (incl. browser/screenshot tests) → bumped to v0.5.1 → full suite green → committed `d84f11f`.
- **A10 KeyHolderAI** (v0.4.1→v0.5.1): vendor tree re-synced (`go mod vendor`; vendor/ is gitignored there), focused probe+handler tests green, committed.
- **A10 DiscordSync** (v0.4.1→v0.5.1): the v0.5.0 start-validation broke two test fixtures exactly as designed — dashboard fixture registered only "database" (Start failed on discord-gateway/projection-runtime → disarmed loop → empty cache), and the readyz-cache tests used the production probe over a single-checker injector. Fixed both (fixture now registers all `criticalHealthServiceNames()`; cache-mechanics tests use a critical-free probe). Full suites green. Two commits.
- **A10 dnsblockd** (v0.4.1→v0.5.1): full suite green including the response-overlay tests; committed (repo's post-commit hook re-verified UI artifacts byte-fresh).
- Discovered en route: **Zlota44, library-policy, nsfw-classifier, PMA were ALREADY at v0.5.1** from prior sessions (library-policy/nsfw/PMA pushed; **Zlota44 3-ahead unpushed**); fleet-skew's remote view lags the local truth by those unpushed commits.

---

## b) PARTIALLY DONE / IN FLIGHT

- **A07 / B025 — buildflow full mode: 95%, exit-0 demand NOT met; root cause pinned UPSTREAM.** Three full runs: (1) no env export → 8 steps fail ("running go 1.27.0" vs the tools/doanalyzerv2 graph needing 1.27.1); (2) with `GOTOOLCHAIN=auto` exported in the launching shell → 6 of 8 fixed; (3) with a `tools/bin/go-licenses` wrapper + `tool_paths` entry → license-check STILL failed and the wrapper was verifiably never reached (`buildflow history --step … --last-error` shows the spawn still ran go 1.27.0). Findings: BuildFlow's locked nixpkgs `go_1_27` = 1.27.0 (closure go); config `env: GOTOOLCHAIN: auto` does not reach full-mode fan-out spawns; `EnsureGotoolchainAuto` can't fire (config env counts as "set"); `tool_paths` is honored in single-step but NOT in full-mode fan-out. The four stragglers (license-check, govalid-generate × root+tools) pass single-step under the same env → a BuildFlow full-mode spawn env/resolution bug, not a go-health defect. The unproven wrapper was REMOVED (premise disproven), `tools/bin/govalid` hardened to unconditional `GOTOOLCHAIN=auto`, `.buildflow.yml` comment updated with the verified truth.
- **A10 webphone:** `go get` done; `go mod tidy` FAILED on vendor inconsistency (needs `go mod vendor`) — interrupted exactly there.
- **A10 go-taskqueue:** go.mod bumped to v0.5.1, tidy clean — build/test/commit not yet executed.
- **fir (file-and-image-renamer):** already v0.5.1 locally (ahead 4) but carries **2 unexamined dirty files** — not inspected this session.
- **B035:** dashboard push/CI handoff note — folded into this report + §g1.

---

## c) NOT STARTED (from the plan's remaining set)

A11 (CV → v0.5.1 + go 1.27 floor), A18 (golangci LSP fix-or-disable — note: the stale panel lie persisted ALL session, `evaluation_hook_test.go:97`, while the file stayed green), A19 (fleet-skew CI workflow — deliberately sequenced after pushed consumers), A20 (benchmarks `-count=3`), A23 (mergeResponses sketch), A25 (errors.Join in aggregate.New), A26 (SourceStatuses), A27 (federation Healthz note), A28/A29 (fuzz corpus + throttle bench/fuzz), A30 (AwaitReady note), A31 (checks coverage), A33 (federation composition test), B102 (owner-blocked evidence refresh), final gates + push + plan EXECUTED stamp, v0.5.1 announcement checklist, adoption-matrix corrections, B112 fuzz-long watch (fires 2026-10-12).

---

## d) TOTALLY FUCKED UP (this session, no anesthesia)

1. **The daemon raced my staging AGAIN on the A32 commit.** I followed "git add in the same breath" — staged code files immediately after edits — but the daemon committed them into its own `cfbc878` between my staging and my commit; my `903ebcf` carries only the CHANGELOG. Tree is correct and verified, but the logical change is split and my commit message under-describes its own commit. The lesson escalates: **add+commit must be ONE command when a commit is planned**; "same breath" across separate tool calls is not same-breath enough.
2. **The go-licenses wrapper was built on an unverified premise.** I hypothesized "full mode resolves the closure go-licenses" and wrote + wired a wrapper before probing the actual spawn; one full 5-minute run later, `buildflow history --last-error` disproved the premise in seconds — a probe I could have run FIRST. Same "output before prose" rule I wrote into last session's §e, applied to infrastructure hypotheses and violated anyway.
3. **`rg -rn` misuse — TWICE.** The `-r` flag is output-rewriting; my "recursive" invocations silently replaced matched text ("tools/doanalyzerv2" → "tools/n", "GOTOOLCHAIN" → "n") and nearly sent me chasing phantom strings. lessons.md documents EXACTLY this trap (the `rg -r` fabrication lesson). I caught it only because the outputs looked absurd.
4. **jq parsing of buildflow JSON failed four consecutive times** (banner prefix before the JSON, a trailing `===`, wrong line slices) before I switched to a sed-range + python extraction that worked in one shot. I should have LOOKED at the raw file once instead of iterating blind jq incantations.
5. **`buildflow --failed-only` exited 69** ("no tools matched the project state") right after a run with 4 failures — unexplained, untriaged, dropped on the time-box. Suspicious enough to record, not enough to chase.
6. **Two trains left mid-flight at the interruption** (webphone at vendor-sync, go-taskqueue pre-test) and fir's 2 dirty files unexamined — the status demand arrived mid-block; noted so the resume point is explicit.
7. **B100's 2 redirecting URLs were never identified** (two failed extractions, then deliberately abandoned as cosmetic). Honest state: unknown targets, deemed low-value without evidence beyond "0 errors".

---

## e) WHAT WE SHOULD IMPROVE

- **Probe before you mitigate:** for any "tool X resolves Y wrong" hypothesis, extract the actual spawn evidence (`history --last-error`, verbose single-step) BEFORE writing wrappers/config. One probe beats one disproof-run.
- **add+commit as one command, always** — the daemon wins every multi-call race (now empirically twice: release commit, A32).
- **Never type `rg -rn`** — the correct recursive form is plain `rg -n`. Consider a shell alias/guard if this recurs.
- **Inspect a raw output file once, then parse** — the jq fumble cost four round trips; `sed -n '/^{/,/^}/p' | python3` worked first try.
- **Time-boxes need written conclusions:** the BuildFlow forensics got a written verdict (upstream bug + evidence), which is why §b is honest instead of vague — keep doing exactly that, but conclude FASTER (the second full run was already conclusive; the wrapper run was the mistake, not the time-box).

---

## f) NEXT — up to 50, impact-sorted

| #  | Task                                                                                                                                      | Note                                                                     |
| -- | ----------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| 1  | webphone: `go mod vendor` + build + suite + commit                                                                                        | finishing, 10 min                                                        |
| 2  | go-taskqueue: build + suite + commit                                                                                                      | finishing                                                                |
| 3  | fir: inspect the 2 dirty files; confirm or complete the v0.5.1 bump                                                                       |                                                                          |
| 4  | A11: CV v0.1.3→v0.5.1 + `go 1.27` floor + suite + flake verify                                                                            | G3                                                                       |
| 5  | B035: write the dashboard cross-repo handoff note                                                                                         | with pushes per §g1                                                      |
| 6  | Push consumer bumps once §g1 is answered (dashboard, KeyHolderAI, DiscordSync, dnsblockd, webphone, go-taskqueue, Zlota44, CV)            | blocked on owner                                                         |
| 7  | A19: `.github/workflows/fleet-skew.yml` (push-to-master + weekly + dispatch); green run vs v0.5.1                                         | needs pushed consumers for a green first run                             |
| 8  | A18: fix-or-disable the golangci LSP integration; record decision                                                                         | this session's panel lied for ~4 h straight                              |
| 9  | A25: `errors.Join` in `aggregate.New` + multi-error tests + CHANGELOG                                                                     | design doc exists                                                        |
| 10 | A26: `SourceStatuses()` + property test + README/FEATURES rows                                                                            | design doc exists                                                        |
| 11 | A23: mergeResponses primitive sketch vs both merge sites + port plan                                                                      |                                                                          |
| 12 | A20: benchmarks `-count=3` (root, aggregate, checks) → medians + spread → FEATURES                                                        |                                                                          |
| 13 | A27: federation `Healthz()` parity design note (503 conditions + latch)                                                                   | no implementation                                                        |
| 14 | A28: golden fixtures → aggregate fuzz seed corpus; `-count=N` race-stress CI step                                                         |                                                                          |
| 15 | A29: throttled live-path contention benchmark; throttle-window boundary fuzz (fake clock)                                                 |                                                                          |
| 16 | A30: AwaitReady cache-aware poll-interval mini note                                                                                       |                                                                          |
| 17 | A31: `go test -cover ./checks`; close top gaps; re-measure                                                                                |                                                                          |
| 18 | A33: federation-adjacent ErrUnknownCriticalService composition test                                                                       | mirror of aggregate one                                                  |
| 19 | B102: BLOCKED rows' evidence refresh (doanalyzerv2 floor under 1.27.2; auditlog ADR-004; coverage threshold)                              |                                                                          |
| 20 | Final gates on go-health (test-race/vet/lint/vulncheck/security/fuzz/docs-check) on the final tree                                        | docs-check should pass: [Unreleased] has content and compare base exists |
| 21 | Push go-health master                                                                                                                     | G1 already ratified for this repo                                        |
| 22 | Stamp plan v2 EXECUTED with per-task annotations                                                                                          |                                                                          |
| 23 | v0.5.1 announcement checklist draft (sibling the 10-02 one)                                                                               | publishing stays owner's (§g2)                                           |
| 24 | Adoption-matrix corrections: WithGETOnly test-only usage, go-daemon/project-discovery-daemon not-consumers row, PMA/doadapter naming      |                                                                          |
| 25 | TODO_LIST refresh from this report                                                                                                        |                                                                          |
| 26 | BuildFlow upstream task: full-mode fan-out env/resolution bug (4 steps, single-step-green divergence) — investigate in the BuildFlow repo | separate session; evidence in AGENTS gotcha                              |
| 27 | Triage `buildflow --failed-only` exit-69 "no tools matched"                                                                               | BuildFlow repo                                                           |
| 28 | B112: watch the 2026-10-12 fuzz-long 4-target run + corpus artifacts                                                                      | calendar                                                                 |
| 29 | gosec unpin watch (needs gosec ≥ #1772 in nixpkgs)                                                                                        | flake comment tracks                                                     |
| 30 | go override drop watch (nixpkgs go_1_27 ≥ 1.27.2)                                                                                         | flake comment tracks                                                     |
| 31 | B113: post-answer docs-health re-audit once §g is answered                                                                                |                                                                          |
| 32 | Next status report after the tail completes                                                                                               |                                                                          |

---

## g) QUESTIONS FOR YOU (cannot self-answer)

1. **Cross-repo push authority (re-asked, now blocking):** eight consumer repos carry green, committed, UNPUSHED v0.5.1 bumps (dashboard, KeyHolderAI, DiscordSync, dnsblockd, webphone\*, go-taskqueue\*, Zlota44, + CV pending; \* = finishing next). May I push their masters as each full suite passes, or does every consumer commit wait for your review? A19's green first run also depends on the remote fleet being current.
2. **Publishing acts (re-asked):** the samber/do#318 comment (staged, citations current) and the v0.5.0 + v0.1.1/v0.1.2 announcements (checklists complete). Under the blanket directive I file in YOUR repos but stop at third-party/social voice: post them now, or do you review and post yourself?
3. **SemVer rule going forward:** v0.5.1 shipped an Added section per the plan's letter against the repo's additive→minor convention — moot for v0.5.1 (proxy picked it up; re-tagging off the table), but `[Unreleased]` already contains a Fixed entry and A25/A26 will add API surface: is the next release **v0.6.0** (convention: additive → minor) or **v0.5.2** (patch-train continuation)? This decides how I cut the next changelog.

---

**Resume point:** §f1 (webphone vendor sync), then §f2–4 finish the train; §f6 waits on answer 1. Track-A standing after this session: **18 complete, 4 partial (A07 at its honest ceiling pending the upstream BuildFlow fix, A10 two repos from done, A14–A16 owner-staged), 13 not started.** The golangci LSP panel still shows the phantom `evaluation_hook_test.go:97` error as of this writing — the file has been race-green all session.

_Prepared with AI assistance (GLM via Crush); every claim in §a/§b verified against tool output captured in this session; daemon-committed splits (`cfbc878`) attributed, not claimed._
