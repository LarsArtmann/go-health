package httputilinterop_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/larsartmann/httputil"
	servertiming "github.com/larsartmann/httputil/server_timing"
)

// wireCheck and wireResponse mirror the published wire contract
// (docs/openapi.yaml) without importing go-health's types: the point is to
// decode what a fleet consumer actually receives and assert on the JSON.
type wireCheck struct {
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
	DurationNanos int64  `json:"duration_ns,omitempty"`
}

type wireResponse struct {
	Status       string               `json:"status"`
	ShuttingDown bool                 `json:"shutting_down,omitempty"`
	Checks       map[string]wireCheck `json:"checks"`
}

func get(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	return resp
}

func decode(t *testing.T, body []byte) wireResponse {
	t.Helper()

	var wire wireResponse
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("decode readiness body: %v", err)
	}

	return wire
}

// TestLiveness_MiddlewareChainStaysAlwaysUp pins recipe 1: liveness stays
// open and unwrapped by auth/rate limiting, and cross-cutting middleware
// (Recovery outermost, RequestID) around it cannot change its contract —
// 200 with "pass" even while every dependency is failing.
func TestLiveness_MiddlewareChainStaysAlwaysUp(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(map[string]health.CheckFunc{
		"db": func(context.Context) error { return errFailing },
	})

	h := httputil.Chain(
		probe.LivenessHandler(),
		httputil.Recovery(slog.Default()),
		httputil.RequestID(httputil.DefaultRequestIDConfig()),
	)

	resp := get(t, h)

	if resp.Code != http.StatusOK {
		t.Fatalf("liveness through middleware: got %d, want 200", resp.Code)
	}
	if resp.Header().Get("X-Request-Id") == "" {
		t.Fatal("request-id middleware did not run (no X-Request-Id response header)")
	}
	if wire := decode(t, resp.Body.Bytes()); wire.Status != "pass" {
		t.Fatalf(
			"liveness status: got %q, want pass (dependency failures must never gate liveness)",
			wire.Status,
		)
	}
}

// TestReadiness_ChainPreservesCodeSemantics pins recipe 2: the full
// cross-cutting chain (Recovery → RequestID → Timeout → rate limiting)
// preserves go-health's status-code contract for all three roll-ups:
// pass and warn stay 200, fail is 503. Vocabulary note for readers of the
// raw JSON: go-health roll-ups are pass/warn/fail, httputil's stub health
// payload speaks up/down — the two vocabularies coexist, kubelet only reads
// the status code.
func TestReadiness_ChainPreservesCodeSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		checks     map[string]health.CheckFunc
		critical   []string
		wantCode   int
		wantStatus string
	}{
		{
			name: "healthy",
			checks: map[string]health.CheckFunc{
				"db": func(context.Context) error { return nil },
			},
			critical:   []string{"db"},
			wantCode:   http.StatusOK,
			wantStatus: "pass",
		},
		{
			name: "non-critical failure degrades to warn, stays 200",
			checks: map[string]health.CheckFunc{
				"cache": func(context.Context) error { return errFailing },
			},
			wantCode:   http.StatusOK,
			wantStatus: "warn",
		},
		{
			name: "critical failure is 503 fail",
			checks: map[string]health.CheckFunc{
				"db": func(context.Context) error { return errFailing },
			},
			critical:   []string{"db"},
			wantCode:   http.StatusServiceUnavailable,
			wantStatus: "fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := []health.Option{}
			if len(tt.critical) > 0 {
				opts = append(opts, health.WithCriticalServices(tt.critical...))
			}
			probe := health.NewChecks(tt.checks, opts...)

			h := httputil.Chain(
				probe.ReadinessHandler(),
				httputil.Recovery(slog.Default()),
				httputil.RequestID(httputil.DefaultRequestIDConfig()),
				httputil.Timeout(5*time.Second),
				httputil.KeyedRateLimiterMiddleware(httputil.DefaultKeyedRateLimiterConfig()),
			)

			resp := get(t, h)

			if resp.Code != tt.wantCode {
				t.Fatalf("readiness code: got %d, want %d", resp.Code, tt.wantCode)
			}
			if wire := decode(t, resp.Body.Bytes()); wire.Status != tt.wantStatus {
				t.Fatalf("readiness status: got %q, want %q", wire.Status, tt.wantStatus)
			}
		})
	}
}

