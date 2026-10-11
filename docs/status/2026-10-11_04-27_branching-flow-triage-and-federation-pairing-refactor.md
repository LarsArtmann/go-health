# Status Report — Branching-Flow Findings Triage & Federation Pairing Refactor

**Date:** 2026-10-11 04:27 CEST
**Repo:** `github.com/larsartmann/go-health` @ `master` (`a104301` + 2 uncommitted doc edits)
**Session scope:** Triage of the 8 `branching-flow all . --format finding` results (v0.6.4), execution of the actionable ones, gate verification. No other research performed, per instruction.

---

## Headline numbers

| Metric                  | Value                                                                                                            |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------- |
| Findings before session | 8 (1 correctness warning, 6 info, 1 structure warning)                                                           |
| Findings after session  | 7 (all documented accepted / owner-reserved classes)                                                             |
| Code changes            | 1 file (`federation/federation.go`, +17/−10), behavior-identical                                                 |
| Policy/doc changes      | `.buildflow.yml` (budget rationale rewrite, 14→9), `AGENTS.md` (budget line)                                     |
| Verification            | build ✅ vet ✅ test-race ✅ lint 0 issues ✅ `.#gates` exit 0 ✅ `nix flake check` ✅ branching-flow recount ✅ |

---

## a) FULLY DONE

