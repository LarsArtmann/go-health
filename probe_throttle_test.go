package health_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
)

// --- P30: live throttle ---.

func TestWithLiveThrottle_CoalescesLiveEvaluations(t *testing.T) {
	t.Parallel()

	var batches atomic.Int64

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		batches.Add(1)

		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithLiveThrottle(50*time.Millisecond),
	)

	handler := probe.ReadinessHandler()

	var wg sync.WaitGroup

	for range 20 {
		wg.Go(func() {
			rec := doRequest(t, handler, "/readyz")
			if rec.Code != http.StatusOK {
				t.Errorf("status: want 200, got %d", rec.Code)
			}
		})
	}

	wg.Wait()

	if calls := batches.Load(); calls > 5 {
		t.Errorf(
			"20 requests within the throttle window ran %d batches; want coalescing (<=5)",
			calls,
		)
	}

	// After the window elapses a fresh batch runs.
	time.Sleep(60 * time.Millisecond)

	before := batches.Load()
	rec := doRequest(t, handler, "/readyz")

	if rec.Code != http.StatusOK {
		t.Errorf("post-window status: want 200, got %d", rec.Code)
	}

	if after := batches.Load(); after != before+1 {
		t.Errorf("post-window batches: want %d, got %d", before+1, after)
	}
}

// mutableClock is a controllable clock for determinism tests. Evaluate stamps
// Response.Timestamp from it and the live throttle compares freshness against
// it, so tests advance it explicitly instead of sleeping.
type mutableClock struct {
	nowNs atomic.Int64
}

func newMutableClock(t time.Time) *mutableClock {
	c := &mutableClock{}
	c.nowNs.Store(t.UnixNano())

	return c
}

func (c *mutableClock) Now() time.Time { return time.Unix(0, c.nowNs.Load()) }

func (c *mutableClock) Advance(d time.Duration) { c.nowNs.Add(int64(d)) }

// requestReadiness performs one GET against the readiness handler and
// returns the response body, failing the test on a non-200 status.
func requestReadiness(t *testing.T, handler http.HandlerFunc, path string) string {
	t.Helper()

	rec := doRequest(t, handler, path)
	if rec.Code != http.StatusOK {
		t.Fatalf("readiness request: want 200, got %d", rec.Code)
	}

	return rec.Body.String()
}

func TestWithLiveThrottle_FakeClockFreshnessDeterministic(t *testing.T) {
	t.Parallel()

	const window = time.Second

	var batches atomic.Int64

	epoch := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	clock := newMutableClock(epoch)

	probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
		batches.Add(1)

		return map[string]error{"svc": nil}
	},
		health.WithCriticalServices("svc"),
		health.WithRefreshInterval(0),
		health.WithLiveThrottle(window),
		health.WithBootTime(epoch),
		health.WithNowFunc(clock.Now),
	)

	handler := probe.ReadinessHandler()

	first := requestReadiness(t, handler, "/readyz")

	if got := batches.Load(); got != 1 {
		t.Fatalf("first request batches: want 1, got %d", got)
	}

	assertTimestampOf(t, first, epoch)

	// With the clock frozen, any number of requests must serve the stored
	// result byte-identically: one batch total, one response.
	assertFrozenWindow(t, handler, "/readyz", first, 25)

	if got := batches.Load(); got != 1 {
		t.Errorf("25 frozen-clock requests ran %d batches; want exactly 1", got)
	}

	// Crossing the freshness boundary (now - Timestamp >= window) must run
	// exactly one new batch; the new result is then frozen the same way.
	clock.Advance(window + time.Millisecond)

	second := requestReadiness(t, handler, "/readyz")

	if got := batches.Load(); got != 2 {
		t.Fatalf("post-window batches: want 2, got %d", got)
	}

	if second == first {
		t.Error("post-window body equals pre-window body; freshness marker did not advance")
	}

	assertFrozenWindow(t, handler, "/readyz", second, 10)

	if got := batches.Load(); got != 2 {
		t.Errorf("post-window re-frozen requests ran %d batches; want still 2", got)
	}
}

// assertTimestampOf verifies the response's timestamp equals the expected
// instant, proving Evaluate stamps from the injected clock.
func assertTimestampOf(t *testing.T, body string, want time.Time) {
	t.Helper()

	var decoded struct {
		Timestamp time.Time `json:"timestamp"`
	}

	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !decoded.Timestamp.Equal(want) {
		t.Errorf(
			"response timestamp not stamped from fake clock: want %v, got %v",
			want,
			decoded.Timestamp,
		)
	}
}

// assertFrozenWindow fires concurrent requests with the clock frozen and
// verifies every one serves wantBody byte-identically.
func assertFrozenWindow(
	t *testing.T,
	handler http.HandlerFunc,
	path, wantBody string,
	requests int,
) {
	t.Helper()

	var wg sync.WaitGroup

	for range requests {
		wg.Go(func() {
			if body := requestReadiness(t, handler, path); body != wantBody {
				t.Errorf(
					"frozen-clock body drift within window:\n want %s\n got  %s",
					wantBody,
					body,
				)
			}
		})
	}

	wg.Wait()
}
