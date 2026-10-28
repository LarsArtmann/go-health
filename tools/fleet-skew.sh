#!/usr/bin/env bash
# Fleet version-skew detector (A19): every go.mod in tools/fleet-repos.txt
# vs the latest released go-health tag. Fails when any consumer is a minor
# behind the latest tag (the CV class of double-stale skew); a patch-only lag
# warns but passes; a missing pin warns (the repo may have legitimately
# dropped the dependency — report, don't police).
#
# Two pin sources:
#   api   — the authenticated GitHub API (gh; CI provides GITHUB_TOKEN) at each
#           repo's live default branch. Authoritative, needs network + a token.
#   local — the checkout at $FLEET_SKEW_LOCAL_ROOT/<repo-basename> (default
#           ~/projects). Works offline and without a token, at the cost of
#           seeing the local working tree rather than the pushed branch.
# `auto` (the default) picks api when gh is authenticated and local otherwise,
# and always prints which one it chose — a gate that silently measures
# something other than what you think is worse than one that fails.
#
# Usage:
#   fleet-skew.sh [--expect TAG] [--allow N] [--allow-unchecked N] [--source S] [TAG]
#
#   --expect TAG       compare against TAG instead of the newest local tag. The
#                      default is machine-derived on purpose: a hand-typed
#                      expectation is a second source of truth that goes stale
#                      the moment a release happens. CI passes --expect because
#                      it checks out one branch and may not carry the tags.
#   --allow N          tolerate up to N minor-stale consumers. Default: the number
#                      in tools/fleet-skew.allow (0 when that file is absent), so
#                      the backlog is recorded ONCE and every caller (/pre-tag,
#                      CI, a human) inherits it instead of gating zero. Use it
#                      only to record a known backlog, never to hide new drift.
#   --allow-unchecked N
#                      tolerate up to N consumers whose pin could not be read
#                      (default 0 = fail closed). A repo the token cannot see, a
#                      checkout that is missing, or a rate-limit window must never
#                      look the same as a repo that is up to date — that silence
#                      is how 13 of 16 repos once "passed" while measuring
#                      nothing (2026-10-28).
#   --source S         api | local | auto (default auto; see above).
#
# Exit codes: 0 = no minor drift and nothing unread (or within --allow /
# --allow-unchecked), 1 = too many minor-stale or unread consumers,
# 2 = usage error or no version to compare against.
#
# Every GitHub call runs under `gh_timeout`: without one a stalled connection
# hangs the gate forever instead of failing. Observed 2026-10-28 — a run sat
# alive for 55 minutes after printing "latest tag", and the same run's earlier
# attempt reported all 16 repos "unreachable" during a rate-limit window.
set -euo pipefail

gh_timeout="${FLEET_SKEW_TIMEOUT:-30}"
local_root="${FLEET_SKEW_LOCAL_ROOT:-$HOME/projects}"

here=$(cd "$(dirname "$0")" && pwd)
manifest="$here/fleet-repos.txt"
repo_root=$(cd "$here/.." && pwd)

expect=""
allow=""
allow_unchecked=0
source="auto"
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
	--allow-unchecked)
		allow_unchecked="${2:?--allow-unchecked needs a number}"
		shift 2
		;;
	--allow-unchecked=*)
		allow_unchecked="${1#--allow-unchecked=}"
		shift
		;;
	--source)
		source="${2:?--source needs api, local or auto}"
		shift 2
		;;
	--source=*)
		source="${1#--source=}"
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

if [ -z "$allow" ]; then
	allow=0
	allow_file="$here/fleet-skew.allow"
	if [ -f "$allow_file" ]; then
		allow=$(sed -n -E 's/^([0-9]+)[[:space:]]*$/\1/p' "$allow_file" | head -1)
		allow="${allow:-0}"
		echo "backlog: $allow minor-stale consumer(s) allowed (tools/fleet-skew.allow)"
	fi
fi

for pair in "allow:$allow" "allow_unchecked:$allow_unchecked"; do
	name="${pair%%:*}"
	value="${pair#*:}"
	case "$value" in
	'' | *[!0-9]*)
		echo "fleet-skew: --$name takes a non-negative integer, got '$value'" >&2
		exit 2
		;;
	esac
done

case "$source" in
api | local | auto) ;;
*)
	echo "fleet-skew: --source takes api, local or auto, got '$source'" >&2
	exit 2
	;;
esac

latest="${expect:-$(git -C "$repo_root" tag --list 'v*' --sort=-v:refname | head -1)}"
if [ -z "$latest" ]; then
	echo "fleet-skew: no v-prefixed tags found in $repo_root" >&2
	exit 2
