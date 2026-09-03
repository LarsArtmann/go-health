# Roadmap

> Long-term direction and raw ideas. Items here are NOT actionable tasks.
> When an idea is refined into bounded work, it moves to TODO_LIST.md.

## Themes

### 1. Programmatic Health API

Today the library exposes health exclusively through HTTP handlers. A programmatic
API would let non-HTTP consumers (CLIs, background workers, test harnesses) query
health state without spinning up an HTTP server.

Raw ideas:

- `Probe.Status() Status` — roll-up status without HTTP overhead
- `Probe.Alive() bool` / `Probe.Ready() bool` — convenience booleans
- `Probe.AwaitReady(ctx)` — blocking helper for startup orchestration
- `Probe.Healthz()` — single combined endpoint for simpler deployments
- Export `healthCheckFunc` and add `NewWithHealthCheck(fn, opts...)` for injector-free testing

### 2. Observability & Diagnostics

The library deliberately has zero logging coupling and no metrics export. As the
library matures, structured hooks could let host applications observe internals
without the library making observability decisions.

Raw ideas:

- Per-service latency in `Check` struct (currently batch-level only)
- `Response.Timestamp` for when the check was run
- Metrics integration hooks (Prometheus exposition, OpenTelemetry spans)
- `Response.TotalLatencyMs` as `float64` for sub-millisecond precision
- Health-check weights or priorities for nuanced classification beyond binary critical/non-critical

### 3. Operational Hardening

The current design assumes a well-behaved kubelet polling at reasonable intervals.
Real-world deployments may benefit from additional protective mechanisms.

Raw ideas:

- Debounce/throttle for live evaluation mode (`WithRefreshInterval(0)` has no DOS protection)
- `WithShutdownGracePeriod(d)` for automatic two-phase shutdown timing
- Circuit-breaker pattern for flapping dependencies (failsafe-go or similar)
- `WithMaxConcurrentChecks(n)` for limiting parallelism within a batch
- Per-service health-check result caching (not just batch-level)

### 4. Container & Ecosystem Integration

The Probe monitors a `samber/do` injector but does not participate in the
container lifecycle itself. Deeper integration would enable self-registration and
container-managed health.

Raw ideas:

- Implement `do.HealthcheckerWithContext` on Probe for self-registration
- Implement `do.ShutdownerWithError` on Probe for container-managed lifecycle
- Remove `do.Injector` from the `HealthRecorder` interface signature (the current `RecordHealthCheckWithContext(ctx, injector)` shape forces every consumer to know the container type, even after the Probe itself stopped holding one)
- `WithCriticalService(name string, critical bool)` for per-service toggle at runtime
- Child-scope isolation for multi-tenant health checks
- `WithProbeName(string)` for multi-probe setups in one process

### 5. Protocol & Format Flexibility

Currently the library hardcodes JSON output with a fixed `Response` shape. Some
deployments may need alternative formats or stricter contracts.

Raw ideas:

- Custom response formats (e.g., Prometheus exposition, plain text)
- `Response.InstanceID` for multi-replica identification
- OpenAPI schema generation for the health response
- `Status` validation (reject unknown values at construction)
- A "starting" `Status` distinct from pass/warn/fail for finer-grained boot state

### 6. Testability & Internal Architecture

The classify and evaluate logic is coupled to the Probe type. Extracting it would
improve testability and enable alternative classification strategies.

Raw ideas:

- Extract `classify` and `evaluateStartup` into a separate `classifier` type
- `Probe.ResetStartupLatch()` for testing (force re-evaluation)
- `WithNowFunc(func() time.Time)` for testable uptime calculations
- `WithAllowedMethods(...string)` instead of boolean `WithGETOnly()`
- HTTP middleware support for auth/rate-limiting on probe endpoints

## Non-goals

Things we are deliberately NOT pursuing and why:

- **Per-service timeout within this library:** samber/do already provides `do.WithHealthCheckTimeout`. This library controls only the outer batch deadline. Duplicating per-service logic here adds complexity for no benefit.
- **Direct logging:** Libraries must not make logging decisions for the host application. Observability should be injected, not hardcoded. A `WithLogger` option was considered and rejected as anti-pattern coupling.
- **Error library adoption:** Sentinel errors via stdlib `errors.New` / `fmt.Errorf` are sufficient for config-validation errors matched via `errors.Is`. Adopting oops, go-error-family, or cockroachdb/errors adds a dependency for marginal value.
- **Non-stdlib serialization:** Responses serialize through the stdlib `encoding/json/v2` package (migrated from `encoding/json` in v0.0.2+). Third-party JSON or serialization libraries stay out of scope.
