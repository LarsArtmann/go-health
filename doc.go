// Package health provides a health-probe SDK for samber/do v2 containers. It
// turns the three-probe Kubernetes pattern (liveness, readiness, startup) into
// a single [Probe] type with sensible defaults.
//
// The package separates three distinct concerns that are often wrongly
// conflated into a single /health endpoint:
//
//   - Liveness: "Is the process alive?" — trivially fast, dependency-free.
//   - Readiness: "Can I serve traffic?" — checks all services, gates on critical.
//   - Startup: "Am I done booting?" — latches once all critical services pass.
//
// # Quick Start
//
//	injector := do.New()
//
//	// ... register and invoke services ...
//
//	probe := health.New(injector,
//	    health.WithCriticalServices("database", "redis"),
//	    health.WithVersion("1.0.0"),
//	)
//
//	if err := probe.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer probe.Shutdown()
//
//	mux := http.NewServeMux()
//	probe.RegisterRoutes(mux, health.DefaultRoutes())
//
// # Audit Integration (Optional)
//
// When a [HealthRecorder] is provided via [WithHealthRecorder], every
// health-check batch is delegated to the recorder instead of the raw injector.
// This allows an external system to observe health checks.
//
// [github.com/larsartmann/samber-do-auditlog].Plugin satisfies
// [HealthRecorder] implicitly — pass it directly:
//
//	plugin, _ := auditlog.New(auditlog.Config{Enabled: true})
//	injector := do.NewWithOpts(plugin.Opts())
//
//	probe := health.New(injector, health.WithHealthRecorder(plugin))
//
// # Programmatic Health API
//
// Not every consumer speaks HTTP. [Probe.Status], [Probe.Alive], and
// [Probe.Ready] read the cached view without triggering dependency checks;
// [Probe.AwaitReady] blocks until the instance can serve or the context is
// done; [Probe.Healthz] combines the startup latch, readiness roll-up, and
// shutdown state into one 200/503 handler for deployments that expose a
// single endpoint.
//
// No samber/do injector? Build a probe from any batch function:
//
//	probe := health.NewWithHealthCheck(func(ctx context.Context) map[string]error {
//	    return map[string]error{"database": db.PingContext(ctx)}
//	}, health.WithCriticalServices("database"))
//
// The probe is also a first-class samber/do citizen: [Probe.HealthCheck]
// satisfies do.HealthcheckerWithContext (register the probe in its own
// injector) and [Probe.AsShutdowner] adapts it to do.ShutdownerWithError.
//
// # Observability
//
// [WithEvaluationHook] registers a synchronous callback invoked with every
// classified response — the seam for metrics and alerting. Prometheus
// exposition and OpenTelemetry compose on top without new dependencies.
//
// # Live Evaluation and Throttling
//
// [WithRefreshInterval] with zero disables the background cache: readiness
// evaluates live on every request. Pair it with [WithLiveThrottle] to coalesce
// request floods into one batch per window.
//
// # Method-Set Guard
//
// [WithAllowedMethods] restricts the HTTP methods the handlers accept;
// rejected requests get 405 with a sorted Allow header (GET is always
// allowed). [WithGETOnly] is the deprecated zero-argument equivalent.
//
// # Instance Identity
//
// [WithInstanceID] stamps every response with a replica identifier
// (instance_id) so dashboards can attribute responses to the pod that
// produced them behind a shared load balancer.
//
// # Why Three Probes?
//
// A single /health endpoint conflates "process alive" with "dependencies
// reachable." When a dependency blips, the endpoint returns 503, the kubelet
// restarts the pod, and a restart cascade follows — even though the process
// itself is fine. Splitting probes breaks this coupling:
//
//   - /healthz (liveness) never checks dependencies. Only a deadlocked or
//     crashed process fails.
//   - /readyz (readiness) checks dependencies but only returns 503 for
//     critical failures. Non-critical failures (e.g. metrics exporter) are
//     surfaced in the response body without removing the pod from rotation.
//   - /startupz (startup) lets slow-booting apps use a generous kubelet
//     failureThreshold without affecting liveness/readiness sensitivity.
//
// # Background Caching
//
// Kubelet and load balancers poll health endpoints frequently (often every
// second). Without caching, each readiness check calls Ping() on every
// dependency, hammering downstream systems. The Probe runs health checks on a
// bounded background loop (default: every 1 second) and serves cached results
// so the HTTP endpoint is always O(1).
//
// Disable caching for low-traffic or development scenarios:
//
//	probe := health.New(injector, health.WithRefreshInterval(0))
//
// # Shutdown Awareness
//
// Call [Probe.Shutdown] (or [Probe.MarkShuttingDown] for two-phase graceful
// shutdown) during your server's graceful-drain path. Readiness immediately
// returns 503 so load balancers stop sending traffic before connections close.
// Liveness stays 200 because the process is still alive.
//
// # Timeouts
//
// [WithTimeout] sets a batch-level deadline: all services in one evaluation
// share the same context. A slow dependency can starve faster ones of their
// time budget. For per-service isolation, configure samber/do's native option
// at injector creation time:
//
//	injector := do.NewWithOpts(do.WithHealthCheckTimeout(2 * time.Second))
//
// This library does not override that setting; it only controls the outer batch
// deadline (default: 5 seconds).
package health
