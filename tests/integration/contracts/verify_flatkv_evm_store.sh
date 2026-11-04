#!/bin/bash

set -euo pipefail

export PATH="$PATH:/root/go/bin:/usr/local/go/bin"

paxhome=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
flatkv_dir=${FLATKV_DIR:-/root/.pax/data/state_commit/flatkv}
dump_dir=${FLATKV_EVM_DUMP_DIR:-/tmp/flatkv-evm-dump}
storage_dump="$dump_dir/storage"
contract_addr_file="$paxhome/tests/integration/contracts/flatkv_evm_contract_addr.txt"

cd "$paxhome"

if [ ! -x build/paxdb ]; then
  echo "Building paxdb for FlatKV smoke verification..."
  GOPROXY=${GOPROXY:-https://proxy.golang.org,direct} go build -o build/paxdb ./storage/tools/cmd/paxdb
fi

rm -rf "$dump_dir"
mkdir -p "$dump_dir"

dump_flatkv_layout() {
  # Snapshot the on-disk state of $flatkv_dir for triage. dump-flatkv fails
  # opaquely ("clone aborted after 3 retries...") when, for example, the
  # live node never wrote a snapshot (=> no `current` symlink) or wrote it
  # to a different path than $flatkv_dir. Printing the layout removes that
  # ambiguity from the next CI run.
  echo "==================== app.toml FlatKV-related settings ====================" >&2
  grep -E '^(sc-write-mode|sc-keys-to-migrate-per-block|evm-ss-split)' \
    /root/.pax/config/app.toml >&2 2>/dev/null || true
  for candidate in "$flatkv_dir" /root/.pax/data/flatkv; do
    echo "==================== FlatKV directory state at $candidate ====================" >&2
    if [ ! -d "$candidate" ]; then
      echo "(directory $candidate does not exist)" >&2
    else
      ls -la "$candidate" >&2 || true
      for snap in "$candidate"/snapshot-*; do
        [ -d "$snap" ] || continue
        echo "---- $snap ----" >&2
        ls -la "$snap" >&2 || true
      done
    fi
  done
  local paxd_log="/pax-protocol/pax-chain/build/generated/logs/paxd-0.log"
  echo "==================== paxd-0.log (head 60) ====================" >&2
  head -60 "$paxd_log" >&2 2>/dev/null || echo "(no paxd-0.log)" >&2
}
trap 'rc=$?; if [ "$rc" -ne 0 ]; then dump_flatkv_layout; fi; exit $rc' EXIT

echo "Dumping FlatKV storage bucket from $flatkv_dir..."
build/paxdb dump-flatkv --db-dir "$flatkv_dir" --output-dir "$dump_dir" --bucket storage

if [ ! -s "$storage_dump" ]; then
  echo "FlatKV storage dump is missing or empty: $storage_dump" >&2
  exit 1
fi

if ! grep -q '^Key:' "$storage_dump"; then
  echo "FlatKV storage dump has no key/value rows: $storage_dump" >&2
  exit 1
fi

if [ ! -s "$contract_addr_file" ]; then
  echo "Missing FlatKV EVM fixture contract address: $contract_addr_file" >&2
  exit 1
fi

# The receipt supplies the slot's modification height, not the later RPC head.
python3 "$paxhome/tests/integration/contracts/flatkv_contract_checks.py" storage \
  "$storage_dump" "$paxhome/tests/integration/contracts"

echo "FlatKV storage bucket smoke verification passed: $storage_dump"
