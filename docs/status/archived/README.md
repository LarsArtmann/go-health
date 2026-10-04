# Archived status reports

Point-in-time status reports that are **fully resolved** — every numbered item
carries an inline `~~strikethrough~~ verdict` (done / routed / Won't implement
/ NOT-DO), so a reader can see at a glance where each item landed. They are
kept for history; do not treat them as current truth.

## Archive manifest — 2026-10-04 docs-health sweep

| File | Classification | Deciding reason |
| ---- | -------------- | --------------- |
| `2026-09-15_06-56_issue-2-per-check-metadata-session.md` | ARCHIVE | Issue #2 shipped as v0.2.0; dashboard adopted; ADR-006 + cookbook landed |
| `2026-09-16_11-46_issue-2-closure-verification-and-followups.md` | ARCHIVE | Issue #2 closed; every follow-up done or routed (this run executed its §f49 rename) |
| `2026-09-16_12-42_v020-release-session.md` | ARCHIVE | v0.2.0 released + verified; dashboard adopted; pkg.go.dev renders |
| `2026-10-02_13-48_wire-format-vs-paperless-and-consumer-survey.md` | ARCHIVE | Superseded by 15-06 + 10-03; findings shipped as adoption-matrix / genre doc / cookbook / ADR-007 |
| `2026-10-02_15-06_dx-right-way-and-session-consolidation.md` | ARCHIVE | Superseded by 10-03; footgun → `ErrUnknownCriticalService`, batteries → `health/checks` |

## Still open (kept in `docs/status/`)

| File | Why it stays |
| ---- | ------------ |
| `2026-09-15_08-55_otel-monitoring-integration-assessment-and-self-review.md` | OTEL composition example + Gatus recipe still open (ROADMAP Theme 2) |
| `2026-09-18_09-46_next-level-hardening-and-concurrent-federation-session.md` | Kept — open ideas harvested | WithTransitionHook, `healthtest`, OpenAPI-federation all routed to ROADMAP raw ideas 2026-10-04; report not yet fully annotated |
| `2026-09-22_*.md` (4) | cross-repo fleet remediation; local-only pushes + several open consumer fixes |
| `2026-09-04_*.md` (3) | reference anchors cited by older archived reports |
| `2026-10-03_03-11_pareto-plan-executed-and-verified.md` | the current harvest source (not yet superseded) |

_Archived files are annotated non-destructively: original text is preserved,
resolutions are added inline. Never rewrite an archived report._
