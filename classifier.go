package health

import (
	"errors"
)

// classifier rolls raw health-check results into the library's decisions:
// the three-state roll-up status and the startup-latch verdict. It holds the
// critical set and nothing else — no lifecycle state, no synchronization —
// so every method is a pure function of its inputs and safe for concurrent
// use.
type classifier struct {
	critical map[string]struct{}
}

// classify computes the roll-up status from health-check results:
//
//   - StatusFail when shutting down, any critical service failed, or the
//     batch panicked (recovered panic: results are untrustworthy — see
//     docs/panic-recovery-design.md).
//   - StatusWarn when no critical service failed but at least one
//     non-critical service failed (degraded but still serving traffic).
//   - StatusPass when every checked service is healthy.
func (c classifier) classify(results map[string]error, shuttingDown bool) Status {
	if shuttingDown {
		return StatusFail
	}

	hasWarning := false

	for name, err := range results {
		if err == nil {
			continue
		}

		if errors.Is(err, ErrPanicDuringHealthCheck) {
			return StatusFail
		}

		if _, critical := c.critical[name]; critical {
			return StatusFail
		}

		hasWarning = true
	}

	if hasWarning {
		return StatusWarn
	}

	return StatusPass
}

// evaluateStartup checks whether all critical services are present and
// healthy in the results map. Returns true if the startup latch should be
// set. With no critical set the latch flips on the first batch.
func (c classifier) evaluateStartup(results map[string]error) bool {
	if len(c.critical) == 0 {
		return true
	}

	for name := range c.critical {
		err, found := results[name]

		if !found || err != nil {
			return false
		}
	}

	return true
}

// grades maps a single failing result to its per-check severity: a failing
// critical service is fail, a failing non-critical service is warn, and a
// recovered panic is always fail regardless of criticality.
func (c classifier) grades(name string, err error) Status {
	if errors.Is(err, ErrPanicDuringHealthCheck) {
		return StatusFail
	}

	if _, critical := c.critical[name]; critical {
		return StatusFail
	}

	return StatusWarn
}
