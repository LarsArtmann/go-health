# Status — 2026-09-22 19:51 — TODO sweep: Healthz, lockstep check, tracker A/B benchmarks

Session scope: work through `TODO_LIST.md` "Open — unblocked" rows (repo-local first), per the
user's execute-and-verify loop. Self-review pass included: what was forgotten, what was botched,
what is still open.

Environment note discovered mid-session: `go.mod` now requires **go 1.27.1** and the flake builds
with `go_1_27` — the host `go` (1.26.7) fails with "requires go >= 1.27.1". Bare `go` commands
must go through `nix develop -c …` or flake apps. AGENTS.md still claims go1.26 in two places
(fix listed below). Baselines recorded this session ran on **go1.27.1**, CPU
`AMD RYZEN AI MAX+ 395` (32 threads) — possibly different hardware from the 2026-09-04/09-18
baseline rows; treat cross-baseline comparisons with suspicion.

---

## a) FULLY DONE

1. **TODO harvest of 2026-09-18 resolved rows** — verified each in code before deleting:
   ADR-005 exists, OpenAPI aggregate coverage in spec, aggregate merge property tests
   (`TestAggregateMerge_MergeIdempotent`/`_MergeCommutative`), README "Which probe should I
   hit?" table, `ExampleNewWithDetailedCheck` determinism (asserts `DurationNanos > 0`, no raw
   timing printed). Rows deleted per lifecycle; harvest note updated in TODO_LIST.md.
