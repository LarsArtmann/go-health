# AGENTS.md — go-health

Standalone Kubernetes health-probe SDK for samber/do v2. Three-probe pattern (liveness, readiness, startup) with critical/non-critical classification, background caching, and shutdown awareness.

**Module**: `github.com/larsartmann/go-health` · **Packages**: `health`, `health/aggregate` · **Go**: 1.26.7 · **Status**: v0.1.1 (alpha)

---

## Commands

| Command               | Purpose                          |
| --------------------- | -------------------------------- |
| `nix run .#test`      | Run all tests                    |
| `nix run .#test-race` | Run all tests with race detector |
| `nix run .#lint`      | Run golangci-lint                |
| `nix run .#vet`       | Run go vet                       |
| `nix run .#coverage`  | Run tests with coverage report   |
| `nix run .#fuzz`      | Run fuzz targets (short budget)  |
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
doc.go           — Package doc comment (quick start, three-probe rationale, caching, shutdown)
types.go         — Status enum (pass/fail/warn, frozen), Check, Response data model (incl. instance_id, timestamp omitzero)
probe.go         — Probe struct, config struct, 13 Option functional options (write to config), HealthRecorder interface, New(), resolveHealthCheck (free function), Validate(), lifecycle (Start/Shutdown/MarkShuttingDown), guard (method-set enforcement), now/uptime clock seam, Evaluate, CachedResponse (lock-free read + shutdown overlay), accessors, runHealthChecks (with panic recovery), buildChecks
classifier.go    — Read-only classifier: classify (three-state), evaluateStartup, per-check grading; constructed once, evaluated lock-free
handlers.go      — LivenessHandler, ReadinessHandler, StartupHandler, RegisterRoutes, Routes, DefaultRoutes, readinessResponse/throttledLiveResponse, writeResponse + SanitizeResponse (UTF-8 coercion)
accessors.go     — ErrProbeUnhealthy, HealthCheckFunc, NewWithHealthCheck, Status/Alive/Ready, AwaitReady, HealthCheck (do conformance), ProbeShutdowner/AsShutdowner, Healthz
export_test.go   — ResetStartupLatchForTest (test builds only; public latch stays one-way)
```

Sub-package `aggregate` (source: `aggregate/aggregate.go`) merges N in-process probes into one
`health.Response`: `Source{Name, Probe}`, `New(sources...) (*Aggregate, error)` (rejects empty,
duplicate, or nil sources), `CachedResponse` (merge-on-read: N lock-free loads, worst-of status,
`"source/check"` namespacing, shutdown overlay, max latency), `RefreshInterval` (slowest source),
`StartupComplete` (AND of latches), the three kubelet handlers (liveness 200, readiness 503 on
fail, startup 503 until all latches), `RegisterRoutes`.

### Key Design Decisions

- **Liveness never checks dependencies** — returns in microseconds, always 200. Prevents restart cascades.
- **Readiness gates on critical services only** — non-critical failures set roll-up to `warn` (HTTP 200, degraded), critical failures set it to `fail` (HTTP 503).
- **Startup latches** — once all critical services pass, always returns 200 without re-checking.
- **Background caching by default** (1s refresh) — kubelet/LB polling doesn't hammer dependencies. Set `WithRefreshInterval(0)` for live mode.
- **Shutdown-aware** — `Shutdown()` flips readiness to 503 immediately (even from stale cache); liveness stays 200.
- **Method-set enforcement** — `WithAllowedMethods(...)` wraps all handlers: non-allowed methods get 405 with a sorted `Allow` header (GET always included; duplicates collapse). Off by default. `WithGETOnly()` is **deprecated** (v0.1.1) but still functional — it is the zero-arg equivalent; keep its tests until removal is decided. Middleware composes outside this guard (see docs/middleware-design.md).
- **Deterministic clock seam** — `p.now()` (backed by `WithNowFunc`) drives uptime, `Response.Timestamp`, and live-throttle freshness. Latency measurement stays on the real clock. Tests inject a fixed clock instead of sleeping.
- **HealthRecorder interface** — replaces the old concrete `*auditlog.Plugin` dependency. Any type with `RecordHealthCheckWithContext(ctx, injector) map[string]error` satisfies it. `samber-do-auditlog.Plugin` implements it implicitly.
- **Three-state classify** — `classify` returns `pass` (all healthy), `warn` (only non-critical failures), or `fail` (critical failure or shutting down).
- **`aggregate` is passive and lock-free by construction** — merge-on-read: every read performs
  one atomic `CachedResponse` load per source. No goroutines, no scheduler, no staleness of its
  own; freshness is bounded by the slowest source's refresh interval. Scalars (`Version`,
  `Uptime`) deliberately do not survive a merge — they are per-process and would lie in an
  aggregate view.
- **Stdlib errors by design** — sentinels (`ErrInvalidTimeout`, `ErrInvalidRefreshInterval`,
  `aggregate.ErrNoSources`, `aggregate.ErrInvalidSource`) use `errors.New`; `Validate()` wraps them with `fmt.Errorf("%w: ...")` to include the offending value and remediation. No error library (samber/oops, go-error-family, cockroachdb/errors) is adopted: this is a single-dependency library whose only Go-level errors are config-validation sentinels matched via `errors.Is`, not errors at an HTTP/CLI boundary that need classification. HTTP failures are communicated via status codes, not error returns.
- **Injector resolved at construction** — `New` captures the health-check capability into a `healthCheckFunc` via the `resolveHealthCheck` free function. Options write to a construction-only `config` struct, not the `Probe` directly. The Probe never stores `do.Injector` or `HealthRecorder` as fields, avoiding the injector-in-service anti-pattern (DO-6) and eliminating dead construction-only fields.
- **Panic recovery in health checks, fail closed** — `runHealthChecks` recovers panics from the recoverable surface (recorder implementations, batch machinery) and converts them to a synthetic `health-check` error wrapping `ErrPanicDuringHealthCheck`; `classify` maps any recovered panic to `fail` (503), never `warn`. Service `HealthCheck` panics on the injector path are process-fatal: samber/do runs each check in its own goroutine, so no probe-side recover can catch them. See [docs/panic-recovery-design.md](docs/panic-recovery-design.md).
- **Validate-on-Start** — `Start()` calls `Validate()` and returns an error on invalid configuration (zero/negative timeout, negative refresh interval). Fail-fast instead of silent runtime degradation.
- **Zero logging coupling** — the library does not import `log/slog` or any logging package. HTTP write failures (client disconnect) are silently swallowed. A library must not make logging decisions for the host application.
- **Observability via hook, not library** — `WithEvaluationHook` is the metrics/alerting seam; Prometheus/OpenTelemetry formats are consumer composition (docs/prometheus-exposition-design.md). No client_golang dependency.
- **Programmatic API mirrors handlers** — `Status/Alive/Ready/AwaitReady` read the cached view (never trigger checks); `Healthz` answers "route traffic here?"; `HealthCheck`/`AsShutdowner` make the probe a first-class do citizen.

### Decoupling from samber-do-auditlog

This package was extracted from [`samber-do-auditlog`](https://github.com/larsartmann/samber-do-auditlog) to eliminate the transitive dependency cost. The old `WithPlugin(p *auditlog.Plugin)` option is now `WithHealthRecorder(r HealthRecorder)`. The `*auditlog.Plugin` type implicitly satisfies `HealthRecorder` via its `RecordHealthCheckWithContext` method — pass it directly.

Migration guide for pre-extraction code: [docs/migration-plugin-to-recorder.md](docs/migration-plugin-to-recorder.md).

**doanalyzerv2:** the private `branching-flow/pkg/doanalyzerv2` AST analyzer
(persisted in-repo as the `tools/doanalyzerv2` replace-module runner,
sidestepping the nix-sandbox `go install` block) reports 0 findings for
DO-1..DO-6 across all source files. Invoke with
`(cd tools/doanalyzerv2 && go run . ..)`; it requires the go-design-smells
checkout at `/home/lars/projects/branching-flow` (the replace path in
`tools/doanalyzerv2/go.mod`).

**Consumer verification:** `samber-do-auditlog` does NOT import go-health (post-extraction, dependency-free both ways); the only known consumer is [`go-health-dashboard`](https://github.com/larsartmann/go-health-dashboard), which requires released v0.1.1 directly (NO replace directive since ~2026-09-04 — the older "pinned below v0.1.1 via replace" note is stale). Re-verified 2026-09-04 post-fix: dashboard build + vet + full test suite green against released v0.1.1, and build + tests green against go-health HEAD (temporary replace directive, restored after; HEAD incl. the instance_id sanitize fix and the aggregate slash-name validation). Consumers building against go-health need `GOEXPERIMENT=jsonv2` (json/v2 is behind the experiment on go1.26). Any public API change must be coordinated with the dashboard consumer.

### Data Flow

1. User creates `Probe` via `New(injector, opts...)` — options write to a `config` struct, consumed and discarded at construction
2. `Start(ctx)` validates config and optionally launches background cache refresh loop
3. HTTP handlers serve cached or live health-check results
4. Readiness/startup delegate to `runHealthChecks` (with panic recovery), which calls the `healthCheckFunc` resolved by the `resolveHealthCheck` free function at construction (recorder or raw injector)
5. `Shutdown()` marks probe as shutting down (readiness → 503, liveness stays 200)

### Concurrency Model

- `shuttingDown` and `startupPassed` are `atomic.Bool` — no mutex needed.
- `latest` is `atomic.Pointer[Response]` — lock-free cache reads.
- `mu` protects `cancel` and serializes WaitGroup Add/Wait (lifecycle race fix, 2026-09-04).
- `throttleMu` serializes throttled live evaluations; the `classifier` is read-only after construction (no lock on the evaluate path).
- All handlers are safe for concurrent use.

---

## Testing Patterns

- Standard `testing.T` + table-driven tests. No ginkgo/testify.
- Each test creates its own `do.Injector` — no shared state.
- `mockRecorder` type replaces the old auditlog integration tests.
- Benchmarks: `LivenessHandler`, `ReadinessHandler_CacheHit`, `ReadinessHandler_LiveEval`, `ReadinessHandler_RecorderPath`, `StartupHandler_Unlatched`, `StartupHandler_Contention`, `CachedResponse_ParallelReads`, `Evaluate`, `GuardOverhead`.
- Fuzz targets: `FuzzResponseMarshalDeterministic` + `FuzzHandlerInput` (root), `FuzzAggregateMergeInvariants` (aggregate); run via `nix run .#fuzz`.
- **Seam-swap tests must not be parallel** — tests swapping a package-global
  seam (`marshalResponse` in both packages) mutate shared state, so they omit
  `t.Parallel()` (marked `//nolint:paralleltest`). A parallel seam test
  corrupts concurrent handler tests (observed: spurious 500s).

