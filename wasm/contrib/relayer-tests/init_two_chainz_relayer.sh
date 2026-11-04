#!/bin/bash
# init_two_chainz_relayer creates two wasmd chains and configures the relayer

set -euo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
WASMD_DATA="$SCRIPTDIR/data"
RELAYER_HOME="$SCRIPTDIR/.relayer"
HARNESS="$SCRIPTDIR/harness.py"

for dependency in rly wasmd jq python3; do
  command -v "$dependency" >/dev/null || { echo "Error: $dependency is required." >&2; exit 1; }
done

echo "WASMD VERSION INFO:"
wasmd version --long

# Refuse ambiguous homes and stop only previously recorded harness children.
python3 "$HARNESS" homes "$SCRIPTDIR"
python3 "$HARNESS" stop "$WASMD_DATA"
rm -rf -- "$WASMD_DATA" "$RELAYER_HOME"

supervisors=()
cleanup() {
  result=$?
  trap - EXIT INT TERM
  if (( result != 0 )); then
    # Initialization may still be running: signal the supervisors, whose traps
    # validate and stop their own nodes, then reap those direct children.
    for supervisor in "${supervisors[@]}"; do
      kill -TERM "$supervisor" 2>/dev/null || true
    done
    for supervisor in "${supervisors[@]}"; do
      wait "$supervisor" || true
    done
    python3 "$HARNESS" stop "$WASMD_DATA" || result=1
  fi
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

chainid0=ibc-0
chainid1=ibc-1

echo "Generating wasmd configurations..."
mkdir -p "$WASMD_DATA"
"$SCRIPTDIR/one_chain.sh" wasmd "$chainid0" "$WASMD_DATA" 26657 26656 6060 9090 &
supervisors+=("$!")
"$SCRIPTDIR/one_chain.sh" wasmd "$chainid1" "$WASMD_DATA" 26557 26556 6061 9091 &
supervisors+=("$!")
for chain in "$chainid0" "$chainid1"; do
  ready=0
  for ((attempt=0; attempt<60; attempt++)); do
    for supervisor in "${supervisors[@]}"; do
      kill -0 "$supervisor" || { echo "Chain initialization failed" >&2; exit 1; }
    done
    if [[ -f "$WASMD_DATA/pids/$chain.json" ]]; then
      ready=1
      break
    fi
    sleep 1
  done
  (( ready == 1 )) || { echo "Timed out initializing $chain" >&2; exit 1; }
  echo "$chain initialized. Watch $WASMD_DATA/$chain.log for execution."
done

echo "Generating rly configurations..."
rly --home "$RELAYER_HOME" config init
rly --home "$RELAYER_HOME" config add-chains "$SCRIPTDIR/configs/wasmd/chains"
rly --home "$RELAYER_HOME" config add-paths "$SCRIPTDIR/configs/wasmd/paths"

SEED0=$(jq -er '.mnemonic' "$WASMD_DATA/ibc-0/key_seed.json")
SEED1=$(jq -er '.mnemonic' "$WASMD_DATA/ibc-1/key_seed.json")
rly --home "$RELAYER_HOME" keys restore ibc-0 testkey "$SEED0"
rly --home "$RELAYER_HOME" keys restore ibc-1 testkey "$SEED1"
echo "Creating light clients..."
sleep 3
rly --home "$RELAYER_HOME" light init ibc-0 -f
rly --home "$RELAYER_HOME" light init ibc-1 -f
