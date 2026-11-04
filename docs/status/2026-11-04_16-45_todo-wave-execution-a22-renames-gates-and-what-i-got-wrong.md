# TODO-wave execution: backlog cleared, A22 renames shipped, gates hardened — and what I got wrong

**Date:** 2026-11-04 16:45 CET · **Session window:** ~06:45–08:40 CET (work), report written 16:45
**Scope:** this report covers ONE session's run against the 2026-10-28 TODO_LIST. Eight hours passed between the work and this report; other sessions ran in between (observed: dashboard commits through 08:38, TODO_LIST now 119 lines). Nothing here was re-researched — observations are from this session plus the timestamp check.
**Verification verdict:** full test suite, race suite, `nix flake check`, `docs-check`, `fuzz-registry` and the lint gate green at session end. NOT run this session: `coverage-gate`, `vulncheck`, `.#security`, `.#interop`, `.#ci-emulation` — see §e1 and §f3.

## a) Fully done (verified)

1. **pkg.go.dev render for v0.8.1 verified.** Full README renders (TOC, samples, troubleshooting), complete doc index including the freshness/federation API, MIT, Published Oct 27 2026. The "not in the latest version" string in raw HTML was proven to be page chrome (it appears on the bare module URL, which by definition resolves to latest; the proxy enumerates v0.8.1 as newest). TODO row closed with evidence.
2. **`Response.Clone` nil-Checks godoc** added (types.go:180). The pinned test (`TestResponse_CloneIsIndependent`) already existed from an earlier session — only the sentence was owed; CHANGELOG says so explicitly instead of claiming the test.
3. **`WithCriticalService[T]` recorder-renames-keys godoc** added (service_name.go) — and the claim ("Start fails with `ErrUnknownCriticalService` naming the raw do key") was verified against `validateCriticalNames`'s batch-based implementation BEFORE writing it.
4. **`tools/fuzz-registry-check.sh` banner + `EXIT=` line** — added, executed, verified output and exit code. The 2026-10-11 "gate output tails lie" lesson applied to the last silent gate.
5. **Fuzz corpus promoted:** 3,660 coverage-guided inputs from `/mnt/buildcache/go-build/fuzz` into per-package `testdata/fuzz/` (root 1,041 / aggregate 234 / checks 965 / federation 1,411 / internal/merge 17), basename-deduped. Every entry verified to load and pass against current signatures (`go test -run Fuzz ./...`).
6. **FEATURES inventory lockstep gate** added to `docsDriftCheck` (flake.nix): every exported func/type/const/var across root, aggregate, checks, federation — 101 symbols — must be named in FEATURES.md (GNU `\b` word-boundary). First run caught 28 unnamed symbols (handler methods, Status constants, checks sentinels, aggregate/federation constructor errors, `WithClient`, `InstanceID`/`InstanceIDBrand`/`NewInstanceID`, recorder interface methods, …); all folded into their owning rows. Negative-tested: bogus symbol drifts, real symbol matches. Notably `Name` traced to `func (InstanceIDBrand) Name()` — a legit exported method, not extraction noise.
7. **Design note `docs/check-severity-override-design.md`** written for the PMA/go-taskqueue ask: `WithInformational(names...)` as a third contribution tier (excluded from roll-up fold + startup latch, additive `"severity":"informational"` row field, panic rule stays fail-closed), alternatives rejected with rationale (incl. why `Off` must not absorb it), wire/merge/rollout/validation/testing sections, demand-gated decision request. Registered in docs/INDEX.md.
8. **Disk triage to the unprivileged limit:** `/mnt/buildcache` has 99G free again (53% used, inodes 28%); zero kernel I/O/USB errors on the SanDisk SDSSDA240G in every retained boot; persistent journal vacuumed to 5 boots since 2026-11-02 so the 2026-10-11 reboot cause is no longer determinable; retained boots end abruptly with no shutdown sequence (power-loss/hard-reset shape); mount runs `data=writeback,commit=120` — stale/garbage cache content after such a reset is EXPECTED, a plausible mechanism for gosec's "cache entry not found" class. Root residue (SMART, offline e2fsck, possible `data=ordered` remount) recorded.
9. **Four cross-repo defects recorded** as TODO_LIST rows with fix paths. The go-appkit exec-bit row was deleted after verification: the script is already `-rwxr-xr-x` (fixed upstream since Oct 19) — stale defect, not a done fix of mine.
10. **Concurrent-writer claim preflight adopted:** rule added to AGENTS.md Gotchas (two duplicate golangci-LSP bullets merged into one to stay under the 220-line cap: 219/220), practiced before every edit batch — it caught one genuinely-stale read (FEATURES.md mid-daemon) and one false alarm (my own `%H:%M` time-style hiding a yesterday date) — both investigated before editing, not edited blind.
11. **A22 step 3 — `SanitizeResponse` → `CoerceValidUTF8`:** canonical function extracted, `SanitizeResponse` kept as a working alias with a `// Deprecated:` paragraph; all in-repo write seams (handlers.go, aggregate, federation) switched; ARCHITECTURE file-map + api fence list BOTH names (the lockstep demanded the alias too — first docs-check run after the rename caught my one-sided fence edit); FEATURES row updated; CHANGELOG `### Deprecated` with migration snippet. Deprecation policy followed except its README clause — see §d5.
12. **A22 step 4 — `Check.Since` → `StatusSince`:** Go-field clean cut, JSON `"since,omitzero"` frozen and verified; doc pass across 15+ files (ARCHITECTURE, DOMAIN_LANGUAGE glossary, INDEX, adoption-matrix, cache-freshness, check-metadata incl. its code fence, federation-design, panic-recovery, response-ownership, synthetic-check-overlay, naming-integrity left as the historical record, AGENTS ×2, FEATURES ×3, my own design note); CHANGELOG breaking entry with migration snippet. Prepositions and `time.Since` correctly spared.
13. **Lint-gate reds fixed on sight** in `docs/drafts/finding1_repro/main.go` (not my change, but it red the gate): `WriteHeader(200)` → `http.StatusOK`, gosec G112 (ReadHeaderTimeout), mnd (named constant, house pattern). Took three iterations — each finding surfaced the next one.
14. **Dashboard drain e2e assertion added** (`federation_test.go`, `TestFederation_ReadyDrainsTo503WithFlag`): readyz → 503 with `"shutting_down":true` (decoded-JSON style per house convention), healthz stays 200. Build + vet + federation family green in that repo; committed by its daemon (cfba93d → 3e198be).
15. **TODO_LIST hygiene:** completed rows deleted (live in CHANGELOG), verification rows marked DONE with evidence, drain row corrected from "hub comment still says the method does not exist" (stale — the wiring shipped earlier today) to the verified state.

