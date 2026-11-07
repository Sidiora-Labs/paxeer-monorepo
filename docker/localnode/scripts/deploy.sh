#!/usr/bin/env bash
set -euo pipefail

# All nodes must receive the same fresh LOCALNODE_RUN_ID and explicit local consent.
# The coordinator exclusively creates build/localnode-runs/$LOCALNODE_RUN_ID;
# atomic per-node results and failures are consumed under one startup deadline.
export GOPATH="${GOPATH:-$HOME/go}"
export GOBIN="$GOPATH/bin"
export BUILD_PATH="$PWD/build"
export PATH="$GOBIN:$PATH:/usr/local/go/bin:$BUILD_PATH"
mkdir -p "$GOBIN"
exec python3 docker/localnode/scripts/localnode_runtime.py deploy
