# Cookbook: per-check durations on the wire

How to get `duration_ns` into your health documents, depending on how your
probe's checks are sourced. Three paths, easiest first — plus the one path
that cannot report durations. Design rationale lives in
[check-metadata-design.md](check-metadata-design.md); the units policy
(why ns here, ms for roll-ups) lives in
[ADR-006](adr/ADR-006-duration-units.md).

Who computes what — the split that makes the metadata trustworthy:

- **`duration_ns` is executor-reported.** Only the code that runs a check
  knows how long it took. Zero means unknown and is _absent_ from JSON
  (`omitzero`) — a `0` would read as "instantly healthy", a lie.
- **`since` is probe-observed.** The probe stamps when a check _entered its
  current status_ (first batch reporting it, carried while it holds). Checks
  cannot report it, because a check cannot know its own graded status. It
  resets on process restart.

## Path 1: `NewChecks` — durations for free

If your checks are plain named functions, [`NewChecks`](../README.md) times
each one for you. No timing code, no metadata plumbing:

```go
probe := health.NewChecks(map[string]health.CheckFunc{
    "db":      checkDB,
    "cache":   checkCache,
    "stripe":  checkStripe, // non-critical: failure → warn, never 503
}, health.WithCriticalServices("db", "cache"))
```

Every execution lands on the wire with its duration, panic recovery, and
batch-deadline abandonment handled by the executor:

```json
"checks": {
  "db":     { "status": "pass", "duration_ns": 1432000 },
  "cache":  { "status": "pass", "duration_ns": 89400 },
  "stripe": { "status": "warn", "error": "timeout", "duration_ns": 5002168423 }
}
```

Use this unless you need full control of the batch.

## Path 2: `NewWithDetailedCheck` — self-timed composed checks

When you own the batch (composed checks, foreign DI containers, external
probe aggregation), return `CheckDetail` values instead of bare errors and
time each dependency at the call site:

```go
probe := health.NewWithDetailedCheck(func(ctx context.Context) map[string]health.CheckDetail {
    return map[string]health.CheckDetail{
        "db":    timed(ctx, pingDB),
        "cache": timed(ctx, pingCache),
    }
}, health.WithCriticalServices("db"))
```

with a small helper that keeps the timing honest — the context deadline is
the batch budget shared by all checks
([timeout-design.md](timeout-design.md)), so time the check, not the helper:

```go
func timed(ctx context.Context, check func(context.Context) error) health.CheckDetail {
    start := time.Now()
    err := check(ctx)

    return health.CheckDetail{Err: err, Duration: time.Since(start)}
}
```

Rules the probe enforces on this path: `Err` is graded against the critical
set exactly like a plain `map[string]error` result; `Duration` is carried
through unchanged (zero → omitted); a panic in the function is recovered
fail-closed; the function must be safe for concurrent use (the refresh loop
and live handlers can both invoke it).

Fan-out note: if you run checks concurrently with a `WaitGroup`/errgroup,
time each check inside its own goroutine around its own call — a shared
stopwatch measured before `wg.Wait()` reports the _slowest_ check's duration
as every check's duration.

## Path 3: `DetailedHealthRecorder` — upgrading a recorder

A recorder (e.g. an audit log) registered via `WithHealthRecorder` can
optionally implement one extra method; the probe detects it by type
assertion at construction and switches to the detailed batch:

```go
type DetailedHealthRecorder interface {
    RecordDetailedHealthCheckWithContext(
        ctx context.Context,
        injector do.Injector,
    ) map[string]health.CheckDetail
}
```

A recorder that implements only `HealthRecorder` keeps working — durations
stay absent, nothing breaks. Implement the detailed method when your recorder
already times checks internally and you want that on the wire.

## The path that cannot report durations: `New(injector)`

The raw samber/do path gets per-service results from
`do.HealthCheckWithContext`, which returns only `map[string]error` — there is
no timing channel. `duration_ns` stays absent for every check on this path,
and no probe-side option can change it (wrapping the injector's services in
timers would change service identity, and a batch-level stopwatch cannot
attribute time per service).

Escape hatches, in order of preference:

1. Restructure with `NewChecks`/`NewWithDetailedCheck` alongside the injector
   (resolve dependencies eagerly at boot, call them from your own checks).
2. A recorder that implements `DetailedHealthRecorder` (path 3).
3. Upstream: samber/do exposing per-service timing would light this path up
   for free (tracked in TODO_LIST.md).

## Reading the numbers

- Durations are **per execution**, not per request: a cached readiness
  response reports the duration of the _background refresh's_ batch, not of
  your HTTP call.
- A check that hits the batch deadline reports whatever duration it reached
  when the batch gave up on it (fail-closed abandonment in path 1).
- `since` + `duration_ns` together answer "how long has this check been
  failing" and "how long does a passing check take" — the two questions
  dashboards actually ask.
