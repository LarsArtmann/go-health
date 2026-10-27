# Architecture — go-health

Per-file and per-package map for `github.com/larsartmann/go-health`. Moved out
of `AGENTS.md` on 2026-10-27 (item 40 of the supplier-wave queue) so the
session-preflight file stays under its 220-line cap; the design decisions and
gotchas stay in [AGENTS.md](../AGENTS.md), the per-document index in
[INDEX.md](INDEX.md).

Keep this page current with any file that is added, split, or renamed — a stale
map is worse than none, because it teaches the wrong shape.

---

Single-package library (`health`) with these source files:

```
doc.go           — Package doc comment (quick start, three-probe rationale, caching, shutdown)
types.go         — Status enum (check level: pass/fail/warn/off; the roll-up stays pass/fail/warn), Off/OffError configured-off sentinel, Check (incl. Since, DurationNanos), CheckDetail, Response data model (incl. instance_id, timestamp omitzero) + Response.Clone (deep-copies Checks)
probe.go         — Probe struct, config struct, Option functional options (write to config), HealthRecorder + DetailedHealthRecorder interfaces, New(), resolveHealthCheck (free function), Validate() (incl. ErrRecorderNotApplicable), lifecycle (Start/Shutdown/MarkShuttingDown), guard (method-set enforcement), now/uptime clock seam, Evaluate (no publish), EvaluateAndCache, CachedResponse (lock-free read-only view + shutdown overlay), CacheAge/CachedResponseFresh, accessors, runHealthChecks (with panic recovery), buildChecks (Since stamping + duration carry), errorsOf/detailOf adapters
tracker.go       — transitionTracker: per-check status-transition tracking behind a mutex; stamps Check.Since inside buildChecks (probe-observed, prunes absent checks)
classifier.go    — Read-only classifier: classify (three-state), evaluateStartup, per-check grading; constructed once, evaluated lock-free
handlers.go      — LivenessHandler, ReadinessHandler, StartupHandler, RegisterRoutes (empty patterns skipped), Routes, DefaultRoutes, readinessResponse/throttledLiveResponse, writeResponse + SanitizeResponse (UTF-8 coercion), marshalResponse (test seam)
accessors.go     — ErrProbeUnhealthy, HealthCheckFunc, NewWithHealthCheck, DetailedHealthCheckFunc, NewWithDetailedCheck, NewWithRecorder (injector-free recorder constructor, new_with_recorder.go), Status/Alive/Ready, AwaitReady, HealthCheck (do conformance), ProbeShutdowner/AsShutdowner, Healthz + applyStartupFailureOverlay (shared overlay shape with aggregate/federation)
checks.go        — CheckFunc + NewChecks (injector-free named-check constructor: concurrent execution, per-check duration, panic recovery, nil fail-closed, batch-deadline abandonment), runNamedChecks/runBoundedCheck executors — design: docs/named-checks-design.md
recorder.go      — ChainRecorders (ordered, nil-skipping, detailed-duration-preserving) + HealthRecorderFunc/DetailedHealthRecorderFunc adapters; service_name.go — DoServiceName[T]/WithCriticalService[T] (do's inferred key via do's own naming helper)
version.go       — VersionHandler: build-version endpoint (ldflags/VCS stamp as {"version":"..."}); GET-only 405 unconditionally, UTF-8-coerced, payload pre-marshaled at construction, marshalVersionResponse test seam; wire shape in docs/openapi.yaml (VersionResponse)
export_test.go   — ResetStartupLatchForTest / SetTrackerDisabledForTest (test builds only; public latch stays one-way)
```

Sub-package `aggregate` (source: `aggregate/aggregate.go`) merges N in-process probes into one
`health.Response`: `Source{Name, Probe}`, `New(sources...)` (rejects empty, duplicate, slash-containing, or nil-probe
sources — docs/aggregate-source-name-design.md), merge-on-read `CachedResponse` (N lock-free loads onto the shared
`internal/merge` primitive), `StartupComplete` (AND of latches), the three kubelet handlers, and a standalone
`Healthz()` handler (docs/aggregate-healthz-design.md).

