# Draft: comment on samber/do#318 — per-service duration on `HealthOutcome`

- **Target**: https://github.com/samber/do/issues/318 (own issue, open)
- **Action**: post comment (owner call — not filed; third-party repo)
- **Verified against**: samber/do v2.1.0 (module cache) + `master` scope.go
  (identical, 2026-09-22; **line numbers re-verified against master
  2026-10-09** — see B052 note below); go-health v0.3.0 behavior (scratch
  test, this repo)
- **Prior-art search**: `duration OR timing OR elapsed` and `health check`
  across samber/do issues+PRs — no timing proposal exists (#318 is states-only;
  #195 is hooks; #317 is transient-nil). Re-checked 2026-10-09: #318 still
  has 0 comments, no evolution.
- **B052 re-verification (2026-10-09, master)**: `HealthCheckWithContext`
  still `scope.go:307`; the batch machinery was refactored —
  `serviceHealthCheck`/`raceWithTimeout` (old `scope.go:735/:752`) now live
  in `queueServiceHealthcheck` (`root_scope.go:213`, per-service timeout at
  `:228-241`, pool path `:245`); shutdown per-service timing now stamped at
  `scope.go:496` into the `:434` map (report `:517`); `service_lazy.go`
  `buildTime` `:110` + getter `:241` unchanged. Post body below already uses
  these current numbers.

---

The same "thrown away at the boundary" argument covers per-service timing, not just states.

`HealthCheckWithContext` returns `map[string]error` (v2.1.0 `scope.go:307`; identical on `master`, re-verified 2026-10-09). The wall-clock of each check exists right up until it doesn't: `queueServiceHealthcheck` (`root_scope.go:213` on `master`; v2.1.0 called this `serviceHealthCheck`/`raceWithTimeout`) invokes the actual check under a per-service timeout (`root_scope.go:228-241`, pool path `:245`), and that duration is discarded — the caller-facing channel carries only `error`.

do already ships this exact pattern one batch over: shutdown collects per-service durations into `ShutdownReport.ServiceShutdownTime map[ServiceDescription]time.Duration` (field map `scope.go:434`, stamped at `scope.go:496` with `time.Now`/`time.Since` around each service, report at `:517`). Lazy services even keep `buildTime` (`service_lazy.go:110`, getter at `:241`). Health checks are the one batch that measures everything and reports nothing.

Why not measure outside do: my [go-health](https://github.com/LarsArtmann/go-health) health documents carry per-check `duration_ns`, but the samber/do injector path can never populate it — `map[string]error` has no timing channel, while go-health's own batch executors (`NewChecks`, `NewWithDetailedCheck`) do. Wrapping services in external timers means re-registering them (identity changes), and a batch-level stopwatch can't attribute time per service when checks run concurrently under `HealthCheckParallelism`.

**Proposal: add one field to the `HealthOutcome` from this issue.**

```go
type HealthOutcome struct {
	State    HealthState
	Err      error
	Duration time.Duration // wall-clock of the check invocation; 0 = unknown
}
```

Stamped where `serviceHealthCheck` already invokes the check — same shape as `ServiceShutdownTime`. Zero stays "unknown" so a check abandoned at `HealthCheckTimeout` isn't misread as instant.

---

<sub>Prepared with AI assistance (GLM-5.3-Flash via [Crush](https://github.com/charmbracelet/crush)); every claim was verified against the samber/do v2.1.0 source and master before commenting.</sub>

---

## Posting checklist (owner)

1. Re-verify scope.go line numbers against the master branch at posting time (`gh api repos/samber/do/contents/scope.go`) — the draft cites v2.1.0 + master as of 2026-09-22.
2. Confirm #318 hasn't evolved (new comments proposing the same).
3. Post as comment on #318 (not a new issue — it extends this proposal's struct).
4. After posting: update TODO_LIST.md row (File samber/do upstream) → DONE, note the comment permalink.
5. If samber/do ships it: go-health's `resolveHealthCheck` injector path maps `HealthOutcome.Duration` → `CheckDetail.Duration` (seam already exists; see docs/detailed-checks-cookbook.md §"The path that cannot report durations").
