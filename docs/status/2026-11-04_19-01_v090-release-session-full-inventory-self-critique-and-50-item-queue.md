# Status Report: v0.9.0 Release Session — Full Inventory, Self-Critique, and the 50-Item Queue

**Date:** 2026-11-04 19:01 CET
**Session scope:** "Time for a new release?" executed end to end (assess → fix → cut → gates → tag → publish → verify → adopt), the three pre-existing master CI reds, four latent release-gate bugs, the ci-emulation gate-tool audit, the dashboard adoption, and this self-critique.
**Repo state at writing:** go-health `master` @ `13cfa17` local (daemon pushes within minutes; a **parallel session** was landing 27-file refactors concurrently — `ea2df5b`, `c18354e` — and shipped `2f96d43` on the dashboard). Tag `v0.9.0` @ `2d8dc4a` — immutable, pushed, released. Dashboard `master` @ `2f96d43` (parallel session's batch on top of my `b443480`).

---

## a) FULLY DONE (verified with commands/artifacts)

| #  | Item                                                                                                                                                                                                                                                                                                                                                                                                     | Evidence                                                                    |
| -- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| 1  | **v0.9.0 released end to end**: version determination (MINOR — two wire-compatible breaking Go-API changes in 0.x), CHANGELOG cut with preamble + empty placeholders + retargeted compare links (`1b19bea`), README stability line + AGENTS.md status line (219/220 cap held)                                                                                                                            | `git show v0.9.0:CHANGELOG.md`                                              |
| 2  | **Pre-tag sweep green, EXIT=0**: test-race, vet, lint, coverage-gate (99.9% total / 100% root+aggregate+checks+merge / 99.3% federation), vulncheck ("No vulnerabilities found."), security (0 issues, private GOCACHE), fuzz (9 targets), fuzz-registry, interop (fixture green against published httputil), docs-check, flake check, fleet skew (all consumers at v0.8.1, correct pre-release)         | `/tmp/pretag2.out`, `pre-tag: ready to tag v0.9.0`                          |
| 3  | **Annotated SSH-signed tag** on the release commit; `git show v0.9.0:go.mod` verified before push                                                                                                                                                                                                                                                                                                        | `git tag --points-at HEAD`                                                  |
| 4  | **CI green 5/5 on the release commit** (`41867742776`) **and on the tag ref** (`41868854706`) — the v0.8.0-class red-tag failure did not recur                                                                                                                                                                                                                                                           | `gh run list`                                                               |
| 5  | **Module proxy + sum.golang.org**: `go list -m -versions` lists v0.9.0; clean-dir `go get @v0.9.0` + `go mod download` exit 0                                                                                                                                                                                                                                                                            | `/tmp/release-verify-v090` (trashed after)                                  |
| 6  | **Consumer compile+run proof**: throwaway module ran `NewInstanceID`, `InstanceID.Get()`, `CoerceValidUTF8` against the published version — exit 0                                                                                                                                                                                                                                                       | verify.go session                                                           |
| 7  | **pkg.go.dev live and Latest**: README rendered including the new "Response sanitization (`CoerceValidUTF8`)" section; `InstanceID`/`NewInstanceID` in the API index                                                                                                                                                                                                                                     | fetched page                                                                |
| 8  | **GitHub Release** with curated notes (breaking-first, migration snippets, consumer-impact table), `--latest`                                                                                                                                                                                                                                                                                            | releases/tag/v0.9.0                                                         |
| 9  | **Three pre-existing master CI reds fixed**: coverage-gate excludes `docs/` draft packages from the floor arithmetic (still compiles/runs them), the draft's gosec G112 + mnd fixed (`443dc63`)                                                                                                                                                                                                          | coverage-gate EXIT=0, lint EXIT=0 local; CI green from `41870447847` onward |
| 10 | **`pre-tag` made satisfiable** — two latent bugs fixed: the docs-check dance demanded previous-release docs in the git view and pending-release docs in the store view (now: newest CHANGELOG heading competes with newest tag by version, `8859089`); the section regex required `## [0.9.0]` while every release ever reads `## [v0.9.0]` (`80cbe07`). This session was the app's first green run ever | this session's sweeps                                                       |
| 11 | **Interop fixture module refreshed**: `go mod tidy` (go-branded-id → v0.7.1 by MVS), pinned require renamed to v0.9.0, build+test green against HEAD via `../..` (`2d8dc4a`)                                                                                                                                                                                                                             | `nix run .#interop` EXIT=0                                                  |
| 12 | **Gate-tool audit**: six apps ran on borrowed tools; every external command now declared — fuzz-registry gawk+diffutils (`8ce243c`), security coreutils, coverage-gate gnugrep+gnused+git, recorder-audit gnused, openapi-lockstep coreutils (`fc93e82`)                                                                                                                                                 | flake.nix                                                                   |
| 13 | **Per-app stripped-PATH verification** of the audit: openapi-lockstep, recorder-audit, coverage-gate, security all EXIT=0 under `PATH=tmpbin:/usr/bin:/bin` (fuzz-registry proven by ciemul3)                                                                                                                                                                                                            | session outputs                                                             |
| 14 | **Dashboard adoption** `b443480`: pin → v0.9.0; `StatusSince` renames across selectors AND nine struct-literal keys the checklist missed; `CoerceValidUTF8` + comment; `nlreturn` fix (first-ever dashboard lint run → 0 issues); build, test-race, vet, lint green locally; pushed                                                                                                                      | `git log`, CI jobs                                                          |
| 15 | **Release-notes honesty correction**: the Verification line claimed `WithInstanceID`/`StatusSince` were call-exercised; they were only compile-proven — the published notes now say exactly that (`gh release edit`)                                                                                                                                                                                     | release page                                                                |
| 16 | **TODO_LIST v0.9.0 adoption-wave section** + AGENTS.md "daemon also PUSHES and rebases" gotcha (`b857b7e`, `7adcc8c`)                                                                                                                                                                                                                                                                                    | TODO_LIST.md, AGENTS.md:208                                                 |
| 17 | **Session report + INDEX rows** for this report and the unindexed 16:45 report (`13cfa17`); zero-width-space typo caught via `cat -A` before commit                                                                                                                                                                                                                                                      | docs/INDEX.md                                                               |
| 18 | **go.mod hygiene**: tidy + verify clean, no replace directives, no pseudo-versions                                                                                                                                                                                                                                                                                                                       | Phase-3 checks                                                              |
| 19 | **Fleet skew measured post-release**: 15 minor-behind (13+2 that became readable), weekly alarm is design-not-incident; wave task queued                                                                                                                                                                                                                                                                 | `nix run .#skew`                                                            |

## b) PARTIALLY DONE

1. **Full `ci-emulation` green** — never achieved this session. Run 3 died at `security` (undeclared mktemp, fixed); the final run graded a TORN TREE from the parallel session (`ErrRecorderNotApplicable redeclared in probe.go and errors.go` — the sentinel was mid-move). Substituted per-app stripped-PATH verification (a13), which proves MY fixes but not the whole-sweep claim. The settled-tree sweep is left for the next window (and the next `pre-tag` will run it anyway).
2. **Dashboard master CI** — my commit's own scope is green (Build, Lint, Vuln Scan, Version const), but master carries three reds: the FEATURES test-count guard (my `git add -A` froze the daemon's mid-edit 347; actual 348 — the parallel session's tree already carries 348 and their `2f96d43` batch should green it), the Browser CSP screenshot test (the parallel session's own new `assertCaptureContent` guard, introduced in their window, fails deterministically on CI — reproduced on rerun; docs-only commits between the last-green and first-red failed identically, so it is not the adoption), and Hygiene templ drift (passed the original run, failed the rerun on the same commit — non-deterministic,undiagnosed). Their `2f96d43` CI run was queued at writing.
3. **Fleet adoption wave** — only go-health-dashboard adopted (the one consumer reading the renamed field). 15 consumers remain a minor behind; the weekly skew alarm fires Monday 06:17 until the wave closes — that is the design, and TODO_LIST carries the task.
4. **pkg.go.dev deprecation rendering** — I verified the page, the README section, and the `SanitizeResponse` entry in the API index, but never confirmed the `// Deprecated:` strikethrough rendering on the symbol page.
5. **AGENTS.md gotcha surface** — the daemon-pushes correction and the tool-declaration rule are recorded, but the "release dance" semantics (cut → tag → docs-check states) live only in CHANGELOG prose and this report; a future cutting agent must re-derive them.
6. **Global memory** — the two cross-project lessons (gate apps must declare every command; stripped-PATH emulation is the audit; a release intentionally stales the fleet) are recorded in project files only, not in crush-config `references/lessons.md` (requires a commit there by policy).

