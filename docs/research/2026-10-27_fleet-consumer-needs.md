# Fleet consumer needs — what the 36 consumers actually require from go-health

**Date:** 2026-10-27 · **Scope:** go-health (SDK) and the three bridges that
multiply its reach · **Method:** local checkouts, `go.mod` pins, call-site
reads, consumer docs, and the open issue tracker. Every claim below names a
file or a command that produced it.

---

## 1. Reach: who actually depends on this library

The dashboard inventory (36 consumers of go-health-dashboard v0.12.1) is not
the whole story. go-health reaches those repos **through three layers**:

| Layer                                             | Carriers                                                                                                                                                                                                                                                        | Count     |
| ------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| Direct `require github.com/larsartmann/go-health` | company-master-data, cqrs-htmx/health, cv, CV, DiscordSync, dnsblockd, file-and-image-renamer, go-appkit/health, go-health-dashboard, go-taskqueue, KeyHolderAI, library-policy, nsfw-classifier, projects-management-automation, timesheets, webphone, Zlota44 | 15 repos  |
| Bridge A: `cqrs-htmx/health` (v0.7.0)             | every cqrs-htmx app that mounts health (the bulk of the 36)                                                                                                                                                                                                     | ~20 repos |
| Bridge B: `go-appkit/health` (v0.5.1)             | company-master-data + go-appkit's integration suite + appkit-based services                                                                                                                                                                                     | 4+ repos  |
| Presentation: `go-health-dashboard` (v0.7.0)      | the 36-repo inventory                                                                                                                                                                                                                                           | 36 repos  |

Commands used:

```bash
cd ~/projects && rg -l 'github.com/larsartmann/go-health' --glob '**/go.mod'
for d in CV cv DiscordSync dnsblockd ...; do rg -m1 'go-health v' "$d/go.mod"; done
```

**Ceiling on blast radius:** every option, default, and grade rule is
inherited by ~39 repos, most of which never read this repo's source. Two of
the three bridges are themselves libraries, so a change here ships to them
only when they bump — and they are currently behind (`go-appkit/health`
v0.5.1, `cqrs-htmx/health` v0.7.0). This is the single most important
supplier fact in this document: **the API is wider than the release notes
assume, and the release cadence is the real bottleneck.**

### Version skew (verified today)

| Pin                     | Repos                                                                                                                                                                                               |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| v0.7.0 (latest release) | dashboard, cqrs-htmx/health, DiscordSync, file-and-image-renamer, go-taskqueue, library-policy, nsfw-classifier, projects-management-automation, timesheets, webphone, Zlota44, company-master-data |
| v0.5.1                  | CV, cv, dnsblockd, KeyHolderAI, go-appkit/health, go-appkit/integration                                                                                                                             |

Master additionally carries unreleased work that **no consumer can get**: the
federation drain (issue #5), the per-check-panic grading fix (landed
2026-10-27, `classifier.go` `isRecoveredPanic`), and this session's APIs.
The v0.8.0 cut is already listed as an owner action in `TODO_LIST.md`.

### Skew against the v0.8.0 tag (`tools/fleet-skew.sh v0.8.0`, 2026-10-27)

The manifest now lists all 15 direct consumers plus both bridges
(`tools/fleet-repos.txt`; company-master-data is forgejo-hosted and therefore
outside the GitHub-API sweep). Result at the moment v0.8.0 was cut:

| Verdict                           | Repos                                                                                                                                                                                    |
| --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| FAIL — minor behind (v0.7.0)      | go-health-dashboard, Zlota44, library-policy, file-and-image-renamer, DiscordSync, nsfw-classifier, webphone, projects-management-automation, timesheets, cqrs-htmx/health, go-taskqueue |
| FAIL — minor behind (v0.5.1)      | CV, cv, dnsblockd, KeyHolderAI                                                                                                                                                           |
| WARN — no pin on GitHub `@master` | go-appkit/health (its local checkout still pins v0.5.1; the remote branch lags)                                                                                                          |

`minor-stale: 15, warnings: 1` — the expected end state: the tag exists, no
consumer has moved yet. This table is the before-picture for the adoption
milestones; after the consumer bumps it should read `minor-stale: 0`.

---

## 2. Implementation patterns in the fleet

