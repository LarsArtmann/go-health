# ADR-007: No system-inventory metadata on the probe

> Status: ACCEPTED · Date: 2026-10-02 · Deciders: maintainer
> Formalizes the paperless-ngx `/api/status/` comparison (2026-10-02 session)
> and closes the recurring "add install type / server OS / storage /
> database metadata" request class. Full field-by-field analysis:
> [docs/system-status-vs-probe.md](../system-status-vs-probe.md).

## Context

Feature requests arrive asking the probe to report deployment diagnostics
the way paperless-ngx's staff-only `/api/status/` does: install type
(bare-metal/k8s/docker), `server_os`, storage totals/free, database
engine/URL/migration status, task-runner states. go-health currently exposes
none of these, and every proposed field collides with a core constraint:

- Probe endpoints are **unauthenticated** — kubelets and load balancers poll
  without credentials. OS strings, storage layout, and database DSNs are
  reconnaissance payloads and credential-leak vectors.
- Probe reads are **cached and cheap** (lock-free pointer load); statvfs /
  platform / migration queries per poll reintroduce the load the cache
  exists to prevent.
- Scalars (`Version`, `Uptime`, `InstanceID`, `Timestamp`) deliberately do
  not survive `aggregate`/`federation` merges — they are per-process and
  would lie in a fleet view. Install type and OS would lie harder (a fleet
  dashboard wants the union, not one process's answer).

## Decision

go-health stays a **probe**: pass/warn/fail verdicts, per-check errors,
`since`/`duration_ns` metadata. System inventory is out of the library's
genre, permanently. Consumers who need a status page compose it:

1. run the probe unchanged for k8s traffic;
2. mount [go-health-dashboard](https://github.com/larsartmann/go-health-dashboard)
   (HTML + content-negotiated JSON) for humans;
3. build authenticated, synchronous diagnostics (statvfs, migrations,
   `platform.platform()`) in a staff-only view of the application, free to
   read `probe.CachedResponse()` as one input.

`VersionHandler` remains the single identity surface (build stamp, GET-only,
never 503) — identity, not inventory.

## Consequences

- Scope-drift requests are answered by link, not re-analysis.
- The composition point (dashboard + app-owned status view) is the
  supported path; a `WithSystemInventory`-style option will be rejected.
- The genre boundary is now recorded alongside the other standing
  rejections (content negotiation, logging, weights/circuit breakers,
  multi-tenancy).