Sub-package `federation` (source: `federation/federation.go`, `federation/healthz.go`) is the network sibling: it pulls
N remote go-health instances over HTTP into one `health.Response` — same source-name contract as aggregate, merge-on-read
with one parallel fetch per remote mapped onto the shared `internal/merge` primitive (fetch error → `FetchErr`), per-remote
startup latches (advanced federation-side on successful fetch, never inside the merge), and one synthetic
`name/reachable` FAIL check per unreachable/undecodable remote (never a silent freeze, never a status override). Wire input
is untrusted (invalid documents refused whole); `Check.Since`/`DurationNanos` survive verbatim, scalars do not.
`WithClient(*http.Client)` is the transport escape hatch (pooling, TLS, proxy); `MarkShuttingDown()` adds a one-way hub-side
drain (readiness 503 with `shutting_down: true`, fetches continue so bodies stay fresh; liveness/startup unaffected).
`SourceStatuses()` mirrors aggregate's per-remote roll-up (unreachable/draining remotes report fail; no latch movement).
`Healthz()` completes single-endpoint parity (503 while any latch is unset or the merged roll-up is fail; warn stays 200).
Full semantics (fetch caps, dashboard-`Prober` structural assertion, API surface): docs/federation-design.md +
docs/federation-source-statuses-design.md + docs/federation-healthz-design.md; remote names never hit
`ErrUnknownCriticalService`: docs/federation-validation-semantics.md.

Sub-package `checks` (source: `checks/checks.go`) is the batteries package: zero-dependency,
stdlib-only `func(ctx) error` constructors — `Disk`, `Memory`, `HTTP`, `Database`, each with a
documented sentinel error — composing with `NewChecks`, `NewWithHealthCheck`, or any recorder
path. Resource checks are warn-by-default: criticality stays with the caller; thresholds are
arguments, not constants. Ownership decision + constraints:
docs/batteries-ownership-decision.md; tests use an in-process `sql.Register` fake driver.

