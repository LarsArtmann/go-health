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
