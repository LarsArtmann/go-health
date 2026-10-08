# Status Report — brutal self-review, Pareto plan, execution: drift gate, lifecycle bug, checks fuzzing

**Date:** 2026-10-08 19:42 CEST
**Session scope:** three-phase continuation of the 2026-10-08 docs-health audit:
(1) brutal self-review of the audit itself (11 questions, HTML report),
(2) a work-vs-impact sorted execution plan, (3) execution of the top tiers —
interrupted for this report mid-phase-3 with 3 lint findings open.
**Trigger:** "What did you forget? … Create a Comprehensive Multi-Step
Execution Plan … sort by work required vs impact … consider Type models …
well established libs … Run git commit after each smallest change, git push
when done."
**Harness note:** the auto-commit daemon raced the session throughout; several
self-contained changes were swept into `chore:` commits before my explicit
commits (details in §d2).

---

## a) FULLY DONE

### Phase 1 — self-review + plan artifacts

| # | Deliverable                                                                                                                                                                                          | Evidence                                                                                 |
| - | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| 1 | Brutal self-review of the audit: 11 questions answered, 6 findings (2 High, 2 Medium, 2 Low), 1 self-inflicted split brain found                                                                     | `docs/reviews/2026-10-08_19-04_brutal-self-review.html` (nav 6/6 anchors, tags balanced) |
| 2 | Prior-series cross-check: the 2026-10-04 process report's fabricated scorecard (§d1 of its own critique) was still uncorrected — became finding #2                                                   | `docs/status/2026-10-04_13-13_fleet-mapping-html-process-report-and-self-critique.md`    |
| 3 | Execution plan: 9 steps sorted by work-vs-impact, tier-tagged (1%/4%/20%), embedded in the report; type-model and library questions answered inside (reuse the v0.6 designs; zero-dep ADR respected) | report §04                                                                               |
| 4 | Full `nix run .#gates` sweep on unreleased master — replaced the audit's commit-message trust with machine evidence                                                                                  | "all gates green", 19:04, `/tmp/gates-sweep.log`                                         |

### Phase 2/3 — fixes and features executed (each verified before moving on)

