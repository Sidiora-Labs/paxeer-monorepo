#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
BASE=$(realpath -m "${TMPDIR:-/tmp}/lx-localnode")
ENV_OUT=$BASE/localnode.env
GATEWAY=http://127.0.0.1:0
NODE_HOME=
SUCCESS=0
FAILED_COMMAND=
USDL=0x85FcD13735F4309833A503EE804ea32395851479
CHAIN=hyperpax_125-1

fail() {
    FAILED_COMMAND=${FAILED_COMMAND:-$*}
    echo "v6120-node: $*" >&2
    exit 1
}

stop_node() {
    local dir pid role command attempt
    dir=$(realpath -e "$1")
    [[ "$dir" == "$BASE"/node.* ]] && [ -f "$dir/.localnode" ] || fail "not a disposable localnode home: $dir"
    for role in feeder node; do
        [ -f "$dir/$role.pid" ] || continue
        pid=$(cat "$dir/$role.pid")
        [[ "$pid" =~ ^[1-9][0-9]*$ ]] || fail "invalid $role pid"
        if kill -0 "$pid" 2>/dev/null; then
            [ "$(awk '{print $3}' "/proc/$pid/stat" 2>/dev/null)" != Z ] || continue
            command=$(tr '\0' ' ' < "/proc/$pid/cmdline")
            [[ "$command" == *"$dir"* ]] || fail "$role pid no longer belongs to $dir"
            kill "$pid"
            for ((attempt = 0; attempt < 100; attempt++)); do
                kill -0 "$pid" 2>/dev/null || break
                [ "$(awk '{print $3}' "/proc/$pid/stat" 2>/dev/null)" != Z ] || break
                sleep 0.1
            done
            if kill -0 "$pid" 2>/dev/null && [ "$(awk '{print $3}' "/proc/$pid/stat" 2>/dev/null)" != Z ]; then
                fail "$role did not stop; see $dir/$role.log"
            fi
        fi
    done
}

rpc() {
    curl --fail --silent --show-error --max-time 3 -H 'Content-Type: application/json' \
        --data "$(jq -nc --arg method "$1" --argjson params "${2:-[]}" \
            '{jsonrpc:"2.0",id:1,method:$method,params:$params}')" "$EVM"
}

wait_receipt() {
    local hash=$1 receipt deadline=$((SECONDS + 60))
    [[ "$hash" =~ ^0x[0-9a-fA-F]{64}$ ]] || fail "missing EVM transaction hash"
    while (( SECONDS < deadline )); do
        kill -0 "$(cat "$NODE_HOME/node.pid")" 2>/dev/null || fail "node exited: $NODE_HOME/node.log"
        receipt=$(rpc eth_getTransactionReceipt "[\"$hash\"]")
        if jq -e '.result != null' <<< "$receipt" >/dev/null; then
            [ "$(jq -r '.result.status' <<< "$receipt")" = 0x1 ] || fail "EVM transaction reverted: $hash"
            return
        fi
        sleep 0.2
    done
    fail "EVM transaction did not commit: $hash"
}

evm_call() {
    local sender=$1 address=$2 data=$3 output hash sender_address nonce response
    local deadline=$((SECONDS + 60))
    sender_address=$(jq -er '.evm_address' "$NODE_HOME/$sender.json")
    # Setup calls are serial and await receipts, so use the committed nonce.
    while :; do
        kill -0 "$(cat "$NODE_HOME/node.pid")" 2>/dev/null || fail "node exited: $NODE_HOME/node.log"
        if response=$(rpc eth_getTransactionCount "[\"$sender_address\",\"latest\"]") \
            && nonce=$(jq -er '.result | strings | select(test("^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$"))' <<< "$response"); then
            break
        fi
        (( SECONDS < deadline )) || fail "committed nonce unavailable for $sender_address: $response"
        sleep 0.2
    done
    output=$("$PAXD" tx evm call-contract "$address" "$data" --from "$sender" \
        --home "$NODE_HOME" --keyring-backend test --chain-id "$CHAIN" --node "$NODE" \
        --evm-rpc "$EVM" --nonce "$((nonce))" --gas-fee-cap 1000000000000 --gas-limit 7000000 -y)
    hash=$(sed -n 's/^Transaction hash: //p' <<< "$output")
    wait_receipt "$hash"
}

