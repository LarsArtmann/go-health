package health_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

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

// ExampleNewWithDetailedCheck shows the metadata-rich check function: each
// report carries its execution duration (surfaced as Check.DurationNanos),
// while classification stays with the probe. Combined with the automatic
// Check.StatusSince transition tracking, dashboards can answer "failing since
// when?" and "how slow is it?" without instrumenting every dependency.
func ExampleNewWithDetailedCheck() {
	probe := health.NewWithDetailedCheck(func(ctx context.Context) map[string]health.CheckDetail {
		start := time.Now()

		err := pingDatabase(ctx)

		return map[string]health.CheckDetail{
			"database": {Err: err, Duration: time.Since(start)},
		}
	},
		health.WithCriticalServices("database"),
	)

	resp := probe.Evaluate(context.Background())

	fmt.Println("status:", resp.Status)
	fmt.Println("database since set:", !resp.Checks["database"].StatusSince.IsZero())
	fmt.Println("database timed:", resp.Checks["database"].DurationNanos > 0)

	// Output:
	// status: pass
	// database since set: true
	// database timed: true
}

// pingDatabase stands in for a real dependency ping.
func pingDatabase(context.Context) error { return nil }

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

// ExampleVersionHandler wires the build-version endpoint next to the probe
// routes. In real code the version comes from the build pipeline, stamped
// via -ldflags "-X main.buildVersion=$(git describe --tags)".
func ExampleVersionHandler() {
	buildVersion := "v1.2.3"

	mux := http.NewServeMux()
	mux.HandleFunc("/version", health.VersionHandler(buildVersion))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/version", nil))

	fmt.Println(rec.Code, rec.Body.String())

	// Output:
	// 200 {"version":"v1.2.3"}
}
