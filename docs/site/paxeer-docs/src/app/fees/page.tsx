import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/fees')

export default function Fees() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Fees and payment costs</h1>
        <p className="page-description">Understand Paxeer X EVM gas, kernel execution fees, program occupancy, merchant pricing, spending ceilings and fee estimation.</p>
      </div>
      <h2>Identify the charge you are approving</h2>
      <table><thead><tr><th>Cost</th><th>What it pays for</th><th>Where to inspect it</th></tr></thead><tbody>
        <tr><td>EVM gas</td><td>EVM transaction execution, measured in gas and priced in the native gas token</td><td>Transaction fee fields, current EVM parameters, gas estimate, execution receipt</td></tr>
        <tr><td>Native protocol fee</td><td>Canonical activity admission and execution under the committed kernel fee schedule</td><td>Fee estimate, signed <code>fee_limit</code>, canonical receipt</td></tr>
        <tr><td>Programs CALL fee</td><td>Program execution under its versioned execution budget and fee schedule</td><td>Program-specific estimate, authorized budget, CALL outcome</td></tr>
        <tr><td>Programs occupancy</td><td>Namespace bytes that persist across protocol batches</td><td>Occupancy receipt and batch settlement evidence</td></tr>
        <tr><td>Service or merchant price</td><td>The resource, cart, subscription, or metered usage being purchased</td><td>Accepted quote, x402 requirement, order digest, payment receipt</td></tr>
        <tr><td>Provider or conversion cost</td><td>Charges attached to the selected external payment mechanism or rail</td><td>The provider&apos;s terms and the accepted route quote</td></tr>
      </tbody></table>
      <p>A purchase amount and its network execution cost are separate terms. A merchant cart total is the sum of catalog prices and quantities; it does not define the protocol schedule. There is no universal merchant processing or ramp percentage to apply across every route. Review the specific quote and provider terms for the mechanism you select.</p>
      <h2>EVM gas</h2>
      <p>EVM transactions use Ethereum-style gas accounting. Dynamic-fee transactions carry maximum fee and priority fee fields; execution receipts report gas used and effective gas price. Query the current network settings when preparing a transaction rather than assuming fixed base-fee bounds.</p>
      <p>Use the EVM RPC and wallet fee controls for an EVM transfer or contract call. Native LayerX activity fee estimates apply to canonical activities rather than arbitrary EVM transactions. A workflow spanning both execution planes should expose the costs and evidence of its individual legs.</p>
      <h2>The native committed schedule</h2>
      <p>Native activity fees are deterministic functions of committed schedule bytes and execution meters. Parameters include <code>base_fee</code>, <code>per_activity_type_unit</code>, <code>per_encoded_byte</code>, <code>per_execution_unit</code>, <code>per_storage_unit</code>, <code>multiplier_basis_points</code>, Asset prices, and module prices.</p>
      <table><thead><tr><th>Schedule version</th><th>Pricing capabilities</th><th>Canonical encoding</th></tr></thead><tbody>
        <tr><td>1</td><td>Base and resource coefficients</td><td>86 bytes</td></tr>
        <tr><td>2</td><td>Coefficients plus ten named Asset prices</td><td>87 + 16 × 10 bytes</td></tr>
        <tr><td>3</td><td>Eleven Asset prices, adding the withdrawal slot</td><td>Version 2 plus 8 bytes</td></tr>
        <tr><td>4</td><td>Version 3 pricing plus seven named module prices</td><td>Version 3 head plus a module-count byte and 16 × 7 price bytes</td></tr>
      </tbody></table>
      <p>Schedule versions select an encoding and pricing contract; they are distinct from an individual estimate&apos;s observed state. The withdrawal price slot is encoded as an unsigned 64-bit value even though other price slots use unsigned 128-bit values. An available schedule slot does not by itself determine which activity encodings a particular submission interface accepts.</p>
      <h2>How the fee is calculated</h2>
      <ol>
        <li>For a Programs CALL carrying an exact program fee, use the declared exact fee units under the applicable program fee schedule.</li>
        <li>Otherwise begin with <code>base_fee</code>.</li>
        <li>For an Asset activity under schedule version 2 or later, add its named flat Asset price. For other activity types, add <code>per_activity_type_unit × activity_type</code>.</li>
        <li>Under version 4, add the named module price for escrow, budget, stream, service, perpetuals, governance, or bridge activities.</li>
        <li>Add the encoded-byte, execution-unit, and storage-unit coefficients multiplied by their corresponding meters.</li>
        <li>Multiply the total by <code>multiplier_basis_points</code>, divide by 10,000, and round upward.</li>
      </ol>
      <pre><code>{`ordinary_fee = ceil(
  (base_fee
   + activity_component
   + applicable_module_price
   + per_encoded_byte * canonical_encoded_bytes
   + per_execution_unit * execution_units
   + per_storage_unit * storage_units)
  * multiplier_basis_points / 10000
)`}</code></pre>
      <p>The exact Programs fee path is valid for Programs module 9, CALL ordinal 3, with a nonzero program fee schedule version. Unknown Asset ordinals fail as <code>LXP_ERR_UNKNOWN_ACTIVITY</code>; unsupported schedule versions fail as <code>LXP_ERR_VERSION_UNSUPPORTED</code>. Integer arithmetic and canonical encodings keep independent execution and replay calculations consistent.</p>
      <h2>Asset and module prices</h2>
      <table><thead><tr><th>Asset operation</th><th>Parameter</th><th>Ordinal</th></tr></thead><tbody>
        <tr><td>Register</td><td><code>fee.asset.register</code></td><td>1</td></tr>
        <tr><td>Pause / unpause</td><td><code>fee.asset.pause</code> / <code>fee.asset.unpause</code></td><td>2 / 3</td></tr>
        <tr><td>Open account</td><td><code>fee.asset.account_open</code></td><td>4</td></tr>
        <tr><td>Send / receive</td><td><code>fee.asset.send</code> / <code>fee.asset.receive</code></td><td>5 / 6</td></tr>
        <tr><td>Issue / revoke grant</td><td><code>fee.asset.grant_issue</code> / <code>fee.asset.grant_revoke</code></td><td>7 / 8</td></tr>
        <tr><td>Mint / burn</td><td><code>fee.asset.mint</code> / <code>fee.asset.burn</code></td><td>10 / 11</td></tr>
        <tr><td>Withdrawal price slot</td><td><code>fee.asset.withdraw</code></td><td>9; schedule version 3 or later</td></tr>
      </tbody></table>
      <p>Version 4 module parameters are <code>fee.module.escrow</code>, <code>fee.module.budget</code>, <code>fee.module.stream</code>, <code>fee.module.service</code>, <code>fee.module.perps</code>, <code>fee.module.governance</code>, and <code>fee.module.bridge</code>. Programs uses its own exact CALL pricing path rather than a slot in this seven-module table.</p>
      <h2>Estimate before signing</h2>
      <p>There is no public <code>lx_estimateFee</code> JSON-RPC method in this checkout. The hosted gateway refuses every <code>lx_*</code> method name with JSON-RPC <code>-32601</code> (<code>Method not found</code>). Do not present an <code>lx_estimateFee</code> request, or response fields such as <code>observed_head_sequence</code>, <code>state_root</code>, or <code>verification: "authenticated_committed_snapshot"</code>, as a public estimation contract.</p>
      <p>The hosted core fee estimate that does exist prices an EVM transaction: it computes <code>gas_limit × max_fee_per_gas</code> and returns only <code>fee</code> (decimal string), <code>parameter_version: 0</code>, an empty <code>canonical_schedule</code>, and <code>canonical_bytes</code>. For native activities, price the exact canonical bytes against the committed schedule described above and set the signed <code>fee_limit</code> from that calculation.</p>
      <p>An estimate reserves no balance, locks no schedule, and proves no execution. When the schedule needs execution or storage meters that estimation cannot establish, the estimator refuses instead of treating the missing meters as zero. LNI <code>fee_estimate</code> and <code>session_fee_state</code> expose the committed schedule projection with the same non-reservation semantics.</p>
      <h2>Set and fund the fee ceiling</h2>
      <ol><li>Prepare the exact canonical activity and estimate its fee.</li><li>Choose a maximum in integer base units and disclose that <code>fee_limit</code> with the other payment terms.</li><li>Ensure the actor&apos;s spendable fee balance covers the full limit, even when the estimate is lower.</li><li>Sign and submit the approved bytes, preserving their activity identifier and idempotency key.</li><li>Verify the final receipt, charged fee, result code, and requested commitment.</li></ol>
      <table><thead><tr><th>Outcome</th><th>Sequence and fee behavior</th><th>Module effects</th></tr></thead><tbody>
        <tr><td>Admission refused</td><td>No global sequence, no consumed account sequence, no fee</td><td>None</td></tr>
        <tr><td>Insufficient spendable balance for the limit</td><td><code>LXP_ERR_FEE_UNPAYABLE</code>; refused before sequence assignment</td><td>None</td></tr>
        <tr><td>Admitted execution succeeds</td><td>Sequence consumed and computed fee charged within the limit</td><td>Committed</td></tr>
        <tr><td>Admitted execution fails</td><td>Sequence consumed; fee charged within the limit; failure receipt emitted</td><td>Rolled back</td></tr>
        <tr><td>Computed fee exceeds the signed limit</td><td><code>LXP_ERR_FEE_LIMIT</code>; charge capped at the limit</td><td>Rolled back</td></tr>
      </tbody></table>
      <p>Protocol fee charges settle as <code>402LXP</code> transfer legs with reason <code>LXP_REASON_PROTOCOL_FEE</code> to treasury account <code>system:fees</code>. Replay verifies the recorded charge, actor debit, treasury credit, schedule version, and resulting state. A failed payment can therefore have a fee-bearing receipt even when its requested transfer did not apply.</p>
      <h2>Programs occupancy and persistent storage</h2>
      <p>Occupancy charges for namespace bytes held across protocol batches. Its time unit is the batch number, not elapsed wall-clock time. The CALL flow prices its execution budget and carves authorized occupancy out of the signed fee limit. Due maintenance transfers settle as ordinary <code>402LXP</code> legs.</p>
      <p>Occupancy is a separate batch settlement. A CALL that writes persistent storage is not fully settled until occupancy settlement for its containing batch completes. Program outcomes carry occupancy byte-batches, fee units, Asset identifier, evidence digest, and transfer root. Preserve the outcome and batch evidence when reconciling persistent storage costs; simulation alone does not perform batch occupancy settlement.</p>
      <h2>Merchant, metered, and subscription pricing</h2>
      <p>A merchant quote binds cart lines, integer totals, Asset, recipient, scheme, network, and timeout into a request digest. One cart uses one payment Asset and recipient. Catalog changes create new quote digests. In x402, accept the offered requirement without editing its amount or recipient, and distinguish that resource price from the network fee needed to settle it.</p>
      <p>Metered and subscription grants authorize bounded future draws with explicit purpose, expiry, and billing windows. Each draw or renewal remains a separately authorized receive and settlement. Price a complete workflow by its actual activities and storage obligations, rather than assuming that an initial grant covers every future network charge.</p>
      <h2>Governance and parameter changes</h2>
      <p>Governance controls <code>fee.base</code>, <code>fee.activity</code>, <code>fee.byte</code>, <code>fee.exec</code>, <code>fee.storage</code>, <code>fee.multiplier_bps</code>, and <code>fee.encoding</code>, together with the named Asset and module prices. Committed schedule bytes are stored under <code>fee.schedule</code>; version 4 also uses <code>fee.module-prices</code>. Read the active committed schedule when estimating instead of copying fixed prices into application code.</p>
      <h2>Related guides</h2>
      <ul><li><Link href="/payments">Payments and settlement</Link></li><li><Link href="/payments-quickstart">First payment walkthrough</Link></li><li><Link href="/evm">EVM execution and gas</Link></li><li><Link href="/programs">Programs and occupancy</Link></li><li><Link href="/platform-api">Public payment API</Link></li><li><Link href="/governance">Governance</Link></li></ul>
    </DocsLayout>
  )
}
