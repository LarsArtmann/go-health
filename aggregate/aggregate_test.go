package aggregate_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	health "github.com/larsartmann/go-health"
	aggregate "github.com/larsartmann/go-health/aggregate"
	"github.com/samber/do/v2"
)

// --- Test service types ---.

type healthyService struct{}

var _ do.HealthcheckerWithContext = (*healthyService)(nil)

func (healthyService) HealthCheck(_ context.Context) error { return nil }

var errUnhealthy = errors.New("service unhealthy")

type unhealthyService struct{ reason string }

var _ do.HealthcheckerWithContext = (*unhealthyService)(nil)

func (u *unhealthyService) HealthCheck(_ context.Context) error {
	return fmt.Errorf("%w: %s", errUnhealthy, u.reason)
}

// --- Test helpers ---.

func newStartedProbe(t *testing.T, critical, unhealthy bool) *health.Probe {
	t.Helper()

	injector := do.New()

	if unhealthy {
		do.ProvideNamed(injector, "dependency", func(_ do.Injector) (*unhealthyService, error) {
			return &unhealthyService{reason: "connection refused"}, nil
		})

		// samber/do only health-checks instantiated services: without the
		// eager invoke, the dependency is invisible and the probe stays pass.
		do.MustInvokeNamed[*unhealthyService](injector, "dependency")
	} else {
		do.ProvideNamed(injector, "dependency", func(_ do.Injector) (*healthyService, error) {
			return &healthyService{}, nil
		})
		do.MustInvokeNamed[*healthyService](injector, "dependency")
	}

	opts := []health.Option{health.WithRefreshInterval(0)}
	if critical {
		opts = append(opts, health.WithCriticalServices("dependency"))
	}

	probe := health.New(injector, opts...)

	ctx, cancel := context.WithCancel(context.Background())

	t.Cleanup(func() {
		cancel()
		injector.Shutdown()
	})

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}

	return probe
}

func mustAggregate(t *testing.T, sources ...aggregate.Source) *aggregate.Aggregate {
	t.Helper()

	agg, err := aggregate.New(sources...)
	if err != nil {
		t.Fatalf("aggregate.New: %v", err)
	}

	return agg
}

// decodeResponse unmarshals a handler body into a map for status-code and
// shape assertions without depending on struct equality.
func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	return body
}

// --- Construction ---.

