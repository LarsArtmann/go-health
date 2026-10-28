# Feature adoption matrix (fleet)

**Date:** 2026-10-02 · **Method:** alias-safe `rg` over local checkouts of every known consumer
(direct go-health imports + the go-appkit/cqrs-htmx bridges), go.mod requires

- source trace. "—" = zero adoption found in the fleet.

**Re-verified:** 2026-10-09 — option-symbol grep (vendor-excluded) cross-checked
against go.mod requires and call-site package qualifiers. Rows below reflect
the re-verified state; corrections are documented in
[2026-10-09 re-verification](#2026-10-09-re-verification).

| Feature                                                             | Adopters (direct, verified in source)                                                                              | Verdict                                                                                                                                  |
| ------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------- |
| core `New` + handlers + `WithCriticalServices`                      | all 14 direct consumers                                                                                            | core                                                                                                                                     |
| `WithAllowedMethods`                                                | dnsblockd, file-and-image-renamer, KeyHolderAI, nsfw-classifier, projects-management-automation, Zlota44           | healthy (all six re-verified 2026-10-09)                                                                                                 |
| `WithGETOnly` (deprecated → removed, post-v0.6.1)                   | KeyHolderAI (tests only, `cmd/keyholderai/health_probe_http_test.go` — switched to `WithAllowedMethods()` locally) | zero production adoption — removal EXECUTED under the deprecation policy's zero-production-adopter path (migration snippet in CHANGELOG) |
| `MarkShuttingDown`                                                  | CV, DiscordSync, KeyHolderAI, nsfw-classifier, go-appkit (+doadapter)                                              | healthy                                                                                                                                  |
| `WithEvaluationHook`                                                | dnsblockd, go-appkit/health                                                                                        | healthy                                                                                                                                  |
| `AwaitReady`                                                        | dnsblockd, go-appkit/health                                                                                        | healthy                                                                                                                                  |
| `WithNowFunc`                                                       | dnsblockd (tests)                                                                                                  | healthy (test seam)                                                                                                                      |
| `NewChecks`                                                         | webphone, go-health-dashboard example, typespec-eventsourcing (tests)                                              | healthy, low adoption — keep (it is the batteries composition point)                                                                     |
| `NewWithDetailedCheck` / `DetailedHealthRecorder`                   | go-health-dashboard only (`timedScreenshotRecorder`)                                                               | low adoption; feeds the bridge detailed-variant gap (docs/bridge-golden-path.md)                                                         |
| `WithInstanceID`                                                    | go-health-dashboard                                                                                                | single consumer — document-as-intended (dashboard correlation), retire only with owner sign-off. v0.9.0 breaking: parameter is now `health.InstanceID` (`health.NewInstanceID(...)`); reader uses `.Get()` — wire format unchanged |
| `WithLiveThrottle`                                                  | —                                                                                                                  | ghost: live mode + flood coalescing has no adopter. Keep (semantically required for safe live mode), document as protective              |
| `WithShutdownGracePeriod`                                           | —                                                                                                                  | ghost (2026-10-09): both 10-02 entries were go-daemon's same-named `SocketServerOption`, not go-health — see E5 below                    |
| `Healthz()` single-endpoint                                         | go-health-dashboard                                                                                                | low direct adoption; primary use is external LB composition — document-as-intended                                                       |
| `aggregate` subpackage                                              | go-health-dashboard (multiple)                                                                                     | single-consumer but deep (loadtests, SSE, fuzz) — healthy                                                                                |
| `federation` subpackage                                             | go-health-dashboard (`cmd/health-hub`)                                                                             | single-consumer but production-wired — healthy                                                                                           |
| `WithRefreshInterval`, `WithTimeout`, `WithVersion`, recorder paths | all consumers (spread)                                                                                             | core                                                                                                                                     |

## Resolved unknowns

- **go-taskqueue** (previously unresolved in the 2026-10-02 survey): IS a
  consumer — `go.mod` requires go-health **v0.4.1** + go-health-dashboard
  v0.10.1; check source at `internal/webui/health.go`. Direct-consumer count
  therefore 14 including go-taskqueue's webui (still "~14 direct" as stated
  in AGENTS.md; the survey's earlier "grep empty" was the resolution miss).
- **aggregate/federation "non-adoption" claim (earlier session): CORRECTED.**
  Both have one deep consumer (go-health-dashboard). The earlier claim rested
  on a non-alias-hardened grep; alias-safe search above finds real usage.

## 2026-10-09 re-verification

Correcting three findings from the 2026-10-02 survey (method: vendor-excluded
symbol grep + `go.mod` require check + call-site qualifier inspection):

- **`WithShutdownGracePeriod` had zero real adopters.** go-daemon defines its
  own `SocketServerOption` named `WithShutdownGracePeriod` (`socket.go:126`);
  project-discovery-daemon (`godmn.` prefix) and bank-sync (`daemon.` prefix)
  call go-daemon's option, and none of the three requires go-health. The
  10-02 survey matched the symbol name without qualifying the package.
- **`WithGETOnly` in KeyHolderAI is test-only** (`health_probe_http_test.go`);
  the only non-test hits are the vendored library's own doc comments. No
  production usage anywhere in the fleet. The symbol has since been removed
  (post-v0.6.1, Unreleased) under the zero-production-adopter path;
  KeyHolderAI's test call site now uses `WithAllowedMethods()` (local
  commit, unpushed).
