# AGENTS.md — go-health

Standalone Kubernetes health-probe SDK for samber/do v2. Three-probe pattern (liveness, readiness, startup) with critical/non-critical classification, background caching, and shutdown awareness.

**Module**: `github.com/larsartmann/go-health` · **Packages**: `health`, `health/aggregate`, `health/federation`, `health/checks` · **Go**: 1.27 · **Status**: v0.8.1 released 2026-10-27 (alpha; the consumer-needs wave: cache-freshness primitives, recorder composition, federation per-remote visibility; `internal/merge` owns the shared worst-of fold for aggregate+federation — rank ties no longer let off/unknown remote roll-ups leak into federation's merged status — and `WithGETOnly` is removed under the zero-production-adopter path, `WithAllowedMethods()` with no arguments is the canonical GET-only spelling; gate toolchain on go 1.27.2, gosec's toolchain pinned to 1.27.1).

---

## Commands

| Command                      | Purpose                                                                                                                                                                                                             |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `nix run .#test`             | Run all tests                                                                                                                                                                                                       |
| `nix run .#test-race`        | Run all tests with race detector                                                                                                                                                                                    |
| `nix run .#lint`             | Run golangci-lint                                                                                                                                                                                                   |
| `nix run .#vet`              | Run go vet                                                                                                                                                                                                          |
| `nix run .#coverage`         | Run tests with coverage report                                                                                                                                                                                      |
| `nix run .#fuzz`             | Run fuzz targets (short budget)                                                                                                                                                                                     |
| `nix run .#gates`            | Full pre-push gate sweep, fail-fast (subset: `nix run .#gates -- lint`)                                                                                                                                             |
| `nix run .#ci-emulation`     | Re-run gates under a go-free PATH (CI emulation)                                                                                                                                                                    |
| `nix run .#fuzz-long`        | Fuzz targets, 5 min each (weekly CI budget; override: `-- -fuzztime=10s`)                                                                                                                                           |
| `nix run .#vulncheck`        | Run govulncheck                                                                                                                                                                                                     |
| `nix run .#security`         | Run gosec                                                                                                                                                                                                           |
| `nix run .#build`            | Build all packages                                                                                                                                                                                                  |
| `nix run .#openapi-lockstep` | Verify the golden wire format stays covered by `docs/openapi.yaml` (also a `checks.*` under `nix flake check`)                                                                                                      |
| `nix run .#fuzz-registry`    | Verify both `fuzz` apps in `flake.nix` list exactly the `Fuzz*` targets in the tree (also a `checks.*`, and part of `.#gates`)                                                                                      |
| `nix run .#docs-check`       | Docs drift alarm: README/AGENTS/CHANGELOG/FEATURES in sync with the latest tag, non-empty `[Unreleased]` past the tag, federation API block matches the exported surface (also a `checks.*`, and part of `.#gates`) |
| `nix fmt`                    | Format code (gofumpt, gofmt, golines, nixfmt — Go and Nix only; see the dprint gotcha)                                                                                                                              |
| `nix flake check`            | Validate flake + formatting                                                                                                                                                                                         |

Uses `flake.nix` with `flake-parts` + `treefmt-nix`. Two runtime dependencies: `github.com/samber/do/v2 v2.1.0` (plus `github.com/samber/go-type-to-string`, the naming helper samber/do itself uses, so type-inferred service keys cannot drift) and `github.com/larsartmann/go-branded-id v0.7.0` (the strong `InstanceID` type, adopted 2026-10-28 by owner directive — see docs/servicename-design.md).

---

## Architecture

Single-package library (`health`) plus `aggregate`, `federation`, `checks` and `internal/merge`.
The per-file map, each sub-package's contract, and the test-file layout live in
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — read it before adding a file or moving a
responsibility (moved out of this file 2026-10-27 for the 220-line cap). The decisions and the
gotchas stay here.

### Key Design Decisions

