package health_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// Canned dependency failures for ordering assertions.
var (
	errCacheDown = errors.New("cache down")
	errQueueDown = errors.New("queue down")
)

// plainRecorder reports a canned plain batch.
type plainRecorder struct {
	batch map[string]error
}

func (r plainRecorder) RecordHealthCheckWithContext(
	context.Context,
	do.Injector,
) map[string]error {
	return r.batch
}

// detailedRecorder reports a canned detailed batch and therefore satisfies
// both recorder interfaces.
type detailedRecorder struct {
	batch map[string]health.CheckDetail
}

func (r detailedRecorder) RecordHealthCheckWithContext(
	context.Context,
	do.Injector,
) map[string]error {
	plain := make(map[string]error, len(r.batch))

	for name, detail := range r.batch {
		plain[name] = detail.Err
	}

	return plain
}

func (r detailedRecorder) RecordDetailedHealthCheckWithContext(
	context.Context,
	do.Injector,
) map[string]health.CheckDetail {
	return r.batch
}

func TestChainRecorders_MergesInOrderWithLaterWinning(t *testing.T) {
	t.Parallel()

	first := plainRecorder{batch: map[string]error{
		"database": nil,
		"cache":    errCacheDown,
	}}
	second := plainRecorder{batch: map[string]error{
		"cache": nil,
		"queue": errQueueDown,
	}}

	chain := health.ChainRecorders(first, nil, second)

	merged := chain.RecordHealthCheckWithContext(context.Background(), do.New())

	if got := merged["database"]; got != nil {
		t.Errorf("database = %v, want nil (first recorder)", got)
	}

	if got := merged["cache"]; got != nil {
		t.Errorf("cache = %v, want nil (later recorder wins)", got)
	}

	if got := merged["queue"]; got == nil {
		t.Error("queue missing: every recorder must contribute")
	}
}

func TestChainRecorders_PreservesDetailedDurations(t *testing.T) {
	t.Parallel()

	plain := plainRecorder{batch: map[string]error{"database": nil}}
	detailed := detailedRecorder{batch: map[string]health.CheckDetail{
		"cache": {Duration: 3 * time.Millisecond},
	}}

	probe := health.New(
		do.New(),
		health.WithRefreshInterval(0),
		health.WithHealthRecorder(health.ChainRecorders(plain, detailed)),
	)
	t.Cleanup(probe.Shutdown)

	resp := probe.Evaluate(context.Background())

	cache, ok := resp.Checks["cache"]
	if !ok {
		t.Fatal("cache check missing: the detailed member must contribute")
	}

	if cache.DurationNanos != int64(3*time.Millisecond) {
		t.Errorf("cache duration = %d ns, want %d (detailed durations must survive the chain)",
			cache.DurationNanos, int64(3*time.Millisecond))
	}

	if db, ok := resp.Checks["database"]; !ok || db.DurationNanos != 0 {
		t.Errorf("database = %+v, want a plain member with zero duration", db)
	}
}

func TestChainRecorders_EmptyChainReportsNoChecks(t *testing.T) {
	t.Parallel()

	probe := health.New(
		do.New(),
		health.WithRefreshInterval(0),
		health.WithHealthRecorder(health.ChainRecorders(nil, nil)),
	)
	t.Cleanup(probe.Shutdown)

	resp := probe.Evaluate(context.Background())

	if len(resp.Checks) != 0 {
		t.Errorf("checks = %v, want none for an all-nil chain", resp.Checks)
	}

	if resp.Status != health.StatusPass {
		t.Errorf("status = %q, want pass for an empty batch", resp.Status)
	}
}

func TestValidate_RejectsRecorderOnFunctionPath(t *testing.T) {
	t.Parallel()

	recorder := plainRecorder{batch: map[string]error{"database": nil}}

	tests := []struct {
		name  string
		probe *health.Probe
	}{
		{
			name: "NewChecks",
			probe: health.NewChecks(
				map[string]health.CheckFunc{"database": func(context.Context) error { return nil }},
				health.WithHealthRecorder(recorder),
			),
		},
		{
			name: "NewWithHealthCheck",
			probe: health.NewWithHealthCheck(
				func(context.Context) map[string]error { return map[string]error{"database": nil} },
				health.WithHealthRecorder(recorder),
			),
		},
		{
			name: "NewWithDetailedCheck",
			probe: health.NewWithDetailedCheck(
				func(context.Context) map[string]health.CheckDetail {
					return map[string]health.CheckDetail{"database": {}}
				},
				health.WithHealthRecorder(recorder),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := tt.probe.Validate(); !errors.Is(err, health.ErrRecorderNotApplicable) {
				t.Errorf("Validate = %v, want ErrRecorderNotApplicable", err)
			}

			if err := tt.probe.Start(
				t.Context(),
			); !errors.Is(
				err,
				health.ErrRecorderNotApplicable,
			) {
				t.Errorf("Start = %v, want ErrRecorderNotApplicable", err)
			}
		})
	}
}

