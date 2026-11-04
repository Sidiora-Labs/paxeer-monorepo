#!/usr/bin/env bash

beta_tool() {
    python3 "$SCRIPT_DIR/beta_tool.py" "$@"
}

native_policy_args() {
    local recipient
    recipient=$(python3 "$EVM_TOOL" address "$SECRETS_DIR/native-treasury.key")
    printf '%s\n' "$NODE_NETWORK_ID" "$NODE_ASSET_ID" "${recipient#0x}" | tr 'A-F' 'a-f'
}

native_paxd() {
    docker run --rm --user "$(id -u):$(id -g)" \
        --mount "type=bind,src=$NATIVE_HOME,dst=/native" \
        --entrypoint /usr/local/bin/paxd "$(image_ref kernel)" "$@" --home /native
}

native_genesis_prepare() {
    local recipient account
    mkdir -p "$SECRETS_DIR"
    chmod 0700 "$SECRETS_DIR"
    if [ -n "${LAYERX_BETA_NATIVE_HOME:-}" ]; then
        [ -n "${LAYERX_BETA_TREASURY_KEY_FILE:-}" ] || fail 'native home import requires LAYERX_BETA_TREASURY_KEY_FILE'
        NATIVE_HOME=$(realpath "$LAYERX_BETA_NATIVE_HOME")
        (umask 077; cp "$LAYERX_BETA_TREASURY_KEY_FILE" "$SECRETS_DIR/native-treasury.key")
        native_paxd validate-genesis
        return
    fi
    NATIVE_HOME="$WORK_DIR/native-home"
    [ ! -e "$NATIVE_HOME" ] || fail 'native home already exists; select retained material instead of regenerating a validator'
    (umask 077; mkdir "$NATIVE_HOME")
    native_paxd init layerx-beta --chain-id hyperpax_125-1 --mode validator
    native_paxd keys add validator --keyring-backend test --output json > "$LOG_DIR/native-validator.json"
    chmod 0600 "$LOG_DIR/native-validator.json"
    native_paxd add-genesis-account validator 100000000000000000000uhpx --keyring-backend test
    evm_key_generate native-treasury
    recipient=$(python3 "$EVM_TOOL" address "$SECRETS_DIR/native-treasury.key")
    account=$(native_paxd debug addr "${recipient#0x}" | sed -n 's/^Bech32 Acc: //p')
    [[ $account == pax1* ]] || fail 'native treasury account derivation failed'
    native_paxd add-genesis-account "$account" 1000000000000000000000000uhpx
    native_paxd gentx validator 7000000000000000uhpx --chain-id hyperpax_125-1 --keyring-backend test
    native_paxd collect-gentxs
    native_paxd sequencer gen-receipt-key --output /native/config/receipt.key > "$LOG_DIR/native-receipt-key.json"
    local -a policy
    mapfile -t policy < <(native_policy_args)
    beta_tool native-configure "$NATIVE_HOME" "$account" "${policy[@]}" > /dev/null
    # These are the native runtime's actual home-relative configuration paths.
    python3 - "$NATIVE_HOME" <<'PY'
from pathlib import Path
import os
import re
import sys
home = Path(sys.argv[1])
config = home / 'config/config.toml'
value = config.read_text()
value = re.sub(r'^mode = .*$', 'mode = "validator"', value, flags=re.M)
config.write_text(value)
app = home / 'config/app.toml'
value = re.sub(r'^minimum-gas-prices = .*$', 'minimum-gas-prices = "0.01uhpx"', app.read_text(), flags=re.M)
app.write_text(value)
for directory in (home, home / 'config', home / 'data'):
    os.chmod(directory, 0o700)
for path in (home / 'config').iterdir():
    if path.is_file(): os.chmod(path, 0o600)
os.chmod(home / 'data/priv_validator_state.json', 0o600)
PY
    native_paxd validate-genesis
}

native_material_prepare() {
    mkdir -p "$SECRETS_DIR" "$LOG_DIR"
    local -a policy
    case "${LAYERX_BETA_RETAIN_MATERIAL:-0}" in
        0)
            native_genesis_prepare
            mapfile -t policy < <(native_policy_args)
            beta_tool native-save "$NATIVE_HOME" "$SECRETS_DIR/native" "${policy[@]}" > "$LOG_DIR/native-material.json"
            ;;
        1)
            mapfile -t policy < <(native_policy_args)
            beta_tool native-open "$SECRETS_DIR/native" "${policy[@]}" > "$LOG_DIR/native-material-reopened.json"
            [ -s "$SECRETS_DIR/native-readiness-transaction" ] || fail 'retained native readiness transaction is missing'
            ;;
        *) fail 'LAYERX_BETA_RETAIN_MATERIAL must be 0 or 1' ;;
    esac
}

