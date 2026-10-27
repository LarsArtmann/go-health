# ADR-008: A recovered panic always fails closed

> Status: ACCEPTED · Date: 2026-10-27 · Deciders: maintainer
> Fixes go-health#7 finding 1 and settles the grading question for every
> recovery surface. Implementation: `classifier.isRecoveredPanic`,
> `probe.go` (`runHealthChecks`), `checks.go` (`runBoundedCheck`).

## Context

go-health recovers panics at three independent surfaces, and each one has to
decide what status the recovered panic deserves:

| Surface                          | Recovery point                      | Panic sentinel                               |
| -------------------------------- | ----------------------------------- | -------------------------------------------- |
| Recorder/injector batch          | `runHealthChecks`                   | `ErrPanicDuringHealthCheck`                  |
| `NewChecks` per-check executor   | `runBoundedCheck`                   | `ErrCheckPanicked`                           |
| `WithEvaluationHook` observer    | hook invocation in `probe.go`       | `ErrPanicDuringEvaluationHook`               |

The grading rule for the batch and hook surfaces was settled earlier: a
recovered panic reads as **fail** for the batch (a check that panicked did not
pass) and as **warn** for the hook (the evaluation itself completed before the
observer ran, so the verdict it produced is still true).

The `NewChecks` executor was the gap. Its sentinel (`ErrCheckPanicked`) was
produced with a correct, documented intent — its godoc says "a recovered panic
must read as a failure, never a warning" — but `classifier.isRecoveredPanic`
matched only `ErrPanicDuringHealthCheck`. So a panicking `NewChecks` check was
graded by the ordinary non-critical rule: **warn**, and the roll-up stayed
`warn`. A check that crashed the process's control flow reported as a
degradation.

That mattered beyond cosmetics. The roll-up is what readiness answers on:
`warn` stays HTTP 200, so a check whose executor panicked kept the instance in
rotation while reporting green-ish. The dashboard family then renders that as
a warning chip, which is exactly the "greens that cannot be proven" shape the
evidence surface exists to expose.

## Decision

**A recovered panic fails closed at every surface where it replaces a check's
verdict.** `isRecoveredPanic` matches both batch sentinels
(`ErrPanicDuringHealthCheck` and `ErrCheckPanicked`), so a non-critical
panicking check grades the roll-up to `fail` and readiness answers 503.

The `WithEvaluationHook` surface keeps its earlier, deliberately different
grading (`warn`): the hook is an observer that runs *after* the verdict was
computed, so its panic cannot have corrupted the answer. Recording it as a
failure would take a healthy instance out of rotation because a metrics
shipper crashed.

Criticality still decides *which* status a healthy-or-failing check
contributes; it no longer decides whether a panicking one gets to look
non-fatal.

## Consequences

- **Behavior change, no compile break.** A non-critical check that panics now
  rolls up `fail` (503) instead of `warn` (200). The only consumers affected
  are those with a panicking check — a defect either way. CHANGELOG carries
  the migration note; `go-appkit/health` pinned the old behavior in
  `probe_test.go:111` and flips that pin in the same change that bumps past
  v0.8.0.
- **Two sentinels, one rule.** Future recovery surfaces compare against
  `isRecoveredPanic` rather than re-deriving the grading, so the decision
  cannot drift again between executors.
- **The hook exception is now documented as an exception**, not as an
  oversight: an observer's panic is `warn`, a check's panic is `fail`.
- Pinned by `TestNewChecks_NonCriticalPanicFailsClosed` (a non-critical
  panicking check fails the roll-up) alongside the pre-existing batch and hook
  tests.
