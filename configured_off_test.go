package health_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	health "github.com/larsartmann/go-health"
)

// offDetail is the canonical enable-recipe detail used across the off tests.
const offDetail = "not configured: set database.url / CV_DATABASE_URL to enable analytics"

// errQueueRefused is the standing failure used by the roll-up ordering tests.
var errQueueRefused = errors.New("connection refused")

// newOffProbe builds a live-mode probe (no Start ceremony) whose checks are
// described by name→outcome: nil = healthy, error = that error.
func newOffProbe(t *testing.T, critical []string, checks map[string]error) *health.Probe {
	t.Helper()

	executors := make(map[string]health.CheckFunc, len(checks))
	for name, err := range checks {
		executors[name] = func(context.Context) error { return err }
	}

	opts := make([]health.Option, 0, 2)
	opts = append(opts, health.WithRefreshInterval(0))

	if len(critical) > 0 {
		opts = append(opts, health.WithCriticalServices(critical...))
	}

	return health.NewChecks(executors, opts...)
}

// readinessOf runs the probe's readiness handler once and returns the HTTP
// status code plus the decoded body.
func readinessOf(t *testing.T, probe *health.Probe) (int, health.Response) {
	t.Helper()

	rec := httptest.NewRecorder()
	probe.ReadinessHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	var resp health.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode readiness body %q: %v", rec.Body.String(), err)
	}

	return rec.Code, resp
}

// startupCode runs the probe's startup handler once and returns the status
// code.
func startupCode(t *testing.T, probe *health.Probe) int {
	t.Helper()

	rec := httptest.NewRecorder()
	probe.StartupHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/startupz", nil))

	return rec.Code
}

// TestOff_OffCheckIsVisibleNeverVerdict pins the core contract: an off check
// renders with StatusOff and its detail in Error, the readiness response
// stays HTTP 200, and the roll-up is pass — off is visibility, never a
// verdict.
func TestOff_OffCheckIsVisibleNeverVerdict(t *testing.T) {
	t.Parallel()

	probe := newOffProbe(t, nil, map[string]error{
		"db":    health.Off(offDetail),
		"cache": nil,
	})

	code, resp := readinessOf(t, probe)
	if code != http.StatusOK {
		t.Fatalf("readiness with an off check: want 200, got %d", code)
	}

	if want := health.StatusPass; resp.Status != want {
		t.Errorf("roll-up with only off+pass checks: want %q, got %q", want, resp.Status)
	}

	dbCheck, ok := resp.Checks["db"]
	if !ok {
		t.Fatalf("want db check, got %v", resp.Checks)
	}

	if dbCheck.Status != health.StatusOff {
		t.Errorf("db status: want %q, got %q", health.StatusOff, dbCheck.Status)
	}

	if dbCheck.Error != offDetail {
		t.Errorf("db error: want detail verbatim %q, got %q", offDetail, dbCheck.Error)
	}

	if dbCheck.Since.IsZero() {
		t.Error("off check must carry probe-observed Since like any other status")
	}
}

// TestOff_WarnStillBeatsOff pins the merge order at the roll-up: off is
// pass-tier, so a warn on another check keeps the overall at warn.
func TestOff_WarnStillBeatsOff(t *testing.T) {
	t.Parallel()

	probe := newOffProbe(t, nil, map[string]error{
		"db":    health.Off(offDetail),
		"queue": errQueueRefused,
	})

	_, resp := readinessOf(t, probe)
	if want := health.StatusWarn; resp.Status != want {
		t.Errorf("roll-up off+warn: want %q, got %q", want, resp.Status)
	}

	if resp.Checks["db"].Status != health.StatusOff {
		t.Errorf("db must stay off, got %q", resp.Checks["db"].Status)
	}
}

// TestOff_CriticalOffNeverFails pins that off beats criticality: a critical
// check returning Off must not fail readiness and must not block the
// startup latch — a deliberate absence has nothing to wait for. (A critical
// check reporting off is a configuration smell; surface it via
// WithEvaluationHook, not the verdict.)
func TestOff_CriticalOffNeverFails(t *testing.T) {
	t.Parallel()

	probe := newOffProbe(t, []string{"db"}, map[string]error{
		"db": health.Off(offDetail),
	})

	code, resp := readinessOf(t, probe)
	if code != http.StatusOK {
		t.Fatalf("readiness with an off critical check: want 200, got %d", code)
	}

	if want := health.StatusPass; resp.Status != want {
		t.Errorf("roll-up: want %q, got %q", want, resp.Status)
	}

	if got := startupCode(t, probe); got != http.StatusOK {
		t.Errorf("startup with off critical check: want 200, got %d", got)
	}
}

// TestOff_WrappedSentinelStillOff pins that wrapping preserves the grade:
// errors.As traversal must find the sentinel through %w wrappers.
func TestOff_WrappedSentinelStillOff(t *testing.T) {
	t.Parallel()

	probe := newOffProbe(t, nil, map[string]error{
		"db": fmt.Errorf("resolve config: %w", health.Off(offDetail)),
	})

	_, resp := readinessOf(t, probe)

	dbCheck, ok := resp.Checks["db"]
	if !ok {
		t.Fatalf("want db check, got %v", resp.Checks)
	}

	if dbCheck.Status != health.StatusOff {
		t.Errorf("wrapped sentinel must still grade off, got %q", dbCheck.Status)
	}

	if !strings.Contains(dbCheck.Error, offDetail) {
		t.Errorf("wrapped detail must survive, got %q", dbCheck.Error)
	}
}

// TestOff_NewWithHealthCheckPath pins the same grading on the batch-function
// constructor and the samber/do conformance view: an all-off instance is
// healthy to do (HealthCheck returns nil).
func TestOff_NewWithHealthCheckPath(t *testing.T) {
	t.Parallel()

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		return map[string]error{"analytics": health.Off(offDetail)}
	}, health.WithRefreshInterval(0))

	code, resp := readinessOf(t, probe)
	if code != http.StatusOK || resp.Status != health.StatusPass {
		t.Fatalf("batch-function off: want 200/pass, got %d/%q", code, resp.Status)
	}

	if err := probe.HealthCheck(context.Background()); err != nil {
		t.Errorf("do conformance: an all-off instance must be healthy, got %v", err)
	}
}
