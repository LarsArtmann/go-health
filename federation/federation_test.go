package federation_test

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	federation "github.com/larsartmann/go-health/federation"
)

// dashboardProber is the consumer-side interface go-health-dashboard
// defines (dashboard.Prober). go-health cannot import the dashboard —
// the dependency points the other way — so the conformance is asserted
// structurally: if this compiles, a *federation.Prober renders in the
// dashboard like any local probe.
type dashboardProber interface {
	CachedResponse() health.Response
	RefreshInterval() time.Duration
	LivenessHandler() http.HandlerFunc
	ReadinessHandler() http.HandlerFunc
	StartupHandler() http.HandlerFunc
}

var _ dashboardProber = (*federation.Prober)(nil)

// errQueueRefused is the canned non-critical failure for probe-backed
// upstreams (static, so the classification is errors.Is-able).
var errQueueRefused = errors.New("connection refused")

// --- Test helpers ---.

// jsonRemote stands up one upstream HTTP server for the test's lifetime.
type jsonRemote struct {
	server *httptest.Server
}

func newRemote(tb testing.TB, handler http.HandlerFunc) jsonRemote {
	tb.Helper()

	server := httptest.NewServer(handler)
	tb.Cleanup(server.Close)

	return jsonRemote{server: server}
}

func serveDocument(tb testing.TB, doc health.Response) jsonRemote {
	tb.Helper()

	payload, err := json.Marshal(doc, json.Deterministic(true))
	if err != nil {
		tb.Fatalf("marshal test document: %v", err)
	}

	return newRemote(tb, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	})
}

// healthyUpstream serves a valid pass document and rejects requests
// without the Accept header federation must send — the content
// negotiation contract with dashboard-hosted remotes.
func healthyUpstream(tb testing.TB) jsonRemote {
	tb.Helper()

	return newRemote(tb, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			w.WriteHeader(http.StatusNotAcceptable)

			return
		}

		_, _ = w.Write([]byte(`{"status":"pass","checks":{}}`))
	})
}

func mustNew(
	tb testing.TB,
	remotes []federation.Remote,
	opts ...federation.Option,
) *federation.Prober {
	tb.Helper()

	prober, err := federation.New(remotes, opts...)
	if err != nil {
		tb.Fatalf("New: unexpected error: %v", err)
	}

	return prober
}

// darkUpstream is an address with nothing listening: connection refused.
func darkUpstream() string {
	return "http://127.0.0.1:1/readyz"
}

// TestNew_Validation walks the construction contract: every invalid
// configuration is rejected with the documented sentinel and a cause
// naming the offending value; the valid baseline constructs.
func TestNew_Validation(t *testing.T) {
	t.Parallel()

	good := federation.Remote{Name: "ok", URL: "http://localhost:9101/readyz"}

	tests := []struct {
		name    string
		remotes []federation.Remote
		opts    []federation.Option
		wantErr error
	}{
		{
			name:    "no remotes",
			remotes: nil,
			wantErr: federation.ErrNoRemotes,
		},
		{
			name:    "empty name",
			remotes: []federation.Remote{{Name: "", URL: "http://x/readyz"}},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name:    "name contains slash",
			remotes: []federation.Remote{{Name: "a/b", URL: "http://x/readyz"}},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name:    "empty URL",
			remotes: []federation.Remote{{Name: "nas", URL: ""}},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name: "duplicate name",
			remotes: []federation.Remote{
				{Name: "nas", URL: "http://nas/readyz"},
				{Name: "nas", URL: "http://other/readyz"},
			},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name:    "unparseable URL",
			remotes: []federation.Remote{{Name: "nas", URL: "http://[::1"}},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name:    "non-http scheme",
			remotes: []federation.Remote{{Name: "nas", URL: "ftp://nas/readyz"}},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name:    "hostless URL",
			remotes: []federation.Remote{{Name: "nas", URL: "http:///readyz"}},
			wantErr: federation.ErrInvalidRemote,
		},
		{
			name:    "zero timeout",
			remotes: []federation.Remote{good},
			opts:    []federation.Option{federation.WithTimeout(0)},
			wantErr: federation.ErrInvalidTimeout,
		},
		{
			name:    "negative timeout",
			remotes: []federation.Remote{good},
			opts:    []federation.Option{federation.WithTimeout(-time.Second)},
			wantErr: federation.ErrInvalidTimeout,
		},
		{
			name:    "valid baseline",
			remotes: []federation.Remote{good},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prober, err := federation.New(tt.remotes, tt.opts...)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("New: unexpected error: %v", err)
				}

				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("New: want %v, got %v", tt.wantErr, err)
			}

			if prober != nil {
				t.Errorf("New: want nil prober on error, got %+v", prober)
			}
		})
	}
}

// TestRefreshInterval_IsZero pins the live-fetch contract: federation has
// no cadence of its own, so consumers set their own read cadence.
func TestRefreshInterval_IsZero(t *testing.T) {
	t.Parallel()

	fed := mustNew(t, []federation.Remote{{Name: "a", URL: "http://localhost:9101/readyz"}})

	if got := fed.RefreshInterval(); got != 0 {
		t.Errorf("RefreshInterval: want 0, got %v", got)
	}
}

// TestStartupComplete_LatchesOnFirstSuccess walks the one-way startup
// latch: a remote that has never answered keeps startup incomplete; one
// successful fetch latches it permanently — including across later
// failures.
func TestStartupComplete_LatchesOnFirstSuccess(t *testing.T) {
	t.Parallel()

	var healthy atomic.Bool

	healthy.Store(false)

	flaky := newRemote(t, func(w http.ResponseWriter, _ *http.Request) {
		if !healthy.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		_, _ = w.Write([]byte(`{"status":"pass","checks":{}}`))
	})

	fed := mustNew(t, []federation.Remote{{Name: "flaky", URL: flaky.server.URL}})

	_ = fed.CachedResponse()

	if fed.StartupComplete() {
		t.Fatal("a remote that has never answered must keep startup incomplete")
	}

	healthy.Store(true)

	_ = fed.CachedResponse()

	if !fed.StartupComplete() {
		t.Fatal("a successful fetch must latch startup complete")
	}

	healthy.Store(false)

	_ = fed.CachedResponse()

	if !fed.StartupComplete() {
		t.Fatal("startup latch is one-way: a later failure must not un-latch")
	}
}
