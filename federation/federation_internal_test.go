package federation

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

// TestWriteResponse_MarshalError forces the defensive encode-failure
// branch: the client gets a plain-text 500 carrying the underlying cause,
// never a half-written JSON body with a committed health status.
//
//nolint:paralleltest // swaps the package marshal seam, not parallel-safe
func TestWriteResponse_MarshalError(t *testing.T) {
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

	if body := w.Body.String(); !strings.Contains(body, "federation: failed to encode response") ||
		!strings.Contains(body, "boom") {
		t.Errorf("marshal error body must carry the cause, got: %s", body)
	}
}
