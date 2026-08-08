# AGENTS.md — go-health

Standalone Kubernetes health-probe SDK for samber/do v2. Three-probe pattern (liveness, readiness, startup) with critical/non-critical classification, background caching, and shutdown awareness.

**Module**: `github.com/larsartmann/go-health` · **Package**: `health` · **Go**: 1.26.5 · **Status**: v0.0.1 (alpha)

---

## Commands

| Command               | Purpose                          |
| --------------------- | -------------------------------- |
| `nix run .#test`      | Run all tests                    |
| `nix run .#test-race` | Run all tests with race detector |
| `nix run .#lint`      | Run golangci-lint                |
| `nix run .#vet`       | Run go vet                       |
| `nix run .#coverage`  | Run tests with coverage report   |
| `nix run .#vulncheck` | Run govulncheck                  |
| `nix run .#security`  | Run gosec                        |
| `nix run .#build`     | Build all packages               |
| `nix fmt`             | Format code (gofumpt, goimports) |
| `nix flake check`     | Validate flake + formatting      |

Uses `flake.nix` with `flake-parts` + `treefmt-nix`. Single dependency: `github.com/samber/do/v2 v2.1.0`.

---

## Architecture

Single-package library (`health`) with these source files:

```
doc.go      — Package doc comment (quick start, three-probe rationale, caching, shutdown)
types.go    — Status enum (pass/fail/warn), Check, Response data model
probe.go    — Probe struct, config struct, 7 Option functional options (write to config), HealthRecorder interface, New(), resolveHealthCheck (free function), Validate(), lifecycle (Start returns error/Shutdown/MarkShuttingDown), Evaluate, runHealthChecks (with panic recovery), classify (three-state), evaluateStartup, buildChecks
handlers.go — LivenessHandler, ReadinessHandler, StartupHandler, RegisterRoutes, Routes, DefaultRoutes, writeResponse
```

### Key Design Decisions

- **Liveness never checks dependencies** — returns in microseconds, always 200. Prevents restart cascades.
- **Readiness gates on critical services only** — non-critical failures set roll-up to `warn` (HTTP 200, degraded), critical failures set it to `fail` (HTTP 503).
- **Startup latches** — once all critical services pass, always returns 200 without re-checking.
- **Background caching by default** (1s refresh) — kubelet/LB polling doesn't hammer dependencies. Set `WithRefreshInterval(0)` for live mode.
- **Shutdown-aware** — `Shutdown()` flips readiness to 503 immediately (even from stale cache); liveness stays 200.
- **GET-only enforcement** — `WithGETOnly()` wraps all handlers to reject non-GET with 405 + `Allow: GET` header. Off by default.
- **HealthRecorder interface** — replaces the old concrete `*auditlog.Plugin` dependency. Any type with `RecordHealthCheckWithContext(ctx, injector) map[string]error` satisfies it. `samber-do-auditlog.Plugin` implements it implicitly.
- **Three-state classify** — `classify` returns `pass` (all healthy), `warn` (only non-critical failures), or `fail` (critical failure or shutting down).
- **Stdlib errors by design** — sentinels (`ErrInvalidTimeout`, `ErrInvalidRefreshInterval`) use `errors.New`; `Validate()` wraps them with `fmt.Errorf("%w: ...")` to include the offending value and remediation. No error library (samber/oops, go-error-family, cockroachdb/errors) is adopted: this is a single-dependency library whose only Go-level errors are config-validation sentinels matched via `errors.Is`, not errors at an HTTP/CLI boundary that need classification. HTTP failures are communicated via status codes, not error returns.
- **Injector resolved at construction** — `New` captures the health-check capability into a `healthCheckFunc` via the `resolveHealthCheck` free function. Options write to a construction-only `config` struct, not the `Probe` directly. The Probe never stores `do.Injector` or `HealthRecorder` as fields, avoiding the injector-in-service anti-pattern (DO-6) and eliminating dead construction-only fields.
- **Panic recovery in health checks** — `runHealthChecks` recovers panics from misbehaving recorders or services and converts them to synthetic errors. A panicking recorder shows up as a failed `health-check` entry instead of crashing the process.
- **Validate-on-Start** — `Start()` calls `Validate()` and returns an error on invalid configuration (zero/negative timeout, negative refresh interval). Fail-fast instead of silent runtime degradation.
- **Zero logging coupling** — the library does not import `log/slog` or any logging package. HTTP write failures (client disconnect) are silently swallowed. A library must not make logging decisions for the host application.

