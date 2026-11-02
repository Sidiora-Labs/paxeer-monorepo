#!/usr/bin/env bash
set -euo pipefail
umask 077
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
cd "$root"
: "${PAXEER_X_RUNTIME_EVIDENCE:?private evidence directory required}"
case "${1:-verify}" in
build)
    : "${PAXEER_X_RUNTIME_NATIVE_REVISION:?archive producer revision required}"
    mkdir -p "$PAXEER_X_RUNTIME_EVIDENCE"
    chmod 0700 "$PAXEER_X_RUNTIME_EVIDENCE"
    [[ -z $(git status --porcelain) ]] || { echo 'refuse dirty source' >&2; exit 1; }
    [[ -z $(git diff --name-only "$PAXEER_X_RUNTIME_NATIVE_REVISION" HEAD -- src include programs) ]] || { echo 'native archive source differs' >&2; exit 1; }
    ;;
verify)
    ;;
*) echo 'usage: 24.14.sh build|verify' >&2; exit 2 ;;
esac
