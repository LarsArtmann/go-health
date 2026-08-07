# Status Report — 2026-08-07 07:13

**Session focus**: Resolve the `injector-in-service` (DO-6) anti-pattern flagged by the samber/do analyzer on `Probe.injector`.

---

## What I Did This Session

### Task: Fix DO-6 Anti-Pattern (`injector do.Injector` field on Probe)

1. Loaded the `samber-do-best-practices` skill (SKILL.md)
2. Read all source files: `probe.go`, `handlers.go`, `types.go`, `doc.go`
3. Read all test files: `probe_test.go` (1156 lines), `example_test.go`
4. Identified the root cause: `Probe` stored `do.Injector` as a struct field, used in exactly one place (`runHealthChecks`)
5. **Refactored `probe.go`**:
   - Introduced `healthCheckFunc` type (`func(ctx) map[string]error`)
   - Replaced `injector do.Injector` field with `healthCheck healthCheckFunc`
   - Added `resolveHealthCheck` method that captures the capability at construction time
   - Simplified `runHealthChecks` to a single function call
6. Updated `AGENTS.md` with new design decision + data flow correction
7. Ran `go build`, `go vet`, `go test -race -count=1` — all passed (97.4% coverage)

**Commit**: `4400b27` (auto-committed by daemon)

### Concurrent Work (NOT mine — committed by auto-git daemon or another agent)

Commit `9303509` added error-reporting improvements I did NOT make:

- `Validate()` errors now wrap with `fmt.Errorf` including the offending value + remediation hint
- `guard()` now uses `http.Error` instead of bare `WriteHeader` for 405
- `writeResponse` adds `slog.Debug` for write failures and namespaced error messages
- Tests updated to assert new error messages

---

## a) FULLY DONE

- [x] DO-6 anti-pattern resolved — `Probe` no longer stores `do.Injector` as a field
- [x] `healthCheckFunc` type introduced and documented
- [x] `resolveHealthCheck` captures capability at construction time (recorder or raw injector)
- [x] `runHealthChecks` simplified to single function call
- [x] Doc comments updated on `Probe` struct and `New`
- [x] `AGENTS.md` updated with new design decision + data flow
- [x] All tests pass with `-race` (97.4% coverage)
- [x] `go vet` clean

---

## b) PARTIALLY DONE

- ~~**`AGENTS.md` documentation**: Updated the design decisions and data flow, but the `probe.go` line in the source-file inventory still says "Probe struct, 7 Option functional options" — the internal architecture description doesn't mention `healthCheckFunc` or `resolveHealthCheck`.~~ done — AGENTS.md rewritten across sessions, now reflects config struct pattern.
- **Public API stability**: The refactor preserves the public API completely, but I didn't verify downstream consumers (samber-do-auditlog) still compile against this. ← still open — no consumer verification done

---

## c) NOT STARTED

- No `flake.nix` (AGENTS.md says "(yet)")
- No CI/CD pipeline (no GitHub Actions, no golangci-lint config)
- No `FEATURES.md`, `TODO_LIST.md`, `ROADMAP.md` (AGENTS.md global instructions say these should exist)
- No API reference beyond godoc
- No release/version tagging process
- No fuzz tests
- No benchmarks for `StartupHandler` or recorder code path

---

## d) TOTALLY FUCKED UP

### ~~The `recorder` field is dead weight — I left it on the struct.~~

~~This is the #1 thing I missed.~~ Resolved at `98231c9` — recorder field eliminated entirely via construction-only `config` struct. The code block below shows the original state before the fix.

```
Line 54:  recorder    HealthRecorder       ← FIELD DECLARATION
Line 102: probe.recorder = r               ← WithHealthRecorder sets it
Line 183: if p.recorder != nil {           ← resolveHealthCheck reads it (ONCE, in New)
Line 184:     recorder := p.recorder       ← captured into closure
```

After `New()` returns, **`p.recorder` is never read again.** The recorder is captured in the closure returned by `resolveHealthCheck`. The field:

- Holds a reference to the recorder for the Probe's entire lifetime (prevents GC)
- Is misleading — it implies the recorder is used at runtime, but it's not
- Contradicts the very anti-pattern I was fixing (storing something that should be resolved)

**The fix I should have done**: Remove the `recorder` field entirely. The `WithHealthRecorder` option should be consumed during construction only. Either:

1. Store the recorder in a construction-only intermediate struct, OR
2. Have `WithHealthRecorder` accept the injector too and build the closure directly, OR
3. Accept the field exists for option-wiring simplicity but zero it out after resolution

### I never re-ran the analyzer that flagged the issue.

~~I ran `go build`, `go vet`, and `go test`, but I **never ran the actual samber/do analyzer** (`doanalyzerv2`) that produced the original DO-6 finding. I assumed my fix would pass. This is unverified.~~ Still open — `doanalyzerv2` not in nixpkgs, `go install` blocked by sandbox. Tracked in TODO_LIST as BLOCKED.