### Decoupling from samber-do-auditlog

This package was extracted from [`samber-do-auditlog`](https://github.com/larsartmann/samber-do-auditlog) to eliminate the transitive dependency cost. The old `WithPlugin(p *auditlog.Plugin)` option is now `WithHealthRecorder(r HealthRecorder)`. The `*auditlog.Plugin` type implicitly satisfies `HealthRecorder` via its `RecordHealthCheckWithContext` method — pass it directly.

### Data Flow

1. User creates `Probe` via `New(injector, opts...)` — options write to a `config` struct, consumed and discarded at construction
2. `Start(ctx)` validates config and optionally launches background cache refresh loop
3. HTTP handlers serve cached or live health-check results
4. Readiness/startup delegate to `runHealthChecks` (with panic recovery), which calls the `healthCheckFunc` resolved by the `resolveHealthCheck` free function at construction (recorder or raw injector)
5. `Shutdown()` marks probe as shutting down (readiness → 503, liveness stays 200)

### Concurrency Model

- `shuttingDown` and `startupPassed` are `atomic.Bool` — no mutex needed.
- `latest` is `atomic.Pointer[Response]` — lock-free cache reads.
- `mu` protects only `cancel` (background loop lifecycle).
- All handlers are safe for concurrent use.

---

## Testing Patterns

- Standard `testing.T` + table-driven tests. No ginkgo/testify.
- Each test creates its own `do.Injector` — no shared state.
- `mockRecorder` type replaces the old auditlog integration tests.
- Benchmarks: `LivenessHandler`, `ReadinessHandler_CacheHit`, `ReadinessHandler_LiveEval`, `ReadinessHandler_RecorderPath`, `StartupHandler_Unlatched`, `Evaluate`.

---

## Gotchas

- **samber/do v2.1.0 behavior** — never-invoked lazy services appear in `HealthCheckWithContext` results with nil error. Eagerly invoke critical services at boot for the startup probe to be meaningful.
- **Three-state classify** — `classify` returns `pass`/`warn`/`fail`. The readiness handler maps only `fail` to HTTP 503; `warn` and `pass` both return 200.
- **Config validation** — `Probe.Validate()` checks `timeout > 0` and `refreshInterval >= 0`. `Start()` calls `Validate()` and returns an error on invalid config — callers should check the error from `Start()`.
- **No GOEXPERIMENT=jsonv2 needed** — this package only depends on `samber/do/v2` + stdlib. No templ, no go-output, no SSE infrastructure.
- **erraudit enforcement flags are opt-in** — `--enforce-samber-oops` and `--enforce-go-error-family` flag stdlib constructors (`errors.New`, `fmt.Errorf`) as violations. These flags are for projects that have already adopted those libraries. This project deliberately uses stdlib errors, so the correct invocation is `erraudit ./... --type-aware` (reports 0 ERROR violations). Do not cargo-cult a library adoption to silence the linter — the sentinels are config-validation errors, not boundary errors needing classification.
- **`WithTimeout` is batch-level, not per-service** — the deadline is shared across all services in one evaluation. A slow dependency steals time from every other check. samber/do exposes `HealthCheckTimeout` (per-service) via `InjectorOpts` at injector creation time. See [docs/timeout-design.md](docs/timeout-design.md) for the full analysis, including why HTTP query-param timeout overrides are rejected (DoS amplifier + breaks caching).

---

## Project Documentation

| File                                                                     | Purpose                                                                           |
| ------------------------------------------------------------------------ | --------------------------------------------------------------------------------- |
| [FEATURES.md](FEATURES.md)                                               | Honest feature inventory by status                                                |
| [TODO_LIST.md](TODO_LIST.md)                                             | Short-term actionable tasks                                                       |
| [ROADMAP.md](ROADMAP.md)                                                 | Long-term direction and raw ideas                                                 |
| [CHANGELOG.md](CHANGELOG.md)                                             | What changed in each version                                                      |
| [docs/DOMAIN_LANGUAGE.md](docs/DOMAIN_LANGUAGE.md)                       | Domain terms (liveness, readiness, startup, critical, etc.)                       |
| [docs/timeout-design.md](docs/timeout-design.md)                         | Batch-level vs per-service timeout analysis                                       |
| [docs/content-negotiation-design.md](docs/content-negotiation-design.md) | Why content negotiation / HTML rendering is rejected; composition pattern instead |
| [docs/status/](docs/status/)                                             | Historical session reports (point-in-time snapshots)                              |
