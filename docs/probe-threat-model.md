# Threat model: unauthenticated probe endpoints

**Date:** 2026-10-02 · Companion to [ADR-007](adr/ADR-007-no-system-inventory.md) and [SECURITY.md](../SECURITY.md).

## The exposure

go-health handlers are unauthenticated by design: kubelets, load balancers,
and orchestrators must poll them without credentials. That makes every
response body a **public document inside whatever boundary can reach the
port** — cluster-internal peers, and the internet if the port is exposed.

## What is safe to expose (and why the defaults are what they are)

| Data                                                    | Exposure                     | Rationale                                                                                                                                                                      |
| ------------------------------------------------------- | ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| check names + pass/warn/fail + short error text         | accepted                     | The error text is the operator's triage payload; keep messages free of secrets, hosts, and connection strings (consumer responsibility — see the cookbook's DB-metadata trap). |
| `version` (opt-in `WithVersion`)                        | accepted, opt-in             | Build identity; opt-in so nobody leaks it accidentally. `/version` endpoint likewise.                                                                                          |
| `instance_id` (opt-in `WithInstanceID`)                 | accepted, opt-in             | Correlation key, not a secret; only set when a dashboard needs it.                                                                                                             |
| `uptime`                                                | accepted                     | Coarse operational signal.                                                                                                                                                     |
| `timestamp`, `total_latency_ms`, `since`, `duration_ns` | accepted                     | Timing metadata is a minor fingerprinting aid at worst.                                                                                                                        |
| install type / OS / storage / DB engine-URL-migrations  | **never**                    | Inventory = reconnaissance; URLs embed credentials; migrations reveal deployment state. ADR-007.                                                                               |
| HTML rendering                                          | **never** on probe endpoints | Attack surface without operator value; dashboards own rendering.                                                                                                               |

## Threats considered

- **Reconnaissance:** an internal actor learns service topology, dependency
  names, and failure modes from `/readyz` bodies. Mitigation: check names
  are service-level, not schema-level; error messages must stay
  service-scoped. Network policy (not the library) gates who can poll.
- **Credential leak via error text:** a check that returns
  `dial tcp user:pass@host:5432` publishes the password. The library cannot
  redact what it cannot recognize; docs make message hygiene a check-author
  responsibility (`errors.Is`-able sentinels + short constants, as
  `health/checks` models).
- **DoS via evaluation amplification:** the 1s background cache means
  polling handlers costs nothing; live mode is guarded by
  `WithLiveThrottle`. Query-param "override" knobs were rejected
  (docs/timeout-design.md).
- **Method/tamper surface:** `WithAllowedMethods` restricts verb misuse;
  handlers are read-only by construction.

## Consumer checklist

1. Never expose a probe port publicly; network-policy it to the poller.
2. Keep check error messages secret-free (sentinels + short constants).
3. Put diagnostics (inventory, migrations, DSNs) behind authentication in
   your own view — never as check names, error text, or new probe fields.
4. Opt in to `version`/`instance_id` only when a consumer needs them.
