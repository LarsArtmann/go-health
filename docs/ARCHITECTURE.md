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
