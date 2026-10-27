# Status — TODO-wave execution: decisions, audits, and a vacuous-red hunt

**Date:** 2026-10-27 23:42 CET
**Repo:** `github.com/larsartmann/go-health` · **Branch:** `master` (clean, in sync)
**Releases in play:** `v0.8.0` (`4af20ba`) and `v0.8.1` (`b7258b0`) — both published by the peer session earlier tonight
**Author:** Crush (this session) · **Predecessors:** [`21:00`](2026-10-27_21-00_supplier-wave-hardening-coverage-and-gates.md) (mine), [`21:02`](2026-10-27_21-02_distribution-shipped-consumption-zero.md) (peer's)
**Scope:** the repo-local half of the wave-2 plan
([`2026-10-27_21-12_…`](../planning/2026-10-27_21-12_wave2-adoption-closing-the-consumption-gap.md)) —
milestones M05, M10–M17, M20–M23, M25, M26. No consumer repo was modified, no tag was cut, nothing was pushed outside this repo.

---

## 0. The one-paragraph truth

The repo-local plan is **finished except for two gates that could not run on this host**: every decision record, audit, benchmark, matrix and doc refresh that the plan asked for exists and is verified, and six of the nine gates are green. What stopped the sweep twice was not code: **`/mnt/buildcache` is 100 % full (208 GB of 220 GB, 0 bytes free)**, which made gosec fall back to a source-only pass (exit 1 with `Issues: 0`) and made golangci-lint fail to persist facts. Both were reproduced and one is now fixed in the flake (private `GOCACHE` for `.#security`, verified green). A peer session executed five of the same milestones concurrently (M05/M14/M15/M16/M23); I verified those rather than duplicating them, and the verification is recorded below.

---

## a) FULLY DONE (verified this session)

| #   | Item                                                                                                                | Evidence                                                                                                                                                                                                                                        |
| --- | ------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| a1  | **Baseline gate sweep run and honestly read** (first attempt aborted at `lint`)                                      | `nix run .#gates` → test-race green, vet green, `lint` died on `failed to load packages: context deadline exceeded` at load avg >250 with ~15 concurrent golangci language servers                                                                 |
| a2  | **Real lint finding fixed**: `waitgroupgo` in `federation/transport_test.go` (`go func` + `wg.Add/Done` → `wg.Go`)     | `nix develop -c golangci-lint run --timeout=15m ./...` → `0 issues.`                                                                                                                                                                            |
| a3  | **M13 — two benchmarks that were claims, not facts**                                                                 | `BenchmarkCachedResponseFresh` (~75 ns, **0 B/op, 0 allocs/op**) and `BenchmarkEvaluateAndCache` (A/B: 1682 B/7 allocs vs 2066 B/12 allocs → publishing costs +384 B/+5 allocs, and the allocation is the batch-deadline context, not the store). FEATURES "Performance" carries all three new rows (incl. a3's sibling below). |
| a4  | **M11 — accessor read cost measured, decision recorded**                                                              | `BenchmarkMergedReadAndStatuses` (4 remotes): merged_only 34.2–34.8 kB/342–344 allocs, statuses_only 33.6–36.8 kB/341–348, **both 72.0–73.8 kB/692–696** → exactly two passes. [docs/accessor-read-cost-design.md](../accessor-read-cost-design.md) records four rejected alternatives and the revisit conditions. |
| a5  | **M10 — shutdown is one-way, decided not just documented**                                                            | [ADR-009](../adr/ADR-009-shutdown-is-one-way.md): four rejected alternatives (re-arm in `Start`, `Rearm()` API, tri-state enum, docs-only). Linked from `Probe.Start` and `Probe.Shutdown` godoc. Pinned by the pre-existing `TestStart_AfterShutdown_RestartsLoopButStaysDown` (no new test needed — it already asserted loop-re-arm + 503 + liveness 200). |
| a6  | **M12 — `WithStaleAfter` refused, formally**                                                                          | [ADR-010](../adr/ADR-010-freshness-is-a-primitive.md): the bound is per-consumer and derived, the degradation verdict is domain-specific, so freshness stays a three-line primitive. Linked from `CacheAge`'s godoc.                              |
| a7  | **M20 — recorder mis-wire audit built, proven, and run over the fleet**                                               | `tools/recorder-miswire-audit.sh` + `nix run .#recorder-audit`; awk state machine blanks comments/string literals before paren-matching (so doc comments that show the *wrong* wiring do not fire). Fixtures: clean → exit 0, two planted mis-wires → exit 1 naming both lines. Fleet: **16 repos, 8,507 Go files, 0 findings** ([docs/recorder-miswire-audit.md](../recorder-miswire-audit.md)). |
| a8  | **M21 — dashboard handoff drafts, with two plan rows proven stale**                                                   | [docs/drafts/dashboard-family-handoff.md](../drafts/dashboard-family-handoff.md): 2 ready-to-post drafts (dashboard#14 goroutine outlives `Shutdown`, hub SIGTERM → `MarkShuttingDown`) and 2 items verified **already done** (fir retired `datastarCSPBoot` itself; dashboard already runs 13 nightly fuzz targets). |
| a9  | **M17 — adoption matrix re-measured, not re-asserted**                                                                | [docs/adoption-matrix.md](../adoption-matrix.md) §"2026-10-27 post-v0.8 re-verification": `CachedResponseFresh`/`EvaluateAndCache`/`CacheAge` = **0 adopters** (grep, vendor-excluded); `ChainRecorders` = bridge re-export only; `SourceStatuses` = one deep adopter + a vendored copy; federation drain = 0.                                                  |
| a10 | **ROADMAP de-staled**                                                                                                 | Three false claims fixed: `SourceStatuses` "unreleased (v0.6 vehicle)" → shipped v0.6; `federation.Healthz` "implementation stays in this window" → shipped v0.8.0; `WithGETOnly` "removed … Unreleased" → v0.7.0. New "Current wave — consumption, not supply" section. |
| a11 | **M22 — TODO_LIST rebuilt to the real state**                                                                         | 131 → 97 lines: every completed row deleted per the file's own lifecycle rule; "blocked on the v0.8.0 cut" corrected to "blocked on the bridge release + write authority"; the two resolved dashboard asks dropped; the three decisions added as closed. |
| a12 | **M22 — cross-links + markdown formatting**                                                                            | Cross-links added between the 18:57 research doc and the 21:00/21:02 reports (dated "added 2026-10-27 22:xx" lines, history not rewritten). `dprint fmt` over 13 touched markdown files (11 reformatted). Relative-link check over all new/edited docs: **0 broken links**. |
| a13 | **M25 — BuildFlow doanalyzerv2 lint red: documented non-fix *with source-level proof***                               | `.buildflow.yml` now records that BuildFlow's config model has exactly one env knob (`OutputSettings.Env`, `model/config_substructs.go:156`, "for all tool invocations") and **no per-step env field anywhere** (`grep -rn 'yaml:"env"'` → 1 hit), so a step-scoped `GOTOOLCHAIN`/`GOWORK` override cannot be expressed; the global block already sets `GOTOOLCHAIN: auto` and still misses the failing spawn. |
| a14 | **M26 — root cause of the lying LSP identified, and the CLI made robust**                                             | The LSP failure and the CLI failure are the same signature (package loading under a stampede of ~15 language servers on a loaded host). `.golangci.yml` `run.timeout` 5m → 10m with the measured rationale; the CLI is documented as the arbiter. |
| a15 | **A *new* vacuous-red class found, fixed and verified**                                                               | `.#security` read the host's shared `GOCACHE` (`/mnt/buildcache/go-build`); concurrent writers left `reading srcfiles list: cache entry not found`, gosec skipped SSA for all five packages and exited 1 with `Issues: 0`. Fix: the app now runs gosec against a private `mktemp -d` cache. Verified twice: `GOCACHE=$(mktemp -d) nix run .#security` → **exit 0**, and after the flake change `nix run .#security` → **exit 0** with no SSA skips. AGENTS.md carries the gotcha. |
| a16 | **Peer-executed milestones verified, not trusted**                                                                    | M05 (`.#skew` app + `tools/fleet-skew.sh --expect/--allow` + weekly `skew.yml`), M14 (`.#coverage-gate` + `tools/coverage-gate.sh` + CI step + CONTRIBUTING), M15 (generalized `api_lockstep` for root/aggregate/federation with `go api:` fence tags), M16 (ARCHITECTURE completeness, bidirectional, `find`-based so an unstaged file cannot hide). All confirmed green via `nix run .#docs-check`. |
| a17 | **Coverage floor gate green on the real tree**                                                                        | `.#coverage-gate`: root/aggregate/checks/merge **100.0 %**, federation **99.3 %**, total **99.9 %** (floor 99.5 % / 98 %).                                                                                                                                                |
| a18 | **`govulncheck` green**                                                                                               | `No vulnerabilities found.`                                                                                                                                                                                                                       |
| a19 | **Docs-drift gate green after every doc edit**                                                                        | `nix run .#docs-check` → `OK - README/AGENTS/CHANGELOG/FEATURES in sync with v0.8.1` (run after the ADR range bump to ADR-001..010, the INDEX rows, and the CHANGELOG additions).                                                                  |
| a20 | **AGENTS.md still under its hard cap after three new gotchas**                                                        | 194 → 210 lines; the cap (220) is enforced by `.#docs-check` and `nix flake check`.                                                                                                                                                              |

## b) PARTIALLY DONE

| #   | Item                                         | What is done                                                                                                | What is missing                                                                                                                                                                    |
| --- | -------------------------------------------- | ----------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| b1  | Full `nix run .#gates` sweep                 | 6 of 9 gates verified green **today**: test-race, vet, lint, coverage-gate, vulncheck, security (after a15) | `fuzz`, `fuzz-registry`, `flake check` did not run in the final sweep — the run aborted at the (now fixed) security gate, and the host is out of cache space. Earlier today `.#docs-check` and `.#fuzz-registry` were green; `.#fuzz` was **not** re-run after tonight's changes. |
| b2  | Federation coverage figure                   | Measured 99.3 % by the coverage gate                                                                        | FEATURES/AGENTS/CHANGELOG still say 99.5 %. Either a peer edit opened a new uncovered branch, or the earlier figure was measured on a different tree. **Must be reconciled, not fudged** (see f1).                                                                                    |
| b3  | M26 (golangci LSP)                           | Root cause identified; timeout raised; CLI declared arbiter; gotcha already documented five times           | The LSP still emits stale `gci`/`err113`/`modernize` warnings on files the CLI grades clean (`aggregate_benchmark_test.go:95` was reported again tonight with the CLI at `0 issues`). Disabling or repairing the integration lives in the read-only home-manager module (`crush-config`), i.e. another repo. |
| b4  | M22 report lifecycle                         | TODO_LIST refreshed, cross-links added                                                                      | No report was archived tonight. By the keep-window policy the 20:17/20:42/21:00/21:02 reports still have open items, so nothing is eligible yet — but the rule was not re-checked item by item.                                                                                       |

## c) NOT STARTED (owner-gated, deliberately untouched)

| #   | Item                                                              | Why it did not start                                                                                                                         |
| --- | ----------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| c1  | M01 re-pin both bridges to v0.8.1                                 | Writes to `go-appkit` / `cqrs-htmx`. No push authority in this session.                                                                       |
| c2  | M02 push `go-appkit/health` v0.3.0 (tag exists locally only)       | Irreversible publish. This is the single cheapest correctness win on the board and it is one command.                                        |
| c3  | M03 tag `cqrs-htmx/health` (no module tag at all; ~20 of 36 consumers) | Same gate; largest single lever.                                                                                                          |
| c4  | M04 compile 2 bridges + 5 consumers, publish evidence             | Could be drafted as a script, but the evidence is meaningless until c1–c3 land.                                                              |
| c5  | M06–M09, M19 consumer adoptions                                   | Five other repos; each needs write authority and its own verification.                                                                       |
| c6  | M18 announcement + samber/do#318 + issue #5/#7 posts              | Publishing is the owner's act in third-party channels.                                                                                      |
| c7  | M27 breaking window (`CoerceValidUTF8`, `StatusSince`)            | Owner go/no-go still unanswered.                                                                                                            |
| c8  | M24 BuildFlow upstream filings                                    | Needs `verify-before-filing` at source level against a repo whose HEAD was unbuildable (mid-WIP in another session).                          |

## d) TOTALLY FUCKED UP (self-inflicted, in order of embarrassment)

| #   | Mistake                                                                                                                                                                                  | Cost / correction                                                                                                                                    |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| d1  | **Piped a gate sweep through `tail`** (`nix run .#gates 2>&1 \| tail -40; echo GATES_EXIT=$?`) and read `GATES_EXIT=0` as green. `$?` was `tail`'s status; the sweep had failed at lint.    | This is the exact trap written in AGENTS.md ("gate output tails lie; exit codes don't") **and** in the report I had just read. Re-ran with `> log 2>&1; echo EXIT=$? >> log`. |
| d2  | **Killed my own formatter**: `timeout 600 nix fmt` interrupted treefmt mid-`golines`, so it reported `signal: interrupt` and formatted only 1 file (of 70 queued).                        | Re-ran golines directly on the five flagged files (no diff), then verified the tree formats clean.                                                    |
| d3  | **Deleted a doc comment while inserting a benchmark**: I replaced `BenchmarkReadinessHandler_DrainingScaling`'s comment block with my new function instead of inserting *before* the func. | Noticed immediately, re-added the comment — but left no blank line, which golangci caught (`gci`). Fixed.                                            |
| d4  | Wrote a nonsense CHANGELOG phrase ("`docs/check-art-dupl`-style commentary") referencing a file that does not exist.                                                                      | Caught on re-read within a minute; replaced with the real evidence-log path.                                                                          |
| d5  | Wrote a TODO row asserting the coverage gate "began report-only" without checking.                                                                                                        | Corrected to what the script and CI actually do (it also carries `--report-only`, but nothing uses it).                                               |
| d6  | Two wrong `file:line` citations in the first draft of the dashboard handoff (`main.go:236` vs `:230`; `probe.go:518-560` vs `:522-560`).                                                   | Caught by re-grepping each reference before publishing the draft; corrected.                                                                          |
| d7  | Spent the first ten minutes treating the peer session as interference instead of as a work-split signal.                                                                                  | Once I listed which milestones had landed, the job became "verify, then take the untouched ones" — which is what a10–a16 record.                      |

## e) WHAT WE SHOULD IMPROVE

1. **A gate invocation must never be piped.** Convention worth remembering: `cmd > /tmp/x.log 2>&1; echo "EXIT=$?" >> /tmp/x.log`, then read the file. (d1 is the second time this bit.)
2. **Host environment is now a first-class failure mode**, alongside code and toolchain: `/mnt/buildcache` at 100 %, load average 200–340, and ~15 language servers are each capable of turning a green gate red. Gates that load packages should own their cache (`.#security` now does); the rest should at least report *why* they failed (`cache entry not found` vs `no space left on device` vs `context deadline exceeded` are three different problems that all look like "lint red").
3. **Measured numbers need a stated host condition.** Tonight's ns figures are upper bounds (load >250); B/allocs are exact and are what FEATURES quotes. That convention should apply to every future benchmark row.
4. **Verify a backlog row before drafting its artifact.** Two of M21's four items were already done; two of ROADMAP's three claims were false; TODO_LIST's "blocked on the cut" rows had been obsolete for hours. A two-minute `grep` before writing beats a perfect draft of a non-problem.
5. **Two agents, one repo, no claim protocol.** Five milestones were executed twice-over tonight (peer first, me verifying). It worked because the files are small and the gates are strict, but the verification loop is unsound: I cannot tell "my edit landed" from "the peer's identical edit landed".
6. **Feature docs and measured reality drift within hours.** Coverage said 99.5 % in three docs and measured 99.3 % at 23:00. Either the coverage gate should write the number into a doc, or the docs should stop quoting a decimal.
7. **The `docs-check` gate is now genuinely strong** (API lockstep × 3 surfaces + file-map completeness + `[Unreleased]` truth + line caps). That is the pattern to extend: make the doc *fail* on drift instead of asking sessions to remember.

## f) NEXT — up to 50 items, ranked by value ÷ effort

**Blockers first (they make every other number trustworthy)**

1. Reconcile federation coverage 99.3 % vs the documented 99.5 % (find the uncovered block, cover it or update the docs to the measured value).
2. Free `/mnt/buildcache` (it is 100 % full; `go clean -cache` or prune `golangci-lint`/`go-build` subdirs) — needs the owner's OK because other sessions share it.
3. Run the three gates that did not run tonight: `.#fuzz`, `.#fuzz-registry`, `nix flake check`.
4. Re-run `nix run .#gates` end-to-end once the cache has space; that run is the only truth claim this session is missing.
5. Add `GOCACHE` isolation to any future gate that loads packages (candidates: `.#lint`, `.#vulncheck`) or document why it stays shared.

**The plan's Tier 1 (owner, ~40 min total, unlocks ~24 repos)**

6. Re-pin `go-appkit/health` to v0.8.1.
7. Re-pin `cqrs-htmx/health` to v0.8.1.
8. Push `go-appkit/health` v0.3.0 (already tagged locally).
9. Tag + push `cqrs-htmx/health` (next `health/v4.x`).
10. Land cqrs-htmx README PR #37.
11. Verify both tags resolve from a foreign module (`go list -m -versions`).

**Proof (owner)**

12. Compile 2 bridges + 5 consumers against the release; publish the evidence table.
13. File every failure found as an issue in the owning repo.
14. Re-run the recorder audit after each bump; append the result to the audit doc.

**Adoption (owner, one repo at a time)**

15. go-taskqueue → `NewChecks` + `WithRefreshInterval(0)` (largest deletion).
16. dnsblockd → `CacheAge`/`CachedResponseFresh` + `Response.Clone`.
17. cv → `CachedResponseFresh`/`EvaluateAndCache` + `Clone`.
18. DiscordSync → `CachedResponseFresh`/`EvaluateAndCache` + `WithCriticalService[T]`.
19. company-master-data → `checks.Disk` (manual/forgejo verification).
20. go-health-dashboard: land draft 2 (hub SIGTERM → `MarkShuttingDown`) with the /readyz assertion.
21. go-health-dashboard: land draft 1 (`Shutdown` cancels `Start`'s ctx).
22. Flip `Routes.Healthz` on the hub now that federation implements `Healthz()`.

**Repo-local mechanisms still worth building**

23. `.#pre-tag` smoke test: prove it refuses a dirty tree, an existing tag, and a version with no CHANGELOG section (each is a cheap planted-failure test).
24. Coverage gate: add a `--write-doc` or a docs-check rule that fails when FEATURES quotes a percentage the gate disagrees with.
25. Extend the API lockstep to `checks` and `internal/merge` (the last two packages with a published surface).
26. A `.#fleet-proof` script that composes `.#skew` + `.#recorder-audit` + the compile harness into one command.
27. Make `.#gates` print a per-gate PASS/FAIL summary line with exit codes (belt and braces against d1).
28. Add a `host-health` preflight warning (disk free on `/mnt/buildcache`, load average) before a gate sweep starts.
29. Archive the fully-resolved 2026-10-09 reports (the keep-window rule was not audited tonight).
30. Re-check `docs/INDEX.md` completeness (it is 41 kB of rows; nothing verifies that every doc in `docs/` has a row).
31. Add a docs-check rule: every ADR file must be referenced from `docs/INDEX.md`'s ADR row by number.
32. Add a docs-check rule: every `docs/adr/ADR-*.md` link target in code comments must exist.
33. Decide whether `tools/fleet-repos.txt` should also carry local paths (two tools now resolve them independently).
34. Add a `--json` output mode to `tools/recorder-miswire-audit.sh` for a future fleet dashboard.
35. Re-run the recorder audit from CI with a shallow clone of the public consumers only (today it SKIPs everything off-host).
36. Delete `coverage/` artifacts from the working tree if they are stale (a directory named `coverage` exists next to `reports/`).
37. `.golangci.yml` is mode 600 locally (git only tracks the exec bit, so this is cosmetic — but worth normalising for anyone copying the file).
38. Consider `build-tags: goexperiment.*` removal: those are not build tags, they are GOEXPERIMENT values; they are harmless but they teach the wrong thing.
39. Re-measure the three new benchmarks on an idle host and replace the "upper bound" wording with real medians.
40. Add `BenchmarkCachedResponseFresh` to the "Performance" table's provenance note (it currently inherits the 2026-09-18/09-22 stamps).
41. Decide whether `EvaluateAndCache` should skip its `context.WithTimeout` when the caller's deadline is earlier (would remove the +5 allocs). Needs a semantics review, not a micro-opt.
42. Land the `Healthz`-route recommendation for the dashboard as a docs snippet in `docs/bridge-golden-path.md`.

**Owner decisions / upstream**

43. Answer the BuildFlow-canonical question (flake gates vs full mode) — it decides which red matters.
44. File the two BuildFlow upstream issues (dprint `--fix` no-rewrite; full-mode spawn env) after source-level verification.
45. Restore a buildable BuildFlow binary and re-run full mode to exit 0.
46. Owner go/no-go on the two cheap renames (`CoerceValidUTF8`, `StatusSince`).
47. Owner go/no-go on typed `ServiceName` (the last big breaking change).
48. Publish the v0.1.1/v0.1.2 announcement, the samber/do#318 comment, the issue #5 close comment, the issue #7 annotation.
49. Post the two dashboard drafts (draft file is ready; posting is a one-minute act each).
50. Decide whether `samber-do-auditlog` should implement `DetailedHealthRecorder` (it reverses ADR-004).

## g) QUESTIONS — three things I cannot decide or find out myself

1. **May I clear `/mnt/buildcache`?** It is at 100 % (208 GB used, 0 bytes available) and it is shared with other sessions. `go clean -cache` would free tens of GB and make the gates deterministic again, but it can also break a build that is running right now. Alternative: prune only the `golangci-lint` and `go-build` subdirectories, which are pure caches.
2. **Who else is writing this repo tonight, and do you want a claim protocol?** The peer session cut v0.8.0/v0.8.1, closed issues #5/#6/#7, and executed M05/M14/M15/M16/M23 while I was reading the same files; two of my edits were rejected mid-flight. It worked, but I verified the peer's gates instead of my own on five milestones. A one-line claim (file, milestone) in `docs/status/` or a lock file would make the loop sound.
3. **Do I have push/write authority for the six consumer/bridge repos?** M01–M04 and M06–M09/M19 are ~11.8 h of the plan's value and every one of them is owner-gated. With a yes I would start at M01 (re-pin both bridges to v0.8.1) and work down; without it, nothing on the consumption side can move, because an unpushed bridge tag is the current blocker for ~24 of the 36 reachable repos.

---

## Appendix — numbers and the exact verification log

| Metric                                        | Value                                                                       |
| --------------------------------------------- | --------------------------------------------------------------------------- |
| New/changed Go files (this session)           | 3 test files (`transport_test.go`, `probe_benchmark_test.go`, `federation_benchmark_test.go`) + 2 doc-comment-only source edits (`probe.go`, `aggregate/aggregate.go`, `federation/federation.go`) |
| New docs (this session)                       | 6: ADR-009, ADR-010, accessor-read-cost, recorder-miswire-audit, dashboard-family-handoff, this report — plus edits to FEATURES, TODO_LIST, ROADMAP, CHANGELOG, INDEX, AGENTS |
| Repos / files scanned by the new audit        | 16 repos, 8,507 Go files, 0 mis-wires                                       |
| Benchmarks added                              | 3 (`CachedResponseFresh`, `EvaluateAndCache` A/B, `MergedReadAndStatuses`)   |
| Coverage (measured 23:0x)                     | total **99.9 %** (floor 99.5 %); root/aggregate/checks/merge **100 %**; federation **99.3 %** |
| AGENTS.md                                     | 210 lines (cap 220)                                                         |
| TODO_LIST.md                                  | 97 lines (was 131)                                                          |
| Host load during measurements                  | 200–340 (32 cores); `/mnt/buildcache` 100 % full                            |

Verification commands actually run (exits as observed):

```text
nix run .#gates > /tmp/gates2.log 2>&1; echo GATES_EXIT=$? >> /tmp/gates2.log
  test-race      ok (5/5)
  vet            ok
  lint           0 issues (with run.timeout=10m; shared-cache write warnings)
  coverage-gate  99.9 % total, federation 99.3 %, all floors met
  vulncheck      No vulnerabilities found.
  security       EXIT 1 — vacuous red (SSA skipped, Issues: 0) → fixed, see below
  fuzz / fuzz-registry / flake check: not reached

GOCACHE=$(mktemp -d) nix run .#security      → EXIT 0, 17 files, Issues 0
nix run .#security                          → EXIT 0 (private cache inside the app)
nix run .#docs-check                        → OK - in sync with v0.8.1
nix run .#recorder-audit                    → 0 mis-wires over 8,507 files
nix develop -c golangci-lint run --timeout=15m ./... → 0 issues
go vet ./...                                → 0
nix develop -c dprint fmt <13 files>        → 11 reformatted
```

## Completion note policy

Both earlier reports ([21:00](2026-10-27_21-00_supplier-wave-hardening-coverage-and-gates.md), [21:02](2026-10-27_21-02_distribution-shipped-consumption-zero.md))
now cross-link this one and the plan of record. Their open items are unchanged by tonight's work except:
TODO_LIST no longer carries "cut v0.8.0" or "blocked on the cut", and the two dashboard asks moved from TODO to DRAFT (with one of them proven already done).
