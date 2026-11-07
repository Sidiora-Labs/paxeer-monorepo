#!/usr/bin/env bash
# Retired custody governance selector; current governance-window qualification: tools/paxeer-x/gov/test.sh.
set -euo pipefail
if (($# > 1)); then
    echo "usage: $0 [RETIRED_ARTIFACT_DIRECTORY]" >&2
    exit 2
fi
cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
echo '108.1.10 is retired; use tools/paxeer-x/gov/test.sh for governance-window qualification' >&2
exec python3 tools/qualification/paxeer-x/custody_governance.py
