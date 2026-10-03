# Feature adoption matrix (fleet)

**Date:** 2026-10-02 · **Method:** alias-safe `rg` over local checkouts of every known consumer
(direct go-health imports + the go-appkit/cqrs-htmx bridges), go.mod requires

- source trace. "—" = zero adoption found in the fleet.

| Feature                                                             | Adopters (direct, verified in source)                                 | Verdict                                                                                                                     |
| ------------------------------------------------------------------- | --------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| core `New` + handlers + `WithCriticalServices`                      | all 14 direct consumers                                               | core                                                                                                                        |
| `WithAllowedMethods`                                                | dnsblockd, fir, KeyHolderAI, nsfw-classifier, PMA, Zlota44            | healthy                                                                                                                     |
| `WithGETOnly` (deprecated)                                          | KeyHolderAI (legacy path)                                             | document-and-retire candidate at v0.5 (nudge KeyHolderAI first)                                                             |
| `MarkShuttingDown`                                                  | CV, DiscordSync, KeyHolderAI, nsfw-classifier, go-appkit (+doadapter) | healthy                                                                                                                     |
| `WithEvaluationHook`                                                | dnsblockd, go-appkit/health                                           | healthy                                                                                                                     |
| `AwaitReady`                                                        | dnsblockd, go-appkit/health                                           | healthy                                                                                                                     |
| `WithNowFunc`                                                       | dnsblockd (tests)                                                     | healthy (test seam)                                                                                                         |
| `NewChecks`                                                         | webphone, go-health-dashboard example, typespec-eventsourcing (tests) | healthy, low adoption — keep (it is the batteries composition point)                                                        |
| `NewWithDetailedCheck` / `DetailedHealthRecorder`                   | go-health-dashboard only (`timedScreenshotRecorder`)                  | low adoption; feeds the bridge detailed-variant gap (docs/bridge-golden-path.md)                                            |
| `WithInstanceID`                                                    | go-health-dashboard                                                   | single consumer — document-as-intended (dashboard correlation), retire only with owner sign-off                             |
| `WithLiveThrottle`                                                  | —                                                                     | ghost: live mode + flood coalescing has no adopter. Keep (semantically required for safe live mode), document as protective |
| `WithShutdownGracePeriod`                                           | go-daemon, project-discovery-daemon                                   | healthy (infra consumers)                                                                                                   |
| `Healthz()` single-endpoint                                         | go-health-dashboard                                                   | low direct adoption; primary use is external LB composition — document-as-intended                                          |
| `aggregate` subpackage                                              | go-health-dashboard (multiple)                                        | single-consumer but deep (loadtests, SSE, fuzz) — healthy                                                                   |
| `federation` subpackage                                             | go-health-dashboard (`cmd/health-hub`)                                | single-consumer but production-wired — healthy                                                                              |
| `WithRefreshInterval`, `WithTimeout`, `WithVersion`, recorder paths | all consumers (spread)                                                | core                                                                                                                        |

## Resolved unknowns

- **go-taskqueue** (previously unresolved in the 2026-10-02 survey): IS a
  consumer — `go.mod` requires go-health **v0.4.1** + go-health-dashboard
  v0.10.1; check source at `internal/webui/health.go`. Direct-consumer count
  therefore 14 including go-taskqueue's webui (still "~14 direct" as stated
  in AGENTS.md; the survey's earlier "grep empty" was the resolution miss).
- **aggregate/federation "non-adoption" claim (earlier session): CORRECTED.**
  Both have one deep consumer (go-health-dashboard). The earlier claim rested
  on a non-alias-hardened grep; alias-safe search above finds real usage.

## Ghost-feature review input (E5)

Zero-adoption features: `WithLiveThrottle` only. Verdict: keep — it exists
to make live mode safe under floods, a protective default whose value is
precisely that adopters don't have to know about it. Document in FEATURES as
protective rather than retiring. Everything else has ≥1 verified adopter.
