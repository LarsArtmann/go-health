Implemented — went with the first pick: fetches keep running during the drain, so the 503 bodies stay fresh.

`federation.Prober.MarkShuttingDown()` (`federation/federation.go:418` at `7cc9239`, one-way, same contract as the root probe). The overlay sits in `cachedResponse` (`federation/federation.go:294`), so `ReadinessHandler` and `CachedResponse()` readers get the same verdict: `shutting_down: true` + status `fail`. Liveness/startup untouched (`federation/federation_test.go:831`). The fresh-body claim is pinned by a test that mutates the remote document mid-drain and watches the new check row appear (`federation/federation_test.go:733`), plus a 25-phase concurrency stress under `-race` (`federation/federation_test.go:870`). Wire format unchanged — `shutting_down` pre-existed. Aggregate stayed out, as scoped.

`nix run .#gates` exit 0 at `7cc9239`.

Closing as completed: implemented in `856c5d2`.
