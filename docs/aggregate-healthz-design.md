# Aggregate `Healthz` parity — design note

|            |                                                                                                    |
| ---------- | -------------------------------------------------------------------------------------------------- |
| **Date**   | 2026-09-18                                                                                         |
| **Status** | ACCEPTED — implemented 2026-09-22 (unit table, property extension, example shipped)                |
| **Inputs** | `Probe.Healthz()` ([accessors.go](../../accessors.go)), `aggregate` merge-on-read, ROADMAP Theme 7 |

## Problem

`health.Probe` exposes a single-endpoint combined handler, `Probe.Healthz()`:
one URL answering "should traffic be routed here?" — 503 while the startup
latch is unset, the readiness roll-up is `fail`, or the process is draining;
200 otherwise. `aggregate.Aggregate` exposes the three kubelet handlers
(`LivenessHandler`, `ReadinessHandler`, `StartupHandler`) but has **no
`Healthz()` equivalent**.

Deployments that put one URL behind an external load balancer (rather than a
kubelet's three probes) therefore cannot use the aggregate as their single
endpoint, even though the aggregate is exactly the shape they want. They must
compose startup + readiness themselves and hope they agree.

## Options

1. **Add `Aggregate.Healthz() http.HandlerFunc`** — parity with `Probe`, pure
   read-side composition of state the aggregate already merges. No locks, no
   goroutines, no wire changes.
2. **Do nothing; document how to compose it.** Adds a documented gap and
   encourages consumers to reimplement the booting/failing/draining logic the
   library already owns.
3. **Add a `WithHealthzPath`-style option that also registers `/healthz` on
   `Routes`.** Overloads the three-probe `Routes` struct and changes route
   registration semantics; rejected as scope creep.

## Decision (proposed)

Option 1. Define the aggregate's answer as the worst of the same three
conditions the root uses, evaluated across sources:

```
Aggregate.Healthz() == 503  iff  any source has an unset startup latch
                              OR the merged roll-up is fail
                              OR any source is shutting down
                     == 200  otherwise, with the merged body
```

Worst-of-N belongs in a single answer because that is the question the
endpoint exists to answer: a load balancer routing to this process must not
route while _any_ embedded source is booting, failing, or draining. `warn`
(non-critical degradation) stays 200, matching `Probe.Healthz()` and
`Aggregate.ReadinessHandler()`.

Implementation reuses the existing read paths — `StartupComplete()` for the
latch, `CachedResponse()` for roll-up + shutdown — so the handler adds no new
state and no new freshness semantics.

## Contract check

- **Additive:** a new method on `Aggregate`; no existing signature, wire
  format, or handler changes. Dashboard consumer coordination: additive, no
  bump required (same rule as the v0.2.0 metadata fields).
- **Consistent with merge-on-read:** one atomic cache load per source, same as
  `ReadinessHandler`.
- **Consistent with root semantics:** the 503 conditions mirror
  `Probe.Healthz()` exactly, with "source" substituted for "the probe".

## Why deferred

It is small and additive, so it could ship in a patch. Deferring keeps each
release purposeful and lets it land with the other v0.3.0 aggregate work
(`errors.Join` construction errors, `SourceStatuses()`). Promote earlier if a
consumer needs a single aggregate endpoint.

## Enforcement (when implemented)

- Unit: a table over {all latched, one unlatched, merged fail, one shutting
  down, warn} asserting 503/200 and that `warn` stays 200.
- Property: extend `aggregate_property_test.go` so `Healthz`'s status code is
  a pure function of `StartupComplete()` and the merged roll-up, for every
  source topology the file already enumerates.