## c) NOT STARTED

1. Fleet pin-bump wave (15 repos) — needs per-repo build+test+commit+push; several are bridges with their own release trains.
2. go-taskqueue direction (owner question): `WithInformational` third tier vs pass-detail field vs dropping the green notes — design note stays PROPOSED demand-gated; blocks the go-taskqueue → `NewChecks` port.
3. `/mnt/buildcache` triage: `data=writeback,commit=120` intent + root access for SMART/e2fsck (carried owner question).
4. nsfw-classifier + CV/cv clone-lineage decisions (BLOCKED on owner, pre-existing).
5. company-master-data (forgejo-hosted, invisible to the GitHub-API skew check) — never checked for v0.9.0.
6. Body-text dumping in the dashboard `pollJS` timeout path (would have made the Browser failure diagnosable in one run).
7. go-health `scripts/verify-release.sh` — the dashboard has one; go-health's release verification was hand-rolled this session (see e-3).
8. Annotating the 16:45 report with a completion note (its owner question 1 — release timing — is now answered by execution; convention says append a dated Completion note, not edit history).

## d) TOTALLY FUCKED UP

1. **I froze the daemon's mid-edit FEATURES.md (347) into the adoption commit with `git add -A`** → red CI guard on `b443480`. The concurrent-writer preflight exists precisely for this and I applied it to go-health flake.nix but NOT to the dashboard's bulk add. The rule upgrades to: never stage files another writer holds mid-edit; stage by explicit path.
2. **The release notes overstated the verification** ("consumed from a throwaway module (`NewInstanceID`, `WithInstanceID`, `CoerceValidUTF8`, `StatusSince`)") — my verify.go never called `WithInstanceID` or touched `StatusSince`; they were compile-proven only. Published notes corrected post-hoc (a15), but the claim shipped wrong for ~90 minutes. The fix is structural: a verify-release script whose checks and notes are generated from the same list.
3. **The Chrome-update hypothesis was asserted before being checked** — I told the session the runner's Chrome "auto-updated" as the likely Browser-job cause, then the version comparison showed both runs installed the identical 156/157 pair. Classic unverified-external-claim; retracted, but it cost a research cycle and briefly misdirected the diagnosis.
4. **`evidence.go` misidentification + whitespace mangle**: I renamed `row.Since` on the DASHBOARD's own `checkRow` (not health.Check) → build failure; and my whitespace-equivalent multiedit left misaligned indentation that needed a manual repair pass. The type of `row` was visible in the signature I had just read.
5. **Exit-code reads after pipes** — `cmd | tail; EXIT=$?` measured `tail`, not `cmd`, twice (the exact gotcha AGENTS.md documents, and I did it anyway) before switching to capture-then-print.
6. **My first `docs-check` fix draft had a double-v bug** (`vv0.8.1`) — caught by my own run BEFORE commit, but the pattern (write → run → fix instead of trace the printf) is the same haste class as the `[[:<:]]` incident recorded from a previous session.
7. **A zero-width space (U+200B) shipped in my INDEX row** ("daemon-pushe​s") — caught by `cat -A` and fixed pre-commit, but it only existed because I typed the row into a tool that preserved an invisible character and I did not re-read the rendered text.
8. **Two 25-minute ci-emulation sweeps were spent knowing the tree was a moving target** — the second one PROVED its worth (found the security mktemp gap), but the final attempt was predictably racy and produced the torn-tree red; the per-app verification was the right tool and I reached for it one sweep too late.

