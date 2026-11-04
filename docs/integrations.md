# Integrations Index

> One row per integration surface, so recipes do not fragment into orphan
> files (ROADMAP §2, from the SystemNix assessment). Status distinguishes a
> proven recipe from a planned one; planned rows name their source.

| Surface                       | Document                                                               | Status                                                                |
| ----------------------------- | ---------------------------------------------------------------------- | --------------------------------------------------------------------- |
| httputil (HTTP middleware)    | [httputil-integration.md](httputil-integration.md)                     | Proven: `testdata/httputil-interop` via `nix run .#interop` + CI step |
| Prometheus exposition         | [prometheus-exposition-design.md](prometheus-exposition-design.md)     | Composition pattern verified (`prometheus_example_test.go`)           |
| OpenAPI wire contract         | [openapi-design.md](openapi-design.md) + [openapi.yaml](openapi.yaml)  | Static spec, enforced by the `openapi-lockstep` gate                  |
| Gatus (external probing)      | — (planned; the proven `cv.nix` pattern from the SystemNix assessment) | Planned: ROADMAP §2                                                   |
| OpenTelemetry spans on checks | — (planned; top verified gap, non-blocking hook adapter)               | Planned: ROADMAP §2                                                   |
| node_exporter textfile        | — (planned; snippet next to the Prometheus example)                    | Planned: ROADMAP §2                                                   |
