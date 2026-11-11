#!/bin/sh
set -eu

export_paxeer_chain_environment() {
    paxeer_chain_environment=$(python3 - <<'PY'
import json
import os
from pathlib import Path
from urllib.parse import urlsplit

manifest_path = os.environ.get("PAXEER_X_RUNTIME_MANIFEST")
if manifest_path:
    manifest = json.loads(Path(manifest_path).read_text(encoding="utf-8"))
    chain_id = manifest["chain_id"]
    if "ports" in manifest:
        rpc_port = manifest["ports"][0]
    else:
        rpc_port = urlsplit(manifest["chain_rpc"]).port
else:
    chain_id = 125
    rpc_port = os.environ.get("LAYERX_TEST_PAXEER_RPC_PORT", "8545")


def bounded_integer(value, maximum, name):
    text = str(value)
    if not text.isascii() or not text.isdecimal() or not 0 < int(text) <= maximum:
        raise ValueError(f"invalid {name}: {value!r}")
    return int(text)


chain_id = bounded_integer(chain_id, 2**64 - 1, "Paxeer chain id")
rpc_port = bounded_integer(rpc_port, 65535, "Paxeer RPC port")
print(chain_id, rpc_port)
PY
    )
    export LAYERX_TEST_PAXEER_CHAIN_ID=${paxeer_chain_environment% *}
    export LAYERX_TEST_PAXEER_RPC_PORT=${paxeer_chain_environment#* }
}

case "${1-}" in
    test) test_target=agent-test ;;
    sanitizers) test_target=agent-test-sanitize ;;
    prepare) test_target= ;;
    *) echo "expected test, sanitizers or prepare" >&2; exit 2 ;;
esac
if [ "$#" -ne 1 ] || [ "$(id -u)" -ne 0 ]; then
    echo "real-node tests require root to launch the daemon under its distinct uid" >&2
    exit 2
fi

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$repo_root"
ownership_helper=$repo_root/scripts/ci/restore-build-ownership.py
checkout_owner=$(python3 "$ownership_helper" --repo "$repo_root" --capture-owner)
native_build_output=${LAYERX_TEST_NATIVE_BUILD_DIR:-${BUILD_DIR:-$repo_root/build}}
cargo_build_output=${CARGO_TARGET_DIR:-$repo_root/.lane-target}
python3 "$ownership_helper" --repo "$repo_root" --owner "$checkout_owner" \
    --output "$native_build_output" --output "$cargo_build_output" \
    --output "$repo_root/build" --validate-only
restore_build_ownership() {
    primary_exit=$?
    trap - 0 HUP INT TERM
    set +e
    python3 "$ownership_helper" --repo "$repo_root" --owner "$checkout_owner" \
        --output "$native_build_output" --output "$cargo_build_output" \
        --output "$repo_root/build"
    ownership_exit=$?
    if [ "$primary_exit" -ne 0 ]; then
        exit "$primary_exit"
    fi
    exit "$ownership_exit"
}
trap restore_build_ownership 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
export CARGO_BUILD_JOBS=4
export TMPDIR=/tmp
for executable in forge cast anvil go; do
    command -v "$executable" >/dev/null
done
python3 -c "from Crypto.Hash import keccak; from eth_account import Account; from eth_utils import to_checksum_address"
export_paxeer_chain_environment
if [ -n "$test_target" ]; then
    make "$test_target"
    exit 0
fi

run_real_node_tests() {
    : "${CARGO_TARGET_DIR:?the build output directory of the agent-test path, exported by the Makefile variable AGENT_TEST_TARGET_DIR}"
    export LAYERX_TEST_NATIVE_BUILD_DIR=${LAYERX_TEST_NATIVE_BUILD_DIR:-$repo_root/build}
    GOMAXPROCS=4 GOFLAGS="${GOFLAGS:-} -p=4" make paxeer-build

    export LAYERX_TEST_NATIVE_BIN_DIR=$LAYERX_TEST_NATIVE_BUILD_DIR/bin
    export PAXD=$repo_root/build/paxd
    sha256sum "$PAXD"
}

run_real_node_tests
