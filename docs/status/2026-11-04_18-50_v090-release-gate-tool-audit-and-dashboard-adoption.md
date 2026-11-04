# Status Report: v0.9.0 release, the gate-tool audit it forced, and the dashboard adoption

**Date:** 2026-11-04 ~18:50 CET
**Session scope:** "Time for a new release?" — full release execution (assess → cut → gates → tag → publish → verify → adopt), plus the three CI failures and four latent gate bugs the release forced into the open.
**Repo state at writing:** go-health `master` @ `ea2df5b`+ (a PARALLEL session is actively refactoring on top of the release — 27-file test reorganization, sentinel moves; their commits land while this session runs). Tag `v0.9.0` @ `2d8dc4a`, pushed, GitHub Release live. go-health-dashboard `master` @ `b443480` (adoption), pushed.

---

## a) The release — DONE and verified end to end

| Step               | Evidence                                                                                                                                                                                                                                                                                                                                                                                           |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Version determined | v0.9.0 (MINOR): two wire-compatible breaking Go-API changes in 0.x (`InstanceID` branded type, `Check.Since`→`StatusSince`) + `SanitizeResponse` deprecation + the evidence wave                                                                                                                                                                                                                   |
| CHANGELOG cut      | `[Unreleased]` → `## [v0.9.0] - 2026-11-04` with preamble; empty placeholders; compare links retargeted (`1b19bea`)                                                                                                                                                                                                                                                                                |
| Docs sync          | README stability line, AGENTS.md status line (219/220 cap held), doc.go carries no version (nothing to sync)                                                                                                                                                                                                                                                                                       |
| Pre-tag sweep      | `nix run .#pre-tag -- v0.9.0` → **EXIT=0**: test-race, vet, lint, coverage-gate (99.9% total / 100% root+aggregate+checks+merge / 99.3% federation), vulncheck ("No vulnerabilities found."), security (0 issues, private GOCACHE), fuzz (9 targets), fuzz-registry, interop (fixture green against published httputil), docs-check, flake check, fleet skew (all consumers at v0.8.1 pre-release) |
| CI emulation       | See §c — partial, raced a parallel refactor; per-app stripped-PATH verification substituted                                                                                                                                                                                                                                                                                                        |
| Tag                | Annotated, SSH-signed, on the release commit; `git show v0.9.0:go.mod` verified                                                                                                                                                                                                                                                                                                                    |
| Push + CI          | master pushed (daemon beat the explicit push — it pushes now, see §e); CI **green 5/5** on the release commit (`41867742776`) AND on the tag ref (`41868854706`)                                                                                                                                                                                                                                   |
| Proxy              | `go list -m -versions` lists v0.9.0; clean-dir `go get @v0.9.0` + `go mod download` exit 0 (sum.golang.org verified)                                                                                                                                                                                                                                                                               |
| Consumer proof     | throwaway module compiled AND ran `NewInstanceID`, `InstanceID.Get()`, `CoerceValidUTF8` against the published version                                                                                                                                                                                                                                                                             |
| pkg.go.dev         | https://pkg.go.dev/github.com/larsartmann/go-health@v0.9.0 live, marked Latest, README rendered with the new sanitization section                                                                                                                                                                                                                                                                  |
| GitHub Release     | https://github.com/LarsArtmann/go-health/releases/tag/v0.9.0 with curated notes (breaking-first, migration snippets, consumer-impact table)                                                                                                                                                                                                                                                        |

## b) What the release forced open — all fixed before tagging

1. **Three CI jobs on master were ALREADY red** (coverage floor, gosec, lint) — all three traced to the new `docs/drafts/finding1_repro/main.go`: its uncovered `main` counted 0.0% into the library's coverage total (98.9% vs the 99.5% floor) and its raw `http.Server{}` + `WriteHeader(200)` tripped gosec G112 + mnd. The draft's lint findings were fixed in unpushed commits; the coverage gate now excludes `docs/` packages from the floor arithmetic while still compiling and running them (`443dc63`).
2. **`pre-tag` was unsatisfiable in every state of its own dance** — its `docs-check` demanded the PREVIOUS release's docs in the git-tag view and the PENDING one in the store-snapshot view; the fix makes the newest CHANGELOG heading compete with the newest git tag by version (`8859089`).
3. **`pre-tag`'s section check never matched the repo's own heading format** — required `## [0.9.0]`, every release ever written reads `## [v0.9.0]` (`80cbe07`). The app shipped 2026-10-27 and had never run green end to end; this session was its first real use and it took TWO bug fixes before it could pass.
4. **The interop fixture module was stale** — `go mod tidy` had never re-run after go-branded-id became a direct dependency; the gate died with "updates to go.mod needed" before compiling (`2d8dc4a`).

