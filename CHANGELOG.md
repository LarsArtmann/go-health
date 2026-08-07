# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Changed (breaking)

- `Probe.Start(ctx)` now returns `error` instead of being void. It calls `Validate()` and returns `ErrInvalidTimeout` or `ErrInvalidRefreshInterval` on invalid configuration. Callers should check the return value.

### Changed

- Options now write to a construction-only `config` struct instead of the `Probe` directly. The `Probe` struct no longer carries a `recorder` field at all — the recorder reference lives only in the `healthCheck` closure.
- Converted `resolveHealthCheck` from a method to a free function — it is construction-only behavior, not ongoing Probe behavior.
- Reverted `slog.Debug` from `writeResponse` — libraries must not make logging decisions for the host application. HTTP write failures (client disconnect) are silently swallowed.
- Rewrote `WithTimeout` doc comment to clarify that the deadline is batch-level and shared across all services, not per-service.
- Added "Timeouts" section to package doc explaining the relationship between `WithTimeout` (batch-level) and `do.WithHealthCheckTimeout` (per-service).

### Added

- Panic recovery in `runHealthChecks` — panics from misbehaving recorders or services are recovered and reported as a synthetic `health-check` error entry instead of crashing the process.
- `flake.nix` with devShell, treefmt, and `nix run .#test` / `.#lint` / `.#coverage` / `.#vulncheck` / `.#security` commands.
- `.golangci.yml` with enforced linter configuration (gosec, staticcheck, govet, revive, errcheck, and more).
- Expanded `README.md` with configuration reference, troubleshooting guide, and development setup.
- `StartupHandler` benchmark and recorder-path benchmark.
- Test asserting 405 response body contains the actionable "health probes only accept GET" message.
- Test asserting `Shutdown()` without prior `Start()` does not panic.
- Test asserting panic recovery from misbehaving recorder.
- Test asserting startup latch does not flip when a slow critical service times out.
- Test asserting `refreshLoop` ticker fires before shutdown.
- Test asserting JSON `checks` map is alphabetically sorted (deterministic output).
- Test asserting `Start()` returns error on invalid config.

## [0.1.0] - 2026-01-01

### Added

- Three-probe pattern: liveness (`/healthz`), readiness (`/readyz`), startup (`/startupz`).
- Critical/non-critical service classification via `WithCriticalServices`.
- Background caching with configurable refresh interval (`WithRefreshInterval`).
- Batch-level timeout (`WithTimeout`).
- Shutdown awareness: `Shutdown()` and `MarkShuttingDown()` for two-phase graceful drain.
- GET-only enforcement (`WithGETOnly`).
- Optional `HealthRecorder` integration (e.g. `samber-do-auditlog.Plugin`).
- Construction-time capability resolution — Probe holds a resolved `healthCheckFunc`, never the `do.Injector`.
- Config validation via `Validate()` with enriched sentinel errors.
- 97%+ test coverage with race-detector-clean concurrency tests and benchmarks.
