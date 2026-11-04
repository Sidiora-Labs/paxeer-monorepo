#!/usr/bin/env bash
set -euo pipefail
# Set PAX_SCRIPT_BIN to a prebuilt `go build -o pax-script ./scripts/maintenance`
# to avoid compiling this checkout's Go sources on each invocation.
SCRIPT_CWD="$PWD"
if [ -n "${PAX_SCRIPT_BIN:-}" ]; then
    exec "$PAX_SCRIPT_BIN" "$@"
fi
SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SCRIPT_ROOT"
exec go run ./scripts/maintenance --working-directory "$SCRIPT_CWD" "$@"
