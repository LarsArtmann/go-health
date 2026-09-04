package health_test

import (
	"encoding/json/v2"
	"flag"
	"os"
	"testing"
	"time"

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
		InstanceID:     "pod-7f9c",
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

// TestResponseJSONRoundTripIdentity is the property complement to the golden
// snapshot: for any Response the wire format must be a fixed point of
// unmarshal → marshal (byte-identical), and the decoded value must equal the
// original field-for-field. A failure means consumers re-serializing probe
// payloads observe drift.
func TestResponseJSONRoundTripIdentity(t *testing.T) {
	t.Parallel()

	stamped := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

	for name, resp := range map[string]health.Response{
		"fully populated": {
			Status:         health.StatusFail,
			Version:        "1.2.3",
			InstanceID:     "pod-7f9c",
			Uptime:         "4m5s",
			ShuttingDown:   true,
			TotalLatencyMs: 42,
			Timestamp:      stamped,
			Checks: map[string]health.Check{
				"cache": {Status: health.StatusWarn, Error: "connection refused"},
				"db":    {Status: health.StatusFail, Error: `quote " backslash \ ` + "\n"},
			},
		},
		"minimal pass": {
			Status: health.StatusPass,
			Checks: map[string]health.Check{},
		},
		"zero value with nil checks": {},
		"warn degraded no timestamp": {
			Status:         health.StatusWarn,
			TotalLatencyMs: 7,
			Checks: map[string]health.Check{
				"cache": {Status: health.StatusWarn, Error: "timeout"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			first, err := json.Marshal(resp, json.Deterministic(true))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var decoded health.Response
			if err := json.Unmarshal(first, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			second, err := json.Marshal(decoded, json.Deterministic(true))
			if err != nil {
				t.Fatalf("re-marshal: %v", err)
			}

			if string(first) != string(second) {
				t.Fatalf("wire identity broken:\nfirst:  %s\nsecond: %s", first, second)
			}

			assertRoundTripEqual(t, resp, decoded)
		})
	}
}

// assertRoundTripEqual verifies the decoded value equals the original
// Response field-for-field.
func assertRoundTripEqual(t *testing.T, want, got health.Response) {
	t.Helper()

	if got.Status != want.Status ||
		got.Version != want.Version ||
		got.InstanceID != want.InstanceID ||
		got.Uptime != want.Uptime ||
		got.ShuttingDown != want.ShuttingDown ||
		got.TotalLatencyMs != want.TotalLatencyMs {
		t.Fatalf("scalar drift:\nwant: %+v\ngot:  %+v", want, got)
	}

	if !got.Timestamp.Equal(want.Timestamp) {
		t.Fatalf("timestamp drift: want %v, got %v", want.Timestamp, got.Timestamp)
	}

	if len(got.Checks) != len(want.Checks) {
		t.Fatalf("check count drift: want %d, got %d", len(want.Checks), len(got.Checks))
	}

	for key, wantCheck := range want.Checks {
		gotCheck, ok := got.Checks[key]
		if !ok {
			t.Fatalf("check %q lost in round-trip", key)
		}

		if gotCheck != wantCheck {
			t.Fatalf("check %q drift: want %+v, got %+v", key, wantCheck, gotCheck)
		}
	}
}
