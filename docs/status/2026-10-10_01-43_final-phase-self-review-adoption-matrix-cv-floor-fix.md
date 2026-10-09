# Session self-review + full status — final-phase execution, adoption-matrix re-grounding, CV floor fix

**Written:** 2026-10-10 01:43 CEST · **Session covered:** 2026-10-09 ~22:05–22:50 (execution) + this review pass.
**Scope of this report:** exactly what this session did and noticed. No new research beyond
verifying my own claims. Format note: owner demanded `.md` at `docs/status/` — honored over
the HTML default.

**State at write time:** go-health tree clean, `master` == `origin/master` (daemon commit
`708be7f`, ~23:4x, re-padded the markdown tables I had hand-aligned — cosmetic only,
content identical). CV clean, ahead 6, unpushed (owner question ①). CI on the pushed
go-health HEAD: green (`37987766928`, 6m6s).

---

## a) FULLY DONE

1. **Resume verification** — both repos: the deferred G1 push had already landed during the
   pause (origin == HEAD); daemon commits accounted (`954ae91`, `c1363c5`);
   `tools/doanalyzerv2/go.mod` directive flapped `1.27.1`→`1.27`→`1.27.1` (benign toolchain
   churn — and fresh evidence for the "not a local lever" TODO row).
2. **A11 closed as scoped** — CV `nix flake check`: all 7 checks passed (treefmt, golden
   help output, mermaid gate, go-modules FOD, …). Note: this closes what A11 *defined*; see
   d)1 for what the definition missed.
3. **Adoption-matrix corrections, every claim source-verified before writing:**
   - `WithGETOnly` in KeyHolderAI is **tests only** (`cmd/keyholderai/health_probe_http_test.go`;
     non-test hits are vendored library docs) → retirement unblocked; naming-integrity.md
     removal row updated; new v0.6 TODO row added.
   - `WithShutdownGracePeriod` row was **entirely ghost** — both 10-02 entries call
     go-daemon's same-named `SocketServerOption` (go-daemon `socket.go:126`;
     project-discovery-daemon via `godmn.` prefix; neither requires go-health; bank-sync's
     `daemon.` hit is the same collision). Row flipped to "—", folded into E5 with a
     keep-verdict (it is the two-phase-drain opt-in, just fixed in `[Unreleased]`).
   - "fir" → file-and-image-renamer, "PMA" → projects-management-automation (grep-able real
     names); dated "2026-10-09 re-verification" section records method + deltas.
4. **TODO_LIST refresh** — executed rows deleted per lifecycle (dashboard suite, consumer
   train, CV bump, bench re-verify, makezero contract); header evidence note; version-skew
   CI flipped TODO→BLOCKED with the honest reason (pins local-unpushed → day-one red);
   owner-blocked evidence cells refreshed; two v0.6 staging rows (merge port, WithGETOnly
   removal).