Sub-package `internal/merge` (source: `internal/merge/merge.go`) is the single home of the worst-of fold behind both
merge-on-read packages (docs/merge-unification-design.md): `Source{Name, Response, FetchErr}` + `Responses(sources)` — worst-of
via `Rank` with tie stability (`off`/unknown roll-ups can't leak in; order-independent), `"source/check"` namespacing, shutdown
overlay, `"name/reachable"` fail for fetch-errored sources, max-latency, scalars dropped. Pure; the collapsed merge fuzz suite lives here.

---

## Test layout

| Layer                       | Files                                                                                                                       |
| --------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| Root black-box (`health_test`) | `probe_test.go`, `handlers_*_test.go`, `accessors_test.go`, `recorder_test.go`, `checks_test.go`, `configured_off_test.go`, `version_test.go`, `new_with_recorder_test.go`, `service_name_test.go`, `probe_*_test.go` |
| Root white-box (`health`)      | `export_test.go`, `handlers_internal_test.go`, `accessors_internal_test.go`, `version_internal_test.go`, `tracker_internal_test.go` |
| Sub-packages                | one `*_test.go` family per package plus an `*_internal_test.go` where an unexported seam needs pinning                        |
| Fuzz                        | `handlers_fuzz_test.go` (root), `aggregate/`, `checks/`, `federation/` (×3 targets), `internal/merge/` — all registered in both `fuzz` apps, parity enforced by `nix run .#fuzz-registry` |
| Benchmarks                  | `probe_benchmark_test.go`, `probe_test.go`, `aggregate/aggregate_benchmark_test.go`, `federation/federation_benchmark_test.go` |

---

## Exported API surface

These blocks are the published surface, and `nix run .#docs-check` compares them
against the code in both directions: a symbol added to a package without a line
here (or a line here without a symbol) fails the gate. Before the check existed,
`SourceStatuses` and `Healthz` shipped, were tested, and were missing from the
federation block for weeks. Keep every block current in the same change that
touches the symbol.

`federation`'s block lives in [federation-design.md](federation-design.md)
because that document owns the network semantics; the check reads it there.

### Root package `health`

```go api:root
type Probe struct
type Response struct
type Check struct
type CheckDetail struct
type Status string // pass | warn | fail | off
type Routes struct
type Option func(*config)
type OffError struct
type ProbeShutdowner struct
type CheckFunc func(ctx context.Context) error
type HealthRecorder interface
type DetailedHealthRecorder interface
type HealthRecorderFunc func(ctx context.Context, injector do.Injector) map[string]error
type DetailedHealthRecorderFunc func(ctx context.Context, injector do.Injector) map[string]CheckDetail
type HealthCheckFunc func(ctx context.Context) map[string]error
type DetailedHealthCheckFunc func(ctx context.Context) map[string]CheckDetail

// The recorder contract. Segregated on purpose: do's injector path only
// supplies names, so a recorder that needs durations implements the detailed
// variant and everything else keeps working.
func (r HealthRecorder) RecordHealthCheckWithContext(ctx context.Context, injector do.Injector) map[string]error
func (r DetailedHealthRecorder) RecordDetailedHealthCheckWithContext(ctx context.Context, injector do.Injector) map[string]CheckDetail
func (e *OffError) Error() string
func (s Status) Rank() int // pass-tier ordering; off never reaches a roll-up
func (r Response) Clone() Response // deep-copies Checks; CachedResponse aliases them

// Construction. WithHealthRecorder is refused on the three paths that build
// their own recorder (NewWith*Check, NewChecks) and at Start on the injector
// path, so a recorder is never silently dropped.
func New(injector do.Injector, opts ...Option) *Probe
func NewWithHealthCheck(fn HealthCheckFunc, opts ...Option) *Probe
func NewWithDetailedCheck(fn DetailedHealthCheckFunc, opts ...Option) *Probe
func NewWithRecorder(recorder HealthRecorder, opts ...Option) *Probe
func NewChecks(checks map[string]CheckFunc, opts ...Option) *Probe
func ChainRecorders(recorders ...HealthRecorder) HealthRecorder
func (p *Probe) Validate() error
func DefaultRoutes() Routes

// Lifecycle. Shutdown is one-way: Start after it does not re-arm readiness.
func (p *Probe) Start(ctx context.Context) error
func (p *Probe) Shutdown()
func (p *Probe) MarkShuttingDown()
func (p *Probe) AsShutdowner() ProbeShutdowner
func (p *Probe) HealthCheck(_ context.Context) error

// Evaluation. CachedResponse is lock-free; its Checks map aliases the
// published snapshot, so callers that mutate use Response.Clone.
func (p *Probe) Evaluate(ctx context.Context) Response
func (p *Probe) EvaluateAndCache(ctx context.Context) Response
func (p *Probe) CachedResponse() Response
func (p *Probe) CachedResponseFresh(maxAge time.Duration) (Response, bool)
func (p *Probe) CacheAge() (time.Duration, bool)
func (p *Probe) Status() Status
func (p *Probe) Alive() bool
func (p *Probe) Ready() bool // "not currently failing", not a startup gate
func (p *Probe) AwaitReady(ctx context.Context) error
func (p *Probe) StartupComplete() bool // the one-way startup latch
func (p *Probe) RefreshInterval() time.Duration

// HTTP surface.
func (p *Probe) LivenessHandler() http.HandlerFunc
func (p *Probe) ReadinessHandler() http.HandlerFunc
func (p *Probe) StartupHandler() http.HandlerFunc
func (p *Probe) Healthz() http.HandlerFunc
func (p *Probe) RegisterRoutes(mux *http.ServeMux, routes Routes)
func VersionHandler(version string) http.HandlerFunc
func SanitizeResponse(resp Response) Response

// Options.
func WithRefreshInterval(d time.Duration) Option
func WithTimeout(d time.Duration) Option
func WithShutdownGracePeriod(d time.Duration) Option
func WithLiveThrottle(d time.Duration) Option
func WithAllowedMethods(methods ...string) Option
func WithInstanceID(id string) Option
func WithVersion(v string) Option
func WithBootTime(t time.Time) Option
func WithNowFunc(fn func() time.Time) Option
func WithCriticalServices(names ...string) Option
func WithCriticalService[T any]() Option
func WithEvaluationHook(fn func(Response)) Option
func WithHealthRecorder(r HealthRecorder) Option
func DoServiceName[T any]() string
func Off(detail string) *OffError
```

### Sub-package `aggregate`

```go api:aggregate
type Source struct {
    Name  string
    Probe Proberish // any surface with CachedResponse + StartupComplete
}

type Aggregate struct

func New(sources ...Source) (*Aggregate, error)
func (a *Aggregate) CachedResponse() health.Response
func (a *Aggregate) SourceStatuses() map[string]health.Status
func (a *Aggregate) RefreshInterval() time.Duration
func (a *Aggregate) StartupComplete() bool
func (a *Aggregate) LivenessHandler() http.HandlerFunc
func (a *Aggregate) ReadinessHandler() http.HandlerFunc
func (a *Aggregate) StartupHandler() http.HandlerFunc
func (a *Aggregate) Healthz() http.HandlerFunc
func (a *Aggregate) RegisterRoutes(mux *http.ServeMux, routes health.Routes)
```
