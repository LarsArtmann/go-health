# ADR-006: Duration units split by granularity — roll-ups in ms, per-check in ns

> Status: ACCEPTED · Date: 2026-09-22 · Deciders: maintainer
> Resolves the "unify latency units" question (TODO `09-15` §f25). The two
> units both shipped before this record: `total_latency_ms` since v0.1.x,
> `duration_ns` since v0.2.0. This ADR decides whether v0.x should unify them
> while the wire format is still alpha, and codifies the rule future duration
> fields must follow.

## Context

One health document carries three duration shapes:

| Field              | Type on the wire     | Present when                                                                                 | Meaning                                       |
| ------------------ | -------------------- | -------------------------------------------------------------------------------------------- | --------------------------------------------- |
| `total_latency_ms` | `int64` milliseconds | always (json/v2 ignores scalar `omitempty`; pinned by `TestReadinessResponse_JSONOmitEmpty`) | Wall-clock of the whole batch; 0 for liveness |
| `duration_ns`      | `int64` nanoseconds  | only when known (`omitzero`)                                                                 | One check's most recent execution             |
| `uptime`           | human string         | when non-zero                                                                                | Time since boot (`"4m5s"`)                    |

In-process, a fourth shape exists by design: `CheckDetail.Duration` is a
`time.Duration`, converted to ns exactly once in `buildChecks`
([docs/check-metadata-design.md](../check-metadata-design.md)).

The question: two integer units for "how long" is a cognitive tax — every
renderer needs two formatters. Should v0.x unify (`total_latency_ns`) while
breaking changes are still allowed, or keep the split?

## Decision

**Keep both units. Codify the split as the unit policy:**

> **Roll-up scalars — quantities a human scans on a dashboard — are whole
> milliseconds (`int64`, always present). Per-entity measurements — data a
> machine diffs over time — are nanoseconds (`int64`, `omitzero` when
> unknown).**

The split follows the scale of the things measured:

- A healthy check completes in nanoseconds to microseconds. In milliseconds,
  every healthy check would render `0` — the field would carry no signal at
  all. Nanoseconds is what the executor measures (`time.Since` granularity)
  and the only integer unit that survives sub-millisecond checks.
- A batch total spans milliseconds to the configured timeout budget (seconds).
  In nanoseconds, `"total_latency_ns": 42000000` is hostile to the exact
  audience that reads it by eye: an operator scanning a readiness payload or
  a dashboard cell. Milliseconds keeps the number legible.

The always-present/omitzero asymmetry is intentional, not an accident to fix:
a roll-up is always known (liveness's `0` means "no batch ran", distinguishable
by endpoint), while a per-check duration is genuinely unknown on the raw
injector path (do's `HealthCheckWithContext` returns only errors) and must be
absent, not zero — `0 ns` would read as "instantly healthy", a lie.

## Consequences

- No wire-format change, no dashboard coordination, no consumer migration.
  The golden file, `TestReadinessResponse_JSONOmitEmpty`,
  `TestCheck_JSONOmitZero`, and the spec already pin today's shape.
- Future duration fields inherit the rule instead of a new debate: e.g. a
  per-remote fetch-latency roll-up in `federation` would be
  `..._ms`, always present; a per-check retry count's dwell time would be
  `..._ns`, `omitzero`.
- Renderers keep two formatters. That is the accepted cost; the alternative
  costs more (below).
- `uptime` stays a human string: it exists for eyeballs only and no consumer
  computes on it.

## Alternatives rejected

- **Unify on nanoseconds (`total_latency_ns`).** A breaking rename (golden
  file, OpenAPI spec, lockstep check, dashboard rendering, operator muscle
  memory) whose only gain is symmetry — paid in a worse answer to the field's
  actual question. The number humans scan becomes unreadable.
- **Unify on milliseconds (`duration_ms`).** Destroys the per-check data:
  sub-millisecond executions — the overwhelming majority of healthy checks —
  collapse to `0`. The field's entire purpose is fine-grained regression
  detection.
- **`time.Duration` on the wire.** `encoding/json/v2` cannot marshal it at
  all: no default representation exists (go.dev/issue/71631, undecided
  upstream), no struct-tag format is accepted (verified empirically), and the
  only escape is a per-call marshal option every consumer would have to know.
  The ns-`int64` escape is already taken for `Check.DurationNanos`.
- **Float seconds (Prometheus style).** Suits a metrics exposition negotiated
  per scrape; the health document is a stable JSON contract where float
  durations invite rounding debates and break `int64` round-trip identity
  (`TestResponseJSONRoundTripIdentity`).

## Enforcement

- Wire truth: `testdata/readiness_response.golden` +
  `TestReadinessResponse_JSONOmitEmpty` (always-present roll-up) +
  `TestCheck_JSONOmitZero` (`duration_ns` absent when unknown).
- Spec: `docs/openapi.yaml` declares both fields with explicit units;
  `checks.openapi-lockstep` (runs under `nix flake check`) makes spec↔wire
  drift on either field loud.
- Review rule: any new duration field violating the ms/ns split needs a new
  ADR, not a quiet tag.
