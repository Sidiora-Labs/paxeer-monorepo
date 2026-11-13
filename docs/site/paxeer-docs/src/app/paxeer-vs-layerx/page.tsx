import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/paxeer-vs-layerx')

export default function PaxeerAndLayerX() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Paxeer X and LayerX</h1>
        <p className="page-description">One BFT network combines EVM smart contracts, autonomous agent execution, and programmable finance, with finality at block commit.</p>
      </div>

      <h2>One Network, Kernel In-Process</h2>
      <p>Paxeer X is one BFT network with EVM chain ID <code>125</code>. <code>paxd</code> supplies the Cosmos SDK chain and native EVM. The LayerX kernel is embedded in the node through <code>modules/lxphost</code> and participates in block proposal and deterministic execution. <code>node/sequencer</code> supplies execution ordering within the node. Applications compose both through native EVM precompiles and protocol receipts.</p>
      <p>The names describe execution surfaces, not separate networks. A normal agent payment executes as a signed LayerX activity without requiring a separate EVM transaction. Its kernel state changes commit with the rest of the node&apos;s transaction state. A receipt is final once its block is committed. There is no later checkpoint or settlement step.</p>

      <table>
        <thead><tr><th>Capability</th><th>Paxeer surface</th><th>LayerX surface</th></tr></thead>
        <tbody>
          <tr><td>Application execution</td><td>Solidity contracts, native precompiles, Cosmos modules</td><td>Signed activities, economic modules, deterministic WASM programs</td></tr>
          <tr><td>Identity</td><td>EVM and Paxeer addresses</td><td>Agent DIDs, sessions, scoped grants, recovery and revocation</td></tr>
          <tr><td>Money</td><td>Native assets and bank balances</td><td>Payments, holds, escrow, budgets, streams and protocol balances</td></tr>
          <tr><td>Markets</td><td><code>perps</code> at <code>0x1015</code>, <code>pricefeed</code> at <code>0x1020</code>, spot at <code>0x1021</code>–<code>0x1025</code>, launchpad at <code>0x1017</code>, bridge at <code>0x1016</code></td><td>Trading, positions, funding, and liquidation modules</td></tr>
          <tr><td>Evidence</td><td>Committed blocks; <code>pax_sequencerReceipt</code></td><td>Signed receipts, ordered batches, state witnesses and replay</td></tr>
          <tr><td>Security authority</td><td>Validators and governance under BFT consensus</td><td>Actor authority, deterministic execution and independently replayed activity</td></tr>
        </tbody>
      </table>

      <h2>What the Merger Enables</h2>
      <h3>Agent Businesses with Real Financial Controls</h3>
      <p>An agent operates through its own identity and bounded authority. It pays for services, commits funds to escrow, spends against a budget, and receives delivery evidence. Businesses issue assets and agents trade the resulting instruments. Programs coordinate these operations while the protocol controls every balance change.</p>
      <h3>Solidity Applications That Understand Agent Activity</h3>
      <p>A contract consumes a verified service receipt, proves a LayerX account balance, or checks a program result before applying its own rules. The merge precompiles at <code>0x101A</code>–<code>0x101E</code> and <code>0x0901</code>–<code>0x0908</code> expose ledger, payment, authorization, program, and tenant-domain surfaces. Select the interface for the upgrade active at the queried height.</p>
      <h3>Connected Markets and Assets</h3>
      <p>In the v6.12.0 set, <code>perps</code> is served at <code>0x1015</code> in place of the earlier <code>layerxExchange</code>. The same set serves the second-generation <code>launchpad</code> at <code>0x1017</code>, <code>pricefeed</code> at <code>0x1020</code>, and <code>spot_pecor</code>, <code>spot_orders</code>, <code>spot_stoporders</code>, <code>spot_quoter</code> and <code>spot_vault</code> at <code>0x1021</code>–<code>0x1025</code>. Registered external chains connect through <code>layerxBridge</code> at <code>0x1016</code>. Each path retains its own economic and verification rules.</p>

      <h2>A Shared Account, Explicit Authority</h2>
      <p>The address precompile binds an EVM address to a <code>did:layerx</code> identity with consent from both keys. The DID signs a chain-bound, address-bound message with a nonce; the EVM owner submits it. Unified account resolution connects the EVM address, Paxeer address, DID public key, and LayerX main account.</p>
      <p>Binding makes identity discoverable across surfaces. Spending remains subject to the authority of the operation: an EVM transaction, a LayerX session or scoped grant, or a program spending grant. Revocation and expiry constrain delegated actions.</p>

      <h2>Evidence and Finality</h2>
      <ol>
        <li>Execute payments, trades, deliveries, or program calls as signed activities or EVM transactions.</li>
        <li>Receive a signed execution receipt. <code>pax_sequencerReceipt</code> reports <code>submitted</code>, <code>receipted</code>, or <code>committed</code>.</li>
        <li>Treat the result as final only at <code>committed</code>, when the block covering its sequence batch has committed.</li>
      </ol>
      <p>The external checkpoint registry, bonded guarantor attestations, and L0–L4 settlement ladder are retired. The pre-merge <code>layerxVerify</code> (<code>0x1012</code>) and <code>layerxCustody</code> (<code>0x1013</code>) interfaces are retired at the merge. <code>layerxAnchor</code> (<code>0x1014</code>) rejects writes after the merge. These historical interfaces do not provide current funding or finality paths.</p>
      <p>The native asset pays EVM transaction gas. Its human-unit identifier in node parameters is <code>hpx</code>, with base unit <code>uhpx</code>. LayerX activities use their committed fee schedule and declared fee limits. Applications show the activity fee and EVM gas at the corresponding step.</p>

      <h2>Choose Your Starting Point</h2>
      <ul>
        <li>Read <Link href="/unified-network">Unified Network Architecture</Link> for state, proof and authority boundaries.</li>
        <li>Build contracts with <Link href="/foundry">Foundry</Link> or <Link href="/hardhat">Hardhat</Link>, then explore <Link href="/precompiles">native precompiles</Link>.</li>
        <li>Explore <Link href="/modules">protocol modules</Link> and the <Link href="/wasm-runtime">WASM runtime</Link> for programmable execution.</li>
        <li>Use <Link href="/network-parameters">Network Parameters</Link> for chain identity and <Link href="/building-a-frontend">Building a Frontend</Link> for wallet integration.</li>
      </ul>
      <div className="prev-next">
        <Link href="/"><div className="prev-next-label">Previous</div><div className="prev-next-title">Introduction</div></Link>
        <Link href="/unified-network"><div className="prev-next-label">Next</div><div className="prev-next-title">Unified Network</div></Link>
      </div>
    </DocsLayout>
  )
}
