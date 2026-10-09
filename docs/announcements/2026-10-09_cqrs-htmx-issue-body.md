# Filed body — cqrs-htmx/health: detailed recorder variant (2026-10-09)

**Filed against:** LarsArtmann/cqrs-htmx · **Title:** Add a DetailedHealthRecorder so projection check durations reach the wire

## Problem

`health.NewProbe(provider, opts...)` wires the provider through `health.Recorder(provider)` as a plain `gohealth.HealthRecorder` returning `map[string]error` (`health/probe.go:37`, `:44`, `:55`), so responses carry no `duration_ns` — the consumers behind this bridge get `since` but never per-check timing in dashboards.

## Proposal

Add `DetailedRecorder(provider)` implementing `gohealth.DetailedHealthRecorder` (returning `map[string]gohealth.CheckDetail`), timing each `ProjectionStatuses()` read (`health/probe.go:75`), plus a `NewDetailedProbe` (or an option) that uses it. `ProjectionStatusEntry` already exposes per-projection state; the recorder only needs to time the provider call per projection or share the batch read time.

|                   | today          | desired             |
| ----------------- | -------------- | ------------------- |
| `since` per check | works          | same                |
| `duration_ns`     | always omitted | populated per check |

Classification semantics (live/stopped healthy, failed → infrastructure, draining → transient/warn) stay exactly as they are. Verified against go-health v0.5.0 on 2026-10-09 (originally verified against v0.4.x on 2026-10-02 — the relevant API surface is unchanged). Backward compatible; existing consumers inherit it on their next bump.

💘 Generated with Crush
