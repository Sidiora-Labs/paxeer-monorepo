#!/usr/bin/env bash
set -euo pipefail

if [[ $# != 3 ]]; then
    echo "Usage: $0 INITIAL_HALT_HEIGHT SNAPSHOT_INTERVAL CHAIN_ID" >&2
    exit 2
fi
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PAX_DIR="${PAX_HOME:-$HOME/.pax}"
SNAPSHOT_DIR="${SNAPSHOT_DIR:-$HOME/snapshots}"
PAXD_BIN="${PAXD_BIN:-$HOME/go/bin/paxd}"
CONTROLLER="${PAXD_CONTROLLER:-$SCRIPT_DIR/paxd-service.sh}"
[[ -x "$PAXD_BIN" && -d "$PAX_DIR" ]]
PAXD_BIN="$(cd -- "$(dirname -- "$PAXD_BIN")" && pwd)/$(basename -- "$PAXD_BIN")"
mkdir -p -- "$SNAPSHOT_DIR"
exec 9>"$PAX_DIR/.snapshot-cycle.lock"
flock -n 9
HALT_HEIGHT=$(python3 "$SCRIPT_DIR/snapshot_cycle.py" resume "$SNAPSHOT_DIR" "$1" "$2" "$3")
SNAPSHOT_INTERVAL=$2
CHAIN_ID=$3
PAXD_PID=
STAGING=
cleanup() {
    status=$?
    trap - EXIT INT TERM
    if [[ -n "$PAXD_PID" ]]; then
        kill -TERM "$PAXD_PID" 2>/dev/null || true
        wait "$PAXD_PID" || true
    fi
    if [[ -n "$STAGING" ]]; then
        rm -rf -- "$STAGING"
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# The controller owns all writers and must suppress external restarts.
"$CONTROLLER" stop "$PAX_DIR"
"$CONTROLLER" check-stopped "$PAX_DIR"
while true; do
    python3 "$SCRIPT_DIR/snapshot_cycle.py" configure "$PAX_DIR/config/app.toml" "$HALT_HEIGHT"
    STAGING=$(mktemp -d "$SNAPSHOT_DIR/.snapshot-$HALT_HEIGHT.XXXXXXXX")
    "$CONTROLLER" check-stopped "$PAX_DIR"
    "$PAXD_BIN" start --trace --home "$PAX_DIR" --chain-id "$CHAIN_ID" \
        --halt-height "$HALT_HEIGHT" >"$STAGING/node.log" 2>&1 9>&- &
    PAXD_PID=$!
    # A nonzero child exit aborts. No timer or live writer can trigger a snapshot.
    if wait "$PAXD_PID"; then
        PAXD_PID=
    else
        child_status=$?
        PAXD_PID=
        exit "$child_status"
    fi
    grep -Fq 'halting node per configuration' "$STAGING/node.log"
    "$CONTROLLER" check-stopped "$PAX_DIR"
    "$PAXD_BIN" export --home "$PAX_DIR" --height -1 --streaming=false \
        >"$STAGING/committed.json" 9>&-
    python3 "$SCRIPT_DIR/snapshot_cycle.py" committed "$STAGING/committed.json" "$HALT_HEIGHT" "$CHAIN_ID"
    "$PAXD_BIN" snapshot --height "$HALT_HEIGHT" --home "$PAX_DIR" \
        --chain-id "$CHAIN_ID" --snapshot-dir "$STAGING/snapshot" \
        >"$STAGING/snapshot.log" 2>&1 9>&-
    # Publish a checked, durable receipt; advance only after snapshot success.
    HALT_HEIGHT=$(python3 "$SCRIPT_DIR/snapshot_cycle.py" publish "$STAGING" "$SNAPSHOT_DIR" "$HALT_HEIGHT" "$SNAPSHOT_INTERVAL" "$CHAIN_ID")
    STAGING=
done
