import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/public-rpc')

export default function PublicRpc() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Public RPC endpoints</h1><p className="page-description">Connect wallets and applications to Paxeer X chain ID 125 over HTTPS and WebSocket, and choose the correct interface for chain or kernel requests.</p></div>
    <h2>Choose the execution domain</h2>
    <p>The Paxeer X chain exposes Ethereum JSON-RPC for contracts, balances, logs and transaction broadcasts. The unified router connects the wider network, including LayerX-native requests. An EVM transaction hash and a LayerX activity identifier identify different execution paths; use the matching receipt and commitment interface.</p>
    <table><thead><tr><th>Interface</th><th>Endpoint</th><th>Purpose</th></tr></thead><tbody>
      <tr><td>Unified router</td><td><code>https://api-mainnet-beta.paxeer.network</code></td><td>Network routing and unified interfaces.</td></tr>
      <tr><td>Chain HTTPS</td><td><code>https://api1.mainnet-beta.paxeer.network</code></td><td>Ethereum JSON-RPC requests and broadcasts.</td></tr>
      <tr><td>Chain WebSocket</td><td><code>wss://api1.mainnet-beta.paxeer.network/ws</code></td><td>Ethereum subscriptions and persistent RPC sessions.</td></tr>
    </tbody></table>
    <p>The numbered public full-node names run from <code>api1.mainnet-beta.paxeer.network</code> through <code>api16.mainnet-beta.paxeer.network</code>. Each supports HTTPS and WebSocket. Numbers identify nodes rather than a preference order. WebSocket connections also accept the root path.</p>
    <h2>Verify the chain before signing</h2>
    <table><thead><tr><th>Setting</th><th>Value</th></tr></thead><tbody><tr><td>EVM chain ID</td><td>125 / <code>0x7d</code></td></tr><tr><td>Consensus identifier</td><td><code>hyperpax_125-1</code></td></tr><tr><td>Native gas coin</td><td><code>uhpx</code> (bond denom); bank metadata displays PAX at exponent 6 over <code>uhpx</code>. EVM amounts use wei, tracked as a sub-<code>uhpx</code> remainder.</td></tr></tbody></table>
    <pre><code>{`curl https://api1.mainnet-beta.paxeer.network \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'`}</code></pre>
    <p>Check that the response is <code>0x7d</code>, then query <code>eth_blockNumber</code> to check head progress. Signing uses the configured chain ID for replay protection.</p>
    <h2>Subscribe to chain heads</h2>
    <pre><code>{`wscat -c wss://api1.mainnet-beta.paxeer.network/ws
{"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]}`}</code></pre>
    <p>Reconnect a dropped WebSocket and recreate subscriptions. Store your last processed block number so you can query missing logs after reconnecting. Deduplicate logs by transaction hash and log index; handle the chain's documented finality semantics before irreversible downstream actions.</p>
    <h2>Fallback and request limits</h2>
    <p>Each endpoint fronts a full node and may briefly trail another node. Compare heads across two nodes when freshness matters. Spread sustained polling across endpoints or use subscriptions. The public-node configuration rejects request bodies above 10 MB and allows up to one hour of WebSocket inactivity.</p>
    <p>After a broadcast timeout, query the signed transaction's hash before deciding whether to rebroadcast the identical transaction. An interrupted transport does not establish rejection. LayerX writes use their own idempotency and pending-outcome workflow.</p>
    <h2>Continue building</h2>
    <ul><li><Link href="/json-rpc">Ethereum methods and compatibility</Link></li><li><Link href="/json-rpc-unsupported">Unsupported Ethereum methods</Link></li><li><Link href="/platform-api">LayerX and platform API reference</Link></li><li><Link href="/finality">Execution and settlement commitments</Link></li><li><Link href="/building-a-frontend">Wallet and frontend integration</Link></li></ul>
  </DocsLayout>
}
