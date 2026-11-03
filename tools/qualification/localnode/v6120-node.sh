#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
BASE=$(realpath -m "${TMPDIR:-/tmp}/lx-localnode")
ENV_OUT=$BASE/localnode.env
GATEWAY=http://127.0.0.1:0
NODE_HOME=
SEQUENCER=0
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
    shift
    if [ "$#" = 0 ]; then
        set -- feeder node standby bootstrap-peer
    fi
    for role in "$@"; do
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

require_merge_precompiles() {
    local suffix address response data
    local missing=()
    for suffix in 101A 101B 101C 101D 0901 0902 0903 0904 0905 0906 0907 0908; do
        address=0x000000000000000000000000000000000000$suffix
        response=$(rpc eth_call "[{\"to\":\"$address\",\"data\":\"0xffffffff\"},\"latest\"]")
        if jq -e '.result == "0x" and .error == null' <<< "$response" >/dev/null; then
            missing+=("$address")
        elif ! jq -e '.result == null and .error.code == 3
            and (.error.message | strings |
                contains("LXP_PRECOMPILE_UNKNOWN_METHOD") or contains("LXP_AGENT_NOT_ACTIVE"))' \
            <<< "$response" >/dev/null; then
            fail "unexpected merge precompile rejection at $address: $response"
        fi
    done
    if [ "${#missing[@]}" -gt 0 ]; then
        fail "merge precompiles are not ready; addresses without handlers: ${missing[*]}"
    fi
    address=0x000000000000000000000000000000000000101A
    printf -v data '0x3f050cbc%s%064x' \
        1111111111111111111111111111111111111111111111111111111111111111 0
    response=$(rpc eth_call "[{\"to\":\"$address\",\"data\":\"$data\"},\"latest\"]")
    jq -e '(.error == null) and (.result | strings | test("^0x[0-9a-fA-F]{64}$"))' \
        <<< "$response" >/dev/null || fail "native ledger accountId failed: $response"
}

governance_tx() {
    local response hash deadline=$((SECONDS + 30))
    response=$("$PAXD" tx gov "$@" --from validator --home "$NODE_HOME" \
        --keyring-backend test --chain-id "$CHAIN" --node "$NODE" \
        --gas 2000000 --fees 1000000uhpx -b sync -o json -y)
    jq -e '(.code // 0) == 0 and (.txhash | test("^[0-9A-Fa-f]{64}$"))' \
        <<< "$response" >/dev/null || fail "governance broadcast rejected: $response"
    hash=$(jq -r '.txhash' <<< "$response")
    while ! response=$("$PAXD" q tx "$hash" --node "$NODE" -o json 2>/dev/null); do
        kill -0 "$(cat "$NODE_HOME/node.pid")" 2>/dev/null || fail "node exited: $NODE_HOME/node.log"
        (( SECONDS < deadline )) || fail "governance transaction did not commit: $hash"
        sleep 0.2
    done
    jq -e '(.code // 0) == 0' <<< "$response" >/dev/null \
        || fail "governance transaction failed: $response"
}

