package health_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
)

// These tests pin the critical-name contract: a name passed to
// WithCriticalServices that never appears in a health-check batch would
// silently degrade readiness classification and block the startup latch
// forever (the DiscordSync 2026-08-16 bug class). Start() now rejects such
// configurations with ErrUnknownCriticalService. See
// docs/start-validation-design.md.

func newCriticalNamesProbe(
	t *testing.T,
	batch func(ctx context.Context) map[string]error,
	critical []string,
) *health.Probe {
	t.Helper()

	return health.NewWithHealthCheck(batch, health.WithCriticalServices(critical...))
}

func startCriticalProbe(t *testing.T, probe *health.Probe) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
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

// TestStart_RejectsUnknownCriticalName proves the typo case: a critical name
// absent from the batch fails Start with ErrUnknownCriticalService instead of
// silently blocking the startup latch forever.
func TestStart_RejectsUnknownCriticalName(t *testing.T) {
	t.Parallel()

	probe := newCriticalNamesProbe(
		t,
		func(_ context.Context) map[string]error {
			return map[string]error{"database": nil}
		},
		[]string{"databse"},
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := probe.Start(ctx)
	if !errors.Is(err, health.ErrUnknownCriticalService) {
		t.Fatalf("Start with typo'd critical name: want ErrUnknownCriticalService, got %v", err)
	}

	if !strings.Contains(err.Error(), "databse") {
		t.Errorf("error should name the unknown service, got: %v", err)
	}

	if probe.StartupComplete() {
		t.Error("startup latch must remain unset after a failed Start")
	}
}

// TestStart_RejectsUnknownCriticalName_MultipleNamesSorted verifies several
// unknown names are all reported, sorted.
func TestStart_RejectsUnknownCriticalName_MultipleNamesSorted(t *testing.T) {
	t.Parallel()

	probe := newCriticalNamesProbe(
		t,
		func(_ context.Context) map[string]error {
			return map[string]error{"database": nil}
		},
		[]string{"zache", "databse"},
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := probe.Start(ctx)
	if !errors.Is(err, health.ErrUnknownCriticalService) {
		t.Fatalf("want ErrUnknownCriticalService, got %v", err)
	}

	databseIdx := strings.Index(err.Error(), "databse")
	zacheIdx := strings.Index(err.Error(), "zache")

	if databseIdx == -1 || zacheIdx == -1 {
		t.Fatalf("error should name both unknown services, got: %v", err)
	}

	if databseIdx > zacheIdx {
		t.Errorf("unknown names should be reported sorted, got: %v", err)
	}
}

// TestStart_RejectsUnknownCriticalName_MixedBatch verifies a partially
// unknown set reports only the unknown names while known ones start fine
// once fixed.
func TestStart_RejectsUnknownCriticalName_MixedKnownAndUnknown(t *testing.T) {
	t.Parallel()

	probe := newCriticalNamesProbe(
		t,
		func(_ context.Context) map[string]error {
			return map[string]error{"database": nil}
		},
		[]string{"database", "cachee"},
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := probe.Start(ctx)
	if !errors.Is(err, health.ErrUnknownCriticalService) {
		t.Fatalf("want ErrUnknownCriticalService, got %v", err)
	}

	if strings.Contains(err.Error(), "database,") || strings.Contains(err.Error(), "database ") {
		t.Errorf("known name must not be reported as unknown, got: %v", err)
	}
}

// TestStart_AcceptsKnownCriticalNames is the control: names present in the
// batch start cleanly, the latch sets on the first startup probe, and a
// failing critical service still grades fail.
func TestStart_AcceptsKnownCriticalNames(t *testing.T) {
	t.Parallel()

	t.Run("passing batch starts and latches", func(t *testing.T) {
		t.Parallel()

		probe := newCriticalNamesProbe(
			t,
			func(_ context.Context) map[string]error {
				return map[string]error{"database": nil, "cache": nil}
			},
			[]string{"database", "cache"},
		)

		startCriticalProbe(t, probe)

		if code := startupVerdict(t, probe); code != http.StatusOK {
			t.Fatalf(
				"startup handler returned %d although all critical services passed; want 200",
				code,
			)
		}

		if !probe.StartupComplete() {
			t.Fatal("startup latch not set although all critical services passed")
		}
	})

	t.Run("failing critical service grades fail", func(t *testing.T) {
		t.Parallel()

		probe := newCriticalNamesProbe(
			t,
			func(_ context.Context) map[string]error {
				return map[string]error{"database": errConnectionRefused}
			},
			[]string{"database"},
		)

		startCriticalProbe(t, probe)

		if got := probe.Status(); got != health.StatusFail {
			t.Fatalf("failing critical service graded %v; want fail", got)
		}
	})
}

// TestStart_EmptyCriticalSetNeverValidates proves the no-critical-set
// configuration is unaffected by validation.
func TestStart_EmptyCriticalSetNeverValidates(t *testing.T) {
	t.Parallel()

	probe := health.NewWithHealthCheck(func(_ context.Context) map[string]error {
		return map[string]error{"anything": nil}
	})

	startCriticalProbe(t, probe)

	if got := probe.Status(); got != health.StatusPass {
		t.Fatalf("status %v; want pass", got)
	}
}

// TestStart_FailedStartLeavesNoBackgroundLoop verifies Start stays
// side-effect free on validation failure: no refresh loop, no cache.
func TestStart_FailedStartLeavesNoBackgroundLoop(t *testing.T) {
	t.Parallel()

	probe := newCriticalNamesProbe(
		t,
		func(_ context.Context) map[string]error {
			return map[string]error{"database": nil}
		},
		[]string{"databse"},
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := probe.Start(ctx); !errors.Is(err, health.ErrUnknownCriticalService) {
		t.Fatalf("want ErrUnknownCriticalService, got %v", err)
	}

	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		if cached := probe.CachedResponse(); cached.Status == health.StatusPass &&
			len(cached.Checks) > 0 {
			t.Fatal("cache was populated although Start failed (background loop leaked)")
		}

		time.Sleep(10 * time.Millisecond)
	}
}

var errConnectionRefused = errors.New("connection refused")
