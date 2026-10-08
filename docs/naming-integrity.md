# Naming integrity: non-breaking docs now, staged renames for v0.5

**Date:** 2026-10-02 · **Status:** ADOPTED (docs portion shipped; renames re-venued 2026-10-08 to the v0.6 window — v0.5.0 shipped without them; body retains its original v0.5 framing). Automated smell detection: clean (0 findings, naming-smells.sh).

## Non-breaking: document the two confusable names (shipped)

`CheckFunc` vs `HealthCheckFunc` — distinct roles, easy to confuse:

- **`health.CheckFunc`** (checks.go) — one _named check_ passed to
  `NewChecks`: `func(ctx) error`. You write these.
- **`health.HealthCheckFunc`** (accessors.go) — a whole _batch executor_
  passed to `NewWithHealthCheck`: `func(ctx) map[string]error`. You write
  this when you own the batch (composed checks, foreign DI containers).
- `DetailedHealthCheckFunc` is the batch executor's metadata-rich variant;
  `DetailedHealthRecorder` the recorder-side equivalent. Rule of thumb:
  *Func = you call the probe with it; *Recorder = the probe calls you.

Unit-suffix convention (ADR-006 restated): wire fields carry unit suffixes
(`total_latency_ms`, `duration_ns`); Go fields carry the unit when it is
not the obvious one (`DurationNanos`, `TotalLatencyMs`); `time.Duration`
only in-process (`CheckDetail.Duration`).

## Breaking: staged v0.5 rename list

| Now                                | v0.5                                        | Why                                                                                                                                |
| ---------------------------------- | ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `SanitizeResponse`                 | `CoerceValidUTF8`                           | The current name is a euphemism: the function does one thing (replaces invalid UTF-8) and does not sanitize in any security sense. |
| `Check.Since` (JSON `since`)       | Go field `StatusSince` (JSON stays `since`) | "Since when has the check been in its current status" — the JSON key is honest, the Go name is vague. Wire untouched.              |
| `WithGETOnly`                      | **remove** (deprecated since v0.1.1)        | One verified consumer (KeyHolderAI legacy path, adoption matrix); nudge first, then drop.                                          |
| `WithCriticalServices(...string)`  | `(...ServiceName)`                          | docs/servicename-design.md.                                                                                                        |
| `Probe`/`Prober`/`Source`/`Remote` | one "health source" lexicon                 | docs/vocabulary-reconciliation.md.                                                                                                 |

Process per rename: deprecation-policy.md checklist (doc note + SA1019
deprecation marker one minor release before removal where possible), fleet
call sites pre-enumerated from the adoption matrix, CHANGELOG entry with
mechanical migration snippet.
