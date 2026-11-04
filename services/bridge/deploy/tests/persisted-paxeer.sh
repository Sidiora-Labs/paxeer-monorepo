#!/usr/bin/env bash
# Sourced by the two live dry runs; all state belongs to their private WORK.

prepare_paxeer() {
    PAXEER_HOME="$WORK/paxd"
    PAXEER_CHAIN_ID=pax-bridge-dry-run
    read -r PAXEER_RPC_PORT PAXEER_P2P_PORT PAXEER_EVM_PORT < <(python3 - << 'PY'
import socket
sockets = [socket.socket() for _ in range(3)]
for sock in sockets:
    sock.bind(("127.0.0.1", 0))
print(*(sock.getsockname()[1] for sock in sockets))
for sock in sockets:
    sock.close()
PY
    )
    paxd init bridge-dry-run --chain-id "$PAXEER_CHAIN_ID" --mode validator --home "$PAXEER_HOME" \
        > "$WORK/paxeer-init.log" 2>&1 || fail "paxd init failed"
    PAXEER_DENOM=$(python3 "$SCRIPT_DIR/recovery.py" configure "$PAXEER_HOME" \
        "$PAXEER_RPC_PORT" "$PAXEER_P2P_PORT" "$PAXEER_EVM_PORT") || fail "paxd configuration failed"
    paxd keys add validator --keyring-backend test --home "$PAXEER_HOME" \
        > "$WORK/paxeer-key.log" 2>&1 || fail "paxd key creation failed"
    PAXEER_SIGNER=$(paxd keys show validator -a --keyring-backend test --home "$PAXEER_HOME")
    paxd add-genesis-account "$PAXEER_SIGNER" "100000000000000000000$PAXEER_DENOM" \
        --keyring-backend test --home "$PAXEER_HOME" || fail "paxd genesis funding failed"
    paxd gentx validator "7000000000000000$PAXEER_DENOM" --chain-id "$PAXEER_CHAIN_ID" \
        --keyring-backend test --home "$PAXEER_HOME" > "$WORK/paxeer-gentx.log" 2>&1 || fail "paxd gentx failed"
    jq --slurpfile key "$PAXEER_HOME/config/priv_validator_key.json" \
        '.validators = [{pub_key: $key[0].pub_key, power: "7000000000"}]' \
        "$PAXEER_HOME/config/genesis.json" > "$WORK/genesis.json"
    mv "$WORK/genesis.json" "$PAXEER_HOME/config/genesis.json"
    paxd collect-gentxs --home "$PAXEER_HOME" > "$WORK/paxeer-collect.log" 2>&1 || fail "paxd collect-gentxs failed"
    PAXEER_NODE="tcp://127.0.0.1:$PAXEER_RPC_PORT"
    PAXEER_URL="http://127.0.0.1:$PAXEER_EVM_PORT"
    start_paxeer
}

start_paxeer() {
    # On recovery this opens the same database and validator signing state.
    paxd start --home "$PAXEER_HOME" > "$WORK/paxeer.log" 2>&1 &
    PAXEER_PID=$!
    PIDS+=("$PAXEER_PID")
    local ready=0 response
    for _ in $(seq 1 600); do
        kill -0 "$PAXEER_PID" 2> /dev/null || fail "paxd stopped; see $WORK/paxeer.log"
        response=$(curl --silent --fail --max-time 2 --noproxy '*' \
            "http://127.0.0.1:$PAXEER_RPC_PORT/status" 2> /dev/null || true)
        if jq -e '.result.sync_info | (.catching_up == false and (.latest_block_height | tonumber) > 1)' \
            <<< "$response" > /dev/null 2>&1 \
            && curl --silent --fail --max-time 2 --noproxy '*' --header 'Content-Type: application/json' \
                --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}' "$PAXEER_URL" \
                | jq -e '.result | type == "string"' > /dev/null 2>&1; then
            ready=1
            break
        fi
        sleep 0.2
    done
    [ "$ready" -eq 1 ] || fail "paxd did not become ready"
}

paxeer_tx() {
    paxd tx "$@" --from validator --home "$PAXEER_HOME" --keyring-backend test \
        --chain-id "$PAXEER_CHAIN_ID" --node "$PAXEER_NODE" --gas 2000000 \
        --fees "1$PAXEER_DENOM" --broadcast-mode block --output json --yes
}

submit_bridge_proposal() {
    local file=$1 wanted=$2 response id status="" query
    response=$(paxeer_tx gov submit-proposal layerxbridge-proposal "$file" --deposit "1$PAXEER_DENOM") \
        || fail "governance did not accept $file"
    jq -e '.code == 0' <<< "$response" > /dev/null || fail "governance submit failed: $response"
    id=$(jq -er '[.logs[].events[] | select(.type == "submit_proposal") | .attributes[] |
        select(.key == "proposal_id") | .value] | unique | if length == 1 then .[0] else error("proposal id") end' \
        <<< "$response") || fail "governance returned no unique proposal id"
    response=$(paxeer_tx gov vote "$id" yes) || fail "governance vote failed"
    jq -e '.code == 0' <<< "$response" > /dev/null || fail "governance vote failed: $response"
    for _ in $(seq 1 600); do
        query=$(paxd query gov proposal "$id" --node "$PAXEER_NODE" --home "$PAXEER_HOME" --output json) \
            || fail "governance proposal query failed"
        status=$(jq -er '.status' <<< "$query") || fail "governance returned no proposal status"
        case $status in
        PROPOSAL_STATUS_PASSED|PROPOSAL_STATUS_FAILED|PROPOSAL_STATUS_REJECTED) break ;;
        esac
        kill -0 "$PAXEER_PID" 2> /dev/null || fail "paxd stopped while voting"
        sleep 0.2
    done
    expect "governance outcome of ${file##*/}" "$wanted" "$status"
    printf '%s\n' "$query" > "$WORK/proposal-$id.json"
}

assert_paxeer_recovery() {
    # This includes both generated proposals, their original order and results.
    local before=$1 after=$2
    paxd query gov proposals --node "$PAXEER_NODE" --home "$PAXEER_HOME" --output json \
        > "$after" || fail "paxd governance recovery query failed"
    [ "$(jq -S '.proposals' "$before")" = "$(jq -S '.proposals' "$after")" ] \
        || fail "paxd lost or changed its governance state on restart"
}
