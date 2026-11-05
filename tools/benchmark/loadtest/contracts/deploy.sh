#!/usr/bin/env bash
set -euo pipefail
printf 'Admin Key Name: ' >&2
IFS= read -r keyname
printf 'Chain ID: ' >&2
IFS= read -r chainid
printf 'paxd binary: ' >&2
IFS= read -r paxdbin
printf 'pax-chain directory: ' >&2
IFS= read -r paxhome
script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
exec "${PAX_LOADTEST_OPS:-$script_dir/../../../../build/loadtest}" benchmark-ops deploy-wasm \
  --root "$paxhome" --paxd "$paxdbin" --key "$keyname" --chain-id "$chainid"
