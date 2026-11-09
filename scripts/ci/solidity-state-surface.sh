#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"
command -v forge >/dev/null 2>&1 || {
    echo "solidity-state-surface: forge is required; command: sh scripts/ci/solidity-state-surface.sh" >&2
    exit 127
}
exec python3 "$root/scripts/ci/solidity_state_evidence.py"