- **Liveness never checks dependencies** — returns in microseconds, always 200. Prevents restart cascades.
- **Readiness gates on critical services only** — non-critical failures set roll-up to `warn` (HTTP 200, degraded), critical failures set it to `fail` (HTTP 503).
- **Startup latches** — once all critical services pass, always returns 200 without re-checking.
- **Background caching by default** (1s refresh) — kubelet/LB polling doesn't hammer dependencies. Set `WithRefreshInterval(0)` for live mode.
- **Shutdown-aware** — `Shutdown()` flips readiness to 503 immediately (even from stale cache); liveness stays 200.
- **Method-set enforcement** — `WithAllowedMethods(...)` wraps all handlers: non-allowed methods get 405 with a sorted `Allow` header (GET always included; duplicates collapse; no arguments = GET alone). Off by default. `WithGETOnly` was **removed** in v0.7.0 under the deprecation policy's zero-production-adopter path (migration snippet in CHANGELOG). Middleware composes outside this guard (see docs/middleware-design.md).
- **Deterministic clock seam** — `p.now()` (backed by `WithNowFunc`) drives uptime, `Response.Timestamp`, and live-throttle freshness. Latency measurement stays on the real clock. Tests inject a fixed clock instead of sleeping.
- **HealthRecorder interface** — replaces the old concrete `*auditlog.Plugin` dependency. Any type with `RecordHealthCheckWithContext(ctx, injector) map[string]error` satisfies it. `samber-do-auditlog.Plugin` implements it implicitly; so does `go-appkit/flightrecorderhealth.Trigger` (captures a flight-recorder snapshot when checks fail).
- **Three-state classify** — `classify` returns `pass` (all healthy), `warn` (only non-critical failures), or `fail` (critical failure or shutting down). `Off` sentinel results are excluded from the roll-up entirely (docs/configured-off-design.md).
- **Configured-off checks** — `Off(detail)` marks an intentionally unconfigured dependency: renders `"status":"off"` (check level only), pass-tier in `Rank`, excluded from the roll-up, satisfies the startup latch, never fails readiness. The roll-up enum stays frozen; see docs/configured-off-design.md for the starting-status distinction and the federation rollout order (old federation consumers refuse off documents — producers ship last).
- **`aggregate` is passive and lock-free by construction** — merge-on-read: every read performs
  one atomic `CachedResponse` load per source. No goroutines, no scheduler, no staleness of its
  own; freshness is bounded by the slowest source's refresh interval. Scalars (`Version`,
  `Uptime`) deliberately do not survive a merge — they are per-process and would lie in an
  aggregate view.
- **Stdlib errors by design** — sentinels (`ErrInvalidTimeout`, `ErrInvalidRefreshInterval`,
  `aggregate.ErrNoSources`, `aggregate.ErrInvalidSource`) use `errors.New`; `Validate()` wraps them with `fmt.Errorf("%w: ...")` to include the offending value and remediation. No error library (samber/oops, go-error-family, cockroachdb/errors) is adopted: this is a deliberately small-dependency library whose only Go-level errors are config-validation sentinels matched via `errors.Is`, not errors at an HTTP/CLI boundary that need classification. HTTP failures are communicated via status codes, not error returns.
