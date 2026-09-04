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
4. Run the gate sweep — one command, same gates CI runs, fail-fast (or pass a
   subset: `nix run .#gates -- test-race lint`):

```bash
nix run .#gates         # test-race, vet, lint, vulncheck, security, fuzz, flake check
nix fmt                 # gofumpt/goimports/golines/nixfmt
erraudit ./... --type-aware   # error-handling audit (baseline: 0 violations)
```

Bare `go` commands outside the flake need `GOEXPERIMENT=jsonv2` — the code
imports `encoding/json/v2`, which go1.26 only exposes behind that experiment.
The flake exports it for every gate; a fresh shell does not.

5. Emulate CI once before pushing — CI machines have no host Go on `PATH`, so
   gates that shell out to `go` fall back to whatever toolchain the binary was
   built with and can fail there while passing locally (this exact leak
   produced the first red CI run). One command re-runs every gate under a
   `PATH` that contains `nix` and `gcc` but not `go`:

```bash
nix run .#ci-emulation   # subset: nix run .#ci-emulation -- lint
```

Verified 2026-09-04: both apps ran end-to-end green — `.#gates` (all seven
gates) and `.#ci-emulation` (test-race, fuzz, lint, vet, vulncheck, security,
flake check under the stripped PATH). The emulated run caught one real
formatting gap the same day it was introduced; treat a red emulated gate as a
real finding, not noise.

6. If you changed anything touching samber/do usage patterns, re-run the
   doanalyzerv2 audit (baseline: 0 findings, DO-1..DO-6). The analyzer lives
   in a private repo the nix sandbox cannot fetch, so it runs as a local
   replace-module runner checked in under `tools/doanalyzerv2`:

```bash
tools/doanalyzerv2/run.sh ..   # requires the go-design-smells checkout (default /home/lars/projects/branching-flow, override with GO_HEALTH_BRANCHING_FLOW)
```

7. Submit a pull request

## Adding a New Option

Options are the library's entire configuration surface; they follow one
pattern. Checklist for a new `With*` option:

1. **Write to `config`, never to `Probe`** — add the option function in
   `probe.go`, setting a field on the construction-only `config` struct.
2. **Wire through `assemble`** — copy the field into the `Probe` only if it is
   read at runtime; construction-only state stays on `config` and is discarded.
3. **Validate in `Validate()`** if a zero/negative value would degrade
   silently — `Start()` fail-fast depends on it.
4. **Test in `probe_options_test.go`** — default behavior, option applied, and
   every option it composes with (order-dependence is a real bug class: see
   `WithGETOnly` × `WithAllowedMethods`).
5. **Document the interaction** with the background cache when the option
   affects evaluation (see `WithLiveThrottle` in the README).
6. **README Configuration Reference** — add the row (keep the table order
   matching `probe.go`), plus FEATURES.md when user-visible.
7. **Godoc example** (`Example<Name>` in `example_test.go`) when the option
   changes _how_ callers use the probe, not just a value.
8. **Gates**: `nix run .#test-race`, `.#lint`, `nix fmt`, and the CI-emulation
   step above if any gate is affected.

## Release / API-Sync Checklist

Any release, rename, or API batch must sweep every place that names a version
or lists the API surface — these are the spots a docs sync historically missed:

1. README stability line (the `> **Stability:**` quote)
2. README badges + Project Docs section (dead badge = new drift class;
   goreportcard sunset proved it)
3. AGENTS.md status line + Project Documentation table
4. `doc.go` package comment (was missed in the v0.1.1 batch)
5. CHANGELOG `[Unreleased]` and the compare links at the bottom
6. `docs/openapi.yaml` (wire shape) and the golden-file tests

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
- **Coverage** — baseline is **99.7%** statements (2026-09-04, after closing
  the aggregate gaps). The single uncovered statement is the documented
  defensive nil-`Checks` guard in `Healthz` (accessors.go), unreachable via
  the public API today. `nix run .#coverage` must not regress below this.
- **Fuzzing** — `nix run .#fuzz` runs all three targets (root marshal,
  handler input, aggregate merge invariants) on a 10s budget each; the
  aggregate fuzz additionally pins instance_id round-trips and the
  worst-of/shutdown merge rules. Fuzz failures land in `testdata/fuzz/` and
  must be fixed, never deleted.
- **Error-handling audit** — `erraudit ./... --type-aware` baseline is
  **0 violations**. The two intentional `_, _ = w.Write` swallows (root
  `handlers.go` and `aggregate/aggregate.go` write seams) carry
  `//nolint:erraudit` with rationale; `erraudit nolint-audit .` baseline is
  **2 directives: 2 needed, 0 stale** (2026-09-04) and verifies the
  directives stay needed and un-stale.

## Reporting Issues

Please use [GitHub Issues](https://github.com/larsartmann/go-health/issues) to report bugs or request features.

## License

MIT — see [LICENSE](LICENSE).
