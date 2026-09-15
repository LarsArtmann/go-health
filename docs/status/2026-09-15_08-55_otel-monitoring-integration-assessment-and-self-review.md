# Status Report — OTEL/Monitoring Integration Assessment + Brutal Self-Review

**Date:** 2026-09-15 08:55 CEST · **Scope:** this session only (integration assessment of go-health against the SystemNix monitoring stack) · **Project:** go-health v0.1.3 (alpha)

---

## a) FULLY DONE

| # | Item                                                                                                                                                                                                                                                                                                                                                                                                                              | Evidence                               |
| - | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------- |
| 1 | **SystemNix monitoring inventory** — SigNoz (OTLP 4317/4318, ClickHouse, embedded alertmanager), signoz-otel-collector prometheus receiver (the ONLY scraper; no standalone Prometheus server), node_exporter + textfile collectors, Gatus (~70 endpoints → Discord + PapDashboard), monitor365 (agent :9191 / server :3001), plus smartd/watchdogd/systemd watchdogs/scripts. No Grafana/Loki/Jaeger/Tempo/uptime-kuma anywhere. | agent sweep over all SystemNix `*.nix` |
| 2 | **go-health observability surface audit** — `WithEvaluationHook` (sync, per-eval, `probe.go:149`), metrics-ready data model (`Check.Status/Since/DurationNanos`, `types.go:19-88`), Prometheus-by-composition decision (`docs/prometheus-exposition-design.md`), OTEL explicitly deferred to the same seam (design doc line 63).                                                                                                  | file reads + grep                      |
| 3 | **Seam verification, non-vacuous** — first pass used `nix run .#test -- -run ...` which could have passed vacuously; re-ran with `-v`: `TestWithEvaluationHook_InvokedPerEvaluation`, `ExampleWithEvaluationHook`, `ExampleWithEvaluationHook_metrics` all genuinely RUN and PASS.                                                                                                                                                | `go test -v` output                    |
| 4 | **Production consumer confirmed** — cv.nix:707 wires a go-health probe (`/health/live`) into Gatus with `[STATUS] == 200` + response-time conditions; Gatus is SystemNix's kubelet-equivalent.                                                                                                                                                                                                                                    | cv.nix read                            |
| 5 | **Assessment delivered** — per-solution fit matrix, verdict (design matches SystemNix's collector-side architecture), 3 ranked actions, top gap identified: no verified OTEL composition path.                                                                                                                                                                                                                                    | previous message                       |
| 6 | **Consumer version-drift check (added during self-review)** — SystemNix flake.lock holds THREE go-health pins: two at `274d19f` (= the v0.1.3 release commit exactly) and one STALE at `3d26c49` (v0.0.2-era, 2026-08-06, pre-aggregate, pre-hook).                                                                                                                                                                               | flake.lock + local tag mapping         |

## b) PARTIALLY DONE

| # | Item                                                                                                                                                                                                                                                            | What's missing                       |
| - | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------ |
| 1 | **OTEL verdict** — assessed as "seam exists, no verified path"; the actual spike was offered but not executed (awaiting decision).                                                                                                                              | The spike itself.                    |
| 2 | **monitor365 fit** — claimed "generic HTTP only, nothing obviously missing" WITHOUT verifying whether monitor365 has an HTTP-check/collector mechanism that could consume probes natively. Assertion, not verification.                                         | Read monitor365's collector surface. |
| 3 | **Gatus capability claims** — asserted `[BODY] pat(...)` works for JSON status assertions; did not verify against the Gatus version SystemNix actually pins.                                                                                                    | Version-checked confirmation.        |
| 4 | **Consumer enumeration** — found cv via `rg "go-health"`; services vendored as flakes (discordsync, bank-sync, overview) wouldn't match that string in SystemNix. The stale `go-health_3` pin PROVES at least one more consumer exists that I did not identify. | Identify the stale-pin consumer.     |
| 5 | **signoz-coverage interaction** — never checked whether a go-health-only service would pass SystemNix's eval-time coverage registry (every service must emit traces/logs). If it wouldn't, "integrates well" is only half-true for OTEL-first services.         | Registry wiring-class check.         |

## c) NOT STARTED

| # | Item                                                                                                                                                 |
| - | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | OTEL composition example (hook → OTLP gauges/logs via Go SDK, separate example module to keep go.mod single-dependency) — **the identified top gap** |
| 2 | `docs/gatus-integration.md` recipe extracted from the proven cv.nix pattern                                                                          |
| 3 | node_exporter textfile-collector snippet alongside the Prometheus example                                                                            |
| 4 | HARVEST of this report's section (f) into `TODO_LIST.md` / `ROADMAP.md` (docs-health loop-closure — deferred per "wait for instructions")            |

## d) TOTALLY FUCKED UP

Nothing data-destroying, but three honest failures:

1. **Weak initial verification, presented as stronger than it was.** The first seam check (`nix run .#test -- -run '...' ./...`) printed `ok` in 0.004s and I moved on. A `-run` filter matching nothing produces exactly that output — the pipeline-masking failure class already recorded in AGENTS.md, and I walked straight into it. Caught only during this self-review; fixed with `-v` (all 3 tests genuinely pass).
2. **Missed the version split-brain in the first pass.** My assessment said "cv is the production consumer, dashboard is the only known consumer." SystemNix's lock file was one command away and showed THREE go-health pins, one at v0.0.2-era — meaning an unidentified consumer ships a two-month-old go-health without the aggregate package, the hook, or panic recovery. The integration answer "we integrate well" silently assumed consumers run current code.
3. **"Nothing obviously missing" for monitor365 was an unverified hedge dressed as a finding.** I never opened the monitor365 module's check surface. Should have been labeled UNVERIFIED in the delivered matrix.

## e) WHAT WE SHOULD IMPROVE

**Process (this session):**

1. **Never trust a filtered test run without `-v` or an explicit count.** A green `ok` after `-run` is not evidence the test ran. Verify the instrument measured before citing it.
2. **Consumer claims require lock-file evidence, not grep evidence.** `rg "go-health"` finds direct consumers; transitive flake pins are invisible to it. Check `flake.lock` first.
3. **Label assertions as assertions.** The matrix mixed verified rows (Gatus, Prometheus) with unverified rows (monitor365) at the same confidence level. Every row needs an evidence column (which it had in the chat answer — but two cells contained hedges).

**Product (go-health, from this assessment):**

4. **OTEL is the real gap, and it's bigger than "missing example":** SystemNix's coverage registry _enforces_ OTEL per service. go-health's composition story is documented for Prometheus but only asserted for OTEL — the exact environment where the library will be consumed demands the unproven path.
5. **The hook's blocking-contract vs OTEL's network reality is undocumented tension.** `WithEvaluationHook` must be "fast and must not block" (probe.go:146), but an OTLP exporter is a network client. Consumers need a documented non-blocking handoff pattern (channel + draining goroutine + batch exporter), not just the raw seam.
6. **Hook panic safety is untested.** A panicking evalHook on the refresh-loop path — does it kill the loop? Panic recovery exists for health CHECKS (`runHealthChecks`) but I saw no evidence it wraps the hook call. One bad consumer callback could silently freeze the cache.
7. **Docs split-brain risk:** Prometheus has a design doc + verified example; Gatus/OTEL/textfile would each add a doc. Without one canonical "Integrations" index, the composition story fragments across files.

## f) Top 50 things to get done next

**P1 — close the verified gaps (highest impact):**

1. OTEL composition spike: hook → OTLP gauge (`health_check{service=...}`) + duration histogram + log-per-transition, via the official Go SDK, in a **separate example module** (keeps go.mod at one dependency).
2. Verify the spike against a real collector on `localhost:4318` (SystemNix's actual endpoint shape).
3. Non-blocking hook adapter example (channel + drain goroutine) — resolves the "must not block" vs network-exporter tension.
4. Test: panicking evalHook must not kill the refresh loop (add recovery or document the contract honestly).
5. Identify which consumer flake pins the stale `go-health_3` (v0.0.2-era) and bump it to v0.1.3.
6. Check whether a go-health-only service passes SystemNix's `signoz-coverage.nix` registry; register the wiring class if a new one is needed.
7. HARVEST this report → TODO_LIST.md (P1/P2) and ROADMAP.md (P3).

**P2 — documentation/integration recipes:**

8. `docs/gatus-integration.md`: the cv.nix pattern (liveness URL, `[STATUS] == 200` + response-time conditions, alert text style).
9. Recipe: readiness check (not just liveness) in Gatus — cv wires only liveness today; readiness + JSON body assertion (`pat(*"status":"pass"*)`) is the richer pattern.
10. Textfile-collector snippet next to `prometheus_example_test.go` for node_exporter-style consumers.
11. SigNoz PromQL alert-rule examples for the `health_check` gauge (matching `_signoz-alerts.nix` style).
12. `docs/integrations.md` index tying Prometheus/Gatus/OTEL/textfile recipes together (prevents doc fragmentation).
13. Document hook-stream vs served-response divergence (shutdown overlay appears in HTTP/cache but NOT in the eval hook stream — metrics built on the hook miss draining state).
14. Verify + document whether the hook fires on shutdown-overlay transitions at all.
15. Recipe: `WithInstanceID` as the `instance` label aligning with SystemNix's per-service scrape jobs.
16. Recipe: `WithShutdownGracePeriod` aligned with Gatus failure-threshold × interval so drains are observed as 503s before SIGKILL.
17. README: state the `GOEXPERIMENT=jsonv2` consumer requirement prominently (currently only in AGENTS.md).
18. Update FEATURES.md: add OTEL composition row (status: PLANNED/seam-only) next to the Prometheus row.
19. Update ROADMAP.md OTEL-spike item with a link to this report's findings.
20. Document aggregate + OTEL label namespacing (`source/check` → OTEL attributes).

**P3 — SystemNix-side follow-through:**

21. Audit which SystemNix Go services hand-roll `/healthz` instead of using go-health.
22. Adopt go-health in those services → one health pattern across the fleet.
23. Wire readiness endpoints (not just liveness) into Gatus for cv and future consumers.
24. Register future go-health services in `signoz-coverage.nix` once the OTEL example exists.
25. Add a "consumer version drift" step to the go-release checklist (grep consumer locks for stale go-health revs at release time).
26. Decide go-health-dashboard deployment in SystemNix (it's flaked but not deployed).
27. If deployed: dashboard as the Gatus/SigNoz visual complement; document the trio.
28. monitor365: verify HTTP-check capability; document consumption of go-health endpoints if supported.
29. If monitor365 is the long-term platform: design a native go-health collector instead of HTTP polling.
30. Add go-health probe endpoints to the `gatus-coverage-audit.nix` expectations so new services get probes by default.

**P4 — library hardening / ROADMAP fuel:**

31. Consider `WithCheckTransitionHook` (fires only on status change) — the alerting-natural shape; `Since` makes transitions derivable today, but an explicit hook removes per-consumer diffing bugs.
32. Benchmark hook overhead with an OTLP exporter attached (hot-path cost of the sync contract).
33. Example test asserting OTEL output against a mock OTLP receiver (spike must ship verified, like the Prometheus one did).
34. Revisit "writer stays an example" decision when the ~40-line Prometheus writer reaches 3+ consumers (duplication threshold).
35. Consider shipping the Prometheus writer as a tiny `health/promexp` subpackage if #34 triggers.
36. OTEL span-per-evaluation vs log-per-transition tradeoff analysis (SigNoz: logs are cheaper for alerting).
37. Full compose demo: app + Gatus + otel-collector + SigNoz as an integration showcase (roadmap).
38. E2E test running Gatus in a container against an example server (heavy; roadmap).
39. `AwaitReady` + systemd `Type=notify` recipe (readiness signaling beyond HTTP).
40. Guidance: `WithShutdownGracePeriod` vs systemd `TimeoutStopSec` coordination.
41. Assess `Response.Source` field for aggregate-level OTEL attributes (needs design doc first).
42. Aggregate probe + Gatus recipe for multi-process hosts (the SystemNix shape: many services, one host).
43. Fuzz the hook path (currently fuzz targets cover marshaling and handlers, not hook invocation).
44. Check OTEL semantic conventions for health (does a convention exist? align attribute names if so).
45. OpenAPI spec: add a note on the `/metrics` composition pattern (spec covers probes only today).
46. Verify cv repo actually calls `Start()` and checks its error (Validate-on-Start contract) — the SystemNix side proves wiring, not correctness of the consumer's lifecycle.
47. Announce the integrations docs (Gatus/OTEL) in the next release notes — discoverability is currently zero.
48. Cut v0.1.4 once the OTEL example + Gatus recipe land (docs-only + example-module release).
49. Re-run this integration assessment after each minor release (cadence).
50. Evaluate whether `aggregate` should expose per-source OTEL resource attributes when composing (ties into #20, #41).

## g) Questions I cannot answer myself

1. **Which services adopt go-health next?** (dnsblockd? overview? monitor365 components?) The answer decides whether the OTEL example or the Gatus recipe ships first — I can guess, but the fleet's adoption plan is yours.
2. **Is go-health-dashboard planned for deployment in SystemNix?** It's flaked but not deployed. If yes, per-service `/metrics` investment matters less than feeding the dashboard; if no, the OTEL/SigNoz path is the only visualization story.
3. **Is monitor365 the long-term replacement for Gatus/SigNoz, or a complement?** If replacement, deep Gatus/OTEL recipe work is partially throwaway; if complement, it's the highest-value documentation target right now.

---

_Format note: user explicitly requested Markdown; the status-report skill default is HTML. Honored per-user instruction. HARVEST into TODO_LIST.md/ROADMAP.md intentionally deferred — session told to wait for instructions after reporting._
