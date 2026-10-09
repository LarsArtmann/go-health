# AGENTS.md — go-health

Standalone Kubernetes health-probe SDK for samber/do v2. Three-probe pattern (liveness, readiness, startup) with critical/non-critical classification, background caching, and shutdown awareness.

**Module**: `github.com/larsartmann/go-health` · **Packages**: `health`, `health/aggregate`, `health/federation`, `health/checks` · **Go**: 1.27 · **Status**: v0.5.1 released 2026-10-09 (alpha; configured-off checks via `health.Off(detail)`, hook-panic recovery with the `evaluation-hook` warn row, `checks.Disk` G115 guard, and the failed-`Start` disarm fix shipped; gate toolchain on go 1.27.2).

---

## Commands

| Command                      | Purpose                                                                                                                                           |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `nix run .#test`             | Run all tests                                                                                                                                     |
| `nix run .#test-race`        | Run all tests with race detector                                                                                                                  |
| `nix run .#lint`             | Run golangci-lint                                                                                                                                 |
| `nix run .#vet`              | Run go vet                                                                                                                                        |
| `nix run .#coverage`         | Run tests with coverage report                                                                                                                    |
| `nix run .#fuzz`             | Run fuzz targets (short budget)                                                                                                                   |
| `nix run .#gates`            | Full pre-push gate sweep, fail-fast (subset: `nix run .#gates -- lint`)                                                                           |
| `nix run .#ci-emulation`     | Re-run gates under a go-free PATH (CI emulation)                                                                                                  |
| `nix run .#fuzz-long`        | Fuzz targets, 5 min each (weekly CI budget; override: `-- -fuzztime=10s`)                                                                         |
| `nix run .#vulncheck`        | Run govulncheck                                                                                                                                   |
| `nix run .#security`         | Run gosec                                                                                                                                         |
| `nix run .#build`            | Build all packages                                                                                                                                |
| `nix run .#openapi-lockstep` | Verify the golden wire format stays covered by `docs/openapi.yaml` (also a `checks.*` under `nix flake check`)                                    |
| `nix run .#docs-check`       | Docs drift alarm: README/AGENTS/CHANGELOG/FEATURES in sync with the latest tag (also a `checks.*` under `nix flake check`, and part of `.#gates`) |
| `nix fmt`                    | Format code (gofumpt, goimports)                                                                                                                  |
| `nix flake check`            | Validate flake + formatting                                                                                                                       |

Uses `flake.nix` with `flake-parts` + `treefmt-nix`. Single dependency: `github.com/samber/do/v2 v2.1.0`.

---

## Architecture

Single-package library (`health`) with these source files:

```
doc.go           — Package doc comment (quick start, three-probe rationale, caching, shutdown)
types.go         — Status enum (check level: pass/fail/warn/off; the roll-up stays pass/fail/warn), Off/OffError configured-off sentinel, Check (incl. Since, DurationNanos), CheckDetail, Response data model (incl. instance_id, timestamp omitzero)
probe.go         — Probe struct, config struct, Option functional options (write to config), HealthRecorder + DetailedHealthRecorder interfaces, New(), resolveHealthCheck (free function), Validate(), lifecycle (Start/Shutdown/MarkShuttingDown), guard (method-set enforcement), now/uptime clock seam, Evaluate, CachedResponse (lock-free read + shutdown overlay), accessors, runHealthChecks (with panic recovery), buildChecks (Since stamping + duration carry), errorsOf/detailOf adapters
tracker.go       — transitionTracker: per-check status-transition tracking behind a mutex; stamps Check.Since inside buildChecks (probe-observed, prunes absent checks)
classifier.go    — Read-only classifier: classify (three-state), evaluateStartup, per-check grading; constructed once, evaluated lock-free
handlers.go      — LivenessHandler, ReadinessHandler, StartupHandler, RegisterRoutes, Routes, DefaultRoutes, readinessResponse/throttledLiveResponse, writeResponse + SanitizeResponse (UTF-8 coercion)
accessors.go     — ErrProbeUnhealthy, HealthCheckFunc, NewWithHealthCheck, DetailedHealthCheckFunc, NewWithDetailedCheck, Status/Alive/Ready, AwaitReady, HealthCheck (do conformance), ProbeShutdowner/AsShutdowner, Healthz
checks.go        — CheckFunc + NewChecks (injector-free named-check constructor: concurrent execution, per-check duration, panic recovery, nil fail-closed, batch-deadline abandonment), runNamedChecks/runBoundedCheck executors — design: docs/named-checks-design.md
version.go       — VersionHandler: build-version endpoint (ldflags/VCS stamp as {"version":"..."}); GET-only 405 unconditionally, UTF-8-coerced, payload pre-marshaled at construction; wire shape in docs/openapi.yaml (VersionResponse)
export_test.go   — ResetStartupLatchForTest (test builds only; public latch stays one-way)
```

