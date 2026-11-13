import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/json-rpc-unsupported')

export default function JsonRpcUnsupported() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Unsupported JSON-RPC Methods</h1>
        <p className="page-description">
          Ethereum JSON-RPC methods that are explicitly unsupported on Paxeer EVM.
        </p>
      </div>

      <div className="source-note">
        <strong>Source:</strong> <code>rpc/info.go</code>, <code>rpc/filter.go</code>, <code>rpc/tracers.go</code>, <code>rpc/state.go</code> and <code>rpc/tx.go</code> in the Paxeer node source.
      </div>

      <h2>Overview</h2>

      <p>
        Some Ethereum JSON-RPC methods are <strong>registered</strong> on Paxeer's EVM endpoint but return a <strong>JSON-RPC error</strong> instead of a result. This gives clients and tools a stable, documented failure (code <code>-32000</code>) rather than "method not found" (<code>-32601</code>).
      </p>

      <h2>Explicitly Unsupported Methods</h2>

      <table>
        <thead>
          <tr>
            <th>Method</th>
            <th>Typical Error Message</th>
            <th>Reason</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>eth_blobBaseFee</code></td>
            <td>"blobs not supported on this chain"</td>
            <td>EIP-4844 blob transactions not supported</td>
          </tr>
          <tr>
            <td><code>eth_newPendingTransactionFilter</code></td>
            <td>"eth_newPendingTransactionFilter is not supported on Pax EVM RPC"</td>
            <td>Only on nodes without a receipt-log feed; sequencer and standby nodes serve it (see below)</td>
          </tr>
          <tr>
            <td><code>debug_getRawBlock</code></td>
            <td>"debug_getRawBlock is not supported on Pax EVM RPC"</td>
            <td>Raw RLP block payloads not served</td>
          </tr>
          <tr>
            <td><code>debug_getRawHeader</code></td>
            <td>"debug_getRawHeader is not supported on Pax EVM RPC"</td>
            <td>Raw RLP header payloads not served</td>
          </tr>
          <tr>
            <td><code>debug_getRawReceipts</code></td>
            <td>"debug_getRawReceipts is not supported on Pax EVM RPC"</td>
            <td>Raw RLP receipt payloads not served</td>
          </tr>
        </tbody>
      </table>

      <h3>Synchronization and raw transactions</h3>

      <p>
        <code>eth_syncing</code> returns <code>false</code> when consensus reports the node caught up, and otherwise a progress object with <code>startingBlock</code>, <code>currentBlock</code> and <code>highestBlock</code>. <code>debug_getRawTransaction</code> returns raw transaction bytes by hash, or a <em>transaction not found</em> error. The raw header, block and receipt-list methods in the table return explicit unsupported errors.
      </p>

      <h2>Error Response Format</h2>

      <p>
        Unsupported methods return a standard JSON-RPC error:
      </p>

      <pre><code>{`{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32000,
    "message": "blobs not supported on this chain"
  }
}`}</code></pre>

      <h2>Behavior Notes</h2>

      <h3>eth_syncing</h3>

      <p>
        <code>eth_syncing</code> is implemented from the consensus status: <code>false</code> when the node is caught up, otherwise <code>startingBlock</code> (earliest held block), <code>currentBlock</code> and <code>highestBlock</code>.
      </p>

      <h3>eth_newPendingTransactionFilter</h3>

      <p>
        On sequencer and standby nodes, which hold the receipt-log feed, this filter is served: each poll returns the hashes of transactions receipted since the last poll, in sequence order. Full-transaction mode (<code>fullTx: true</code>) is refused — the filter serves transaction hashes only. On every other node the call returns the <code>-32000</code> unsupported error. Independently, the node tracks pending nonces, and <code>eth_getTransactionCount(address, "pending")</code> uses the local pending nonce or forwards the request to the appropriate EVM proxy.
      </p>

      <h3>debug_getRaw* Methods</h3>

      <p>
        Raw RLP-encoded headers, blocks and receipt lists are not served on this RPC surface; use the standard <code>eth_*</code> endpoints, which return structured JSON. Raw transaction bytes are the exception: <code>debug_getRawTransaction</code> is implemented and returns them by hash.
      </p>

      <h3>eth_blobBaseFee</h3>

      <p>
        The current implementation rejects EIP-4844 blob transactions in transaction admission. The blob fee RPC returns the explicit unsupported error shown above.
      </p>

      <h2>Broader Compatibility Notes</h2>

      <p>
        Beyond explicitly unsupported methods, Paxeer's RPC diverges from Ethereum in several conceptual areas:
      </p>

      <h3>Block Tags and Pending Nonces</h3>

      <ul>
        <li>Committed reads resolve <code>latest</code>, <code>safe</code> and <code>finalized</code> through publication watermarks. Supported <code>pending</code> reads use the open execution layer when configured; <code>eth_getProof</code> refuses pending state.</li>
        <li>Pending nonce queries have a separate path. Do not assume <code>pending</code> and <code>latest</code> return the same nonce.</li>
        <li>A BFT commit provides finality under the consensus protocol’s assumptions. RPC freshness and finality are separate properties.</li>
      </ul>

      <h3>Storage Proofs Use a Different Format</h3>

      <ul>
        <li><code>eth_getProof</code> is implemented by <code>StateAPI.GetProof</code>.</li>
        <li>Its result contains <code>address</code>, <code>hexValues</code> and <code>storageProof</code>. Storage proofs contain ICS23 proof operations through the application root rather than Ethereum Merkle Patricia trie proofs.</li>
        <li>The response excludes Ethereum per-account proof fields because the store has no Ethereum per-account root. Ethereum proof verifiers cannot consume this response unchanged.</li>
        <li>The requested height must be readable and the configured store must support proof queries. Historical retention and endpoint availability still apply.</li>
      </ul>

      <h3>No Proof-of-Work</h3>

      <ul>
        <li><code>eth_mining</code> and <code>eth_hashrate</code> have no implementation on any registered API, so they return method-not-found (<code>-32601</code>), not the registered <code>-32000</code> unsupported error. Do not branch on <code>-32000</code> for them.</li>
        <li>Paxeer uses BFT consensus, not PoW</li>
      </ul>

      <p>
        See <Link href="/json-rpc">JSON-RPC API</Link> for full compatibility details.
      </p>

      <h2>Implementation references</h2>

      <p>
        The node sources and contract checks are in this repository:
      </p>

      <pre><code>{`rpc/info_test.go
rpc/state_test.go
rpc/tracers_test.go
rpc/filter_test.go`}</code></pre>

      <p>
        For provider-specific method support, check the endpoint configuration and inspect the JSON-RPC result or error returned by each request.
      </p>

      <div className="prev-next">
        <Link href="/json-rpc">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">JSON-RPC API</div>
        </Link>
        <Link href="/rest-grpc">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">REST & gRPC</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
