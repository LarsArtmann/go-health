package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
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

// invalidUTF8 contains bytes (0xFF, 0xFE) that no valid UTF-8 stream can
// include, so any surviving byte proves a sanitizer skipped a field.
const invalidUTF8 = "pod-\xff\xfe"

// TestSanitizeResponse_CoercesInstanceID pins the v0.1.1 regression found by
// the marshal fuzz: the replica identifier set via WithInstanceID reached the
// marshal seam unsanitized, and encoding/json/v2 — unlike v1 — refuses
// invalid UTF-8, turning every health endpoint into a 500. Every string
// field must leave SanitizeResponse as valid UTF-8.
func TestSanitizeResponse_CoercesInstanceID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sanitized string
	}{
		{name: "instance_id", sanitized: SanitizeResponse(Response{
			Status:     StatusPass,
			InstanceID: invalidUTF8,
			Checks:     map[string]Check{},
		}).InstanceID},
		{name: "version", sanitized: SanitizeResponse(Response{
			Status:  StatusPass,
			Version: invalidUTF8,
			Checks:  map[string]Check{},
		}).Version},
		{name: "uptime", sanitized: SanitizeResponse(Response{
			Status: StatusPass,
			Uptime: invalidUTF8,
			Checks: map[string]Check{},
		}).Uptime},
		{name: "check_error", sanitized: SanitizeResponse(Response{
			Status: StatusPass,
			Checks: map[string]Check{"db": {Status: StatusPass, Error: invalidUTF8}},
		}).Checks["db"].Error},
		{name: "check_name", sanitized: func() string {
			checks := SanitizeResponse(Response{
				Status: StatusPass,
				Checks: map[string]Check{invalidUTF8: {Status: StatusPass}},
			}).Checks

			for name := range checks {
				return name
			}

			return ""
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !utf8.ValidString(tt.sanitized) {
				t.Fatalf("sanitized %q is not valid UTF-8", tt.sanitized)
			}

			if tt.sanitized != "pod-\uFFFD" {
				t.Fatalf(
					"sanitized = %q, want %q (one U+FFFD per invalid run)",
					tt.sanitized,
					"pod-\uFFFD",
				)
			}
		})
	}
}

// TestWriteResponse_InvalidUTF8InstanceIDDoesNot500 proves the served path of
// the same regression end to end: before the fix, this input failed the
// marshal and the handler returned 500.
func TestWriteResponse_InvalidUTF8InstanceIDDoesNot500(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	writeResponse(rec, http.StatusOK, Response{
		Status:     StatusPass,
		InstanceID: invalidUTF8,
		Checks:     map[string]Check{},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}

	if !utf8.Valid(rec.Body.Bytes()) {
		t.Fatalf("body is not valid UTF-8: %q", rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"instance_id"`) {
		t.Fatalf("body %q missing instance_id", rec.Body.String())
	}
}
