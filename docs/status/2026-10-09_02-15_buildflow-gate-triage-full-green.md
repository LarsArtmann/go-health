# Status Report — BuildFlow Gate Triage: 5 Failures → Full Green

**Created:** 2026-10-09 02:15 CEST
**Session scope:** Triage of the failed `buildflow --fix --build-mode=full` run (2026-10-08 ~21:08, exit 69, 5 step failures) through to a verified green full run (exit 0, "BuildFlow passed with warnings 86/97, 36.6s") plus a fast-mode confirmation.
**Method note:** Everything below is from this session's run only — no fresh research into unrelated subsystems.

---

## Headline

| Metric              | Start of session                                                            | End of session                                                           |
| ------------------- | --------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| BuildFlow full run  | ✗ exit 69, 5 steps failed                                                   | ✓ exit 0, 0 steps failed                                                 |
| Gating findings     | go-auto-upgrade OVER budget (7 > 6)                                         | 7 findings, within budget 8                                              |
| Non-gating warnings | go-version-auto-configure (1), AGENTS.md size (221 > 220), binary freshness | go-version-auto-configure (1, documented), binary freshness (unresolved) |
| Test suite          | test-race blocked upstream in the DAG (never ran)                           | green, incl. one fixed load-flake                                        |

---

## a) FULLY DONE

1. **`lychee` link rot — 2 of 2 links fixed.**
   `.github/ISSUE_TEMPLATE/feature_request.md:29` used a GitHub-web-style path (`../../blob/master/ROADMAP.md`) that resolves to a nonexistent local file; corrected to `../../ROADMAP.md` (works on GitHub web AND disk). `docs/planning/2026-09-04_19-34_pareto-master-plan-v2-contract-ship-and-verification.md:3` pointed at a status report that had moved to `docs/status/archived/`; relinked.
2. **`govalid-generate` ×2, `license-check` ×2, `go-generate` ×2 — all green in full mode.** Root-caused to a three-layer problem (see e2), fixed with `tool_paths → tools/bin` wrapper scripts + `.buildflow.yml` `env: GOTOOLCHAIN: auto` + `~/.local/bin/govalid`. Verified: pure full run exit 0, no shell tricks.
3. **`test-race` load-flake fixed at the source.** `aggregate_test.go:398` (`TestCachedResponse_TotalLatencyMsIsSlowestSource`) asserted exact latency equality; under `-race` + suite-wide parallelism the "fast" source's own evaluation jitter (measured 7ms vs the slow source's 2ms sleep) legitimately becomes the merge max. Verified the merge's max-semantics are correct before touching the test; relaxed the assertion to `>=` with a rationale comment. Test ×10 + full race suite green, and it survived the full BuildFlow run under load.
4. **`go-auto-upgrade` budget drift triaged (7 > 6).** All 7 findings enumerated via `--format finding`: 2× `lo-dependency-missing`, 1× `lo.FromPtrOr` (handlers.go:146), 2× `lo.Map` (info), 2× `lo.SliceToMap` (probe.go:396/408 — the new `detailOf`/`errorsOf` adapters). samber/lo deliberately absent (single-dependency policy, `.buildflow.yml` comment of record). Budget raised 6→8 with drift rationale — the tripwire did its job; the response was investigation, not ratcheting.
5. **Toolchain-skew root cause fully mapped and documented.** Chain proven: released `samber-linter v0.4.0` declares `go 1.27.1` → branching-flow's `go mod tidy` bumped to 1.27.1 → `tools/doanalyzerv2` (local replace) bumped likewise. Empirically proved hand-lowering is unstable (`go 1.27` → tidy → `go 1.27.1`, even auto-switching to go1.27.2). The samber-linter LOCAL checkout already complies (`go 1.27`) — it is an unreleased fix. Captured in AGENTS.md gotcha ("Go-directive patch floors cascade through replace chains") + TODO_LIST Blocked row.
6. **`.buildflow.yml` is now accurate, commented policy.** `warnings_budget.go-auto-upgrade: 8` (drift rationale), `env: GOTOOLCHAIN: auto`, `tool_paths` for both generator steps — each with a removal trigger written next to it.
7. **AGENTS.md size warning resolved by deletion-of-concern, not by me.** The preflight flagged 221 lines; the file was already 188 by the time I checked (another session moved the docs index to `docs/INDEX.md`). Verified stale, no action needed.
8. **All session artifacts committed** by the auto-commit daemon (wrappers, config, test fix, link fixes, docs). Working tree clean at report time except this report.

## b) PARTIALLY DONE

