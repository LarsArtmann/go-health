# ADR-010: Freshness is a primitive, not a policy option

> Status: ACCEPTED · Date: 2026-10-27 · Deciders: maintainer
> Formalizes the rejection recorded in `docs/cache-freshness-design.md`
> ("Why not a `WithStaleAfter` / `WithStalenessGuard` option"). Implementation:
> `Probe.CacheAge`, `Probe.CachedResponseFresh`, `Probe.EvaluateAndCache` in
> `probe.go`.

## Context

Five fleet repos hand-roll the same read policy around the background cache
(`docs/research/2026-10-27_fleet-consumer-needs.md` §N1/N2): DiscordSync,
dnsblockd, cv and go-taskqueue each had their own "serve the cache while it is
fresh, otherwise evaluate" adapter. The obvious supplier-side answer is one
option — `WithStaleAfter(d)` — that makes the probe decide for them.

## Decision

**Ship the two halves as primitives; ship no policy.** The whole surface is:

```go
age, ok := probe.CacheAge()                       // age of the served snapshot
resp, fresh := probe.CachedResponseFresh(maxAge)  // serve-or-miss decision, caller's bound
resp = probe.EvaluateAndCache(ctx)                // evaluate and publish
```

There is deliberately no `WithStaleAfter`, `WithStalenessGuard`, or automatic
degradation-to-`warn`/`fail` on staleness.

## Why the policy option is the wrong seam

1. **The bound is per-consumer and derived, not constant.** dnsblockd runs a 1s
   refresh with a 5s timeout and treats 15 s as stale; DiscordSync uses
   2× its interval. A single option would have to be re-derived in every
   caller anyway, at which point it is a primitive with extra state.
2. **The verdict on staleness is domain-specific.** dnsblockd rewrites a real
   check to `fail`; cv folds a synthetic row with its own `off`→`warn` rule.
   The library cannot pick one semantic for 36 consumers without silently
   changing readiness codes for the ones that chose differently.
3. **The decision belongs to the request, not the probe.** Freshness is
   evaluated per read against a caller-chosen window; baking it into
   configuration makes two requests to the same probe unable to disagree, which
   is exactly what a dashboard and a load balancer need them to do.
4. **The primitive is three lines.** The snippet above is shorter than the
   option's godoc would be, and it composes with `Response.Clone` when the
   caller projects extra rows.

## Rejected alternatives

| Alternative                                         | Why not                                                                                                                          |
| --------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `WithStaleAfter(d)` + auto-degrade                  | Picks one degradation semantic for the whole fleet; changes the wire status without an evaluation behind it.                     |
| `WithStalenessGuard(func(age) Status)`              | The callback is the primitive with extra indirection: the caller already owns the decision, it does not need the probe's loop.   |
| A `CachedResponse(maxAge)` overload                 | Go has no overloads; a second name (`CachedResponseFresh`) is the idiomatic spelling and keeps `CachedResponse` allocation-free. |
| Serving the zero response on a miss instead of `ok` | Silently answering "pass" for "never evaluated" is the exact bug the fleet adapters were written to work around.                 |

## Consequences

- Consumers keep their own bound and their own degradation rule; the library
  holds no timer state and starts no goroutine for freshness.
- `CachedResponseFresh` is allocation-free on the hit path
  (`BenchmarkCachedResponseFresh`: 0 B/op, 0 allocs/op) because it aliases the
  published `Checks` map instead of copying it.
- A request for automatic degradation is answered with the three-line snippet
  and this record, not with a new option. Superseding this ADR requires two
  named consumers agreeing on one semantic.
- `docs/cache-freshness-design.md` remains the implementation note; this ADR is
  the standing decision not to grow a policy layer on top of it.
