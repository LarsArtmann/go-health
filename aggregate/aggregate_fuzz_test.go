package aggregate_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	health "github.com/larsartmann/go-health"
	aggregate "github.com/larsartmann/go-health/aggregate"
)

// sevRank mirrors the aggregate's worst-of merge order: lower is worse.
func sevRank(status health.Status) int {
	switch status {
	case health.StatusFail:
		return 0
	case health.StatusWarn:
		return 1
	case health.StatusPass:
		return 2
	default:
		return 2
	}
}

// fuzzRequest builds a plain GET request for handler smoke assertions.
func fuzzRequest(t *testing.T) *http.Request {
	t.Helper()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	return request
}

// fuzzSourceSpec controls one fuzz-built source's cached health view.
type fuzzSourceSpec struct {
	checkName  string
	critical   bool
	failing    bool
	instanceID string
}

// newFuzzSource builds a started probe whose cached response has a controlled
// status: pass when the service is healthy, fail when it fails critically,
// warn when it fails non-critically.
func newFuzzSource(t *testing.T, spec fuzzSourceSpec) *health.Probe {
	t.Helper()

	results := func(context.Context) map[string]error {
		if spec.failing {
			return map[string]error{spec.checkName: errUnhealthy}
		}

		return map[string]error{spec.checkName: nil}
	}

	options := []health.Option{
		health.WithRefreshInterval(0),
		health.WithInstanceID(spec.instanceID),
	}

	if spec.critical {
		options = append(options, health.WithCriticalServices(spec.checkName))
	}

	probe := health.NewWithHealthCheck(results, options...)

	if err := probe.Start(context.Background()); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}

	return probe
}

// FuzzAggregateMergeInvariants fuzzes source topology through the merge-on-read
// path. Invariants under any input: no panic; the merged status is the worst
// of the source statuses (fail when any source shuts down); every merged check
// key is namespaced "source/check" and carries the source's own check value;
// per-process scalars (version, instance_id, uptime, timestamp) never survive
// the merge; and the three handlers only emit their documented status codes.
//
// Source names containing "/" are out of contract here: the documented
// grouping axis is the part before the first "/", and a slash in a source
// name could alias another source's namespace (flagged as a follow-up, not
// exercised as an invariant).
func FuzzAggregateMergeInvariants(f *testing.F) {
	f.Add("edge-a", "svc", "edge-b", "svc", true, false, true, true, "pod-1")
	f.Add("api", "db", "worker", "db", true, true, false, true, "")
	f.Add("s1", "x/y", "s2", "x/y", false, true, true, false, "i-0abc")
	f.Add("a", "", "b", "", false, false, false, false, "replica-7")

	f.Fuzz(func(
		t *testing.T,
		nameOne, checkNameOne, nameTwo, checkNameTwo string,
		critOne, failOne, critTwo, failTwo bool,
		instanceID string,
	) {
		if nameOne == "" || nameTwo == "" || nameOne == nameTwo ||
			strings.Contains(nameOne, "/") || strings.Contains(nameTwo, "/") {
			t.Skip("invalid or colliding source names are covered by unit tests")
		}

		probeOne := newFuzzSource(t, fuzzSourceSpec{
			checkName: checkNameOne, critical: critOne, failing: failOne, instanceID: instanceID,
		})

		probeTwo := newFuzzSource(t, fuzzSourceSpec{
			checkName: checkNameTwo, critical: critTwo, failing: failTwo, instanceID: instanceID,
		})

		agg, err := aggregate.New(
			aggregate.Source{Name: nameOne, Probe: probeOne},
			aggregate.Source{Name: nameTwo, Probe: probeTwo},
		)
		if err != nil {
			t.Fatalf("aggregate.New: %v", err)
		}

		cachedViews := map[string]health.Response{
			nameOne: probeOne.CachedResponse(),
			nameTwo: probeTwo.CachedResponse(),
		}

		wantStatus, wantShutdown, wantLatency, wantChecks := foldSources(cachedViews)
		merged := agg.CachedResponse()

		if merged.Status != wantStatus {
			t.Fatalf("merged status: want %q, got %q", wantStatus, merged.Status)
		}

		if merged.ShuttingDown != wantShutdown {
			t.Fatalf("merged shutting_down: want %v, got %v", wantShutdown, merged.ShuttingDown)
		}

		if merged.TotalLatencyMs != wantLatency {
			t.Fatalf("merged latency: want %d, got %d", wantLatency, merged.TotalLatencyMs)
		}

		if len(merged.Checks) != wantChecks {
			t.Fatalf("merged check count: want %d, got %d", wantChecks, len(merged.Checks))
		}

		assertChecksNamespaced(t, merged.Checks, cachedViews)

		if merged.Version != "" || merged.InstanceID != "" || merged.Uptime != "" || !merged.Timestamp.IsZero() {
			t.Fatalf("per-process scalars survived the merge: %+v", merged)
		}

		wantStartup := probeOne.StartupComplete() && probeTwo.StartupComplete()
		if agg.StartupComplete() != wantStartup {
			t.Fatalf("startup AND: want %v, got %v", wantStartup, agg.StartupComplete())
		}

		assertAggregateHandlers(t, agg, wantStatus, wantStartup)
	})
}