## e) WHAT WE SHOULD IMPROVE

1. **Stage by explicit path, never `git add -A`, in any repo a daemon or parallel session touches.** The preflight (git log -1 + ls -lt) detects motion; staging discipline prevents it.
2. **Mechanize release verification** — port the dashboard's `scripts/verify-release.sh` idea to go-health: proxy hash, sumdb, clean-dir consumer, GH release state, tag-CI verdict, pkg.go.dev presence AND Deprecated-strikethrough, all from one script; generate the release-notes Verification line FROM that script's results so the notes cannot overstate.
3. **Make `pre-tag` testable** — extract its readiness checks (dirty tree, tag-unused, section format) into a script with fixture fixtures; both latent bugs were only findable by running it against a real cut.
4. **Fixture-test `docs-drift-check`'s three states** (pre-cut / cut / post-tag) — the latest-derivation logic was verified by hand-runs this session; a check fixture would pin it.
5. **Retract-before-assert discipline** — compare the versions (Chrome) BEFORE forming and voicing the hypothesis; the verify-external-claims skill applies to my own debugging claims too.
6. **Exit-code capture as muscle memory** — `cmd > out 2>&1; EXIT=$?` before any pipe, every time; AGENTS.md already documents why.
7. **Parallel-session coordination protocol** — a visible "tree owner" marker (TODO_LIST header line or branch convention) so heavy sweeps (ci-emulation) are not scheduled against another writer's open window; and a rule that parallel sessions push torn-work markers instead of letting gates snapshot mid-move.
8. **Torn-tree tolerance in long sweeps** — ci-emulation/gates could `git rev-parse HEAD` before each gate and abort with "tree moved under the sweep" instead of grading a mixed state.
9. **Cross-project lessons to crush-config** — items b-6's two patterns belong in `references/lessons.md` (by commit, not in-session write).
10. **The status-report checklist the owner gave (a–g)** produced the most honest inventory of the session — fold the a/b/c/d structure into the docs-health harvest convention for every significant session, not just on request.

