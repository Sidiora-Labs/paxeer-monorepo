#!/usr/bin/env bash
set -euo pipefail
exec python3 "$(dirname -- "$(realpath -- "${BASH_SOURCE[0]}")")/node_maintenance.py" state-sync "$@"
