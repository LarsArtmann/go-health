# Federation `Healthz` parity — design note

|            |                                                                                                     |
| ---------- | --------------------------------------------------------------------------------------------------- |
| **Date**   | 2026-10-09                                                                                          |
| **Status** | ACCEPTED (design) — implementation stays in the v0.6 window; do NOT implement opportunistically     |
| **Inputs** | `aggregate.Aggregate.Healthz()` (docs/aggregate-healthz-design.md), `federation` merge + latch semantics, ROADMAP Theme 7 |

## Problem

`health.Probe` and `aggregate.Aggregate` both expose a single-endpoint
combined handler `Healthz()`: one URL answering "should traffic be routed
here?" for load balancers that do not speak the three-probe kubelet
contract. `federation.Prober` exposes the three handlers but no `Healthz()`
— a deployment fronting N remote instances through a federation view
cannot get one URL without recomposing booting/failing/draining logic the
package already owns.

## Decision (proposed)

Add `(*Prober).Healthz() http.HandlerFunc`, mirroring the aggregate
condition set with federation's fetch-based substitutions:

```
Prober.Healthz() == 503  iff  any per-remote latch is unset
                             OR the merged roll-up is fail
                                (which already includes any unreachable
                                 remote's synthetic "<name>/reachable" row
                                 and any remote reporting shutting down)
                     == 200  otherwise, with the merged body
```

- `warn` stays 200, matching `Probe.Healthz()` and the aggregate.
- The handler uses the same read path as `ReadinessHandler()`: one parallel
  fetch across remotes, then the merge. No new state, no separate cache, no
  new freshness semantics.
- **Latch semantics:** a remote's latch flips on its first _successful_
  fetch (state lives in `Prober.states[i].latched`). "Latch unset" and
  "unreachable" therefore coincide in practice — a first-fetch failure
  produces the reachable-FAIL row that fails the roll-up anyway. The
  explicit latch condition is kept for parity and for the transient window
  where a fetch is in flight; it is redundant, not load-bearing, and it
  must never grow additional meaning.
- **Unlatched + fail short-circuit:** mirroring `Aggregate.Healthz`, the
  synthetic `startup` check row ("startup latch not set") is added only
  when the latch is unset AND the roll-up is not already fail — never
  overwriting a reachable row that explains the real cause.

## Contract check

- **Additive:** new method on `Prober`; no signature, wire-format, or
  handler change. Dashboard consumer coordination: additive, no bump
  required.
- **Cost note:** unlike the aggregate (atomic cache loads), federation
  Healthz triggers a fetch wave per request — identical to its readiness
  handler, so LB polling frequency against federation should follow the
  same guidance as readiness polling (fetch caps apply unchanged).
- **Consistent with merge-on-read:** the merged body is exactly
  `CachedResponse()`'s, plus the same synthetic startup overlay rule the
  aggregate uses.

## Enforcement (when implemented)

- Unit table: {all latched → 200; one never-answered remote → 503 with its
  reachable row; merged fail from a remote's own checks → 503; one remote
  shutting down → 503; warn → 200}.
- Property: status code is a pure function of "all latches set" and the
  merged roll-up, across the topologies the existing federation property
  test enumerates.
- ROADMAP Theme 7 cross-link updated at implementation time.
