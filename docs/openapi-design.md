# OpenAPI Schema — design note

> Decided: 2026-09-04 · Status: STATIC SPEC DELIVERED · Resolves: ROADMAP "OpenAPI schema generation for the health response"

## Problem

Consumers of go-health endpoints (API gateways, generated clients, contract
tests) benefit from a machine-readable description of the three probes. The
question: runtime generation (a handler that emits the schema) or a static
specification?

## Decision: static spec in docs, not runtime generation

The response schema is _frozen by the golden-file test_
(`TestReadinessResponse_JSONSnapshot`). A static OpenAPI document can be kept
in lockstep with it, reviewed in the same PR, and served by any static file
server. Runtime generation would add a code path whose only output is a
constant — with real downsides:

- **Wire cost on the probe.** The schema is ~100 lines of YAML; generating it
  per request (or embedding it in the binary as a string) grows the library's
  surface for zero dynamic information.
- **Version skew.** Runtime generation tempts users to serve the schema from
  a different build than the probes, reintroducing exactly the drift the
  static spec prevents.
- **Composition.** An application that wants the schema at
  `/.well-known/health-openapi` mounts one file on its mux; no library option
  needed.

## The spec

[`docs/openapi.yaml`](openapi.yaml) describes `/healthz`, `/readyz`,
`/startupz` with their status-code semantics (200 / 405 / 503) and the
`HealthResponse` schema: `status` (pass/warn/fail), `version`,
`instance_id`, `uptime`, `shutting_down`, `total_latency_ms`, `timestamp`
(RFC 3339, omitted until first evaluation), and the `checks` map with
per-service `status` + optional `error`.

Field presence rules are documented as descriptions, because OpenAPI cannot
express them precisely:

- `shutting_down` and `total_latency_ms` are always present (json/v2 does not
  honor scalar `omitempty` — see `TestReadinessResponse_JSONOmitEmpty`).
- `timestamp` uses `omitzero` and is absent before the first evaluation.
- `version`, `instance_id`, `uptime` are omitted when unset/zero.
- `checks` is always present (possibly an empty object).

## Maintenance rule

Any change to `Response` that alters the golden file must update
`docs/openapi.yaml` in the same commit — the changelog entry must call out the
wire-format change (see `handlers_json_test.go`).
