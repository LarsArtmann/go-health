# Status Report — Dedup + v0.4.0 Release Session

**Date**: 2026-09-22 21:32 CEST
**Scope**: This session only (user directive: report what this session did and noticed; no new research).
**Session command**: `art-dupl -t 1` output → "deduplicate! verify! release v0.4.0!"
**End state**: go-health master at `68275f7`, tree clean, v0.4.0 released and verified end-to-end, dashboard bumped, all gates green.

---

## Session Timeline (what actually ran)

1. Loaded skills: deduplicate-code, go-release, buildflow (+ go-ecosystem-upgrade mid-session, status-report now).
2. Noticed `scratch_cookbook_test.go` lint warnings in project diagnostics → file did not exist (stale LSP noise from last session's trash); restarted LSP at session end.
3. Judged 4 clone groups: extracted 2, accepted 2 (details in §a).
4. Tests green, lint 0 issues, art-dupl re-run: 4 groups → 2 (both intentional).
5. CHANGELOG cut, go.mod hygiene checks, full `nix run .#gates` green on the exact release tree.
6. Committed CHANGELOG (`2140e00`), annotated tag `v0.4.0`, pushed master + tag.
7. Post-push verification: proxy `.info` hash matches the tag commit; clean-dir `go get` (one retry, see §d), `go build` of the module OK; pkg.go.dev live and marked Latest; GitHub Release created (`--latest`, full release, title convention matched to v0.3.0); CI green on the tagged commit.
8. Consumer propagation: go-health-dashboard baseline suite green → bumped `v0.2.1-0.20260918...` pseudo-version → `v0.4.0`, tidy + verify, full suite green again (daemon committed `b1128ae` there).
9. Post-release staleness sweep (§f10 of the 21-01 report): fixed stale claims in README ×4, CONTRIBUTING ×1, AGENTS.md ×2, FEATURES.md ×1; flake check green after doc edits; committed `68275f7`.

---

## a) FULLY DONE

| # | Item | Evidence |
| - | ---- | -------- |
| 1 | Clone G2 extracted: the 3× `cfg := buildConfig(opts); cfg.recorder = nil` became one `buildStandaloneConfig` (probe.go:332) — the "explicit function owns batch execution → recorder void" rule now lives in one named place | accessors.go (`NewWithDetailedCheck`, `NewWithHealthCheck`), checks.go (`NewChecks`) |
| 2 | Clone G3 extracted: 2× `context.WithTimeout` + `Evaluate` became one `evaluateBounded` (handlers.go:132) — "live batch under the batch deadline" named once | handlers.go `readinessResponse`, `throttledLiveResponse` |
| 3 | Clones G1 (RegisterRoutes ×3 packages) and G4 (`Option func(*config)` ×2) judged ACCEPTED, not extracted: three-handler contract repeats per package by design; per-package option types configure distinct config structs | rationale recorded in CHANGELOG `[v0.4.0]` Changed |
| 4 | art-dupl verification: 4 groups → 2, remaining groups are the accepted ones | `art-dupl -t 1 --type-aware` re-run |
| 5 | Tests + lint after dedup: full suite green, golangci-lint 0 issues | `nix run .#test`, `nix run .#lint` |
| 6 | CHANGELOG cut: `[Unreleased]` → `[v0.4.0] - 2026-09-22`, empty placeholders left, compare links updated, internal-dedup entry added | CHANGELOG.md |
| 7 | go.mod release hygiene: no replace directives, no pseudo-versions, `go mod tidy` + `go mod verify` clean on go1.27.1 devShell | bash checks |
| 8 | Full gates green on the final release tree (`nix run .#gates`: test-race, vet, lint, vulncheck, security, fuzz, flake check incl. openapi-lockstep) | "all gates green" |
| 9 | Release mechanics: annotated tag with headline message, `git tag --points-at HEAD` verified, `git show v0.4.0:go.mod` sane, pushed master + tag | 2140e00 → tag v0.4.0 |
| 10 | Proxy verification: `proxy.golang.org/.../@v/v0.4.0.info` returned `Hash: 2140e00...` — tag content cryptographically pinned at the right commit | fetch |
| 11 | Consumer verification: clean-dir `go mod init` + `go get go-health@v0.4.0` + `go build ./...` green (after GOTOOLCHAIN=auto retry, see §d-2) | /tmp/release-verify |
| 12 | pkg.go.dev: v0.4.0 indexed, marked **Latest**, docs rendered | fetch |
| 13 | GitHub Release `v0.4.0 — aggregate Healthz + OpenAPI lockstep` created with curated notes, `--latest`, full release (not prerelease) matching v0.3.0/v0.2.0 conventions | gh release create |
| 14 | CI green on the exact tagged commit | gh run list (CI, v0.4.0 push → success) |
| 15 | Dashboard baseline established BEFORE the bump: full suite green on the old pin | `nix run .#test` in go-health-dashboard |
| 16 | Dashboard bumped to v0.4.0 via `go get` + tidy + verify (never manual edits), go.mod re-checked after, full suite green post-bump | commit b1128ae (dashboard) |
| 17 | Staleness fixes: README stability line (v0.2.0 → v0.4.0), README requirements (1.26+ → 1.27+), README compatibility table (CI runs go 1.26.7 → **1.27.1 via Nix**, the truth), README go-directive mention, CONTRIBUTING prerequisite, AGENTS.md header status, AGENTS.md consumer paragraph (dashboard now on v0.4.0), FEATURES.md consumer row (v0.1.2 → v0.4.0) | commits eda9367, 68275f7 |
| 18 | Flake check green after doc edits (treefmt + openapi-lockstep) | `nix flake check` |
| 19 | Stale LSP diagnostics for the trashed scratch file cleared via `lsp_restart` | session end |

## b) PARTIALLY DONE

1. **Release-documentation freshness**: the README/CONTRIBUTING fixes exist on master, but the v0.4.0 **tag froze the stale README** — pkg.go.dev shows "Requirements: Go 1.26+", "Stability: v0.2.0 alpha", and "CI runs go 1.26.7" until the next release. Fix landed one step too late to help the current release's own docs (see §d-1).
2. **Verification completeness**: full gates ran on the release tree; after the doc-only sync I ran `nix flake check` (treefmt + lockstep) but not the full gates. Deliberate docs-only scope call, but it means "gates green" strictly applies to the tag commit, not to `68275f7`.
3. **Consumer propagation**: dashboard bumped **locally only** — that repo is ~6 commits ahead of origin (daemon doesn't push; I don't push unasked). Upstream consumers of the dashboard still see the old pin until someone pushes.
4. **Previous session's 3 owner questions**: 1 of 3 resolved (release vehicle → shipped as v0.4.0). #318 comment post and auditlog decoupling remain owner-gated, untouched.
5. **§f harvest**: the 21-01 report's 50 brainstorm items remain unharvested into TODO_LIST/ROADMAP (you said wait). This report's §f adds to that pile.
6. **go-release Phase 8.2 doc sync**: done, but executed *after* the tag instead of *before* — the ordering mistake half-neutralized the work (see §e-1).

## c) NOT STARTED

1. **samber/do#318 comment post** — draft + verification notes ready in `docs/announcements/2026-09-22_samber-do-issue-318-duration-comment.md`; owner-gated; untouched this session.
2. **auditlog `DetailedHealthRecorder` decision** — blocked-with-evidence row in TODO_LIST; untouched.
3. **v0.1.1/v0.1.2 announcement publish** — draft exists since 2026-09-04; owner-gated; untouched.
4. **v0.4.0 announcement draft** — does not exist; noticed as a gap this session (prior releases have announcement drafts; v0.4.0 shipped without one).
5. **`go 1.27.1` → `go 1.27` directive normalization** — noticed during release (go-ecosystem-upgrade rule: major.minor-only floors); pre-existing since v0.3.0, deliberately not fixed mid-release; would need v0.4.1. Not started.
6. **ROADMAP staleness re-check** — last session retargeted v0.4.0 candidates into ROADMAP; those shipped today; ROADMAP was not re-verified this session (out of scope per your directive).
7. **Post-push dashboard CI check** — can't verify until the dashboard is pushed.

## d) TOTALLY FUCKED UP

1. **The tag froze stale docs — my ordering was wrong.** The pre-tag phase verified *code* (gates, go.mod, CI) but I ran the *documentation* staleness grep only after pushing, because the previous session's plan listed it as a "post-release" step. Consequence: the released v0.4.0 pkg.go.dev page permanently (until v0.4.1+) advertises Go 1.26+ requirements and a CI Go version (1.26.7) that is flatly false — while the *same page's* prose correctly says 1.27+. An internally contradictory release page is worse than a merely outdated one: it teaches readers the docs can't be trusted. The knowable-in-advance failure: one `rg "Go 1\.26|v0\.2\.0 alpha" README.md CONTRIBUTING.md` before tagging would have caught all of it. Doc sync belongs BEFORE the tag, not in Phase 8 cleanup.
2. **Consumer verification first attempt used the wrong toolchain.** I ran `go get` with the host's go 1.26.7 and `GOTOOLCHAIN=local`, which hard-fails on the 1.27.1 floor — a floor this project's own AGENTS.md documents extensively. Wasted a cycle; the correct invocation (GOTOOLCHAIN=auto, or the devShell's go) was predictable from context I already had.
3. **Tagged before CI ran on the exact release commit.** The skill's rule is "never tag while the latest run on the release branch is red or in progress" — technically satisfied (previous master run green), but the commit being tagged (CHANGELOG cut, `2140e00`) had no CI result yet when the tag was created. It passed after push (doc-only commit, low risk), but the safe order — push, watch CI, then tag — was inverted. Combined with #1, the release's "verification" posture was greener than it was rigorous.
4. **Minor: dashboard bump verified by tests only.** I ran the dashboard's test suite, not its full gate set (if any lint/format gates exist there, they didn't run against the bumped tree).

## e) WHAT WE SHOULD IMPROVE

1. **Make the pre-tag staleness grep a mandatory release step** — grep README/CONTRIBUTING/FEATURES/AGENTS for: stale version claims, stale toolchain claims, "stability" lines, "unreleased" language. Better: enforce it (see §f-9). Docs freeze at tag; every doc fix after tagging is a fix for the *next* release only.
2. **Consumer verification should default to `GOTOOLCHAIN=auto`** whenever the project floor ≥ host toolchain — or run the clean-dir check inside the devShell. One command, no retry.
3. **Order releases: commit → push → CI green on the exact commit → tag → push tag → proxy/pkg.go.dev verify → GitHub Release.** I mostly followed this but tagged one step early.
4. **Single source of truth for version/toolchain claims.** README's stability line and compatibility table are hand-maintained and drifted through two releases. The openapi-lockstep check is the in-repo precedent: a docs-lockstep check (README stability line == latest tag; README CI-Go claim == flake's go_1_27) would make this drift class impossible, not just unlikely.
5. **Dedup judgment was sound but art-dupl `-t 1` was your choice — record it.** The accepted-groups rationale currently lives only in a CHANGELOG bullet; if a fourth package ever appears (health/aggregate/federation + X), the RegisterRoutes extraction question should be revisited at N=4. That trigger lives nowhere durable yet (§f-40).
6. **Dashboard cross-repo work should end with an explicit push/CI check or an explicit handoff note** — "bumped and green locally" is 80% of the job; the remaining 20% (push, upstream CI) silently pends on the owner.
7. **The auto-commit daemon committed mid-edit again** (AGENTS.md/FEATURES.md mod-time edit failures). Harmless, but the pattern is now recurring: for multi-file doc batches, either read-edit-commit in one tight window or expect one retry per daemon tick.

