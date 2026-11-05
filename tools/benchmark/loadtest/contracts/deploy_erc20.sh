#!/usr/bin/env bash
set -euo pipefail
: "${BENCH_DEPLOYER_KEY:?BENCH_DEPLOYER_KEY must be set}"
: "${1:?EVM endpoint is required}"
script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
exec "${PAX_LOADTEST_OPS:-$script_dir/../../../../build/loadtest}" benchmark-ops deploy-evm \
  --root "$script_dir/evm" --contract erc20 --endpoint "$1" --paxd "${PAXD_BIN:-$HOME/go/bin/paxd}"
