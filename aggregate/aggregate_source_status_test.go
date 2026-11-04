package aggregate_test

import (
	"testing"

	health "github.com/larsartmann/go-health"
	aggregate "github.com/larsartmann/go-health/aggregate"
)

// TestSourceStatuses_WorstOfMatchesMergedRollup is the property pin for the
// per-source accessor: across every combination of source states (pass, warn,
// fail over three sources), SourceStatuses reports exactly one entry per
// source name with that source's own roll-up, and the merged
// CachedResponse status equals the worst of the per-source values.
// See docs/aggregate-per-source-visibility-design.md.
// sourceStateCombination is one cell of the worst-of property sweep: the
// configured state of a single source and the roll-up it must report.
type sourceStateCombination struct {
	name      string
	critical  bool
	unhealthy bool
	want      health.Status
}

// TestSourceStatuses_WorstOfMatchesMergedRollup is the property pin for the
// per-source accessor: across every combination of source states (pass, warn,
// fail over three sources), SourceStatuses reports exactly one entry per
// source name with that source's own roll-up, and the merged
// CachedResponse status equals the worst of the per-source values.
// See docs/aggregate-per-source-visibility-design.md.
func TestSourceStatuses_WorstOfMatchesMergedRollup(t *testing.T) {
	t.Parallel()

	states := []sourceStateCombination{
		{name: "pass", want: health.StatusPass},
		{name: "warn", unhealthy: true, want: health.StatusWarn},
		{name: "fail", critical: true, unhealthy: true, want: health.StatusFail},
	}

	for _, stateOne := range states {
		for _, stateTwo := range states {
			for _, stateThree := range states {
				t.Run(stateOne.name+"/"+stateTwo.name+"/"+stateThree.name, func(t *testing.T) {
					t.Parallel()

					assertSourceStatusesCombination(t, stateOne, stateTwo, stateThree)
				})
			}
		}
	}
}

func assertSourceStatusesCombination(
	t *testing.T,
	stateOne, stateTwo, stateThree sourceStateCombination,
) {
	t.Helper()

	agg, err := aggregate.New(
		aggregate.Source{
			Name:  "one",
			Probe: newStartedProbe(t, stateOne.critical, stateOne.unhealthy),
		},
		aggregate.Source{
			Name:  "two",
			Probe: newStartedProbe(t, stateTwo.critical, stateTwo.unhealthy),
		},
		aggregate.Source{
			Name:  "three",
			Probe: newStartedProbe(t, stateThree.critical, stateThree.unhealthy),
		},
	)
	if err != nil {
		t.Fatalf("aggregate.New: %v", err)
	}

	got := agg.SourceStatuses()
	if len(got) != 3 {
		t.Fatalf(
			"SourceStatuses entries = %d, want one per source: %v",
			len(got),
			got,
		)
	}

	for _, src := range []struct {
		name string
		want health.Status
	}{
		{name: "one", want: stateOne.want},
		{name: "two", want: stateTwo.want},
		{name: "three", want: stateThree.want},
	} {
		if got[src.name] != src.want {
			t.Errorf(
				"SourceStatuses[%q] = %v, want %v",
				src.name,
				got[src.name],
				src.want,
			)
		}
	}

	wantMerged := worstOfStatuses(stateOne.want, stateTwo.want, stateThree.want)
	if merged := agg.CachedResponse().Status; merged != wantMerged {
		t.Errorf(
			"merged status = %v, want worst of per-source statuses (%v)",
			merged,
			wantMerged,
		)
	}
}

// worstOfStatuses returns the most severe of three statuses by Rank.
func worstOfStatuses(a, b, c health.Status) health.Status {
	worst := a

	for _, candidate := range [2]health.Status{b, c} {
		if candidate.Rank() < worst.Rank() {
			worst = candidate
		}
	}

	return worst
}

// TestSourceStatuses_ShuttingDownSourceReportsFail pins the per-source
// shutdown overlay: a source that is shutting down contributes fail to its
// own entry, matching what the merged view does for the aggregate overall.
func TestSourceStatuses_ShuttingDownSourceReportsFail(t *testing.T) {
	t.Parallel()

	shuttingDown := newStartedProbe(t, false, false)
	healthy := newStartedProbe(t, false, false)

	agg, err := aggregate.New(
		aggregate.Source{Name: "draining", Probe: shuttingDown},
		aggregate.Source{Name: "steady", Probe: healthy},
	)
	if err != nil {
		t.Fatalf("aggregate.New: %v", err)
	}

	shuttingDown.Shutdown()

	got := agg.SourceStatuses()
	if got["draining"] != health.StatusFail {
		t.Errorf("SourceStatuses[draining] = %v, want fail while shutting down", got["draining"])
	}

	if got["steady"] != health.StatusPass {
		t.Errorf(
			"SourceStatuses[steady] = %v, want pass (neighbor's shutdown must not leak)",
			got["steady"],
		)
	}
}
