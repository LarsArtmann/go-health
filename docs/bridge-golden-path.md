# Bridge golden path: contract, findings, upstream actions

**Date:** 2026-10-02 · **Scope:** go-appkit/health and cqrs-htmx/health — the two bridges 16 of 30 consumers inherit through (14 via cqrs-htmx v4.7.0–v4.13.0, 2 via go-appkit/health v0.5.1/v0.7.0).

## The golden-path contract for bridges

A bridge should, in order of importance:

1. **Preserve criticality naming** — check names surfaced to consumers must
   be exactly the strings valid in `WithCriticalServices` (both bridges: yes,
   by construction).
2. **Preserve classification fidelity** — never swallow a failure into nil;
   map states to error/no-error honestly (cqrs-htmx: live/stopped → nil,
   failed → infrastructure error, draining → transient error → grades
   warn; go-appkit: error pass-through, per-check panic isolation).
3. **Survive `Start()` validation** — with v0.4.x+ batch-based
   `ErrUnknownCriticalService` validation, a bridge whose batch omits a
   name the consumer marked critical fails at boot (loud, correct).
4. **Carry metadata when cheap** — the standalone `NewWithHealthCheck` path
   reports zero `duration_ns` (adaptPlainChecks); bridges that can time
   per-check should expose a `DetailedHealthRecorder` or
   `NewWithDetailedCheck` variant so dashboards get `duration_ns`/`since`
   fidelity. (`since` works on every path — it is probe-observed in
   buildChecks.)
5. **Document option contracts** — which SDK options pass through, which
   are dropped and why (go-appkit documents the `WithHealthRecorder`-dropped
   trap inline; good precedent).

## C3 findings: go-appkit/health `NewProbe` (v0.7.0 line)

- **Wiring:** `NewWithHealthCheck(batch, opts...)`; concurrent batch via
  WaitGroup + mutex; per-check panic recovery → that check's error
  (`errorfamily.Infrastructure`), batch stays trustworthy. Stronger
  isolation than the root library's batch-level recovery — worth keeping.
- **Criticality:** fully opt-driven (`WithCriticalServices`), names are the
  map keys — contract 1 satisfied.
- **Metadata gaps:** zero `duration_ns` (plain batch fn). `since` works.
  **Gap G-A1:** no `DetailedHealthRecorder`/detailed variant.
- **Documented trap:** `WithHealthRecorder` silently dropped on this path
  (upstream go-health contract; the bridge redirects users to the injector
  path for recording).
- **Nil/empty map → always-pass probe:** documented as a placeholder; with
  `Start()` validation an empty batch plus critical names now errors —
  coherent.

## C4 findings: cqrs-htmx/health `NewProbe` (v4.x)

- **Wiring:** `gohealth.New(do.New(), WithHealthRecorder(projectionRecorder), opts...)`
  — the empty-injector + recorder pattern. Validation-safe under v0.4.x+
  because validation is batch-based; `ListProvidedServices()`-based
  validation would have broken this bridge outright (see
  docs/start-validation-design.md).
- **Check naming:** projection names verbatim → `WithCriticalServices`
  composes directly; projection names win collisions with injector service
  names (documented).
- **Classification mapping:** live/stopped → healthy; failed →
  infrastructure error (fail if listed critical, else warn); other states
  (draining/catching-up) → transient error → warn. Honest and
  readiness-friendly.
- **Metadata gaps:** plain `map[string]error` recorder → zero
  `duration_ns`. **Gap G-C1:** no `DetailedHealthRecorder` variant;
  projection status entries plausibly carry their own timing that could
  fill it.
- **Version skew:** v4.7.0–v4.13.0 spread across 14 consumers; bridge-level
  fixes reach all of them on their next cqrs-htmx bump.

## Upstream actions (drafts — filing is an owner call)

Both actions are the same shape: **add a metadata-preserving variant**;
the golden path otherwise already holds.

- **go-appkit/health:** add `NewDetailedProbe(map[string]DetailedCheckFunc, opts...)`
  forwarding to `health.NewWithDetailedCheck`, and/or a detailed recorder
  path so `duration_ns` flows. Draft issue text:
  `docs/announcements/2026-10-02_go-appkit-health-detailed-probe.md`.
- **cqrs-htmx/health:** add a `DetailedRecorder` wrapping
  `ProjectionStatusProvider` timings via `health.DetailedHealthRecorder`.
  Draft issue text:
  `docs/announcements/2026-10-02_cqrs-htmx-health-detailed-recorder.md`.

## Non-actions (verified non-gaps)

- Neither bridge can break under `ErrUnknownCriticalService`: both surface
  every consumer-visible name in every batch (projection recorder merges
  injector results + all projections; go-appkit runs the whole map).
- Neither bridge hardcodes criticality, so no consumer silently changes
  readiness semantics on upgrade.
