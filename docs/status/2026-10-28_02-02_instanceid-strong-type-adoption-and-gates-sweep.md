# Status: InstanceID strong-type adoption (go-branded-id) — executed, verified, documented

**Date:** 2026-10-28 02:02 CET · **Session scope:** owner directive to resolve the 4 `brandid-lint` BD003 STRONG_ID findings by adopting `github.com/larsartmann/go-branded-id` for instance identity · **Tree:** all work committed by the auto-commit daemon (only a trailing `docs/ARCHITECTURE.md` fence fix pending at write time) · **Truth source:** `nix run .#gates` → `GATES_EXIT=0`, `brandid-lint -rules all .` → exit 0, zero findings.

---

## a) FULLY DONE

1. **Prior-analysis honored, then superseded deliberately.** Found the 2026-10-11 report's recorded STRONG_ID deferral ("owner go/no-go missing") and the `.buildflow.yml` budget rationale. The owner's directive (run brandid-lint, name the library, "execute until done") *is* the go/no-go; the supersession is recorded in `docs/servicename-design.md`'s new addendum so the next session does not re-litigate instance identity.
2. **Library research from source, not docs.** Read go-branded-id v0.7.0 in the module cache: `ID[B any, V comparable]` struct, `NewID`/`Get`/`IsZero`/`Equal`/`Reset`, `BrandNamer`, `String()` = `"Brand:value"` (debug only — wire marshal uses the raw value), JSON via v1 `MarshalJSON`/`UnmarshalJSON` under build tags (no v2-specific file compiles without `goexperiment.jsonv2`; interop covers both marshalers).
3. **The golden-wire risk was empirically killed before implementation.** A throwaway module run under the flake's go1.27.3 proved: `omitzero` + the type's `IsZero()` omits the unset field under **both** `encoding/json` (v1 has supported `omitzero` since Go 1.24) and `encoding/json/v2`; a set ID marshals as the same plain JSON string; unmarshal of string / `null` / absent all yield the correct value or zero under both marshalers. The 2026-10-11 objection ("zero value marshals as `null` — golden-wire risk") was true only for a tagless field; `omitzero` is the resolution, now pinned in the CHANGELOG text and the field comment (`types.go`: `json:"instance_id,omitzero"`).
4. **The type landed cleanly.** `types.go`: `InstanceIDBrand` (+`Name()`), `type InstanceID = id.ID[InstanceIDBrand, string]` (alias, per the canonical pattern), `NewInstanceID(string)`; `Response.InstanceID` retyped with an honest doc comment. Value type is `string`, not nanoid — replica identifiers are host-supplied text echoed verbatim, not library-generated.
5. **API surface moved in one consistent stroke.** `probe.go` `Probe.instanceID` + `config.instanceID` + `WithInstanceID(InstanceID)`; `handlers.go` `SanitizeResponse` re-wraps via `Get()`/`NewInstanceID` (zero-preserving: `ToValidUTF8("") == ""`). The `assemble`/response-construction sites needed no changes — the type propagates.
6. **Every compile site migrated; nothing discovered at runtime.** `go vet ./...`-driven enumeration fixed 12 test files: `handlers_internal_test.go` (sanitize table + 500-regression test), `handlers_fuzz_test.go` (wrap + `.Get()` round-trip comparison), `probe_options_test.go` (both WithInstanceID tests), `handlers_json_test.go` (both snapshot fixtures), `federation_test.go` (scalar-drop doc + `IsZero()` assertion), `internal/merge/merge_test.go` (doc fixture + table assertion), `merge_fuzz_test.go`, `aggregate_test.go`, `aggregate_json_test.go`, `aggregate/example_test.go` (golden `// Output:` block preserved via `IsZero()` — same `true`), `aggregate_fuzz_test.go`, `prometheus_example_test.go`, `probe_metadata_test.go`.
7. **Fuzz corpus survived untouched** — the `f.Fuzz`/`f.Add` signature is unchanged (harness wraps internally), so no positional-corpus surgery (the documented `58b20d015136c6e5` class of cost was avoided). `fuzz-registry: OK - 9 targets` and all fuzz targets green in gates.
8. **Dependency added the right way.** `go get github.com/larsartmann/go-branded-id@v0.7.0` (latest; `go 1.26` directive, no toolchain pressure on the graph), `go mod tidy` after the import existed. Final `go.mod`: two direct dependencies.
9. **Full gate sweep, explicit exit codes:** `nix run .#gates` → `GATES_EXIT=0` with every gate confirmed individually in the log: test-race ok, vet silent-ok, golangci-lint **0 issues** (only the documented `erraudit` nolint_filter noise), coverage-gate 100/100/100/99.3/99.9-total (floor 99.5), govulncheck, gosec full SSA across all five packages (isolated-GOCACHE fix working), fuzz (incl. seed corpora), fuzz-registry, docs-check, `nix flake check` "all checks passed".
10. **brandid-lint zeroed:** `-rules all` → exit 0, no output. The 4 BD003 findings (Probe field, config field, option param, Response field) are gone by adoption, not suppression — no `//nolint`, no budget hiding.
11. **OpenAPI golden untouched:** `openapi-lockstep` green with zero edits to `docs/openapi.yaml` — the wire promise held exactly as designed.
12. **Docs updated end-to-end:** CHANGELOG `[Unreleased]` (Added: the type + dependency; Changed: breaking, with before/after migration snippet and the wire-invariance claim), README (stability note now two dependencies; `WithInstanceID(id InstanceID)` section with a `NewInstanceID(os.Hostname())` example and `.Get()` reader guidance), `doc.go` Instance Identity section, AGENTS.md (dependency line, "deliberately small-dependency" reword in the stdlib-errors decision, one new dated Key Design Decision bullet — file at **219/220** lines, cap respected), `docs/ARCHITECTURE.md` (types.go map row + `go api:root` fence extended with the three new symbols, `Name()` method, and the retyped option), `docs/DOMAIN_LANGUAGE.md`, `docs/adoption-matrix.md` (breaking note + migration), `.buildflow.yml` (strong-ID class recorded as eliminated; ceiling lowered 9→4 with rationale and a self-correcting re-baseline instruction).
13. **The docs-check bidirectional API lockstep caught two real gaps** — first the missing exported surface (`Name`), then a fused line in the fence. Both fixed; `docs-check` exit 0 with the explicit-code pattern (`> file; echo $?`), and the fence now lists the complete new surface.
14. **`nix fmt` clean** (treefmt: gofumpt/gofmt/golines/nixfmt); `nix flake check` green inside gates.

