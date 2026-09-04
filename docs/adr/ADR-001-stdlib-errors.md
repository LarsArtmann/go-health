# ADR-001: Stdlib errors only — sentinels wrapped with %w

> Status: ACCEPTED · Date: 2026-08-08 (formalized 2026-09-04) · Deciders: maintainer

## Context

A library's error surface is API. go-health needs to signal config
validation failures (`Validate`, `Start`) and synthesize errors for
recovered panics. The Go ecosystem offers enriched error libraries
(samber/oops, cockroachdb/errors, go-error-family) that add stack traces,
context, and classification.

## Decision

Use the standard library only:

- Sentinel errors via `errors.New` (`ErrInvalidTimeout`,
  `ErrInvalidRefreshInterval`, `ErrPanicDuringHealthCheck`), exported for
  `errors.Is` matching.
- Wrap with `fmt.Errorf("%w: ...", sentinel, context...)` to add the
  offending value and remediation.
- HTTP-facing failures are communicated through status codes, not error
  returns; the library sits at an HTTP boundary where the JSON body is the
  error channel.

## Consequences

- Consumers match errors with plain `errors.Is` — no dependency on any
  error library to integrate with go-health.
- No stack traces or structured context. Accepted: the errors are
  config/validation mistakes a developer fixes immediately, not production
  incidents needing forensics; logs are the host's job (see ADR-002).
- `erraudit ./... --type-aware` is the enforcement gate (baseline: 0
  violations; the `--enforce-*` flags stay off because this project
  deliberately has no library to enforce).

## Alternatives rejected

- **samber/oops** — rich context, but adds a dependency every consumer
  inherits, for errors that never cross a process boundary.
- **go-error-family** — classification machinery irrelevant to a
  three-status wire protocol.
- **Typed error structs** — more surface than three sentinels need.
