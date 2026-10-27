# Status report — supplier wave, post-release hardening pass

**Date:** 2026-10-27 20:17 CET
**Repo:** `~/projects/go-health` · **Branch:** `master`, in sync with `origin/master` at `608e98a`
**Released:** `v0.8.0` at `4af20ba` (local tag == `refs/tags/v0.8.0^{}` on origin; proxy-served)
**Author:** Crush (this session) · **Predecessor report:** [`2026-10-27_18-57_fleet-needs-supplier-wave-concurrent-session.md`](2026-10-27_18-57_fleet-needs-supplier-wave-concurrent-session.md)
**Scope of this report:** only what this session did and noticed. No new research.

---

## 0. The one-paragraph truth

The release shipped (`v0.8.0`, resolving from the proxy, smoke-tested from a clean module), the
gate sweep is green three times over, coverage is 99.1% and the hardening tail that the plan had
parked is done. What is **not** done is everything that touches a second repository: the two
bridges and the four pattern-E consumers still pin v0.7.0/v0.5.1, so the wave's value is still
sitting behind six `go.mod` edits that are yours to authorise. Along the way I also did real
damage to one doc comment, misattributed three released features in the CHANGELOG, and nearly
moved a published tag. Those are written up in §d without softeners.

---

## a) FULLY DONE (verified in this session)

