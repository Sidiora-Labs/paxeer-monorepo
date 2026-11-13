import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/wasmbinding')

export default function WasmBinding() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">WASM Bindings</h1>
        <p className="page-description">Use Paxeer custom messages and queries from CosmWasm, including token creation, EVM calls, address association, and native module reads.</p>
      </div>

      <div className="source-note"><strong>Source:</strong> <code>Paxeer-X-Network/node/wasmbinding/</code> and each module&apos;s <code>client/wasm/</code> package.</div>
      <h2>How bindings are registered</h2>
      <p><code>RegisterCustomPlugins</code> installs a custom message encoder and query router in the CosmWasm keeper. These bindings extend standard bank, staking, distribution, and Wasm messages. The package is Go node code; it is not a Rust crate to add to <code>Cargo.toml</code>.</p>
      <p>Contract-side types must serialize to the JSON forms below and implement the appropriate CosmWasm custom message or query traits. Use the schemas in this source revision when generating types or evaluating a client library.</p>

      <h2>Custom messages</h2>
      <p><code>node/wasmbinding/encoder.go</code> dispatches these top-level message variants. Supply one operation per message.</p>
      <table>
        <thead><tr><th>Variant</th><th>Payload and behavior</th></tr></thead>
        <tbody>
          <tr><td><code>create_denom</code></td><td><code>subdenom</code>; creates a tokenfactory denomination under the calling contract.</td></tr>
          <tr><td><code>mint_tokens</code></td><td><code>amount</code> coin object; subject to the denomination&apos;s mint authority.</td></tr>
          <tr><td><code>burn_tokens</code></td><td><code>amount</code> coin object; executes the native tokenfactory burn path.</td></tr>
          <tr><td><code>change_admin</code></td><td><code>denom</code> and <code>new_admin_address</code>; requires the current denomination administrator.</td></tr>
          <tr><td><code>set_metadata</code></td><td>Bank <code>metadata</code> object for the denomination.</td></tr>
          <tr><td><code>set_denom_paused</code></td><td><code>denom</code> and an explicit <code>paused</code> boolean; a missing or null boolean is refused.</td></tr>
          <tr><td><code>call_evm</code></td><td><code>to</code>, integer <code>value</code>, and base64 <code>data</code>; dispatches an internal EVM call.</td></tr>
          <tr><td><code>delegate_call_evm</code></td><td><code>to</code> and base64 <code>data</code>; carries calling-contract and code-hash context into the EVM delegate-call path.</td></tr>
        </tbody>
      </table>
      <pre><code>{`// JSON payload of a custom message, emitted by a CosmWasm contract:
{ "create_denom": { "subdenom": "credits" } }`}</code></pre>
      <p>The tokenfactory encoder supplies the contract as sender. An EVM call likewise derives its sender from the actual execution context; it does not accept an arbitrary wallet address as authority. ABI calldata bytes are base64 in the custom-message JSON, rather than an Ethereum <code>0x</code> hex string.</p>

      <h2>Custom queries</h2>
      <p><code>PaxQueryWrapper</code> selects a route and embeds the route-specific JSON query in <code>query_data</code>. Unknown routes and unknown variants return errors.</p>
      <table>
        <thead><tr><th>Route</th><th>Implemented query families</th></tr></thead>
        <tbody>
          <tr><td><code>evm</code></td><td>Static calls; ERC-20, ERC-721, and ERC-1155 reads; transfer and approval payload generation; interface support; EVM/Paxeer address lookups.</td></tr>
          <tr><td><code>tokenfactory</code></td><td><code>denom_authority_metadata</code>, <code>denoms_from_creator</code>, and <code>denom_paused</code> (which reports the pause state and issuer DID for a denomination).</td></tr>
          <tr><td><code>epoch</code></td><td><code>epoch</code> state.</td></tr>
          <tr><td><code>stakingext</code></td><td><code>unbonding_delegations</code> for a supplied delegator.</td></tr>
          <tr><td><code>oracle</code></td><td><code>exchange_rates</code> and <code>oracle_twaps</code> through the retained native Oracle handler.</td></tr>
        </tbody>
      </table>
      <pre><code>{`// Query the current epoch from inside a CosmWasm contract:
{
  "route": "epoch",
  "query_data": { "epoch": {} }
}`}</code></pre>
      <p>This is a custom querier payload, not an HTTP JSON-RPC request. It must be issued through the contract&apos;s CosmWasm query interface.</p>

      <h2>Address lookup and token queries</h2>
      <p>The EVM route exposes <code>get_pax_address</code> with <code>evm_address</code>, and <code>get_evm_address</code> with <code>pax_address</code>. Responses contain the resolved address and an <code>associated</code> flag. Check that flag before using the result as a linked account.</p>
      <p>ERC query fields are defined in <code>modules/evm/client/wasm/bindings/queries.go</code>. A transfer-payload query only produces encoded calldata; it does not transfer assets. Send the resulting calldata through a supported execution path when a transaction is intended.</p>
      <p>Keep native coin integers, ERC token decimals, and EVM wei values separate. Do not format a bank base-denomination amount using an unrelated ERC token&apos;s display precision.</p>

      <h2>Execution boundaries</h2>
      <p>CosmWasm-to-EVM calls are implemented. The corresponding <Link href="/wasm">wasmd precompile</Link> supports EVM-to-CosmWasm calls, with context and pointer checks. A non-query CosmWasm → EVM → CosmWasm loop is rejected; delegate-call messages should not be treated as unrestricted arbitrary execution.</p>
      <p>The native Oracle querier remains available through this route, while the EVM Oracle precompile is retired. Check available denominations and observation freshness before using price data. See the <Link href="/modules/oracle">Oracle module guide</Link>.</p>

      <h2>Implementation references</h2>
      <ul>
        <li><code>node/wasmbinding/wasm.go</code>: plugin registration.</li>
        <li><code>node/wasmbinding/encoder.go</code> and <code>message_plugin.go</code>: custom variants and module dispatch.</li>
        <li><code>node/wasmbinding/query_plugin.go</code> and <code>queries.go</code>: query wrapper, route selection, and handlers.</li>
        <li><code>modules/evm/client/wasm/encoder.go</code>: base64 decoding and internal EVM message construction.</li>
      </ul>
      <div className="prev-next">
        <Link href="/wasm-runtime"><div className="prev-next-label">Previous</div><div className="prev-next-title">WASM Runtime</div></Link>
        <Link href="/json-rpc"><div className="prev-next-label">Next</div><div className="prev-next-title">JSON-RPC API</div></Link>
      </div>
    </DocsLayout>
  )
}