## b) PARTIALLY DONE

1. **Budget re-baseline is inferred, not measured.** The 9→4 ceiling rests on arithmetic (the old "7 documented findings" = 2 mixin-sharing + 1 size + 4 strong-ID; elimination leaves 3 + ~20% headroom). No measured branching-flow run backs it yet — the yaml comment itself instructs re-baselining from measurement if the next run disagrees. Honest hedge, still an unverified number.
2. **The v1-marshaler omission is verified but not pinned in-repo.** The claim "zero ID is omitted under `encoding/json` too" was proven in the throwaway module; the repo's own tests pin only the v2 path (`json.Deterministic`, `TestWithInstanceID_OmittedFromJSONWhenUnset`, `TestReadinessResponse_JSONSnapshot`). A consumer re-marshaling a decoded `Response` with v1 is load-bearing on the v1 behavior — that deserves a named test, not a `/tmp` memory.
3. **Consumer migration is documented, not executed.** go-health-dashboard (the single `WithInstanceID` consumer) now breaks at compile time until it wraps with `health.NewInstanceID`. Adoption-matrix + CHANGELOG carry the migration; the dashboard repo itself was not touched, and `docs/consumer-verification.md`'s fleet timeline was not updated with this breaking change.
4. **Older docs that mention `InstanceID` were not audited for signature drift.** `docs/system-checks-cookbook.md`, `docs/multi-tenant-design.md`, `docs/prometheus-exposition-design.md`, `docs/probe-threat-model.md` reference instance identity; they describe wire semantics (which did not change), but none were grepped for embedded `WithInstanceID(id string)`-style snippets that are now lies.
5. **FEATURES.md was left untouched.** The Data Model table row still describes the wire shape (still true), but FEATURES has no row for the `InstanceID` strong type / `NewInstanceID` — the one doc surface that enumerates features by name.
6. **BuildFlow (the second quality surface) was not re-run** on the changed tree. `.#gates` is canonical per the repo's own rule, so nothing blocks — but the dprint/branching-flow/license-check surface has not seen the new dependency or code.
7. **HARVEST not yet run.** This report's section (f) should be routed into `TODO_LIST.md`/`ROADMAP.md` by docs-health; deliberately deferred because the owner said "then wait for instructions".
8. **Unverified carry-over:** whether the 2026-10-11 follow-up ("add `Unreleased → Internal` line for the federation pairing refactor") ever landed in the CHANGELOG — not checked this session.
9. **Scratch state left behind:** `/tmp/brandid-check` (the empirical verification module) was never `trash`ed. Harmless, untidy, and it holds the only runnable reproduction of the v1/v2 verification until task (f)#13 pins it in-repo.

