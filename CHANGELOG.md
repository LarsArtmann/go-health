# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Changed

- Removed dead `recorder` field from `Probe` after construction-time resolution (GC-friendly; recorder reference lives only in the `healthCheck` closure).
- Converted `resolveHealthCheck` from a method to a free function — it is construction-only behavior, not ongoing Probe behavior.
- Reverted `slog.Debug` from `writeResponse` — libraries must not make logging decisions for the host application. HTTP write failures (client disconnect) are silently swallowed.
- Rewrote `WithTimeout` doc comment to clarify that the deadline is batch-level and shared across all services, not per-service.
- Added "Timeouts" section to package doc explaining the relationship between `WithTimeout` (batch-level) and `do.WithHealthCheckTimeout` (per-service).

### Added

- Test asserting 405 response body contains the actionable "health probes only accept GET" message.
- Test asserting `Shutdown()` without prior `Start()` does not panic.

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
