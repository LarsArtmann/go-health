# Response ownership: `CachedResponse` is read-only, `Response.Clone` is the copy

**Date:** 2026-10-27 · **Status:** IMPLEMENTED (additive, unreleased)

## The hazard

`Probe.CachedResponse` is a shallow struct copy (`resp := *cached`, probe.go)
of the atomically published snapshot. `Checks` therefore aliases the map the
probe's own handlers marshal concurrently. A consumer that adds or rewrites a
row in place races every concurrent marshal of the same snapshot — a Go
runtime `concurrent map writes` fatal, not an error.

Two fleet repos discovered this independently and cloned manually:

- dnsblockd `internal/server/health.go:121` — "the response we were handed
  aliases the probe's cached map (shallow copy)… degradeStaleHealth clones
  before writing".
- cv `internal/health/handlers.go:131` — `maps.Copy` into a fresh map before
  sealing the synthetic database row.

`docs/synthetic-check-overlay.md` calls the clone "the load-bearing line" and
rules that overlays belong in the serving path, never written back.

## Decision

Keep the shallow copy — the lock-free read is a measured property
(`BenchmarkCachedResponse_ParallelReads`: ~0.03 ns/op, 0 allocs) and deep
copying on every read would tax every handler and dashboard tick to protect
against a mutation almost nobody performs.

Make the rule explicit instead:

- The `CachedResponse` godoc states that the response is a read-only view and
  that `Checks` aliases the published snapshot.
- `Response.Clone()` returns an independent copy, including the `Checks` map
  (values are copied as-is: `Check` carries only strings and times). A nil
  map stays nil.

## Alternatives rejected

- **Always deep-copy in `CachedResponse`.** Turns a zero-alloc lock-free read
  into an O(checks) allocation per read, for every consumer, to make an
  unsupported use safe.
- **Return an immutable wrapper type.** Breaking API shape for a v0.x
  library and a new type to learn; the map is required by the wire contract.
- **No change (docs only in the overlay note).** Two repos already had to
  rediscover the hazard from a crash; a first-class copy method is the
  discoverable fix.

## Invariant to keep

The overlay pattern stays valid and now has a sanctioned spelling:

```go
resp := probe.CachedResponse().Clone()
resp.Checks["evaluation-stale"] = health.Check{Status: health.StatusFail, …}
```

Synthetic rows keep caller-observed `Since` and zero `DurationNanos` — see
`docs/synthetic-check-overlay.md` for the four composition rules.
