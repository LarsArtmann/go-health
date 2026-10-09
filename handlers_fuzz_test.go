package health_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	do "github.com/samber/do/v2"
)

// FuzzResponseMarshalDeterministic fuzzes Response string fields through the
// production marshal path. Invariants under any input: no panic, two
// consecutive marshals are byte-identical (Deterministic), and the payload
// round-trips status, instance_id, and check entries intact — including the
// per-check metadata (Since as nanoseconds-since-Unix-epoch, DurationNanos).
func FuzzResponseMarshalDeterministic(f *testing.F) {
	f.Add("pass", "db", "", "", int64(0), int64(0))
	f.Add("warn", "cache", "connection refused", "pod-1", int64(0), int64(0))
	// Golden-fixture seed: mirrors testdata/readiness_response.golden
	// (warn roll-up, non-critical "cache" failing with "connection refused",
	// instance pod-7f9c) so the corpus always carries the shipped wire shape.
	f.Add("warn", "cache", "connection refused", "pod-7f9c", int64(0), int64(0))
	f.Add(
		"fail",
		"db",
		"context deadline exceeded",
		"i-0abc123def",
		int64(1784000000_000000000),
		int64(1500000),
	)
	f.Add("", "", "", "replica-7.example.internal", int64(0), int64(0))
	f.Add("pass", "a/b c", `quote " backslash \ newline
`, "pod-\xff\xfe", int64(-42_000000000), int64(1))
	f.Add("pass", "db", "", "", int64(1<<62), int64(1<<62))

	f.Fuzz(
		func(t *testing.T, status, checkName, checkErr, instanceID string, sinceNanos, durationNanos int64) {
			resp := health.Response{
				Status:     health.Status(status),
				InstanceID: instanceID,
				Checks: map[string]health.Check{
					checkName: {
						Status:        health.Status(status),
						Error:         checkErr,
						Since:         time.Unix(0, sinceNanos).UTC(),
						DurationNanos: durationNanos,
					},
				},
			}

			// Mirror the production write seam: sanitize, then marshal. Invalid
			// UTF-8 (which json/v2 rejects, unlike v1) must never break serving.
			resp = health.SanitizeResponse(resp)

			first, err := json.Marshal(resp, json.Deterministic(true))
			if err != nil {
				t.Fatalf("marshal must not fail after sanitize: %v", err)
			}

			second, err := json.Marshal(resp, json.Deterministic(true))
			if err != nil {
				t.Fatalf("marshal must not fail on second pass: %v", err)
			}

			if string(first) != string(second) {
				t.Fatalf("non-deterministic output:\n%s\n%s", first, second)
			}

			var decoded health.Response
			if err := json.Unmarshal(first, &decoded); err != nil {
				t.Fatalf("round-trip unmarshal: %v", err)
			}

			if decoded.Status != resp.Status {
				t.Fatalf("status round-trip: want %q, got %q", resp.Status, decoded.Status)
			}

			if decoded.InstanceID != resp.InstanceID {
				t.Fatalf(
					"instance_id round-trip: want %q, got %q",
					resp.InstanceID,
					decoded.InstanceID,
				)
			}

			for name, wantCheck := range resp.Checks {
				check, ok := decoded.Checks[name]
				if !ok {
					t.Fatalf("check %q lost in round-trip", name)
				}

				if check.Error != wantCheck.Error {
					t.Fatalf(
						"check error round-trip: want %q, got %q",
						wantCheck.Error,
						check.Error,
					)
				}

				if !check.Since.Equal(wantCheck.Since) {
					t.Fatalf(
						"check since round-trip: want %v, got %v",
						wantCheck.Since,
						check.Since,
					)
				}

				if check.DurationNanos != wantCheck.DurationNanos {
					t.Fatalf(
						"check duration round-trip: want %d, got %d",
						wantCheck.DurationNanos,
						check.DurationNanos,
					)
				}
			}
		},
	)
}

