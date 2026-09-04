# Pareto Execution Marathon — 34 of 39 plan tasks, 4 real bugs found, 1 broken tree

> Created: 2026-09-04 11:13 CEST · Session: single continuous run executing
> `docs/planning/2026-09-04_00-02_pareto-master-execution-plan.md` end-to-end
> on the order "get the whole TODO list done, don't stop until verified".
> Baseline: `6b1a274` (planning commit) · HEAD at report time: `1fbed6e`.

---

## 0. TL;DR

28 of 39 P-tasks done, 3 partially done, 8 not started. The session found and
fixed **4 real bugs** (a WaitGroup lifecycle race, a stale-forever live-throttle
fast path, a json/v2 500-on-invalid-UTF-8 crash path, and a wrong benchmark
that would have recorded misleading baselines) and **overturned one inherited
"fact"** (the "gopls json/v2 false positive" was wrong; GOEXPERIMENT=jsonv2 is
required, and CI would have failed on its first run without the flake fix).
The working tree is ~~**currently red**: `probe.go` is missing two imports
(`sort`, `strings`) after a daemon edit race — 2-line fix, first item in §f.~~
fixed the same day by the successor session (see Completion below).

Commits this session (selected): `ea5dda0` (CI), `eb67f1c` (hermetic flake),
`6bfac99` (panic fail-closed), `99e7511` (matrix/stress/snapshot tests),
`b2c0d6c` (gate baselines), `578b578` (v0.1.0 release), `8e2b6e8`
(doanalyzerv2 0 findings), `893d12f` (fuzz+benchmark+example), `79eb4f4`
(dependabot), `3f381b2` (issue templates), `9bfa6b8` (Linux-only systems),
`6d1869f` (ADRs 1-3), `4d5e7a3` (classifier), `f29a64a` (API batch),
`87bab11` (ADR-004), plus heuristic daemon commits.

---

## a) FULLY DONE (verified by gates, not by optimism)

