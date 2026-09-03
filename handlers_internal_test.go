package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWriteResponse_MarshalErrorIncludesCause forces the defensive marshal
// error branch and asserts the body carries the underlying cause, so a
// future serialization regression is debuggable from the response alone.
// Not parallel: it swaps the package-level marshal seam.
func TestWriteResponse_MarshalErrorIncludesCause(t *testing.T) {
	original := marshalResponse
	marshalResponse = func(Response) ([]byte, error) {
		return nil, errors.New("boom: unsupported type")
	}

	t.Cleanup(func() { marshalResponse = original })

	rec := httptest.NewRecorder()
	writeResponse(rec, http.StatusOK, Response{Status: StatusPass, Checks: map[string]Check{}})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", rec.Code)
	}

	body := rec.Body.String()

	if !strings.Contains(body, "failed to encode response") {
		t.Errorf("body %q missing the encode-failure prefix", body)
	}

	if !strings.Contains(body, "boom: unsupported type") {
		t.Errorf("body %q missing the underlying cause", body)
	}
}
