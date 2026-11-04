## What this is

The routes release. `DefaultRoutes()` now mounts the build-version endpoint
(`GET /version`) alongside the three kubelet probes, every default pattern is
method-qualified so registration can no longer conflict with common
page-router shapes, and a version endpoint that would answer an empty version
can no longer exist. Two **breaking behavior changes** land in this 0.x minor;
both fail fast at registration — boot time, before the first request.

## The two breaking changes — read before bumping

**1. `DefaultRoutes()` mounts `GET /version` and requires a build stamp.**

A probe registering the defaults must be stamped with `WithVersion(...)`,
otherwise `RegisterRoutes` panics: an endpoint answering `{"version":""}`
would lie about the build. If you hand-wire `/version` today, **remove that
line** — the default mount replaces it (keeping both panics with a duplicate
pattern).

```go
// before (v0.9.x)
probe := health.New(injector, health.WithVersion(version))
mux.HandleFunc("/version", health.VersionHandler(version)) // hand-wired
probe.RegisterRoutes(mux, health.DefaultRoutes())

// after (v0.10.0)
probe := health.New(injector, health.WithVersion(version))
probe.RegisterRoutes(mux, health.DefaultRoutes()) // GET /version included
```

Not stamping? Opt out explicitly: `health.DefaultRoutes()` with an empty
`Routes.Version` (struct literal) keeps `/version` unmounted.

**2. `DefaultRoutes()` patterns are method-qualified.**

`/healthz`, `/readyz`, `/startupz` are now `GET /healthz`, `GET /readyz`,
`GET /startupz` (plus `GET /version`). A bare pattern matches more methods
but a less specific path, which panics at registration against common shapes
like a `GET /{slug}` page route — the kubelet probes are always GET, so the
defaults lose nothing and coexistence becomes the default. Code that read
`DefaultRoutes()` fields as request paths must switch to the literal paths.

## Also in this release

- `health.Routes.Version` mounts `VersionHandler` wherever you point it;
  `aggregate` and `federation` carry no build identity and ignore the field
  (they never serve a made-up version — wire `VersionHandler` yourself if you
  stamp those processes).
- Internal file layout: twelve oversized files split along their concern
  seams (zero exported-API or wire-format changes).
- Gate-tool audit: the fuzz-registry, security, coverage-gate, recorder-audit
  and openapi-lockstep flake apps now declare every external command they run,
  so a stripped-PATH environment fails loudly instead of lending tools from
  the host shell.

Gates at cut: full sweep green (test-race, vet, lint 0 issues, coverage 99.9%
total / 100% root, vulncheck clean, gosec 0, all 9 fuzz targets, interop,
docs-check). Wire format unchanged — this release changes routes, not bytes.
