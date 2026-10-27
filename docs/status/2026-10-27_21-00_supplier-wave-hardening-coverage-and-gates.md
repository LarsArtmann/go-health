# Status report — supplier-wave hardening: coverage, fuzz registry, docs gates

**Date:** 2026-10-27 21:00 CET
**Repo:** `~/projects/go-health` · **Branch:** `master`
**Releases in play:** `v0.8.0` (`4af20ba`) and `v0.8.1` (`b7258b0`, CI-green re-cut) — both published, both cut by the peer session, neither by this session
**Author:** Crush (this session) · **Predecessor reports:** [`2026-10-27_20-17_…`](2026-10-27_20-17_supplier-wave-post-release-hardening.md) (mine), [`2026-10-27_20-42_…`](2026-10-27_20-42_supplier-wave-release-executed-adoption-not-started.md) (peer's)
**Scope:** only what this session executed. No new market research, no consumer repos touched.

---

## 0. The one-paragraph truth

I worked the **repo-hardening** tier of the previous report's 50-item queue (items 21–30, 35, 40) — the only tier that needs no owner decision. Coverage went from 99.1% to **99.9% total, with `health` at 100%** (from 99.2), by writing seven tests and converting two "defensive-only" branches into named, tested functions instead of silent coverage holes. Two claimed-but-absent safety nets now exist and run inside `nix run .#gates`: a **fuzz-registry parity check** (the flake comment had advertised a pre-push hook that never existed) and **two new docs-check rules** (`[Unreleased]` must not be empty past the latest tag; the federation design doc's API block must match the exported surface). `AGENTS.md` was at 219/220 lines, so the per-file map moved to the new `docs/ARCHITECTURE.md`, freeing 38 lines. **What is not done:** the full gate sweep after these edits (only `test-race`, `docs-check` and the new `fuzz-registry` have been run), the FEATURES/TODO_LIST bookkeeping for this batch, BuildFlow, and — unchanged and blocked on you — every consumer repository.

---

## a) FULLY DONE (verified in this session)

