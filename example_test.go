package health_test

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// exampleDB is a minimal service that satisfies do.HealthcheckerWithContext.
type exampleDB struct{}

var _ do.HealthcheckerWithContext = (*exampleDB)(nil)

func (*exampleDB) HealthCheck(_ context.Context) error { return nil }

// ExampleNew shows how to create a health Probe wired to a samber/do
// injector, register a critical service, and evaluate its health.
func ExampleNew() {
	injector := do.New()

	do.ProvideNamed(injector, "database", func(_ do.Injector) (*exampleDB, error) {
		return &exampleDB{}, nil
	})
	_ = do.MustInvokeNamed[*exampleDB](injector, "database")

	probe := health.New(injector,
		health.WithVersion("1.0.0"),
		health.WithCriticalServices("database"),
	)

	resp := probe.Evaluate(context.Background())
	fmt.Println("status:", resp.Status)
	fmt.Println("checks:", len(resp.Checks))

	// Output:
	// status: pass
	// checks: 1
}

// ExampleProbe_LivenessHandler shows the liveness handler in action. Liveness
// never checks dependencies and always returns 200 with status "pass".
func ExampleProbe_LivenessHandler() {
	injector := do.New()
	probe := health.New(injector, health.WithVersion("1.0.0"))

	handler := probe.LivenessHandler()

	w := httptest.NewRecorder()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	if err != nil {
		panic(err)
	}

	handler(w, r)

	fmt.Println("HTTP status:", w.Code)

	var resp health.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		panic(err)
	}

	fmt.Println("health:", resp.Status)

	// Output:
	// HTTP status: 200
	// health: pass
}

// ExampleProbe_ReadinessHandler shows the readiness handler checking critical
// services. All healthy services return 200; a failing critical service would
// return 503.
func ExampleProbe_ReadinessHandler() {
	injector := do.New()

	do.ProvideNamed(injector, "database", func(_ do.Injector) (*exampleDB, error) {
		return &exampleDB{}, nil
	})
	_ = do.MustInvokeNamed[*exampleDB](injector, "database")

	probe := health.New(injector,
		health.WithCriticalServices("database"),
		health.WithRefreshInterval(0), // live evaluation for deterministic output
	)

	handler := probe.ReadinessHandler()

	w := httptest.NewRecorder()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	if err != nil {
		panic(err)
	}

	handler(w, r)

	fmt.Println("HTTP status:", w.Code)

	// Output: HTTP status: 200
}

// ExampleProbe_RegisterRoutes shows the one-liner for mounting all three
// Kubernetes probe endpoints on a standard http.ServeMux.
func ExampleProbe_RegisterRoutes() {
	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	mux := http.NewServeMux()
	probe.RegisterRoutes(mux, health.DefaultRoutes())

	for _, path := range []string{"/healthz", "/readyz", "/startupz"} {
		w := httptest.NewRecorder()

		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		if err != nil {
			panic(err)
		}

		mux.ServeHTTP(w, r)
		fmt.Printf("%s: %d\n", path, w.Code)
	}

	// Output:
	// /healthz: 200
	// /readyz: 200
	// /startupz: 200
}

// ExampleProbe_Start mirrors the README Quick Start's lifecycle: validate
// and start the background cache, register the kubelet routes, and shut down
// cleanly. The HTTP listen step from the README cannot run inside an example.
func ExampleProbe_Start() {
	injector := do.New()

	do.ProvideNamed(injector, "database", func(_ do.Injector) (*exampleDB, error) {
		return &exampleDB{}, nil
	})
	_ = do.MustInvokeNamed[*exampleDB](injector, "database")

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

	fmt.Println("routes registered: 3")

	// Output:
	// routes registered: 3
}

// ExampleNewWithHealthCheck shows an injector-free probe: the health-check
// batch is an ordinary function, so any check source (composed checks,
// another DI container, external endpoints) can drive the same probe
// handlers.
func ExampleNewWithHealthCheck() {
	probe := health.NewWithHealthCheck(func(_ context.Context) map[string]error {
		return map[string]error{"payments-api": nil}
	},
		health.WithCriticalServices("payments-api"),
	)

	resp := probe.Evaluate(context.Background())

	fmt.Println("status:", resp.Status)
	fmt.Println("checks:", len(resp.Checks))

	// Output:
	// status: pass
	// checks: 1
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

// ExampleWithEvaluationHook shows the metrics seam: a callback invoked
// synchronously after every Evaluate with the fully classified response.
// Feed it to Prometheus, OpenTelemetry, or alerting instead of polling.
func ExampleWithEvaluationHook() {
	var evaluations int

	probe := health.NewWithHealthCheck(func(_ context.Context) map[string]error {
		return map[string]error{"db": nil}
	},
		health.WithEvaluationHook(func(resp health.Response) {
			evaluations++
			fmt.Printf("evaluation %d: %s\n", evaluations, resp.Status)
		}),
	)

	_ = probe.Evaluate(context.Background())
	_ = probe.Evaluate(context.Background())

	// Output:
	// evaluation 1: pass
	// evaluation 2: pass
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

// countingRecorder is a custom HealthRecorder: it observes every batch before
// the probe classifies it. Any type with the matching method satisfies the
// interface — including samber-do-auditlog's Plugin, passable directly.
type countingRecorder struct {
	injector do.Injector
	batches  int
}

func (r *countingRecorder) RecordHealthCheckWithContext(
	ctx context.Context,
	_ do.Injector,
) map[string]error {
	r.batches++

	return r.injector.HealthCheckWithContext(ctx)
}

// ExampleWithHealthRecorder shows a custom recorder wired in front of the
// injector: the probe delegates every batch to it, so observers can log,
// audit, or enrich results without touching the probe.
func ExampleWithHealthRecorder() {
	injector := do.New()

	do.ProvideNamed(injector, "database", func(_ do.Injector) (*exampleDB, error) {
		return &exampleDB{}, nil
	})
	_ = do.MustInvokeNamed[*exampleDB](injector, "database")

	recorder := &countingRecorder{injector: injector}

	probe := health.New(injector,
		health.WithHealthRecorder(recorder),
		health.WithCriticalServices("database"),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	fmt.Println("status:", resp.Status)
	fmt.Println("batches observed:", recorder.batches)

	// Output:
	// status: pass
	// batches observed: 1
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
