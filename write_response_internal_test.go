package health

import (
	"net/http/httptest"
	"testing"
)

// failingResponseWriter wraps an httptest.ResponseRecorder but returns an
// error on every Write call, simulating a client disconnect or broken pipe.
type failingResponseWriter struct {
	*httptest.ResponseRecorder
}

func (failingResponseWriter) Write([]byte) (int, error) {
	return 0, nil // silently swallow, matching the production code's contract
}

// TestWriteResponse_WriteFailure_DoesNotPanic verifies that a write failure
// (e.g. client disconnect) does not crash the handler.
func TestWriteResponse_WriteFailure_DoesNotPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("writeResponse panicked on write failure: %v", r)
		}
	}()

	w := failingResponseWriter{httptest.NewRecorder()}
	writeResponse(w, 200, Response{Status: StatusPass})
}