activate_merge() {
    local height info proposal applied deadline=$((SECONDS + 120))
    height=$(rpc eth_blockNumber | jq -er '.result')
    height=$((height + 20))
    info=$(python3 - "$NODE_HOME/config/priv_validator_key.json" "$height" <<'PY'
import base64
import hashlib
import json
import sys
with open(sys.argv[1]) as handle:
    public_key = json.load(handle)['pub_key']['value']
print(json.dumps({
    'upgradeType': 'minor',
    'leaders': [public_key],
    'receipt_keys': [{
        'key_id': base64.b64encode(hashlib.sha256(base64.b64decode(public_key)).digest()).decode(),
        'public_key': public_key,
        'first_height': int(sys.argv[2]) + 1,
        'last_height': 9223372036854775807,
    }],
    'last_seq': 0,
}))
PY
)
    governance_tx submit-proposal software-upgrade v7.0 --upgrade-height "$height" \
        --upgrade-info "$info" --title 'Activate the local merge precompiles' \
        --description 'Apply the merge plan on the disposable local chain.' --deposit 10000000uhpx
    proposal=$("$PAXD" q gov proposals --node "$NODE" -o json | jq -er \
        '.proposals[] | select(.content.plan.name == "v7.0") | .proposal_id')
    governance_tx vote "$proposal" yes
    while :; do
        kill -0 "$(cat "$NODE_HOME/node.pid")" 2>/dev/null || fail "node exited: $NODE_HOME/node.log"
        if grep -q 'ProcessBlock panic:' "$NODE_HOME/node.log"; then
            fail "merge upgrade failed: $NODE_HOME/node.log"
        fi
        if applied=$("$PAXD" q upgrade applied v7.0 --node "$NODE" -o json 2>/dev/null) \
            && [ "$(jq -r '.header.height' <<< "$applied")" = "$height" ]; then
            break
        fi
        (( SECONDS < deadline )) || fail "merge upgrade readiness timed out: $NODE_HOME/node.log"
        sleep 0.5
    done
    require_merge_precompiles
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
    local sender=$1 address=$2 data=$3 output hash sender_address nonce response exit_code
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
    while :; do
        if output=$("$PAXD" tx evm call-contract "$address" "$data" --from "$sender" \
            --home "$NODE_HOME" --keyring-backend test --chain-id "$CHAIN" --node "$NODE" \
            --evm-rpc "$EVM" --nonce "$((nonce))" --gas-fee-cap 1000000000000 --gas-limit 7000000 -y 2>&1); then
            break
        else
            exit_code=$?
        fi
        if [[ "$output" != *"no block is open"* ]] || (( SECONDS >= deadline )); then
            printf '%s\n' "$output" >&2
            return "$exit_code"
        fi
        sleep 0.2
    done
    hash=$(sed -n 's/^Transaction hash: //p' <<< "$output")
    wait_receipt "$hash"
}

feed_loop() {
    local previous=0 height timestamp result hash broadcast_code deadline
    trap 'SUCCESS=1; exit 0' TERM INT
    while kill -0 "$(cat "$NODE_HOME/node.pid")" 2>/dev/null; do
        height=$(rpc eth_blockNumber | jq -er '.result')
        if [ "$height" = "$previous" ]; then
            sleep 0.1
            continue
        fi
        previous=$height
        timestamp=$(date +%s%3N)
        printf 'pricefeed prepare height=%d timestamp_ms=%s txhash=pending broadcast_code=pending commit_height=pending commit_code=pending\n' \
            "$((height))" "$timestamp"
        jq -n --arg feeder "$TEST_ACCOUNT" --arg sequence "$((height))" --arg timestamp "$timestamp" \
            '{body:{messages:[{"@type":"/paxprotocol.paxchain.pricefeed.MsgSubmitPrices",
              feeder:$feeder,observations:[{market_id:"0",sequence:$sequence,index_price:"60000000000000000000000",
              mark_price:"60000000000000000000000",source_timestamp_ms:$timestamp},
              {market_id:"11",sequence:$sequence,index_price:"1000000000000000000",
              mark_price:"1000000000000000000",source_timestamp_ms:$timestamp}]}],
              memo:"",timeout_height:"0",extension_options:[],non_critical_extension_options:[]},
              auth_info:{signer_infos:[],fee:{amount:[],gas_limit:"2000000",payer:"",granter:""}},signatures:[]}' \
            > "$NODE_HOME/observation.json"
        "$PAXD" tx sign "$NODE_HOME/observation.json" --from test \
            --home "$NODE_HOME" --keyring-backend test --chain-id "$CHAIN" --node "$NODE" \
            --output-document "$NODE_HOME/observation-signed.json"
        result=$("$PAXD" tx broadcast "$NODE_HOME/observation-signed.json" \
            --home "$NODE_HOME" --node "$NODE" -b sync -o json)
        jq -c --argjson height "$((height))" \
            '{stage:"broadcast",height:$height,txhash, broadcast_code:.code,commit_height:null,commit_code:null}' <<< "$result"
        jq -e '(.code // 0) == 0 and (.txhash | test("^[0-9A-Fa-f]{64}$"))' <<< "$result" >/dev/null \
            || fail "pricefeed broadcast rejected: $result"
        hash=$(jq -r '.txhash' <<< "$result")
        broadcast_code=$(jq -r '.code // 0' <<< "$result")
        deadline=$((SECONDS + 30))
        while ! result=$("$PAXD" q tx "$hash" --node "$NODE" -o json 2>/dev/null); do
            (( SECONDS < deadline )) || fail "pricefeed transaction did not commit: $hash"
            sleep 0.1
        done
        jq -c --argjson height "$((height))" --argjson broadcast_code "$broadcast_code" \
            '{stage:"commit",height:$height,txhash,broadcast_code:$broadcast_code,commit_height:.height,commit_code:.code}' <<< "$result"
        jq -e '(.code // 0) == 0' <<< "$result" >/dev/null || fail "pricefeed transaction failed: $result"
    done
    fail "node exited"
}