## c) NOT STARTED (all pre-existing unless noted; carried because they are still real)

1. **`ServiceName` typed identity at the v0.6 window** — now the *only* open typed-identity question (instance identity settled by this session). The design doc's addendum asks whether it should follow the `go-branded-id` shape for consistency or stay Option-A defined-string.
2. **golangci LSP integration fix/disable** (standing TODO): the panel displayed a stale `merge_fuzz_test.go:129` typecheck error for the entire session while vet/lint/tests were green; `lsp_restart` was issued early, warning persisted anyway. Every diagnostic had to be arbitrated against the CLI — the tax this bug charges per session.
3. **gosec toolchain pin removal** once nixpkgs ships gosec reading export-data v5 (securego/gosec #1772).
4. **`tools/doanalyzerv2` go-directive floor** — clears only when a post-v0.4.0 samber-linter release propagates through branching-flow; do NOT hand-lower (tidy re-bumps, 2026-10-08 empirical).
5. **go-version-auto-configure warning** on `tools/doanalyzerv2`'s `go 1.27.1` directive — same constraint, lint side.
6. **Weekly `fuzz-long` budget** (5 min/target) — seed corpora ran in gates; the long-budget CI cadence has not seen the new type.
7. **`ci-emulation` (go-free PATH)** has not been re-run on the changed tree (new dependency = new vendor-load path for the `goPkg` tools).
8. **v0.9.0 release** — the breaking change sits in `[Unreleased]`; nothing tagged. Release lifecycle (CHANGELOG promote, tag, proxy verification) untouched, pending owner timing.
9. **Upstream evaluation: go-branded-id native `encoding/json/v2` support** — on go1.27, `encoding/json/v2` is a regular import with no GOEXPERIMENT, so the library's v2-tagged file never activates; go-health consumers run the v1-interface interop path (verified fine). A `MarshalJSONV2` implementation would remove the interop hop. Candidate for a verify-before-filing issue in the owner's repo.
10. **Fleet verification sweeps from the servicename doc's plan table** (webphone, Zlota44, dnsblockd, nsfw-classifier, fir, KeyHolderAI, DiscordSync, CV, go-appkit, cqrs-htmx, dashboard) — pre-staged for the v0.6 window; unchanged.

## d) TOTALLY FUCKED UP

**Nothing at fuckup severity.** No data loss, no red gate escaped the session, no reverted work, no production surface changed beyond the intended API break. Working tree at report time: one pending docs edit (the fence fix), everything else daemon-committed and verified in `HEAD` via `git grep`.

**Nearest misses worth recording (all caught inside the session):**

1. **I repeated a documented gotcha I knew.** The first `docs-check` verification piped output through `tail` and echoed `$?` — which reported **tail's** exit code (0) while docs-check had actually failed with `DRIFT`. AGENTS.md's "gate output tails lie; exit codes don't" and "piping through `tail` also eats the exit status" exist precisely for this. I caught it only because the DRIFT line was visible in the same output. The rule is now burned in: capture with `> file 2>&1; echo EXIT=$?`, always.
2. **A "no-op" edit fused two lines of a structured block.** Removing a trailing newline in an `edit` old_string joined `type InstanceIDBrand struct` with `type HealthRecorder interface` inside the `go api:root` fence, producing `structtype` — which surfaced as a *misleading* drift error ("exports HealthRecorder but block does not list it") pointing away from the real cause. One extra diagnosis round-trip. Lesson: after touching anything inside a fence/table, `grep` the block immediately; "the edit applied cleanly" and "the edit did what I intended" are different claims.
3. **Grep-first enumeration was incomplete.** My first pass over instanceID call sites missed six test files; `go vet` found them all in seconds. The compiler-adjacent tool was the right enumerator from the start; grep was the wasted motion.
4. **Stale-read guard churn.** The auto-commit daemon committed mid-session (expected per AGENTS.md), invalidating a read snapshot and failing one multiedit; several more round-trips came from bash-reading files the edit guard doesn't count. `View` before edit, batched, is the zero-friction path — I knew this too.

## e) WHAT WE SHOULD IMPROVE

1. **Verify claims by measurement, not arithmetic** — the budget ceiling is the standing example. Arithmetic produces a plausible number; a run produces *the* number. Where a gate exists, run it before enshrining its output.
2. **Pin load-bearing empirical results as in-repo tests.** The /tmp verification was the right *research* move (assert-after-verify beats the 2026-10-11 sin of assert-before-verify) — but its conclusion lives in prose until a test reproduces it. Research that ends in a docs claim should end in a `*_test.go` too.
3. **Enumerator hierarchy: vet > grep.** For type-change migrations, `go vet ./...` IS the worklist. Grep is for prose, not for compile sites.
4. **Treat structured blocks (fenced API lists, tables, golden files) as load-bearing code.** They have their own failure modes (fusion, column drift) and their own gates (docs-check, dprint). Diff the block after every touch.
5. **Exit-code discipline is a reflex, not a rule.** Even in "quick verification" one-liners — especially there, because quickness is exactly when the pipe lie slips through.
6. **The daemon changes the edit precondition.** Sessions in this repo should expect mid-flight mod-time invalidation: read, edit, verify — never read long stretches then batch-edit.
7. **Superseding a recorded decision should write the supersession where the old decision lives** (done: servicename-design addendum + budget comment rewrite). This worked well and is worth keeping as the pattern: old analysis edited in place, not orphaned.

## f) NEXT TASKS (up to 50; tiers by impact — most are ROADMAP fuel, not commitments)

**Tier 1 — direct follow-ups (high impact, low effort)**

| #   | Task                                                                                                                                                        | Origin        |
| --- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| 1   | Owner answers the 3 questions in section (g) — they gate items 2, 4, 6, 8                                                                                   | this report   |
| 2   | Cut v0.9.0: promote `[Unreleased]`, tag, verify module proxy + pkg.go.dev (go-release flow)                                                                  | b8/c8         |
| 3   | HARVEST this report's section (f) into `TODO_LIST.md` / `ROADMAP.md` (docs-health)                                                                           | skill loop    |
| 4   | Migrate go-health-dashboard: wrap `NewInstanceID(...)`, `.Get()` reader, run its suite incl. browser/screenshot                                             | b3            |
| 5   | Update `docs/consumer-verification.md` fleet timeline with the InstanceID breaking change + migration note                                                   | b3            |
| 6   | Measured branching-flow run to confirm the 3-finding steady state; re-baseline the ceiling 4 from output, not arithmetic                                     | b1            |
| 7   | Pin the v1-marshaler omission claim as a named root-package test (decode+re-marshal with `encoding/json`, assert `instance_id` absent when unset)             | b2            |
| 8   | Re-run BuildFlow full mode on the changed tree (`GOTOOLCHAIN=auto` in the launching shell); arbitrate reds against `.#gates`                                  | b6            |
| 9   | Audit the four design docs that mention InstanceID for now-stale `WithInstanceID(id string)`-style snippets; fix any found                                   | b4            |
| 10  | FEATURES.md: add a Data Model row for the `InstanceID` strong type (+ `NewInstanceID`), status FULLY_FUNCTIONAL                                               | b5            |
| 11  | `trash /tmp/brandid-check` after #7 pins the claim (or keep it as the repro until then)                                                                       | b9            |
| 12  | Run `erraudit ./... --type-aware` once on the new code (documented standalone invocation; golangci's unknown-linter noise is not erraudit's verdict)          | hygiene       |
| 13  | Re-run `ci-emulation` (go-free PATH) on the tree with the new dependency                                                                                      | c7            |
| 14  | Verify the 2026-10-11 carry-over: does CHANGELOG `[Unreleased]` (or a released section) carry the federation-pairing "Internal" line? Add if missing          | b8            |

**Tier 2 — v0.6 window / typed-identity thread**

| #   | Task                                                                                                                                                        | Origin        |
| --- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| 15  | Decide ServiceName shape: `id.ID[ServiceNameBrand, string]` (consistency with InstanceID) vs Option-A defined string; record in the design doc              | c1, addendum  |
| 16  | If typed: execute the servicename migration plan (signature changes, ~14 fleet call sites, per-repo verification table) at the v0.6 window                    | c1            |
| 17  | ROADMAP: update the typed-identity theme to point at the addendum (instance settled, ServiceName open)                                                        | c1            |
| 18  | OpenAPI: add the `instance_id` semantic description (strong identifier; zero omitted) — was 2026-10-11 f37, shape-locked then, semantics allowed now          | carried       |
| 19  | Fleet skew sweep (`nix run .#skew`) to see which consumer pins are now compile-broken by the type change (informs the dashboard migration order)              | new, cheap    |

**Tier 3 — standing toolchain debt**

| #   | Task                                                                                                                                                        | Origin        |
| --- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| 20  | Fix or disable the golangci LSP integration (standing; it misled for a full session again)                                                                    | c2            |
| 21  | Drop the `gosecGo` pin when nixpkgs ships gosec reading export-data v5 (watch securego/gosec #1772)                                                           | c3            |
| 22  | `tools/doanalyzerv2` directive floor: bump when post-v0.4.0 samber-linter propagates; then re-try the go-directive row                                        | c4            |
| 23  | go-version-auto-configure warning cleanup once #22 lands                                                                                                      | c5            |
| 24  | Evaluate/file upstream: go-branded-id `MarshalJSONV2` (verify-before-filing + github-voice; confirm the interop hop costs anything measurable first)          | c9            |

**Tier 4 — hardening / nice-to-have (ROADMAP fuel)**

| #   | Task                                                                                                                                                         | Origin      |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------- |
| 25  | Weekly `fuzz-long` cadence on the new type (5 min/target)                                                                                                      | c6          |
| 26  | Consider a `Response.Clone` round-trip test asserting `InstanceID` survives clone by value (probably already covered by fuzz; verify, don't assume)            | new         |
| 27  | Benchmark sanity: `InstanceID.Get()`/wrap in the hot path (expect 0 allocs; prove once, then never think about it again)                                        | new         |
| 28  | README quickstart: show `WithInstanceID(health.NewInstanceID(...))` in the primary example, not only the options reference                                      | new         |
| 29  | Consider documenting `String()` ≠ wire value ("InstanceID:pod-abc" is debug-only) in DOMAIN_LANGUAGE to pre-empt dashboard template surprises                   | new         |
| 30  | adoption-matrix: consider a "consumers blocked by breaking change" checklist column for the v0.9.0 window                                                       | new         |
| 31  | Check whether `docs/openapi.yaml`'s `instance_id` description should name the type (`health.InstanceID`) for SDK generators                                     | new         |
| 32  | Sweep for other stringly-typed identifiers the linter would flag next (`Version`? check names?) — decide adopt-or-document before the next brandid run          | new         |
| 33  | AGENTS.md gotcha candidate: "v1 `encoding/json` honors `omitzero` since Go 1.24" (cost us research; one line saves it)                                          | new         |
| 34  | Re-verify `docs/merge-unification-design.md`'s scalar list mentions the type change is wire-invisible (one-line check)                                          | new         |
| 35  | Fleet call-site scan: does any consumer compare `resp.InstanceID == ""` (now `IsZero()`)? Compile catches it — but the skew sweep (#19) makes it visible early  | new         |

**Tier 5 — pre-existing backlog carried from prior reports (still valid, unowned)**

| #   | Task                                                                                                                                                         | Origin      |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------- |
| 36  | Dashboard bridge detailed-variant gap (`docs/bridge-golden-path.md`)                                                                                           | prior       |
| 37  | Adoption-matrix over-count corrections for go-daemon rows (2026-10-11 E5 residue)                                                                              | prior       |
| 38  | Zlota44/dnsblockd const-decl checks, KeyHolderAI literal-vs-typed check, PMA scan on next contact (servicename window sweeps)                                   | prior       |
| 39  | Confirm the weekly CI matrix still covers: gates, fuzz-long, skew — on the post-v0.9.0 tree                                                                     | prior       |
| 40  | Revisit `max-same-issues` golangci trap (documented) — consider raising so identical findings are not hidden                                                    | prior       |
| 41  | upstack: whether `.config/metadata.yaml` (modified outside this session) belongs in the repo or is daemon churn to ignore                                       | noticed     |
| 42  | Consider a CHANGELOG "Dependencies" section convention (two direct deps now; keep-a-changelog has the category)                                                 | new         |
| 43  | Consider go-branded-id version pin policy (flake/renovate/dependabot handling for the second dependency)                                                        | new         |
| 44  | docs/INDEX.md: confirm every new docs-table row gained this session is indexed (docs-check covers ARCHITECTURE naming; INDEX is manual)                         | new         |
| 45  | Post-release: re-run the recorder-audit fleet scan to confirm 0 findings still (cheap, periodic)                                                                | prior       |
| 46  | Sanitize doc: `SanitizeResponse` now documents a conversion step — confirm the godoc example list still matches the implementation lines                        | new         |
| 47  | Consider `TestWithInstanceID_ZeroValueOmitted` naming alignment (existing test name is fine; only if touched anyway)                                            | nit         |
| 48  | Sweep ROADMAP for now-done items ("typed identity" partially resolved; InstanceID row shipped) — mark done                                                      | hygiene     |
| 49  | Check dependabot/config for the new module (private-repo? no — public; confirm no GOPRIVATE entry needed)                                                        | new         |
| 50  | Decide whether the strong-type pattern should spread to `aggregate`/`federation` remote *names* (currently validated strings with a `/` ban) — document-or-adopt | new, design |

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Release timing:** cut **v0.9.0 now** (breaking change isolated in its own release, dashboard migration can start against a tag) or accumulate more `[Unreleased]` work first? The only consumer of the broken API is your own dashboard, so the cost of "now" is near zero — but the version-number call is yours.
2. **Dashboard migration:** should I bump **go-health-dashboard** to the new signature and run its full suite (incl. browser/screenshot) as the immediate next task, or do you want to hand-migrate it? It is the single `WithInstanceID` consumer and the only repo that can verify the reader side (`.Get()`) against real UI code.
3. **ServiceName shape (v0.6 window):** when service-name typing happens, should it be **`id.ID[ServiceNameBrand, string]`** for consistency with `InstanceID` (second brand, same library, same dependency), or the original Option-A **defined string type** (`type ServiceName string`)? This doc's Option C was rejected for v0.5 and the addendum flags the question — it is a pure owner design call that everything downstream (fleet migration cost, ~14 call sites, scanner) keys off.

---

**Waiting for instructions.**
