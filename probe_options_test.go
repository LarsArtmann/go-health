package health_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- WithNowFunc tests ---.

func TestWithNowFunc_UptimeIsDeterministic(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	boot := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := boot.Add(90 * time.Second)

	probe := health.New(injector,
		health.WithBootTime(boot),
		health.WithNowFunc(func() time.Time { return now }),
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if want := "1m30s"; resp.Uptime != want {
		t.Errorf("uptime: want %q, got %q", want, resp.Uptime)
	}

	if !resp.Timestamp.Equal(now) {
		t.Errorf("timestamp: want %v, got %v", now, resp.Timestamp)
	}
}

func TestWithNowFunc_UptimeTracksClockAdvance(t *testing.T) {
	t.Parallel()

	injector := do.New()

	boot := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	current := boot

	probe := health.New(injector,
		health.WithBootTime(boot),
		health.WithNowFunc(func() time.Time { return current }),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())

	if want := "0s"; resp.Uptime != want {
		t.Errorf("uptime at boot: want %q, got %q", want, resp.Uptime)
	}

	current = boot.Add(2 * time.Hour)

	resp = probe.Evaluate(context.Background())

	if want := "2h0m0s"; resp.Uptime != want {
		t.Errorf("uptime after advance: want %q, got %q", want, resp.Uptime)
	}
}

func TestWithNowFunc_DefaultClockUsedWhenUnset(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	before := time.Now()

	resp := probe.Evaluate(context.Background())

	if resp.Timestamp.Before(before) {
		t.Errorf("timestamp %v predates evaluation start %v", resp.Timestamp, before)
	}
}

// --- WithAllowedMethods tests ---.

func TestWithAllowedMethods_AllowsListedAndRejectsOthers(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")
	probe := health.New(injector,
		health.WithAllowedMethods(http.MethodHead),
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	handlers := map[string]http.HandlerFunc{
		"/healthz":  probe.LivenessHandler(),
		"/readyz":   probe.ReadinessHandler(),
		"/startupz": probe.StartupHandler(),
	}

	for path, handler := range handlers {
		w := doRequest(t, handler, path)
		if w.Code != http.StatusOK {
			t.Errorf("GET %s: want 200 (GET always allowed), got %d", path, w.Code)
		}

		headW := request(t, handler, path, http.MethodHead)
		if headW.Code != http.StatusOK {
			t.Errorf("HEAD %s: want 200 (explicitly allowed), got %d", path, headW.Code)
		}

		delW := request(t, handler, path, http.MethodDelete)
		if delW.Code != http.StatusMethodNotAllowed {
			t.Errorf("DELETE %s: want 405, got %d", path, delW.Code)
		}
	}
}

func TestWithAllowedMethods_AllowHeaderSorted(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector,
		health.WithAllowedMethods(http.MethodOptions, http.MethodHead),
		health.WithRefreshInterval(0),
	)

	w := request(t, probe.LivenessHandler(), "/healthz", http.MethodPost)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /healthz: want 405, got %d", w.Code)
	}

	if allow := w.Header().Get("Allow"); allow != "GET, HEAD, OPTIONS" {
		t.Errorf("Allow header: want %q (sorted, GET implied), got %q", "GET, HEAD, OPTIONS", allow)
	}
}

func TestWithAllowedMethods_DuplicateMethodNotDuplicated(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector,
		health.WithAllowedMethods(http.MethodHead),
		health.WithAllowedMethods(http.MethodHead),
		health.WithRefreshInterval(0),
	)

	w := request(t, probe.LivenessHandler(), "/healthz", http.MethodPost)

	if allow := w.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Errorf("Allow header: want %q (no duplicates across calls), got %q", "GET, HEAD", allow)
	}
}

func TestWithAllowedMethods_EmptyActsAsGETOnly(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector,
		health.WithAllowedMethods(),
		health.WithRefreshInterval(0),
	)

	w := doRequest(t, probe.LivenessHandler(), "/healthz")
	if w.Code != http.StatusOK {
		t.Errorf("GET /healthz: want 200, got %d", w.Code)
	}

	postW := request(t, probe.LivenessHandler(), "/healthz", http.MethodPost)
	if postW.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /healthz: want 405, got %d", postW.Code)
	}

	if allow := postW.Header().Get("Allow"); allow != "GET" {
		t.Errorf("Allow header: want GET, got %s", allow)
	}
}

// --- WithInstanceID tests ---.

func TestWithInstanceID_AllHandlersCarryReplicaID(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")
	probe := health.New(injector,
		health.WithInstanceID("pod-abc"),
		health.WithCriticalServices("db"),
		health.WithRefreshInterval(0),
	)

	resp := probe.Evaluate(context.Background())
	if resp.InstanceID != "pod-abc" {
		t.Errorf("Evaluate instance ID: want pod-abc, got %q", resp.InstanceID)
	}

	for path, handler := range map[string]http.HandlerFunc{
		"/healthz":  probe.LivenessHandler(),
		"/readyz":   probe.ReadinessHandler(),
		"/startupz": probe.StartupHandler(),
	} {
		got := decodeResponse(t, doRequest(t, handler, path))

		if got.InstanceID != "pod-abc" {
			t.Errorf("%s instance ID: want pod-abc, got %q", path, got.InstanceID)
		}
	}
}

func TestWithInstanceID_OmittedFromJSONWhenUnset(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	payload, err := json.Marshal(probe.Evaluate(context.Background()), json.Deterministic(true))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(payload), "instance_id") {
		t.Errorf("instance_id should be omitted when unset, got %s", payload)
	}
}

func request(
	t *testing.T,
	handler http.HandlerFunc,
	target, method string,
) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()

	r, err := http.NewRequestWithContext(t.Context(), method, target, nil)
	if err != nil {
		t.Fatal(err)
	}

	handler(w, r)

	return w
}
