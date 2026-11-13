import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import Link from 'next/link'

export const metadata = metadataFor('/receipts')

export default function Receipts() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Receipts &amp; Proofs</h1><p className="page-description">Verify what the protocol executed, what it charged, and whether the covering block has committed.</p></div>
    <p>A receipt is evidence computed by the protocol. Clients submit signed activities; they do not choose receipt fields, balance outcomes, state roots, or result codes. Applications use verified receipts to reconcile payments, release paid resources, display completed work, and recover uncertain submissions.</p>
    <h2>Activity receipt anatomy</h2>
    <table><thead><tr><th>Evidence</th><th>Native fields</th><th>What it establishes</th></tr></thead><tbody>
      <tr><td>Identity and order</td><td><code>activity_id</code>, <code>global_sequence</code>, <code>batch_id</code></td><td>The activity and its position in ordered execution.</td></tr>
      <tr><td>State transition</td><td><code>previous_state_root</code>, <code>resulting_state_root</code>, <code>activity_root</code></td><td>The executed state transition and activity commitments.</td></tr>
      <tr><td>Outcome</td><td><code>result_code</code>, <code>effects</code>, <code>fee_charged</code></td><td>Success or refusal, computed effects, and execution charge.</td></tr>
      <tr><td>Rules</td><td><code>protocol_version</code>, <code>module_id</code>, <code>module_version</code>, <code>parameter_version</code></td><td>The execution rules needed for independent interpretation and replay.</td></tr>
      <tr><td>Authorization</td><td><code>authorization_hash</code>, <code>context_hash</code>, <code>sequencer_signature</code></td><td>The authorization and context commitments authenticated by the receipt signing key.</td></tr>
    </tbody></table>
    <DocArt name="payment-receipt" />
    <h2>Payment evidence</h2>
    <p>A successful payment operation provides protocol-computed before and after balances, asset and amount, sender and recipient account identifiers, sequence facts, transfer-set root, authorization and context hashes, chained state roots, batch identifier, timestamp, and signature. Client-supplied balances cannot establish payment.</p>
    <p>A failed payment produces a failure result with its result code and unchanged payment balances. It does not produce a signed successful payment receipt. The enclosing admitted activity can still incur protocol fee and sequence bookkeeping. Distinguish the payment’s economic outcome from the activity’s execution record.</p>
    <h3>Programs produce receipt-native outcomes</h3>
    <p>Program calls bind terminal success, failure, or resource exhaustion to the activity receipt. Outcome evidence includes runtime and ABI versions, fee and metering schedules, CPU fuel, memory and storage counters, output size, occupancy charges and evidence, call-graph commitments, terminal payload commitments, and transfer roots. A program’s local response or application log does not substitute for the protocol receipt.</p>
    <h2>Finality rungs</h2>
    <p>Node evidence is <code>pax_sequencerReceipt</code>. <code>px_getReceiptKeys</code> lists the node&apos;s pinned receipt keys. A receipt is final once its block is committed. There is no later checkpoint or settlement step.</p>
    <table><thead><tr><th>Rung</th><th>Evidence</th></tr></thead><tbody>
      <tr><td><code>submitted</code></td><td>A known transaction without a signed execution receipt.</td></tr>
      <tr><td><code>receipted</code></td><td>A signed execution receipt awaiting block commitment.</td></tr>
      <tr><td><code>committed</code></td><td>A receipt covered by the committed block’s sequence batch. This is final.</td></tr>
    </tbody></table>
    <p>An activity identifier or submission acknowledgement establishes neither execution nor finality. Authenticate the signed receipt against an independently pinned receipt-key list, match the intended operation, and require covering-block commitment before treating the result as final.</p>
    <p>The agent types crate accepts verification wire ranks 0, 1, 2, 3 and 6: <code>unverified</code>, <code>sequencer_signed</code>, <code>batch_included</code>, <code>state_proven</code> and <code>committed</code>. Receipt signature, batch inclusion and native state-witness checks do not authenticate a CometBFT commit. A parsed label alone does not establish verified evidence.</p>
    <h2>Independent verification</h2>
    <table><thead><tr><th>Surface</th><th>Use</th></tr></thead><tbody>
      <tr><td><code>layerx-proof</code></td><td>Verify signed receipt bytes.</td></tr>
      <tr><td><code>layerx receipt verify</code></td><td>Perform a local check against caller-supplied batch facts.</td></tr>
      <tr><td><code>layerx-portable</code></td><td>Export and verify a <code>layerx-receipt-proof-v2</code> JSON object against independently pinned <code>ReceiptKeyEntry</code> values.</td></tr>
    </tbody></table>
    <p>Portable verification runs offline. Its trust anchor is the caller’s independently pinned receipt-key list, including each key’s allowed signing heights. Merely packaging a receipt with a claimed signer does not make that signer authoritative. The portable proof authenticates a receipt but does not establish block commitment.</p>
    <p>For block-level verification, authenticate the block header and CometBFT commit, then verify transaction or state inclusion against the corresponding authenticated root. State proofs must name the relevant height and key. An unrelated newer head does not prove an earlier receipt&apos;s effects.</p>
    <h3>Verification workflow</h3>
    <ol><li>Decode canonical receipt bytes and authenticate the signature against the pinned receipt key valid at the signed height.</li><li>Match the activity identifier and require the expected success result.</li><li>For payment, compare asset, amount, source, destination, authorization, and context with the intended purchase or transfer.</li><li>Verify the covering block’s receipt coverage and any state proof against the corresponding authenticated root.</li><li>Require the <code>committed</code> rung from <code>pax_sequencerReceipt</code> when finality is required.</li><li>Persist the verified receipt identifier and evidence so retries and reconciliation refer to the original economic result.</li></ol>
    <h2>Unknown outcomes and honest UI states</h2>
    <p>If submission fate cannot be determined, retain the signed transaction and its hash, then query the original receipt before considering a rebroadcast. Display the operation as still checking until verified evidence supports completion. An agent claims a result only after independently verifying node-produced evidence; a local success string is insufficient.</p>
    <h2>Native receipt interfaces</h2>
    <p>The C interface in <code>include/layerx/lxp_receipt.h</code> provides <code>lxp_receipt_encode</code>, <code>lxp_receipt_decode</code>, <code>lxp_receipt_sign</code>, <code>lxp_receipt_verify</code>, and <code>lxp_receipt_digest</code>. Program evidence is bound with <code>lxp_receipt_bind_program_outcome</code> and <code>lxp_receipt_bind_program_artifacts</code>. Supply validation is exposed through <code>lxp_receipt_validate_supply</code>.</p>
    <h2>Continue reading</h2><ul><li><Link href="/activities">Activity submission and recovery</Link></li><li><Link href="/payments">Payments</Link></li><li><Link href="/sequencing">Sequencing</Link></li><li><Link href="/finality">Finality</Link></li><li><Link href="/programs">Program execution evidence</Link></li><li><Link href="/cli">CLI verification</Link></li></ul>
  </DocsLayout>
}
