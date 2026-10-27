# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `nix run .#fuzz-registry` (also `checks.fuzz-registry`, part of `.#gates`) diffs both fuzz apps in `flake.nix` against the `Fuzz*` targets in the tree and against each other, so a new target can no longer be written and silently never fuzzed. Backed by `tools/fuzz-registry-check.sh`.
- `docs-check` grew two drift rules: `[Unreleased]` must document something while HEAD is past the latest tag, and `docs/federation-design.md`'s fenced go API block must match federation's exported surface in both directions (it was missing `SourceStatuses` and `Healthz` while both shipped).
- `docs/ARCHITECTURE.md` now carries the per-file source map, the sub-package contracts and the test-file layout, moved out of `AGENTS.md` to keep that file under its 220-line preflight cap.
- `nix run .#recorder-audit` (with `docs/check-art-dupl`-style commentary in `docs/recorder-miswire-audit.md`) scans every local fleet checkout for `WithHealthRecorder` passed to a function-path constructor — the wiring that v0.8.0 rejects at `Start` but every consumer pinned below it still drops silently. 0 findings over 8,507 files.
- Three decision records settle questions the code cannot answer by itself: [ADR-009](docs/adr/ADR-009-shutdown-is-one-way.md) (shutdown is one-way; no re-arm API), [ADR-010](docs/adr/ADR-010-freshness-is-a-primitive.md) (freshness stays a primitive; no `WithStaleAfter`), and [docs/accessor-read-cost-design.md](docs/accessor-read-cost-design.md) (`CachedResponse` + `SourceStatuses` are two measured passes; no merged accessor).
- `docs-check` also enforces the exported API surface of the root package and `aggregate` against fenced `go api:` blocks, and that every non-test source file is named in `docs/ARCHITECTURE.md`.

### Changed

- Statement coverage is now 100% for `health`, `aggregate`, `checks` and `internal/merge`, 99.5% for `federation` (99.9% total). The one remaining statement is federation's request-construction guard, unreachable for any remote `New` accepts; the reason is recorded at `fetch`.
- New tests pin `federation.WithClient`, the truncated-body fetch path, and a concurrent drain through `federation.Healthz` (race detector plus the invariant that a drain never unlatches startup).
- Two defensive branches became testable instead of silent coverage holes: `applyStartupFailureOverlay` (root `Healthz`) and `versionMarshalFailure` behind a `marshalVersionResponse` seam, matching the existing `marshalResponse` seam.
- The freshness read is now measured rather than asserted: `BenchmarkCachedResponseFresh` confirms 0 B/op and 0 allocs/op, and `BenchmarkEvaluateAndCache` prices publishing at +384 B/+5 allocs (the batch deadline context, not the atomic store). Numbers in FEATURES "Performance".

### Removed

- Nothing yet.

### Fixed

- The `nix fmt` row in `AGENTS.md` claimed goimports while treefmt runs gofmt, golines and nixfmt; corrected, and markdown formatting is now documented as BuildFlow's responsibility because treefmt runs sandboxed and dprint downloads its plugins.

## [v0.8.1] - 2026-10-27

_CI-green re-cut of v0.8.0. Library content is identical to v0.8.0; the
v0.8.0 tag's own CI run was red on the security gate, which scanned a
machine-local tool whose dependency resolves only on the maintainer's box.
A published tag is never moved, so this patch release freezes the fixed
gate. Consumers need no change: prefer v0.8.1 for a green CI trail._


### Added

- `BenchmarkReadinessHandler_DrainingScaling` measures the drain fan-out at 1/2/4/8 remotes: cost is linear in remote count (~80 µs and ~14 kB per remote) because a drained hub still fetches every remote on every request, so a drain window is a load window. Read beside `BenchmarkSourceStatuses` (same fetch wave, different fold). Numbers and method: FEATURES "Performance".

### Changed

- `federation.Prober.Healthz` delegates its startup overlay to `applyStartupLatchOverlay`, which is unit-tested directly. The overlay is unreachable through the public read path — a successful fetch closes its remote's latch before the merge runs, a failed one contributes a reachable fail row — so the contract was only reachable by inspection; it now has a pin, and the nil-checks-map tolerance is exercised instead of assumed. Federation statement coverage 95.0% → 98.1%. No behaviour change.

### Removed

- Nothing yet.

### Fixed

- `gosec` no longer walks `tools/doanalyzerv2`: the local-only runner replaces a dependency with an absolute path that exists on no other machine, so the security gate exited 1 with zero findings while the library itself scanned clean.
- BuildFlow's `dprint-format` step can now pass again: 17 markdown files had unpadded tables (the step's `--fix` does not rewrite them, so `nix develop -c dprint fmt` is the repair).

## [v0.8.0] - 2026-10-27

### Added

