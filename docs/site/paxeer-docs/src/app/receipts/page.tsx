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
      <tr><td>State transition</td><td><code>previous_state_root</code>, <code>resulting_state_root</code>, <code>activity_root</code></td><td>The committed state chain and activity binding.</td></tr>
      <tr><td>Outcome</td><td><code>result_code</code>, <code>effects</code>, <code>fee_charged</code></td><td>Success or refusal, computed effects, and execution charge.</td></tr>
      <tr><td>Rules</td><td><code>protocol_version</code>, <code>module_id</code>, <code>module_version</code>, <code>parameter_version</code></td><td>The execution rules needed for independent interpretation and replay.</td></tr>
      <tr><td>Authorization</td><td><code>authorization_hash</code>, <code>context_hash</code>, <code>sequencer_signature</code></td><td>The authorization and context commitments authenticated by the sequencer.</td></tr>
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
    <p>A queue acknowledgement, HTTP 202, or activity identifier establishes neither execution nor finality. A successful submission or signed receipt alone must not be treated as a committed result. The external checkpoint registry, bonded guarantor attestations, and L0–L4 settlement ladder are retired. Archived certificates describe that former protocol.</p>
    <p>The agent proof crate uses its own rank lattice for portable verification: ranks 0–5, <code>Unverified</code>, <code>SequencerSigned</code>, <code>BatchIncluded</code>, <code>StateProven</code>, <code>CheckpointFinalised</code>, and <code>SettlementAnchored</code>. <code>CheckpointFinalised</code> maps to the <code>committed</code> rung. Do not confuse these crate ranks with a live checkpoint process on the network.</p>
    <h2>Independent verification</h2>
    <table><thead><tr><th>Surface</th><th>Use</th></tr></thead><tbody>
      <tr><td><code>layerx-proof</code></td><td>Verify signed receipt bytes.</td></tr>
      <tr><td><code>layerx receipt verify</code></td><td>Perform a local check against caller-supplied batch facts.</td></tr>
      <tr><td><code>layerx-portable</code></td><td>Export and verify a <code>layerx-receipt-proof-v1</code> JSON object against independently trusted <code>AuthorizedBatch</code> facts.</td></tr>
    </tbody></table>
    <p>Portable verification runs without a node, gateway, daemon, database, clock, or network. Its trust anchor is the independently trusted authorized batch. Merely packaging a receipt with a claimed signer does not make that signer authoritative.</p>
    <p>For block-level verification, authenticate the block header and CometBFT commit, then verify transaction or state inclusion against the corresponding authenticated root. State proofs must name the relevant height and key. An unrelated newer head does not prove an earlier receipt&apos;s effects.</p>
    <h3>Verification workflow</h3>
    <ol><li>Decode canonical receipt bytes and authenticate the signature against the expected authorized sequencer.</li><li>Match the activity identifier and require the expected success result.</li><li>For payment, compare asset, amount, source, destination, authorization, and context with the intended purchase or transfer.</li><li>Verify state-root binding and the exact receipt inclusion proof when batch assurance is required.</li><li>Require the <code>committed</code> rung from <code>pax_sequencerReceipt</code> when finality is required.</li><li>Persist the verified receipt identifier and evidence so retries and reconciliation refer to the original economic result.</li></ol>
    <h2>Unknown outcomes and honest UI states</h2>
    <p>If submission fate cannot be determined, retain the signed activity and its idempotency key and recover the original receipt. Display the operation as still checking until verified evidence supports completion. An agent claims a result only after independently verifying core-produced evidence; a local success string is insufficient.</p>
    <h2>Native receipt interfaces</h2>
    <p>The C interface provides <code>lxp_receipt_encode</code>, <code>lxp_receipt_decode</code>, <code>lxp_receipt_sign</code>, <code>lxp_receipt_verify</code>, and <code>lxp_receipt_digest</code>. Program evidence is bound with <code>lxp_receipt_bind_program_outcome</code> and <code>lxp_receipt_bind_program_artifacts</code>. Supply validation is exposed through <code>lxp_receipt_validate_supply</code>.</p>
    <h2>Continue reading</h2><ul><li><Link href="/activities">Activity submission and recovery</Link></li><li><Link href="/payments">Payments</Link></li><li><Link href="/sequencing">Sequencing</Link></li><li><Link href="/finality">Finality</Link></li><li><Link href="/programs">Program execution evidence</Link></li><li><Link href="/cli">CLI verification</Link></li></ul>
  </DocsLayout>
}
