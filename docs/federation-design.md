# Federation Design — Pulling Remote go-health Instances Into One Surface

|            |                                                                                                                                                                                                                     |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Date**   | 2026-09-18                                                                                                                                                                                                          |
| **Status** | Design note — implemented as `health/federation` (v0.3.0)                                                                                                                                                           |
| **Inputs** | Federation spike (2026-09-03, `docs/planning/archived/2026-09-03_v03-cycle-decisions-notes.md`); the `aggregate` merge-on-read precedent; the dashboard's `Prober` consumer interface and `GroupBySource` rendering |

## Problem

A fleet of services each run a go-health probe (and optionally a
go-health-dashboard). Operating them means visiting N dashboards. The
want: one URL — in the original spike's words, a `health.home.lan` —
that shows every instance's checks, per service, with no per-service
configuration beyond a URL.

The spike already fixed the home: **federation lives in go-health, not
the dashboard.** The dashboard renders, it does not source. This note
designs the go-health side.

## Topology

```
┌─ health.home.lan ────────────────────────────────┐
│  fed := federation.New(remotes)                  │
│  dash := dashboard.New(fed, GroupBySource, ...)  │
└───────┬──────────────┬──────────────┬────────────┘
        │ HTTP GET     │              │
        ▼              ▼              ▼
  jellyfin:9101   nas:9102       pihole:9103
  go-health       go-health      go-health
  /readyz (JSON)  /readyz (JSON) /readyz (JSON)
```

Each remote is any endpoint that answers with a go-health `Response`
JSON document: a bare probe's readiness handler, or a dashboard's
`/health` route (it content-negotiates `Accept: application/json` to
exactly that document). The fetch sends `Accept: application/json` for
this reason.

## Decision: a `federation` subpackage, passive like `aggregate`

`health/federation` mirrors `health/aggregate` in shape and philosophy:

- **Merge-on-read.** Every `CachedResponse` performs one HTTP fetch per
  remote, in parallel, and merges. No goroutines of its own, no
  scheduler, no staleness of its own. Freshness is live; the cost is one
  request per remote per read.
- **The result is a `Prober`.** The exported type is
  `federation.Prober`, satisfying the same five-method surface the
  dashboard's consumer-side `Prober` interface defines (`CachedResponse`,
  `RefreshInterval`, and the three handlers). `dashboard.New(fed)` works
  with zero dashboard changes; `WithGrouping(GroupBySource)` turns the
  namespaced keys into one card per service — the automatic part of
  "automatically aggregated".
- **Scalars do not survive the merge** (`Version`, `Uptime`,
  `InstanceID`, `Timestamp` are per-process and would lie in a federated
  view) — the same rule as `aggregate`. `TotalLatencyMs` is the slowest
  remote's reported batch, mirroring the aggregate's max rule.

### Why pull, not push

Push-based registration (remotes POST their state to the hub) needs a
remote-side agent, a registry, and discovery — three moving parts for a
home fleet. Pull needs a URL per remote and nothing else. Remotes stay
unmodified: any existing go-health deployment federates with zero
upgrade. The pull cost is bounded by the hub's read cadence.

## Semantics

### Merge rules (per `CachedResponse`)

| Input                                                               | Contribution                                                                                                                                     |
| ------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| Remote answers with a valid response                                | Its overall `Status` feeds worst-of; its checks land as `name/check`; `Since`/`DurationNanos` are preserved verbatim; `ShuttingDown` is OR-ed in |
| Remote fails (network, timeout, non-200, undecodable, empty status) | One synthetic check `name/reachable` with status `fail` and the cause as `Error`; contributes overall `fail`; its cached/last state is NOT used  |
| Any remote `ShuttingDown`                                           | Overall status forced to `fail`, mirroring `aggregate` (and `Probe`) semantics                                                                   |

Worst-of uses `Status.Rank()` (fail < warn < pass, unknown ranks as
pass), shared with `aggregate` — one severity ordering for the module.

The synthetic `reachable` check is injected **only on failure**. A
permanent green "reachable" row per service is noise; the failure row is
the honest red that makes a dark remote visible on the worst-of surface
(and in the dashboard's evidence strip, which counts it as a non-pass
observation). Keying the mechanism on the fetch outcome — not on
aggregate internals — also means it works identically for single-remote
configurations.

### `Since` survives federation