| #  | Work                                                                                              | Evidence                                                                                                                                                                                                 |
| -- | ------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1  | `v0.8.0` release verified end to end: tag on origin at `4af20ba`, module resolves                 | `git ls-remote --tags origin` → `4af20ba refs/tags/v0.8.0^{}`; clean-module `go get github.com/larsartmann/go-health@v0.8.0` then `go run .` printed `pass true true`                                    |
| 2  | Gate sweep green, run three times (after the lint fix, after the doc pass, after the final edits) | `nix run .#gates` → `all gates green`, `GATES_EXIT=0`; eight gates (test-race, vet, lint `0 issues`, vulncheck, security, fuzz, docs-check, `nix flake check` incl. treefmt + openapi-lockstep)          |
| 3  | `gocognit` finding gone (the one red left by the previous session)                                | peer extracted `assertResponseRoundTrip`; `golangci-lint run ./...` → `0 issues`                                                                                                                         |
| 4  | `FuzzHealthz_CodeMatchesBody` written, run, registered                                            | 20 s campaign, 219 143 execs, `PASS`; added to both `.#fuzz` and `.#fuzz-long` (`flake.nix:342,357`)                                                                                                     |
| 5  | Benchmarks for every new accessor + the drain fan-out                                             | `BenchmarkSourceStatuses` (federation), `BenchmarkHealthz`, `BenchmarkReadinessHandler_DrainingScaling` 1/2/4/8, `BenchmarkAggregateSourceStatuses`; numbers + method recorded in FEATURES "Performance" |
| 6  | Race test for the drain boundary                                                                  | `TestSourceStatuses_ConcurrentWithMarkShuttingDown` — 8 readers × 4 rounds vs concurrent `MarkShuttingDown`, `-race` clean                                                                               |
| 7  | Healthz startup overlay extracted and pinned (`applyStartupLatchOverlay`)                         | 4 subtests in `federation/federation_internal_test.go`; federation coverage 95.0% → 98.1%, total 98.6% → 99.1%                                                                                           |
| 8  | `docs/federation-healthz-design.md` flipped to IMPLEMENTED + implementation record                | status table line 6; new §"Implementation record (2026-10-27)" naming the hub-drain addition the design had not covered                                                                                  |
| 9  | Both federation accessors documented in `docs/federation-design.md`                               | new sections `SourceStatuses`: the per-remote roll-up (line 118) and `Healthz`: the single-endpoint view (line 143), plus 2 API-list lines                                                               |
| 10 | Research doc §5b addendum: the six features that landed after §5 was written                      | `docs/research/2026-10-27_fleet-consumer-needs.md` §5b, each row with file + closing need + test                                                                                                         |
| 11 | v0.8.0 version-skew table recorded                                                                | §"Skew against the v0.8.0 tag": `minor-stale: 15, warnings: 1` from `tools/fleet-skew.sh v0.8.0`                                                                                                         |
| 12 | `tools/fleet-repos.txt` corrected to 15 direct + 2 bridges                                        | header explains the two deliberate absences (forgejo-hosted company-master-data, CV's nested modules)                                                                                                    |
| 13 | README failure-mode → battery table                                                               | four rows (disk, memory, http, database) + sentinel/composition note, in "Batteries"                                                                                                                     |
| 14 | Five-pattern consistency across consumer docs                                                     | `docs/consumer-verification.md` now says "Five implementation patterns" with pattern E integrated, not appended                                                                                          |
| 15 | TODO_LIST reconciled: 7 rows flipped to DONE/DRAFT/READY with evidence                            | drain follow-through (3), consumer-needs wave (3), v0.8.0 row                                                                                                                                            |
| 16 | Issue #7 annotation drafted in Lars's comment register                                            | `docs/drafts/issue7-annotation.md` — one terse paragraph per finding                                                                                                                                     |
| 17 | BuildFlow `dprint-format` unblocked (the only failure a human could repair)                       | `nix develop -c dprint fmt` → 17 files, whitespace-only; `buildflow -s dprint-format` → `✔ detect`                                                                                                       |
| 18 | AGENTS.md documents the dprint repair gap                                                         | gotcha paragraph, still 219/220 lines                                                                                                                                                                    |
| 19 | CHANGELOG `[Unreleased]` corrected to the truth after the tag                                     | added drain-scaling bench, the overlay extraction, the gosec scope fix, the dprint repair                                                                                                                |
| 20 | Plan progress log appended with 10 executed milestones                                            | `docs/planning/2026-10-27_19-18_supplier-wave-…md` §11                                                                                                                                                   |
| 21 | Everything committed and pushed                                                                   | `46b86fe`, `608e98a`; `master...origin/master` with no ahead/behind                                                                                                                                      |

---

## b) PARTIALLY DONE

| # | Item                                          | What exists                                                                                                             | What is missing                                                                                                                                                                                                                                                                                                                   |
| - | --------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | **BuildFlow full mode**                       | Ran it; 7 failures → 4 → 3 after the dprint repair and the peer's gosec fix. Root causes identified per step            | Still red: `govalid-generate` and `license-check` (documented upstream gaps in `.buildflow.yml`), `golangci-lint [tools/doanalyzerv2]` (go-directive floor, blocked). I proved the last one passes under `GOWORK=off GOTOOLCHAIN=auto` manually, which means the step env is the fault, not the code — and I did not fix the step |
| 2 | **Fleet adoption**                            | `fleet-skew.sh` runs, manifest correct, before-picture recorded                                                         | Zero consumer bumped. 15 repos minor-stale by design; the release is installable and nobody has installed it                                                                                                                                                                                                                      |
| 3 | **Fuzz registry integrity**                   | Both lists carry the new target; comment in `flake.nix` claims a pre-push check diffs the registry against `^func Fuzz` | That check does not exist in this repo (`scripts/` absent — only `tools/`). The comment is aspirational; a new target can still be added to code and forgotten in the flake                                                                                                                                                       |
| 4 | **Pressure on the two open design questions** | M15 lifecycle docs done, ADR-008 written, migration snippets written                                                    | The _decisions_ the plan gates on (non-critical panic → 503 as released behavior; BuildFlow-vs-gates authority) are still owner questions; I documented them, I did not settle them                                                                                                                                               |
| 5 | **Coverage "baseline"**                       | 99.1% measured with a per-function breakdown                                                                            | No before/after _delta_ table per package; the number I first wrote into TODO_LIST (98.6%) was pre-change and I had to correct it to 99.1%                                                                                                                                                                                        |
| 6 | **Documentation of the two-plan collision**   | Master plan carries a coordination note; this report names it                                                           | The plan still tells the next session to "execute the peer file's T-IDs"; the peer file is at v0.8.0 too and its remaining T-items are the same owner-gated consumer work. Two plans, no single owner decision                                                                                                                    |

---

## c) NOT STARTED

| #  | Item                                                                                                               | Why it was not started                                                                                                                                  |
| -- | ------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1  | Bump `cqrs-htmx/health` v0.7.0 → v0.8.0; delete its `RecorderChain`; widen `NewDashboard`                          | Cross-repo write + push; recorded as owner-gated in TODO_LIST                                                                                           |
| 2  | Bump `go-appkit/health` v0.5.1 → v0.8.0; flip its warn pin; add `NewDetailedProbe`                                 | Same gate                                                                                                                                               |
| 3  | Adopt cache primitives in dnsblockd / DiscordSync / cv / go-taskqueue                                              | Same gate                                                                                                                                               |
| 4  | Typed-critical sweep (`WithCriticalService[T]`) in CV, KeyHolderAI, nsfw-classifier, DiscordSync                   | Same gate                                                                                                                                               |
| 5  | `NewWithRecorder` adoption in PMA + Zlota44; `VersionHandler` in timesheets                                        | Same gate                                                                                                                                               |
| 6  | Dashboard wave (`SourceStatuses` cards, `Healthz`, SIGTERM drain E2E)                                              | Separate repo                                                                                                                                           |
| 7  | Breaking-window renames (`ServiceName` cut, `SanitizeResponse` → `CoerceValidUTF8`, `Check.Since` → `StatusSince`) | Explicitly owner-gated (BLOCKED row) and now post-tag, so they belong to v0.9.0                                                                         |
| 8  | Lexicon reconciliation + per-check severity override design note (M22)                                             | Tail-tier, depends on the renames                                                                                                                       |
| 9  | Publishing queue: issue #7 comment, issue #5 close, samber/do#318, v0.1.1/v0.1.2 announcement                      | All four drafts exist; posting is your act                                                                                                              |
| 10 | Version-skew CI workflow                                                                                           | Blocked in TODO_LIST on push authority; with v0.8.0 published it would now legitimately run red (15 stale) — which is the point, but it needs your call |
| 11 | LSP integration fix/disable                                                                                        | Standing owner call, untouched this session                                                                                                             |
| 12 | Coverage-threshold CI job                                                                                          | Policy call (CONTRIBUTING says 99.7%, measured 99.1%)                                                                                                   |
| 13 | `DetailedHealthRecorder` in samber-do-auditlog                                                                     | Blocked; the row's premise was already disproven                                                                                                        |
| 14 | Per-check severity override design note                                                                            | Depends on M22                                                                                                                                          |

---

## d) TOTALLY FUCKED UP (my own errors, with the repair)

| # | What I broke                                                                                                                                                                                                                                                                                                                                                                                        | How bad                                                                                                                                    | Repair                                                                                                                                                                                |
| - | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | **I severed a doc comment.** Inserting `BenchmarkAggregateSourceStatuses` used `// BenchmarkAggregateHandlers measures…` as the anchor and replaced it instead of prepending, so `BenchmarkAggregateHandlers` shipped for ~10 minutes with an orphaned comment body starting `// merge + UTF-8 sanitize…`. A peer's commit then "fixed" it by deleting a blank line, which made it look intentional | Low blast radius, high embarrassment: it is exactly the "read before you write" failure AGENTS.md forbids                                  | Restored the first line (`aggregate/aggregate_benchmark_test.go:95`); lint is green                                                                                                   |
| 2 | **I nearly moved a published tag.** I had planned to retag `v0.8.0` at the final commit, believing the peer's tag was local-only. It had already been pushed (`refs/tags/v0.8.0` → `4af20ba`)                                                                                                                                                                                                       | Would have violated the repo's own "no tag moves" rule on a public module, permanently                                                     | Caught by re-reading `git ls-remote` before touching anything; no tag command was run. The cost of the near-miss is that my hardening work is _unreleased_, not that history is dirty |
| 3 | **I misattributed released features.** My first `[Unreleased]` CHANGELOG entry claimed `FuzzSourceStatuses_Invariants`, `FuzzHealthz_CodeMatchesBody` and three benchmarks as new. All of them are inside the `v0.8.0` tree                                                                                                                                                                         | A consumer reading it would think a patch release was pending that does not exist                                                          | Verified with `git show v0.8.0:…                                                                                                                                                      |
| 4 | **I wrote a stale number into TODO_LIST** (`total 98.6%`) from a profile taken before the overlay test, instead of re-measuring after                                                                                                                                                                                                                                                               | Small, but it is the same class of error as quoting a log tail instead of an exit code                                                     | Re-measured (99.1%), corrected the row today                                                                                                                                          |
| 5 | **I ran formatters and the linter by hand before loading the BuildFlow skill**, which explicitly forbids duplicating its steps                                                                                                                                                                                                                                                                      | Process, not damage; it cost nothing here because the commands agreed                                                                      | Loaded the skill, then used `buildflow -s dprint-format` for the verify step                                                                                                          |
| 6 | **My first gate run failed on treefmt** and I briefly assumed pre-existing drift; it was my own unformatted test files plus the peer's                                                                                                                                                                                                                                                              | One wasted gate cycle (~5 min)                                                                                                             | `nix fmt`, then green                                                                                                                                                                 |
| 7 | **I edited the same repo as a second live agent** for the second session in a row: three of my edits were rejected with "modified since last read", my CHANGELOG prep was overwritten once, and this repo now carries two plans and two status reports for one wave                                                                                                                                 | The real damage is duplicated thinking: I re-derived facts the peer had already established, and the plan-of-record question is still open | Nothing lost (auto-commit daemon captured every state); documented in §g as the first question                                                                                        |

---

## e) WHAT WE SHOULD IMPROVE

1. **One writer per repo.** A lock file, a worktree per agent, or an explicit hand-off. Two agents
   on one Go module is survivable because git is forgiving, but it doubles the analysis cost and
   leaves the repo with competing plan documents.
2. **Verify the remote before reasoning about release state.** I spent time planning a tag that
   already existed on origin. `git ls-remote --tags` is one command and belongs at the start of any
   release-adjacent task.
3. **The release should not have shipped mid-hardening.** `v0.8.0` was tagged at `4af20ba` while the
   hardening pass was still landing minutes later; the result is that the release notes describe a
   tree that is now six commits behind and my work needs a version of its own.
4. **Machine-check the doc drifts I fixed by hand.** Two of them (unpadded markdown, `[Unreleased]`
   claiming "Nothing yet." while commits exist past the tag) are trivially detectable and cost
   nothing to enforce.
5. **The fuzz-registry check the flake advertises does not exist.** Either implement it or delete
   the claim — an unverifiable assertion in a config comment is worse than no comment.
6. **BuildFlow step env is a repeat offender.** The `doanalyzerv2` lint failure is not a code
   problem; it is `GOWORK`/`GOTOOLCHAIN` not being pinned in the step. This is the third documented
   instance of the same class in this repo.
7. **Coverage tells two stories.** 99.1% total looks excellent; `federation.WithClient` at 0% means
   the documented escape hatch for custom transports has never been executed by a test.
8. **Plan hygiene.** Consolidate. A 726-line master plan, a 469-line peer plan, and now three status
   reports for one 24-hour wave is more documentation than the work it describes.

---

## f) NEXT 50 THINGS (ordered by value/effort)

**Release + fleet (the only tier with real customer value)**

1. Decide whether post-tag hardening becomes `v0.8.1` or rides to `v0.9.0`.
2. Bump `cqrs-htmx/health` to v0.8.0 and run its suite.
3. Delete cqrs-htmx `RecorderChain`, call `health.ChainRecorders`; add a duration-survival test.
4. Widen `cqrs-htmx/health.NewDashboard` to the dashboard `Prober` interface so aggregate/federation render.
5. Bump `go-appkit/health` to v0.8.0 and run `go test ./health`.
6. Flip `go-appkit/health/probe_test.go` warn → fail for the panic pin (same commit as #5).
7. Retire the "silently dropped recorder" godoc in go-appkit (it now fails at `Start`).
8. Add `appkithealth.NewDetailedProbe` so the bridge can expose durations.
9. dnsblockd: replace the zero-timestamp guard with `CachedResponseFresh`, keep the 15 s policy.
10. dnsblockd: replace hand-rolled `maps.Copy` with `Response.Clone`.
11. DiscordSync: replace the readyz snapshot cache with `CachedResponseFresh`/`EvaluateAndCache`.
12. DiscordSync: `WithCriticalService[T]` for the labeled services; label table reads `DoServiceName`.
13. cv: `CacheAge`/`CachedResponseFresh` in `internal/health/handlers.go`, `Clone` for the overlay.
14. go-taskqueue: map `queueProber` onto `NewChecks` + `WithRefreshInterval(0)` + live throttle.
15. go-taskqueue: delete the re-implemented `dashboard.Prober`.
16. CV / KeyHolderAI / nsfw-classifier: drop `samber/go-type-to-string` from health wiring.
17. PMA + Zlota44: `NewWithRecorder` instead of `health.New(do.New(), …)`.
18. timesheets: swap its local version handler for `health.VersionHandler`.
19. company-master-data (forgejo, manual): bump + verify, then re-run `fleet-skew.sh` → expect `minor-stale: 0`.
20. Add the skew run to CI only after #2-#19 land, so its first run is green.

**Repo hardening (verifiable, no owner input)**

21. Test `federation.WithClient` — it is 0% covered and it is the documented transport escape hatch.
22. Cover the remaining `fetch` branches (92.6%): oversized body, non-200 body drain, bad URL.
23. Cover `Probe.Healthz` 93.8% and `aggregate.Healthz`'s nil-checks path (same defensive class I just pinned in federation).
24. Cover `evaluateStartup` 90.9%, `VersionHandler` 91.7%, `tracker.stamp` 95.2%.
25. Cover `DetailedHealthRecorderFunc.RecordHealthCheckWithContext` (0%) — the adapter that projects durations away.
26. Implement the fuzz-registry parity check the flake comments claim exists.
27. Add a `docs-check` rule: `[Unreleased]` must not read "Nothing yet." when `HEAD != latest tag`.
28. Add a `docs-check` rule: the federation/aggregate API list in `docs/federation-design.md` must match exported methods (it had drifted until today).
29. Race-test `Healthz` under concurrent drain (I only did `SourceStatuses`).
30. Fix the `.config/metadata.yaml` diff that dprint produced — confirm it is formatting-only, not semantic.
31. Decide BuildFlow-vs-`.#gates` authority and write it into CONTRIBUTING.
32. Fix the `golangci-lint [tools/doanalyzerv2]` step env (`GOWORK=off`, `GOTOOLCHAIN`) instead of documenting it a fourth time.
33. File the BuildFlow upstream issue: `dprint-format --fix` does not rewrite unpadded tables (verified twice today).
34. File the second BuildFlow upstream issue: full-mode spawn env ignores `GOTOOLCHAIN=auto` for `license-check`.
35. Add `dprint` to treefmt so `nix fmt` covers markdown — kills the whole drift class.
36. Investigate the two BuildFlow regression verdicts the run reported (`go-auto-upgrade` +190%, `go-version-auto-configure` +450%).
37. Coverage-threshold CI job at the measured floor, or delete the claim from CONTRIBUTING.
38. Fix or disable the golangci LSP integration (fourth session it has lied: it still reports `gci` on a green file).
39. Resolve the `tools/doanalyzerv2` go-directive floor, then drop `tools/bin` wrappers and the `GOTOOLCHAIN` backstop.
40. AGENTS.md is 219/220: move the per-file map into a generated `docs/ARCHITECTURE.md` before the next edit trips the cap.

**Decisions + publishing (your call, drafts ready)**

41. Post the issue #7 annotation (`docs/drafts/issue7-annotation.md`).
42. Post the issue #5 close comment (`docs/drafts/issue5-close-comment.md`).
43. Post the samber/do#318 duration comment.
44. Publish the v0.1.1/v0.1.2 announcement.
45. Write v0.8.0 GitHub release notes with the consumer-impact paragraph (additive vs behavior change vs nothing breaking).
46. Confirm the non-critical-panic → 503 grading as shipped behavior (ADR-008) or schedule a revert.
47. Settle `ServiceName` cut go/no-go, then run `tools/servicename-scan.sh` and fix the 10 break sites.
48. `SanitizeResponse` → `CoerceValidUTF8` alias + deprecate; `Check.Since` → `StatusSince` with JSON frozen.
49. Consolidate the two plans and three status reports into one canonical plan + one report, archiving the rest.
50. Design the per-check severity override (PMA/go-taskqueue ask) with an explicit reject-or-accept note.

---

## g) THREE QUESTIONS I CANNOT ANSWER MYSELF

1. **Who is the second session?** A peer agent has been editing this repo all evening (it committed
   the release, re-tagged `v0.8.0` at `4af20ba`, and pushed master + tag while I was mid-hardening).
   I cannot tell from the filesystem whether that is you driving another window, a scheduled agent,
   or two Crush instances you did not mean to start. It determines whether I should stand down, take
   a lock, or keep going — and whether the two plans should be merged by me or by that session.

2. **Was `v0.8.0` at `4af20ba` the release you wanted, or should my post-tag hardening be its own
   version?** The tag predates the overlay extraction, the drain-scaling benchmark, the gosec scope
   fix and the dprint repair. Publishing `v0.8.1` for them is defensible (they are test/doc/tooling
   only) and cheap; folding them into `v0.9.0` is equally defensible. I cannot choose because it
   changes what six consumer repos bump to.

3. **Do I have authority to write and push to the six consumer repositories** (cqrs-htmx/health,
   go-appkit/health, dnsblockd, DiscordSync, cv, go-taskqueue)? `TODO_LIST.md` records this as
   blocked on your answer, and every remaining high-value item in §f sits behind it. With v0.8.0
   installable, I can start the moment you say yes.

---

## Appendix — numbers from this session

| Measurement                                   | Value                                                                                | Command                                          |
| --------------------------------------------- | ------------------------------------------------------------------------------------ | ------------------------------------------------ |
| Coverage, total                               | 99.1% (was 98.6%)                                                                    | `go test -coverprofile … && go tool cover -func` |
| Coverage, by package                          | health 99.2, aggregate 100, checks 100, federation 98.1, merge 100                   | same                                             |
| Fuzz, `FuzzHealthz_CodeMatchesBody`           | 219 143 execs / 20 s, no findings                                                    | `go test -fuzz … -fuzztime=20s`                  |
| Bench, `BenchmarkReadinessHandler_Draining`   | ~113 µs · 12.3 kB · 110 allocs (median of 3)                                         | `-benchmem -count=3`                             |
| Bench, drain scaling                          | 67 / 172 / 311 / 628 µs at 1/2/4/8 remotes                                           | same                                             |
| Bench, `BenchmarkSourceStatuses` (federation) | 77 / 207 / 389 / 1000 µs at 1/2/4/8, ~86 allocs per remote                           | same                                             |
| Bench, `BenchmarkAggregateSourceStatuses`     | 576 / 587 / 886 / 876 ns, 336 B · 2 allocs flat                                      | same                                             |
| Fleet skew at v0.8.0                          | `minor-stale: 15, warnings: 1`                                                       | `tools/fleet-skew.sh v0.8.0`                     |
| BuildFlow full mode                           | 7 failures → 3 (`govalid-generate`, `license-check`, `golangci-lint [doanalyzerv2]`) | `buildflow --fix --build-mode=full`              |
| Gates                                         | `all gates green`, exit 0 (×3)                                                       | `nix run .#gates`                                |