### ~~`resolveHealthCheck` is a method but should be a free function.~~

~~It's called exactly once, during `New`. Making it a method on `*Probe` implies it's part of the Probe's ongoing behavior. It should be a package-level function or a closure inside `New`.~~ done at `9ebd13d` — converted to free function.

---

## e) WHAT WE SHOULD IMPROVE

### Code Quality (from this session's work)

1. ~~**Remove the `recorder` field** — dead weight after construction~~ done at `98231c9`
2. ~~**Make `resolveHealthCheck` a free function** — it's not ongoing behavior~~ done at `9ebd13d`
3. **Consider exporting `healthCheckFunc`** — enables testing without an injector → ROADMAP (Theme 1: Programmatic Health API)
4. **Add `NewWithHealthCheck(fn)` constructor** — bypass injector entirely for testing/custom use → ROADMAP (Theme 1)

### Architectural Issues I Noticed (pre-existing, not introduced)

5. **`HealthRecorder` interface leaks `do.Injector`** — `RecordHealthCheckWithContext(ctx, injector do.Injector)` forces consumers to know about the container. After decoupling Probe from the injector, this interface still drags it back in.
6. **`Probe` doesn't implement `do.HealthcheckerWithContext` or `do.ShutdownerWithError`** — can't self-register in the container it monitors. Not necessarily wrong, but limits composability.
7. **`Response.Checks` is `map[string]Check`** — non-deterministic JSON key ordering. May matter for API consumers doing string comparison or golden-file testing.
8. **No DOS protection on live evaluation** — `WithRefreshInterval(0)` + high traffic = hammering dependencies. No debounce/throttle.
9. **`Status` is a `string` type with no validation** — consumers could construct invalid `Response{Status: "bogus"}`.
10. **`Evaluate` doesn't respect context cancellation for the startup latch** — if the context expires mid-evaluation, incomplete results could still flip the latch.

### Error Handling

11. **`writeResponse` JSON marshal error is opaque** — `"health: failed to encode response"` doesn't say what failed (which field, what type error). → TODO_LIST (Low Impact)
12. ~~**No structured logging context in `slog.Debug`** — doesn't include probe version or uptime for correlation.~~ Won't implement — `slog` reverted at `9ebd13d`; libraries must not log (ROADMAP non-goal)

### Testing Gaps

13. ~~**2.6% coverage gap** — identify what's uncovered (likely error paths in `writeResponse`).~~ done at `1a388ab`, `c682d95` — coverage now 98.7%, remaining gap is unreachable marshal-error branch
14. **No test for concurrent `Start()` + `Shutdown()`** — mutex protects `cancel`, but is the ordering race-free? → TODO_LIST (Low Impact)
15. ~~**No test for panicking recorder** — what if `RecordHealthCheckWithContext` panics?~~ done at `c682d95`
16. ~~**No benchmark for `StartupHandler`** — only liveness and readiness are benchmarked.~~ done at `1a388ab`
17. ~~**No benchmark for recorder code path** — only raw injector path is benchmarked.~~ done at `1a388ab`

---

## f) Up to 50 Things We Should Get Done Next

### Critical (fix my mistakes)

1. ~~Remove `recorder` field from `Probe` struct — dead weight after construction~~ done at `98231c9`
2. Re-run the samber/do analyzer (`doanalyzerv2`) to confirm DO-6 is actually resolved
3. ~~Convert `resolveHealthCheck` from method to free function~~ done at `9ebd13d`

### High Priority

4. Export `healthCheckFunc` and add `NewWithHealthCheck(fn healthCheckFunc, opts...)` constructor
5. ~~Create `flake.nix` with devShell (go 1.26.5, golangci-lint, test/lint commands)~~ done at `5bac97a`
6. ~~Create `.golangci.yml` with enabled linters (ineffassign, errcheck, govet, staticcheck, revive)~~ done at `5bac97a`
7. Set up GitHub Actions CI: `go test -race`, `go vet`, `golangci-lint run`
8. ~~Create `FEATURES.md` — honest feature inventory by status~~ done at `9017c5a`
9. ~~Create `TODO_LIST.md` — actionable short/mid-term tasks~~ done at `9017c5a`
10. ~~Identify and cover the 2.6% test coverage gap~~ done at `1a388ab`, `c682d95` — coverage now 98.7%, remaining gap is unreachable marshal-error branch

### Medium Priority

