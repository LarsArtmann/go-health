# go-health

Production-ready Kubernetes health-probe SDK for [samber/do](https://github.com/samber/do) v2 containers.

Turns the three-probe Kubernetes pattern (liveness, readiness, startup) into a single `Probe` type with sensible defaults, critical/non-critical service classification, background caching, and shutdown awareness.

## Install

```bash
go get github.com/larsartmann/go-health
```

## Quick Start

```go
import "github.com/larsartmann/go-health"

injector := do.New()

// ... register and invoke services ...

probe := health.New(injector,
    health.WithCriticalServices("database", "redis"),
    health.WithVersion("1.0.0"),
)

probe.Start(ctx)
defer probe.Shutdown()

mux := http.NewServeMux()
probe.RegisterRoutes(mux, health.DefaultRoutes())
```

## Three Probes

| Endpoint    | Purpose                        | Returns 503 when...                         |
| ----------- | ------------------------------ | ------------------------------------------- |
| `/healthz`  | Liveness — process alive?      | Never (always 200 unless process is dead)   |
| `/readyz`   | Readiness — can serve traffic? | Any critical service fails or shutting down |
| `/startupz` | Startup — done booting?        | Not all critical services have passed yet   |

## Key Features

- **Liveness never checks dependencies** — returns in microseconds, always 200. Prevents restart cascades.
- **Readiness gates on critical services only** — non-critical failures set status to `warn` (HTTP 200, degraded).
- **Startup latches** — once all critical services pass, always returns 200 without re-checking.
- **Background caching** (1s default) — kubelet/LB polling doesn't hammer dependencies.
- **Shutdown-aware** — `Shutdown()` flips readiness to 503 immediately; liveness stays 200.
- **GET-only enforcement** — `WithGETOnly()` rejects non-GET with 405.
- **Optional recorder** — wire any `HealthRecorder` (e.g. `samber-do-auditlog.Plugin`) to observe every check batch.

## Audit Integration

When a `HealthRecorder` is provided via `WithHealthRecorder`, every health-check batch is delegated to the recorder. [`samber-do-auditlog`](https://github.com/larsartmann/samber-do-auditlog)'s `*Plugin` satisfies the interface implicitly:

```go
plugin, _ := auditlog.New(auditlog.Config{Enabled: true})
injector := do.NewWithOpts(plugin.Opts())

probe := health.New(injector, health.WithHealthRecorder(plugin))
```

## License

MIT