`Check.Since` is probe-observed on the remote and carried through the
JSON document unchanged. A federated check's "since 6h" is as truthful
as a local one's, which keeps the dashboard's since-age and evidence
surfaces meaningful across federation. This is the quiet payoff of
pulling the full document instead of a synthetic overall status.

### The three kubelet handlers

- **Liveness** — process-local, always 200/pass with empty checks, no
  fetches. Restarting the hub because a remote is down would cause a
  cascade; the hub being alive says nothing about its remotes (same
  rationale as `aggregate` and `Probe`). Unaffected by the drain.
- **Readiness** — serves the merged view: 503 on overall fail (which
  includes any unreachable remote, any shutting-down remote, and the hub
  itself once marked for drain), 200 otherwise. A dark remote takes the
  hub out of rotation — deliberate: the hub's whole answer is its
  remotes.
- **Startup** — fetches (which is also how latches move), then: 200 when
  every remote has answered successfully at least once; 503 otherwise
  with one fail check per not-yet-latched remote. The latch is one-way
  per remote (atomic), mirroring `Probe`'s startup latch; a remote that
  flaps after latching does not re-block startup. Unaffected by the
  drain.

`StartupComplete()` reports the AND of the latches.

### `SourceStatuses`: the per-remote roll-up

`Prober.SourceStatuses()` answers the question a hub dashboard asks next —
"is remote `api` healthy overall?" — without refolding its `api/*` check
keys. It returns one entry per remote name from the same
one-parallel-fetch-per-remote pass the merged read performs, so calling
both costs two passes; callers that need both should prefer the merged
view's namespaced checks or two reads.

- An unreachable or undecodable remote reports `fail` for itself,
  mirroring its synthetic `name/reachable` row.
- A remote whose document carries `shutting_down` also reports `fail`, so
  a draining remote's card can never look greener than the page.
- The hub's own drain does **not** rewrite remote statuses: it is a
  property of this hub, not of any remote.
- Reads that only need per-remote verdicts **do not advance startup
  latches**; only the readiness/startup/healthz paths do.

Every reported value is `pass`/`warn`/`fail`/`off`. A document whose own
roll-up is outside that set is refused whole at decode, so the map can
never carry a value the dashboard would rank as healthy without knowing
it. The accessor mirrors `aggregate.Aggregate.SourceStatuses`, so one
capability interface covers both merge surfaces. Full rationale:
docs/federation-source-statuses-design.md.

### `Healthz`: the single-endpoint view

`Prober.Healthz()` exists for deployments behind an external load balancer
that does not speak the kubelet three-probe contract. It returns 503 while
any per-remote startup latch is unset, the merged roll-up is `fail` (which
already covers unreachable remotes, shutting-down remotes, and this hub
once marked for drain), or the hub itself is draining; 200 otherwise with
the merged body. `warn` stays 200, matching `Probe.Healthz` and the
aggregate. It is standalone: `RegisterRoutes` still wires only the three
kubelet probes, and `health.DefaultRoutes` already claims `/healthz` for
liveness. Cost is one fetch wave per request — the same as readiness, so
the polling guidance above applies unchanged. Full rationale:
docs/federation-healthz-design.md.

### Two-phase drain (`MarkShuttingDown`)

