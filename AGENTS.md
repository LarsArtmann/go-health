# AGENTS.md — go-health

Standalone Kubernetes health-probe SDK for samber/do v2. Three-probe pattern (liveness, readiness, startup) with critical/non-critical classification, background caching, and shutdown awareness.

**Module**: `github.com/larsartmann/go-health` · **Package**: `health` · **Go**: 1.26.5 · **Status**: ALPHA

---

## Commands

| Command               | Purpose                                         |
| --------------------- | ----------------------------------------------- |
| `go test ./...`       | Run all tests                                   |
| `go test -race ./...` | Run all tests with race detector                |
| `go vet ./...`        | Static analysis                                 |
| `go mod tidy`         | Sync go.sum                                     |

No Makefile, no justfile, no flake.nix (yet). Single dependency: `github.com/samber/do/v2 v2.1.0`.

---

## Architecture

Single-package library (`health`) with these source files:

```
doc.go      — Package doc comment (quick start, three-probe rationale, caching, shutdown)
types.go    — Status enum (pass/fail/warn), Check, Response data model
probe.go    — Probe struct, 7 Option functional options, HealthRecorder interface, New(), Validate(), lifecycle (Start/Shutdown/MarkShuttingDown), Evaluate, classify (three-state), evaluateStartup, buildChecks
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

### Decoupling from samber-do-auditlog

This package was extracted from [`samber-do-auditlog`](https://github.com/larsartmann/samber-do-auditlog) to eliminate the transitive dependency cost. The old `WithPlugin(p *auditlog.Plugin)` option is now `WithHealthRecorder(r HealthRecorder)`. The `*auditlog.Plugin` type implicitly satisfies `HealthRecorder` via its `RecordHealthCheckWithContext` method — pass it directly.

### Data Flow

1. User creates `Probe` via `New(injector, opts...)`
2. `Start(ctx)` optionally launches background cache refresh loop
3. HTTP handlers serve cached or live health-check results
4. Readiness/startup delegate to `runHealthChecks` which uses `HealthRecorder` when available, raw injector otherwise
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
- Benchmarks: LivenessHandler, ReadinessHandler (cache hit + live eval), Evaluate.

---

## Gotchas

- **samber/do v2.1.0 behavior** — never-invoked lazy services appear in `HealthCheckWithContext` results with nil error. Eagerly invoke critical services at boot for the startup probe to be meaningful.
- **Three-state classify** — `classify` returns `pass`/`warn`/`fail`. The readiness handler maps only `fail` to HTTP 503; `warn` and `pass` both return 200.
- **Config validation** — `Probe.Validate()` checks `timeout > 0` and `refreshInterval >= 0`. Not enforced in `New()` (no API change); callers call it explicitly for early misconfiguration detection.
- **No GOEXPERIMENT=jsonv2 needed** — this package only depends on `samber/do/v2` + stdlib. No templ, no go-output, no SSE infrastructure.
