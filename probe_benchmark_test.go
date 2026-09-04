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

// BenchmarkStartupHandler_Contention measures the startup handler with many
// goroutines requesting concurrently while the latch is still open, so every
// request runs a live evaluation batch. This is the worst-case startup shape
// (pre-latch), complementing BenchmarkStartupHandler_Unlatched's serial loop.
// Recorded baseline (2026-09-04, go1.26.7 linux/amd64, 8 cores):
//
//	BenchmarkStartupHandler_Contention-8   proportional to service count;
//	per-op cost ≈ one full health-check batch, lock-free on p.startupPassed.
func BenchmarkStartupHandler_Contention(b *testing.B) {
	injector := do.New()
	b.Cleanup(func() { injector.Shutdown() })

	provideHealthy(injector, "db")
	do.MustInvokeNamed[*healthyService](injector, "db")

	provideHealthy(injector, "cache")
	do.MustInvokeNamed[*healthyService](injector, "cache")

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
	b.Cleanup(func() { injector.Shutdown() })

	provideHealthy(injector, "db")
	do.MustInvokeNamed[*healthyService](injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(50*time.Millisecond), // background cache, cheap relative to the loop
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
