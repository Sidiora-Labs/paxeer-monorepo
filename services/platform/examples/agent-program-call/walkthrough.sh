#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 0 ]] || { echo "usage: walkthrough.sh" >&2; exit 64; }
example_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
state_file=${LAYERX_CALL_STATE_FILE:-${XDG_STATE_HOME:-$HOME/.local/state}/layerx/agent-program-call.json}
rpc_url=${LAYERX_RPC_URL:?set LAYERX_RPC_URL to the published JSON-RPC endpoint ending in /rpc}

if [[ -e $state_file || -L $state_file ]]; then
  if ! result=$(node "$example_dir/submission.mjs" recover "$state_file" "$rpc_url"); then
    jq . <<<"$result"
    exit 75
  fi
else
  registry=$(layerx --rpc "$rpc_url" --json program registry list)
  mapfile -t candidates < <(jq -er '.data.program_ids[]' <<<"$registry")
  program_id=
  discovery=
  for candidate in "${candidates[@]}"; do
    observed=$(layerx --rpc "$rpc_url" --json program discover "$candidate")
    if [[ $(jq -r '.data.lifecycle' <<<"$observed") == active ]]; then
      program_id=$candidate; discovery=$observed; break
    fi
  done
  [[ -n $program_id ]]

  interface_digest=absent
  if [[ $(jq -r '.data.interface_status // "published"' <<<"$discovery") == published ]]; then
    interface=$(layerx --rpc "$rpc_url" --json program interface get "$program_id")
    interface_digest=$(jq -er '.data.interface_digest' <<<"$interface")
  fi
  fee_limit=50000000

  simulateCall() {
    layerx --rpc "$rpc_url" --json program simulate "$program_id" --fuel 1000000 --fee-limit "$fee_limit"
  }

  simulation=$(simulateCall)
  if ! jq -e '.data.outcome.terminal_result == 0 and .data.outcome.terminal_kind == 1' <<<"$simulation" >/dev/null; then
    jq -n --arg program_id "$program_id" --arg interface_digest "$interface_digest" \
      --argjson refusal "$(jq -e '.data.outcome' <<<"$simulation")" \
      '{program_id:$program_id,interface_digest:$interface_digest,status:"typed-refusal",refusal:$refusal}'
    exit 0
  fi

  if ! result=$(node "$example_dir/submission.mjs" submit "$state_file" "$rpc_url" \
    "$program_id" "$interface_digest" \
    "${LAYERX_EVM_KEY_FILE:?set LAYERX_EVM_KEY_FILE to the wallet derive export file}"); then
    jq . <<<"$result"
    exit 75
  fi
fi

program_id=$(jq -er '.program_id' <<<"$result")
interface_digest=$(jq -er '.interface_digest' <<<"$result")
if [[ $(jq -r '.submitted' <<<"$result") == false ]]; then
  jq . <<<"$result"
  exit 1
fi
if ! jq -e '.tx_hash | type == "string" and test("^0x[0-9a-f]{64}$")' <<<"$result" >/dev/null; then
  jq . <<<"$result"
  exit 75
fi
call_error=$(mktemp)
trap 'rm -f "$call_error"' EXIT
tx_hash=$(jq -er '.tx_hash | select(test("^0x[0-9a-f]{64}$"))' <<<"$result")
if ! resolved=$(layerx --rpc "$rpc_url" --json receipt get "$tx_hash" 2>"$call_error") \
  || ! jq -e --arg tx_hash "$tx_hash" \
    '.data.txHash == $tx_hash and .data.rung == "receipted" and
     (.data.receipt | type == "string" and test("^0x[0-9a-fA-F]+$")) and
     (.data.status == "0x1" or .data.status == "0x0")' <<<"$resolved" >/dev/null \
  || ! rung=$(node "$example_dir/verify-receipt.mjs" "$tx_hash" 2>"$call_error"); then
  jq -n --arg program_id "$program_id" --arg tx_hash "$tx_hash" --arg rung "submitted" \
    '{program_id:$program_id,tx_hash:$tx_hash,rung:$rung,status:"unknown-fate",action:"Query receipt get with this transaction hash; do not resubmit the call"}'
  exit 75
fi
jq -n --arg program_id "$program_id" --arg interface_digest "$interface_digest" \
  --arg tx_hash "$tx_hash" --arg rung "$rung" \
  --argjson outcome "$(jq '.data | {status,returnData,vmError}' <<<"$resolved")" \
  '{program_id:$program_id,interface_digest:$interface_digest,tx_hash:$tx_hash,rung:$rung,status:"called",outcome:$outcome}'
