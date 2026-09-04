# HTTP Middleware — design note

> Decided: 2026-09-04 · Status: DESIGN SETTLED, SPIKE VERIFIED · Resolves: ROADMAP Theme 6 "HTTP middleware support"

## Problem

Deployments want to wrap probe endpoints with cross-cutting behavior: bearer
or mTLS auth on readiness, rate limiting on live-mode readiness, request
logging, per-route timeouts. Should go-health grow middleware support?

## Decision: none needed — handlers are plain http.HandlerFunc, by design

Every exported handler (`LivenessHandler`, `ReadinessHandler`,
`StartupHandler`, `Healthz`) returns an `http.HandlerFunc`. That is the
middleware seam — the entire `net/http` ecosystem composes with it directly:

```go
mux.HandleFunc("/healthz", probe.LivenessHandler())
mux.HandleFunc("/readyz", bearerAuth("s3cret", probe.ReadinessHandler()))
```

A library-side middleware concept (`Use(...)` chains, `Middleware` types,
ordering rules) would re-implement what `http.Handler` composition already
standardizes, and would only cover routes the library registers — the
spike's pattern covers any route, registered any way.

A verified spike lives in `middleware_example_test.go`
(`ExampleProbe_ReadinessHandler_middleware`): a 10-line auth middleware that
returns 200 (liveness, open), 401 (readiness, missing token), and 200
(readiness, valid token).

## Which endpoints to wrap: the split that matters

- **Liveness stays open and unwrapped.** kubelet liveness probes cannot send
  credentials, and the handler must stay microsecond-cheap and always-200;
  an auth rejection would read as a container failure and trigger restarts.
- **Readiness/startup may be wrapped** when exposed beyond the cluster;
  kubelet-facing routes should stay unauthenticated for the same reason
  (startup gating on a 401 would hang boot).
- **`Healthz`** is the combined endpoint for external load balancers; wrap it
  exactly when the load balancer supports headers (most support none — keep
  it open and rely on network-layer isolation).

## Library invariants middleware must not break

- Middleware runs _before_ the guard (`WithGETOnly` / `WithAllowedMethods`);
  a 401/429 from middleware is unaffected by method enforcement.
- The shutdown overlay and startup latch live inside the handlers, so no
  middleware can accidentally bypass them by construction — but middleware
  must not cache responses across requests (it would freeze the shutdown
  transition).
