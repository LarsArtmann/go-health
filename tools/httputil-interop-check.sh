#!/usr/bin/env bash
# httputil interop proof: compile and run the testdata/httputil-interop
# fixture module, proving the recipes in docs/httputil-integration.md against
# the PUBLISHED github.com/larsartmann/httputil release (module proxy).
# Exits non-zero when a go-health change breaks a documented composition.
# Needs network for the first module fetch; that is why this is an app, not a
# sandboxed flake check (same reason httputil's otel-interop is a CI job).
#
# Read-only on the tree. Exit 0 = fixture green, 1 = fixture red,
# 2 = cannot run.
set -euo pipefail

if [ $# -gt 1 ]; then
	echo "usage: httputil-interop-check.sh [REPO_DIR]" >&2
	exit 2
fi

cd "${1:-.}"

if [ ! -f flake.nix ]; then
	echo "interop-check: flake.nix not found in $(pwd)" >&2
	exit 2
fi

cd testdata/httputil-interop

# GOWORK=off: the fixture is its own module and must resolve its own
# dependencies, never the fleet workspace.
GOWORK=off go build ./...
GOWORK=off go test ./...

echo "interop-check: fixture green against the published httputil release"
