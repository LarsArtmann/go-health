package federation_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// Combine two services' health endpoints into one federated surface with
// the conventional Kubernetes paths. Remote names must be unique,
// non-empty, and free of "/" (they become the "name/check" key prefixes).
// Point a go-health-dashboard at the Prober to render the federation: it
// satisfies the dashboard's consumer interface unchanged.
func ExampleNew() {
	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil, "cache": nil}
	}, health.WithRefreshInterval(0))

	api := httptest.NewServer(probe.ReadinessHandler())
	defer api.Close()

	web := httptest.NewServer(probe.ReadinessHandler())
	defer web.Close()

	fed, err := federation.New(
		[]federation.Remote{
			{Name: "api", URL: api.URL},
			{Name: "web", URL: web.URL},
		},
	)
	if err != nil {
		fmt.Println("error:", err)

		return
	}

	mux := http.NewServeMux()
	fed.RegisterRoutes(mux, health.DefaultRoutes())

	// The merged view namespaces every check as "remote/check" and takes
	// the worst status across remotes.
	merged := fed.CachedResponse()

	fmt.Println(merged.Status)

	for _, name := range []string{"api/cache", "api/db", "web/db"} {
		fmt.Println(name, merged.Checks[name].Status)
	}

	// Output:
	// pass
	// api/cache pass
	// api/db pass
	// web/db pass
}

// ExampleProber_MarkShuttingDown shows the federation hub's two-phase
// graceful shutdown: flip the flag first (readiness 503 so the load
// balancer drains the hub, liveness stays 200 so the kubelet does not
// restart the pod), then close the listener. The fetches keep running
// while draining, so the readiness bodies stay fresh.
func ExampleProber_MarkShuttingDown() {
	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil}
	})

	upstream := httptest.NewServer(probe.ReadinessHandler())
	defer upstream.Close()

	fed, err := federation.New([]federation.Remote{{Name: "api", URL: upstream.URL}})
	if err != nil {
		fmt.Println("error:", err)

		return
	}

	get := func(handler http.HandlerFunc, path string) int {
		w := httptest.NewRecorder()

		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		if err != nil {
			panic(err)
		}

		handler(w, r)

		return w.Code
	}

	fmt.Println("before: liveness", get(fed.LivenessHandler(), "/healthz"),
		"readiness", get(fed.ReadinessHandler(), "/readyz"))

	fed.MarkShuttingDown()

	fmt.Println("draining: liveness", get(fed.LivenessHandler(), "/healthz"),
		"readiness", get(fed.ReadinessHandler(), "/readyz"))

	// Output:
	// before: liveness 200 readiness 200
	// draining: liveness 200 readiness 503
}
