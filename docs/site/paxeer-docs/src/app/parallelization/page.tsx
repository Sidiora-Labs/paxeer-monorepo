import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/parallelization')

export default function Guide() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Design for Parallel Execution</h1>
        <p className="page-description">Reduce shared-state contention in Paxeer contracts, preserve on-chain invariants, and measure gas separately from throughput.</p>
      </div>
      <p>Paxeer supports optimistic concurrency control (OCC): independent work proceeds concurrently, while conflicting access is detected and resolved in a deterministic order. In <code>node/app.go</code>, tenant-domain transactions use synchronous V2 processing. Other transactions use Giga OCC when <code>giga_executor.enabled</code> and <code>giga_executor.occ_enabled</code> are both <code>true</code> and synchronous processing is not selected. Both flags default to <code>true</code>. With Giga enabled, the remaining path is synchronous Giga execution. With Giga disabled, the context OCC setting and synchronous-processing decision select V2 OCC (<code>ProcessTXsWithOCCV2</code>) or synchronous V2 execution. Contract storage layout influences how much work is independent in any of these paths. See the <Link href="/engine">execution engine</Link> and <Link href="/sdk">SDK integration</Link>.</p>
      <h2>Find the shared writes</h2>
      <p>Start by listing the state each entry point reads and writes, including state touched by external calls. Two different users can still conflict if both transactions update one total, one queue head, or the same pool.</p>
      <table><thead><tr><th>Pattern</th><th>Contention source</th><th>Design option</th></tr></thead><tbody><tr><td>Global counter on every update</td><td>All writers touch the same slot</td><td>Use per-user state when the aggregate is only needed for display</td></tr><tr><td>One order book for all markets</td><td>Unrelated markets share mutable state</td><td>Partition by market where business rules allow</td></tr><tr><td>Large push distribution</td><td>One transaction writes many recipient balances</td><td>Use bounded batches or independent claims</td></tr><tr><td>Shared token or pool call</td><td>External state reintroduces a common dependency</td><td>Include the external call’s storage accesses in your design</td></tr></tbody></table>
      <h2>Partition state by the unit of work</h2>
      <p>The following illustrative Solidity contract partitions quantities by caller and position ID, giving each pair its own storage entry.</p>
      <pre><code className="language-solidity">{"// SPDX-License-Identifier: MIT\npragma solidity ^0.8.0;\n\ncontract PositionNotes {\n    mapping(address => mapping(uint256 => int256)) public quantities;\n    event QuantityChanged(address indexed owner, uint256 indexed id, int256 quantity);\n\n    function setQuantity(uint256 id, int256 quantity) external {\n        quantities[msg.sender][id] = quantity;\n        emit QuantityChanged(msg.sender, id, quantity);\n    }\n}"}</code></pre>
      <p>Updates for different caller/ID pairs reach distinct mapping entries. Updates to the same entry still depend on one another. Calls from one sender also share nonce and balance state, so include sender distribution when measuring concurrency.</p>
      <h2>Keep invariants authoritative</h2>
      <p>Keep supply caps, collateral requirements, and conservation checks in on-chain state. These shared dependencies preserve the contract’s accounting rules.</p>
      <p>When an aggregate is used only for a dashboard, events can feed an indexer instead of requiring every transaction to update the same display counter. Preserve an explicit replay range and durable event cursor as described in <Link href="/querying-evm">Query the EVM</Link>.</p>
      <h2>Bound work and inspect packing</h2>
      <ul><li>Bound arrays and loops that touch persistent storage; measure the largest permitted input.</li><li>Calculate temporary values in memory and write only the final required state.</li><li>Consider pull-based claims when recipients can act independently, while preserving authorization, funding, and reentrancy protections.</li><li>Inspect packed fields before combining them: two logical values in one storage slot can introduce a shared write.</li><li>Treat an external call as part of the access set, including shared token balances, allowances, and precompile-backed state.</li></ul>
      <h2>Measure the actual workload</h2>
      <p>Use two workloads with the same contract and transaction shape: one distributes work across independent keys, and the other concentrates it on the same keys. Keep the binary revision, hardware, sender distribution, and submission rate fixed.</p>
      <ol><li>Measure single-call gas with Foundry or Hardhat before changing storage layout.</li><li>Run the workload against a Paxeer development cluster with the intended executor configuration.</li><li>Record submitted, included, successful, and failed transactions separately.</li><li>Compare throughput and latency percentiles at the same load, and inspect available conflict/re-execution diagnostics.</li><li>Check the resulting state and invariants after every run, including retries and contention cases.</li></ol>
      <pre><code className="language-bash">{"forge test --gas-report\nforge snapshot"}</code></pre>
      <p>Track gas per call and network throughput separately. Fewer conflicts can improve scheduling while the gas charged per transaction stays the same. Use the development-cluster workload to compare throughput and latency.</p>
      <h2>Review before adoption</h2>
      <p>Keep a storage-access sketch beside benchmark results: which keys are independent, which invariants require a shared dependency, and which external calls can serialize work. Compare each change against the same workload and check that the resulting state remains correct.</p>
      <p>Continue with <Link href="/debugging-transactions">transaction diagnostics</Link> to inspect execution failures and <Link href="/storage">storage architecture</Link> for the underlying state engines.</p>
      <div className="source-note"><strong>Paxeer source:</strong> <code>engine/executor/config/config.go</code> (OCCEnabled), node/app.go (ExecuteTxsConcurrently and ProcessTXsWithOCCGiga), and modules/evm/types/params.go (DefaultPaxSstoreSetGasEIP2200).</div>
      <div className="prev-next">
        <Link href="/debugging-transactions"><div className="prev-next-label">Previous</div><div className="prev-next-title">Debug Transactions</div></Link>
        <Link href="/node-troubleshooting"><div className="prev-next-label">Next</div><div className="prev-next-title">Troubleshoot a Node</div></Link>
      </div>
    </DocsLayout>
  )
}