1. **BuildFlow binary rebuild + reinstall.** The installed binary (ec8d2d3) is ~50 commits stale and is the underlying hazard: it resolves generator tools to a 1.27.0-era go and loses the configured env for generator spawns in full mode (single-step passes; config env and shell exports verified lossy there; `BUILDFLOW_NO_RESULT_CACHE=1` runs passed — env-path difference is real but the exact mechanism in the stale binary is unmapped). Rebuild attempted and **blocked**: BuildFlow HEAD was non-compiling across 3 consecutive commits in a 5-minute poll (concurrent session mid-refactor, unused imports in `execution/pipeline_1000.go`). Plan B (build pre-refactor rev 20b89aa via `git+file://`) was scoped but not executed — the investigation showed the binary's OWN flake lock already had go_1_27=1.27.1, so a rebuild of that rev wouldn't fix govalid; the govalid binary itself (system package, built 2026-09-17, go 1.27.0 era) is the actual stale layer.
2. **The mitigation stack is scaffolding, not the root fix.** Four layers now exist: `tools/bin/{go,govalid}` (load-bearing), `.buildflow.yml` `env:` + `tool_paths`, `~/.local/bin/govalid` (backstop, partially redundant with tools/bin), and the documentation. They work; they are also debt with a written removal trigger that nothing enforces.
3. **`go-version-auto-configure` finding (`tools/doanalyzerv2/go.mod:3` = `go 1.27.1`).** Non-gating warning; unfixable locally (tidy re-bump proven); clears only when the samber-linter release cascade lands. Documented in AGENTS.md + TODO_LIST.
4. **`license-check` stderr noise.** go-licenses now passes but logs E-lines about missing LICENSE files in `tools/doanalyzerv2` (real: no LICENSE file there) and — odd — in branching-flow despite a LICENSE file existing at its root. Non-fatal, left as-is.
5. **`govulncheck [root]` 13 "requires newer Go version go1.27" warnings.** Detect-only. BuildFlow's own hint file names the cause (the binary's embedded go/packages loader is older than the analyzed code's directive) and states no env fix helps — clears only with a rebuild. Not actioned.
6. **This report's section (f) has NOT been harvested** into TODO_LIST/ROADMAP — per instruction, waiting.

## c) NOT STARTED

1. Release samber-linter v0.4.1 (the root fix; needs tag+push — outside session authorization).
2. `go get samber-linter@v0.4.1` in branching-flow + re-lower both go directives.
3. BuildFlow rebuild/reinstall once its HEAD compiles; then re-verify the full run with the new binary and re-check whether single-step/full discrepancy disappears (would validate the env-loss root cause).
4. Upstream BuildFlow improvement: bundle govalid with `goPkg` in `tools.nix` (goLicenses-style override) so generator toolchains stop depending on the system PATH.
5. LICENSE file for `tools/doanalyzerv2` (or an explicit decision to accept the go-licenses noise).
6. Diagnosis of why branching-flow's existing LICENSE isn't matched by go-licenses.
7. Removal of the mitigation stack (tools/bin wrappers, `tool_paths` block, `env:` block, `~/.local/bin/govalid`) once the root fix or a rebuilt BuildFlow lands.
8. vulnix CVE backlog (async-2.2.6, binutils 2.46/2.47, cargo 1.98.1, +23 more) — nixpkgs/system-rebuild territory.
9. The 9 unavailable BuildFlow tools (incl. `interrogate`) — `buildflow doctor --verbose` was never run.
10. gitleaks + codespell (on-demand only by config; never invoked this session).
11. gopls error on the nested module (same directive chain; would need gopls env wiring or an explicit ignore).
12. lychee redirect hint (2 redirecting URLs worth resolving to their targets).

## d) TOTALLY FUCKED UP

