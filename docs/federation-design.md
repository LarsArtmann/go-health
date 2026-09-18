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
  rationale as `aggregate` and `Probe`).
- **Readiness** — serves the merged view: 503 on overall fail (which
  includes any unreachable remote and any shutting-down remote), 200
  otherwise. A dark remote takes the hub out of rotation — deliberate:
  the hub's whole answer is its remotes.
- **Startup** — fetches (which is also how latches move), then: 200 when
  every remote has answered successfully at least once; 503 otherwise
  with one fail check per not-yet-latched remote. The latch is one-way
  per remote (atomic), mirroring `Probe`'s startup latch; a remote that
  flaps after latching does not re-block startup.

`StartupComplete()` reports the AND of the latches.

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
func (p *Prober) RefreshInterval() time.Duration // always 0
func (p *Prober) LivenessHandler() http.HandlerFunc
func (p *Prober) ReadinessHandler() http.HandlerFunc
func (p *Prober) StartupHandler() http.HandlerFunc
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