## b) Partially done

1. **Federation drain follow-through:** the wiring itself was done by an earlier session TODAY (I verified, not trusted: `fed.MarkShuttingDown()` in the ctx.Done branch, `drainObservation` hold, stale comment replaced). My addition is the handler-level assertion the draft asked for. What does NOT exist: a true process-level SIGTERM test (held SSE client, real signal). The ctx.Done branch is the same code path, so the gap is representational, not functional.
2. **Disk triage:** unprivileged part complete; SMART (`sudo smartctl -a /dev/sdb`) and offline `e2fsck -n` (needs unmount) are root-gated — recorded BLOCKED with exact commands.
3. **Cross-repo defects:** recorded; the company-master-data failing test needs its own session in that repo; the doanalyzerv2 LICENSE needs an owner licensing call.
4. **Concurrent-writer preflight:** adopted as manual discipline + practiced; no tooling enforces it yet.
5. **"All gates green":** true for what ran (suite, race, flake check, docs-check, fuzz-registry, lint), but the full `.#gates` sweep was never executed end to end this session — `coverage-gate`, `vulncheck`, `.#security`, `.#interop`, `.#ci-emulation` did not run. The lint-gate reds I fixed were found by the lint subset only.

## c) Not started (all deliberately gated, none silently dropped)

