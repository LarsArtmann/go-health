package health_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/larsartmann/go-health"
)

func TestVersionHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		version  string
		method   string
		wantCode int
		wantBody string
	}{
		{
			name:     "serves stamped version",
			version:  "v1.2.3",
			method:   http.MethodGet,
			wantCode: http.StatusOK,
			wantBody: `{"version":"v1.2.3"}`,
		},
		{
			name:     "unstamped binary serves empty string",
			version:  "",
			method:   http.MethodGet,
			wantCode: http.StatusOK,
			wantBody: `{"version":""}`,
		},
		{
			name:     "invalid UTF-8 is coerced at the write seam",
			version:  "v1\xFF.0",
			method:   http.MethodGet,
			wantCode: http.StatusOK,
			wantBody: "{\"version\":\"v1\uFFFD.0\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			health.VersionHandler(tt.version).
				ServeHTTP(rec, httptest.NewRequest(tt.method, "/version", nil))

			if rec.Code != tt.wantCode {
				t.Errorf("code: want %d, got %d", tt.wantCode, rec.Code)
			}

			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body: want %s, got %s", tt.wantBody, got)
			}

			if tt.wantCode != http.StatusOK {
				return
			}

			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type: want application/json, got %s", ct)
			}

			if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
				t.Errorf("Cache-Control: want no-cache, got %s", cc)
			}

			if !utf8.ValidString(rec.Body.String()) {
				t.Errorf("body is not valid UTF-8: %q", rec.Body.String())
			}
		})
	}
}

func TestVersionHandler_CoercedUTF8SurvivesJSONv2(t *testing.T) {
	t.Parallel()

	// encoding/json/v2 refuses to marshal invalid UTF-8; the handler must
	// never turn that into a 500 the way an unsanitized health response
	// would (the fuzz-found class SanitizeResponse guards for).
	rec := httptest.NewRecorder()
	health.VersionHandler("\x80bad").
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/version", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("code: want 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "\uFFFD") {
		t.Errorf("body lacks U+FFFD replacement: %q", rec.Body.String())
	}
}

func TestVersionHandler_RejectsNonGET(t *testing.T) {
	t.Parallel()

	methods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodHead,
		http.MethodPatch,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			health.VersionHandler("v1.2.3").
				ServeHTTP(rec, httptest.NewRequest(method, "/version", nil))

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("code: want 405, got %d", rec.Code)
			}

			if allow := rec.Header().Get("Allow"); allow != http.MethodGet {
				t.Errorf("Allow: want GET, got %q", allow)
			}

			if body := rec.Body.String(); !strings.Contains(
				body,
				"version endpoint only accepts GET",
			) {
				t.Errorf("body: unexpected message %q", body)
			}
		})
	}
}
