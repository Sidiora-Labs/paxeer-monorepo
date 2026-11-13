import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import Link from 'next/link'

export const metadata = metadataFor('/debugging-transactions')

export default function Guide() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Debug Transactions</h1>
        <p className="page-description">Diagnose reverted or missing Paxeer transactions with receipts, calldata, gas checks, traces, and reproducible local tests.</p>
      </div>
      <p>Begin with the transaction hash, chain ID, and the endpoint that accepted the submission. Separate transport failures, rejected submissions, and transactions that executed with a reverted receipt; they require different next steps.</p>
      <DocArt name="debugging" />
      <h2>Collect the transaction and receipt</h2>
      <p>Set <code>PAXEER_RPC_URL</code> and <code>TX_HASH</code>, then use Cast to inspect the recorded transaction. These commands only query the endpoint.</p>
      <pre><code className="language-bash">{"cast chain-id --rpc-url \"$PAXEER_RPC_URL\"\ncast tx \"$TX_HASH\" --rpc-url \"$PAXEER_RPC_URL\" --json\ncast receipt \"$TX_HASH\" --rpc-url \"$PAXEER_RPC_URL\" --json"}</code></pre>
      <table><thead><tr><th>Observation</th><th>Meaning</th><th>Next check</th></tr></thead><tbody><tr><td>No transaction or receipt</td><td>The endpoint cannot find the hash</td><td>Confirm chain 125, the submission response, and whether the provider has indexed that height</td></tr><tr><td>Transaction present, receipt absent</td><td>Execution is not yet visible through this endpoint</td><td>Check a known-good endpoint and the sender nonce before resubmitting</td></tr><tr><td>Receipt status 0</td><td>The transaction was included but execution reverted</td><td>Inspect the ABI, revert data, and execution trace</td></tr><tr><td>Receipt status 1</td><td>Execution succeeded</td><td>Compare emitted events and resulting application state with the intended action</td></tr></tbody></table>
      <p>A timeout after submission does not prove the transaction failed. Search for the original hash first; a new transaction can repeat the application action.</p>
      <h2>Check the sender and call</h2>
      <p>Read the sender’s native balance, nonce, and the current fee estimate. Set <code>SENDER_ADDRESS</code> to the sender recorded in the transaction.</p>
      <pre><code className="language-bash">{"cast balance \"$SENDER_ADDRESS\" --rpc-url \"$PAXEER_RPC_URL\"\ncast nonce \"$SENDER_ADDRESS\" --rpc-url \"$PAXEER_RPC_URL\"\ncast gas-price --rpc-url \"$PAXEER_RPC_URL\"\ncast code \"$CONTRACT_ADDRESS\" --rpc-url \"$PAXEER_RPC_URL\"\ncast calldata 'setNumber(uint256)' 42"}</code></pre>
      <p>Compare the encoded calldata to the transaction input. Confirm the contract address and ABI against the deployed version. PAX pays gas even when execution reverts. Increasing the gas limit will not repair a failed authorization check or invalid function argument.</p>
      <h2>Simulate the exact inputs</h2>
      <p>For the counter example, reproduce a call with the original sender and arguments. <code>cast call</code> uses <code>eth_call</code>; it does not persist storage changes.</p>
      <pre><code className="language-bash">{"cast call \"$CONTRACT_ADDRESS\" 'setNumber(uint256)' 42   --from \"$SENDER_ADDRESS\" --rpc-url \"$PAXEER_RPC_URL\""}</code></pre>
      <p>A call at <code>latest</code> tests current state. It may differ from the state at the time of the failed transaction. A call at the previous block is also only an approximation when earlier transactions in the same block changed relevant state.</p>
      <h2>Trace historical execution</h2>
      <p>If your provider enables the debug namespace and retains the required state, request a call trace. Inspect the failed frame, its input, revert data, and nested calls.</p>
      <pre><code className="language-bash">{"cast rpc debug_traceTransaction \"$TX_HASH\"   '{\"tracer\":\"callTracer\",\"timeout\":\"30s\"}'   --rpc-url \"$PAXEER_RPC_URL\""}</code></pre>
      <p>Provider policies and node versions determine available tracers. The node accepts <code>callTracer</code>, <code>prestateTracer</code>, and <code>flatCallTracer</code>. For a <code>-32601</code> response, use an endpoint with the debug namespace enabled. For a historical-state error, use a node retaining the requested height. See <Link href="/json-rpc">Paxeer debug methods</Link>.</p>
      <h2>Reproduce in a local test</h2>
      <p>Write a minimal test using the exact contract version, sender, and arguments. For ordinary Solidity state, a fork pinned to a known block can help:</p>
      <pre><code className="language-bash">{"anvil --fork-url \"$PAXEER_RPC_URL\" --fork-block-number \"$BLOCK_NUMBER\"\n\n# In another terminal, read the local fork.\ncast call \"$CONTRACT_ADDRESS\" 'number()(uint256)'   --rpc-url http://127.0.0.1:8545"}</code></pre>
      <p>Use Anvil for ordinary Solidity contract state. For Cosmos modules, Paxeer precompiles, parallel scheduling, or consensus behavior, reproduce the issue with the matching Paxeer binary in a <Link href="/docker">development cluster</Link>. The source RPC must retain the historical state needed by your fork.</p>
      <h2>Keep a useful bug report</h2>
      <ul><li>Transaction hash, chain ID, block number, and receipt status.</li><li>Contract address, exact source revision, ABI, compiler version, and optimizer settings.</li><li>Original sender, function arguments, value, gas parameters, and expected state transition.</li><li>Decoded error or failing trace frame and a minimal reproduction.</li><li>Provider type and node version when available; remove authentication tokens from URLs and logs.</li></ul>
      <p>Reference: <a href="https://getfoundry.sh/cast/overview/">Foundry Cast</a>. Continue with <Link href="/parallelization">parallel execution design</Link> when the issue is contention or throughput rather than a revert.</p>
      <div className="source-note"><strong>Paxeer source:</strong> <code>Paxeer-X-Network/</code> — rpc/tracers.go (DebugAPI), rpc/tx.go (TransactionAPI), and rpc/simulate.go (SimulationAPI).</div>
      <div className="prev-next">
        <Link href="/python"><div className="prev-next-label">Previous</div><div className="prev-next-title">Python Quickstart</div></Link>
        <Link href="/parallelization"><div className="prev-next-label">Next</div><div className="prev-next-title">Design for Parallel Execution</div></Link>
      </div>
    </DocsLayout>
  )
}