## c) The gate-tool audit (ci-emulation's gift, still racing the parallel session)

`nix run .#ci-emulation` (stripped PATH) exposed that gate apps were RUNNING ON BORROWED TOOLS — the flake sandbox got awk implicitly from stdenv, the dev shell lent the rest from the host PATH:

- `fuzz-registry`: needed `gawk` (parse) and `diffutils` (diff, comm) — a missing `diff` reads as "files differ" because its stderr is silenced (`8ce243c`, proven with byte-identical inputs)
- `security`: ran `mktemp`/`rm` with only gosec + a Go toolchain declared
- `coverage-gate`: script uses `grep`/`sed`, wrapper probes root via `git` — none declared
- `recorder-audit`: needed `sed`
- `openapi-lockstep`: `mktemp`/`rm` without coreutils

All declared in `fc93e82`. **Verified per-app under a fully stripped PATH**: openapi-lockstep, recorder-audit, coverage-gate, security — all EXIT=0. The FULL `ci-emulation` sweep could not be completed green this session: the parallel session's refactor moved `ErrRecorderNotApplicable` between files mid-run and the sweep graded a torn tree (`ErrRecorderNotApplicable redeclared` — the tree builds clean the moment their commit lands). The sweep is left for the next settled window; `nix run .#pre-tag` on the next release will run it against that tree anyway.

## d) Dashboard adoption — DONE on my commit's scope

`b443480`: pin `v0.8.2-0.20261104…` → `v0.9.0`; `Check.Since` → `StatusSince` everywhere health.Check is read (the status-report checklist MISSED the struct-literal keys — the compiler found nine more in test fixtures); `SanitizeResponse` → `CoerceValidUTF8` + comment; the repo's first-ever lint run exposed one pre-existing `nlreturn` finding, fixed. Jobs green: Build, Lint (0 issues), Vulnerability Scan, Version const, race tests locally.

**Remaining reds on dashboard master are NOT the adoption's:**

- FEATURES test-count guard: my commit froze the daemon's mid-edit `347` (actual 348) — the daemon's working tree already carries 348; its next commit greens this.
- Browser (runtime CSP) `TestCaptureREADME_Screenshot`: the new `assertCaptureContent`/`waitForJSTo` guard introduced in the `dae5251..37ddb50` window fails on CI — it failed identically on the docs-only commits between, so it is not the adoption. A rerun reproduced it (deterministic, not flaky). The guard times out waiting for the page text including the SERVER-RENDERED title, so `/health` never rendered its shell under CI's Chrome 157 — needs body-text dumping on timeout to diagnose (the guard decodes nothing on the deadline path).
- Hygiene templ drift: failed the rerun but passed the original run on the SAME commit — non-deterministic or a rebase artifact; parallel session's lane.

**Not mine to touch:** a parallel session owns both trees right now (go-health 27-file refactor at 18:29, dashboard doc edits in flight) — colliding further risks the documented torn-tree/daemon-commit class.

## e) Corrections to standing beliefs

- **The daemon PUSHES.** Every go-health commit today reached origin within minutes (my explicit push printed "Everything up-to-date"). AGENTS.md gotcha updated. The dashboard gotchas.md (v0.7.0 era) still says "NEVER pushes" — their file, left alone.
- **The daemon rebases.** It pushed my pre-amend `8957684` while I had amended locally to `273f3db`; the pull --rebase turned my amend into a follow-up commit (`8ce243c`). History stayed linear; amend-after-daemon-push is not safe.
- My commit `b443480` froze the daemon's mid-edit FEATURES.md via `git add -A` — the concurrent-writer preflight must EXCLUDE files another writer is mid-editing, not just detect motion.

## f) Open items

1. Fleet wave: 15 consumers a minor behind v0.9.0 (weekly skew alarm is the design, TODO_LIST row added). Only go-health-dashboard needed code edits; the rest are pin-only.
2. go-health full `ci-emulation` green on a settled tree (§c).
3. Dashboard Browser guard diagnosis + Hygiene determinism (§d, parallel session's lane).
4. Owner questions still open: `/mnt/buildcache` `data=writeback,commit=120` intent + root access for SMART; go-taskqueue direction (`WithInformational` note is PROPOSED demand-gated).
