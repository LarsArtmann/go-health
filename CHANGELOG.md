# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Nothing yet.

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
  explicitly, because v2 does not sort map keys by default. The wire format is
  unchanged (keys stay alphabetically ordered), guarded by
  `TestReadiness_JSONChecksAreSortedAlphabetically`.
- Toolchain bumped to Go 1.26.7; Nix flake inputs and lockfile refreshed.

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

[Unreleased]: https://github.com/larsartmann/go-health/compare/v0.0.2...HEAD
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
