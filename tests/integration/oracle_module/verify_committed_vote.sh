#!/usr/bin/env bash
set -euo pipefail

response=$1
height=$2
validator=$3
feeder=$4
rates=$5
[[ "$height" =~ ^[1-9][0-9]*$ ]] || exit 1
code=$(jq -er '.code | select(. == 0)' <<< "$response")
txhash=$(jq -er '.txhash | select(test("^[A-Fa-f0-9]{64}$")) | ascii_uppercase' <<< "$response")

# Block and execution-result queries work even when transaction indexing is off.
deadline=$((SECONDS + 60))
while (( SECONDS < deadline )); do
    latest=$(paxd status | jq -er '.SyncInfo.latest_block_height | select(test("^[1-9][0-9]*$"))')
    while (( height <= latest )); do
        block=$(curl --fail --silent --show-error --max-time 5 "http://localhost:26657/block?height=$height")
        jq -e --arg height "$height" '.result.block.header.height == $height' <<< "$block" >/dev/null
        transactions=$(jq -r '.result.block.data.txs[]?' <<< "$block")
        index=0
        while IFS= read -r encoded; do
            [[ -n "$encoded" ]] || continue
            observed=$(printf '%s' "$encoded" | base64 --decode | sha256sum)
            observed=${observed%% *}
            if [[ "${observed^^}" == "$txhash" ]]; then
                results=$(curl --fail --silent --show-error --max-time 5 "http://localhost:26657/block_results?height=$height")
                # ExecTxResult.code is uint32 with omitempty: absence encodes zero.
                jq -e --arg height "$height" --argjson index "$index" \
                    --arg validator "$validator" --arg feeder "$feeder" --arg rates "$rates" '
                    .result | select(.height == $height) | .txs_results[$index] |
                    (.code == 0 or (has("code") | not)) and
                    any(.events[]; .type == "aggregate_vote" and
                        any(.attributes[]; (.key | @base64d) == "voter" and (.value | @base64d) == $validator) and
                        any(.attributes[]; (.key | @base64d) == "exchange_rates" and (.value | @base64d) == $rates)) and
                    any(.events[]; .type == "message" and
                        any(.attributes[]; (.key | @base64d) == "module" and (.value | @base64d) == "oracle") and
                        any(.attributes[]; (.key | @base64d) == "sender" and (.value | @base64d) == $feeder))
                ' <<< "$results" >/dev/null
                printf '%s\n' "$code"
                exit 0
            fi
            index=$((index + 1))
        done <<< "$transactions"
        height=$((height + 1))
        (( SECONDS < deadline )) || break
    done
    sleep 0.5
done
echo "Timed out waiting for committed oracle vote $txhash" >&2
exit 1
