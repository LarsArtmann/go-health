# Design: typed service identity (`ServiceName`) — v0.6 candidate

**Date:** 2026-10-02 · **Status:** DESIGN ONLY — re-venued 2026-10-08 to the v0.6 breaking-change window (v0.5.0 shipped 2026-10-05 without it; body retains its original v0.5 framing). Breaking by nature. See TODO_LIST "v0.6 window — staging".

## Problem (data-model lens, P1/P7)

Service identity exists as a bare `string` in three renderings that must
agree: `WithCriticalServices(names ...string)`, the batch map key
(`map[string]error`), and `Response.Checks`. Nothing ties them together —
which is exactly the critical-name footgun class (now guarded at runtime by
`ErrUnknownCriticalService`; this design kills it at compile time).

## Options considered

| Option                               | Shape                                                                   | Verdict                                                                                                                                               |
| ------------------------------------ | ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| **A. Defined string type**           | `type ServiceName string`; `WithCriticalServices(names ...ServiceName)` | **RECOMMENDED** — one-line definition, zero runtime cost, error `%w`-friendly, JSON-marshals as its string. Builders: `checks.Named("database", fn)`. |
| B. Struct handle (identity object)   | `ServiceName{...}` with methods                                         | Rejected — heavyweight; identity is just a name in this domain.                                                                                       |
| C. Sealed/branded type with registry | `NewServiceName[T]()` deriving from the type                            | Rejected for v0.5 — couples go-health to naming conventions (typetostring) some consumers don't use (cqrs-htmx projections are plain strings).        |

## Migration plan (v0.5 window)

1. v0.5.0 introduces `type ServiceName string` and **changes** the
   signatures: `WithCriticalServices(names ...ServiceName)`,
   `NewChecks(map[ServiceName]CheckFunc)`, `NewWithHealthCheck(func(...)
   map[ServiceName]error)`. Wire format unchanged (`ServiceName` marshals as
   a JSON string).
2. Existing string literals migrate mechanically: `"database"` →
   `health.ServiceName("database")`. Every fleet call site is known
   (docs/adoption-matrix.md + the A3 scan): ~14 call sites, most already
   route names through constants (`CheckDiskSpace`, `healthServiceDatabase`)
   or typetostring — the conversion lands in one constant block per repo.
3. No alias shims: Go has no way to keep `...string` and `...ServiceName`
   accepting the same literals without generics gymnastics that would
   worsen the API. A clean major-version cut is the honest move.
4. Validation (`ErrUnknownCriticalService`) is unchanged — it compares the
   same names after conversion; the type only moves the _typo_ class from
   runtime to review time for constant-based consumers.

## Non-goals

- Renaming check names on the wire (frozen since v0.1.3).
- Forcing typetostring conventions into the library.

---

## Fleet inventory (A21, 2026-10-09 — all local checkouts, source-verified)

**Blast-radius reduction (the load-bearing finding):** untyped string
literals and untyped `const` names survive the `...ServiceName` change
untouched (assignability). Only **typed-string values** and **`...string`
spreads** break. The fleet's breakage set is therefore six sites in six
repos (library-policy included, swept 2026-10-09), not "every call site" — the v0.5-window migration estimate above was
pessimistic; the real v0.6 work is one collection-type change per affected
repo.

