#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"
exec python3 "$root/scripts/ci/solidity_state_evidence.py"
