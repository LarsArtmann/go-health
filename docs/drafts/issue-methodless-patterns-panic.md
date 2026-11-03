> [!NOTE]
> This filing was drafted by GLM via Crush from an AI-run investigation, not at my request. When the failure traces to an external report, that source is linked in the body.
>
> - [ ] MANUALLY REVIEWED by `@Lars Artmann` at `[<date-time>]`

TL;DR: `DefaultRoutes()` registers `/healthz`, `/readyz`, `/startupz` without an HTTP method, and on a Go 1.22+ `ServeMux` such patterns conflict with common page-router shapes (`GET /{slug}`) — `RegisterRoutes` then panics at registration time, before the server ever serves a request. Suggested change: default the three patterns to `GET /healthz`, `GET /readyz`, `GET /startupz`.

## Problem

`registerRoute` mounts every route with `mux.HandleFunc(pattern, handler)` (`handlers.go:138-143`), and `DefaultRoutes()` fills `Routes` with bare paths (`handlers.go:18-23`). A consumer that registers its own routes first and calls `RegisterRoutes(mux, DefaultRoutes())` second gets:

```text
panic: pattern "/healthz" (registered at .../go-health/handlers.go:143) conflicts with pattern "GET /{slug}" (registered at .../main.go:14):
        /healthz matches more methods than GET /{slug}, but has a more specific path pattern
```

Minimal repro (also committed under `docs/drafts/finding1_repro/`, go 1.27, current HEAD `d783d78`; same mechanism verified against v0.8.1):

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /{slug}", pageHandler) // typical page router

probe := health.NewWithHealthCheck(func(context.Context) map[string]error { return nil })
probe.RegisterRoutes(mux, health.DefaultRoutes()) // panics here
```

The pattern-pairing problem is not exotic: any consumer with a catch-all page route (`GET /{slug}`), a method-less wildcard, or a static subtree hits the conflict rule, and Go reports it as a panic, not an error. This repo's consumer (`ChastityAPI`) hit the sibling case with its own `/assets/` mount and now scopes every general pattern with a method.

## Suggested change

Default to method-qualified patterns:

```go
func DefaultRoutes() Routes {
	return Routes{
		Liveness:  "GET /healthz",
		Readiness: "GET /readyz",
		Startup:   "GET /startupz",
	}
}
```

Kubernetes liveness/readiness/startup probes are always GET, so the default loses nothing; custom `Routes` values keep working unchanged for non-GET transports. Registration-order surprises shrink to the genuinely-ambiguous cases.

💘 Generated with Crush

Assisted-by: Crush:glm-5.3-flash
