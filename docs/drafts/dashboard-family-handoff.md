# Dashboard-family handoff drafts

**Date:** 2026-10-27 · **Status:** 2 drafts ready to post, 2 items already
resolved upstream/elsewhere (verified, so no draft)
**Milestone:** M21 · **Owner acts:** posting is `gh issue create` in
`LarsArtmann/go-health-dashboard`, nothing here is filed yet.

Every item below was checked against the current code before drafting — two of
the four plans in the backlog turned out to be already done, and a draft for
them would have been noise.

| Item                                                | State at 2026-10-27                                                       | Action        |
| --------------------------------------------------- | ------------------------------------------------------------------------- | ------------- |
| fir DD9 — adopt templ-components `HTMLDataAttrs`    | **RESOLVED**: `layout.Base.HTMLDataAttrs` exists at templ-components v1.24.0, and file-and-image-renamer retired `datastarCSPBoot` itself (`healthd/views/nonce_attrs.go`, pinned by `healthd/csp_root_attrs_test.go`) | none |
| dashboard#17 — nightly fuzz                         | **RESOLVED**: `.github/workflows/fuzz.yml` runs all 13 targets daily at 60s each with an issues-on-failure step | none |
| dashboard#14 — `Shutdown` does not cancel `Start`'s ctx | **OPEN**, confirmed in source                                          | draft below   |
| hub SIGTERM → `federation.MarkShuttingDown`         | **OPEN**, unblocked by go-health v0.8.0 (the hub comment still says the method does not exist) | draft below |

---

## Draft 1 — dashboard#14: `Shutdown` leaves the pusher goroutine running

Proposed title: `Shutdown does not cancel Start's context, so the pusher goroutine outlives it`

````markdown
> [!NOTE]
> This filing was drafted by DeepSeek-V4.1-Flash via Crush from an AI-run fleet
> review, not at my request.
>
> - [ ] MANUALLY REVIEWED by `@Lars Artmann` at `[<date-time>]`

`Dashboard.Shutdown()` stops the pusher by closing the broadcaster, but it never
cancels the context `Start` was given — so the goroutine only exits when the
*caller* cancels, not when the Dashboard is shut down.

## Problem

`pusher.start` has exactly one graceful exit:

```go
// pusher.go:92-108
func (p *pusher) start(ctx context.Context) {
	...
	for {
		select {
		case <-ctx.Done():
			p.broadcaster.Close()
			return
		case <-ticker.C:
			p.broadcast()
		}
	}
}
```

`Shutdown` closes the broadcaster and returns without touching that context:

```go
// dashboard.go:243-258
func (d *Dashboard) Shutdown() {
	p := d.push.Swap(nil)
	if p == nil {
		return
	}
	...
	p.broadcaster.Close()
}
```

`Start` (dashboard.go:228-237) passes the caller's ctx straight to the
goroutine and keeps no cancel function:

```go
d.push.Store(p)
d.started.Store(true)
go p.start(ctx)
```

## Why it matters

- A caller whose ctx outlives the Dashboard (`signal.NotifyContext` for the
  process, or `context.Background()` in a test) leaks one goroutine per
  Dashboard, still ticking at `interval` and calling `broadcast()` against a
  closed broadcaster for the rest of the process.
- The hub masks it: `cmd/health-hub/main.go:230` has `defer cancel()`, so the
  leak is invisible in the single deployment that runs today. Anything that
  restarts a Dashboard in-process (tests, a control loop, a config reload) pays
  it.
- go-health's `Probe` already solved the identical shape — `Start` stores a
  cancel function under a mutex, `Shutdown` cancels and waits:
  `probe.go:522-560` (Start) and `probe.go:660-700` (Shutdown) in
  `github.com/larsartmann/go-health` v0.8.1.

## Proposal

- `Start`: derive `ctx, cancel := context.WithCancel(ctx)`, store `cancel` (plus
  a done channel or `sync.WaitGroup`) on the Dashboard under the existing lock.
- `Shutdown`: call `cancel()` after the drain window and wait for the pusher to
  return before returning, so "Shutdown returned" means "nothing is ticking".
