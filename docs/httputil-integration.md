# httputil Integration — composition recipes

> Status: PROVEN · Proof: `testdata/httputil-interop` via `nix run .#interop`
> (CI step in the test job) · Index: [integrations.md](integrations.md)

[httputil](https://github.com/larsartmann/httputil) is the fleet's HTTP
middleware library; go-health is the fleet's health-probe SDK. Most fleet
services depend on both. This document owns the composition: what each side
contributes, the recipes that pair them, and where each recipe is
machine-proved. Keep it in lockstep with the fixture — every recipe number
below has a matching test there.

## The boundary

Neither library imports the other, by policy: httputil's `depguard` forbids
third-party dependencies outright, and go-health keeps its three-dependency
footprint. Integration is composition at the `http.HandlerFunc` seam plus
this document — never a dependency.

| Concern                                                      | Owner                                                |
| ------------------------------------------------------------ | ---------------------------------------------------- |
| Roll-up classification, caching, shutdown, wire format       | go-health                                            |
| Recovery, request ID, rate limiting, timeouts, Server-Timing | httputil                                             |
| Composition point                                            | plain `http.HandlerFunc` (docs/middleware-design.md) |

Vocabulary differs by design; the HTTP status code is the shared contract.
go-health roll-ups are `pass`/`warn`/`fail` (check-level `off`), mapped to
200/200/503; httputil's stub health payload speaks `up`/`down` (200/503).
Kubelet and load balancers read only the code.

## Recipe 1 — the endpoint split

Wrap readiness/startup with httputil middleware when exposure demands it;
leave liveness open and unwrapped (kubelet cannot send credentials, and an
auth rejection on liveness reads as a container failure — restart cascade).
Recovery goes outermost, per httputil's Chain contract:

```go
h := httputil.Chain(
    probe.LivenessHandler(),                            // open, always 200
    httputil.Recovery(slog.Default()),
    httputil.RequestID(httputil.DefaultRequestIDConfig()),
)
```

Proven by `TestLiveness_MiddlewareChainStaysAlwaysUp`: with every dependency
failing, liveness through the chain still answers 200 `"pass"` and the
request-id middleware demonstrably ran.

## Recipe 2 — cross-cutting readiness

The full chain preserves go-health's code semantics exactly: `pass` and
`warn` stay 200, only `fail` is 503. Rate limiting matters on the live path
(`WithRefreshInterval(0)`); with the background cache (default) the handler
is already O(1) per request:

```go
h := httputil.Chain(
    probe.ReadinessHandler(),
    httputil.Recovery(slog.Default()),
    httputil.RequestID(httputil.DefaultRequestIDConfig()),
    httputil.Timeout(5*time.Second),
    httputil.KeyedRateLimiterMiddleware(httputil.DefaultKeyedRateLimiterConfig()),
)
```

Proven by `TestReadiness_ChainPreservesCodeSemantics` over all three roll-ups.

## Recipe 3 — composition order and the method guard

Middleware runs **before** the probe's method guard
(`WithAllowedMethods`): a non-allowed method traverses the whole chain and
gets the guard's `405` + sorted `Allow` header — never a middleware-level
rejection. Middleware must also never cache responses across requests: it
would freeze the shutdown transition (docs/middleware-design.md).

Proven by `TestMethodGuard_ComposesOutsideMiddleware` (405/Allow through a
rate-limited chain) and `TestShutdown_FlipsReadinessThroughChain`
(`probe.Shutdown()` → 503 `shutting_down: true` through the chain).

### Drain ordering with httputil.Server

`probe.Shutdown()` must take effect **before** `httputil.Server.Shutdown`
starts draining, so the load balancer stops routing while in-flight requests
finish. samber/do dispatches `Shutdowner` services in reverse invocation
order, and the HTTP server is typically invoked after the probe — so do not
rely on `AsShutdowner` ordering here. Flip the probe first, then hand over to
the container:

```go
probe.Shutdown()                    // readiness 503 immediately
_ = inj.ShutdownWithContext(ctx)    // do discovers *httputil.Server, drains
```

## Recipe 4 — Server-Timing per-check durations

httputil's `server_timing` sub-module emits the W3C `Server-Timing` header;
go-health's detailed checks report per-check `DurationNanos`. The bridge is
~10 lines, consumer-side, and records **before** delegating — the middleware
commits the header at the response's first write, so metrics recorded after
that never ship. Metric names are HTTP tokens: go-health names containing
`/` (aggregate/federation source rows) are sanitized to `_` by the
`servertiming` package. Without the middleware in the chain the collector is
absent and `RecordServerTiming` is a nil-safe no-op:

```go
func serverTimingBridge(probe *health.Probe, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        for name, check := range probe.CachedResponse().Checks {
            if check.DurationNanos > 0 {
                servertiming.RecordServerTiming(r.Context(), name, "", time.Duration(check.DurationNanos))
            }
        }
        next.ServeHTTP(w, r)
    })
}

h := servertiming.ServerTimingMiddleware()(serverTimingBridge(probe, chainedReadiness))
```

Proven by `TestServerTiming_CheckDurationsOnTheWire` (`db;dur=1.5` plus the
middleware's `total` on the wire).

## What is proven, and where

| Guarantee                                                           | Proof                                             |
| ------------------------------------------------------------------- | ------------------------------------------------- |
| Recipes compile and hold against the **published** httputil release | `testdata/httputil-interop` + `nix run .#interop` |
| Gate sweep includes the proof (fail-fast)                           | `interop` in the `.#gates` default list           |
| Every CI run re-proves it                                           | `httputil interop fixture` step in the test job   |

The fixture module resolves httputil and `server_timing` from the module
proxy at their released tags and replaces go-health with the working tree,
so the proof tracks HEAD's behavior, not a frozen snapshot.

## Deliberately out of scope

Recorded decisions that shape this integration — do not reopen them as
"integration improvements":

- ETag/`If-None-Match` on health endpoints: status-truth must never come
  from a cache (docs/etag-rejection-design.md).
- HTML/content-negotiated responses (docs/content-negotiation-design.md).
- A library-side middleware concept (docs/middleware-design.md).
- A dependency in either direction (see "The boundary").
