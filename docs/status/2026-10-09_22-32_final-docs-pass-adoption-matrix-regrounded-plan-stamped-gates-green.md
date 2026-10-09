# Final docs pass — adoption matrix re-grounded, plan stamped EXECUTED, gates green

**Session:** 2026-10-09 ~22:05–22:35 · **Trigger:** owner resume directive after the 21:57 pause.
**Scope:** the unblocked remainder of the 21:57 report's "Exact next steps" §1–2 (final phase).
Everything owner-gated stayed untouched; the three questions (① consumer pushes, ② publishing,
③ v0.6.0-vs-v0.5.2 vehicle) are still open and still gate A19 / A14–A16 / the release cut.

## Resume verification (before any work)

- go-health tree clean; master == `origin/master` at `c1363c5` — **the deferred G1 push already
  happened during the pause** (owner or daemon). Nothing was re-pushed blindly; this session's
  new commits push below after gates.
- Daemon landed the 21:57 report + doc edits during the pause (`954ae91`, `c1363c5`), as
  predicted. `tools/doanalyzerv2/go.mod` flapped `1.27.1`→`1.27`→`1.27.1` across those two
  commits (toolchains rewriting it) — net zero, and fresh evidence for the TODO_LIST
  doanalyzerv2 row's "not a local lever" claim.
- CV: clean, ahead 5, unpushed (as expected — consumer pushes are owner question ①).

## A11 fully closed

`nix flake check` in CV: **all checks passed** (7 flake checks incl. treefmt, golden help
output, mermaid docs gate, go-modules FOD). Combined with last session's green `nix build`,
A11 is done end-to-end.

## Adoption-matrix corrections — the planned three, plus one bigger finding

Every correction was verified against consumer source before writing (vendor-excluded grep +
go.mod require + call-site qualifier):

1. **`WithShutdownGracePeriod` row was entirely ghost — worse than planned.** The planned
   correction was "go-daemon is not a consumer". Verification showed go-daemon defines its own
   `SocketServerOption` named `WithShutdownGracePeriod` (`socket.go:126`), and the row's OTHER
   entry (project-discovery-daemon, `godmn.` prefix) also calls go-daemon's option; neither
   repo requires go-health. Zero real adopters → row flipped to "—", verdict = ghost, folded
   into E5 alongside `WithLiveThrottle` (keep: it is the two-phase-drain opt-in and just got
   the dead-window fix in `[Unreleased]`).
2. **`WithGETOnly` in KeyHolderAI is tests only** (`cmd/keyholderai/health_probe_http_test.go`;
   non-test hits are vendored library docs). Zero production adoption → retirement unblocked,
   no nudge needed; naming-integrity.md removal row updated.
3. **Abbreviations expanded to real repo names:** "fir" → file-and-image-renamer (verified
   consumer at v0.5.1, `pkg/injector/providers.go:135`), "PMA" → projects-management-automation.
   The 10-02 matrix's "fir"/"PMA" were ungrepable shorthand.

- A dated "2026-10-09 re-verification" section records method + findings, so the 10-02 survey
  provenance stays intact and the delta is auditable.

New TODO_LIST row added (newly unblocked): remove `WithGETOnly` at the v0.6 window via the
deprecation-policy checklist.

## TODO_LIST + planning doc

- Executed rows deleted per lifecycle (dashboard suite, consumer train, CV bump, bench
  re-verify, makezero contract) with a header note pointing at the 21:57 report for evidence.
- Version-skew CI row flipped TODO→BLOCKED with the reason (consumer pins are local-unpushed;
  the workflow would observe stale pins and run red on day one).
- Owner-blocked evidence cells refreshed honestly (doanalyzerv2 flap re-confirmed); auditlog
  and coverage-threshold rows unchanged (still owner decisions).
- **Plan stamped EXECUTED** on `docs/planning/2026-10-09_14-49_*`: per-task scorecard for all
  35 A-tasks with evidence pointers; gates G1–G5 all executed (G1 push, G2 own-repo filings
  go-appkit#25 + cqrs-htmx#31, G3 CV, G4 v0.5.1, G5 hook-panic). Tally: **29 DONE,
  4 PARTIAL/OWNER-STAGED (A07, A14–A16), 2 PENDING (A08 behind A07, A19 behind ①)** —
  A08 surfaced as the one task the 21:57 tally omitted.

## Gates + push

- `nix fmt`: 0 changes (doc edits were already treefmt-clean).
- `nix run .#gates`: **all gates green** (full chain incl. treefmt-check, docs-drift-check,
  openapi-lockstep, flake check). The LSP panel's stale findings (gocognit/varnamelen/
  typecheck/golines) remain lies — `nix run .#lint` inside gates is the truth source, again.
- Master pushed after gates (G1-ratified). Daemon commits `4f031cb`/`90f2f94` carried the doc
  content; this report commits with attribution.

## Still open (owner)

1. **① Consumer pushes** — 8 green bumps + crush-config, all local-unpushed. Also unlocks A19.
2. **② Publishing** — samber/do#318 comment + v0.5.x/v0.1.x announcements (staged, current).
3. **③ Release vehicle** — v0.6.0 vs v0.5.2 for `[Unreleased]` (grace fix, SourceStatuses,
   errors.Join, test hardening). Design docs lean v0.6.0.