| # | Change                                                                                                                                                                                                                                                                                                                                           | Verification                                                                                                                                                                                                                            |
| - | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | v0.5/v0.6 vehicle de-drift: status lines of `docs/servicename-design.md`, `docs/merge-unification-design.md`, `docs/naming-integrity.md` now record the re-venue; TODO_LIST cross-linked                                                                                                                                                         | one-line edits, committed by daemon `233ed4d`                                                                                                                                                                                           |
| 2 | Correction banner on the 2026-10-04 fabricated-metrics HTML (inline, history preserved)                                                                                                                                                                                                                                                          | `5c60f0d`                                                                                                                                                                                                                               |
| 3 | v0.5.0 announcement draft — leads with the boot-contract validation change; carries the fresh-user-sim result as durable evidence (closes self-review finding #4)                                                                                                                                                                                | `docs/announcements/2026-10-08_v0.5.0.md`, commit `9b87a88`                                                                                                                                                                             |
| 4 | **Drift-alarm gate**: `docsDriftCheck` script + `checks.docs-drift-check` (flake check) + `apps.docs-check` + default `.#gates` member + AGENTS commands row. Mechanizes CONTRIBUTING checklist items 1/3/5 + the ADR range. Store mode falls back to the CHANGELOG's newest release heading (no `.git` in the flake store)                      | positive: "OK … in sync with v0.5.0"; negative: injected README drift → `DRIFT` + rc=1; `nix flake check` all checks passed; `nix run .#gates -- docs-check` green                                                                      |
| 5 | **REAL BUG found and fixed**: `Probe.Start` armed the refresh-loop WaitGroup + cancel before the initial evaluation; critical-name validation failed without disarming → since v0.5.0, a probe with a rejected critical name hangs FOREVER in `Shutdown()` and every retry-`Start` silently no-ops. Failure path now disarms under the same lock | `aggregate/aggregate_validation_test.go` (the 600s test-timeout hang that found it), `TestStart_CriticalValidationFailure_DisarmsLifecycle` (root), race-clean suite, commit `ad9e46b` + CHANGELOG `[Unreleased]` Fixed entry `a806cd5` |
| 6 | `health/checks` fuzz target `FuzzBatteries` (6 params; pins sentinel-or-nil error surfaces over untrusted paths/thresholds/URL-paths/status-codes/DSNs; per-request status via query param — no handler data race)                                                                                                                               | 10s run: 438k execs, 0 failures; full `nix run .#fuzz` (now 4 targets) green                                                                                                                                                            |
| 7 | `health/checks` benchmarks ×4, measured `-benchtime=1s -count=3`, medians recorded in FEATURES (Disk ~335 ns · Memory ~9.5 µs · HTTP ~18.3 µs · Database ~533 ns)                                                                                                                                                                                | `checks/checks_benchmark_test.go`; FEATURES Performance rows added                                                                                                                                                                      |
| 8 | Fuzz app wiring: `.#fuzz` + `.#fuzz-long` gained the checks target; `fuzz-long.yml` "four targets" + `checks/testdata/fuzz/` artifact path                                                                                                                                                                                                       | flake.nix, workflow edited; app run green                                                                                                                                                                                               |
| 9 | `openDB` helper widened `*testing.T` → `testing.TB` so benchmarks share the fake driver                                                                                                                                                                                                                                                          | builds clean                                                                                                                                                                                                                            |

### Also landed earlier in this conversation (the audit itself, recap)

Living-docs drift fixed (README/AGENTS status lines → v0.5.0, CHANGELOG
`[Unreleased]` Fixed, ROADMAP re-venue), TODO_LIST rewritten open-only,
fresh-user sim against released v0.5.0 executed and passed (proxy resolve, no
replace, batteries + validation + VersionHandler), CONTRIBUTING docs/CHANGELOG
boundary note.

---

## b) PARTIALLY DONE

1. **Lint cleanliness of the new Go code.** First lint run found 12 findings
   (paralleltest ×1, wsl_v5 ×11); one fix round landed (blank lines,
   `t.Parallel()`); **3 findings remain open right now**: `cyclop` on
   `FuzzBatteries` (complexity 14 > max 12) and 2 wsl_v5. Tests stay green;
   the gate is red. This is the blocking tail of plan step 5.
2. **The self-review HTML report's status table** claims "Fixed — banner
   added / labels updated / in announcement" — all true as of now, but the
   daemon committed the report before those fixes landed (see §d2). At push
   time every claim is true; intermediate history temporarily lied.
3. **TODO_LIST sync** (plan step 9): four rows are now done in reality
   (drift-alarm gate, aggregate-validation test, checks fuzz+bench,
   announcement draft) but TODO_LIST still lists them as TODO; the
   ServiceName row's "vehicle label still says v0.5" parenthetical is also
   stale after the de-drift. Not yet deleted/updated.
4. **Commit granularity vs the daemon**: explicit commits landed for the
   banner, announcement, drift gate, lifecycle fix, and CHANGELOG; but the
   drift-gate flake.nix work, the lint-fix round, FEATURES rows, AGENTS rows,
   and the workflow edits were swept into `chore:` daemon commits
   (`2b4268d`, `c6ffd5b`, `712b221`, `d4f7f3d`, `15b669d`, `247dc64`), so the
   history's change-grouping is partially heuristic rather than deliberate.

---

## c) NOT STARTED

1. **Plan step 8 — federation validation semantics doc** (remotes never hit
   `ErrUnknownCriticalService`; fetch-side is a different universe; plan R21).
2. **Plan step 9 remainder — DOMAIN_LANGUAGE + AGENTS symbol-ref anti-rot**
   (bare `probe.go:54`-class line refs → symbol names; plan R20).
3. **Final gate sweep + push.** The session's standing instruction was "git
   push when done"; the tree is ~12 commits ahead of origin with the lint
   gate currently red — not pushed, deliberately.
4. TODO_LIST deletion pass (see b3) and the drift-gate self-check afterwards.
5. From the plan's owner-blocked list (untouched by design): dashboard suite
   against released v0.5.0, consumer test train, CV bump (G3), upstream
   filings (G2), archive sweep, version-skew CI, FEATURES benchmark re-verify
   at `-count=3` fresh runs, the v0.6 type window (ServiceName, merge
   primitive, staged renames).

---

## d) TOTALLY FUCKED UP

1. **Committed before the lint gate.** The lifecycle fix (`ad9e46b`) and the
   checks files went in, then lint found 12 findings — one in probe.go itself.
   The repo's own bar is gates-before-push, and I ran the gates AFTER
   committing. Root cause: momentum ("it compiles and tests pass" ≠ gate
   green). Consequence: the fix history includes a known-red state; the 3
   open findings are the leftover.
2. **Lost the commit-granularity race with the auto-commit daemon.** The
   instruction "git commit after each smallest self-contained change" was
   repeatedly preempted: flake.nix drift-gate work, the report, and the lint
   fixes each got swept into heuristic `chore:` commits seconds after saving.
   The self-review report then claimed statuses whose fixes lived in later
   commits — intermediate history briefly asserted things that were not yet
   true. I did not anticipate the daemon's cadence and did not structure
   writes to race it.
3. **Three burned builds on Nix indented-string basics.** Missing `''$`
   escapes (`${ver…}` interpolated as Nix), a `'''` typo, `pkgs.grep` not
   resolving (needed `pkgs.gnugrep`), and shellcheck SC2010 (`ls | grep`).
   Four feedback loops that a standalone `bash -n` + shellcheck dry run of
   the script text would have avoided before touching the flake.
4. **Wrote code against APIs I had not read.** The aggregate test called
   `Shutdown()` expecting an error return (it returns nothing); the first
   fuzz draft shared `requestedStatus` with the handler goroutine (a data
   race I caught pre-commit only because I re-read the file). "Read before
   write" applied to signatures, not just files.
5. **Trust chain slip inherited from the audit**: the session started because
   the audit trusted a commit message over running the gates — and this
   session's phase 1 repeated the pattern at smaller scale (committing the
   fix before running lint). Same failure class, one level down.

