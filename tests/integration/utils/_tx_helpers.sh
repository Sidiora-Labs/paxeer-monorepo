#!/bin/bash
#
# Transaction wait helpers for deploy bash scripts. Exact inclusion, execution
# and Wasm identity use finalized block bytes and results via _tx_receipts.py.
# This works without the KV transaction index that -b block requires under
# Autobahn. The remaining sequence/governance helpers poll their state effects.
#
# Source from a deploy script after setting $paxdbin, $keyname, $chainid.

: "${TX_WAIT_TIMEOUT:=30}"
: "${TX_WAIT_INTERVAL:=0.5}"

# Cosmos account sequence for an address; 0 if the account doesn't exist
# yet. Causal "tx committed" signal: a sender's sequence advances
# atomically when its tx is included in a block, regardless of whether
# the tx's intended side effect happened.
_get_account_sequence() {
    $paxdbin q account "$1" -o json 2>/dev/null | jq -r '.sequence // 0'
}

# Cosmos account sequence for an address at a historical height. Echoes
# the sequence number (0 if the account didn't exist yet at <height>),
# or empty on query failure — distinct from 0 so the caller can retry
# instead of misreading a transient RPC error as "pre-tx sequence".
_get_account_sequence_at_height() {
    local raw; raw=$($paxdbin q account "$1" --height "$2" -o json 2>/dev/null)
    [ -z "$raw" ] && return
    echo "$raw" | jq -r '.sequence // 0'
}

# Poll an arbitrary check until it exits 0. Mirrors lib.js's
# waitForCondition.
# Usage: _wait_until <description> <check_cmd>
_wait_until() {
    local description="$1"
    local check_cmd="$2"
    local deadline=$(($(date +%s) + TX_WAIT_TIMEOUT))
    while [ "$(date +%s)" -lt "$deadline" ]; do
        if eval "$check_cmd" >/dev/null 2>&1; then return 0; fi
        sleep "$TX_WAIT_INTERVAL"
    done
    echo "timed out waiting for $description within ${TX_WAIT_TIMEOUT}s" >&2
    return 1
}

# Keep the receipt worker next to the sourced helper, independent of caller cwd.
_TX_RECEIPTS="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/_tx_receipts.py"

_tx_receipt() {
    python3 "$_TX_RECEIPTS" "$@" --paxdbin "$paxdbin" --chain-id "$chainid" \
        --timeout "$TX_WAIT_TIMEOUT" --interval "$TX_WAIT_INTERVAL"
}

# Capture the finalized head before submission so the transaction can be found
# without a KV transaction index. Failure to resolve its hash is a hard timeout.
# Usage: height=$(bank_send_and_get_height <from-key> <to-addr> <amount-with-denom>) || exit 1
bank_send_and_get_height() {
    local from_key="$1"
    local to_addr="$2"
    local amount_denom="$3"
    local context; context=$(_tx_receipt prepare) || return 1
    local resp; resp=$(printf "12345678\n" | $paxdbin tx bank send "$from_key" "$to_addr" "$amount_denom" \
        -y --chain-id="$chainid" --gas=5000000 --fees=1000000uhpx \
        --broadcast-mode=sync --output=json) || return 1
    printf '%s\n' "$resp" | _tx_receipt height --context "$context"
}

# Poll until the chain's latest height exceeds the given height. Useful
# as a barrier between two sends from the same key when callers need
# them to land in distinct blocks (e.g., historical balance queries).
# Usage: wait_until_height_exceeds <min-height>
wait_until_height_exceeds() {
    local min_height="$1"
    _wait_until "chain height > $min_height" \
        "[ \$($paxdbin status | jq -r .SyncInfo.latest_block_height) -gt $min_height ]"
}

# Echo the committed execution code for this hash, including message failures
# after successful ante handling. CheckTx rejection is named separately on stderr
# and echoed immediately; missing committed evidence returns shell failure.
# Usage: code=$(submit_tx_and_wait <from-key> <subcmd-and-args...>) || exit 1
submit_tx_and_wait() {
    local from_key="$1"; shift
    local context; context=$(_tx_receipt prepare) || return 1
    local resp; resp=$(printf "12345678\n" | $paxdbin tx "$@" --from "$from_key" \
        -y --chain-id="$chainid" --broadcast-mode=sync --output=json) || return 1
    printf '%s\n' "$resp" | _tx_receipt code --context "$context"
}

# Return only the code ID in this transaction's store_code event, after checking
# the committed code metadata against the uploader and uncompressed Wasm hash.
# Usage: code_id=$(store_wasm <path/to.wasm>) || exit 1
store_wasm() {
    local wasm_path="$1"
    local creator; creator=$(printf "12345678\n" | $paxdbin keys show "$keyname" -a) || return 1
    [ -n "$creator" ] || return 1
    local context; context=$(_tx_receipt prepare) || return 1
    local resp; resp=$(printf "12345678\n" | $paxdbin tx wasm store "$wasm_path" \
        -y --from="$keyname" --chain-id="$chainid" \
        --gas=5000000 --fees=1000000uhpx \
        --broadcast-mode=sync --output=json) || return 1
    printf '%s\n' "$resp" | _tx_receipt store --context "$context" \
        --creator "$creator" --wasm-path "$wasm_path"
}

