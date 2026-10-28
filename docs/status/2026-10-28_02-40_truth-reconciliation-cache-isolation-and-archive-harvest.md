# Status: truth reconciliation — the coverage number's toolchain, the cache's lies, and 26 reports audited

**Created:** 2026-10-28 02:40 CET · **Repo:** `github.com/larsartmann/go-health` · **Branch:** `master`
**Session scope:** finish the repo-local residue after the wave-2 self-authorised batch — run the gates that never ran, settle the coverage discrepancy, audit the status-report archive, and verify every claim the plan and the docs still make.
**Tree:** the sweep launched at `053c567` and finished at `78d7727`; a peer session committed throughout (see §d3), so every result below is stated with the tree it covers.
**Verdict:** the tree is green end to end (`GATES_EXIT=0`, ten gates plus `nix flake check`), the coverage discrepancy was a **toolchain artifact and not a regression**, and three of tonight's reds were **the shared build cache lying**, not the code.
**See also:** [`2026-10-28_01-18_…`](2026-10-28_01-18_consumption-wave-gates-shipped-fleet-still-at-zero.md) (peer, mechanism half) · [`2026-10-28_02-02_…`](2026-10-28_02-02_instanceid-strong-type-adoption-and-gates-sweep.md) (peer, strong InstanceID) · [`2026-10-27_23-42_…`](2026-10-27_23-42_todo-wave-decisions-audits-and-vacuous-red-hunt.md) (my prior report)

---

## 0. TL;DR

| Open question at 00:20                                                 | Answer at 02:40                                                                                                                                                               |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Three gates never ran (`.#fuzz`, `.#fuzz-registry`, `nix flake check`) | All three ran and passed; the whole sweep then ran green: **`GATES_EXIT=0`**                                                                                                  |
| Federation is 99.3% or 99.5%?                                          | Both, and neither is a regression: **the same single uncovered statement** counts as 147/148 under go1.27.3 and 206/207 under go1.27.0. Instrumentation granularity, not code |
| Is `.#lint` red because of load or code?                               | Neither — the shared `GOCACHE` was serving half-written std export data. `.#lint` now runs on a private cache (`privateGocache`) like `.#security`                            |
| The archive rule was never applied                                     | All **26** unarchived reports read item by item: **1** fully resolved (archived), **11 clusters** of untracked residue harvested into `TODO_LIST.md`                          |
| `/mnt/buildcache` 100% full blocking gates                             | Freed to 65 GB by other work; the _remaining_ damage is a corrupted host `GOCACHE`, documented and worked around                                                              |
| Is `.#skew` trustworthy?                                              | It could hang forever **and** it passed while reading nothing. Both are now impossible: a 30 s per-call deadline, `unreadable` gated at 0, and a local-checkout mode that needs no API. Real reading of the fleet: **14 minor-stale, 2 in sync** |

---

## a) FULLY DONE

### a1. The full gate sweep, with an exit code that came from the sweep

```
GOWORK=off GOTOOLCHAIN=auto nix run .#gates > /tmp/gates5.log 2>&1
echo "GATES_EXIT=$?" >> /tmp/gates5.log      # GATES_EXIT=0
```

Every gate confirmed individually in the log rather than inferred from the tail:

| Gate            | Evidence line                                                                  |
| --------------- | ------------------------------------------------------------------------------ |
| `test-race`     | five `ok` lines, race detector on                                              |
| `vet`           | silent success                                                                 |
| `lint`          | `0 issues.`                                                                    |
| `coverage-gate` | `coverage-gate: every package and the total meet their floor`                  |
| `vulncheck`     | `No vulnerabilities found.`                                                    |
| `security`      | `Issues : 0`, full SSA across all five packages                                |
| `fuzz`          | all 9 targets `PASS` (root ~1.1M execs, merge ~1.0M, federation three targets) |
| `fuzz-registry` | `OK - 9 targets registered in both fuzz apps`                                  |
| `docs-check`    | `docs-drift-check: OK - README/AGENTS/CHANGELOG/FEATURES in sync with v0.8.1`  |
| `flake check`   | `all checks passed!`                                                           |

