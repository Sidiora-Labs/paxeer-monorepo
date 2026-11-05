#!/usr/bin/env bash
# coverage-gate.sh — enforce the KindleLaunch coverage gate (SECTION 17).
#
# Reads per-module coverage profiles from .cover/<module-with-slashes-as-_>.out
# (produced by `make cover`) and fails if any module is below its threshold:
#   - money / correctness-critical modules : 90%   (env MONEY="shared protocol ...")
#   - all other modules                    : 85%
#
# MODULES is the complete expected inventory. RUN_MANIFEST and COVER_RUN_ID
# identify the completed coverage_inventory.py run; every module must have a
# fresh profile or an explicit empty result from successful discovery.
set -euo pipefail

COVER_DIR="${COVER_DIR:-.cover}"
DEFAULT_MIN="${DEFAULT_MIN:-85.0}"
MONEY_MIN="${MONEY_MIN:-90.0}"
MONEY="${MONEY:-}"

if [ ! -d "$COVER_DIR" ]; then
	echo "coverage-gate: no $COVER_DIR directory (run 'make cover' first)" >&2
	exit 1
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
python3 "$SCRIPT_DIR/coverage_inventory.py" check "$COVER_DIR" \
	"${RUN_MANIFEST:?coverage-gate: RUN_MANIFEST is required}" \
	--run-id "${COVER_RUN_ID:?coverage-gate: COVER_RUN_ID is required}" \
	--modules "${MODULES:?coverage-gate: MODULES inventory is required}" || exit 1

fail=0

is_money() {
	local mod="$1" m
	for m in $MONEY; do [ "$m" = "$mod" ] && return 0; done
	return 1
}

# filter_generated <profile> <module-dir>
# Echoes the path to a copy of <profile> with all lines for generated files
# (those whose source carries the canonical "// Code generated ... DO NOT EDIT."
# marker, e.g. abigen bindings, sqlc output) removed, so machine-written code is
# never counted toward the coverage gate. If nothing is filtered, the original
# profile path is echoed unchanged. Caller removes the temp file when it differs.
filter_generated() {
	local prof="$1" mod="$2"
	[ -f "$mod/go.mod" ] || { echo "$prof"; return; }

	local modpath
	modpath="$(awk '/^module /{print $2; exit}' "$mod/go.mod")"

	local patt
	patt="$(mktemp)"
	local f rel
	while IFS= read -r f; do
		rel="${f#"$mod"/}"
		printf '%s/%s:\n' "$modpath" "$rel"
	done < <(grep -rlE '^// Code generated .* DO NOT EDIT\.$' "$mod" --include='*.go' 2>/dev/null || true) >"$patt"

	if [ ! -s "$patt" ]; then
		rm -f "$patt"
		echo "$prof"
		return
	fi

	local out
	out="$(mktemp)"
	grep -vF -f "$patt" "$prof" >"$out" || true
	rm -f "$patt"
	echo "$out"
}

printf '%-26s %8s %8s   %s\n' "MODULE" "COVER" "MIN" "STATUS"
printf '%-26s %8s %8s   %s\n' "------" "-----" "---" "------"

for mod in $MODULES; do
	prof="$COVER_DIR/${mod//\//_}.out"
	# Successful discovery of a package-free module is explicitly recorded above.
	[ -f "$prof" ] || continue

	# Empty results include header-only profiles and zero-statement blocks.
	if ! awk 'NR > 1 && $2 > 0 {found=1} END {exit !found}' "$prof"; then
		printf '%-26s %8s %8s   %s\n' "$mod" "n/a" "-" "EMPTY (no measurable statements)"
		continue
	fi

	# Drop generated files (abigen/sqlc) before measuring — only hand-written
	# code counts toward the gate.
	fprof="$(filter_generated "$prof" "$mod")"
	pct="$(go tool cover -func="$fprof" | awk '/^total:/ {gsub(/%/,"",$3); print $3}')"
	[ "$fprof" != "$prof" ] && rm -f "$fprof"
	if [ -z "$pct" ]; then
		printf '%-26s %8s %8s   %s\n' "$mod" "n/a" "-" "EMPTY (generated code only)"
		continue
	fi

	if is_money "$mod"; then min="$MONEY_MIN"; else min="$DEFAULT_MIN"; fi

	if awk -v p="$pct" -v m="$min" 'BEGIN{exit !(p+0 >= m+0)}'; then
		printf '%-26s %7s%% %7s%%   %s\n' "$mod" "$pct" "$min" "PASS"
	else
		printf '%-26s %7s%% %7s%%   %s\n' "$mod" "$pct" "$min" "FAIL"
		fail=1
	fi
done

if [ "$fail" -ne 0 ]; then
	echo "" >&2
	echo "coverage-gate: FAILED — one or more modules below threshold" >&2
	exit 1
fi
echo ""
echo "coverage-gate: PASSED"
