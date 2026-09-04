package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	do "github.com/samber/do/v2"
)

// BenchmarkStartupHandler_Contention measures the startup handler's true
// worst case: a failing critical service keeps the startup latch open, so
// every concurrent request runs a full live evaluation batch (and gets 503).
// It complements BenchmarkStartupHandler_Unlatched, whose first successful
// batch latches the probe and therefore only measures the latch check.
// Recorded baseline (2026-09-04, go1.26.7 linux/amd64, 32 threads): see
// FEATURES.md "Performance".
func BenchmarkStartupHandler_Contention(b *testing.B) {
	injector := do.New()

	provideUnhealthy(injector, "db", "still booting")
	do.MustInvokeNamed[*unhealthyService](injector, "db")

	provideHealthy(injector, "cache")
	do.MustInvokeNamed[*healthyService](injector, "cache")

	b.Cleanup(func() { injector.Shutdown() })

	probe := health.New(injector,
		health.WithCriticalServices("db", "cache"),
		health.WithRefreshInterval(0), // live evaluation: every request hits the batch
	)

	handler := probe.StartupHandler()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/startupz", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()

		for pb.Next() {
			w.Body.Reset()
			handler(w, r)
		}
	})
}

// BenchmarkCachedResponse_ParallelReads measures lock-free cache reads under
// full contention: the upper bound every other read path should stay near.
func BenchmarkCachedResponse_ParallelReads(b *testing.B) {
	injector := do.New()

	provideHealthy(injector, "db")
	do.MustInvokeNamed[*healthyService](injector, "db")

	probe := health.New(
		injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(
			50*time.Millisecond,
		), // background cache, cheap relative to the loop
	)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	wg.Go(func() {
		_ = probe.Start(ctx)
	})

	b.Cleanup(func() {
		cancel()
		probe.Shutdown()
		wg.Wait()
	})

	// let the cache populate once so reads hit p.latest
	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = probe.CachedResponse()
		}
	})
}

// BenchmarkGuardOverhead measures the method-set guard's cost on the liveness
// path across its three operating modes: no guard (handler returned bare),
// method in the allowed set (map hit), and method unlisted (405 path). The
// delta between no-guard and allowed is the guard's true overhead.
func BenchmarkGuardOverhead(b *testing.B) {
	cases := []struct {
		name   string
		opts   []health.Option
		method string
	}{
		{name: "no-guard", method: http.MethodGet},
		{name: "allowed", opts: []health.Option{health.WithAllowedMethods(http.MethodGet)}, method: http.MethodGet},
		{name: "unlisted", opts: []health.Option{health.WithAllowedMethods(http.MethodGet)}, method: http.MethodPost},
	}

	for _, bench := range cases {
		b.Run(bench.name, func(b *testing.B) {
			probe := health.NewWithHealthCheck(
				func(context.Context) map[string]error { return nil },
				bench.opts...,
			)

			handler := probe.LivenessHandler()

			r, err := http.NewRequestWithContext(context.Background(), bench.method, "/healthz", nil)
			if err != nil {
				b.Fatal(err)
			}

			w := httptest.NewRecorder()

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				w.Body.Reset()
				handler(w, r)
			}
		})
	}
}
