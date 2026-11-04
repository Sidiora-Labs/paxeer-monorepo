#!/bin/bash
#
# GIGA Mixed-Mode EVM Integration Tests
#
# This script replaces the default cluster with a mixed-mode cluster where:
#   - Node 0: GIGA_EXECUTOR=true GIGA_OCC=true (concurrent giga executor)
#   - Nodes 1-3: Standard V2 executor
#
# If giga produces different results from V2, the giga node will halt with
# a consensus failure (AppHash or LastResultsHash mismatch).
#
# The GitHub workflow starts a default cluster before running matrix scripts.
# This wrapper tears it down and starts a mixed-mode cluster instead.
#

set -euo pipefail

mixed_failure_diagnostics() {
    local status=$?
    trap - EXIT
    if [ "$status" -ne 0 ]; then
        echo "Mixed-mode test failed (exit $status); validator diagnostics follow" >&2
        for node in 0 1 2 3; do
            echo "=== pax-node-${node}: build/generated/logs/paxd-${node}.log ===" >&2
            timeout 10s docker logs --tail 200 "pax-node-${node}" >&2 || true
            timeout 10s docker exec "pax-node-${node}" curl --fail --silent --show-error \
                --max-time 5 http://localhost:26657/status >&2 || true
        done
    fi
    exit "$status"
}
trap mixed_failure_diagnostics EXIT

validator_rpc() {
    timeout 10s docker exec "pax-node-$1" curl --fail --silent --show-error \
        --max-time 5 "http://localhost:26657/$2"
}

validator_height() {
    validator_rpc "$1" status | jq -er '
        select(.error == null and .result.sync_info.catching_up == false)
        | .result.sync_info.latest_block_height
        | select(type == "string" and test("^[1-9][0-9]*$"))'
}

check_mixed_final_agreement() {
    local node height target=0 deadline reference= hashes
    local -a baseline=()
    for node in 0 1 2 3; do
        height=$(validator_height "$node")
        baseline[node]=$height
        if [ "$height" -gt "$target" ]; then target=$height; fi
    done
    # A header's AppHash is the state after the preceding block. Choose a
    # common header beyond every post-workload baseline, including that lag.
    target=$((target + 2))
    deadline=$((SECONDS + 120))
    for node in 0 1 2 3; do
        while true; do
            height=$(validator_height "$node")
            if [ "$height" -ge "$target" ] && [ "$height" -gt "${baseline[node]}" ]; then
                break
            fi
            if [ "$SECONDS" -ge "$deadline" ]; then
                echo "ERROR: validator $node did not advance to $target after workload (height $height)" >&2
                return 1
            fi
            sleep 2
        done
        hashes=$(validator_rpc "$node" "block?height=$target" | jq -er --arg height "$target" '
            select(.error == null and .result.block.header.height == $height)
            | .result.block.header
            | select((.app_hash | type == "string" and test("^[A-Fa-f0-9]{64}$"))
                and (.last_results_hash | type == "string" and test("^[A-Fa-f0-9]{64}$")))
            | [.app_hash, .last_results_hash] | @tsv')
        if [ "$node" -eq 0 ]; then
            reference=$hashes
        elif [ "$hashes" != "$reference" ]; then
            echo "ERROR: validator $node disagrees with validator 0 at committed height $target: $hashes != $reference" >&2
            return 1
        fi
        echo "Validator $node advanced from ${baseline[node]} to $height; height $target hashes: $hashes"
    done
}

echo "=== GIGA Mixed-Mode Integration Test ==="
echo "=== Node 0: GIGA_EXECUTOR=true GIGA_OCC=true, Nodes 1-3: standard V2 ==="

# Stop the default cluster that the workflow started
echo "Stopping default cluster..."
make docker-cluster-stop || true

# Start mixed-mode cluster (node 0 = giga, nodes 1-3 = V2)
# build-docker-node is a no-op since the image was already built
echo "Starting mixed-mode cluster..."
DOCKER_DETACH=true make docker-cluster-start-giga-mixed

# Wait for all 4 nodes to be ready
echo "Waiting for mixed cluster to be ready..."
timeout=300
elapsed=0
while [ $elapsed -lt $timeout ]; do
    if [ -f "build/generated/launch.complete" ] && [ $(cat build/generated/launch.complete | wc -l) -ge 4 ]; then
        echo "All 4 nodes are ready (took ${elapsed}s)"
        break
    fi
    sleep 5
    elapsed=$((elapsed + 5))
    echo "  Waiting... (${elapsed}s elapsed)"
done
if [ $elapsed -ge $timeout ]; then
    echo "ERROR: Mixed cluster failed to start within ${timeout}s"
    exit 1
fi

echo "Waiting 10s for nodes to stabilize..."
sleep 10

# Run the same giga EVM tests — they hit node 0 (giga) via paxlocal RPC
echo "=== Running GIGA EVM Tests against mixed cluster ==="
./tests/integration/evm_module/scripts/evm_giga_tests.sh
check_mixed_final_agreement

echo "=== All four mixed-mode validators advanced and agree ==="
