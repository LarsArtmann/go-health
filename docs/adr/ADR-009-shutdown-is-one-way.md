# ADR-009: Shutdown is a one-way valve

> Status: ACCEPTED · Date: 2026-10-27 · Deciders: maintainer
> Answers fleet-needs question Q3 (`docs/research/2026-10-27_fleet-consumer-needs.md`
> §N7). Implementation: `Probe.Shutdown`, `Probe.MarkShuttingDown`,
> `Probe.Start` in `probe.go`.

## Context

`Shutdown()` and `MarkShuttingDown()` set one `atomic.Bool`. Nothing in the
package ever clears it: readiness answers 503 from the moment either is called,
for the rest of the process's life.

Two facts keep the question alive:

1. **`Start` genuinely re-arms part of the lifecycle.** Calling `Start` after
   `Shutdown` re-validates the configuration and may start a fresh refresh loop
   (the grace-window code captures the cancel function *after* the window
   precisely so a `Start` racing a drain is still stopped by that drain). A
   reader who sees "the loop came back" naturally expects readiness to come
   back with it.
2. **The flag is invisible in the API surface.** There is no `IsShuttingDown()`
   and no way to undo it, so a caller who wants a reusable probe has to infer
   the rule from behavior.

The fleet review asked directly whether `Start` should clear the flag. Re-arming
would be a behavior change for every one of the 36 consumers, so it is a
decision, not a refactor.

## Decision

**Both shutdown entry points are one-way. No API clears the draining state.**
`Start` may re-arm the refresh loop; readiness keeps answering 503, liveness
keeps answering 200.

The supported recoveries are:

- a supervisor that wants the service serving again starts a **new process**;
- a test or hot-reload host that wants a fresh probe **constructs a new probe**
  (`health.New` is microseconds, it carries no process state);
- a `Start` that **failed validation** is retried on the same probe, because
  that path never set the flag.

## Rejected alternatives

| Alternative                                             | Why not                                                                                                                                                                                                                                                                                                                                  |
| ------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Start` clears `shuttingDown`                           | A stray or supervisor-driven restart would return an instance to rotation after it announced departure — the exact failure the flag exists to prevent. It also races a drain that is still in progress: a load balancer mid-drain would see checks pass again and route traffic back to a process that is about to exit. |
| `Probe.Rearm()` / `Probe.Reset()`                       | New public API with no verified adopter, and it makes "a response can get greener without a new evaluation" reachable — the invariant the cache and the startup latch are both built on. Constructing a new probe is strictly simpler and has no shared state to unwind.                                                                  |
| A tri-state lifecycle enum (`starting/serving/draining`) | Bigger surface, more states for every handler to consider, and it still has to answer the same question (does anything clear `draining`?). The enum would buy readability, not semantics.                                                                                                                                                |
| Document it and change nothing                          | That was the status quo; the fleet review shows the ambiguity survives documentation alone. A decision record is the cheap half of the fix — the test below is the other half.                                                                                                                                                           |

## Consequences

- **Fleet contract:** an in-process "restart" of a probe is a new `Probe`.
  Consumers that restart in-process must construct, not re-`Start`.
- **Pinned by tests:** `TestStart_AfterShutdown_RestartsLoopButStaysDown`
  (loop re-arms, readiness 503, liveness 200, cached `ShuttingDown` true) and
  the lifecycle stress test in `probe_lifecycle_test.go`, which interleaves
  `Start`/`Shutdown`/`MarkShuttingDown` under the race detector.
- **A future re-arm is a superseding ADR**, not a silent edit to `Start`. If a
  consumer ever needs it, the record above is the list of hazards it must
  answer first.
- No wire-format or API change: this ADR freezes existing behavior.
