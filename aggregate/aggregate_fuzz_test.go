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
func sevRank(s health.Status) int {
	switch s {
	case health.StatusFail:
		return 0
	case health.StatusWarn:
		return 1
	default:
		return 2
	}
}

// fuzzRequest builds a plain GET request for handler smoke assertions.
func fuzzRequest(t *testing.T) *http.Request {
	t.Helper()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	return r
}

// newFuzzSource builds a started probe whose cached response has a controlled
// status: pass when the service is healthy, fail when it fails critically,
// warn when it fails non-critically.
func newFuzzSource(t *testing.T, checkName string, critical, failing bool, instanceID string) *health.Probe {
	t.Helper()

	results := func(context.Context) map[string]error {
		if failing {
			return map[string]error{checkName: errors.New("service unhealthy")}
		}

		return map[string]error{checkName: nil}
	}

	opts := []health.Option{
		health.WithRefreshInterval(0),
		health.WithInstanceID(instanceID),
	}

	if critical {
		opts = append(opts, health.WithCriticalServices(checkName))
	}

	probe := health.NewWithHealthCheck(results, opts...)

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
		name1, checkName1, name2, checkName2 string,
		crit1, fail1, crit2, fail2 bool,
		instanceID string,
	) {
		if name1 == "" || name2 == "" || name1 == name2 ||
			strings.Contains(name1, "/") || strings.Contains(name2, "/") {
			t.Skip("invalid or colliding source names are covered by unit tests")
		}

		p1 := newFuzzSource(t, checkName1, crit1, fail1, instanceID)
		p2 := newFuzzSource(t, checkName2, crit2, fail2, instanceID)

		agg, err := aggregate.New(
			aggregate.Source{Name: name1, Probe: p1},
			aggregate.Source{Name: name2, Probe: p2},
		)
		if err != nil {
			t.Fatalf("aggregate.New: %v", err)
		}

		src1 := p1.CachedResponse()
		src2 := p2.CachedResponse()
		sources := map[string]health.Response{name1: src1, name2: src2}

		wantStatus := health.StatusPass
		anyShutdown := false

		var wantLatency int64

		wantChecks := 0

		for _, src := range []health.Response{src1, src2} {
			if sevRank(src.Status) < sevRank(wantStatus) {
				wantStatus = src.Status
			}

			if src.ShuttingDown {
				anyShutdown = true
			}

			if src.TotalLatencyMs > wantLatency {
				wantLatency = src.TotalLatencyMs
			}

			wantChecks += len(src.Checks)
		}

		if anyShutdown {
			wantStatus = health.StatusFail
		}

		merged := agg.CachedResponse()

		if merged.Status != wantStatus {
			t.Fatalf("merged status: want %q, got %q", wantStatus, merged.Status)
		}

		if merged.ShuttingDown != anyShutdown {
			t.Fatalf("merged shutting_down: want %v, got %v", anyShutdown, merged.ShuttingDown)
		}

		if merged.TotalLatencyMs != wantLatency {
			t.Fatalf("merged latency: want %d, got %d", wantLatency, merged.TotalLatencyMs)
		}

		if len(merged.Checks) != wantChecks {
			t.Fatalf("merged check count: want %d, got %d", wantChecks, len(merged.Checks))
		}

		for key, check := range merged.Checks {
			prefix, rest, _ := strings.Cut(key, "/")
			src, ok := sources[prefix]
			if !ok {
				t.Fatalf("merged check %q has prefix %q which is not a source name", key, prefix)
			}

			want, ok := src.Checks[rest]
			if !ok {
				t.Fatalf("merged check %q absent from source %q", key, prefix)
			}

			if want != check {
				t.Fatalf("merged check %q value drift: want %+v, got %+v", key, want, check)
			}
		}

		if merged.Version != "" || merged.InstanceID != "" || merged.Uptime != "" || !merged.Timestamp.IsZero() {
			t.Fatalf("per-process scalars survived the merge: %+v", merged)
		}

		wantStartup := p1.StartupComplete() && p2.StartupComplete()
		if agg.StartupComplete() != wantStartup {
			t.Fatalf("startup AND: want %v, got %v", wantStartup, agg.StartupComplete())
		}

		assertCode := func(name string, handler http.HandlerFunc, want int) {
			t.Helper()

			w := httptest.NewRecorder()
			handler(w, fuzzRequest(t))

			if w.Code != want {
				t.Fatalf("%s handler: want %d, got %d", name, want, w.Code)
			}
		}

		assertCode("liveness", agg.LivenessHandler(), http.StatusOK)

		wantCode := http.StatusOK
		if wantStatus == health.StatusFail {
			wantCode = http.StatusServiceUnavailable
		}

		assertCode("readiness", agg.ReadinessHandler(), wantCode)

		wantCode = http.StatusServiceUnavailable
		if wantStartup {
			wantCode = http.StatusOK
		}

		assertCode("startup", agg.StartupHandler(), wantCode)
	})
}
