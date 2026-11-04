package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Benchmarks ---.

func BenchmarkLivenessHandler(b *testing.B) {
	injector := do.New()
	probe := health.New(injector, health.WithVersion("1.0.0"))
	handler := probe.LivenessHandler()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler(w, r)
	}
}

func BenchmarkReadinessHandler_CacheHit(b *testing.B) {
	injector := do.New()
	provideHealthy(injector, "db")
	do.MustInvokeNamed[*healthyService](injector, "db")

	probe := health.New(injector, health.WithCriticalServices("db"))

	if err := probe.Start(context.Background()); err != nil {
		b.Fatal(err)
	}
	defer probe.Shutdown()

	handler := probe.ReadinessHandler()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler(w, r)
	}
}

func BenchmarkReadinessHandler_LiveEval(b *testing.B) {
	injector := do.New()
	provideHealthy(injector, "db")
	do.MustInvokeNamed[*healthyService](injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)
	handler := probe.ReadinessHandler()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler(w, r)
	}
}

func BenchmarkEvaluate(b *testing.B) {
	injector := do.New()
	provideHealthy(injector, "db")
	provideHealthy(injector, "cache")
	do.MustInvokeNamed[*healthyService](injector, "db")
	do.MustInvokeNamed[*healthyService](injector, "cache")

	probe := health.New(injector, health.WithCriticalServices("db", "cache"))

	ctx := context.Background()

	b.ResetTimer()

	for b.Loop() {
		_ = probe.Evaluate(ctx)
	}
}

func BenchmarkStartupHandler_Unlatched(b *testing.B) {
	injector := do.New()
	provideHealthy(injector, "db")
	do.MustInvokeNamed[*healthyService](injector, "db")

	probe := health.New(injector,
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	handler := probe.StartupHandler()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/startupz", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler(w, r)
	}
}

func BenchmarkReadinessHandler_RecorderPath(b *testing.B) {
	recorder := &mockRecorder{result: map[string]error{"db": nil}}

	probe := health.New(do.New(),
		health.WithHealthRecorder(recorder),
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	handler := probe.ReadinessHandler()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler(w, r)
	}
}

// --- CachedResponse tests ---.

func TestCachedResponse_ReturnsCachedValue_WhenAvailable(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector, health.WithCriticalServices("db"))

	mustStart(t, probe, t.Context())
	defer probe.Shutdown()

	cached := probe.CachedResponse()

	if cached.Status != health.StatusPass {
		t.Errorf("cached status: want pass, got %s", cached.Status)
	}

	if len(cached.Checks) != 1 {
		t.Errorf("cached checks: want 1, got %d", len(cached.Checks))
	}
}

func TestCachedResponse_ReturnsZeroValue_WhenNoCache(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithRefreshInterval(0))

	// Never started, so no cache exists.
	cached := probe.CachedResponse()

	if cached.Status != health.StatusPass {
		t.Errorf("zero-value cached status: want pass, got %s", cached.Status)
	}

	if len(cached.Checks) != 0 {
		t.Errorf("zero-value cached checks: want empty, got %d", len(cached.Checks))
	}
}

func TestCachedResponse_OverlaysShutdownFlag(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	probe := health.New(injector, health.WithCriticalServices("db"))
	mustStart(t, probe, t.Context())

	// The cache was populated while healthy. Now mark shutdown.
	probe.Shutdown()

	cached := probe.CachedResponse()

	if !cached.ShuttingDown {
		t.Error("cached response after shutdown: want ShuttingDown=true")
	}

	if cached.Status != health.StatusFail {
		t.Errorf("cached status after shutdown: want fail, got %s", cached.Status)
	}
}

// --- RefreshInterval accessor test ---.

func TestRefreshInterval_ReturnsConfiguredValue(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithRefreshInterval(42*time.Second))
	if got := probe.RefreshInterval(); got != 42*time.Second {
		t.Errorf("refresh interval: want 42s, got %s", got)
	}
}

func TestRefreshInterval_DefaultIsOneSecond(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New())
	if got := probe.RefreshInterval(); got != 1*time.Second {
		t.Errorf("default refresh interval: want 1s, got %s", got)
	}
}

func TestRefreshInterval_ZeroInLiveMode(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithRefreshInterval(0))
	if got := probe.RefreshInterval(); got != 0 {
		t.Errorf("live mode refresh interval: want 0, got %s", got)
	}
}
