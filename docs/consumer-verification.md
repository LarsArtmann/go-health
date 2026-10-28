# Consumer Verification — go-health fleet compatibility

> Moved verbatim from AGENTS.md (2026-10-08, agents-md-size trim). This is the
> authoritative consumer-compatibility record; AGENTS.md links here.

**Consumer verification:** `samber-do-auditlog` does NOT import go-health (post-extraction, dependency-free both ways); the most closely verified consumer is [`go-health-dashboard`](https://github.com/larsartmann/go-health-dashboard), which requires a released tag directly (no replace directive; bumped to v0.1.3 at release time 2026-09-04, full timeline below). Verified at the v0.1.3 release (2026-09-04): dashboard build green and all go-health-related tests (aggregate, integration, dashboard) green against released v0.1.3 with source names `api`/`web` (contract-compatible); its pre-existing CSP/Datastar test failures are unchanged from v0.1.2 and unrelated to go-health. At the v0.2.0 release (2026-09-16): dashboard verified compatible with the unreleased metadata fields pre-tag (build green 2026-09-16); v0.2.0 is additive, so no dashboard bump was required at release time. Verified 2026-09-22: the dashboard has fully adopted the v0.2.0 metadata — it renders `since` ("since 14:02:05 UTC (17m)") and `duration_ns` adaptively per check, exports both via its trend JSON, collapses healthy groups, and pins the rendering with a golden test + a `DetailedHealthRecorder` screenshot fixture (`timedScreenshotRecorder`). Bumped to released v0.4.0 on 2026-09-22 (from a 2026-09-18 v0.2.x pseudo-version); the full dashboard suite is green against the release, including the browser-backed screenshot tests. Wider consumer inventory re-verified 2026-10-02 via local checkouts + `go.mod` requires: **15 direct consumers** (go-health-dashboard v0.4.1, CV, CV/result, Zlota44, dnsblockd, library-policy, file-and-image-renamer, DiscordSync, KeyHolderAI, nsfw-classifier, webphone, projects-management-automation, cqrs-htmx, go-appkit, go-taskqueue v0.4.1 — internal/webui/health.go) and **16 indirect** via the two bridges (14 through cqrs-htmx v4.7.0–v4.13.0, 2 through go-appkit/health v0.5.1/v0.7.0). Five implementation patterns across the fleet: (A) injector + `WithCriticalServices` (KeyHolderAI di.go:442, CV, DiscordSync); (B) `NewWithHealthCheck` check-func map (dnsblockd, library-policy, fir); (C) recorder bridge — often over an EMPTY injector + `WithHealthRecorder` (cqrs-htmx/health NewProbe, Zlota44, projects-management-automation); (D) framework bridge (go-appkit/health `NewProbe` + doadapter); (E) hand-rolled cache adapters — dnsblockd, DiscordSync, cv and go-taskqueue each rebuilt "serve cache if fresh, else evaluate" around `Response.Timestamp` because the primitive did not exist before v0.8.0's `CacheAge`/`CachedResponseFresh`/`EvaluateAndCache` (2026-10-27 sweep below). Empty-injector+recorder probes are why critical-name validation must be batch-based, never `ListProvidedServices()`-based (see below). Consumers building against go-health need Go 1.27+ (`encoding/json/v2` is stable there; go.mod's `go 1.27` directive excludes older toolchains outright). Any public API change must be coordinated with the dashboard consumer and the go-appkit bridge modules (both pin released go-health tags). Do NOT add a go-health dependency to samber-do-auditlog to implement `DetailedHealthRecorder` without an owner decision — it reverses this deliberate decoupling (tracked in go-health TODO_LIST.md, Blocked section).

**Federation surface growth (2026-10-11, [Unreleased]):** `federation.Prober` gained `MarkShuttingDown()` (hub-side two-phase drain; issue #5). The method is purely additive on the concrete `*Prober` type — no interface narrowing, no signature change — so every fleet consumer compiles unchanged against it; the dashboard's `dashboardProber` structural assertion (which pins only the pre-existing surface) stays green by construction. Consumers that want the drain (the dashboard hub's SIGTERM path is the motivating case) must bump to the release carrying it; the wire format does not change (`shutting_down` pre-existed in the schema), so mixed-version fleets render drained hubs correctly without a coordinated bump.

**Re-count and needs sweep (2026-10-27):** the direct inventory is still 15 repos, but the reach is **36 consumers of go-health-dashboard v0.12.1** through three layers: the dashboard itself, `cqrs-htmx/health` v0.7.0 (~20 apps), and `go-appkit/health` v0.5.1 (4+ repos). Version skew: 12 repos on v0.7.0; CV, cv, dnsblockd, KeyHolderAI, and both go-appkit health modules still on v0.5.1. A fifth implementation pattern is now recorded — **E: hand-rolled cache adapters** (dnsblockd, DiscordSync, cv, go-taskqueue re-implement "serve cache if fresh, else evaluate" because the primitive did not exist; see the sweep). Release-coordination rule derived from this: any option-default, grade, or wire change must ship **with** a bump of both bridges, otherwise ~24 repos inherit nothing. Authoritative needs matrix, evidence, and the supplier plan: [docs/research/2026-10-27_fleet-consumer-needs.md](research/2026-10-27_fleet-consumer-needs.md).

## Evidence table — v0.8.1 fleet state (measured 2026-10-28, plan M04)

The compiles and pin bumps of the v0.8.x wave happened ad hoc; this table is the
record. Evidence levels are deliberately separated so nobody mistakes a pin read
for a build:

- **L1 pin-read** — each repo's `go.mod` (or `health/go.mod` for sub-modules) read
  from its live default branch through the authenticated GitHub contents API by
  `tools/fleet-skew.sh`. Proves what the repository *declares*, proves nothing
  about whether it compiles.
- **L2 build** — a local `go build ./...` / `go vet ./...` against the pinned
  version.
- **L3 suite** — a local `go test` run against the pinned version.

| Repository | Module file | Pinned go-health | Evidence | Result |
| --- | --- | --- | --- | --- |
| go-health-dashboard | go.mod @master | v0.8.1 | L1 + L2 + L3 | `go build`/`go vet` clean; unit suite green except 7 goldens drifting on the unaudited templ-components v1.24.0 climb (UI, not go-health) |
| CV | go.mod @master | v0.8.1 | L1 | pin aligned |
| cv | go.mod @master | v0.8.1 | L1 | pin aligned (local lineage still unpushed, unrelated) |
| Zlota44 | go.mod @master | v0.8.1 | L1 | pin aligned |
| dnsblockd | go.mod @master | v0.8.1 | L1 | pin aligned |
| library-policy | go.mod @master | v0.8.1 | L1 | pin aligned |
| file-and-image-renamer | go.mod @master | v0.8.1 | L1 | pin aligned |
| DiscordSync | go.mod @master | v0.8.1 | L1 + L2 + L3 | `internal/mediapipeline` and `internal/web/golden` green; `internal/web` snapshot suite red from templ-components v1.24.0 markup, not go-health |
| KeyHolderAI | go.mod @master | v0.8.1 | L1 | pin aligned |
| nsfw-classifier | go.mod @master | v0.8.1 | L1 | pin aligned |
| webphone | go.mod @main | v0.8.1 | L1 | pin aligned (default branch is `main`) |
| projects-management-automation | go.mod @master | v0.8.1 | L1 | pin aligned |
| timesheets | go.mod @master | v0.8.1 | L1 | pin aligned |
| cqrs-htmx | health/go.mod @master | v0.8.1 | L1 | pin aligned (bridge module) |
| go-appkit | health/go.mod @master | v0.8.1 | L1 | pin aligned (bridge module) |
| go-taskqueue | go.mod @master | v0.8.1 | L1 | pin aligned |

Skew verdict at that moment: `minor-stale: 0 (allowed 0), warnings: 0,
unreadable: 0` — the allowance file was deleted in `15fad10` once the backlog
reached zero, so the gate now fails on any drift by construction.

**What v0.8.x added and who can use it:** `StatusOff`, the `checks` battery,
`VersionHandler`, `WithInstanceID` and `MarkShuttingDown` (v0.5.x) plus the
unified merge core (v0.6.0-v0.7.0) and federation's `Healthz()`,
`SourceStatuses()` and `MarkShuttingDown()` (v0.8.x). The dashboard is the only
consumer that has adopted all of them: `TestFederation_HealthzCapabilityForwarded`
proves /livez serves the merged view and answers 503 while the hub drains,
`sourcestatus_test.go` pins federation as a `SourceStatuser`, and
`cmd/health-hub` runs the two-phase drain (verified live: `/readyz` 200 before
SIGTERM, 503 within 2ms after, process exit 0 two seconds later).