5. **Plan stamped EXECUTED** — `docs/planning/2026-10-09_14-49_*`: per-task scorecard for
   all 35 A-tasks with evidence pointers; gates G1–G5 ledger (all executed; G2 for own-repo
   filings go-appkit#25 + cqrs-htmx#31); tally 29 DONE / 4 owner-staged / 2 pending.
6. **Gates + push + CI** — `nix fmt` 0 changes; `nix run .#gates` ALL GREEN; closing report
   committed (`2c6757c`), pushed; remote CI verified green.
7. **nix-hash-fix triage (CV)** — root cause found: BuildFlow's repair targets the file that
   textually *mentions* `vendorHash` (the refresh app's script in `apps.nix`) instead of the
   definition sites in `nix/packages.nix`, so the fix no-ops. Documented in CV's canonical
   `docs/agents/nix-deployment.md` with the working path (`nix run .#refresh-vendor-hash`)
   and the upstream fix (match assignments, not mentions).
8. **CV go-1.27 floor completion** — 8 member modules bumped 1.26.x→1.27 via `go mod edit`;
   per-module `go vet` green; full pre-commit hook green (vet+build all modules,
   test-count guard); committed `e425ffa60` (doc note landed via daemon `68b54c382`).

## b) PARTIALLY DONE

1. **Adoption-matrix re-verification was row-local.** I re-derived only the rows I touched;
   the core row's "all 14 direct consumers" count was NOT re-derived — and it disagrees with
   consumer-verification.md's "15 direct" from the same survey date (see e)6/e)7).
2. **E5's "document in FEATURES as protective" is half-done.** The matrix now carries the
   ghost verdicts; FEATURES.md carries no protective/zero-adopter wording for
   `WithLiveThrottle` (pre-existing gap) or `WithShutdownGracePeriod` (new). Verified by
   grep tonight.
3. **The 22:32 closing report predates the session's real end** — it omits the nix-hash-fix
   triage and the CV floor fix (both happened after it was written). This report supersedes.
4. **Inherited:** A07 (BuildFlow full-mode 95% ceiling) and A08 (budget re-review behind it)
   remain partial/pending; recorded in TODO_LIST.

## c) NOT STARTED (owner-gated or windowed — deliberately untouched)

- **① Consumer pushes** — 8 green bumps + crush-config, all local-unpushed. Unlocks A19.
- **② Publishing acts** — samber/do#318 comment + v0.5.x / v0.1.x announcements (staged,
  citation-current). A14–A16.
- **③ Release vehicle** — v0.6.0 vs v0.5.2 for `[Unreleased]` (grace fix, SourceStatuses,
  errors.Join, test hardening). Design docs lean v0.6.0.
- **v0.6 implementation queue** — `internal/merge` primitive + port, federation
  `Prober.Healthz()`, ServiceName rename execution, staged renames (design-first done).
- **Fuzz corpus harvest** — interesting inputs live in GOCACHE only; cherry-pick after the
  fuzz-long CI window (2026-10-12).
- **golangci LSP disable/fix decision** — owner call; panel lied all session again.
- **gosec unpin + go override drop** — watch items (flake.nix comments track them).
- **CV `TestAdoptionPolicyWordingAcrossHomes`** — pre-existing red on clean HEAD; owner /
  CV session (flagged in the 21:57 report, unchanged tonight).

## d) TOTALLY FUCKED UP

1. **A11 was declared complete on an incomplete floor — my own defect from the prior
   session, discovered only by accident tonight.** The go 1.27 move bumped root go.mod,
   go.work, and the nix pins but left 8 of 9 member modules at `go 1.26.x`. The first
   hook-running commit then failed `go vet` in `chat` (json APIs require the 1.27 language
   version). My "all go pins move together" lesson enumerated 4 pin sites and never ran
   `find . -name go.mod`. Worse: the prior session's "all committed, green" claims rested on
   daemon commits that bypass pre-commit hooks — the latent breakage was invisible until I
   committed with hooks tonight. Fixed (`e425ffa60`), but the honest sequencing is: shipped
   partially-broken, discovered late, repaired by luck.
2. **multiedit padding failure on TODO_LIST** — 3 of 4 edits applied; I reconstructed the
   table's `old_string` from memory instead of copying the freshest view (the Status column
   separator was 7 dashes, I typed 6). Caught by post-edit verification; one retry burned.
3. **"fir" repo confusion** — hunted for a directory named `fir` (does not exist), then used
   an ambiguous grep fallback (`|| echo "no require"`) that fires for *missing files* as well
   as *no match* — nearly mis-concluded the consumer didn't exist. Resolved to
   file-and-image-renamer only via usage evidence (`pkg/injector/providers.go:135`).
4. **Sloppy placeholder in a permanent doc** — the planning-doc scorecard says "22:5x" and
   "(22:5x)" where a real timestamp belonged.
5. **Report-before-end** — wrote the closing status report two work items before the actual
   session end (see b)3).
