#!/bin/bash

# This script is used to deploy the NoopToken contract to the target network
# This avoids trying to predict what address it might be deployed to

BENCH_DEPLOYER_KEY() {
  if [[ -z "${BENCH_DEPLOYER_KEY:-}" ]]; then
    printf 'BENCH_DEPLOYER_KEY must be set\n' >&2
    exit 1
  fi
}
BENCH_DEPLOYER_KEY

evm_endpoint=$1

# first fund account if necessary
THRESHOLD=100000000000000000000 # 100 Eth
ACCOUNT=$(cast wallet address --private-key "$BENCH_DEPLOYER_KEY")
BALANCE=$(cast balance $ACCOUNT --rpc-url "$evm_endpoint")
if (( $(echo "$BALANCE < $THRESHOLD" | bc -l) )); then
  printf "12345678\n" | ~/go/bin/paxd tx evm send $ACCOUNT 100000000000000000000 --from admin --evm-rpc "$evm_endpoint"
  sleep 3
fi
cd tools/benchmark/loadtest/contracts/evm || exit 1

./setup.sh > /dev/null 2>&1

/root/.foundry/bin/forge create --broadcast -r "$evm_endpoint" --private-key "$BENCH_DEPLOYER_KEY" src/NoopToken.sol:NoopToken --json --constructor-args "NoopToken" "NT"  |jq -r '.deployedTo'
