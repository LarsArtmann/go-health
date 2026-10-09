# Upstream issue draft — cqrs-htmx/health: detailed recorder variant

**Repo:** LarsArtmann/cqrs-htmx (module `health`) · **Type:** feature · **Filed:** 2026-10-09 as [cqrs-htmx#31](https://github.com/LarsArtmann/cqrs-htmx/issues/31) (voice-checked 0 FAIL/0 WARN; re-verified against v0.5.0 + live `health/probe.go:37,44,55,75` before filing; filed body preserved in [2026-10-09_cqrs-htmx-issue-body.md](2026-10-09_cqrs-htmx-issue-body.md))

## Title

Add a DetailedHealthRecorder so projection check durations reach the wire

## Body

`health.NewProbe(provider, opts...)` builds its probe through a plain
`gohealth.HealthRecorder` returning `map[string]error`, so responses carry no
`duration_ns` — the 14 consumers behind this bridge get `since` but never
per-check timing in dashboards.

Proposal: add `DetailedRecorder(provider)` implementing
`gohealth.DetailedHealthRecorder` (returning
`map[string]gohealth.CheckDetail`), timing each `ProjectionStatuses()` read,
and a `NewDetailedProbe` (or an option) that uses it. `ProjectionStatusEntry`
already exposes per-projection state; the recorder only needs to time the
provider call per projection or share the batch read time.

Classification semantics (live/stopped healthy, failed → infrastructure,
draining → transient/warn) stay exactly as they are. `since` already works on
every path. Verified against go-health v0.4.x 2026-10-02 — see go-health
`docs/bridge-golden-path.md` (gap G-C1).

Backward compatible; v4.7.0–v4.13.0 consumers inherit it on their next bump.