sequencer_evidence() {
    local role port status_file log
    for role in node bootstrap-peer; do
        if [ "$role" = node ]; then
            port=${RPC_PORT:-}
        else
            port=${BOOTSTRAP_RPC_PORT:-}
        fi
        [ -n "$port" ] || continue
        status_file=$NODE_HOME/$role-status.json
        printf '\n--- %s /status; evidence: %s ---\n' "$role" "$status_file" >&2
        if curl --silent --show-error --max-time 3 \
            "http://127.0.0.1:$port/status" > "$status_file"; then
            cat "$status_file" >&2
        else
            printf 'v6120-node: %s /status request failed; response follows\n' "$role" >&2
            cat "$status_file" >&2
        fi
        printf '\n' >&2
    done
    for log in node bootstrap-peer; do
        [ -f "$NODE_HOME/$log.log" ] || continue
        printf '\n--- sequencer failures; full log retained: %s ---\n' "$NODE_HOME/$log.log" >&2
        grep -Ein -A 40 'crashed|panic' "$NODE_HOME/$log.log" >&2 || :
        if grep -Ein 'sequencer|replicat|commit failed for application|panic recovered in commit listener|standby stopped|refused|no ack|fenced|method handler crashed|panic|goroutine ' \
            "$NODE_HOME/$log.log" >&2; then
            :
        else
            printf 'v6120-node: no matching sequencer failure lines in %s\n' "$log.log" >&2
        fi
    done
    for role in node; do
        port=${PRIMARY_PROFILE_PORT:-}
        [ -n "$port" ] || continue
        log=$NODE_HOME/$role-goroutines.txt
        if curl --fail --silent --show-error --max-time 3 \
            "http://127.0.0.1:$port/debug/pprof/goroutine?debug=2" > "$log"; then
            printf '\n--- %s goroutines; evidence: %s ---\n' "$role" "$log" >&2
            cat "$log" >&2
        fi
    done
}

cleanup() {
    local status=$1 command=$2 log
    if [ "$SUCCESS" = 0 ]; then
        printf 'v6120-node: failed (exit %s): %s; evidence: %s\n' \
            "$status" "${FAILED_COMMAND:-$command}" "${NODE_HOME:-$BASE}" >&2
        if [ "${FEEDER_PROCESS:-0}" = 0 ]; then
            if [ "$SEQUENCER" = 1 ] && [ -n "$NODE_HOME" ]; then
                sequencer_evidence
            fi
            for log in init validate-genesis bootstrap bootstrap-peer node feeder; do
                if [ -n "$NODE_HOME" ] && [ -f "$NODE_HOME/$log.log" ]; then
                    printf '\n--- %s ---\n' "$NODE_HOME/$log.log" >&2
                    tail -n 40 "$NODE_HOME/$log.log" >&2
                fi
            done
        fi
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
        --sequencer) SEQUENCER=1; shift ;;
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
if [ "$SEQUENCER" = 1 ]; then
    BOOTSTRAP_HOME=$NODE_HOME/bootstrap-peer
    "$PAXD" init local-v6120-bootstrap --chain-id "$CHAIN" --mode full --home "$BOOTSTRAP_HOME" >> "$NODE_HOME/init.log" 2>&1
