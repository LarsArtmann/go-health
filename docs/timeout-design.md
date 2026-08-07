# Timeout Design Analysis

## Current Behavior

```
go-health applies: context.WithTimeout(ctx, p.timeout)  // default 5s
        ↓
samber/do HealthCheckWithContext(ctx):
  ├── (optional) applies opts.HealthCheckGlobalTimeout if set at injector creation
  ├── runs ALL registered services in parallel (async, via worker pool)
  └── each service: raceWithTimeout(ctx, service.healthcheck)
        └── all share the SAME context deadline — no per-service isolation
```

`WithTimeout(5s)` applies **one deadline to the entire batch**, not per service.

Source: `scope.go:307-326` in samber/do v2.1.0 — all services are checked
concurrently against the same context. If service A consumes 4.9s of the 5s
budget, service B only has 0.1s before it's timed out.

## The Gap

The method name and API shape (`WithTimeout`) invite the mental model "each
health check gets this long." The actual semantics are "the entire batch gets
this long, shared across all services." A slow dependency silently steals time
from every other check.

samber/do v2 exposes two relevant injector-level options via `InjectorOpts`
(`injector.go:144-149`):

- `HealthCheckGlobalTimeout` — batch-level timeout (same concept as our `WithTimeout`)
- `HealthCheckTimeout` — per-service timeout (we do not expose or document this)

These are set at `do.NewWithOpts()` time by the caller, not by this library.

## Rejected: HTTP Query-Param Timeout Override

**Decision: Rejected. Do not implement.**

### Reason 1: Architectural contradiction with caching

The library's core value proposition is the background cache loop: evaluate
with a configured timeout, serve cached results in O(1). A `?timeout=30s`
query param forces one of two bad outcomes:

- Serve stale cache → the param does nothing (useless feature)
- Evaluate live → bypasses the cache, hammers dependencies (defeats the library)

There is no third option that respects both the param and the cache.

### Reason 2: DoS amplifier

Health endpoints are typically unauthenticated (kubelet needs them).
`GET /readyz?timeout=10m` holds each health check open for 10 minutes per
request. An attacker can stall the DB connection pool, Redis pool, and every
downstream dependency by looping that URL.

### Reason 3: Wrong consumer

Kubelet sends `GET /readyz` — no query params, ever. Load balancers — same.
The only consumer who would use `?timeout=` is a human debugging, and that
human already has `Evaluate(ctx)` for programmatic access with any deadline.

## Action Items

1. **Fix `WithTimeout` doc comment** — state clearly that this is a batch-level
   deadline shared across all services, not a per-service timeout.

2. **Document the samber/do `HealthCheckTimeout` option** — users who need
   per-service timeouts should set `do.NewWithOpts(do.WithHealthCheckTimeout(d))`
   at injector creation time. This library should mention this in its doc.go.

3. **Consider exposing per-service timeout** — either a new option
   (`WithPerServiceTimeout`) that configures the injector, or a doc-level
   pointer to the samber/do native option. Low priority until a user hits the
   problem in practice.
