# ADR-002: Zero logging coupling

> Status: ACCEPTED · Date: 2026-08-08 (formalized 2026-09-04) · Deciders: maintainer

## Context

Health probes run inside a host application that already has a logging
stack (or deliberately none). The library must emit operational messages —
write failures on client disconnect, background loop exits, recovered
panics — and must choose whether to log them itself.

## Decision

The library imports no logging package (`log/slog`, zap, zerolog, ...).
Nothing is logged. Operational events are surfaced through the API instead:

- Client disconnects after the status line is committed: silently ignored
  (`_, _ = w.Write`, annotated `//nolint:erraudit` — there is no recovery
  action and the status is already sent).
- Recovered panics: become a synthetic `health-check` failure in the
  response body, visible to whoever polls the endpoint.
- Background refresh loop: exits when the Start context is cancelled; its
  lifetime is owned by the caller's context.

## Consequences

- A library never makes logging decisions for the host: no format drift, no
  dependency injection surface for loggers, no risk of logging sensitive
  error payloads.
- Diagnosing subtle failures requires reading health responses or wiring a
  `HealthRecorder`. Accepted: that is exactly the observation surface the
  library already provides.

## Alternatives rejected

- **`log/slog` default logger** — writes to stderr uninvited; hostile to
  library consumers.
- **Optional logger injection (`WithLogger`)** — an API surface, a nil-check
  tax on every event, and a slippery slope toward structured logging
  dependencies.
