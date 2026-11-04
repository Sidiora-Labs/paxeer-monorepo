#!/usr/bin/env bash
set -euo pipefail
# Run execution-apis .io/.iox tests against local Pax EVM RPC. Start cluster first: make docker-cluster-start
# Usage: from repo root: ./tests/integration/evm_module/scripts/evm_rpc_tests.sh
cd "$(dirname "$0")/../../../.."

CONTAINER="${PAX_EVM_IO_TX_CONTAINER:-pax-node-0}"
PASSWORD="${PAX_EVM_IO_TX_PASSWORD:-12345678}"
FROM="${PAX_EVM_IO_TX_FROM:-admin}"
RECIPIENT="${PAX_EVM_IO_TX_RECIPIENT:-0xF87A299e6bC7bEba58dbBe5a5Aa21d49bCD16D52}"
PROJECT_ROOT="${PAX_EVM_IO_PROJECT_ROOT:-/pax-protocol/pax-chain}"
CONTRACT_HEX="${PROJECT_ROOT}/tests/integration/evm_module/scripts/contracts/minimal_contract.hex"
REVERTER_HEX="${PROJECT_ROOT}/tests/integration/evm_module/scripts/contracts/reverter_contract.hex"
EVM_RPC_URL="${PAX_EVM_RPC_URL:-http://localhost:8545}"
KEYRING_ARGS=()
if [[ -n "${PAX_EVM_IO_KEYRING_BACKEND:-}" ]]; then
  KEYRING_ARGS+=(--keyring-backend "$PAX_EVM_IO_KEYRING_BACKEND")
fi

run() {
  docker exec --env PAX_EVM_IO_PASSWORD="$PASSWORD" "$CONTAINER" /bin/bash -c \
    'export PATH=$PATH:/root/go/bin && printf "%s\n" "$PAX_EVM_IO_PASSWORD" | "$@"' bash "$@"
}

# Setup is mandatory: do not run suites with missing fixture bindings.
FROM_ADDR=$(run paxd keys show "$FROM" "${KEYRING_ARGS[@]}" -a)
if [[ -z "$FROM_ADDR" ]]; then
  echo "ERROR: sender address is empty" >&2
  exit 1
fi

get_from_seq() {
  run paxd q account "$FROM_ADDR" -o json | jq -er '
    .sequence | select(type == "string" and test("^[0-9]+$"))'
}

wait_from_seq_advance() {
  local prev="$1" cur deadline
  if [[ ! "$prev" =~ ^[0-9]+$ ]]; then
    echo "ERROR: invalid sender sequence: $prev" >&2
    return 1
  fi
  deadline=$((SECONDS + 5))
  while [ "$SECONDS" -lt "$deadline" ]; do
    cur=$(get_from_seq) || return 1
    if [ "$cur" -gt "$prev" ]; then
      return 0
    fi
    sleep 0.5
  done
  echo "ERROR: sender sequence did not advance past $prev within 5s" >&2
  return 1
}

# CmdDeployContract emits exactly 'Transaction hash: <hash>'. Do not accept
# an unrelated address or arbitrary hex from an error message as the tx hash.
deploy_tx_hash() {
  local hash
  hash=$(printf '%s\n' "$1" | awk '/^Transaction hash: / {print $3}')
  if [[ ! "$hash" =~ ^0x[[:xdigit:]]{64}$ ]]; then
    echo "ERROR: deployment did not return a transaction hash: $1" >&2
    return 1
  fi
  printf '%s\n' "$hash"
}

wait_deploy_receipt() {
  local tx="$1" response attempt
  for attempt in {1..30}; do
    response=$(docker exec "$CONTAINER" curl --fail --silent --show-error --max-time 5 \
      -X POST -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$tx\"]}" "$EVM_RPC_URL") || return 1
    printf '%s\n' "$response" | jq -e '.error == null and has("result")' >/dev/null || return 1
    if printf '%s\n' "$response" | jq -e '.result != null' >/dev/null; then
      if ! printf '%s\n' "$response" | jq -e --arg tx "$tx" '
        .result | .status == "0x1" and .transactionHash == $tx
        and (.blockNumber | type == "string" and test("^0x[1-9a-fA-F][0-9a-fA-F]*$"))
        and (.contractAddress | type == "string" and test("^0x[0-9a-fA-F]{40}$")
          and . != "0x0000000000000000000000000000000000000000")' >/dev/null; then
        echo "ERROR: deployment receipt is unsuccessful or invalid: $response" >&2
        return 1
      fi
      printf '%s\n' "$response"
      return 0
    fi
    sleep 1
  done
  echo "ERROR: deployment receipt unavailable after 30 attempts: $tx" >&2
  return 1
}

