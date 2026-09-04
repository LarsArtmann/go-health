# go-health

[![CI](https://github.com/larsartmann/go-health/actions/workflows/ci.yml/badge.svg)](https://github.com/larsartmann/go-health/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/larsartmann/go-health.svg)](https://pkg.go.dev/github.com/larsartmann/go-health)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Kubernetes health-probe SDK for [samber/do](https://github.com/samber/do) v2 containers.

Turns the three-probe Kubernetes pattern (liveness, readiness, startup) into a single `Probe` type with sensible defaults, critical/non-critical service classification, background caching, and shutdown awareness.

> **Stability:** v0.1.1 alpha. The three-probe API surface is stable; internal details may change before v1.0. Single dependency, zero transitive deps beyond samber/do.

---

## Table of Contents

- [Why three probes?](#why-three-probes)
- [Install](#install)
- [Compatibility](#compatibility)
- [Quick Start](#quick-start)
- [Sample Responses](#sample-responses)
- [Three Probes](#three-probes)
- [Key Features](#key-features)
- [Configuration Reference](#configuration-reference)
- [Shutdown Awareness](#shutdown-awareness)
- [Programmatic Health API](#programmatic-health-api)
- [Metrics](#metrics)
- [Middleware](#middleware)
- [Aggregating Multiple Probes](#aggregating-multiple-probes)
- [Audit Integration](#audit-integration)
- [Kubernetes Wiring](#kubernetes-wiring)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

---

## Why three probes?

A single `/health` endpoint conflates "process alive" with "dependencies reachable." When a dependency blips, the endpoint returns 503, the kubelet restarts the pod, and a restart cascade follows, even though the process itself is fine.

Splitting probes breaks this coupling:

- **Liveness** never checks dependencies. Only a deadlocked or crashed process fails.
- **Readiness** checks dependencies but only returns 503 for critical failures. Non-critical failures (e.g. metrics exporter) appear in the response body without removing the pod from rotation.
- **Startup** lets slow-booting apps use a generous kubelet `failureThreshold` without affecting liveness/readiness sensitivity.

## Install

```bash
go get github.com/larsartmann/go-health
```

**Requirements:** Go 1.26+. Single dependency: `github.com/samber/do/v2`.

## Compatibility

What CI actually tests on every push — not what merely compiles:

| Dimension | Tested                                        |
| --------- | --------------------------------------------- |
| Go        | 1.26.x — CI runs go 1.26.7 on linux/amd64     |
| samber/do | v2.1.0 (the pinned `go.mod` dependency)       |
| Aggregate | same module version, tested in the same suite |

Bare `go` commands outside this repo's flake need `GOEXPERIMENT=jsonv2`: the
library imports `encoding/json/v2`, which go1.26 only exposes behind that
experiment. Building through the flake, or with Go 1.27+ where json/v2 is
stable, needs nothing special.

Older Go toolchains are unsupported. Other samber/do v2.x versions are
expected to work but are not covered by CI — if you bump it, run the test
suite against your version before relying on it.

## Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/larsartmann/go-health"
    "github.com/samber/do/v2"
)

func main() {
    injector := do.New()

    // ... register and eagerly invoke services ...

    probe := health.New(injector,
        health.WithCriticalServices("database", "redis"),
        health.WithVersion("1.0.0"),
    )

    if err := probe.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
    defer probe.Shutdown()

    mux := http.NewServeMux()
    probe.RegisterRoutes(mux, health.DefaultRoutes())

    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

## Sample Responses

**Healthy (200):**

```json
{
  "status": "pass",
  "version": "1.0.0",
  "uptime": "5m32s",
  "total_latency_ms": 12,
  "checks": {
    "database": { "status": "pass" },
    "redis": { "status": "pass" }
  }
}
```

**Degraded, non-critical failure (200):**

```json
{
  "status": "warn",
  "version": "1.0.0",
  "uptime": "5m32s",
  "total_latency_ms": 15,
  "checks": {
    "database": { "status": "pass" },
    "redis": { "status": "pass" },
    "metrics-exporter": { "status": "warn", "error": "connection refused" }
  }
}
```

**Critical failure (503):**

```json
{
  "status": "fail",
  "version": "1.0.0",
  "uptime": "5m32s",
  "total_latency_ms": 5004,
  "checks": {
    "database": { "status": "fail", "error": "context deadline exceeded" },
    "redis": { "status": "pass" }
  }
}
```

## Three Probes

| Endpoint    | Purpose                       | Returns 503 when...                         |
| ----------- | ----------------------------- | ------------------------------------------- |
| `/healthz`  | Liveness, process alive?      | Never (always 200 unless process is dead)   |
| `/readyz`   | Readiness, can serve traffic? | Any critical service fails or shutting down |
| `/startupz` | Startup, done booting?        | Not all critical services have passed yet   |

## Key Features

- **Liveness never checks dependencies** — returns in microseconds, always 200. Prevents restart cascades.
- **Readiness gates on critical services only** — non-critical failures set status to `warn` (HTTP 200, degraded).
- **Startup latches** — once all critical services pass, always returns 200 without re-checking.
- **Background caching** (1s default) — kubelet/LB polling doesn't hammer dependencies.
- **Shutdown-aware** — `Shutdown()` flips readiness to 503 immediately; liveness stays 200.
- **Method-set enforcement** — `WithAllowedMethods(...)` rejects non-allowed methods with 405 and a sorted `Allow` header (`WithGETOnly` is the deprecated zero-arg equivalent).
- **Programmatic health API** — `Status()`, `Alive()`, `Ready()`, `AwaitReady(ctx)`, `Healthz()` — query health without spinning up HTTP; register the probe in its own injector via `HealthCheck`.
- **Observability seam** — `WithEvaluationHook(fn)` observes every classified response; Prometheus exposition and OpenTelemetry compose on top without new dependencies.
- **Panic-hardened** — panics from misbehaving recorders are recovered, reported as a failed check wrapping `health.ErrPanicDuringHealthCheck`, and roll readiness up to 503 (fail closed) instead of crashing your process or lying with a 200. (Note: a service whose own `HealthCheck` panics on the raw-injector path crashes the process — samber/do runs each check in a goroutine; keep service checks total.)
- **Read-only accessors** — `CachedResponse()` and `RefreshInterval()` let dashboards and middleware read cached health state without triggering a synchronous evaluation.
- **Flood-safe live mode** — `WithLiveThrottle(d)` coalesces live-mode request floods into one evaluation per window.
- **Deterministic tests** — `WithNowFunc(fn)` drives uptime, timestamps, and throttle freshness from an injected clock; no sleeps.
- **Config validation** — `Start()` validates configuration and returns an error on invalid settings (zero/negative timeout, negative refresh interval).
- **Optional recorder** — wire any `HealthRecorder` (e.g. `samber-do-auditlog.Plugin`) to observe every check batch.

## Configuration Reference

### `WithCriticalServices(names ...string)`

Marks services as critical. If any fails its health check, readiness returns 503. Services not listed are non-critical — their failures appear in the response body but do not change the HTTP status code.

```go
health.WithCriticalServices("database", "redis")
```

### `WithVersion(v string)`

Sets the application version string included in health responses.

### `WithTimeout(d time.Duration)`

Sets the **batch-level** context deadline shared across ALL services in a single evaluation (default: 5s). All checks run concurrently against the same deadline — a slow dependency can silently steal time from every other check.

For **per-service** timeout isolation, configure samber/do's native option at injector creation time:

```go
injector := do.NewWithOpts(do.WithHealthCheckTimeout(2 * time.Second))
```

This library does not override that setting; it only controls the outer batch deadline. See [docs/timeout-design.md](docs/timeout-design.md) for the full analysis.

### `WithRefreshInterval(d time.Duration)`

Controls background cache refresh cadence (default: 1s):

- **Greater than zero** — launches a goroutine that re-evaluates health checks on this interval. Readiness handlers serve the cached result for O(1) response time.
- **Zero** — readiness handlers evaluate live on every request. Use for low-traffic or development scenarios.

```go
health.WithRefreshInterval(0) // live mode
```

### `WithBootTime(t time.Time)`

Overrides the boot timestamp used to compute uptime. Defaults to the time `New()` was called. Useful for testing.

### `WithGETOnly()` — deprecated

> **Deprecated:** use [`WithAllowedMethods(...)`](#withallowedmethodsmethods-string) instead — the method-set superset (`WithAllowedMethods()` with no arguments behaves identically). `WithGETOnly` keeps working; no removal is planned in the v0.x line. See [docs/deprecation-policy.md](docs/deprecation-policy.md) for the full policy.

Wraps all handlers to reject non-GET requests with 405 Method Not Allowed. Kubernetes probes always use GET; enabling this surfaces misconfigurations (e.g. a load balancer sending HEAD or POST) early.

### `WithAllowedMethods(methods ...string)`

Method-set variant of `WithGETOnly`: the listed methods are accepted too (GET is always allowed). Rejected requests get a 405 whose `Allow` header lists the full set, sorted:

```go
health.WithAllowedMethods(http.MethodHead) // kubelet GET + LB HEAD both pass
```

### `WithInstanceID(id string)`

Sets a replica identifier echoed in every response as `instance_id`. Use it when several instances serve behind one load balancer and dashboards must attribute a response to the pod that produced it.

### `WithEvaluationHook(fn func(Response))`

Registers a callback invoked synchronously after every evaluation with the fully classified response — the seam for metrics (e.g. a Prometheus gauge) and alerting without polling. The hook must be fast and must not mutate the response.

### `WithLiveThrottle(d time.Duration)`

Coalesces live-mode request floods: within the throttle window, requests are served the stored result of the previous evaluation instead of each starting its own batch. One evaluation per window, no matter how many requests arrive.

**Interaction with the `Start`-populated cache.** The throttle changes the readiness read path: a handler serves any stored result younger than the window — including the one the background refresh loop writes — and evaluates live only when the stored result is older than the window. Concretely: with the default cache (`WithRefreshInterval(1s)`) a window at least as large as the refresh interval means requests never trigger evaluations; the loop refreshes, requests only read. With `WithRefreshInterval(0)` (live mode) the window is what turns a request flood into one batch per window. Liveness is unaffected; startup never throttles.

### `WithShutdownGracePeriod(d time.Duration)`

Automatic two-phase shutdown timing: `Shutdown()` first marks the probe as draining (readiness 503) and stops the loop after the grace period, replacing the manual `MarkShuttingDown()` + sleep + `Shutdown()` sequence.

### `WithNowFunc(fn func() time.Time)`

Overrides the clock used for uptime, response timestamps, and live-throttle freshness. Defaults to `time.Now`. Inject a fixed clock in tests for fully deterministic assertions.

### `WithHealthRecorder(r HealthRecorder)`

Wires a `HealthRecorder` so every health-check batch is observable by an external system. When nil (the default), checks run against the raw injector.

## Shutdown Awareness

Call `Shutdown()` during your server's graceful-drain path. Readiness immediately returns 503 so load balancers stop sending traffic before connections close. Liveness stays 200 because the process is still alive.

For **two-phase** graceful shutdown, call `MarkShuttingDown()` first (starts draining), then `Shutdown()` after a grace period (stops the refresh loop):

```go
// Phase 1: signal load balancers to drain
probe.MarkShuttingDown()

// ... wait for connections to drain ...

// Phase 2: stop background loop
probe.Shutdown()
```

## Programmatic Health API

Not every consumer speaks HTTP. CLIs, background workers, and test harnesses can query the same cached state the handlers serve — with zero dependency checks:

```go
if probe.Ready() {
    serveTraffic()
}

// Block until the instance can serve, or give up after 30s.
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := probe.AwaitReady(ctx); err != nil {
    log.Fatal("instance never became ready: ", err)
}
```

For deployments that expose a single health endpoint (external load balancers, composition layers), `Healthz()` answers one question — "should traffic be routed here?" — combining the startup latch, readiness roll-up, and shutdown state into one 200/503 answer.

The probe is also a first-class samber/do citizen: `HealthCheck(ctx)` satisfies `do.HealthcheckerWithContext` (register the probe in its own injector; it wraps `health.ErrProbeUnhealthy` when the roll-up is fail), and `AsShutdowner()` adapts it to `do.ShutdownerWithError` for container-managed shutdown.

No samber/do injector? Build a probe from any batch function:

```go
probe := health.NewWithHealthCheck(func(ctx context.Context) map[string]error {
    return map[string]error{"database": db.PingContext(ctx)}
}, health.WithCriticalServices("database"))
```

## Metrics

The library ships no metrics client — it provides the seam instead. `WithEvaluationHook(fn)` fires synchronously after every evaluation with the fully classified response, so a few lines of composition produce a Prometheus exposition without adding a dependency:

```go
probe := health.New(injector,
    health.WithEvaluationHook(func(resp health.Response) {
        healthGauge.Set(statusValue(resp.Status))
        for name, check := range resp.Checks {
            checkGauge.WithLabelValues(name, string(check.Status)).Set(1)
        }
    }),
)
```

A complete, verified ~40-line stdlib exposition writer lives in [`prometheus_example_test.go`](prometheus_example_test.go); the design rationale is in [docs/prometheus-exposition-design.md](docs/prometheus-exposition-design.md).

## Middleware

Handlers are plain `http.HandlerFunc`, so standard middleware composes directly — no library concept needed:

```go
mux.Handle("/readyz", authMiddleware(probe.ReadinessHandler()))
```

Wrap readiness (and startup) when they need auth or rate limiting; leave liveness unwrapped — the kubelet cannot send credentials, and liveness must never fail on auth. See [docs/middleware-design.md](docs/middleware-design.md) and the verified example in [`middleware_example_test.go`](middleware_example_test.go).

## Aggregating Multiple Probes

One process can embed several independently configured probes — one per logical service or tenant — and expose their combined state as a single go-health surface via the [`aggregate`](aggregate/) sub-package:

```go
import "github.com/larsartmann/go-health/aggregate"

apiProbe := health.New(apiInjector, health.WithCriticalServices("db"))
webProbe := health.New(webInjector, health.WithCriticalServices("db", "cache"))

agg, err := aggregate.New(
    aggregate.Source{Name: "api", Probe: apiProbe},
    aggregate.Source{Name: "web", Probe: webProbe},
)
if err != nil {
    log.Fatal(err)
}

agg.RegisterRoutes(mux, health.DefaultRoutes())
```

The aggregate is passive: it adds no goroutines and merges on read (one lock-free cache load per source), so freshness is bounded by the slowest source's refresh interval. Overall status is the worst of the sources; every check is namespaced `source/check`, which keeps keys collision-free and gives consumers a stable grouping axis. Liveness stays dependency-blind (always 200); readiness is 503 on `fail` or any source shutting down; startup latches only when every source has booted.

## Audit Integration

When a `HealthRecorder` is provided via `WithHealthRecorder`, every health-check batch is delegated to the recorder instead of the raw injector. [`samber-do-auditlog`](https://github.com/larsartmann/samber-do-auditlog)'s `*Plugin` satisfies the interface implicitly:

> Coming from the pre-extraction API? See [docs/migration-plugin-to-recorder.md](docs/migration-plugin-to-recorder.md) for the `WithPlugin` → `WithHealthRecorder` rename.

```go
plugin, _ := auditlog.New(auditlog.Config{Enabled: true})
injector := do.NewWithOpts(plugin.Opts())

probe := health.New(injector, health.WithHealthRecorder(plugin))
```

## Kubernetes Wiring

Wire the three probes in your Deployment manifest:

```yaml
spec:
  containers:
    - name: app
      ports:
        - containerPort: 8080
      livenessProbe:
        httpGet:
          path: /healthz
          port: 8080
        periodSeconds: 10
      readinessProbe:
        httpGet:
          path: /readyz
          port: 8080
        periodSeconds: 5
      startupProbe:
        httpGet:
          path: /startupz
          port: 8080
        failureThreshold: 30
        periodSeconds: 10
```

With `failureThreshold: 30` and `periodSeconds: 10`, the startup probe allows up to 5 minutes for slow-booting applications before the kubelet kills the container. Liveness and readiness probes only activate after startup succeeds.

## Troubleshooting

### Startup probe always returns 200 immediately

samber/do v2.1.0 reports never-invoked lazy services as healthy (nil error) in `HealthCheckWithContext`. Eagerly invoke critical services at boot so their `HealthCheck` methods are actually exercised:

```go
// Force instantiation so HealthCheck is called
do.MustInvokeNamed[*Database](injector, "database")
```

### Readiness returns 503 but my service is fine

Check whether the failing service is marked as critical. Non-critical failures return 200 (degraded), not 503. Only critical service failures or shutdown state produce 503.

### Health checks timing out

The default timeout is 5 seconds shared across ALL services. If one service is slow, it steals time from every other check. Either increase the batch timeout via `WithTimeout`, or configure per-service timeouts via `do.WithHealthCheckTimeout` at injector creation time.

### 405 Method Not Allowed with an `Allow` header

A method-set guard is active (`WithAllowedMethods` or the deprecated `WithGETOnly`). The `Allow` header lists every accepted method, sorted. Add the missing method to the set — or remove the guard — if a caller (e.g. a load balancer sending HEAD) is legitimate.

### pkg.go.dev shows old docs

pkg.go.dev picks up new versions from the module proxy with propagation lag (up to ~1h after tagging). If the latest tag is still missing, wait and re-check; the proxy itself (`go get module@version`) works immediately.

## Contributing

This project uses Nix for reproducible builds. See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, code conventions, and PR process.

```bash
nix develop          # Enter dev shell
nix run .#test       # Run tests
nix run .#test-race  # Run tests with race detector
nix run .#lint       # Run golangci-lint
```

## Project Docs

- [FEATURES.md](FEATURES.md) — honest feature inventory by status
- [TODO_LIST.md](TODO_LIST.md) — current work items
- [ROADMAP.md](ROADMAP.md) — long-term direction and non-goals
- [docs/DOMAIN_LANGUAGE.md](docs/DOMAIN_LANGUAGE.md) — domain glossary
- [docs/openapi.yaml](docs/openapi.yaml) — OpenAPI 3.1 spec for the three probes

Design notes: [timeout](docs/timeout-design.md) · [panic recovery](docs/panic-recovery-design.md) · [middleware](docs/middleware-design.md) · [Prometheus exposition](docs/prometheus-exposition-design.md) · [OpenAPI](docs/openapi-design.md) · [classification 2.0](docs/classification-2.0-design.md) · [multi-tenant](docs/multi-tenant-design.md) · [starting status](docs/starting-status-design.md) · [content negotiation (rejected)](docs/content-negotiation-design.md) · [`WithPlugin` → `WithHealthRecorder` migration](docs/migration-plugin-to-recorder.md) · [ADRs](docs/adr/)

## License

MIT
