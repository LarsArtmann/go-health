# Status Report — Issue #2: Per-Check Since/Duration Metadata

> 2026-09-15 06:56 CEST · Session scope: implementation of
> [go-health#2](https://github.com/LarsArtmann/go-health/issues/2) only
> (design → code → tests → docs → gates → consumer verification).
> Auto-commits `eb5f990..e38e248` captured the work. Format: Markdown per
> explicit user request (house default for status reports is HTML).

---

## a) FULLY DONE

| Item                                                                                                                                                                                                                                                                                                                                                                                                                | Evidence                                                |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| **Research**: issue, dashboard expectations (draft doc + repo), samber/do v2.1.0 API limits (no per-check timing through `map[string]error` batches), jsonv2 omission semantics                                                                                                                                                                                                                                     | session; `/tmp` probes since trashed                    |
| **Design doc** `docs/check-metadata-design.md` — Since semantics (probe-observed), DurationNanos (executor-reported, opt-in), wire rationale, rejected alternatives                                                                                                                                                                                                                                                 | committed                                               |
| **Data model**: `Check.Since time.Time` (`since,omitzero`), `Check.DurationNanos int64` (`duration_ns,omitzero`), `CheckDetail{Err, Duration}`                                                                                                                                                                                                                                                                      | `types.go`                                              |
| **Transition tracker** (`tracker.go`): mutex-guarded, stamped in `buildChecks` on every evaluation path, prunes absent checks, zero-value usable                                                                                                                                                                                                                                                                    | `tracker.go`                                            |
| **Internal seam rework**: `healthCheckFunc` returns `map[string]CheckDetail`; `resolveHealthCheck` type-switch (detailed recorder → plain recorder → injector); `adaptPlainChecks`/`detailOf`/`errorsOf` adapters; panic recovery adapted                                                                                                                                                                           | `probe.go`, `handlers.go`                               |
| **New public API**: `NewWithDetailedCheck` + `DetailedHealthCheckFunc` (`accessors.go`), optional `DetailedHealthRecorder` interface (`probe.go`) — all additive, classification stays with the probe                                                                                                                                                                                                               | committed                                               |
| **Tests** (14 new): first-observation stamp, carry-forward, transition both directions, error-text-change ≠ status change, prune/reappear restart, startup-path participation, concurrency race test, liveness/Healthz zero-Since, detailed-func durations + grading, injector-path zero duration, detailed + plain recorder paths, wire omitzero lock, full handler-path payload lock, aggregate merge passthrough | `probe_metadata_test.go`, `aggregate/aggregate_test.go` |
| **Golden files**: root golden byte-unchanged (back-compat proven); aggregate golden regenerated deterministically (fixed clock) to lock merged `since`                                                                                                                                                                                                                                                              | `testdata/`, `aggregate/testdata/`                      |
| **Godoc example** `ExampleNewWithDetailedCheck`                                                                                                                                                                                                                                                                                                                                                                     | `example_test.go`                                       |
| **Docs sweep**: README (samples + feature bullets), doc.go section, CHANGELOG `[Unreleased]`, FEATURES.md rows, DOMAIN_LANGUAGE entries (5), openapi.yaml Check schema (`since`, `duration_ns`), AGENTS.md (file list, 2 design decisions, concurrency model, new gotcha, doc table)                                                                                                                                | committed                                               |
| **Gates** (subset, individually): test, test-race, lint (0 issues), vet, fmt, `nix flake check`, fuzz (short budget)                                                                                                                                                                                                                                                                                                | all green                                               |
| **Key discovery documented**: jsonv2 **cannot marshal `time.Duration` at all** (go.dev/issue/71631; no tag format; only per-call `FormatDurationAsNano`) → plain int64 ns on the wire; pinned by `TestCheck_JSONOmitZero` + AGENTS.md gotcha                                                                                                                                                                        | session                                                 |
| **Real consumer verification**: go-health-dashboard cloned, built, and **full test suite passed** against local go-health via replace directive                                                                                                                                                                                                                                                                     | /tmp clone since trashed                                |

## b) PARTIALLY DONE

1. **Gate sweep**: ran the gates individually, but never the one-command
   `nix run .#gates` meta-sweep; gosec/govulncheck not run this session
   (no new deps, no crypto — low risk, but the project's own pre-push bar
   was not executed end-to-end).
2. **Fuzz**: `nix run .#fuzz` ran; visible tail confirmed the aggregate
   target PASS — the root package's fuzz result scrolled out of the tail
   filter (earlier plain `go test` runs pass the seed corpus). Verifiable in
   one command, not re-verified before reporting "fuzz pass".
3. **Performance story**: I claim tracker/adapter overhead is "negligible"
   but never measured it. The evaluate path now allocates 2 extra maps per
   batch (`errorsOf` view + tracker rebuild); no benchmark added or re-run;
   FEATURES.md performance table predates the change.

## c) NOT STARTED

1. **Issue #2 reply/close** — no comment posted with the design answer
   (omitzero + `duration_ns` rationale). Deliberately not posted without
   owner instruction.
2. **Dashboard adoption** — the entire point of the issue. The dashboard
   still renders placeholders; nothing consumes `since`/`duration_ns` yet.
   Verified compatible, not yet adapted.
3. **`samber-do-auditlog`** — does not implement `DetailedHealthRecorder`
   yet, even though it likely times checks internally for its audit log;
   the optional interface ships with zero real implementors.
4. **TODO_LIST.md harvest** — follow-ups from this session (below) are not
   routed into TODO_LIST/ROADMAP yet.
5. **Upstream ask to samber/do** — richer batch results (per-check timing)
   would let the injector path populate `duration_ns`; no issue filed.

## d) TOTALLY FUCKED UP!

Nothing in the shipped state is broken (suite, race, lint, vet, golden
locks, consumer build all green). The honest failures were all **caught and
fixed in-session** — listed here because they were real defects I authored:

1. **Designed against an unverified encoder assumption.** The design doc
   originally proposed `Duration time.Duration json:"duration,omitzero"`.
   jsonv2 refuses to marshal `time.Duration` entirely — discoverable in a
   5-minute probe program before freezing the design; I discovered it via a
   failing test mid-implementation and pivoted to `duration_ns int64`
   (design doc corrected twice: once for a `time.Duration`/`time.Time`
   typo, once for the 71631 pivot). A detailed-check probe serving
   durations would have 500'd every request if the wire test hadn't caught
   it — exactly the class `writeResponse`'s defensive comment warns about.
2. **Wrote a data race into a race-detector test** (`failing = !failing`
   from multiple goroutines). Caught by reflection before running; fixed
   with `atomic.Bool`.
3. **Clock-arithmetic bug in a test expectation** (`recoveredAt` off by the
   wrong advance), plus a dead `_ = reason` placeholder left in a first
   draft. Both caught by the tests themselves.
4. **Compile error from a foreseeable method-value signature mismatch**
   (`adaptPlainChecks(r.RecordHealthCheckWithContext)` keeps both params).
5. **Three lint round-trips** chasing `wsl_v5` whitespace findings that a
   single post-write `nix run .#lint` would have listed at once; I leaned
   on stale LSP diagnostics instead.

## e) WHAT WE SHOULD IMPROVE!

Self-review (the 11 questions, condensed):

1. **What did you forget?** Benchmarks for the new evaluate-path cost;
   TODO_LIST harvest; `.#gates` as one command; issue-thread reply;
   dashboard-side adoption is the actual value delivery and it hasn't
   started.
2. **Something stupid we do anyway?** Two latency units now coexist
   (`total_latency_ms` ms vs `duration_ns` ns) — documented rationale, but
   a real wart consumers must handle; only a v0.2 breaking change can
   unify it. Also: my `Example` asserts `DurationNanos > 0` from real
   `time.Since` — safe in practice, but it is a wall-clock assertion in an
   otherwise deterministic example.
3. **What could you have done better?** Empirically probe encoder behavior
   BEFORE writing the wire design (the 71631 pivot should have been a
   pre-design fact, not a mid-implementation surprise); write wsl-clean
   test files or lint immediately after writing; foresee bound-method
   signatures.
4. **What can you still improve?** Measure, don't vibe: add an
   `Evaluate`/`buildChecks` benchmark and re-baseline FEATURES.md.
5. **Did you lie to you?** No statement in the final summary was false, but
   "fuzz pass" rested on a tail-filtered log (root target not visibly
   confirmed) — re-verify before relying on it.
6. **How to be less stupid?** For any wire-format decision: write the
   10-line marshal probe first. The project's own golden/omitzero tests
   exist precisely to catch this; use them at design time, not just at
   test time.
7. **Ghost systems?** None created — tracker, DetailedHealthRecorder,
   NewWithDetailedCheck, CheckDetail are all wired, tested, documented, and
   exampled. (The one zero-implementor interface is deliberate forward
   seam for auditlog; flagged in c).)
