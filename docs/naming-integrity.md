# Naming integrity: non-breaking docs now, staged renames for v0.5

**Date:** 2026-10-02 · **Status:** ADOPTED (docs portion shipped; renames re-venued 2026-10-08 to the v0.6 window — v0.5.0 shipped without them; body retains its original v0.5 framing). Automated smell detection: clean (0 findings, naming-smells.sh).

## Non-breaking: document the two confusable names (shipped)

`CheckFunc` vs `HealthCheckFunc` — distinct roles, easy to confuse:

- **`health.CheckFunc`** (checks.go) — one _named check_ passed to
  `NewChecks`: `func(ctx) error`. You write these.
- **`health.HealthCheckFunc`** (accessors.go) — a whole _batch executor_
  passed to `NewWithHealthCheck`: `func(ctx) map[string]error`. You write
  this when you own the batch (composed checks, foreign DI containers).
- `DetailedHealthCheckFunc` is the batch executor's metadata-rich variant;
  `DetailedHealthRecorder` the recorder-side equivalent. Rule of thumb:
  *Func = you call the probe with it; *Recorder = the probe calls you.

Unit-suffix convention (ADR-006 restated): wire fields carry unit suffixes
(`total_latency_ms`, `duration_ns`); Go fields carry the unit when it is
not the obvious one (`DurationNanos`, `TotalLatencyMs`); `time.Duration`
only in-process (`CheckDetail.Duration`).

## Breaking: staged v0.5 rename list

| Now                                | v0.5                                        | Why                                                                                                                                |
| ---------------------------------- | ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `SanitizeResponse`                 | `CoerceValidUTF8`                           | The current name is a euphemism: the function does one thing (replaces invalid UTF-8) and does not sanitize in any security sense. |
| `Check.Since` (JSON `since`)       | Go field `StatusSince` (JSON stays `since`) | "Since when has the check been in its current status" — the JSON key is honest, the Go name is vague. Wire untouched.              |
| `WithGETOnly`                      | **remove** (deprecated since v0.1.1)        | One verified consumer (KeyHolderAI legacy path, adoption matrix); nudge first, then drop.                                          |
| `WithCriticalServices(...string)`  | `(...ServiceName)`                          | docs/servicename-design.md.                                                                                                        |
| `Probe`/`Prober`/`Source`/`Remote` | one "health source" lexicon                 | docs/vocabulary-reconciliation.md.                                                                                                 |

Process per rename: deprecation-policy.md checklist (doc note + SA1019
deprecation marker one minor release before removal where possible), fleet
call sites pre-enumerated from the adoption matrix, CHANGELOG entry with
mechanical migration snippet.

---

## v0.6 staged-rename decision table (A22, 2026-10-09 — consumer impact source-verified)

Principle that splits the table: **alias where Go allows it, break only
where Go forces it.** A pure name change can ship as new-name-canonical +
old-name-deprecated-alias (SA1019, one-minor grace per deprecation-policy);
a signature change cannot be aliased (`...string` and `...ServiceName`
cannot share a literal call site), so it takes the clean cut.

| Rename | Breaks (verified) | Loudness | Alias vs clean cut | v0.6 mechanics |
| ------ | ----------------- | -------- | ------------------ | ------------- |
| `WithCriticalServices` → `WithCriticalChecks` | 8 constructor repos (webphone, Zlota44, dnsblockd, nsfw-classifier, fir, KeyHolderAI, DiscordSync, CV) + 2 bridges forwarding options | compile error, mechanical | **alias + deprecate**: `WithCriticalChecks` canonical, `WithCriticalServices` thin deprecated wrapper (same body), remove no earlier than v0.7 | compose with the ServiceName cut: alias keeps `...string`? — no: alias takes the NEW `...ServiceName` too; untyped literals still compile at both names, so the alias quiets only name drift, not type drift |
| `WithCriticalServices(...string)` → `(...ServiceName)` | 4 sites (nsfw-classifier typed getter; fir/DiscordSync/CV spreads) + 2 const-decl verifications | compile error at typed/spread sites only; literals survive | **clean cut, no shim** (impossible to alias a signature; naming-integrity ruling stands) | per A21 inventory: one collection-type change per repo; scanner `tools/servicename-scan.sh` |
| `SanitizeResponse` → `CoerceValidUTF8` | dashboard only (`dashboard.go`, 1 prod + 1 test file) | compile error, 1-line | **alias + deprecate** (trivial wrapper) | quietest possible: single consumer notified via go-appkit#25-era bridge notes |
| `Check.Since` → `StatusSince` (Go only, JSON `since` frozen) | dashboard only (`status.go:311,391` prod + 3 test files' literals) | compile error at named-literal sites | **clean cut** — a field cannot be aliased in Go; wire untouched so dashboards/payloads see no change | pair with the doc pass in check-metadata-design.md |
| `WithGETOnly` removal | KeyHolderAI test file only (`health_probe_http_test.go`) — no prod usage anywhere (lighter than the adoption-matrix "legacy path" read) | test compile error | **remove at v0.6** after one nudge to KeyHolderAI (deprecation live since v0.1.1) | delete + CHANGELOG migration line: replace with `WithAllowedMethods(http.MethodGet)` |
| `Probe`/`Prober`/`Source`/`Remote` lexicon | dashboard (aggregate+federation deep), bridges, docs fleet-wide | widest blast of the set | **defer past v0.6**: highest coupling, lowest lie-factor of the set (names are consistent within their packages); revisit only with vocabulary-reconciliation.md enforcement tooling | keep as vocabulary doc, not a rename row |

Order at window-open: ServiceName cut (A21, unblocks the alias decision) →
WithCriticalChecks alias → SanitizeResponse alias → StatusSince →
WithGETOnly removal → lexicon (deferred). Each rides its own minor-tagged
CHANGELOG entry with the mechanical migration snippet.
