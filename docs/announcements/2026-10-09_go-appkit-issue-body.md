# Filed body — go-appkit/health: detailed probe variant (2026-10-09)

**Filed against:** LarsArtmann/go-appkit · **Title:** Add a metadata-preserving probe variant so per-check durations reach the wire

## Problem

`health.NewProbe(checks, opts...)` builds its batch through `go-health.NewWithHealthCheck` (`health/probe.go:52`, `:78`), whose plain batch function cannot report per-check timing: `duration_ns` never reaches the wire, so dashboards that render it adaptively show nothing for the consumers behind this bridge.

`since` works on every path (go-health's probe-observed transition tracking); durations are the only gap. Verified against go-health v0.5.0 on 2026-10-09 (originally verified against v0.4.x on 2026-10-02 — the relevant API surface is unchanged).

## Proposal

go-health already carries the metadata channel: `health.NewWithDetailedCheck` and the `health.DetailedHealthRecorder` interface put `CheckDetail{Err, Duration}` on the wire as `duration_ns`.

|                   | today          | desired             |
| ----------------- | -------------- | ------------------- |
| `since` per check | works          | same                |
| `duration_ns`     | always omitted | populated per check |
| `NewProbe`        | —              | unchanged           |

Either or both:

- add `NewDetailedProbe(map[string]DetailedCheckFunc, opts...)` forwarding to `health.NewWithDetailedCheck` (each check self-times), or
- a timing recorder wrapper implementing `health.DetailedHealthRecorder`.

Backward compatible: existing `NewProbe` unchanged.

💘 Generated with Crush
