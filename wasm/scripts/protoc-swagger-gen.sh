#!/usr/bin/env bash

set -euo pipefail

# Compatibility entry point; the root Buf workspace owns imports and versions.
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
exec bash "$repo_root/scripts/protoc.sh" --wasm-swagger "$@"
