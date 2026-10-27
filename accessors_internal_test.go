package health

import "testing"

// TestApplyStartupFailureOverlay pins the overlay [Probe.Healthz] applies to
// its readiness read: the unlatched startup turn becomes a synthetic failing
// "startup" row plus a fail roll-up, an explained fail is never rewritten, and
// the nil-map guard holds. The guard is unreachable through today's read path
// (buildChecks always allocates), so testing it directly is the only way to
// prove the defensive branch works.
func TestApplyStartupFailureOverlay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		resp            Response
		startupComplete bool
		wantStatus      Status
		wantStartupRow  bool
		wantChecksKept  []string
	}{
		{
			name: "open latch stamps the failure",
			resp: Response{
				Status: StatusPass,
				Checks: map[string]Check{"db": {Status: StatusPass}},
			},
			startupComplete: false,
			wantStatus:      StatusFail,
			wantStartupRow:  true,
			wantChecksKept:  []string{"db"},
		},
		{
			name:            "open latch tolerates a nil checks map",
			resp:            Response{Status: StatusWarn},
			startupComplete: false,
			wantStatus:      StatusFail,
			wantStartupRow:  true,
		},
		{
			name:            "closed latch is a no-op",
			resp:            Response{Status: StatusPass, Checks: map[string]Check{}},
			startupComplete: true,
			wantStatus:      StatusPass,
			wantStartupRow:  false,
		},
		{
			name: "explained fail keeps its cause",
			resp: Response{
				Status: StatusFail,
				Checks: map[string]Check{"db": {Status: StatusFail, Error: "connection refused"}},
			},
			startupComplete: false,
			wantStatus:      StatusFail,
			wantStartupRow:  false,
			wantChecksKept:  []string{"db"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := applyStartupFailureOverlay(tt.resp, tt.startupComplete)

			if got.Status != tt.wantStatus {
				t.Errorf("status: want %q, got %q", tt.wantStatus, got.Status)
			}

			startupRow, hasStartupRow := got.Checks["startup"]
			if hasStartupRow != tt.wantStartupRow {
				t.Fatalf("startup row present = %t, want %t", hasStartupRow, tt.wantStartupRow)
			}

			if tt.wantStartupRow {
				if startupRow.Status != StatusFail {
					t.Errorf("startup row status: want %q, got %q", StatusFail, startupRow.Status)
				}

				if startupRow.Error != "startup latch not set" {
					t.Errorf("startup row error: got %q", startupRow.Error)
				}
			}

			for _, name := range tt.wantChecksKept {
				if _, kept := got.Checks[name]; !kept {
					t.Errorf("check %q was dropped by the overlay", name)
				}
			}
		})
	}
}