The three previously-missing gates were also run standalone first (`.#fuzz`, `.#fuzz-registry` → `EXIT=0`), so a sweep failure could be attributed to a specific gate rather than to the sequence.

### a2. The coverage discrepancy is a toolchain artifact, proven by profile diff

Same source, same single uncovered block, two different denominators:

| Toolchain                                | Federation blocks | Hit | Reported  | Total           |
| ---------------------------------------- | ----------------- | --- | --------- | --------------- |
| go1.27.3 (dev shell + `.#coverage-gate`) | 148               | 147 | **99.3%** | 99.9% (744/745) |
| go1.27.0 (host shell)                    | 207               | 206 | **99.5%** | 99.9% (744/745) |

`grep -v '^mode:' /tmp/nixcov.out | awk '$3==0'` returns exactly one row under both:
`federation/federation.go:403.3,404.1` — the request-construction guard inside `fetch`,
unreachable for any remote `New` accepts. Nothing else changed.

**What was actually wrong was the documentation**, in six places, and one of them was wrong about _which_ statement is uncovered:

- `CONTRIBUTING.md` claimed the uncovered statement was "the documented defensive nil-`Checks` guard in `Healthz`". It is not — it is `fetch`'s request-construction branch. Fixed, with the toolchain explanation inline.
- `AGENTS.md`, `CHANGELOG.md`, `tools/coverage-gate.sh`, `flake.nix`, `.github/workflows/ci.yml` each quoted a bare `99.5%` that would read as a regression the moment someone ran the gate. Each now states the gate's number and why the host reads it differently.

### a3. The shared `GOCACHE` was faking both a finding and a broken toolchain

`.#lint` failed the first sweep with two `typecheck` issues and no real finding:

```
could not import internal/runtime/sys (-: could not load export data:
open /mnt/buildcache/go-build/04/04c8…-d: no such file or directory)
```

`GOCACHE=$(mktemp -d)` builds the same tree green, so the fix is structural: `flake.nix` gained a shared `privateGocache` snippet, and `.#lint`, `.#vulncheck` and `.#security` now all run their package-loading tool against a private `mktemp -d` cache. `.#lint` afterwards: **0 issues**.

The corruption goes further than the gate: a plain host-shell `go build ./...` now dies with

```
runtime/proc.go:14:2: package internal/runtime/exithook is not in std
time/sys_unix.go:12:2: package syscall is not in std
```

while `GOCACHE=$(mktemp -d) go build ./...` → **exit 0** and `nix develop -c go build ./...` → **exit 0**. The host `GOROOT` is itself a module-cache toolchain (`/mnt/buildcache/go-mod/golang.org/toolchain@v0.0.1-go1.27.0…`), whose std is fine — the half-written _cache_ is the whole problem. Recorded as one consolidated AGENTS gotcha; the previous two-symptom bullet was merged into it to stay under the 220-line cap (file at 219).

### a4. The archive rule, applied to all 26 reports

`docs/INDEX.md` defines the rule as resolution-based ("fully resolved reports move to `docs/status/archived/`"), so every unarchived report was read and every claimed-open item was checked against `TODO_LIST.md`/`ROADMAP.md`.