- Test: `Start(context.Background())`, then `Shutdown()`, and assert the pusher
  stopped within a bounded wait — no caller cancel involved.

Keeping the current `ErrPusherShutDown` behaviour is unaffected: the state swap
happens first, the context cancel only stops the loop.

💘 Generated with Crush
````

---

## Draft 2 — hub SIGTERM should drain readiness, not just stop the server

Proposed title: `hub: flip /readyz to 503 on SIGTERM (federation.MarkShuttingDown now exists)`

````markdown
> [!NOTE]
> This filing was drafted by DeepSeek-V4.1-Flash via Crush from an AI-run fleet
> review, not at my request.
>
> - [ ] MANUALLY REVIEWED by `@Lars Artmann` at `[<date-time>]`

`/readyz` cannot drain today because the hub never marks the federation prober
as shutting down, and its package comment still claims the method does not
exist — go-health v0.8.0 shipped `federation.Prober.MarkShuttingDown()`.

## Problem

The hub serves the federation's own readiness handler:

```go
// cmd/health-hub/main.go:238, 316-320
mux := newServeMux(dash)
...
dash.RegisterRoutes(mux) // /readyz -> d.probe.ReadinessHandler()
```

and documents the gap as permanent:

```go
// cmd/health-hub/main.go:38-42
// Drain-safety limitation: go-health's federation Prober has no
// MarkShuttingDown, so /readyz keeps answering from merge-on-read fetches
// until the listener closes ...
```

On SIGTERM the code goes straight to `dash.Shutdown()` and
`server.Shutdown()` (`cmd/health-hub/main.go:288-290`). From the moment the
signal lands until the listener closes, `/readyz` keeps answering 200, so a
load balancer can keep routing new traffic to a process that is already
leaving.

go-health v0.8.0 added the missing half: `federation.Prober.MarkShuttingDown()`
is one-way, keeps fetches running (bodies stay fresh), flips `ReadinessHandler`
to 503 with `shutting_down: true`, and leaves `/healthz` and `/startupz`
answering 200. The same release added `federation.Prober.Healthz()`, which the
dashboard's `Routes.Healthz` option can now enable because the prober satisfies
`Healthzer` structurally.

## Proposal

1. On SIGTERM, before stopping anything: `fed.MarkShuttingDown()`.
2. Keep the existing drain order (dashboard, then server) so SSE clients are
   still disconnected promptly — the mark is additive, it does not replace the
   drain window.
3. Replace the package comment's "Drain-safety limitation" paragraph with the
   new contract, including that the flag is one-way and that `/healthz` stays
   200 by design.
4. Bump `github.com/larsartmann/go-health` to v0.8.1 in `go.mod` (v0.8.0's tag
   ran red CI; v0.8.1 is the same library with a fixed gate).
5. End-to-end assertion: hold an SSE client, send SIGTERM, poll `/readyz`
   during the drain window and assert 503 + `shutting_down: true` while
   `/healthz` still answers 200. `cmd/health-hub/main_test.go` already has a
   `stubProber` harness to extend.

💘 Generated with Crush
````

---

## Why two drafts instead of four

The two closed items were re-verified rather than trusted:

- **DD9** is not blocked upstream any more — `HTMLDataAttrs` is in
  templ-components `v1.24.0` (`layout/base.templ:148-156`, rendered at
  `layout/base.templ:231`), and file-and-image-renamer already dropped the boot
  script: `healthd/views/nonce_attrs.go` builds the root attribute payload and
  `healthd/csp_root_attrs_test.go:84` fails if `datastarCSPBoot` reappears.
- **dashboard#17** exists: `.github/workflows/fuzz.yml` (daily 03:00, 60s per
  target, 13 targets including `FuzzParseRemotes` in `cmd/health-hub`, crasher
  printing and a duplicate-aware issue opener). The improvement worth having is
  a longer budget, not a new workflow.

Both rows were stale TODO_LIST/plan entries; they are struck there rather than
carried forward.