## f) Things we should get done next (brainstorm — harvest input, not commitments)

*Impact-sorted within groups. [O] = owner-gated / owner action. Items 1–16 follow directly from this session; 17–50 are grounded in session observations + still-open items from the 21-01 report.*

**Release follow-ups (v0.4.0 aftermath)**
| # | Item | Impact | Effort |
| - | ---- | ------ | ------ |
| 1 | Push go-health-dashboard (~6 local commits incl. the v0.4.0 bump), verify its CI green [O] | High | 5min |
| 2 | Draft the v0.4.0 announcement (channels + checklist, like prior releases) [O publishes] | High | 30min |
| 3 | Decide v0.4.1: normalize `go 1.27.1` → `go 1.27` (major.minor floor rule) + refreshes the frozen stale README on pkg.go.dev [O] | High | 15min |
| 4 | Add a docs-lockstep flake check (README stability line == latest tag; README CI-Go claim == flake go_1_27), extending the openapi-lockstep precedent | High | 1h |
| 5 | Codify the release order in AGENTS.md (new "Release" section): staleness grep BEFORE tag → gates → commit → push → CI on exact commit → tag → proxy/pkg.go.dev verify → GitHub Release → dashboard bump | High | 30min |
| 6 | AGENTS.md gotcha: "docs freeze at tag — never fix release-page docs after tagging" | Medium | 10min |
| 7 | AGENTS.md gotcha: patch-precision `go` floors are copied verbatim into consumers by `go get`/tidy (the dashboard inherited 1.27.1 twice now) | Medium | 10min |
| 8 | README: note `GOTOOLCHAIN=auto` for hosts trailing Go 1.27 in the install/compat section (a real consumer hit this — this session's first go get) | Medium | 5min |
| 9 | Decide enforcement vs. checklist for release doc hygiene: flake app `release-check` vs. documented manual grep [O] | Medium | 30min–2h |
| 10 | Re-verify at next release that pkg.go.dev renders the fixed README (the §f10 grep must move pre-tag) | Medium | 2min |
| 11 | Run dashboard's full gates (not just tests) after its push, if it has lint/format gates | Medium | 10min |
| 12 | Sweep dashboard repo for docs/comments still citing the old pseudo-version or v0.1.3 (only go.mod was bumped) | Low | 15min |
| 13 | Re-run the touched handler/eval benchmarks to confirm the `evaluateBounded` extraction is noise-level (it is a single call on an already-microsecond path) | Low | 20min |
| 14 | Record cross-project lesson in crush-config `references/lessons.md`: "release tags freeze docs — staleness grep is a PRE-tag step" | Low | 10min |
| 15 | Record lesson: "consumer verify with GOTOOLCHAIN=auto when project floor > host go" | Low | 5min |
| 16 | Add the "revisit RegisterRoutes/Option extraction at N=4 packages" trigger note next to the accepted-clone rationale | Low | 5min |

**Still owner-gated (unchanged, drafts/evidence ready)**
| # | Item | Impact | Effort |
| - | ---- | ------ | ------ |
| 17 | Post samber/do#318 comment (re-verify scope.go line numbers on master first) [O] | High | 5min |
| 18 | Owner decision: auditlog `DetailedHealthRecorder` (reverses ADR-004; evidence in TODO_LIST) [O] | High | decision |
| 19 | Publish the v0.1.1/v0.1.2 announcement (draft ready since 09-04) [O] | Medium | 15min |
| 20 | If auditlog is declined: mark the row permanently not-planned | Low | 5min |

**Carried from the 21-01 report (unharvested §f — still open)**
| # | Item | Impact | Effort |
| - | ---- | ------ | ------ |
| 21 | Owner decisions 1–6 of the 21-01 §f (SourceStatuses, errors.Join, etc.) as the v0.5.0 candidate set [O] | High | decision |
| 22 | HARVEST both §f lists (21-01 + this report) into TODO_LIST/ROADMAP once you pick — two reports' worth of brainstorm is now entombed | High | 30min |
| 23 | Re-verify ROADMAP: v0.4.0 candidates rows shipped today; retarget the section | Medium | 15min |
| 24 | Verify no remaining "unreleased federation/v0.3.0 vehicle" language anywhere (AGENTS.md history mentions were fixed for header only — a full grep for "unreleased" hasn't run this session) | Low | 10min |

**Ecosystem / upstream**
| # | Item | Impact | Effort |
| - | ---- | ------ | ------ |
| 25 | Track nixpkgs `go_1_27` bumps vs. the 1.27.1 floor (floor stays valid when nixpkgs moves forward; risk only if nixpkgs trails — it currently matches exactly) | Low | ongoing |
| 26 | When posting #318: cite the dashboard's `timedScreenshotRecorder` as shipping prior art (verify the draft already does) | Medium | 5min |
| 27 | Check whether do's per-service-timing landed upstream before re-posting #318 wording | Medium | 10min |

**Hygiene / nice-to-have**
| # | Item | Impact | Effort |
| - | ---- | ------ | ------ |
| 28 | Decide a policy for post-tag doc-only edits: full gates vs. flake-check-only (document whichever is chosen) | Low | 10min |
| 29 | Add scratch-file pattern to tooling ignore so trashed-file diagnostics stop reappearing (or always `lsp_restart` after trash — lesson already known) | Low | 5min |
| 30 | Consider a repo-local `scripts/pre-release-check.sh` equivalence note (go-release skill references one; this repo's equivalent is `nix run .#gates`) — document, don't duplicate | Low | 10min |
| 31 | Run `art-dupl -t 2/-t 3` once for a deeper clone pass (this session used your `-t 1`; lower thresholds find subtler clones) | Low | 20min |
| 32 | Re-check the 21-01 report's §f items 31–36 (testing ideas) for anything the dedup touched | Low | 10min |
| 33 | Extend `docs-health` VERIFY: the README compatibility table was wrong for ≥2 releases — a periodic claims-vs-reality audit would have caught it | Medium | 1h |
| 34 | Announcement cadence decision: announcements exist for v0.1.x only; v0.2.0/v0.3.0/v0.4.0 shipped without published announcements [O] | Medium | decision |
| 35 | Batch the go-health + dashboard announcements into one post if you prefer fewer publications [O] | Low | decision |
| 36 | Add "dashboard bump" as an explicit checklist row in the go-release flow (it's currently tribal knowledge from this repo's AGENTS.md consumer paragraph) | Low | 10min |
| 37 | Grep go-health docs for remaining "v0.1.2"/"v0.1.3" claims (FEATURES consumer row fixed; others may exist — not swept this session) | Low | 10min |
| 38 | Consider tagging GitHub Release notes with the CHANGELOG anchor link (`#v040---2026-09-22`) for deep links | Low | 5min |
| 39 | Verify the v0.4.0 tag message renders well in `git tag -n99` / GitHub tag view (annotated message was written by hand) | Low | 2min |
| 40 | If a 4th package ever appears: extract the shared RegisterRoutes shape (trigger note, see #16) | Low | — |

*(Stopped at 40 grounded items rather than padding to 50 — items 41–50 of the 21-01 report's nice-to-have block remain valid and are carried by #22's harvest.)*

## g) Three questions I cannot figure out myself

1. **v0.4.1 now or fold into v0.5.0?** The tag froze a README that claims Go 1.26+ (false) and a CI Go version (1.26.7) that contradicts the page next to it. A quick v0.4.1 (doc-fix + `go 1.27` directive normalization) cleans the public record within a day; folding into v0.5.0 leaves the false claims on pkg.go.dev for weeks. Which do you want — and if v0.4.1, should it contain anything else, or stay minimal?
2. **Dashboard push policy:** that repo is ~6 commits ahead of origin, including today's v0.4.0 bump (suite green locally). Should I push it and watch its CI as part of this work, or do you batch/push that repo yourself (and if so, is there a reason it lags — deploy-on-push concerns?).
3. **Enforcement vs. discipline for release-doc hygiene:** do you want an enforced docs-lockstep check (flake app that fails when README's stability line ≠ latest tag or its CI-Go claim ≠ the flake's Go pin — the openapi-lockstep precedent), or is a written pre-tag checklist in AGENTS.md sufficient? I can build either; I can't decide how much machinery you want around your release process.

---

*Written per your explicit format instruction (Markdown at `docs/status/`), which overrides the status-report skill's HTML default. Not harvested into TODO_LIST/ROADMAP — you said WAIT FOR INSTRUCTIONS, and §f-22 tracks the harvest once you pick.*
