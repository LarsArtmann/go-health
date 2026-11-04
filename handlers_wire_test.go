package health_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Content-Type and format tests ---.

func TestResponse_ContentTypeIsJSON(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector)

	w := doRequest(t, probe.LivenessHandler(), "/healthz")

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("content-type: want application/json, got %s", ct)
	}
}

func TestResponse_NoCacheHeader(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector)

	w := doRequest(t, probe.LivenessHandler(), "/healthz")

	cc := w.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("cache-control: want no-cache, got %s", cc)
	}
}

func TestReadiness_JSONChecksAreSortedAlphabetically(t *testing.T) {
	t.Parallel()

	injector := do.New()
	provideUnhealthy(injector, "zebra", "down")
	provideHealthy(injector, "alpha")
	provideUnhealthy(injector, "mongo", "slow")
	invoke[*unhealthyService](t, injector, "zebra")
	invoke[*healthyService](t, injector, "alpha")
	invoke[*unhealthyService](t, injector, "mongo")

	probe := health.New(injector, health.WithRefreshInterval(0))

	// Verify alphabetical ordering: "alpha" must appear before "mongo" before
	// "zebra" in the raw JSON body. encoding/json/v2 serializes map keys in
	// random Go map order unless json.Deterministic(true) is passed, so this
	// test locks the property that writeResponse opted into.
	body := doRequest(t, probe.ReadinessHandler(), "/readyz").Body.String()

	indices := make([]int, 0, 3)

	for _, name := range []string{"alpha", "mongo", "zebra"} {
		idx := strings.Index(body, `"`+name+`"`)
		if idx < 0 {
			t.Fatalf("service %q missing from JSON output: %s", name, body)
		}

		indices = append(indices, idx)
	}

	if indices[0] >= indices[1] || indices[1] >= indices[2] {
		t.Errorf(
			"checks are not alphabetically sorted in JSON: positions alpha=%d, mongo=%d, zebra=%d",
			indices[0],
			indices[1],
			indices[2],
		)
	}
}

func TestDefault_AllowsNonGETWithoutGuard(t *testing.T) {
	t.Parallel()

	injector := do.New()
	probe := health.New(injector, health.WithRefreshInterval(0))

	w := httptest.NewRecorder()

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	probe.LivenessHandler()(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("POST /healthz without GETOnly: want 200 (no guard), got %d", w.Code)
	}
}
