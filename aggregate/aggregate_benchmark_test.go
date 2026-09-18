package aggregate_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	aggregate "github.com/larsartmann/go-health/aggregate"
)

// benchAggregate builds an aggregate over n started, cache-primed sources,
// each contributing three healthy checks. The throttled live path stores its
// first evaluation, which is exactly what the aggregate's merge-on-read
// consumes.
func benchAggregate(b *testing.B, sourceCount int) *aggregate.Aggregate {
	b.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	if err != nil {
		b.Fatal(err)
	}

	sources := make([]aggregate.Source, 0, sourceCount)

	for range sourceCount {
		probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
			return map[string]error{"db": nil, "cache": nil, "queue": nil}
		},
			health.WithRefreshInterval(0),
			health.WithLiveThrottle(time.Hour),
		)

		if err := probe.Start(context.Background()); err != nil {
			b.Fatal(err)
		}

		probe.ReadinessHandler()(httptest.NewRecorder(), req)

		sources = append(sources, aggregate.Source{
			Name:  fmt.Sprintf("src%02d", len(sources)),
			Probe: probe,
		})
	}

	agg, err := aggregate.New(sources...)
	if err != nil {
		b.Fatal(err)
	}

	return agg
}

// BenchmarkAggregateCachedResponse measures the merge-on-read cost against
// source count: one atomic cache load per source, a fresh map of
// source-count × 3 check entries, and the worst-of status fold per read.
// Recorded baseline (2026-09-04, go1.26.7 linux/amd64): FEATURES.md
// "Performance".
func BenchmarkAggregateCachedResponse(b *testing.B) {
	for _, sourceCount := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprintf("sources=%d", sourceCount), func(b *testing.B) {
			agg := benchAggregate(b, sourceCount)

			b.ReportAllocs()

			for range b.N {
				agg.CachedResponse()
			}
		})
	}
}

// BenchmarkAggregateHandlers measures the aggregate HTTP path per handler:
// merge + UTF-8 sanitize + deterministic JSON marshal + write. Liveness is
// the constant-time floor (empty checks, no merge); readiness is the merge
// cost plus a marshaled body; startup is the unlatched 503 branch (no source
// has run its startup probe). Complements BenchmarkAggregateCachedResponse,
// which isolates the merge from the wire path.
func BenchmarkAggregateHandlers(b *testing.B) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	if err != nil {
		b.Fatal(err)
	}

	for _, sourceCount := range []int{1, 4} {
		b.Run(fmt.Sprintf("sources=%d", sourceCount), func(b *testing.B) {
			agg := benchAggregate(b, sourceCount)

			for _, handler := range []struct {
				name string
				fn   http.HandlerFunc
			}{
				{name: "liveness", fn: agg.LivenessHandler()},
				{name: "readiness", fn: agg.ReadinessHandler()},
				{name: "startup_unlatched", fn: agg.StartupHandler()},
			} {
				b.Run(handler.name, func(b *testing.B) {
					b.ReportAllocs()

					for range b.N {
						handler.fn(httptest.NewRecorder(), req)
					}
				})
			}
		})
	}
}