fi

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
PERPS_GENESIS=$(jq -ce '.app_state.perps | select(type == "object")' "$NODE_HOME/init-genesis.json")
jq --arg kept "$KEPT" --arg operator "$VALIDATOR" --arg old "$OLD_ACCOUNT" --arg account "${ACCOUNTS[2]}" \
    --arg cons "$CONS" --argjson pv "$PV" --arg now "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --argjson consensus "$CONSENSUS" --argjson sequencer "$SEQUENCER" --argjson perps "$PERPS_GENESIS" '
    def set_pool($address; $amount):
        .app_state.bank.balances = ([.app_state.bank.balances[] | select(.address != $address)]
          + if $amount == 0 then [] else [{address:$address,coins:[{denom:"uhpx",amount:($amount|tostring)}]}] end);
    walk(if type == "string" then
        if . == $kept then $operator elif . == $old then $account else . end else . end)
    | .chain_id = "hyperpax_125-1" | .genesis_time = $now
    | .app_state.staking.validators |= map(if .operator_address == $operator then
        .consensus_pubkey.key = $pv.pub_key.value | .jailed = false | .status = "BOND_STATUS_BONDED"
        | .tokens = ((([1000000000, (.tokens|tonumber)] | max)
          * if $sequencer == 1 then 3 else 1 end)|tostring)
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
    | ($now | fromdateiso8601) as $genesis_time
    | .app_state.perps = $perps
    | .app_state.perps.markets = [
        {id:0,symbol:"BTC",name:"Bitcoin",oracle_symbol:"BTC",sz_decimals:5,px_decimals:1,
         max_leverage:1000,tier_class:0,impact_notional_usd:"20000000000000000000000",
         oi_cap_usd:"500000000000000000000000000",funding_cap_ppm_per_hour:40000,
         listed_at:$genesis_time,enabled:true},
        {id:11,symbol:"PAX",name:"Paxeer Network",oracle_symbol:"PAX",sz_decimals:0,px_decimals:5,
         max_leverage:1000,tier_class:4,impact_notional_usd:"6000000000000000000000",
         oi_cap_usd:"0",funding_cap_ppm_per_hour:40000,listed_at:$genesis_time,enabled:true}]
    | .app_state.perps.market_states = [0,11] | .app_state.perps.market_states |= map(
        {market_id:.,index_price:"0",pricefeed_mark:"0",price_timestamp_ms:0,fresh:false,
         mark_price:"0",premium_ema:"0",last_trade_price:"0",funding_index:"0",hourly_rate:"0",
         premium_sum:"0",oracle_sum:"0",premium_count:0,last_sample_time:0,
         last_index_hour:(($genesis_time / 3600 | floor) - 1),
         long_open_interest:"0",short_open_interest:"0"})
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
if [ "$SEQUENCER" = 1 ]; then
    for home in "$NODE_HOME"; do
        "$PAXD" sequencer gen-receipt-key --output "$home/config/receipt-key.hex" \
            > "$home/receipt-public.json"
        chmod 600 "$home/config/receipt-key.hex"
        public=$(jq -er '.public_key' "$home/receipt-public.json")
        "$PAXD" add-genesis-receipt-key --home "$NODE_HOME" --public-key "$public" \
            --first-height 1 --last-height 9223372036854775807 >> "$NODE_HOME/init.log" 2>&1
    done
    for home in "$NODE_HOME"; do
        public=$(python3 - "$home/config/priv_validator_key.json" <<'PYKEY'
import base64
import json
import sys
with open(sys.argv[1]) as handle:
    print(base64.b64decode(json.load(handle)['pub_key']['value'], validate=True).hex())
PYKEY
        )
        "$PAXD" add-genesis-sequencer-leader --home "$NODE_HOME" --pubkey "$public" >> "$NODE_HOME/init.log" 2>&1
    done
    jq '.app_state.sequencer.activation_height = ((.initial_height | tonumber) + 2)' \
        "$GENESIS" > "$GENESIS.tmp"
    mv "$GENESIS.tmp" "$GENESIS"
    python3 - "$GENESIS" "$NODE_HOME/config/priv_validator_key.json" <<'PYIDENTITY'
import base64
import hashlib
import json
import sys

with open(sys.argv[1]) as handle:
    genesis = json.load(handle)
with open(sys.argv[2]) as handle:
    primary = json.load(handle)
staking = genesis['app_state']['staking']
primary_validator = next(v for v in staking['validators'] if v['status'] == 'BOND_STATUS_BONDED')
primary_power = int(genesis['validators'][0]['power'])
public = primary['pub_key']['value']
address = hashlib.sha256(base64.b64decode(public, validate=True)).hexdigest().upper()[:40]
validators = genesis['validators']
bonded = [v for v in staking['validators']
          if v['status'] == 'BOND_STATUS_BONDED' and not v['jailed']]
if (primary['address'] != address
        or len(validators) != 1 or len(bonded) != 1
        or validators[0]['address'] != address
        or validators[0]['pub_key'] != primary['pub_key']
        or bonded[0]['consensus_pubkey']['key'] != public
        or staking['last_validator_powers'] != [
            {'address': primary_validator['operator_address'], 'power': str(primary_power)}]
        or int(staking['last_total_power']) != primary_power
        or genesis['app_state']['sequencer']['leaders'] != [public]):
    raise SystemExit('sequencer committee and leaders must contain only the primary')
with open(sys.argv[1], 'w') as handle:
    json.dump(genesis, handle)
PYIDENTITY
fi
jq --slurpfile init "$NODE_HOME/init-genesis.json" '
    reduce ["layerxledger", "layerxauth", "layerxpay", "layerxagent"][] as $module (.;
      if .app_state[$module] == null then
        .app_state[$module] = ($init[0].app_state[$module]
          // error("missing merge module genesis: " + $module))
      else . end)
' "$GENESIS" > "$GENESIS.tmp"
mv "$GENESIS.tmp" "$GENESIS"
jq '.app_state.gov.voting_params.voting_period = "5s"' "$GENESIS" > "$GENESIS.tmp"
mv "$GENESIS.tmp" "$GENESIS"
jq --slurpfile init "$NODE_HOME/init-genesis.json" '
    .app_state = ($init[0].app_state + .app_state)
' "$GENESIS" > "$NODE_HOME/validation-genesis.json"
if [ "$SEQUENCER" = 1 ]; then
    cp "$GENESIS" "$BOOTSTRAP_HOME/config/genesis.json"
fi
"$PAXD" validate-genesis "$NODE_HOME/validation-genesis.json" --home "$NODE_HOME" > "$NODE_HOME/validate-genesis.log" 2>&1

read -r RPC_PORT P2P_PORT EVM_PORT GRPC_PORT WEB_PORT BOOTSTRAP_RPC_PORT BOOTSTRAP_P2P_PORT BOOTSTRAP_EVM_PORT BOOTSTRAP_GRPC_PORT BOOTSTRAP_WEB_PORT PRIMARY_PROFILE_PORT < <(python3 - "$SEQUENCER" <<'PY'
import socket
import sys
listeners = [socket.socket() for _ in range(11 if sys.argv[1] == "1" else 5)]
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
configure_node() {
    local home=$1 rpc_port=$2 p2p_port=$3 evm_port=$4 grpc_port=$5 web_port=$6 mode=$7
    local CONFIG=$home/config/config.toml APP=$home/config/app.toml
    set_toml "$CONFIG" '' mode "\"$mode\""
    set_toml "$CONFIG" rpc laddr "\"tcp://127.0.0.1:$rpc_port\""
    set_toml "$CONFIG" rpc pprof-laddr '""'
    set_toml "$CONFIG" p2p laddr "\"tcp://127.0.0.1:$p2p_port\""
    for setting in external-address bootstrap-peers persistent-peers; do
        set_toml "$CONFIG" p2p "$setting" '""'
    done
    set_toml "$CONFIG" p2p pex false
    set_toml "$CONFIG" instrumentation prometheus false
    set_toml "$APP" api enable false
    set_toml "$APP" grpc enable true
    set_toml "$APP" grpc address "\"127.0.0.1:$grpc_port\""
    set_toml "$APP" grpc-web enable false
    set_toml "$APP" grpc-web address "\"127.0.0.1:$web_port\""
    set_toml "$APP" evm http_enabled true
    set_toml "$APP" evm http_address '"127.0.0.1"'
    set_toml "$APP" evm http_port "$evm_port"
    set_toml "$APP" evm ws_enabled false
    set_toml "$APP" evm enable_test_api false
    if [ "$SEQUENCER" = 1 ]; then
        set_toml "$APP" state-commit sc-async-commit-buffer 0
    fi
}
configure_node "$NODE_HOME" "$RPC_PORT" "$P2P_PORT" "$EVM_PORT" "$GRPC_PORT" "$WEB_PORT" validator
HEIGHT=$(jq -er '.initial_height | tonumber | select(. > 0)' "$GENESIS")
jq -n --argjson height "$HEIGHT" '{name:"v6.12.0",height:$height}' > "$NODE_HOME/data/upgrade-info.json"
SEQUENCER_ARGS=()
if [ "$SEQUENCER" = 1 ]; then
    BOOTSTRAP_HEIGHT=$((HEIGHT + 1))
    configure_node "$BOOTSTRAP_HOME" "$BOOTSTRAP_RPC_PORT" "$BOOTSTRAP_P2P_PORT" \
        "$BOOTSTRAP_EVM_PORT" "$BOOTSTRAP_GRPC_PORT" "$BOOTSTRAP_WEB_PORT" full
    set_toml "$BOOTSTRAP_HOME/config/app.toml" grpc enable false
    primary_id=$("$PAXD" tendermint show-node-id --home "$NODE_HOME")
    bootstrap_id=$("$PAXD" tendermint show-node-id --home "$BOOTSTRAP_HOME")
    for home in "$NODE_HOME" "$BOOTSTRAP_HOME"; do
        set_toml "$home/config/config.toml" p2p allow-duplicate-ip true
    done
    set_toml "$NODE_HOME/config/config.toml" p2p persistent-peers \
        "\"$bootstrap_id@127.0.0.1:$BOOTSTRAP_P2P_PORT\""
    set_toml "$BOOTSTRAP_HOME/config/config.toml" p2p persistent-peers \
        "\"$primary_id@127.0.0.1:$P2P_PORT\""
    for home in "$BOOTSTRAP_HOME"; do
        cp "$NODE_HOME/data/upgrade-info.json" "$home/data/upgrade-info.json"
    done
    nohup "$PAXD" start --home "$BOOTSTRAP_HOME" --halt-height "$BOOTSTRAP_HEIGHT" \
        > "$NODE_HOME/bootstrap-peer.log" 2>&1 < /dev/null &
    echo "$!" > "$NODE_HOME/bootstrap-peer.pid"
    nohup "$PAXD" start --home "$NODE_HOME" --halt-height "$BOOTSTRAP_HEIGHT" \
        > "$NODE_HOME/bootstrap.log" 2>&1 < /dev/null &
    bootstrap_pid=$!
    echo "$bootstrap_pid" > "$NODE_HOME/node.pid"
    deadline=$((SECONDS + 120))
    while kill -0 "$bootstrap_pid" 2>/dev/null; do
        (( SECONDS < deadline )) || fail "bootstrap halt timed out: $NODE_HOME/bootstrap.log"
        sleep 0.2
    done
    bootstrap_status=0
    wait "$bootstrap_pid" || bootstrap_status=$?
    case "$bootstrap_status" in
        0|130|143) ;;
        *) fail "bootstrap failed (exit $bootstrap_status): $NODE_HOME/bootstrap.log" ;;
    esac
    grep -Eq "halting node per configuration.*height=$BOOTSTRAP_HEIGHT time=0$" "$NODE_HOME/bootstrap.log" \
        || fail "bootstrap did not reach its halt height: $NODE_HOME/bootstrap.log"
    stop_node "$NODE_HOME" bootstrap-peer
    cp -a "$NODE_HOME/data/." "$BOOTSTRAP_HOME/data/"
    for home in "$NODE_HOME" "$BOOTSTRAP_HOME"; do
        set_toml "$home/config/config.toml" consensus.sequencer enable true
    done
    set_toml "$NODE_HOME/config/config.toml" rpc pprof-laddr "\"127.0.0.1:$PRIMARY_PROFILE_PORT\""
    nohup "$PAXD" start --home "$BOOTSTRAP_HOME" >> "$NODE_HOME/bootstrap-peer.log" 2>&1 < /dev/null &
    echo "$!" > "$NODE_HOME/bootstrap-peer.pid"
    export PAX_LOG_LEVEL=info PAX_LOG_OUTPUT=stderr
    SEQUENCER_ARGS=(--log_level info --sequencer.enable --sequencer.mode sequential
        --sequencer.replication-timeout 10s --sequencer.lease-duration 30s
        --sequencer.nonce-queue-max-per-sender 16 --sequencer.nonce-queue-max-total 128)
    SEQUENCER_ARGS+=(--sequencer.role primary --sequencer.wal-dir "$NODE_HOME/sequencer-wal"
        --sequencer.receipt-key-file "$NODE_HOME/config/receipt-key.hex")
fi
nohup "$PAXD" start --home "$NODE_HOME" "${SEQUENCER_ARGS[@]}" > "$NODE_HOME/node.log" 2>&1 < /dev/null &
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
if [ "$SEQUENCER" = 1 ]; then
    deadline=$((SECONDS + 120))
    while :; do
        for role in node bootstrap-peer; do
            kill -0 "$(cat "$NODE_HOME/$role.pid")" 2>/dev/null || fail "$role exited: $NODE_HOME/$role.log"
        done
        if curl --fail --silent --show-error --max-time 3 "http://127.0.0.1:$RPC_PORT/status" \
            | jq -e '.sync_info.catching_up == false' >/dev/null; then
            break
        fi
        (( SECONDS < deadline )) || fail 'sequencer primary did not leave block sync'
        sleep 0.2
    done
    curl --fail --silent --show-error --max-time 3 \
        "http://127.0.0.1:$RPC_PORT/validators?per_page=100" > "$NODE_HOME/committee.json"
    python3 - "$GENESIS" "$NODE_HOME/committee.json" <<'PYCOMMITTEE'
import json
import sys

with open(sys.argv[1]) as handle:
    expected = json.load(handle)['validators']
with open(sys.argv[2]) as handle:
    result = json.load(handle)
actual = result['validators']
if (len(expected) != 1 or int(result['total']) != 1 or len(actual) != 1
        or [(v['address'], v['pub_key'], int(v['voting_power'])) for v in actual]
        != [(v['address'], v['pub_key'], int(v['power'])) for v in expected]):
    raise SystemExit('primary RPC committee must contain only the bonded primary sequencer leader')
PYCOMMITTEE
    stop_node "$NODE_HOME" bootstrap-peer
fi
printf -v MINT_DATA '40c10f19000000000000000000000000%s%064x' "${ETH_ADDRESSES[1]#0x}" 1000000000000
evm_call deployer "$USDL" "$MINT_DATA"
PERPS=0x0000000000000000000000000000000000001015
COLLATERAL_AMOUNT=1000000000
printf -v APPROVE_DATA '095ea7b3000000000000000000000000%s%064x' "${PERPS#0x}" "$COLLATERAL_AMOUNT"
evm_call test "$USDL" "$APPROVE_DATA"
printf -v DEPOSIT_DATA '47e7ef24000000000000000000000000%s%064x' "${USDL#0x}" "$COLLATERAL_AMOUNT"
evm_call test "$PERPS" "$DEPOSIT_DATA"
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
deadline=$((SECONDS + 90))
while :; do
    kill -0 "$(cat "$NODE_HOME/feeder.pid")" 2>/dev/null || fail "feeder exited: $NODE_HOME/feeder.log"
    if price=$("$PAXD" q pricefeed price 0 --node "$NODE" -o json 2>/dev/null) \
        && jq -e '.fresh == true and .index == "60000000000000000000000"' <<< "$price" >/dev/null \
        && price=$("$PAXD" q pricefeed price 11 --node "$NODE" -o json 2>/dev/null) \
        && jq -e '.fresh == true and .index == "1000000000000000000"' <<< "$price" >/dev/null; then
        break
    fi
    if (( SECONDS >= deadline )); then
        for market in 0 11; do
            printf '\n--- pricefeed price %s ---\n' "$market" >&2
            if ! "$PAXD" q pricefeed price "$market" --node "$NODE" -o json >&2; then
                printf 'v6120-node: pricefeed price %s query failed\n' "$market" >&2
            fi
        done
        printf '\n--- pricefeed params ---\n' >&2
        if ! "$PAXD" q pricefeed params --node "$NODE" -o json >&2; then
            printf 'v6120-node: pricefeed params query failed\n' >&2
        fi
        printf '\n--- latest block time (Unix seconds in hex) ---\n' >&2
        if ! rpc eth_getBlockByNumber '["latest",false]' \
            | jq '{number:.result.number,timestamp:.result.timestamp,error}' >&2; then
            printf 'v6120-node: latest block time query failed\n' >&2
        fi
        fail "BTC and PAX prices did not become fresh: $NODE_HOME/feeder.log"
    fi
    sleep 0.2
done
if [ "$SEQUENCER" = 0 ]; then
    activate_merge
fi
{
    printf 'LOCAL_NODE_RPC=%q\n' "$EVM"
    printf 'LOCAL_NODE_HOME=%q\n' "$NODE_HOME"
    printf 'LOCAL_NODE_KEY=%q\n' "0x$KEY"
    if [ "$SEQUENCER" = 1 ]; then
        printf 'LOCAL_NODE_SEQUENCER_RPC=%q\n' "$EVM"
        printf 'LOCAL_NODE_ADDRESS=%q\n' "${ETH_ADDRESSES[1]}"
        printf 'LOCAL_NODE_PROFILE_RPC=%q\n' "http://127.0.0.1:$PRIMARY_PROFILE_PORT"
    fi
    printf 'PERPS_DATA_TEST_RPC=%q\n' "$EVM"
    printf 'PERPS_DATA_TEST_COMET=%q\n' "http://127.0.0.1:$RPC_PORT"
    printf 'NEXT_PUBLIC_PERPS_GATEWAY_URL=%q\n' "$GATEWAY"
} > "$NODE_HOME/localnode.env"
cp "$NODE_HOME/localnode.env" "$ENV_OUT"
SUCCESS=1
printf 'v6120-node: ready; home=%s env=%s\n' "$NODE_HOME" "$ENV_OUT" >&2
