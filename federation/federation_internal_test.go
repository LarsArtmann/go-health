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

// TestApplyStartupLatchOverlay pins the single-endpoint startup overlay: while
// the latch is open the body gains one failing "startup" row and the roll-up
// becomes fail; a fail the merge already produced is never rewritten, so the
// cause the merge named survives; and a nil checks map is tolerated rather than
// panicked on. The overlay is unreachable through the public API (any
// successful fetch closes the latch before the merge, any failed fetch fails
// the roll-up), so this test is the contract's only direct pin.
func TestApplyStartupLatchOverlay(t *testing.T) {
	t.Parallel()

	t.Run("open latch adds the startup row", func(t *testing.T) {
		t.Parallel()

		got := applyStartupLatchOverlay(health.Response{
			Status: health.StatusPass,
			Checks: map[string]health.Check{"a/db": {Status: health.StatusPass}},
		}, false)

		if got.Status != health.StatusFail {
			t.Errorf("status = %q, want fail", got.Status)
		}

		startup, ok := got.Checks["startup"]
		if !ok {
			t.Fatalf("startup row missing, checks = %v", got.Checks)
		}

		if startup.Status != health.StatusFail {
			t.Errorf("startup row status = %q, want fail", startup.Status)
		}

		if got.Checks["a/db"].Status != health.StatusPass {
			t.Error("overlay dropped or rewrote an existing check")
		}
	})

	t.Run("open latch tolerates a nil checks map", func(t *testing.T) {
		t.Parallel()

		got := applyStartupLatchOverlay(health.Response{Status: health.StatusWarn}, false)

		if got.Status != health.StatusFail {
			t.Errorf("status = %q, want fail", got.Status)
		}

		if got.Checks["startup"].Status != health.StatusFail {
			t.Errorf("startup row status = %q, want fail", got.Checks["startup"].Status)
		}
	})

	t.Run("existing fail keeps its cause", func(t *testing.T) {
		t.Parallel()

		original := health.Response{
			Status: health.StatusFail,
			Checks: map[string]health.Check{
				"nas/reachable": {Status: health.StatusFail, Error: "connection refused"},
			},
		}

		got := applyStartupLatchOverlay(original, false)

		if _, added := got.Checks["startup"]; added {
			t.Error("overlay invented a startup row over a reachable fail")
		}

		if got.Checks["nas/reachable"].Error != "connection refused" {
			t.Errorf("reachable cause rewritten: %q", got.Checks["nas/reachable"].Error)
		}
	})

	t.Run("closed latch is a no-op", func(t *testing.T) {
		t.Parallel()

		original := health.Response{
			Status: health.StatusWarn,
			Checks: map[string]health.Check{"a/q": {Status: health.StatusWarn}},
		}

		got := applyStartupLatchOverlay(original, true)

		if got.Status != health.StatusWarn {
			t.Errorf("status = %q, want warn", got.Status)
		}

		if _, added := got.Checks["startup"]; added {
			t.Error("overlay ran with the latch closed")
		}
	})
}
