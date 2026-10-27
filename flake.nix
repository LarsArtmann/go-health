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
          #
          # Patch pin DROPPED (2026-10-11): nixpkgs' go_1_27 now ships
          # 1.27.2 natively, which carries the vuln-DB fixes
          # (GO-2026-6603..6617) that forced the 2026-10-09 overrideAttrs
          # source bump. goPkg stays the single toolchain seam for every
          # app that shells out to `go` — if nixpkgs ever lags the
          # toolchain the vuln DB requires again, re-introduce the bump
          # HERE only, never per-app.
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
              pkgs.gnugrep
              pkgs.gnused
              pkgs.gawk
              pkgs.coreutils
            ];
            text = ''
              root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
              if [ -n "$root" ]; then
                cd "$root"
              fi

              fail=0
              drift() {
                echo "docs-drift-check: DRIFT: $*" >&2
                fail=1
              }

              latest="$(git tag -l --sort=-v:refname 2>/dev/null | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n1 || true)"
              if [ -z "$latest" ]; then
                # No git (the flake check runs this script against the store
                # source snapshot, which has no .git): fall back to the newest
                # release heading in the CHANGELOG so the doc-sync checks stay
                # meaningful there.
                heading="$(grep -m1 -E '^## \[?v?[0-9]+\.[0-9]+\.[0-9]+\]' CHANGELOG.md || true)"
                rel="$(printf '%s' "$heading" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -n1 || true)"
                if [ -n "$rel" ]; then
                  latest="v$rel"
                fi
              fi
              if [ -z "$latest" ]; then
                echo "docs-drift-check: could not determine the latest release (no git tag, no CHANGELOG release heading)" >&2
                exit 1
              fi
              ver="''${latest#v}"
              ver_re="''${ver//./\\.}"

              grep -q "> \*\*Stability:\*\* ''${latest} alpha" README.md \
                || drift "README stability line does not read ''${latest} alpha (CONTRIBUTING checklist item 1)"

              grep -q "''${latest} released" AGENTS.md \
                || drift "AGENTS.md status line does not name the latest tag ''${latest} (checklist item 3)"

              grep -q "^\[Unreleased\]: .*/compare/''${latest}\.\.\.HEAD" CHANGELOG.md \
                || drift "CHANGELOG [Unreleased] compare base is not ''${latest} (checklist item 5)"

              grep -Eq "^\[(v)?''${ver_re}\]: " CHANGELOG.md \
                || drift "CHANGELOG compare-link definition missing for ''${latest}"

              adr_max="$(printf '%s\n' docs/adr/* | grep -oE '[0-9]+' | sort -n | tail -n1)"
              if [ -n "$adr_max" ]; then
                grep -qE "ADR-001\.\.0?''${adr_max}" FEATURES.md \
                  || drift "FEATURES.md ADR range does not cover ADR-00''${adr_max}"
              fi

              # AGENTS.md line cap: go-structure-linter enforces the 220-line
              # preflight cap at ERROR severity, but only BuildFlow runs that
              # tool — the canonical gate sweep stayed blind to it (an
              # over-cap edit shipped gates-green once). Mirror the cap here
              # so `nix run .#gates` and `nix flake check` enforce it too.
              agents_lines="$(wc -l < AGENTS.md)"
              if [ "$agents_lines" -gt 220 ]; then
                drift "AGENTS.md is ''${agents_lines} lines, over the 220-line preflight cap (move detail to docs/INDEX.md)"
              fi

              # [Unreleased] truth: when HEAD has moved past the latest tag the
              # Unreleased section must say what moved. A release that promotes
              # the section but leaves "Nothing yet." behind makes the changelog
              # lie about the tree it describes (v0.8.0 and v0.8.1 both did).
              if [ -d .git ]; then
                tagged="$(git rev-list -n1 "$latest" 2>/dev/null || true)"
                head_rev="$(git rev-parse HEAD 2>/dev/null || true)"
                if [ -n "$tagged" ] && [ -n "$head_rev" ] && [ "$tagged" != "$head_rev" ]; then
                  unreleased="$(awk '/^## \[Unreleased\]/ { inside = 1; next } /^## / { inside = 0 } inside' CHANGELOG.md)"
                  documented="$(printf '%s\n' "$unreleased" |
                    grep -E '^[-*] ' |
                    grep -vE '^[-*] Nothing yet\.$' || true)"
                  if [ -z "$documented" ]; then
                    drift "CHANGELOG [Unreleased] documents nothing although HEAD is past ''${latest} (add the entries, or cut the tag)"
                  fi
                fi
              fi

              # API-surface lockstep: each design doc's fenced go block is a
              # published surface, so a symbol added or removed without
              # touching it is drift. Three surfaces drifted this way before
              # the check existed (SourceStatuses and Healthz shipped, tested
              # and absent from federation-design.md; aggregate had no block at
              # all). The fence carries the surface tag (```go api:root) so one
              # document can hold several blocks without them merging into one
              # list. The code side is receiver-agnostic on purpose: pinning a
              # receiver type would silently skip methods that hang off any new
              # exported type.
              api_lockstep() {
                pkg_dir="$1"
                api_doc="$2"
                api_tag="$3"
                api_code="$(mktemp)"
                api_listed="$(mktemp)"
                for file in "$pkg_dir"/*.go; do
                  case "$file" in
                  *_test.go) continue ;;
                  esac
                  {
                    grep -hoE '^func (\([^)]*\) )?[A-Z][A-Za-z0-9]*' "$file" || true
                    grep -hoE '^type [A-Z][A-Za-z0-9]*' "$file" || true
                  }
                done |
                  sed -E 's/^func (\([^)]*\) )?//; s/^type //' |
                  sort -u >"$api_code"
                awk -v fence='```' -v tag="$api_tag" '
                  $0 == fence "go " tag { inside = 1; next }
                  $0 == fence { inside = 0; next }
                  inside
                ' "$api_doc" |
                  grep -oE '^(func (\([^)]*\) )?|type )[A-Z][A-Za-z0-9]*' |
                  sed -E 's/^func (\([^)]*\) )?//; s/^type //' |
                  sort -u >"$api_listed"
                if [ ! -s "$api_code" ]; then
                  drift "$pkg_dir exports no symbols, so the lockstep has nothing to compare"
                elif [ ! -s "$api_listed" ]; then
                  drift "$api_doc has no '\`\`\`go $api_tag' API block to compare against $pkg_dir"
                else
                  while IFS= read -r name; do
                    [ -n "$name" ] || continue
                    grep -qxF "$name" "$api_listed" ||
                      drift "$pkg_dir exports $name but $api_doc's $api_tag block does not list it"
                  done <"$api_code"
                  while IFS= read -r name; do
                    [ -n "$name" ] || continue
                    grep -qxF "$name" "$api_code" ||
                      drift "$api_doc's $api_tag block lists $name but $pkg_dir does not export it"
                  done <"$api_listed"
                fi
                rm -f "$api_code" "$api_listed"
              }
              api_lockstep . docs/ARCHITECTURE.md api:root
              api_lockstep aggregate docs/ARCHITECTURE.md api:aggregate
              api_lockstep federation docs/federation-design.md api:federation
              # Architecture completeness: every non-test source file in the
              # library must be named in docs/ARCHITECTURE.md, in both
              # directions (no file without a row). The map is the only place a
              # reader learns where a behaviour lives, so a file nobody wrote
              # down is a file the next session rediscovers from scratch. Root
              # files match by basename, sub-packages by path so
              # `aggregate/aggregate.go` cannot hide behind a same-named root
              # file. tools/ is a separate module and stays out of the map.
              # find, not `git ls-files`: a file that is written but not yet
              # staged is exactly the file whose map row gets forgotten, and
              # listing tracked files made the check blind to it until the
              # commit (verified by planting one).
              arch_files="$(find . -name '*.go' -not -path './tools/*' -not -name '*_test.go' | sed 's|^\./||' | sort)"
              for arch_file in $arch_files; do
                case "$arch_file" in
                tools/*) continue ;;
                esac
                grep -qF "$arch_file" docs/ARCHITECTURE.md ||
                  drift "docs/ARCHITECTURE.md does not name the source file $arch_file"
              done

              if [ "$fail" -eq 0 ]; then
                echo "docs-drift-check: OK - README/AGENTS/CHANGELOG/FEATURES in sync with ''${latest}"
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

          checks = {
            format = config.treefmt.build.check self;

            openapi-lockstep = pkgs.runCommand "openapi-lockstep" { } ''
              cd ${self}
              ${lib.getExe openapiLockstep}
              touch $out
            '';

            docs-drift-check = pkgs.runCommand "docs-drift-check" { } ''
              cd ${self}
              ${lib.getExe docsDriftCheck}
              touch $out
            '';

            fuzz-registry =
              pkgs.runCommand "fuzz-registry"
                {
                  nativeBuildInputs = [
                    pkgs.bash
                    pkgs.coreutils
                    pkgs.findutils
                    pkgs.gnugrep
                    pkgs.gnused
                  ];
                }
                ''
                  cd ${self}
                  bash tools/fuzz-registry-check.sh
                  touch $out
                '';
          };

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
              # BuildFlow runs these via `nix run nixpkgs#<tool>` when they
              # are missing from the devShell (without project deps). Having
              # them here keeps every quality tool project-pinned.
              pkgs.dprint
              pkgs.lychee
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

            # Fuzz-registry parity: both fuzz apps are hand-maintained shell
            # text, so a target written in Go could silently never be fuzzed.
            # One script backs the flake check (CI + nix flake check) and this
            # app; it also asserts fuzz and fuzz-long list the same targets.
            fuzz-registry =
              mkApp "fuzz-registry" "Verify the flake's fuzz registry matches the Fuzz* targets in the tree"
                [
                  pkgs.bash
                  pkgs.coreutils
                  pkgs.findutils
                  pkgs.git
                  pkgs.gnugrep
                  pkgs.gnused
                ]
                ''
                  root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
                  if [ -n "$root" ]; then
                    cd "$root"
                  fi
                  bash tools/fuzz-registry-check.sh
                '';

            # Coverage floor: CONTRIBUTING has carried a percentage since
            # v0.5.1 and nothing enforced it, so the number was an aspiration
            # a deletion could silently contradict. The gate runs the suite
            # once and grades both the total and each package; the floors sit
            # several points below the measured value (2026-10-27: 99.9%
            # total, federation 99.5%) so an unrelated refactor cannot red it.
            coverage-gate =
              mkApp "coverage-gate" "Fail when total or per-package coverage drops below the floor"
                [
                  goPkg
                  pkgs.bash
                  pkgs.gawk
                  pkgs.coreutils
                ]
                ''
                  root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
                  if [ -n "$root" ]; then
                    cd "$root"
                  fi
                  bash tools/coverage-gate.sh "$@"
                '';

            # Fleet version-skew: the script has existed since 2026-10-09 and
            # nothing invoked it, so a consumer pinning a two-minor-old release
            # was invisible until its build broke. The expectation is
            # machine-derived (the newest tag) unless --expect says otherwise,
            # because a hand-typed version becomes a second source of truth.
            skew =
              mkApp "skew" "Compare every fleet consumer's go-health pin against the newest release"
                [
                  pkgs.bash
                  pkgs.coreutils
                  pkgs.git
                  pkgs.gh
                  pkgs.gnugrep
                  pkgs.gnused
                ]
                ''
                  root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
                  if [ -n "$root" ]; then
                    cd "$root"
                  fi
                  bash tools/fleet-skew.sh "$@"
                '';

            # Fleet recorder mis-wire audit. Deliberately NOT part of `.#gates`:
            # it reads every consumer checkout under $FLEET_ROOT, so on CI (no
            # checkouts) it would report SKIP for all 16 repos and pass
            # vacuously — a green gate with nothing behind it is worse than no
            # gate. It is a maintainer command whose evidence lands in
            # docs/recorder-miswire-audit.md.
            recorder-audit =
              mkApp "recorder-audit" "Audit fleet checkout for WithHealthRecorder passed to a function-path constructor"
                [
                  pkgs.bash
                  pkgs.coreutils
                  pkgs.findutils
                  pkgs.gawk
                  pkgs.gnugrep
                ]
                ''
                  root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
                  if [ -n "$root" ]; then
                    cd "$root"
                  fi
                  bash tools/recorder-miswire-audit.sh "$@"
                '';

            # The release gate. `.#gates` already covers the same commands CI
            # runs, but the v0.8.0 tag went red because the tagger ran the
            # tests and not the gates — a missing invocation, not a missing
            # gate. This app adds what a tag additionally needs: a clean tree,
            # a CHANGELOG section for the version, an unused tag name, and a
            # fleet that is not behind. `nix run .#pre-tag -- v0.9.0`.
            pre-tag =
              mkApp "pre-tag" "Run every gate plus release readiness before cutting a tag"
                [
                  pkgs.nix
                  pkgs.bash
                  pkgs.coreutils
                  pkgs.git
                  pkgs.gnugrep
                ]
                ''
                  root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
                  if [ -n "$root" ]; then
                    cd "$root"
                  fi

                  version="''${1:-}"
                  if [ -n "$version" ]; then
                    case "$version" in
                    v[0-9]*.[0-9]*.[0-9]*) ;;
                    *)
                      echo "pre-tag: version must look like vX.Y.Z, got '$version'" >&2
                      exit 1
                      ;;
                    esac
                  fi

                  # Readiness first: no point paying for the full gate sweep
                  # (several minutes) when the tree cannot be tagged at all.
                  if [ -n "$(git status --porcelain)" ]; then
                    echo "pre-tag: working tree is dirty — commit or stash before tagging" >&2
                    git status --short >&2
                    exit 1
                  fi

                  if [ -n "$version" ]; then
                    if git rev-parse -q --verify "refs/tags/$version" >/dev/null; then
                      echo "pre-tag: tag $version already exists; published tags are never moved — pick a new patch" >&2
                      exit 1
                    fi
                    if ! grep -qE "^## \[?''${version#v}\]?" CHANGELOG.md; then
                      echo "pre-tag: CHANGELOG.md has no '## [$version]' section — cut the release section first" >&2
                      exit 1
                    fi
                  else
                    echo "pre-tag: no version argument, tag-specific checks skipped (pass e.g. v0.9.0)"
                  fi

                  echo "=== pre-tag: gate sweep (.#gates) ==="
                  nix run .#gates

                  echo "=== pre-tag: fleet version skew ==="
                  # A local tag nobody has bumped to is still unpublished work.
                  # --allow records a known backlog instead of hiding new drift.
                  nix run .#skew

                  echo "pre-tag: ready to tag ''${version:-<version>}"
                '';

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
              let
                # gosec 2.29.0 embeds a go1.26-era x/tools: its typechecker
                # decodes export data up to v4, while go 1.27.2 emits v5
                # ("export data version 5 is greater than maximum supported
                # version 4" — 39 import errors, zero real findings, exit
                # 1). The v4/v5 boundary sits between 1.27.1 and 1.27.2, so
                # gosec's `go list` must run under a HARDCODED 1.27.1 — not
                # goPkg (1.27.2 emits v5) and not pkgs.go_1_27 either:
                # nixpkgs silently moved that to 1.27.2 in the 2026-10-10
                # lock bump, which is exactly what turned this gate (and
                # the v0.6.0 tag's CI run) red with zero code changes.
                # 1.27.1 satisfies go.mod's `go 1.27` floor; the repo uses
                # no 1.27.2-only language or stdlib features. Drop the pin
                # — use goPkg — when nixpkgs ships a gosec carrying
                # securego/gosec#1772 (go1.27 export-data support).
                gosecGo = pkgs.go_1_27.overrideAttrs (_old: rec {
                  version = "1.27.1";
                  src = pkgs.fetchurl {
                    url = "https://go.dev/dl/go${version}.src.tar.gz";
                    hash = "sha256-TkCKuuEm2Ra2FkYnGT8sVPDjyhMS1pO4bbRfhiqyOLE=";
                  };
                });
              in
              mkApp "security" "Run gosec security scan"
                [
                  pkgs.gosec
                  gosecGo
                ]
                ''
                  # `./...` walks nested module directories too, so gosec also
                  # typechecks tools/doanalyzerv2 — a LOCAL-ONLY tool whose
                  # go.mod replaces github.com/larsartmann/go-design-smells
                  # with an absolute /home/lars/projects path and whose
                  # dependency is not published anywhere. On CI that path does
                  # not exist, the package fails to import
                  # ("invalid package name"), and gosec exits 1 with ZERO
                  # findings — a red gate reporting nothing. It was the only
                  # thing keeping the security job red from 2026-10-25 to
                  # 2026-10-27 (the v0.8.0 tag's CI run included) while the
                  # library itself scanned clean. The tool stays a local gate
                  # (tools/doanalyzerv2/run.sh); the security gate scans the
                  # library.
                  gosec -exclude-dir=tools ./...
                '';

            # Every target in the tree is listed here AND in fuzz-long, in
            # lockstep; a new target must be added to both in the same change.
            # The `fuzz-registry` app (and checks.fuzz-registry) diff this
            # registry against `^func Fuzz` per package, so a forgotten
            # registration fails `nix run .#gates` instead of silently never
            # fuzzing. Before that check existed this comment claimed a
            # pre-push hook did the diff, and no such hook existed.
            fuzz = mkApp "fuzz" "Run fuzz targets with a short time budget" [ goPkg ] ''
              go test . -run '^$' -fuzz=FuzzResponseMarshalDeterministic -fuzztime=10s
              go test . -run '^$' -fuzz=FuzzHandlerInput -fuzztime=10s
              go test . -run '^$' -fuzz=FuzzThrottleWindowBoundary -fuzztime=10s
              go test ./aggregate -run '^$' -fuzz=FuzzAggregateMergeInvariants -fuzztime=10s
              go test ./checks -run '^$' -fuzz=FuzzBatteries -fuzztime=10s
              go test ./federation -run '^$' -fuzz=FuzzCachedResponse_ArbitraryRemoteBodies -fuzztime=10s
              go test ./federation -run '^$' -fuzz=FuzzSourceStatuses_Invariants -fuzztime=10s
              go test ./federation -run '^$' -fuzz=FuzzHealthz_CodeMatchesBody -fuzztime=10s
              go test ./internal/merge -run '^$' -fuzz=FuzzResponsesMergeInvariants -fuzztime=10s
            '';

            # Weekly deep fuzz (see .github/workflows/fuzz-long.yml). A
            # trailing -fuzztime argument wins over the 5m default, so a
            # local dry run is `nix run .#fuzz-long -- -fuzztime=10s`.
            fuzz-long = mkApp "fuzz-long" "Run fuzz targets with a 5-minute budget each" [ goPkg ] ''
              go test . -run '^$' -fuzz=FuzzResponseMarshalDeterministic -fuzztime=5m "$@"
              go test . -run '^$' -fuzz=FuzzHandlerInput -fuzztime=5m "$@"
              go test . -run '^$' -fuzz=FuzzThrottleWindowBoundary -fuzztime=5m "$@"
              go test ./aggregate -run '^$' -fuzz=FuzzAggregateMergeInvariants -fuzztime=5m "$@"
              go test ./checks -run '^$' -fuzz=FuzzBatteries -fuzztime=5m "$@"
              go test ./federation -run '^$' -fuzz=FuzzCachedResponse_ArbitraryRemoteBodies -fuzztime=5m "$@"
              go test ./federation -run '^$' -fuzz=FuzzSourceStatuses_Invariants -fuzztime=5m "$@"
              go test ./federation -run '^$' -fuzz=FuzzHealthz_CodeMatchesBody -fuzztime=5m "$@"
              go test ./internal/merge -run '^$' -fuzz=FuzzResponsesMergeInvariants -fuzztime=5m "$@"
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
                  gates="''${*:-test-race vet lint coverage-gate vulncheck security fuzz fuzz-registry docs-check}"
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
                  gates="''${*:-test-race fuzz fuzz-registry lint vet vulncheck security}"
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
