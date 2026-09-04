# Domain Language

> Ubiquitous language for the go-health library. Terms used consistently in code,
> docs, and conversation. Based on Kubernetes probe semantics and samber/do v2.

## Glossary

| Term                  | Definition                                                                                                                                 | Where in code                          |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------- |
| Probe                 | The central type that orchestrates health checks and exposes three HTTP endpoints. Holds the resolved health-check capability.             | `probe.go:52` (`Probe` struct)         |
| Liveness              | "Is the process alive?" — trivially fast, dependency-free. Always returns 200 unless the process is dead. Prevents restart cascades.       | `handlers.go:32` (`LivenessHandler`)   |
| Readiness             | "Can this instance serve traffic?" — runs a full health-check batch, gates on critical services. Returns 503 when critical services fail.  | `handlers.go:58` (`ReadinessHandler`)  |
| Startup               | "Is the application done booting?" — evaluates critical services until all pass, then latches. Allows generous kubelet failure thresholds. | `handlers.go:86` (`StartupHandler`)    |
| Critical service      | A service whose failure causes readiness to return 503 (remove pod from rotation). Configured via `WithCriticalServices`.                  | `probe.go:97`                          |
| Non-critical service  | A service whose failure sets the roll-up to `warn` (HTTP 200, degraded) without removing the pod from rotation.                            | `probe.go:494` (`buildChecks`)         |
| Roll-up status        | The aggregate `Status` computed from all individual check results: `fail`, `warn`, or `pass`.                                              | `probe.go:401` (`classify`)            |
| Startup latch         | A one-way atomic boolean. Flips to true once all critical services pass a startup evaluation. After that, startup always returns 200.      | `probe.go:57` (`startupPassed`)        |
| Health-check batch    | A single invocation of all registered services' health checks within one context deadline.                                                 | `probe.go:382` (`runHealthChecks`)     |
| Health-check function | The resolved capability (`healthCheckFunc`) captured at construction time. Either delegates to a recorder or calls the injector directly.  | `probe.go:39`                          |
| HealthRecorder        | Interface that intercepts health-check batches for observation (e.g. audit logging). When nil, checks run against the raw injector.        | `probe.go:21`                          |
| Background cache      | An `atomic.Pointer[Response]` refreshed by a goroutine at a configurable interval. Readiness handlers serve the cached result for O(1).    | `probe.go:63` (`latest`)               |
| Cached response       | The last background-refreshed `Response`, read lock-free via `CachedResponse()`. Overlays the live shutdown state in cached and no-cache fallback paths. | `probe.go:443` (`CachedResponse`)      |
| Refresh interval      | How often the background loop re-evaluates health checks. Default 1s. Zero means live evaluation on every request.                         | `probe.go:124` (`WithRefreshInterval`) |
| Batch-level timeout   | The context deadline shared across ALL services in a single evaluation. Default 5s. Distinct from samber/do's per-service timeout.         | `probe.go:139` (`WithTimeout`)         |
| Shutdown              | Two-phase: `MarkShuttingDown()` flips readiness to 503 (start drain); `Shutdown()` stops the background loop. Liveness stays 200.          | `probe.go:325,343`                     |
| Config struct         | Construction-only struct populated by `Option` functions. Consumed by `New()` and discarded — the Probe never carries a reference.         | `probe.go:75` (`config`)               |
| Evaluate              | Public method that runs a full health-check batch and returns the aggregate `Response`. Usable outside HTTP handlers.                      | `probe.go:355`                         |
| Status                | Three-value enum: `pass` (healthy), `warn` (degraded), `fail` (unhealthy or shutting down).                                                | `types.go:4`                           |
| Check                 | Per-service health result: a `Status` and optional error message.                                                                          | `types.go:17`                          |
| Response              | Aggregate health response served by all probe handlers: status, version, uptime, shutting_down flag, total latency, checks map.            | `types.go:25`                          |
| Combined endpoint     | `Healthz()` — one handler answering "should traffic be routed here?": 503 while booting, failing, or draining; 200 otherwise.               | `accessors.go` (`Healthz`)             |
| Programmatic accessors| `Status()`, `Alive()`, `Ready()`, `AwaitReady(ctx)` — read the cached view; never trigger dependency checks.                                | `accessors.go`                         |
| Live throttle         | `WithLiveThrottle(d)` coalesces live-mode request floods: one evaluation per window, stored result served in between.                       | `handlers.go` (`throttledLiveResponse`)| 
| Evaluation hook       | `WithEvaluationHook(fn)` — synchronous observer of every classified response; the metrics/alerting seam.                                    | `probe.go` (`WithEvaluationHook`)      |
| Clock seam            | `p.now()`, backed by `WithNowFunc` — drives uptime, response timestamps, and throttle freshness; tests inject a fixed clock.                | `probe.go` (`now`, `uptime`)           |
| Method-set guard      | `WithGETOnly` / `WithAllowedMethods` handler wrapper; rejects non-allowed methods with 405 + sorted `Allow` header (GET always included).   | `probe.go` (`guard`)                   |
| Instance ID           | Replica identifier via `WithInstanceID`, echoed in every response as `instance_id`; distinguishes pods behind one load balancer.            | `types.go` (`Response.InstanceID`)     |
| Self-registration     | `HealthCheck(ctx)` makes the Probe itself a `do.HealthcheckerWithContext` in its own injector; wraps `ErrProbeUnhealthy` on fail.           | `accessors.go` (`HealthCheck`)         |
| Classifier            | Read-only type owning classification, startup evaluation, and grading. Constructed once; evaluated lock-free.                              | `classifier.go`                        |

## Bounded contexts

| Context       | Scope                                                         | Key types                                 |
| ------------- | ------------------------------------------------------------- | ----------------------------------------- |
| HTTP handlers | Liveness, readiness, startup endpoints and route registration | `Routes`, handler methods                 |
| Evaluation    | Health-check execution, classification, panic recovery        | `Evaluate`, `classify`, `runHealthChecks` |
| Configuration | Option pattern, config struct, validation                     | `Option`, `config`, `Validate`            |
| Lifecycle     | Background loop, shutdown, startup latch                      | `Start`, `Shutdown`, `MarkShuttingDown`   |
| Integration   | HealthRecorder interface, capability resolution               | `HealthRecorder`, `resolveHealthCheck`    |
