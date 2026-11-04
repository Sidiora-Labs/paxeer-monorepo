#!/bin/bash

set -euo pipefail

if [ "$#" -ne 1 ] || [ "$1" != "cw20_base" ]; then
  echo "Usage: $0 cw20_base" >&2
  exit 1
fi

paxdbin=$(command -v "$HOME/go/bin/paxd")
keyname=$(printf "12345678\n" | "$paxdbin" keys list --output json | jq -er '.[0].name')
keyaddress=$(printf "12345678\n" | "$paxdbin" keys show "$keyname" -a)
chainid=$("$paxdbin" status | jq -er '.NodeInfo.network')
paxhome=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
source "$(dirname "$0")/../utils/_tx_helpers.sh"
cd "$paxhome"

contract_id=$(store_wasm contracts/wasm/cw20_base.wasm)
params=$(jq -cn --arg owner "$keyaddress" \
  '{name:"TestToken",symbol:"TTK",decimals:6,initial_balances:[{address:$owner,amount:"1000000"}]}')
contract_addr=$(instantiate_wasm "$contract_id" "$params" cw20_base --no-admin)

# Exercise the supported CW20 product operation and prove its committed effect.
code=$(submit_tx_and_wait "$keyname" wasm execute "$contract_addr" \
  '{"burn":{"amount":"1"}}' --gas=500000 --fees=1000000uhpx)
[ "$code" = "0" ] || exit 1
_wait_until "CW20 burn committed" \
  "[ \$(\"\$paxdbin\" q wasm contract-state smart \"$contract_addr\" '{\"token_info\":{}}' --output json | jq -r '.data.total_supply') = 999999 ]"
info=$("$paxdbin" q wasm contract-state smart "$contract_addr" '{"token_info":{}}' --output json)
printf '%s\n' "$info" | jq -e '.data | .name == "TestToken" and .symbol == "TTK" and .decimals == 6 and .total_supply == "999999"' >/dev/null
balance_query=$(jq -cn --arg owner "$keyaddress" '{balance:{address:$owner}}')
"$paxdbin" q wasm contract-state smart "$contract_addr" "$balance_query" --output json \
  | jq -e '.data.balance == "999999"' >/dev/null
printf '%s\n' "$contract_addr" > tests/integration/contracts/cw20_base-addr.txt
printf 'Deployed CW20 contract: %s\n' "$contract_addr"
