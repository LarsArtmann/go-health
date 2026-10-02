# Design: `health.Setup` / `MustNew` convenience constructors

**Date:** 2026-10-02 · **Status:** REJECTED (for now)

## Proposal (from the 2026-10-02 DX analysis)

New integrators must read several pages to assemble a working probe: `New`
vs the standalone constructors, `Start` + error check, `RegisterRoutes` +
`DefaultRoutes`, `Shutdown`. A convenience API was floated:

```go
probe, err := health.Setup(health.WithCriticalServices("db"), health.WithHealthHTTP(":8080"))
// or
probe := health.MustNew(...)
```

## Why rejected

1. **The awkward step is doing its job.** The two-error surface (`Start`
   returning `ErrInvalidTimeout`/`ErrUnknownCriticalService`) exists so a
   misconfigured probe fails loudly at boot. `MustNew`-style helpers that
   swallow the error or defer it invite the exact silent-boot failure modes
   this library exists to prevent.
2. **HTTP ownership is deliberate.** The library never binds sockets or
   picks ports (zero-logging, composition-over-configuration posture; see
   docs/middleware-design.md). A `WithHealthHTTP(addr)` option would smuggle
   a server lifecycle into a probe — a new shutdown ordering problem
   (listener vs probe drain) for a three-line saving.
3. **Constructor choice is already small.** The README "Which constructor
   should I use?" table (2026-10-02) resolves the five-way choice in one
   screen; a `Setup` wrapper hiding that choice would need every option
   anyway to stay useful.
4. **API-surface cost is permanent.** Every constructor added is a promise
   to keep wire- and semantics-compatible across the 30-repo consumer fleet.
   Convenience must clear a high bar.

## When to revisit

If a fresh-user simulation (plan task F3) shows real, repeated assembly
mistakes that the golden path does not prevent, reconsider a minimal
`Setup(injector, opts...) (*Probe, error)` that is exactly `New` + `Start`
with no HTTP ownership — and nothing more.
