# ADR-004: HealthRecorder stays a minimal structural interface

> Status: ACCEPTED · Date: 2026-09-04 · Deciders: maintainer
> Gate: this decision was blocked pending consumer verification (P5). The
> gate resolved 2026-09-04: `go-health-dashboard` is a live consumer on
> v0.1.0; `samber-do-auditlog` does not import go-health.

## Context

`HealthRecorder` replaced the concrete `*auditlog.Plugin` dependency during
the extraction from samber-do-auditlog:

```go
type HealthRecorder interface {
	RecordHealthCheckWithContext(ctx context.Context, injector do.Injector) map[string]error
}
```

Because it is structural (implicit), the plugin satisfies it without
importing go-health. Open question resolved by this ADR: should the surface
evolve — narrow the signature, move it behind an adapter, or add options to
it — now that the consumer situation is known?

## Decision

Keep the interface exactly as-is.

1. **No signature change.** The only known consumer compiles against HEAD;
   any change to the one-method interface is a breaking change requiring a
   coordinated migration (semver major, or a deprecation cycle at minimum).
   The value of a cosmetic reshape does not cover that cost.
2. **No concrete dependency reintroduced.** The extraction happened to cut
   the transitive dependency cost; wiring auditlog back in (even optionally)
   would re-create it.
3. **Structural satisfaction is the feature.** `auditlog.Plugin` passes
   `WithHealthRecorder(plugin)` with zero glue — verified end-to-end against
   `samber-do-auditlog v0.10.0` (compile + live probe). Adapters, generics,
   or wrappers would only add ceremony.
4. **New observability goes through separate seams.** Metrics-style needs
   use `WithEvaluationHook`; they do not grow HealthRecorder.

## Consequences

- Consumers keep compiling; `WithPlugin`-era code needs only the option
  rename documented in [docs/migration-plugin-to-recorder.md](../migration-plugin-to-recorder.md).
- The interface cannot gain methods without breaking implementers — if a
  second method is ever justified, add a NEW interface and type-assert, do
  not extend this one.

## Alternatives rejected

- **Narrow to `func(ctx, do.Injector) map[string]error`.** Function-typed
  recorders lose named identity for logging/registration and buy nothing
  the hook option does not already cover.
- **Move to an adapter type (`health.RecorderFrom(auditlog.New(...))`).**
  Forces every consumer to wrap; breaks the "pass it directly" promise that
  made the extraction cheap.
- **Accept any `func(...)` via `WithEvaluationHook` and deprecate
  HealthRecorder.** The recorder REPLACES batch execution (it decides how
  services are checked); the hook only observes. They are different roles.
