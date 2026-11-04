#!/usr/bin/env bash
# Maintained owner: domain 8; ../node/bootstrap.sh owns bank and ledger genesis funding.
# Custody roots and relay group mounts are not initialization inputs.
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
for argument in "$@"; do
    case "$argument" in
        --custody-genesis|--custody-genesis=*|--deposit-root|--deposit-root=*|--sequencer-authorizations|--sequencer-authorizations=*)
            echo "init-chain: relay genesis inputs are retired; use native bank and ledger funding via ../node/bootstrap.sh" >&2
            exit 2 ;;
    esac
done
exec paxeer-native-tool bootstrap --bootstrap-script "$SCRIPT_DIR/../node/bootstrap.sh" "$@"
