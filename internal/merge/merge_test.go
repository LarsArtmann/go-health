package merge_test

import (
	"reflect"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	merge "github.com/larsartmann/go-health/internal/merge"
)

// doc builds a source response with the given roll-up and check set.
func doc(status health.Status, checks map[string]health.Check) health.Response {
	return health.Response{
		Status:     status,
		Version:    "1.2.3",
		InstanceID: "pod-1",
		Uptime:     "4h",
		Timestamp:  time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
		Checks:     checks,
	}
}

// assertChecks compares the merged check set against the expectation,
// including per-entry equality.
func assertChecks(t *testing.T, merged map[string]health.Check, want map[string]health.Check) {
	t.Helper()

	if len(merged) != len(want) {
		t.Fatalf("checks = %v, want %v", merged, want)
	}

	for key, wantCheck := range want {
		got, ok := merged[key]
		if !ok {
			t.Errorf("missing merged check %q", key)

			continue
		}

		if got != wantCheck {
			t.Errorf("check %q = %+v, want %+v", key, got, wantCheck)
		}
	}
}

// responsesTableCases is the case table for TestResponses_Table, held in its own
// function to stay under the funlen budget.
// check builds a Check row for the case table.
func check(status health.Status, err string) health.Check {
	return health.Check{Status: status, Error: err}
}

func responsesTableCases() []struct {
	name          string
	sources       []merge.Source
	wantStatus    health.Status
	wantShutDown  bool
	wantLatencyMs int64
	wantChecks    map[string]health.Check
} {
	return []struct {
		name          string
		sources       []merge.Source
		wantStatus    health.Status
		wantShutDown  bool
		wantLatencyMs int64
		wantChecks    map[string]health.Check
	}{
		{
			name:       "zero sources fold to pass with no checks",
			sources:    nil,
			wantStatus: health.StatusPass,
			wantChecks: map[string]health.Check{},
		},
		{
			name: "single healthy source",
			sources: []merge.Source{
				{
					Name: "a",
					Response: doc(
						health.StatusPass,
						map[string]health.Check{"db": check(health.StatusPass, "")},
					),
				},
			},
			wantStatus: health.StatusPass,
			wantChecks: map[string]health.Check{"a/db": check(health.StatusPass, "")},
		},
		{
			name: "worst-of fail beats warn beats pass",
			sources: []merge.Source{
				{
					Name: "a",
					Response: doc(
						health.StatusPass,
						map[string]health.Check{"db": check(health.StatusPass, "")},
					),
				},
				{
					Name: "b",
					Response: doc(
						health.StatusWarn,
						map[string]health.Check{"cache": check(health.StatusWarn, "degraded")},
					),
				},
				{
					Name: "c",
					Response: doc(
						health.StatusFail,
						map[string]health.Check{"db": check(health.StatusFail, "down")},
					),
				},
			},
			wantStatus: health.StatusFail,
			wantChecks: map[string]health.Check{
				"a/db":    check(health.StatusPass, ""),
				"b/cache": check(health.StatusWarn, "degraded"),
				"c/db":    check(health.StatusFail, "down"),
			},
		},
		{
			name: "off check row survives verbatim and stays pass-tier",
			sources: []merge.Source{
				{Name: "a", Response: health.Response{
					Status: health.StatusPass,
					Checks: map[string]health.Check{
						"queue": check(health.StatusOff, "not configured: set queue.url"),
					},
				}},
			},
			wantStatus: health.StatusPass,
			wantChecks: map[string]health.Check{
				"a/queue": check(health.StatusOff, "not configured: set queue.url"),
			},
		},
		{
			name: "unknown roll-up status ranks pass-tier and never leaks",
			sources: []merge.Source{
				{
					Name: "a",
					Response: health.Response{
						Status: "degraded",
						Checks: map[string]health.Check{},
					},
				},
			},
			wantStatus: health.StatusPass,
			wantChecks: map[string]health.Check{},
		},
		{
			name: "shutdown overlay forces fail",
			sources: []merge.Source{
				{
					Name: "a",
					Response: doc(
						health.StatusPass,
						map[string]health.Check{"db": check(health.StatusPass, "")},
					),
				},
				{
					Name: "b",
					Response: health.Response{
						Status:         health.StatusPass,
						ShuttingDown:   true,
						TotalLatencyMs: 9,
					},
				},
			},
			wantStatus:    health.StatusFail,
			wantShutDown:  true,
			wantLatencyMs: 9,
			wantChecks: map[string]health.Check{
				"a/db": check(health.StatusPass, ""),
			},
		},
		{
			name: "fetch error yields one synthetic reachable fail",
			sources: []merge.Source{
				{Name: "ghost", FetchErr: "connection refused"},
			},
			wantStatus: health.StatusFail,
			wantChecks: map[string]health.Check{
				"ghost/reachable": check(health.StatusFail, "fetch: connection refused"),
			},
		},
		{
			name: "fetch error ignores the carried response entirely",
			sources: []merge.Source{
				{
					Name:     "ghost",
					FetchErr: "timeout",
					Response: doc(
						health.StatusPass,
						map[string]health.Check{"db": check(health.StatusPass, "")},
					),
				},
			},
			wantStatus: health.StatusFail,
			wantChecks: map[string]health.Check{
				"ghost/reachable": check(health.StatusFail, "fetch: timeout"),
			},
		},
		{
			name: "mixed: healthy, degraded, and dark sources",
			sources: []merge.Source{
				{
					Name: "a",
					Response: doc(
						health.StatusPass,
						map[string]health.Check{"db": check(health.StatusPass, "")},
					),
				},
				{Name: "ghost", FetchErr: "no such host"},
				{
					Name: "c",
					Response: doc(
						health.StatusWarn,
						map[string]health.Check{"cache": check(health.StatusWarn, "slow")},
					),
				},
			},
			wantStatus: health.StatusFail,
			wantChecks: map[string]health.Check{
				"a/db":            check(health.StatusPass, ""),
				"ghost/reachable": check(health.StatusFail, "fetch: no such host"),
				"c/cache":         check(health.StatusWarn, "slow"),
			},
		},
		{
			name: "latency is the max across successful sources only",
			sources: []merge.Source{
				{
					Name:     "a",
					Response: health.Response{Status: health.StatusPass, TotalLatencyMs: 42},
				},
				{Name: "ghost", FetchErr: "boom", Response: health.Response{TotalLatencyMs: 999}},
				{
					Name:     "c",
					Response: health.Response{Status: health.StatusPass, TotalLatencyMs: 17},
				},
			},
			wantStatus:    health.StatusFail,
			wantLatencyMs: 42,
			wantChecks: map[string]health.Check{
				"ghost/reachable": check(health.StatusFail, "fetch: boom"),
			},
		},
	}
}