2. **Fuzz (weekly long) workflow dispatched** — run
   [35756511889](https://github.com/LarsArtmann/go-health/actions/runs/35756511889) via
   `workflow_dispatch` on master. Bonus finding: the TODO's premise ("never executed on GitHub")
   was stale — scheduled runs 2026-09-14 and 2026-09-21 already completed green (15m each), so
   the YAML was already proven on GitHub infra.
3. **pkg.go.dev verification (stronger than the row asked)** — v0.2.0 root + aggregate render
   with metadata fields (Per-Check Metadata docs, `CheckDetail`, `DetailedHealthRecorder`,
   examples) AND v0.3.0 root + aggregate + **federation** all render fully. Proxy
   `@latest` = **v0.3.0** (released 2026-09-19, tag `d5ee523`, includes `federation/`). The
   pkg.go.dev "not in the latest version of its module" banner is cache noise — the proxy is
   authoritative and confirms v0.3.0.
4. **Stale CHANGELOG version-link block fixed** — `[Unreleased]` pointed at `v0.1.3...HEAD`;
   added `[v0.3.0]`/`[v0.2.0]` refs and v-prefixed all labels so headers resolve. (Auto-daemon
   committed this as `c65fbbe`.)
5. **`Aggregate.Healthz()` implemented** (design doc accepted):
   - `aggregate/aggregate.go`: standalone handler mirroring `Probe.Healthz` — 503 iff any source
     unlatched / merged fail (shutdown folded in by `CachedResponse`); synthetic `startup` check
     only when the latch is the reason (mirrors root's guard); `RegisterRoutes` unchanged
     (`DefaultRoutes` already claims /healthz for liveness — the collision the design rejected
     solving via route options).
   - Tests: `TestHealthzHandler_SingleEndpointTable` (5 cases: all latched, one unlatched + body
     assertions, warn stays 200, critical fail w/o startup key, shutdown w/o startup key);
     property test extended (`TestAggregateHandlers_MirrorMergedStatus` now asserts healthz code
     = f(`StartupComplete()`, merged roll-up) across every ordered fixture selection, including
     `ghost`); `ExampleAggregate_Healthz` runnable example. All pass under `-race`.
   - Docs: design doc status → ACCEPTED (implemented 2026-09-22), CHANGELOG `[Unreleased]`,
     AGENTS.md aggregate paragraph, FEATURES.md aggregation row.
6. **OpenAPI ↔ golden-file lockstep check shipped** — new `openapiLockstep` script in flake.nix
   (yq YAML→JSON + jq): every top-level golden wire key must be a declared `HealthResponse`
   property, every per-check key a declared `Check` property, status must be in the spec enum.
   Wired as BOTH `checks.openapi-lockstep` (runs under `nix flake check` → existing CI
   "Flake + Formatting" job + `.#gates`, zero CI-yaml drift) and `nix run .#openapi-lockstep`
   (paths overridable). Verified positive case passes and injected drift (renamed spec property)
   fails. One jq scoping bug found and fixed during verification (`keys[] | X | has(.)` loses the
   key; needs `keys[] as $k | X | has($k)`). Documented: AGENTS.md command table, CHANGELOG,
   TODO row harvested.
7. **`//nolint:erraudit` golangci warning resolved-by-decision (accept + document)** — verified
   upstream: golangci-lint `nolint_filter.go` `Finish()` warns unconditionally on unknown
   directive names; no config gate exists (v2.13.2). Verified erraudit side: its suppression
   engine parses `//nolint[:linter[:rule]]` (`pkg/suppression/parser.go`), so the directives
   work. Rejected bare `//nolint` (would silence the warning but disable every linter on those
   lines). AGENTS.md gotcha written; TODO row harvested.
8. **Stale AGENTS.md gotcha corrected** — old text claimed a live-mode probe "never populates
   its cache"; actually `Start` calls `refreshCache` unconditionally (probe.go:479) even at
   interval 0, so started live-mode probes hold a boot-time snapshot (the aggregate shutdown
   test depends on this). Rewrote the gotcha to the accurate model and named the real
   never-populated case (Start never called).
9. **Benchmark seam + A/B benchmark added** — `transitionTracker.disabled` (test-only field,
   guard-first in `stamp`), `SetTrackerDisabledForTest` in export_test.go (same pattern as
   `ResetStartupLatchForTest`), and `BenchmarkEvaluate_TrackerDelta` (8 stable checks, A/B in
   one binary, `b.Loop()`).

## b) PARTIALLY DONE

1. **Benchmark task row (FEATURES.md delta + re-baseline)** — measurement DONE, write-up NOT:
   - `BenchmarkEvaluate_TrackerDelta` (go1.27.1, 32 threads): `tracker=false` **892 ns/op ·
     1154 B · 5 allocs** vs `tracker=true` **1149 ns/op · 1682 B · 7 allocs** → Since-stamping
     marginal cost ≈ **257 ns/op (+29%), +528 B, +2 allocs** on the steady-state warm path. The
     +528 B matches the tracker's fresh per-batch map — coherent with the standalone
     `BenchmarkTransitionTrackerStamp` numbers.
   - Re-ran on go1.27.1: `Evaluate_Scaling` 1svc 1080 ns / 8svc 1795 ns / 64svc 15351 ns;
     `TrackerStamp` 1: 190.7 ns, 8: 347.2 ns, 64: 3760 ns; `BenchmarkEvaluate` (2-svc injector)
     3958 ns · 3530 B · 42 allocs.
   - **NOT done**: FEATURES.md "Performance" table not yet updated (no delta row, no re-baseline
     note), only the Evaluate/tracker family re-ran — liveness/readiness/startup/guard/aggregate
     rows not re-measured, TODO row not harvested, CHANGELOG has no entry for the seam+benchmark.
2. **Gates verification** — aggregate `-race` green after Healthz; targeted lint green
   (0 issues); `nix flake check` green (includes the new lockstep check). BUT the full gate
   sweep has not re-run on the final tree (the earlier background `.#gates` run started mid-edit
   and died at lint on a `wsl_v5` finding I then fixed), and root `-race` hasn't re-run since the
   tracker seam landed.
3. **AGENTS.md staleness sweep** — two gotchas fixed (see a8); the header line still says
   "**Go**: 1.26 · **Status**: v0.2.0 (alpha), federation unreleased (v0.3.0 vehicle)" and the
   GOEXPERIMENT gotcha still says "The flake keeps go_1_26 pinned" — both falsified by go.mod
   1.27.1 + flake `go_1_27` and the 2026-09-19 v0.3.0 release. Noticed, not yet fixed.

## c) NOT STARTED (still open in TODO_LIST.md or surfaced this session)

- Prose review: `middleware_example_test.go` / `prometheus_example_test.go` wire examples (Low).
- ADR: unify latency units (`total_latency_ms` vs `duration_ns`) in v0.3 (Low).
- Detailed-checks cookbook (Low).
- File samber/do upstream: richer batch results / per-service timing (Medium; requires
  verify-before-filing + github-voice).
- samber-do-auditlog: implement `DetailedHealthRecorder` (Medium; other repo).
- All four go-health-dashboard tasks (High/Medium; other repo): `since` rendering, status-change
  timeline, `duration_ns` adaptive, "stable for Xh" collapse; plus the integration test pinned
  _after_ adoption.
- Owner-blocked (untouched by design): branch protection, coverage-threshold CI, announcement
  publishing.
- New gaps surfaced this session (not yet rows):
  - `Aggregate.Healthz()` parity for **federation** (`Prober` has no `Healthz()` — same
    single-endpoint gap the aggregate just closed).
  - openapi.yaml `info.version` still `0.2.0`; spec says nothing about the root/aggregate
    single-endpoint `Healthz()` mount (acceptable — it's a wiring choice — but worth a sentence).
  - Release-vehicle decision: CHANGELOG `[Unreleased]` (Healthz + lockstep check + benchmarks)
    needs a v0.4.0 (or patch) decision; the TODO note's "v0.3.0 candidate" phrasing is obsolete
    since v0.3.0 shipped 2026-09-19.

## d) TOTALLY FUCKED UP (own mistakes, in order)

1. **Ran the full gate sweep mid-edit** — background `.#gates` started while the Healthz test
   had a `wsl_v5` violation; the run burned ~10 min and died at lint, proving nothing about the
   final tree. Should have finished the edit first or gated the sweep on a clean `git status`.
2. **flake.nix placement error** — defined `openapiLockstep` as an attribute of the `in { … }`
   set (referencing it as a variable then fails: "undefined variable"); also wrote a nonsense
   `config.openapiLockstep or openapiLockstep` fallback before simplifying. Two wasted nix
   builds.
3. **jq scoping bug shipped into first verification run** — `keys[] | PROPS | has(.)` evaluates
   `.` as the properties object; the "positive" case failed on first run. Caught immediately
   because I tested both directions.
4. **`rg -rln` misuse, twice** — `-r` is `--replace`; mangled output badly enough that I
   misread erraudit's directive syntax for a moment (`//ln:` garbage). Cost: extra search rounds.
5. **`curl` attempt** — banned tool, instant error; should have gone to `fetch` first.
6. **Bare `go test` failed on toolchain mismatch** — host go1.26.7 vs go.mod 1.27.1. Root cause
   is stale AGENTS.md (says 1.26 flake pin), but I should sanity-check `go.mod`/toolchain at
   session start in Go repos.
7. **Harvest inconsistency** — first marked a resolved row `DONE` although this file's lifecycle
   is delete-with-note; self-corrected next edit.
8. **edit-before-read failures** — three tool errors (CHANGELOG/FEATURES/AGENTS.md edit attempts
   without a fresh view; TODO_LIST whitespace mismatch after daemon reformat). Each cost a
   round trip.
9. **Did not confirm the dispatched fuzz run finished** — dispatched, saw `in_progress`, moved
   on; ~15 min runtime means it finished by now, but unverified.

## e) WHAT WE SHOULD IMPROVE

- **Session-start environment check** for Go repos: `go.mod` directive vs host `go version`,
  flake `goPkg` — one command, prevents the whole class of toolchain surprises this session hit.
- **Finish-then-verify**: never start a multi-minute verification sweep while edits are pending;
  run gates exactly once on the final tree.
- **AGENTS.md freshness contract**: two sections (header status line, GOEXPERIMENT gotcha) went
  stale after the 1.27 bump + v0.3.0 release despite the aggressive-update protocol — the release
  session that bumped go.mod didn't sweep the doc. A post-release AGENTS.md grep for the old
  version string would have caught it (`rg -n "1\.26|v0\.2\.0" AGENTS.md`).
- **Cross-repo TODO rows need repo pointers at row level**: dashboard/auditlog rows live in this
  repo's list but mean nothing without `cd` targets; several rows say "Own repo" in prose only.
- **Benchmark baselines need machine pinning**: 09-04/09-18 rows vs today's run may differ in
  hardware; the FEATURES note records go version + threads but not CPU model. Record CPU model
  going forward.

## f) NEXT (ranked, ~30 items)

1. Update FEATURES.md "Performance": add `BenchmarkEvaluate_TrackerDelta` row (892→1149 ns,
   +257 ns/+29%, +528 B, +2 allocs), re-baseline the Evaluate/TrackerStamp rows to go1.27.1
   numbers, add CPU model to the baseline note, flag hardware-comparability caveat.
2. Re-run the full `nix run .#gates` sweep on the final tree (test-race, vet, lint, vulncheck,
   security, fuzz, flake check) — one clean pass.
3. Root package `-race` run after the tracker seam (gates covers it, but confirm explicitly).
4. CHANGELOG `[Unreleased]`: entry for the tracker A/B seam + delta benchmark.
5. Harvest the `BenchmarkEvaluate` TODO row once 1–2 land.
6. Fix AGENTS.md header: Go 1.27, status v0.3.0 (released 2026-09-19), federation released.
7. Fix AGENTS.md GOEXPERIMENT gotcha: flake now `go_1_27`; json/v2 stable under 1.27 (verify
   whether `GOEXPERIMENT=jsonv2` export is still needed at all in the flake and simplify if not).
8. Verify fuzz run 35756511889 completed green; then delete/annotate nothing further (row already
   harvested).
9. Release-vehicle decision for `[Unreleased]` (Healthz, lockstep check, benchmark seam) — owner;
   likely v0.4.0 given additive API.
10. Federation `Prober.Healthz()` parity (design note first, mirror aggregate's decision).
11. openapi.yaml: bump `info.version`, add one sentence documenting the mountable single-endpoint
    `Healthz()` (root + aggregate).
12. Prose review of `middleware_example_test.go` / `prometheus_example_test.go` (open TODO row).
13. ADR: latency-unit unification (`total_latency_ms` vs `duration_ns`) — open TODO row.
14. Detailed-checks cookbook (open TODO row).
15. samber/do upstream feature request: per-service timing in batch results — verify current do
    source first (verify-before-filing), draft in Lars's voice, ask before filing.
16. samber-do-auditlog: implement `DetailedHealthRecorder` (sibling repo session).
17. Dashboard: render "failing since HH:MM (Nm)" (High, sibling repo).
18. Dashboard: status-changes timeline from `since` (High).
19. Dashboard: adaptive `duration_ns` rendering.
20. Dashboard: "stable for Xh" collapse summaries.
21. Dashboard: integration test pinning `since`/`duration_ns` (after adoption).
22. Branch protection on master (owner decision G3; ready-to-run command in TODO_LIST).
23. Coverage-threshold CI job (owner policy call).
24. Publish v0.1.1/v0.1.2 announcement (owner; draft ready).
25. `Aggregate.SourceStatuses()` / per-source visibility (deferred beyond v0.3.0 per CHANGELOG).
26. `errors.Join` aggregate construction errors (deferred item, verified spike exists).
27. ROADMAP Theme 7 mark: aggregate/federation single-endpoint question — update once federation
    Healthz is decided.
28. Add CPU model + commit hash to FEATURES baseline note format (tiny, do with item 1).
29. Consider `nix run .#openapi-lockstep` in BuildFlow? No — project-specific check, correctly
    flake-owned (documented decision; revisit only if more LarsArtmann repos adopt OpenAPI
    lockstep).
30. Post-release AGENTS.md staleness grep as a habit: `rg -n "1\.26|v0\.2\.0|unreleased" AGENTS.md`
    after every release session.

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **Release vehicle**: shall `Aggregate.Healthz()` + the lockstep check + benchmark seam ship as
   **v0.4.0** now, or stay in `[Unreleased]` until `SourceStatuses()` / `errors.Join` batch up?
   (Affects whether I run the go-release flow this week.)
2. **samber/do upstream**: file the per-service-timing feature request on samber/do now (after
   source verification, drafted in your voice), or draft-only for your review? Filing externally
   under your name is yours to trigger.
3. **Dashboard scope**: pull `go-health-dashboard` into the next session(s) for the two High
   rows (`since` rendering, status-change timeline), or keep sessions go-health-local until the
   v0.4.0 metadata fields are actually tagged?

---

_Point-in-time snapshot; open work lives in TODO_LIST.md, completed work in CHANGELOG.md._
