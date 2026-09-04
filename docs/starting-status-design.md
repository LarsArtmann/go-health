# "starting" Status & Status Validation — design note

> Decided: 2026-09-04 · Status: REJECTED (three-state retained) · Resolves: open ROADMAP idea "Status validation + 'starting' Status"

## Problem

Two related questions about the `Status` enum (`pass` / `warn` / `fail`):

1. Should the unlatched startup state report a distinct `StatusStarting` value
   so consumers can tell "boot in progress" from "broken"?
2. Should `Status` values be validated anywhere (constructor, decoder)?

## "starting" Status: rejected

The startup probe reports `fail` (HTTP 503) until all critical services pass
once (`buildStartupResponse`, handlers.go). A fourth value `starting` sounds
more truthful, but every argument is against it:

1. **Kubernetes never reads the body.** The kubelet maps any non-2xx startup
   response to "container not started". A fourth body status adds zero value
   at the layer that decides.
2. **Nonstandard vocabulary.** The de-facto health-check JSON convention is
   pass/fail/warn. Every dashboard, aggregator, and alert rule that consumes
   `status` would need a fourth case — for a state that is
   status-code-indistinguishable from `fail` (both 503).
3. **Roll-up ambiguity.** `Response.Status` is a roll-up. The same instance
   during boot would report `starting` on `/startupz` but `fail` on `/readyz`
   (readiness gates on the same failing dependencies). Two different strings
   for one machine state — every consumer must pick a winner.
4. **The latch is per-process, and it can regress.** After a probe-level
   reset (today only `ResetStartupLatchForTest`; tomorrow, conceivably, an
   admin endpoint), an instance with days of uptime would flip back to
   `starting` — contradicting its own `uptime` field. A status that can go
   `pass → starting` is not a status, it's a mood.
5. **No do-conformance path.** `Probe.HealthCheck` maps states onto the
   samber/do `Healthchecker` contract, which knows only error / non-error.
   `starting` cannot be expressed through it; the enum would fork.

**If monitoring needs the distinction**, the signal already exists and is
cheaper: alert on `/startupz` returning non-200 for longer than the expected
boot window (the same decision the kubelet's `failureThreshold` encodes), or
correlate on `uptime` / `shutting_down` fields.

## Status validation: none, by design

`Status` values are produced exclusively inside the package
(`classify`, `buildStartupResponse`, aggregate roll-up). No public API accepts
a `Status` as input — no option, no constructor, no decoder boundary. There
is no untrusted-input seam to validate at, so validation would be dead code.
If a future feature lets callers supply `Status` values (e.g. custom
check-results), validation belongs at that new boundary, not retrofitted
onto the enum.

## Consequences

- The three-state enum is frozen: `pass`, `warn`, `fail`.
- Startup-unlatched remains `fail` + HTTP 503, documented in
  [DOMAIN_LANGUAGE.md](DOMAIN_LANGUAGE.md) ("Startup latch").
- `aggregate`'s roll-up (`aggregate.go`) keeps a three-way `switch`; a fourth
  case would have been a compile-error forcing every aggregate consumer to
  change — one more reason the cost outweighed the benefit.