1. **The released-module patch-floor chain itself.** samber-linter v0.4.0 shipped `go 1.27.1` in its tag — exactly the anti-pattern go-version-auto-configure exists to prevent — and the error then propagated through two downstream module graphs, breaking every toolchain that runs < 1.27.1 (BuildFlow's closure go, gopls on this machine). This is a fleet-level policy violation living in a released tag; only a release heals it.
2. **BuildFlow HEAD is broken-dirty and has been for 5+ consecutive observed minutes** (3+ different non-compiling commits from the concurrent refactor). Until it lands, every fleet repo runs on an increasingly stale binary whose failures (like today's) get misattributed to project code before anyone checks the binary.
3. **My first-fix loop was partly wasted effort — own goal.** I validated fixes via single-step runs, which pass under conditions full mode doesn't reproduce, and then burned ~3 full-run cycles rediscovering mode-dependent behavior. The evidence was pointing at "full vs single-step difference" for a while before I treated it as the primary fact instead of noise.
4. **One transcript lie I had to walk back:** a `go build ./... | head && echo COMPILES_NOW` pipeline that printed success on a failing build (head's exit code masked go's). Caught and corrected in the same batch, but it happened. Shell verification must check exit codes explicitly, always.

## e) WHAT WE SHOULD IMPROVE

1. **Verify at the level of the user's invocation.** Single-step passes proved nothing about full mode; the full command should have been the primary verification loop from the start, with single-steps only for drill-down.
2. **Read the tool's own packaging before theorizing.** BuildFlow's `nix/tools.nix` already documented that govalid packaging lags (go1.26-built binary can't type-check json/v2, gotcha #169). Reading `nix/` early would have short-circuited several hypothesis loops.
3. **Exit-code-strict shell checks.** No `cmd | head && echo OK` patterns in verification contexts, ever.
4. **Mitigation colocation.** The winning pattern was repo-local wrapper scripts referenced by the config that needs them (`tool_paths → tools/bin`), with the removal trigger written inline. The `~/.local/bin/govalid` backstop predates it and is now 80% redundant — one mitigation per layer, documented precedence.
5. **Cross-repo edits need a harder stop.** I hand-edited `tools/doanalyzerv2/go.mod` (reverted by tidy, net zero) and considered editing branching-flow's go.mod knowing their own tooling would re-bump it. Both were reverted/abandoned, but the instinct to "just fix it over there" should have been checked against the "their tooling will fight it" fact before the first keystroke.
6. **Test edits to fresh code deserve a louder flag.** The aggregate test I relaxed was written hours earlier by another session. The change is correct (merge semantics verified first), but it's an owner-visible judgment call and should be reviewed (question 2 below).
7. **Concurrent-session hygiene.** TODO_LIST.md changed under me twice mid-edit; switching to exact-match scripted replacement was the right adaptation — should have been the default after the first conflict.
8. **The 221-line AGENTS.md warning cost me a planning cycle.** Checking the current state BEFORE planning the fix (one `wc -l`) is the cheap first move for every preflight warning.

## f) UP TO 50 THINGS TO GET DONE NEXT

_(brainstorm, sorted roughly by impact; most items beyond ~15 are ROADMAP fuel, not commitments)_

**Root-fix cascade (highest impact, blocked on #1):**

1. Release samber-linter v0.4.1 with major.minor-only `go` directive (local checkout already complies).
2. `go get github.com/larsartmann/samber-linter@v0.4.1` in branching-flow; confirm its directive re-lowers to `go 1.27`.
3. Re-lower `tools/doanalyzerv2/go.mod` to `go 1.27`; confirm tidy keeps it.
4. Remove `tools/bin/` wrappers + `tool_paths` block from `.buildflow.yml`.
5. Remove `env: GOTOOLCHAIN: auto` from `.buildflow.yml`.
6. Remove `~/.local/bin/govalid` wrapper.
7. Update AGENTS.md gotcha + TODO_LIST row to "resolved" with the release versions.
8. Re-run full BuildFlow on the clean state; expect go-version-auto-configure warning to disappear.
9. Verify branching-flow's own BuildFlow run goes green (their go-version warning clears too).

**BuildFlow binary health:**
10. Wait for BuildFlow HEAD to compile; `nix build . && nix run .#reinstall`.
11. Confirm binary-freshness preflight warn clears.
12. Re-run full suite on new binary; confirm govalid/go-generate pass without wrappers (validates env-loss diagnosis).
13. Re-check govulncheck [root]'s 13 "requires newer Go version" loader warnings on the new binary.
14. Upstream BuildFlow task: bundle govalid with `goPkg` in `tools.nix` (goLicenses-style) so generator toolchains don't depend on system PATH.
15. File/ask upstream (verify-before-filing first): is full-mode generator env loss (config `env:` not reaching generator spawns) intended phase-order semantics or a bug?
16. Ask upstream: should `EnsureGotoolchainAuto` (HEAD feature) also scan nested modules like `tools/*/go.mod`, not just root + go.work members?
17. `buildflow doctor --verbose` — triage the 9 unavailable tools (interrogate et al.); install or explicitly accept.
18. Run `buildflow config lint` to confirm the new `.buildflow.yml` keys (env, tool_paths, raised budgets) are all canonical and minimal.

**Repo hygiene (this session's leftovers):**
19. Add LICENSE to `tools/doanalyzerv2` (or document accepting go-licenses E-log noise).
20. Diagnose why branching-flow's LICENSE file isn't matched by go-licenses' name regexp.
21. Resolve the 2 redirecting URLs lychee flagged (replace with targets).
22. Run gitleaks + codespell once on-demand (config-skipped in full mode; never run this session).
23. Shellcheck the `tools/bin/` wrapper scripts (confirm BuildFlow's shfmt/shellcheck steps actually cover `tools/bin` — the green run may not have scanned them).
24. Confirm treefmt/nix-fmt leaves the wrapper scripts alone on every run.
25. Sweep `aggregate_test.go`/federation tests for other exact-latency/equality assertions of the same flake class.
26. Consider a stress-mode CI canary for the race suite (parallel load) to catch timing flakes before they gate.
27. Owner review of the aggregate_test.go `>=` relaxation (see question 2).
28. Re-read the concurrent session's `docs/status/2026-10-09_00-30` docs-health sweep for conflicts with this session's edits.
29. Confirm the auto-commit daemon's commits captured every session artifact correctly (spot-check the 6+ commits it made during this session).
30. HARVEST section (f) of this report into TODO_LIST/ROADMAP (docs-health HARVEST) — on instruction.

**Skew-class prevention (fleet):**
31. Sweep the fleet for other replace-chain patch floors (branching-flow pattern) — go-ecosystem-upgrade skill territory.
32. Add a fleet-level check (BuildFlow provider idea): fail when a released dependency's `go` directive has a patch component AND a local replace pulls it into the graph.
33. Consider pinning `GOWORK=off` explicitly in `.buildflow.yml` env (gotcha mentions workspace interference; the env block is the natural home).
34. Track branching-flow's go-version-auto-configure warning to green (same chain, their repo).
35. Track samber-linter's CHANGELOG/release notes for v0.4.1 (go-release skill when it happens).

**Security/backlog surfaced by the run (mostly system-level, detect-only):**
36. vulnix: async-2.2.6 CVE-2021-43138 (7.8 high) — nixpkgs bump.
37. vulnix: binutils 2.46/2.47 advisory cluster (17 advisories) — nixpkgs/system.
38. vulnix: cargo 1.98.1 advisory cluster incl. 2 criticals — system rebuild.
39. The remaining +23 vulnix findings — batch-triage, most nixpkgs-level.
40. `nix-flake-check` aarch64-linux omission — decide if darwin/linux-arm support matters or stays documented.

**Documentation/consistency:**
41. Cross-check AGENTS.md BuildFlow gotcha numbers after the concurrent session's edits (both sessions touched it today).
42. Re-verify `.buildflow.yml` budgets (art-dupl 80, branching-flow 14, go-auto-upgrade 8) against actual counts on the next 2 runs; tighten if over-provisioned.
43. Decide whether markdown-lint stays config-skipped in full mode or moves on-demand like gitleaks/codespell.
44. Review lychee's 27 excluded URLs + 33 unscanned files (path-excluded dirs) for staleness.
45. Confirm `nix run .#docs-check` still passes after today's AGENTS/TODO churn (it ran green inside the BuildFlow run — re-confirm after harvest).

**Small/deferred:**
46. Decide fate of the `~/.local/bin/go-licenses` wrapper's GOROOT-alignment approach now that govalid got a sibling wrapper — one documented pattern for both, or consolidate.
47. Instrument or document the `BUILDFLOW_NO_RESULT_CACHE=1` env-path difference (it changed generator outcomes at 00:53) — understanding it may explain the env loss.
48. Add the tools/bin removal trigger to whatever checklist owns `.buildflow.yml` reviews (currently only inline comments).
49. After root fix: delete the AGENTS.md gotcha's mitigation paragraphs, keep only the mechanism lesson.
50. Consider naming the wrapper-directory convention (`tools/bin`) in docs/INDEX.md so the next session doesn't rediscover it.

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **May I release samber-linter v0.4.1** (its local checkout already carries the fixed `go 1.27` directive) and then bump branching-flow + re-lower the two directives? It's a tag+push on another repo — the root fix for the entire skew chain, and I won't push without explicit approval. If yes: bundle the branching-flow bump + go-health re-lowering into the same authorization?
2. **Is the `aggregate_test.go` latency-assertion relaxation (`==` → `>=`) acceptable**, or do you want exactness preserved via a deterministic redesign (injected clock seam, like the probe's `WithNowFunc`) at the cost of test-complexity? The merge logic itself is verified correct; only the assertion was over-specified.
3. **When the root fix lands, should the mitigation stack be removed immediately** (my recommendation: yes — wrappers, tool_paths, env block, ~/.local/bin/govalid, per the inline removal triggers), or kept as defense-in-depth for the fleet's other replace-chain repos? Related sub-question: may I run `nix run .#reinstall` (nix profile mutation) autonomously next time BuildFlow's own advisory recommends it, or do you want to gate profile changes personally?

---

_Point-in-time snapshot 2026-10-09 02:15 CEST. Section (f) is HARVEST-ready for docs-health on instruction._
