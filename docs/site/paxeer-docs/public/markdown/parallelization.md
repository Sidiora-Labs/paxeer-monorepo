<!-- Source: https://docs.paxeer.app/parallelization/ -->

# Design for Parallel Execution

Reduce shared-state contention in Paxeer contracts, preserve on-chain invariants, and measure gas separately from throughput.

Paxeer supports optimistic concurrency control (OCC): independent work proceeds concurrently, while conflicting access is detected and resolved in a deterministic order. In `node/app.go`, tenant-domain transactions use synchronous V2 processing. Other transactions use Giga OCC when `giga_executor.enabled` and `giga_executor.occ_enabled` are both `true` and synchronous processing is not selected. Both flags default to `true`. With Giga enabled, the remaining path is synchronous Giga execution. With Giga disabled, the context OCC setting and synchronous-processing decision select V2 OCC (`ProcessTXsWithOCCV2`) or synchronous V2 execution. Contract storage layout influences how much work is independent in any of these paths. See the [execution engine](https://docs.paxeer.app/engine) and [SDK integration](https://docs.paxeer.app/sdk).

## Find the shared writes

Start by listing the state each entry point reads and writes, including state touched by external calls. Two different users can still conflict if both transactions update one total, one queue head, or the same pool.

| Pattern | Contention source | Design option |
| --- | --- | --- |
| Global counter on every update | All writers touch the same slot | Use per-user state when the aggregate is only needed for display |
| One order book for all markets | Unrelated markets share mutable state | Partition by market where business rules allow |
| Large push distribution | One transaction writes many recipient balances | Use bounded batches or independent claims |
| Shared token or pool call | External state reintroduces a common dependency | Include the external call’s storage accesses in your design |

## Partition state by the unit of work

The following illustrative Solidity contract partitions quantities by caller and position ID, giving each pair its own storage entry.

```
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract PositionNotes {
    mapping(address => mapping(uint256 => int256)) public quantities;
    event QuantityChanged(address indexed owner, uint256 indexed id, int256 quantity);

    function setQuantity(uint256 id, int256 quantity) external {
        quantities[msg.sender][id] = quantity;
        emit QuantityChanged(msg.sender, id, quantity);
    }
}
```

Updates for different caller/ID pairs reach distinct mapping entries. Updates to the same entry still depend on one another. Calls from one sender also share nonce and balance state, so include sender distribution when measuring concurrency.

## Keep invariants authoritative

Keep supply caps, collateral requirements, and conservation checks in on-chain state. These shared dependencies preserve the contract’s accounting rules.

When an aggregate is used only for a dashboard, events can feed an indexer instead of requiring every transaction to update the same display counter. Preserve an explicit replay range and durable event cursor as described in [Query the EVM](https://docs.paxeer.app/querying-evm).

## Bound work and inspect packing

- Bound arrays and loops that touch persistent storage; measure the largest permitted input.

- Calculate temporary values in memory and write only the final required state.

- Consider pull-based claims when recipients can act independently, while preserving authorization, funding, and reentrancy protections.

- Inspect packed fields before combining them: two logical values in one storage slot can introduce a shared write.

- Treat an external call as part of the access set, including shared token balances, allowances, and precompile-backed state.

## Measure the actual workload

Use two workloads with the same contract and transaction shape: one distributes work across independent keys, and the other concentrates it on the same keys. Keep the binary revision, hardware, sender distribution, and submission rate fixed.

1. Measure single-call gas with Foundry or Hardhat before changing storage layout.
2. Run the workload against a Paxeer development cluster with the intended executor configuration.
3. Record submitted, included, successful, and failed transactions separately.
4. Compare throughput and latency percentiles at the same load, and inspect available conflict/re-execution diagnostics.
5. Check the resulting state and invariants after every run, including retries and contention cases.

```
forge test --gas-report
forge snapshot
```

Track gas per call and network throughput separately. Fewer conflicts can improve scheduling while the gas charged per transaction stays the same. Use the development-cluster workload to compare throughput and latency.

## Review before adoption

Keep a storage-access sketch beside benchmark results: which keys are independent, which invariants require a shared dependency, and which external calls can serialize work. Compare each change against the same workload and check that the resulting state remains correct.

Continue with [transaction diagnostics](https://docs.paxeer.app/debugging-transactions) to inspect execution failures and [storage architecture](https://docs.paxeer.app/storage) for the underlying state engines.

**Paxeer source:** `engine/executor/config/config.go` (OCCEnabled), node/app.go (ExecuteTxsConcurrently and ProcessTXsWithOCCGiga), and modules/evm/types/params.go (DefaultPaxSstoreSetGasEIP2200).
