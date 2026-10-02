# System status page vs health probe: two genres

**Date:** 2026-10-02 · **Comparison baseline:** paperless-ngx `d8b2e70b0c4e28c72717da4c25f32e45de698b64` (2026-09-18, recent main; llmindex task block present) — `src/documents/views.py` `SystemStatusView` (:4137–4410), tests `src/documents/tests/test_api_status.py` (19 tests incl. auth-challenge, container detection, redis/celery pings).

## TL;DR

go-health deliberately serves **none** of paperless-ngx's system-inventory
fields. This is a genre difference, not a gap: a probe is unauthenticated,
cache-backed, and answers "route traffic here?" in microseconds; a status
page is authenticated, synchronous, and answers "what is the state of this
deployment?" for a human operator. Formalized as ADR-007. The composition
point for status-page needs is [go-health-dashboard](https://github.com/larsartmann/go-health-dashboard).

## Field-by-field comparison

| paperless-ngx `/api/status/` field | go-health equivalent | Why the difference |
| --- | --- | --- |
| `pngx_version` | `version` (opt-in, `WithVersion`), `/version` endpoint (`VersionHandler`) | Present, but identity not health: never 503s, survives as a per-process scalar only. |
| `server_os` (platform.platform()) | — | Probe genre: no host inventory. Would leak OS details on an unauthenticated endpoint (see F2 threat model). |
| `install_type` (bare-metal/k8s/docker) | — | Deployment identity is the orchestrator's business; env detection in a library couples it to container runtimes. |
| `storage{total, available}` (statvfs) | — | No system metrics by design; CV (`internal/health/systemresources.go`) and fir (`CheckDiskSpace`) compose their own — see docs/system-checks-cookbook.md. |
| `database{type, url, status, error, migration_status}` | a check named `database` (pass/warn/fail + error text) | Check names carry the *health verdict* only. Type/URL/migrations are diagnostics with secret-leak risk (URLs embed credentials), not probe data. |
| tasks blocks (redis, celery, index, classifier, sanity, llmindex) | ordinary checks (pass/fail per dependency) | Same capability, different encoding: verdict-per-check instead of nested task state. |
| auth: `IsAuthenticated` + staff-only | unauthenticated by design | Kubelets/LBs must poll without credentials. Anything sensitive belongs behind a composition layer (dashboard, separate mux), not in the probe. |
| cadence: synchronous per request | cached (1s default) or live | Probe answers in microseconds from cache so a 30s kubelet poll interval never hammers dependencies. |
| status codes: 200 with JSON state | 200/503 semantics per probe (pass/warn/fail) | The probe's body is a bonus; the status code is the contract for k8s. |

## When you need a paperless-style page

Do not extend the probe. Compose:

1. run the probe as-is for k8s (`/healthz`, `/readyz`, `/startupz`);
2. mount go-health-dashboard (content-negotiated JSON + HTML) for humans;
3. build authenticated, synchronous diagnostics (statvfs, migrations,
   `platform.platform()`) in your own staff-only view, free to reuse
   `probe.CachedResponse()` as one input.

## Evidence for the genre split (from this repo's design history)

- Scalars (`Version`, `Uptime`, `InstanceID`, `Timestamp`) deliberately do
  not survive `aggregate`/`federation` merges — they are per-process and
  would lie in an aggregate view. System inventory would lie even harder.
- docs/content-negotiation-design.md rejects HTML in the probe; a status
  page is the same rejection one level up.
- docs/classification-2.0-design.md rejects turning the probe into a
  monitoring system; inventory is the same category error.
