# Automation design notes: agent skill + critical-name analyzer

**Date: 2026-10-02 · Status: REJECTED for now (both) — revisit triggers recorded.**

## 1. Agent wiring skill / template

**Idea:** a reusable skill (or `go-new`-style template) that wires a
go-health probe into any Go service the way the fleet's golden consumers do
(Pattern A: injector + `WithCriticalServices` + eager invocation +
`RegisterRoutes` + `Shutdown` + `AwaitReady` gate).

**Verdict: rejected for now.**

- The golden path is now fully documented (README constructor table +
  version recipe + quickstart); the skill would duplicate it and drift.
- Fleet usage is heterogeneous (4 implementation patterns); a template
  encodes Pattern A and quietly biases the other three.
- Every consumer repo already has a working, tested wiring; the audience
  (net-new services) is served better by the README than by a
  session-scoped skill.

**Revisit when:** a second fresh-user simulation (F3) shows the README
still produces assembly mistakes, or a consumer asks for scaffolding.

## 2. Critical-name vet analyzer (doanalyzerv2 reuse)

**Idea:** an AST analyzer that checks every `WithCriticalServices(...)`
literal against the `do.ProvideNamed`/typetostring registrations in the same
package — moving the `ErrUnknownCriticalService` guarantee to build time.

**Verdict: rejected for now.**

- **Redundant:** `ErrUnknownCriticalService` at `Start()` catches the same
  defect with zero tooling, at boot, for every consumer including the
  dynamic/bridge patterns an AST analyzer cannot see (recorder batches,
  projection names, cross-package registration).
- **Blind spots create false confidence:** an analyzer that passes code the
  runtime rejects is worse than no analyzer (the erraudit cargo-cult lesson
  in AGENTS.md).
- **Cost:** doanalyzerv2 is a private replace-module runner; generalizing
  it for public consumption is a project of its own.

**Revisit when:** consumers with boot-order constraints (probe started
before services provided) become common enough that a static check is the
only way to catch drift before deploy — or if a v0.5 `ServiceName` rollout
wants per-repo call-site inventory automation.
