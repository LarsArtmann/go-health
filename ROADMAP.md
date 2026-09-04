# Roadmap

> Long-term direction and raw ideas. Items here are NOT actionable tasks.
> When an idea is refined into bounded work, it moves to TODO_LIST.md.
>
> Re-curated 2026-09-04 evening (docs-health run): ideas shipped through v0.1.2
> moved to FEATURES.md/CHANGELOG.md; decided-against ideas live in the Non-goals
> section and theme notes with design-note links; unharvested report ideas
> routed into the theme raw-idea lists below.

## Themes

### 1. Programmatic Health API — core shipped in v0.1.0

`Status()`, `Alive()`, `Ready()`, `AwaitReady(ctx)`, `Healthz()`,
`NewWithHealthCheck(fn, opts...)`, and `Probe.HealthCheck` conformance all
shipped. Godoc examples: seven programmatic-API examples shipped in v0.1.2;
the shutdown-grace, AsShutdowner, and aggregate examples landed right after
(sit in `[Unreleased]`). Remaining raw ideas:
- `AwaitReady` with a cache-aware poll interval (respect the source's
  refresh interval instead of a fixed 50ms poll)
- Aggregate `Healthz` parity → promoted to a v0.2.0 candidate (Theme 7)

### 2. Observability & Diagnostics

The library deliberately has zero logging coupling and no metrics export.
`WithEvaluationHook` (shipped) is the observability seam; formats are a
composition concern (see [prometheus-exposition-design.md](docs/prometheus-exposition-design.md)).

Raw ideas:

- OpenTelemetry spans on Evaluate/checks (same seam: hook + trace context)
- `Response.TotalLatencyMs` as `float64` for sub-millisecond precision
- `Probe.Snapshot()`-style accessor for structured-logging consumers
  (only with a concrete consumer need)
- ~~Per-service latency in `Check`~~ — infeasible in core, `samber/do` owns
  the batch; see [classification-2.0-design.md](docs/classification-2.0-design.md) §4

### 3. Operational Hardening

Shipped: `WithLiveThrottle` (request-flood coalescing) and
`WithShutdownGracePeriod` (two-phase shutdown timing).

Decided against (see [classification-2.0-design.md](docs/classification-2.0-design.md)):
circuit-breaker in core, `WithMaxConcurrentChecks`, per-service result
caching. Escape hatch for all three: a custom `HealthRecorder` owns batch
execution.

### 4. Container & Ecosystem Integration

Shipped: `Probe.HealthCheck` (do.HealthcheckerWithContext self-registration),
`AsShutdowner` / `ProbeShutdowner` (do.ShutdownerWithError).

Raw ideas:

- Remove `do.Injector` from the `HealthRecorder` interface signature —
  deferred: ADR-004 froze the interface for v0.x; revisit at v1.0

Decided against (see [multi-tenant-design.md](docs/multi-tenant-design.md)):
`WithCriticalService` runtime toggle, child-scope isolation (N probes +
aggregate), `WithProbeName` (aggregate namespacing + `WithInstanceID` cover it).

### 5. Protocol & Format Flexibility

Shipped: `Response.InstanceID` + `WithInstanceID`, static OpenAPI spec
([docs/openapi.yaml](docs/openapi.yaml)), Prometheus exposition via composition
([docs/prometheus-exposition-design.md](docs/prometheus-exposition-design.md)).

Raw ideas:

- ETag/`If-None-Match` caching headers on health endpoints — write the
  rejection note (caching is a proxy/CDN composition concern) before someone
  asks for it

Decided against (see [starting-status-design.md](docs/starting-status-design.md)):
"starting" Status, Status validation (no injection boundary exists).

### 6. Testability & Internal Architecture — core shipped in v0.1.0

`classifier` extraction, `ResetStartupLatchForTest` (test builds only; the
public latch stays one-way), `WithNowFunc` clock seam (deterministic uptime,
timestamps, and throttle windows), `WithAllowedMethods` method-set guard.
Middleware: no library concept needed — handlers are plain `http.HandlerFunc`
([middleware-design.md](docs/middleware-design.md)).

Raw ideas (quality polish, none scheduled):

- Aggregate handler (HTTP-path) benchmarks complementing the merge benchmark
- Feed golden-fixture inputs into the aggregate fuzz seed corpus
- Benchmark the throttled live path under contention (parallel load)
- Fuzz the throttle-window boundary under concurrency with a fake clock
- Combine the aggregate handler fuzz with throttle/cache modes (currently
  fuzzed only over started+cached sources)
- `-count=N` race-suite stress in CI if flakiness stays at zero

### 7. Release & Ecosystem Strategy

How the v0.x line matures.

#### v0.2.0 candidates (feature-driven, unscheduled)

