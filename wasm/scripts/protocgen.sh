#!/usr/bin/env bash

set -euo pipefail

# Compatibility entry point; use the monorepo mappings and output ownership.
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
exec bash "$repo_root/scripts/protoc.sh" "$@"
