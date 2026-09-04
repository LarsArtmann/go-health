# Roadmap

> Long-term direction and raw ideas. Items here are NOT actionable tasks.
> When an idea is refined into bounded work, it moves to TODO_LIST.md.
>
> Pruned 2026-09-04 after the Pareto master-plan execution: ideas shipped in
> v0.1.0 moved to FEATURES.md/CHANGELOG.md; ideas decided against now live in
> the Non-goals section with design-note links.

## Themes

### 1. Programmatic Health API — SHIPPED in v0.1.0

`Status()`, `Alive()`, `Ready()`, `AwaitReady(ctx)`, `Healthz()`,
`NewWithHealthCheck(fn, opts...)`, and `Probe.HealthCheck` conformance all
shipped. Remaining raw ideas:

- Examples: custom `HealthRecorder`, two-phase shutdown, live-vs-cached mode
  (patterns exist in tests/docs; promoted examples are tracked in TODO_LIST)

### 2. Observability & Diagnostics

The library deliberately has zero logging coupling and no metrics export.
`WithEvaluationHook` (shipped) is the observability seam; formats are a
composition concern (see [prometheus-exposition-design.md](docs/prometheus-exposition-design.md)).

Raw ideas:

- OpenTelemetry spans on Evaluate/checks (same seam: hook + trace context)
- `Response.TotalLatencyMs` as `float64` for sub-millisecond precision
- ~~Per-service latency in `Check`~~ — infeasible in core, `samber/do` owns
  the batch; see [classification-2.0-design.md](docs/classification-2.0-design.md) §4

### 3. Operational Hardening

Shipped: `WithLiveThrottle` (request-flood coalescing) and
`WithShutdownGracePeriod` (two-phase shutdown timing).

Decided against (see [classification-2.0-design.md](docs/classification-2.0-design.md)):
circuit-breaker in core, `WithMaxConcurrentChecks`, per-service result
caching. Escape hatch for all three: a custom `HealthRecorder` owns batch
execution.

### 4. Container & Ecosystem Integration

Shipped: `Probe.HealthCheck` (do.HealthcheckerWithContext self-registration),
`AsShutdowner` / `ProbeShutdowner` (do.ShutdownerWithError).

Raw ideas:

- Remove `do.Injector` from the `HealthRecorder` interface signature —
  deferred: ADR-004 froze the interface for v0.x; revisit at v1.0

Decided against (see [multi-tenant-design.md](docs/multi-tenant-design.md)):
`WithCriticalService` runtime toggle, child-scope isolation (N probes +
aggregate), `WithProbeName` (aggregate namespacing + `WithInstanceID` cover it).

### 5. Protocol & Format Flexibility

Shipped: `Response.InstanceID` + `WithInstanceID`, static OpenAPI spec
([docs/openapi.yaml](docs/openapi.yaml)), Prometheus exposition via composition
([docs/prometheus-exposition-design.md](docs/prometheus-exposition-design.md)).

Decided against (see [starting-status-design.md](docs/starting-status-design.md)):
"starting" Status, Status validation (no injection boundary exists).

### 6. Testability & Internal Architecture — SHIPPED in v0.1.0

`classifier` extraction, `ResetStartupLatchForTest` (test builds only; the
public latch stays one-way), `WithNowFunc` clock seam (deterministic uptime,
timestamps, and throttle windows), `WithAllowedMethods` method-set guard.
Middleware: no library concept needed — handlers are plain `http.HandlerFunc`
([middleware-design.md](docs/middleware-design.md)).

## Non-goals

Things we are deliberately NOT pursuing and why:

- **Per-service timeout within this library:** samber/do already provides
  `do.WithHealthCheckTimeout`. This library controls only the outer batch
  deadline. See [docs/timeout-design.md](docs/timeout-design.md).
- **Direct logging:** Libraries must not make logging decisions for the host
  application. A `WithLogger` option was considered and rejected as
  anti-pattern coupling.
- **Error library adoption:** Sentinel errors via stdlib `errors.New` /
  `fmt.Errorf` are sufficient for config-validation errors matched via
  `errors.Is`. See AGENTS.md "Stdlib errors by design".
- **Non-stdlib serialization:** Responses serialize through the stdlib
  `encoding/json/v2` package. Third-party serialization stays out of scope.
- **Content negotiation / HTML rendering:** formats are a composition
  concern. See [docs/content-negotiation-design.md](docs/content-negotiation-design.md).
- **Health-check weights, in-probe circuit breakers, per-service caching,
  batch concurrency limits:** analyzed and rejected;
  see [docs/classification-2.0-design.md](docs/classification-2.0-design.md).
- **Multi-tenant primitives (child scopes, probe names, runtime criticality
  toggles):** N probes + health/aggregate composition covers the topology;
  see [docs/multi-tenant-design.md](docs/multi-tenant-design.md).
- **Fourth Status value ("starting") and Status input validation:**
  see [docs/starting-status-design.md](docs/starting-status-design.md).
