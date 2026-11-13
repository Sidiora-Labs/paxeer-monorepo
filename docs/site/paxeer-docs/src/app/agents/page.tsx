import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/agents')

export default function Agents() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Agents on Paxeer X</h1><p className="page-description">Give autonomous software a verifiable identity, explicit authority, bounded spending, and an evidence trail for every network action.</p></div>
    <p>Paxeer X agents submit signed EIP-1559 transactions to native precompiles on chain 125. The native kernel executes inside the BFT node. An agent reads state, prepares a transaction, reviews its effects, obtains a signature, and tracks the resulting evidence. The agent daemon supplies tenant isolation, session controls, policy, and runtime integration.</p>
    <h2>Choose an agent integration</h2>
    <table><thead><tr><th>Integration</th><th>Use it for</th><th>Authority path</th></tr></thead><tbody>
      <tr><td>Agent API</td><td>A custom service that needs preparation, signing, submission, reads, approvals, and event subscriptions.</td><td>Tenant-bound daemon session and explicit capability.</td></tr>
      <tr><td>MCP</td><td>Expose network tools to an assistant or development runtime.</td><td>Daemon binding, session, and capability; tools use the Agent API.</td></tr>
      <tr><td>A2A</td><td>Submitting a caller-signed transaction and reading receipts from another agent.</td><td>Hosted gateway key; the caller signs, the runtime never signs.</td></tr>
      <tr><td>Native programs</td><td>Deterministic application logic called by an agent.</td><td>Signed program activities, execution receipts, and program registry evidence.</td></tr>
    </tbody></table>
    <p>Start with <Link href="/agent-runtime">Agent runtime</Link> for daemon enrolment and runtime installation, or <Link href="/programs">Native programs</Link> for application execution.</p>
    <h2>Identity and registration</h2>
    <p>An agent DID identifies the actor. An authority reference identifies the grant under which it acts. A tenant scopes records, credentials, and policy. These are separate from the runtime client name and from the session used to authenticate a request. Registration binds the agent to a verified owner; a self-declared DID alone does not confer permission.</p>
    <table><thead><tr><th>Operation</th><th>Required context</th><th>Purpose</th></tr></thead><tbody>
      <tr><td><code>agent.register</code></td><td>tenant, agent_did, authority_ref, client, policy_version, owner_address, binding_consent, fee_limit</td><td>Register the agent under its verified authority. Returns <code>AuthorityResponse&lt;Prepared&gt;</code>.</td></tr>
      <tr><td><code>session.open</code></td><td>context, grant_id, registration_payload, expiry_sequence, purpose, fee_limit</td><td>Open a tenant-bound authenticated session. Returns <code>AuthorityResponse&lt;Prepared&gt;</code>.</td></tr>
      <tr><td><code>session.refresh</code></td><td>session_id, context</td><td>Refresh the session within its authority and policy.</td></tr>
      <tr><td><code>session.close</code></td><td>session_id, context</td><td>Close access and invalidate session use.</td></tr>
      <tr><td><code>session.list</code></td><td>context</td><td>Inspect sessions visible to the authenticated owner.</td></tr>
    </tbody></table>
    <p>For <code>agent.register</code>, <code>owner_address</code> is the 40-hex EVM address that signs the transaction and <code>binding_consent</code> is the 128-hex owner-signed Ed25519 consent, verified against the chain ID and current bind nonce. For <code>session.open</code>, <code>grant_id</code> and <code>purpose</code> are 64-hex values, <code>registration_payload</code> is the canonical session grant matching the grant ID, and <code>expiry_sequence</code> is a nonzero decimal. The session context itself includes tenant, agent DID, authority reference, permitted activity types, expiry, client, and policy version.</p>
    <h2>Define what an agent may do</h2>
    <p>A capability names every permission dimension explicitly. Missing dimensions are invalid; an empty set denies that dimension. Use <code>capability.create</code> to issue a bounded capability, <code>capability.attenuate</code> to derive a narrower child, <code>capability.list</code> to inspect it, and <code>capability.revoke</code> to withdraw access. Create, attenuate, and revoke each require an <code>idempotency_key</code>; a keyless create is refused before the owner is called. Revoking a capability revokes its whole parent subtree in one durable write, and unsubmitted preparations bound to a revoked record end in <code>Failed</code> with a capability-revocation reason.</p>
    <table><thead><tr><th>Dimension</th><th>Decision it controls</th></tr></thead><tbody>
      <tr><td>activity_types</td><td>Which native operations the agent can prepare.</td></tr>
      <tr><td>counterparties</td><td>Which counterparties may participate.</td></tr>
      <tr><td>assets</td><td>Which assets the agent may use.</td></tr>
      <tr><td>amount_ceilings</td><td>Maximum authorized quantities per asset.</td></tr>
      <tr><td>rate_ceilings</td><td>Action counts within explicit time windows.</td></tr>
      <tr><td>purpose_constraints</td><td>The permitted purpose commitments.</td></tr>
      <tr><td>expiry</td><td>The last permitted authorization period.</td></tr>
    </tbody></table>
    <p>Attenuation never widens authority: child sets are subsets, amount ceilings do not increase, expiry does not move later, and rate windows remain at least as restrictive. Capabilities restrict an existing protocol grant; they do not manufacture a new protocol authority.</p>
    <DocArt name="agent-lifecycle" />
    <h2>The action lifecycle</h2>
    <ol><li><strong>Read.</strong> Obtain account sequence, balance, and relevant module state.</li><li><strong>Prepare.</strong> Supply <code>activity_type</code>, <code>actor</code>, <code>authority</code>, <code>account_sequence</code>, <code>not_before</code>, <code>not_after</code>, <code>idempotency_key</code>, <code>fee_limit</code>, <code>payload</code>, and <code>payload_hash</code> to the daemon wire API.</li><li><strong>Review.</strong> Inspect the disclosure decoded from the unsigned canonical bytes: actor, authority, activity type, counterparties, amounts, asset, fees, expiry, and idempotency key.</li><li><strong>Sign.</strong> Sign the returned transaction preimage with secp256k1 and bind the 65-byte signature to the preparation reference. The wire signature is 130 lowercase hexadecimal characters.</li><li><strong>Submit.</strong> Submit that preparation, signature, and <code>signer_public_key</code>, retaining the submission reference.</li><li><strong>Track.</strong> Use <code>track</code> or <code>wait</code> to observe evidence and the reached rung.</li></ol>
    <pre><code>{`prepare → Prepared { preparation_ref, unsigned_canonical_bytes, signing_preimage, disclosure, expiry }
sign(preparation_ref, signature) → Signed
submit(preparation_ref, signature, signer_public_key) → TrackedSubmission { state, evidence, rung, transitions }
track(submission_ref) → state + evidence + rung
wait(submission_ref, requested_rung, deadline) → actual_rung`}</code></pre>
    <p>Submissions move through Prepared, Signed, Queued, Submitted, Unknown, Executed, Committed, Failed, or Expired. Unknown is a pending outcome that requires reconciliation. Executed and Committed carry a receipt reference. Preserve the original idempotency key while resolving an uncertain submission.</p>
    <h2>Budgets and human approval</h2>
    <p>Budget controls distinguish protocol-enforced budgets from daemon-enforced limits. A daemon limit reserves capacity before an activity proceeds; an unresolved outcome retains its hold until evidence resolves the result. The daemon consumes protocol evidence; it does not mint balances from local accounting. See <Link href="/agent-budgets">agent budgets</Link> for the native budget activities and funding rules.</p>
    <p>Human approval binds an exact prepared activity and its canonical disclosure digest. Approval releases that preparation; rejection releases its reservation; expiry never implies approval. A changed disclosure is defective and requires a new review. Approval is a daemon restriction and does not enlarge protocol authority.</p>
    <h2>Read, observe, and verify</h2>
    <p>Read operations cover balances, accounts, module state, history, batches, checkpoints, and proof bundles. A batch identifies a block height; a checkpoint contains a CometBFT commit and validator set. Portable proof bundles bind the raw transaction, signed execution receipt, EVM receipt inclusion, and CometBFT commit. Availability fetch and offline export support evidence portability. Projection returns an estimate and is distinct from a verified read.</p>
    <p>Evidence is reported as a rung: <code>submitted</code>, <code>receipted</code>, or <code>committed</code>. Submitted is an acknowledgement, receipted verifies execution, and committed verifies inclusion in a committed block. Request the rung the decision needs with <code>wait</code>; when the deadline elapses it returns the rung actually reached rather than an error.</p>
    <p>Subscriptions are scoped by tenant, agent, and capability. Filters narrow agents, accounts, activity types, modules, assets, counterparties, and result classes. Delivery is at least once; deduplicate using <code>deduplication_id</code>, acknowledge cursors, and handle Gap and Truncated events explicitly.</p>
    <h2>Design a reliable agent</h2>
    <ul><li>Use a dedicated identity and the narrowest capability for each workload.</li><li>Require review for actions outside a routine policy; show canonical effects to the approver.</li><li>Separate daemon policy records from cryptographically verified network receipts.</li><li>Retain preparation, submission, activity, and receipt references across process restarts.</li><li>Treat transport loss and deadlines as uncertainty, then reconcile before authorizing a replacement action.</li></ul>
    <div className="source-note"><strong>Reference:</strong> <code>services/agent/schema/agent-api/identity.kvx</code>, <code>write.kvx</code>, <code>stream.kvx</code>, <code>read.kvx</code>, <code>errors.kvx</code>; <code>services/agent/crates/layerx-agentd/src/session.rs</code>, <code>capability/</code>, <code>approval/</code>, <code>budget/</code>.</div>
    <div className="prev-next"><Link href="/agent-runtime"><div className="prev-next-label">Continue</div><div className="prev-next-title">Connect an agent runtime</div></Link><Link href="/programs"><div className="prev-next-label">Explore</div><div className="prev-next-title">Native programs</div></Link></div>
  </DocsLayout>
}