The four long-standing patterns recorded in `docs/consumer-verification.md`
still hold, with one important addition (pattern E, which the consumer record
now carries too):

| Pattern | Shape                                                  | Consumers                                                                       |
| ------- | ------------------------------------------------------ | ------------------------------------------------------------------------------- |
| A       | injector + `WithCriticalServices`                      | KeyHolderAI, CV, DiscordSync, nsfw-classifier, webphone, Zlota44                |
| B       | `NewWithHealthCheck` / `NewChecks` check-func map      | dnsblockd, library-policy, file-and-image-renamer, timesheets, go-appkit/health |
| C       | recorder bridge over an often-empty injector           | cqrs-htmx/health, projects-management-automation, Zlota44                       |
| D       | framework bridge (`appkithealth.NewProbe` + doadapter) | company-master-data and appkit services                                         |
| E       | **hand-rolled cache/Prober adapter**                   | dnsblockd, DiscordSync, cv, go-taskqueue, file-and-image-renamer healthd        |

Pattern E is the interesting one: five independent repos re-implement the
same missing primitive around `CachedResponse`.

---

## 3. What they need — ranked, with evidence

### N1 (P0, 4 repos): a cache-freshness primitive — "serve cache if fresh, else evaluate"

Every one of these forks exists because `CachedResponse()` cannot say how old
its snapshot is in one call, and `Evaluate()` cannot publish:

| Repo         | Fork                                        | What it does                                                                      |
| ------------ | ------------------------------------------- | --------------------------------------------------------------------------------- |
| DiscordSync  | `cmd/discordsync/health_readyz_cache.go:60` | `Timestamp.IsZero()` + `now - Timestamp > maxAge` → live sweep                    |
| dnsblockd    | `internal/server/health.go:96`              | zero-timestamp → one bounded `Evaluate`; older than 15s → `degradeStaleHealth`    |
| cv           | `internal/health/handlers.go:124`           | empty cache → `Evaluate`, otherwise project the cache                             |
| go-taskqueue | `internal/webui/health.go:86`               | reimplements the whole `dashboard.Prober`; evaluates when older than its interval |

They differ only in the policy number; the primitive is identical. Three of
them also need N2 (below) and two hit N3.

**Shipped this session:** `Probe.CacheAge() (time.Duration, bool)`,
`Probe.CachedResponseFresh(maxAge) (Response, bool)`, and
`Probe.EvaluateAndCache(ctx) Response`. Together they reduce the fork to:

```go
resp, ok := probe.CachedResponseFresh(2 * probe.RefreshInterval())
if !ok {
    resp = probe.EvaluateAndCache(ctx)
}
```

### N2 (P0, 5 repos): "no evaluation yet" must not read as a green pass

`CachedResponse()` synthesizes `StatusPass` with an empty `Checks` map when
no snapshot exists (`probe.go`, `latest == nil`). The readiness handler is
fail-closed (it evaluates live), but every consumer that reads
`CachedResponse` directly has to know this. cv guards with `len(Checks)==0`,
DiscordSync/dnsblockd with `Timestamp.IsZero()`, go-taskqueue by never using
`CachedResponse` at all, and file-and-image-renamer's custom `healthd` renders
the synthesized pass untouched (`healthd/dashboard_data.go:19`,
`healthd/server_handlers.go:89`).

**Shipped:** the boolean half of `CacheAge` is the explicit signal
(`false` = never evaluated), documented on `CachedResponse` itself.

### N3 (P0, 2 repos + a design note): the cached snapshot aliases probe state

`CachedResponse()` is a shallow struct copy; `Checks` aliases the published
map. Editing it in place is a concurrent-map-write crash while handlers
marshal. dnsblockd (`health.go:121`) and cv (`handlers.go:131`) both clone
manually; `docs/synthetic-check-overlay.md` calls the clone "the load-bearing
line".

**Shipped:** `Response.Clone()` — a documented, tested deep copy, and the
`CachedResponse` godoc now states the ownership rule. See
`docs/response-ownership-design.md`.

### N4 (P0, 3 repos + issue #7 F3): `Evaluate` deliberately does not publish, but that is invisible