// foldSources computes the merge expectations from the sources' cached views.
func foldSources(sources map[string]health.Response) (status health.Status, shutdown bool, latency int64, checks int) {
	status = health.StatusPass

	for _, cached := range sources {
		if sevRank(cached.Status) < sevRank(status) {
			status = cached.Status
		}

		if cached.ShuttingDown {
			shutdown = true
		}

		if cached.TotalLatencyMs > latency {
			latency = cached.TotalLatencyMs
		}

		checks += len(cached.Checks)
	}

	if shutdown {
		status = health.StatusFail
	}

	return status, shutdown, latency, checks
}

// assertChecksNamespaced verifies every merged check key is prefixed by a
// source name and carries exactly that source's check value.
func assertChecksNamespaced(t *testing.T, merged map[string]health.Check, sources map[string]health.Response) {
	t.Helper()

	for key, check := range merged {
		prefix, rest, _ := strings.Cut(key, "/")

		source, ok := sources[prefix]
		if !ok {
			t.Fatalf("merged check %q has prefix %q which is not a source name", key, prefix)
		}

		want, ok := source.Checks[rest]
		if !ok {
			t.Fatalf("merged check %q absent from source %q", key, prefix)
		}

		if want != check {
			t.Fatalf("merged check %q value drift: want %+v, got %+v", key, want, check)
		}
	}
}

// assertAggregateHandlers verifies the three handlers answer with their
// documented status codes for the merged state.
func assertAggregateHandlers(t *testing.T, agg *aggregate.Aggregate, wantStatus health.Status, wantStartup bool) {
	t.Helper()

	cases := []struct {
		name    string
		handler http.HandlerFunc
		want    int
	}{
		{name: "liveness", handler: agg.LivenessHandler(), want: http.StatusOK},
	}

	if wantStatus == health.StatusFail {
		cases = append(cases, struct {
			name    string
			handler http.HandlerFunc
			want    int
		}{name: "readiness", handler: agg.ReadinessHandler(), want: http.StatusServiceUnavailable})
	} else {
		cases = append(cases, struct {
			name    string
			handler http.HandlerFunc
			want    int
		}{name: "readiness", handler: agg.ReadinessHandler(), want: http.StatusOK})
	}

	if wantStartup {
		cases = append(cases, struct {
			name    string
			handler http.HandlerFunc
			want    int
		}{name: "startup", handler: agg.StartupHandler(), want: http.StatusOK})
	} else {
		cases = append(cases, struct {
			name    string
			handler http.HandlerFunc
			want    int
		}{name: "startup", handler: agg.StartupHandler(), want: http.StatusServiceUnavailable})
	}

	for _, testCase := range cases {
		recorder := httptest.NewRecorder()
		testCase.handler(recorder, fuzzRequest(t))

		if recorder.Code != testCase.want {
			t.Errorf("%s handler: want %d, got %d", testCase.name, testCase.want, recorder.Code)
		}
	}
}
