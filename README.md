# go-health

[![CI](https://github.com/larsartmann/go-health/actions/workflows/ci.yml/badge.svg)](https://github.com/larsartmann/go-health/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/larsartmann/go-health.svg)](https://pkg.go.dev/github.com/larsartmann/go-health)
[![Go Report Card](https://goreportcard.com/badge/github.com/larsartmann/go-health)](https://goreportcard.com/report/github.com/larsartmann/go-health)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Kubernetes health-probe SDK for [samber/do](https://github.com/samber/do) v2 containers.

Turns the three-probe Kubernetes pattern (liveness, readiness, startup) into a single `Probe` type with sensible defaults, critical/non-critical service classification, background caching, and shutdown awareness.

> **Stability:** v0.1.0 alpha. The three-probe API surface is stable; internal details may change before v1.0. Single dependency, zero transitive deps beyond samber/do.

---

## Table of Contents

- [Why three probes?](#why-three-probes)
- [Install](#install)
- [Quick Start](#quick-start)
- [Sample Responses](#sample-responses)
- [Three Probes](#three-probes)
- [Key Features](#key-features)
- [Configuration Reference](#configuration-reference)
- [Shutdown Awareness](#shutdown-awareness)
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
- **GET-only enforcement** — `WithGETOnly()` rejects non-GET with 405.
- **Panic-hardened** — panics from misbehaving recorders are recovered, reported as a failed check wrapping `health.ErrPanicDuringHealthCheck`, and roll readiness up to 503 (fail closed) instead of crashing your process or lying with a 200. (Note: a service whose own `HealthCheck` panics on the raw-injector path crashes the process — samber/do runs each check in a goroutine; keep service checks total.)
- **Read-only accessors** — `CachedResponse()` and `RefreshInterval()` let dashboards and middleware read cached health state without triggering a synchronous evaluation.
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

### `WithGETOnly()`

Wraps all handlers to reject non-GET requests with 405 Method Not Allowed. Kubernetes probes always use GET; enabling this surfaces misconfigurations (e.g. a load balancer sending HEAD or POST) early.

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

## License

MIT