- **Injector resolved at construction** — `New` captures the health-check capability into a `healthCheckFunc` via the `resolveHealthCheck` free function. Options write to a construction-only `config` struct, not the `Probe` directly. The Probe never stores `do.Injector` or `HealthRecorder` as fields, avoiding the injector-in-service anti-pattern (DO-6) and eliminating dead construction-only fields.
- **Panic recovery in health checks, fail closed** — `runHealthChecks` recovers panics from the recoverable surface (recorder implementations, batch machinery) and converts them to a synthetic `health-check` error wrapping `ErrPanicDuringHealthCheck`; `NewChecks` recovers a panicking individual check into `ErrCheckPanicked`. `classify`/`grades` map any recovered panic, batch-level or per-check, to `fail` (503), never `warn` (issue #7 finding 1). Service `HealthCheck` panics on the injector path are process-fatal: samber/do runs each check in its own goroutine, so no probe-side recover can catch them. See [docs/panic-recovery-design.md](docs/panic-recovery-design.md).
- **Hook panics recovered, warn-not-fail (2026-10-09)** — the `WithEvaluationHook` call site is a separate surface: the hook runs after evidence collection completes, so a recovered hook panic degrades the response to a synthetic non-critical `evaluation-hook` warn row (sentinel `ErrPanicDuringEvaluationHook`), never `fail`, and never kills the refresh loop. The hook receives a defensive copy (own `Checks` map), so it cannot corrupt the served/cached response. The "recovered panics never warn" rule binds the data-collection surface only — see the design doc's evaluation-hook addendum.
- **Validate-on-Start** — `Start()` calls `Validate()` and returns an error on invalid configuration (zero/negative timeout, negative refresh interval). Fail-fast instead of silent runtime degradation.
- **Cache freshness is a primitive, not a policy (2026-10-27)** — `CacheAge`/`CachedResponseFresh`/`EvaluateAndCache` supply the serve-cache-or-evaluate mechanics four fleet consumers had hand-rolled; `CachedResponse` stays a zero-alloc read-only view (`Checks` aliases the snapshot) and `Response.Clone` is the sanctioned copy before overlays (docs/cache-freshness-design.md, docs/response-ownership-design.md).
- **Fail at boot, not silently (2026-10-27)** — function-path constructors reject `WithHealthRecorder` through `ErrRecorderNotApplicable` in `Validate`/`Start` (go-health#7 F2), and `ChainRecorders` preserves detailed durations across sinks (`recorder.go`). A recovered per-check panic grades `fail` even for non-critical checks (`isRecoveredPanic`, #7 F1); go-appkit must flip its warn pin when it bumps past this release.
- **Zero logging coupling** — the library does not import `log/slog` or any logging package. HTTP write failures (client disconnect) are silently swallowed. A library must not make logging decisions for the host application.
- **Observability via hook, not library** — `WithEvaluationHook` is the metrics/alerting seam; Prometheus/OpenTelemetry formats are consumer composition (docs/prometheus-exposition-design.md). No client_golang dependency.
- **Programmatic API mirrors handlers** — `Status/Alive/Ready/AwaitReady` read the cached view (never trigger checks); `Healthz` answers "route traffic here?"; `HealthCheck`/`AsShutdowner` make the probe a first-class do citizen.
- **Version endpoint is build identity, not health** — `VersionHandler(stamp)` is a free function (no Probe, no injector): it never evaluates checks, never 503s, and ignores shutdown. GET-only 405 + `Allow: GET` unconditionally (the method guard's posture without its config surface). The `version` field is always present (no omitempty): an unstamped binary answers `""`. Payload marshaled once at construction — version is immutable per process.
- **Instance identity is a branded strong type (2026-10-28)** — `Response.InstanceID` and `WithInstanceID` carry `InstanceID = id.ID[InstanceIDBrand, string]` (go-branded-id; owner directive superseding the 2026-10-11 STRONG_ID deferral, addendum in docs/servicename-design.md). Wire-preserving by construction: `omitzero` + the type's `IsZero` keeps unset omitted, set values marshal as the plain string (verified under both marshalers); `SanitizeResponse` re-wraps via `Get`/`NewInstanceID`; fuzz seed corpus survived untouched (harness signature unchanged).
- **Per-check Since is probe-observed** — a `transitionTracker` stamps `Check.Since` inside `buildChecks` on every evaluation path (refresh, live, startup): the first batch reporting the current status, carried while it holds, restarted on change/reappearance, reset on process restart. Never service-reported (services cannot know their graded status). Liveness's empty checks and the Healthz synthetic `startup` check never carry Since.
- **Per-check Duration is executor-reported, opt-in** — `CheckDetail{Err, Duration}` is the internal seam; plain `map[string]error` sources are adapted with zero duration. `NewWithDetailedCheck` and the optional `DetailedHealthRecorder` interface populate `Check.DurationNanos` (int64 ns, omitzero). The raw injector path cannot: do's `HealthCheckWithContext` returns only errors (see docs/check-metadata-design.md).

### Decoupling from samber-do-auditlog

This package was extracted from [`samber-do-auditlog`](https://github.com/larsartmann/samber-do-auditlog) to eliminate the transitive dependency cost. The old `WithPlugin(p *auditlog.Plugin)` option is now `WithHealthRecorder(r HealthRecorder)`. The `*auditlog.Plugin` type implicitly satisfies `HealthRecorder` via its `RecordHealthCheckWithContext` method — pass it directly.

Migration guide for pre-extraction code: [docs/migration-plugin-to-recorder.md](docs/migration-plugin-to-recorder.md).

**doanalyzerv2:** the private AST analyzer (in-repo runner `tools/doanalyzerv2`; invoke
`(cd tools/doanalyzerv2 && go run . ..)`) reports 0 DO-1..DO-6 findings. It needs the
go-design-smells checkout at `/home/lars/projects/branching-flow` (the go.mod replace path).

**Consumer verification:** fleet compatibility inventory (15 direct repos → 36 dashboard consumers through three bridges; five implementation patterns, incl. hand-rolled cache adapters), the per-release dashboard verification timeline, and the samber-do-auditlog reverse-dependency prohibition live in [docs/consumer-verification.md](docs/consumer-verification.md); the 2026-10-27 needs sweep + supplier plan is [docs/research/2026-10-27_fleet-consumer-needs.md](docs/research/2026-10-27_fleet-consumer-needs.md).

### Data Flow

1. User creates `Probe` via `New(injector, opts...)` — options write to a `config` struct, consumed and discarded at construction
2. `Start(ctx)` validates config and optionally launches background cache refresh loop
3. HTTP handlers serve cached or live health-check results
4. Readiness/startup delegate to `runHealthChecks` (with panic recovery), which calls the `healthCheckFunc` resolved by the `resolveHealthCheck` free function at construction (recorder or raw injector)
5. `Shutdown()` marks probe as shutting down (readiness → 503, liveness stays 200)

### Concurrency Model

- `shuttingDown` and `startupPassed` are `atomic.Bool` — no mutex needed.
- `latest` is `atomic.Pointer[Response]` — lock-free cache reads.
- `mu` protects `cancel` and serializes WaitGroup Add/Wait (lifecycle race fix, 2026-09-04).
- `throttleMu` serializes throttled live evaluations; the `classifier` is read-only after construction (no lock on the evaluate path).
- `transitions` (transitionTracker) serializes Since stamping per batch inside `buildChecks`; never touched on cached-read paths. Overlapping evaluations (refresh loop + unlatched startup probes) serialize on it; out-of-order completions may attribute a transition to the later batch's clock (bounded by batch duration, self-correcting — see docs/check-metadata-design.md).
- All handlers are safe for concurrent use.

---

## Testing Patterns

- Standard `testing.T` + table-driven tests. No ginkgo/testify.
- Each test creates its own `do.Injector` — no shared state.
- `mockRecorder` type replaces the old auditlog integration tests.
- Benchmarks: `LivenessHandler`, `ReadinessHandler_CacheHit`, `ReadinessHandler_LiveEval`, `ReadinessHandler_RecorderPath`, `StartupHandler_Unlatched`, `StartupHandler_Contention`, `CachedResponse_ParallelReads`, `Evaluate`, `GuardOverhead`.
- Fuzz targets: `FuzzResponseMarshalDeterministic` + `FuzzHandlerInput` + `FuzzThrottleWindowBoundary` (root; the throttle fuzz drives the window rule on a fake clock), `FuzzResponsesMergeInvariants` (internal/merge; the collapsed merge-property suite over arbitrary Source pairs — worst-of/tie stability, reachable row, namespacing, scalar drop), `FuzzAggregateMergeInvariants` (aggregate; the thin mapping harness — merged output must equal `merge.Responses` over the mapped views; source freshness modes — live/cache/throttled — derived from name parity so the corpus signature stays valid), `FuzzBatteries` (checks — pins the sentinel-or-nil error surface of the four batteries over untrusted inputs); run via `nix run .#fuzz`.
- **Seam-swap tests must not be parallel** — tests swapping a package-global
  seam (`marshalResponse` in both packages, `marshalVersionResponse` in
  version.go) mutate shared state, so they omit `t.Parallel()` (marked
  `//nolint:paralleltest`). A parallel seam test corrupts concurrent handler
  tests (observed: spurious 500s).
- **Coverage floor (measured 2026-10-27): root 100%, aggregate/checks/merge
  100%, federation 99.3%, total 99.9%** — the one uncovered statement is
  federation's request-construction guard, unreachable for every remote `New`
  accepts (rationale at `fetch`); closing it would mean deleting an error check.
  That percentage is toolchain-relative: the gate's go1.27.3 counts 148 blocks
  (99.3%), a host go1.27.0 counts 207 (99.5%) over the same source and the same
  uncovered line. Neither is a regression — quote the gate's number with the
  toolchain, and never "fix" the gap by editing the figure.
- **Defensive branches get extracted, never suppressed** — a branch reachable
  only by a future read-path change is moved into a named function with the
  reason in its doc comment, then pinned by an `*_internal_test.go`:
  `applyStartupFailureOverlay` (accessors.go), `applyStartupLatchOverlay`
  (federation/healthz.go), `versionMarshalFailure` + the marshal seam.

---

## Gotchas

- **samber/do v2.1.0 behavior** — never-invoked lazy services appear in `HealthCheckWithContext` results with nil error. Eagerly invoke critical services at boot for the startup probe to be meaningful.
- **Three-state classify** — `classify` returns `pass`/`warn`/`fail` (off results are excluded — docs/configured-off-design.md). The readiness handler maps only `fail` to HTTP 503; `warn` and `pass` both return 200.
- **Config validation** — `Probe.Validate()` checks `timeout > 0` and `refreshInterval >= 0`. `Start()` calls `Validate()` and returns an error on invalid config — callers should check the error from `Start()`.
- **No GOEXPERIMENT needed since the go 1.27 floor** — `handlers.go` (and
  `aggregate/aggregate.go`) import `encoding/json/v2`, stable stdlib on go1.27
  (verified 2026-09-22: build + vet + full suite green with `GOEXPERIMENT`
  unset; the flake exports no experiment anywhere). A 1.26-era AGENTS.md
  revision once wrongly claimed otherwise — the host shell leaked
  `GOEXPERIMENT=jsonv2` into every nix invocation. Enduring lesson: host env
  vars leak into nix run/develop; gates must set or unset what they depend on
  explicitly. Set `GOWORK=off` to avoid workspace interference.
- **Tools that shell out to `go` need `goPkg` in their flake app — and the tool's own export-data reader must keep up with what that `go` emits** —
  `golangci-lint`, `govulncheck`, and `gosec` load packages by invoking a `go`
  binary from PATH; on CI (no Go on PATH) they fall back to the GOROOT they
  were compiled with, which cannot satisfy go.mod's `go 1.27` directive (the
  first CI run caught it). Fix: `goPkg` in every such app's `runtimeInputs`.
  Second layer (2026-10-11 incident): a tool built against an older toolchain
  cannot decode the export-data format a newer `go` writes — gosec 2.29.0
  (go1.26-era x/tools, decodes up to v4) exited 1 with zero real findings on
  go 1.27.2's v5 data ("export data version 5 is greater than maximum
  supported version 4"), making the gate VACUOUS-red, not clean (all four
  packages skipped SSA). The v4/v5 boundary sits between 1.27.1 and 1.27.2,
  and nixpkgs' silent go_1_27 1.27.1→1.27.2 move flipped the gate red with
  zero code changes — including the v0.6.0 tag's CI run, permanently. Fix:
  pin the tool's PATH `go` to the newest emitter it can read, HARDCODED via
  overrideAttrs (`gosecGo` in the security app) — never a moving nixpkgs
  symbol; drop the pin when nixpkgs ships the fixed tool (securego/gosec
  #1772). Rule of thumb: any new flake app that indirectly runs `go` must
  list `goPkg`, and the tool must decode that toolchain's output — the same
  host-shell-env leak class, two layers down.
- **Gate output tails lie; exit codes don't (2026-10-11)** — the v0.6.0 tag
  was cut on a "green" sweep whose security gate had already failed: the log
  tail showed gosec's `Issues: 0` summary and no exit-code check followed,
  so the vacuous pass read as green (gates are fail-fast — a later gate's
  output appearing is proof of nothing without `EXIT=$?`; piping through
  `tail` also eats the exit status). Every gate invocation must end with an
  explicit exit-code echo; `nix run .#gates` returning 0 is the only truth.
- **The shared `GOCACHE` fakes findings, then fakes broken code (2026-10-27/28)**
  — the host points `GOCACHE` at `/mnt/buildcache/go-build`, mutated mid-run by
  other sessions and ~15 language servers. First symptom: gosec's package load
  hit `reading srcfiles list: cache entry not found`, skipped SSA for all five
  packages and exited 1 with `Issues: 0`. After that disk hit 0 bytes free it
  went further — `.#lint` failed with `could not load export data: … no such
  file or directory` (2 typecheck issues, 0 findings) and a plain host-shell
  `go build ./...` fails with `package io is not in std`. Both are cache
  artifacts, never code: `GOCACHE=$(mktemp -d)` is green on the same tree, which
  is why `.#security`, `.#lint` and `.#vulncheck` take the shared
  `privateGocache` helper in `flake.nix`. `.#test`/`.#test-race`/`.#vet` are
  immune (the go tool rebuilds on a miss; third-party loaders bail out). Treat
  any `go` reached through PATH on this host as suspect, and never "fix" code
  a CLI gate reports red this way — arbitrate with an isolated cache first.
- **`.#lint` needs headroom on a loaded host (2026-10-27)** — with
  `run.timeout: 5m`, package loading gave up (`failed to load packages:
  context deadline exceeded`) twice at load average >250 before any linter
  reported anything; the same tree finished in ~9m under a 15m deadline and
  reported exactly one real finding. `.golangci.yml` now sets 10m. A deadline
  cannot hide a finding; it can only stop a green run from looking red.
- **BuildFlow failed-step output is head+tail truncated — replay, don't post-mortem (2026-10-11)** — a `test-coverage` failure whose visible stream ends `ok … federation` + bare `FAIL` hid the real failing package: BuildFlow stores only ~20 head + ~20 tail lines per step (`step_outputs.command_output` in `~/.local/state/buildflow/buildflow.db`, ~1979 chars). Recipe: read the stored output with python3/sqlite3, then replay the exact invocation with the printed seed — `go test -v -coverprofile=reports/coverage.out -covermode=atomic -parallel=$(nproc) -shuffle=<seed> -timeout=10m -count=1 ./...` (coverage mode adds `-covermode=atomic -count=1` vs `.#test`; the seed makes shuffle deterministic). That night's failure was one instance of a 19-repo fleet window around the 23:15 reboot and never reproduced (exact-seed replay green on the plain and exact-coverage commands, 12/12 random-seed replays green, full mode EXIT=0 after) — transient environmental, no code change. Upstream candidate: persist full failed-step output to a file.
- **`encoding/json/v2` does not sort map keys by default** — under v2 semantics `json.Marshal` serializes maps in random Go map order unless `json.Deterministic(true)` is passed (v1's always-sorted behavior was a compatibility default, not a v2 one). `writeResponse` opts in (handlers.go); `TestReadiness_JSONChecksAreSortedAlphabetically` guards the property. Any new marshal site must pass the option too.
- **`encoding/json/v2` cannot marshal `time.Duration` AT ALL** — no default representation exists (go.dev/issue/71631, undecided upstream) and no struct-tag format is accepted (verified empirically on go1.26.7: `int`, `ns`, `nanoseconds`, … all rejected); the only escape is the per-call `json.FormatDurationAsNano` option, which every re-marshaling consumer would have to know to pass. That is why `Check.DurationNanos` is a plain `int64` while the in-process seam `CheckDetail.Duration` stays `time.Duration`, converted once in `buildChecks`. Pinned by `TestCheck_JSONOmitZero`. Related v2 trap: scalar `omitempty` (bool/int) is not honored — only strings and `omitzero` omit; see `TestReadinessResponse_JSONOmitEmpty`.
- **erraudit enforcement flags are opt-in** — `--enforce-samber-oops` and `--enforce-go-error-family` flag stdlib constructors (`errors.New`, `fmt.Errorf`) as violations. These flags are for projects that have already adopted those libraries. This project deliberately uses stdlib errors, so the correct invocation is `erraudit ./... --type-aware` (reports 0 ERROR violations). Do not cargo-cult a library adoption to silence the linter — the sentinels are config-validation errors, not boundary errors needing classification.
- **`WithTimeout` is batch-level, not per-service** — the deadline is shared across all services in one evaluation. A slow dependency steals time from every other check. samber/do exposes `HealthCheckTimeout` (per-service) via `InjectorOpts` at injector creation time. See [docs/timeout-design.md](docs/timeout-design.md) for the full analysis, including why HTTP query-param timeout overrides are rejected (DoS amplifier + breaks caching).
- **`aggregate` sources must be eagerly invoked too** — the samber/do lazy-service gotcha applies per source: a source probe whose services were never invoked health-checks as pass, and the aggregate propagates that false confidence. Invoke critical services at boot.
- **Fuzz signature changes invalidate the testdata corpus** — corpus files in `testdata/fuzz/` are positional; changing an `f.Add`/`f.Fuzz` signature (adding a parameter) makes every saved corpus entry fail to load ("mismatched number of parameters"). Hand-edit the corpus files in the same commit as the signature change (see the 4-string update to `handlers_fuzz` seed `58b20d015136c6e5`), or accept losing the accumulated inputs. That cost is why the aggregate never-started path is unit-pinned instead of widening `FuzzAggregateMergeInvariants`.
- **Throttled live eval is the only path that stores into `latest` outside the refresh loop** — in live mode (`WithRefreshInterval(0)`) the background loop does not exist, so after `Start`'s one initial `refreshCache` (probe.go calls it unconditionally, even at interval 0) the cache holds a boot-time snapshot and only throttled readiness evaluations refresh it. A probe whose `Start` was never called has an empty cache: `CachedResponse()` returns the zero `Response` and an aggregate over such sources merges emptiness (pinned by `TestCachedResponse_NeverStartedSource`; test fixtures that need fresh state must prime via a throttled readiness request, see `newPrimedSource`/`newStateProbe`).
- **golangci's `nolint_filter` warning for `//nolint:erraudit` is expected noise** — erraudit is a standalone tool whose suppression engine honors `//nolint[:linter[:rule]]` (erraudit `pkg/suppression/parser.go`), but golangci-lint does not know `erraudit` as a linter and its `NolintFilter.Finish` warns unconditionally ("Found unknown linters in //nolint directives: erraudit", exit code still 0). No upstream suppression option exists. Do NOT switch those directives to bare `//nolint` to silence the warning — that would disable every golangci linter on the affected lines. Affects `handlers.go`, `aggregate/aggregate.go`, `federation/federation.go`.
- **The golangci LSP panel holds stale typechecks — restart, don't chase** — a `typecheck: expected 1 expression` on `evaluation_hook_test.go:97` persisted through an entire 2026-10-09 session while the file was vet/build/race/lint green (the flagged line was `t.Parallel()`; the diagnostic's line numbers even pointed into an older file revision). Decision (A18): `lsp_restart golangci_lint_ls` clears it; the CLI gates (`nix run .#vet`, `.#lint`, `.#test-race`) are the truth source. If a diagnostic contradicts a green CLI run, restart the LSP first — never "fix" code the compiler accepts.
- **This repo is BuildFlow-covered (`.buildflow.yml`, since 2026-10-08)** — `buildflow --fix --build-mode=full` is an additional quality surface alongside the flake gates; whether it REPLACES them is an open owner question (status report 2026-10-08 20:58 §g3). Full mode verified exit 0 on 2026-10-11 (rebuilt binary 200ab99 + `GOTOOLCHAIN=auto` in the launching shell; the 2026-10-09 stragglers pass) — but the binary is not stationary: the profile later resolved to the `chore/linter-v050-guard` branch install (5aaf336), whose 1.27.0 tool closure re-reddened full-mode `license-check` (root + tools) on the branching-flow go-directive floor while single-step passes; known upstream gap per the yaml (`license-check/govalid-generate` remain upstream tasks), so arbitrate full-mode reds against `.#gates` before believing them. Budgets (art-dupl 80, branching-flow 9, go-auto-upgrade 8) are policy documents with rationale in the yaml — re-reviewed 2026-10-11 (7/9, 63/80, 7/8), still right. dprint is the one BuildFlow failure a human must repair: `dprint check` exits 20 on unpadded markdown tables and the step's `--fix` does not rewrite them, so run `nix develop -c dprint fmt` (2026-10-27: 17 files, whitespace-only). Gotcha of record: a makezero regression reached master through this seam (`b2b9ed0`); see the makezero contract below.
- **makezero `always: true` contract (settled 2026-10-09, closing 20:58 §b1's open question)** — flags EVERY two-arg `x := make([]T, n)` bound to a plain identifier unless the length is the literal `0`; usage is irrelevant (append not required, index writes do not exempt). Makes inside composite literals (`field: make([]T, n)`) are INVISIBLE to the rule — the analyzer only visits assignment-statement RHS — which is why the pre-refactor `startup: make([]atomic.Bool, len(remotes))` passed while the refactored `states := make(...)` tripped: form, not usage. Blessed forms: `make([]T, 0, n)` + append, or index-written pre-sizing with a rationale `//nolint:makezero` (see federation.go `results`). Diagnostic trap: golangci's default `max-same-issues: 3` can hide additional identical findings in one run. Full contract also recorded as a comment at the `makezero:` block in `.golangci.yml`.
- **Markdown formatting stays with BuildFlow; treefmt cannot own it (deliberate non-fix, 2026-10-27)** — `dprint.json` lists four remote wasm plugins, and `dprint` _downloads and compiles_ them on first use (verified: a fresh `XDG_CACHE_HOME` run prints `Compiling https://plugins.dprint.dev/markdown-0.22.1.wasm` and writes it to the cache). treefmt runs inside the nix sandbox with no network, so `programs.dprint.enable = true` would turn `nix flake check` red on every clean machine. Formatting is BuildFlow's responsibility for this repo (`.buildflow.yml` `dprint-format`, which does have network); treefmt owns only the hermetic subset (gofumpt, gofmt, golines, nixfmt). The recurring symptom is unpadded markdown tables that `dprint check` reports and `--fix` will not rewrite — repair with `nix develop -c dprint fmt`.
- **New drift gates (2026-10-27) run inside `nix run .#gates`** — `fuzz-registry` (`tools/fuzz-registry-check.sh`) diffs both fuzz apps in `flake.nix` against `^func Fuzz` per package _and_ against each other; `docs-check` now also fails when `[Unreleased]` documents nothing while HEAD is past the latest tag, and when `docs/federation-design.md`'s fenced go API block and federation's exported surface disagree (both directions). That last rule exists because `SourceStatuses` and `Healthz` were shipped, tested, and missing from the doc.
- **The golangci LSP integration lies** — its diagnostics panel has carried provably false typecheck errors and stale lint findings across sessions (observed 2026-10-08, 2026-10-03). Arbitrate every diagnostic with a real build (`nix run .#lint` / `.#test`); never fix what the panel claims without gate confirmation. Fixing or disabling the integration is a standing TODO.
- **Go-directive patch floors cascade through replace chains; the lever is the toolchain, not the directive** — `go mod tidy` auto-bumps a module's `go` directive to the highest patch requirement in its graph, so hand-lowering is unstable (empirical 2026-10-08: `go 1.27` → tidy → `go 1.27.1`, "switching to go1.27.2"). Chain: released samber-linter v0.4.0 declares `go 1.27.1` → branching-flow (go-design-smells) → `tools/doanalyzerv2` via the local replace. Consequence: any go 1.27.0 tool cannot load that graph; gopls shows the same error. Resolution status (re-verified 2026-10-11 on rebuilt binary `200ab99`): full mode now exits 0 end to end with `GOTOOLCHAIN=auto` exported in the launching shell (79/79 steps; the four 2026-10-09 stragglers pass) — the binary rebuild plus the exported env closed the fan-out gap. Keep exporting `GOTOOLCHAIN=auto` until a run proves it unnecessary without it; the `tools/bin` wrappers stay until the graph no longer requires a newer patch than BuildFlow's toolchain (directive row still BLOCKED — the go-version-auto-configure warning persisted in the green 2026-10-11 run). The go-version-auto-configure warning on tools/doanalyzerv2's `go 1.27.1` ("must be major.minor only") is the same constraint from the lint side — samber-linter HEAD already declares `go 1.27`, so it clears only when a post-v0.4.0 release propagates through branching-flow; do NOT hand-lower (tidy re-bumps, 2026-10-08 empirical). The makezero seam note above stays load-bearing either way: full mode is a quality SURFACE, never the only gate — `nix run .#gates` remains canonical.

---

## Project Documentation

The per-doc index — FEATURES, TODO_LIST, ROADMAP, the design notes, the ADRs,
and the quality-tool suppressions — lives in [docs/INDEX.md](docs/INDEX.md)
(moved out of this file 2026-10-08 for headroom under the 220-line preflight
cap). Every docs-table row gained since then must land there.
**Critical-name validation (2026-10-02):** `Start()` fails with `ErrUnknownCriticalService` when a `WithCriticalServices` name never appears in the initial evaluation batch — closing the DiscordSync 2026-08-16 bug class (typo'd names silently blocked the startup latch and downgraded fail→warn, previously caught only by that repo's private guard test). Validation is batch-based by design, NOT `ListProvidedServices()`-based; decision + fleet compat scan: docs/start-validation-design.md.
