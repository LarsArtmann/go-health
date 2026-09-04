---
name: Feature request
about: Propose a capability for go-health
labels: ["enhancement"]
---

**Problem to solve**

<!-- What can't you do today? Describe the user-facing need, not the implementation. -->

**Proposed API sketch**

```go
// what the surface could look like (optional but helpful)
```

**Alternatives considered**

<!-- Other libraries, workarounds, or composition patterns you tried. -->

**Constraints this library keeps**

go-health deliberately keeps: stdlib + `samber/do/v2` only, no logging, no
error libraries, frozen JSON wire format, dependency-blind liveness. Proposals
that break these need strong justification.

**Checklist**

- [ ] I checked [ROADMAP.md](../blob/master/ROADMAP.md) for existing plans
- [ ] I checked existing issues for duplicates
