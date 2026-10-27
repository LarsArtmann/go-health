package health_test

import (
	"context"
	"errors"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

var errRecorderDependency = errors.New("dependency down")

func TestNewWithRecorder_BatchesComeFromRecorder(t *testing.T) {
	t.Parallel()

	recorder := health.HealthRecorderFunc(
		func(context.Context, do.Injector) map[string]error {
			return map[string]error{"db": errRecorderDependency}
		},
	)

	probe := health.NewWithRecorder(recorder, health.WithCriticalServices("db"))

	resp := probe.Evaluate(t.Context())

	if resp.Status != health.StatusFail {
		t.Errorf("status = %q, want fail", resp.Status)
	}

	check, ok := resp.Checks["db"]
	if !ok {
		t.Fatalf("check %q missing from response", "db")
	}

	if check.Status != health.StatusFail {
		t.Errorf("check status = %q, want fail", check.Status)
	}
}

func TestNewWithRecorder_StartsWithoutInjector(t *testing.T) {
	t.Parallel()

	var calls int

	recorder := health.HealthRecorderFunc(
		func(context.Context, do.Injector) map[string]error {
			calls++

			return map[string]error{"db": nil}
		},
	)

	probe := health.NewWithRecorder(recorder, health.WithRefreshInterval(time.Hour))
	t.Cleanup(probe.Shutdown)

	if err := probe.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if _, ok := probe.CachedResponse().Checks["db"]; !ok {
		t.Error("Start did not publish the recorder's batch into the cache")
	}

	if calls == 0 {
		t.Error("recorder was never called")
	}
}

func TestNewWithRecorder_NilRecorderReportsPass(t *testing.T) {
	t.Parallel()

	probe := health.NewWithRecorder(nil)

	resp := probe.Evaluate(t.Context())

	if resp.Status != health.StatusPass {
		t.Errorf("empty recorder: status = %q, want pass", resp.Status)
	}

	if len(resp.Checks) != 0 {
		t.Errorf("empty recorder: checks = %v, want none", resp.Checks)
	}
}

func TestNewWithRecorder_HonorsRecorderOption(t *testing.T) {
	t.Parallel()

	recorder := health.HealthRecorderFunc(
		func(context.Context, do.Injector) map[string]error {
			return map[string]error{"db": nil}
		},
	)

	probe := health.NewWithRecorder(nil, health.WithHealthRecorder(recorder))

	if _, ok := probe.Evaluate(t.Context()).Checks["db"]; !ok {
		t.Error("WithHealthRecorder option was not used as the recorder")
	}
}

func TestDetailedHealthRecorderFunc_PreservesDurations(t *testing.T) {
	t.Parallel()

	recorder := health.DetailedHealthRecorderFunc(
		func(context.Context, do.Injector) map[string]health.CheckDetail {
			return map[string]health.CheckDetail{
				"db": {Duration: 3 * time.Millisecond},
			}
		},
	)

	probe := health.NewWithRecorder(recorder)

	check := probe.Evaluate(t.Context()).Checks["db"]
	if check.DurationNanos != (3 * time.Millisecond).Nanoseconds() {
		t.Errorf(
			"duration_ns = %d, want %d",
			check.DurationNanos,
			(3 * time.Millisecond).Nanoseconds(),
		)
	}
}

// TestDetailedHealthRecorderFunc_PlainViewProjectsErrors pins the
// [health.HealthRecorder] half of the adapter. Only the detailed interface
// carries durations, so a consumer holding the plain view must still get every
// check's error — a projection that dropped failures would silently turn a
// failing dependency into a passing one.
func TestDetailedHealthRecorderFunc_PlainViewProjectsErrors(t *testing.T) {
	t.Parallel()

	var plain health.HealthRecorder = health.DetailedHealthRecorderFunc(
		func(context.Context, do.Injector) map[string]health.CheckDetail {
			return map[string]health.CheckDetail{
				"db":    {Err: errRecorderDependency, Duration: time.Second},
				"cache": {Duration: time.Millisecond},
			}
		},
	)

	got := plain.RecordHealthCheckWithContext(t.Context(), do.New())

	if len(got) != 2 {
		t.Fatalf("plain batch has %d checks, want 2 (one per detail)", len(got))
	}

	if !errors.Is(got["db"], errRecorderDependency) {
		t.Errorf("db error = %v, want %v", got["db"], errRecorderDependency)
	}

	if got["cache"] != nil {
		t.Errorf("cache error = %v, want nil", got["cache"])
	}
}
