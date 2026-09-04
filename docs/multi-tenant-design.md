# Multi-Tenant Probes — design note

> Decided: 2026-09-04 · Status: ANALYZED, COVERED BY EXISTING PIECES · Resolves: ROADMAP Theme 4 "child-scope isolation", "WithProbeName", "WithCriticalService toggle"

## Context

"Multi-tenant" here means: one process hosts health for several logical
services or tenants, each with its own dependency set and criticality
configuration, while operations wants one health surface. Three proposed
library features are analyzed against what already ships.

## 1. Child-scope isolation — solved by N probes + aggregate

A `Probe` is cheap: one struct, no goroutine unless refresh caching is on,
one atomic pointer read per handler hit. There is nothing to pool. The
isolation a "child scope" would provide is already available by building one
`Probe` per tenant (via `New` on a per-tenant injector, or
`NewWithHealthCheck` on a per-tenant check function) and merging:

```go
agg, err := aggregate.New(
    aggregate.Source{Name: "tenant-a", Probe: probeA},
    aggregate.Source{Name: "tenant-b", Probe: probeB},
)
```

Checks are namespaced `source/check`, the roll-up is worst-of-N, and one
shutting-down source fails the aggregate — all semantics a bespoke
child-scope feature would have to reinvent. Verified against
`aggregate/aggregate.go` (construction validates unique non-empty names;
merging is lock-free on read).

## 2. `WithProbeName` — redundant with aggregate namespacing and `WithInstanceID`

A probe-name string on every response would identify *which* probe produced
it. But: single-probe deployments already identify the response (it is the
only one); multi-probe deployments are aggregate deployments, where
`Source.Name` already prefixes every check key; and replica identity — the
part that actually varies across a fleet — is `WithInstanceID` (P36). Adding
`probe_name` now would create a third identity field with overlapping
semantics and an aggregate-merge rule nobody has specified (whose name wins
the top-level field?). Rejected until a concrete deployment shows a need that
namespaced checks do not meet.

## 3. `WithCriticalService(name, critical bool)` runtime toggle — rejected; immutability is the feature

Toggling criticality on a live probe would make the criticality set mutable
while evaluators read it concurrently. Today the `classifier` (extracted in
P34) is written once at construction and read lock-free — the evaluate path
touches no mutex for classification. A toggle needs a synchronization
scheme (RWMutex or atomic map copy) on every evaluation to support a
configuration change that is better expressed as *rebuild the probe*: probes
are value-cheap, construction validates eagerly, and the old probe's cache
dies with it instead of leaving half-toggled state. Kubernetes config reload
patterns (rebuild and swap on the mux) compose with this; in-place mutation
fights it. The aggregate package additionally demonstrates that combining
probes — not mutating them — is the intended composition axis.
