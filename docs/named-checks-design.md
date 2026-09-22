# Named Checks Without a Container — design note

> Decided: 2026-09-19 · Status: DECIDED · Vehicle: v0.3.0

## Problem

`New(injector, ...)` requires a samber/do v2 container, and the v0.2.0
injector-free constructors (`NewWithHealthCheck`, `NewWithDetailedCheck`) hand
the caller the whole batch: a static binary that just wants "three named
lambdas, run concurrently, honest results" must hand-roll its own executor —
and typically gets timing, panic recovery, or hang containment wrong.

The trigger was webphone (single-binary Go app, deliberately no DI container —
see its AGENTS.md): its readiness story needed a named-check probe, and the
only zero-boilerplate path was a container it rejected.

## Decision

Add `NewChecks` in a new `checks.go`:

```go
type CheckFunc func(ctx context.Context) error

func NewChecks(checks map[string]CheckFunc, opts ...Option) *Probe
```

It adapts the map into a `DetailedHealthCheckFunc` and funnels through the
same `assemble`/`config` path as every other constructor, so classification,
caching, handlers, shutdown awareness, and validation are unchanged.

### Per-check failure contract (`runBoundedCheck`)

| Failure                       | Behavior                                                         |
| ----------------------------- | ---------------------------------------------------------------- |
| Check returns nil             | pass                                                             |
| Check returns error           | error surfaced under its name, graded against the critical set   |
| Check panics                  | recovered; `"check %q panicked: %v"` error, siblings unaffected  |
| Check is nil                  | fail-closed `"check %q is nil"` error                            |
| Check ignores ctx, batch ends | abandoned; `"check %q did not finish before the batch deadline"` |

The abandonment branch is what samber/do's native health checks cannot do:
a goroutine that never returns cannot be killed, but the batch stops waiting
at the context deadline and reports the straggler as failed. `Probe.Evaluate`
callers that bypass the handlers/`Start` must bound their own context
(handlers and the refresh loop apply `WithTimeout` automatically) — same
contract the probe already documents.

Durations are executor-measured, so `duration_ns` is populated — something the
raw injector path cannot do (see `docs/check-metadata-design.md`).

## Non-goals

- No per-check timeout knob: the batch deadline bounds everything; per-check
  isolation stays the container's job (`do.WithHealthCheckTimeout`).
- No ordered/dependent checks: health checks are independent by definition.
- No API change to `New`/recorders: purely additive.

## Verification

`checks_test.go` pins all five contract rows plus concurrency (barrier
deadlock test — a serial runner cannot pass), duration population, empty-map
pass, and a doc example. Full suite with `-race` green before release.
