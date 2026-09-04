# Migrating from `WithPlugin` to `WithHealthRecorder`

> Applies to: code that used the pre-extraction `go-health` API inside
> [samber-do-auditlog](https://github.com/larsartmann/samber-do-auditlog).
> Effort: ~5 minutes for a typical integration.

## What changed

go-health was extracted from `samber-do-auditlog` to remove the transitive
dependency cost: the health SDK no longer imports the audit library (and vice
versa — verified 2026-09-04, `samber-do-auditlog` does not depend on
go-health). The audit-specific option was replaced by a generic interface.

| Before (samber-do-auditlog monolith)    | After (go-health v0.0.1+)                     |
| --------------------------------------- | --------------------------------------------- |
| `health.WithPlugin(p *auditlog.Plugin)` | `health.WithHealthRecorder(r HealthRecorder)` |

`HealthRecorder` is a one-method interface:

```go
type HealthRecorder interface {
	RecordHealthCheckWithContext(ctx context.Context, injector do.Injector) map[string]error
}
```

## Migrating

### Before

```go
import (
	health "github.com/larsartmann/go-health"
	"github.com/larsartmann/samber-do-auditlog"
)

plugin, err := auditlog.New(auditlog.Config{Enabled: true})
if err != nil {
	log.Fatal(err)
}

probe := health.New(injector, health.WithPlugin(plugin))
```

### After

```go
import (
	health "github.com/larsartmann/go-health"
	"github.com/larsartmann/samber-do-auditlog"
)

plugin, err := auditlog.New(auditlog.Config{Enabled: true})
if err != nil {
	log.Fatal(err)
}

// *auditlog.Plugin satisfies HealthRecorder implicitly — pass it directly.
probe := health.New(injector, health.WithHealthRecorder(plugin))
```

That is the entire change: rename the option. No wrapping, no adapter, no
configuration transfer.

## Writing your own recorder

Any type with the method above works — you are not locked into the auditlog
plugin:

```go
type promRecorder struct {
	registry *prometheus.Registry // any state you need
}

func (r *promRecorder) RecordHealthCheckWithContext(
	ctx context.Context,
	injector do.Injector,
) map[string]error {
	results := injector.HealthCheckWithContext(ctx)
	// observe results ...
	return results
}

probe := health.New(injector, health.WithHealthRecorder(&promRecorder{}))
```

The returned map is the health-check result the probe classifies and serves:
return the real results (or your own errors for entries you add), never nil on
the happy path — a nil map is treated as "no services checked".

## Semantics worth knowing

- When a recorder is configured, the probe delegates the **entire batch** to
  it. `injector.HealthCheckWithContext(ctx)` is NOT called by the probe; the
  recorder decides how (and whether) services are checked.
- A panic inside the recorder is recovered and reported as a synthetic
  `health-check` failure that rolls readiness up to 503 (fail closed — see
  [panic-recovery-design.md](panic-recovery-design.md)).

## Verification

The "after" integration is compile-verified against go-health HEAD with
`samber-do-auditlog v0.10.0` and passes a live end-to-end probe
(`Start` → `Evaluate` → pass).
