# Cookbook: system and resource checks

**Date:** 2026-10-02 · Companion to [docs/batteries-ownership-decision.md](batteries-ownership-decision.md).

## 1. Disk / memory checks — use `health/checks`

Since the `health/checks` subpackage, the CV (`SystemResources`) and fir
(`CheckDiskSpace`) private implementations have a shared home:

```go
import "github.com/larsartmann/go-health/checks"

probe := health.NewChecks(map[string]health.CheckFunc{
    "disk-space": checks.Disk(dataDir, 1<<30),  // fail below 1 GiB free
    "memory":     checks.Memory(1 << 30),       // fail above 1 GiB heap
}, /* do NOT list these as critical */)
```

Threshold recipes from the fleet:

- **Disk free-bytes** (fir, <1 GB critical for a rename workload) — pass
  `minFreeBytes` sized to your write burst; list the check in
  `WithCriticalServices` only if the process cannot degrade.
- **Disk used-percent with warn band** (CV, 85% degraded / 97.3% unhealthy)
  — wrap `checks.Disk` with your own two-threshold logic and return errors
  for both bands; as a non-critical check both grade `warn` and the message
  carries the numbers.
- **Memory** — `checks.Memory` reads Go heap (`runtime.MemStats`), not RSS.
  For container RSS limits compose `os.ReadFile("/sys/fs/cgroup/...")`
  yourself; keep it non-critical (CV's rule: a full resource should ALERT,
  not restart the pod).
- **Naming convention:** prefix host-level checks `host/…` (`host/disk`,
  `host/memory`) so fleet dashboards can group them apart from dependency
  checks; keep names stable — they are wire identifiers.
- **fir's disk-CRITICAL choice is kubelet-correct when needed:** marking
  `disk-space` critical makes readiness 503 (pod leaves rotation) while
  liveness stays 200 — no restart cascade, the pod just stops receiving
  traffic. Choose critical only when the process cannot degrade; warn when
  it can (CV).

## 2. The DB-metadata trap

Do not model database _diagnostics_ as check output: engine type, DSN/URL,
and migration status do not belong in a probe response.

- Check names carry a **verdict** (pass/warn/fail + short error), nothing
  else. paperless-ngx-style `database{type,url,migration_status}` is a
  status-page concern (docs/system-status-vs-probe.md).
- **URLs leak credentials.** Connection strings routinely embed passwords;
  an unauthenticated `/readyz` body is world-readable inside the cluster
  (and scraped by dashboards). If you need migration visibility, expose a
  bounded "migrations-current: pass/fail" check and put details behind auth.
- Scalar fields (`Version`, `InstanceID`, `Uptime`) deliberately do not
  survive aggregate/federation merges — per-process diagnostics cannot be
  aggregated honestly.

## 3. Warn semantics in practice — the dnsblockd pattern

dnsblockd marks `database` and `dns` critical but leaves
`blocklist-sources` non-critical:

```go
health.New(injector,
    health.WithCriticalServices("database", "dns"),
    // blocklist-sources registered but NOT listed:
    // its failure degrades readiness to warn (200) — the server still
    // resolves from the loaded blocklist while sources are unreachable.
)
```

Rule of thumb: critical = "cannot serve traffic at all"; everything else
non-critical so the response body tells the operator _what_ is degraded
while the pod keeps serving. Escalate to critical only when serving stale
data is worse than serving nothing.
