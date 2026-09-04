# Status: v0.1.1 Release Session — Deprecation, Docs Sync, First Green CI

> 2026-09-04 13:17 CEST · Point-in-time snapshot of the session that closed out
> the 2026-09-04 Pareto master plan (P35–P39 + P23/P24 + final sweep) and
> shipped release v0.1.1. Predecessor:
> [11-13 marathon report](2026-09-04_11-13_pareto-execution-marathon-and-api-batch.md).

## TL;DR

The 39-task plan is now **fully executed**. This session: fixed the inherited
broken build, finished P35–P39, ran the full gate sweep, synced all living
docs, marked `WithGETOnly` deprecated on request, and — after the user answered
the three open questions — pushed `master`, went green on CI (after fixing a
real CI-only toolchain bug the first run caught), tagged **v0.1.1**, pushed the
tag, verified the module proxy with a real consumer, and published the GitHub
release. Four stale-doc spots were discovered while writing this report and are
logged below as forgotten items, not yet fixed.

---

## a) FULLY DONE

### P35 — small options batch (closed the inherited broken state)

- **Build fix**: `sort` + `strings` imports restored in `probe.go`
  (inherited red state from commit `1fbed6e`); verified `WithNowFunc` /
  `WithAllowedMethods` each defined exactly once; lint `varnamelen` finding
  fixed (`c` → `cfg`).
- **Clock-seam completion (code fix beyond the plan)**: `Evaluate` stamped
  `resp.Timestamp = time.Now()` and the live-throttle freshness check used
  `time.Since(...)` — both bypassed the `WithNowFunc` seam. Now `p.now()`
  drives uptime, timestamps, *and* throttle freshness; latency measurement
  deliberately stays on the real clock.
- **B100**: 3 determinism tests (`TestWithNowFunc_UptimeIsDeterministic`,
  `_UptimeTracksClockAdvance`, `_DefaultClockUsedWhenUnset`).
- **B101**: 4 method-set guard tests (GET always allowed; HEAD listed → 200;
  DELETE → 405 with sorted `Allow: GET, HEAD, OPTIONS`; duplicate collapse
  across two `WithAllowedMethods` calls; empty-call ≡ `WithGETOnly`).
- **B102**: `export_test.go` with `ResetStartupLatchForTest` (test builds
  only) + `TestStartupLatch_ResetForTest_ReEvaluatesAndRelatches` pinning
  latch → reset → 503 → re-latch. Public latch stays one-way.
- **B103**: [docs/starting-status-design.md](../../docs/starting-status-design.md)
  — "starting" Status **rejected** (kubelet never reads the body; two strings
  for one machine state; latch regression would lie against `uptime`);
  Status input validation **none by design** (no injection boundary exists).

### P36 — format batch

- **`WithInstanceID` + `Response.InstanceID`** (`instance_id`, omitempty):
  populated in all four response-build sites (liveness, startup-latched,
  `buildStartupResponse`, `Evaluate`); 2 new tests; golden file regenerated
  (`instance_id` after `version` — struct-order marshal, deterministic).
- **Prometheus spike (verified)**:
  `prometheus_example_test.go` — ~40-line stdlib exposition writer
  (`health_up` + `health_check` gauges, `instance`/`service` labels) +
  `ExampleWithEvaluationHook_metrics` passing;
  [docs/prometheus-exposition-design.md](../../docs/prometheus-exposition-design.md)
  — decision: composition via `WithEvaluationHook`, never `client_golang`.
- **OpenAPI**: [docs/openapi-design.md](../../docs/openapi-design.md)
  (static spec over runtime generation, rationale) +
  [docs/openapi.yaml](../../docs/openapi.yaml) — OpenAPI 3.1 for all three
  probes with status-code semantics and the field-presence rules that OpenAPI
  cannot express.

### P37 / P38 / P39 — design notes with verified evidence

- [docs/classification-2.0-design.md](../../docs/classification-2.0-design.md):
  weights (rejected — binary at every consumer boundary), circuit-breaker
  (rejected in core — dependency-wrapper seam), `WithMaxConcurrentChecks`
  (rejected — do owns the batch; recorder is the escape hatch), per-service
  caching + per-service latency (blocked by do owning the batch — the P28
  finding folded in).
- [docs/multi-tenant-design.md](../../docs/multi-tenant-design.md):
  child-scope (N probes + aggregate already solve it, verified against
  `aggregate.go`), `WithProbeName` (redundant — aggregate namespacing +
  InstanceID), `WithCriticalService` toggle (rejected — classifier
  immutability is the lock-free feature).