- **Archived (1):** `2026-10-11_19-17_federation-twophase-drain-issue5-gates-green.md` — every item is done or tracked elsewhere (drain wiring → TODO_LIST, issue-#5 comment → Owner Actions with the issue closed, BuildFlow items → §Blocked, ADR/hub-helper/federation-hook parity → ROADMAP).
- **Kept (25):** each has at least one item that lived **nowhere** but inside a point-in-time report.
- **Harvested (11 rows):** the residue was not 25 separate problems but **five recurring clusters** — announcement drafts with no home, BuildFlow upstream filings, the orphaned `syserr` proposal, cross-repo defects found but never recorded, and process/mechanism items (claim preflight, disk health, fuzz-corpus promotion, a gate for "new symbol appears in FEATURES", per-check severity design note, pkg.go.dev verification). All eleven are now rows in `TODO_LIST.md` §"Harvested from the status-report archive audit".

The honest outcome is 1 archive, not 13: most reports were _written_ as handoffs and were never _closed_ as handoffs. That is a process finding, not a paperwork chore.

### a5. Two options with zero adopters are now documented as such

Grepped all **16** fleet checkouts in `tools/fleet-repos.txt`:

```
WithLiveThrottle:          0 repos
WithShutdownGracePeriod:   0 repos
```

Both were `FULLY_FUNCTIONAL` in `FEATURES.md` with no adopter signal — exactly the state `WithGETOnly` was in before it was removed. `FEATURES.md` now carries the marker, and `TODO_LIST.md` carries the **deprecate-or-advertise** decision under the next breaking window. Deliberately not deleted: removal is an owner call inside a breaking window, and the marker is the honest intermediate state.

### a6. `.#skew` could hang forever; it now fails instead

Two runs of `nix run .#skew` were observed today, one of which printed `latest tag: v0.8.1` and then **sat alive for 55 minutes** (a peer session's `.#pre-tag`, which shells out to the same script). An earlier run of mine reported all 16 repos `unreachable` inside a rate-limit window — with `2>/dev/null` hiding the reason.

`tools/fleet-skew.sh` now gives every GitHub call `timeout 30` (overridable via `FLEET_SKEW_TIMEOUT` so the path is testable), which degrades a stall into the `WARN` the script already understands, and the WARN text names the three real causes (removed, renamed, rate-limited, timed out) instead of guessing one.

**Proven, not asserted:** with a fake `gh` that sleeps 30 s and `FLEET_SKEW_TIMEOUT=1`, the script now terminates in **16.0 s** for 16 repos instead of hanging, printing one honest WARN per repo.

### a6b. The bigger finding under it: the gate passed when it learned nothing

While proving the timeout path I ran the script with every API call failing and no `--allow`: **exit 0**, with `minor-stale: 0, warnings: 16 (allowed: 0)`. A fleet where *not one* repo could be read reported success. That is the vacuous-green class this repo has legislated against twice ("a green gate with nothing behind it is worse than no gate") reappearing inside the newest gate.

A peer session rewrote the same script minutes later and solved it better than my draft guard would have: unreadable repos are now a separate `unreadable` counter gated by `--allow-unchecked` (default **0**), and `--source local` reads the fleet checkouts directly so the gate does not depend on the GitHub API at all. Verified end to end:

```
nix run .#skew                                        # source: local, exit 1
minor-stale: 14 (allowed 0), warnings: 0, unreadable: 0 (allowed 0)
OK    LarsArtmann/cqrs-htmx (health/go.mod): v0.8.1
OK    LarsArtmann/go-appkit (health/go.mod): v0.8.1
```

Fourteen minor-stale consumers is the truthful reading of the fleet, and the two `OK` rows independently confirm the bridge re-pin the peer's report claims. The weekly workflow already runs `--allow 14 --allow-unchecked 0`, so the baseline is tolerated and a *new* drift, or an outage, is not.

### a7. Small truthful repairs, each verified

| Repair                                                    | Why it was wrong before                                                                                                                                                                                                                 |
| --------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `tools/doanalyzerv2/LICENSE` added (copy of the root MIT) | The nested module shipped with no license file, so `go-licenses`/BuildFlow's per-module check had nothing to read                                                                                                                       |
| `CONTRIBUTING.md` fuzzing bullet                          | Claimed "all three targets"; the tree has **nine** (and `fuzz-registry` exists precisely to keep that list honest)                                                                                                                      |
| Plan doc execution banner + two errata rows               | The plan said nothing about what had executed; its §2 table has **no M16 row** though §5 verifies M16, and M22 lists "concurrent-writer preflight (T27)" as if it were a deliverable when it is a habit                                 |
| Plan banner reconciled with the peer report               | My first draft said M01–M04 had not started; the peer had already pushed `go-appkit/health` v0.3.0 and re-pinned both bridges to v0.8.1 (verified directly: `go-appkit/health/go.mod` and `cqrs-htmx/health/go.mod` both read `v0.8.1`) |

---

## b) PARTIALLY DONE

### b1. `.#recorder-audit` remains a maintainer-only gate

Still not in `.#gates` (correctly): off-host it would `SKIP` all 16 repos and pass vacuously. No change attempted tonight; it is recorded, not fixed.

### b2. M26 (golangci LSP) — the evidence base got better, the fix still lives in another repo

Tonight produced the strongest explanation yet for the panel's false diagnostics: the same host whose `go build` now fails with `package io is not in std` is the one every `golangci-lint-langserver` invokes. Whether that is _the_ cause or only a co-symptom is not proven, so the disable/repair decision stays open and stays in `crush-config`.

### b3. `fuzz` corpus promotion

A 10-second run reports dozens of "new interesting" inputs that live only in the build cache. The mechanism is real and now tracked; nothing was promoted tonight because corpus additions deserve a deliberate commit, not a sweep byproduct.

---

## c) NOT STARTED

Unchanged from the peer reports and now carried in `TODO_LIST.md` rather than duplicated here: the fleet-adoption half (`M06`–`M09`, `M19`), the evidence table (`M04`), the announcement batch (`M18`), the two BuildFlow filings (`M24`), and the breaking-window renames (`M27`). Every one needs write access to a repository this session may not touch.

---

## d) TOTALLY FUCKED UP

**d1. I repeated the exact mistake my own documentation warns about, within 24 hours of writing it.** `go build ./... 2>&1 | head -3; echo "exit=$?"` printed `root build exit=0` for a **failing** build: `$?` was `head`'s status. The trap is literally a bullet in `AGENTS.md` ("piping through `tail` also eats the exit status"). Every command after that writes the exit code into a file with `> log 2>&1; echo "EXIT=$?" >> log`.

**d2. My first sweep died with a tool call, and took the log with it.** I ran `nix run .#gates` as a background job, an unrelated `job_output` call was interrupted, and the shell was gone — `/tmp/gates3.log` did not exist. Ten minutes of evidence lost, and no way to tell whether anything had failed. Fixed by launching detached (`setsid … &`) and polling the file.

**d3. I nearly recorded a peer's half-finished edit as a repository defect.** The second sweep's `fuzz` gate failed with `no required module provides package github.com/larsartmann/go-branded-id`. It was a mid-edit snapshot: the peer was adding that dependency, `types.go` imported it before `go.mod` did. Verified transient (`go build ./...` green, `go.sum` has both entries) before saying anything. **This is the second time in two days that I have needed the sentence "check whether someone else is mid-write before believing a red."**

**d4. I propagated a wrong attribution before checking it.** My first draft of the `CONTRIBUTING.md` coverage bullet repeated the file's existing claim that the uncovered statement was `Healthz`'s nil-`Checks` guard. The profile says `fetch`'s request-construction guard. The lesson is narrow and repeatable: **read the coverage profile, do not trust the prose about the coverage profile.**

**d5. Two of my edits were rejected by concurrent writes** (`file has been modified since it was last read`, twice, on `CONTRIBUTING.md` and `AGENTS.md`). Recovering was cheap, but I kept editing into a file a peer was editing five minutes later, which is exactly the collision the plan's own §8 question 1 asks about.

**d6. I let a hung gate run for minutes before investigating.** The first `.#skew` timeout was spent waiting instead of reading `ps`. Two `ps` commands identified a 55-minute hang and a rate-limit window; the fix (a `timeout` per network call) is 6 lines. **Diagnose the hang before waiting it out.**

**d7. I edited a script a peer was rewriting, and my own sweep caught the collision as a syntax error.** I added timeout handling to `tools/fleet-skew.sh` while another session was replacing the whole file with a `--source api|local|auto` model. A `nix run .#skew` launched inside that window died on `line 125: syntax error near unexpected token` — a defect that never existed in either version, only in the interval between them. Both edits survived where they were useful (the timeout; their unreadable-counter model), but I should have checked `ls -l --time-style` before touching a gate script for the second time in one night.

---

## e) WHAT WE SHOULD IMPROVE

1. **A number that carries a toolchain must say so.** 99.3% vs 99.5% survived four reports because nobody re-derived it. Every future coverage claim quotes the gate's figure _and_ the toolchain.
2. **Gate scripts that touch the network need deadlines.** `.#skew` (and `.#pre-tag` through it) could hang indefinitely. A gate that never returns is indistinguishable from a gate nobody started.
3. **Third-party package loaders must not inherit the shared cache.** Fixed for three apps; the rule is now written down instead of rediscovered.
4. **A point-in-time report is not a backlog.** 25 of 26 reports were handoffs that were never closed. The harvest cost ~90 minutes; the alternative — reading 26 reports again next month — costs more.
5. **The drift alarm catches absence, not error.** `docs-check` catches a missing release mention; it cannot catch `CONTRIBUTING` naming the wrong uncovered statement. Only re-derivation does.
6. **Two writers on one branch need a claim file**, not politeness. Third session in two days with rejected edits and duplicated work.
7. **"Unreadable" is not "healthy".** Every gate must distinguish *no data* from *good data* and fail on the first. `.#skew` did not, and would have blessed a fleet it could not see; the same audit is worth applying to `.#recorder-audit` (which has the vacuity rule written down) and to any future gate that reads an external service.
8. **A gate's own failure modes deserve the same test discipline as code.** The timeout path and the vacuous path were both proven with a fake `gh` in under a minute; neither had ever been exercised.

---

## f) UP TO 50 THINGS TO DO NEXT

The eleven archive-harvest rows live in `TODO_LIST.md` (§"Harvested from the status-report archive audit") and are not duplicated here. New from tonight:

| #  | Task                                                                                                                                                          | Impact   | Effort |
| -- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- | ------ |
| 1  | Repair or clear the corrupted host `GOCACHE` (`go clean -cache`); until then `go` on PATH is untrustworthy and every agent will re-diagnose it                | **High** | 20m    |
| 2  | Land the claim-file protocol so two sessions stop overwriting each other (rejected edits, duplicated sweeps, two `.#skew` runs at once)                       | **High** | 45m    |
| 3  | Push `master` — the branch is ahead of origin and the peer's own report flags it                                                                              | **High** | 5m     |
| 4  | Tag `cqrs-htmx/health`; the pin is already v0.8.1, only the tag is missing, and it is the ~20-app lever                                                       | **High** | 60m    |
| 5  | Decide `WithLiveThrottle` + `WithShutdownGracePeriod` (both zero-adopter) in the next breaking window                                                         | Med      | 30m    |
| 6  | Promote accumulated fuzz inputs into `testdata/fuzz/` in one deliberate commit                                                                                | Med      | 15m    |
| 7  | Add a step banner + final `EXIT=` line to `tools/fuzz-registry-check.sh` (the one silent gate)                                                                | Low      | 10m    |
| 8  | Gate: a newly exported symbol must appear in `FEATURES.md` (generalises the two new `docs-check` rules)                                                       | Low      | 1h     |
| 9  | ~~Re-check `.#skew` under `timeout`~~ — done tonight; the lingering job is to keep `--allow 14 --allow-unchecked 0` honest as the fleet shrinks | Med      | 10m    |
| 10 | Verify the pkg.go.dev render for `v0.8.1` (never viewed; two reports flag it)                                                                                 | Low      | 5m     |
| 11 | Design note: per-check severity override (fleet ask, untracked until tonight)                                                                                 | Low      | 2h     |
| 12 | File the two BuildFlow upstream issues (failed-step output truncation; `dprint --fix` no-rewrite) after `verify-before-filing`                                | Low      | 1h     |
| 13 | Record the four cross-repo defects (CM-data red test, go-appkit exec bit — now fixed, gopls phantom `undefined: reflect`, `doanalyzerv2` license — now fixed) | Low      | 30m    |
| 14 | Re-derive the "three fuzz targets" class of claim in every doc; `CONTRIBUTING` was wrong for at least a week                                                  | Low      | 20m    |
| 15 | Triage the disk that caused all of this (`journalctl -b -1`, SMART, `e2fsck -n /dev/sdb1`)                                                                    | Med      | 30m    |
| 16 | Archive the next batch of fully-resolved reports once the harvested rows close — target: the 2026-10-08/09 cluster                                            | Low      | 30m    |
| 17 | Audit `.#recorder-audit` and `.#skew` for the "unreadable vs healthy" rule (skew is done; recorder-audit only documents it)                                   | Med      | 20m    |
| 18 | Register `.#skew` in CI's required checks, or state in CONTRIBUTING why weekly-only is enough                                                                  | Med      | 15m    |

---

## g) QUESTIONS I CANNOT ANSWER MYSELF

1. **May I clear the corrupted shared `GOCACHE`?** `/mnt/buildcache/go-build` currently makes plain `go build ./...` fail with `package io is not in std`. `go clean -cache` would fix it and cost every session on this host one cold rebuild. I will not touch a shared cache without your word.
2. **Who owns the second session's work, and how do we stop colliding?** Two peers committed through this session (reports at 01:18, 02:02, and the `go-branded-id` adoption), and my sweep's `fuzz` gate failed on their half-written `go.mod`. A claim file or a "one session per repo" rule is the cheapest fix; I will implement whichever you prefer.
3. **Is `WithLiveThrottle`/`WithShutdownGracePeriod` dead code we should delete, or unused features we should advertise?** Both have zero adopters across all 16 fleet repos. Deleting them is a breaking change for anyone outside the fleet; keeping them costs nothing but attention.

---

## Appendix — commands and exit codes as run

```
GOWORK=off GOTOOLCHAIN=auto nix run .#fuzz-registry        # EXIT=0  "9 targets registered in both fuzz apps"
GOWORK=off GOTOOLCHAIN=auto nix run .#fuzz                 # EXIT=0  all 9 targets PASS
GOWORK=off GOTOOLCHAIN=auto nix flake check                # EXIT=0  all checks passed
go test -count=1 -coverprofile=/tmp/cov.out ./...           # federation 99.5% (host go1.27.0, 207 blocks)
nix develop -c go test ./... -coverprofile=/tmp/nixcov.out  # federation 99.3% (go1.27.3, 148 blocks)
grep -v '^mode:' /tmp/nixcov.out | awk '$3==0'              # one row: federation.go:403.3,404.1
GOCACHE=$(mktemp -d) GOWORK=off go build ./...              # EXIT=0  (shared cache: EXIT=1, "package io is not in std")
nix develop -c go build ./...                               # EXIT=0
GOWORK=off GOTOOLCHAIN=auto nix run .#lint                  # EXIT=0  "0 issues."
GOWORK=off GOTOOLCHAIN=auto nix run .#gates > /tmp/gates5.log 2>&1; echo GATES_EXIT=$? >> /tmp/gates5.log
                                                            # GATES_EXIT=0, ten gates + flake check
GOWORK=off GOTOOLCHAIN=auto nix build .#checks.x86_64-linux.{treefmt,docs-drift-check,format}
                                                            # EXIT=0
nix develop -c dprint fmt <edited markdown>                 # 1–3 files reformatted per batch
bash -n tools/fleet-skew.sh                                 # SYNTAX_OK
git mv docs/status/2026-10-11_19-17_… docs/status/archived/  # archived (history preserved)
grep -r --include='*.go' WithLiveThrottle ~/projects/<16 fleet repos>   # 0 files

# Prove the skew gate's own failure modes, with a fake `gh` that sleeps 30s
env PATH="/tmp/fakebin:$PATH" FLEET_SKEW_TIMEOUT=1 bash tools/fleet-skew.sh --allow 999
                                                            # 16.0s total (was: hung 55+ min), 16 WARNs, EXIT=0
env PATH="/tmp/fakebin:$PATH" FLEET_SKEW_TIMEOUT=1 bash tools/fleet-skew.sh
                                                            # EXIT=1 "all 16 repos were unreachable, so this run proves nothing"
GOWORK=off GOTOOLCHAIN=auto nix run .#skew                  # EXIT=1, source: local
                                                            # minor-stale 14, unreadable 0, 2 OK (both bridges v0.8.1)
```
