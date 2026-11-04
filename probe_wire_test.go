package health_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Wire format (omitzero absence) ---.

// TestCheck_JSONOmitZero pins the issue #2 open question: under jsonv2,
// omitzero gives strict absence for both fields — zero StatusSince and zero
// duration disappear instead of marshaling as "0001-01-01T00:00:00Z" / 0.
// Duration is a plain int64 (nanoseconds) because jsonv2 cannot marshal
// time.Duration at all without per-call options (verified:
// go.dev/issue/71631 — no valid tag format exists).
func TestCheck_JSONOmitZero(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(map[string]health.Check{
		"absent": {Status: health.StatusPass},
	}, json.Deterministic(true))
	if err != nil {
		t.Fatalf("marshal zero-metadata check: %v", err)
	}

	if strings.Contains(string(payload), "since") || strings.Contains(string(payload), "duration") {
		t.Errorf("zero metadata must be omitted, got %s", payload)
	}

	stamped := time.Date(2026, 9, 15, 14, 2, 0, 0, time.UTC)

	payload, err = json.Marshal(health.Check{
		Status:        health.StatusFail,
		Error:         "down",
		StatusSince:   stamped,
		DurationNanos: (1500 * time.Microsecond).Nanoseconds(),
	}, json.Deterministic(true))
	if err != nil {
		t.Fatalf("marshal populated check: %v", err)
	}

	want := `{"status":"fail","error":"down","since":"2026-09-15T14:02:00Z","duration_ns":1500000}`
	if string(payload) != want {
		t.Errorf("populated check wire format:\nwant: %s\ngot:  %s", want, payload)
	}
}

// TestReadinessHandler_WireOmitZeroOnInjectorPath is the end-to-end
// companion to TestCheck_JSONOmitZero: through the real handler write path,
// a plain-injector probe serves checks whose probe-observed since is present
// while the unknown duration_ns stays absent — never a literal
// "duration_ns":0.
func TestReadinessHandler_WireOmitZeroOnInjectorPath(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector, health.WithRefreshInterval(0))

	w := httptest.NewRecorder()
	probe.ReadinessHandler()(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("readiness: want 200, got %d", w.Code)
	}

	body := w.Body.String()

	if !strings.Contains(body, `"since"`) {
		t.Errorf("probe-observed since must be served, got %s", body)
	}

	if strings.Contains(body, "duration_ns") {
		t.Errorf("unknown duration must be omitted, got %s", body)
	}
}

// TestResponse_JSONNilChecksMarshalAsEmptyObject pins the wire-integrity
// question from the 2026-10-02 adoption/wire review: a never-evaluated
// probe's zero Response carries a nil Checks map, and under encoding/json/v2
// nil maps marshal as {} — never null — so consumers never see
// "checks": null (verified empirically; see docs/adoption-matrix.md era
// review). If this ever changes, dashboards keying on checks must be
// null-guarded before the wire changes.
func TestResponse_JSONNilChecksMarshalAsEmptyObject(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(
		health.Response{Status: health.StatusPass},
		json.Deterministic(true),
	)
	if err != nil {
		t.Fatalf("marshal zero response: %v", err)
	}

	if strings.Contains(string(payload), "null") {
		t.Errorf("nil Checks must marshal as {}, never null, got %s", payload)
	}

	empty, err := json.Marshal(
		health.Response{Status: health.StatusPass, Checks: map[string]health.Check{}},
		json.Deterministic(true),
	)
	if err != nil {
		t.Fatalf("marshal empty response: %v", err)
	}

	if string(payload) != string(empty) {
		t.Errorf(
			"nil and empty Checks must be wire-identical:\nnil:   %s\nempty: %s",
			payload,
			empty,
		)
	}
}
