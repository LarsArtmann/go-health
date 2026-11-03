// Minimal repro: method-less probe patterns panic a consumer's page-router mux.
// Run: go run ./docs/drafts/finding1_repro
package main

import (
	"context"
	"net/http"

	"github.com/larsartmann/go-health"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{slug}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error { return map[string]error{} })
	probe.RegisterRoutes(mux, health.DefaultRoutes())
	_ = http.Server{Handler: mux}
}