1. **INDEX_OUT_OF_RANGE eliminated structurally** (the only code-actionable finding, `federation.go:280`). Verified first: `merge` has exactly one call site, no test constructs its input, so there was no live panic path. But the `p.states`/`results` parallel-array pairing was an _incidental_ invariant (enforced only by convention). Fix: new unexported `fetchedRemote{state *remoteState; result fetchResult}` pairs each remote with its fetch outcome in one slice; `cachedResponse` fills it under `wg.Go`, `merge` iterates pairs. No cross-slice index exists anymore — the invariant became unrepresentable rather than accepted. The `Prober.states` doc comment (which claimed "index i is the single pairing key") was updated to match.
2. **Triage of all 8 findings with evidence, not vibes.** Every accepted finding was re-verified against its recorded rationale before accepting: `.buildflow.yml` budget block (2026-10-09) covers mixins/large-struct/slice-index/strong-ID; `AGENTS.md` Key Design Decisions covers the construction-only-config idiom; `docs/servicename-design.md` proves strong-ID is an owner-reserved v0.6-window decision and that branded-type coupling (the suggestion's exact shape) was already REJECTED as Option C for `ServiceName`.
3. **Budget policy re-review (the "don't let them ratchet" duty).** `.buildflow.yml` branching-flow ceiling lowered `14 → 9` (7 stable documented findings + ~20% headroom per the block's own policy); stale "two remaining slice-index warnings" rationale replaced with the elimination record. `AGENTS.md` budget parenthetical synced. Committed by the daemon / pending in working tree respectively.
4. **Full verification sweep with explicit exit codes** (per the "gate output tails lie" gotcha): `nix run .#build`, `.#vet`, `.#test-race` (race detector exercises the concurrent index writes), `.#lint` (0 issues; only the documented `nolint_filter` noise), `nix run .#gates` → `GATES_EXIT=0` (fail-fast sweep incl. fuzz-short, gosec, govulncheck, docs-drift, openapi-lockstep), `nix flake check` → green (treefmt included).
5. **Post-hoc gap closure on my own claims:** `docs/federation-design.md:46` checked — its "one fetch per remote, in parallel, and merges" description remains accurate (no doc drift from the refactor). `aggregate/aggregate.go` checked — no equivalent parallel-slice/index-write pattern exists there.
6. **This report.**

## b) PARTIALLY DONE

1. **STRONG_ID findings (4×): analyzed, deliberately NOT implemented.** The repo reserves wire-shape identity decisions to the owner (`servicename-design.md` is DESIGN-ONLY, staged for the v0.6 breaking window). What exists: a complete recommendation (see section e.3) grounded in the recorded Option-C rejection, the single-dependency policy, the golden wire format, and one now-source-verified detail: `SanitizeResponse` coerces `resp.InstanceID` with `strings.ToValidUTF8` (handlers.go:212), which a branded `id.ID` field would complicate. What's missing: the owner's go/no-go, and source-verification of the `go-branded-id` claims themselves (see e.4).
2. **Second quality surface (BuildFlow) not re-run this session.** `nix run .#gates` is canonical and green; `buildflow --build-mode full` was not executed (known: needs `GOTOOLCHAIN=auto` in the launching shell, 4 straggler steps are an upstream task). The repo's own rule says full mode is a _surface_, never the only gate — nothing blocks, but the second surface hasn't seen the refactor.
3. **Federation merge test depth.** The refactor is covered through the public `CachedResponse()` path (14 federation tests + merge-table + fuzz + race). No direct unit test feeds hand-built result slices into `merge` — defensible (unexported, single caller, structural pairing now makes malformed inputs impossible), but it's a seam with no direct pin.

## c) NOT STARTED (all pre-existing, noticed in passing)

1. `ServiceName` typed identity at the v0.6 window (design + fleet inventory + scanner all ready in `docs/servicename-design.md`; execution untouched).
2. HARVEST of this report's section (f) into `TODO_LIST.md` / `ROADMAP.md`.
3. golangci LSP integration fix/disable (standing TODO, AGENTS.md).
4. BuildFlow full-mode fan-out env fix (upstream task; 4 known stragglers).
5. `gosecGo` nix pin removal once nixpkgs ships gosec reading export-data v5 (securego/gosec #1772).
6. `WithGETOnly` removal decision (deprecated since v0.1.1, still functional by design).
7. v0.6-window fleet verification sweeps from the servicename doc (Zlota44/dnsblockd const-decl checks, KeyHolderAI literal-vs-typed check, PMA scan on next contact, adoption-matrix over-count corrections for go-daemon rows).

## d) TOTALLY FUCKED UP

**Nothing at fuckup severity.** All gates green, no regressions, no data loss, working tree clean except the two intended doc edits.

Nearest miss worth recording: the _class_ of bug the flagged line hinted at (an index-alignment invariant held only by convention) shipped in v0.6.0/v0.6.1 as accepted "safe by construction" — it was never reachable, but it was also never _enforced_. It is now structurally impossible, which is the better resting state.

## e) WHAT WE SHOULD IMPROVE (includes: what I forgot, what I could have done better)

1. **I asserted before I verified — once.** My STRONG_ID analysis cited the `SanitizeResponse` UTF-8 interplay as a "consideration" from memory of the AGENTS.md file map; I only source-verified it (handlers.go:212) after the main work, while writing this report. `verify-external-claims` applies to my own intermediate claims, not just filed artifacts. Fix: make source-verification part of the triage step, not the report step.
2. **No CHANGELOG entry for the refactor.** It's internal (no API/wire change) so `docs-check` stayed green, but an "Unreleased → Internal" line would keep the release-cutting habit honest. Forgotten.
3. **No before/after benchmark.** `fetchedRemote` grows each element by one pointer (8 bytes/remote, one slice, read once per merge). Provably negligible — but "provably" here means "obviously", not "measured". A `CachedResponse` federation benchmark existed for the root package's handlers, not for federation merge throughput.
4. **The STRONG_ID recommendation itself (for the owner):** if identity typing happens at v0.6, the consistent shape is a _defined string type_ (`type ServiceName string`, per the design doc's Option A) — possibly `type InstanceID string` alongside — NOT `go-branded-id`: it would be a second dependency in a single-dependency library, its zero value marshals as `null` (vs today's omitted `""` under `instance_id,omitzero` — golden-wire risk), and `SanitizeResponse`'s `ToValidUTF8` would need a conversion. Additionally the tool's own suggestion (`WithInstanceID(id WithInstanceID)` — a type named after an option) is name-mangled nonsense, which further reduces trust in this suggestion set.
5. **Budget ceilings are judgment calls made autonomously.** 14→9 restores the documented ~20%-headroom policy and keeps the noise tripwire meaningful — but it's a policy change an owner should ratify (question 2 below).
6. **Accepted findings re-report every run.** The 7 survivors are healthy _as an inventory_ (they force re-reading the rationale), but if the noise annoys, a branching-flow suppression config is the alternative — owner preference (question 3 below).
7. **Session hygiene:** the auto-commit daemon captured the code change (`a104301`) before the doc edits landed — fine here, but the split means `git log` shows the refactor without the budget-context edits in the same commit. For deliberate work, committing atomically beats racing the daemon.

## f) NEXT TASKS (up to 50; tiers by impact — most are ROADMAP fuel, not commitments)

**Tier 1 — direct follow-ups from this session (high impact, low effort)**

1. Owner: decide the v0.6 typed-identity question (ServiceName / InstanceID / neither) — blocks items 2–5.
2. Record the STRONG_ID deferral-or-adoption decision as an addendum in `docs/servicename-design.md` so the next session doesn't re-litigate it.
3. Add "Unreleased → Internal: federation merge pairs state+result (fetchedRemote), no parallel-slice invariant" to `CHANGELOG.md`.
4. Run HARVEST (`docs-health`) to route this report's section (f) into `TODO_LIST.md` / `ROADMAP.md`.
5. Commit the two pending doc edits (`.buildflow.yml`, `AGENTS.md`) atomically, or verify the daemon did — keep log coherent.
6. Add a federation `CachedResponse`/merge benchmark; record the pre-refactor number for posterity if the old binary is still buildable at `a104301^`.
7. Source-verify the `go-branded-id` v0.5.1 claims (zero-marshals-as-null, omitzero×IsZero interplay) _if and only if_ question 1 goes anywhere — the analysis currently leans on the skill reference, not the library source.

**Tier 2 — v0.6 breaking window (high impact, planned)**

8. Execute `docs/servicename-design.md` Option A: introduce `type ServiceName string`, change the three signatures, ship the window.
9. Run the per-repo fleet verification plan (11 repos listed in the doc) after the rewrite.
10. Settle the two const-decl verifications (Zlota44 `checkSQLite`, dnsblockd `healthService*`).
11. Verify KeyHolderAI multi-line names literal-vs-typed.
12. Scan PMA on next contact (no local checkout).
13. Correct the adoption-matrix over-counts (go-daemon / project-discovery-daemon rows marked "NOT consumers").
14. Decide `WithGETOnly` removal (deprecated v0.1.1; AGENTS.md says keep tests until decided).
15. Re-baseline the branching-flow budget after v0.6 typing lands (finding mix will change).
16. Cut v0.6.0: CHANGELOG cut, gates, tag, proxy/pkg.go.dev verification (`go-release` skill flow).

**Tier 3 — quality surfaces & tooling (medium impact)**

17. Fix or disable the golangci LSP integration (standing TODO; it lied twice before).
18. Run `buildflow --build-mode full` (with exported `GOTOOLCHAIN=auto`) once over the refactor as the second surface.
19. Chase BuildFlow upstream on full-mode fan-out env loss (4 straggler steps).
20. Drop `tools/bin` wrappers + `tool_paths` block once the graph stops needing a newer patch than BuildFlow's toolchain (conditional on 19).
21. Drop the `gosecGo` overrideAttrs pin when nixpkgs ships gosec with export-data v5 support (#1772).
22. Consider a branching-flow suppression config for the 7 accepted findings (vs. living-inventory status quo — question 3).
23. File the branching-flow STRONG_ID name-mangling bug (`WithInstanceID` suggested as a _type_ named after the option) upstream in the branching-flow repo — it's the user's own tool; one verified reproduction is in this session's output.
24. Re-check `go.mod`'s `go 1.27` directive vs tidy auto-bump drift next dependency touch (standing gotcha).
25. Confirm `.#fuzz-long` (5 min/target) actually runs weekly in CI as documented; wire it if not.
26. Re-run `nix run .#test-race -count=2` style stress on federation once as cheap extra assurance for the concurrent index writes (race ran once in gates).

**Tier 4 — docs & hygiene (lower impact)**

27. Consolidate the "~20% headroom" budget policy into one sentence in `.buildflow.yml`'s block intro (it's currently implied by arithmetic).
28. Add a one-line pointer in `federation-design.md`'s merge section to the `fetchedRemote` pairing (the doc is accurate but silent on the mechanism).
29. Fold the "parallel-slice → structural pairing" lesson into `references/lessons.md` (crush-config, by commit) only if it generalizes beyond this repo — it probably does (it's the zip-vs-parallel-arrays classic).
30. Sweep `docs/INDEX.md` after items 2/28 so the per-doc index stays truthful (per the 220-line AGENTS.md cap rule).
31. Annotate this report as superseded when items 1–7 close (`docs-health` ANNOTATE mode, non-destructive).

**Tier 5 — candidate/roadmap ideas (uncosted)**

32. Federation: per-remote fetch timeout already exists; consider documenting worst-case fan-out latency in the design doc (slowest-remote bound).
33. Aggregate vs federation: a shared merge-core extraction was consciously avoided (different trust models) — re-derive that decision in one paragraph in `merge-unification-design.md` if it isn't already there.
34. Root probe: consider whether `Response.Timestamp`-based throttle freshness deserves a fuzz corpus entry (throttle fuzz exists; freshness-boundary case may be underfuzzed).
35. `checks` package: batteries coverage vs. the four shipped constructors — a periodic "which battery next" review (ownership doc exists).
36. Dashboard contract: `Prober` structural assertion is pinned for federation; confirm aggregate satisfies the identical assertion test (likely; verify).
37. OpenAPI: consider adding `instance_id` semantic description (currently shape-locked; semantics live in godoc only).
38. Error-message audit: `ErrInvalidRemote` causes are human-readable; spot-check they never leak remote URL credentials (URLs are validated http(s), query strings pass through — decide if that's acceptable).
39. CI: gate on `nix run .#ci-emulation` in addition to `.#gates` (exists, green locally, unclear if scheduled).
40. Release hygiene: after v0.6.0, re-run the full gates on the tag itself (the v0.6.0 lesson: tag CI was red with zero code changes due to toolchain drift).

_(Stopping at 40 — items 41–50 would be padding; the honest count of real, articulable next tasks is 40.)_

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF (3)

1. **Typed identity at v0.6:** do you want `type ServiceName string` (per the design doc's recommendation), and if so should `type InstanceID string` join it in the same breaking window — or do we permanently reject typing instance identity and record that? This is the exact decision the repo reserves to you; everything downstream (fleet migration, budget re-baseline, my STRONG_ID stance) keys off it.
2. **Budget ceiling:** ratify `branching-flow: 14 → 9` (restores the documented ~20% headroom over the now-7 stable findings, tripwire fires on +2 noise) — or do you prefer keeping 14 as slack for the v0.6 churn?
3. **Accepted-findings visibility:** should the 7 documented-accepted branching-flow findings stay re-reported on every run (living inventory that forces rationale re-reads — today's state), or do you want them suppressed in tool config so only _new_ classes surface?

---

_Point-in-time snapshot. Section (f) is HARVEST fuel for `TODO_LIST.md`/`ROADMAP.md` (not yet harvested — see c.2). Waiting for instructions._

---

## Addendum (05:45, follow-up session)

b2 is closed: `buildflow --build-mode full` re-run with `GOTOOLCHAIN=auto` exported → **EXIT=0, "passed with warnings", 79 success / 0 failed** (test-race ✔ 2.1s, test-coverage ✔ 0.8s) — the federation refactor has now seen the second quality surface. The run also cleared the two gate items from the 01:38 failed sweep: `SECURITY.md` gained a Security Practices section + a "Response time" heading (securitymd now 0 findings), and the `test-coverage` step failure was diagnosed as transient-environmental (one instance of a 19-repo fleet window around the 23:15 reboot; exact-seed replay plus 12/12 random-seed replays green; full mode EXIT=0 after) — the head+tail output-truncation post-mortem recipe is recorded in AGENTS.md Gotchas. Remaining warnings are the documented within-budget classes (govulncheck version-skew, nix-checker info, file-size, art-dupl 63/80, branching-flow 8/9, go-auto-upgrade 7/8).