feed_loop() {
    local previous=0 height timestamp result hash deadline
    trap 'exit 0' TERM INT
    while kill -0 "$(cat "$NODE_HOME/node.pid")" 2>/dev/null; do
        height=$(rpc eth_blockNumber | jq -er '.result')
        if [ "$height" = "$previous" ]; then
            sleep 0.1
            continue
        fi
        previous=$height
        timestamp=$(date +%s%3N)
        jq -n --arg feeder "$TEST_ACCOUNT" --arg sequence "$((height))" --arg timestamp "$timestamp" \
            '{body:{messages:[{"@type":"/paxprotocol.paxchain.pricefeed.MsgSubmitPrices",
              feeder:$feeder,observations:[{market_id:"0",sequence:$sequence,index_price:"60000000000000000000000",
              mark_price:"60000000000000000000000",source_timestamp_ms:$timestamp}]}],
              memo:"",timeout_height:"0",extension_options:[],non_critical_extension_options:[]},
              auth_info:{signer_infos:[],fee:{amount:[],gas_limit:"2000000",payer:"",granter:""}},signatures:[]}' \
            > "$NODE_HOME/observation.json"
        "$PAXD" tx sign "$NODE_HOME/observation.json" --from test \
            --home "$NODE_HOME" --keyring-backend test --chain-id "$CHAIN" --node "$NODE" \
            --output-document "$NODE_HOME/observation-signed.json"
        result=$("$PAXD" tx broadcast "$NODE_HOME/observation-signed.json" \
            --home "$NODE_HOME" --node "$NODE" -b sync -o json)
        jq -e '(.code // 0) == 0 and (.txhash | test("^[0-9A-Fa-f]{64}$"))' <<< "$result" >/dev/null \
            || fail "pricefeed broadcast rejected: $result"
        hash=$(jq -r '.txhash' <<< "$result")
        deadline=$((SECONDS + 30))
        while ! result=$("$PAXD" q tx "$hash" --node "$NODE" -o json 2>/dev/null); do
            (( SECONDS < deadline )) || fail "pricefeed transaction did not commit: $hash"
            sleep 0.1
        done
        jq -e '(.code // 0) == 0' <<< "$result" >/dev/null || fail "pricefeed transaction failed: $result"
    done
    fail "node exited"
}

cleanup() {
    local status=$1 command=$2 log
    if [ "$SUCCESS" = 0 ]; then
        printf 'v6120-node: failed (exit %s): %s; evidence: %s\n' \
            "$status" "${FAILED_COMMAND:-$command}" "${NODE_HOME:-$BASE}" >&2
        for log in init validate-genesis node feeder; do
            if [ -n "$NODE_HOME" ] && [ -f "$NODE_HOME/$log.log" ]; then
                printf '\n--- %s ---\n' "$NODE_HOME/$log.log" >&2
                tail -n 40 "$NODE_HOME/$log.log" >&2
            fi
        done
        if [ -n "$NODE_HOME" ] && [ "${FEEDER_PROCESS:-0}" = 0 ]; then
            (stop_node "$NODE_HOME") || true
        fi
    fi
    return "$status"
}
trap 'FAILED_COMMAND=$BASH_COMMAND' ERR
trap 'cleanup "$?" "$BASH_COMMAND"' EXIT

if [ "${1:-}" = --feed ]; then
    [ "$#" = 2 ] || fail '--feed requires a home'
    NODE_HOME=$(realpath -e "$2")
    [[ "$NODE_HOME" == "$BASE"/node.* ]] && [ -f "$NODE_HOME/.localnode" ] || fail 'invalid feeder home'
    FEEDER_PROCESS=1
    . "$NODE_HOME/process.env"
    feed_loop
    exit
fi

while [ "$#" -gt 0 ]; do
    case "$1" in
        --env-out) [ "$#" -ge 2 ] || fail '--env-out requires a file'; ENV_OUT=$2; shift 2 ;;
        --gateway-url) [ "$#" -ge 2 ] || fail '--gateway-url requires a URL'; GATEWAY=$2; shift 2 ;;
        --stop) [ "$#" = 2 ] || fail '--stop requires a home'; stop_node "$2"; SUCCESS=1; exit ;;
        *) fail "unknown argument: $1" ;;
    esac
done

