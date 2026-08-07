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
4. Ensure all checks pass:

```bash
go test -race -count=1 -cover ./...
go vet ./...
golangci-lint run ./...
```

5. Submit a pull request

## Code Conventions

- **Single package** — all code lives in package `health`. No sub-packages.
- **Functional options** — configuration uses `With*` option functions writing to a construction-only `config` struct.
- **No logging** — the library must not import `log/slog` or any logging package. Observability is the host application's responsibility.
- **Stdlib errors only** — sentinel errors via `errors.New` and `fmt.Errorf("%w: ...")`. No external error libraries.
- **No external dependencies** beyond `samber/do/v2`. Think twice before adding any import.
- **Tests** — standard `testing.T` + table-driven. No ginkgo, testify, or other frameworks.
- **Concurrency** — use `atomic` types for shared state. `sync.Mutex` only for lifecycle coordination.

## Reporting Issues

Please use [GitHub Issues](https://github.com/larsartmann/go-health/issues) to report bugs or request features.

## License

MIT — see [LICENSE](LICENSE).
