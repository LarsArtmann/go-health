package aggregate_test

import (
	"context"
	"encoding/json/v2"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	"github.com/larsartmann/go-health/aggregate"
)

// updateAggregateGolden regenerates the aggregate golden file when
// `go test ./aggregate -update` runs.
var updateAggregateGolden = flag.Bool("update", false, "rewrite aggregate testdata golden files")

// newPrimedSource builds a probe whose cache is populated exactly once: the
// throttled live path stores its evaluation into `latest`, which is what the
// aggregate's merge-on-read consumes. Without the priming request the probe
// reports an empty cached view.
func newPrimedSource(
	t *testing.T,
	results func(context.Context) map[string]error,
	instanceID string,
) *health.Probe {
	t.Helper()

	probe := health.NewWithHealthCheck(results,
		health.WithRefreshInterval(0),
		health.WithLiveThrottle(time.Hour),
		health.WithInstanceID(instanceID),
	)

	if err := probe.Start(context.Background()); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}

	rec := httptest.NewRecorder()
	probe.ReadinessHandler()(rec, fuzzRequest(t))

	return probe
}

// TestAggregateReadiness_JSONSnapshot locks the merged wire format of the
// aggregate readiness endpoint through the real handler path: namespaced
// "source/check" keys, worst-of status, and the wire-level drop of
// per-process scalars (both sources set instance_id; the merged body has no
// instance_id field). `total_latency_ms` is real wall-clock measurement
// (sub-millisecond batches legitimately measure 0), so it is normalized to 0
// before comparison; its presence in the raw body is asserted separately. Any
// change to the golden file is a wire-format change and must be called out in
// the changelog.
func TestAggregateReadiness_JSONSnapshot(t *testing.T) {
	t.Parallel()

	apiProbe := newPrimedSource(t, func(context.Context) map[string]error {
		return map[string]error{"cache": nil, "db": errUnhealthy}
	}, "pod-api-1")

	webProbe := newPrimedSource(t, func(context.Context) map[string]error {
		return map[string]error{"db": nil, "render": nil}
	}, "pod-web-9")

	agg, err := aggregate.New(
		aggregate.Source{Name: "api", Probe: apiProbe},
		aggregate.Source{Name: "web", Probe: webProbe},
	)
	if err != nil {
		t.Fatalf("aggregate.New: %v", err)
	}

	rec := httptest.NewRecorder()
	agg.ReadinessHandler()(rec, fuzzRequest(t))

	if rec.Code != http.StatusOK {
		t.Fatalf("readiness status: want 200 (warn roll-up), got %d", rec.Code)
	}

	var decoded health.Response

	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if !strings.Contains(rec.Body.String(), "total_latency_ms") {
		t.Errorf("raw body %s missing total_latency_ms", rec.Body.String())
	}

	decoded.TotalLatencyMs = 0

	payload, err := json.Marshal(decoded, json.Deterministic(true))
	if err != nil {
		t.Fatalf("marshal normalized response: %v", err)
	}

	const golden = "testdata/aggregate_readiness_response.golden"

	if *updateAggregateGolden {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatalf("create testdata dir: %v", err)
		}

		if err := os.WriteFile(golden, payload, 0o600); err != nil {
			t.Fatalf("update golden file: %v", err)
		}
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden file (run `go test ./aggregate -update` once to create): %v", err)
	}

	if string(payload) != string(want) {
		t.Errorf(
			"aggregate wire format drifted from golden file:\nwant: %s\ngot:  %s",
			want,
			payload,
		)
	}
}
