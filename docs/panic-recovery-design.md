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

---

## Addendum 2026-10-09: the evaluation-hook surface

> Decided: 2026-10-09 (Pareto plan v2 A04/A05) · Status: DECIDED

### Problem

`Probe.Evaluate` invokes the `WithEvaluationHook` callback after
classification. That call is a third panic surface, and it was unrecovered: a
panicking hook unwinds whatever frame called `Evaluate` —

- the background refresh loop: goroutine death, **process crash** (the TODO
  row, flagged 2026-09-15 §e6, verified unrecovered 2026-10-08);
- throttled live evaluations and direct `Evaluate` callers: `net/http`
  recovers per-connection, but the handler contract (JSON body, status code)
  is broken and the panic leaks into the host's error log.

### Why this surface differs from the batch surface

The fail-closed rule above binds surfaces where a panic interrupts **evidence
collection**: services after the panic point were never checked, so the result
map is incomplete and must not be laundered into a 200. The hook runs **after**
the response is fully built and classified. A hook panic means the *observer*
(metrics/alerting) failed, not the health evaluation: every service was
checked and graded before the hook ran. Fail-closed here would assert
unhealthiness the probe knows to be false — readiness 503, traffic drained,
for a metrics bug — and at fleet scale (shared hook) it re-couples
availability to a non-essential subsystem, the exact coupling this library
refuses elsewhere (liveness never checks dependencies; non-critical failures
stay 200).

### Options

1. **Document the contract (no recover).** "Hooks must not panic", parity
   with injector-path services. Rejected: the refresh-loop path is
   process-fatal — the hole this decision exists to close — and it treats a
   one-line callback mistake as a crash-the-process offense while recorder
   panics (the analogous consumer-supplied callback) are already recovered.
   Asymmetric and harsh.
2. **Recover + fail-closed** (synthesized fail row, `classify` parity).
   Rejected: it reports a healthy instance as failing because an observer
   broke. The "recovered panics never map to warn" rule is about interrupted
   evidence; here the evidence is complete.
3. **Recover + non-critical synthetic row (warn).** The panic is recovered on
   every path and made *visible* instead of fatal or invisible: the response
   gains a synthetic `"evaluation-hook"` check graded `warn` whose error
   wraps the new sentinel `ErrPanicDuringEvaluationHook`, and the roll-up is
   raised to `warn` at minimum (never lowered: `fail` stays `fail`).
   Readiness stays 200 — degraded-but-serving, which is exactly what
   happened: the services serve, the observability pipeline does not.

### Decision: option 3, plus a defensive clone

Mechanics:

1. When a hook is configured, `Evaluate` calls it with a shallow copy of the
   response whose `Checks` map is a fresh clone. This is not paranoia about
   the happy path — the option contract already forbids retaining or mutating
   the map — it is boundary defense: a recovered hook leaves map integrity
   unverifiable (a hook can mutate and then panic), and a misbehaving hook
   must not corrupt the served or cached response even without panicking.
   Cost: one map copy per evaluation, only for hook-configured probes.
2. The hook call uses the same small-free-function recover pattern as
   `recoverHealthChecks`. On panic, `Evaluate` appends the synthetic
   `"evaluation-hook"` row (`StatusWarn`, error wrapping the sentinel plus
   the panic value) to the pristine response and raises the roll-up via
   `Rank`: `fail` stays `fail`, `pass`/`warn` become `warn`.
3. The synthetic row never carries `Since` or `DurationNanos` (synthetic rows
   never do — same as liveness's empty set and Healthz's startup entry), is
   never in the critical set, and never affects the startup latch.
4. The sentinel is matchable with `errors.Is` at internal sites and tests;
   externally the row's `Error` text carries the sentinel message plus the
   panic value, so dashboards can alert on the row by name or text.

### Consequences

- Process-stable on all three paths (refresh loop, throttled live, direct
  `Evaluate`).
- The background cache stores the warn-marked response: dashboards and
  aggregate/federation consumers see the degraded row and propagate worst-of
  (warn) naturally — no merge changes needed.
- Self-healing: a fixed hook drops the row on the next evaluation; the
  transition tracker is not involved (no `Since` to carry).
- Wire format: additive row on the panic path only; frozen fields unchanged.
- Scope amendment to the rule above: "a recovered panic never maps to `warn`"
  binds the data-collection surface (`runHealthChecks`). The observation
  surface (hook) maps to `warn` by design: complete evidence, failed
  observer.

### Behavioral change (hook surface)

A panicking evaluation hook previously crashed the process (refresh-loop
path) or broke the handler contract (live path). It now yields a 200-warn
response carrying a visible `"evaluation-hook"` row on every affected
surface.