# Poll sequences between signed submissions; all setup failures are fatal.
docker exec "$CONTAINER" /bin/bash -c "tr -d '[:space:]' < \"$CONTRACT_HEX\" > /tmp/minimal_contract.hex"
SEQ_BEFORE_ASSOC=$(get_from_seq)
ASSOC_OUT=$(run paxd tx evm associate-address --from "$FROM" "${KEYRING_ARGS[@]}" --chain-id pax --evm-rpc "$EVM_RPC_URL" -b sync -y)
printf '%s\n' "${ASSOC_OUT#Response: }" | jq -e '.error == null and has("result")' >/dev/null
wait_from_seq_advance "$SEQ_BEFORE_ASSOC"
SEQ_BEFORE_SEND=$(get_from_seq)
run paxd tx evm send "$RECIPIENT" 1 --from "$FROM" "${KEYRING_ARGS[@]}" --chain-id pax --evm-rpc "$EVM_RPC_URL" -b sync -y
wait_from_seq_advance "$SEQ_BEFORE_SEND"
SEQ_BEFORE_MINIMAL=$(get_from_seq)
DEPLOY_OUT=$(run paxd tx evm deploy /tmp/minimal_contract.hex --from "$FROM" "${KEYRING_ARGS[@]}" --chain-id pax --evm-rpc "$EVM_RPC_URL" -b sync -y)
DEPLOY_TX=$(deploy_tx_hash "$DEPLOY_OUT")
RECEIPT=$(wait_deploy_receipt "$DEPLOY_TX")
PAX_EVM_IO_SEED_BLOCK=$(printf '%s\n' "$RECEIPT" | jq -er '.result.blockNumber')
PAX_EVM_IO_DEPLOY_TX_HASH=$DEPLOY_TX
export PAX_EVM_IO_SEED_BLOCK PAX_EVM_IO_DEPLOY_TX_HASH

# Deploy the reverter only after the minimal deployment committed.
docker exec "$CONTAINER" /bin/bash -c "tr -d '[:space:]' < \"$REVERTER_HEX\" > /tmp/reverter_contract.hex"
wait_from_seq_advance "$SEQ_BEFORE_MINIMAL"
REVERTER_OUT=$(run paxd tx evm deploy /tmp/reverter_contract.hex --from "$FROM" "${KEYRING_ARGS[@]}" --chain-id pax --evm-rpc "$EVM_RPC_URL" -b sync -y)
REVERTER_TX=$(deploy_tx_hash "$REVERTER_OUT")
REVERTER_RECEIPT=$(wait_deploy_receipt "$REVERTER_TX")
PAX_EVM_IO_REVERTER_ADDRESS=$(printf '%s\n' "$REVERTER_RECEIPT" | jq -er '.result.contractAddress')
export PAX_EVM_IO_REVERTER_ADDRESS

: "${PAX_EVM_IO_SEED_BLOCK:?missing committed seed block}"
: "${PAX_EVM_IO_DEPLOY_TX_HASH:?missing committed deploy transaction}"
: "${PAX_EVM_IO_REVERTER_ADDRESS:?missing committed reverter contract}"

export PAX_EVM_IO_RUN_INTEGRATION=1
go test ./tests/integration/evm_module/rpc_io_test/ -v -count=1

# WebSocket integration tests (eth_subscribe et al.). Lives in a sibling
# package because the .io/.iox framework cannot represent streaming
# methods. The test itself is consensus-mode agnostic, so the same
# invocation works under standard CometBFT and Autobahn clusters alike.
export PAX_EVM_WS_RUN_INTEGRATION=1
go test ./tests/integration/evm_module/ws_test/ -v -count=1
