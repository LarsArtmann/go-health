package federation_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	health "github.com/larsartmann/go-health"
	"github.com/larsartmann/go-health/federation"
)

// A remote whose WithCriticalServices name never runs in a batch fails its
// own Start with ErrUnknownCriticalService (the boot contract is the remote
// process's). The federation never sees that error: remote-side validation is
// a different universe from fetch-side names
// (docs/federation-validation-semantics.md). This test pins the composition:
//
//   - the Prober constructs fine over both remotes (validation never leaks
//     across the HTTP boundary);
//   - the misconfigured remote is not "unreachable" — its readiness handler
//     still evaluates and serves a document, so no synthetic
//     "misconfigured/reachable" fail row appears;
//   - the served document flows through namespaced ("misconfigured/ok"),
//     never silently frozen;
//   - the federation latch flips for it anyway: the latch answers "did this
//     remote answer a fetch", not "did the remote boot correctly". The loud
//     failure belongs to the remote operator's boot log, not the merge.
//
// Mirror of the aggregate composition test
// (aggregate/aggregate_validation_test.go).
func TestRemoteCriticalValidationComposesIntoFederation(t *testing.T) {
	t.Parallel()

	healthy := health.NewChecks(map[string]health.CheckFunc{
		"ok": func(context.Context) error { return nil },
	})

	if err := healthy.Start(context.Background()); err != nil {
		t.Fatalf("healthy remote Start() = %v", err)
	}

	t.Cleanup(healthy.Shutdown)

	misconfigured := health.NewChecks(map[string]health.CheckFunc{
		"ok": func(context.Context) error { return nil },
	}, health.WithCriticalServices("typoed-critical"))

	if err := misconfigured.Start(
		context.Background(),
	); !errors.Is(
		err,
		health.ErrUnknownCriticalService,
	) {
		t.Fatalf("misconfigured remote Start() = %v, want ErrUnknownCriticalService", err)
	}

	// No Shutdown needed: the failed Start disarmed the refresh loop (the
	// v0.5.1 wedge fix), so nothing leaks — and shutting the probe down would
	// flip its served document to shutting-down, which is not what this test
	// measures.

	serve := func(p *health.Probe) string {
		t.Helper()

		server := httptest.NewServer(p.ReadinessHandler())
		t.Cleanup(server.Close)

		return server.URL
	}

	prober, err := federation.New([]federation.Remote{
		{Name: "healthy", URL: serve(healthy)},
		{Name: "misconfigured", URL: serve(misconfigured)},
	})
	if err != nil {
		t.Fatalf(
			"federation.New() = %v, want nil (remote validation never leaks to the fetch side)",
			err,
		)
	}

	resp := prober.CachedResponse()

	if _, ok := resp.Checks["misconfigured/ok"]; !ok {
		t.Fatalf(
			"federation response missing misconfigured/ok (the served document must flow through), got %v",
			resp.Checks,
		)
	}

	if _, ok := resp.Checks["healthy/ok"]; !ok {
		t.Fatalf("federation response missing healthy/ok, got %v", resp.Checks)
	}

	for _, name := range []string{"misconfigured/reachable", "healthy/reachable"} {
		if _, ok := resp.Checks[name]; ok {
			t.Fatalf(
				"unexpected synthetic check %q (both remotes serve documents; reachable rows are for fetch failures only)",
				name,
			)
		}
	}

	if resp.Status != health.StatusPass {
		t.Fatalf("merged status = %q, want pass (both served documents roll up pass)", resp.Status)
	}

	if !prober.StartupComplete() {
		t.Fatal(
			"federation StartupComplete() = false, want true (the latch tracks fetch success, not remote boot correctness)",
		)
	}
}
