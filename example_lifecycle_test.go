package health_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// ExampleProbe_Start mirrors the README Quick Start's lifecycle: validate
// and start the background cache, register the kubelet routes, and shut down
// cleanly. The HTTP listen step from the README cannot run inside an example.
func ExampleProbe_Start() {
	injector := do.New()

	do.ProvideNamed(injector, "database", func(_ do.Injector) (*exampleDB, error) {
		return &exampleDB{}, nil
	})
	do.ProvideNamed(injector, "redis", func(_ do.Injector) (*exampleDB, error) {
		return &exampleDB{}, nil
	})
	_ = do.MustInvokeNamed[*exampleDB](injector, "database")
	_ = do.MustInvokeNamed[*exampleDB](injector, "redis")

	probe := health.New(injector,
		health.WithCriticalServices("database", "redis"),
		health.WithVersion("1.0.0"),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := probe.Start(ctx); err != nil {
		fmt.Println("start:", err)

		return
	}

	defer probe.Shutdown()

	mux := http.NewServeMux()
	probe.RegisterRoutes(mux, health.DefaultRoutes())

	fmt.Println("routes registered: 4")

	// Output:
	// routes registered: 4
}

// ExampleProbe_Healthz shows the single-endpoint combined health handler:
// one URL answering "should traffic be routed here?". It stays 503 until the
// startup latch is set, then follows readiness.
func ExampleProbe_Healthz() {
	probe := health.NewWithHealthCheck(func(_ context.Context) map[string]error {
		return map[string]error{"database": nil}
	},
		health.WithCriticalServices("database"),
	)

	get := func(handler http.HandlerFunc, path string) int {
		w := httptest.NewRecorder()

		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		if err != nil {
			panic(err)
		}

		handler(w, r)

		return w.Code
	}

	booting := get(probe.Healthz(), "/healthz")
	latched := get(probe.StartupHandler(), "/startupz") // all critical services pass: latch flips
	ready := get(probe.Healthz(), "/healthz")

	fmt.Println("while booting:", booting)
	fmt.Println("startup latch:", latched)
	fmt.Println("after latch:", ready)

	// Output:
	// while booting: 503
	// startup latch: 200
	// after latch: 200
}

// ExampleProbe_AwaitReady shows blocking on readiness before accepting
// traffic: AwaitReady polls the cached view until it turns ready, so with a
// background cache it returns as soon as the first refresh lands.
func ExampleProbe_AwaitReady() {
	probe := health.NewWithHealthCheck(func(_ context.Context) map[string]error {
		return map[string]error{"database": nil}
	},
		health.WithCriticalServices("database"),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := probe.Start(ctx); err != nil {
		panic(err)
	}

	defer probe.Shutdown()

	ctxReady, cancelReady := context.WithTimeout(context.Background(), time.Second)
	defer cancelReady()

	fmt.Println("await:", probe.AwaitReady(ctxReady))

	// Output:
	// await: <nil>
}

// ExampleProbe_MarkShuttingDown shows the manual two-phase drain: flip the
// flag first (readiness 503, liveness stays 200 so the kubelet does not
// restart the pod), stop accepting work, then call Shutdown to stop the
// refresh loop. WithShutdownGracePeriod automates the delay.
func ExampleProbe_MarkShuttingDown() {
	probe := health.NewWithHealthCheck(func(_ context.Context) map[string]error {
		return map[string]error{"db": nil}
	},
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	get := func(handler http.HandlerFunc, path string) int {
		w := httptest.NewRecorder()

		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		if err != nil {
			panic(err)
		}

		handler(w, r)

		return w.Code
	}

	fmt.Println("before: liveness", get(probe.LivenessHandler(), "/healthz"),
		"readiness", get(probe.ReadinessHandler(), "/readyz"))

	probe.MarkShuttingDown()

	fmt.Println("draining: liveness", get(probe.LivenessHandler(), "/healthz"),
		"readiness", get(probe.ReadinessHandler(), "/readyz"))

	probe.Shutdown()

	// Output:
	// before: liveness 200 readiness 200
	// draining: liveness 200 readiness 503
}

// ExampleProbe_CachedResponse shows live evaluation versus the background
// cache: Evaluate runs a fresh batch on every call, while CachedResponse
// serves the last background result without touching any dependency.
func ExampleProbe_CachedResponse() {
	var batches int

	probe := health.NewWithHealthCheck(func(_ context.Context) map[string]error {
		batches++

		return map[string]error{"db": nil}
	},
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(time.Hour), // cache effectively frozen
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := probe.Start(ctx); err != nil {
		panic(err)
	}

	defer probe.Shutdown()

	_ = probe.Evaluate(context.Background()) // live batch: 2nd overall
	cached := probe.CachedResponse()         // cached read: no batch

	fmt.Println("status:", cached.Status)
	fmt.Println("checks in cache:", len(cached.Checks))
	fmt.Println("batches run:", batches)

	// Output:
	// status: pass
	// checks in cache: 1
	// batches run: 2
}

// WithShutdownGracePeriod turns the manual MarkShuttingDown + sleep +
// Shutdown sequence into one call: Shutdown flips to draining immediately
// (readiness 503, Alive false), keeps the background refresh loop running
// for the grace window, then stops it.
func ExampleWithShutdownGracePeriod() {
	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil}
	},
		health.WithRefreshInterval(10*time.Millisecond),
		health.WithShutdownGracePeriod(100*time.Millisecond),
	)

	ctx := context.Background()

	if err := probe.Start(ctx); err != nil {
		log.Fatal(err)
	}

	done := make(chan struct{})

	go func() {
		probe.Shutdown()

		close(done)
	}()

	// Inside the grace window the probe is already draining, while the
	// refresh loop keeps the served state fresh for dashboards.
	time.Sleep(30 * time.Millisecond)

	fmt.Println("draining:", !probe.Alive())

	<-done

	fmt.Println("alive after shutdown:", probe.Alive())

	// Output:
	// draining: true
	// alive after shutdown: false
}

// AsShutdowner adapts the probe to do.ShutdownerWithError, so samber/do can
// shut the probe down like any other registered service.
func ExampleProbe_AsShutdowner() {
	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil}
	}, health.WithRefreshInterval(0))

	injector := do.New()

	do.ProvideNamed(
		injector,
		"probe-shutdowner",
		func(do.Injector) (do.ShutdownerWithError, error) {
			return probe.AsShutdowner(), nil
		},
	)

	service := do.MustInvokeNamed[do.ShutdownerWithError](injector, "probe-shutdowner")

	if err := service.Shutdown(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("alive after do-shutdown:", probe.Alive())

	// Output:
	// alive after do-shutdown: false
}