Sub-package `aggregate` (source: `aggregate/aggregate.go`) merges N in-process probes into one
`health.Response`: `Source{Name, Probe}`, `New(sources...)` (rejects empty, duplicate,
slash-containing, or nil-probe sources — docs/aggregate-source-name-design.md), merge-on-read
`CachedResponse` (N lock-free loads, worst-of status, `"source/check"` namespacing, shutdown
overlay), `StartupComplete` (AND of latches), the three kubelet handlers, and a standalone
`Healthz()` single-endpoint handler (docs/aggregate-healthz-design.md).

Sub-package `federation` (source: `federation/federation.go`) is the network sibling: it pulls N
remote go-health instances over HTTP into one `health.Response` — same source-name contract as
aggregate, merge-on-read with one parallel fetch per remote, worst-of status, per-remote startup
latches, and one synthetic `name/reachable` FAIL check per unreachable/undecodable remote (never a
silent freeze, never a status override). Wire input is untrusted (invalid documents refused whole),
`Check.Since`/`DurationNanos` survive verbatim, scalars do not. Full semantics (fetch caps,
dashboard-`Prober` structural assertion, API surface): docs/federation-design.md. Remote
names never hit `ErrUnknownCriticalService` (fetch-side is a different universe):
docs/federation-validation-semantics.md.

Sub-package `checks` (source: `checks/checks.go`) is the batteries package: zero-dependency,
stdlib-only `func(ctx) error` constructors — `Disk`, `Memory`, `HTTP`, `Database`, each with a
documented sentinel error — composing with `NewChecks`, `NewWithHealthCheck`, or any recorder
path. Resource checks are warn-by-default: criticality stays with the caller; thresholds are
arguments, not constants. Ownership decision + constraints:
docs/batteries-ownership-decision.md; tests use an in-process `sql.Register` fake driver.

### Key Design Decisions

- **Liveness never checks dependencies** — returns in microseconds, always 200. Prevents restart cascades.
- **Readiness gates on critical services only** — non-critical failures set roll-up to `warn` (HTTP 200, degraded), critical failures set it to `fail` (HTTP 503).
- **Startup latches** — once all critical services pass, always returns 200 without re-checking.
- **Background caching by default** (1s refresh) — kubelet/LB polling doesn't hammer dependencies. Set `WithRefreshInterval(0)` for live mode.
- **Shutdown-aware** — `Shutdown()` flips readiness to 503 immediately (even from stale cache); liveness stays 200.
- **Method-set enforcement** — `WithAllowedMethods(...)` wraps all handlers: non-allowed methods get 405 with a sorted `Allow` header (GET always included; duplicates collapse). Off by default. `WithGETOnly()` is **deprecated** (v0.1.1) but still functional — it is the zero-arg equivalent; keep its tests until removal is decided. Middleware composes outside this guard (see docs/middleware-design.md).
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
  `aggregate.ErrNoSources`, `aggregate.ErrInvalidSource`) use `errors.New`; `Validate()` wraps them with `fmt.Errorf("%w: ...")` to include the offending value and remediation. No error library (samber/oops, go-error-family, cockroachdb/errors) is adopted: this is a single-dependency library whose only Go-level errors are config-validation sentinels matched via `errors.Is`, not errors at an HTTP/CLI boundary that need classification. HTTP failures are communicated via status codes, not error returns.
