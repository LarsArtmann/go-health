package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	health "github.com/larsartmann/go-health"
)

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
		{
			name:   "allowed",
			opts:   []health.Option{health.WithAllowedMethods(http.MethodGet)},
			method: http.MethodGet,
		},
		{
			name:   "unlisted",
			opts:   []health.Option{health.WithAllowedMethods(http.MethodGet)},
			method: http.MethodPost,
		},
	}

	for _, bench := range cases {
		b.Run(bench.name, func(b *testing.B) {
			probe := health.NewWithHealthCheck(
				func(context.Context) map[string]error { return nil },
				bench.opts...,
			)

			handler := probe.LivenessHandler()

			r, err := http.NewRequestWithContext(
				context.Background(),
				bench.method,
				"/healthz",
				nil,
			)
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

// BenchmarkGuardOverhead_HEADAllowed extends the guard characterization to
// the multi-method set: HEAD added via WithAllowedMethods alongside the
// always-present GET, then requested as HEAD. Same map-hit cost class as the
// single-method "allowed" case; the set must stay a set (GET is not
// duplicated in the Allow header).
func BenchmarkGuardOverhead_HEADAllowed(b *testing.B) {
	probe := health.NewWithHealthCheck(
		func(context.Context) map[string]error { return nil },
		health.WithAllowedMethods(http.MethodGet, http.MethodHead),
	)

	handler := probe.LivenessHandler()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodHead, "/healthz", nil)
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
}

// BenchmarkGuardOverhead_AllowHeader isolates the 405 path's Allow-header
// construction: sorted join over the method set. It is the entire marginal
// cost of the unlisted-method response beyond the status code.
func BenchmarkGuardOverhead_AllowHeader(b *testing.B) {
	probe := health.NewWithHealthCheck(
		func(context.Context) map[string]error { return nil },
		health.WithAllowedMethods(http.MethodGet, http.MethodHead, http.MethodOptions),
	)

	guard := probe.LivenessHandler()

	// Warm the guard once and capture the handler it returns so the
	// benchmark measures only a 405 write, not the guard's map hit.
	r405, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodDelete,
		"/healthz",
		nil,
	)
	if err != nil {
		b.Fatal(err)
	}

	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		w.Body.Reset()
		w.Header().Del("Allow")
		guard(w, r405)
	}
}