- **Abbreviated repo names expanded** for grep-ability: "fir" →
  file-and-image-renamer (confirmed consumer at v0.5.1,
  `pkg/injector/providers.go`), "PMA" → projects-management-automation.

## Ghost-feature review input (E5)

Zero-adoption features: `WithLiveThrottle` and `WithShutdownGracePeriod`.
`WithLiveThrottle` verdict: keep — it exists
to make live mode safe under floods, a protective default whose value is
precisely that adopters don't have to know about it. Document in FEATURES as
protective rather than retiring. `WithShutdownGracePeriod` verdict: keep —
it is the opt-in behind two-phase graceful drain (probe.go:174) and just
received a correctness fix in `[Unreleased]` (dead-window skip); retiring it
would be an owner call with no consumer cost today. Everything else has ≥1
verified adopter.

## 2026-10-27 post-v0.8 re-verification (the consumption gap)

**Method:** vendor-excluded `grep -rlE` for each symbol over the local checkouts
of the nine repos that matter most (dashboard, cv, dnsblockd, DiscordSync,
KeyHolderAI, cqrs-htmx, go-appkit, go-taskqueue, company-master-data), plus the
bridge `go.mod` pins. This is the same method as the two earlier sweeps; it
measures what is _in source_, not what a repo could adopt.

**Headline:** v0.7.0/v0.8.0 shipped 11 consumer-facing capabilities and the
fleet's adoption of the newest six is zero. That is not a documentation failure
— it is a release failure: neither bridge module is `go get`-able
(`go-appkit/health` v0.3.0 is a local-only tag; `cqrs-htmx/health` has no module
tag at all), so ~24 of the 36 reachable repos cannot resolve the wave.

| Capability (shipped in)                                | Symbol grep                                           | Adopters found                                                                           | Verdict                                                  |
| ------------------------------------------------------ | ----------------------------------------------------- | ---------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Freshness primitives (v0.8.0)                          | `CachedResponseFresh`, `EvaluateAndCache`, `CacheAge` | **0 files**                                                                              | zero adoption — the four hand-rolled adapters remain     |
| `Response.Clone` (v0.8.0)                              | `Response.Clone` / `.Clone()`                         | dashboard only                                                                           | single adopter                                           |
| `ChainRecorders` (v0.8.0)                              | `ChainRecorders`                                      | **bridge re-export only** (`cqrs-htmx/health/probe.go` + its test); no consumer calls it | zero consumer adoption                                   |
| `ErrRecorderNotApplicable` (v0.8.0)                    | `ErrRecorderNotApplicable`                            | none (it is a rejection, not an adoption)                                                | loud-on-bump; `go-appkit` still documents the silent era |
| Per-check `Since` + `DurationNanos` (v0.8.0)           | `DurationNanos`, `detailed-checks`                    | dashboard (`timedScreenshotRecorder`)                                                    | single adopter                                           |
| Merge-surface per-source roll-ups / `Healthz` (v0.8.0) | `SourceStatuses`, `Healthz`                           | dashboard (4 files; go-taskqueue only vendors dashboard's copy)                          | single deep adopter                                      |
| Federation drain (v0.8.0)                              | `MarkShuttingDown` on a `federation.Prober`           | **0** — dashboard's hub still shuts down without the drain                               | payoff not wired (dashboard repo)                        |
| `NewChecks` (v0.3.0) / `checks` batteries (v0.5.0)     | `NewChecks`, `checks.Disk`                            | webphone, dashboard example, typespec-eventsourcing (tests)                              | healthy, low volume                                      |

**What this changes in this table:** the rows above the divider describe
_capability_, not _use_. Until M01-M04 of the consumption-wave plan land, every
"FULLY_FUNCTIONAL" row that names a v0.8.0 symbol should be read as "shipped and
tested here; unused in the fleet".

**Evidence files:** `docs/research/2026-10-27_fleet-consumer-needs.md` §1 (reach
and skew), `docs/status/2026-10-27_21-02_distribution-shipped-consumption-zero.md`
(bridge/pin state), `docs/recorder-miswire-audit.md` (the one trap class that is
mechanically clean).
