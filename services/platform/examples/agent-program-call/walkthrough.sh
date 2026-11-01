#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 0 ]] || { echo "usage: walkthrough.sh" >&2; exit 64; }
registry=$(layerx --json program registry list)
mapfile -t candidates < <(jq -er '.data.program_ids[]' <<<"$registry")
program_id=
discovery=
for candidate in "${candidates[@]}"; do
  observed=$(layerx --json program discover "$candidate")
  if [[ $(jq -r '.data.lifecycle' <<<"$observed") == active ]]; then
    program_id=$candidate; discovery=$observed; break
  fi
done
[[ -n $program_id ]]

interface_digest=absent
if [[ $(jq -r '.data.interface_status // "published"' <<<"$discovery") == published ]]; then
  interface=$(layerx --json program interface get "$program_id")
  interface_digest=$(jq -er '.data.interface_digest' <<<"$interface")
fi
fee_limit=50000000

simulateCall() {
  layerx --json program simulate "$program_id" --fuel 1000000 --fee-limit "$fee_limit"
}

call() {
  layerx --json program call "$program_id" --fuel 1000000 --fee-limit "$fee_limit" \
    --evm-key-file "${LAYERX_EVM_KEY_FILE:?set LAYERX_EVM_KEY_FILE to the wallet derive export file}"
}

simulation=$(simulateCall)
if ! jq -e '.data.outcome.terminal_result == 0 and .data.outcome.terminal_kind == 1' <<<"$simulation" >/dev/null; then
  jq -n --arg program_id "$program_id" --arg interface_digest "$interface_digest" \
    --argjson refusal "$(jq -e '.data.outcome' <<<"$simulation")" \
    '{program_id:$program_id,interface_digest:$interface_digest,status:"typed-refusal",refusal:$refusal}'
  exit 0
fi

call_output=$(mktemp)
call_error=$(mktemp)
trap 'rm -f "$call_output" "$call_error"' EXIT
if ! call >"$call_output" 2>"$call_error"; then
  failure=$(<"$call_error")
  jq -n --arg program_id "$program_id" --arg refusal "$failure" \
    '{program_id:$program_id,status:"local-or-deterministic-refusal",refusal:$refusal,submitted:false}'
  exit 0
fi
result=$(<"$call_output")
tx_hash=$(jq -er '.data.tx_hash | select(test("^0x[0-9a-fA-F]{64}$"))' <<<"$result")
if ! resolved=$(layerx --json receipt get "$tx_hash" 2>"$call_error") \
  || ! jq -e --arg tx_hash "$tx_hash" \
    '.data.txHash == $tx_hash and .data.rung == "receipted" and
     (.data.receipt | type == "string" and test("^0x[0-9a-fA-F]+$")) and
     (.data.status == "0x1" or .data.status == "0x0")' <<<"$resolved" >/dev/null \
  || ! rung=$(node "$(dirname "${BASH_SOURCE[0]}")/verify-receipt.mjs" "$tx_hash" 2>"$call_error"); then
  jq -n --arg program_id "$program_id" --arg tx_hash "$tx_hash" --arg rung "submitted" \
    '{program_id:$program_id,tx_hash:$tx_hash,rung:$rung,status:"unknown-fate",action:"Query receipt get with this transaction hash; do not resubmit the call"}'
  exit 0
fi
jq -n --arg program_id "$program_id" --arg interface_digest "$interface_digest" \
  --arg tx_hash "$tx_hash" --arg rung "$rung" \
  --argjson outcome "$(jq '.data | {status,returnData,vmError}' <<<"$resolved")" \
  '{program_id:$program_id,interface_digest:$interface_digest,tx_hash:$tx_hash,rung:$rung,status:"called",outcome:$outcome}'