8. **Scope creep?** One moment: `DetailedHealthRecorder` ships with no
   implementor. Defensible (additive, tested, one type-switch branch), but
   it is the closest thing to speculative API in this change.
9. **Removed something useful?** No. All changes additive; golden files
   prove the old wire survives.
10. **Split brains?** One, acknowledged: dual latency units (ms response
    scalar vs ns per-check). Also the LSP ran stale "unused tracker"
    warnings all session (CLI lint: 0 issues) — I never `lsp_restart`ed;
    environment hygiene to improve.
11. **Tests?** Strong on behavior (14 new, incl. race + wire + golden +
    consumer suite), weak on performance (no benchmark delta) and fuzz
    seeds (no populated-Since/DurationNanos seed in the marshal fuzz
    corpus).

## f) Up to 50 things to do next (session-scoped, ranked)

1. Reply to issue #2 with the design answer + link the design doc (owner
   decides close-vs-await-release).
2. Dashboard: render "failing since HH:MM (Nm)" column from `check.since`.
3. Dashboard: derive the status-changes timeline from `since` instead of
   the sampling clock; kill the placeholder columns.
4. Dashboard: render `duration_ns` (format µs/ms adaptively); hide when
   absent.
5. Dashboard: "stable for Xh" collapse summaries for healthy groups.
6. Decide the release vehicle: v0.1.4 now (CHANGELOG `[Unreleased]` is
   meaty) or batch; then tag + proxy-verify per go-release skill.
