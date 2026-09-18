package federation_test

import (
	"fmt"
	"log"
	"net/http"
	"time"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// ExampleNew builds a federated view over two services' readiness
// endpoints and registers the hub's own kubelet handlers, the way a
// health.home.lan hub would. Point a go-health-dashboard at the hub to
// render it: dashboard.New(fed) accepts a *federation.Prober unchanged.
func ExampleNew() {
	fed, err := federation.New(
		[]federation.Remote{
			{Name: "jellyfin", URL: "http://jellyfin.lan:8096/readyz"},
			{Name: "nas", URL: "http://nas.lan:9102/readyz"},
		},
		federation.WithTimeout(2 * time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	fed.RegisterRoutes(mux, health.DefaultRoutes())

	got := fed.CachedResponse()
	fmt.Println(got.Status)
	// Output depends on whether the remotes are reachable:
	// a refused fetch surfaces as the namespaced fail row
	//   jellyfin/reachable: fail
	//   nas/reachable: fail
	// instead of a silently frozen last-known state.
}