native_material_save() {
    local -a policy
    mapfile -t policy < <(native_policy_args)
    beta_tool native-open "$SECRETS_DIR/native" "${policy[@]}" > /dev/null
    [ -s "$SECRETS_DIR/native-readiness-transaction" ] || fail 'native material cannot be saved before committed receipt readiness'
}

native_secrets_apply() {
    local -a policy
    mapfile -t policy < <(native_policy_args)
    kube create namespace "$TESTNET_NAMESPACE" --dry-run=client -o yaml | kube apply -f - > /dev/null
    beta_tool native-secret "$SECRETS_DIR/native" "${policy[@]}" | kube apply -f - > /dev/null
}

native_manifests_render() {
    mkdir -p "$MANIFESTS_DIR"
    render_manifest "$NODE_MANIFEST" "$MANIFESTS_DIR/node.yaml"
}

native_receipt_ready() {
    local transaction deadline
    local -a policy
    mapfile -t policy < <(native_policy_args)
    if [ -s "$SECRETS_DIR/native-readiness-transaction" ]; then
        transaction=$(cat "$SECRETS_DIR/native-readiness-transaction")
    else
        python3 "$EVM_TOOL" send --rpc "$NODE_URL" --chain 125 \
            --key-file "$SECRETS_DIR/native-treasury.key" "0x${policy[2]}" --value 0 \
            > "$LOG_DIR/native-readiness-receipt.json"
        transaction=$(jq -er .transactionHash "$LOG_DIR/native-readiness-receipt.json")
        (umask 077; printf '%s' "$transaction" > "$SECRETS_DIR/native-readiness-transaction")
    fi
    deadline=$(( $(date +%s) + READY_TIMEOUT ))
    while ! beta_tool native-ready "$SECRETS_DIR/native" "$SECRETS_DIR/native-readiness.json" \
        "$NODE_URL" "$CONSENSUS_URL" "$transaction" "${policy[@]}" > "$WORK_DIR/readyz.json" 2> "$LOG_DIR/native-readiness.log"; do
        [ "$(date +%s)" -lt "$deadline" ] || fail "native committed receipt readiness failed; see $LOG_DIR/native-readiness.log"
        sleep 2
    done
}

native_cluster_up() {
    local run_boundary_checks=$1
    [ "$run_boundary_checks" != 1 ] || fail "--boundary-checks requires hosted adapter migration; the native chain gate covers committed receipt readiness only"
    require_tool docker curl python3 git go jq openssl tar
    REVISION=$(revision)
    mkdir -p "$WORK_DIR" "$LOG_DIR"
    PULL_POLICY=IfNotPresent
    [ "$(cluster_mode)" = owner ] && PULL_POLICY=Always
    preflight_disk "$MIN_FREE_GIB" LAYERX_BETA_MIN_FREE_GIB 'build the native beta images'
    tools_install
    if [ "${LAYERX_BETA_RETAIN_MATERIAL:-0}" = 1 ]; then material_prepare; fi
    prepare_images
    cluster_create
    load_images
    if [ "${LAYERX_BETA_RETAIN_MATERIAL:-0}" != 1 ]; then material_prepare; fi
    secrets_apply
    manifests_render
    kube apply -f "$MANIFESTS_DIR/node.yaml" > /dev/null
    wait_for_pod_ready "$TESTNET_NAMESPACE" app=layerx-node "$READY_TIMEOUT"
    NODE_URL=http://127.0.0.1:19446
    CONSENSUS_URL=http://127.0.0.1:19461
    port_forward native-rpc "$TESTNET_NAMESPACE" layerx-native-chain 19446 8545
    port_forward native-consensus "$TESTNET_NAMESPACE" layerx-native-chain 19461 26657
    native_receipt_ready
    material_save
    (umask 077; printf 'export LAYERX_NODE_URL=%q\nexport LAYERX_PAXEER_RPC_URL=%q\nexport LAYERX_CHAIN_ID=125\n' "$NODE_URL" "$NODE_URL" > "$ENV_FILE")
    log "native chain 125 has a committed receipt; environment exported to $ENV_FILE"

}

native_cluster_render() {
    require_tool docker python3 git go jq openssl
    REVISION=$(revision)
    PULL_POLICY=IfNotPresent
    mkdir -p "$WORK_DIR" "$LOG_DIR"
    : > "$WORK_DIR/images"
    local name canonical dockerfile
    for name in "${IMAGE_NAMES[@]}"; do
        read -r canonical dockerfile <<<"$(image_source "$name")"
        printf '%s %s %s unbuilt\n' "$name" "$canonical" "$(image_ref "$name")" >> "$WORK_DIR/images"
    done
    material_prepare
    manifests_render
    log "rendered native manifests under $MANIFESTS_DIR"
}
