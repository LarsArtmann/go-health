package federation_test

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// errBodyReadFault is a package-level sentinel for the mid-body transport
// failure: a connection that dies after the headers arrived.
var errBodyReadFault = errors.New("connection reset mid-body")

// roundTripFunc adapts a function to [http.RoundTripper] so a test can
// inspect the outgoing request and answer without a live server.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// failingBody fails on the first read, simulating a truncated response.
type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errBodyReadFault }
func (failingBody) Close() error             { return nil }

// passDocument marshals the minimal valid remote document.
func passDocument(tb testing.TB) []byte {
	tb.Helper()

	payload, err := json.Marshal(
		health.Response{Status: health.StatusPass, Checks: map[string]health.Check{}},
		json.Deterministic(true),
	)
	if err != nil {
		tb.Fatalf("marshal canned document: %v", err)
	}

	return payload
}

// TestWithClient_UsesSuppliedClient pins the transport escape hatch: the
// supplied client's transport — not net/http's default — carries every fetch,
// which is what lets a consumer add pooling, TLS config, or a proxy. The
// request must also keep the Accept header the package promises.
func TestWithClient_UsesSuppliedClient(t *testing.T) {
	t.Parallel()

	var (
		requests atomic.Int64
		accept   atomic.Value
	)

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requests.Add(1)
			accept.Store(r.Header.Get("Accept"))

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader(passDocument(t))),
				Request:    r,
			}, nil
		}),
	}

	fed := mustNew(t,
		[]federation.Remote{{Name: "api", URL: "http://remote.invalid/readyz"}},
		federation.WithClient(client),
	)

	got := fed.CachedResponse()

	if requests.Load() != 1 {
		t.Fatalf("supplied client carried %d requests, want 1", requests.Load())
	}

	if accept.Load() != "application/json" {
		t.Errorf("Accept header = %v, want application/json", accept.Load())
	}

	if got.Status != health.StatusPass {
		t.Errorf("status = %q, want pass", got.Status)
	}
}

// TestCachedResponse_BodyReadErrorIsReported covers the truncated-response
// branch: headers arrive, the body read fails. It must surface as a reachable
// fail row naming the cause, never as a silent pass or a panic.
func TestCachedResponse_BodyReadErrorIsReported(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       failingBody{},
				Request:    r,
			}, nil
		}),
	}

	fed := mustNew(t,
		[]federation.Remote{{Name: "api", URL: "http://remote.invalid/readyz"}},
		federation.WithClient(client),
	)

	got := fed.CachedResponse()

	if got.Status != health.StatusFail {
		t.Fatalf("truncated body: want overall fail, got %q", got.Status)
	}

	reachable := got.Checks["api/reachable"]
	if reachable.Status != health.StatusFail {
		t.Fatalf("reachable row = %+v, want a fail row", reachable)
	}

	if !strings.Contains(reachable.Error, "read body") ||
		!strings.Contains(reachable.Error, errBodyReadFault.Error()) {
		t.Errorf("reachable cause %q must name the body read and its fault", reachable.Error)
	}
}

// TestHealthz_ConcurrentDrainIsRaceFree drives the combined handler while the
// hub drains. The race detector (gates run -race) is the primary assertion;
// the behavioural ones are that a drain is never rolled back and that it does
// not reopen the startup latch.
func TestHealthz_ConcurrentDrainIsRaceFree(t *testing.T) {
	t.Parallel()

	upstream := healthyUpstream(t)
	fed := mustNew(t, []federation.Remote{{Name: "api", URL: upstream.server.URL}})

	handler := fed.Healthz()

	var wg sync.WaitGroup

	for range 8 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 4 {
				handler(
					httptest.NewRecorder(),
					httptest.NewRequest(http.MethodGet, "/healthz", nil),
				)
			}
		}()
	}

	fed.MarkShuttingDown()
	wg.Wait()

	code, resp := healthzCall(t, fed)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("drained hub: want 503, got %d", code)
	}

	if !resp.ShuttingDown {
		t.Error("drained body must carry shutting_down")
	}

	if !fed.StartupComplete() {
		t.Error("a drain must not unset the startup latch")
	}
}