DiscordSync, dnsblockd, and cv all fall back to `Evaluate` — which answers
correctly once and leaves the _older_ snapshot in the cache, so the next read
regresses to stale data. `webphone/TODO_LIST.md` records the same seam from
the other side: a `WithEvaluationHook` wired on a path that never calls
`Evaluate` fires for nothing (their "M18 park watch").

**Shipped:** `EvaluateAndCache` (publishes, timeout-bounded) plus an explicit
"Evaluate does not publish" paragraph on `Evaluate`.

### N5 (P0, issue #7 F2): `WithHealthRecorder` on function-path constructors is silently dead

`go-appkit/health/probe.go:25` has a whole section warning that the option
"is silently ignored: no callbacks fire… with no error and no warning". The
flagship flight-recorder wiring compiles and captures nothing.

**Shipped:** `ErrRecorderNotApplicable`, returned from `Validate()` and
therefore `Start()`, for `NewWithHealthCheck`, `NewWithDetailedCheck`, and
`NewChecks`. This is non-breaking at compile time (same signatures) and loud
at boot — the same fail-fast point as `ErrUnknownCriticalService`. Documented
on the option and on all three constructors.

### N6 (P1, cqrs-htmx + issue #7 F2 sibling): recorder composition loses durations

`cqrs-htmx/health/probe.go:97`: "detailed recorders do not survive the merge"
in its own `RecorderChain`. That is a bridge-level re-implementation of a
library-shaped problem: observe one batch through several sinks.

**Shipped:** `health.ChainRecorders(...)` — order-preserving, nil-skipping,
and always satisfying `DetailedHealthRecorder` so durations from detailed
members survive while plain members contribute zero. The cqrs-htmx chain can
be deleted on its next bump.

### N7 (P1, issue #7 F4): Shutdown is a one-way valve, Start reads re-armable

go-appkit documents the runtime fact (`health/doc.go:96`): after
`Shutdown()` + `Start()`, readiness is 503 forever, because `shuttingDown` is
never cleared. The `Start`/`Shutdown` godoc do not both say so.

**Status:** docs-only fix planned. Clearing the latch would change semantics
for every consumer and needs its own decision; the rejection-style fix
(shutting down is a lifecycle end) is cheaper and safer.

### N8 (P1, go-health#6): federation cannot answer "is remote `api` healthy?"

`aggregate.Aggregate.SourceStatuses()` shipped in v0.6.0; federation did not
get it. The concrete cost, measured on the dashboard (v0.7.0, commit
`1473fc2`): a draining remote contributes all-pass rows and forces the merged
roll-up to fail, so its card folds green under a red page.

**Shipped:** `(*federation.Prober).SourceStatuses()` — one parallel fetch
pass, `fail` for unreachable and draining remotes, no startup-latch
movement, hub drain does not rewrite remote verdicts. The signature matches
aggregate's, so the dashboard's `SourceStatuser` capability now covers both
merge surfaces. Design: `docs/federation-source-statuses-design.md`.

### N9 (P1, all consumers): release cadence is the bottleneck

Four shipped fixes and one feature sit unreleased on master while four apps
and the go-appkit bridge are two minors behind. The earlier fleet
verification (2026-10-09) already found consumer bumps "local-unpushed
pending the owner's push-authority answer", and the version-skew CI script is
blocked on the same question (`TODO_LIST.md`).

### N10 (P1, bridge shape): both bridges narrow to `*health.Probe`

`cqrs-htmx/health/dashboard.go` takes `*gohealth.Probe`, so aggregate or
federated surfaces cannot render through that bridge; `appkithealth.NewProbe`
likewise returns a concrete probe. The dashboard itself already accepts an
interface (`dashboard.Prober`), so this is a bridge typing choice, not a
library limit. Accepted need, no go-health change required.

### N11 (P2): the `checks` battery is under-discovered

Only file-and-image-renamer wires `checks.*`; company-master-data's audit
scores `checks.Disk` as "Partial — fits the file store; unused" (row 34 of
`docs/research/2026-10-25_httputil-fleet-stack-deep-dive.html`). The failure
mode is real (a full data directory silently truncating loads). Cookbook
entry exists (`docs/system-checks-cookbook.md`); the need is a louder
"which check for which failure mode" pointer in README, not new API.

### N12 (P2): `AwaitReady` passes instantly on the zero-value pass

