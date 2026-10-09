#!/usr/bin/env bash
# ServiceName migration scanner (staging, A21): detects call sites that
# break when WithCriticalServices/NewChecks/NewWithHealthHealthCheck move to
# the typed health.ServiceName. Untyped literals/consts survive (assignable);
# only typed-string values and []string spreads break. Detection only — the
# v0.6 rewrite is human-reviewed per docs/servicename-design.md.
set -euo pipefail

if [ $# -eq 0 ]; then
	echo "usage: servicename-scan.sh REPO_DIR [REPO_DIR...]" >&2
	exit 2
fi

for repo in "$@"; do
	name=$(basename "$repo")
	rg -n --type go -g '!*_test.go' -g '!vendor' \
		-e 'WithCriticalServices\(' -e 'health\.NewChecks\(' -e 'NewWithHealthCheck\(' \
		"$repo" 2>/dev/null |
		while IFS= read -r line; do
			file=${line%%:*}
			rest=${line#*:}
			lineno=${rest%%:*}
			text=${rest#*:}
			breaks=""
			case "$text" in
			*"...)"*) breaks="SPREAD([]string -> []ServiceName)" ;;
			*GetType\[*) breaks="TYPED-GETTER(wrap in ServiceName(...))" ;;
			esac
			if [ -n "$breaks" ]; then
				printf '%s\t%s:%s\t%s\t%s\n' "$name" "${file#"$repo"/}" "$lineno" "$breaks" "$text"
			fi
		done
done
