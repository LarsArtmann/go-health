# Contributing

Thanks for your interest in contributing to go-health!

## Development Setup

### Prerequisites

- **Go 1.26+** — [install](https://go.dev/doc/install)
- **Nix** (optional, recommended) — [install](https://nixos.org/download.html) for reproducible builds

### With Nix (recommended)

```bash
nix develop                    # Enter dev shell with all tools
nix run .#test                 # Run tests
nix run .#test-race            # Run tests with race detector
nix run .#lint                 # Run golangci-lint
nix run .#coverage             # Run tests with coverage report
nix run .#vulncheck            # Run govulncheck
nix run .#security             # Run gosec
nix fmt                        # Format code (treefmt)
```

### Without Nix

```bash
go test -race -count=1 -cover ./...
go vet ./...
golangci-lint run ./...        # Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Workflow

1. Fork the repository
2. Create a feature branch from `master`
3. Make your changes
4. Ensure all checks pass (the same gates CI runs):

```bash
nix run .#test-race    # tests with race detector
nix run .#vet          # go vet
nix run .#lint         # golangci-lint
nix run .#vulncheck    # govulncheck
nix run .#security     # gosec
nix flake check        # flake + formatting
erraudit ./... --type-aware   # error-handling audit (baseline: 0 violations)
nix fmt                # gofumpt/goimports/golines/nixfmt
```

Bare `go` commands outside the flake need `GOEXPERIMENT=jsonv2` — the code
imports `encoding/json/v2`, which go1.26 only exposes behind that experiment.
The flake exports it for every gate; a fresh shell does not.

5. Submit a pull request

## Status Reports

Significant sessions may add a point-in-time snapshot to `docs/status/`, named
`YYYY-MM-DD_HH-MM_<slug>.md`. These are historical records: never rewrite them.
Resolve their numbered items inline (strikethrough + commit hash) once the work
ships elsewhere, and archive fully-resolved reports under `docs/status/archived/`.

## Code Conventions

- **Packages** — root package `health` holds the probe SDK; `health/aggregate` merges multiple in-process probes into one health surface.
- **Functional options** — configuration uses `With*` option functions writing to a construction-only `config` struct.
- **No logging** — the library must not import `log/slog` or any logging package. Observability is the host application's responsibility.
- **Stdlib errors only** — sentinel errors via `errors.New` and `fmt.Errorf("%w: ...")`. No external error libraries.
- **No external dependencies** beyond `samber/do/v2`. Think twice before adding any import.
- **Tests** — standard `testing.T` + table-driven. No ginkgo, testify, or other frameworks.
- **Concurrency** — use `atomic` types for shared state. `sync.Mutex` only for lifecycle coordination.

## Testing Strategy

- **Table-driven, standard library only** — `testing.T`, no ginkgo/testify.
- **Classify matrix** — `probe_classify_test.go` walks every combination of
  health states × critical sets through the public API and asserts against a
  spec written independently of the implementation.
- **Stress tests** — `probe_lifecycle_test.go` interleaves
  `Start`/`Shutdown`/`MarkShuttingDown` under the race detector. Any lifecycle
  change must keep this suite race-clean.
- **Wire format** — `testdata/readiness_response.golden` locks the JSON shape
  (field order, `json.Deterministic` key sorting, json/v2's
  ignore-`omitempty`-on-scalars behavior). Regenerate with `go test . -update`
  and call the change out in the changelog — the wire format is frozen.
- **Coverage** — baseline is **97.6%** statements (2026-09-04, post-
  json/v2 migration and aggregate package). The main gap is the defensive
  shutdown-overlay branch in `CachedResponse` plus aggregate error paths.
- **Error-handling audit** — `erraudit ./... --type-aware` baseline is
  **0 violations**. The two intentional `_, _ = w.Write` swallows carry
  `//nolint:erraudit` with rationale; `erraudit nolint-audit .` verifies the
  directives stay needed and un-stale.

## Reporting Issues

Please use [GitHub Issues](https://github.com/larsartmann/go-health/issues) to report bugs or request features.

## License

MIT — see [LICENSE](LICENSE).
