# Deprecation Policy

How go-health deprecates API, and how long deprecated symbols live.

## What counts as deprecated

A symbol is deprecated only when all of the following are true:

1. Its godoc carries a `// Deprecated:` paragraph naming the replacement
   (visible on pkg.go.dev with a strikethrough).
2. The README's Configuration Reference (or the relevant section) repeats the
   deprecation and points at the replacement.
3. CHANGELOG.md records it under the release's `### Deprecated` heading.
4. The symbol keeps working. A deprecated symbol never silently changes
   behavior — it is a rename hint, not a removal.

## Lifetime of a deprecated symbol

- **v0.x (current):** deprecated symbols are **not removed**. The v0.x line
  treats removals as breaking no matter how small the symbol; current
  example: `WithGETOnly` (deprecated in v0.1.1, no removal planned).
- **v1.0 and later:** a deprecated symbol may be removed **no earlier than
  the second minor release after the one that deprecated it** (deprecated in
  v1.2 → earliest removal in v1.4), and never in a patch release. Removals
  are announced in the CHANGELOG of every release in between.

## Deprecation vs. wire format

The JSON wire format served by the handlers is frozen separately from the Go
API and guarded by a golden-file test. Deprecation policy does not apply to
it: wire-format changes are called out in the CHANGELOG as breaking and do
not ride along with deprecations.

## For consumers using deprecated symbols

`staticcheck` (SA1019) and most IDEs will flag usage of a deprecated symbol.
Suppressing that warning at the call site is your decision; the upstream
project does not require it. Within go-health's own test suite, four tests
intentionally pin the deprecated `WithGETOnly` so its compatibility promise
is enforced by CI — the in-repo SA1019 findings are accepted, not `nolint`ed.

**Confirmed 2026-09-04 (decision G5):** this is the formal SA1019 policy —
in-repo pin-tests stay, accepted SA1019 findings are the documented default,
and no `//nolint:staticcheck` suppressions are introduced for them.
