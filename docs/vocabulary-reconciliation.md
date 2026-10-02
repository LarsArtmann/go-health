# Vocabulary reconciliation: one word per concept

**Date:** 2026-10-02 · **Status:** PROPOSAL — v0.5 window. Grounded in docs/adoption-matrix.md.

## The split brain today

| Concept | Names in use | Where |
| --- | --- | --- |
| a thing that produces a health.Response | `*Probe` (root), `Source` (aggregate), `Remote` (federation), `Prober` (dashboard consumer interface, mirrored by federation) | 4 names, 4 packages |
| a named health verdict | `Check` (wire/type), "service" (options, docs: `WithCriticalServices`) | 2 metaphors |

## Proposal

**1. Sources are "health sources."** Keep the concrete type names as-is
(`Probe`, `aggregate.Source`, `federation.Remote` — each is idiomatic in
its package and renaming `Remote` to `Source` would blur the transport
difference that justifies the two packages). Instead, standardize the
*generic* term in docs and interfaces:

- docs/DOMAIN_LANGUAGE.md gains the entry: **health source** — anything
  producing a `health.Response`: a local `Probe`, an aggregate source, or a
  federation remote.
- `federation.Prober` stays (it exists to satisfy the dashboard's
  consumer-side interface structurally; renaming breaks that contract for
  zero gain).

**2. Checks win over services.** The wire says `checks`, the type says
`Check`, `NewChecks` says checks — only `WithCriticalServices` says
"services." Renaming the option is the breaking part; document now, rename
in v0.5 alongside `ServiceName`:

- docs + DOMAIN_LANGUAGE.md: a *check* is one named verdict; a *critical
  check* is one whose failure forces readiness to fail. "Service" survives
  only where samber/do services are literally meant (the injector path's
  checks *are* do services there).
- v0.5 candidates: `WithCriticalServices` → `WithCriticalChecks`
  (docs-first; both accepted via a temporary alias option during the v0.5
  window if fleet pressure demands).

## What we deliberately do NOT do

- No `Source` interface in the root package — `health.Response` already is
  the interchange type; aggregate/federation take their concrete inputs and
  map onto the merge primitive (docs/merge-unification-design.md).
- No renames inside v0.4.x; every breaking vocabulary change rides the v0.5
  window with per-repo call-site lists from the adoption matrix.
