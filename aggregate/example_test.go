package aggregate_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	health "github.com/larsartmann/go-health"
	aggregate "github.com/larsartmann/go-health/aggregate"
)

// primeCached runs one readiness request so the throttled live path stores
// its evaluation in the probe's cache — exactly what the aggregate's
// merge-on-read consumes. Without it a live-mode probe reports an empty
// cached view.
func primeCached(probe *health.Probe) {
	probe.ReadinessHandler()(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/readyz", nil),
	)
}

func latchStartup(probe *health.Probe) {
	probe.StartupHandler()(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/startupz", nil),
	)
}

// Combine an API probe and a web probe into one health surface with the
// conventional Kubernetes paths. Source names must be unique, non-empty, and
// free of "/" (they become the "name/check" key prefixes).
func ExampleNew() {
	apiProbe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil, "cache": nil}
	},
		health.WithRefreshInterval(time.Second),
		health.WithLiveThrottle(time.Hour),
	)

	webProbe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"render": nil}
	},
		health.WithRefreshInterval(time.Second),
		health.WithLiveThrottle(time.Hour),
	)

	ctx := context.Background()

	if err := apiProbe.Start(ctx); err != nil {
		log.Fatal(err)
	}

	if err := webProbe.Start(ctx); err != nil {
		log.Fatal(err)
	}

	primeCached(apiProbe)
	primeCached(webProbe)

	agg, err := aggregate.New(
		aggregate.Source{Name: "api", Probe: apiProbe},
		aggregate.Source{Name: "web", Probe: webProbe},
	)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	agg.RegisterRoutes(mux, health.DefaultRoutes())

	// The merged view namespaces every check as "source/check" and takes the
	// worst status across sources.
	merged := agg.CachedResponse()

	fmt.Println(merged.Status)
	for _, name := range []string{"api/cache", "api/db", "web/render"} {
		fmt.Println(name, merged.Checks[name].Status)
	}

	// Output:
	// pass
	// api/cache pass
	// api/db pass
	// web/render pass
}

// Merging drops per-process scalars: version, instance_id, and uptime belong
// to a single process and would lie in an aggregate view. The merged status
// is the worst of the sources.
func ExampleNew_scalarsDropped() {
	degraded := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"queue": fmt.Errorf("connection refused")}
	},
		health.WithRefreshInterval(0),
		health.WithLiveThrottle(time.Hour),
		health.WithInstanceID("pod-abc"),
		health.WithVersion("1.2.3"),
	)

	healthy := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil}
	},
		health.WithRefreshInterval(0),
		health.WithLiveThrottle(time.Hour),
	)

	ctx := context.Background()

	if err := degraded.Start(ctx); err != nil {
		log.Fatal(err)
	}

	if err := healthy.Start(ctx); err != nil {
		log.Fatal(err)
	}

	primeCached(degraded)
	primeCached(healthy)

	agg, err := aggregate.New(
		aggregate.Source{Name: "worker", Probe: degraded},
		aggregate.Source{Name: "api", Probe: healthy},
	)
	if err != nil {
		log.Fatal(err)
	}

	merged := agg.CachedResponse()

	fmt.Println(merged.Status)
	fmt.Println(merged.InstanceID == "" && merged.Version == "")
	fmt.Println(merged.Checks["worker/queue"].Status, merged.Checks["worker/queue"].Error)

	// Output:
	// warn
	// true
	// warn connection refused
}

// The aggregate startup handler reports 503 until every source has completed
// its own startup, then latches to 200 without re-checking.
func ExampleAggregate_StartupHandler() {
	fast := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil}
	}, health.WithRefreshInterval(0), health.WithLiveThrottle(time.Hour))

	slow := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"migrate": nil}
	}, health.WithRefreshInterval(0), health.WithLiveThrottle(time.Hour))

	ctx := context.Background()

	if err := fast.Start(ctx); err != nil {
		log.Fatal(err)
	}

	if err := slow.Start(ctx); err != nil {
		log.Fatal(err)
	}

	agg, err := aggregate.New(
		aggregate.Source{Name: "fast", Probe: fast},
		aggregate.Source{Name: "slow", Probe: slow},
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("all booted:", agg.StartupComplete())

	// A successful startup evaluation flips each source's latch — here
	// simulated by one startup request per source:
	latchStartup(fast)
	latchStartup(slow)

	fmt.Println("all booted:", agg.StartupComplete())

	// Output:
	// all booted: false
	// all booted: true
}
