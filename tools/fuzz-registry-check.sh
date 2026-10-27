#!/usr/bin/env bash
# Fuzz-registry parity check: the flake's `fuzz` and `fuzz-long` apps must list
# exactly the `Fuzz*` targets that exist in the tree — no more, no fewer, and
# the two lists must agree with each other. Both lists are hand-maintained
# shell text in flake.nix, so before this check existed a new target could be
# written in Go and silently never fuzzed (the flake comment claimed a
# pre-push hook did this diff; no such hook existed in this repo).
#
# Read-only. Exit 0 = registry and tree agree, 1 = drift, 2 = cannot run.
set -euo pipefail

if [ $# -gt 1 ]; then
	echo "usage: fuzz-registry-check.sh [REPO_DIR]" >&2
	exit 2
fi

cd "${1:-.}"

if [ ! -f flake.nix ]; then
	echo "fuzz-registry: flake.nix not found in $(pwd)" >&2
	exit 2
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# Registry rows: "<block>|<pkg>|<Target>" parsed from the two mkApp blocks.
awk '
	/fuzz = mkApp "fuzz"/      { block = "fuzz"; next }
	/fuzz-long = mkApp/        { block = "fuzz-long"; next }
	/^[[:space:]]*.{2};$/      { block = ""; next }
	block != "" && /-fuzz=/ {
		pkg = ""
		target = ""
		for (i = 1; i < NF; i++) {
			if ($i == "go" && $(i + 1) == "test") { pkg = $(i + 2) }
			if ($i ~ /^-fuzz=/) { split($i, parts, "="); target = parts[2] }
		}
		if (pkg != "" && target != "") { print block "|" pkg "|" target }
	}
' flake.nix | sort -u >"$tmp/registry"

# Tree rows: "<pkg>|<Target>" for every `func Fuzz...` in a test file.
# Nested modules (tools/*) are their own build units and are excluded.
nested=$(find . -mindepth 2 -name go.mod -printf '%h\n' 2>/dev/null || true)

find . -name '*_test.go' -not -path './.git/*' -print0 |
	while IFS= read -r -d '' file; do
		skip=0
		for dir in $nested; do
			case "$file" in
			"$dir"/*) skip=1 ;;
			esac
		done
		[ "$skip" -eq 1 ] && continue

		pkg="."
		case "$file" in
		./*/*) pkg="./${file#./}" && pkg=$(dirname "$pkg") ;;
		esac

		{ grep -oE '^func Fuzz[A-Za-z0-9_]+' "$file" || true; } |
			sed -e 's/^func //' -e "s|^|$pkg\||"
	done | sort -u >"$tmp/tree"

fail=0
drift() {
	echo "fuzz-registry: DRIFT: $*" >&2
	fail=1
}

for block in fuzz fuzz-long; do
	awk -F'|' -v b="$block" '$1 == b { print $2 "|" $3 }' "$tmp/registry" |
		sort -u >"$tmp/$block"

	if [ ! -s "$tmp/$block" ]; then
		drift "no fuzz targets parsed from the $block app in flake.nix"
		continue
	fi

	while IFS='|' read -r pkg target; do
		[ -n "$target" ] || continue
		grep -qxF "$pkg|$target" "$tmp/tree" ||
			drift "$block lists $target for $pkg, but no such target exists in the tree"
	done <"$tmp/$block"

	while IFS='|' read -r pkg target; do
		[ -n "$target" ] || continue
		grep -qxF "$pkg|$target" "$tmp/$block" ||
			drift "$pkg defines $target, but $block does not run it"
	done <"$tmp/tree"
done

if ! diff -q "$tmp/fuzz" "$tmp/fuzz-long" >/dev/null 2>&1; then
	drift "fuzz and fuzz-long list different targets (they must stay in lockstep)"
	echo "--- fuzz only ---" >&2
	comm -23 "$tmp/fuzz" "$tmp/fuzz-long" >&2 || true
	echo "--- fuzz-long only ---" >&2
	comm -13 "$tmp/fuzz" "$tmp/fuzz-long" >&2 || true
fi

if [ "$fail" -eq 0 ]; then
	targets=$(wc -l <"$tmp/fuzz" | tr -d ' ')
	echo "fuzz-registry: OK - $targets targets registered in both fuzz apps"
fi

exit "$fail"