// FuzzThrottleWindowBoundary fuzzes clock advances around the
// [health.WithLiveThrottle] window on a fake clock. For any two advances
// (positive, zero, or negative — a backward clock), the number of live
// evaluation batches must exactly match the window rule — serve the stored
// result while it is younger than the window, re-evaluate otherwise — and
// every request must return 200 with a decodable body. Pins the boundary
// itself (window-1 serves cache, window re-evaluates) and the backward-clock
// case (negative freshness never re-evaluates) against arbitrary fuzz inputs.
func FuzzThrottleWindowBoundary(f *testing.F) {
	const window = time.Second

	f.Add(int64(0), int64(0))
	f.Add(int64(window-1), int64(1))
	f.Add(int64(window), int64(window))
	f.Add(int64(-1), int64(2*window))
	f.Add(int64(window+1), int64(-1))

	f.Fuzz(func(t *testing.T, advanceOneNs, advanceTwoNs int64) {
		// Cap magnitude (keeping sign) so huge fuzz values cannot overflow
		// time.Duration arithmetic.
		const capNs = int64(time.Hour)

		advanceOne := time.Duration(advanceOneNs % capNs)
		advanceTwo := time.Duration(advanceTwoNs % capNs)

		var batches atomic.Int64

		epoch := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
		clock := newMutableClock(epoch)

		probe := health.NewWithHealthCheck(func(context.Context) map[string]error {
			batches.Add(1)

			return map[string]error{"db": nil}
		},
			health.WithRefreshInterval(0),
			health.WithLiveThrottle(window),
			health.WithNowFunc(clock.Now),
		)

		if err := probe.Start(context.Background()); err != nil {
			t.Fatalf("probe.Start: %v", err)
		}

		// Simulate the window rule: lastEvalNs tracks when the stored result
		// was stamped; a request re-evaluates iff now-lastEval >= window.
		lastEvalNs := clock.Now().UnixNano()

		wantBatches := batches.Load() // Start's initial refresh already ran
		handler := probe.ReadinessHandler()

		request := func() {
			t.Helper()

			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

			if rec.Code != http.StatusOK {
			t.Fatalf("readiness status: want 200, got %d", rec.Code)
			}

			var body health.Response
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}

			nowNs := clock.Now().UnixNano()
			if nowNs-lastEvalNs >= int64(window) {
				wantBatches++
				lastEvalNs = nowNs
			}
		}

		request()

		clock.Advance(advanceOne)
		request()

		clock.Advance(advanceTwo)
		request()

		if got := batches.Load(); got != wantBatches {
			t.Fatalf("evaluation batches: want %d by the window rule, got %d (advances %v, %v)",
				wantBatches, got, advanceOne, advanceTwo)
		}
	})
}

// FuzzHandlerInput fuzzes HTTP method and request target against all three
// handlers (with GET-only enforcement on). Invariants: the handler never
// panics and always answers with a status code in the supported set.
func FuzzHandlerInput(f *testing.F) {
	f.Add("GET", "/healthz")
	f.Add("POST", "/readyz")
	f.Add("HEAD", "/startupz")
	f.Add("DELETE", "/healthz?probe=all")
	f.Add("GET", "//localhost:8080/readyz")

	f.Fuzz(func(t *testing.T, method, target string) {
		injector := do.New()

		provideHealthy(injector, "db")
		invoke[*healthyService](t, injector, "db")

		t.Cleanup(func() { injector.Shutdown() })

		probe := health.New(injector,
			health.WithCriticalServices("db"),
			health.WithGETOnly(),
		)

		handlers := map[string]http.HandlerFunc{
			"liveness":  probe.LivenessHandler(),
			"readiness": probe.ReadinessHandler(),
			"startup":   probe.StartupHandler(),
		}

		for name, handler := range handlers {
			r, err := http.NewRequestWithContext(context.Background(), method, target, nil)
			if err != nil {
				return // malformed request line: nothing to test
			}

			w := httptest.NewRecorder()

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("%s handler panicked on %s %s: %v", name, method, target, r)
					}
				}()

				handler(w, r)
			}()

			switch w.Code {
			case http.StatusOK, http.StatusMethodNotAllowed, http.StatusServiceUnavailable:
			default:
				t.Fatalf("%s handler: unexpected status %d for %s %s", name, w.Code, method, target)
			}
		}
	})
}
