package health_test

import (
	"encoding/json/v2"
	"flag"
	"os"
	"testing"

	health "github.com/larsartmann/go-health"
)

// updateGolden regenerates testdata golden files when `go test -update` runs.
var updateGolden = flag.Bool("update", false, "rewrite testdata golden files")

// TestReadinessResponse_JSONSnapshot locks the JSON wire format of a fully
// populated readiness response: field names, omitempty behavior, alphabetical
// key order (json.Deterministic), and value shapes. Any change to this golden
// file is a wire-format change and must be called out in the changelog.
func TestReadinessResponse_JSONSnapshot(t *testing.T) {
	t.Parallel()

	resp := health.Response{
		Status:         health.StatusWarn,
		Version:        "1.2.3",
		Uptime:         "4m5s",
		ShuttingDown:   true,
		TotalLatencyMs: 42,
		Checks: map[string]health.Check{
			"cache": {Status: health.StatusWarn, Error: "connection refused"},
			"db":    {Status: health.StatusPass},
		},
	}

	payload, err := json.Marshal(resp, json.Deterministic(true))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	const golden = "testdata/readiness_response.golden"

	if *updateGolden {
		if err := os.WriteFile(golden, payload, 0o600); err != nil {
			t.Fatalf("update golden file: %v", err)
		}
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden file (run `go test ./... -update` once to create): %v", err)
	}

	if string(payload) != string(want) {
		t.Errorf("wire format drifted from golden file:\nwant: %s\ngot:  %s", want, payload)
	}
}

// TestReadinessResponse_JSONOmitEmpty pins which fields disappear when empty.
// Note: under encoding/json/v2, scalar `omitempty` is NOT honored — bool and
// int fields are always emitted (v2 reserves omission for `omitzero`). The
// tags on Response are historical; the wire format below is the truth.
func TestReadinessResponse_JSONOmitEmpty(t *testing.T) {
	t.Parallel()

	resp := health.Response{
		Status: health.StatusPass,
		Checks: map[string]health.Check{},
	}

	payload, err := json.Marshal(resp, json.Deterministic(true))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	want := `{"status":"pass","shutting_down":false,"total_latency_ms":0,"checks":{}}`
	if string(payload) != want {
		t.Errorf("empty response shape: want %s, got %s", want, payload)
	}
}
