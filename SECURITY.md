# Security Policy

## Supported versions

go-health is pre-v1: only the latest release line receives security fixes.
Please keep your `go get` pin current.

| Version                     | Supported |
| --------------------------- | --------- |
| latest release on this repo | yes       |
| older releases              | no        |

## Reporting a vulnerability

**Do not open a public issue for a security report.**

Use GitHub's private vulnerability reporting:
[Security → Report a vulnerability](https://github.com/LarsArtmann/go-health/security/advisories/new).

Include what you can of:

- Affected version (`go list -m github.com/larsartmann/go-health`) and Go version.
- A minimal probe setup that shows the impact (handlers, options, service graph).
- Observed vs expected HTTP behavior (status code / response body).

## What counts as a vulnerability here

go-health is a Kubernetes health-probe SDK. Reports are most valuable when
they affect the probe contract itself, for example:

- A way to make liveness, readiness, or startup return the wrong status code
  (e.g. 200 while a critical dependency is down, or 503 under crafted input),
  other than by calling `Shutdown`/`MarkShuttingDown`.
- A handler crash (panic) reachable from request content or method.
- Cross-tenant information leakage through the aggregate package's merged
  responses.
- A denial-of-service amplifier beyond the documented, opt-in behaviors
  (live mode without `WithLiveThrottle`; cache mode exists to prevent this).

## What is explicitly out of scope

- Wrong configuration by the host application (e.g. exposing the readiness
  endpoint publicly without a method guard or throttling).
- Failures caused by misclassified criticality (`WithCriticalServices`) —
  that is a usage decision, not a flaw.
- Vulnerabilities in samber/do service graphs themselves; report those
  upstream to [samber/do](https://github.com/samber/do).

## Response targets

- Acknowledgement: within 7 days.
- Triage and severity assessment: within 14 days.
- Fix or mitigation for accepted reports: within 30 days, released as a
  patch version with credit to the reporter unless anonymity is requested.

## Hardening guidance for users

- Keep the default background cache (`WithRefreshInterval`) on for any
  kubelet/load-balancer-polled endpoint; it bounds check frequency no matter
  how often the endpoint is polled.
- Prefer `WithAllowedMethods` over the deprecated `WithGETOnly` so HEAD/OPTIONS
  from infrastructure probes are a deliberate choice.
- Remember that liveness is designed to always return 200 while the process
  runs — never wire it to dependency checks.
