#!/usr/bin/env bash
set -euo pipefail
if (($#)); then
    echo "usage: $0 (no arguments)" >&2
    exit 2
fi
cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
# The fee-relay removal keeps this governed-schedule qualification runner.
exec python3 tools/qualification/paxeer-x/program_fee_governance.py
