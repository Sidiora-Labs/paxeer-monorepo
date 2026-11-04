#!/usr/bin/env bash
# Sourced only by coordinator-owned stages, from the mounted repository.
set -euo pipefail
umask 077
python3 docker/localnode/scripts/localnode_runtime.py check-stage
: "${PAX_RUN_DIR:?}" "${PAX_HOME:?}" "${PAXD_BIN:?}" "${PAX_PRIVATE_DIR:?}"
SHARED_DIR="$PAX_RUN_DIR"
paxd() {
    command "$PAXD_BIN" --home "$PAX_HOME" "$@"
}