- **Middleware spike (verified)**:
  `middleware_example_test.go` (`ExampleProbe_ReadinessHandler_middleware`:
  200/401/200) +
  [docs/middleware-design.md](../../docs/middleware-design.md) — no library
  concept needed; handlers are plain `http.HandlerFunc`; liveness stays
  unwrapped (kubelet can't send credentials).

### P23-final / P24 — curation + read-through

- **Agent sweep of all 7 annotated reports**: verdict — every
  TODO_LIST-pointing item from history was completed in the marathon;
  remaining opens are user decisions + cosmetics. No missed work.
- **TODO_LIST rewritten**: 3 BLOCKED rows (push / release version / name
  sign-off — all since resolved), dashboard re-verify row, 2 Low rows,
  and a "Resolved this cycle" traceability section. Verified the `go.mod`
  toolchain row was **moot** (`go mod tidy` drops a `toolchain` line equal to
  the `go` directive — added it, tidy removed it, documented).

### Docs sync (living documents)

- **ROADMAP.md**: pruned to genuinely-remaining ideas; shipped items marked;
  Non-goals extended with links to all design notes.
- **FEATURES.md**: full rewrite — 13 options, programmatic API rows, method
  -set guard, clock seam, `SanitizeResponse`, aggregate, fuzz/stress/CI/erraudit
  /doanalyzerv2/ADR rows; stale "PLANNED" rows corrected to shipped.
- **AGENTS.md**: source-file map (6 files + export_test), method-set + clock
  -seam decisions, hook/composition + programmatic-API decisions, concurrency
  model (WaitGroup serialization, `throttleMu`, read-only classifier).
- **DOMAIN_LANGUAGE.md**: 9 new terms (combined endpoint, programmatic
  accessors, live throttle, evaluation hook, clock seam, method-set guard,
  instance id, self-registration, classifier).
- **README.md**: Key Features (+3), Configuration Reference (+6 options),
  new "Programmatic Health API" section, TOC updated.
- **CHANGELOG [Unreleased]**: the entire API batch (it post-dates the v0.1.0
  tag) — Added ×6 groups, Deprecated, and the UTF-8 Fixed entry.

### Full gate sweep (all green)

build · test · test-race · lint 0 issues · vet · fmt · **coverage 98.5%** ·
govulncheck 0 · gosec 0 · fuzz PASS (1.59M execs) · `nix flake check` ·
erraudit `--type-aware` 0 violations · `nolint-audit` 2 needed / 0 stale ·
doanalyzerv2 **7 files, 0 findings** · dashboard consumer compile + tests
green against HEAD via replace.

### Release v0.1.1 (per go-release skill, after user sign-off)

- `WithGETOnly` **deprecated** (godoc `// Deprecated:` + README + FEATURES +
  AGENTS + CHANGELOG `### Deprecated`); still functional, no removal planned
  in v0.x. API names approved as-is (incl. `ResetStartupLatchForTest`).
- CHANGELOG cut to `[0.1.1] - 2026-09-04`; go.mod hygiene verified (no
  replace, no pseudo-versions, `go mod verify` clean).
- **First CI run FAILED** → root-caused and fixed (see d/e below) → **second
  run all 4 jobs green**, and the tag run green too.
- Annotated tag `v0.1.1` pushed; proxy verified with a real consumer
  program (`go get` resolves v0.1.1; imports `NewWithHealthCheck` — a symbol
  that only exists in this release — and prints `pass`);
  [GitHub release published](https://github.com/LarsArtmann/go-health/releases/tag/v0.1.1)
  (not prerelease, matching v0.1.0 convention).
- TODO_LIST release rows closed; AGENTS.md gotcha added; commits pushed.

---

## b) PARTIALLY DONE

1. ~~pkg.go.dev verification for v0.1.1~~ — still propagating at the 2026-09-04 docs-health run (proxy resolves it; page 404); tracked in TODO_LIST. The v0.1.1 godoc (incl. the new
   `// Deprecated:` marker on `WithGETOnly`) is not yet human-visible there.
2. **Deprecation completeness in-repo** — 4 tests still call the now-deprecated
   `WithGETOnly` (intentional: they pin the deprecated path), but there is no
   explicit policy note in the tests or `.golangci.yml` decision recorded for
   SA1019-style flags in other repos consuming ours.
3. ~~**Docs tables lag the new design notes** — the AGENTS.md "Project
   Documentation" table and the README "Project Docs" section predate the six
   new notes (see a-forgot list below).~~ done at `e366fcc` — both tables list all design notes + openapi.yaml now.
4. **CHANGELOG section order** — `[0.1.1]` orders Deprecated before Added;
   Keep-a-Changelog canon is Added, Changed, Deprecated, Removed, Fixed.
   Cosmetic.
5. **doanalyzerv2 runner** — reconstructed ad-hoc in `/tmp` (worked: 7 files,
   0 findings) but thrown away again. Still no persisted, repeatable
   invocation; this is the second session in a row that had to rebuild it.
6. ~~**Master-plan closure** — `docs/planning/2026-09-04_00-02_pareto-master-
   execution-plan.md` has no "COMPLETED" header stamp; the 11-13 report has
   no completion addendum pointing here.~~ done — 2026-09-04 docs-health run (plan stamped COMPLETED; 11-13 report got a Completion section and inline f-list verdicts).

## c) NOT STARTED

1. Branch protection on `master` (require the 4 CI checks + linear history —
   the workflow header recommends it; needs admin/owner action).
2. Examples: custom `HealthRecorder`, two-phase shutdown, live-vs-cached
   (TODO_LIST Low — patterns exist only in tests/docs).
3. ~~Hash-citation top-up for the two 2026-08-07 reports (TODO_LIST Low).~~ done — 2026-09-04 docs-health run: all shipped/routed items in `09-12`, `19-01`, `19-11` now carry hashes or explicit Won't-implement verdicts.
4. ~~Go Report Card / goreportcard re-scan for v0.1.1 (badge exists; grade not re-verified this session).~~ **Won't implement — the service has been sunset; badge removed** `e366fcc`.
5. Release-automation evaluation (manual tag→release flow worked twice;
   GoReleaser/actions automation never assessed).
6. SECURITY.md (vuln disclosure path for a public module) — absent.
7. aggregate-package fuzz targets (fuzzing covers the root package only).
8. ~~Dependency freshness sweep: is `samber/do v2.1.0` still the newest 2.x?~~ **Won't implement as a manual task — dependabot (weekly gomod) now owns this.**

## d) TOTALLY FUCKED UP (and what each cost)

1. **Pushed to CI before emulating CI.** The go-release skill explicitly says
   CI catches environment-dependent failures local runs skip, and the exact
   same leak class (host PATH/env making gates pass) was *already documented
   in AGENTS.md* from the GOEXPERIMENT incident. The `lint`, `vulncheck`, and
   `security` flake apps didn't include `goPkg`; on CI the tools fell back to
   an older bundled GOROOT and hard-failed on `GOEXPERIMENT=jsonv2`. Cost:
   one red first CI run on a fresh repo (permanent in the run history), one
   debug cycle, one extra push. The sanitized-`env -i` verification I ran
   *afterwards* took 30 seconds and would have caught it before the push.
2. **`go mod tidy` vs `toolchain` directive fumble.** Added
   `toolchain go1.26.7`, immediately broke the build with "updates to go.mod
   needed", needed two extra round trips to discover tidy *removes* a
   toolchain equal to the `go` line. Known semantics; should have been
   checked before editing.
3. **Example-naming compile errors ×2.**
   `ExampleRegisterRoutes_middleware` → "refers to unknown identifier" →
   `ExampleReadinessHandler_middleware` → same error → finally
   `ExampleProbe_ReadinessHandler_middleware`. Go's example-naming rule
   (must reference a real identifier; methods need the type prefix) is
   textbook — two wasted compile cycles.
4. **Root-package confusion ×2.** Ran `go test ./health/` ("directory not
   found") and wrote the import as `.../go-health/health` before noticing
   the root package *is* `health` (module path ≠ package directory). Should
   have read the layout once instead of assuming.
5. **doanalyzerv2 runner heredoc fumbling.** ~4 bash round trips fighting
   shell escaping inside `python3 - <<EOF` string replacements before
   switching to the `write` tool, which solved it immediately. Wrong tool
   for in-place edits, repeatedly.
6. **Unresolved imports in fresh files.** Three newly written test files
   initially missed imports (`httptest`, `json/v2`+`strings`, `fmt`) — each
   cost a compile cycle. Pattern: writing code before resolving its imports.
7. **No-op multiedit + failed multiedit.** One multiedit contained an edit
   with `old_string == new_string` (sloppy), another was rejected with "only
   the first edit can have empty old_string" and had to be re-issued as two
   separate `edit` calls.
8. **Stale-doc blind spot.** The docs sync updated six living documents but
   missed four spots that reference version/API surface (README stability
   line, README/AGENTS doc tables, `doc.go`) — caught only while writing
   this report, i.e. by introspection rather than by the sweep itself. The
   sweep had no checklist of "every place that names a version or lists the
   API", which is exactly what a sync needs.

## e) WHAT WE SHOULD IMPROVE

1. **Pre-push CI emulation is mandatory for release pushes.** Add a step to
   the release checklist: `env -i HOME=... PATH=<nix-only> nix run .#<gate>`
   for every gate the workflow runs. All local gates passed while CI failed —
   "green locally" is not evidence until PATH is sanitized.
2. **Persist the doanalyzerv2 runner** (small script under `tools/` or a
   documented one-liner in CONTRIBUTING) — two sessions have now paid the
   rebuild cost.
3. **Version/API-sync checklist for docs**: README stability line, README
   badges, README Project Docs, AGENTS.md Project Documentation table, doc.go
   package comment, CHANGELOG links. Any release or API batch runs this list.
4. **Write files with resolved imports** — assemble the import block from
   the used symbols before writing, not after the compiler complains.
5. **Use `write` for generated scratch code, never heredoc+sed chains.**
6. **Kill the inherited `getOnly bool` internal duplication** — the guard
   reads two flags; internally `WithGETOnly` could seed `allowedMethods` and
   delete the second code path (behavior-preserving refactor, simplifies
   `guard`).
7. **Add a fake-clock throttle test** — the clock-seam fix to
   `throttledLiveResponse` has no dedicated determinism test (B100 covered
   uptime/timestamp only).
8. **CHANGELOG ordering convention** — adopt canonical Keep-a-Changelog
   category order so future cuts are mechanical.
9. **Coverage gap visibility** — 98.5% is recorded but the uncovered 1.5%
   (which functions) was never enumerated; know *what* is uncovered, not just
   how much.
10. **Reconcile editor LSP diagnostics with the CLI linter** — gopls/LSP
    showed wsl_v5/stdversion warnings all session while `nix run .#lint` was
    clean; aligning configs removes constant low-level doubt during edits.

## f) NEXT — up to 50, priority-ordered

**Now (release hygiene, ≤15 min each)**

1. ~~Verify pkg.go.dev renders v0.1.1 (incl. the `Deprecated` section) once
   propagation completes.~~ re-checked in the 2026-09-04 docs-health run: still
   propagating (page 404, proxy resolves it) — stays on TODO_LIST until rendered.
2. ~~Update README stability line "v0.1.0 alpha" → "v0.1.1".~~ done at `1d0e1ea`
3. ~~Add the 6 new design notes + openapi.yaml to README "Project Docs".~~ done at `e366fcc`
4. ~~Add the 6 new design notes to the AGENTS.md Project Documentation table.~~ done at `e366fcc`
5. ~~Refresh `doc.go` package comment for the programmatic API + new options
   (currently zero mentions of Healthz/AwaitReady/hook/throttle/InstanceID).~~ done at `1d0e1ea`
6. ~~Reorder `[0.1.1]` CHANGELOG categories to Keep-a-Changelog canon.~~ done at `e366fcc`
7. ~~Stamp the master-plan file COMPLETED; add completion addendum to the 11-13
   report.~~ done — 2026-09-04 docs-health run
8. ~~Confirm Go Report Card re-scanned post-release.~~ **Won't implement — goreportcard.com has been sunset; badge removed from the README** `e366fcc`

**CI / repo hardening**

9. Configure branch protection: require the 4 CI checks + linear history
   (needs owner).
10. Persist a repeatable doanalyzerv2 runner in-repo (tools/ + CONTRIBUTING).
11. Add a sanitized-PATH "CI emulation" note to CONTRIBUTING's release
    checklist.
12. Decide SA1019 policy for in-repo use of deprecated symbols (pin-tests vs
    nolint), record in `.golangci.yml` or AGENTS.
13. Add SECURITY.md (disclosure contact for the public module).
14. Verify dependabot covers both `gomod` and `github-actions` ecosystems.
15. Add PR template (Dependabot PRs benefit; issue templates exist).
16. Optional: coverage-threshold job (fail < 97%) — policy decision.
17. Optional: dependabot/renovate auto-merge rules for patch bumps.

**Code / tests**

18. Fake-clock determinism test for `WithLiveThrottle` (pin the seam fix).
19. Enumerate the uncovered 1.5% coverage functions; cover or document each.
20. Refactor `guard`/config: collapse `getOnly bool` into the method set.
21. Fuzz targets for the `aggregate` package (merge-on-read invariants).
22. Benchmark: method-set guard overhead (allowed vs unlisted vs no-guard).
23. Verify + document aggregate's merge rule for `InstanceID` (per-replica
    scalar — presumably dropped like Version/Uptime; confirm and write it
    into the aggregate doc comment + openapi note).
24. Examples: custom `HealthRecorder`; two-phase shutdown;
    live-vs-cached (promote from tests to `example_test.go`).
25. Fuzz seed corpus: include `instance_id`-populated responses.
26. Add godoc example for `AwaitReady` (blocking-helper discoverability).
27. Consider `TotalLatencyMs float64` upgrade (deferred by design — reopen
    only with a consumer need; keep on roadmap).
28. Compatibility matrix in README (tested Go versions × samber/do versions).
29. Deprecation policy doc: how long deprecated symbols live; what v1.0
    promises (WithGETOnly removal timeline lives here).
30. Link `docs/openapi.yaml` from README; optionally validate it in CI
    (spectral/redocly) so it can't drift from the golden file silently.

**Consumers / ecosystem**

31. Bump `go-health-dashboard` to v0.1.1 (drop its replace; real release
    upgrade in that repo).
32. Re-verify `samber-do-auditlog` stays dependency-free and compatible.
33. Check samber/do for 2.x releases newer than v2.1.0; sweep if so.
34. Track Go 1.26.x patch releases in the flake input (dependabot may cover).

**Docs / polish**

35. Top up `done at <hash>` citations in the two 2026-08-07 reports.
36. Add a short "Middleware" section to README (link the design note +
    example).
37. Add "Metrics" section to README (hook + Prometheus pattern).
38. Add troubleshooting entry: "405 with Allow header" (method-set guard).
39. Troubleshooting entry: "pkg.go.dev shows old docs" (propagation lag).
40. DOMAIN_LANGUAGE: add "deprecation" convention entry.
41. Consider retiring `TotalLatencyMs`/`shutting_down`'s always-emitted quirk
    note into the OpenAPI descriptions (already there — verify wording).
42. Consider `.gitignore` for `coverage.out` (the clean app trashes it;
    confirm it's ignored).

**Strategic (needs discussion)**

43. v0.2.0 scoping: what accumulates next (feature list or date-based).
44. v1.0 criteria draft (API freeze list, stability promises, deprecation
    burn-down incl. WithGETOnly).
45. Release automation evaluation (actions-based tag→release; GoReleaser is
    overkill for a library).
46. Announcement channel for v0.1.1 (user's call).
47. Evaluate `erraudit`/doanalyzerv2 as optional CI steps if either tool ever
    becomes public (currently documented local gates).
48. Property-based round-trip test: JSON → unmarshal → marshal identity
    (complements golden snapshot).
49. Consider exposing `Probe.Snapshot()` style accessor for structured
    logging consumers (only with a concrete consumer need).
50. Post-mortem habit: add the sanitized-PATH release check to every future
    Go release in every project (global AGENTS.md candidate).

## g) QUESTIONS I CANNOT ANSWER MYSELF

1. **Branch protection**: shall I require the 4 CI checks + linear history on
   `master` (needs owner/admin settings access; the workflow header
   recommends it)?
2. **Deprecation removal timeline**: when may `WithGETOnly` actually be
   removed — v0.2.0, v1.0, or indefinitely (the godoc currently promises no
   removal in v0.x, which hard-codes the answer until you override it)?
3. **Consumer bump**: shall I upgrade `go-health-dashboard` to v0.1.1 now
   (drop the replace, bump go.mod, run its suite) as the first real
   downstream consumer, or is that repo's release cadence handled separately?

---

## Verification trail for this report

- `nix run .#test` / `.#test-race` / `.#lint` green after last push
  (`2568e3e`).
- CI run `33865426899` (master) and the `v0.1.1` tag run: both `success`, 4/4
  jobs.
- Proxy: `go mod tidy` in a scratch module resolved
  `github.com/larsartmann/go-health v0.1.1`; consumer run printed `pass`.
- Release: `gh release view v0.1.1` → published, not prerelease.
- pkg.go.dev: 404 at 13:15 CEST (propagation lag — tracked as item 1).
