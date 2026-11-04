#!/usr/bin/env bash
# Render using the same compiled template exercised by the Go gate.
set -euo pipefail
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <ContractName> <OutputFile>" >&2
    exit 1
fi
exec "$(dirname "$0")/../build/paxload" scaffold scenario "$1" "$2"
