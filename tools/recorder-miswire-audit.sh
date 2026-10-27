#!/usr/bin/env bash
# Fleet recorder mis-wire audit (T20).
#
# The trap: `WithHealthRecorder(...)` only has an effect on the injector path
# (`health.New(injector, ...)`). The function-path constructors
# (`NewWithHealthCheck`, `NewWithDetailedCheck`, `NewChecks`) own batch execution
# themselves, so a recorder passed to them would be accepted and silently
# ignored: no callbacks, no traces, no error. v0.8.0 turned that into a
# Start-time `ErrRecorderNotApplicable`, but every consumer pinned below v0.8.0
# still fails silently — which is why the class needs a static audit and not
# just a runtime guard.
#
# The rule: a function-path constructor call whose own argument list also
# contains `WithHealthRecorder` is a mis-wire. Each file is reduced to a
# "shadow" copy with comments and string/char literals blanked out (offsets and
# newlines preserved), then every constructor call is read to its matching
# closing paren on that shadow. That keeps `// example: NewChecks(fn,
# WithHealthRecorder(r))` in a doc comment from firing and makes nested calls
# safe.
#
# What it does not catch, on purpose:
#   - a recorder whose method signature does not match `HealthRecorder`
#     (that is a compile error, not a silent one);
#   - a constructor aliased into another identifier before the call.
# Both are reported by the compiler; this audit covers the silent half.
#
# Usage:
#   recorder-miswire-audit.sh [PATH...]
#
# Paths default to the local checkouts of the repos in tools/fleet-repos.txt,
# resolved under $FLEET_ROOT (default: $HOME/projects). A repo that is not
# checked out locally is reported as SKIP, never as a pass.
#
# Exit codes: 0 = no mis-wire found, 1 = at least one mis-wire, 2 = usage.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
manifest="$here/fleet-repos.txt"
fleet_root="${FLEET_ROOT:-$HOME/projects}"

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
	sed -n '2,38p' "$0" | sed 's/^# \{0,1\}//'
	exit 0
fi

scan_program='
function shadow_line(line,    n, k, c, d, out, pad) {
	n = length(line)
	out = ""
	k = 1
	while (k <= n) {
		c = substr(line, k, 1)
		d = substr(line, k + 1, 1)
		if (in_block) {
			if (c == "*" && d == "/") { in_block = 0; out = out "  "; k += 2; continue }
			out = out " "
			k++
			continue
		}
		if (in_back) {
			if (c == "`") in_back = 0
			out = out " "
			k++
			continue
		}
		if (in_str) {
			if (esc) esc = 0
			else if (c == "\\") esc = 1
			else if (c == "\"") in_str = 0
			out = out " "
			k++
			continue
		}
		if (c == "/" && d == "/") {
			pad = ""
			while (length(pad) < n - k + 1) pad = pad " "
			out = out pad
			k = n + 1
			continue
		}
		if (c == "/" && d == "*") { in_block = 1; out = out "  "; k += 2; continue }
		if (c == "\"") { in_str = 1; out = out " "; k++; continue }
		if (c == "`") { in_back = 1; out = out " "; k++; continue }
		out = out c
		k++
	}
	return out
}
function scan(text, file,    n, i, rest, p, j, c, depth, call, nl, parts) {
	n = length(text)
	i = 1
	while (i <= n) {
		rest = substr(text, i)
		if (!match(rest, /(NewWithHealthCheck|NewWithDetailedCheck|NewChecks)\(/)) return
		p = i + RSTART - 1
		j = p + RLENGTH - 1
		depth = 0
		while (j <= n) {
			c = substr(text, j, 1)
			if (c == "(") depth++
			else if (c == ")") {
				depth--
				if (depth == 0) { j++; break }
			}
			j++
		}
		call = substr(text, p, j - p)
		if (call ~ /WithHealthRecorder/) {
			nl = split(substr(text, 1, p), parts, "\n")
			printf "FAIL %s:%d %s passes WithHealthRecorder to a function-path constructor\n", \
				file, nl, substr(call, 1, index(call, "("))
		}
		i = j + 1
	}
}
{ shadow = shadow shadow_line($0) "\n" }
END { scan(shadow, FILENAME) }
'

bad=0
files=0

if [ "$#" -gt 0 ]; then
	roots=("$@")
else
	roots=()
	while read -r repo modpath; do
		[ -n "$repo" ] || continue
		case "$repo" in \#*) continue ;; esac
		roots+=("$fleet_root/${repo##*/}")
	done <"$manifest"
fi

for root in "${roots[@]}"; do
	if [ ! -d "$root" ]; then
		echo "SKIP $root (not checked out under $fleet_root)"
		continue
	fi

	found=0
	while IFS= read -r file; do
		files=$((files + 1))
		out="$(awk "$scan_program" "$file")" || true
		if [ -n "$out" ]; then
			printf '%s\n' "$out"
			found=1
			bad=$((bad + $(printf '%s\n' "$out" | grep -c '^FAIL ')))
		fi
	done < <(
		find "$root" -name '*.go' \
			-not -path '*/vendor/*' \
			-not -path '*/node_modules/*' \
			-not -path '*/.git/*' \
			-not -path '*/testdata/*' |
			sort
	)

	if [ "$found" -eq 0 ]; then
		echo "OK   $root"
	fi
done

echo "---"
echo "recorder-miswire: scanned $files Go files"

if [ "$bad" -gt 0 ]; then
	echo "recorder-miswire: $bad mis-wired call(s) — use health.New(injector, ...) for recorder callbacks," >&2
	echo "recorder-miswire: or drop WithHealthRecorder from the function-path constructor" >&2
	exit 1
fi

echo "recorder-miswire: no recorder passed to a function-path constructor"
