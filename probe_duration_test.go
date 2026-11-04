package health_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Check.Duration (executor-reported) ---.

// TestNewWithDetailedCheck_DurationCarriedAndStatusGraded pins the detailed
// seam: durations flow into the response untouched while classification
// stays with the probe (critical failure stays fail, message becomes Error).
func TestNewWithDetailedCheck_DurationCarriedAndStatusGraded(t *testing.T) {
	t.Parallel()

	probe := health.NewWithDetailedCheck(func(context.Context) map[string]health.CheckDetail {
		return map[string]health.CheckDetail{
			"db":     {Err: errUnhealthy, Duration: 1500 * time.Microsecond},
			"cache":  {Duration: 400 * time.Microsecond},
			"search": {Err: errUnhealthy, Duration: 250 * time.Millisecond},
		}
	},
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if resp.Status != health.StatusFail {
		t.Fatalf("roll-up: want fail (critical db failed), got %s", resp.Status)
	}

	dbCheck := resp.Checks["db"]
	if dbCheck.Status != health.StatusFail || dbCheck.Error != errUnhealthy.Error() {
		t.Errorf("db check: want fail with error, got %+v", dbCheck)
	}

	if dbCheck.DurationNanos != (1500 * time.Microsecond).Nanoseconds() {
		t.Errorf("db duration: want 1500µs, got %v", time.Duration(dbCheck.DurationNanos))
	}

	wantCache := (400 * time.Microsecond).Nanoseconds()
	if got := resp.Checks["cache"].DurationNanos; got != wantCache {
		t.Errorf("cache duration: want 400µs, got %v", time.Duration(got))
	}

	wantSearch := (250 * time.Millisecond).Nanoseconds()
	if got := resp.Checks["search"].DurationNanos; got != wantSearch {
		t.Errorf("search duration: want 250ms, got %v", time.Duration(got))
	}
}

// TestReadinessHandler_DetailedCheckWirePayload exercises the full handler
// path (evaluate → sanitize → deterministic marshal → write) with both
// metadata fields populated, locking the served bytes a dashboard sees:
// since + duration_ns present for the failing critical check, since-only for
// the healthy one.
func TestReadinessHandler_DetailedCheckWirePayload(t *testing.T) {
	t.Parallel()

	epoch := time.Date(2026, 9, 15, 14, 2, 0, 0, time.UTC)

	probe := health.NewWithDetailedCheck(func(context.Context) map[string]health.CheckDetail {
		return map[string]health.CheckDetail{
			"db":    {Err: errUnhealthy, Duration: 2 * time.Millisecond},
			"cache": {Duration: 300 * time.Microsecond},
		}
	},
		health.WithCriticalServices("db"),
		health.WithInstanceID(health.NewInstanceID("pod-1")),
		health.WithRefreshInterval(0),
		health.WithNowFunc(func() time.Time { return epoch }),
	)

	w := doRequest(t, probe.ReadinessHandler(), "/readyz")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status: want 503 (critical db failed), got %d", w.Code)
	}

	body := w.Body.String()

	for _, want := range []string{
		`"db":{"status":"fail","error":"service unhealthy","since":"2026-09-15T14:02:00Z","duration_ns":2000000}`,
		`"cache":{"status":"pass","since":"2026-09-15T14:02:00Z","duration_ns":300000}`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body %s missing %s", body, want)
		}
	}
}

// TestNew_DurationZeroOnPlainInjectorPath pins the honest-absence contract:
// samber/do's batch API cannot measure per-check timing, so the raw injector
// path reports zero (unknown), never a fabricated number.
func TestNew_DurationZeroOnPlainInjectorPath(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector, health.WithRefreshInterval(0))

	resp := probe.Evaluate(context.Background())

	if got := resp.Checks["db"].DurationNanos; got != 0 {
		t.Errorf("injector-path duration: want 0 (unknown), got %v", got)
	}
}

// mockDetailedRecorder implements both the plain and detailed recorder
// methods so the optional-interface upgrade path can be exercised.
type mockDetailedRecorder struct {
	mockRecorder

	details map[string]health.CheckDetail
}

func (m *mockDetailedRecorder) RecordDetailedHealthCheckWithContext(
	context.Context,
	do.Injector,
) map[string]health.CheckDetail {
	return m.details
}

// TestWithHealthRecorder_DetailedRecorderDurationsFlow pins the optional
// recorder upgrade: a recorder that also implements DetailedHealthRecorder
// has its per-check durations carried into the response, with no changes to
// how it is wired.
func TestWithHealthRecorder_DetailedRecorderDurationsFlow(t *testing.T) {
	t.Parallel()

	recorder := &mockDetailedRecorder{
		details: map[string]health.CheckDetail{
			"db": {Duration: 3 * time.Millisecond},
		},
	}

	probe := health.New(do.New(),
		health.WithHealthRecorder(recorder),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	wantDuration := (3 * time.Millisecond).Nanoseconds()
	if got := resp.Checks["db"].DurationNanos; got != wantDuration {
		t.Errorf("detailed recorder duration: want 3ms, got %v", got)
	}

	if got := resp.Checks["db"].Status; got != health.StatusPass {
		t.Errorf("nil Err must grade pass, got %s", got)
	}
}

// TestWithHealthRecorder_PlainRecorderDurationsStayZero pins that a plain
// recorder (no detailed method) keeps the old behavior: durations unknown.
func TestWithHealthRecorder_PlainRecorderDurationsStayZero(t *testing.T) {
	t.Parallel()

	recorder := &mockRecorder{result: map[string]error{"db": nil}}

	probe := health.New(do.New(),
		health.WithHealthRecorder(recorder),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if got := resp.Checks["db"].DurationNanos; got != 0 {
		t.Errorf("plain recorder duration: want 0 (unknown), got %v", got)
	}
}
