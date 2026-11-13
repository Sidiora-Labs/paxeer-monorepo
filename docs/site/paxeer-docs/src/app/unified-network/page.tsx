import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import Link from 'next/link'

export const metadata = metadataFor('/unified-network')

export default function UnifiedNetwork() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Unified Network Architecture</h1>
        <p className="page-description">How Paxeer X combines EVM and Cosmos execution with LayerX activities, identity, programs, and receipts in one BFT network.</p>
      </div>

      <DocArt name="system-overview" />
      <h2>The Network Model</h2>
      <p>Paxeer X is one BFT network with EVM chain ID <code>125</code> (<code>0x7d</code>). The node binary <code>paxd</code> runs the Cosmos SDK application and native EVM. The LayerX kernel runs in-process inside <code>paxd</code> through <code>modules/lxphost</code>, and <code>node/sequencer</code> supplies execution ordering within the node.</p>
      <p>Kernel state changes commit with the rest of the node&apos;s transaction state. A receipt is final once its block is committed. Applications establish finality from the committed block and the receipt&apos;s inclusion evidence.</p>
      <p>Sub-chains are tenant domains stored in the database and managed by <code>modules/layerxchains</code>. Creating a domain namespaces state within the shared network. It does not require new node infrastructure.</p>

      <h2>State and Authority</h2>
      <table>
        <thead><tr><th>State</th><th>Authoritative owner</th><th>How applications use it</th></tr></thead>
        <tbody>
          <tr><td>EVM contracts and Cosmos balances</td><td>Paxeer application state committed by chain consensus</td><td>Contract calls, transactions, native modules and registered token pointers</td></tr>
          <tr><td>Agent identities and delegated authority</td><td>LayerX identity kernel</td><td>DIDs, primary keys, sessions, scoped grants, recovery and revocation</td></tr>
          <tr><td>Economic activity and balances</td><td>Kernel state committed in <code>paxd</code> blocks</td><td>Receipts, replay, module queries and state witnesses</td></tr>
          <tr><td>Receipt finality</td><td>Committed block</td><td><code>pax_sequencerReceipt</code> rungs: <code>submitted</code>, <code>receipted</code>, <code>committed</code></td></tr>
          <tr><td>Tenant domains</td><td><code>modules/layerxchains</code></td><td>Namespaced state within the shared network</td></tr>
          <tr><td>Indexes and user interfaces</td><td>Projections of committed evidence</td><td>Search, account history and operational views, with receipts retained for verification</td></tr>
        </tbody>
      </table>
      <p>An address binding identifies the same participant across the EVM and LayerX identity systems. Grants still determine the actions that participant or its delegates can perform. A verified signature establishes who signed evidence. Authorization and application policy establish whether that evidence is acceptable for a specific operation.</p>

      <h2>From Intent to Committed Activity</h2>
      <h3>Canonical Signed Activities</h3>
      <p>A LayerX state change uses a canonical binary envelope containing protocol and network identifiers, activity type, actor DID, authority, account sequence, timestamp bounds, idempotency key, fee limit, payload binding, and signature. Fixed-width integers and canonical decoding keep independent executors in agreement. Time checks use the batch timestamp.</p>
      <p>The kernel resolves authority, checks sequence and payload integrity, computes the fee, and executes the module inside a state journal. Successful execution commits its effects. An admitted operation that fails rolls back module effects while still recording its fee, sequence, and receipt. An admission refusal consumes neither fee nor sequence. An idempotent retry returns the original receipt without repeating the economic effect.</p>
      <h3>Receipts, Replay and State Roots</h3>
      <p>Every admitted activity receives a global sequence and a signed receipt linked to the previous and resulting state roots. The node executes transactions before proposing them in receipt order. The signed receipt records execution. The committed block establishes finality. <code>pax_sequencerReceipt</code> distinguishes a known transaction without a receipt (<code>submitted</code>), a signed receipt awaiting commitment (<code>receipted</code>), and a receipt covered by the committed block&apos;s sequence batch (<code>committed</code>). The hosted gateway&apos;s <code>px_getReceiptKeys</code> method returns pinned node receipt keys.</p>
      <p>For independent verification, authenticate the block header and CometBFT commit, then verify transaction or state inclusion against the corresponding authenticated root. A successful submission or signed receipt alone must not be treated as a committed result.</p>

      <h2>Native Precompiles</h2>
      <p>Contracts reach native modules through <code>precompiles/</code>. The merge plan is <code>v7.0</code>. The thirteen merge entries are the five construct precompiles and eight agent-function precompiles in precompiles/setup.go. The table below also includes other native interfaces. Select the interface for the upgrade active at the queried height.</p>
      <table>
        <thead><tr><th>Precompile</th><th>Address suffix</th><th>Role</th></tr></thead>
        <tbody>
          <tr><td>Addr</td><td><code>0x1004</code></td><td>Resolve Paxeer and EVM identities; bind and resolve LayerX DIDs and unified accounts.</td></tr>
          <tr><td>Perps</td><td><code>0x1015</code></td><td>Perpetuals interface selected by the v6.12.0 precompile set in <code>node/precompiles_v6120.go</code>.</td></tr>
          <tr><td>LayerXBridge</td><td><code>0x1016</code></td><td>Mint and burn bridged assets under registered-chain, attestor, threshold and cap rules.</td></tr>
          <tr><td>Launchpad</td><td><code>0x1017</code></td><td>Token launch interface selected by the v6.12.0 precompile set.</td></tr>
          <tr><td>FeeToken / XWeb</td><td><code>0x1018</code> / <code>0x1019</code></td><td>Fee-token preferences and attested web requests, respectively.</td></tr>
          <tr><td>layerxledger, layerxpay, layerxauth, layerxprograms, layerxchains</td><td><code>0x101A</code>–<code>0x101E</code></td><td>Merge precompiles for ledger, payment, authorization, programs, and tenant domains.</td></tr>
          <tr><td>orob, clearing, oracle aggregator, pofq, scheduler, streams, tee attestor, eip712</td><td><code>0x0901</code>–<code>0x0908</code></td><td>Remaining merge precompiles.</td></tr>
          <tr><td>Pricefeed and spot</td><td><code>0x1020</code>–<code>0x1025</code></td><td><code>pricefeed</code> at <code>0x1020</code>; <code>spot_pecor</code>, <code>spot_orders</code>, <code>spot_stoporders</code>, <code>spot_quoter</code> and <code>spot_vault</code> at <code>0x1021</code>–<code>0x1025</code> in the v6.12.0 set.</td></tr>
        </tbody>
      </table>
      <p>Each suffix identifies a 20-byte address padded with leading zeros. See <Link href="/precompiles">Precompiles</Link> for contract interfaces.</p>
      <p>The served precompile set depends on the queried block height and module activation. Use the ABI in the matching <code>precompiles/</code> directory and check the target module&apos;s state before submitting a transaction. The node selects versioned implementations through <code>node/precompiles_v6120.go</code> and its merge activation gates.</p>

      <h2>Unified Identity</h2>
      <p>A LayerX identity is rendered as <code>did:layerx:</code> followed by its 32-byte Ed25519 public key in lowercase hexadecimal. The canonical main account name is <code>agent:did:layerx:&lt;public-key&gt;:main</code>. The address precompile resolves the EVM address, Paxeer address, DID key, and derived LayerX main account together.</p>
      <p>Binding requires both keys: the DID signs the binding domain, EVM chain ID, EVM address, and nonce; the EVM account submits the transaction. Bindings are one-to-one. Unbinding consumes a nonce, and previously signed binding messages cannot be replayed as a fresh association.</p>

      <h2>Finality</h2>
      <ol>
        <li><strong>Submit:</strong> A signed activity or transaction is known to the sequencer (<code>submitted</code>).</li>
        <li><strong>Execute:</strong> The kernel executes it and issues a signed receipt (<code>receipted</code>).</li>
        <li><strong>Commit:</strong> The block containing its sequence batch commits (<code>committed</code>). The receipt is final at this point.</li>
      </ol>
      <p>Finality depends on BFT block commitment. Keep the receipt, authenticated block header, commit, and inclusion evidence needed to verify the result. See <Link href="/checkpoints">Checkpoints</Link> and <Link href="/finality">Finality</Link> for the evidence boundaries.</p>

      <h2>Developer and Service Boundaries</h2>
      <p>EVM applications use Ethereum-compatible tooling and JSON-RPC. Public node namespaces are <code>debug</code>, <code>echo</code>, <code>eth</code>, <code>net</code>, <code>pax</code>, <code>test</code> and <code>txpool</code>. Method availability depends on node configuration. The hosted gateway exposes a separate <code>px_</code> API. Its deployment URL and credentials come from the operator. Gateway method availability does not follow from public node URLs.</p>
      <p>Workflows carry the actor identity, intended action, and resulting evidence through routing, execution, and indexing. A unified experience can present one account and one history while retaining the receipt and EVM transaction identifiers needed to audit each step.</p>

      <h2>Build Across the Whole System</h2>
      <ul>
        <li>Compose agent commerce from identity, budgets, escrow, delivery attestations and receipt-backed payment.</li>
        <li>Build programmable businesses with WASM programs, program-owned accounts and bounded spending grants.</li>
        <li>Connect Solidity applications to verified agent execution and committed LayerX state.</li>
        <li>Combine token launches, spot and perps activity, and external-chain assets under their own verification rules.</li>
      </ul>
      <p>Continue with <Link href="/paxeer-vs-layerx">Paxeer X and LayerX</Link>, <Link href="/modules">Modules</Link>, <Link href="/contracts">Contracts</Link>, and <Link href="/consensus">Consensus</Link>.</p>
      <div className="prev-next">
        <Link href="/paxeer-vs-layerx"><div className="prev-next-label">Previous</div><div className="prev-next-title">Paxeer X and LayerX</div></Link>
        <Link href="/network-parameters"><div className="prev-next-label">Next</div><div className="prev-next-title">Network Parameters</div></Link>
      </div>
    </DocsLayout>
  )
}
