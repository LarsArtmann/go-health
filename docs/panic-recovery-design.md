# Panic-Recovery Criticality — design note

> Decided: 2026-09-04 · Status: DECIDED · Supersedes: open TODO "Decide: should panic recovery treat critical services as fail?"

## Problem

`runHealthChecks` recovers panics from the health-check batch and reports them
as one synthetic `"health-check"` error. That key is not in the critical set,
so `classify` mapped a recovered panic to `warn` — HTTP 200, degraded. An
instance whose health evaluation just blew up would keep receiving traffic.

## Two panic surfaces (empirically verified)

**1. Recorder path — recoverable.** With `WithHealthRecorder(r)`, the recorder
runs inside `runHealthChecks`' call frame. A panicking recorder (or a panic in
`samber/do`'s batch entry itself) hits the probe's `recover` and is converted
to the synthetic error. `TestWithHealthRecorder_PanicRecovered_DoesNotCrash`
pins this.

**2. Injector path — process-fatal, NOT recoverable.** Without a recorder,
`do.HealthCheckWithContext` runs each service's `HealthCheck` in its own
goroutine (`raceWithTimeout` in samber/do v2.1.0). A panic in a service's
`HealthCheck` unwinds that goroutine — no probe-side `recover` can catch it;
the process crashes. This is samber/do's design, not a gap go-health can close
without reimplementing do's iteration (and losing its timeout/audit
semantics).

## Decision (recoverable surface): fail closed

A recovered panic rolls up to `StatusFail` (readiness 503), never `warn`.

Mechanics:

1. Exported sentinel `ErrPanicDuringHealthCheck`; the synthetic error is
   `fmt.Errorf("%w: %v", ErrPanicDuringHealthCheck, panicValue)`, so consumers
   can `errors.Is` a recovered panic apart from ordinary service failures.
2. `classify` returns `StatusFail` whenever any result error matches the
   sentinel.
3. `buildChecks` grades the synthetic `"health-check"` entry `fail` (not
   `warn`) so the JSON body agrees with the roll-up status.
4. Recovery is confined to a small free function (`recoverHealthChecks`) with
   no named returns and no linter suppressions.

### Why fail closed

- **Unchecked is not healthy.** After a panic, services after the panic point
  were never checked. Reporting warn (200) asserts "degraded but serving"
  over data that was never collected.
- **The worst case must win.** The recovery site cannot know which service
  panicked. warn is only safe if the panic was definitely non-critical.
- **Trust is the product.** A health probe's value is the truthfulness of its
  signal. A corrupted evaluation must not be laundered into a 200.
- **Bounded blast radius.** Readiness 503 stops traffic; liveness stays 200,
  so Kubernetes does not restart the pod. A panic that heals restores
  readiness on the next evaluation without operator action.

### Rejected alternatives

- **Per-service recovery.** Reimplements `do.HealthCheckWithContext`
  iteration, bypasses recorder audit semantics, doubles latency on the panic
  path — and cannot help the injector path at all (goroutine panics).
- **Configurable criticality (`WithPanicIsCritical`).** A knob for a case
  with one defensible answer.
- **Keep warn semantics.** Preserves traffic in the non-critical case but
  silently misreports corrupted evaluations.

## Injector-path guidance

A service whose `HealthCheck` panics will crash the process. Health checks
must be total functions: recover inside your own service if its check can
panic. The synthetic-error path only ever fires for recorder implementations
and for panics in the batch machinery itself.

## Behavioral change

Recovered panics now produce 503 with the `"health-check"` entry graded
`fail` (previously 200 warn / entry `warn`). JSON wire format unchanged.
