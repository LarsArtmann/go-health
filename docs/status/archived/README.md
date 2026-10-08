# Archived status reports

Point-in-time status reports that are **fully resolved** — every numbered item
carries an inline `~~strikethrough~~ verdict` (done / routed / Won't implement
/ NOT-DO), so a reader can see at a glance where each item landed. They are
kept for history; do not treat them as current truth.

Archive rule (one line): a report is archived when every resolved-work item is
struck with a verdict and every still-open item is routed to TODO_LIST.md /
ROADMAP.md. §a (work evidence), §d (process confessions), and §e (improvement
notes) are left unstruck by convention — they are the session's historical
record, not open work. This file is a manifest, exempt from the strike gate
(it contains the literal `~~strikethrough~~` phrase by necessity).

## Archive manifest — 2026-10-04 docs-health sweep

| File                                                               | Classification | Deciding reason                                                                                   |
| ------------------------------------------------------------------ | -------------- | ------------------------------------------------------------------------------------------------- |
| `2026-09-15_06-56_issue-2-per-check-metadata-session.md`           | ARCHIVE        | Issue #2 shipped as v0.2.0; dashboard adopted; ADR-006 + cookbook landed                          |
| `2026-09-16_11-46_issue-2-closure-verification-and-followups.md`   | ARCHIVE        | Issue #2 closed; every follow-up done or routed (this run executed its §f49 rename)               |
| `2026-09-16_12-42_v020-release-session.md`                         | ARCHIVE        | v0.2.0 released + verified; dashboard adopted; pkg.go.dev renders                                 |
| `2026-10-02_13-48_wire-format-vs-paperless-and-consumer-survey.md` | ARCHIVE        | Superseded by 15-06 + 10-03; findings shipped as adoption-matrix / genre doc / cookbook / ADR-007 |
| `2026-10-02_15-06_dx-right-way-and-session-consolidation.md`       | ARCHIVE        | Superseded by 10-03; footgun → `ErrUnknownCriticalService`, batteries → `health/checks`           |

## Archive manifest — 2026-10-08 docs-health sweep

13 reports annotated inline (strikethrough + per-item verdicts; ~700 verdicts)
and archived. The three 2026-09-04 reports were the "anchor" question left
open by the 2026-10-04 sweep — decided ARCHIVE: every item resolved or routed;
nothing cites them as live state anymore.

| File                                                                               | Classification | Deciding reason                                                                                              |
| ---------------------------------------------------------------------------------- | -------------- | ------------------------------------------------------------------------------------------------------------ |
| `2026-09-04_21-31_pareto-plan-v2-full-execution-v012-released.md`                  | ARCHIVE        | v0.1.2/v0.1.3 shipped; §f items done or routed; owner asks answered (branch protection ON, vehicle shipped)  |
| `2026-09-04_22-15_evening-docs-health-run-annotate-archive-and-roadmap-reaudit.md` | ARCHIVE        | Its own §f follow-ups shipped (ADR-005, lockstep, property tests, README table); supersession complete       |
| `2026-09-04_22-37_v013-release-contract-ship-and-consumer-verification.md`         | ARCHIVE        | v0.1.3 released + verified; §b/§f items shipped, superseded, or cross-repo                                   |
| `2026-09-15_08-55_otel-monitoring-integration-assessment-and-self-review.md`       | ARCHIVE        | OTEL/integration ideas routed to ROADMAP Theme 2; the hook-panic finding verified and upgraded to a TODO row |
| `2026-09-18_09-46_next-level-hardening-and-concurrent-federation-session.md`       | ARCHIVE        | Healthz/lockstep/property tests shipped; sevRank split brain fixed in this sweep                             |
| `2026-09-22_19-51_todo-sweep-healthz-lockstep-tracker-ab.md`                       | ARCHIVE        | §h was already the same-evening resolution record; remaining items shipped in v0.4.0                         |
| `2026-09-22_19-51_fleet-remediation-execution-status.md`                           | ARCHIVE        | Execution record exists in docs/planning/; §f is almost entirely cross-repo (verdicts say which repo owns)   |
| `2026-09-22_21-01_brutal-status-todo-sweep-complete.md`                            | ARCHIVE        | v0.4.0 shipped; dashboard rows verified done; owner gates tracked in TODO_LIST                               |
| `2026-09-22_21-32_v040-release-and-dedup-session.md`                               | ARCHIVE        | v0.4.0/v0.4.1 shipped; docs-lockstep ask answered by `.#docs-check` (2026-10-08)                             |
| `2026-10-03_03-11_pareto-plan-executed-and-verified.md`                            | ARCHIVE        | Superseded: the release train shipped as v0.5.0; v0.5-window items re-venued to v0.6                         |
| `2026-10-04_11-39_docs-health-audit-living-docs-and-archive-sweep.md`              | ARCHIVE        | Its §c/§f asks executed by this sweep (remaining archives, drift gate, anti-rot)                             |
| `2026-10-04_13-13_fleet-mapping-html-process-report-and-self-critique.md`          | ARCHIVE        | Report-v2 series retired (correction banner `5c60f0d`); its one buildable idea shipped (`.#docs-check`)      |
| `2026-10-08_19-42_self-review-plan-execution-lifecycle-bug-and-gates.md`           | ARCHIVE        | Successor report (20:58) carries the live handoff; lint tail + plan steps 8/9 closed same day                |

## Still open (kept in `docs/status/`)

| File                                                                                | Why it stays                                                                                     |
| ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| `2026-10-08_20-58_execution-resume-makezero-fix-federation-doc-glossary-antirot.md` | Freshest handoff: §g owner questions (push, v0.5.1 vehicle, BuildFlow boundary) still open       |
| `2026-10-08_20-59_buildflow-gate-triage-and-toolchain-skew.md`                      | BuildFlow gate NOT green; §f1–15 gate-restoration list + §g patch-pin question are the live work |

_Archived files are annotated non-destructively: original text is preserved,
resolutions are added inline. Never rewrite an archived report._