---

## e) WHAT WE SHOULD IMPROVE

1. **Gate order is part of the definition of done**: tests green ≠ done;
   lint/vet/gosec green ≠ done until run. Run `nix run .#gates -- lint`
   before every commit, not after.
2. **Check return signatures before writing call sites** — one LSP symbols
   query (`lsp_symbols`/definition) per unfamiliar function costs seconds and
   would have prevented d4 entirely.
3. **For Nix-embedded shell scripts: author the script standalone first**
   (`bash -n` + shellcheck on a plain file), then lift it into the `''`
   string with the escape map ( `${` → `''${`; backslashes literal) applied
   mechanically.
4. **Fuzz targets must be race-free by construction**: per-request state rides
   in the request itself (query param), never in shared fuzz-body variables.
5. **Daemon-aware committing**: for surgical sessions, either pause the
   daemon or write+commit each change in one step; and never write
   forward-looking "Fixed" claims into an artifact that commits before the
   fix does — phrase statuses as of the artifact's own commit.
6. **The cyclop limit (12) is a design signal**: FuzzBatteries at 14 wants
   the per-battery invariant checks extracted into helpers
   (`fuzzDisk`, `fuzzMemory`, `fuzzHTTP`, `fuzzDatabase`) — which also makes
   each surface independently testable.
7. **Keep using the "one real bug per test-writing session" leverage**: the
   aggregate-composition test was written to close a coverage TODO and found
   a shipped-bug instead — integration tests over composition seams are the
   highest-yield lines in this repo (the 2026-09-04 lifecycle race was found
   the same way).

---

## f) Up to 50 things we should get done next

_Ranked by impact vs effort. Items 1–6 are the interrupted tail of THIS
session's plan; 7+ are the standing TODO_LIST/backlog. (B) = blocked on
owner/external._

