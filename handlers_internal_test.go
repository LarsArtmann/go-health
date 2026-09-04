package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// errMarshalTestCause is a package-level sentinel so the dynamic-error
// linter stays satisfied while the test simulates a marshal failure.
var errMarshalTestCause = errors.New("boom: unsupported type")

// TestWriteResponse_MarshalErrorIncludesCause forces the defensive marshal
// error branch and asserts the body carries the underlying cause, so a
// future serialization regression is debuggable from the response alone.
// Not parallel: it swaps the package-level marshal seam.
//
//nolint:paralleltest // swaps the package marshal seam, not parallel-safe
func TestWriteResponse_MarshalErrorIncludesCause(t *testing.T) {
	original := marshalResponse
	marshalResponse = func(Response) ([]byte, error) {
		return nil, errMarshalTestCause
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