## f) UP TO 50 THINGS TO DO NEXT (ranked roughly by impact; parallel-session-owned rows marked ⏸)

**The fleet wave (the release's value half):**

1. Fleet pin-bump to v0.9.0: cqrs-htmx/health bridge (release train), go-appkit/health bridge, timesheets, projects-management-automation, webphone, dnsblockd, library-policy, file-and-image-renamer, DiscordSync, KeyHolderAI, Zlota44 + remaining pin-only consumers (build+test+commit+push each).
2. Re-run `nix run .#skew` after the wave; confirm minor-stale 0; run skew.yml via workflow_dispatch so the alarm history shows the wave closed.
3. company-master-data (forgejo): manual v0.9.0 check — the skew script cannot see it.
4. Record the wave in docs/consumer-verification.md (the evidence table pattern).
5. go-taskqueue direction decision (owner) → then either implement `WithInformational` per the design note or close the note as rejected; unblocks the `NewChecks` port.
6. nsfw-classifier / CV+cv lineage: owner decision, then bump or archive.

**go-health gates and tooling:**
7. ⏸ Full `ci-emulation` green on the settled post-refactor tree.
8. ⏸ Verify the parallel session's refactor CI verdicts (ea2df5b, c18354e, and whatever follows) — the sentinel move (`errors.go`) and the 500-line test-file split need their gates.
9. Port `scripts/verify-release.sh` to go-health (proxy, sumdb, consumer, GH release, tag CI, pkg.go.dev, Deprecated strikethrough) — one command per future release.
10. Regenerate the v0.9.0 release-notes Verification line FROM that script once it exists (the corrected line is prose-correct today).
11. Fixture-test `docs-drift-check`'s three release states (pre-cut / cut / post-tag).
12. Extract `pre-tag`'s readiness checks into a testable script with negative fixtures (dirty tree, used tag, wrong heading format).
13. Check whether nixpkgs' gosec now carries securego/gosec#1772 and drop the hardcoded `gosecGo` 1.27.1 pin (CI's gosec 2.30.0 already behaved).
14. Re-run coverage-gate + fuzz-registry on the settled tree (the test-file split changed the per-package layout the floors grade).
15. Torn-tree guard for long sweeps: abort with "tree moved under the sweep" when HEAD changes mid-run.
16. Decide whether `docs/drafts/finding1_repro` becomes a `testdata`-style fixture module (out of `./...` entirely) instead of living on the coverage-gate exclusion.
17. Update README Compatibility if the pinned toolchain moved (it still claims CI runs go 1.27.2 — verify against the current flake).
18. Refresh `docs/recorder-miswire-audit.md` evidence with a post-v0.9.0 audit run.
19. ADR or design-note line for the docs-check "release in flight" semantics (a gate semantic change currently recorded only in CHANGELOG prose).
20. Root-module go-branded-id at v0.7.0 while the fixture/MVS resolves v0.7.1 — bump the root for one-graph consistency or document why the split is fine.

