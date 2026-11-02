#!/usr/bin/env bash
set -euo pipefail
if (($# > 1)); then
    echo "usage: $0 [ARTIFACT_DIRECTORY]" >&2
    exit 2
fi
cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
artifacts=${1:-${PAXEER_X_CUSTODY_GOVERNANCE_ARTIFACTS:?set the prebuilt custody governance artifact directory}}
exec python3 tools/qualification/paxeer-x/custody_governance.py \
    --manifest "$artifacts/manifest.json"