6. **Pushed-unverified-then-verified** — the push landed via admin bypass ("Bypassed rule
   violations", checks still pending); I verified CI green only afterwards. Defensible under
   the ratified G1 posture, but the sequence is worth naming: the remote briefly pointed at
   an unverified HEAD.

## e) WHAT WE SHOULD IMPROVE

1. **Floor moves enumerate every go.mod** — `find . -name go.mod -not -path '*/vendor/*'`,
   not a memorized pin list. Fold into the AGENTS "go pins move together" lesson (go-health
   AGENTS has the CV-specific variant; CV's own docs should carry it too).
2. **Treat hook-bypassing daemon commits as unproven** — a repo whose session commits all
   went through the daemon has NOT exercised its pre-commit hooks; the first deliberate
   commit must expect surprises (vet first if any floor/dep moved since the last hook-run
   commit).
3. **Copy exact text from the freshest View for table edits; never reconstruct padding.**
   (Cost me a round trip tonight; this is the documented failure mode.)
4. **Disambiguate grep fallbacks** — "no match" vs "no such file" need different echoes;
   a silent conflation nearly produced a false adoption claim.
5. **Session reports at true session end** — or explicitly marked interim.
6. **One count, one source of truth for the fleet** — adoption-matrix says 14 direct
   consumers, consumer-verification.md says 15, both dated 2026-10-02. The matrix should
   either cite the verification doc's number or the two must be re-derived together.
7. **FEATURES must carry the protective/ghost framing** the matrix verdicts reference
   (E5's own instruction), otherwise the verdict and the feature inventory drift apart.
8. **Consider a machine guard** — a workspace directive-equality check (all member go.mod
   `go` directives agree with the root) in CV's go-change-gate or upstream in BuildFlow's
   gomod-check; tonight's breakage is exactly the class it would catch.
9. **Real timestamps in permanent docs** — no "22:5x" placeholders.

## f) Next up to 50 (grouped by gate; ★ = unblocked now)

**Owner answers first (everything below cascades from these):**
1. ① Authorize consumer pushes (8 repos + crush-config) — then push and…
2. …draft + land A19 `fleet-skew.yml` (push-to-master + weekly + dispatch) and verify a green run.
3. ② Authorize publishing: post samber/do#318 comment (draft citation-current).
4. ② Publish v0.5.x announcement (channels checklist staged).
5. ② Publish v0.1.1/v0.1.2 announcement (draft since 09-04).
6. ③ Decide vehicle: cut v0.6.0 vs v0.5.2 from `[Unreleased]` (full go-release train either way).

**Fleet / verification:**
7. ★ Reconcile the 14-vs-15 direct-consumer count (adoption-matrix vs consumer-verification.md).
8. ★ Add protective/zero-adopter wording for `WithLiveThrottle` + `WithShutdownGracePeriod` to FEATURES.md.
9. ★ Cross-check consumer-verification.md patterns (A–D) against tonight's qualifier findings (bank-sync is a non-consumer; confirm the inventory has no other go-daemon-collision entries).
10. ★ Full adoption-matrix re-derivation (standing §e5 item, now with two ghosts known).
11. Push CV (after ①) and re-run its full gate on a hook-run commit path.
12. CV: fix or deliberately re-pin `TestAdoptionPolicyWordingAcrossHomes` (owner/CV session).
13. CV: add the member-module directive-equality leg to `scripts/go-change-gate.sh` (see e)8).

**v0.6 window (design-first already done):**
14. Implement `internal/merge` primitive per the re-grounded A23 sketch.
15. Port `aggregate` merge onto the primitive; collapse duplicated fuzz properties.
16. Port `federation` merge onto the primitive; collapse its fuzz duplication.
17. Implement `federation.Prober.Healthz()` per the accepted design (+ unit table).
18. Execute ServiceName typed-identity migration per docs/servicename-design.md (scanner exists).
19. Execute staged renames per naming-integrity.md (`SanitizeResponse`→`CoerceValidUTF8`, `Since`→`StatusSince`).
20. Remove `WithGETOnly` via the deprecation-policy checklist (now unblocked — zero production adopters).
21. AwaitReady cache-aware poll interval — only if a concrete consumer need appears (A30).

**Hardening (TODO_LIST rows):**
22. Panicking `WithEvaluationHook` on the refresh-loop path: recover-vs-document decision + pinning test (probe.go:661-663).
23. Fix or disable the stale golangci LSP integration (owner call; discipline note exists).
24. Restore `buildflow --fix --build-mode=full` to exit 0 (BuildFlow upstream fan-out env bug; then A08 budget re-review).
25. ★ go-health AGENTS: extend the go-pins lesson with the find-all-go.mods rule (e)1).

**Hygiene / watch:**
26. Cherry-pick interesting fuzz inputs into `testdata/fuzz` seeds after the 2026-10-12 fuzz-long window (B112).
27. Investigate the Disk B/op outlier (48 vs 32 B) from the A20 run.
28. gosec unpin + go override drop when their tracked conditions land.
29. BuildFlow upstream: nix-hash-fix definition-site matching (finding documented in CV tonight).
30. BuildFlow upstream: full-mode fan-out env fix (the A07 ceiling).
31. ★ doanalyzerv2 directive: re-check after the next samber-linter release (flap re-confirmed tonight).
32. ★ Replace "22:5x" placeholders in the planning-doc scorecard with real timestamps (next doc touch).
33. Owner decision: coverage-threshold CI job (BLOCKED row).
34. Owner decision: samber-do-auditlog `DetailedHealthRecorder` (BLOCKED row, ADR-004 tension).

**Small doc debts from tonight:**
35. ★ Mark the 22:32 report as superseded-by-this-report at its top (one line).
36. ★ adoption-matrix: cite consumer-verification.md as the count source once 7) lands.

## g) Questions I cannot answer myself

1. **① Push authority:** may I push the 8 verified-green consumer bumps (+ crush-config)?
   They are all local-unpushed; A19 is blocked behind them.
2. **② Publishing:** do you post the samber/do#318 comment and the two announcements
   yourself, or do I execute the staged drafts under a new explicit go-ahead?
3. **③ Release vehicle:** v0.6.0 or v0.5.2 for the current `[Unreleased]` set (grace fix +
   SourceStatuses + errors.Join + test hardening)? Design docs lean v0.6.0.

---

**Verdict:** the final phase is executed and verified end-to-end; the plan is stamped with an
honest scorecard; two real defects were found and fixed (one of them mine from the prior
session). Everything still open is owner-gated, windowed, or recorded with evidence in
TODO_LIST. **Waiting for instructions.**
