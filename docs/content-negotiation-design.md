# Content Negotiation & HTML Rendering Analysis

## Question

Should go-health support content negotiation (e.g., `application/json` vs
`text/html` for browsers) on its health endpoints?

## Short Answer

**No — not inside go-health itself.** Content negotiation and HTML rendering
belong in a separate composition layer (the host application or a sibling
package) that depends on **both** go-health and a rendering library like
[`templ-components`](https://github.com/larsartmann/templ-components).

---

## Current Behavior

All three endpoints (`/healthz`, `/readyz`, `/startupz`) funnel through
`writeResponse` in `handlers.go`, which hardcodes:

```go
w.Header().Set("Content-Type", "application/json")
w.Header().Set("Cache-Control", "no-cache")
payload, err := json.Marshal(resp)
```

No `Accept` header is inspected. No alternate representation exists. The
`Response` struct is designed for machine consumption (`omitempty` fields,
flat map of checks).

---

## Why Not Add It Here

### 1. Single-Dependency Principle

go-health's value proposition is being a **single-dependency** library
(`samber/do/v2` only). Content negotiation with HTML rendering requires:

| Dependency | Why | Pulled in by |
|---|---|---|
| `a-h/templ` | templ runtime | templ-components |
| `tailwind-merge-go` | class merging | templ-components |
| `go-error-family` | error types | templ-components |

Every consumer of go-health would transitively depend on a templ runtime,
Tailwind tooling, and CSS infrastructure — even if they only use JSON probes
for Kubernetes.

### 2. Endpoint Semantics

`/healthz`, `/readyz`, `/startupz` follow **Kubernetes conventions** — they
are machine-to-machine endpoints. Kubelet sends `Accept: */*` and only checks
the HTTP status code (200 vs 503). Load balancers do the same.

The only consumer that benefits from HTML is a **human with a browser**.
But that human doesn't want a prettified JSON dump — they want a **dashboard**
with history, trends, grouping, and alerting. That's a different product.

### 3. Performance

The library's design emphasizes speed:

- Liveness returns in **microseconds** (no dependency checks)
- Background caching avoids hammering dependencies
- `writeResponse` is a hot path called on every kubelet poll

HTML rendering (even with compiled templ) adds template execution, string
building, and larger payloads to every request — or requires a separate
caching layer for HTML output that doesn't exist today.

### 4. Complexity Cost

Proper content negotiation is non-trivial:

- Accept-header parsing with quality values (`text/html;q=0.9,
  application/json;q=0.8`)
- Fallback ordering when no match
- Wildcard handling (`*/*`, `text/*`)
- Testing matrix across representations
- Maintaining two output shapes in sync

All of this serves **zero production traffic** — only humans browsing
directly during incidents.

### 5. Separation of Concerns

Health-checking infrastructure (probe scheduling, dependency checking,
status classification) is fundamentally different from health-check
**presentation** (HTML rendering, dashboards, styling). Mixing them:

- Creates a god-module with two reasons to change
- Couples release cycles (an HTML layout change forces a health-SDK release)
- Makes the SDK harder to adopt for teams that don't want HTML

---

## The Right Architecture: Composition Layer

```
┌─────────────────────────────────────────────────────────┐
│  Host Application                                       │
│                                                         │
│  ┌──────────────────┐     ┌──────────────────────────┐  │
│  │  go-health       │     │  health-dashboard        │  │
│  │  (this repo)     │     │  (sibling package/app)   │  │
│  │                  │     │                          │  │
│  │  JSON probes     │     │  Accept: application/json│  │
│  │  /healthz        │     │    → delegate to Probe   │  │
│  │  /readyz         │     │  Accept: text/html       │  │
│  │  /startupz       │     │    → templ-components    │  │
│  └──────────────────┘     └──────────────────────────┘  │
│              ↑                        ↑                 │
│              └───────────┬────────────┘                 │
│                   composes both                         │
└─────────────────────────────────────────────────────────┘
```

A `health-dashboard` package (or example app) would:

1. Accept an `*http.Request` and inspect `Accept` headers
2. For `application/json` (or no match): delegate to go-health's existing
   `http.HandlerFunc` handlers — zero overhead
3. For `text/html`: call `probe.Evaluate(ctx)` to get a `Response`, then
   render a templ-components page

Neither library knows about the other. The host application wires them
together.

---

## How templ-components Maps to Health Status

If a composition layer were built using
[`github.com/larsartmann/templ-components`](https://github.com/larsartmann/templ-components),
the mapping is nearly one-to-one:

| go-health concept | templ-components component | Notes |
|---|---|---|
| `Response.Status` (pass/warn/fail) | `feedback.Alert` | Full-width banner: "All systems operational" or "2 checks failing" |
| `Response.Checks` map | `display.Table` | One row per check, with status badge in each row |
| `StatusPass` / `StatusWarn` / `StatusFail` | `display.StatusBadge` | Built-in `statusToBadgeMap` already maps "healthy"→green, "degraded"→yellow, "unhealthy"→red |
| Version, uptime, latency | `display.StatCard` | Metrics row at top of dashboard |
| Critical vs non-critical grouping | `display.Card` | Group checks into cards by classification |
| Auto-refresh | `htmx.PolledRegion` | Hit the JSON endpoint on interval, re-render dashboard region |

`display.StatusBadge` in particular has a built-in mapping that already
recognizes health-domain vocabulary:

```go
var statusToBadgeMap = map[string]BadgeType{
    "active": BadgeSuccess, "healthy": BadgeSuccess, "success": BadgeSuccess,
    "degraded": BadgeWarning, "warning": BadgeWarning, "pending": BadgeWarning,
    "error": BadgeError, "failed": BadgeError, "unhealthy": BadgeError,
}
```

go-health's `Status` values (`pass`, `warn`, `fail`) would need a thin
adapter to map to these strings, but the semantic alignment is exact.

---

## What About a Plain `text/plain` Option?

A lighter alternative to full HTML is `text/plain` — returning just `pass`,
`warn`, or `fail` as a bare string body. Some load balancers (HAProxy) support
plaintext health checks.

This is **also not worth adding to go-health** because:

1. The HTTP status code (200/503) already communicates pass/fail to any
   load balancer — plaintext adds nothing the status code doesn't
2. It still requires content negotiation infrastructure
3. Anyone who needs it can wrap the handler in 3 lines of code

---

## Decision

| Option | Verdict | Rationale |
|---|---|---|
| Add content negotiation to go-health | **Rejected** | Breaks single-dependency principle, adds complexity for zero production benefit |
| Add HTML rendering to go-health | **Rejected** | Couples health-checking with presentation; forces transitive deps |
| Add `text/plain` to go-health | **Rejected** | Status code already communicates this; not worth the negotiation cost |
| Document composition pattern (this doc) | **Accepted** | Captures the decision and shows the right architecture |
| Build `health-dashboard` sibling package | **Future** | Right place for HTML rendering using templ-components; separate module, separate deps |

---

## If You Want to Build the Dashboard

The `health-dashboard` package (separate repo or `go-health/examples/dashboard`)
would have this shape:

```go
// Module: github.com/larsartmann/go-health-dashboard
// Dependencies: go-health + templ-components

func DashboardHandler(probe *health.Probe) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if acceptsHTML(r) {
            resp := probe.Evaluate(r.Context())
            component := renderDashboard(resp)
            component.Render(r.Context(), w)
            return
        }
        // Default: delegate to go-health's JSON handler
        probe.ReadinessHandler()(w, r)
    }
}
```

This keeps go-health lean while enabling rich browser dashboards for teams
that want them.
