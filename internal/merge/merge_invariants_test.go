package merge_test

import (
	"reflect"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	merge "github.com/larsartmann/go-health/internal/merge"
)

// TestResponses_MetadataSurvivesVerbatim pins that a source check's StatusSince
// and DurationNanos carry through the merge untouched: the probe-observed
// "since when" and the executor-reported duration must stay truthful end to
// end.
func TestResponses_MetadataSurvivesVerbatim(t *testing.T) {
	t.Parallel()

	since := time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)
	want := health.Check{
		Status:        health.StatusWarn,
		Error:         "degraded",
		StatusSince:   since,
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
