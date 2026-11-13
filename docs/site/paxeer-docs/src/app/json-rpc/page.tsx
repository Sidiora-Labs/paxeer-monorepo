import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/json-rpc')

export default function JsonRpc() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">JSON-RPC API</h1>
        <p className="page-description">
          Paxeer's EVM JSON-RPC interface with standard Ethereum compatibility and Paxeer-specific enhancements.
        </p>
      </div>

      <p>For complete read-only examples, follow <Link href="/querying-evm">Query the EVM</Link> or the <Link href="/python">Python Quickstart</Link>. Use <Link href="/debugging-transactions">Debug Transactions</Link> to work from a failed receipt to a reproducible diagnosis.</p>

      <div className="source-note">
        <strong>Source:</strong> <code>rpc/server.go</code> (registration), <code>tx.go</code> (pending nonces), <code>state.go</code> (proofs), and <code>subscribe.go</code> (subscriptions) in the monorepo
      </div>

      <h2>Architecture Overview</h2>

      <p>
        Paxeer provides a comprehensive RPC interface that combines standard <a href="https://ethereum.org/en/developers/docs/apis/json-rpc/">Ethereum JSON-RPC</a> compatibility with Paxeer-specific enhancements. The API includes these namespaces:
      </p>

      <h3>Namespace Summary</h3>

      <table>
        <thead>
          <tr>
            <th>Namespace</th>
            <th>Transaction Visibility</th>
            <th>Use Case</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>eth_</code></td>
            <td>EVM transactions only</td>
            <td>Pure EVM applications, Ethereum tooling compatibility</td>
          </tr>
          <tr>
            <td><code>pax_</code></td>
            <td>EVM + Cosmos transactions (synthetic receipts)</td>
            <td>Full chain visibility, cross-chain event indexing</td>
          </tr>
          <tr>
            <td><code>pax2_</code></td>
            <td>EVM + Cosmos + bank transfers</td>
            <td>Complete transaction view including native transfers</td>
          </tr>
          <tr>
            <td><code>debug_</code></td>
            <td>EVM tracing and debugging</td>
            <td>Transaction replay, gas profiling, state inspection</td>
          </tr>
          <tr>
            <td><code>px_</code></td>
            <td>Sequencer receipt keys</td>
            <td><code>px_getReceiptKeys</code> for local receipt verification (HTTP only)</td>
          </tr>
          <tr>
            <td><code>echo_</code>, <code>net_</code>, <code>web3_</code></td>
            <td>Liveness, network identity, client version</td>
            <td>Standard utility namespaces</td>
          </tr>
          <tr>
            <td><code>txpool_</code></td>
            <td>Mempool inspection</td>
            <td>Pending/queued transaction views (HTTP only)</td>
          </tr>
        </tbody>
      </table>

      <p>
        Transport coverage differs. HTTP registers <code>echo</code>, <code>eth</code>, <code>pax</code>, <code>pax2</code>, <code>px</code>, <code>net</code>, <code>txpool</code>, <code>web3</code> and <code>debug</code>. WebSocket registers only <code>echo</code>, <code>net</code>, <code>web3</code> and <code>eth</code> — there is no <code>pax_</code>, <code>pax2_</code>, <code>debug_</code>, <code>txpool_</code> or <code>px_</code> on WebSocket connections.
      </p>

      <h2>eth_ Endpoints</h2>

      <p>
        The <code>eth_</code> prefixed endpoints provide a pure EVM-compatible view:
      </p>

      <ul>
        <li><strong>EVM-only:</strong> Only process and return EVM transactions</li>
        <li><strong>Ethereum tooling:</strong> Use ethers.js, viem, web3.js, Hardhat, and Foundry with the method-specific differences below</li>
        <li><strong>Transaction view:</strong> EVM transactions and their emitted logs</li>
        <li><strong>Method contracts:</strong> Standard request shapes with documented Paxeer differences</li>
      </ul>

      <h3>Key eth_ Methods</h3>

      <ul>
        <li><code>eth_blockNumber</code></li>
        <li><code>eth_getBlockByNumber</code>, <code>eth_getBlockByHash</code></li>
        <li><code>eth_getTransactionByHash</code></li>
        <li><code>eth_getTransactionReceipt</code></li>
        <li><code>eth_getLogs</code></li>
        <li><code>eth_call</code>, <code>eth_estimateGas</code></li>
        <li><code>eth_sendRawTransaction</code></li>
        <li><code>eth_getBalance</code>, <code>eth_getCode</code></li>
      </ul>

      <h2>pax_ Endpoints</h2>

      <p>
        The <code>pax_</code> prefixed endpoints provide an enhanced view that includes both EVM and relevant Cosmos transactions:
      </p>

      <ul>
        <li><strong>Synthetic transactions:</strong> Cosmos events (CW20, CW721) exposed as EVM logs</li>
        <li><strong>Full visibility:</strong> See both EVM and Cosmos activity</li>
        <li><strong>Cross-chain events:</strong> Index pointer contracts and token transfers</li>
        <li><strong>Trace filtering:</strong> Variants to exclude pre-state check failures</li>
      </ul>

      <h3>Synthetic Transaction Endpoints</h3>

      <p>
        These endpoints bridge Cosmos and EVM by exposing Cosmos-native events as EVM-compatible logs:
      </p>

      <ul>
        <li><code>pax_getLogs</code> — Enhanced <code>eth_getLogs</code> with synthetic logs</li>
        <li><code>pax_getFilterLogs</code> — Enhanced <code>eth_getFilterLogs</code> with synthetic logs</li>
        <li><code>pax_getBlockByNumber</code>, <code>pax_getBlockByHash</code> — Include synthetic transactions</li>
        <li><code>pax_getBlockReceipts</code> — Include receipts for synthetic transactions</li>
      </ul>

      <div className="source-note">
        <strong>Receipt lookup:</strong> Use <code>eth_getTransactionReceipt</code> with the synthetic transaction hash.
      </div>

      <h3>Trace Failure Filtering</h3>

      <p>
        A block can contain transactions that fail before EVM execution, including nonce or balance checks. Use the following endpoints when you need a view that excludes those failures:
      </p>

      <ul>
        <li><code>pax_traceBlockByNumberExcludeTraceFail</code></li>
        <li><code>pax_traceBlockByHashExcludeTraceFail</code></li>
        <li><code>pax_getTransactionReceiptExcludeTraceFail</code></li>
        <li><code>pax_getBlockByNumberExcludeTraceFail</code></li>
        <li><code>pax_getBlockByHashExcludeTraceFail</code></li>
      </ul>

      <h2>pax2_ Endpoints (Bank Transfers)</h2>

      <p>
        The <code>pax2_</code> namespace exposes the same block shape as <code>pax_</code> but includes <strong>bank transfers</strong> in block payloads:
      </p>

      <ul>
        <li>Seven methods: block, block receipts, transaction counts, <code>ExcludeTraceFail</code> variants</li>
        <li>No <code>pax2_</code> transaction or filter API</li>
        <li>HTTP only (not WebSocket)</li>
      </ul>

      <h3>Legacy API Gating</h3>

      <p>
        Both <code>pax_*</code> and <code>pax2_*</code> are <strong>legacy APIs</strong> gated by <code>[evm].enabled_legacy_pax_apis</code> in <code>app.toml</code>:
      </p>

      <ul>
        <li><strong>Deprecated:</strong> Scheduled for removal</li>
        <li><strong>Allow list:</strong> Only methods in the config array are enabled</li>
        <li><strong>Default config:</strong> Three <code>pax_*</code> address/Cosmos helpers pre-filled (<code>pax_getPaxAddress</code>, <code>pax_getEVMAddress</code>, <code>pax_getCosmosTx</code>)</li>
        <li><strong>Disabled response:</strong> JSON-RPC error <code>-32601</code>, message explains deprecation</li>
      </ul>

      <div className="source-note">
        <strong>Coverage:</strong> <code>rpc/pax_legacy_test.go</code>
      </div>

      <h2>Transaction Index Mismatches</h2>

      <p>
        <strong>Important:</strong> Transaction indices differ between <code>eth_</code> and <code>pax_</code> endpoints.
      </p>

      <h3>Example</h3>

      <p>
        Consider a block with:
      </p>

      <ol>
        <li>EVM Transaction 1</li>
        <li>Cosmos Transaction 1</li>
        <li>EVM Transaction 2</li>
      </ol>

      <h4>eth_getBlockReceipts</h4>

      <p>
        Returns only EVM transactions with sequential indices:
      </p>

      <ul>
        <li>EVM Transaction 1 (tx index: 0)</li>
        <li>EVM Transaction 2 (tx index: 1)</li>
      </ul>

      <h4>pax_getBlockReceipts</h4>

      <p>
        Returns all transactions (EVM + Cosmos) with sequential indices:
      </p>

      <ul>
        <li>EVM Transaction 1 (tx index: 0)</li>
        <li>Cosmos Transaction 1 (tx index: 1)</li>
        <li>EVM Transaction 2 (tx index: 2)</li>
      </ul>

      <h3>Receipts and Logs</h3>

      <ul>
        <li><strong>EVM-originating:</strong> Synthetic events included in both <code>eth_getLogs</code> and <code>eth_getTransactionReceipt</code></li>
        <li><strong>Cosmos-originating:</strong> Synthetic events <em>not</em> in <code>eth_</code> methods; use <code>pax_getLogs</code> and <code>pax_getBlockReceipts</code></li>
        <li><strong>logIndex values:</strong> Strictly increasing and consistent within each namespace</li>
      </ul>

      <h3>Best Practice</h3>

      <ul>
        <li>Use the same endpoint consistently within your application</li>
        <li>Account for index differences when switching endpoints</li>
        <li>Prefer transaction hashes over indices (hashes are consistent across endpoints)</li>
      </ul>

      <h2>debug_ Endpoints</h2>

      <p>
        Use <code>debug_trace*</code> methods to inspect historical execution:
      </p>

      <ul>
        <li>Inspect failed call frames and returned error data</li>
        <li>Compare gas consumption across calls and opcodes</li>
        <li>Review the state accesses and changes exposed by the selected tracer</li>
      </ul>

      <h3>Key debug_ Methods</h3>

      <ul>
        <li><code>debug_traceBlockByNumber</code>, <code>debug_traceBlockByHash</code></li>
        <li><code>debug_traceTransaction</code></li>
        <li><code>debug_traceCall</code></li>
      </ul>

      <h2>Paxeer RPC Distinctions</h2>

      <p>
        Paxeer's RPC deviates from Ethereum in several areas:
      </p>

      <h3>Block Tags and Pending Nonces</h3>

      <ul>
        <li>Committed-state reads resolve supported latest, safe, and finalized tags through the RPC publication watermarks</li>
        <li>Pending account nonces have a separate path: <code>eth_getTransactionCount(address, "pending")</code> uses the mempool’s next nonce or a configured proxy</li>
        <li>Track submitted hashes and use the pending nonce when coordinating multiple transactions from one account</li>
      </ul>

      <h3>No Uncle Blocks</h3>

      <ul>
        <li>Paxeer finalizes blocks through BFT consensus and does not produce Ethereum uncle blocks</li>
        <li>The uncle endpoints are registered on HTTP and WebSocket and answer with the empty result: <code>eth_getUncleByBlockNumberAndIndex</code> and <code>eth_getUncleByBlockHashAndIndex</code> return <code>null</code>, and <code>eth_getUncleCountByBlockNumber</code> / <code>eth_getUncleCountByBlockHash</code> return <code>0x0</code> for known blocks</li>
      </ul>

      <h3>Storage Proof Format</h3>

      <ul>
        <li><code>eth_getProof</code> is implemented for proof-capable stores</li>
        <li>The result contains <code>address</code>, <code>hexValues</code>, and IAVL <code>storageProof</code> operations; it is not Ethereum’s Merkle Patricia account-proof shape</li>
        <li>Use an IAVL proof verifier and an endpoint retaining proof-capable state at the requested height</li>
      </ul>

      <h3>No Proof-of-Work</h3>

      <ul>
        <li><code>eth_mining</code> and <code>eth_hashrate</code> are not registered on any API, so they return method-not-found (<code>-32601</code>) rather than the explicit <code>-32000</code> unsupported error</li>
        <li>Paxeer uses BFT consensus, not PoW</li>
      </ul>

      <h3>No Blobs</h3>

      <ul>
        <li>EIP-4844 blob transactions not supported</li>
        <li><code>eth_blobBaseFee</code> returns error <code>-32000</code>: "blobs not supported on this chain"</li>
      </ul>

      <p>
        See <Link href="/json-rpc-unsupported">Unsupported Methods</Link> for the complete list.
      </p>

      <h2>Consistent Historical Reads</h2>

      <p>
        Pin related reads to one block number and retain its block hash with indexed data. Use a provider with the historical state your application needs, and keep the same namespace when reading blocks, receipts, and logs.
      </p>

      <h2>WebSocket Support</h2>

      <p>
        Paxeer RPC supports WebSocket for:
      </p>

      <ul>
        <li><code>eth_subscribe</code> for <code>newHeads</code> and <code>logs</code></li>
        <li>Pending-transaction subscriptions are not implemented by <code>SubscriptionAPI</code></li>
        <li><code>eth_unsubscribe</code></li>
      </ul>

      <p>
        Legacy <code>pax2_*</code> endpoints are HTTP-only, as are <code>pax_</code>, <code>debug_</code>, <code>txpool_</code> and <code>px_</code>. WebSocket serves the <code>echo</code>, <code>net</code>, <code>web3</code> and <code>eth</code> namespaces.
      </p>

      <div className="prev-next">
        <Link href="/wasm-runtime">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">WASM Runtime</div>
        </Link>
        <Link href="/json-rpc-unsupported">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">Unsupported Methods</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