// TestResponses_Table walks the shared fold: worst-of with tie stability,
// the shutdown overlay, the synthetic reachable row, namespacing with value
// carry, latency max, and scalar non-survival.
func TestResponses_Table(t *testing.T) {
	t.Parallel()

	for _, testCase := range responsesTableCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			merged := merge.Responses(testCase.sources)

			if merged.Status != testCase.wantStatus {
				t.Errorf("status = %q, want %q", merged.Status, testCase.wantStatus)
			}

			if merged.ShuttingDown != testCase.wantShutDown {
				t.Errorf("shutting_down = %v, want %v", merged.ShuttingDown, testCase.wantShutDown)
			}

			if merged.TotalLatencyMs != testCase.wantLatencyMs {
				t.Errorf("latency = %d, want %d", merged.TotalLatencyMs, testCase.wantLatencyMs)
			}

			assertChecks(t, merged.Checks, testCase.wantChecks)

			if merged.Version != "" || merged.InstanceID != "" || merged.Uptime != "" ||
				!merged.Timestamp.IsZero() {
				t.Errorf("per-process scalars survived the merge: %+v", merged)
			}
		})
	}
}

// TestResponses_MetadataSurvivesVerbatim pins that a source check's Since
// and DurationNanos carry through the merge untouched: the probe-observed
// "since when" and the executor-reported duration must stay truthful end to
// end.
func TestResponses_MetadataSurvivesVerbatim(t *testing.T) {
	t.Parallel()

	since := time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)
	want := health.Check{
		Status:        health.StatusWarn,
		Error:         "degraded",
		Since:         since,
		DurationNanos: 42,
	}

	merged := merge.Responses([]merge.Source{
		{Name: "a", Response: health.Response{
			Status: health.StatusWarn,
			Checks: map[string]health.Check{"cache": want},
		}},
	})

	got, ok := merged.Checks["a/cache"]
	if !ok {
		t.Fatalf("merged checks = %v, want a/cache", merged.Checks)
	}

	if got != want {
		t.Errorf("check metadata drift: want %+v, got %+v", want, got)
	}
}

// TestResponses_OrderIndependent pins that the fold commutes: the same
// sources in any order produce a bit-for-bit equal response. The tie rule
// (rank ties never replace the accumulator) is what makes this hold even
// across off and unknown source roll-ups.
func TestResponses_OrderIndependent(t *testing.T) {
	t.Parallel()

	response := func(status health.Status) health.Response {
		return health.Response{Status: status, TotalLatencyMs: 5}
	}

	permutations := [][]merge.Source{
		{
			{Name: "a", Response: response(health.StatusPass)},
			{Name: "b", Response: response(health.StatusWarn)},
			{Name: "c", Response: response(health.StatusOff)},
			{Name: "d", Response: response("degraded")},
		},
		{
			{Name: "d", Response: response("degraded")},
			{Name: "c", Response: response(health.StatusOff)},
			{Name: "b", Response: response(health.StatusWarn)},
			{Name: "a", Response: response(health.StatusPass)},
		},
		{
			{Name: "b", Response: response(health.StatusWarn)},
			{Name: "d", Response: response("degraded")},
			{Name: "a", Response: response(health.StatusPass)},
			{Name: "c", Response: response(health.StatusOff)},
		},
	}

	first := merge.Responses(permutations[0])

	for _, sources := range permutations[1:] {
		other := merge.Responses(sources)
		if !reflect.DeepEqual(first, other) {
			t.Errorf("fold depends on order:\nfirst: %+v\nother: %+v", first, other)
		}
	}

	if first.Status != health.StatusWarn {
		t.Errorf("status = %q, want warn (the only real severity present)", first.Status)
	}
}