- `Probe.CacheAge()` and `Probe.CachedResponseFresh(maxAge)` expose cache freshness as a contract instead of a timestamp convention. `CacheAge` separates "fresh snapshot" from "never evaluated" (the synthesized pass response is not evidence), uses the probe's injected clock so tests stay deterministic, and clamps a backwards clock step to zero. `CachedResponseFresh` returns the cached view (with the live shutdown overlay) only while `0 < age <= maxAge`; a non-positive window never reports fresh. Both are lock-free and allocate nothing. Four fleet consumers (DiscordSync, dnsblockd, cv, go-taskqueue) had hand-rolled the same adapters around `Timestamp`. Design: docs/cache-freshness-design.md.
- `Probe.EvaluateAndCache(ctx)` runs one `WithTimeout`-bounded batch, publishes it as the served snapshot, and returns it. `Evaluate` deliberately does not publish, so a fallback built on it answered correctly once and then kept serving the older snapshot — the bug shape behind three consumer adapters and the reason the documented "does not update the cache" note was not enough. Design: docs/cache-freshness-design.md.
- `Response.Clone()` returns an independent copy including the `Checks` map. `CachedResponse` is a lock-free read whose `Checks` map necessarily aliases the published snapshot; two consumers (dnsblockd, cv) hit the concurrent-map-write crash and cloned by hand, and docs/synthetic-check-overlay.md called that clone "the load-bearing line". The godoc now states the ownership rule and Clone is the sanctioned copy. Design: docs/response-ownership-design.md.
- `ErrRecorderNotApplicable` makes the function-path recorder trap loud: `NewWithHealthCheck`, `NewWithDetailedCheck`, and `NewChecks` accept `WithHealthRecorder` and previously dropped it silently (no callbacks, no traces, no error — the exact wiring go-appkit warns about in its own godoc). `Validate()` and therefore `Start()` now reject it, matching the `ErrUnknownCriticalService` fail-at-boot precedent. No signature changes. See go-health#7 finding 2.
- `ChainRecorders(...HealthRecorder)` composes several recorders into one ordered batch, skipping nils and winning collisions later. The chain always satisfies `DetailedHealthRecorder`, so per-check durations from detailed members survive while plain members contribute zero. This removes a bridge-level re-implementation (cqrs-htmx/health's own `RecorderChain` documented that "detailed recorders do not survive the merge").
- `federation.Prober.SourceStatuses()` (issue #6) mirrors `aggregate.Aggregate.SourceStatuses`: one parallel-fetch pass yields a per-remote roll-up, `fail` for unreachable and draining remotes, no startup-latch movement, and no rewrite of remote verdicts when the hub itself drains. The identical signature lets one dashboard capability cover both merge surfaces. Design: docs/federation-source-statuses-design.md.
- `WithCriticalService[T]()` and `DoServiceName[T]()` name samber/do's type-inferred service key without spelling it by hand. Four fleet repos (CV, KeyHolderAI, nsfw-classifier, DiscordSync) import `samber/go-type-to-string` solely to reconstruct the string `WithCriticalServices` demands, and the two sides can drift: a typo surfaces only at `Start` (and only when the service ran in the first batch), while a changed registration type fails silently. Both helpers call the same naming function samber/do uses, so the critical set and the batch keys agree by construction. `github.com/samber/go-type-to-string` becomes a direct requirement; it was already in the module graph through samber/do, so the runtime dependency count is unchanged.
- `NewWithRecorder(recorder, opts...)` is the injector-free recorder constructor. PMA, Zlota44, and cqrs-htmx all build `health.New(do.New(), health.WithHealthRecorder(...))`, passing a fake empty container only to satisfy the signature. The new constructor hands the recorder a nil injector (the documented shape for a recorder that owns its checks), composes with `WithHealthRecorder`, and yields an all-pass empty batch when no recorder is supplied.
- `HealthRecorderFunc` and `DetailedHealthRecorderFunc` adapt plain functions to the recorder interfaces, the recorder-side `http.HandlerFunc`. One-method recorders no longer need a named struct (DiscordSync, CV, and Zlota44 each carry one). The detailed adapter implements both interfaces and projects durations away on the plain path.
- `federation.Prober.Healthz()` completes the single-endpoint parity that `health.Probe` and `aggregate.Aggregate` already had: 503 while a remote startup latch is unset, the merged roll-up is fail, or the hub itself drains; 200 otherwise with the merged body. Non-critical `warn` stays 200. The read path and fetch cost are identical to readiness. Design: docs/federation-healthz-design.md.
- `RegisterRoutes` on all three surfaces (`Probe`, `Aggregate`, `federation.Prober`) now skips an empty `Routes` field instead of panicking in `ServeMux`. A service that only exposes readiness passes `health.Routes{Readiness: "/readyz"}`; `DefaultRoutes()` still mounts all three. Requested by Zlota44 ("method-scoped RegisterRoutes").
- `federation`: `Prober.MarkShuttingDown()` gives the federation hub the two-phase drain a single probe already had (`health.Probe.MarkShuttingDown`, issue #5). Once marked, readiness answers 503 with `shutting_down: true` and overall `fail` even with every remote healthy, while the merge-on-read fetches keep running so drained bodies stay fresh for dashboards and monitoring; liveness and startup are unaffected. This closes the LB hole where a hub behind a load balancer answered 200 "ready" all the way through graceful shutdown. There is no `Shutdown` counterpart: the prober runs no background loop, so phase two is the host process draining its own HTTP server. Design: docs/federation-design.md §"Two-phase drain".
- Test-surface hardening (no library behavior change): the drain is now pinned through the wired surface (`TestMarkShuttingDown_DrainsThroughRegisterRoutes` mounts the handlers on a real mux and observes only readiness flip to 503), through the fuzz safety net (`FuzzCachedResponse_ArbitraryRemoteBodies` gained a pre-marked draining twin prober, so the overlay branch — fail + `shutting_down` over ANY merge result — is fuzz-covered, corpus signature unchanged), and in the benchmark matrix (`BenchmarkReadinessHandler_Draining` records the drained readiness path end to end; federation's test helpers generalized from `*testing.T` to `testing.TB`, call-site compatible). The flake's `docs-drift-check` gate now also enforces the AGENTS.md 220-line preflight cap, so `nix run .#gates` and `nix flake check` catch what previously only BuildFlow's go-structure-linter saw.

### Changed

- Behavior change (loud failure, no compile break): a probe built by
  `NewWithHealthCheck`, `NewWithDetailedCheck`, or `NewChecks` with
  `WithHealthRecorder` now fails `Validate()`/`Start()` with
  `ErrRecorderNotApplicable`. Previously the option compiled and captured
  nothing. Consumers that passed it by accident get a boot-time error naming
  the fix; consumers that genuinely need recorder callbacks use
  `New(injector, ...)`. Fleet scan (2026-10-27): no production call site
  passes the option to a function-path constructor.
  **Migration:** switch the constructor to `New(injector, ...)` (recorder-path
  with a container) or `NewWithRecorder(recorder, opts...)` (injector-free).
  Nothing else changes: the option, the recorder interfaces, and the wire
  format are identical, and the failure lands at boot rather than at the first
  silent batch.
- `federation` now refuses a document whose own roll-up `status` is outside
  pass/warn/fail/off, exactly as it already refused an out-of-set per-check
  status. The merged verdict was never at risk (the merge recomputes from the
  checks), but `SourceStatuses()` reports each remote's own claim verbatim and
  `Status.Rank` scores an unrecognized value pass-tier, so an unvalidated
  roll-up could render as a healthy source card. Found by the new
  `FuzzSourceStatuses_Invariants` target. **Migration:** none for go-health
  producers (the library never emits an out-of-set status); a hand-written
  JSON endpoint that answered e.g. `{"status":"healthy"}` now surfaces as its
  `name/reachable` fail row — the intended loud failure.

### Removed

- Nothing yet.

### Fixed

- A recovered per-check panic (`ErrCheckPanicked`, the `NewChecks` executor)
  now grades the roll-up to `fail` even when the panicking check is
  non-critical, matching its godoc ("a recovered panic must read as a failure,
  never a warning"). `classifier.isRecoveredPanic` previously matched only the
  batch-level `ErrPanicDuringHealthCheck`. go-appkit/health pins the old warn
  behavior on v0.5.1 and must flip that pin in the same change that bumps it
  past this release (go-health#7 finding 1). Decision: ADR-008.
  **Migration:** a non-critical check that panics now rolls up `fail` (readiness
  503) instead of `warn` (200). A panicking check is a defect at either grade,
  but the impact widens: a pod carrying one is removed from rotation instead of
  staying in it while reporting a warning. The `WithEvaluationHook` observer
  keeps its older, deliberately different `warn` grading — it runs after the
  verdict exists, so its panic cannot have corrupted the answer. Fix the check,
  or wrap it so it returns an error instead of panicking.

## [v0.7.0] - 2026-10-11

### Changed

- `federation`: fetch results now carry their remote's state pointer (`fetchedRemote`) instead of pairing with `p.states` by index, so the merge loop can never rely on two slices staying length-aligned. Internal only — no exported API and no behavior change.
- `aggregate` and `federation` now share one merge implementation: the new `internal/merge` package owns the worst-of fold (worst source status via `Status.Rank`, `"source/check"` namespacing, the shutdown overlay, max-latency, per-process-scalar drop), and both packages map their acquisitions onto it. No exported API change. One rule was unified while doing so: rank ties (a source whose own roll-up is `off` or unknown) no longer replace the accumulated status in federation's merged roll-up, which keeps the merged status inside the frozen pass/warn/fail enum and makes the fold order-independent; previously federation let the last same-rank value win, so a remote document with top-level `status: "off"` followed by a passing remote flipped the merged status to `off`. The duplicated merge fuzz/property assertions collapsed into one suite over the primitive (`FuzzResponsesMergeInvariants`); each package keeps thin mapping tests, and every pre-existing package test passed unchanged as the port's oracle. Design: docs/merge-unification-design.md.

### Removed

- `WithGETOnly()`, deprecated since v0.1.1: `WithAllowedMethods()` with no arguments is the canonical, behavior-identical spelling (both seed the accepted method set with GET alone). The adoption matrix (2026-10-09) found zero production adopters — KeyHolderAI's usage was tests only and is fixed locally; removal rides the staged rename decision table (docs/naming-integrity.md §"v0.6 staged-rename decision table") under the deprecation policy's new zero-production-adopter removal path. Migration: replace `health.WithGETOnly()` with `health.WithAllowedMethods()` — or list methods explicitly (e.g. `health.WithAllowedMethods(http.MethodGet)`).

### Fixed

- Test-surface fix: `TestCachedResponse_NeverStartedSource` hardcoded `TotalLatencyMs == 0`, but the primed source's own evaluation latency can legitimately read 1ms under full-suite parallelism (observed once under the race-stress fan-out), flaking the never-started merge property it pins. The assertion now checks the real property — the never-started source contributes nothing, so the merged total equals the real source's own cached batch. Test-only; no library change.
- Test-surface fix: `TestRefreshLoop_TickerFiresBeforeShutdown` slept a fixed 50ms and then required two health-check calls; under the full-suite race-stress fan-out the 5ms ticker could starve past the fixed window (observed once), flaking the loop-fires property it pins. The test now polls for the second call with a 5s deadline. Test-only; no library change.

## [v0.6.1] - 2026-10-11

### Fixed

- Security-gate tooling only — no library change, library content identical to v0.6.0. gosec 2.29.0 embeds a go1.26-era x/tools that cannot decode the export data (v5) that go 1.27.2 emits, so when nixpkgs silently moved its `go_1_27` package from 1.27.1 to 1.27.2, the gate exited 1 on 39 import errors with a vacuous zero-findings scan (all packages skipped typechecking). That broke master CI and the v0.6.0 tag's CI run (frozen at the tag, permanently red there — the same content verified fully green under the fixed gate). The gate now runs gosec's `go list` under a hardcoded go 1.27.1 (the v4/v5 export-data boundary sits between 1.27.1 and 1.27.2), and the `goPkg` overrideAttrs source bump is dropped per its own documented drop-condition (nixpkgs ships 1.27.2 natively, carrying the GO-2026-6603..6617 stdlib fixes). This release exists so consumers and the fleet have a tagged release whose CI is fully green.

## [v0.6.0] - 2026-10-11

### Added

- Test-surface hardening (no library behavior change): `FuzzThrottleWindowBoundary` drives the `WithLiveThrottle` window rule on a fake clock for arbitrary advances (including a backward clock); the aggregate merge fuzz now covers sources in live, cache-mode (background refresh loop), and throttled-live freshness modes — modes derived from name parity so the accumulated corpus stays loadable; both marshal/handler fuzz corpora carry golden-fixture seeds mirroring `testdata/readiness_response.golden`; `BenchmarkReadinessHandler_LiveEvalContention` records the throttled live path under full parallel load (baseline in FEATURES.md); CI gains a race-stress step (`-count=3`).
- `aggregate.(*Aggregate).SourceStatuses()` — per-source roll-up visibility: one `map[string]health.Status` entry per source name, one atomic cache load per source, no evaluation, no wire impact. A shutting-down source reports `fail` for itself; the merged status stays the worst of these values. Answers the dashboard question "is source api healthy overall?" without refolding `source/check` keys. Design: docs/aggregate-per-source-visibility-design.md; pinned by a 27-combination worst-of property test.

### Changed

- `aggregate.New` now reports every invalid source at once instead of failing on the first: the error joins one wrapped `ErrInvalidSource` per problem (one line each, multi-line message), so a misconfigured fleet costs one fix-run cycle instead of N. `errors.Is` against `ErrInvalidSource`/`ErrNoSources` is unchanged. Behavior change with a callout per docs/errors-join-design.md; pinned by `TestNew_JoinsAllInvalidSources`.

### Fixed

- `Shutdown()` no longer blocks for the `WithShutdownGracePeriod` window on a probe whose lifecycle never started (`Start` not called, or failed critical-name validation): with nothing to drain, the window was dead time — up to the full grace duration between a failed `Start` and the retry. Probes whose `Start` succeeded still honor the window in every mode, including live mode (`WithRefreshInterval(0)`), where the block is the caller's drain window even though no refresh loop exists. Pinned by `TestShutdown_GraceWindowSkippedWhenLifecycleNeverStarted` (never-started, failed-Start, and live-mode-honors-window cases).

## [v0.5.1] - 2026-10-09

### Added

- `health.Off(detail)` / `StatusOff` — express an intentionally unconfigured dependency as a visible check row instead of a fake failure or a silent omission. Any executor channel (CheckFunc, `NewWithHealthCheck` batch, DetailedHealthRecorder, samber/do `HealthcheckerWithContext`) can return the sentinel; the check renders `"status":"off"` with the enable recipe in its `error` field. Off is visibility, never a verdict: readiness stays 200, the roll-up stays three-state (`Response.Status` never reports off), an all-off instance rolls up pass, the startup latch treats an off critical service as satisfied, and `Status.Rank()` ranks off pass-tier so aggregate/federation merges never warn on it. Federation accepts `off` as a known status (unknown statuses are still refused whole); consumers older than this release refuse off documents, so producers may emit off only after their federation path has bumped — rollout order in docs/configured-off-design.md.

### Fixed

- A panicking `WithEvaluationHook` callback no longer kills the process on the background-refresh-loop path (unrecovered goroutine panic) or breaks the handler contract on live paths. It is recovered and made visible: the response gains a synthetic non-critical `evaluation-hook` check (`warn`, error wrapping the new `ErrPanicDuringEvaluationHook` sentinel text plus the panic value) and the roll-up degrades to `warn` at most — never to `fail`, because the evaluation completed before the observer ran. The hook also now receives a defensive copy of the response (its own `Checks` map), so a misbehaving hook can no longer mutate the served or cached response even without panicking. Decision and rationale: docs/panic-recovery-design.md ("the evaluation-hook surface").
- `checks.Disk` no longer trips gosec G115: the `stat.Bsize` int64→uint64 conversion is guarded before the free-bytes multiplication. Pre-existing since the battery shipped in v0.5.0; caught by the `.#security` gate during the configured-off work.
- A probe whose `Start()` fails critical-name validation no longer wedges the lifecycle: the refresh loop armed before the initial evaluation is now disarmed on the failure path. Since v0.5.0, such a probe blocked forever in `Shutdown()` (a WaitGroup counter nothing would release) and every retry-`Start` silently no-oped. Pinned by a lifecycle regression test and a new aggregate-composition integration test (a misconfigured source fails its own `Start` loudly; the aggregate keeps reporting it as never-started).

## [0.5.0] - 2026-10-05

### Added

- `health/checks` subpackage — batteries-included, zero-dependency health-check functions: `Disk(path, minFreeBytes)`, `Memory(maxAllocBytes)`, `HTTP(url, timeout)` (wraps `ErrHTTPCheckFailed`), `Database(db, pingTimeout)` (wraps `ErrDatabaseUnreachable`). Each is a `CheckFunc`-compatible function composing with `NewChecks`/`NewWithHealthCheck`/recorders. Resource checks are warn-by-default (criticality stays with the caller); thresholds are arguments. Ownership decision + constraints: docs/batteries-ownership-decision.md.
- `Start()`-time critical-name validation with `ErrUnknownCriticalService` — a name passed to `WithCriticalServices` that never appears in the first health-check batch now fails `Start` (naming the offending services, sorted) instead of silently degrading readiness classification and blocking the startup latch forever. Validation reads the initial evaluation batch, so it applies uniformly to the injector, recorder, and standalone-constructor paths — including probes built over an empty injector with a recorder. Wire format unchanged; see docs/start-validation-design.md for the gating decision and the fleet compatibility scan.
- `health.VersionHandler(version)` — opt-in `/version` endpoint helper serving the binary's build stamp (ldflags/VCS pattern) as `{"version":"..."}`, wire-it-yourself next to the probe routes. Identity, not health: never evaluates checks, never 503, unaffected by shutdown; GET-only (405 + `Allow: GET`) unconditionally, mirroring the method guard's posture. The version is UTF-8-coerced and the payload pre-marshaled at construction (immutable for the process lifetime — per-request cost is headers plus one slice copy). Wire shape documented in docs/openapi.yaml (`VersionResponse`; spec bumped to 0.5.0 for the addition). Routed from the go-version-auto-configure session (TODO_LIST 2026-09-25). Additive — no consumer change required.

### Fixed

- Nothing yet.

## [v0.4.1] - 2026-09-25

### Fixed

- `go` directive lowered from `go 1.27.1` to `go 1.27` (major.minor-only form). v0.4.0's patch-form floor forced every consumer's `go mod tidy` to lift its directive to `1.27.1` — the known fleet floor-poisoning class (go-version-auto-configure's dep-floor gate classified consumers like go-health-dashboard as dep-forced). Floor-lowering is consumer-compatible: consumers on `1.27.1` keep building unchanged.

## [v0.4.0] - 2026-09-22

### Added

- `Aggregate.Healthz()` — the aggregate's single-endpoint handler, mirroring `Probe.Healthz()`: 503 while any source has an unset startup latch, the merged roll-up is fail (which includes any source shutting down), or a source is draining; 200 otherwise, with the merged body (warn stays 200). Standalone like the root probe — `RegisterRoutes` keeps wiring the three kubelet probes. See docs/aggregate-healthz-design.md.
- `nix run .#openapi-lockstep` (also enforced as `checks.openapi-lockstep` under `nix flake check`, so the "Flake + Formatting" CI job runs it) — verifies every wire key in `testdata/readiness_response.golden` is declared in the `docs/openapi.yaml` HealthResponse/Check schemas and that status values are within the spec's enum, closing the silent-drift gap between the spec (redocly-linted) and the wire format (golden-file-tested).
- Measured the tracker's cost (`BenchmarkEvaluate_TrackerDelta`, A/B via a test-only `SetTrackerDisabledForTest` seam): Since stamping adds ~355 ns (+47%), 528 B, and 2 allocs per full `Evaluate` (8 stable checks, go1.27.1; B/allocs exact, ns ±15% run-to-run). Evaluate/tracker benchmark rows re-baselined in FEATURES.md.

### Changed

- Tooling: `GOEXPERIMENT=jsonv2` dropped from the flake (every app + the devShell) — `encoding/json/v2` is stable stdlib on the go 1.27 floor (verified 2026-09-22: build + vet + full suite green with the experiment unset). Contributors need Go 1.27+ and nothing else; the experiment era's host-shell-leak lesson is preserved in AGENTS.md.
- The Prometheus example (`ExampleWithEvaluationHook_metrics`) now emits `health_check` series in sorted order, making the reference implementation byte-for-byte deterministic across scrapes — matching the determinism the JSON wire format pins.
- ADR-006: duration units stay split by granularity — roll-up scalars in always-present milliseconds (`total_latency_ms`), per-entity measurements in `omitzero` nanoseconds (`duration_ns`); unifying on either unit is rejected (ms would zero out sub-millisecond checks, ns is human-hostile for batch totals). Codifies the unit policy future duration fields must follow.
- Docs: detailed-checks cookbook ([docs/detailed-checks-cookbook.md](docs/detailed-checks-cookbook.md)) — how to get `duration_ns` onto the wire per check source: `NewChecks` (executor times for free), `NewWithDetailedCheck` (self-timed composed checks, with an honest `timed` helper and fan-out guidance), `DetailedHealthRecorder` (recorder upgrade), and why the raw `New(injector)` path stays duration-less (do returns only errors). Snippets verified against the actual API.
- Internal dedup, no behavior change: the three standalone constructors (`NewWithHealthCheck`, `NewWithDetailedCheck`, `NewChecks`) share one config builder enforcing the recorder-voiding rule, and the live handler paths share one batch-deadline application; the parallel `RegisterRoutes`/`Option` shapes across `health`/`aggregate`/`federation` stay deliberately similar (the three-handler contract repeats per package).

## [v0.3.0] - 2026-09-19

### Added

- `NewChecks(checks map[string]CheckFunc, opts ...Option)` — the ergonomic
  injector-free constructor for plain named checks (`checks.go` +
  `docs/named-checks-design.md`): checks run concurrently per batch, each is
  timed (surfaces as `duration_ns`), a panicking check is recovered into that
  check's error, a nil check fails closed, and a check that ignores its
  context is abandoned at the batch deadline with a fail-closed error — the
  probe can no longer hang on a wedged check. Classification, caching,
  handlers, and shutdown semantics are identical to `New`.
- Aggregate merge property tests (`aggregate/aggregate_property_test.go`):
  idempotence, source-order commutativity, namespaced-union completeness and
  disjointness, worst-of roll-up (status/shutdown/latency), an absorbing
  shutdown invariant, and handler-status mirroring — enumerated over every
  source topology of up to three distinct cached states (pass, warn, fail,
  shutting down, never-started).
- Benchmarks covering previously unmeasured paths: `BenchmarkAggregateHandlers`
  (the aggregate HTTP path per handler), `BenchmarkEvaluate_Scaling`
  (injector-free evaluation vs. check count), and
  `BenchmarkTransitionTrackerStamp` (the per-batch Since-stamp cost).
  Baselines recorded in FEATURES.md.
- ADR-005: the aggregate source-name contract (`/` rejected) promoted from a
  design note into the formal ADR series.
- Design note for proposed aggregate `Healthz()` parity
  (`docs/aggregate-healthz-design.md`) — the single-endpoint handler the
  aggregate lacks; implementation deferred beyond v0.3.0 (this release ships
  federation + `NewChecks`; the aggregate handler is tracked in ROADMAP).
- README "Which probe should I hit?" decision table mapping each consumer
  (kubelet probes, load balancer, dashboard, in-process code) to its probe.
- Federation: `health/federation` pulls remote go-health instances over
  HTTP and merges them into one surface — `federation.New(remotes,
  opts...)` returns a `Prober` satisfying the same five-method surface
  the go-health-dashboard consumes, so a hub (e.g. `health.home.lan`)
  renders every service's checks per remote with zero dashboard changes.
  Merge-on-read (one parallel fetch per remote per read, per-fetch
  timeout, 1 MiB body cap); checks land namespaced as `name/check`;
  `Check.Since`/`DurationNanos` survive the wire verbatim; an
  unreachable, non-200, undecodable, or status-invalid remote
  contributes one synthetic `name/reachable` fail check instead of
  freezing silently. Liveness is fetch-free, readiness serves the
  merged verdict, startup latches per remote on first successful fetch.
  Design: `docs/federation-design.md`.
- `Status.Rank()` is public: the severity ordering (fail 0 < warn 1 <
  pass 2, unknown ranks as pass) behind the merge sites, so consumers
  merging statuses share one ordering. `aggregate` delegates to it.

### Changed

- The OpenAPI spec now states explicitly that it covers the aggregate
  endpoints (same paths and schemas), documenting the body-level differences:
  `source/check` keys, dropped per-process scalars, slowest-source latency,
  and absorbing shutdown. Spec version bumped to 0.2.0.
- `ExampleNewWithDetailedCheck` output label corrected: it asserts the `since`
  transition timestamp is set, not a failure state.
- `Status.Rank` switch made exhaustive (explicit `StatusPass` case) to satisfy
  the `exhaustive` linter; behavior unchanged.

## [0.2.0] - 2026-09-16

### Added

- Per-check metadata answering "how long has it been failing?" and "how
  slow is it?" (issue #2): `Check.Since` (probe-observed time the check
  entered its current status — carried forward while the status holds,
  restarted on change, reset on process restart) and `Check.DurationNanos`
  (executor-reported execution time, nanoseconds). Both omitted from JSON
  via `omitzero` when unknown, so existing wire payloads are unchanged on
  paths that do not produce them. Full semantics:
  `docs/check-metadata-design.md`.
- `CheckDetail` (executor's raw report: outcome + duration) with two opt-in
  sources: `NewWithDetailedCheck(fn)` for injector-free probes and the
  optional `DetailedHealthRecorder` interface a `HealthRecorder` can
  implement to pass per-check durations through. Classification stays with
  the probe on both paths. The raw samber/do injector path reports zero
  duration (unknown): do's batch API cannot measure per-check timing.
- Wire-format note: the per-check duration is `duration_ns` (int64
  nanoseconds), not a `time.Duration` — `encoding/json/v2` cannot marshal
  `time.Duration` at all (go.dev/issue/71631; no tag format exists), so a
  plain lossless integer keeps every consumer's re-marshal working. The
  aggregate golden file now deterministically locks the merged `since`
  fields (primed sources use a fixed clock).

### Changed

- The `go` directive is relaxed from `1.26.7` to `1.26`: any go1.26.x
  toolchain now builds the module. The `GOEXPERIMENT=jsonv2` requirement on
  go1.26 is unchanged. Toolchain-only; no API or behavior change.

## [0.1.3] - 2026-09-04

### Added

- `nix run .#gates` — one-command pre-push gate sweep (same gates as CI,
  fail-fast, subsettable) and `nix run .#ci-emulation` — the same gates under
  a go-free PATH, replacing the copy-paste CI-emulation recipe in
  CONTRIBUTING.md. Both verified end-to-end on 2026-09-04.
- `nix run .#fuzz-long` — 5-minute-per-target fuzz budget backing the new
  weekly scheduled `Fuzz (weekly long)` workflow (corpus uploaded on failure).
- Aggregate readiness JSON golden file
  (`aggregate/testdata/aggregate_readiness_response.golden`, regenerate with
  `go test ./aggregate -update`) locking the merged wire format through the
  real handler path.
- Aggregate merge benchmark `BenchmarkAggregateCachedResponse` (N=1,2,4,8
  sources) and guard benchmark variants `BenchmarkGuardOverhead_HEADAllowed`
  / `_AllowHeader`; baselines in FEATURES.md.
- Throttle × Start tests: requests against the loop-refreshed cache trigger
  zero batches, and a stale cached result triggers exactly one live
  evaluation — the README paragraph is now test-backed.
- Named regression tests pinning the instance_id UTF-8 sanitize fix
  (unit + end-to-end no-500 path) and the aggregate never-started-source
  merge behavior.
- Aggregate godoc examples (`ExampleNew`, `ExampleNew_scalarsDropped`,
  `ExampleAggregate_StartupHandler`) and root examples
  (`ExampleWithShutdownGracePeriod`, `ExampleProbe_AsShutdowner`).
- `tools/doanalyzerv2/run.sh` wrapper: validates the analyzer checkout,
  honors `GO_HEALTH_BRANCHING_FLOW`, and prints actionable remediation
  instead of a raw module-resolution failure.

### Changed

- `aggregate.New` now rejects source names containing `/` (wrapped
  `ErrInvalidSource`): the name becomes the `"name/check"` key prefix, and a
  slash would silently alias another source's namespace and break the
  documented grouping axis. Check names may still contain `/`; everything
  before the first slash in a merged key is the source name. Rationale and
  migration guidance in `docs/aggregate-source-name-design.md`.

## [0.1.2] - 2026-09-04

### Added

- Godoc examples for the programmatic API: `ExampleNewWithHealthCheck`,
  `ExampleProbe_Healthz`, `ExampleWithEvaluationHook`, `ExampleProbe_AwaitReady`,
  `ExampleWithHealthRecorder`, `ExampleProbe_MarkShuttingDown`, and
  `ExampleProbe_CachedResponse`.
- Fuzz target for the `aggregate` package's merge-on-read invariants (worst-of
  status, `source/check` namespacing, scalar dropping, handler codes) and
  `instance_id`-populated seeds for the root marshal fuzz.
- Property test pinning unmarshal → marshal wire identity of `Response`.
- `tools/doanalyzerv2`: persisted local runner for the private do-analyzer
  audit (see CONTRIBUTING.md).
- `SECURITY.md` vulnerability-disclosure policy, PR template, and
  `docs/deprecation-policy.md`.
- CI job linting `docs/openapi.yaml` with Redocly; the spec now states its
  unauthenticated-by-design posture and MIT license explicitly.

### Deprecated

- Nothing yet.

### Fixed

- Invalid UTF-8 in the replica identifier set via `WithInstanceID` failed
  JSON encoding of every health response, turning all probe endpoints into
  HTTP 500. `SanitizeResponse` now coerces `instance_id` like the other
  string fields. (Found by the new marshal-fuzz seeds.)

## [0.1.1] - 2026-09-04

### Added

- doanalyzerv2 verification: 0 anti-pattern findings (DO-1..DO-6 incl. injector-in-service) across the root and aggregate packages.

- Exhaustive classify matrix test (8 health-state assignments × 8 critical
  sets, asserted against an independent spec), lifecycle stress tests for
  concurrent `Start`/`Shutdown`/`MarkShuttingDown` interleavings, a
  restart-after-shutdown contract test, and JSON golden snapshot tests
  (`testdata/readiness_response.golden`, regenerate with `go test . -update`).

- Programmatic health API (purely additive): `Probe.Status()`, `Probe.Alive()`,
  `Probe.Ready()` cached-view accessors; `Probe.AwaitReady(ctx)` blocking
  startup wait; `Probe.Healthz()` combined single-endpoint handler; and
  `Probe.HealthCheck(ctx)` — the probe now satisfies
  `do.HealthcheckerWithContext` for self-registration, wrapping the new
  exported `ErrProbeUnhealthy` sentinel when the roll-up is fail.

- Injector-free construction: `NewWithHealthCheck(fn, opts...)` plus the
  exported `HealthCheckFunc` type — run a probe from any batch function,
  no samber/do container required.

- New options: `WithEvaluationHook(fn)` (observe every classified response
  synchronously — the metrics/alerting seam), `WithLiveThrottle(d)` (coalesce
  live-mode request floods to one batch per window), `WithShutdownGracePeriod(d)`
  (two-phase shutdown timing), `WithNowFunc(fn)` (deterministic clock for
  uptime, response timestamps, and throttle freshness in tests),
  `WithAllowedMethods(methods...)` (method-set guard replacing the GET-only
  boolean; 405 responses carry a sorted `Allow` header, GET always included),
  and `WithInstanceID(id)` (replica identifier in every response:
  `instance_id`).

- `Response.Timestamp` — RFC 3339 evaluation completion time, omitted from
  JSON until the first evaluation (`omitzero`).

- Conformance helpers: `ProbeShutdowner` / `Probe.AsShutdowner()` adapt a
  probe to `do.ShutdownerWithError`; `SanitizeResponse` coerces invalid UTF-8
  (which `encoding/json/v2` rejects — found by fuzzing) to valid UTF-8,
  restoring v1 wire behavior for malformed error strings.

- `ResetStartupLatchForTest()` (test builds only, `export_test.go`): clears
  the one-way startup latch so tests can exercise the full latch lifecycle.
  The public latch remains strictly one-way.

- Middleware, Prometheus, and OpenAPI guidance with verified spikes:
  `ExampleProbe_ReadinessHandler_middleware` (auth-wrapped readiness),
  `prometheus_example_test.go` (hook + stdlib exposition writer), and a static
  `docs/openapi.yaml` kept in lockstep with the golden-file test. Design
  notes: `docs/middleware-design.md`, `docs/prometheus-exposition-design.md`,
  `docs/openapi-design.md`, `docs/classification-2.0-design.md`,
  `docs/multi-tenant-design.md`, `docs/starting-status-design.md`.

### Changed

- Recovered health-check panics now roll up to `fail` (readiness 503)
  instead of `warn`, via the new `ErrPanicDuringHealthCheck` sentinel; the
  synthetic `health-check` JSON entry is graded `fail` to match. Rationale
  in `docs/panic-recovery-design.md`. Injector-path service panics remain
  process-fatal (samber/do runs checks in goroutines).
- `writeResponse` marshal-failure body now includes the underlying cause.

### Deprecated

- `WithGETOnly()` — superseded by `WithAllowedMethods(...)` (the method-set
  superset; `WithAllowedMethods()` with no arguments behaves identically).
  The option keeps working and no removal is planned in the v0.x line.

### Fixed

- Lifecycle race: concurrent `Start` and `Shutdown` could trigger
  `sync: WaitGroup is reused before previous Wait has returned`. The
  WaitGroup `Add` and `Wait` are now serialized under the probe mutex.

- Wire-format regression from the v0.1.0 `encoding/json/v2` migration: v2
  rejects invalid UTF-8 where v1 replaced it, so a service returning a
  malformed error string could turn a health endpoint into a 500.
  `SanitizeResponse` now coerces responses to valid UTF-8 at both write
  seams (root and aggregate) before marshaling. Found by fuzzing.

## [0.1.0] - 2026-09-04

### Added

- New `aggregate` sub-package: merge multiple in-process `*health.Probe`
  instances into a single health surface. `aggregate.New(sources...)` combines
  N probes (unique non-empty names, non-nil probes) into one `*Aggregate` with
  the full go-health-compatible surface: `CachedResponse` (merge-on-read,
  worst-of status, `"source/check"` namespaced keys, shutdown overlay),
  `RefreshInterval` (slowest source), `StartupComplete` (AND of startup
  latches), liveness/readiness/startup handlers, and `RegisterRoutes`.
  Purely additive — no changes to the root `health` package.

### Changed

- Migrated JSON serialization from `encoding/json` to `encoding/json/v2`
  (Go 1.26+). All three handlers serialize responses through the new
  implementation, and `writeResponse` now passes `json.Deterministic(true)`
  explicitly, because v2 does not sort map keys by default. Key order stays
  alphabetical (`TestReadiness_JSONChecksAreSortedAlphabetically`), but note
  one behavioral difference found by the JSON snapshot tests: v2 ignores
  `omitempty` on scalar fields, so `"shutting_down":false` and
  `"total_latency_ms":0` are now always emitted where v1 omitted them.
  Consumers parsing strictly should tolerate these always-present fields.
- Toolchain bumped to Go 1.26.7; Nix flake inputs and lockfile refreshed.
- The flake now exports `GOEXPERIMENT=jsonv2` in every app and the devShell,
  making the gates hermetic (previously they only worked because the
  maintainer's shell leaked the variable).

## [0.0.2] - 2026-08-08

Adds read-only accessors so composition layers (e.g. `go-health-dashboard`) and
middleware can inspect cached health state without triggering a synchronous
evaluation through every dependency. Purely additive — no breaking changes.

### Added

- `Probe.CachedResponse() Response` — returns the last background-refreshed
  health `Response` from the atomic `latest` pointer, enabling lock-free,
  zero-dependency reads for callers that need to query health state without a
  full evaluation pass.
- `Probe.RefreshInterval() time.Duration` — accessor returning the configured
  background cache refresh interval; zero indicates live (non-caching)
  evaluation mode.

### Changed

- `CachedResponse()` now reflects the live shutdown state in both the cached and
  the no-cache fallback paths. Previously the no-cache fallback returned
  `StatusPass` even while shutting down; it now returns `ShuttingDown=true` and
  `Status=StatusFail` so load balancers and orchestrators stop routing traffic
  to a draining instance.

## [0.0.1] - 2026-08-07

First public release. Three-probe Kubernetes health-probe SDK for samber/do v2.

### Added

- Three-probe pattern: liveness (`/healthz`), readiness (`/readyz`), startup (`/startupz`).
- Critical/non-critical service classification via `WithCriticalServices`.
- Background caching with configurable refresh interval (`WithRefreshInterval`, default 1s).
- Batch-level timeout (`WithTimeout`, default 5s).
- Shutdown awareness: `Shutdown()` and `MarkShuttingDown()` for two-phase graceful drain.
- GET-only enforcement (`WithGETOnly`).
- Optional `HealthRecorder` integration (e.g. `samber-do-auditlog.Plugin`).
- Construction-time capability resolution — Probe holds a resolved `healthCheckFunc`, never the `do.Injector`.
- Config validation via `Validate()` with enriched sentinel errors (`ErrInvalidTimeout`, `ErrInvalidRefreshInterval`).
- `Start(ctx) error` validates configuration and returns sentinel errors on invalid settings.
- Panic recovery in `runHealthChecks` — panics from misbehaving recorders or services are recovered and reported as a synthetic `health-check` error.
- 7 functional options: `WithVersion`, `WithCriticalServices`, `WithHealthRecorder`, `WithRefreshInterval`, `WithTimeout`, `WithBootTime`, `WithGETOnly`.
- Public `Evaluate(ctx) Response` method for custom handler scenarios.
- `StartupComplete() bool` method to query the startup latch.
- `RegisterRoutes` and `DefaultRoutes` for conventional Kubernetes path registration.
- `flake.nix` with devShell (flake-parts + treefmt), Nix apps for test, test-race, build, vet, lint, coverage, vulncheck, security, clean.
- `.golangci.yml` with curated linter configuration (0 issues).
- Comprehensive test suite with race detector coverage.
- `example_test.go` with runnable examples.

[Unreleased]: https://github.com/larsartmann/go-health/compare/v0.8.1...HEAD
[v0.8.1]: https://github.com/larsartmann/go-health/compare/v0.8.0...v0.8.1
[v0.8.0]: https://github.com/larsartmann/go-health/compare/v0.7.0...v0.8.0
[v0.7.0]: https://github.com/larsartmann/go-health/compare/v0.6.1...v0.7.0
[v0.6.1]: https://github.com/larsartmann/go-health/compare/v0.6.0...v0.6.1
[v0.6.0]: https://github.com/larsartmann/go-health/compare/v0.5.1...v0.6.0
[v0.5.1]: https://github.com/larsartmann/go-health/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/larsartmann/go-health/compare/v0.4.1...v0.5.0
[v0.4.1]: https://github.com/larsartmann/go-health/compare/v0.4.0...v0.4.1
[v0.4.0]: https://github.com/larsartmann/go-health/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/larsartmann/go-health/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/larsartmann/go-health/compare/v0.1.3...v0.2.0
[v0.1.3]: https://github.com/larsartmann/go-health/compare/v0.1.2...v0.1.3
[v0.1.2]: https://github.com/larsartmann/go-health/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/larsartmann/go-health/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/larsartmann/go-health/releases/tag/v0.1.0
[0.0.2]: https://github.com/larsartmann/go-health/releases/tag/v0.0.2
[0.0.1]: https://github.com/larsartmann/go-health/releases/tag/v0.0.1