- **Injector resolved at construction** — `New` captures the health-check capability into a `healthCheckFunc` via the `resolveHealthCheck` free function. Options write to a construction-only `config` struct, not the `Probe` directly. The Probe never stores `do.Injector` or `HealthRecorder` as fields, avoiding the injector-in-service anti-pattern (DO-6) and eliminating dead construction-only fields.
- **Panic recovery in health checks, fail closed** — `runHealthChecks` recovers panics from the recoverable surface (recorder implementations, batch machinery) and converts them to a synthetic `health-check` error wrapping `ErrPanicDuringHealthCheck`; `classify` maps any recovered panic to `fail` (503), never `warn`. Service `HealthCheck` panics on the injector path are process-fatal: samber/do runs each check in its own goroutine, so no probe-side recover can catch them. See [docs/panic-recovery-design.md](docs/panic-recovery-design.md).
- **Hook panics recovered, warn-not-fail (2026-10-09)** — the `WithEvaluationHook` call site is a separate surface: the hook runs after evidence collection completes, so a recovered hook panic degrades the response to a synthetic non-critical `evaluation-hook` warn row (sentinel `ErrPanicDuringEvaluationHook`), never `fail`, and never kills the refresh loop. The hook receives a defensive copy (own `Checks` map), so it cannot corrupt the served/cached response. The "recovered panics never warn" rule binds the data-collection surface only — see the design doc's evaluation-hook addendum.
- **Validate-on-Start** — `Start()` calls `Validate()` and returns an error on invalid configuration (zero/negative timeout, negative refresh interval). Fail-fast instead of silent runtime degradation.
- **Zero logging coupling** — the library does not import `log/slog` or any logging package. HTTP write failures (client disconnect) are silently swallowed. A library must not make logging decisions for the host application.
- **Observability via hook, not library** — `WithEvaluationHook` is the metrics/alerting seam; Prometheus/OpenTelemetry formats are consumer composition (docs/prometheus-exposition-design.md). No client_golang dependency.
- **Programmatic API mirrors handlers** — `Status/Alive/Ready/AwaitReady` read the cached view (never trigger checks); `Healthz` answers "route traffic here?"; `HealthCheck`/`AsShutdowner` make the probe a first-class do citizen.
- **Version endpoint is build identity, not health** — `VersionHandler(stamp)` is a free function (no Probe, no injector): it never evaluates checks, never 503s, and ignores shutdown. GET-only 405 + `Allow: GET` unconditionally (the method guard's posture without its config surface). The `version` field is always present (no omitempty): an unstamped binary answers `""`. Payload marshaled once at construction — version is immutable per process.
- **Per-check Since is probe-observed** — a `transitionTracker` stamps `Check.Since` inside `buildChecks` on every evaluation path (refresh, live, startup): the first batch reporting the current status, carried while it holds, restarted on change/reappearance, reset on process restart. Never service-reported (services cannot know their graded status). Liveness's empty checks and the Healthz synthetic `startup` check never carry Since.
- **Per-check Duration is executor-reported, opt-in** — `CheckDetail{Err, Duration}` is the internal seam; plain `map[string]error` sources are adapted with zero duration. `NewWithDetailedCheck` and the optional `DetailedHealthRecorder` interface populate `Check.DurationNanos` (int64 ns, omitzero). The raw injector path cannot: do's `HealthCheckWithContext` returns only errors (see docs/check-metadata-design.md).

### Decoupling from samber-do-auditlog

This package was extracted from [`samber-do-auditlog`](https://github.com/larsartmann/samber-do-auditlog) to eliminate the transitive dependency cost. The old `WithPlugin(p *auditlog.Plugin)` option is now `WithHealthRecorder(r HealthRecorder)`. The `*auditlog.Plugin` type implicitly satisfies `HealthRecorder` via its `RecordHealthCheckWithContext` method — pass it directly.

Migration guide for pre-extraction code: [docs/migration-plugin-to-recorder.md](docs/migration-plugin-to-recorder.md).

**doanalyzerv2:** the private AST analyzer (in-repo runner `tools/doanalyzerv2`; invoke
`(cd tools/doanalyzerv2 && go run . ..)`) reports 0 DO-1..DO-6 findings. It needs the
go-design-smells checkout at `/home/lars/projects/branching-flow` (the go.mod replace path).

**Consumer verification:** fleet compatibility inventory (15 direct + 16 indirect consumers, four implementation patterns), the per-release dashboard verification timeline, and the samber-do-auditlog reverse-dependency prohibition live in [docs/consumer-verification.md](docs/consumer-verification.md).

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
- Fuzz targets: `FuzzResponseMarshalDeterministic` + `FuzzHandlerInput` + `FuzzThrottleWindowBoundary` (root; the throttle fuzz drives the window rule on a fake clock), `FuzzAggregateMergeInvariants` (aggregate; source freshness modes — live/cache/throttled — derived from name parity so the corpus signature stays valid), `FuzzBatteries` (checks — pins the sentinel-or-nil error surface of the four batteries over untrusted inputs); run via `nix run .#fuzz`.
- **Seam-swap tests must not be parallel** — tests swapping a package-global
  seam (`marshalResponse` in both packages) mutate shared state, so they omit
  `t.Parallel()` (marked `//nolint:paralleltest`). A parallel seam test
  corrupts concurrent handler tests (observed: spurious 500s).

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
- **Tools that shell out to `go` need `goPkg` in their flake app** —
  `golangci-lint`, `govulncheck`, and `gosec` load packages by invoking a `go`
  binary from PATH; on CI (no Go on PATH) they fall back to the GOROOT they
  were compiled with, which cannot satisfy go.mod's `go 1.27` directive (the
  first CI run caught it). Fix: `goPkg` in every such app's `runtimeInputs`.
  Rule of thumb: any new flake app that indirectly runs `go` must list
  `goPkg` — the same host-shell-env leak class, one layer down.
- **`encoding/json/v2` does not sort map keys by default** — under v2 semantics `json.Marshal` serializes maps in random Go map order unless `json.Deterministic(true)` is passed (v1's always-sorted behavior was a compatibility default, not a v2 one). `writeResponse` opts in (handlers.go); `TestReadiness_JSONChecksAreSortedAlphabetically` guards the property. Any new marshal site must pass the option too.
- **`encoding/json/v2` cannot marshal `time.Duration` AT ALL** — no default representation exists (go.dev/issue/71631, undecided upstream) and no struct-tag format is accepted (verified empirically on go1.26.7: `int`, `ns`, `nanoseconds`, … all rejected); the only escape is the per-call `json.FormatDurationAsNano` option, which every re-marshaling consumer would have to know to pass. That is why `Check.DurationNanos` is a plain `int64` while the in-process seam `CheckDetail.Duration` stays `time.Duration`, converted once in `buildChecks`. Pinned by `TestCheck_JSONOmitZero`. Related v2 trap: scalar `omitempty` (bool/int) is not honored — only strings and `omitzero` omit; see `TestReadinessResponse_JSONOmitEmpty`.
- **erraudit enforcement flags are opt-in** — `--enforce-samber-oops` and `--enforce-go-error-family` flag stdlib constructors (`errors.New`, `fmt.Errorf`) as violations. These flags are for projects that have already adopted those libraries. This project deliberately uses stdlib errors, so the correct invocation is `erraudit ./... --type-aware` (reports 0 ERROR violations). Do not cargo-cult a library adoption to silence the linter — the sentinels are config-validation errors, not boundary errors needing classification.
- **`WithTimeout` is batch-level, not per-service** — the deadline is shared across all services in one evaluation. A slow dependency steals time from every other check. samber/do exposes `HealthCheckTimeout` (per-service) via `InjectorOpts` at injector creation time. See [docs/timeout-design.md](docs/timeout-design.md) for the full analysis, including why HTTP query-param timeout overrides are rejected (DoS amplifier + breaks caching).
- **`aggregate` sources must be eagerly invoked too** — the samber/do lazy-service gotcha applies per source: a source probe whose services were never invoked health-checks as pass, and the aggregate propagates that false confidence. Invoke critical services at boot.
- **Fuzz signature changes invalidate the testdata corpus** — corpus files in `testdata/fuzz/` are positional; changing an `f.Add`/`f.Fuzz` signature (adding a parameter) makes every saved corpus entry fail to load ("mismatched number of parameters"). Hand-edit the corpus files in the same commit as the signature change (see the 4-string update to `handlers_fuzz` seed `58b20d015136c6e5`), or accept losing the accumulated inputs. That cost is why the aggregate never-started path is unit-pinned instead of widening `FuzzAggregateMergeInvariants`.
- **Throttled live eval is the only path that stores into `latest` outside the refresh loop** — in live mode (`WithRefreshInterval(0)`) the background loop does not exist, so after `Start`'s one initial `refreshCache` (probe.go calls it unconditionally, even at interval 0) the cache holds a boot-time snapshot and only throttled readiness evaluations refresh it. A probe whose `Start` was never called has an empty cache: `CachedResponse()` returns the zero `Response` and an aggregate over such sources merges emptiness (pinned by `TestCachedResponse_NeverStartedSource`; test fixtures that need fresh state must prime via a throttled readiness request, see `newPrimedSource`/`newStateProbe`).
- **golangci's `nolint_filter` warning for `//nolint:erraudit` is expected noise** — erraudit is a standalone tool whose suppression engine honors `//nolint[:linter[:rule]]` (erraudit `pkg/suppression/parser.go`), but golangci-lint does not know `erraudit` as a linter and its `NolintFilter.Finish` warns unconditionally ("Found unknown linters in //nolint directives: erraudit", exit code still 0). No upstream suppression option exists. Do NOT switch those directives to bare `//nolint` to silence the warning — that would disable every golangci linter on the affected lines. Affects `handlers.go`, `aggregate/aggregate.go`, `federation/federation.go`.
- **This repo is BuildFlow-covered (`.buildflow.yml`, since 2026-10-08)** — `buildflow --fix --build-mode=full` is an additional quality surface alongside the flake gates; whether it REPLACES them is an open owner question (status report 2026-10-08 20:58 §g3). Budgets (art-dupl 80, branching-flow 14, go-auto-upgrade 8) are policy documents with rationale in the yaml — re-review them instead of letting them ratchet. Gotcha of record: a makezero regression reached master through this seam (`b2b9ed0`); see the makezero contract below.
- **makezero `always: true` contract (settled 2026-10-09, closing 20:58 §b1's open question)** — flags EVERY two-arg `x := make([]T, n)` bound to a plain identifier unless the length is the literal `0`; usage is irrelevant (append not required, index writes do not exempt). Makes inside composite literals (`field: make([]T, n)`) are INVISIBLE to the rule — the analyzer only visits assignment-statement RHS — which is why the pre-refactor `startup: make([]atomic.Bool, len(remotes))` passed while the refactored `states := make(...)` tripped: form, not usage. Blessed forms: `make([]T, 0, n)` + append, or index-written pre-sizing with a rationale `//nolint:makezero` (see federation.go `results`). Diagnostic trap: golangci's default `max-same-issues: 3` can hide additional identical findings in one run. Full contract also recorded as a comment at the `makezero:` block in `.golangci.yml`.
- **The golangci LSP integration lies** — its diagnostics panel has carried provably false typecheck errors and stale lint findings across sessions (observed 2026-10-08, 2026-10-03). Arbitrate every diagnostic with a real build (`nix run .#lint` / `.#test`); never fix what the panel claims without gate confirmation. Fixing or disabling the integration is a standing TODO.
- **Go-directive patch floors cascade through replace chains; the lever is the toolchain, not the directive** — `go mod tidy` auto-bumps a module's `go` directive to the highest patch requirement in its graph, so hand-lowering is unstable (empirical 2026-10-08: `go 1.27` → tidy → `go 1.27.1`, "switching to go1.27.2"). Chain: released samber-linter v0.4.0 declares `go 1.27.1` → branching-flow (go-design-smells) → `tools/doanalyzerv2` via the local replace. Consequence: any go 1.27.0 tool cannot load that graph; gopls shows the same error. Resolution status (re-verified 2026-10-09 against the rebuilt binary `1b99ae2`, doctor binary-freshness green): the full-mode spawn-env loss is NOT fixed upstream — `.buildflow.yml` `env: GOTOOLCHAIN: auto` and the `tools/bin` wrappers are honored in single-step mode but full-mode fan-out still spawns go 1.27.0 for some steps (a license-check wrapper in `tool_paths` was verifiably never reached; probe with `buildflow history --step ... --last-error`). Working invocation: export `GOTOOLCHAIN=auto` in the launching shell — full mode then reaches 95% (70/74 steps, 2026-10-09 run 20261009-160343); the four stragglers (license-check, govalid-generate, root + tools variants) pass single-step under the same env and are a BuildFlow upstream task (full-mode fan-out env/resolution), not a repo defect. Drop the wrappers + tool_paths once the graph no longer requires a newer patch than BuildFlow's toolchain AND full-mode spawn env is fixed upstream. The makezero seam note above stays load-bearing either way: full mode is a quality SURFACE, never the only gate — `nix run .#gates` remains canonical.

---

## Project Documentation

The per-doc index — FEATURES, TODO_LIST, ROADMAP, the design notes, the ADRs,
and the quality-tool suppressions — lives in [docs/INDEX.md](docs/INDEX.md)
(moved out of this file 2026-10-08 for headroom under the 220-line preflight
cap). Every docs-table row gained since then must land there.
**Critical-name validation (2026-10-02):** `Start()` fails with `ErrUnknownCriticalService` when a `WithCriticalServices` name never appears in the initial evaluation batch — closing the DiscordSync 2026-08-16 bug class (typo'd names silently blocked the startup latch and downgraded fail→warn, previously caught only by that repo's private guard test). Validation is batch-based by design, NOT `ListProvidedServices()`-based; decision + fleet compat scan: docs/start-validation-design.md.
