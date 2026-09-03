# Panic-Recovery Criticality — design note

> Decided: 2026-09-04 · Status: DECIDED · Supersedes: open TODO "Decide: should panic recovery treat critical services as fail?"

## Problem

`runHealthChecks` recovers panics from the health-check batch and reports them
as one synthetic `"health-check"` error. That key is not in the critical set,
so `classify` mapped a recovered panic to `warn` — HTTP 200, degraded — even
when the panicking code path was a **critical** service's `HealthCheck`.
A instance whose critical dependency just crashed its check would keep
receiving traffic.

Per-service identity is lost at the recovery point: the batch runs inside one
call (`do.HealthCheckWithContext` or the recorder's equivalent), and a panic
mid-iteration discards both the partial result map and the name of the service
that panicked. The recovery site cannot know what failed — only THAT the
evaluation is incomplete and untrustworthy.

## Decision

**Fail closed.** A recovered panic rolls up to `StatusFail` (readiness 503),
never `warn`.

Mechanics:

1. New exported sentinel `ErrPanicDuringHealthCheck`; the synthetic error is
   `fmt.Errorf("%w: %v", ErrPanicDuringHealthCheck, panicValue)`, so consumers
   can `errors.Is` a recovered panic apart from ordinary service failures.
2. `classify` returns `StatusFail` whenever any result error matches the
   sentinel.
3. Recovery is confined to a small free function (`recoverHealthChecks`) with
   no named returns and no linter suppressions.

## Why fail closed

- **Unchecked is not healthy.** After a panic, every service after the panic
  point was never checked. Reporting warn (200) asserts "degraded but
  serving" over data that was never collected.
- **The worst case must win.** A panic in a critical service is
  indistinguishable from a panic in a non-critical one. warn is only safe if
  the panic was definitely non-critical — which we cannot know.
- **Trust is the product.** A health probe's value is the truthfulness of its
  signal. A corrupted evaluation must not be laundered into a 200.
- **Bounded blast radius.** Readiness 503 stops traffic; liveness stays 200,
  so Kubernetes does not restart the pod. The next successful evaluation
  restores readiness — a panicking check that fixes itself heals without
  operator action.

## Rejected alternatives

- **Per-service recovery (re-run checks one-by-one).** Reimplements
  `do.HealthCheckWithContext` iteration, bypasses the recorder's audit
  semantics, doubles latency on the panic path, and still cannot recover the
  original partial batch. Rejected: complexity and audit drift.
- **Configurable criticality (`WithPanicIsCritical(bool)`).** A config knob
  for a case with only one defensible answer. Rejected: API surface without
  a real decision to defer.
- **Keep warn semantics.** Preserves traffic in the non-critical case, but
  silently misreports the critical case — the exact bug this note closes.

## Behavioral change

Non-critical-service panics now produce 503 (previously 200 warn). This is
intentional and covered by tests
(`TestWithHealthRecorder_PanicRecovered_DoesNotCrash`,
`TestPanic_CriticalService_Fails`, `TestPanic_NonCriticalService_Fails`).
The JSON wire format is unchanged: the synthetic entry remains
`"health-check": {"status": "fail", "error": "..."}`.