// TestValidate_RecorderRejectionOutranksConfigErrors pins the precedence the
// rejection path took when it was added: a probe that is wrong in two ways at
// once reports the recorder mis-wiring first. The recorder problem is the one
// with no runtime evidence (it captures nothing at all), so naming it first is
// what saves a consumer from "fixing" a timeout and still observing nothing.
func TestValidate_RecorderRejectionOutranksConfigErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		probe *health.Probe
	}{
		{
			name: "recorder plus invalid timeout",
			probe: health.NewChecks(
				map[string]health.CheckFunc{"database": func(context.Context) error { return nil }},
				health.WithHealthRecorder(plainRecorder{batch: map[string]error{"database": nil}}),
				health.WithTimeout(0),
			),
		},
		{
			name: "recorder plus invalid refresh interval",
			probe: health.NewWithHealthCheck(
				func(context.Context) map[string]error { return map[string]error{"database": nil} },
				health.WithHealthRecorder(plainRecorder{batch: map[string]error{"database": nil}}),
				health.WithRefreshInterval(-time.Second),
			),
		},
		{
			name: "recorder plus both config errors",
			probe: health.NewWithDetailedCheck(
				func(context.Context) map[string]health.CheckDetail {
					return map[string]health.CheckDetail{"database": {}}
				},
				health.WithHealthRecorder(plainRecorder{batch: map[string]error{"database": nil}}),
				health.WithTimeout(-time.Second),
				health.WithRefreshInterval(-time.Second),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := tt.probe.Validate(); !errors.Is(err, health.ErrRecorderNotApplicable) {
				t.Errorf("Validate = %v, want ErrRecorderNotApplicable", err)
			}

			if err := tt.probe.Start(
				t.Context(),
			); !errors.Is(
				err,
				health.ErrRecorderNotApplicable,
			) {
				t.Errorf("Start = %v, want ErrRecorderNotApplicable", err)
			}
		})
	}
}

// TestNewWithRecorder_ChainReachesServedResponse wires the two injector-free
// pieces together: a chain of a plain and a detailed recorder, handed to
// NewWithRecorder, must both run and its detailed member's durations must
// reach the served response. Pins the composition the bridge replacement
// depends on (cqrs-htmx's hand-rolled RecorderChain lost durations).
func TestNewWithRecorder_ChainReachesServedResponse(t *testing.T) {
	t.Parallel()

	var plainCalls int

	chain := health.ChainRecorders(
		health.HealthRecorderFunc(
			func(context.Context, do.Injector) map[string]error {
				plainCalls++

				return map[string]error{"audit-log": nil}
			},
		),
		health.DetailedHealthRecorderFunc(
			func(context.Context, do.Injector) map[string]health.CheckDetail {
				return map[string]health.CheckDetail{
					"database": {Duration: 7 * time.Millisecond},
				}
			},
		),
	)

	probe := health.NewWithRecorder(chain, health.WithRefreshInterval(0))
	t.Cleanup(probe.Shutdown)

	if err := probe.Start(t.Context()); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	resp := probe.CachedResponse()

	if plainCalls == 0 {
		t.Error("plain chain member never ran")
	}

	for _, name := range []string{"audit-log", "database"} {
		if _, ok := resp.Checks[name]; !ok {
			t.Errorf("check %q missing from the served response: %v", name, resp.Checks)
		}
	}

	if got := resp.Checks["database"].DurationNanos; got != int64(7*time.Millisecond) {
		t.Errorf("database duration = %d ns, want %d (detailed member's duration lost)",
			got, int64(7*time.Millisecond))
	}
}

func TestValidate_AcceptsRecorderOnInjectorPath(t *testing.T) {
	t.Parallel()

	probe := health.New(
		do.New(),
		health.WithRefreshInterval(0),
		health.WithHealthRecorder(plainRecorder{batch: map[string]error{"database": nil}}),
	)
	t.Cleanup(probe.Shutdown)

	if err := probe.Start(t.Context()); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	if _, ok := probe.CachedResponse().Checks["database"]; !ok {
		t.Error("recorder batch missing from the injector-path probe")
	}
}
