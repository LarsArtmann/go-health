package health_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// prometheusWriter renders a [health.Response] in the Prometheus text
// exposition format (version 0.0.4) using only the standard library.
//
// It is the reference implementation for the composition pattern described in
// docs/prometheus-exposition-design.md: go-health ships the raw evaluation
// stream ([health.WithEvaluationHook], [health.Probe.Evaluate]); the consumer
// owns the wire format. Metric names follow the "health_" prefix convention;
// per-service health is a gauge whose labels identify the service.
func prometheusWriter(w *strings.Builder, resp health.Response) {
	instance := resp.InstanceID

	fmt.Fprintf(w, "# HELP health_up Whether this instance reports itself healthy.\n")
	fmt.Fprintf(w, "# TYPE health_up gauge\n")
	fmt.Fprintf(w, "health_up{instance=%q} %d\n", instance, statusGauge(resp.Status))

	fmt.Fprintf(w, "# HELP health_check Whether the named service passes its health check.\n")
	fmt.Fprintf(w, "# TYPE health_check gauge\n")

	names := make([]string, 0, len(resp.Checks))
	for name := range resp.Checks {
		names = append(names, name)
	}

	for _, name := range names {
		check := resp.Checks[name]
		fmt.Fprintf(
			w,
			"health_check{instance=%q,service=%q} %d\n",
			instance,
			name,
			statusGauge(check.Status),
		)
	}
}

// statusGauge maps the three-state Status onto Prometheus conventions:
// 1 = pass, 0 = fail. Warn degrades to 1 (up) — the alerting distinction is
// carried by health_check per service, not by the roll-up.
func statusGauge(s health.Status) int {
	if s == health.StatusPass || s == health.StatusWarn {
		return 1
	}

	return 0
}

// ExampleWithEvaluationHook demonstrates the metrics-integration seam: a hook
// observes every evaluation and feeds a Prometheus-format writer, served from
// a dedicated /metrics endpoint. The probe stays dependency-free.
func ExampleWithEvaluationHook_metrics() {
	injector := do.New()

	do.ProvideNamed(injector, "database", func(_ do.Injector) (*exampleDB, error) {
		return &exampleDB{}, nil
	})
	_ = do.MustInvokeNamed[*exampleDB](injector, "database")

	var latest health.Response

	probe := health.New(injector,
		health.WithInstanceID(os.Getenv("POD_NAME")),
		health.WithCriticalServices("database"),
		health.WithEvaluationHook(func(resp health.Response) { latest = resp }),
	)

	_ = probe.Evaluate(context.Background())

	var buf strings.Builder

	prometheusWriter(&buf, latest)

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		var b strings.Builder

		prometheusWriter(&b, latest)
		fmt.Fprint(w, b.String())
	})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	fmt.Println(rec.Code)
	fmt.Println(strings.Count(rec.Body.String(), "# TYPE"))

	// Output:
	// 200
	// 2
}