---

## Gotchas

- **samber/do v2.1.0 behavior** — never-invoked lazy services appear in `HealthCheckWithContext` results with nil error. Eagerly invoke critical services at boot for the startup probe to be meaningful.
- **Three-state classify** — `classify` returns `pass`/`warn`/`fail`. The readiness handler maps only `fail` to HTTP 503; `warn` and `pass` both return 200.
- **Config validation** — `Probe.Validate()` checks `timeout > 0` and `refreshInterval >= 0`. `Start()` calls `Validate()` and returns an error on invalid config — callers should check the error from `Start()`.
- **GOEXPERIMENT=jsonv2 IS required since the jsonv2 migration** — `handlers.go` (and
  `aggregate/aggregate.go`) import `encoding/json/v2`, which go1.26 only exposes behind
  `GOEXPERIMENT=jsonv2` (verified: `env -u GOEXPERIMENT go build ./...` fails with "build
  constraints exclude all Go files in encoding/json/v2"; `env GOEXPERIMENT=jsonv2` builds).
  An older revision of this file claimed no GOEXPERIMENT was needed — that claim was an
  artifact of the host shell leaking `GOEXPERIMENT=jsonv2` into every nix invocation.
  The flake now sets it explicitly in every app and the devShell, so the gates are
  hermetic; only bare `go` commands outside the flake need it manually. gopls' stdversion
  warning ("json.Marshal requires go1.27") is expected and benign while the experiment is
  enabled — it reflects the stabilized json/v2 landing in go1.27, not a real build failure.
  Set `GOWORK=off` to avoid workspace interference.
- **Tools that shell out to `go` need `goPkg` in their flake app** —
  `golangci-lint`, `govulncheck`, and `gosec` load packages by invoking a `go`
  binary from PATH. Their apps originally declared only the tool, so CI (no Go
  on PATH) fell back to the GOROOT the binary was compiled with — an older Go
  that hard-fails on `GOEXPERIMENT=jsonv2` ("unknown GOEXPERIMENT jsonv2").
  First CI run caught it; the fix puts `goPkg` in every such app's
  `runtimeInputs` (verified by running the gates with the host Go removed
  from PATH). Rule of thumb: any new flake app that indirectly runs `go` must
  list `goPkg` — the same leak class as the GOEXPERIMENT gotcha above, one
  layer down.
- **`encoding/json/v2` does not sort map keys by default** — under v2 semantics `json.Marshal` serializes maps in random Go map order unless `json.Deterministic(true)` is passed (v1's always-sorted behavior was a compatibility default, not a v2 one). `writeResponse` opts in (handlers.go); `TestReadiness_JSONChecksAreSortedAlphabetically` guards the property. Any new marshal site must pass the option too.
- **erraudit enforcement flags are opt-in** — `--enforce-samber-oops` and `--enforce-go-error-family` flag stdlib constructors (`errors.New`, `fmt.Errorf`) as violations. These flags are for projects that have already adopted those libraries. This project deliberately uses stdlib errors, so the correct invocation is `erraudit ./... --type-aware` (reports 0 ERROR violations). Do not cargo-cult a library adoption to silence the linter — the sentinels are config-validation errors, not boundary errors needing classification.
- **`WithTimeout` is batch-level, not per-service** — the deadline is shared across all services in one evaluation. A slow dependency steals time from every other check. samber/do exposes `HealthCheckTimeout` (per-service) via `InjectorOpts` at injector creation time. See [docs/timeout-design.md](docs/timeout-design.md) for the full analysis, including why HTTP query-param timeout overrides are rejected (DoS amplifier + breaks caching).
- **`aggregate` sources must be eagerly invoked too** — the samber/do lazy-service gotcha applies per source: a source probe whose services were never invoked health-checks as pass, and the aggregate propagates that false confidence. Invoke critical services at boot.

---

## Project Documentation

| File                                                                                      | Purpose                                                                                                                       |
| ----------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| [FEATURES.md](FEATURES.md)                                                                | Honest feature inventory by status                                                                                            |
| [TODO_LIST.md](TODO_LIST.md)                                                              | Short-term actionable tasks                                                                                                   |
| [ROADMAP.md](ROADMAP.md)                                                                  | Long-term direction and raw ideas                                                                                             |
| [CHANGELOG.md](CHANGELOG.md)                                                              | What changed in each version                                                                                                  |
| [docs/DOMAIN_LANGUAGE.md](docs/DOMAIN_LANGUAGE.md)                                        | Domain terms (liveness, readiness, startup, critical, etc.)                                                                   |
| [docs/timeout-design.md](docs/timeout-design.md)                                          | Batch-level vs per-service timeout analysis                                                                                   |
| [docs/aggregate-source-name-design.md](docs/aggregate-source-name-design.md)              | Slash rejected in aggregate source names (G2): collision + grouping-axis rationale                                            |
| [docs/panic-recovery-design.md](docs/panic-recovery-design.md)                            | Panic criticality decision: fail closed; recoverable vs process-fatal surfaces                                                |
| [docs/middleware-design.md](docs/middleware-design.md)                                    | Why handlers stay plain `http.HandlerFunc`; middleware composes outside the guard                                             |
| [docs/prometheus-exposition-design.md](docs/prometheus-exposition-design.md)              | Metrics via `WithEvaluationHook` composition, never `client_golang`                                                           |
| [docs/openapi-design.md](docs/openapi-design.md) + [docs/openapi.yaml](docs/openapi.yaml) | Static OpenAPI 3.1 spec over runtime generation                                                                               |
| [docs/classification-2.0-design.md](docs/classification-2.0-design.md)                    | Rejected: weights, circuit-breaker, MaxConcurrent, per-service caching                                                        |
| [docs/multi-tenant-design.md](docs/multi-tenant-design.md)                                | Rejected: child scopes, `WithProbeName`, runtime criticality toggles                                                          |
| [docs/starting-status-design.md](docs/starting-status-design.md)                          | Rejected: fourth Status value, Status input validation                                                                        |
| [docs/content-negotiation-design.md](docs/content-negotiation-design.md)                  | Why content negotiation / HTML rendering is rejected; composition pattern instead                                             |
| [docs/migration-plugin-to-recorder.md](docs/migration-plugin-to-recorder.md)              | `WithPlugin` → `WithHealthRecorder` migration for pre-extraction consumers                                                    |
| [docs/deprecation-policy.md](docs/deprecation-policy.md)                                  | Deprecation checklist, symbol lifetime, SA1019 stance                                                                         |
| [SECURITY.md](SECURITY.md)                                                                | Vulnerability disclosure path, in/out of scope, response targets                                                              |
| [docs/adr/](docs/adr/)                                                                    | Architecture decision records: stdlib errors (001), zero logging (002), three-state classify (003), recorder decoupling (004) |
| [docs/status/](docs/status/)                                                              | Historical session reports (point-in-time snapshots); fully-resolved reports move to `docs/status/archived/`                  |
