# `AwaitReady` cache-aware poll interval — mini design note

|            |                                                                                                 |
| ---------- | ----------------------------------------------------------------------------------------------- |
| **Date**   | 2026-10-09                                                                                      |
| **Status** | SKETCHED — demand-gated; implement only when a concrete consumer need appears (ROADMAP Theme 1) |

## Problem

`Probe.AwaitReady` polls the cached view on a fixed 50 ms ticker. With a
background cache (the default, 1 s refresh) the cache cannot change between
ticks more often than the refresh interval, so most of those wakeups read
identical state; in the opposite regime a consumer with a very slow refresh
(10 s) still pays 200 wakeups per fresh observation. The interval should
track how fast the observed state can actually change.

## Proposed rule

```go
interval := refreshInterval / 2
if refreshInterval <= 0 { interval = 50 * time.Millisecond } // live mode
interval = clamp(interval, 10*time.Millisecond, 500*time.Millisecond)
```

- **refreshInterval/2:** Nyquist-style — polling at twice the state-change
  rate bounds observed staleness after readiness flips to at most one
  refresh period, while never polling faster than the cache can answer
  differently.
- **Live mode (interval 0):** the cache updates only when other callers
  trigger throttled evaluations, which can happen at any moment — the
  fixed 50 ms stays.
- **Clamps:** 10 ms floor keeps a misconfigured tiny refresh from turning
  into a spin; 500 ms ceiling bounds added boot-detection latency for very
  slow refreshes (a 10 s refresh polls at 500 ms, observing a flip within
  one refresh + 500 ms worst case).
- The clock seam (`WithNowFunc`) does not apply: the ticker is wall-clock
  sleep, not measured latency — tests keep using generous ctx timeouts.

## Why demand-gated

The fixed 50 ms is correct and cheap (an atomic load per tick). The
cache-aware interval only matters for consumers that (a) call `AwaitReady`
with tight deadlines and (b) run slow refresh intervals — no known fleet
member does both. Ship when one appears; the rule above is the whole
implementation plus a table test over {live, 20 ms, 1 s, 10 s} asserting
the chosen interval and first-observation latency bounds.
