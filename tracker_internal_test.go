package health

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkTransitionTrackerStamp isolates the Since-stamping cost that every
// evaluation batch pays inside buildChecks: a fresh tracked map plus one
// mutex-guarded pass over the checks. Measured warm (statuses hold across
// iterations), which is the common case — transitions themselves are rare and
// only add a struct allocation per changed check.
//
// Recorded baseline (2026-09-18, go1.26.7 linux/amd64, 32 threads): see
// FEATURES.md "Performance".
func BenchmarkTransitionTrackerStamp(b *testing.B) {
	for _, checkCount := range []int{1, 8, 64} {
		b.Run(fmt.Sprintf("checks=%d", checkCount), func(b *testing.B) {
			now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

			checks := make(map[string]Check, checkCount)
			for i := range checkCount {
				checks[fmt.Sprintf("svc%02d", i)] = Check{Status: StatusPass}
			}

			var tracker transitionTracker

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				tracker.stamp(checks, now)
			}
		})
	}
}
