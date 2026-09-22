# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `Aggregate.Healthz()` — the aggregate's single-endpoint handler, mirroring `Probe.Healthz()`: 503 while any source has an unset startup latch, the merged roll-up is fail (which includes any source shutting down), or a source is draining; 200 otherwise, with the merged body (warn stays 200). Standalone like the root probe — `RegisterRoutes` keeps wiring the three kubelet probes. See docs/aggregate-healthz-design.md.
- `nix run .#openapi-lockstep` (also enforced as `checks.openapi-lockstep` under `nix flake check`, so the "Flake + Formatting" CI job runs it) — verifies every wire key in `testdata/readiness_response.golden` is declared in the `docs/openapi.yaml` HealthResponse/Check schemas and that status values are within the spec's enum, closing the silent-drift gap between the spec (redocly-linted) and the wire format (golden-file-tested).
- Measured the tracker's cost (`BenchmarkEvaluate_TrackerDelta`, A/B via a test-only `SetTrackerDisabledForTest` seam): Since stamping adds ~355 ns (+47%), 528 B, and 2 allocs per full `Evaluate` (8 stable checks, go1.27.1; B/allocs exact, ns ±15% run-to-run). Evaluate/tracker benchmark rows re-baselined in FEATURES.md.

### Changed

- Nothing yet.

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

[Unreleased]: https://github.com/larsartmann/go-health/compare/v0.3.0...HEAD
[v0.3.0]: https://github.com/larsartmann/go-health/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/larsartmann/go-health/compare/v0.1.3...v0.2.0
[v0.1.3]: https://github.com/larsartmann/go-health/compare/v0.1.2...v0.1.3
[v0.1.2]: https://github.com/larsartmann/go-health/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/larsartmann/go-health/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/larsartmann/go-health/releases/tag/v0.1.0
[0.0.2]: https://github.com/larsartmann/go-health/releases/tag/v0.0.2
[0.0.1]: https://github.com/larsartmann/go-health/releases/tag/v0.0.1
