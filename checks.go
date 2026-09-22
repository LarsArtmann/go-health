package health

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CheckFunc runs one named health check. It receives the batch context, which
// carries the probe deadline ([WithTimeout], applied by every handler path
// and by the background refresh); long-running checks should honor it.
// Returning nil means healthy.
type CheckFunc func(ctx context.Context) error

// NewChecks creates a [Probe] from plain named checks — no samber/do injector
// involved. It is the ergonomic spelling of [NewWithHealthCheck] for the
// common case of "a handful of named lambdas":
//
//	probe := health.NewChecks(map[string]health.CheckFunc{
//	    "sqlite":   func(ctx context.Context) error { return db.PingContext(ctx) },
//	    "blob-dir": probeBlobDir,
//	}, health.WithCriticalServices("sqlite"))
//
// Semantics, all inherited from the [New] machinery:
//
//   - Checks run concurrently on every batch; each is timed and the duration
//     surfaces as [Check].DurationNanos.
//   - A panicking check is recovered into that check's error — one bad check
//     cannot take down the batch or the process.
//   - A check that ignores its context is abandoned at the batch deadline with
//     a fail-closed error naming it; the probe never hangs on a wedged check.
//     The deadline is whatever the batch context carries — handlers and
//     [Probe.Start] apply [WithTimeout] automatically, so only callers invoking
//     [Probe.Evaluate] directly must bound their context themselves.
//   - Names are graded against [WithCriticalServices] exactly like injector
//     services; an empty map yields a permanently-passing batch (the same
//     contract as a readiness handler with no checks).
//   - [WithHealthRecorder] has no effect here: the given checks own batch
//     execution. All other options apply normally.
func NewChecks(checks map[string]CheckFunc, opts ...Option) *Probe {
	cfg := buildStandaloneConfig(opts)

	runChecks := runNamedChecks(checks)

	return assemble(func(ctx context.Context) map[string]CheckDetail { return runChecks(ctx) }, cfg)
}

// runNamedChecks adapts a named-check map into the executor seam
// ([DetailedHealthCheckFunc]) the probe assembles around. One goroutine per
// check per batch; results are collected under a mutex.
func runNamedChecks(checks map[string]CheckFunc) DetailedHealthCheckFunc {
	return func(ctx context.Context) map[string]CheckDetail {
		var (
			resultsMu sync.Mutex
			wg        sync.WaitGroup
			out       = make(map[string]CheckDetail, len(checks))
		)

		for name, check := range checks {
			wg.Add(1)

			go func(name string, check CheckFunc) {
				defer wg.Done()

				start := time.Now()
				err := runBoundedCheck(name, check, ctx)

				resultsMu.Lock()
				out[name] = CheckDetail{Err: err, Duration: time.Since(start)}
				resultsMu.Unlock()
			}(name, check)
		}

		wg.Wait()

		return out
	}
}

// ErrNilCheck is wrapped into the fail-closed error reported when a
// [NewChecks] map carries a nil [CheckFunc]. Match with errors.Is.
var ErrNilCheck = errors.New("health: check is nil")

// ErrCheckPanicked is wrapped into the per-check error when a [NewChecks]
// check panics and the panic is recovered. Match with errors.Is: like the
// batch-level [ErrPanicDuringHealthCheck], a recovered panic must read as a
// failure, never a warning.
var ErrCheckPanicked = errors.New("health: check panicked")

// runBoundedCheck runs one check and stays honest under every failure mode:
// the check's own error is returned as-is, a panic is recovered into an error
// naming the check, and a check that ignores its context is abandoned at the
// batch deadline with a fail-closed error. The abandoned goroutine leaks until
// the check returns; done is buffered so its late result can never block.
func runBoundedCheck(name string, check CheckFunc, ctx context.Context) error {
	if check == nil {
		return fmt.Errorf("%w: %q", ErrNilCheck, name)
	}

	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("%w: %q: %v", ErrCheckPanicked, name, r)
			}
		}()

		done <- check(ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf(
			"health: check %q did not finish before the batch deadline: %w",
			name,
			ctx.Err(),
		)
	}
}
