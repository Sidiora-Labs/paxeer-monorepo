#!/bin/bash

set -euo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
HARNESS="$SCRIPTDIR/harness.py"

display_usage() {
	echo "\nMissing $1 parameter. Please check if all parameters were specified."
	echo "\nUsage: ./one-chain [BINARY] [CHAIN_ID] [CHAIN_DIR] [RPC_PORT] [P2P_PORT] [PROFILING_PORT] [GRPC_PORT]"
  echo "\nExample: ./one-chain $BINARY test-chain-id ./data 26657 26656 6060 9090 \n"
  exit 1
}

KEYRING=--keyring-backend="test"
SILENT=1

redirect() {
  if [ "$SILENT" -eq 1 ]; then
    "$@" > /dev/null 2>&1
  else
    "$@"
  fi
}

BINARY=${1:-}
CHAINID=${2:-}
CHAINDIR=${3:-}
RPCPORT=${4:-}
P2PPORT=${5:-}
PROFPORT=${6:-}
GRPCPORT=${7:-}

if [ -z "$BINARY" ]; then
  display_usage "[BINARY] ($BINARY|akash)"
fi

if [ -z "$CHAINID" ]; then
  display_usage "[CHAIN_ID]"
fi

if [ -z "$CHAINDIR" ]; then
  display_usage "[CHAIN_DIR]"
fi

if [ -z "$RPCPORT" ]; then
  display_usage "[RPC_PORT]"
fi

if [ -z "$P2PPORT" ]; then
  display_usage "[P2P_PORT]"
fi

if [ -z "$PROFPORT" ]; then
  display_usage "[PROFILING_PORT]"
fi

if [ -z "$GRPCPORT" ]; then
  display_usage "[GRPC_PORT]"
fi

