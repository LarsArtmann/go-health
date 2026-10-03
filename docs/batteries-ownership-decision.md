# Decision: system-check "batteries" ownership

**Date:** 2026-10-02 · **Status:** DECIDED — `health/checks` subpackage in the core module, zero dependencies

## The duplication (verified in source)

| Consumer                                            | Implementation                                                                         | Semantics                                                          |
| --------------------------------------------------- | -------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| CV `internal/health/systemresources.go` (136 lines) | filesystem writability + memory MB thresholds + disk %-used via `syscall`              | non-critical (warn): "a full disk must ALERT, not restart the pod" |
| fir `pkg/injector/health_checks.go` (133 lines)     | `syscall.Statfs` disk GB-free (<1 GB critical), process age, AI provider, file watcher | mixed: disk critical, age warn                                     |

Same underlying need (statfs/memory pressure), independently invented, with
divergent criticality and threshold vocabularies. Every future consumer
without a batteries option re-invents them.

## Options considered

| Option                                 | Verdict    | Reason                                                                                                                                                                                                                                                                                                                 |
| -------------------------------------- | ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **`health/checks` subpackage in core** | **CHOSEN** | The checks are small, pure-stdlib (`syscall`, `runtime`, `net/http`, `database/sql`), and semantically tied to go-health's pass/warn/fail vocabulary — putting them anywhere else forces a second dependency or copy-paste. Subpackage (not root) keeps the root import graph clean for consumers who don't want them. |
| Separate contrib repo/module           | Rejected   | A whole repository for ~200 lines of stdlib checks is overhead without a payoff; version-skew coordination with core would be constant.                                                                                                                                                                                |
| go-appkit/health                       | Rejected   | Reaches only the 2 bridge consumers; core batteries reach all 30. go-appkit can re-export later if it wants.                                                                                                                                                                                                           |

## Design constraints (binding for C2)

1. **Zero dependencies** — stdlib only. `database/sql` handles the DB check
   generically; no driver imports.
2. **Warn-by-default posture** — resource checks (Disk, Memory) return
   errors meant to grade `warn`; whether a check is critical stays with the
   caller via `WithCriticalServices`/`NewChecks` composition, exactly like
   every other check. CV's "alert, don't restart" insight is the default
   contract; fir's critical-disk remains available by listing the name as
   critical.
3. **Thresholds are arguments, not constants** — consumers proved
   thresholds are policy (CV 85%/97.3% used; fir 1 GB free).
4. **`InvokeCritical` helper: rejected** — with `ErrUnknownCriticalService`
   validation at `Start()`, the eager-invocation gotcha is now guarded at
   boot; a reflection helper would add magic to save one line of
   `do.MustInvokeNamed`.

## Shipped surface (C2)

`health/checks`: `Disk(path string, minFreeBytes uint64)`,
`Memory(minFreeBytes uint64)`, `HTTP(url string, timeout time.Duration)`,
`Database(db *sql.DB, pingTimeout time.Duration)` — each a
`health.CheckFunc`-compatible `func(ctx context.Context) error`.