ENV_OUT=$(realpath -m "$ENV_OUT")
[[ "$ENV_OUT" == "$BASE"/* ]] || fail "env file must be beneath $BASE"
[[ "$BASE/" != "$ROOT/"* ]] || fail 'node homes must be outside the repository'
[[ "$GATEWAY" =~ ^https?://[^[:space:]\'\"]+$ ]] || fail 'invalid gateway URL'
for tool in jq curl python3 openssl; do
    command -v "$tool" >/dev/null || fail "$tool is required"
done
if [ -z "${PAXD:-}" ]; then
    INSTALL_BIN=$(go env GOBIN)
    if [ -z "$INSTALL_BIN" ]; then
        INSTALL_BIN=$(go env GOPATH)
        INSTALL_BIN=${INSTALL_BIN%%:*}/bin
    fi
    PAXD=$INSTALL_BIN/paxd
fi
mkdir -p "$BASE" "$(dirname "$ENV_OUT")"
if [ ! -x "$PAXD" ]; then
    (cd "$ROOT" && make -f chain.mk install) > "$BASE/install.log" 2>&1 \
        || fail "paxd build failed: $BASE/install.log"
fi
[ -x "$PAXD" ] || fail "paxd is not executable: $PAXD"
NODE_HOME=$(mktemp -d "$BASE/node.XXXXXXXX")
touch "$NODE_HOME/.localnode"
"$PAXD" init local-v6120 --chain-id "$CHAIN" --home "$NODE_HOME" > "$NODE_HOME/init.log" 2>&1
cp "$NODE_HOME/config/genesis.json" "$NODE_HOME/init-genesis.json"

NAMES=(deployer test validator)
ETH_ADDRESSES=()
ACCOUNTS=()
for index in 0 1 2; do
    "$PAXD" keys add "${NAMES[$index]}" --home "$NODE_HOME" --keyring-backend test --output json \
        > "$NODE_HOME/${NAMES[$index]}.json" 2>> "$NODE_HOME/keys.log"
    "$PAXD" keys show "${NAMES[$index]}" --home "$NODE_HOME" --keyring-backend test --output json \
        > "$NODE_HOME/${NAMES[$index]}.json" 2>> "$NODE_HOME/keys.log"
    ACCOUNTS+=("$(jq -er '.address' "$NODE_HOME/${NAMES[$index]}.json")")
    ETH_ADDRESSES+=("$(jq -er '.evm_address | select(test("^0x[0-9a-fA-F]{40}$"))' "$NODE_HOME/${NAMES[$index]}.json")")
done
TEST_ACCOUNT=${ACCOUNTS[1]}
VALIDATOR=$("$PAXD" keys show validator -a --bech val --home "$NODE_HOME" --keyring-backend test)
CONS=$("$PAXD" tendermint show-address --home "$NODE_HOME")
KEY=$(printf 'y\n' | "$PAXD" keys export test --unsafe --unarmored-hex --home "$NODE_HOME" --keyring-backend test 2> "$NODE_HOME/export-key.log")
[[ "$KEY" =~ ^[0-9a-fA-F]{64}$ ]] || fail 'test key export failed'

# Re-key the retained validator and its references, preserving exported module state.
SOURCE=$ROOT/node/testdata/v6120-halt-export.json
KEPT=$(jq -er '.app_state.staking.validators[0].operator_address' "$SOURCE")
OLD_ACCOUNT=$("$PAXD" debug addr "$KEPT" 2>/dev/null | sed -n 's/^Bech32 Acc: //p')
PV=$(jq -c . "$NODE_HOME/config/priv_validator_key.json")
GENESIS=$NODE_HOME/config/genesis.json
CONSENSUS=$(jq -ce '.consensus_params' "$GENESIS")
jq --arg kept "$KEPT" --arg operator "$VALIDATOR" --arg old "$OLD_ACCOUNT" --arg account "${ACCOUNTS[2]}" \
    --arg cons "$CONS" --argjson pv "$PV" --arg now "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --argjson consensus "$CONSENSUS" '
    def set_pool($address; $amount):
        .app_state.bank.balances = ([.app_state.bank.balances[] | select(.address != $address)]
          + if $amount == 0 then [] else [{address:$address,coins:[{denom:"uhpx",amount:($amount|tostring)}]}] end);
    walk(if type == "string" then
        if . == $kept then $operator elif . == $old then $account else . end else . end)
    | .chain_id = "hyperpax_125-1" | .genesis_time = $now
    | .app_state.staking.validators |= map(if .operator_address == $operator then
        .consensus_pubkey.key = $pv.pub_key.value | .jailed = false | .status = "BOND_STATUS_BONDED"
        | .tokens = (([1000000000, (.tokens|tonumber)] | max)|tostring)
        else .jailed = true | .status = "BOND_STATUS_UNBONDED" end)
    | (.app_state.staking.validators[] | select(.operator_address == $operator) | .tokens|tonumber) as $tokens
    | (($tokens / 1000000)|floor|tostring) as $power
    | .app_state.staking.params.max_voting_power_ratio = "1.000000000000000000"
    | .app_state.staking.last_validator_powers = [{address:$operator,power:$power}]
    | .app_state.staking.last_total_power = $power
    | .validators = [{address:$pv.address,pub_key:$pv.pub_key,power:$power,name:"local-v6120"}]
    | (.app_state.auth.accounts[] | select(.name == "bonded_tokens_pool") | .base_account.address) as $bonded
    | (.app_state.auth.accounts[] | select(.name == "not_bonded_tokens_pool") | .base_account.address) as $unbonded
    | ([.app_state.staking.validators[] | select(.status != "BOND_STATUS_BONDED") | .tokens|tonumber]
        + [.app_state.staking.unbonding_delegations[]?.entries[]?.balance|tonumber] | add // 0) as $unbonded_tokens
    | set_pool($bonded; $tokens) | set_pool($unbonded; $unbonded_tokens)
    | .app_state.slashing.signing_infos = ([.app_state.slashing.signing_infos[]? | select(.address != $cons)]
      + [{address:$cons,validator_signing_info:{address:$cons,start_height:.initial_height,index_offset:"0",
          jailed_until:"1970-01-01T00:00:00Z",tombstoned:false,missed_blocks_counter:"0"}}])
    | .app_state.oracle.params.min_valid_per_window = "0.000000000000000000"
    | .consensus_params.timeout = $consensus.timeout
    | .consensus_params.timeout.commit = "1000000000"
    | .consensus_params.synchrony = $consensus.synchrony
    | .consensus_params.block.max_gas_wanted = $consensus.block.max_gas_wanted
    | del(.app_state.pricefeed, .app_state.perps, .app_state.spot)
    | .app_state.bank.supply = []
' "$SOURCE" > "$GENESIS"
for index in 0 1 2; do
    if [ "$index" != 2 ]; then
        "$PAXD" add-genesis-account "${ACCOUNTS[$index]}" 1000000000000000uhpx --home "$NODE_HOME" >> "$NODE_HOME/init.log" 2>&1
    else
        # The validator account was re-keyed with the export; top it up without adding a duplicate auth account.
        jq --arg a "${ACCOUNTS[$index]}" '.app_state.bank.balances += [{address:$a,coins:[{denom:"uhpx",amount:"1000000000000000"}]}]' \
            "$GENESIS" > "$GENESIS.tmp"
        mv "$GENESIS.tmp" "$GENESIS"
    fi
    jq --arg a "${ACCOUNTS[$index]}" --arg e "${ETH_ADDRESSES[$index]}" '
        .app_state.evm.address_associations = ([.app_state.evm.address_associations[]? | select(.pax_address != $a)]
          + [{eth_address:$e,pax_address:$a}])' "$GENESIS" > "$GENESIS.tmp"
    mv "$GENESIS.tmp" "$GENESIS"
done
# Sum arbitrary-sized bank amounts exactly rather than rounding through jq doubles.
python3 - "$GENESIS" "$USDL" "${ETH_ADDRESSES[0]}" \
    "$ROOT/modules/launchpad/keeper/testdata/v6120/usdl.json" <<'PY'
import base64
import json
import subprocess
import sys
from collections import defaultdict
path, usdl, deployer, fixture_path = sys.argv[1:]
with open(path) as handle:
    genesis = json.load(handle)
# Keep the exported USDL runtime and all migration balances. Grant only the
# disposable deployer MINTER_ROLE in the token's existing AccessControl layout.
def keccak(value):
    return subprocess.check_output(['openssl', 'dgst', '-keccak-256', '-binary'], input=value)

evm = genesis['app_state']['evm']
runtime = next(base64.b64decode(item['code']) for item in evm['codes']
               if item['address'].lower() == usdl.lower())
with open(fixture_path) as handle:
    fixture = json.load(handle)
if '0x' + keccak(runtime).hex() != fixture['codeHash']:
    raise SystemExit('exported USDL runtime differs from the migration fixture')
role_root = keccak(keccak(b'MINTER_ROLE') + (6).to_bytes(32, 'big'))
if role_root not in runtime:
    raise SystemExit('USDL runtime does not carry the expected minter role layout')
minter_slot = base64.b64encode(keccak(bytes.fromhex(deployer[2:]).rjust(32, b'\0') + role_root)).decode()
evm['states'] = [item for item in evm['states']
                 if not (item['address'].lower() == usdl.lower() and item['key'] == minter_slot)]
evm['states'].append({'address': usdl, 'key': minter_slot,
                      'value': base64.b64encode((1).to_bytes(32, 'big')).decode()})
bank = genesis['app_state']['bank']
balances = defaultdict(lambda: defaultdict(int))
for balance in bank['balances']:
    for coin in balance['coins']:
        balances[balance['address']][coin['denom']] += int(coin['amount'])
bank['balances'] = [{'address': address, 'coins': [{'denom': denom, 'amount': str(amount)}
                    for denom, amount in sorted(coins.items()) if amount]}
                    for address, coins in sorted(balances.items())]
supply = defaultdict(int)
for coins in balances.values():
    for denom, amount in coins.items():
        supply[denom] += amount
bank['supply'] = [{'denom': denom, 'amount': str(amount)} for denom, amount in sorted(supply.items())]
with open(path, 'w') as handle:
    json.dump(genesis, handle)
PY
jq --slurpfile init "$NODE_HOME/init-genesis.json" '
    .app_state = ($init[0].app_state + .app_state)
' "$GENESIS" > "$NODE_HOME/validation-genesis.json"
"$PAXD" validate-genesis "$NODE_HOME/validation-genesis.json" --home "$NODE_HOME" > "$NODE_HOME/validate-genesis.log" 2>&1

read -r RPC_PORT P2P_PORT EVM_PORT GRPC_PORT WEB_PORT < <(python3 - <<'PY'
import socket
listeners = [socket.socket() for _ in range(5)]
for listener in listeners:
    listener.bind(('127.0.0.1', 0))
print(*(listener.getsockname()[1] for listener in listeners))
for listener in listeners:
    listener.close()
PY
)
NODE=tcp://127.0.0.1:$RPC_PORT
EVM=http://127.0.0.1:$EVM_PORT
set_toml() {
    local file=$1 section=$2 key=$3 value=$4
    python3 - "$file" "$section" "$key" "$value" <<'PY'
import re
import sys
path, section, key, value = sys.argv[1:]
with open(path) as handle:
    lines = handle.readlines()
current = ''
found = False
for index, line in enumerate(lines):
    if line.startswith('['):
        current = line.strip()[1:-1]
    if current == section and re.match(r'^' + re.escape(key) + r'\s*=', line):
        lines[index] = f'{key} = {value}\n'
        found = True
if not found:
    raise SystemExit(f'missing setting {section}.{key} in {path}')
with open(path, 'w') as handle:
    handle.writelines(lines)
PY
}
CONFIG=$NODE_HOME/config/config.toml
APP=$NODE_HOME/config/app.toml
set_toml "$CONFIG" '' mode '"validator"'
set_toml "$CONFIG" rpc laddr "\"$NODE\""
set_toml "$CONFIG" rpc pprof-laddr '""'
set_toml "$CONFIG" p2p laddr "\"tcp://127.0.0.1:$P2P_PORT\""
for setting in external-address bootstrap-peers persistent-peers; do
    set_toml "$CONFIG" p2p "$setting" '""'
done
set_toml "$CONFIG" p2p pex false
set_toml "$CONFIG" instrumentation prometheus false
set_toml "$APP" api enable false
set_toml "$APP" grpc address "\"127.0.0.1:$GRPC_PORT\""
set_toml "$APP" grpc-web enable false
set_toml "$APP" grpc-web address "\"127.0.0.1:$WEB_PORT\""
set_toml "$APP" evm http_enabled true
set_toml "$APP" evm http_address '"127.0.0.1"'
set_toml "$APP" evm http_port "$EVM_PORT"
set_toml "$APP" evm ws_enabled false
set_toml "$APP" evm enable_test_api false
HEIGHT=$(jq -er '.initial_height | tonumber | select(. > 0)' "$GENESIS")
jq -n --argjson height "$HEIGHT" '{name:"v6.12.0",height:$height}' > "$NODE_HOME/data/upgrade-info.json"
nohup "$PAXD" start --home "$NODE_HOME" > "$NODE_HOME/node.log" 2>&1 < /dev/null &
echo "$!" > "$NODE_HOME/node.pid"
deadline=$((SECONDS + 120))
while :; do
    kill -0 "$(cat "$NODE_HOME/node.pid")" 2>/dev/null || fail "node exited: $NODE_HOME/node.log"
    if grep -q 'ProcessBlock panic:' "$NODE_HOME/node.log"; then
        fail "first-block upgrade failed: $NODE_HOME/node.log"
    fi
    if [ "$(rpc eth_chainId 2>/dev/null | jq -r '.result')" = 0x7d ] \
        && applied=$("$PAXD" q upgrade applied v6.12.0 --node "$NODE" -o json 2>/dev/null) \
        && [ "$(jq -r '.header.height' <<< "$applied")" = "$HEIGHT" ]; then
        break
    fi
    (( SECONDS < deadline )) || fail "upgrade readiness timed out: $NODE_HOME/node.log"
    sleep 0.5
done
printf -v MINT_DATA '40c10f19000000000000000000000000%s%064x' "${ETH_ADDRESSES[1]#0x}" 1000000000000
evm_call deployer "$USDL" "$MINT_DATA"
PUBKEY=$(python3 - "$NODE_HOME/test.json" <<'PYKEY'
import base64
import json
import sys
with open(sys.argv[1]) as handle:
    public_key = json.loads(json.load(handle)['pubkey'])
print(base64.b64decode(public_key['key'], validate=True).hex())
PYKEY
)
# registerFeedKey(bytes): selector, dynamic offset, byte count, compressed pubkey.
SELECTOR=0x5d6617d9
printf -v REGISTER_DATA '%s%064x%064x%-128s' "${SELECTOR#0x}" 32 33 "$PUBKEY"
REGISTER_DATA=${REGISTER_DATA// /0}
evm_call validator 0x0000000000000000000000000000000000001020 "$REGISTER_DATA"
"$PAXD" q pricefeed feed-key "$VALIDATOR" --node "$NODE" -o json > "$NODE_HOME/feed-key.json"
[ "$(jq -r '.feed_account' "$NODE_HOME/feed-key.json")" = "$TEST_ACCOUNT" ] || fail 'feed account registration mismatch'
{
    printf 'PAXD=%q\nNODE=%q\nEVM=%q\nTEST_ACCOUNT=%q\n' "$PAXD" "$NODE" "$EVM" "$TEST_ACCOUNT"
} > "$NODE_HOME/process.env"
nohup bash "${BASH_SOURCE[0]}" --feed "$NODE_HOME" > "$NODE_HOME/feeder.log" 2>&1 < /dev/null &
echo "$!" > "$NODE_HOME/feeder.pid"
deadline=$((SECONDS + 30))
while :; do
    kill -0 "$(cat "$NODE_HOME/feeder.pid")" 2>/dev/null || fail "feeder exited: $NODE_HOME/feeder.log"
    if price=$("$PAXD" q pricefeed price 0 --node "$NODE" -o json 2>/dev/null) \
        && jq -e '.fresh == true and .index == "60000000000000000000000"' <<< "$price" >/dev/null; then
        break
    fi
    (( SECONDS < deadline )) || fail "BTC price did not become fresh: $NODE_HOME/feeder.log"
    sleep 0.2
done
{
    printf 'LOCAL_NODE_RPC=%q\n' "$EVM"
    printf 'LOCAL_NODE_KEY=%q\n' "0x$KEY"
    printf 'PERPS_DATA_TEST_RPC=%q\n' "$EVM"
    printf 'NEXT_PUBLIC_PERPS_GATEWAY_URL=%q\n' "$GATEWAY"
} > "$NODE_HOME/localnode.env"
cp "$NODE_HOME/localnode.env" "$ENV_OUT"
SUCCESS=1
printf 'v6120-node: ready; home=%s env=%s\n' "$NODE_HOME" "$ENV_OUT" >&2
