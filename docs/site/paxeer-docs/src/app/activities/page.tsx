import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import Link from 'next/link'

export const metadata = metadataFor('/activities')

export default function Activities() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Activities</h1><p className="page-description">The signed, canonical unit of agent action across the Paxeer X execution system.</p></div>
    <p>A payment, escrow capture, budget spend, stream draw, service delivery, trade, governance action, bridge claim, or program call enters LayerX as one signed activity. The kernel verifies the envelope and authority, orders admitted work globally, executes the operation, and produces protocol-computed receipt evidence. An EVM transaction and an activity are distinct operation types on the same network. Kernel state changes commit with the node&apos;s blocks.</p>
    <DocArt name="request-envelope" />
    <h2>The exact envelope</h2>
    <p>The <code>lxp_activity</code> structure carries exactly twelve fields. Omitting, repeating, or introducing an undeclared field makes an envelope malformed. LXC/1 uses canonical binary encoding, fixed-width big-endian integers, and no floating-point values or unordered maps. Trailing bytes are refused.</p>
    <table><thead><tr><th>Field</th><th>Meaning</th></tr></thead><tbody>
      <tr><td><code>protocol_version</code></td><td>The enabled execution rules. Version 1 is legacy, 2 includes occupancy, and 3 includes state commitment.</td></tr>
      <tr><td><code>network_id</code></td><td>The exact execution network; prevents cross-network replay.</td></tr>
      <tr><td><code>activity_type</code></td><td>A 32-bit module and operation discriminator.</td></tr>
      <tr><td><code>actor_did</code></td><td>The identity whose authority and sequence authorize the action.</td></tr>
      <tr><td><code>authority</code></td><td>A primary-key, session-key, or scoped-grant authorization reference.</td></tr>
      <tr><td><code>account_sequence</code></td><td>The actor identity’s exact next expected counter.</td></tr>
      <tr><td><code>timestamp_bound</code></td><td><code>not_before</code> and <code>not_after</code> bounds checked against batch time.</td></tr>
      <tr><td><code>idempotency_key</code></td><td>A 32-byte actor-scoped key binding retries to one economic result.</td></tr>
      <tr><td><code>fee_limit</code></td><td>The maximum authorized deterministic fee, represented as <code>lxp_u128</code>.</td></tr>
      <tr><td><code>payload_hash</code></td><td>A 32-byte domain-separated commitment to the canonical payload.</td></tr>
      <tr><td><code>payload</code></td><td>The module’s canonical operation body.</td></tr>
      <tr><td><code>signature</code></td><td>Ed25519 authorization over the canonical envelope signing preimage, excluding this field.</td></tr>
    </tbody></table>
    <h3>Module and ordinal</h3>
    <pre><code>{`activity_type = (module_id << 16) | operation_ordinal
module_id = lxp_activity_module_id(activity_type)
operation_ordinal = lxp_activity_type_ordinal(activity_type)

Programs module: 0x09
Governance migration budget: 0x0007000a
Governance programs fee:     0x0007000b`}</code></pre>
    <p>The ordinal is module-specific. Use the operation encoding for the target module rather than treating every payload as a transfer. The governance header defines only the two activity types above for module <code>0x0007</code>. The compile-time <code>LXP_PROTOCOL_VERSION</code> default is 2; applications explicitly select the protocol version enabled by their execution environment.</p>
    <h2>Admission and execution</h2>
    <ol><li>Check protocol version, network, and payload binding in that order.</li><li>Resolve the actor’s current authority and verify its signature.</li><li>Enforce the exact account sequence and the batch-time validity window.</li><li>Resolve idempotency and compute fees against the active schedule and fee limit.</li><li>Validate and execute the module operation within the state journal.</li><li>Commit successful effects or roll back failed module effects; produce the applicable receipt and bookkeeping.</li></ol>
    <table><thead><tr><th>Outcome</th><th>Application consequence</th></tr></thead><tbody>
      <tr><td>Malformed or wrong-network envelope</td><td>No admission, sequence consumption, or execution fee.</td></tr>
      <tr><td>Payload binding mismatch</td><td>Refused before authority evaluation.</td></tr>
      <tr><td>Sequence too low or too high</td><td>Replay or gap refusal. Recover the actor’s current identity counter before preparing new work.</td></tr>
      <tr><td>Fee exceeds the limit</td><td>The requested effects are not applied.</td></tr>
      <tr><td>Admitted execution failure</td><td>Module effects roll back; the sequence, global position, and applicable fee bookkeeping persist.</td></tr>
      <tr><td>Idempotent retry</td><td>The original receipt returns without a second economic effect.</td></tr>
    </tbody></table>
    <h2>Submitting and recovering</h2>
    <p>The hosted gateway does not serve <code>lx_*</code> methods. It answers <code>-32601 Method not found</code> for every <code>lx_*</code> method. Do not build submission or recovery flows on <code>lx_sendActivity</code>, <code>lx_getActivityStatus</code>, or <code>lx_getReceipt</code>. The live hosted gateway API is the <code>px_</code> family defined by its OpenRPC contract, including <code>px_register</code>, <code>px_resolveAccount</code>, <code>px_getReceiptKeys</code>, and <code>px_getSequenceBatch</code>. Obtain its deployment URL and credentials from the operator. Public nodes expose <code>pax_sequencerReceipt</code> for receipt progress.</p>
    <ol><li>Prepare the module payload and exact envelope; retain the idempotency key.</li><li>Sign the canonical preimage with currently valid authority.</li><li>Submit the same canonical bytes through the submission path your operator provides, and retain the activity identifier.</li><li>Check the verified receipt’s result code before reporting success.</li><li>Track finality with <code>pax_sequencerReceipt</code>: <code>submitted</code>, then <code>receipted</code>, then <code>committed</code>. A receipt is final only at <code>committed</code>. If the outcome is unknown, recover the original result instead of creating a replacement economic action.</li></ol>
    <h3>Keep the two sequence systems separate</h3>
    <p>The envelope’s account sequence belongs to the actor DID. Transfer sets also carry an actor sequence for their designated sequence account, usually the debited asset record. A per-asset account can start at zero while its owner identity already has a long activity history. Read the correct counter for each protocol structure.</p>
    <h2>Native codec interfaces</h2>
    <p><code>lxp_activity_encode</code> and <code>lxp_activity_decode</code> handle canonical bytes; <code>lxp_activity_id</code> identifies those bytes; <code>lxp_activity_signing_preimage</code> produces the signing commitment. Envelope, payload-hash, and signature verification are exposed through <code>lxp_activity_check_envelope</code>, <code>lxp_activity_verify_payload_hash</code>, and <code>lxp_activity_verify_signature</code>.</p>
    <h2>Continue reading</h2><ul><li><Link href="/accounts">Accounts and sequence ownership</Link></li><li><Link href="/identity">Identity and delegated authority</Link></li><li><Link href="/receipts">Receipts and result verification</Link></li><li><Link href="/layerx-kernel">Kernel execution</Link></li><li><Link href="/modules">Module operations</Link></li></ul>
  </DocsLayout>
}
