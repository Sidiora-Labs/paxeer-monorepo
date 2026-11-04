#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <height> (offline paxd home; logical genesis export)" >&2
    exit 1
fi
PAXD_BIN="${PAXD_BIN:-$HOME/go/bin/paxd}"
PAX_HOME="${PAX_HOME:-$HOME/.pax}"
# Native storage backends must be supported by paxd export; its failure is fatal.
# The archive contains the complete genesis and a height/module-store manifest.
exec bash "$SCRIPT_DIR/maintenance.sh" export "$PAXD_BIN" "$PAX_HOME" "$1" "$HOME/state_$1.zip"