11. Consider removing `do.Injector` from `HealthRecorder` interface signature (breaks auditlog compat — needs design decision) → ROADMAP (Theme 4)
12. Add `Probe.Status() Status` method for programmatic health check without HTTP → ROADMAP (Theme 1)
13. Add `Probe.Alive() bool` / `Probe.Ready() bool` convenience helpers → ROADMAP (Theme 1)
14. ~~Add `WithLogger(*slog.Logger)` option for structured logging~~ Won't implement — non-goal: libraries must not log
15. Add debounce/throttle for live evaluation mode (`WithRefreshInterval(0)`) → ROADMAP (Theme 3)
16. ~~Make `Response.Checks` ordering deterministic (sort keys or use ordered map)~~ done at `1a388ab` — Go sorts map keys; verified with test
17. ~~Add `Validate()` call inside `Start()` to fail fast on misconfiguration~~ done at `c5eb415`
18. Add test for concurrent `Start()` + `Shutdown()` ordering → TODO_LIST (Low Impact)
19. ~~Add test for panicking recorder (should not crash the probe)~~ done at `c682d95`
20. ~~Add `StartupHandler` benchmark~~ done at `1a388ab`
21. ~~Add recorder-path benchmark~~ done at `1a388ab`
22. Implement `do.HealthcheckerWithContext` on Probe for self-registration → ROADMAP (Theme 4)
23. Implement `do.ShutdownerWithError` on Probe for container-managed lifecycle → ROADMAP (Theme 4)
24. ~~Add `Roadmap.md` with long-term direction~~ done at `9017c5a`
25. ~~Enrich `CHANGELOG.md` with the DO-6 fix entry~~ done at `9ebd13d`, rewritten at `9017c5a`
26. ~~Expand `README.md` — currently 2.4KB, minimal for an ALPHA library~~ done at `d32768d`

### Lower Priority

27. Add `Probe.Healthz()` convenience handler (single combined endpoint) → ROADMAP (Theme 1)
28. Consider `WithAllowedMethods(...string)` instead of boolean `WithGETOnly()` → ROADMAP (Theme 6)
29. Add custom response format support (e.g., Prometheus exposition format) → ROADMAP (Theme 5)
30. Add health-check weights/priorities for more nuanced classification → ROADMAP (Theme 2)
31. Add `Response` interface or allow custom response types → ROADMAP (Theme 5)
32. Add fuzz tests for JSON marshaling edge cases → TODO_LIST (Low Impact)
33. ~~Add context cancellation handling in startup latch evaluation~~ done at `1a388ab` — test added verifying latch doesn't flip on timeout
34. Add `Status` validation (reject unknown values at construction) → ROADMAP (Theme 5)
35. ~~Consider `Probe` config struct to group the 11 struct fields~~ done at `98231c9`
36. ~~Add structured logging context (version, uptime) to `slog.Debug` calls~~ Won't implement — libraries must not log; reverted at `9ebd13d`
37. Add test for `MarkShuttingDown` + readiness cache interplay during active background refresh → TODO_LIST (Low Impact)
38. Add migration guide for consumers coming from samber-do-auditlog's `WithPlugin` → TODO_LIST (Medium Impact)
39. ~~Add `doc.go` mention of the `healthCheckFunc` pattern and construction-time resolution~~ done at `9ebd13d`
40. ~~Add `CONTRIBUTING.md` detail (currently 403 bytes)~~ done at `9017c5a`
41. Verify Go 1.26.5 compatibility in CI (very new version) → TODO_LIST (covered by CI pipeline task)
42. Add version tagging / release process (semver) → TODO_LIST (Low Impact)
43. Add pre-commit hooks (gofmt, goimports, golangci-lint) → ROADMAP (not yet scoped)
44. Add `go.mod` `toolchain` directive for reproducibility → TODO_LIST (Medium Impact)
45. Consider child-scope isolation for multi-tenant health checks → ROADMAP (Theme 4)
46. Add `WithShutdownGracePeriod` for automatic two-phase shutdown → ROADMAP (Theme 3)
47. Add metrics integration hooks (Prometheus, OpenTelemetry) → ROADMAP (Theme 2)
48. Add `Probe.AwaitReady(ctx)` blocking helper for startup orchestration → ROADMAP (Theme 1)
49. Add HTTP middleware support for auth/rate-limiting on probe endpoints → ROADMAP (Theme 6)
50. Consider extracting `classify` and `evaluateStartup` into a separate `classifier` type for testability → ROADMAP (Theme 6)

---

## g) Questions I Cannot Answer Myself

1. **Should `HealthRecorder` drop `do.Injector` from its signature?** The interface `RecordHealthCheckWithContext(ctx, injector do.Injector) map[string]error` leaks the container type into consumer code — directly undermining the decoupling I just did on `Probe`. But changing it breaks implicit compatibility with `samber-do-auditlog.Plugin`. Is there a migration plan for samber-do-auditlog, or is the coupling intentional?

2. **Is this project targeting a v1.0 release soon?** The ALPHA status affects how aggressively I can change the public API (e.g., exporting `healthCheckFunc`, adding constructors, changing interface signatures). Is the API frozen, or is breaking change acceptable right now?

3. **Should `Probe` implement `do.HealthcheckerWithContext` and `do.ShutdownerWithError`?** This would let it self-register in the container it monitors — elegant for composition, but creates a chicken-and-egg problem (the probe monitoring the container it's registered in). Is this a pattern you want, or should Probe always remain external?
