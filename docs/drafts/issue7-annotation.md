# Issue #7 annotation — draft (owner posts)

**Target:** https://github.com/LarsArtmann/go-health/issues/7
**Register:** comment (terse status annotation, not a report). Post with
`gh issue comment 7 --body-file docs/drafts/issue7-annotation.md`.

---

F1 fixed on master: `classifier` now routes every recovered panic through one
predicate covering both sentinels. `classify` had the same bug, so a
non-critical panic was rolling up `warn` even after `grades` was patched —
both fail closed now. ADR-008 has the reasoning; go-appkit's warn pin needs to
flip when it bumps.

F2 took the sentinel: `Validate`/`Start` return `ErrRecorderNotApplicable` on
the function path, no signature change, fails at boot instead of silently
capturing nothing.

F3 both: `Evaluate`'s godoc says it does not publish, and `EvaluateAndCache`
publishes for callers that need the read side to see it.

F4 documented, not cleared: `Start` after `Shutdown` cannot be made honest —
the drain flag is what makes the 503 trustworthy — so the one-way valve is now
stated on `Start`, `Shutdown` and `AwaitReady`.
