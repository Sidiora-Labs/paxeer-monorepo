#!/usr/bin/env bash
set -euo pipefail
script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
exec "${PAX_LOADTEST_OPS:-$script_dir/../../../../build/loadtest}" benchmark-ops deploy-ten \
  --paxd "${PAXD_BIN:-$HOME/go/bin/paxd}" --config "$script_dir/../config.json" "$@"