| #  | Task                                                                                                                                                  | Bucket     | Impact  | Effort |
| -- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | ---------- | ------- | ------ |
| 1  | Fix the 3 open lint findings: extract per-battery fuzz helpers (cyclop 14→≤12) + 2 wsl blanks                                                         | lint       | High    | 15min  |
| 2  | TODO_LIST sync: delete 4 done rows (drift gate, aggregate test, checks fuzz/bench, announcement), refresh ServiceName row note, re-run `.#docs-check` | docs       | High    | 15min  |
| 3  | Federation validation semantics doc (plan R21)                                                                                                        | docs       | Low-Med | 25min  |
| 4  | DOMAIN_LANGUAGE + AGENTS symbol-ref anti-rot (plan R20)                                                                                               | docs       | Low     | 30min  |
| 5  | Final full `nix run .#gates` sweep                                                                                                                    | verify     | High    | 10min  |
| 6  | Push (decision: only after 1–5)                                                                                                                       | release    | High    | 2min   |
| 7  | (B) go-health-dashboard full suite against released v0.5.0 — the one deep consumer; also covers the lifecycle-fix regression risk                     | verify     | High    | 40min  |
| 8  | (B) Decide v0.5.1 bugfix release for the Shutdown-hang fix (it shipped in v0.5.0; sitting in `[Unreleased]`) vs waiting for the next feature release  | release    | High    | owner  |
| 9  | (B) File go-appkit/health + cqrs-htmx/health upstream issues (drafts ready, G2)                                                                       | upstream   | High    | 40min  |
| 10 | (B) Bump CV to go-health v0.5.x + go 1.27 (G3)                                                                                                        | consumer   | Med     | 45min  |
| 11 | (B) Consumer test train vs v0.5.0 tag: fir, KeyHolderAI, DiscordSync, go-taskqueue, webphone, nsfw-classifier                                         | verify     | Med     | 45min  |
| 12 | (B) Publish the v0.5.0 announcement (draft ready)                                                                                                     | owner      | Med     | 10min  |
| 13 | (B) Publish v0.1.1/v0.1.2 announcement (draft ready since 2026-09-04)                                                                                 | owner      | Low     | 15min  |
| 14 | (B) Post samber/do#318 comment (draft ready)                                                                                                          | owner      | Med     | 5min   |
| 15 | (B) Coverage-threshold CI job decision                                                                                                                | owner      | Med     | 20min  |
| 16 | Version-skew CI script: fleet go.mod pins vs latest tag, fail-on-drift                                                                                | automation | Med     | 45min  |
| 17 | FEATURES benchmark re-verify at fresh `-count=3` (all rows, not just checks)                                                                          | verify     | Low-Med | 45min  |
| 18 | Status-report archive sweep: 2026-09-15_08-55 (route §f first), 2026-09-18_09-46, four 2026-09-22 reports, three 2026-09-04 anchors                   | docs       | Low     | 2h     |
| 19 | Aggregate handler (HTTP-path) benchmarks complementing the merge benchmark (ROADMAP)                                                                  | code       | Low     | 45min  |
| 20 | Feed golden-fixture inputs into the aggregate fuzz seed corpus (ROADMAP)                                                                              | code       | Low     | 30min  |
| 21 | Throttled live path benchmark under contention (ROADMAP)                                                                                              | code       | Low     | 40min  |
| 22 | Throttle-window boundary fuzz with fake clock (ROADMAP)                                                                                               | code       | Low     | 45min  |
| 23 | Combine aggregate handler fuzz with throttle/cache modes (ROADMAP)                                                                                    | code       | Low     | 45min  |
| 24 | `-count=N` race-suite stress in CI if flakiness stays zero (ROADMAP)                                                                                  | CI         | Low     | 30min  |
| 25 | OpenTelemetry spans on Evaluate via the hook seam (ROADMAP Theme 2)                                                                                   | feature    | Med     | 2h+    |
| 26 | `Response.TotalLatencyMs` as float64 for sub-ms precision (ROADMAP Theme 2)                                                                           | feature    | Low     | 1h     |
| 27 | `Probe.Snapshot()` structured-logging accessor (ROADMAP Theme 2, demand-gated)                                                                        | feature    | Low     | 1h     |
| 28 | `AwaitReady` cache-aware poll interval (ROADMAP Theme 1)                                                                                              | feature    | Low     | 1h     |
| 29 | `errors.Join` in `aggregate.New` (next-minor candidate, design + spike ready)                                                                         | feature    | Med     | 1h     |
| 30 | `Aggregate.SourceStatuses()` per-source accessor (next-minor candidate)                                                                               | feature    | Med     | 1h     |
| 31 | `federation.Prober.Healthz()` parity design note (next-minor candidate)                                                                               | design     | Low-Med | 45min  |
| 32 | Design note: `WithTransitionHook` (ROADMAP harvested idea)                                                                                            | design     | Low-Med | 1h     |
| 33 | Design note: `healthtest` consumer helper package                                                                                                     | design     | Low-Med | 1h     |
| 34 | Extend openapi.yaml to cover federation endpoints                                                                                                     | docs       | Low-Med | 1h     |
| 35 | Port the drift-alarm idea to the fleet: same gate in the other 15 consumers (mechanize fleet-wide doc sync)                                           | automation | Med     | 2h     |
| 36 | Extend `.#docs-check`: verify AGENTS "Packages" list == go.mod packages; FEATURES option count == `grep -c '^func With'`                              | automation | Low-Med | 30min  |
| 37 | Add `.#docs-check` to the `ci-emulation` gate list (git-free PATH coverage parity)                                                                    | automation | Low     | 10min  |
| 38 | Annotate the archived `2026-09-16_11-46` §c-style bullet lists the tooling skipped (completeness tail)                                                | docs       | Low     | 30min  |
| 39 | Decide the "keep window" policy for docs/status/ (newest 2–3 non-archived)                                                                            | decision   | Low     | 15min  |
| 40 | Add the docs-health sweep step to the release checklist in CONTRIBUTING (process, not memory)                                                         | process    | Low     | 10min  |
| 41 | Wire `.#docs-check` failure into the release checklist as a pre-tag step                                                                              | process    | Low     | 5min   |
| 42 | Sweep the archived reports for remaining bare items (grep gate)                                                                                       | docs       | Low     | 30min  |
| 43 | Re-run the internal-link sweep across docs/** (not just living docs), skipping code fences                                                            | verify     | Low     | 30min  |
| 44 | Render-check the two HTML reports (screenshot pass, not just structural grep)                                                                         | verify     | Low     | 20min  |
| 45 | Reconcile the 2026-10-04 process report v2 upgrades (derived statistics) with this report's counts — or retire the series                             | docs       | Low     | 30min  |
| 46 | Consider `checks` package coverage report (plan R13 remainder: coverage-gap close)                                                                    | verify     | Low-Med | 30min  |
| 47 | Document the fuzz corpus signature-freeze contract in checks/checks_fuzz_test.go (done) + add the same note to the other three targets                | docs       | Low     | 15min  |
| 48 | `WithShutdownGracePeriod` interaction test for the new disarm path (failed Start while grace configured)                                              | test       | Low-Med | 20min  |
| 49 | Add `ErrUnknownCriticalService` composition test for federation-adjacent standalone probes (mirror of the aggregate one)                              | test       | Low     | 15min  |
| 50 | Re-run this self-review series after the v0.5.1/v0.6 decision to measure whether the drift gate + checklist mechanization closed the class            | process    | Low     | 20min  |

---

## g) Questions I cannot answer myself

1. **Push policy for this session:** the tree is ~12 commits ahead of origin
   with 3 lint findings open. Do you want me to finish items 1–5 and push
   gates-green (my recommendation), or push the current state now and fix
   lint in a follow-up?
2. **Release vehicle for the Shutdown-hang fix:** it is a real bug in
   released v0.5.0 (hang on `Shutdown()` after a rejected critical name).
   Do you want a v0.5.1 cut once gates are green, or should the fix ride in
   the next feature release? (This is a G1-class timing call.)
3. **Daemon vs commit discipline:** the auto-commit daemon kept sweeping
   half-finished work into `chore:` commits, which made "commit after each
   smallest change" produce misleading intermediate history (§d2). Should
   execution sessions like this pause the daemon, or should I keep racing it
   and phrase artifact claims conservatively?

---

_Point-in-time snapshot of the 2026-10-08 session (audit → self-review →
plan → execution, interrupted at plan step 5's lint tail). Counts and statuses
verified against the working tree and git log at report time._
