package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestVersionMarshalFailure pins the defensive 500 [VersionHandler] falls back
// to if the version payload ever becomes unmarshalable. Today's single
// UTF-8-coerced string field cannot fail, so the branch is unreachable through
// the public API; answering with a body that names the cause is what makes a
// future regression debuggable instead of a silent empty 200.
func TestVersionMarshalFailure(t *testing.T) {
	t.Parallel()

	cause := errors.New("json: unsupported type")
	rec := httptest.NewRecorder()

	versionMarshalFailure(cause).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/version", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "failed to encode version") {
		t.Errorf("body %q missing the encode-failure prefix", body)
	}

	if !strings.Contains(body, cause.Error()) {
		t.Errorf("body %q missing the underlying cause", body)
	}
}
