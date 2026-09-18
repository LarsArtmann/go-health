# ETag / `If-None-Match` on Health Endpoints — Rejected

## Question

Should go-health serve `ETag` headers and answer `If-None-Match` with
`304 Not Modified` on `/healthz`, `/readyz`, and `/startupz` (e.g., via
[`github.com/larsartmann/go-etag`](https://github.com/larsartmann/go-etag))?

## Short Answer

**No — not inside go-health.** Health endpoints publish status-truth
(200 vs 503) that load balancers and the kubelet must never receive from a
cache. Conditional-request support is a proxy/CDN composition concern; if a
deployment ever needs it, it belongs in a wrapper around go-health's handlers,
not in the library. This closes ROADMAP Theme 5's "write the rejection note
before someone asks" item.

---

## Why Not

### 1. A cached health response is a lie with consequences

The entire value of a health endpoint is freshness: readiness flips between
200 and 503, and the kubelet, load balancer, and service mesh act on that flip
within seconds. An intermediary that answers `If-None-Match` with a cached 200
extends the life of stale "ready" state precisely when correctness matters
most. This failure mode is not hypothetical — go-etag's own v0.3.0 changelog
motivation is a field report of a CDN serving a **two-day-stale 200**
(`Age: 137882`) whose ETag faithfully described the stale entity. A health
endpoint must not be cacheable in the first place; `writeResponse` already
sends `Cache-Control: no-cache` for this reason.

### 2. The problem ETag would solve is already solved — better

ETag on a polling endpoint exists to avoid re-transfer and, more importantly,
to avoid re-computation behind the endpoint. go-health solves the
re-computation problem at the correct layer: a background cache
(`WithRefreshInterval`, 1s by default) decouples kubelet/LB poll frequency
from dependency checks, with a `CachedResponse` path that never triggers
checks at all. The bodies are small JSON documents (hundreds of bytes), so the
bandwidth saving of a 304 is negligible. What remains is overhead: an extra
middleware buffer-and-hash pass on every response.

### 3. Liveness must stay unconditional

`LivenessHandler` returns 200 in microseconds, always, without touching the
checks map — that contract prevents restart cascades. Inserting conditional
logic between the kubelet and that answer (304 semantics, weak/strong
comparison, `If-Match` on a liveness probe) adds failure modes to the one
endpoint whose only job is to be boring.

### 4. Single-dependency principle

go-health's value proposition is exactly one dependency (`samber/do/v2`).
Adopting go-etag (which itself requires `go-error-family`) would break that
for zero functional gain. Every rejected-design analysis in this repo
([content-negotiation](content-negotiation-design.md),
[prometheus exposition](prometheus-exposition-design.md),
[middleware](middleware-design.md)) lands on the same principle: go-health
ships plain `http.HandlerFunc`s; richer HTTP behavior is composed outside.

---

## When to Revisit

If a consumer ever serves health JSON to a very large WAN poller population
where transfer cost dominates, wrap the handlers in a composition layer:

```go
mux.Handle("/readyz", etag.New(etag.DefaultETagConfig())(probe.ReadinessHandler()))
```

...with two non-negotiables established by the proxy/CDN layer, not go-health:
`Cache-Control: no-cache` preserved on every response (go-etag's
`SkipIfPresent`/header-passthrough semantics must not weaken it), and no
shared-cache deployment in front of the probes. go-health itself should never
grow this.

## Related

- [middleware-design.md](middleware-design.md) — why handlers stay plain and middleware composes outside
- [content-negotiation-design.md](content-negotiation-design.md) — same rejection pattern for representation concerns
- [docs/2026-09-18_go-etag-deep-dive.html](research/2026-09-18_go-etag-deep-dive.html) — ecosystem-wide go-etag audit (2026-09-18)
