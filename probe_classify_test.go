package health_test

import (
	"context"
	"strings"
	"testing"

	health "github.com/larsartmann/go-health"
	do "github.com/samber/do/v2"
)

// state is one service's health state in the classify matrix.
type state struct {
	healthy bool
}

// serviceNames are the three probe services exercised by the matrix.
var serviceNames = []string{"a", "b", "c"}

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

	for _, states := range enumerateStates() {
		for _, critical := range enumerateCriticals() {
			t.Run(matrixLabel(states, critical), func(t *testing.T) {
				t.Parallel()

				assertMatrixCombo(t, states, critical)
			})
		}
	}
}

// enumerateStates enumerates every assignment of healthy/failing to the
// three services: 2^3 = 8 rows.
func enumerateStates() []map[string]state {
	rows := make([]map[string]state, 0, 8)

	for mask := range 8 {
		states := make(map[string]state, len(serviceNames))
		for i, name := range serviceNames {
			states[name] = state{healthy: mask&(1<<i) != 0}
		}

		rows = append(rows, states)
	}

	return rows
}

// enumerateCriticals enumerates every critical subset: 2^3 = 8 rows.
func enumerateCriticals() []map[string]bool {
	rows := make([]map[string]bool, 0, 8)

	for mask := range 8 {
		critical := make(map[string]bool, len(serviceNames))
		for i, name := range serviceNames {
			critical[name] = mask&(1<<i) != 0
		}

		rows = append(rows, critical)
	}

	return rows
}

// matrixLabel renders one matrix cell as a readable subtest name.
func matrixLabel(states map[string]state, critical map[string]bool) string {
	var b strings.Builder

	for _, name := range serviceNames {
		mark := "ok"
		if !states[name].healthy {
			mark = "FAIL"
		}

		b.WriteString(name)
		b.WriteString("=")
		b.WriteString(mark)

		if critical[name] {
			b.WriteString("(crit)")
		}

		b.WriteString(" ")
	}

	return b.String()
}

// assertMatrixCombo evaluates one (states, critical set) cell through the
// public API and checks roll-up and per-check grading against the spec.
func assertMatrixCombo(t *testing.T, states map[string]state, critical map[string]bool) {
	t.Helper()

	injector := do.New()
	t.Cleanup(func() { injector.Shutdown() })

	wantCritical := make([]string, 0, len(serviceNames))

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