fi
latest_minor=$(echo "$latest" | sed -E 's/^v([0-9]+)\.([0-9]+).*/\1.\2/')

gh_usable=0
if command -v gh >/dev/null 2>&1; then
	if timeout "$gh_timeout" gh api user --jq .login >/dev/null 2>&1; then
		gh_usable=1
	fi
fi

if [ "$source" = auto ]; then
	if [ "$gh_usable" -eq 1 ]; then
		source=api
	else
		source=local
	fi
fi

case "$source" in
api)
	if [ "$gh_usable" -ne 1 ]; then
		echo "fleet-skew: --source api needs an authenticated gh ('gh auth login', or use --source local)" >&2
		exit 2
	fi
	echo "source: api (authenticated gh, live default branch)"
	;;
local)
	if [ ! -d "$local_root" ]; then
		echo "fleet-skew: --source local needs a checkout root, $local_root does not exist" >&2
		exit 2
	fi
	echo "source: local ($local_root working trees — not the pushed branches)"
	;;
esac

echo "latest tag: $latest (minor $latest_minor)"
fails=0
warns=0
unchecked=0

while read -r repo modpath; do
	[ -z "$repo" ] && continue
	case "$repo" in \#*) continue ;; esac

	pin=""
	where=""
	if [ "$source" = api ]; then
		branch=$(timeout "$gh_timeout" gh api "repos/$repo" --jq .default_branch 2>/dev/null) || branch=""
		if [ -n "$branch" ]; then
			pin=$(
				timeout "$gh_timeout" gh api "repos/$repo/contents/$modpath?ref=$branch" --jq .content 2>/dev/null |
					base64 -d 2>/dev/null |
					grep -oE 'github\.com/larsartmann/go-health v[0-9]+\.[0-9]+\.[0-9]+[^[:space:]]*' | head -1 |
					grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' || true
			)
			where="$branch"
		fi
	else
		checkout="$local_root/$(basename "$repo")"
		if [ -f "$checkout/$modpath" ]; then
			pin=$(
				grep -oE 'github\.com/larsartmann/go-health v[0-9]+\.[0-9]+\.[0-9]+[^[:space:]]*' "$checkout/$modpath" |
					head -1 | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' || true
			)
			where="local $(git -C "$checkout" rev-parse --abbrev-ref HEAD 2>/dev/null || echo '?')"
		else
			echo "FAIL  $repo ($modpath): no checkout at $checkout/$modpath" >&2
			unchecked=$((unchecked + 1))
			continue
		fi
	fi

	if [ -z "$where" ]; then
		echo "FAIL  $repo: repo unreadable via the GitHub API (removed, renamed, rate-limited, timed out, or invisible to the token)" >&2
		unchecked=$((unchecked + 1))
		continue
	fi

	if [ -z "$pin" ]; then
		echo "WARN  $repo ($modpath @$where): no go-health pin found (dropped or moved)"
		warns=$((warns + 1))
		continue
	fi

	pin_minor=$(echo "$pin" | sed -E 's/^v([0-9]+)\.([0-9]+).*/\1.\2/')
	if [ "$pin" = "$latest" ]; then
		echo "OK    $repo ($modpath @$where): $pin"
	elif [ "$pin_minor" = "$latest_minor" ]; then
		echo "WARN  $repo ($modpath @$where): $pin (patch behind $latest)"
		warns=$((warns + 1))
	else
		echo "FAIL  $repo ($modpath @$where): $pin (minor behind $latest)"
		fails=$((fails + 1))
	fi
done <"$manifest"

echo "---"
echo "minor-stale: $fails (allowed $allow), warnings: $warns, unreadable: $unchecked (allowed $allow_unchecked)"
rc=0
if [ "$fails" -gt "$allow" ]; then
	echo "fleet-skew: $fails consumer(s) are a minor behind $latest, above the allowed $allow" >&2
	echo "fleet-skew: bump them (go get github.com/larsartmann/go-health@$latest) or explain the backlog in tools/fleet-repos.txt" >&2
	rc=1
fi
if [ "$unchecked" -gt "$allow_unchecked" ]; then
	echo "fleet-skew: $unchecked consumer(s) could not be read, above the allowed $allow_unchecked" >&2
	echo "fleet-skew: an unreadable repo is not an up-to-date repo — fix the source (gh auth login, missing checkout) or pass --allow-unchecked with a reason" >&2
	rc=1
fi
exit "$rc"
