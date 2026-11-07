#!/usr/bin/env sh
set -eu
umask 077

echo "Configure and initialize environment"

for command in paxd python3 cp mv mktemp rm grep sed; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "Missing prerequisite: $command" >&2
    exit 1
  }
done
for file in docker/rpcnode/config/app.toml docker/rpcnode/config/config.toml build/generated/genesis.json docker/rpcnode/scripts/state_sync.py; do
  [ -s "$file" ] && [ -r "$file" ] || {
    echo "Missing readable prerequisite: $file" >&2
    exit 1
  }
done

# Stage validated discovery before paxd init or any node configuration edit.
INIT_DIR=$(mktemp -d build/generated/rpc-init.XXXXXX)
trap 'rm -rf "$INIT_DIR"' EXIT
trap 'exit 1' HUP INT TERM
python3 docker/rpcnode/scripts/state_sync.py \
  --config docker/rpcnode/config/config.toml \
  --genesis build/generated/genesis.json \
  --output "$INIT_DIR/config.toml" --peers "$INIT_DIR/PEERS"

paxd version
MONIKER="pax-rpc-node"
paxd init --chain-id pax "$MONIKER"

cp docker/rpcnode/config/app.toml "$HOME/.pax/config/app.toml"
cp "$INIT_DIR/config.toml" "$HOME/.pax/config/config.toml"
cp build/generated/genesis.json "$HOME/.pax/config/genesis.json"
mv "$INIT_DIR/PEERS" build/generated/PEERS

# Apply Giga Storage overrides so the RPC node's app hash matches the validators.
GIGA_STORAGE=${GIGA_STORAGE:-false}
GIGA_FLATKV_ONLY=${GIGA_FLATKV_ONLY:-false}
if [ "$GIGA_STORAGE" = "true" ] && [ "$GIGA_FLATKV_ONLY" != "true" ]; then
  # Default receipt backend to pebble when giga storage is on; callers may
  # still override via an explicit RECEIPT_BACKEND env var.
  RECEIPT_BACKEND=${RECEIPT_BACKEND:-pebble}
  echo "Enabling Giga Storage for RPC node..."

  # SC layer: must match validators (test_only_dual_write)
  if grep -q '^sc-write-mode[[:space:]]*=' ~/.pax/config/app.toml; then
    sed -i 's/^sc-write-mode[[:space:]]*=.*/sc-write-mode = "test_only_dual_write"/' ~/.pax/config/app.toml
  else
    sed -i '/^\[state-store\]/i sc-write-mode = "test_only_dual_write"' ~/.pax/config/app.toml
  fi

  # SS layer: enable EVM split
  sed -i 's/^evm-ss-split[[:space:]]*=.*/evm-ss-split = true/' ~/.pax/config/app.toml
fi

if [ "$GIGA_FLATKV_ONLY" = "true" ]; then
  echo "Booting RPC node in flatkv_only mode..."
  if grep -q '^sc-write-mode[[:space:]]*=' ~/.pax/config/app.toml; then
    sed -i 's/^sc-write-mode[[:space:]]*=.*/sc-write-mode = "flatkv_only"/' ~/.pax/config/app.toml
  else
    sed -i '/^\[state-store\]/i sc-write-mode = "flatkv_only"' ~/.pax/config/app.toml
  fi
  sed -i 's/^evm-ss-split[[:space:]]*=.*/evm-ss-split = false/' ~/.pax/config/app.toml
fi

# Apply receipt backend override if requested
RECEIPT_BACKEND=${RECEIPT_BACKEND:-}
if [ -n "$RECEIPT_BACKEND" ]; then
  echo "Setting receipt store backend to '$RECEIPT_BACKEND' for RPC node..."
  if grep -q "\[receipt-store\]" ~/.pax/config/app.toml; then
    sed -i "s/rs-backend = .*/rs-backend = \"$RECEIPT_BACKEND\"/" ~/.pax/config/app.toml
  else
    echo "" >> ~/.pax/config/app.toml
    echo "[receipt-store]" >> ~/.pax/config/app.toml
    echo "rs-backend = \"$RECEIPT_BACKEND\"" >> ~/.pax/config/app.toml
  fi
fi
