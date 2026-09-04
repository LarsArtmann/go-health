package aggregate

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	health "github.com/larsartmann/go-health"
)

// errMarshalBoom is the canned marshal failure for the seam test.
var errMarshalBoom = errors.New("boom")

// TestStatusRank pins the worst-of merge order, including the contract that
// unknown statuses rank as healthy: the Status type is validated at the
// boundaries, so an unknown value arriving here must not fail an aggregate.
func TestStatusRank(t *testing.T) {
	t.Parallel()

	for status, want := range map[health.Status]int{
		health.StatusFail: 0,
		health.StatusWarn: 1,
		health.StatusPass: 2,
		"degraded":        2,
		"":                2,
	} {
		if got := statusRank(status); got != want {
			t.Errorf("statusRank(%q): want %d, got %d", status, want, got)
		}
	}
}

// TestWriteResponse_MarshalError forces the defensive encode-failure branch:
// the client gets a plain-text 500 carrying the underlying cause, never a
// half-written JSON body with a committed health status.
func TestWriteResponse_MarshalError(t *testing.T) {
	t.Parallel()

	original := marshalResponse

	t.Cleanup(func() { marshalResponse = original })

	marshalResponse = func(health.Response) ([]byte, error) {
		return nil, errMarshalBoom
	}

	w := httptest.NewRecorder()

	writeResponse(w, http.StatusOK, health.Response{
		Status: health.StatusPass,
		Checks: map[string]health.Check{},
	})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("marshal error: want 500, got %d", w.Code)
	}

	if body := w.Body.String(); !strings.Contains(body, "aggregate: failed to encode response") ||
		!strings.Contains(body, "boom") {
		t.Errorf("marshal error body must carry the cause, got: %s", body)
	}
}
