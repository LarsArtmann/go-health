package health_test

import (
	"context"
	"errors"
	"testing"
	"time"

	health "github.com/larsartmann/go-health"
	do "github.com/samber/do/v2"
)

// --- P31: shutdown grace period ---.

func TestWithShutdownGracePeriod_BlocksButStops(t *testing.T) {
	t.Parallel()

	const grace = 120 * time.Millisecond

	// Apply the option by building a second probe with the same injector.
	injector := do.New()

	provideHealthy(injector, "db")
	invoke[*healthyService](t, injector, "db")

	t.Cleanup(func() { injector.Shutdown() })

	graceProbe := health.New(injector,
		health.WithRefreshInterval(5*time.Millisecond),
		health.WithShutdownGracePeriod(grace),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := graceProbe.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	done := make(chan struct{})

	go func() {
		graceProbe.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Shutdown with grace period returned before the window elapsed")
	case <-time.After(grace / 2):
		// expected: still draining
	}

	<-done

	if graceProbe.Alive() {
		t.Error("Alive after graceful Shutdown: want false")
	}
}

// TestShutdown_GraceWindowSkippedWhenLifecycleNeverStarted pins the grace ×
// lifecycle interaction (plan A32): WithShutdownGracePeriod exists to drain a
// probe that serves, so Shutdown honors the window after any successful Start
// — even in live mode, where no refresh loop exists — and skips it entirely
// when Start was never called or failed validation. Blocking there would be
// dead time on a probe that never answered anything.
func TestShutdown_GraceWindowSkippedWhenLifecycleNeverStarted(t *testing.T) {
	t.Parallel()

	const grace = 10 * time.Second

	t.Run("never started returns immediately", func(t *testing.T) {
		t.Parallel()

		probe := health.NewWithHealthCheck(
			func(context.Context) map[string]error { return nil },
			health.WithShutdownGracePeriod(grace),
		)

		done := make(chan struct{})

		go func() {
			probe.Shutdown()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Shutdown slept the grace period although Start was never called")
		}
	})

	t.Run("failed Start returns immediately", func(t *testing.T) {
		t.Parallel()

		probe := health.NewWithHealthCheck(
			func(context.Context) map[string]error { return map[string]error{"ok": nil} },
			health.WithCriticalServices("ghost"),
			health.WithShutdownGracePeriod(grace),
		)

		if err := probe.Start(t.Context()); !errors.Is(err, health.ErrUnknownCriticalService) {
			t.Fatalf("Start = %v, want ErrUnknownCriticalService", err)
		}

		done := make(chan struct{})

		go func() {
			probe.Shutdown()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Shutdown slept the grace period although Start failed validation")
		}
	})

	t.Run("live mode still honors the window", func(t *testing.T) {
		t.Parallel()

		window := 120 * time.Millisecond

		probe := health.NewWithHealthCheck(
			func(context.Context) map[string]error { return map[string]error{"ok": nil} },
			health.WithRefreshInterval(0),
			health.WithShutdownGracePeriod(window),
		)

		if err := probe.Start(t.Context()); err != nil {
			t.Fatalf("Start: %v", err)
		}

		done := make(chan struct{})
		startedAt := time.Now()

		go func() {
			probe.Shutdown()
			close(done)
		}()

		select {
		case <-done:
			t.Fatal("live-mode Shutdown returned before the grace window elapsed")
		case <-time.After(window / 2):
			// expected: still draining, although no loop is armed
		}

		<-done

		if elapsed := time.Since(startedAt); elapsed < window {
			t.Fatalf(
				"live-mode Shutdown blocked for %v; want at least the %v window",
				elapsed,
				window,
			)
		}
	})
}
