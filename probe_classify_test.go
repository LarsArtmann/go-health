package health_test

import (
	"context"
	"testing"

	do "github.com/samber/do/v2"

	health "github.com/larsartmann/go-health"
)

// state is one service's health state in the classify matrix.
type state struct {
	healthy bool
}

// TestClassify_ExhaustiveMatrix walks all combinations of three services in
// {healthy, failing} states against all 2^3 critical sets and checks the
// roll-up against the spec, computed independently of classify:
//
//   - fail when any critical service is failing
//   - warn when no critical service is failing but at least one
//     non-critical service is failing
//   - pass when every service is healthy
//
// Per-check grading is asserted alongside the roll-up: a failing critical
// check is fail, a failing non-critical check is warn, a passing check is
// pass.
func TestClassify_ExhaustiveMatrix(t *testing.T) {
	t.Parallel()

	serviceNames := []string{"a", "b", "c"}

	// allStates enumerates every assignment of healthy/failing to the three
	// services: 2^3 = 8 rows.
	allStates := make([]map[string]state, 0, 8)
	for mask := 0; mask < 8; mask++ {
		states := make(map[string]state, len(serviceNames))
		for i, name := range serviceNames {
			states[name] = state{healthy: mask&(1<<i) != 0}
		}

		allStates = append(allStates, states)
	}

	// allCriticals enumerates every critical subset: 2^3 = 8 rows.
	allCriticals := make([]map[string]bool, 0, 8)
	for mask := 0; mask < 8; mask++ {
		critical := make(map[string]bool, len(serviceNames))
		for i, name := range serviceNames {
			critical[name] = mask&(1<<i) != 0
		}

		allCriticals = append(allCriticals, critical)
	}

	for _, states := range allStates {
		for _, critical := range allCriticals {
			states, critical := states, critical

			label := ""
			for _, name := range serviceNames {
				mark := "ok"
				if !states[name].healthy {
					mark = "FAIL"
				}

				label += name + "=" + mark
				if critical[name] {
					label += "(crit)"
				}

				label += " "
			}

			t.Run(label, func(t *testing.T) {
				t.Parallel()

				injector := do.New()
				t.Cleanup(func() { injector.Shutdown() })

				var wantCritical []string

				for _, name := range serviceNames {
					if states[name].healthy {
						provideHealthy(injector, name)
						invoke[*healthyService](t, injector, name)
					} else {
						provideUnhealthy(injector, name, "matrix failure")
						invoke[*unhealthyService](t, injector, name)
					}

					if critical[name] {
						wantCritical = append(wantCritical, name)
					}
				}

				probe := health.New(injector, health.WithCriticalServices(wantCritical...))

				resp := probe.Evaluate(context.Background())

				want := expectedStatus(states, critical)
				if resp.Status != want {
					t.Errorf("roll-up: want %s, got %s", want, resp.Status)
				}

				if len(resp.Checks) != len(serviceNames) {
					t.Fatalf("checks: want %d entries, got %d", len(serviceNames), len(resp.Checks))
				}

				for _, name := range serviceNames {
					check, ok := resp.Checks[name]
					if !ok {
						t.Fatalf("check %q missing from response", name)
					}

					wantCheck := health.StatusPass
					if !states[name].healthy {
						if critical[name] {
							wantCheck = health.StatusFail
						} else {
							wantCheck = health.StatusWarn
						}
					}

					if check.Status != wantCheck {
						t.Errorf("check %q: want %s, got %s", name, wantCheck, check.Status)
					}
				}
			})
		}
	}
}

// expectedStatus is the classification spec, written out independently of the
// implementation so the matrix test cannot drift into tautology.
func expectedStatus(states map[string]state, critical map[string]bool) health.Status {
	anyNonCriticalFailure := false

	for name, st := range states {
		if st.healthy {
			continue
		}

		if critical[name] {
			return health.StatusFail
		}

		anyNonCriticalFailure = true
	}

	if anyNonCriticalFailure {
		return health.StatusWarn
	}

	return health.StatusPass
}
