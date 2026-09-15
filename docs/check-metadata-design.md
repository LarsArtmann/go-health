# Per-Check Since/Duration Metadata — design note

> Decided: 2026-09-15 · Status: DECIDED · Closes: [issue #2](https://github.com/LarsArtmann/go-health/issues/2)

## Problem

`Check` carried exactly two fields: a status enum and an error string. Operators
reading a health surface (dashboard, JSON endpoint) could answer "what is
wrong" but not:

- **how long** has this check been in its current state ("failing since
  14:02"), and
- **how long** did the last execution of this check take.

The go-health-dashboard rendered placeholders for both and approximated a
status-changes timeline from its own sampling clock — a client-side guess that
resets when the *dashboard* restarts and quantizes to the poll interval.

## Decision

Add two fields to `Check`:

```go
type Check struct {
    Status Status `json:"status"`
    Error  string `json:"error,omitempty"`

    // Since is when the probe first observed this check in its current
    // status. Zero (and omitted from JSON) when unknown.
    Since time.Time `json:"since,omitzero"`
    // DurationNanos is how long the most recent execution of this check
    // took, in nanoseconds. Zero (and omitted from JSON) when the
    // executor did not report it.
    DurationNanos int64 `json:"duration_ns,omitzero"`
}
```

(Exact Go declarations in [types.go](../types.go); the sketch above is the
contract.)

### Field 1: `Since` — probe-observed transition time

**Semantics:** the time of the first batch that reported the check in its
*current* status, as observed by this probe. While the status is unchanged,
every response carries the same `Since`; the moment a batch reports a
different status, `Since` restarts at that batch's clock.

**Mechanics:** a `transitionTracker` (mutex-guarded map of
`name → {status, firstSeen}`) is stamped inside `buildChecks`, so every batch
the probe builds — background refresh, throttled/live readiness, startup
evaluations — observes and updates it. Checks absent from a batch are pruned;
if one returns, its `Since` restarts (the gap was unobserved). `Since` uses
the `WithNowFunc` clock seam, so tests get deterministic transitions.

**What it honestly is not:**

- *Not service-reported.* The check seam (`map[string]error`) cannot carry a
  timestamp, and no service knows its own roll-up status (critical vs
  non-critical grading happens in the probe). One source of truth: the probe.
- *Not restart-surviving.* The tracker is in-process memory. After a process
  restart, `Since` is the restart-time observation. This is still strictly
  better than the dashboard's sampling clock, which also resets — but on the
  *dashboard's* restarts, not the checked service's.
- *Not serialized across concurrent batches.* Overlapping evaluations (refresh
  loop + unlatched startup probes) stamp under one lock; an out-of-order
  completion can attribute a transition to the later batch's clock. The error
  is bounded by batch duration and self-corrects on the next transition.
  Sequencing evaluations globally would couple startup probes to the refresh
  loop — not worth it.

### Field 2: `DurationNanos` — executor-reported execution time

**Semantics:** how long the most recent execution of this check took, as
measured by whatever executed it. Zero means unknown and is omitted.

**Mechanics:** samber/do's batch API (`HealthCheckWithContext`) returns only
`map[string]error` — per-check timing is not observable through it, and
reimplementing do's fan-out (pool, per-service timeouts, scope recursion) to
time checks ourselves would fork its semantics. So `Duration` is populated
only where the executor can report it, via one additive seam:

```go
// The executor's raw report for one check.
type CheckDetail struct {
    Err      error        // nil = healthy; the probe still owns grading
    Duration time.Duration // zero = unknown
}
```

- `NewWithDetailedCheck(fn)` where `fn(ctx) map[string]CheckDetail` — the
  metadata-rich variant of `NewWithHealthCheck`. Composed/external check
  functions self-time each dependency.
- A `HealthRecorder` may optionally implement `DetailedHealthRecorder`
  (`RecordDetailedHealthCheckWithContext`) to pass per-check durations through;
  plain recorders keep working unchanged.

The probe still owns classification: `CheckDetail.Err` is graded by the
critical set exactly like plain results; a detailed source cannot set its own
`Status`, `Since`, or `Error` text.

On the raw-injector path `Duration` stays zero (unknown). If samber/do ever
exposes richer batch results, that path can populate it without a wire change.

### Wire format: `omitzero`, `int64` nanoseconds, no pointers

The issue's open question — zero `time.Time` still marshals under
`omitempty`, so strict absence needs a pointer or encoder gymnastics — is
answered by `encoding/json/v2`'s `omitzero`, already the house mechanism
(`Response.Timestamp`): it consults `IsZero()` (`time.Time` has it) or the
type's zero value (`int64` == 0). Both fields therefore disappear cleanly
when unknown, no pointers, and every existing golden payload stays
byte-identical because existing construction paths leave them zero.

A second wire constraint, discovered while implementing (and pinned by
`TestCheck_JSONOmitZero`): **jsonv2 cannot marshal `time.Duration` at all**
— no default representation exists ([go.dev/issue/71631]), no struct-tag
format is accepted (verified empirically against go1.26.7: `int`, `ns`,
`nanoseconds`, … all rejected), and the only escape hatch is the per-call
`json.FormatDurationAsNano` option. A wire field that every consumer must
remember to encode with a special flag — or their re-marshal of a go-health
payload fails — is hostile, and the default representation is an explicitly
undecided stdlib question we must not couple our wire contract to. So the
wire field is a plain `int64` in nanoseconds (`duration_ns`), while the
in-process seam (`CheckDetail.Duration`) stays a Go-idiomatic
`time.Duration`, converted once at `buildChecks`.

Nanoseconds rather than milliseconds (the unit of the pre-existing
`total_latency_ms`, which keeps its unit for compatibility): sub-millisecond
checks are the common case — a warm cache ping is ~400µs — and
`duration_ms` would truncate exactly the signal the field exists to carry
to "unknown". Consumers format for display; the wire stays lossless.

## What this unlocks downstream (per issue #2)

- "failing since 14:02 (17m)" per check — from the service, not the
  dashboard's clock
- status-change timelines reported by the checked process
- honest "stable for 6h" summaries for healthy groups
- per-check latency wherever the executor reports it

[go.dev/issue/71631]: https://go.dev/issue/71631

## Rejected alternatives

- **Service-reported `Since`** — no service knows its graded status, and the
  probe already observes every batch; two clocks for one fact would drift.
- **Batch duration copied onto every check** — a lie; the batch includes fan-out
  wait, and all checks would show the same number.
- **Probe-side per-check timing on the injector path** — requires replacing
  `do.HealthCheckWithContext` with a hand-rolled fan-out over
  `HealthCheckNamedWithContext`, forking do's pool/timeout/scope semantics.
  Rejected for the same class of reason as the panic-recovery split: go-health
  must not silently re-implement samber/do's execution model.
- **`Since`/`DurationNanos` pointers** — `omitzero` achieves strict absence
  without nil-handling spreading to every consumer.
- **`time.Duration` on the wire** — jsonv2 has no marshalable default (or
  tag-format) representation for it; every consumer re-marshaling a payload
  would need `json.FormatDurationAsNano`, and the eventual default is an
  undecided stdlib question (issue 71631).
- **Millisecond `duration_ms`** — truncates sub-millisecond checks to "unknown".