7. Run the full `nix run .#gates` once (incl. gosec/govulncheck) as the
   pre-release check.
8. Add `BenchmarkEvaluate` (and/or buildChecks) before/after tracker;
   record the delta in FEATURES.md performance table.
9. Re-baseline the existing FEATURES.md benchmark rows on current code.
10. Add a populated `Since`/`DurationNanos` case to
    `FuzzResponseMarshalDeterministic` seeds (anchor the new fields).
11. File the samber/do upstream issue: richer health-check batch results
    (per-service timing) so the injector path can populate `duration_ns`.
12. samber-do-auditlog: implement `DetailedHealthRecorder` (it already
    times checks internally for the audit log) — first real implementor.
13. HARVEST this report's items 1–5/7–25 into TODO_LIST.md / ROADMAP.md
    (docs-health HARVEST).
14. Consider a `TestCheck_JSONOmitZero` companion asserting
    `duration_ns:0` is absent _inside a full response_ through
    `writeResponse` (plain injector path e2e).
15. Startup handler godoc: mention checks carry `since` (one line).
16. Fix the openapi.yaml `info.version` (still 0.1.0 — pre-existing drift).
17. Make the `ExampleNewWithDetailedCheck` output fully deterministic
    (assert non-negative or inject duration semantics).
18. `lsp_restart` hygiene: stale "unused" diagnostics persisted all
    session; note in global memory to distrust LSP when CLI lint
    disagrees.
19. Record the "probe the encoder before designing the wire" lesson in
    global AGENTS.md cross-cutting lessons.
20. Consider tracker allocation micro-optimization ONLY IF the benchmark
    (item 8) shows it matters (reuse buffer / COW map swap).
21. Evaluate-path: consider fusing `errorsOf` into classifier (classifier
    over CheckDetail) to drop one per-batch map — again, only if measured.
22. Document (README or cookbook) a "detailed checks cookbook": how a
    service self-times and composes via `NewWithDetailedCheck`.
23. Dashboard cookbook entry for the new fields (docs/integrations.md
    upstream).
24. Once dashboard adopts: add an integration test in the dashboard repo
    pinning `since`/`duration_ns` rendering (placeholder regression).
25. Deprecation decision (v0.2 planning): unify latency units — either
    `total_latency_ns` or document the split forever; ADR-style doc if
    deferred.
26. Add `tracker.go` to the doanalyzerv2 analyzer sweep inputs (already in
    package — verify 0 findings claim still holds for new file).
27. Check `middleware_example_test.go`/`prometheus_example_test.go` docs
    for stale wire examples (they pass; just prose review).
28. Next session start: re-run `nix run .#fuzz` and read the FULL output
    (close the b-2 gap from this report).

## g) Questions I cannot answer myself

1. **Release**: is this the v0.1.4 release vehicle (tag now), or hold for
   more items — and should I post the design answer on issue #2 in your
   voice (github-voice) for you to approve?
2. **Dashboard adoption**: want me to implement the `since`/`duration_ns`
   rendering in `go-health-dashboard` next (new session, its own repo), or
   do you own that side?
3. **Units**: keep dual latency units forever (`total_latency_ms` +
   `duration_ns`), or plan a v0.2 unification (breaking) while the library
   is still v0.x-alpha?

---

_Point-in-time snapshot; annotate, never rewrite (docs-health ANNOTATE for
updates). Report written by the session that did the work; no unrelated
project audit was performed._
