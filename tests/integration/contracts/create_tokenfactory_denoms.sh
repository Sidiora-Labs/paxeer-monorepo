#!/bin/bash

paxdbin=$(which ~/go/bin/paxd | tr -d '"')
keyname=$(printf "12345678\n" | $paxdbin keys list --output json | jq ".[0].name" | tr -d '"')
keyaddress=$(printf "12345678\n" | $paxdbin keys list --output json | jq ".[0].address" | tr -d '"')
chainid=$($paxdbin status | jq ".NodeInfo.network" | tr -d '"')
paxhome=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)

source "$(dirname "$0")/../utils/_tx_helpers.sh"

cd $paxhome || exit
echo "Deploying first set of tokenfactory denoms..."

beginning_block_height=$($paxdbin status | jq -r '.SyncInfo.latest_block_height')
echo "$beginning_block_height" > $paxhome/tests/integration/contracts/tfk_beginning_block_height.txt
echo "$keyaddress"  > $paxhome/tests/integration/contracts/tfk_creator_id.txt

# Create a tokenfactory denom and wait for it to appear on chain. The
# denom is deterministic: factory/<creator>/<subdenom>. Submitted via
# -b sync (the cosmos KV indexer isn't fed under Autobahn, so -b block
# hangs); poll denom-authority-metadata for non-empty admin as the
# side-effect signal that DeliverTx committed.
create_denom_and_wait() {
    local subdenom="$1"
    local new_token_denom="factory/$keyaddress/$subdenom"
    local resp; resp=$(printf "12345678\n" | $paxdbin tx tokenfactory create-denom "$subdenom" \
        -y --from="$keyname" --chain-id="$chainid" --gas=500000 --fees=100000uhpx \
        --broadcast-mode=sync --output=json)
    local code; code=$(echo "$resp" | jq -r '.code // 0')
    if [ "$code" != "0" ]; then
        echo "create_denom_and_wait CheckTx rejected: $(echo "$resp" | jq -r '.raw_log')" >&2
        return 1
    fi
    # denom-authority-metadata returns {"authority_metadata":{"admin":""}}
    # (no error) for a non-existent denom; only a non-empty admin
    # indicates create-denom is committed.
    _wait_until "tokenfactory denom $new_token_denom" "
        admin=\$($paxdbin q tokenfactory denom-authority-metadata $new_token_denom -o json 2>/dev/null | jq -r '.authority_metadata.admin // \"\"')
        [ -n \"\$admin\" ]
    " || return 1
    echo "$new_token_denom"
}

# create first set of tokenfactory denoms
for i in {1..10}
do
    echo "Creating first set of tokenfactory denoms #$i..."
    new_token_denom=$(create_denom_and_wait "$i") || exit 1
    echo "Got token $new_token_denom for iteration $i"
done


first_set_block_height=$($paxdbin status | jq -r '.SyncInfo.latest_block_height')
echo "$first_set_block_height" > $paxhome/tests/integration/contracts/tfk_first_set_block_height.txt

sleep 5

# create second set of tokenfactory denoms
for i in {11..20}
do
    echo "Creating first set of tokenfactory denoms #$i..."
    new_token_denom=$(create_denom_and_wait "$i") || exit 1
    echo "Got token $new_token_denom for iteration $i"
done

second_set_block_height=$($paxdbin status | jq -r '.SyncInfo.latest_block_height')
echo "$second_set_block_height" > $paxhome/tests/integration/contracts/tfk_second_set_block_height.txt

sleep 5

# create third set of tokenfactory denoms
for i in {21..30}
do
    echo "Creating first set of tokenfactory denoms #$i..."
    new_token_denom=$(create_denom_and_wait "$i") || exit 1
    echo "Got token $new_token_denom for iteration $i"
done

third_set_block_height=$($paxdbin status | jq -r '.SyncInfo.latest_block_height')
echo "$third_set_block_height" > $paxhome/tests/integration/contracts/tfk_third_set_block_height.txt

num_denoms=$(python3 - "$paxdbin" "$keyaddress" <<'PY'
import base64
import json
import subprocess
import sys


def creator_queries(paxdbin, creator):
    """Exchange CLI argument lists and JSON pages, then return the denom count."""
    if not paxdbin or not creator:
        raise ValueError("binary and creator address are required")
    expected = {f"factory/{creator}/{i}" for i in range(1, 31)}
    denoms = set()
    seen_keys = set()
    page_key = ""
    while True:
        command = [
            paxdbin, "q", "tokenfactory", "denoms-from-creator", creator,
            "--output", "json", "--limit", "100",
        ]
        if page_key:
            command.extend(["--page-key", page_key])
        page = json.loads((yield command))
        if not isinstance(page, dict) or not isinstance(page.get("denoms"), list):
            raise ValueError("creator query must return a denoms array")
        for denom in page["denoms"]:
            if not isinstance(denom, str) or not denom:
                raise ValueError("creator query returned an invalid denom")
            if denom in denoms:
                raise ValueError("creator query repeated a denom across pages")
            denoms.add(denom)
        pagination = page.get("pagination")
        if pagination is None:
            next_key = None
        elif isinstance(pagination, dict):
            next_key = pagination.get("next_key")
        else:
            raise ValueError("creator query returned invalid pagination")
        if next_key is None or next_key == "":
            break
        if not isinstance(next_key, str):
            raise ValueError("creator query returned an invalid next_key")
        # JSON encodes PageResponse bytes as base64; ReadPageRequest uses
        # []byte(pageKey), so --page-key must receive the decoded store key.
        page_key = base64.b64decode(next_key, validate=True).decode("utf-8")
        if not page_key or "\0" in page_key or page_key in seen_keys:
            raise ValueError("creator query returned a repeated or invalid page key")
        seen_keys.add(page_key)
    missing = expected - denoms
    if missing:
        raise ValueError(
            "creator query is missing expected denoms: " + ", ".join(sorted(missing))
        )
    return len(denoms)


def verify_creator_denoms(paxdbin, creator):
    queries = creator_queries(paxdbin, creator)
    command = next(queries)
    while True:
        response = subprocess.run(command, check=True, stdout=subprocess.PIPE, text=True)
        try:
            command = queries.send(response.stdout)
        except StopIteration as complete:
            return complete.value


if __name__ == "__main__":
    try:
        print(verify_creator_denoms(*sys.argv[1:]))
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"tokenfactory creator verification failed: {error}", file=sys.stderr)
        sys.exit(1)
PY
) || exit 1
echo "$num_denoms"

exit 0
