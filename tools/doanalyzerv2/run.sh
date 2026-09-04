#!/usr/bin/env bash
# Entry point for the private doanalyzerv2 audit runner. The analyzer module
# (go-design-smells) lives in a local checkout whose path this script
# validates before building — a wrong path otherwise fails deep inside the
# Go module resolution with no hint about the fix.
#
# Usage (from the repo root):
#   tools/doanalyzerv2/run.sh ..
#
# Override the checkout location:
#   GO_HEALTH_BRANCHING_FLOW=/path/to/go-design-smells tools/doanalyzerv2/run.sh ..
set -euo pipefail

cd "$(dirname "$0")"

readonly module=github.com/larsartmann/go-design-smells
root="${GO_HEALTH_BRANCHING_FLOW:-/home/lars/projects/branching-flow}"

if [ ! -f "$root/go.mod" ]; then
	echo "doanalyzerv2-runner: analyzer checkout not found at $root" >&2
	echo "  the analyzer lives in the private repo $module," >&2
	echo "  which the nix sandbox cannot fetch. Fix either way:" >&2
	echo "    git clone git@github.com:LarsArtmann/go-design-smells.git $root" >&2
	echo "    export GO_HEALTH_BRANCHING_FLOW=/path/to/existing/checkout" >&2
	exit 2
fi

go mod edit -replace "$module=$root"
exec go run . "$@"