`Prober.MarkShuttingDown()` flips a one-way hub-side flag that the
merged read overlays (fail + `shutting_down: true`), so a hub behind a
load balancer answers 503 for the rest of its drain window while the
merge-on-read fetches keep running — drained bodies stay fresh, and a
dashboard keeps seeing real remote state. This is the federation
counterpart of `Probe.MarkShuttingDown`: the one surface whose drain a
caller cannot reach by marking remotes (aggregate's drain flows through
its sources' own probes). The overlay consults the flag live on every
read, so the flip takes effect on the next request even with every
remote healthy. There is no unmark and no `Shutdown` counterpart: the
prober runs no background loop, so phase two of the drain is the host
process draining its own HTTP server. Liveness and startup are
unaffected, mirroring `Probe`.

Operational note for the drain window: the hub keeps fetching every
remote on every merged read, so its fetch volume during the drain is the
read cadence (dashboard push interval + kubelet/LB polling) × N remotes.
Bound the load balancer's drain window accordingly — a hub holding 503
for minutes while scraped every second still polls each remote every
second.

### `RefreshInterval()` returns zero

Federation fetches live; it has no background cache and no cadence of
its own, and it cannot know the remotes' refresh intervals. Zero is the
honest answer ("live evaluation") and the documented dashboard fallback
kick in: consumers set their own push cadence (`WithPushInterval`).
The push cadence then doubles as the remote polling cadence — for a
home fleet this is exactly the desired "poll everyone every few
seconds" behavior.

## API

```go
type Remote struct {
    Name string // unique, non-empty, no "/": becomes the "name/check" prefix
    URL  string // absolute http(s) URL to a go-health JSON document
}

func New(remotes []Remote, opts ...Option) (*Prober, error)

func WithClient(*http.Client) Option // transport control, pooling, TLS
func WithTimeout(time.Duration) Option // per-fetch deadline, default 5s

func (p *Prober) CachedResponse() health.Response
func (p *Prober) SourceStatuses() map[string]health.Status // per-remote roll-up, own claim verbatim
func (p *Prober) RefreshInterval() time.Duration // always 0
func (p *Prober) MarkShuttingDown()              // one-way drain: readiness 503, fetches continue
func (p *Prober) LivenessHandler() http.HandlerFunc
func (p *Prober) ReadinessHandler() http.HandlerFunc
func (p *Prober) StartupHandler() http.HandlerFunc
func (p *Prober) Healthz() http.HandlerFunc // single-endpoint view for LBs without kubelet probes
func (p *Prober) StartupComplete() bool
func (p *Prober) RegisterRoutes(mux *http.ServeMux, routes health.Routes)
```

Construction validates eagerly (the aggregate contract): at least one
remote, unique non-empty slash-free names, parseable absolute http(s)
URLs, positive timeout. Sentinel errors (`ErrNoRemotes`,
`ErrInvalidRemote`, `ErrInvalidTimeout`) wrap with `%w` + the offending
value, matching the root package's validation style.

Fetch hardening: per-request `context.WithTimeout` deadline (so custom
clients get it too), `LimitReader` cap on response bodies (1 MiB —
health payloads are kilobytes; the cap bounds a misbehaving remote),
body drain on non-200 responses for connection reuse.

## Non-goals (deliberate)

- **No cache/TTL.** Every read is live. The hub's read cadence (push
  interval + probe polling) is a fine remote load for a home fleet; if a
  deployment scrapes aggressively, `WithClient` carries the fix (a
  caching transport) without new package surface. Revisit
  `WithCacheTTL` only with a concrete demand signal.
- **No push/registration protocol.** Pull keeps remotes zero-config.
- **No per-remote auth in v1.** A LAN fleet needs none; `WithClient`
  covers custom transports when one does. Adding `Remote.Headers` later
  is additive.
- **No per-remote interval tracking, no staleness gauge here.** Freeze
  detection is the dashboard's job (the 2026-09-17 staleness design keys
  on the response snapshot, which federation feeds — the designs
  compose without touching).
- **No health status synthesis for staleness.** An unreachable remote
  reports `fail` because the fetch failed — that is an observation, not
  a guess about the remote's services.
- **No unmark, no drain-state queries.** The mark is one-way by design
  (mirroring `Probe`); a host that needs to undo a drain decision
  replaces the prober. The drain is observable through
  `CachedResponse().ShuttingDown` — there is deliberately no `IsDraining()`
  getter to keep beside it.
- **No `ErrUnknownCriticalService` on the fetch side.** `Start()`-time
  critical-name validation applies to a probe's own evaluation batch;
  remote names are a different universe (a typo'd remote name surfaces
  as its `name/reachable` fail row instead). See
  [federation-validation-semantics.md](federation-validation-semantics.md).

## Alternatives considered

- **Dashboard-side source** (a `federation` option in go-health-dashboard).
  Rejected by the spike: the dashboard would gain an HTTP client, remote
  semantics, and configuration surface — all source concerns — and every
  other go-health consumer (metrics scrapers, load balancers) would have
  to reimplement it. The dashboard renders.
- **Federation as synthetic checks on a regular `Probe`**
  (`NewWithHealthCheck` mapping each remote to `map[string]error`).
  Simplest possible API, but it discards per-check detail: the hub would
  show "nas: fail" instead of which of nas's checks failed, and `Since`
  would be hub-observed (reset on hub restart) instead of probe-observed.
  The full-document merge costs little more and loses nothing.
- **SSE/streaming from remotes.** A second protocol surface (the
  WebSocket spike's rejection applies verbatim) with no hub use case for
  push: the hub polls on its own cadence anyway.
