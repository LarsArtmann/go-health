# Status Report — GitHub Issues #8/#9 (Routes surface) — Session 2026-11-04 ~19:20–19:52 CET

**Scope line.** This report covers ONE session: the review-and-fix of both open GitHub issues
(#8 "DefaultRoutes uses method-less patterns that panic overlapping muxes", #9 "Routes has no
slot for the version endpoint").

**Verified this session** (with artifacts):

- `nix run .#gates` → **EXIT=0** on the exact tree now at HEAD (test-race, vet, lint 0 issues,
  coverage-gate 99.9% total / 100% root / 100% aggregate / 99.3% federation, vulncheck clean,
  gosec 0 issues / 27 files, all 9 fuzz targets pass, fuzz-registry, interop, docs-check,
  flake check all green).
- New tests run individually with `-race -v`: all pass (root × 3, aggregate × 1, federation × 1).
- `gh issue list --state all`: #8 and #9 **CLOSED** (reason: completed), each with a
  voice-checked comment (check-draft: 0 FAIL, 0 WARN).
- `git fetch` + `rev-list origin/master..HEAD` → **0** (all session commits pushed; CI run
  41891168646 **in progress** at write time — result NOT yet verified).
- `git ls-tree` → temp draft files never entered git history; working tree clean.

**NOT verified** (and why):

- Remote CI verdict on the pushed commits (still running at write time).
- Fleet consumers: no `nix run .#skew` run, no scan of the 15 direct repos for the one
  breakage class this change can cause (see b1/c2).
- HEAD-request semantics under method-qualified defaults: reasoned from ServeMux docs
  ("GET pattern also matches HEAD"), **no test pins it** (see c1).
- ci-emulation, fuzz-long, BuildFlow full mode: not run (no new tool deps / no parser change /
  upstream binary still broken respectively).

---

## a) FULLY DONE

1. **Issue #8 fix** — `DefaultRoutes()` returns method-qualified patterns
   (`GET /healthz`, `GET /readyz`, `GET /startupz`), `handlers.go`. Rationale documented in the
   doc comment: kubelet probes are always GET, and the method prefix avoids the
   registration-time panic against `GET /{slug}`-style page routes. GET serving is unchanged;
   `DefaultRoutes()` consumers that only GET are unaffected.
2. **Issue #9 fix** — `Routes.Version` field added (`handlers.go`); all three `RegisterRoutes`
   surfaces honor it: root serves the probe's `WithVersion` stamp
   (`handlers.go` `Probe.RegisterRoutes`), aggregate and federation mount `VersionHandler("")`
   (documented: those surfaces carry no build identity, empty version is the existing wire
   truth for "unstamped"). `DefaultRoutes().Version` stays empty — the `/version` endpoint is
   opt-in, so consumers hand-wiring it today cannot double-register.
3. **Regression tests** — root: `TestDefaultRoutes_AreMethodQualified` (pins exact values),
   `TestRegisterRoutes_DefaultsCoexistWithCatchAllPage` (the issue #8 repro as a test),
   `TestRegisterRoutes_MountsVersionEndpoint` (stamp body `{"version":"v1.2.3"}`, POST → 405,
   default `Routes` → 404). Sub-packages: `TestRegisterRoutes_MountsVersionEndpoint` in
   `aggregate/routes_selective_test.go` and `federation/routes_selective_test.go`
   (`{"version":""}` + default-not-mounted).
4. **Existing-test migration** — 8 call sites that used `DefaultRoutes()` field values as
   request paths moved to literal paths (`probe_lifecycle_test.go` ×3, `probe_throttle_test.go`,
   `accessors_test.go`, `probe_throttle_cache_test.go` ×5).
5. **Lint fix at root cause** — first `.#gates` run caught `unparam` (literalizing made
   `assertFrozenWindow`'s `path` constant): dropped the constant `path` params from
   `requestReadiness`/`assertFrozenWindow` instead of suppressing. Second `.#gates` run: 0 issues.
6. **Docs sync (every gate-checked surface)** — CHANGELOG `[Unreleased]` (Added: Routes.Version;
   Changed: DefaultRoutes behavior change with migration note); README (route table row, two
   feature bullets, version-stamping recipe); FEATURES (3 rows, incl. stale `aggregate.go`/
   `federation.go` file references corrected to `handlers.go`); AGENTS.md version bullet
   extended in place (219/220 line cap preserved); ARCHITECTURE.md handlers.go + version.go map
   rows; docs/federation-design.md "wires only the three kubelet probes" claim made precise;
   `version.go` + `doc.go` doc comments now mention the `Routes.Version` wiring.
7. **Issue lifecycle** — terse fix comments posted on #8/#9 (github-voice register,
   mechanically checked), both closed as completed.
8. **Hygiene** — `nix fmt` + `dprint fmt` clean; temp comment drafts trashed before the
   auto-daemon ever captured them; final auto-commit (`3aee7e2`) verified to contain exactly
   the unparam fix + federation-design prose.
9. **Push** — all 4 session commits on origin/master; CI triggered.

## b) PARTIALLY DONE

1. **Consumer-impact verification for the behavior change.** `DefaultRoutes()` values changed —
   the documented break class is "code reading `DefaultRoutes()` fields as request paths".
   I fixed every instance in THIS repo (a4), but the fleet (15 direct repos, 36 dashboard
   consumers, five implementation patterns per docs/consumer-verification.md) was neither
   scanned nor skew-checked. The CHANGELOG note is the only mitigation shipped so far.
2. **Remote verification.** Local gates are green on HEAD's exact tracked content, but the CI
   run on the pushed commits had not finished at write time.
3. **docs/drafts/finding1_repro.** The owner already dropped the compiled binary; the source
   `main.go` remains and is compiled by every `./...` gate run (it shows up as a 0%-coverage,
   no-test-files package in gates output). Post-fix it no longer reproduces anything. I left it
   untouched deliberately (owner's artifact, referenced by the closed issue) — but that leaves
   a half-decided state: dead repro code inside the module graph.
4. **"Three kubelet probes" prose.** Made precise in docs/federation-design.md; the same
   absolute phrasing survives in two doc comments (`aggregate/handlers.go` Healthz comment,
   `federation/healthz.go`) where it is still true _for defaults_ — imprecise now that a fourth
   mount is possible.

## c) NOT STARTED

1. **HEAD-request pin test.** The fix's safety argument partially rests on ServeMux's documented
   "GET pattern also matches HEAD" behavior (LBs doing `HEAD /healthz` keep working). That
   load-bearing external contract is untested in this repo.
2. **Fleet skew / consumer scan** for `DefaultRoutes()`-field-as-request-path readers (b1).
3. **docs/consumer-verification.md timeline entry** for the v0.10-bound behavior change
   (per-release dashboard verification is the documented discipline; nothing added yet).
4. **TODO_LIST harvest** of this report's section (f) — user instructed report-then-wait, so
   routing into TODO_LIST/ROADMAP is pending (docs-health HARVEST).
5. **ci-emulation** run over the current tree (the new tests were never run under a go-free PATH).
6. **v0.10.0 release prep** — `[Unreleased]` now carries a behavior change; the release dance
   (section cut, tag, README/AGENTS stability lines) is untouched.
7. **Mux-level 405 shape test** — what exactly a non-GET request gets on a method-qualified
   default mount (Go's default 405 body + `Allow: GET` header) vs the guard's JSON 405 for
   disallowed-but-mux-allowed methods; no test pins the composition.

## d) TOTALLY FUCKED UP

Nothing destroyed, nothing forced, no data loss, no broken consumer. Two self-inflicted
iterations, both caught by the repo's own defenses and fixed at root cause:

1. **Missed 8 call sites on first pass.** When replacing `DefaultRoutes()` field reads, I
   edited only the sites visible in my first grep window; `accessors_test.go` (×3) and
   `probe_throttle_test.go` (×4) kept referencing the removed `routes` variable. Caught by
   project diagnostics — notably the golangci LSP panel was _right_ this time, while I had
   pre-labeled it a liar per AGENTS.md A18 and almost dismissed it. Real build arbitrated.
2. **First gates run red on unparam.** I literalized the request paths without checking the
   helpers' signatures, so `assertFrozenWindow` gained a provably-constant parameter. The gate
   did its job; the lesson (grep ALL callers of a helper before changing what feeds it) is in
   section (e).
3. **Minor:** first tool call of the doc-phase (`rg -rn`) was hook-blocked — the `-r` replace
   trap, which the hook message says is on its _third strike_ on this host. That trap is
   apparently not yet recorded where I would have learned it (candidate AGENTS.md gotcha, f11).

## e) WHAT WE SHOULD IMPROVE

1. **Preflight caller census.** Before changing any value that flows into a helper, grep ALL
   callers of that helper (and of the value's producer). Would have prevented both d1 and d2.
2. **Behavior changes to shipped defaults need a consumer pass in the same session** — a
   CHANGELOG "Changed" bullet alone is documentation, not verification. The fleet inventory
   (docs/consumer-verification.md) exists precisely for this; use it before the release cut,
   not after a dashboard breaks.
3. **Pin load-bearing external contracts with tests the moment a fix depends on them** (the
   HEAD-matching behavior, the mux-405 shape). Doc-recall is not a regression gate.
4. **Trust-but-verify tooling, even documented liars.** A18 says restart-don't-chase the
   golangci LSP panel; this session it was the only thing that caught d1. Arbitrate every
   diagnostic with a real build — including the ones the panel is "supposed" to get wrong.
5. **Issue hygiene after fixes:** comments should reference commits that are already pushed (or
   say "push pending"); a closed issue whose fix commits are not on origin yet reads as a
   broken promise to anyone clicking through during the gap.
6. **Repro lifecycle:** AI-filed issue repro drafts should carry a marker (e.g. `FIXED.md`) or
   be deleted in the fix commit, so the module graph doesn't accumulate dead `main` packages.

## f) Up to 50 things to get done next

_Ranked by impact; the top ~10 are this session's direct debt, the rest are noticed adjacent
items and ROADMAP fuel (docs-health HARVEST must apply extra routing rigor below ~#15)._

1. Scan the 15 direct fleet repos for `DefaultRoutes()` field values used as request paths; fix or annotate each (b1 — the only real breakage class of this change).
2. Add the HEAD pin test: `HEAD /healthz` → 200 under method-qualified defaults (c1).
3. Add the mux-405-shape test: non-GET on a default mount → 405 + `Allow: GET`, and its composition with `WithAllowedMethods` (c7).
4. Update docs/consumer-verification.md with the DefaultRoutes behavior-change row + per-repo verification plan (b1/c3).
5. Verify remote CI is green on the pushed fix commits (b2).
6. Run `nix run .#skew` (fleet skew) on the current tree.
7. Harvest section (f) into TODO_LIST.md / ROADMAP.md (docs-health HARVEST).
8. Decide `docs/drafts/finding1_repro` fate: delete the dir or keep it behind a FIXED marker (b3/f44).
9. Prepare and cut v0.10.0 (section cut, tag, stability lines, docs-check dance) — Unreleased carries a behavior change (c6).
10. Run `nix run .#ci-emulation` on the current tree (c5).
11. Record the `rg -rn` replace-trap as an AGENTS.md gotcha (d3).
12. Restore AGENTS.md headroom (219/220 — the next bullet that adds a line breaches the cap).
13. Repair the golangci LSP integration (standing A18 TODO; this session proved it is intermittently right, which makes its false negatives worse).
14. Make the two remaining "three kubelet probes" doc comments precise (b4).
15. Add a godoc example for `Routes.Version` wiring (root probe variant; aggregate/federation variant answering empty).
16. Add the bare-pattern `Version: "/version"` test (handler-level 405 path through `registerRoute`).
17. Add an all-zero `Routes{}` test (mounts nothing) on root + aggregate + federation.
18. Verify pkg.go.dev renders the new `Routes.Version` doc correctly after the release.
19. Check ROADMAP.md for a line this closes (partial route mounting / version endpoint).
20. Confirm openapi.yaml's `/version` entry documents the 405 response shape (read-only check; wire unchanged).
21. ADR candidate: identity semantics for merged surfaces' version mounts — empty-string answer vs rejecting non-empty `Routes.Version` at construction (see question g2).
22. CHANGELOG polish at release cut: turn "(issue #8/#9)" into full permalinks.
23. README main quickstart: add the optional `Version: "GET /version"` line to the RegisterRoutes call (recipe section already has it).
24. Sweep FEATURES.md rows for other file references invalidated by the file-size split (I fixed two; audit the rest).
25. Verify docs/ARCHITECTURE.md test-file layout still matches after this session's test additions (all edits landed in existing files — confirm the layout section agrees).
26. When the weekly budget is due: `nix run .#fuzz-long`.
27. Cheap weekly: `nix run .#recorder-audit` (unchanged surface, zero findings expected).
28. BuildFlow full mode: retry after upstream builds again (docs-gates provider unknown-revision blocks HEAD; advisory until then).
29. Watch nixpkgs gosec for the fixed export-data reader; drop the hardcoded `gosecGo` pin when it ships (securego/gosec #1772).
30. Watch samber-linter post-v0.4.0 release propagating through branching-flow to clear the doanalyzerv2 go-directive warning.
31. Consider extending fleet-skew to AST-detect the `DefaultRoutes()`-field-as-path pattern (f1 automation instead of one-off grep).
32. Issue-comment template: note push state so closed issues never dangle on unpushed commits (e5).
33. Repro-draft convention: `docs/drafts/<issue>/FIXED.md` marker or delete-at-fix (e6).
34. Double-check `ExampleVersionHandler` doc comment still matches the now-recommended wiring (it shows the manual form; add "or Routes.Version").
35. Confirm gates intentionally tolerate `docs/drafts` 0% coverage — add a one-line comment to the coverage-gate script so nobody "fixes" it.
36. Sanity-sweep remaining `DefaultRoutes()` references: only registration + examples should remain (grep found ~20; all register-or-docs — do a final read-through).
37. Check the two closed issues' `MANUALLY REVIEWED` boxes get ticked by the owner (owner ledger, not mine).
38. Post-release: verify the CHANGELOG compare-link for v0.10.0 exists before tagging (docs-check catches it, but the pre-tag app does it earlier).
39. Consider a `Routes` doc example block (the struct comment is prose-only today).
40. Evaluate whether `RegisterRoutes` should report conflicts instead of panicking (ServeMux panics by design — likely reject via ADR; low priority).
41. Naming audit of the new field: `Routes.Version` (mount pattern) vs e.g. `VersionRoute` — considered, kept `Version` for symmetry with siblings; record the alternative in the design note if one gets written (f21).
42. Re-check `DefaultRoutes` doc comment: it says "method-qualified" but doesn't name the Go 1.22+ mux requirement — the `Routes` comment does; align both.
43. README "What you get" bullet order: method-qualified defaults bullet landed next to partial-mounting — consider grouping both under one "routing" pair.
44. Add the aggregate/federation `Routes.Version` semantics to docs/aggregate-healthz-design.md if it discusses RegisterRoutes (verify whether it does).
45. Confirm `example_lifecycle_test.go`'s "routes registered: 3" golden output is the intended guard for "defaults stay version-free" (it is — but pin the intent in a comment).
46. Consider asserting `Allow` header value (not just status) in the version-mount 405 test.
47. Sweep for stray `health.DefaultRoutes().` field reads added by future sessions — cheap CI grep candidate (aligns with f31).
48. go-modernize pass over the touched test files (new lints may have appeared; gates were green at `nix fmt` time — rerun after next go bump).
49. Keep the docs/status convention: this report's (f) must not get entombed — HARVEST is the close-out (duplicate of 7 at the process level; keep one).
50. After v0.10.0: re-run the fleet dashboard adoption timeline against the released tag (the established discipline from docs/consumer-verification.md).

## g) Questions I cannot figure out myself

1. **`DefaultRoutes().Version`**: stay empty (my choice — opt-in avoids double-registration panics for consumers who hand-wire `/version` today), or should v0.10.0 default it to `"GET /version"` now that the pattern is method-qualified anyway?
2. **Aggregate/federation version mounts**: is `Routes.Version` on those surfaces answering `{"version":""}` acceptable, or should a _non-empty_ `Version` field on a surface with no build identity be rejected loudly at construction (fail-fast) instead of serving an empty stamp?
3. **Release timing**: cut v0.10.0 now to ship the behavior change behind a version boundary, or batch it with more `[Unreleased]` work first? (Affects when fleet consumers must do the f1 scan.)
