# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Nothing yet.

### Deprecated

- Nothing yet.

### Fixed

- Nothing yet.

## [0.1.1] - 2026-09-04

### Deprecated

- `WithGETOnly()` — superseded by `WithAllowedMethods(...)` (the method-set
  superset; `WithAllowedMethods()` with no arguments behaves identically).
  The option keeps working and no removal is planned in the v0.x line.

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

[Unreleased]: https://github.com/larsartmann/go-health/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/larsartmann/go-health/releases/tag/v0.1.0
[0.0.2]: https://github.com/larsartmann/go-health/releases/tag/v0.0.2

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

[0.0.1]: https://github.com/larsartmann/go-health/releases/tag/v0.0.1
