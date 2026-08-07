# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

No version has been tagged yet. The library is in ALPHA — breaking changes
are expected until v1.0.

## [Unreleased]

### Breaking

- **`Probe.Start(ctx)` now returns `error`** instead of being void. It calls `Validate()` internally and returns `ErrInvalidTimeout` or `ErrInvalidRefreshInterval` on invalid configuration. Callers must check the return value: `if err := probe.Start(ctx); err != nil { ... }`.
- **`Option` type signature changed** from `func(*Probe)` to `func(*config)`. The `config` type is unexported, so consumers who wrote custom `Option` functions must switch to the provided `With*` functions. This was necessary to eliminate the dead `recorder` field from the `Probe` struct.

### Added

- Panic recovery in `runHealthChecks` — panics from misbehaving recorders or services are recovered and reported as a synthetic `health-check` error entry instead of crashing the process.
- `flake.nix` with devShell (flake-parts + treefmt), Nix apps for test, test-race, build, vet, lint, coverage, vulncheck, security, clean.
- `.golangci.yml` with enforced linter configuration (gosec, staticcheck, govet, revive, errcheck, and more).
- `FEATURES.md` with honest feature inventory by status.
- `TODO_LIST.md` with actionable short/mid-term tasks.
- `ROADMAP.md` with long-term direction and raw ideas.
- `docs/DOMAIN_LANGUAGE.md` defining liveness, readiness, startup, critical service, and other domain terms.
- Expanded `README.md` with configuration reference for all 7 options, troubleshooting section, shutdown/two-phase drain docs, and development setup.
- Test asserting JSON `checks` map is alphabetically sorted (deterministic output).
- Test asserting `Start()` returns error on invalid timeout and refresh interval.
- Test asserting panic recovery from misbehaving recorder.
- Test asserting startup latch does not flip when a slow critical service times out.
- Test asserting `refreshLoop` ticker fires before shutdown.
- Test asserting 405 response body contains the actionable error message.
- Test asserting `Shutdown()` without prior `Start()` does not panic.
- `StartupHandler` benchmark and recorder-path benchmark.

### Changed

- Options now write to a construction-only `config` struct instead of the `Probe` directly. The `Probe` struct no longer carries a `recorder` field — the recorder reference lives only in the `healthCheck` closure.
- Converted `resolveHealthCheck` from a method to a free function — it is construction-only behavior, not ongoing Probe behavior.
- Reverted `slog.Debug` from `writeResponse` — libraries must not make logging decisions for the host application. HTTP write failures are silently swallowed.
- Rewrote `WithTimeout` doc comment to clarify that the deadline is batch-level and shared across all services, not per-service.
- Added "Timeouts" section to package doc explaining the relationship between `WithTimeout` (batch-level) and `do.WithHealthCheckTimeout` (per-service).
- Removed `write_response_internal_test.go` — the test exercised a code path that is unobservable by design (write errors are silently swallowed).

## [0.1.0] - 2026-08-07

### Added

- Three-probe pattern: liveness (`/healthz`), readiness (`/readyz`), startup (`/startupz`).
- Critical/non-critical service classification via `WithCriticalServices`.
- Background caching with configurable refresh interval (`WithRefreshInterval`, default 1s).
- Batch-level timeout (`WithTimeout`, default 5s).
- Shutdown awareness: `Shutdown()` and `MarkShuttingDown()` for two-phase graceful drain.
- GET-only enforcement (`WithGETOnly`).
- Optional `HealthRecorder` integration (e.g. `samber-do-auditlog.Plugin`).
- Construction-time capability resolution — Probe holds a resolved `healthCheckFunc`, never the `do.Injector`.
- Config validation via `Validate()` with enriched sentinel errors.
- 7 functional options: `WithVersion`, `WithCriticalServices`, `WithHealthRecorder`, `WithRefreshInterval`, `WithTimeout`, `WithBootTime`, `WithGETOnly`.
- Public `Evaluate(ctx) Response` method for custom handler scenarios.
- `StartupComplete() bool` method to query the startup latch.
- `RegisterRoutes` and `DefaultRoutes` for conventional Kubernetes path registration.

[Unreleased]: https://github.com/larsartmann/go-health/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/larsartmann/go-health/releases/tag/v0.1.0
