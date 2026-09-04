# Prometheus Exposition — design note

> Decided: 2026-09-04 · Status: SPIKE VERIFIED, composition pattern recommended · Resolves: ROADMAP "Metrics integration hooks (Prometheus exposition)" and "Custom response formats"

## Problem

Should go-health ship a Prometheus metrics endpoint? Kubernetes deployments
typically scrape health endpoints, but health JSON is not the Prometheus
exposition format, and scraping `/readyz` at scrape frequency would defeat the
caching design (every scrape hitting the batch deadline path in live mode, or
serving the same cached snapshot Prometheus already had).

## What a scrape actually needs

The Prometheus text exposition format (0.0.4) is deliberately tiny:

```
# HELP health_up Whether this instance reports itself healthy.
# TYPE health_up gauge
health_up{instance="pod-7f9c"} 1
# HELP health_check Whether the named service passes its health check.
# TYPE health_check gauge
health_check{instance="pod-7f9c",service="db"} 1
```

That is the entire integration. `# HELP` / `# TYPE` comment lines, then one
line per series: `name{labels} value`. No dependency on
`prometheus/client_golang` is required — and adding it would multiply this
zero-dependency library's footprint for ~20 lines of `fmt.Fprintf`.

## Decision: composition, not incorporation

go-health ships the evaluation stream; the consumer owns the wire format:

1. **`WithEvaluationHook`** (P29) observes every classified response
   synchronously — background refreshes and live evaluations alike. Stashing
   the latest response into a package variable is the whole integration.
2. **A ~40-line stdlib writer** renders the format. A verified reference
   implementation lives in `prometheus_example_test.go`
   (`prometheusWriter` + `ExampleWithEvaluationHook_metrics`), exercising
   `WithInstanceID` (P36) as the `instance` label and both `# TYPE` lines.
3. The consumer registers the writer on its own `/metrics` mux entry,
   alongside `probe.RegisterRoutes` for the three probes.

This mirrors the content-negotiation decision
([content-negotiation-design.md](content-negotiation-design.md)): formats are
a composition concern, not library surface.

## Why not `prometheus/client_golang`?

- **Dependency economics.** The library's only dependency is `samber/do/v2`.
  client_golang pulls in prometheus/common, procfs, and friends — an order of
  magnitude more transitive code than this entire library.
- **Duplicate registry concerns.** Applications already have a metrics
  registry and their own scrape pipeline. A library registering its own
  default-registry metrics creates double-scrape and test-pollution problems
  that the application cannot opt out of cleanly.
- **The data is one gauge fan-out.** client_golang's value (histograms,
  summaries, exemplars, process collectors) is irrelevant for a health roll-up.

## Non-goals

- Pushgateway / OpenTelemetry spans: same composition seam
  (`WithEvaluationHook`) applies; no library support planned.
- Scrape-frequency coupling: `/metrics` reading a hook-stored snapshot never
  triggers dependency checks, by construction.