func TestNew_RejectsInvalidSources(t *testing.T) {
	t.Parallel()

	healthy := newStartedProbe(t, false, false)

	tests := []struct {
		name    string
		sources []aggregate.Source
		wantErr error
	}{
		{
			name:    "no sources",
			sources: nil,
			wantErr: aggregate.ErrNoSources,
		},
		{
			name:    "nil probe",
			sources: []aggregate.Source{{Name: "api", Probe: nil}},
			wantErr: aggregate.ErrInvalidSource,
		},
		{
			name:    "empty name",
			sources: []aggregate.Source{{Name: "", Probe: healthy}},
			wantErr: aggregate.ErrInvalidSource,
		},
		{
			name: "duplicate name",
			sources: []aggregate.Source{
				{Name: "api", Probe: healthy},
				{Name: "api", Probe: healthy},
			},
			wantErr: aggregate.ErrInvalidSource,
		},
		{
			name:    "name containing slash",
			sources: []aggregate.Source{{Name: "team/api", Probe: healthy}},
			wantErr: aggregate.ErrInvalidSource,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := aggregate.New(tt.sources...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("aggregate.New error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestNew_JoinsAllInvalidSources pins the errors.Join behavior: construction
// reports every invalid source at once (one line per problem) instead of
// stopping at the first, while errors.Is against the sentinel keeps matching
// through the joined tree. See docs/errors-join-design.md.
func TestNew_JoinsAllInvalidSources(t *testing.T) {
	t.Parallel()

	healthy := newStartedProbe(t, false, false)

	_, err := aggregate.New(
		aggregate.Source{Name: "nil-probe", Probe: nil},
		aggregate.Source{Name: "", Probe: healthy},
		aggregate.Source{Name: "slash/name", Probe: healthy},
		aggregate.Source{Name: "api", Probe: healthy},
		aggregate.Source{Name: "api", Probe: healthy},
		aggregate.Source{Name: "ok", Probe: healthy},
	)
	if err == nil {
		t.Fatal("aggregate.New unexpectedly succeeded")
	}

	if !errors.Is(err, aggregate.ErrInvalidSource) {
		t.Fatalf(
			"joined error does not match %v via errors.Is: %v",
			aggregate.ErrInvalidSource,
			err,
		)
	}

	if errors.Is(err, aggregate.ErrNoSources) {
		t.Fatalf("joined error unexpectedly matches %v: %v", aggregate.ErrNoSources, err)
	}

	for _, want := range []string{
		`source "nil-probe" has a nil Probe`,
		"source name must not be empty",
		`source name "slash/name" must not contain '/'`,
		`duplicate source name "api"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("joined error message missing %q;\ngot:\n%s", want, err.Error())
		}
	}

	if got := strings.Count(err.Error(), "\n"); got != 3 {
		t.Errorf("joined error line count = %d, want 4 lines (3 newlines):\n%s", got, err.Error())
	}
}

// TestNew_SlashNameContract pins both halves of the source-name contract:
// source names with "/" are rejected because the name becomes the check-key
// prefix ("name/check") and a slash would blur the grouping axis and could
// alias another source's namespace; check names with "/" are accepted and
// everything before the first slash stays the source name.
// See docs/aggregate-source-name-design.md.
func TestNew_SlashNameContract(t *testing.T) {
	t.Parallel()

	t.Run("rejects slash in source name", func(t *testing.T) {
		t.Parallel()

		_, err := aggregate.New(aggregate.Source{
			Name:  "team/api",
			Probe: newStartedProbe(t, false, false),
		})
		if !errors.Is(err, aggregate.ErrInvalidSource) {
			t.Fatalf("aggregate.New error = %v, want %v", err, aggregate.ErrInvalidSource)
		}
	})

	t.Run("accepts slash in check name, grouping axis intact", func(t *testing.T) {
		t.Parallel()

		results := func(context.Context) map[string]error {
			return map[string]error{"pool/primary": nil}
		}

		probe := health.NewWithHealthCheck(results, health.WithRefreshInterval(0))

		if err := probe.Start(context.Background()); err != nil {
			t.Fatalf("probe.Start: %v", err)
		}

		agg, err := aggregate.New(aggregate.Source{Name: "api", Probe: probe})
		if err != nil {
			t.Fatalf("aggregate.New: %v", err)
		}

		merged := agg.CachedResponse()

		if len(merged.Checks) != 1 {
			t.Fatalf("merged check count = %d, want 1", len(merged.Checks))
		}

		if _, ok := merged.Checks["api/pool/primary"]; !ok {
			t.Fatalf("merged checks = %v, want key \"api/pool/primary\"", merged.Checks)
		}
	})
}

func TestNew_RefreshIntervalIsSlowestSource(t *testing.T) {
	t.Parallel()

	fast := health.New(do.New(), health.WithRefreshInterval(1*1e9)) // 1s
	slow := health.New(do.New(), health.WithRefreshInterval(5*1e9)) // 5s
	live := health.New(do.New(), health.WithRefreshInterval(0))

	agg := mustAggregate(t,
		aggregate.Source{Name: "fast", Probe: fast},
		aggregate.Source{Name: "slow", Probe: slow},
		aggregate.Source{Name: "live", Probe: live},
	)

	if got := agg.RefreshInterval(); got != 5*1e9 {
		t.Fatalf("RefreshInterval = %s, want 5s (slowest source)", got)
	}
}
