package health

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Start validates the Probe configuration and, if valid, launches the
// background cache refresh loop (when RefreshInterval > 0) and performs an
// immediate evaluation so the cache is populated before the first request
// arrives. Calling Start more than once is a no-op.
//
// Returns [ErrInvalidTimeout], [ErrInvalidRefreshInterval], or
// [ErrUnknownCriticalService] if the configuration is unusable. Call
// [Probe.Validate] separately to check configuration before starting.
//
// The provided ctx controls the lifetime of the background goroutine. Call
// [Probe.Shutdown] to stop the loop and mark the probe as shutting down.
//
// Shutdown is one-way, and Start does not undo it: calling Start on a shut
// down probe re-validates and may re-arm the refresh loop, but the draining
// flag stays set, so readiness keeps answering 503. This is deliberate — a
// process that announced it is leaving rotation must not be resurrected by a
// stray restart call (a supervisor that wants the service back starts a new
// process). The supported retry is a Start that FAILED validation: that path
// never set the flag, so retrying it recovers. Pinned by
// TestStart_AfterShutdown_RestartsLoopButStaysDown; the decision and its
// rejected alternatives are docs/adr/ADR-009-shutdown-is-one-way.md.
func (p *Probe) Start(ctx context.Context) error {
	if err := p.Validate(); err != nil {
		return err
	}

	p.mu.Lock()

	if p.cancel != nil {
		p.mu.Unlock()

		return nil
	}

	runCtx := ctx

	if p.refreshInterval > 0 {
		runCtx, p.cancel = context.WithCancel(ctx)

		// Register with the WaitGroup in the same critical section that
		// publishes cancel: Shutdown swaps cancel under the lock before it
		// waits, so a concurrent Start can never slip an Add past a Wait.
		p.wg.Add(1)
	}

	p.mu.Unlock()

	evalCtx, cancelEval := context.WithTimeout(ctx, p.timeout)

	resp := p.Evaluate(evalCtx)

	cancelEval()

	if err := p.validateCriticalNames(resp); err != nil {
		// The loop was armed before the initial evaluation (the same lock
		// section doubles as the Start-idempotence marker). Disarm it, or a
		// failed Start leaves a WaitGroup counter that Shutdown would wait
		// on forever and a cancel that makes every retry-Start silently
		// no-op on a probe that never ran.
		if p.refreshInterval > 0 {
			p.mu.Lock()

			if p.cancel != nil {
				p.cancel()
				p.cancel = nil
				p.wg.Done()
			}

			p.mu.Unlock()
		}

		return err
	}

	p.latest.Store(&resp)
	p.lifecycleStarted.Store(true)

	if p.refreshInterval > 0 {
		go p.refreshLoop(runCtx)
	}

	return nil
}

// validateCriticalNames rejects critical names absent from the initial
// health-check batch: such a name can never influence classification and the
// startup latch can never set (docs/start-validation-design.md).
func (p *Probe) validateCriticalNames(resp Response) error {
	if len(p.rollups.critical) == 0 {
		return nil
	}

	unknown := make([]string, 0)

	for name := range p.rollups.critical {
		if _, ok := resp.Checks[name]; !ok {
			unknown = append(unknown, name)
		}
	}

	if len(unknown) == 0 {
		return nil
	}

	sort.Strings(unknown)

	return fmt.Errorf(
		"%w: %s (no check ran under this name; fix WithCriticalServices or the check registration)",
		ErrUnknownCriticalService,
		strings.Join(unknown, ", "),
	)
}

// refreshLoop runs the periodic cache refresh until the start context is cancelled.
func (p *Probe) refreshLoop(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.refreshCache(ctx)
		}
	}
}

// refreshCache evaluates health checks with a timeout-bounded context and
// stores the result atomically for cached handlers.
func (p *Probe) refreshCache(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	resp := p.Evaluate(ctx)
	p.latest.Store(&resp)
}

// Shutdown marks the probe as shutting down and stops the background refresh
// loop if one is running. After Shutdown:
//
//   - Liveness continues to return 200 (the process is still alive).
//   - Readiness returns 503 so load balancers drain traffic.
//   - Startup returns its latched value (200 if it had previously passed).
//
// With [WithShutdownGracePeriod], the grace window keeps a running refresh
// loop serving fresh 503s while load balancers drain. It is honored whenever
// the probe's lifecycle started successfully (cached or live mode alike: even
// without a loop, the block is the caller's drain window before teardown) and
// skipped when it never did (Start not called, or failed validation): there
// is no lifecycle to drain, and blocking for the window would be dead time.
//
// Shutdown is one-way: the draining flag is never cleared, so a later
// [Probe.Start] may re-arm the loop but cannot make the probe ready again.
// Call it once from the host's teardown path; repeat calls are harmless but
// re-run the grace window. For a two-phase drain use
// [Probe.MarkShuttingDown] first. See docs/adr/ADR-009-shutdown-is-one-way.md.
func (p *Probe) Shutdown() {
	p.shuttingDown.Store(true)

	// Grace window: with WithShutdownGracePeriod, keep the refresh loop
	// running so cached readiness responses stay fresh 503s while load
	// balancers drain. The draining flag is already set, so the loop only
	// observes; it cannot resurrect readiness. Checking lifecycleStarted
	// (not the armed cancel) also honors the window in live mode, where a
	// successfully started probe has no loop but still needs its drain
	// time; only never-started and failed-Start probes skip it.
	if p.lifecycleStarted.Load() {
		if grace := p.shutdownGrace; grace > 0 {
			time.Sleep(grace)
		}
	}

	// Add (in Start) and Wait are serialized under p.mu on purpose: the
	// WaitGroup contract forbids an Add from a zero counter running
	// concurrently with a Wait, which concurrent Start/Shutdown callers
	// would otherwise trigger ("WaitGroup is reused before previous Wait
	// has returned"). refreshLoop does not take p.mu, so waiting under the
	// lock cannot deadlock the loop we are waiting for. Capturing cancel
	// after the grace window also means a Start that re-armed the loop
	// during the drain gets stopped by this same Shutdown call.
	p.mu.Lock()
	cancel := p.cancel
	p.cancel = nil

	if cancel != nil {
		cancel()
		p.wg.Wait()
	}

	p.mu.Unlock()
}

// MarkShuttingDown flips the shutdown flag without stopping the background
// loop. Use this for a two-phase graceful shutdown: mark first so load
// balancers start draining, then call [Probe.Shutdown] after a grace period
// to stop the refresh loop.
func (p *Probe) MarkShuttingDown() {
	p.shuttingDown.Store(true)
}
