#!/usr/bin/env bash
# Coverage floor gate: run the suite once, then refuse a total or per-package
# percentage below the floor. CONTRIBUTING has documented a percentage since
# v0.5.1 and nothing ever enforced it, so "99.7% baseline" was an aspiration
# that a deletion could quietly contradict.
#
# Floors are deliberately several points below the measured value (2026-10-27:
# total 99.9%, root/aggregate/checks/merge 100%, federation 99.5%) because a
# gate pinned to the current number is a gate that fails on someone else's
# unrelated refactor. Raise a floor only after a deliberate coverage
# improvement lands, never to make a red run green.
#
# Usage:
#   coverage-gate.sh [--total PCT] [--package PCT] [--report-only]
#   coverage-gate.sh --profile FILE     # grade an existing profile, run nothing
#
# Exit codes: 0 = every package and the total meet their floor (or
# --report-only), 1 = a floor was missed, 2 = bad usage or no profile data.
set -euo pipefail

total_min=99.5
package_min=98
report_only=0
profile=""

while [ "$#" -gt 0 ]; do
	case "$1" in
	--total)
		total_min="${2:?--total needs a percentage}"
		shift 2
		;;
	--package)
		package_min="${2:?--package needs a percentage}"
		shift 2
		;;
	--profile)
		profile="${2:?--profile needs a path}"
		shift 2
		;;
	--report-only)
		report_only=1
		shift
		;;
	-h | --help)
		sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
		exit 0
		;;
	*)
		echo "coverage-gate: unknown argument $1" >&2
		exit 2
		;;
	esac
done

if [ -z "$profile" ]; then
	profile="$(mktemp)"
	trap 'rm -f "$profile"' EXIT
	# The profile is only meaningful over a green suite: a failing test leaves
	# partial counters that would report a *higher* percentage.
	if ! go test ./... -coverprofile="$profile" -covermode=atomic; then
		echo "coverage-gate: the test suite failed, so the profile is not graded" >&2
		exit 1
	fi
fi

if [ ! -s "$profile" ]; then
	echo "coverage-gate: $profile is empty (no coverage data)" >&2
	exit 2
fi

# Per-package percentages from block counters: group the profile's block rows
# by their package directory. This is the same arithmetic `go tool cover`
# performs, so the total below and the package rows agree by construction.
report="$(
	awk -v total_min="$total_min" -v package_min="$package_min" '
		BEGIN { FS = "[ \t]+" }
		/^mode:/ { next }
		NF < 3 { next }
		{
			split($1, parts, ":")
			file = parts[1]
			dir = file
			if (!sub(/\/[^\/]+$/, "", dir)) dir = "."
			stmts[dir] += $2
			if ($3 + 0 > 0) hit[dir] += $2
			total_stmts += $2
			if ($3 + 0 > 0) total_hit += $2
		}
		END {
			PROCINFO["sorted_in"] = "@ind_str_asc"
			if (total_stmts == 0) {
				print "coverage-gate: the profile has no statement counters"
				exit 2
			}
			for (d in stmts) {
				pct = 100 * hit[d] / stmts[d]
				verdict = (pct + 0 >= package_min + 0) ? "ok  " : "FAIL"
				if (verdict == "FAIL") failed = 1
				printf "%s %6.1f%%  %s\n", verdict, pct, d
			}
			total_pct = 100 * total_hit / total_stmts
			verdict = (total_pct + 0 >= total_min + 0) ? "ok  " : "FAIL"
			if (verdict == "FAIL") failed = 1
			printf "%s %6.1f%%  total (floor %s%%)\n", verdict, total_pct, total_min
			exit failed
		}
	' "$profile"
)" && status=0 || status=$?

printf '%s\n' "$report"

if [ "$status" -ne 0 ]; then
	if [ "$report_only" -eq 1 ]; then
		echo "coverage-gate: below floor, but --report-only is set (exit 0)"
		exit 0
	fi
	echo "coverage-gate: at least one percentage is below its floor (total $total_min%, package $package_min%)" >&2
	echo "coverage-gate: add a test for the newly reachable path, or lower the floor deliberately in this file" >&2
	exit 1
fi

echo "coverage-gate: every package and the total meet their floor"
