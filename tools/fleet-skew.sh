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
#   fleet-skew.sh [--expect TAG] [--allow N] [TAG]
#
#   --expect TAG  compare against TAG instead of the newest local tag. The
#                 default is machine-derived on purpose: a hand-typed
#                 expectation is a second source of truth that goes stale the
#                 moment a release happens. CI passes --expect because it
#                 checks out one branch and may not carry the tags.
#   --allow N     tolerate up to N minor-stale consumers (default 0). Use it
#                 only to record a known backlog, never to hide new drift.
#
# Exit codes: 0 = no minor drift (or within --allow), 1 = too many
# minor-stale consumers, 2 = no version to compare against.
#
# Every GitHub call runs under `gh_timeout`: without one a stalled connection
# hangs the gate forever instead of failing. Observed 2026-10-28 — a run sat
# alive for 55 minutes after printing "latest tag", and the same run's earlier
# attempt reported all 16 repos "unreachable" during a rate-limit window. A
# timeout turns both shapes into the WARN this script already understands.
set -euo pipefail

gh_timeout="${FLEET_SKEW_TIMEOUT:-30}"

here=$(cd "$(dirname "$0")" && pwd)
manifest="$here/fleet-repos.txt"
repo_root=$(cd "$here/.." && pwd)

expect=""
allow=0
while [ "$#" -gt 0 ]; do
	case "$1" in
	--expect)
		expect="${2:?--expect needs a tag argument}"
		shift 2
		;;
	--expect=*)
		expect="${1#--expect=}"
		shift
		;;
	--allow)
		allow="${2:?--allow needs a number}"
		shift 2
		;;
	--allow=*)
		allow="${1#--allow=}"
		shift
		;;
	-*)
		echo "fleet-skew: unknown flag $1 (see the usage comment at the top of tools/fleet-skew.sh)" >&2
		exit 2
		;;
	*)
		expect="$1"
		shift
		;;
	esac
done

case "$allow" in
'' | *[!0-9]*)
	echo "fleet-skew: --allow takes a non-negative integer, got '$allow'" >&2
	exit 2
	;;
esac

latest="${expect:-$(git -C "$repo_root" tag --list 'v*' --sort=-v:refname | head -1)}"
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

	branch=$(timeout "$gh_timeout" gh api "repos/$repo" --jq .default_branch 2>/dev/null) || branch=""
	if [ -z "$branch" ]; then
		echo "WARN  $repo: repo unreachable (removed, renamed, rate-limited or timed out)"
		warns=$((warns + 1))
		continue
	fi

	pin=$(
		timeout "$gh_timeout" gh api "repos/$repo/contents/$modpath?ref=$branch" --jq .content 2>/dev/null |
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
echo "minor-stale: $fails, warnings: $warns (allowed: $allow)"
if [ "$fails" -gt "$allow" ]; then
	echo "fleet-skew: $fails consumer(s) are a minor behind $latest, above the allowed $allow" >&2
	echo "fleet-skew: bump them (go get github.com/larsartmann/go-health@$latest) or explain the backlog in tools/fleet-repos.txt" >&2
	exit 1
fi