`Ready()` is "not currently failing", which is true on the synthesized pass
before any evaluation. dnsblockd gates startup on `AwaitReady` (bounded 5s)
after its probe starts, so it is safe today; go-appkit's examples rely on it
after `Start`. No consumer has been bitten, but the trap is real and cheap to
document: point `AwaitReady` callers at `StartupComplete`/`CacheAge`.

### N13 (done, unreleased): per-check panic graded warn for non-critical checks

issue #7 finding 1. `classifier.isRecoveredPanic` now matches
`ErrCheckPanicked` and grades fail; `TestNewChecks_NonCriticalPanicFailsClosed`
pins it. **Coordination:** go-appkit/health v0.5.1 pins the old warn behavior
(`probe_test.go:111`), so it must not bump above v0.7.0 without flipping that
pin in the same change.

### N14 (dashboard repo, same supplier family)

Consumer asks found in the sweep, out of go-health's scope but blocking them:
file-and-image-renamer needs `HTMLDataAttrs` on the dashboard's `<html>` for a
CSP-clean nonce (their TODO DD9, draft ready); dashboard#14 (`Shutdown` does
not cancel the `Start` ctx); dashboard#17 (nightly fuzz red). These are
recorded here so they are not lost when the go-health wave lands.

---

## 4. Supplier posture: what "better supplier" means, concretely

1. **Ship the release.** v0.8.0 (drain + panic grading + this session's
   additive APIs) is the unblocker for every item above. Nothing else in this
   report reaches a consumer without it.
2. **Bump the bridges in the same wave.** `cqrs-htmx/health` and
   `go-appkit/health` are the two multipliers; every release must include a
   bridge bump + green bridge suite, or ~24 repos inherit nothing.
3. **Publish a consumer-impact line per release.** Additive vs
   behavior-change vs breaking, naming the bridges. The CHANGELOG already
   has the detail; the release notes need the one-line verdict.
4. **Keep the consumer inventory alive.** `docs/consumer-verification.md`
   still says 15 direct + 16 indirect; the dashboard inventory is 36, and the
   direct inventory now shows a fifth pattern (hand-rolled cache adapters).
   Re-derive it when a release is cut (the commands are in §1).
5. **Prefer primitives to policy.** The fleet forks differ in their staleness
   bounds and their synthetic-row rules; `CacheAge` + `CachedResponseFresh` +
   `EvaluateAndCache` + `Clone` give them the mechanics without guessing.
   This is exactly the line `docs/synthetic-check-overlay.md` drew, now
   honoring the repeated demand.
6. **Fail at Start, not silently.** `ErrRecorderNotApplicable` follows the
   `ErrUnknownCriticalService` precedent: boot-time failure is the only
   reliable teacher across 39 repos.

---

## 5. Session log — what changed in this repo

| Change                                           | Kind                   | Tests                                                                                      |
| ------------------------------------------------ | ---------------------- | ------------------------------------------------------------------------------------------ |
| `Response.Clone()`                               | additive               | `TestResponse_CloneIsIndependent`                                                          |
| `Probe.CacheAge()`                               | additive               | `TestCacheAge_*` (4 cases)                                                                 |
| `Probe.CachedResponseFresh(maxAge)`              | additive               | `TestCachedResponseFresh_*`                                                                |
| `Probe.EvaluateAndCache(ctx)`                    | additive               | `TestEvaluateAndCache_PublishesButEvaluateDoesNot`                                         |
| `ErrRecorderNotApplicable` at `Validate`/`Start` | behavior change (loud) | `TestValidate_RejectsRecorderOnFunctionPath`, `TestValidate_AcceptsRecorderOnInjectorPath` |
| `ChainRecorders(...)`                            | additive               | `TestChainRecorders_*` (3 cases)                                                           |
| `federation.Prober.SourceStatuses()`             | additive               | `TestSourceStatuses_*` (4 cases)                                                           |

Verification (green at the time of writing):

```bash
cd ~/projects/go-health && GOWORK=off go test ./...
# ok github.com/larsartmann/go-health
# ok github.com/larsartmann/go-health/aggregate
# ok github.com/larsartmann/go-health/checks
# ok github.com/larsartmann/go-health/federation
# ok github.com/larsartmann/go-health/internal/merge
```