1. `WithInformational` implementation — demand-gated per the design note; needs owner + a named first adopter.
2. `WithCriticalServices(...ServiceName)` typed-identity cut — BLOCKED on owner go/no-go (unchanged).
3. Probe/Prober/Source/Remote lexicon reconciliation — BLOCKED (unchanged).
4. go-taskqueue → `NewChecks` port — BLOCKED (API-shape decision above).
5. Announcements (v0.5.1/v0.7.0/v0.8.x), samber/do#318 comment, issue-#5 close comment, issue-#7 annotation, dashboard#14 — all drafts ready, all owner acts; untouched by design.
6. syserr transfer to go-error-family; BuildFlow upstream issues (three) — BLOCKED, need verify-before-filing.
7. golangci LSP integration fix/disable — owner call; I merged its duplicate AGENTS bullets but did not touch the integration.
8. Release v0.9.0 — the breaking window is loaded (InstanceID strong type from the prior session + my two renames + deprecation) but cutting a release is the owner's push authority.

## d) Totally fucked up (owned, in order of badness)

1. **My sed rename broke the build on first try.** `\bSince\b` → `StatusSince` also rewrote `time.Since(...)` (7 call sites) into the non-existent `time.StatusSince`. I had mapped `.Since` selector reads but not qualified-call forms. Caught by the immediate build, fixed with a targeted second pass. The right tool was `lsp_restart` + `lsp_rename` (gopls) — I fell back to sed because the LSP connection was closed and did not even attempt the restart.
2. **I shipped a regex feature the sandbox grep does not have.** `[[:<:]]/[[:>:]]` word boundaries: GNU grep 3.12 rejects them ("Invalid character class name"), so the first real docs-check run spewed errors and false DRIFTs. I tested the spelling only AFTER wiring the gate. The \b fallback was known-compatible — I should have picked it first, or tested `[[:<:]]` against the exact binary before committing to it.
3. **I overstated the daemon's push behavior.** Mid-session I inferred "the daemon auto-pushes" from origin being at b51a73d. At 16:45 the truth: go-health is 6 commits ahead of origin, dashboard 6 — NOTHING after b51a73d was ever pushed. Consequence: the dashboard cannot resolve a pseudo-version containing my renames until someone pushes; my adoption row says "release-sequenced" but must also say "push-gated". Corrected here and worth fixing in the row.
4. **A silent sed no-op in the doc pass** (`omitzzero` typo) — `sed` exits 0 on zero matches, so the failure was invisible in the chain; only my fallback pattern + post-verification grep caught it. Silent-no-op sed in `&&` chains is a trap I walked into knowingly and should design against (grep-for-expected-result after every transform).
5. **Deprecation-policy README clause not satisfied and the deviation not recorded anywhere until now.** Policy item 2 says the README repeats the deprecation. README never documents `SanitizeResponse`, so I judged the clause vacuous and skipped it — but I never wrote that reading down, which makes it look like an oversight. Either amend the policy for never-README-documented symbols or add one README line. Owner call.
6. **Five-plus wasted edit round trips** ("read the file before editing", "old_string not found") because I kept using bash sed/grep output as my read instead of the View tool. No damage, but sloppy mechanics.
7. **One false alarm I raised on myself:** FEATURES.md mtime "10:15" looked like a concurrent writer with a skewed clock; it was my own `--time-style='+%H:%M'` hiding a yesterday date. The preflight did its job (I investigated instead of editing) — but the investigation was needed only because my preflight output was ambiguous.

## e) What we should improve

