package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// errVersionMarshalCause is a package-level sentinel so the dynamic-error
// linter stays satisfied while the test simulates a payload that cannot be
// marshaled.
var errVersionMarshalCause = errors.New("json: unsupported type")

// TestVersionHandler_MarshalErrorIncludesCause forces the defensive branch
// [VersionHandler] takes if the payload ever becomes unmarshalable, and
// asserts the body names the cause: a version endpoint that answered an empty
// 200 would report a build identity nobody can trust. Not parallel: it swaps
// the package-level marshal seam.
//
//nolint:paralleltest // swaps the package marshal seam, not parallel-safe
func TestVersionHandler_MarshalErrorIncludesCause(t *testing.T) {
	original := marshalVersionResponse
	marshalVersionResponse = func(versionResponse) ([]byte, error) {
		return nil, errVersionMarshalCause
	}

	t.Cleanup(func() { marshalVersionResponse = original })

	rec := httptest.NewRecorder()
	VersionHandler("v1.2.3").
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/version", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "failed to encode version") {
		t.Errorf("body %q missing the encode-failure prefix", body)
	}

	if !strings.Contains(body, errVersionMarshalCause.Error()) {
		t.Errorf("body %q missing the underlying cause", body)
	}
}
