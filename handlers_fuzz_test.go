package health_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	health "github.com/larsartmann/go-health"
	do "github.com/samber/do/v2"
)

// FuzzResponseMarshalDeterministic fuzzes Response string fields through the
// production marshal path. Invariants under any input: no panic, two
// consecutive marshals are byte-identical (Deterministic), and the payload
// round-trips status and check entries intact.
func FuzzResponseMarshalDeterministic(f *testing.F) {
	f.Add("pass", "db", "")
	f.Add("warn", "cache", "connection refused")
	f.Add("fail", "db", "context deadline exceeded")
	f.Add("", "", "")
	f.Add("pass", "a/b c", `quote " backslash \ newline
`)

	f.Fuzz(func(t *testing.T, status, checkName, checkErr string) {
		resp := health.Response{
			Status: health.Status(status),
			Checks: map[string]health.Check{
				checkName: {Status: health.Status(status), Error: checkErr},
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

		for name, wantCheck := range resp.Checks {
			check, ok := decoded.Checks[name]
			if !ok {
				t.Fatalf("check %q lost in round-trip", name)
			}

			if check.Error != wantCheck.Error {
				t.Fatalf("check error round-trip: want %q, got %q", wantCheck.Error, check.Error)
			}
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