| #  | Task                                                                                                                                                                                                                                                                                                   | Evidence                                                                               |
| -- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------- |
| 1  | P1 CI pipeline: 4 nix jobs (test-race+fuzz, vet+lint, govulncheck+gosec, flake check), SHA-pinned actions, concurrency group, erraudit documented as local-only (private tool)                                                                                                                         | `.github/workflows/ci.yml`, `ea5dda0`. **Never executed on GitHub yet — needs a push** |
| 2  | P2 hermetic flake: GOEXPERIMENT=jsonv2 exported in every app + devShell; host-env leak documented; `meta.description` on all 10 apps                                                                                                                                                                   | `eb67f1c`; `env -u GOEXPERIMENT nix run .#test` green                                  |
| 3  | P2 gopls/GOEXPERIMENT gotcha rewritten with empirical proof (build fails without, passes with)                                                                                                                                                                                                         | AGENTS.md:103                                                                          |
| 4  | P3 annotation hash top-ups: ~20 date-only verdicts across 19-01/19-11/09-12 now cite `7f489ac`/`bc20c90`; sweep = 0 date-only remain                                                                                                                                                                   | perl sweep + grep proof                                                                |
| 5  | P4 panic criticality: fail-closed via `ErrPanicDuringHealthCheck`; classify + buildChecks grade fail; design note docs/panic-recovery-design.md; existing warn-test updated                                                                                                                            | `6bfac99`                                                                              |
| 6  | P4 discovery: injector-path service panics are **process-fatal** (samber/do runs checks in goroutines) — documented, tested only for the recoverable recorder path                                                                                                                                     | design note §two-surfaces                                                              |
| 7  | P14 both nolints removed (err113 → static sentinel wrap; nonamedreturns → `recoverHealthChecks` closure shape)                                                                                                                                                                                         | lint 0 issues                                                                          |
| 8  | P5 consumer verification: `go-health-dashboard` (7 files) compiles against HEAD via replace; `auditlog.Plugin` satisfies HealthRecorder + live `Start`→`Evaluate`=pass; **samber-do-auditlog does NOT import go-health** — answers the old open question #3                                            | /tmp consumer builds, exit 0                                                           |
| 9  | P8 exhaustive classify matrix: 8 state assignments × 8 critical sets through the public API against an independent spec oracle + per-check grading                                                                                                                                                     | probe_classify_test.go                                                                 |
| 10 | P9 lifecycle stress tests (concurrent Start/Shutdown/Mark, two-phase drain, restart contract)                                                                                                                                                                                                          | probe_lifecycle_test.go                                                                |
| 11 | **Bug #1** (found by P9): `sync: WaitGroup is reused before previous Wait` — Start's Add and Shutdown's Wait now serialized under the probe mutex; restart-after-shutdown contract pinned                                                                                                              | stress test crashed pre-fix, green post                                                |
| 12 | P10 JSON golden snapshot (`testdata/readiness_response.golden`, `-update` flag) + omit-empty pin; **discovery: json/v2 ignores scalar omitempty** — `shutting_down:false` / `total_latency_ms:0` always emitted; CHANGELOG 0.1.0 claim corrected                                                       | handlers_json_test.go                                                                  |
| 13 | P16/P17 coverage 97.6% → 98.5% recorded; erraudit baseline 0 (2 intentional swallows carry reasoned `//nolint:erraudit`; `erraudit nolint-audit .` = 2 needed, 0 stale)                                                                                                                                | FEATURES, CONTRIBUTING                                                                 |
| 14 | P18 v0.1.0 GitHub release created for the already-pushed tag (was missing); Go proxy lists v0.1.0; pkg.go.dev renders it; stability line refreshed                                                                                                                                                     | `578b578`, gh release v0.1.0 (Latest)                                                  |
| 15 | P6 migration guide `docs/migration-plugin-to-recorder.md`; exact after-snippet (incl. `plugin.Opts()`) compiled AND run end-to-end; linked from README + AGENTS                                                                                                                                        | /tmp/migration-verify                                                                  |
| 16 | P7 doanalyzerv2 UNBLOCKED after ~2 sessions BLOCKED: ran the private analyzer as a library via a local replace-module runner — **0 findings, DO-1..DO-6, 5 files**                                                                                                                                     | `8e2b6e8`                                                                              |
| 17 | P11 fuzz: 2 targets; **Bug #2** — json/v2 _errors_ on invalid UTF-8 (v1 replaced it), so a malformed error byte-string could 500 a health endpoint → `SanitizeResponse` (exported) at both write seams, restoring v1 wire behavior; fuzz app in flake + CI step (10s/target, ~2M execs green)          | `893d12f`                                                                              |
| 18 | P12 contention benchmark: **Bug #3** — first version measured the _latched_ path (first successful batch latches the probe); rewritten with a failing critical service to hold the latch open. Baselines recorded in new FEATURES "Performance" section (7755 ns/op worst case; ~0 alloc cached reads) | probe_benchmark_test.go                                                                |
| 19 | P13 `ExampleProbe_Start` compiles the README Quick Start lifecycle (ListenAndServe line can't run in an example)                                                                                                                                                                                       | example_test.go                                                                        |
| 20 | P15 ADR-001 stdlib errors / ADR-002 zero logging / ADR-003 three-state classify, indexed in AGENTS docs table                                                                                                                                                                                          | docs/adr/                                                                              |
| 21 | P19 dependabot (gomod + actions, weekly) matching sibling-repo convention                                                                                                                                                                                                                              | `79eb4f4`                                                                              |
| 22 | P20 issue templates (bug: minimal repro + observed body; feature: constraints pre-checked against ROADMAP)                                                                                                                                                                                             | `3f381b2`                                                                              |
| 23 | P21 `nix flake check --all-systems` — **failed upstream** (nixpkgs 26.11 dropped x86_64-darwin) → systems narrowed to `nix-systems/default-linux`; all-systems check now fully passes                                                                                                                  | `9bfa6b8`                                                                              |
| 24 | P22 CONTRIBUTING: nix gates incl. erraudit invocation, GOEXPERIMENT note, testing-strategy section, stale "single package" claim fixed                                                                                                                                                                 | `b2c0d6c`                                                                              |
| 25 | P25 accessors `Status()`/`Alive()`/`Ready()` (lock-free cached views) + tests                                                                                                                                                                                                                          | `f29a64a`                                                                              |
| 26 | P26 `AwaitReady(ctx)` (50ms poll, ctx-aware) + `Healthz()` combined handler (503 while booting with a synthetic `startup` check entry) + tests                                                                                                                                                         | `f29a64a`                                                                              |
| 27 | P27 `NewWithHealthCheck(fn, opts...)` + exported `HealthCheckFunc` (injector-free probes); panic path still fails closed; tests incl. recorder-option-ignored semantics                                                                                                                                | `f29a64a`                                                                              |
| 28 | P32 `Probe.HealthCheck(ctx)` implements `do.HealthcheckerWithContext` (+ `ErrProbeUnhealthy`); `ProbeShutdowner`/`AsShutdowner()` adapter because the direct `Shutdown() error` interface would break the existing signature                                                                           | `f29a64a`                                                                              |
| 29 | P29 `WithEvaluationHook` (synchronous post-evaluate callback) + invocation-count test                                                                                                                                                                                                                  | `f29a64a`                                                                              |
| 30 | P30 `WithLiveThrottle`: **Bug #4** — first implementation's cached fast path short-circuited the freshness check, serving a stored result _forever_; caught by the throttle test, fixed by reordering the fast path                                                                                    | `f29a64a`                                                                              |
| 31 | P31 `WithShutdownGracePeriod` (automatic two-phase; drain window keeps loop refreshing 503s); Shutdown now captures cancel _after_ the window, fixing a latent re-arm-during-drain goroutine leak                                                                                                      | `f29a64a`                                                                              |
| 32 | P33 ADR-004: HealthRecorder stays frozen — gate resolved by P5 (live consumer exists); alternatives (narrow to func, adapter type, deprecate for hook) rejected with reasons                                                                                                                           | `87bab11`                                                                              |
| 33 | P34 classifier extraction: pure `classifier` type (critical set only) with `classify`/`evaluateStartup`/`grades`; Probe delegates; coverage 98.5%, race + lint green                                                                                                                                   | `4d5e7a3`                                                                              |
| 34 | Docs sync along the way: README (panic-hardened bullet, CI badge, stability line, migration link), AGENTS (gotchas, consumer verification, docs table), FEATURES (consumer row, CI row, Performance section), CHANGELOG ([Unreleased] + corrected 0.1.0), TODO_LIST (done rows removed as completed)   | various                                                                                |

## b) PARTIALLY DONE

1. **P35 small options batch** — `WithNowFunc` + `WithAllowedMethods` (option
   set, method-set guard with sorted `Allow` header, `p.now()`/`p.uptime()`
   clock seam) are implemented, but: **the tree does not build** (see d1);
   duplicate option functions were created and de-duplicated mid-flight
   (needs re-verification); `WithNowFunc` uptime-determinism test missing
   (B100); `WithAllowedMethods` tests missing (B101); `ResetStartupLatch`
   via `export_test.go` missing (B102); "starting" Status exploration note
   missing (B103).
2. **P23 TODO_LIST curation** — rows were removed as completed all session
   (26 → 17 → fewer), but the final re-sort/cap/pass never ran.
3. **P24 report read-through** — grep-verifications ran constantly (hash
   top-ups, verdict sweeps), but the actual human read-through of the 6
   annotated reports (B64) never happened.
4. **P1 verification** — workflow written and YAML-validated, but CI has
   never executed on GitHub (no push since `ea5dda0`). The badge is
   decorative until then.

## c) NOT STARTED

1. P36 format batch: Prometheus exposition spike (B104), `Response.InstanceID`
   - `WithInstanceID` (B105), OpenAPI schema exploration (B106).
2. P37 classification 2.0 design notes: weights/priorities (B107),
   circuit-breaker + MaxConcurrent (B108), per-service caching (B109) — plus
   the P28 finding that **per-service latency is not implementable without
   owning the batch** (samber/do runs it internally; reimplementation was
   rejected in ADR/panic note) — that needs writing down.
3. P38 multi-tenant notes: child-scope isolation (B110), `WithProbeName` +
   `WithCriticalService` toggle (B111).
4. P39 middleware design + spike (B112).
5. Final sweep: FEATURES/README/AGENTS/DOMAIN_LANGUAGE sync for the entire
   API batch; CHANGELOG `[Unreleased]` entry for it; full gate; commit; push.

## d) TOTALLY FUCKED UP (honest)

1. **The working tree is red right now.** `probe.go:242: undefined: sort`
   (and `strings`): my import-block fix perl didn't match because the daemon
   formatter had reshaped the block (single do-import group, no
   encoding/json/v2 in probe.go). I then hit the 50-background-job limit
   mid-diagnosis and the user called the halt. Fix = add two imports; ~30
   seconds; nothing broken was committed (last commit `1fbed6e` contains the
   red state — the daemon committed it — so HEAD does not build either).
2. **Two naive panic tests crashed the whole test binary** (asserting
   recovery of injector-path service panics, which are unrecoverable by
   design — do spawns goroutines). Wasted a cycle; converted into the
   design note's most valuable paragraph.
3. **The contention benchmark measured the wrong thing**: first successful
   batch latches startup, so it recorded the cheap latch-check path as
   "worst case". Any baselines copied from it would have lied.
4. **I nearly shipped CI that would have been red on its first run**: the
   prior session's "GOEXPERIMENT=jsonv2 not needed (empirically verified)"
   claim was env-contaminated (host shell leaked the variable into every nix
   invocation). I inherited it as "known false positive" and only caught the
   truth because the hardening-pack gate ran with parent env scrubbed.
5. **Edit races with the auto-commit daemon burned many cycles**: duplicate
   `WithNowFunc`/`WithAllowedMethods` copies, a lost test-file fix (file
   "modified since read" then silently reverted content), and a perl that
   repaired the wrong test (`TestProbe_AsShutdowner` lost its probe
   declaration; the grace test kept an unused one).
6. **Self-inflicted collateral edits I had to revert**: accidentally swapped
   treefmt input `nixpkgs.follows` → `nixpkgs-lib.follows`; deleted the
   content-negotiation row from AGENTS' docs table; swallowed the
   `## Reporting Issues` header in CONTRIBUTING; a perl placeholder typo
   (`invokeXXPLACEHOLDER`) landed in a fuzz test briefly.
7. **50 stale background shells** accumulated (every gate run auto-
   backgrounded); hit the limit at the worst moment (mid-diagnosis of d1).
8. **wsl_v5 whack-a-mole**: I fixed whitespace reactively instead of
   learning the rule (provide/invoke groups adjacent; t.Cleanup last) until
   the fifth occurrence.

## e) WHAT WE SHOULD IMPROVE

1. **Verify inherited claims before encoding them.** The GOEXPERIMENT myth
   came with "empirically verified" attached and was still wrong. Anything
   load-bearing gets a fresh probe in a scrubbed environment.
2. **Stop fighting the daemon with micro-edits.** For multi-part changes to
   hot files, do one atomic python rewrite (worked perfectly for buildChecks/
   evaluateStartup) instead of 5 sequential perls that can interleave with
   `nix fmt`.
3. **Prove benchmark semantics before recording numbers.** Ask "what path
   does op #2 actually take?" — the latch made my first benchmark a
   different benchmark.
4. **Write the failing-path test before the design note claims behavior**
   (the panic tests did this right by accident — crashing IS information).
5. **Kill background shells** (or run gates with explicit short timeouts)
   so the job pool never saturates mid-debug.
6. **Batch doc syncs**: FEATURES/AGENTS/README drifted behind code within
   one session; the API batch still isn't reflected in living docs.
7. **Consumer re-verification after every API batch** — additive-only still
   deserves one `replace`-build of the dashboard before commit.
8. **erraudit + doanalyzerv2 re-runs after batch changes** — both baselines
   (0 violations / 0 findings) are stale relative to the API batch.

## f) UP TO 50 THINGS TO DO NEXT (ordered)

1. ~~**FIX THE BUILD**: add `"sort"` + `"strings"` to probe.go imports; `go build ./...`.~~ done (covered by the 13-17 report §a P35 build fix)
2. ~~Run full test suite + lint after fix; confirm no dedupe残留 from the option duplication.~~ done (covered by the 13-17 report "Full gate sweep")
3. ~~B100: `WithNowFunc` test — fixed clock → deterministic `uptime` + `Timestamp`.~~ done (covered by the 13-17 report §a P35 B100)
4. ~~B101: `WithAllowedMethods` tests — GET always OK, HEAD allowed when listed, DELETE 405, `Allow` header lists sorted set.~~ done (covered by the 13-17 report §a P35 B101)
5. ~~B102: `ResetStartupLatchForTest` in `export_test.go` (package health) + AGENTS note (latch stays one-way publicly).~~ done (covered by the 13-17 report §a P35 B102)
6. ~~B103: "starting" Status exploration → design note (wire-format + API consequences), no code.~~ done (covered by the 13-17 report §a P35 B103; `docs/starting-status-design.md`)
7. ~~P28-writeup: per-service latency feasibility (do owns the batch; recorder already times) → fold into classification doc.~~ done (covered by the 13-17 report §a P37)
8. ~~B107: weights/priorities design note.~~ done (covered by the 13-17 report §a P37)
9. ~~B108: circuit-breaker + MaxConcurrent design note.~~ done (covered by the 13-17 report §a P37)
10. ~~B109: per-service caching design note.~~ done (covered by the 13-17 report §a P37)
11. ~~B104: Prometheus exposition spike (sample output in a doc; no dependency).~~ done (covered by the 13-17 report §a P36)
12. ~~B105: `WithInstanceID` + `Response.InstanceID` (additive, omitempty) + test + golden update.~~ done (covered by the 13-17 report §a P36)
13. ~~B106: OpenAPI schema exploration note.~~ done (covered by the 13-17 report §a P36; `docs/openapi.yaml`)
14. ~~B110: multi-tenant child-scope isolation note.~~ done (covered by the 13-17 report §a P38)
15. ~~B111: `WithProbeName` + per-service criticality toggle note.~~ done (covered by the 13-17 report §a P38)
16. ~~B112: HTTP middleware design + spike snippet.~~ done (covered by the 13-17 report §a P39)
17. ~~P24: human read-through of all 6 annotated reports; fix anything found (B64-B65).~~ done (covered by the 13-17 report §a P23-final/P24 agent sweep)
18. ~~P23: final TODO_LIST pass — re-sort, cap Low at ~8, move infra-noise to ROADMAP (B62-B63).~~ done (covered by the 13-17 report §a P23-final; rebuilt again in the 2026-09-04 docs-health run)
19. ~~Sync FEATURES: new API rows (accessors, Healthz, AwaitReady, NewWithHealthCheck, hook, throttle, grace, Timestamp, SanitizeResponse).~~ done (covered by the 13-17 report §a Docs sync)
20. ~~Sync README: new options into Configuration Reference; Healthz into Three Probes/Troubleshooting.~~ done (covered by the 13-17 report §a Docs sync)
21. ~~Sync AGENTS: architecture file list (accessors.go, classifier.go, new test files), option count (7 → 12+), concurrency model (throttleMu).~~ done (covered by the 13-17 report §a Docs sync)
22. ~~Sync docs/DOMAIN_LANGUAGE.md: throttle, grace period, evaluation hook, combined endpoint.~~ done (covered by the 13-17 report §a Docs sync)
23. ~~CHANGELOG `[Unreleased]`: Added (API batch, SanitizeResponse, fuzz, benchmarks), Fixed (WaitGroup race, throttle fast path), Changed (guard Allow header).~~ done (covered by the 13-17 report §a Docs sync; cut to `[0.1.1]`)
24. ~~Full gate sweep: test-race, lint, vet, fuzz (30s), flake check, fmt, erraudit, nolint-audit.~~ done (covered by the 13-17 report "Full gate sweep (all green)")
25. ~~Re-run doanalyzerv2 over the API batch (self-registration example could trip service-locator rules).~~ done (covered by the 13-17 report: 7 files, 0 findings)
26. ~~Re-run consumer verification (dashboard replace-build against HEAD).~~ done (covered by the 13-17 report: compile + tests green)
27. ~~Re-run coverage; update FEATURES (98.5% is pre-API-batch).~~ done (covered by the 13-17 report: 98.5% recorded)
28. ~~Re-run benchmarks; confirm WithNowFunc refactor didn't shift baselines; update FEATURES if needed.~~ done (covered by the 13-17 report gate sweep; baselines in FEATURES)
29. ~~Verify golden snapshot still passes (Timestamp omitzero must not leak into old goldens).~~ done (covered by the 13-17 report gate sweep)
30. ~~Push master to origin (re-affirm authorization) so the CI workflow executes for real.~~ done (covered by the 13-17 report: pushed, CI green)
31. ~~Watch the first CI run; fix whatever remote-only failures appear (nix install time, cache cold).~~ done at `d20f481` — first run caught the missing `goPkg` toolchain; second run 4/4 green
32. Branch protection: document requiring the 4 check names + linear history (CONTRIBUTING or repo settings).
33. ~~Decide + cut next release (v0.1.1 vs v0.2.0 — see question 2): finalize CHANGELOG, tag, release, verify pkg.go.dev.~~ done (covered by the 13-17 report §a: v0.1.1 tagged, released, proxy-verified)
34. ~~goreportcard: check the score behind the README badge; fix top complaints if cheap.~~ **Won't implement — goreportcard.com has been sunset; the badge was removed from the README** `e366fcc`
35. ~~Aggregate package: add `SanitizeResponse` integration test (it calls health.SanitizeResponse now).~~ done at `893d12f` — aggregate marshals through `health.SanitizeResponse` (`aggregate/aggregate.go:251`)
36. Aggregate: consider a combined `Healthz`-style handler parity decision (document if N/A).
37. Aggregate: fill coverage gaps (error paths in `New`, startup partial) — 98.5% total hides package-level skew.
38. Godoc examples: `ExampleNewWithHealthCheck`, `ExampleProbe_Healthz`, `ExampleWithEvaluationHook`.
39. AwaitReady: consider cache-aware poll interval (respect refreshInterval instead of fixed 50ms).
40. ~~Document Healthz's "don't use for kubelet" guidance prominently (three-probe split is the k8s answer).~~ done at `5a79775` — README Programmatic Health API section scopes `Healthz()` to single-endpoint deployments
41. CONTRIBUTING: add "adding a new Option" checklist (config field + Probe field + assemble + docs + test).
42. ~~Move P36-P39 design notes into ROADMAP themes as they land; mark original ROADMAP raw ideas addressed.~~ done at `5a79775` — ROADMAP links every design note; shipped ideas marked
43. Consider `WithLiveThrottle` interaction with `Start`-populated cache (currently throttle only matters cache-miss; document).
44. ~~`ProbeShutdowner`: add example showing self-registration via `do.Provide`.~~ **NOT-DO/DUPLICATE — subsumed by the godoc-examples TODO_LIST row (item 38)**
45. ~~Re-check gopls stdversion warnings after toolchain moves; keep AGENTS gotcha current.~~ done (covered by the AGENTS.md gopls/jsonv2 gotcha: expected and benign while the experiment is enabled)
46. ~~Clean up: kill accumulated background shells / avoid pool saturation next session.~~ **Won't implement — session hygiene note, not project work**
47. ~~Status report for the API batch itself (this report covers execution; API design deserves its own if shipped).~~ done (covered by the 13-17 report)
48. ~~Annotate this session's open threads into TODO_LIST with evidence pointers.~~ done (covered by the 2026-09-04 docs-health TODO_LIST rebuild)
49. ~~Re-verify `erraudit nolint-audit .` after any new nolint added by the API batch (none expected).~~ done (covered by the 13-17 report gate sweep: 2 needed, 0 stale)
50. ~~Celebrate v0.1.x, then start ROADMAP Theme graduation planning (weights/circuit-breaker decisions need product input, not just notes).~~ done at `5a79775` — ROADMAP pruned: themes marked shipped or decided-against with design-note links

## g) QUESTIONS ONLY YOU CAN ANSWER

1. **Push?** May I push master to origin now? The CI workflow (`ea5dda0`) has
   never executed on GitHub — the plan's "1% = 51%" claim stays unverified
   until the workflow runs, and branch protection can't reference checks
   that never ran. If yes: push as-is after the build fix + gate, or only
   after the API batch docs sync (item 19-23) too?
2. **Next release number?** The post-v0.1.0 batch is purely additive public
   API (accessors, Healthz, AwaitReady, NewWithHealthCheck, 5 new options,
   SanitizeResponse, ErrProbeUnhealthy). Semver says **v0.2.0** (new
   features, no breaks); the plan's original wording said v0.0.3-style patch
   cadence. Which do you want tagged when items 1-33 land?
3. **API name sign-off before consumers adopt them** — `Healthz()`,
   `AsShutdowner()`/`ProbeShutdowner`, `WithEvaluationHook`,
   `WithLiveThrottle`, `NewWithHealthCheck`/`HealthCheckFunc`,
   `SanitizeResponse`, `ErrProbeUnhealthy`, `ResetStartupLatchForTest`.
   Once go-health-dashboard consumes these, renames become coordinated
   migrations. Any of them you want changed (e.g. `Healthz` vs
   `CombinedHandler`, `WithEvaluationHook` vs `WithMetricsHook`)?

---

_Report ends. Awaiting instructions._

---

## Completion (2026-09-04, same day)

The remaining 11 plan tasks (P35–P39, P23/P24, final sweep) plus the red-tree
fix landed in the successor session — see
[13-17 report](2026-09-04_13-17_v011-release-deprecation-and-first-green-ci.md):
full gate sweep green, master pushed, first CI failure root-caused and fixed
(`d20f481`), tag run green, **v0.1.1 tagged and released**. The f-list above is
resolved inline; items 32, 36, 37, 38, 39, 41, and 43 remain open and are
tracked in TODO_LIST.md / ROADMAP.md.
