#!/usr/bin/env bash
set -euo pipefail
script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
exec "${PAX_LOADTEST_OPS:-$script_dir/../../../../build/loadtest}" benchmark-ops generate-bindings \
  --bindings "$script_dir/../contracts/evm/bindings" --abigen "${ABIGEN_BIN:-abigen}" "$@"
