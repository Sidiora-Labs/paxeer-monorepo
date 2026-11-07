#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <directory_path>" >&2
    exit 1
fi
exec bash "$SCRIPT_DIR/maintenance.sh" checksums "$1"
