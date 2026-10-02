# Upstream issue draft — go-appkit/health: detailed probe variant

**Repo:** LarsArtmann/go-appkit (module `health`) · **Type:** feature · **Filing:** owner call

## Title

Add a metadata-preserving probe variant so per-check durations reach the wire

## Body

`NewProbe(checks, opts...)` forwards to `go-health.NewWithHealthCheck`, whose
plain batch function reports zero per-check durations — `duration_ns` never
appears in responses, so dashboards rendering it adaptively show nothing for
the 2 consumers behind this bridge.

go-health's `NewWithDetailedCheck` + `DetailedHealthRecorder` already carry
`CheckDetail{Err, Duration}` to the wire. Proposal:

- add `NewDetailedProbe(map[string]DetailedCheckFunc, opts...)` forwarding to
  `health.NewWithDetailedCheck` (each check self-times), and/or
- a timing recorder wrapper implementing `health.DetailedHealthRecorder`.

`since` already works on every path (probe-observed transition tracking), so
durations are the only gap. Verified against go-health v0.4.x 2026-10-02 —
see go-health `docs/bridge-golden-path.md` (gap G-A1).

Backward compatible: existing `NewProbe` unchanged.