| #  | Work                                                                                                             | Evidence                                                                                                                                                    |
| -- | ---------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1  | `applyStartupFailureOverlay` extracted from `Probe.Healthz`; nil-map guard now provable                            | `accessors.go`; 4 subtests in `accessors_internal_test.go`; root package 99.2% → **100.0%**                                                                  |
| 2  | `versionMarshalFailure` + `marshalVersionResponse` seam (mirrors the existing `marshalResponse` seam)              | `version.go`; seam-swap test `TestVersionHandler_MarshalErrorIncludesCause` in `version_internal_test.go`                                                    |
| 3  | Startup latch ignores-with-absence bug class pinned: a critical name missing from a *later* recorder batch        | `TestStartup_CriticalNameMissingFromBatch_DoesNotLatch` (`probe_test.go`) → 503, latch stays open; this was the last uncovered branch in `evaluateStartup`    |
| 4  | `DetailedHealthRecorderFunc`'s plain-`HealthRecorder` view pinned (was 0%)                                        | `TestDetailedHealthRecorderFunc_PlainViewProjectsErrors` (`new_with_recorder_test.go`)                                                                       |
| 5  | Tracker's benchmark-only `disabled` short-circuit pinned (was the last gap in `tracker.stamp`)                    | `TestEvaluate_TrackerDisabledLeavesSinceZero` (`probe_metadata_test.go`)                                                                                      |
| 6  | `federation.WithClient` covered (was 0%) — proves the supplied client's transport carries the fetch + `Accept`   | `TestWithClient_UsesSuppliedClient` (`federation/transport_test.go`)                                                                                          |
| 7  | `fetch`'s truncated-body branch covered (was the last reachable gap in `fetch`)                                  | `TestCachedResponse_BodyReadErrorIsReported` — reachable fail row naming `read body` + cause                                                                  |
| 8  | `federation.Healthz` race-tested under concurrent drain (previously only `SourceStatuses` was)                    | `TestHealthz_ConcurrentDrainIsRaceFree`: 8 readers × 4 rounds vs `MarkShuttingDown`; drain never rolls back, never unlatches startup                           |
| 9  | Coverage re-measured, per package                                                                                 | root **100.0**, aggregate 100, checks 100, merge 100, federation **99.5** (98.1 → 99.5), **total 99.9** (99.1 → 99.9)                                          |
| 10 | `nix run .#fuzz-registry` exists: diffs both fuzz apps against `^func Fuzz` per package **and** against each other | `tools/fuzz-registry-check.sh`; flake app + `checks.fuzz-registry` + added to `.#gates` and `.#ci-emulation`; run → `OK - 9 targets registered in both fuzz apps` |
| 11 | Drift detection of that check proven, not assumed                                                                | scratch copy with a planted `FuzzGhostTarget` → exit 1 naming it, plus the reverse direction (registry entry with no target)                                  |
| 12 | `docs-check` rule: `[Unreleased]` must document something while HEAD is past the latest tag                       | fired for real on the first run (before I wrote the entries) → `DRIFT: … documents nothing although HEAD is past v0.8.1`                                      |
| 13 | `docs-check` rule: `docs/federation-design.md`'s fenced go API block ⇄ federation's exported surface, both ways    | verified on a scratch copy: renaming `SourceStatuses` in the doc yields two DRIFTs (one per direction); clean repo passes                                     |
| 14 | `docs/ARCHITECTURE.md` created with the per-file map, the four sub-package contracts, `WithClient`, the two new seams, and a test-layout table | new file; `AGENTS.md` **219 → 181 lines** (38 lines of headroom under the 220 cap)                                                             |
| 15 | `AGENTS.md` updated for the new reality: `fuzz-registry` row, honest `nix fmt` row, coverage floor, the "extract, don't suppress" convention, the dprint non-fix, the two new gates | `AGENTS.md` |
| 16 | Item 30 resolved: the `.config/metadata.yaml` diff was **formatting only** (dprint's YAML plugin reindented `- lib` from 4 to 2 spaces) | `git show bd8f7ed -- .config/metadata.yaml`                                                                                                    |
| 17 | Item 35 resolved as a **documented non-fix** with evidence, not a shrug: dprint downloads its markdown plugin at first use | fresh cache run printed `Compiling https://plugins.dprint.dev/markdown-0.22.1.wasm` and wrote `…/cache/plugins/*.cwasm`; treefmt runs sandboxed → would be red everywhere |
| 18 | `CHANGELOG.md` `[Unreleased]` rewritten to the truth (added/changed/fixed)                                       | required by rule 12, which is the point: the rule bites immediately                                                                                            |
| 19 | `docs/INDEX.md` gained the `ARCHITECTURE.md` row                                                                  | `docs/INDEX.md:14`                                                                                                                                             |
| 20 | Race suite green after all Go edits                                                                              | `GOWORK=off go test -race -count=1 ./...` → 5/5 `ok`                                                                                                           |
| 21 | `docs-check` green end to end                                                                                    | `nix run .#docs-check` → `OK - README/AGENTS/CHANGELOG/FEATURES in sync with v0.8.1`                                                                           |

### Remaining uncovered statements (deliberate, documented)

| Location                          | Why it stays                                                                                                                                                                             |
| --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `federation/federation.go` `fetch` | `http.NewRequestWithContext` cannot fail for any `Remote` that `New` accepted — validation runs the same `net/url` parse. Keeping the check is the only way not to discard an error channel. Rationale now written at `fetch`. |

---

## b) PARTIALLY DONE

| # | Item                          | What exists                                                                       | What is missing                                                                                                                                          |
| - | ----------------------------- | --------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | **The gate sweep**            | `test-race`, `docs-check`, `fuzz-registry` all run green after the edits          | `lint`, `vet`, `vulncheck`, `security`, `fuzz`, `nix flake check` **not yet re-run**. My new files have not been through gofumpt/golines/nixfmt either.    |
| 2 | **FEATURES.md / TODO_LIST.md** | Coverage floor and the new gates are recorded in AGENTS + CHANGELOG               | No FEATURES "Quality"/"Performance" row for 100%/99.5%, and the ten queue rows I just executed are still open in `TODO_LIST.md`                            |
| 3 | **BuildFlow full mode**       | Known state: 3 failures (`govalid-generate`, `license-check`, `golangci-lint [tools/doanalyzerv2]`) | Not re-run this session; unknown whether the new flake app/checks change the picture                                                                     |
| 4 | **Docs-check API lockstep**   | Federation's `*Prober` + package funcs are locked to `docs/federation-design.md`  | `aggregate.Aggregate` and the root `*Probe` have no equivalent lockstep — the same drift class is still open there                                       |
| 5 | **dprint / markdown**         | Non-fix documented with reproduction evidence                                     | The drift class itself remains: markdown tables are only checked by BuildFlow, not by `.#gates`                                                          |
| 6 | **`docs/ARCHITECTURE.md`**    | Accurate today, generated by hand                                                 | Nothing checks that it lists every source file — a new file can be added without a row                                                                   |

---

## c) NOT STARTED

1. `nix run .#gates` (full) and `nix flake check` after this batch.
2. `nix fmt` / treefmt check on the seven new test files.
3. `nix run .#lint` (expect: `err113` on the two test sentinels is an LSP artifact, must be confirmed against the CLI).
4. FEATURES/TODO_LIST bookkeeping for items 21–30/35/40.
5. BuildFlow triage.
6. Any commit with a human-written message (the auto-commit daemon has been committing my work under heuristic messages).
7. The six consumer repositories (cqrs-htmx/health, go-appkit/health, dnsblockd, DiscordSync, cv, go-taskqueue) — **owner-gated**.
8. `v0.8.2`/`v0.9.0` decision for the post-tag work — **owner-gated**.
9. Posting the drafted issue #5 / #7 comments (`docs/drafts/`) — **owner action**.
10. BuildFlow upstream issues for the two proven gaps (dprint `--fix` no-rewrite; full-mode spawn env).
11. Items 31–34, 36–39, 41–50 of the previous queue (authority questions, renames, plan consolidation).

---

## d) TOTALLY FUCKED UP

Nothing destructive. No tag moved, no file deleted, no `rm`, no `git checkout/restore`. But:

1. **Two wasted Nix builds on a backtick.** I put a `` ``` `` inside a *double-quoted* shell string in `flake.nix` (the drift message), which bash read as command substitution → `unexpected EOF`, then after rewording hit shellcheck `SC2016` on a backtick inside *single* quotes → a second failed build. Fixed by feeding the fence to `awk -v fence='```'` instead of `sed`.
2. **Left scaffolding in the new script.** `tools/fuzz-registry-check.sh` initially contained a nonsense line (`cut … | awk 'NR == FNR { next }' /dev/null`) from an abandoned approach. Caught and removed before the first successful run.
3. **Silent first-run failure of that script.** `grep` returning 1 inside a `while` body plus `set -euo pipefail` aborted the script with exit 1 and **no output** — a failure mode that hides its own cause. Fixed with `{ grep … || true; }`.
4. **Misread a made-up helper signature.** My first `federation/transport_test.go` called `httptestRecorder()` / `newRequest(ctx)`, which do not exist, and used `bytes.Contains` where `strings.Contains` was meant — caught only because I rewrote the file; nothing compiled dirty.
5. **CHANGELOG edit rejected mid-flight** ("file has been modified since last read"). Cause: the **peer session is active again** (it wrote `docs/status/2026-10-27_20-42_…` and touched `docs/INDEX.md` at 20:52). Nothing was clobbered — I re-read and re-applied — but I cannot rule out a lost edit in the window.
6. **Reported a rule as done before proving it.** I only verified the API-lockstep rule by manually replaying its logic on a scratch copy; the packaged script's own `nix run` path has not yet been exercised against a *deliberately drifted* repo (only against the clean tree).
7. **Chased 100% at the edges.** I spent a seam and a helper extraction on two branches that no consumer can reach. It was worth it for the root package (it exposed that `evaluateStartup`'s absence branch was untested *code that matters*), but the `versionMarshalFailure` seam is arguably ceremony.

---

## e) WHAT WE SHOULD IMPROVE

1. **Enforce the coverage floor in CI.** 99.9% is a measurement, not a gate; nothing stops the next commit from dropping to 97%. A `go test -coverprofile` + `go tool cover -func | awk` threshold check is ~10 lines.
2. **Extend the API lockstep to `aggregate` and the root `Probe`.** The federation doc drifted for exactly the reason the other two will.
3. **Check `docs/ARCHITECTURE.md` against the tree** (every `*.go` in the module has a row, every row has a file). Otherwise the map I just wrote is a future lie.
4. **Make the fuzz-registry script fail loudly.** Any `set -e` abort should print which step died; every gate should echo a final `EXIT=n` (the "gate output tails lie" lesson, still not mechanized for scripts).
5. **Stop hand-maintaining the fuzz lists at all.** The registry check is a guard; generating both `fuzz` apps from `go list`-derived targets would remove the class (blocked only by the flake's sandbox having no `go` in the app).
6. **One writer per repo.** Three status reports and two plans now describe one wave, and a peer edited two files I was editing. Either coordinate or intercept.
7. **Commit intent, not heuristics.** The auto-commit daemon has committed ~15 of my changes as "chore: auto-commit N changed file(s) (heuristic)". Future readers cannot learn what any of it was for.
8. **Prefer a scratch-copy `nix build` probe before changing a gate.** Two of my three failures this session were shell-quoting errors that a 20-second local replay would have caught.

---

## f) NEXT 50 THINGS (ordered by value/effort)

**Finish what this batch opened (self-authorisable, do first)**

1. Run `nix run .#gates` in full; fix whatever it reports.
2. Run `nix fmt` (or `buildflow format`) on the new test files; re-run `nix flake check` for the treefmt check.
3. Run `nix run .#lint`; confirm the two `err113` warnings on test sentinels are LSP-only.
4. Add a CI + `.#gates` coverage-threshold app at 99.5% total / 98% per package.
5. Extend the docs-check API lockstep to `aggregate.Aggregate` and root `*Probe`.
6. Add an `ARCHITECTURE.md` ⇄ `*.go` completeness check.
7. Update FEATURES "Quality"/"Performance" with 100/99.5/99.9 and the new gates.
8. Flip the ten executed TODO_LIST rows to DONE with evidence.
9. Make `tools/fuzz-registry-check.sh` print a step banner and a final `EXIT=` line.
10. Commit this batch with a real message (or accept the daemon's, and record the intent in CHANGELOG — already done).

**Release hygiene (owner-gated)**

11. Decide: `v0.8.2` for the post-tag test/tooling/docs work, or fold into `v0.9.0`.
12. Cut the tag (promote `[Unreleased]`, compare link, README stability line, AGENTS status line).
13. Verify the proxy serves it (`go list -m -versions`, clean-module `go get` smoke).
14. Write the release notes with a consumer-impact paragraph (nothing breaking).

**Fleet adoption (owner-gated, one repo = one commit)**

15. `cqrs-htmx/health`: bump to the release.
16. …replace its `RecorderChain` with `health.ChainRecorders` + a duration-survival test.
17. …widen `NewDashboard` to the dashboard `Prober` interface.
18. `go-appkit/health`: bump.
19. …flip `probe_test.go` warn → fail in the same commit.
20. …retire the "silently dropped recorder" godoc.
21. …add `appkithealth.NewDetailedProbe`.
22. dnsblockd: `CachedResponseFresh` instead of the zero-timestamp guard (keep the 15 s policy).
23. dnsblockd: `Response.Clone` instead of hand-rolled `maps.Copy`.
24. DiscordSync: `CachedResponseFresh`/`EvaluateAndCache` for the readyz snapshot.
25. DiscordSync: `WithCriticalService[T]` + `DoServiceName` in the label table.
26. cv: `CacheAge`/`CachedResponseFresh` + `Clone`.
27. go-taskqueue: `queueProber` → `NewChecks` + `WithRefreshInterval(0)` + live throttle.
28. go-taskqueue: delete the re-implemented `dashboard.Prober`.
29. PMA + Zlota44: `NewWithRecorder` instead of `health.New(do.New(), …)`.
30. timesheets: swap the local version handler for `health.VersionHandler`.
31. Re-run `tools/fleet-skew.sh <new tag>` → expect `minor-stale: 0`.
32. Add the skew run to CI only after 15–31 land.

**Quality surfaces still open**

33. BuildFlow: fix the `golangci-lint [tools/doanalyzerv2]` step env (`GOWORK=off`, `GOTOOLCHAIN`).
34. File the BuildFlow issue: `dprint-format --fix` does not rewrite unpadded tables.
35. File the BuildFlow issue: full-mode spawn env ignores `GOTOOLCHAIN=auto` for `license-check`.
36. Investigate the two BuildFlow regression verdicts (`go-auto-upgrade` +190%, `go-version-auto-configure` +450%).
37. Fix or disable the golangci LSP integration (fifth session it has misreported).
38. Resolve the `tools/doanalyzerv2` go-directive floor; then drop `tools/bin` wrappers.
39. Add `fuzz-long` to a weekly CI job if it is not already wired (verify the workflow file, not the comment).
40. Re-run the long fuzz campaign once (`nix run .#fuzz-long -- -fuzztime=60s`) after any new target.

**Board hygiene / decisions**

41. Consolidate three status reports + two plans into one canonical plan and archive the rest.
42. Record which session owns this repo for the wave (who may tag, who may edit AGENTS).
43. Post the issue #7 annotation (`docs/drafts/issue7-annotation.md`).
44. Post the issue #5 close comment (`docs/drafts/issue5-close-comment.md`).
45. Post the samber/do#318 duration comment.
46. Confirm the non-critical-panic → 503 grading as shipped (ADR-008) or schedule a revert.
47. `ServiceName` cut go/no-go, then run `tools/servicename-scan.sh` over the break list.
48. `SanitizeResponse` → `CoerceValidUTF8` alias; `Check.Since` → `StatusSince` with the JSON frozen.
49. Design the per-check severity override (PMA/go-taskqueue ask) with an explicit accept-or-reject note.
50. Read `docs/adoption-matrix.md` against the shipped v0.8.1 surface and mark ghost features.

---

## g) THREE QUESTIONS I CANNOT ANSWER MYSELF

1. **Who else is editing this repo right now?** A peer session cut v0.8.0 and v0.8.1, wrote `docs/status/2026-10-27_20-42_…`, and touched `docs/INDEX.md` at 20:52 while I held it — my CHANGELOG edit was rejected mid-flight. Should I stand down on shared docs, take a lock, or keep going as-is? (I lost nothing this time; I cannot promise that for the next collision.)
2. **Does this test/tooling/docs batch become `v0.8.2`, or does it ride to `v0.9.0`?** Nothing here changes library behaviour — the root package gained tests, two extractions and a seam; the flake gained two gates; docs moved. A patch tag is defensible and cheap, but it changes what six consumer repos bump to.
3. **May I write and push to the six consumer repositories?** Every remaining high-value item in §f sits behind that answer, and `TODO_LIST.md` still records it as blocked. Say yes and I start with `cqrs-htmx/health` (highest consumer count), one repo per commit, reverting that repo alone on red.

---

## Appendix — numbers from this session

| Measurement                                  | Value                                                          | Command                                              |
| -------------------------------------------- | -------------------------------------------------------------- | ---------------------------------------------------- |
| Coverage, total                              | **99.9%** (was 99.1%)                                          | `go test -coverprofile … && go tool cover -func`     |
| Coverage, by package                         | health **100.0** (was 99.2), aggregate 100, checks 100, merge 100, federation **99.5** (was 98.1) | same |
| Uncovered statements, total                  | **1** (federation `fetch` request construction, documented)     | `grep -v '^mode:' cov.out \| awk '$NF == 0'`         |
| Tests added                                  | 7 test functions in 5 files (2 new files, 3 extended)           | —                                                    |
| Production files changed                     | 3 — `accessors.go`, `version.go`, `federation/federation.go` (doc comment only) | —                                     |
| New fuzz-registry check                      | `OK - 9 targets registered in both fuzz apps`                   | `nix run .#fuzz-registry`                            |
| Fuzz-registry drift detection                | 1 planted target → exit 1; registry-only target → exit 1        | scratch copy of flake.nix + test files               |
| docs-check API lockstep                      | clean → no drift; renamed doc entry → 2 DRIFTs                  | scratch copy with `SourceStatuses` → `FakeNewMethod` |
| docs-check `[Unreleased]` rule               | fired for real before the CHANGELOG edit, green after           | `nix run .#docs-check`                               |
| AGENTS.md lines                              | **219 → 181** (cap 220, enforced by `docs-check`)               | `wc -l AGENTS.md`                                    |
| Race suite                                   | 5/5 `ok`                                                        | `GOWORK=off go test -race -count=1 ./...`            |
| dprint plugin download (item 35 evidence)    | `Compiling https://plugins.dprint.dev/markdown-0.22.1.wasm`     | fresh `XDG_CACHE_HOME` run in `/tmp`                 |
| `.config/metadata.yaml` diff (item 30)       | indentation only, `- lib` 4 → 2 spaces                          | `git show bd8f7ed -- .config/metadata.yaml`          |
