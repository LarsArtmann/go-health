# Status: Pareto master plan executed & verified — go-health

**Date:** 2026-10-03 03:11 CEST
**Session scope:** 2026-10-02 ~16:11 → 2026-10-03 03:11. Two phases: (1) full execution of
`docs/planning/2026-10-02_16-11_right-way-pareto-master-plan.md` (all 27 L1 tasks),
(2) a verification pass that audited my own output and fixed the gaps it found.
This report covers that session only. The two 2026-10-02 status reports
(13-48, 15-06) and the plan file are frozen snapshots — this one supersedes them.

---

## a) FULLY DONE

**Tier 1 — the footgun, proven then fixed (the session's core promise)**

| Item | Evidence |
| --- | --- |
| Repro tests for the critical-name typo bug class | `probe_critical_names_test.go`; discovered en route that the startup latch only flips via `StartupHandler` requests (handlers.go:110), never via `Start`'s initial batch — the first control test encoded the wrong mental model and was rewritten |
| samber/do enumeration verified | `ListProvidedServices() []ServiceDescription` confirmed in v2.1.0 source — then **rejected** as the validation universe |
| Fleet compat scan | Every `WithCriticalServices` call site inspected; decisive discovery: Zlota44 + projects-management-automation build probes over **empty injectors + recorders**, so injector-list validation would false-positive on real consumers |
| Gating decision | Hard error at `Start()`, batch-based; hook/dev-strict rejected; no escape hatch — rationale + acceptance criteria in `docs/start-validation-design.md` |
| `ErrUnknownCriticalService` shipped | `Start()` validates critical names against the initial evaluation batch before publishing the cache or launching the loop; error names unknown services sorted; 8 unit tests incl. no-leak-on-failure; README troubleshooting + CHANGELOG; ExampleProbe_Start fixed to register all critical services |

**Tier 2 — design truth + knowledge capture**

- AGENTS.md: consumer inventory rebuilt (**15 direct** — go-taskqueue was the resolution miss; 16 indirect via 2 bridges), 4 implementation patterns with file refs, validation note, `health/checks` paragraph, 15 new docs registered in the Project Docs table.
- Harvest: TODO_LIST + ROADMAP updated; both status reports' §f lists (50 + 50 items) audited item-by-item in pass 2; leftover ideas routed (dashboard-side inventory, inventory-as-federation-remote, metadata-map rejection → ROADMAP; remaining suites + CV skew → TODO_LIST).
- README golden path: constructor decision table (6 paths), version-stamping recipe (ldflags + VCS note, `WithVersionFromBuildInfo` rejected), quickstart hint about the new validation, "What go-health is NOT" section, TOC fixed.

**Tier 3 — genre, recipes, bridges, adoption truth**

- Genre doc `docs/system-status-vs-probe.md` (paperless-ngx pinned at `d8b2e70b`, 2026-09-18; test file skimmed: 19 tests) + **ADR-007** (no system inventory, ever) + FEATURES "Deliberately NOT included" + README NOT-section + threat model `docs/probe-threat-model.md` cross-linked from SECURITY.md.
- Cookbook `docs/system-checks-cookbook.md`: disk/memory recipes with fleet thresholds, DB-metadata trap (secrets in error text), dnsblockd warn semantics, `host/` naming convention, fir's disk-critical-is-kubelet-correct rationale.
- Bridge inspections: go-appkit/health (`NewProbe` → standalone path; per-check panic isolation is *stronger* than core; gap G-A1: no detailed variant) and cqrs-htmx/health (empty-injector + recorder; projection state mapping honest; gap G-C1: no `DetailedHealthRecorder`). Golden-path contract + both upstream issue drafts written (`docs/announcements/2026-10-02_*.md`) — filing is an owner call.
- Adoption matrix `docs/adoption-matrix.md`: per-feature sweep across the fleet; **corrected the earlier session's false claim** — aggregate AND federation each have one deep consumer (the dashboard), the "non-adoption" belief rested on a non-alias-hardened grep. go-taskqueue confirmed direct (v0.4.1, `internal/webui/health.go`). Ghost verdict: only `WithLiveThrottle` has zero adopters → keep, document-as-protective.

**Tier 4 — implementation + integrity + tail**

- `health/checks` shipped: `Disk`, `Memory`, `HTTP`, `Database` — zero-dep (tests use an in-process `sql.Register` fake driver; no driver import ever), warn-by-default, thresholds-as-arguments. Full test suite + README wiring section.
- Wire integrity pinned: `encoding/json/v2` marshals nil `Checks` as `{}`, never `null` — empirically verified then locked with `TestResponse_JSONNilChecksMarshalAsEmptyObject`. openapi-lockstep green.
- Consumer verification train: all 8 direct apps **build**; CV + dnsblockd health suites **pass**. Version-skew table: everything v0.4.1 except **CV at v0.1.3 + `go 1.26.7`** (doubly stale; works, but the bump needs the 1.27 toolchain).
- v0.5 design set: `ServiceName` typed identity (Option A recommended, migration plan), merge unification (internal `mergeResponses` primitive), naming integrity (docs shipped; staged rename list incl. `SanitizeResponse`→`CoerceValidUTF8`, `Since`→`StatusSince`, `WithGETOnly` removal), vocabulary reconciliation (checks-vs-services, "health source" lexicon — DOMAIN_LANGUAGE.md updated), synthetic-overlay rules (dnsblockd pattern legitimized with the clone-before-write rule), automation notes (**both ideas rejected** with revisit triggers).
- Fresh-user simulation: README quickstart in a temp module **fails actionably** on an unregistered critical service — the new guard works for exactly its intended audience.

**Verification pass (self-audit)**

- Found and fixed: stale README TOC; AGENTS.md missing `health/checks`; FEATURES.md missing the two new features; DOMAIN_LANGUAGE.md not yet carrying the lexicon; design note missing acceptance criteria + the Validate()-stays-config-only decision; one broken ADR link; plan header still said "awaiting approval".
- Gates at end of session: `nix flake check` pass · fuzz **PASS** (205k execs, corpus grew) · build OK · test 4/4 · race 4/4 · lint **0 issues** · vet clean · openapi-lockstep covered · working tree clean (auto-daemon committed).

## b) PARTIALLY DONE

| Item | State | What remains |
| --- | --- | --- |
| Consumer verification train | 8/8 build, 2/8 suites run | fir, KeyHolderAI, DiscordSync, go-taskqueue, webphone, nsfw-classifier suites (TODO rows added) |
| Bridge upstream work | Golden-path doc + 2 complete drafts | Actually filing the go-appkit/cqrs-htmx issues (owner-publish by design) |
| Release | Everything sits in CHANGELOG `[Unreleased]` | v0.4.2 cut (validation + checks), tag, proxy verify, dashboard consumer check |
| CV currency | Skew identified and recorded | The actual bump (separate repo, `go 1.26.7` → 1.27 + go-health v0.4.x) |
| Pre-existing owner rows | Untouched (correctly) | v0.1.1/v0.1.2 announcement publish, samber/do#318 comment, coverage-threshold policy |

## c) NOT STARTED

- v0.5 window itself (ServiceName, renames, `WithCriticalChecks`) — design-only by plan; nothing executed.
- Bridge detailed variants (G-A1/G-C1 implementations) — upstream first.
- Any CI/skew automation (version-skew table is manual).
- `health/checks` fuzz target + benchmarks (package is tested, not fuzzed).
- docs/status archival: the two 2026-10-02 reports are harvested but still sit unarchived (ANNOTATE + move to `docs/status/archived/` when next touched).

## d) TOTALLY FUCKED UP

No broken artifacts shipped — final state is all-gates-green — but four process failures cost real cycles and one nearly caused damage:

1. **Wrote tests before reading the latch write-site.** My first repro assumed `Start()`'s initial batch flips the startup latch; it does not (only `StartupHandler` does). The control test failed and forced a rewrite. Lesson I keep re-learning: read the *mutation* site, not just the read site, before writing assertions.
2. **Write-tool race on my own file.** I overwrote `probe_critical_names_test.go` while a stale buffer was loaded, the write was rejected, and I then ran tests against the old file wondering why old tests still failed — burning a cycle on a tool contract I already knew.
3. **Nearly violated the zero-dependency constraint.** The checks tests initially imported `modernc.org/sqlite` — a test-only dep that would still have poisoned go.mod and the single-dependency story. Caught by typecheck *before* `go mod tidy`, rewritten with `sql.Register` fake. The AGENTS.md constraint was in my context the whole time; I reached for the familiar driver first.
4. **Trusted stale LSP diagnostics over gates.** Repeatedly the diagnostics showed lint warnings from pre-fix file states; I learned to treat them as hints and verify via `nix run .#lint`/`.#test`. No wrong fix shipped because of it, but I nearly chased ghosts twice.

Also worth recording: `TestStart_FailedStartLeavesNoBackgroundLoop` first encoded the *opposite* contract (cache populated before validation). Restructuring `Start` to validate before publishing was the better design — but the test should have been written from the invariant ("failed Start = side-effect free"), not from the first implementation's accident.

## e) WHAT WE SHOULD IMPROVE

1. **Test-first means invariant-first.** Derive assertions from the contract, then read mutation sites; never from the first implementation shape. (Failure #1/#4.)
2. **Constraint check before any new import** — even test-only — against AGENTS.md's dependency posture. (Failure #3.)
3. **Verify repo state between overwrites** (re-View before re-Write). (Failure #2.)
4. **L1 estimates were directionally right but L2-level timing drifted** (A4 cost extra for the latch semantics; D3 builds were far cheaper than budgeted). Re-baseline future plans on actuals, not the plan's guess.
5. **AGENTS.md inventory went stale between sessions** (9→14→15 direct consumers across three claims). The adoption matrix is now the source of truth with AGENTS.md pointing at it — keep it that way, and re-verify the matrix itself at every release.
6. **Line-number references in DOMAIN_LANGUAGE.md rot** (`probe.go:123` etc. drift with every edit). Prefer symbol names + file over bare line numbers in memory docs.
7. **The `checks` package has tests but no fuzz/bench** while the root package has both — asymmetry a future session will flag.

## f) Up to 50 things we should get done next

*Brainstorm, not commitment list. Top rows are release-critical; the tail is ROADMAP fuel.*

| # | Task | Bucket |
|---|------|--------|
| 1 | Cut v0.4.2: `ErrUnknownCriticalService` validation + `health/checks` (CHANGELOG ready; verify dashboard consumer against the tag pre-release) | release |
| 2 | File the go-appkit/health detailed-probe issue (draft ready in docs/announcements/) | upstream |
| 3 | File the cqrs-htmx/health detailed-recorder issue (draft ready) | upstream |
| 4 | Run remaining consumer suites: fir, KeyHolderAI, DiscordSync, go-taskqueue, webphone, nsfw-classifier | verify |
| 5 | Bump CV: go-health v0.1.3→v0.4.x + `go 1.26.7`→1.27 (skew table row) | consumer |
| 6 | Re-verify go-health-dashboard suite against v0.4.2 pre-tag (the one deep aggregate+federation consumer) | verify |
| 7 | `WithGETOnly` removal train: nudge KeyHolderAI (sole consumer), then remove in v0.5 per deprecation-policy | deprecation |
| 8 | Aggregate/federation integration test: probe-with-unknown-critical-name inside a source (validation composes, aggregate reports the source unhealthy) | test |
| 9 | Add godoc examples for `health/checks` (package example + NewChecks composition) | docs |
| 10 | Document `WithLiveThrottle` as protective in FEATURES (adoption-matrix verdict) | docs |
| 11 | Add "Fleet" section to README linking docs/adoption-matrix.md (sales proof: 15 direct + 16 indirect) | docs |
| 12 | doc.go package documentation: mention Start-time critical-name validation | docs |
| 13 | Archive the two 2026-10-02 status reports (docs-health ANNOTATE → archived/) | docs |
| 14 | Harvest this report's §f into TODO_LIST/ROADMAP (docs-health loop) | docs |
| 15 | Fuzz target for `health/checks` (Disk/Memory/HTTP paths) | code |
| 16 | Benchmarks for `checks` constructors (cost per evaluation) | code |
| 17 | Consider `checks.TCP(addr, timeout)` (dial check) — fleet need scan first | design |
| 18 | Version-skew CI check: script comparing fleet go.mod pins against latest tag | automation |
| 19 | Evaluate `WithInstanceID` verdict (single consumer = dashboard): owner confirm document-as-intended | decision |
| 20 | Open the v0.5 window decision: bundle ServiceName + renames + WithGETOnly removal, or stage | decision |
| 21 | ServiceName migration prep: per-repo call-site list → mechanical rewrite script | v0.5 |
| 22 | Vocabulary: decide `WithCriticalChecks` alias-vs-rename for v0.5 | v0.5 |
| 23 | Merge unification: port aggregate+federation property tests onto `mergeResponses` when window opens | v0.5 |
| 24 | cqrs-htmx: verify `ProjectionStatusEntry` carries timing (feasibility for the DetailedRecorder draft) | upstream |
| 25 | go-appkit: define `DetailedCheckFunc` type for the upstream proposal (make the draft concrete) | upstream |
| 26 | Replace bare line-number refs in DOMAIN_LANGUAGE.md with symbol names (anti-rot) | docs |
| 27 | `checks` asymmetry: add seam-style tests if any constructor grows options | code |
| 28 | Threat model: link from README (currently only SECURITY.md points at it) | docs |
| 29 | Confirm CV/result + fir submodule pins in the skew table (only CV parent checked) | verify |
| 30 | nsfw-classifier `WithAllowedMethods()` empty-call audit (guard with zero methods = GET-only? verify semantics documented) | verify |
| 31 | Announce v0.4.2 draft in docs/announcements/ (validation behavior change needs a consumer-facing note) | release |
| 32 | Evaluate `checks.Goroutines(max)` (goroutine-leak check) — brainstorm, only if fleet asks | design |
| 33 | README troubleshooting: "aggregating a probe that failed Start" entry (cache-empty semantics) | docs |
| 34 | Consider exporting validation as test helper (consumers' guard tests can assert the same contract) — needs API review | design |
| 35 | go-taskqueue pattern classification (A/B/C/D) for the matrix's pattern map | survey |
| 36 | Federation + validation: document that remote names never hit `ErrUnknownCriticalService` (fetch-side is different universe) | docs |
| 37 | openapi.yaml: confirm no delta from checks/validation (lockstep green; keep proving each release) | verify |
| 38 | Benchmark `validateCriticalNames` (should be ~ns; assert it stays trivial) | code |
| 39 | ROADMAP: capture "checks registry/metadata" only if a second consumer asks | docs |
| 40 | Coverage: measure `checks` package coverage; close gaps | code |
| 41 | Bridge drafts: attach failing-`duration_ns` example JSON to make upstream issues vivid | upstream |
| 42 | Consider `health.NewChecks` README example swap to use `checks` (golden path coherence) | docs |
| 43 | Check fleet for anyone calling `Start` before providing services (validation's boot-order risk in the wild) | survey |
| 44 | Add CHANGELOG links to the three decision docs (start-validation, batteries, ADR-007) | docs |
| 45 | Weekly long fuzz: confirm new tests didn't invalidate corpus (signatures unchanged — verify once) | verify |
| 46 | Evaluate a fleet-consumer smoke repo (tiny app importing checks + validation for CI-level proof) | design |
| 47 | DOMAIN_LANGUAGE: entry for "validation universe" (batch) after fleet feedback | docs |
| 48 | Pin golangci nolint_filter warning suppression strategy (pre-existing noise, no action exists — document-and-ignore stands) | docs |
| 49 | Website: checks + validation deserve a section if the go-health site refreshes (website-launch lever) | design |
| 50 | Re-run fresh-user sim after v0.4.2 against the released module (no replace directive) | verify |

## g) Three questions I cannot figure out myself

1. **Release timing:** ship `ErrUnknownCriticalService` + `health/checks` as **v0.4.2 now**, or hold the release until the bridge/upstream outcomes land so consumers bump once? The validation is a behavior change (boot can now fail loudly) — do you want it in a patch release or announced separately first?
2. **Single-consumer verdicts:** `WithInstanceID` (only the dashboard uses it) and deprecated `WithGETOnly` (only KeyHolderAI's legacy path) — do you confirm **document-as-intended / remove-at-v0.5**, or do you want a consumer-nudge round first before locking those verdicts?
3. **Cross-repo authority:** should I execute the **CV bump** (go-health v0.4.x + `go 1.27`) in the CV repo myself, or is that repo owned by a separate flow you run — i.e., are consumer bumps mine to do or only mine to record?

---

*Point-in-time snapshot. Section (f) is the HARVEST input for TODO_LIST/ROADMAP; routing happens on the docs-health loop, not by entombing here.*
