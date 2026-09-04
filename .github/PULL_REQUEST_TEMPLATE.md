# Why

<!-- What problem does this solve for users of the probe? Link the issue, or
     describe the user-facing need in one or two sentences. -->

# What changed

<!-- Files and behavior. One bullet per logical change; flag anything that
     alters the wire format, the public API, or the three-probe contract. -->

# Checklist

- [ ] Gates pass locally (`nix run .#test-race`, `.#vet`, `.#lint`, `.#vulncheck`, `.#security`, `nix flake check`, `nix fmt`)
- [ ] CI-emulation step from CONTRIBUTING.md run if a gate shells out to `go`
- [ ] Wire-format changes called out and the golden file regenerated deliberately (`go test . -update`)
- [ ] New options follow the "Adding a New Option" checklist in CONTRIBUTING.md
- [ ] Docs updated where user-visible: README, FEATURES.md, CHANGELOG.md (Unreleased), AGENTS.md for non-obvious context
- [ ] No new dependencies beyond `samber/do/v2` (or a strong justification above)