1. **Gate coverage honesty:** "all gates green" must mean `nix run .#gates` exit 0 (the canonical sweep), not "the gates I happened to run". Run the full sweep before any claim, or name exactly which gates ran (this report now does).
2. **Test tool-version assumptions before wiring them into gates** (the grep `[[:<:]]` lesson; candidate for crush-config `references/lessons.md` — cross-project, commit-only per policy).
3. **Prefer semantic renames; treat sed as plan B with a collision checklist** (`pkg.Ident(` qualified forms, string literals, comments). If gopls is down, restart it first — sed should be the documented fallback, not the reflex.
4. **Never put a bare sed transform in a `&&` chain without a post-grep assertion** of the expected result count.
5. **Record policy deviations in the moment** (a line in the CHANGELOG entry or the TODO row), not in a status report eight hours later.
6. **Update stale comments in the same commit as renames** — `version_test.go:86` still says "the fuzz-found class SanitizeResponse guards for"; my rename pass touched code and doc-comment mentions but did not sweep for the old function name in comments.
7. **Consumer repos deserve their own gate run:** for the dashboard I ran build + vet + targeted tests, but never its formatters/lint config. A cross-repo change is not "verified" until the consumer's own quality surface says so.
8. **Cross-repo state lines in handoffs must include push state** (ahead-count), not just commit presence — a local commit does not exist for a consumer's `go get`.
9. **The preflight rule should print unambiguous output** (full timestamps) so it cannot cause the false alarm it exists to prevent.

## f) Up to 50 things to do next (ordered by leverage, not severity)

**Release window (unblocks the most):**
1. Owner: push the 6 go-health commits + 6 dashboard commits (or authorize push).
2. Cut v0.9.0 through the manual flow: CHANGELOG cut → `nix run .#pre-tag -- v0.9.0` → annotated tag → `gh release create` → proxy/pkg.go.dev verify.
3. Run the FULL `nix run .#gates` on the release tree (coverage-gate, vulncheck, security, interop included) + `nix run .#ci-emulation`.
4. Re-verify the pkg.go.dev render for v0.9.0.
5. Bump dashboard pin to the release; apply the adoption checklist (`status.go:490,576`, `trend.go:59`, `concurrency_test.go:232,236`, `status_bench_test.go:23`, comments `evidence.go:205` + `webhook_payload_test.go:24`, optional `dashboard.go:209` → `CoerceValidUTF8`); build + test + the dashboard's own lint/format.
6. Re-run `nix run .#skew` post-release; record 16/16 or document the dashboard posture.
7. Update the AGENTS.md status header line at release (219/220 lines — in-place only).
8. Update the TODO_LIST adoption row to "push-gated + release-sequenced" (correct my §d3 wording).

**Unblock decisions (each unblocks a row):**
9. Owner ruling: publish or retire the owed announcements (v0.5.1/v0.7.0/v0.8.x) — six reports archivable after.
10. Owner go/no-go on typed identity → unblocks `WithCriticalServices(...ServiceName)` + `tools/servicename-scan.sh`.
11. go-taskqueue direction: `WithInformational` vs pass-detail field vs drop the notes (interacts with item 12).
12. Owner + first-adopter call on `WithInformational` (design note ready); if approved: implement option + fold exclusion + `"severity":"informational"` field + Start validation (tier conflict) + openapi.yaml + merge invariants + fuzz harnesses in one commit.
13. doanalyzerv2 LICENSE decision (mirror MIT?) + add the file.
14. golangci LSP integration: repair or disable (owner call; I can prepare either).

**Fixes small enough to just do:**
15. Fix stale comment `version_test.go:86` (still names SanitizeResponse).
16. Investigate dashboard `concurrency_test.go:232` (`check.Since != nil` — nil comparison on what should be a value type; either a dashboard-owned struct or a latent bug).
17. Amend docs/deprecation-policy.md: what item 2 (README repetition) means for symbols the README never documented.
18. Repo-size sanity after the corpus promotion: `git count-objects -vH`, one CI-job timing comparison.
19. Sweep for remaining "SanitizeResponse" mentions in comments/docs repo-wide (audit grep; I know of one in version_test.go).
20. ROADMAP: cross-reference the severity-override design note from the `WithTransitionHook` raw idea.
21. A real process-level SIGTERM drain test for the hub (held SSE client, actual signal) — completes draft 2's literal wording.
22. Add the preflight rule to tooling (a tiny script or Crush hook printing full timestamps), retiring the manual discipline.
23. docs/status archive pass: today's + 2026-10-28 reports that are now fully resolved → `docs/status/archived/` per the resolution rule.
24. docs-health run to harvest this report's §b/§c residue into TODO_LIST.
25. Short `.#fuzz` re-run post-rename to re-baseline the coverage-guided state (cheap, confirms corpus value).

