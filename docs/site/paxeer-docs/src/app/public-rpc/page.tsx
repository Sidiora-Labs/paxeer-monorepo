import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/public-rpc')

export default function PublicRpc() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Public RPC endpoints</h1><p className="page-description">Connect wallets and applications to Paxeer mainnet-beta, EVM chain ID 125, using node JSON-RPC at the endpoint root.</p></div>
    <h2>Connect to the node</h2>
    <p>Paxeer is one BFT network. The kernel runs inside <code>paxd</code>, and the proposer executes transactions and proposes their block. The node exposes Ethereum JSON-RPC for contracts, balances, logs and broadcasts. A receipt is final once its covering block commits.</p>
    <table><thead><tr><th>Interface</th><th>Endpoint</th><th>Purpose</th></tr></thead><tbody>
      <tr><td>Node HTTP</td><td><code>PAXEER_RPC_URL</code></td><td>Your configured root JSON-RPC URL for reads and broadcasts.</td></tr>
      <tr><td>Node WebSocket</td><td><code>PAXEER_WS_URL</code></td><td>Your configured WebSocket URL for subscriptions and RPC sessions.</td></tr>
      <tr><td>Hosted gateway</td><td><code>services/platform/hosted/gateway/openrpc.json</code></td><td>Contract for the hosted gateway’s methods, parameters, results and errors.</td></tr>
    </tbody></table>
    <p>Set the example variables to the endpoints supplied for your deployment. These are client-side variable names, not node configuration keys. HTTP JSON-RPC requests use the endpoint root. Node listener defaults and enabled methods are defined in <code>rpc/config/config.go</code> and <code>rpc/server.go</code>; public provider limits depend on its configuration.</p>
    <h2>Verify the chain before signing</h2>
    <table><thead><tr><th>Setting</th><th>Value</th></tr></thead><tbody><tr><td>EVM chain ID</td><td>125 / <code>0x7d</code></td></tr><tr><td>Consensus identifier</td><td><code>hyperpax_125-1</code></td></tr><tr><td>Native gas coin</td><td><code>uhpx</code> (bond denom); bank metadata displays PAX at exponent 6 over <code>uhpx</code>. EVM amounts use wei, tracked as a sub-<code>uhpx</code> remainder.</td></tr></tbody></table>
    <pre><code>{`curl "$PAXEER_RPC_URL" \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'`}</code></pre>
    <p>Check that the response is <code>0x7d</code>, then query <code>eth_blockNumber</code> to check head progress. Signing uses the configured chain ID for replay protection.</p>
    <h2>Subscribe to chain heads</h2>
    <pre><code>{`{"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]}`}</code></pre>
    <p>Send the subscription request over your configured WebSocket connection. Reconnect a dropped connection and recreate subscriptions. Store your last processed block number so you can query missing logs after reconnecting. Deduplicate logs by transaction hash and log index; require covering-block commitment before treating an outcome as final.</p>
    <h2>Fallback and request limits</h2>
    <p>A node can trail another node. Compare block heights when freshness matters and use subscriptions where available. Bound event queries: <code>evm.max_blocks_for_log</code> defaults to 2000 and <code>evm.max_log_no_block</code> defaults to 10000. Operators can configure different limits.</p>
    <p>After a broadcast timeout, query the signed transaction's hash before deciding whether to rebroadcast the identical transaction. An interrupted transport does not establish rejection. Use <code>pax_sequencerReceipt</code> to distinguish submitted, receipted and committed transactions. The node’s governed receipt-key list is available through <code>px_getReceiptKeys</code>.</p>
    <h2>Continue building</h2>
    <ul><li><Link href="/json-rpc">Ethereum methods and compatibility</Link></li><li><Link href="/json-rpc-unsupported">Unsupported Ethereum methods</Link></li><li><Link href="/platform-api">Hosted platform API reference</Link></li><li><Link href="/finality">Receipt and block commitment</Link></li><li><Link href="/building-a-frontend">Wallet and frontend integration</Link></li></ul>
  </DocsLayout>
}
