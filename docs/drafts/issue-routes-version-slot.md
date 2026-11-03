> [!NOTE]
> This filing was drafted by GLM via Crush from an AI-run investigation, not at my request. When the failure traces to an external report, that source is linked in the body.
>
> - [ ] MANUALLY REVIEWED by `@Lars Artmann` at `[<date-time>]`

TL;DR: `VersionHandler` exists and `WithVersion` stamps probe responses with the build version, but the `Routes` struct has no field for it — so `RegisterRoutes` can never mount `/version` and every consumer hand-registers the endpoint. Suggested change: an empty-skippable `Version string` field on `Routes`, matching the existing "empty pattern is skipped" convention.

## Problem

The building blocks are all there:

- `VersionHandler(version string) http.HandlerFunc` (`version.go:51`)
- `NewChecks(..., WithVersion(...))` wires the version into probe responses

but `Routes` is only three fields (`handlers.go:11-15`):

```go
type Routes struct {
	Liveness  string
	Readiness string
	Startup   string
}
```

So a fleet consumer that wants the conventional `GET /version` endpoint registers it by hand. Mine does (`ChastityAPI`, `internal/app/router.go`, `mux.HandleFunc("GET /version", health.VersionHandler(BuildVersion))`), which is exactly the duplication the `Routes` abstraction exists to prevent.

## Suggested change

```go
type Routes struct {
	Liveness  string
	Readiness string
	Startup   string
	Version   string // empty = not mounted
}
```

`registerRoute` already skips empty patterns (`handlers.go:139-141`), so `DefaultRoutes()` stays conservative (no `/version` unless asked) while `Routes{..., Version: "GET /version"}` becomes a one-liner. With the method-qualified defaults from the sibling issue, `DefaultRoutes().Version = "GET /version"` would also be defensible — the empty-skippable field works either way.

💘 Generated with Crush

Assisted-by: Crush:glm-5.3-flash