**Dashboard (mostly ⏸ parallel session's lane):**
21. ⏸ Diagnose `TestCaptureREADME_Screenshot`: add body-text dump to `pollJS`'s timeout path; the guard cannot currently say WHAT rendered.
22. ⏸ Pin Chrome in the workflow (apt `google-chrome-stable` is a moving target across runner images).
23. ⏸ Root-cause the Hygiene templ-drift non-determinism (same commit: green original, red rerun).
24. ⏸ Confirm `2f96d43`'s CI verdict greens the FEATURES guard and Hygiene.
25. Correct dashboard `docs/gotchas.md` "daemon NEVER pushes" (contradicted today on both repos).
26. Add the Strikethrough/Deprecated-visibility check to the dashboard's release checklist if it mirrors go-health's.

**Docs and memory:**
27. Completion note on the 16:45 report: its question 1 (release timing) is answered by this release; annotate per convention, never rewrite.
28. Cross-project lesson (crush-config commit): "declare every external command a gate app runs; stripped-PATH emulation is the audit that proves it".
29. Cross-project lesson: "a release intentionally stales the fleet; the post-release skew red is design, not incident".
30. AGENTS.md: one compact gotcha for the release dance states (pre-cut / cut / tagged) and what docs-check expects in each.
31. Harvest this report's §f into TODO_LIST (the fleet row exists; the gate rows below belong there too).
32. Archive the fully-resolved 2026-10-27/28 wave reports once their open items close (convention: resolved reports move to `docs/status/archived/`).

**Release follow-ups:**
33. Verify the rendered GitHub Release page (tables, migration snippets) — I verified the notes file, never the rendering.
34. Watch Monday's skew run: red until the wave closes is correct; confirm it fails with the expected 15 and not something worse.
35. Re-check pkg.go.dev Deprecated strikethrough on `SanitizeResponse` (b-4).
36. Branch protection: confirm the five required checks are unchanged after the workflow edits of recent weeks.
37. Confirm the daemon's push of `13cfa17` (this report) and that no later commit reverts the INDEX rows.
38. go.work/GOWORK audit: confirm every flake app sets or unsets exactly the env it needs (the host-leak gotcha class).
39. Consider a `nix run .#release-notes` helper: render the CHANGELOG section to the gh notes file with the verification line appended from verify-release output.
40. Tag-CI race hygiene: document that master and tag pushes may interleave (today the daemon pushed master before the explicit push; ordering claims need `git fetch` first).

**Smaller cleanups:**
41. `tools/fuzz-registry-check.sh`: the `comm` drift-lists redirect to stderr with `|| true` — print a marker when `comm` itself is missing so the next undeclared-tool regression is self-describing.
42. `privateGocache`: consider exporting `GOLANGCI_LINT_CACHE` under a per-gate suffix (it already does) — verify security/lint/vulncheck do not share one cache dir across parallel gates.
43. Dashboard `contrast_test.go` nlreturn fix — confirm it survives the parallel session's batch (it was in my commit; their batch touched adjacent test files).
44. README stability line: keep the dependency sentence in sync when go-branded-id bumps (v0.7.0 → v0.7.1 root-side decision pending, item 20).
45. `docs/research/2026-10-27_fleet-consumer-needs.md`: mark the dashboard adoption row adopted (it lists go-health-dashboard as the known StatusSince consumer).
46. Sweep `docs/INDEX.md` for the 2026-11-04 planning/status rows the parallel session may have added (keep the index honest in both directions).
47. erraudit run on the post-refactor tree (`./... --type-aware`, baseline 0) — the sentinel move is exactly the kind of change that shifts error-wrapping patterns.
48. `nix flake check` on the settled tree after the parallel session's flake.nix (+15 lines in their window) — the skew/pre-tag-related additions need the hermetic check.
49. Keep `/tmp` release scratch out of the tree (already trashed) — note the pattern: draft release notes in /tmp, never in-repo, while sweeps run.
50. When the fleet wave closes, cut v0.9.1 or fold into the next minor per the CHANGELOG-state at that time (weekly skew decides the urgency).

## g) THREE QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **The parallel session's status and intent.** Someone (another agent or you) landed a 27-file go-health refactor (test-file splits, the `ErrRecorderNotApplicable` move to `errors.go`) and dashboard batches while I worked, and their CI runs are still pending. Should I treat both trees as OWNED until that session signals completion (i.e., hold items 7–9, 21–24), or is it finished/interrupted work I should verify and finish myself?
2. **Fleet-wave authority and timing.** Bumping 15 consumers means build+test+commit+push across many repos, several with their own release trains (the bridges). Do you want that wave executed NOW by me (hours of cross-repo work, private repos included), or is it reserved for a coordinated fleet session like the 2026-10-28 one?
3. **go-taskqueue direction.** The `WithInformational` design note stays PROPOSED and demand-gated on a named first adopter: do you want the informational tier (soft checks evaluated, visible, excluded from the roll-up), a pass-detail field, or the informative green rows dropped? This decides whether go-taskqueue's port is mechanical or waits on new API.

---

_Completion notes to this report follow the 2026-10-08 convention: append a dated `## Completion note` section below rather than editing history._
