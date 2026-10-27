# Fleet recorder mis-wire audit (T20)

**Date:** 2026-10-27 · **Status:** IMPLEMENTED (tool) · **Result:** 0 mis-wires in
16 local checkouts / 8,507 Go files
**Tool:** `tools/recorder-miswire-audit.sh`, `nix run .#recorder-audit`

## The trap this exists for

`WithHealthRecorder(...)` only has an effect on the injector path:
`health.New(injector, health.WithHealthRecorder(r))`. The function-path
constructors — `NewWithHealthCheck`, `NewWithDetailedCheck`, `NewChecks` — run
their own batch and cannot call a recorder. Before v0.8.0 a recorder passed to
one of them was accepted and **silently ignored**: no callbacks, no traces, no
error, and a `probe_test.go` that asserted "warn stays warn" kept passing.

v0.8.0 turned that into a `Start`-time `ErrRecorderNotApplicable`
(`Probe.Validate`), so the trap is loud for anyone on v0.8.x. It stays silent
for every consumer still pinned below v0.8.0 — which, at the time of writing, is
almost the whole fleet (`docs/research/2026-10-27_fleet-consumer-needs.md`).

That asymmetry is why a static audit is worth having next to the runtime guard:
the fix protects the next bump, the audit tells us whether the current pins are
already wrong.

## The rule

A call to a function-path constructor whose **own argument list** contains
`WithHealthRecorder` is a mis-wire.

Implementation detail that matters for trust: each file is first reduced to a
shadow copy with comments and string/rune/raw literals blanked out (offsets and
newlines preserved), then every constructor call is read to its matching closing
paren on that shadow. Without the shadow pass, the rule fires on doc comments
that show the wrong way (`// example: NewChecks(fn, WithHealthRecorder(r))`) —
which would make the audit unusable on exactly the repos that documented the
trap.

What it deliberately does **not** catch:

| Case                                                        | Why not                                          | Caught by     |
| ----------------------------------------------------------- | ------------------------------------------------ | ------------- |
| A recorder whose method signature does not satisfy the interface | It never compiles.                          | the compiler  |
| A constructor aliased into another identifier before the call    | Requires type information, not text.         | the compiler  |
| A recorder passed to a *third-party* wrapper constructor     | Unknown name, not in the rule.                   | code review   |

Both blind spots fail loudly (compile error), so the audit covers the silent
half and nothing else.

## Result

`nix run .#recorder-audit` on the maintainer's machine, 2026-10-27:

```
OK   go-health-dashboard      OK   CV                     OK   cv
OK   Zlota44                  OK   dnsblockd              OK   library-policy
OK   file-and-image-renamer   OK   DiscordSync            OK   KeyHolderAI
OK   nsfw-classifier          OK   webphone               OK   projects-management-automation
OK   timesheets               OK   cqrs-htmx              OK   go-appkit
OK   go-taskqueue
---
recorder-miswire: scanned 8507 Go files
recorder-miswire: no recorder passed to a function-path constructor
```

Sixteen repos are checked out locally and every one is clean. The audit is
manifest-driven (`tools/fleet-repos.txt`, the same list the skew check uses), so
a repo that is **not** checked out is reported as `SKIP` rather than passing by
omission.

The two call sites that carry a recorder in the fleet use the injector path
correctly (`go-appkit/integration/health_stack_test.go:81`,
`go-health-dashboard`'s screenshot recorder), which is what the audit asserts.

## Why it is not a gate

`.#gates` must be green on CI, where no consumer checkout exists. Running the
audit there would print `SKIP` for all 16 repos and exit 0 — a green gate with
nothing behind it. It stays a maintainer command, and this document is its
evidence log; re-run it whenever a consumer bumps its pin, and record the result
here.

## Follow-up that this audit cannot do

The real fix for a mis-wire is a version bump: from v0.8.0 the constructor
rejects the wiring at `Start`, so a consumer that is currently silent becomes
loud on the next bump. That work is tracked in the consumption-wave plan
(`docs/planning/2026-10-27_21-12_wave2-adoption-closing-the-consumption-gap.md`,
M01-M04) and needs push authority.
