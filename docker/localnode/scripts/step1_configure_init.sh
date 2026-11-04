#!/usr/bin/env bash
source docker/localnode/scripts/localnode_common.sh

# Input parameters
NODE_ID=${ID:-0}
NUM_ACCOUNTS=${NUM_ACCOUNTS:-5}
echo "Configure and initialize environment"


# Prepare shared folders
NODE_DIR="$SHARED_DIR/node_${NODE_ID}"
mkdir -m 700 -p "$NODE_DIR/gentx" "$SHARED_DIR/exported_keys"
chmod 700 "$SHARED_DIR/exported_keys"

# Testing whether paxd works or not
paxd version # Uncomment the below line if there are any dependency issues
# ldd build/paxd

# Initialize validator node
MONIKER="pax-node-$NODE_ID"

paxd init "$MONIKER" --chain-id pax >/dev/null 2>&1

# Copy configs
APP_CONFIG_FILE="$NODE_DIR/app.toml"
TENDERMINT_CONFIG_FILE="$NODE_DIR/config.toml"
cp docker/localnode/config/app.toml "$APP_CONFIG_FILE"
cp docker/localnode/config/config.toml "$TENDERMINT_CONFIG_FILE"


# Set up persistent peers
PAX_NODE_ID=$(paxd tendermint show-node-id)
NODE_IP=$(hostname -i | awk '{print $1}')
P2P_PORT=26656  # Must match [p2p] laddr in config.toml
EVMRPC_PORT=8545  # Must match the EVM RPC HTTP port (evmrpc DefaultConfig HTTPPort).
echo "$PAX_NODE_ID@$NODE_IP:$P2P_PORT" > "$NODE_DIR/persistent_peer.txt"

# Store autobahn-compatible pubkeys and address for config generation
cp "$PAX_HOME"/config/validator_pubkey.txt "$NODE_DIR/" || { echo "ERROR: failed to copy validator_pubkey.txt"; exit 1; }
cp "$PAX_HOME"/config/node_pubkey.txt "$NODE_DIR/" || { echo "ERROR: failed to copy node_pubkey.txt"; exit 1; }
echo "$NODE_IP:$P2P_PORT" > "$NODE_DIR/autobahn_address.txt"
echo "http://$NODE_IP:$EVMRPC_PORT" > "$NODE_DIR/evmrpc_url.txt"

# Create a new account
ACCOUNT_NAME="node_admin"
echo "Adding account $ACCOUNT_NAME"
printf "12345678\n12345678\ny\n" | paxd keys add "$ACCOUNT_NAME" >/dev/null 2>&1

# Get genesis account info
GENESIS_ACCOUNT_ADDRESS=$(printf "12345678\n" | paxd keys show "$ACCOUNT_NAME" -a)
echo "$GENESIS_ACCOUNT_ADDRESS" > "$NODE_DIR/genesis_account.txt"

# Add funds to genesis account
paxd add-genesis-account "$GENESIS_ACCOUNT_ADDRESS" 10000000uhpx,10000000uusdc,10000000uatom

# Create gentx
printf "12345678\n" | paxd gentx "$ACCOUNT_NAME" 10000000uhpx --chain-id pax
cp "$PAX_HOME"/config/gentx/* "$NODE_DIR/gentx/"

# Creating some testing accounts
echo "Creating $NUM_ACCOUNTS accounts"
python3 tools/benchmark/loadtest/scripts/populate_genesis_accounts.py "$NUM_ACCOUNTS" loc --accounts "$PAX_PRIVATE_DIR/test_accounts" >/dev/null 2>&1
echo "Finished $NUM_ACCOUNTS accounts creation"

# Keys stay in the disposable private home; never export operator private keys.
# Before any shared deployment, stop the cluster, verify private-home cleanup,
# remove historical build/generated/exported_keys and ~/exported_keys, and
# replace all disposable credentials. See the checked cleanup in the task record.
