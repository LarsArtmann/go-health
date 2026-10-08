{
  description = "go-health — Kubernetes health-probe SDK for samber/do v2";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    # Linux-only: nixpkgs 26.11 dropped x86_64-darwin, so darwin systems
    # cannot even evaluate the flake, let alone be verified. Widen again
    # (nix-systems/default) if a darwin build target becomes verifiable.
    systems.url = "github:nix-systems/default-linux";
  };

  outputs =
    inputs@{
      self,
      flake-parts,
      treefmt-nix,
      systems,
      ...
    }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = import systems;

      imports = [
        treefmt-nix.flakeModule
      ];

      perSystem =
        { config, pkgs, ... }:
        let
          inherit (pkgs) lib;
          # go.mod carries a go 1.27 directive (the ecosystem-wide relax,
          # same as go-cqrs-lite/cqrs-htmx) and GOTOOLCHAIN=local (sandbox
          # forbids toolchain downloads) cannot satisfy it under go_1_26,
          # so the toolchain is nixpkgs' go_1_27.
          goPkg = pkgs.go_1_27;

          # encoding/json/v2 is stable on go1.27: no GOEXPERIMENT is needed
          # anywhere (verified 2026-09-22: build + vet + full suite green
          # with GOEXPERIMENT unset). Apps stay hermetic via runtimeInputs.
          mkApp =
            name: description: runtimeInputs: text:
            let
              script = pkgs.writeShellApplication {
                inherit name runtimeInputs;
                inherit text;
              };
            in
            {
              type = "app";
              program = lib.getExe script;
              meta.description = description;
            };

          # OpenAPI ↔ golden-file lockstep: every wire key the golden readiness
          # response exhibits must be declared in the spec's HealthResponse /
          # Check schemas, and every status value must be one of the spec's
          # enum values. Without this, the spec (redocly-linted) and the wire
          # format (golden-file-tested) can drift silently — CI checks them
          # independently, never against each other. One script definition
          # backs both the flake check (CI + nix flake check) and the local
          # app (nix run .#openapi-lockstep).
          openapiLockstep = pkgs.writeShellApplication {
            name = "openapi-lockstep";
            runtimeInputs = [
              pkgs.yq-go
              pkgs.jq
            ];
            text = ''
              spec="$(mktemp)"
              trap 'rm -f "$spec"' EXIT
              yq -o=json '.' "''${1:-docs/openapi.yaml}" > "$spec"
              golden="''${2:-testdata/readiness_response.golden}"
              jq -e --slurpfile spec "$spec" '
                . as $wire
                | $spec[0].components.schemas as $s
                | [
                    ($wire | keys[] as $k | $s.HealthResponse.properties | has($k)),
                    ($s.HealthResponse.properties.status.enum | index($wire.status) != null),
                    ($wire.checks | to_entries[] | .value | keys[] as $ck | $s.Check.properties | has($ck))
                  ]
                | all
              ' "$golden" > /dev/null || {
                echo "openapi-lockstep: $golden drifted from docs/openapi.yaml (HealthResponse/Check properties or status enum)" >&2
                exit 1
              }
              echo "openapi-lockstep: $golden is fully covered by docs/openapi.yaml"
            '';
          };

          # Docs drift alarm: mechanizes the CONTRIBUTING "Release / API-Sync
          # Checklist" items that a manual release keeps skipping (the v0.5.0
          # release left the README stability line and the AGENTS status line
          # stale for three days). One script backs the flake check (CI runs
          # it) and the local app (nix run .#docs-check).
          docsDriftCheck = pkgs.writeShellApplication {
            name = "docs-drift-check";
            runtimeInputs = [
              pkgs.git
              pkgs.grep
            ];
            text = ''
              root="$(git rev-parse --show-toplevel)"
              cd "$root"

              fail=0
              drift() {
                echo "docs-drift-check: DRIFT: $*" >&2
                fail=1
              }

              latest="$(git tag -l --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n1 || true)"
              if [ -z "$latest" ]; then
                echo "docs-drift-check: no semver tag found; nothing to verify" >&2
                exit 1
              fi
              ver="${latest#v}"
              ver_re="${ver//./\\.}"

              grep -q "> \*\*Stability:\*\* ${latest} alpha" README.md \
                || drift "README stability line is not '${latest} alpha' (CONTRIBUTING checklist item 1)"

              grep -q "${latest} released" AGENTS.md \
                || drift "AGENTS.md status line does not name the latest tag ${latest} (checklist item 3)"

              grep -q "^\[Unreleased\]: .*/compare/${latest}\.\.\.HEAD" CHANGELOG.md \
                || drift "CHANGELOG [Unreleased] compare base is not ${latest} (checklist item 5)"

              grep -Eq "^\[(v)?${ver_re}\]: " CHANGELOG.md \
                || drift "CHANGELOG compare-link definition missing for ${latest}"

              adr_max="$(ls docs/adr | grep -oE '[0-9]+' | sort -n | tail -n1)"
              if [ -n "$adr_max" ]; then
                grep -qE "ADR-001\.\.0?${adr_max}" FEATURES.md \
                  || drift "FEATURES.md ADR range does not cover ADR-00${adr_max}"
              fi

              if [ "$fail" -eq 0 ]; then
                echo "docs-drift-check: OK - README/AGENTS/CHANGELOG/FEATURES in sync with ${latest}"
              fi
              exit "$fail"
            '';
          };
        in
        {
          treefmt = {
            projectRootFile = "go.mod";
            programs = {
              gofumpt.enable = true;
              # gofmt (not goimports): the sandboxed format check cannot
              # download the go1.27.1 toolchain that nixpkgs' goimports
              # resolves via GOTOOLCHAIN=auto. gofmt is the layout subset of
              # the repo's current gofumpt+goimports fixed point — zero churn.
              gofmt.enable = true;
              golines.enable = true;
              nixfmt.enable = true;
            };
          };

          checks.format = config.treefmt.build.check self;

          checks.openapi-lockstep = pkgs.runCommand "openapi-lockstep" { } ''
            cd ${self}
            ${lib.getExe openapiLockstep}
            touch $out
          '';

          checks.docs-drift-check = pkgs.runCommand "docs-drift-check" { } ''
            cd ${self}
            ${lib.getExe docsDriftCheck}
            touch $out
          '';

          devShells.default = pkgs.mkShell {
            packages = [
              goPkg
              pkgs.golangci-lint
              pkgs.gofumpt
              pkgs.golines
              pkgs.gopls
              pkgs.gotools
              pkgs.govulncheck
              pkgs.gosec
              pkgs.trash-cli
            ];

            GOWORK = "off";

            shellHook = ''
              echo "go-health dev shell — $(go version)"
            '';
          };

          apps = {
            openapi-lockstep = {
              type = "app";
              program = lib.getExe openapiLockstep;
              meta.description = "Verify the golden wire format stays covered by docs/openapi.yaml (paths override: spec golden)";
            };

            docs-check = {
              type = "app";
              program = lib.getExe docsDriftCheck;
              meta.description = "Verify README/AGENTS/CHANGELOG/FEATURES stay in sync with the latest tag (CONTRIBUTING release checklist)";
            };

            test = mkApp "test" "Run all tests" [ goPkg ] ''
              go test ./... -count=1 "$@"
            '';

            test-race = mkApp "test-race" "Run all tests with the race detector" [ goPkg ] ''
              go test ./... -race -count=1 "$@"
            '';

            build = mkApp "build" "Build all packages" [ goPkg ] ''
              go build ./...
            '';

            vet = mkApp "vet" "Run go vet static analysis" [ goPkg ] ''
              go vet ./...
            '';

            lint =
              mkApp "lint" "Run golangci-lint"
                [
                  pkgs.golangci-lint
                  # golangci-lint shells out to a `go` binary for package
                  # loading; without goPkg on PATH it falls back to the GOROOT
                  # it was compiled with (an older Go that rejects go.mod's
                  # go 1.27 directive) and fails on any machine whose host
                  # PATH does not already provide Go 1.27.
                  goPkg
                ]
                ''
                  golangci-lint run ./...
                '';

            coverage = mkApp "coverage" "Run tests with coverage report" [ goPkg ] ''
              go test ./... -coverprofile=coverage.out -covermode=atomic "$@"
              go tool cover -func=coverage.out
            '';

            vulncheck =
              mkApp "vulncheck" "Run govulncheck vulnerability scan"
                [
                  pkgs.govulncheck
                  goPkg
                ]
                ''
                  govulncheck ./...
                '';

            security =
              mkApp "security" "Run gosec security scan"
                [
                  pkgs.gosec
                  goPkg
                ]
                ''
                  gosec ./...
                '';

            fuzz = mkApp "fuzz" "Run fuzz targets with a short time budget" [ goPkg ] ''
              go test . -run '^$' -fuzz=FuzzResponseMarshalDeterministic -fuzztime=10s
              go test . -run '^$' -fuzz=FuzzHandlerInput -fuzztime=10s
              go test ./aggregate -run '^$' -fuzz=FuzzAggregateMergeInvariants -fuzztime=10s
            '';

            # Weekly deep fuzz (see .github/workflows/fuzz-long.yml). A
            # trailing -fuzztime argument wins over the 5m default, so a
            # local dry run is `nix run .#fuzz-long -- -fuzztime=10s`.
            fuzz-long = mkApp "fuzz-long" "Run fuzz targets with a 5-minute budget each" [ goPkg ] ''
              go test . -run '^$' -fuzz=FuzzResponseMarshalDeterministic -fuzztime=5m "$@"
              go test . -run '^$' -fuzz=FuzzHandlerInput -fuzztime=5m "$@"
              go test ./aggregate -run '^$' -fuzz=FuzzAggregateMergeInvariants -fuzztime=5m "$@"
            '';

            clean =
              mkApp "clean" "Remove coverage artifacts and clear the test cache"
                [
                  goPkg
                  pkgs.trash-cli
                ]
                ''
                  trash-put coverage.out 2>/dev/null || true
                  go clean -testcache
                '';

            # One command before every push: runs the same gates CI runs,
            # fail-fast, reusing the app definitions above as the single
            # source of truth (no copied gate commands to drift). Pass a
            # subset to run less: nix run .#gates -- test-race lint
            gates =
              mkApp "gates" "Run the full pre-push gate sweep, fail-fast (same gates as CI)" [ pkgs.nix ]
                ''
                  gates="''${*:-test-race vet lint vulncheck security fuzz docs-check}"
                  # shellcheck disable=SC2086 # word splitting over gate names is intended
                  for gate in $gates; do
                    echo "=== gate: $gate ==="
                    nix run ".#$gate"
                  done
                  echo "=== gate: flake check ==="
                  nix flake check
                  echo "all gates green"
                '';

            # CI machines have no host Go on PATH; gates that shell out to a
            # `go` binary fall back to whatever toolchain that binary was
            # compiled with and can fail there while passing locally. This
            # wrapper re-runs the gates with nix and gcc as the only external
            # tools on PATH — the same leak class that produced the first red
            # CI run. Subset: nix run .#ci-emulation -- lint
            ci-emulation =
              mkApp "ci-emulation" "Re-run gates under a PATH without the host Go toolchain (CI emulation)"
                [
                  pkgs.coreutils
                  pkgs.gcc
                  pkgs.nix
                ]
                ''
                  tmpbin="$(mktemp -d)"
                  trap 'rm -rf "$tmpbin"' EXIT
                  ln -s "$(command -v nix)" "$tmpbin/nix"
                  ln -s "$(command -v gcc)" "$tmpbin/gcc"
                  gates="''${*:-test-race fuzz lint vet vulncheck security}"
                  # shellcheck disable=SC2086 # word splitting over gate names is intended
                  for gate in $gates; do
                    echo "=== ci-emulation: $gate (no host go on PATH) ==="
                    env PATH="$tmpbin:/usr/bin:/bin" nix run ".#$gate"
                  done
                  echo "=== ci-emulation: flake check ==="
                  env PATH="$tmpbin:/usr/bin:/bin" nix flake check
                  echo "all emulated gates green"
                '';
          };
        };
    };
}
