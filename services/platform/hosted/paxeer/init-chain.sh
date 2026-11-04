#!/usr/bin/env bash
# Compatibility entrypoint for the maintained native bootstrap. Supply its native
# --data-dir, --native-inputs and signing-file arguments explicitly.
set -euo pipefail
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
for name in "${!LAYERX_PAXEER_@}"; do
    if [ -n "$name" ]; then
        echo "init-chain: legacy LAYERX_PAXEER configuration is retired; use ../node/bootstrap.sh with authoritative native genesis inputs" >&2
        exit 2
    fi
done
exec "$SCRIPT_DIR/../node/bootstrap.sh" "$@"
