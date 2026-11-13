import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'
export const metadata = metadataFor('/emulator')
export default function EmulatorPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Local Protocol Emulator</h1><p className="page-description">Develop Paxeer X applications against real native protocol transitions, local signing identities and verifiable receipts.</p></div>
    <h2>What the emulator provides</h2>
    <p>The emulator runs a local gateway around the native core transition engine. It provides deterministic development workflows for accounts, payments, canonical activities and Programs, with a sequencer identity that clients explicitly trust. Local prefunding supports repeatable scenarios without moving production value.</p>
    <table><thead><tr><th>Surface</th><th>Purpose</th></tr></thead><tbody>
      <tr><td><code>GET /healthz</code></td><td>Check local readiness.</td></tr>
      <tr><td><code>GET /v1/sequencer</code></td><td>Read the advertised native network ID and sequencer public key.</td></tr>
      <tr><td><code>POST /__emulator/accounts/prefund</code></td><td>Establish local development funding through emulator account tooling.</td></tr>
      <tr><td><code>GET /v1/state</code>, <code>GET /v1/protocol/account-state/head</code></td><td>Inspect emulator state and the current account-state head.</td></tr>
      <tr><td><code>{'GET /v1/dids/{did}/sequence'}</code></td><td>Read an identity sequence.</td></tr>
      <tr><td><code>POST /__emulator/time/set</code>, <code>POST /__emulator/time/advance</code></td><td>Set or advance local protocol time for deterministic scenarios.</td></tr>
      <tr><td><code>POST /__emulator/faults</code></td><td>Inject a fault for failure-path testing.</td></tr>
      <tr><td><code>GET</code> / <code>PUT /__emulator/snapshot</code></td><td>Export or import an emulator snapshot.</td></tr>
      <tr><td><code>/v1/settle</code></td><td>Retired: returns HTTP 410.</td></tr>
      <tr><td>Native activity and receipt workflows</td><td>Exercise signing, state transitions and independently verifiable outcomes.</td></tr>
      <tr><td>Programs lifecycle and calls</td><td>Develop deployment, discovery, simulation and execution journeys.</td></tr>
    </tbody></table>
    <h2>Provision a sequencer identity</h2>
    <pre><code>{`layerx --json emulator provision`}</code></pre>
    <p>Provisioning generates a sequencer seed from operating-system randomness and publishes its public trust anchor. It creates <code>emulator/sequencer.seed</code> and <code>emulator/sequencer.anchor</code> under the CLI profile directory using owner-only storage and atomic identity publication. Retain the paths returned by the command. The seed is secret; the anchor is the public verification identity.</p>
    <p>Existing files cause <code>sequencer_seed_exists</code> or <code>sequencer_trust_anchor_exists</code>. Use <code>layerx emulator provision --force</code> only when deliberately replacing the local identity, then rebind clients to the new anchor.</p>
    <h2>Start the gateway</h2>
    <pre><code>{`layerx emulator up \
  --listen 127.0.0.1:9402 \
  --network-id 402 \
  --sequencer-seed-file <provisioned-seed-path>`}</code></pre>
    <p>Run the gateway in a dedicated terminal. The listener must be loopback because emulator control routes provide local development authority. Network ID 402 is the local default, distinct from Paxeer X EVM chain ID 125. Network ID zero is reserved.</p>
    <table><thead><tr><th>Option</th><th>Use</th></tr></thead><tbody>
      <tr><td><code>--sequencer-seed-file</code></td><td>Required provisioned signing seed.</td></tr>
      <tr><td><code>--listen</code></td><td>Choose an IP socket address on loopback.</td></tr>
      <tr><td><code>--network-id</code></td><td>Bind activities and client handshake to a local native network.</td></tr>
      <tr><td><code>--protocol-version</code></td><td>Select the supported protocol version for a compatibility scenario.</td></tr>
      <tr><td><code>--time-ms</code></td><td>Choose the local protocol time in milliseconds for deterministic scenarios.</td></tr>
      <tr><td><code>--prefund</code></td><td>Repeat a DID, public key and amount tuple for initial local account funding.</td></tr>
    </tbody></table>
    <h2>Bind the CLI to that identity</h2>
    <pre><code>{`layerx environment use emulator \
  --endpoint http://127.0.0.1:9402 \
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
    <pre><code>{`layerx key create alice
layerx key default alice
layerx account create --key alice --initial-amount 1000000
layerx key create bob
layerx account create --key bob --initial-amount 0
layerx account get --did <alice-did>`}</code></pre>
    <p>Key creation stores secret material in the selected credential store. Account creation in the emulator uses its prefunding route. Read public DIDs and account identifiers from the returned metadata instead of inventing bindings. The wallet convenience flow <code>layerx wallet create &lt;name&gt;</code> also registers a local identity and main account.</p>
    <h3>Prefund at startup</h3>
    <pre><code>{`layerx emulator up --sequencer-seed-file <seed-path> \
  --prefund '<did>,<64-hex-public-key>,1000000'`}</code></pre>
    <p>Prefunding binds the supplied DID and 32-byte public key to the local account setup. The amount accepts a decimal low component or an explicit high:low pair. Keep the signing key, public binding and DID consistent across setup and activity construction.</p>
    <h2>Exercise payments and receipts</h2>
    <pre><code>{`layerx --json payment test \
  --from <alice-account> --to <bob-account> \
  --currency <asset-id> --amount 1000 \
  --idempotency-key emulator-payment-0001
layerx --json receipt get <0x-prefixed-transaction-hash>`}</code></pre>
    <p>Retain the quote, commit outcome, transaction hash and canonical receipt. <code>receipt get</code> fetches by 0x-prefixed transaction hash. Reuse the stable idempotency key for the same payment after an uncertain response. Independently verify receipt bytes with <code>layerx receipt verify</code>, supplying the batch ID, asset, previous and resulting state roots and trusted sequencer public key. A journey response alone does not replace receipt verification.</p>
    <h2>Develop deterministic Programs</h2>
    <pre><code>{`layerx new local-program
layerx program build --manifest-path local-program/Cargo.toml
layerx program discover <program-id>
layerx program interface get <program-id>`}</code></pre>
    <p>Use deploy and upgrade to exercise lifecycle authorization, simulate to obtain non-commit evidence, and call to obtain receipt-bound execution. Preserve current sequences, state roots, fuel limits, validity windows and interface digests. Simulation establishes <code>committed: false</code>; execution verifies the returned activity, ABI, sequencer identity and terminal payload before typed success.</p>
    <h2>Move the application to a hosted profile</h2>
    <p>Keep emulator prefunding and local sequencer keys in the development environment. Bind a hosted profile with its own HTTPS endpoint, native network ID and trusted sequencer identity, then establish hosted identity and gateway credentials. A2A installation uses a hosted beta or production profile. MCP installation instead follows its enrolled daemon binding.</p>
    <p>Continue with <Link href="/cli">the CLI reference</Link>, <Link href="/platform-api">platform API</Link>, <Link href="/a2a">A2A agents</Link> and <Link href="/wasm-runtime">Programs</Link>.</p>
  </DocsLayout>
}
