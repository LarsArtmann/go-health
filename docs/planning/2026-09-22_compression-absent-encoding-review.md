# Compression `AbsentEncoding` Behavioral Review — v1.2.0 Fleet Decision

**Date**: 2026-09-22 · **Parent plan**: [fleet-dependency-currency-remediation](2026-09-18_13-54_fleet-dependency-currency-remediation.md) (C10/M41) · **Scope**: httputil v1.1.1 → v1.2.0 compression behavioral change

## The change

httputil v1.2.0 changed the default for requests that carry **no `Accept-Encoding` header**:
the Compression middleware now serves **identity** (uncompressed) instead of compressing
anyway. Serving gzip to a client that never announced support was a correctness bug class
(client gets undecodable bytes); identity is always decodable. `AbsentEncodingFirstConfigured`
remains available for deployments that want the old behavior deliberately.

## Fleet inventory (consumers of `httputil.Compression`)

| Consumer                      | Client profile              | Header-less clients?                        | Decision                                                                                                                                  |
| ----------------------------- | --------------------------- | ------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| CV                            | human site (browsers)       | no — browsers always send `Accept-Encoding` | identity default: no effect                                                                                                               |
| DiscordSync `internal/api`    | API (Go http.Clients, bots) | no — Go transport auto-sends `gzip`         | identity default: no effect                                                                                                               |
| InboxClean                    | human web app               | no                                          | identity default: no effect                                                                                                               |
| artmann-technologies-website  | public site (browsers/CDN)  | no                                          | identity default: no effect                                                                                                               |
| crush-daily `internal/server` | local UI (browser)          | no                                          | identity default: no effect                                                                                                               |
| dynamic-markdown-site         | public site                 | no                                          | identity default: no effect                                                                                                               |
| go-website-template           | template site               | no                                          | identity default: no effect                                                                                                               |
| library-policy `httpapi`      | API (CI + tooling)          | possible raw `curl`                         | identity default: **adopted** — plain `curl` without `--compressed` now receives valid identity bytes instead of gzip it could not decode |
| nsfw-classifier               | browsers + SDK + SSE        | SSE (excluded via `IncompressibleTypes`)    | identity default: **adopted** — SSE must be identity regardless; JSON endpoints unchanged for negotiating clients                         |

## Decision

**Fleet-wide: accept the v1.2.0 identity default. No consumer sets
`AbsentEncodingFirstConfigured`.** No deployment holds a contract that requires gzip on a
missing `Accept-Encoding`; the only header-less clients in the fleet (raw `curl`, SSE
readers) are exactly the ones identity serves correctly. This closes C10; revisit only if a
consumer gains a bandwidth-sensitive header-less client class.