| Repo                                   | Pin    | Constructor (prod, non-test)                                                                                              | Critical names form                                          | Breaks under `ServiceName`?                                        |
| -------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------ | ------------------------------------------------------------------ |
| webphone                               | v0.5.0 | `health.New` (app.go:262) + `health.NewChecks` (server.go:385)                                                            | literals `"sqlite"`, `"blob-dir"` (documented as contract)   | no (untyped literals)                                              |
| Zlota44                                | v0.4.1 | `health.New` (health.go:102)                                                                                              | `checkSQLite` const (health.go:105)                          | no if untyped const (verify decl)                                  |
| dnsblockd                              | v0.4.1 | `health.NewChecks` (health.go:53)                                                                                         | consts `healthServiceDatabase`, `healthServiceDNS` (:59)     | no if untyped consts (verify decls)                                |
| nsfw-classifier                        | v0.5.0 | `health.New` (app.go:363)                                                                                                 | `typetostring.GetType[...]()` typed string (:360)            | **YES — typed getter**                                             |
| file-and-image-renamer                 | v0.5.0 | `gohealth.New` (providers.go:131)                                                                                         | `critical...` spread of `[]string` (:133)                    | **YES — spread**                                                   |
| KeyHolderAI                            | v0.4.1 | `health.New` (di.go:466)                                                                                                  | multi-line names (di.go:469) — verify literal vs typed       | verify                                                             |
| DiscordSync                            | v0.4.1 | `health.New` (health_dashboard.go:82)                                                                                     | `criticalHealthServiceNames()...` spread of `[]string` (:95) | **YES — spread**                                                   |
| CV                                     | v0.1.3 | `health.New` (health_probe.go:61)                                                                                         | `critical...` spread (:58)                                   | **YES — spread**                                                   |
| go-appkit (bridge)                     | v0.5.0 | `health.NewWithHealthCheck` (health/probe.go:78)                                                                          | pass-through `opts ...health.Option` — consumer-facing       | only if bridge signature changes (it forwards options; unaffected) |
| cqrs-htmx (bridge)                     | v0.5.0 | `gohealth.New(do.New(), all...)` (health/probe.go:46)                                                                     | pass-through options; doc literals only                      | unaffected (forwards options)                                      |
| go-taskqueue                           | v0.5.0 | none — implements dashboard `Prober` over `health.Response` types (internal/webui/health.go)                              | n/a (types-only consumer)                                    | no                                                                 |
| go-health-dashboard                    | v0.5.0 | `health.New` in tests/example; prod wiring via aggregate/federation                                                       | literals in tests ("postgres", "redis")                      | no (untyped literals)                                              |
| projects-management-automation ("PMA") | v0.4.x | **no local checkout** — recorder bridge (consumer-verification pattern C)                                                 | —                                                            | scan on next contact                                               |
| library-policy                         | v0.5.0 | `health.NewWithHealthCheck` (cmd/library-policy/internal/httpapi/health.go:36)                                            | `critical...` spread of `[]string` (:38)                     | **YES — spread**                                                   |
| typespec-eventsourcing                 | —      | `NewChecks` in tests only (adoption-matrix)                                                                               | —                                                            | no (test literals)                                                 |
| go-daemon, project-discovery-daemon    | —      | **NOT consumers**: `WithShutdownGracePeriod` there is go-daemon's own `ServerOption` (socket.go:128), no go-health import | —                                                            | adoption-matrix row over-counts; correct on next matrix pass       |

### Migration mechanics (updated by the inventory)

1. Six-site break set: nsfw-classifier (typed getter), fir/DiscordSync/CV/
   library-policy (`[]string` spreads). Fix per repo: change the collection to
   `[]health.ServiceName` (or generate via `ServiceName(typetostring...)`).
2. Two const-decl verifications (Zlota44 `checkSQLite`, dnsblockd
   `healthService*`): untyped `const` → no change; `var`/typed → wrap.
3. The scanner below finds every breaking shape mechanically; per-repo test
   commands are the verification plan.

### Scanner (staging artifact — detects, never rewrites)

`tools/servicename-scan.sh` flags (a) `WithCriticalServices`/`NewChecks`/
`NewWithHealthCheck` call sites, (b) `...)` spreads, (c) typed-string
getters inside those calls, across a fleet checkout. Dry-run output
2026-10-09 over the 14 local repos: six candidates = the five true breaks
above plus one known false-positive class (go-appkit `health/probe.go:78`
spreads `opts ...health.Option` — option forwarding, not a string
collection; triage rule: a spread breaks only when it spreads a
`[]string`). Rewrite at v0.6 window-open is human-reviewed per the
table; a blind sed would also touch the untyped literals that survive —
deliberately not attempted.

### Verification plan (per-repo, post-rewrite at v0.6 window)

| Repo                | Command                                            | Gate                    |
| ------------------- | -------------------------------------------------- | ----------------------- |
| webphone            | `nix run .#test` (flake) or `go test ./...`        | suite green             |
| Zlota44             | `go test ./...`                                    | suite green             |
| dnsblockd           | `go test ./...` + dashboard overlay test           | suite green             |
| nsfw-classifier     | `go test ./...`                                    | suite green             |
| fir                 | `go test ./...`                                    | suite green             |
| KeyHolderAI         | `go test ./...`                                    | suite green             |
| DiscordSync         | `go test ./...` + private critical-name guard test | suite green incl. guard |
| CV                  | `go test ./...`                                    | suite green             |
| go-appkit           | `cd health && go test ./...`                       | module green            |
| cqrs-htmx           | `cd health && go test ./...`                       | module green            |
| go-health-dashboard | `nix run .#test` incl. browser/screenshot suite    | full green              |
