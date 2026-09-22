package health

import (
	"sync"
	"time"
)

// trackedStatus is one check's current status and the time the probe first
// observed it in that status.
type trackedStatus struct {
	status Status
	since  time.Time
}

// transitionTracker remembers when each check last changed status so
// responses can answer "how long has it been like this?" ("failing since
// 14:02"). It observes every batch the probe builds — background refreshes,
// throttled and live evaluations, startup probes — via stamp.
//
// Since is probe-observed time, not service-reported: it is stamped from the
// probe's clock (see WithNowFunc) on the first batch reporting the current
// status, so it resets when the status changes and never predates the
// process. Checks absent from a batch are pruned; a returning check restarts
// its Since because the gap was unobserved.
//
// The zero value is ready to use. Safe for concurrent use: stamping
// serializes on a mutex, which is fine because it runs once per batch (the
// expensive part — the checks themselves — happens before stamping) and
// never on the cached-read path.
type transitionTracker struct {
	mu     sync.Mutex
	tracks map[string]trackedStatus

	// disabled short-circuits stamp for benchmark A/B runs only (see
	// BenchmarkEvaluate_TrackerDelta): production always stamps, so this
	// field is never set outside test builds.
	disabled bool
}

// stamp fills every check's Since field in place: a check whose status is
// unchanged keeps its first-seen time; a new or changed status is stamped
// now; names absent from checks are forgotten. now comes from the probe's
// clock seam so tests get deterministic transitions.
func (t *transitionTracker) stamp(checks map[string]Check, now time.Time) {
	if t.disabled {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	next := make(map[string]trackedStatus, len(checks))

	for name, check := range checks {
		track, unchanged := t.tracks[name]
		if !unchanged || track.status != check.Status {
			track = trackedStatus{status: check.Status, since: now}
		}

		next[name] = track
		check.Since = track.since
		checks[name] = check
	}

	t.tracks = next
}