**Bigger, demand-gated (from ROADMAP, unchanged by this session):**
26. OTEL spike (separate example module, real OTLP collector) — top verified gap from the SystemNix assessment.
27. Integration index `docs/integrations.md` round-up (Gatus, node_exporter, Prometheus, OTEL, textfile).
28. `AwaitReady` cache-aware poll interval (design sketched, demand-gated).
29. `Response.TotalLatencyMs` as float64 (sub-millisecond precision).
30. `healthtest` consumer helper package (needs design note + owner decision).
31. Extend `docs/openapi.yaml` to federation endpoints.
32. Aggregate handler HTTP benchmarks; throttled-live contention benchmark; `-count=N` race stress in CI.
33. Aggregate fuzz seed corpus from golden fixtures; combine aggregate fuzz with throttle/cache modes.
34. Hub-drain orchestration helper (only if the dashboard wiring proves error-prone — it has not).
35. Formalize aggregate's sources-only drain stance as an ADR.
36. `WithTransitionHook` design note (distinct from severity override; needs severity mapping + overlap + panic sections).
37. Per-check severity override — IF the owner green-lights item 12, this is the same work tracked once (do not double-plan).
38. Federation `WithEvaluationHook` parity (consumer-need gated).
39. Non-nix CI matrix job (plain `go test`) for honest OS/arch statement; arm64 runner evaluation.
40. Raise per-push fuzz budget above 10s/target if CI cost allows.
41. README install note: `GOTOOLCHAIN=auto` for hosts trailing the 1.27 floor.
42. Dependabot/Renovate extension to flake inputs + action SHAs.
43. Promote erraudit/doanalyzerv2 to CI if they ever go public.
44. Dashboard-side system-inventory composition (ADR-007's named composition point) — dashboard repo work.
45. System-inventory federation exporter (only if a consumer asks).

**Hygiene:**
46. Record the GNU grep 3.12 `[[:<:]]` removal in crush-config `references/lessons.md` (by commit in that repo, per policy).
47. Give `tools/doanalyzerv2` the same doc-map treatment ARCHITECTURE got, if it ever leaves private status.
48. Decide whether `docs/drafts/` modules belong in lint scope at all (they red the gate today; policy is unstated).
49. Consider a corpus cap/LFS policy if testdata grows past a threshold (not now — 15MB total is fine).
50. Next session's first action: re-read this report's §d list before touching anything.

## g) Questions only the owner can answer

1. **Release:** do you want v0.9.0 cut now (InstanceID break + `StatusSince` cut + `CoerceValidUTF8` deprecation + corpus are all staged in `[Unreleased]`), or should the window absorb more first? Everything downstream (dashboard pin bump, skew re-verification, pkg.go.dev re-check) is blocked on this plus the 6+6 unpushed commits.
2. **The buildcache mount:** is `data=writeback,commit=120` on `/mnt/buildcache` a deliberate throughput tradeoff or an accident of setup — and may I have root to run `smartctl`/offline `e2fsck` and, if you agree, remount `data=ordered`? I cannot determine health past the OS-visible layer without it.
3. **go-taskqueue:** for its BLOCKED port — should the direction be my `WithInformational` proposal, a pass-detail channel on `Check`, or dropping the informative green rows? The severity note partially covers them, but the pass-detail gap is a separate API decision I cannot make from here.
