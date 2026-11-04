package health_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)


type healthyService struct{}

var _ do.HealthcheckerWithContext = (*healthyService)(nil)

func (healthyService) HealthCheck(_ context.Context) error { return nil }

var errUnhealthy = errors.New("service unhealthy")

type unhealthyService struct{ reason string }

var _ do.HealthcheckerWithContext = (*unhealthyService)(nil)

func (u *unhealthyService) HealthCheck(_ context.Context) error {
	return fmt.Errorf("%w: %s", errUnhealthy, u.reason)
}

type slowService struct {
	delay time.Duration
}

var _ do.HealthcheckerWithContext = (*slowService)(nil)

func (s *slowService) HealthCheck(ctx context.Context) error {
	select {
	case <-time.After(s.delay):
		return nil
	case <-ctx.Done():
		return fmt.Errorf("health check interrupted: %w", ctx.Err())
	}
}

type countingService struct {
	calls atomic.Int64
}

var _ do.HealthcheckerWithContext = (*countingService)(nil)

func (c *countingService) HealthCheck(_ context.Context) error {
	c.calls.Add(1)

	return nil
}

// --- Mock HealthRecorder ---.

type mockRecorder struct {
	calls  atomic.Int64
	result map[string]error
}

func (m *mockRecorder) RecordHealthCheckWithContext(
	_ context.Context,
	_ do.Injector,
) map[string]error {
	m.calls.Add(1)

	return m.result
}

// panicRecorder simulates a misbehaving recorder that panics during health checks.
type panicRecorder struct{}

func (panicRecorder) RecordHealthCheckWithContext(
	_ context.Context,
	_ do.Injector,
) map[string]error {
	panic("recorder exploded")
}

// --- Test helpers ---.

func provideHealthy(i do.Injector, name string) {
	do.ProvideNamed(i, name, func(_ do.Injector) (*healthyService, error) {
		return &healthyService{}, nil
	})
}

func provideUnhealthy(i do.Injector, name, reason string) {
	do.ProvideNamed(i, name, func(_ do.Injector) (*unhealthyService, error) {
		return &unhealthyService{reason: reason}, nil
	})
}

func provideCounting(i do.Injector, name string) *countingService {
	svc := &countingService{}

	do.ProvideNamed(i, name, func(_ do.Injector) (*countingService, error) {
		return svc, nil
	})

	return svc
}

func mustStart(t *testing.T, probe *health.Probe, ctx context.Context) {
	t.Helper()

	if err := probe.Start(ctx); err != nil {
		t.Fatalf("probe.Start: %v", err)
	}
}

func invoke[T any](t *testing.T, i do.Injector, name string) T {
	t.Helper()

	return do.MustInvokeNamed[T](i, name)
}

func doRequest(t *testing.T, handler http.HandlerFunc, target string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}

	handler(w, r)

	return w
}

func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) health.Response {
	t.Helper()

	var resp health.Response

	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	return resp
}
