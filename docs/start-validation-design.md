# Design: critical-name validation at Start()

**Date:** 2026-10-02 · **Status:** DECIDED — hard error at `Start()`, batch-based validation
**Related:** [docs/panic-recovery-design.md](panic-recovery-design.md), [ADR-001](adr/ADR-001-stdlib-errors.md), [docs/timeout-design.md](timeout-design.md)

## Problem

`WithCriticalServices(names ...string)` takes loose strings. A name that never
appears in a health-check batch is silently ignored:

- **Readiness:** the intended-critical service is graded non-critical; its
  failure degrades the roll-up to `warn` (HTTP 200) instead of `fail` (503).
- **Startup:** `evaluateStartup` (classifier.go) requires every critical name
  to be present and nil-error in a batch. A typo'd name is never present, so
  the latch **never sets** — the pod never passes its startup probe — with
  zero diagnostics anywhere.

This is test-pinned in `probe_critical_names_test.go`
(`TestCriticalNameTypo_BlocksStartupLatchForever`,
`TestCriticalNameTypo_FailingServiceGradedWarn`).

**Field evidence:** DiscordSync hit exactly this on 2026-08-16
(`WithCriticalServices("database", "bot")` matched zero services) and now
carries a dedicated guard test (`health_critical_guard_test.go`). Every
consumer should get that guard for free, at boot, without writing a test.

## Validation universe: the batch, not the injector

The obvious implementation compares critical names against
`do.Injector.ListProvidedServices()` (verified to exist in samber/do
v2.1.0 — `Injector` interface, injector.go:42; returns
`[]ServiceDescription{ScopeID, ScopeName, Service}`). **Rejected.** The
injector is not always the check universe:

- Recorder-based probes construct `health.New(do.New(), WithHealthRecorder(r))`
  with an **empty injector** — Zlota44 (`internal/server/health.go:101`) and
  projects-management-automation (`internal/infrastructure/health/server.go:100`)
  both do this. Every critical name would false-positive as "unknown".
- On the injector path, only *healthcheck-registered* services can appear in a
  batch; `ListProvidedServices()` lists all provided services, a superset.
- Standalone constructors (`NewWithHealthCheck`, `NewChecks`,
  `NewWithDetailedCheck`) have no injector at all.

**Chosen universe: the keys of the initial evaluation batch.** `Start()`
already runs one `refreshCache` before returning (probe.go), so the
comparison is free. The invariant is exactly the latch's own requirement: a
critical name absent from every batch can *never* satisfy
`evaluateStartup`. If it is absent from the first batch, startup is already
doomed — erroring is strictly more informative than hanging. Validation
therefore applies uniformly to **all** construction paths.

## Gating posture: three options considered

| Option | Behavior | Verdict |
| --- | --- | --- |
| **Hard error at `Start()`** | `Start` returns `ErrUnknownCriticalService` (wrapped with the sorted unknown names) after the initial batch. | **CHOSEN** — the defect is a silent never-ready pod; a loud boot failure is the correct trade. `Start` already returns validation errors (`ErrInvalidTimeout`, `ErrInvalidRefreshInterval`), so the contract does not change shape. |
| Non-fatal diagnostic via `WithEvaluationHook` | Report unknown names, keep serving. | Rejected as default: the failure mode we are fixing is that nobody looks. A hook only helps consumers who already wired observability — the ones least likely to have the typo. |
| Dev-strict (env-var/build-tag gated) | Error in dev builds, warn in prod. | Rejected: environment-dependent health semantics are worse than either pure option; the fleet cannot test what only fails elsewhere. |

**Migration risk assessment (fleet scan 2026-10-02, all local
`WithCriticalServices` call sites):**

| Consumer | Path | Critical names | Batch-visible? |
| --- | --- | --- | --- |
| KeyHolderAI (di.go:442) | injector | typetostring-typed service names | yes (do healthcheckers) |
| DiscordSync (health_dashboard.go:86) | injector | `criticalHealthServiceNames()` (guarded since the 2026-08-16 bug) | yes |
| CV (di/health_probe.go:58) | injector | typetostring-typed, mode-conditional | yes |
| nsfw-classifier (app.go:285) | injector | typed classifier service | yes |
| go-appkit bridge (probe.go) | injector | derived from registered checks | yes |
| cqrs-htmx bridge (probe.go) | recorder | projection names | yes (recorder emits them) |
| Zlota44 (health.go:105) | empty injector + recorder | `checkSQLite` | yes (recorder emits it) |
| PMA (server.go:101) | empty injector + recorder | `registry.CriticalNames()` | yes (registry emits them) |
| dnsblockd (health.go:49) | injector | `database`, `dns` | yes |
| library-policy (health.go:38) | standalone | dynamic `critical []string` | yes (batch fn emits them) |
| fir (providers.go:125) | standalone | `CheckDiskSpace`, `CheckAIProvider` | yes |
| webphone (app.go:216, server.go:311) | mixed | `sqlite`, `blob-dir` | yes |

No call site found where a critical name legitimately never appears in a
batch. Environment-conditional sets (CV) toggle *which* names are passed, not
whether they are registered — the guard is safe.

**Escape hatch:** none shipped in v0.4.x. A consumer genuinely blocked by
boot-ordering (calling `Start()` before providing services) should fix the
ordering — samber/do lazy services that are never invoked report nil-error
and make the startup probe meaningless anyway (see AGENTS.md gotchas). If a
real-world case emerges, an opt-out option can be added in a minor release;
removing a hard error later is impossible.

## Decisions on placement

- **`Validate()` stays config-only** (timeout, refresh interval). It cannot
  know the batch (it runs before any evaluation), so critical-name checking
  does not grow there; it lives in `Start()` as a distinct
  `validateCriticalNames` step. Alternative (a separate
  `ValidateAgainstBatch` public method) rejected: no consumer needs to run
  validation without starting.
- **Acceptance criteria** (all pinned by `probe_critical_names_test.go`):
  1. A `WithCriticalServices` name absent from the initial batch makes
     `Start` return an error satisfying `errors.Is(err, ErrUnknownCriticalService)`.
  2. The error names every unknown service, sorted, and none of the known ones.
  3. A failed `Start` launches no background loop and publishes no cache.
  4. Known names start unchanged; an empty critical set never validates.
  5. Validation applies on the injector, recorder (empty injector), and
     standalone-constructor paths alike.

## Semantics

- Sentinel: `ErrUnknownCriticalService` — match with `errors.Is`.
- Wrap with the sorted unknown names and remediation:
  `health: unknown critical service(s) "databse" (no check with this name ran; fix WithCriticalServices or the check registration)`.
- Validation runs **after** the initial batch, **only on the first effective
  `Start()`** (repeat `Start` stays a no-op).
- Panic-recovered batches produce the synthetic `health-check` key; that does
  not affect the comparison.
- Live-mode probes (`WithRefreshInterval(0)`): `Start` still performs its one
  initial evaluation, so validation applies identically.

## Consequences

- The DiscordSync class of bug becomes a boot-time error for every consumer.
- Startup probes can no longer hang forever on a typo.
- Behavior change, not wire change: the HTTP surface and JSON are untouched;
  only the (already error-returning) `Start()` contract gains a case.
- Consumers with dynamic late registration must provide/register checks
  before `Start()` — the documented, recommended order.