# Return this transaction's instantiate event address after checking its creator,
# code, label and INIT history message at the exact committed height.
# Usage: addr=$(instantiate_wasm <code_id> <init_msg> <label> [extra flags...]) || exit 1
instantiate_wasm() {
    local code_id="$1"; shift
    local init_msg="$1"; shift
    local label="$1"; shift
    local creator; creator=$(printf "12345678\n" | $paxdbin keys show "$keyname" -a) || return 1
    [ -n "$creator" ] || return 1
    local context; context=$(_tx_receipt prepare) || return 1
    local resp; resp=$(printf "12345678\n" | $paxdbin tx wasm instantiate "$code_id" "$init_msg" \
        --label="$label" -y --from="$keyname" --chain-id="$chainid" \
        --gas=5000000 --fees=1000000uhpx \
        "$@" --broadcast-mode=sync --output=json) || return 1
    printf '%s\n' "$resp" | _tx_receipt instantiate --context "$context" \
        --creator "$creator" --code-id "$code_id" --label "$label" --init-msg "$init_msg"
}

# Submit `bank send` via -b sync and wait for the sender's account
# sequence to advance — a denom-agnostic causal "tx committed" signal.
# Use between consecutive sends from the same key so the next CLI
# invocation reads a fresh sequence rather than racing the prior commit.
# Usage: bank_send_and_wait <from-key> <to-addr> <amount-with-denom>
bank_send_and_wait() {
    local from_key="$1"
    local to_addr="$2"
    local amount_denom="$3"
    local from_addr; from_addr=$(printf "12345678\n" | $paxdbin keys show "$from_key" -a 2>/dev/null)
    local seq_before; seq_before=$(_get_account_sequence "$from_addr")
    local resp; resp=$(printf "12345678\n" | $paxdbin tx bank send "$from_key" "$to_addr" "$amount_denom" \
        -y --chain-id="$chainid" --gas=5000000 --fees=1000000uhpx \
        --broadcast-mode=sync --output=json)
    local code; code=$(echo "$resp" | jq -r '.code // 0')
    if [ "$code" != "0" ]; then
        echo "bank_send_and_wait CheckTx rejected: $(echo "$resp" | jq -r '.raw_log')" >&2
        return 1
    fi
    _wait_until "$from_addr sequence > $seq_before" \
        "[ \$(_get_account_sequence $from_addr) -gt $seq_before ]" || return 1
}

# Highest existing gov proposal id (0 if none / on transient query
# failure). `paxd q gov proposals` exits non-zero with "no proposals
# found" on an empty gov set, in which case the jq pipeline reads
# empty stdin and emits nothing — explicit fallback to 0 keeps callers
# from hitting "integer expression expected" on empty $().
_get_max_proposal_id() {
    local out; out=$($paxdbin q gov proposals --reverse --limit 1 -o json 2>/dev/null)
    if [ -z "$out" ]; then echo 0; return; fi
    local id; id=$(echo "$out" | jq -r '.proposals[0].proposal_id // .proposals[0].id // 0' 2>/dev/null)
    echo "${id:-0}"
}

# After a -b sync gov proposal submission, scan gov state for the new
# proposal whose title matches and echo its id. The diff against
# max_id_before pins it to *this* submission rather than a stale
# prior-run proposal with the same title. Mirrors lib.js's
# findProposalByTitle.
# Usage: find_proposal_by_title <title> <max_id_before> [<tx_hash_for_error>]
find_proposal_by_title() {
    local title="$1"
    local max_id_before="$2"
    local tx_hash="${3:-unknown}"
    local deadline=$(($(date +%s) + TX_WAIT_TIMEOUT))
    while [ "$(date +%s)" -lt "$deadline" ]; do
        local cur; cur=$(_get_max_proposal_id)
        if [ "$cur" -gt "$max_id_before" ]; then
            local id
            local query_failed=0
            for ((id=max_id_before+1; id<=cur; id++)); do
                local raw; raw=$($paxdbin q gov proposal "$id" -o json 2>/dev/null)
                if [ -z "$raw" ]; then
                    # Transient query failure (RPC blip, indexer lag,
                    # etc.) — leave the id for re-scan on the next
                    # iteration rather than treating empty-response as
                    # "wrong title" and skipping past it.
                    query_failed=1
                    continue
                fi
                local observed; observed=$(echo "$raw" | jq -r '.content.title // .title // ""')
                if [ "$observed" = "$title" ]; then echo "$id"; return 0; fi
            done
            # Only advance the window if every id in the range was
            # successfully scanned. A transient miss leaves max_id_before
            # unchanged so the failed id is re-checked next iteration —
            # without this, the proposal we just submitted could be
            # permanently skipped by a single failed query.
            if [ "$query_failed" = 0 ]; then
                max_id_before=$cur
            fi
        fi
        sleep "$TX_WAIT_INTERVAL"
    done
    echo "find_proposal_by_title: proposal (tx $tx_hash) with title '$title' did not appear in gov state within ${TX_WAIT_TIMEOUT}s" >&2
    return 1
}

# Submit a gov proposal via -b sync and echo the new proposal id once
# it appears in gov state. Bundles _get_max_proposal_id, the submit,
# and find_proposal_by_title so yaml tests can capture PROPOSAL_ID in
# a single cmd: without parsing DeliverTx event logs.
# Usage: id=$(submit_gov_proposal <from-key> <title> <subcmd-and-args...>)
submit_gov_proposal() {
    local from_key="$1"; shift
    local title="$1"; shift
    local max_before; max_before=$(_get_max_proposal_id)
    local resp; resp=$(printf "12345678\n" | $paxdbin tx "$@" --from "$from_key" \
        -y --chain-id="$chainid" --broadcast-mode=sync --output=json)
    local code; code=$(echo "$resp" | jq -r '.code // 0')
    if [ "$code" != "0" ]; then
        echo "submit_gov_proposal CheckTx rejected: $(echo "$resp" | jq -r '.raw_log')" >&2
        return 1
    fi
    local txhash; txhash=$(echo "$resp" | jq -r '.txhash')
    find_proposal_by_title "$title" "$max_before" "$txhash"
}
