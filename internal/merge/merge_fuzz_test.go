package merge_test

import (
	"strings"
	"testing"

	health "github.com/larsartmann/go-health"
	merge "github.com/larsartmann/go-health/internal/merge"
)

// FuzzResponsesMergeInvariants is the collapsed merge-property suite that
// both merge-on-read packages previously asserted by hand. Under any input:
// no panic; the merged status is the independently recomputed fold (worst
// rank wins, ties never replace the accumulator, shutdown forces fail); a
// fetch-errored source contributes exactly one synthetic "name/reachable"
// fail row and nothing else; successful sources contribute their checks
// namespaced "name/check" with values carried verbatim; TotalLatencyMs is
// the max across successful sources; and the per-process scalars never
// survive.
//
// Source names violating the no-slash/no-empty/no-duplicate contract are
// skipped: both packages reject them at construction, and that rejection is
// unit-tested there, not here.
func FuzzResponsesMergeInvariants(f *testing.F) {
	f.Add(
		"edge-a",
		"svc",
		"edge-b",
		"svc",
		"pass",
		"fail",
		false,
		true,
		false,
		false,
		int64(3),
		int64(9),
	)
	f.Add(
		"api",
		"db",
		"worker",
		"db",
		"warn",
		"warn",
		false,
		false,
		false,
		false,
		int64(0),
		int64(4),
	)
	f.Add("s1", "x/y", "s2", "x/y", "pass", "off", true, false, true, false, int64(12), int64(1))
	f.Add("a", "", "b", "", "fail", "", true, true, false, false, int64(2), int64(2))
	f.Add(
		"golden",
		"cache",
		"golden-alt",
		"db",
		"warn",
		"pass",
		false,
		false,
		false,
		false,
		int64(42),
		int64(7),
	)
	f.Add("hub", "dark", "", "x", "pass", "pass", true, false, true, true, int64(-3), int64(0))

	f.Fuzz(func(
		t *testing.T,
		nameOne, checkNameOne, nameTwo, checkNameTwo string,
		statusOne, statusTwo string,
		shutOne, shutTwo, fetchOne, fetchTwo bool,
		latencyOne, latencyTwo int64,
	) {
		if nameOne == "" || nameTwo == "" || nameOne == nameTwo ||
			strings.Contains(nameOne, "/") || strings.Contains(nameTwo, "/") {
			t.Skip("invalid or colliding source names are covered by unit tests")
		}

		sourceOne := merge.Source{
			Name: nameOne,
			Response: health.Response{
				Status:         health.Status(statusOne),
				ShuttingDown:   shutOne,
				TotalLatencyMs: latencyOne,
			},
			FetchErr: fetchErrFor(fetchOne),
		}
		sourceTwo := merge.Source{
			Name: nameTwo,
			Response: health.Response{
				Status:         health.Status(statusTwo),
				ShuttingDown:   shutTwo,
				TotalLatencyMs: latencyTwo,
			},
			FetchErr: fetchErrFor(fetchTwo),
		}

		merged := merge.Responses([]merge.Source{sourceOne, sourceTwo})
		want := foldOracle(sourceOne, sourceTwo)

		assertMergedMatches(t, merged, want)
	})
}

// assertMergedMatches compares a merged response against the folded
// expectation: status, shutdown overlay, latency, checks, and scalar
// non-survival.
func assertMergedMatches(t *testing.T, merged health.Response, want foldExpectation) {
	t.Helper()

	if merged.Status != want.status {
		t.Errorf("status = %q, want %q", merged.Status, want.status)
	}

	if merged.ShuttingDown != want.shuttingDown {
		t.Errorf("shutting_down = %v, want %v", merged.ShuttingDown, want.shuttingDown)
	}

	if merged.TotalLatencyMs != want.latencyMs {
		t.Errorf("latency = %d, want %d", merged.TotalLatencyMs, want.latencyMs)
	}

	assertChecks(t, merged.Checks, want.checks)

	if merged.Version != "" || merged.InstanceID != "" || merged.Uptime != "" ||
		!merged.Timestamp.IsZero() {
		t.Errorf("per-process scalars survived the merge: %+v", merged)
	}
}

// fetchErrFor maps the fuzz flag to a FetchErr: empty on success, a cause
// string on failure.
func fetchErrFor(failed bool) string {
	if failed {
		return "synthetic fetch failure"
	}

	return ""
}

// foldExpectation is the independently recomputed fold result.
type foldExpectation struct {
	status       health.Status
	shuttingDown bool
	latencyMs    int64
	checks       map[string]health.Check
}

// foldOracle recomputes the documented merge from the sources themselves,
// without calling into the implementation under test.
func foldOracle(sources ...merge.Source) foldExpectation {
	want := foldExpectation{
		status: health.StatusPass,
		checks: map[string]health.Check{},
	}

	for _, src := range sources {
		if src.FetchErr != "" {
			want.checks[src.Name+"/reachable"] = health.Check{
				Status: health.StatusFail,
				Error:  "fetch: " + src.FetchErr,
			}
			want.status = worstOracle(want.status, health.StatusFail)

			continue
		}

		if src.Response.ShuttingDown {
			want.shuttingDown = true
		}

		if src.Response.TotalLatencyMs > want.latencyMs {
			want.latencyMs = src.Response.TotalLatencyMs
		}

		want.status = worstOracle(want.status, src.Response.Status)

		for name, check := range src.Response.Checks {
			want.checks[src.Name+"/"+name] = check
		}
	}

	if want.shuttingDown {
		want.status = health.StatusFail
	}

	return want
}

// worstOracle mirrors the documented rank order: lower rank wins, ties keep
// the accumulated status, so off and unknown statuses rank pass-tier.
func worstOracle(a, b health.Status) health.Status {
	if a.Rank() <= b.Rank() {
		return a
	}

	return b
}
