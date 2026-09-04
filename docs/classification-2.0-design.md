# Classification 2.0 — design notes

> Decided: 2026-09-04 · Status: ANALYZED, ALL REJECTED OR DEFERRED (seams documented) · Resolves: ROADMAP Theme 2 "weights / priorities" and Theme 3 "circuit-breaker, MaxConcurrent, per-service caching"

Four ideas that would generalize the binary critical/non-critical model.
Each is analyzed against the same three constraints: kubelet consumes status
codes, not JSON; the batch evaluation is owned by `samber/do`; and every
added knob lands on every consumer's construction call.

## 1. Weights / priorities (Theme 2)

**Idea.** Replace binary criticality with per-service weights; roll-up by
weighted score with configurable warn/fail thresholds.

**Verdict: rejected.** The status code is binary at every consumer boundary:
kubelet maps non-2xx to "not ready", and `warn` already encodes "partial
degradation" as 200-with-detail. A weighted score adds a threshold-tuning
problem (which score is "warn"?) with no wire-format change to show for it —
two deployments with identical services but different weights would disagree
on the same failure set. Composition covers the real use case: a service that
matters "somewhat" belongs in the non-critical set, or — when a deployment
genuinely has three tiers — in its own `Probe`, merged via
`health/aggregate` (worst-of-N roll-up, namespaced checks). Tier modeling
belongs to topology, not to a score function.

## 2. Circuit-breaker for flapping dependencies (Theme 3)

**Idea.** Integrate a breaker (e.g. failsafe-go) so a flapping dependency
stops being re-checked, reporting its cached verdict instead.

**Verdict: rejected for core; dependency-wrapper is the correct seam.** A
breaker is stateful policy (failure threshold, open duration, half-open
probing) whose correct scope is *the dependency client*, not the probe: the
same flapping dependency that poisons health checks also poisons request
paths, and both want the same breaker instance. Wrapping the service's
`HealthCheck` method (or its client) at `do.ProvideNamed` registration time
gives that for free — go-health then observes a stable verdict without
knowing a breaker exists. Inside the probe it would also interact badly with
caching: a scrape-driven probe would see identical cached failures and count
them as fresh evidence, tripping breakers on polling artifacts.

## 3. `WithMaxConcurrentChecks(n)` (Theme 3)

**Idea.** Bound the parallelism of a health-check batch.

**Verdict: rejected.** The batch is owned by `samber/do`
(`HealthCheckWithContext` fans out internally); bounding it from the probe
would mean reimplementing do's iteration and losing its timeout and audit
semantics. The motive is also weak: a health batch has one service per
dependency — dozens, not thousands — each already deadline-bounded. When a
dependency's own check is expensive, the fix is at that dependency (cache its
check, lighten its probe), not in batch scheduling. The recorder seam remains
the escape hatch: a `HealthRecorder` fully owns batch execution and may
serialize or pool as it sees fit.

## 4. Per-service result caching (Theme 3) + per-service latency (P28 finding)

**Idea.** Cache each service's verdict under its own TTL, re-evaluating only
stale entries; and expose per-service latency in `Check`.

**Both blocked by the same fact, verified in P28: `samber/do` owns batch
execution and returns only `map[string]error`.** The probe never sees
per-service timing or completion moments, and do v2 exposes no per-service
hook. Concretely:

- *Per-service latency* is infeasible in core. Viable paths, in order of
  preference: (a) time inside the service's own `HealthCheck` and expose it
  in the error string or via the service's own metrics; (b) a timing
  `HealthRecorder` (it sees the batch result, though not per-service splits —
  only batch wall time); (c) upstream `samber/do` feature.
- *Per-service caching* would require splitting the batch do runs — same
  blocker — and the payoff is marginal: batch-level caching (1s refresh)
  already bounds dependency load to one evaluation per second *total*, and
  kubelet probe intervals are seconds. A caching `HealthRecorder` is the
  composition-layer answer if a deployment ever needs asymmetric TTLs.

**Net:** `Response.TotalLatencyMs` stays the only latency signal in core. The
`float64` upgrade (Theme 2) remains trivially possible later — `Evaluate`
already measures with the injected clock — but is deferred until a consumer
needs sub-millisecond precision.
