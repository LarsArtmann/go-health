# Status Report — Wire-Format Comparison (paperless-ngx) & Consumer Implementation Survey

**Date:** 2026-10-02 13:48 CEST
**Session scope:** Read-only analysis. No code changed. One deliverable: the comparison answer in chat, now persisted here.
**Trigger:** "Do we include Version / Install-Type / Server OS / Storage / Databases? How does it compare to paperless-ngx? How are real projects implementing us?" (+ 30-consumer tree from paste)

---

## TL;DR

- go-health deliberately exposes **health semantics only**: `status`, opt-in `version`, `instance_id`, `uptime`, `shutting_down`, `total_latency_ms`, `timestamp`, and per-check `status/error/since/duration_ns`. **No install type, no server OS, no storage stats, no database metadata blocks.**
- paperless-ngx's `/api/status/` is a different genre: an **authenticated, staff-only, synchronous admin diagnostics page** (`install_type`, `server_os`, `storage{total,available}`, `database{type,url,status,error,migration_status}`, redis/celery/index/classifier/sanity/llmindex task blocks). go-health is an **unauthenticated, cached kubelet probe**. Same word ("health"), different contract.
- The fleet (13 local checkouts surveyed of 30 consumers) fills the metadata gap **with custom checks, not structured fields**: file-and-image-renamer's `CheckDiskSpace` (critical), CV's `SystemResources` (filesystem/memory/disk). Four distinct implementation patterns identified (A: injector, B: check-func map, C: recorder bridge, D: framework bridge).
- **New finding from the survey:** the consumer tree in this session's paste shows **14 direct consumers**; AGENTS.md's inventory section lists ~9 and misses nsfw-classifier, webphone, go-taskqueue, projects-management-automation, and cqrs-htmx as direct consumers. Stale doc, not yet fixed (see (f) item 1).

---

## What this session actually did (evidence trail)

| # | Action | Evidence |
|---|--------|----------|
| 1 | Read the go-health wire format | `types.go:81-106` (Response), `types.go:61` (`duration_ns,omitzero`), `probe.go:121` (`WithVersion`), `probe.go:128` (`WithInstanceID`) |
| 2 | Located and read paperless-ngx system status | local checkout `paperless-ngx/src/documents/views.py:4137-4410` (SystemStatusView, schema + implementation) |
| 3 | Surveyed go-health API usage across 13 local consumer checkouts | go-appkit, cqrs-htmx, dnsblockd, CV, library-policy, DiscordSync, KeyHolderAI, file-and-image-renamer, go-taskqueue (empty — unresolved), nsfw-classifier, zlota44, webphone, projects-management-automation |
| 4 | Checked version/aggregate/federation adoption | `WithVersion`/`VersionHandler` in dnsblockd, KeyHolderAI, fir, nsfw-classifier; aggregate/federation imports found in **no** app consumer (grep-based; not alias-hardened) |
| 5 | Answered the three questions in chat | comparison table + 4-pattern survey |

---

## a) FULLY DONE

1. **Wire-format inventory of go-health** — every Response field with its option source (`types.go:81-106`): `status` (pass/warn/fail), `version` (opt-in `WithVersion`, probe.go:121), `instance_id` (opt-in, probe.go:128), `uptime`, `shutting_down`, `total_latency_ms`, `timestamp` (omitzero), `checks{name → status, error, since, duration_ns}`. Separate `/version` endpoint (VersionHandler) as build identity.
2. **paperless-ngx `/api/status/` full field inventory from source** — `pngx_version`, `server_os` (`platform.platform()`), `install_type` (bare-metal/kubernetes/docker sniffed from `KUBERNETES_SERVICE_HOST` / `PNGX_CONTAINERIZED`), `storage{total,available}` via `statvfs(MEDIA_ROOT)`, `database{type, url, status, error, migration_status{latest, unapplied}}`, plus redis/celery/index/classifier/sanity-check/llmindex task blocks (views.py:4213-4410). Auth posture confirmed: `IsAuthenticated` + `is_staff`, synchronous per-request.
3. **Genre distinction articulated** — probe (unauthenticated, background-cached, three-state rollup, kubelet semantics) vs status page (authenticated, on-demand, rich structured diagnostics); consistent with go-health's existing rejected-designs (content negotiation, ETag, classification 2.0) and the scalars-don't-survive-merges rule.
4. **Consumer implementation-pattern survey (13 checkouts)** with file:line evidence:
   - **Pattern A — injector path:** `health.New(injector, WithCriticalServices(...))` — KeyHolderAI (di.go:439-447: SSE hub, ResilientAIService, PersonaRepository critical), CV (internal/di/health_probe.go), DiscordSync (health_dashboard.go:73-86).
   - **Pattern B — `NewWithHealthCheck` func map:** dnsblockd (health.go:47-67: `database`, `dns` critical; `blocklist-sources` non-critical → warn), library-policy (httpapi/health.go:36).
   - **Pattern C — recorder bridge:** cqrs-htmx/health `NewProbe` — one named check per CQRS projection merged with injector checks via `projectionRecorder` (probe.go:25-45).
   - **Pattern D — framework bridge:** go-appkit/health `NewProbe(map[string]CheckFunc)` + doadapter (mount.go, doadapter.go) — consumed indirectly by Rolls-Royce-mtuGoHelpCenter-golang.