## 5b. Late-wave addendum — features that landed after §5 was written

§5 was captured mid-session. Six more capabilities landed before the wave
closed; they are additive except where noted, and each closes a named need
from §3.

| Change                                                                      | File                                                                | Closes                                                                                                                       | Kind                | Tests                                               |
| --------------------------------------------------------------------------- | ------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- | ------------------- | --------------------------------------------------- |
| `NewWithRecorder(recorder, opts...)` — injector-free recorder constructor   | `new_with_recorder.go`                                              | N5 (a recorder-only consumer no longer manufactures an empty `do.Injector`)                                                  | additive            | `new_with_recorder_test.go` (5 cases)               |
| `HealthRecorderFunc` / `DetailedHealthRecorderFunc` adapters                | `recorder.go`                                                       | N6 (a consumer can pass a method/closure where an interface is required)                                                     | additive            | `TestDetailedHealthRecorderFunc_PreservesDurations` |
| `DoServiceName[T]()` / `WithCriticalService[T]()` — typed critical identity | `service_name.go`                                                   | N9/N10 (stringly-typed critical sets drift from the service registration)                                                    | additive            | `service_name_test.go` (2 cases)                    |
| `RegisterRoutes` skips empty patterns on all three surfaces                 | `handlers.go`, `aggregate/aggregate.go`, `federation/federation.go` | fleet-wide: `routes.Liveness == ""` used to mount `""` on the default mux                                                    | additive            | `routes_selective_test.go` ×3                       |
| `federation.Prober.Healthz()`                                               | `federation/healthz.go`                                             | N8 sibling + ROADMAP Theme 7 (an LB in front of a hub had no single endpoint)                                                | additive            | `healthz_test.go` (6 cases)                         |
| Recovered panic fails closed in **both** `classify` and `grades`            | `classifier.go`                                                     | N13 (the grader named one site; the roll-up path was independently wrong, so a non-critical panic still rolled up as `warn`) | **behavior change** | `TestNewChecks_NonCriticalPanicFailsClosed`         |

The last row is the wave's only behavior change. It ships with an ADR
(`docs/adr/`), a CHANGELOG migration snippet, and a required pin flip in
`go-appkit/health`, whose `health/probe_test.go` still asserts the old
`warn` grading.

## 6. Remaining queue (owner-ordered)

| #  | Item                                                                                       | Impact | Effort | Gate            |
| -- | ------------------------------------------------------------------------------------------ | ------ | ------ | --------------- |
| 1  | Cut v0.8.0 (drain + panic fix + these APIs)                                                | High   | 30 min | owner push      |
| 2  | Bump both bridges; flip go-appkit's warn pin; delete cqrs-htmx `RecorderChain`             | High   | 2 h    | #1              |
| 3  | Adopt `CachedResponseFresh`/`EvaluateAndCache` in DiscordSync, dnsblockd, cv, go-taskqueue | High   | 3 h    | #1              |
| 4  | Widen `cqrs-htmx/health.NewDashboard` to the dashboard `Prober` interface                  | Med    | 30 min | #1              |
| 5  | Document the one-way shutdown latch on `Start`/`Shutdown`; `AwaitReady` warning            | Med    | 30 min | none            |
| 6  | Refresh `docs/consumer-verification.md` to the 36-consumer inventory                       | Med    | 1 h    | none            |
| 7  | `checks` discoverability row in README (failure mode → battery)                            | Med    | 30 min | none            |
| 8  | Bridge-level detailed constructor for `appkithealth` (`NewDetailedProbe`)                  | Med    | 1 h    | go-appkit       |
| 9  | Version-skew CI script                                                                     | Med    | 45 min | push authority  |
| 10 | Typed `ServiceName`, `SanitizeResponse`/`Since` renames                                    | Low    | 2 h    | breaking window |
| 11 | Adopt `NewWithRecorder` in PMA + Zlota44 (drops the empty-injector dance)                  | Med    | 30 min | #1              |
| 12 | Adopt `WithCriticalService[T]` in CV, KeyHolderAI, nsfw-classifier, DiscordSync labels     | Med    | 1 h    | #1              |
| 13 | Offer `federation.Healthz()` to the dashboard hub's LB-facing route                        | Med    | 30 min | #1              |
