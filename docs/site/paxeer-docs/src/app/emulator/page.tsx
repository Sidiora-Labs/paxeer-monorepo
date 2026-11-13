import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'
export const metadata = metadataFor('/emulator')
export default function EmulatorPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Local Protocol Emulator</h1><p className="page-description">Inspect local protocol state with explicit signing identities, account prefunding, time controls and snapshots.</p></div>
    <h2>What the emulator provides</h2>
    <p>The emulator wraps the native core for local inspection and account setup. Its surface document declares the available routes and reports <code>payments.supported: false</code>. Local prefunding, time controls and snapshots support repeatable inspection scenarios. Payment and program execution use the merged node.</p>
    <table><thead><tr><th>Surface</th><th>Purpose</th></tr></thead><tbody>
      <tr><td><code>GET /healthz</code></td><td>Check local readiness.</td></tr>
      <tr><td><code>GET /v1/sequencer</code></td><td>Read the advertised native network ID and sequencer public key.</td></tr>
      <tr><td><code>POST /__emulator/accounts/prefund</code></td><td>Establish local development funding through emulator account tooling.</td></tr>
      <tr><td><code>GET /v1/state</code>, <code>GET /v1/protocol/account-state/head</code></td><td>Inspect local state; the account-state head route returns HTTP 503 with <code>principal_state_proof_unavailable</code>.</td></tr>
      <tr><td><code>{'GET /v1/dids/{did}/sequence'}</code></td><td>Read an identity sequence.</td></tr>
      <tr><td><code>POST /__emulator/time/set</code>, <code>POST /__emulator/time/advance</code></td><td>Set or advance local protocol time for deterministic scenarios.</td></tr>
      <tr><td><code>POST /__emulator/faults</code></td><td>Inject a fault for failure-path testing.</td></tr>
      <tr><td><code>GET</code> / <code>PUT /__emulator/snapshot</code></td><td>Export or import an emulator snapshot.</td></tr>
      <tr><td><code>GET /v1/surface</code></td><td>Read the route list and declared execution capabilities.</td></tr>
      <tr><td>Payment execution</td><td>Use merged-node <code>eth_sendRawTransaction</code> and <code>pax_sequencerReceipt</code>.</td></tr>
      <tr><td>Local receipt reads</td><td>Existing receipt entries return HTTP 410; an unknown receipt returns HTTP 404.</td></tr>
    </tbody></table>
    <h2>Provision a sequencer identity</h2>
    <pre><code>{`layerx --json emulator provision`}</code></pre>
    <p>Provisioning generates a sequencer seed from operating-system randomness and publishes its public trust anchor. It creates <code>emulator/sequencer.seed</code> and <code>emulator/sequencer.anchor</code> under the CLI profile directory using owner-only storage and atomic identity publication. Retain the paths returned by the command. The seed is secret; the anchor is the public verification identity.</p>
    <p>Existing files cause <code>sequencer_seed_exists</code> or <code>sequencer_trust_anchor_exists</code>. Use <code>layerx emulator provision --force</code> only when deliberately replacing the local identity, then rebind clients to the new anchor.</p>
    <h2>Start the gateway</h2>
    <pre><code>{`layerx emulator up \
  --listen <loopback-socket> \
  --network-id 402 \
  --sequencer-seed-file <provisioned-seed-path>`}</code></pre>
    <p>Run the gateway in a dedicated terminal. Set the listener to a loopback socket; its default port is 9402. Emulator control routes provide local development authority. Network ID 402 is the local default, separate from the merged network&apos;s EVM chain ID 125. Network ID zero is reserved.</p>
    <table><thead><tr><th>Option</th><th>Use</th></tr></thead><tbody>
      <tr><td><code>--sequencer-seed-file</code></td><td>Required provisioned signing seed.</td></tr>
      <tr><td><code>--listen</code></td><td>Choose an IP socket address on loopback.</td></tr>
      <tr><td><code>--network-id</code></td><td>Bind activities and client handshake to a local native network.</td></tr>
      <tr><td><code>--protocol-version</code></td><td>Select protocol version 2 or 3; the default is 2.</td></tr>
      <tr><td><code>--time-ms</code></td><td>Choose the local protocol time in milliseconds for deterministic scenarios.</td></tr>
      <tr><td><code>--prefund</code></td><td>Repeat a DID, public key and amount tuple for initial local account funding.</td></tr>
    </tbody></table>
    <h2>Bind the CLI to that identity</h2>
    <pre><code>{`layerx environment use emulator \
  --endpoint <emulator-url> \
  --network-id 402 \
  --sequencer-trust-anchor-file <provisioned-anchor-path>
layerx --json environment current`}</code></pre>
    <p>Supply endpoint, network ID and one trust-anchor input together. The CLI waits for the listener and reads <code>/v1/sequencer</code>, then checks the network and public identity before accepting the profile. A profile can also use <code>--sequencer-trust-anchor &lt;hex&gt;</code> in place of the file. Subsequent selection uses <code>layerx environment use emulator</code>.</p>
    <h3>Diagnose handshake failures</h3>
    <table><thead><tr><th>Code</th><th>Resolution</th></tr></thead><tbody>
      <tr><td><code>environment_input_missing</code></td><td>Provide the complete endpoint, network and anchor set.</td></tr>
      <tr><td><code>sequencer_identity_unavailable</code></td><td>Start the listener and confirm the endpoint and identity route.</td></tr>
      <tr><td><code>network_id_mismatch</code></td><td>Match client configuration to the running emulator network.</td></tr>
      <tr><td><code>sequencer_trust_anchor_mismatch</code></td><td>Use the public anchor corresponding to the running sequencer seed.</td></tr>
      <tr><td><code>sequencer_trust_anchor_unreadable</code>, <code>sequencer_trust_anchor_malformed</code></td><td>Correct the anchor path or public-key encoding.</td></tr>
    </tbody></table>
    <h2>Create development accounts</h2>
    <pre><code>{`layerx key create payer
layerx key default payer
layerx account create --key payer --initial-amount 1000000
layerx key create payee
layerx account create --key payee --initial-amount 0
layerx account get --did <payer-did>`}</code></pre>
    <p>Key creation stores secret material in the selected credential store. Account creation in the emulator uses its prefunding route. Read public DIDs and account identifiers from the returned metadata instead of inventing bindings. This account setup is local to the emulator; it does not fund an EVM address on the merged network.</p>
    <h3>Prefund at startup</h3>
    <pre><code>{`layerx emulator up --sequencer-seed-file <seed-path> \
  --prefund '<did>,<64-hex-public-key>,1000000'`}</code></pre>
    <p>Prefunding binds the supplied DID and 32-byte public key to the local account setup. The amount accepts a decimal low component or an explicit high:low pair. Keep the signing key, public binding and DID consistent across setup and activity construction.</p>
    <h2>Read merged-network receipts</h2>
    <pre><code>{`layerx --json --rpc <node-url> receipt get <0x-prefixed-transaction-hash>`}</code></pre>
    <p>Point <code>--rpc</code> at the merged node&apos;s root JSON-RPC origin. <code>receipt get</code> calls <code>pax_sequencerReceipt</code> with the transaction hash. Preserve the response and independently verify its receipt authority and block-commit evidence. A receipt is final once its block is committed.</p>
    <h2>Build programs and query the merged node</h2>
    <pre><code>{`layerx new local-program
layerx program build --manifest-path local-program/Cargo.toml
layerx --rpc <node-url> program discover <program-id>
layerx --rpc <node-url> program interface get <program-id>`}</code></pre>
    <p>Scaffolding and building create local artifacts. Discovery and interface reads query the program precompile through the selected node. Program lifecycle transactions, simulation and calls use the merged network rather than the emulator&apos;s inspection routes. Preserve code hashes, ABI versions, interface digests and execution receipts.</p>
    <h2>Move the application to a hosted profile</h2>
    <p>Keep emulator prefunding and local signing keys in the development environment. Configure the node origin for merged-network commands and a hosted profile for authenticated hosted APIs. A2A installation uses a hosted beta or production profile; MCP installation follows its enrolled daemon binding.</p>
    <p>Continue with <Link href="/cli">the CLI reference</Link>, <Link href="/platform-api">platform API</Link>, <Link href="/a2a">A2A agents</Link> and <Link href="/wasm-runtime">Programs</Link>.</p>
  </DocsLayout>
}
