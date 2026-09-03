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
    systems.url = "github:nix-systems/default";
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
          goPkg = pkgs.go_1_26;

          # Every Go command in this flake needs the json/v2 experiment
          # enabled: the code imports encoding/json/v2, which go1.26 only
          # exposes behind GOEXPERIMENT=jsonv2. Exporting it here keeps the
          # gates hermetic — they must not depend on the host shell's env.
          mkApp =
            name: description: runtimeInputs: text:
            let
              script = pkgs.writeShellApplication {
                inherit name runtimeInputs;
                text = ''
                  export GOEXPERIMENT=jsonv2
                  ${text}
                '';
              };
            in
            {
              type = "app";
              program = lib.getExe script;
              meta.description = description;
            };
        in
        {
          treefmt = {
            projectRootFile = "go.mod";
            programs = {
              gofumpt.enable = true;
              goimports.enable = true;
              golines.enable = true;
              nixfmt.enable = true;
            };
          };

          checks.format = config.treefmt.build.check self;

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
            GOEXPERIMENT = "jsonv2";

            shellHook = ''
              echo "go-health dev shell — $(go version)"
            '';
          };

          apps = {
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

            lint = mkApp "lint" "Run golangci-lint" [ pkgs.golangci-lint ] ''
              golangci-lint run ./...
            '';

            coverage = mkApp "coverage" "Run tests with coverage report" [ goPkg ] ''
              go test ./... -coverprofile=coverage.out -covermode=atomic "$@"
              go tool cover -func=coverage.out
            '';

            vulncheck = mkApp "vulncheck" "Run govulncheck vulnerability scan" [ pkgs.govulncheck ] ''
              govulncheck ./...
            '';

            security = mkApp "security" "Run gosec security scan" [ pkgs.gosec ] ''
              gosec ./...
            '';

            clean = mkApp "clean" "Remove coverage artifacts and clear the test cache" [
              goPkg
              pkgs.trash-cli
            ] ''
              trash-put coverage.out 2>/dev/null || true
              go clean -testcache
            '';
          };
        };
    };
}
