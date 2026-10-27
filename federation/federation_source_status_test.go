package federation_test

import (
	"sync"
	"testing"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// TestSourceStatuses_PerRemoteVerdicts pins the accessor that mirrors
// aggregate.Aggregate.SourceStatuses: one honest roll-up per remote,
// including the draining and unreachable cases that a row fold cannot see.
func TestSourceStatuses_PerRemoteVerdicts(t *testing.T) {
	t.Parallel()

	healthy := serveDocument(t, health.Response{Status: health.StatusPass})
	degraded := serveDocument(t, health.Response{Status: health.StatusWarn})
	draining := serveDocument(t, health.Response{Status: health.StatusPass, ShuttingDown: true})

	prober := mustNew(t, []federation.Remote{
		{Name: "healthy", URL: healthy.server.URL},
		{Name: "degraded", URL: degraded.server.URL},
		{Name: "draining", URL: draining.server.URL},
		{Name: "dark", URL: darkUpstream()},
	})

	statuses := prober.SourceStatuses()

	want := map[string]health.Status{
		"healthy":  health.StatusPass,
		"degraded": health.StatusWarn,
		"draining": health.StatusFail,
		"dark":     health.StatusFail,
	}

	if len(statuses) != len(want) {
		t.Fatalf("statuses = %v, want one entry per remote (%d)", statuses, len(want))
	}

	for name, expected := range want {
		if got, ok := statuses[name]; !ok || got != expected {
			t.Errorf("%s = %q (present=%v), want %q", name, got, ok, expected)
		}
	}
}

// TestSourceStatuses_MatchesMergedWorstOf pins the invariant the dashboard
// relies on: the merged roll-up is the worst per-remote verdict.
func TestSourceStatuses_MatchesMergedWorstOf(t *testing.T) {
	t.Parallel()

	healthy := serveDocument(t, health.Response{Status: health.StatusPass})
	degraded := serveDocument(t, health.Response{Status: health.StatusWarn})

	prober := mustNew(t, []federation.Remote{
		{Name: "healthy", URL: healthy.server.URL},
		{Name: "degraded", URL: degraded.server.URL},
	})

	worst := health.StatusPass
	for _, status := range prober.SourceStatuses() {
		if status.Rank() < worst.Rank() {
			worst = status
		}
	}

	if merged := prober.CachedResponse(); merged.Status != worst {
		t.Errorf("merged status = %q, worst source status = %q; they must agree",
			merged.Status, worst)
	}
}

// TestSourceStatuses_DoesNotAdvanceStartupLatch pins the read-only contract:
// asking for per-remote verdicts must not satisfy an unrelated startup probe.
func TestSourceStatuses_DoesNotAdvanceStartupLatch(t *testing.T) {
	t.Parallel()

	healthy := serveDocument(t, health.Response{Status: health.StatusPass})

	prober := mustNew(t, []federation.Remote{
		{Name: "healthy", URL: healthy.server.URL},
	})

	_ = prober.SourceStatuses()

	if prober.StartupComplete() {
		t.Error("SourceStatuses advanced the startup latch; it must be read-only")
	}
}

// TestSourceStatuses_HubDrainLeavesRemoteVerdictsAlone pins the boundary:
// a hub marking itself draining changes the hub's readiness, not any
// remote's own health.
func TestSourceStatuses_HubDrainLeavesRemoteVerdictsAlone(t *testing.T) {
	t.Parallel()

	healthy := serveDocument(t, health.Response{Status: health.StatusPass})

	prober := mustNew(t, []federation.Remote{
		{Name: "healthy", URL: healthy.server.URL},
	})

	prober.MarkShuttingDown()

	if got := prober.SourceStatuses()["healthy"]; got != health.StatusPass {
		t.Errorf("healthy = %q after the hub drained, want pass", got)
	}

	if merged := prober.CachedResponse(); merged.Status != health.StatusFail {
		t.Errorf("merged status = %q after the hub drained, want fail", merged.Status)
	}
}

// TestSourceStatuses_ConcurrentWithMarkShuttingDown is the race-detector case
// for the drain boundary: readers hammer SourceStatuses and the merged view
// while another goroutine flips the hub's drain. Neither accessor may observe
// a torn or partially updated state — the per-remote map stays exactly one
// entry per remote with a known status across the whole run, and the drain
// flag is an atomic load rather than shared map state. Run under -race.
func TestSourceStatuses_ConcurrentWithMarkShuttingDown(t *testing.T) {
	t.Parallel()

	healthy := serveDocument(t, health.Response{Status: health.StatusPass})
	warning := serveDocument(t, health.Response{
		Status: health.StatusWarn,
		Checks: map[string]health.Check{"q": {Status: health.StatusWarn}},
	})

	prober := mustNew(t, []federation.Remote{
		{Name: "healthy", URL: healthy.server.URL},
		{Name: "warning", URL: warning.server.URL},
	})

	const readers = 8

	var wg sync.WaitGroup

	for range readers {
		wg.Go(func() {
			for range 4 {
				statuses := prober.SourceStatuses()
				if len(statuses) != 2 {
					t.Errorf("statuses = %v, want two remotes", statuses)

					return
				}

				switch statuses["healthy"] {
				case health.StatusPass, health.StatusWarn,
					health.StatusFail, health.StatusOff:
				default:
					t.Errorf("healthy status %q left the enum", statuses["healthy"])

					return
				}

				merged := prober.CachedResponse()
				if merged.Status == "" {
					t.Error("merged status is empty under a concurrent drain")

					return
				}
			}
		})
	}

	wg.Go(func() {
		for range 4 {
			prober.MarkShuttingDown()
		}
	})

	wg.Wait()

	if !prober.StartupComplete() {
		t.Error("latches never advanced; the fixture did not exercise the read path")
	}
}
