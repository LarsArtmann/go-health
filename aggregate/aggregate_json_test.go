package aggregate_test

import (
	"context"
	"encoding/json/v2"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	health "github.com/larsartmann/go-health"
	aggregate "github.com/larsartmann/go-health/aggregate"
)

// updateAggregateGolden regenerates the aggregate golden file when
// `go test ./aggregate -update` runs.
var updateAggregateGolden = flag.Bool("update", false, "rewrite aggregate testdata golden files")

// TestAggregateReadiness_JSONSnapshot locks the merged wire format of the
// aggregate readiness endpoint through the real handler path: namespaced
// "source/check" keys, worst-of status, and the wire-level drop of
// per-process scalars (both sources set instance_id; the merged body has no
// instance_id field). `total_latency_ms` is real wall-clock measurement, so
// it is normalized to 0 before comparison; its presence and integer shape in
// the raw body are asserted separately. Any change to the golden file is a
// wire-format change and must be called out in the changelog.
func TestAggregateReadiness_JSONSnapshot(t *testing.T) {
	t.Parallel()

	apiProbe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"cache": nil, "db": errUnhealthy}
	},
		health.WithRefreshInterval(0),
		health.WithInstanceID("pod-api-1"),
	)

	webProbe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"db": nil, "render": nil}
	},
		health.WithRefreshInterval(0),
		health.WithInstanceID("pod-web-9"),
	)

	for _, probe := range []*health.Probe{apiProbe, webProbe} {
		if err := probe.Start(context.Background()); err != nil {
			t.Fatalf("probe.Start: %v", err)
		}
	}

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

	if decoded.TotalLatencyMs <= 0 {
		t.Errorf("raw body total_latency_ms = %d, want a positive measurement", decoded.TotalLatencyMs)
	}

	decoded.TotalLatencyMs = 0

	payload, err := json.Marshal(decoded, json.Deterministic(true))
	if err != nil {
		t.Fatalf("marshal normalized response: %v", err)
	}

	const golden = "testdata/aggregate_readiness_response.golden"

	if *updateAggregateGolden {
		if err := os.WriteFile(golden, payload, 0o600); err != nil {
			t.Fatalf("update golden file: %v", err)
		}
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden file (run `go test ./aggregate -update` once to create): %v", err)
	}

	if string(payload) != string(want) {
		t.Errorf("aggregate wire format drifted from golden file:\nwant: %s\ngot:  %s", want, payload)
	}
}
