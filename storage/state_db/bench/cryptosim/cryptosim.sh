#!/usr/bin/env bash

# Resolve script directory (handles symlinks and relative paths).
SCRIPT_SOURCE="${BASH_SOURCE[0]}"
[[ "$SCRIPT_SOURCE" != /* ]] && SCRIPT_SOURCE="$(pwd)/${SCRIPT_SOURCE#./}"
while [[ -L "$SCRIPT_SOURCE" ]]; do
  SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_SOURCE")" && pwd)"
  SCRIPT_SOURCE="$(readlink "$SCRIPT_SOURCE")"
  [[ "$SCRIPT_SOURCE" != /* ]] && SCRIPT_SOURCE="${SCRIPT_DIR}/${SCRIPT_SOURCE}"
done
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_SOURCE")" && pwd)"
BINARY="${SCRIPT_DIR}/bin/cryptosim"

# Build binaries (no-op if already up to date; Go's build cache handles staleness).
make -C "$SCRIPT_DIR" build || exit "$?"

# Read VCS build settings from the executable just built, including dirty state.
# A failed metadata read also stops the run.
go version -m "$BINARY" || exit "$?"
sha256sum "$BINARY" || exit "$?"

# Configure paxlog env vars from the config file. The config file is the sole
# source of truth -- any pre-existing PAX_LOG_* env vars are overwritten.
if [[ $# -ge 1 && -f "$1" ]]; then
  LOGGER_OUTPUT=$("${SCRIPT_DIR}/bin/configure-logger" "$1") || {
    echo "configure-logger failed" >&2
    exit 1
  }
  eval "$LOGGER_OUTPUT"
fi

# Run the benchmark.
exec "$BINARY" "$@"
