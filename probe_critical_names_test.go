package health_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
)

// These tests pin the critical-name footgun: WithCriticalServices takes loose
// strings, and a name that never appears in a health-check batch is silently
// ignored — degrading readiness classification (a failing service graded warn
// instead of fail) and blocking the startup latch forever with zero
// diagnostics. They document the defect until Start()-time validation
// (ErrUnknownCriticalService) lands; once it does, they double as proof the
// validation catches exactly these scenarios.

func newCriticalNamesProbe(
	t *testing.T,
	batch func(ctx context.Context) map[string]error,
	critical []string,
) *health.Probe {
	t.Helper()

	return health.NewWithHealthCheck(batch, health.WithCriticalServices(critical...))
}

// startupVerdict serves one request on the startup handler and reports the
// status code: 200 means the latch has set, 503 means booting (or stuck).
func startupVerdict(t *testing.T, probe *health.Probe) int {
	t.Helper()

	rec := httptest.NewRecorder()

	probe.StartupHandler().
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz/startup", nil))

	return rec.Code
}

// TestCriticalNameTypo_BlocksStartupLatchForever proves the startup failure
// mode: a typo'd critical name is never found in the batch, so
// evaluateStartup never sees it pass and the latch never sets — silently, no
// matter how many startup probes arrive and how healthy the batch is.
func TestCriticalNameTypo_BlocksStartupLatchForever(t *testing.T) {
	t.Parallel()

	batch := func(_ context.Context) map[string]error {
		return map[string]error{"database": nil}
	}

	probe := newCriticalNamesProbe(t, batch, []string{"databse"})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if code := startupVerdict(t, probe); code == http.StatusOK {
			t.Fatal(
				"startup latch set despite unmatched critical name (behavior changed — revisit this test)",
			)
		}

		time.Sleep(20 * time.Millisecond)
	}

	if probe.StartupComplete() {
		t.Fatal("startup latch set despite unmatched critical name")
	}
}

// TestCriticalNameTypo_FailingServiceGradedWarn proves the readiness failure
// mode: with a typo'd critical set, a failing service that was meant to be
// critical is graded warn (HTTP 200, degraded) instead of fail (HTTP 503).
func TestCriticalNameTypo_FailingServiceGradedWarn(t *testing.T) {
	t.Parallel()

	probe := newCriticalNamesProbe(
		t,
		func(_ context.Context) map[string]error {
			return map[string]error{"database": errConnectionRefused}
		},
		[]string{"databse"},
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if got := probe.Status(); got != health.StatusWarn {
		t.Fatalf(
			"typo'd critical name downgraded a failing service to %v; want warn (non-critical degradation)",
			got,
		)
	}

	rec := httptest.NewRecorder()

	probe.ReadinessHandler().
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz/ready", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"readiness returned %d for a failing intended-critical service; want 200 (the footgun)",
			rec.Code,
		)
	}
}

// TestCriticalNames_MatchedNameClassifiesCorrectly is the control: with names
// that match the batch, a failing critical service grades fail and the latch
// sets on the first startup probe once every critical service has passed.
func TestCriticalNames_MatchedNameClassifiesCorrectly(t *testing.T) {
	t.Parallel()

	t.Run("failing critical service grades fail", func(t *testing.T) {
		t.Parallel()

		probe := newCriticalNamesProbe(
			t,
			func(_ context.Context) map[string]error {
				return map[string]error{"database": errConnectionRefused}
			},
			[]string{"database"},
		)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		if err := probe.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}

		if got := probe.Status(); got != health.StatusFail {
			t.Fatalf("failing critical service graded %v; want fail", got)
		}

		if probe.StartupComplete() {
			t.Fatal("startup latch set while a critical service fails")
		}
	})

	t.Run("passing critical services set the latch", func(t *testing.T) {
		t.Parallel()

		probe := newCriticalNamesProbe(
			t,
			func(_ context.Context) map[string]error {
				return map[string]error{"database": nil, "cache": nil}
			},
			[]string{"database", "cache"},
		)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		if err := probe.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}

		if code := startupVerdict(t, probe); code != http.StatusOK {
			t.Fatalf(
				"startup handler returned %d although all critical services passed; want 200",
				code,
			)
		}

		if !probe.StartupComplete() {
			t.Fatal("startup latch not set although all critical services passed")
		}

		if got := probe.Status(); got != health.StatusPass {
			t.Fatalf("all-healthy batch graded %v; want pass", got)
		}
	})
}

var errConnectionRefused = errors.New("connection refused")
