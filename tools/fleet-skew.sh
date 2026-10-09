#!/usr/bin/env bash
# Fleet version-skew detector (A19): every go.mod in tools/fleet-repos.txt
# vs the latest released go-health tag. Fails when any consumer is a minor
# behind the latest tag (the CV class of double-stale skew); a patch-only lag
# warns but passes; a missing pin warns (the repo may have legitimately
# dropped the dependency — report, don't police). Most fleet repos are
# private, so pins are read through the authenticated GitHub API (gh; CI
# provides GITHUB_TOKEN) at each repo's live default branch.
#
# Usage:
#   fleet-skew.sh [latest-tag]     # tag defaults to the newest local tag (v-prefixed)
#
# Exit codes: 0 = no minor drift, 1 = at least one consumer is minor-stale.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
manifest="$here/fleet-repos.txt"
repo_root=$(cd "$here/.." && pwd)

latest="${1:-$(git -C "$repo_root" tag --list 'v*' --sort=-v:refname | head -1)}"
if [ -z "$latest" ]; then
	echo "fleet-skew: no v-prefixed tags found in $repo_root" >&2
	exit 2
fi
latest_minor=$(echo "$latest" | sed -E 's/^v([0-9]+)\.([0-9]+).*/\1.\2/')

echo "latest tag: $latest (minor $latest_minor)"
fails=0
warns=0

while read -r repo modpath; do
	[ -z "$repo" ] && continue
	case "$repo" in \#*) continue ;; esac

	branch=$(gh api "repos/$repo" --jq .default_branch 2>/dev/null) || branch=""
	if [ -z "$branch" ]; then
		echo "WARN  $repo: repo unreachable (removed or renamed?)"
		warns=$((warns + 1))
		continue
	fi

	pin=$(
		gh api "repos/$repo/contents/$modpath?ref=$branch" --jq .content 2>/dev/null |
			base64 -d 2>/dev/null |
			grep -oE 'github\.com/larsartmann/go-health v[0-9]+\.[0-9]+\.[0-9]+[^[:space:]]*' | head -1 |
			grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' || true
	)

	if [ -z "$pin" ]; then
		echo "WARN  $repo ($modpath @$branch): no go-health pin found (dropped or moved)"
		warns=$((warns + 1))
		continue
	fi

	pin_minor=$(echo "$pin" | sed -E 's/^v([0-9]+)\.([0-9]+).*/\1.\2/')
	if [ "$pin" = "$latest" ]; then
		echo "OK    $repo ($modpath @$branch): $pin"
	elif [ "$pin_minor" = "$latest_minor" ]; then
		echo "WARN  $repo ($modpath @$branch): $pin (patch behind $latest)"
		warns=$((warns + 1))
	else
		echo "FAIL  $repo ($modpath @$branch): $pin (minor behind $latest)"
		fails=$((fails + 1))
	fi
done <"$manifest"

echo "---"
echo "minor-stale: $fails, warnings: $warns"
[ "$fails" -eq 0 ]