5. **System-resource checks exist only consumer-side** — fir `CheckDiskSpace` via `syscall.Statfs` (pkg/injector/health_checks.go:32-34, critical) + `CheckAIProvider`; CV `SystemResources` (internal/health/systemresources.go: filesystem, memory, disk) — pass/fail only, no usage numbers anywhere in the fleet.
6. **Answer delivered in chat** — comparison table, gap analysis, pattern survey, dashboard flagged as the composition point for paperless-style diagnostics.

## b) PARTIALLY DONE

1. **Consumer survey coverage** — 13 of 30 consumers inspected directly; the 16 indirect ones (via cqrs-htmx v4.7.0–v4.13.0, go-appkit v0.5.1/v0.7.0) were not opened. Coverage is broad but not exhaustive.
2. **aggregate/federation non-adoption claim** — based on one grep pass over member access (`aggregate\.|federation\.`); not alias-hardened (an import aliased `healthaggregate` could slip through). go-health-dashboard's federation usage was taken from AGENTS.md, not re-verified in dashboard source this session.
3. **go-taskqueue** — grep for the surveyed constructors returned nothing. Either it uses go-health differently (alias, re-export, go-appkit bridge) or not at all. Unresolved.
4. **Feature-adoption audit** — `WithVersion` checked; `WithInstanceID`, `NewChecks`, `NewWithDetailedCheck`, `DetailedHealthRecorder`, `WithEvaluationHook`, `WithAllowedMethods`, `MarkShuttingDown`, `AwaitReady`, `Healthz` adoption **not** audited.
5. **paperless-ngx comparison depth** — read the view, not its tests (`test_api_status.py`) and not the checkout's git pin; the llmindex block suggests a recent main, so the comparison target's exact version is unrecorded.

## c) NOT STARTED

1. **Persisting the findings** — AGENTS.md consumer-inventory update (14 direct consumers vs ~9 listed), FEATURES.md "deliberately not included" section, ROADMAP raw idea (system-inventory composition), TODO_LIST harvest. Nothing written until this report.
2. **A durable genre-comparison doc** — the paperless-ngx case study exists only in chat + this report; no `docs/` design doc or ADR.
3. **Contrib/recipes for consumer-written system checks** — CV's `SystemResources` and fir's `CheckDiskSpace` are near-identical needs solved twice privately; no shared module or documented recipe.
4. **Version-skew analysis of indirect consumers** (cqrs-htmx spread v4.7.0→v4.13.0; go-appkit v0.5.1/v0.7.0) against v0.4.1.
5. **Any code changes** — none were required; session was read-only by design.

## d) TOTALLY FUCKED UP

Nothing destructive (read-only session), but brutal honesty about the sloppy parts:

1. **A broken bash call** — `rg -rn "" -l` dumped go-health's entire file list into the output (misconstructed grep). Harmless, but noise, and it wasted a roundtrip.
2. **Overconfident claim** — "nobody in the fleet exposes structured metadata" was presented as fact while resting on a single, non-alias-hardened grep pass. Directionally almost certainly right, but stated stronger than verified.
3. **First paperless grep was too broad** — pattern `install_type|server_os|storage|database` matched 40 `storage_path` lines before the target; cost an extra roundtrip to re-scope.
4. **Memory-protocol violation** — per AGENTS.md's aggressive update protocol, the new durable facts (4 consumer patterns, 14-direct-consumer inventory, go-taskqueue anomaly) should have been written to AGENTS.md **at discovery**, not left in chat. This report is the recovery, not the compliance.