// TestMethodGuard_ComposesOutsideMiddleware pins the composition order
// docs/middleware-design.md settled: middleware runs BEFORE the probe's
// method guard, so a non-allowed method traverses the whole chain and gets
// the guard's 405 + Allow header — never a middleware-level rejection.
func TestMethodGuard_ComposesOutsideMiddleware(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(map[string]health.CheckFunc{
		"db": func(context.Context) error { return nil },
	}, health.WithAllowedMethods(http.MethodGet))

	h := httputil.Chain(
		probe.ReadinessHandler(),
		httputil.Recovery(slog.Default()),
		httputil.KeyedRateLimiterMiddleware(httputil.DefaultKeyedRateLimiterConfig()),
	)

	postResp := httptest.NewRecorder()
	h.ServeHTTP(postResp, httptest.NewRequest(http.MethodPost, "/readyz", nil))
	if postResp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST readiness: got %d, want 405 from the method guard", postResp.Code)
	}
	if allow := postResp.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("Allow header: got %q, want %q", allow, http.MethodGet)
	}

	if resp := get(t, h); resp.Code != http.StatusOK {
		t.Fatalf("GET readiness: got %d, want 200", resp.Code)
	}
}

// TestShutdown_FlipsReadinessThroughChain pins the drain ordering recipe:
// probe.Shutdown() turns readiness 503 immediately (shutting_down: true),
// so the load balancer stops routing while httputil.Server drains its
// in-flight requests. No middleware may cache or mask that transition.
func TestShutdown_FlipsReadinessThroughChain(t *testing.T) {
	t.Parallel()

	probe := health.NewChecks(map[string]health.CheckFunc{
		"db": func(context.Context) error { return nil },
	}, health.WithCriticalServices("db"))

	h := httputil.Chain(
		probe.ReadinessHandler(),
		httputil.Recovery(slog.Default()),
		httputil.RequestID(httputil.DefaultRequestIDConfig()),
	)

	probe.Shutdown()

	resp := get(t, h)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness after Shutdown: got %d, want 503", resp.Code)
	}
	if wire := decode(t, resp.Body.Bytes()); !wire.ShuttingDown {
		t.Fatal("readiness after Shutdown must carry shutting_down: true")
	}
}

// serverTimingBridge is recipe 4 verbatim: publish each check's
// executor-reported duration as a W3C Server-Timing metric. It records
// BEFORE delegating, because the Server-Timing middleware commits the header
// at the response's first write — metrics recorded after that never ship.
// Metric names are HTTP tokens: go-health names containing '/' are sanitized
// to '_' by the servertiming package.
func serverTimingBridge(probe *health.Probe, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for name, check := range probe.CachedResponse().Checks {
			if check.DurationNanos > 0 {
				servertiming.RecordServerTiming(
					r.Context(),
					name,
					"",
					time.Duration(check.DurationNanos),
				)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// TestServerTiming_CheckDurationsOnTheWire proves the bridge: with the
// middleware in place, the readiness response carries per-check durations
// from NewWithDetailedCheck, plus the middleware's own total.
func TestServerTiming_CheckDurationsOnTheWire(t *testing.T) {
	t.Parallel()

	probe := health.NewWithDetailedCheck(func(context.Context) map[string]health.CheckDetail {
		return map[string]health.CheckDetail{
			"db": {Duration: 1500 * time.Microsecond},
		}
	})
	probe.EvaluateAndCache(context.Background())

	chained := httputil.Chain(
		probe.ReadinessHandler(),
		httputil.Recovery(slog.Default()),
	)
	h := servertiming.ServerTimingMiddleware()(serverTimingBridge(probe, chained))

	resp := get(t, h)

	if resp.Code != http.StatusOK {
		t.Fatalf("readiness code: got %d, want 200", resp.Code)
	}
	header := resp.Header().Get("Server-Timing")
	if !strings.Contains(header, "db;dur=") {
		t.Fatalf("Server-Timing header %q does not carry the db check duration", header)
	}
	if !strings.Contains(header, "total;") {
		t.Fatalf("Server-Timing header %q does not carry the middleware total metric", header)
	}
}

// errFailing is the fixture's failing check outcome; health.CheckDetail and
// NewChecks grade the error, so any error works.
var errFailing = errTestFailed{}

type errTestFailed struct{}

func (errTestFailed) Error() string { return "fixture check failed" }