Scoped 2026-09-04 from the open idea inventory. All are additive;
the first two carry a written design, the `Healthz` parity decision note is
still to be written:

- `errors.Join` in `aggregate.New` — report all invalid sources instead of
  the first ([docs/errors-join-design.md](docs/errors-join-design.md), spike verified)
- `Aggregate.SourceStatuses()` — per-source roll-up accessor
  ([docs/aggregate-per-source-visibility-design.md](docs/aggregate-per-source-visibility-design.md))
- Aggregate `Healthz` parity: one combined endpoint across all sources
  (decide whether worst-of-N belongs in a single 200/503 answer; no design
  note yet — write it before implementing)
- Toolchain floor bump: go.mod directive → 1.27, drop
  `GOEXPERIMENT=jsonv2` from the flake — verified to need no code changes
  (see AGENTS.md GOEXPERIMENT gotcha); ship when 1.26 support is dropped

#### v1.0 criteria draft

What "1.0" will mean here — none of this is promised yet:

- **API freeze:** the JSON wire format, the three-probe contract, and the
  full public surface (options, `Response`, `Probe` methods, the aggregate
  API) carry explicit backward-compatibility promises
- **Deprecation burn-down:** `WithGETOnly` removal happens no earlier than
  v1.0 (it is kept through all of v0.x per
  [docs/deprecation-policy.md](docs/deprecation-policy.md)); the
  `HealthRecorder` signature revisit (ADR-004) lands here if at all
- **Toolchain floor:** go.mod ≥ 1.27 so json/v2 needs no experiment
- **Consumer proof:** at least the known consumer
  (go-health-dashboard) running the released module without a replace

#### Release automation — DECIDED 2026-09-04: manual tag flow adopted

Rationale: this is a pure-Go library — no binaries, archives, or checksums
to produce, which is the work GoReleaser automates. The manual flow
(CHANGELOG cut → `nix run .#gates` → annotated tag → `gh release create` →
proxy/pkg.go.dev verify) has shipped three releases cleanly (v0.1.0–v0.1.2).
GoReleaser would add config and CI surface for zero deliverables. Revisit
triggers: shipping a CLI binary, or a sustained cadence above ~4
releases/year.

#### Internal architecture — seam extraction rejected 2026-09-04

The root and aggregate packages each carry a ~15-line `marshalResponse` /
`writeResponse` seam. Extracting a shared internal package would couple the
two packages' evolution to deduplicate code that is duplicated on purpose:
each package stays independently readable, and the aggregate deliberately
mirrors root semantics without importing root internals. Revisit only if a
third write seam appears.

Raw ideas, none scheduled:

- Promote `erraudit` / `doanalyzerv2` from local gates to CI steps if either
  tool ever becomes public
- Dependency automation: extend Dependabot/Renovate to flake inputs + pinned
  action SHAs (subsumes Go 1.26.x patch tracking; auto-merge rules are a
  separate policy decision)
- Non-nix CI matrix job (plain `go test`) to widen the honestly-tested OS/arch
  statement beyond linux/amd64; arm64 native runner evaluation if QEMU stays
  too slow for race jobs
- Raise the per-push fuzz budget above 10s/target if CI cost allows
- Editor-experience: suppress the gopls stdversion warning while GOEXPERIMENT
  stays enabled (AGENTS gotcha documents it as benign; noise only)

## Non-goals

Things we are deliberately NOT pursuing and why:

- **Per-service timeout within this library:** samber/do already provides
  `do.WithHealthCheckTimeout`. This library controls only the outer batch
  deadline. See [docs/timeout-design.md](docs/timeout-design.md).
- **Direct logging:** Libraries must not make logging decisions for the host
  application. A `WithLogger` option was considered and rejected as
  anti-pattern coupling.
- **Error library adoption:** Sentinel errors via stdlib `errors.New` /
  `fmt.Errorf` are sufficient for config-validation errors matched via
  `errors.Is`. See AGENTS.md "Stdlib errors by design".
- **Non-stdlib serialization:** Responses serialize through the stdlib
  `encoding/json/v2` package. Third-party serialization stays out of scope.
- **Content negotiation / HTML rendering:** formats are a composition
  concern. See [docs/content-negotiation-design.md](docs/content-negotiation-design.md).
- **Health-check weights, in-probe circuit breakers, per-service caching,
  batch concurrency limits:** analyzed and rejected;
  see [docs/classification-2.0-design.md](docs/classification-2.0-design.md).
- **Multi-tenant primitives (child scopes, probe names, runtime criticality
  toggles):** N probes + health/aggregate composition covers the topology;
  see [docs/multi-tenant-design.md](docs/multi-tenant-design.md).
- **Fourth Status value ("starting") and Status input validation:**
  see [docs/starting-status-design.md](docs/starting-status-design.md).