## e) WHAT WE SHOULD IMPROVE

1. **Verify adoption claims against import lines**, not member access (alias-safe greps), before asserting fleet-wide negatives.
2. **Primary-source discipline applies to in-repo docs too** — dashboard's federation usage should be re-confirmed in dashboard source, not inherited from AGENTS.md (the same verify-external-claims posture, pointed inward).
3. **Persist durable findings immediately** (memory maintenance), especially consumer-inventory facts that go stale silently.
4. **Record the comparison target's version/pin** whenever comparing against an external project's main branch.
5. **Tighter first-pass greps** — scoped patterns (`install_type|server_os|pngx_version`) would have saved the re-scope roundtrip.

## f) Top 50 things we should get done next

*Brainstorm, not commitment list (per status-report skill: >25 items are ROADMAP fuel; docs-health HARVEST must apply routing rigor). Sorted roughly by impact.*

| # | Task | Bucket |
|---|------|--------|
| 1 | Update AGENTS.md consumer-inventory: 14 direct consumers (add nsfw-classifier, webphone, go-taskqueue, projects-management-automation, cqrs-htmx) | docs |
| 2 | Update AGENTS.md with the 4 consumer implementation patterns (A–D) + adoption facts | docs |
| 3 | Alias-safe re-verification of aggregate/federation non-adoption (grep import lines `go-health/aggregate`, `go-health/federation`) | verify |
| 4 | Resolve go-taskqueue's go-health usage (empty grep — check go.mod, aliases, bridge) | verify |
| 5 | Re-verify go-health-dashboard federation usage in dashboard source (not via AGENTS.md) | verify |
| 6 | Write the genre-comparison doc: probe vs status page, paperless-ngx case study (candidate: `docs/system-status-vs-probe.md` or rejected-design ADR-007 "no system inventory on the probe") | docs |
| 7 | FEATURES.md: add "Deliberately NOT included: install-type, server OS, storage stats, DB metadata" with rationale links | docs |
| 8 | README: add "What go-health is NOT" section (probe, not a diagnostics page) — sales framing | docs |
| 9 | ROADMAP.md: capture "opt-in system-inventory composition (dashboard-side)" as raw idea | docs |
| 10 | TODO_LIST.md: harvest this report's Top-50 via docs-health HARVEST | docs |
| 11 | Consumer-feature adoption matrix (consumer × WithVersion/WithInstanceID/NewChecks/DetailedCheck/Hook/method-guard) in docs/ | survey |
| 12 | Audit `WithInstanceID` adoption across all 30 consumers | survey |
| 13 | Audit `NewChecks` adoption — is the new named-checks constructor reaching consumers at all? | survey |
| 14 | Audit `NewWithDetailedCheck` / `DetailedHealthRecorder` adoption (v0.2.0 metadata — real users?) | survey |
| 15 | Audit `WithEvaluationHook` adoption (Prometheus composition story: real users or none?) | survey |
| 16 | Audit `WithAllowedMethods` method-guard adoption (is the guard dead weight?) | survey |
| 17 | Audit deprecated `WithGETOnly` usage across consumers (removal decision input) | survey |
| 18 | Audit `MarkShuttingDown` / `AsShutdowner` / `AwaitReady` adoption across the fleet | survey |
| 19 | Verify all direct consumers build against v0.4.1 (fleet currency sweep; 2026-09-28 check was presence-only) | verify |
| 20 | Run consumer test suites (not just go.mod presence) for the 8 direct app consumers — next verification train | verify |
| 21 | Version-skew audit of indirect consumers (cqrs-htmx v4.7.0–v4.13.0, go-appkit v0.5.1/v0.7.0) against v0.4.1 wire format | verify |
| 22 | go-appkit bridge fidelity: does v0.7.0 pass through `since`/`duration_ns` or drop them? | verify |
| 23 | Extract CV `SystemResources` (filesystem/memory/disk checks) into a reusable module (contrib candidate) | code |
| 24 | Extract fir `CheckDiskSpace` (Statfs threshold) into the same module — de-duplicate the fleet | code |
| 25 | Decide ownership for #23/#24: new repo vs docs recipes vs go-appkit/health | decision |
| 26 | `docs/detailed-checks-cookbook.md`: add "system resource checks" recipe referencing CV/fir | docs |
| 27 | Document the DB-metadata trap: why `database{type,url,migrations}` doesn't belong in checks (secrets in Error text, wire exposure) | docs |
| 28 | Document warn-semantics with dnsblockd `blocklist-sources` as the canonical non-critical example (README or DOMAIN_LANGUAGE.md) | docs |
| 29 | Verify `docs/openapi.yaml` matches the field list reported this session (lockstep gate) | verify |
| 30 | Diff reported fields against `testdata/readiness_response.golden` to confirm completeness (omitzero behavior) | verify |
| 31 | Security comparison note: paperless `/api/status/` is staff-only+authenticated; go-health endpoints unauthenticated by design — info-leak threat-model note (SECURITY.md or docs/) | docs |
| 32 | Document timing difference: paperless per-block timings vs go-health `TotalLatencyMs` + `WithEvaluationHook` seam | docs |
| 33 | go-health-dashboard: evaluate host/OS/storage panels fed by consumer-side checks (not probe fields) | feature |
| 34 | Standardize system-check naming convention (`host/disk`, `host/memory`) in DOMAIN_LANGUAGE.md | docs |
| 35 | Federation adoption analysis: which of the 30 consumers deploy multi-process and would benefit? | analysis |
| 36 | Aggregate adoption analysis: same for in-process multi-probe | analysis |
| 37 | Audit `Healthz` (aggregate single-endpoint) adoption | survey |
| 38 | Investigate dnsblockd health.go:134 manual `resp.Checks[...] = health.Check{...}` — bypasses `buildChecks` (no Since stamping): legit escape hatch or doc gap? | verify |
| 39 | Add README example: `WithVersion` + `VersionHandler` used together coherently (stamp once, serve both) | docs |
| 40 | Rejected-design doc if ever proposed: extensible metadata map on Response (scalars-don't-merge rationale) | docs |
| 41 | Check whether "system status page" vs "health probe" terminology is missing from DOMAIN_LANGUAGE.md | docs |
| 42 | Verify fleet consumers' `go 1.27` floor directives (go-health requires 1.27+ toolchains) | verify |
| 43 | Document fir's disk-CRITICAL choice: disk-full → readiness 503; liveness stays 200 (no restart cascade) — the kubelet-correct way to gate on storage | docs |
| 44 | Evaluate "system inventory as federation remote": tiny exporter serving inventory JSON merged via federation | feature |
| 45 | Check consumers for pre-extraction `WithPlugin`-era patterns (migration completeness) | survey |
| 46 | Check whether any consumer relies on live mode (`WithRefreshInterval(0)`) vs default caching — parameter-use distribution | survey |
| 47 | Record the paperless-ngx checkout pin (commit/date) used for this comparison; note llmindex block is newer-main | verify |
| 48 | Read paperless `test_api_status.py` for response shapes the view hides (error cases) | verify |
| 49 | Consider README wire-format example block (JSON sample) since the comparison showed outsiders misread scope | docs |
| 50 | Carry the 3 open questions (below) to a decision | decision |

## g) Three questions I cannot figure out myself

1. **Genre boundary:** Should "system status" data (OS, install type, storage stats, DB metadata) *ever* become a go-health feature (opt-in), or is the probe permanently minimal with that data living consumer-side / dashboard-side? This decides between writing a rejected-design ADR (cheap) and starting a feature design (expensive).
2. **Survey blind spots:** Are there consumers **not checked out locally** (other machines, private forks) whose health implementations I could not see — particularly any already using aggregate/federation or `NewChecks`? My "only the dashboard merges" claim is only as good as the local checkouts.
3. **Persistence:** Do you want this session's durable findings promoted now (AGENTS.md inventory + patterns update, plus the genre-comparison doc), or should they stay in this report until you call?

---

*Auto-commit daemon will pick this file up; no manual commit (harness rule: no commit without explicit request).*

**NEXT STEP AFTER THIS REPORT:** docs-health → HARVEST section (f) into TODO_LIST.md / ROADMAP.md — awaiting instructions.