python3 "$HARNESS" ports "$RPCPORT" "$P2PPORT" "$PROFPORT" "$GRPCPORT"
CHAINDIR="$(mkdir -p -- "$CHAINDIR" && cd "$CHAINDIR" && pwd -P)"
python3 "$HARNESS" chain-home "$CHAINDIR" "$CHAINID"
python3 "$HARNESS" stop "$CHAINDIR" "$CHAINID"
BINARY="$(command -v "$BINARY")"
child=""
cleanup() {
  result=$?
  trap - EXIT INT TERM
  if [[ -n "$child" ]]; then
    if [[ ! -f "$CHAINDIR/pids/$CHAINID.json" ]]; then
      python3 "$HARNESS" record "$CHAINDIR" "$CHAINID" "$child" "$BINARY" || exit 1
    fi
    if python3 "$HARNESS" stop "$CHAINDIR" "$CHAINID" "$child"; then
      wait "$child" || true
    else
      result=1
    fi
  fi
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

echo "Creating $BINARY instance: home=$CHAINDIR | chain-id=$CHAINID | p2p=:$P2PPORT | rpc=:$RPCPORT | profiling=:$PROFPORT | grpc=:$GRPCPORT"

# Add dir for chain, exit if error
if ! mkdir -p "$CHAINDIR/$CHAINID" 2>/dev/null; then
    echo "Failed to create chain folder. Aborting..."
    exit 1
fi

# Build genesis file incl account for passed address
chain_one_coins="100000000000stake,100000000000umuon,100000000000test"
chain_two_coins="100000000000stake,100000000000umuon"
delegate="100000000000stake"

redirect "$BINARY" --home "$CHAINDIR/$CHAINID" --chain-id $CHAINID init $CHAINID
sleep 1
"$BINARY" --home "$CHAINDIR/$CHAINID" keys add validator $KEYRING --output json > "$CHAINDIR/$CHAINID/validator_seed.json" 2> /dev/null
sleep 1
"$BINARY" --home "$CHAINDIR/$CHAINID" keys add user $KEYRING --output json > "$CHAINDIR/$CHAINID/key_seed.json" 2> /dev/null
sleep 1
redirect "$BINARY" --home "$CHAINDIR/$CHAINID" add-genesis-account $("$BINARY" --home "$CHAINDIR/$CHAINID" keys $KEYRING show user -a) $chain_one_coins
sleep 1
redirect "$BINARY" --home "$CHAINDIR/$CHAINID" add-genesis-account $("$BINARY" --home "$CHAINDIR/$CHAINID" keys $KEYRING show validator -a) $chain_two_coins
sleep 1
redirect "$BINARY" --home "$CHAINDIR/$CHAINID" gentx validator $delegate $KEYRING --chain-id $CHAINID
sleep 1
redirect "$BINARY" --home "$CHAINDIR/$CHAINID" collect-gentxs
sleep 1

# Check platform
platform='unknown'
unamestr=`uname`
if [ "$unamestr" = 'Linux' ]; then
   platform='linux'
fi

# Set proper defaults and change ports (use a different sed for Mac or Linux)
echo "Change settings in config.toml file..."
if [ $platform = 'linux' ]; then
  sed -i 's#"tcp://127.0.0.1:26657"#"tcp://0.0.0.0:'"$RPCPORT"'"#g' "$CHAINDIR/$CHAINID/config/config.toml"
  sed -i 's#"tcp://0.0.0.0:26656"#"tcp://0.0.0.0:'"$P2PPORT"'"#g' "$CHAINDIR/$CHAINID/config/config.toml"
  sed -i 's#"localhost:6060"#"localhost:'"$PROFPORT"'"#g' "$CHAINDIR/$CHAINID/config/config.toml"
  sed -i 's/timeout_commit = "5s"/timeout_commit = "1s"/g' "$CHAINDIR/$CHAINID/config/config.toml"
  sed -i 's/timeout_propose = "3s"/timeout_propose = "1s"/g' "$CHAINDIR/$CHAINID/config/config.toml"
  sed -i 's/index_all_keys = false/index_all_keys = true/g' "$CHAINDIR/$CHAINID/config/config.toml"
  # sed -i '' 's#index-events = \[\]#index-events = \["message.action","send_packet.packet_src_channel","send_packet.packet_sequence"\]#g' $CHAINDIR/$CHAINID/config/app.toml
else
  sed -i '' 's#"tcp://127.0.0.1:26657"#"tcp://0.0.0.0:'"$RPCPORT"'"#g' "$CHAINDIR/$CHAINID/config/config.toml"
  sed -i '' 's#"tcp://0.0.0.0:26656"#"tcp://0.0.0.0:'"$P2PPORT"'"#g' "$CHAINDIR/$CHAINID/config/config.toml"
  sed -i '' 's#"localhost:6060"#"localhost:'"$PROFPORT"'"#g' "$CHAINDIR/$CHAINID/config/config.toml"
  sed -i '' 's/timeout_commit = "5s"/timeout_commit = "1s"/g' "$CHAINDIR/$CHAINID/config/config.toml"
  sed -i '' 's/timeout_propose = "3s"/timeout_propose = "1s"/g' "$CHAINDIR/$CHAINID/config/config.toml"
  sed -i '' 's/index_all_keys = false/index_all_keys = true/g' "$CHAINDIR/$CHAINID/config/config.toml"
  # sed -i '' 's#index-events = \[\]#index-events = \["message.action","send_packet.packet_src_channel","send_packet.packet_sequence"\]#g' $CHAINDIR/$CHAINID/config/app.toml
fi

# Keep the generated gRPC listener consistent with the startup override.
if [ "$platform" = 'linux' ]; then
  sed -i 's#"0.0.0.0:9090"#"0.0.0.0:'"$GRPCPORT"'"#g' "$CHAINDIR/$CHAINID/config/app.toml"
else
  sed -i '' 's#"0.0.0.0:9090"#"0.0.0.0:'"$GRPCPORT"'"#g' "$CHAINDIR/$CHAINID/config/app.toml"
fi

# Verify all four generated listeners before starting.
python3 "$HARNESS" config "$CHAINDIR/$CHAINID/config/config.toml" "$RPCPORT" "$P2PPORT" "$PROFPORT" "$GRPCPORT"
"$BINARY" --home "$CHAINDIR/$CHAINID" start --pruning=nothing --grpc.address="0.0.0.0:$GRPCPORT" > "$CHAINDIR/$CHAINID.log" 2>&1 &
child=$!
python3 "$HARNESS" record "$CHAINDIR" "$CHAINID" "$child" "$BINARY"
exit_code=0
wait "$child" || exit_code=$?
child=""
exit "$exit_code"
