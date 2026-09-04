# ADR-003: Three-state classification (pass / warn / fail)

> Status: ACCEPTED · Date: 2026-08-08 (formalized 2026-09-04) · Deciders: maintainer

## Context

Kubernetes probes are binary (HTTP 200 vs 503), but real dependency tops are
not: a failed metrics exporter degrades observability without removing the
instance from rotation, while a failed database makes the instance useless.
A two-state roll-up conflates "degraded but serving" with "cannot serve",
and handlers lose the ability to warn humans while keeping traffic flowing.

## Decision

`classify` rolls every health-check batch into exactly one of three states:

| Status | Meaning | Readiness HTTP | Triggered by |
| ------ | ------- | -------------- | ------------ |
| `pass`  | fully healthy | 200 | every checked service healthy |
| `warn`  | degraded but serving | 200 | ≥1 non-critical failure, no critical failure |
| `fail`  | cannot serve | 503 | any critical failure, shutdown, or recovered panic |

Criticality is a static, construction-time set (`WithCriticalServices`).
The startup probe consumes the same results through a latch (first
all-criticals-pass wins, permanent).

## Consequences

- Consumers get a stable tri-state enum in JSON (`"status"`) and can build
  dashboards on `warn` without scraping per-check errors.
- The two HTTP outcomes (200/503) remain a pure function of the tri-state,
  so Kubernetes semantics stay predictable.
- Criticality is not dynamic: changing the critical set requires a new
  probe. Accepted — dynamic criticality made roll-ups impossible to reason
  about during incidents.

## Alternatives rejected

- **Two-state only** — hides degradation or over-alerts on it.
- **Numeric health scores** — unfalsifiable thresholds; a probe must be
  auditable at 3am.
- **Per-check HTTP override hooks** — moves the classification decision
  into every consumer's code.
