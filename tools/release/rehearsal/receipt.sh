#!/usr/bin/env bash

write_receipt() {
	local result=$1 temporary
	[ -n "$receipt" ] || return 0
	mkdir -p "$(dirname "$receipt")"
	temporary=$(mktemp "${receipt}.XXXXXX")
	if [ "$result" = pass ]; then
		jq -n --arg version "$plan" --argjson nodes "$nodes" --argjson served "$precompiles_served" \
			--argjson pools "$pools" --argjson markets "$markets" --argjson fresh "$oracle_fresh" \
			--argjson feed_keys "$feed_keys" --arg sha "$new_sha" \
			'{version: $version, plan: $version, nodes_checked: $nodes, nodes_passed: $nodes,
			  precompiles_served: $served, pools: $pools, markets: $markets, oracle_fresh: $fresh,
			  feed_keys: $feed_keys, rollback_rehearsed: true, new_bin_sha256: $sha, result: "pass"}' >"$temporary"
	else
		jq -n --arg version "$plan" --arg sha "$new_sha" \
			'{version: $version, plan: $version, rollback_rehearsed: false, new_bin_sha256: $sha, result: "fail"}' >"$temporary"
	fi
	mv -fT -- "$temporary" "$receipt"
}

write_rehearsal_handoff() {
	local output=$1
	python3 - "$release" "$rehearsal_env" "$pools_file" "$output" <<'PY_HANDOFF'
from pathlib import Path
import sys

sys.path.insert(0, sys.argv[1])
from fleet_check import write_rehearsal_handoff

write_rehearsal_handoff(*map(Path, sys.argv[2:]))
PY_HANDOFF
}
