import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/subscriptions')

export default function Subscriptions() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Event subscriptions</h1><p className="page-description">Build durable agent consumers with scoped visibility, acknowledged cursors, explicit gap recovery, and verified receipt references.</p></div>
    <p>Paxeer X subscriptions let an agent observe activity without repeatedly rebuilding its view from individual queries. The contract persists the subscription definition, acknowledgement position, continuity state, and lifecycle controls in the agent daemon. An event carries its identity, bytes, deduplication identifier, cursor, and an optional verified receipt reference.</p>
    <p>The Agent API routes <code>subscription.create</code>, <code>subscription.list</code> and <code>subscription.health</code> to the daemon&apos;s durable subscription store. Creation uses an <em>idempotent mutation</em>; list and health reads require the authenticated session&apos;s operation permit.</p>
    <DocArt name="event-subscriptions" />
    <h2>Choose the event interface</h2>
    <table><thead><tr><th>Interface</th><th>Consumer model</th><th>Recovery mechanism</th></tr></thead><tbody>
      <tr><td>Agent subscriptions</td><td>Authenticated tenant, agent, and capability; explicit filter and delivery target.</td><td>Durable acknowledged cursor, backfill barrier, truncation notice.</td></tr>
      <tr><td><Link href="/webhooks">Hosted webhooks</Link></td><td>Developer HTTPS endpoints for journey, payment, approval, and program families.</td><td>Signed delivery, retries, dead letters, and signed replay cursors.</td></tr>
    </tbody></table>
    <p>Choose the agent subscription interface when an autonomous runtime needs visibility constrained by its session capability. Choose webhooks when an external application receives hosted event families over HTTPS. Their cursor and verification representations belong to their respective interfaces.</p>
    <h2>Create a scoped subscription</h2>
    <p><code>subscription.create</code> requires <code>scope</code>, <code>filter</code>, <code>start</code>, and <code>delivery_target</code>, inside an idempotent mutation. The scope names tenant, agent, and capability and must equal the authenticated owner coordinates. The same key and request return the original record; a changed request under that key is refused.</p>
    <table><thead><tr><th>Filter field</th><th>Restriction</th></tr></thead><tbody>
      <tr><td>agents</td><td>Selected agent identities.</td></tr><tr><td>accounts</td><td>Selected accounts visible to the owner.</td></tr><tr><td>activity_types</td><td>Selected native operation types.</td></tr><tr><td>modules</td><td>Selected module activity.</td></tr><tr><td>assets</td><td>Selected assets.</td></tr><tr><td>counterparties</td><td>Selected interacting principals.</td></tr><tr><td>result_classes</td><td>Selected outcome classes.</td></tr>
    </tbody></table>
    <p>Evaluation applies tenant restriction first, capability scope second, and filter third. A filter narrows visibility and cannot expand a capability or disclose another tenant&apos;s records. Administrative list and health operations also require the authenticated session&apos;s operation permit.</p>
    <h2>Consume and acknowledge</h2>
    <ol><li>Restore the persisted subscription and last acknowledged cursor.</li><li>Receive an Event, Gap, or Truncated delivery.</li><li>For an Event, claim its <code>deduplication_id</code> in your durable consumer store.</li><li>Verify any receipt evidence required for the business decision.</li><li>Commit the application effect and deduplication record together.</li><li>Acknowledge the delivered cursor after that commit.</li></ol>
    <pre><code>{`EventDelivery
  event_identity
  event_bytes
  deduplication_id
  cursor
  receipt_reference: None | Verified { receipt_ref, rung }

cursor = { block_height: "125", tx_index: "0", log_index: "0" }

subscription.acknowledge(scope, subscription_id, cursor)`}</code></pre>
    <p>Delivery is at least once. An acknowledgement records progress; it does not make a business handler transactional. If the process crashes after applying an effect but before acknowledging, the repeated delivery must find the completed deduplication record. Each cursor is an object with block_height, tx_index, and log_index fields encoded as canonical decimal strings. Preserve all three coordinates when saving or acknowledging progress.</p>
    <p>The daemon refuses acknowledgements for never-delivered cursors and rejects regressing positions. Keep processing and acknowledgement ordered for a subscription instead of advancing past unfinished work.</p>
    <h2>Resolve gaps before advancing</h2>
    <p>A Gap contains <code>missing_first</code>, <code>missing_last</code>, <code>backfill_cursor</code>, and <code>backfill_attempted</code>. It forms a continuity barrier: later events remain blocked until successful backfill or an explicit truncation notice. Persist that blocked state so a restart cannot accidentally cross the missing interval.</p>
    <p>If retention no longer contains the requested interval, Truncated reports <code>requested_first</code>, <code>oldest_available</code>, and <code>resume_cursor</code>. Rebuild the application view from authoritative state or retained history, record the discontinuity, and continue from the declared resume point. Treat truncation as an explicit loss of continuity.</p>
    <h2>Pause, resume, and inspect health</h2>
    <table><thead><tr><th>Operation</th><th>Required fields</th><th>Use</th></tr></thead><tbody>
      <tr><td><code>subscription.list</code></td><td>scope</td><td>Find subscriptions visible to the authenticated owner.</td></tr>
      <tr><td><code>subscription.health</code></td><td>scope, subscription_id</td><td>Inspect acknowledged progress and pending backfill.</td></tr>
      <tr><td><code>subscription.acknowledge</code></td><td>scope, subscription_id, cursor</td><td>Persist completed delivery progress.</td></tr>
      <tr><td><code>subscription.pause</code></td><td>scope, subscription_id</td><td>Stop delivery temporarily while preserving position.</td></tr>
      <tr><td><code>subscription.resume</code></td><td>scope, subscription_id</td><td>Continue under current authorization and continuity controls.</td></tr>
      <tr><td><code>subscription.delete</code></td><td>scope, subscription_id</td><td>Terminate delivery while retaining its audit record.</td></tr>
    </tbody></table>
    <p>Health returns the target, <code>last_acknowledged</code>, <code>last_delivery_at</code>, and <code>pending_backfill</code>. The current daemon returns null for the delivery timestamp. Pending backfill reports the missing sequence range, including truncated continuity over the subscription&apos;s own coordinates.</p>
    <h2>Revocation and restart safety</h2>
    <p>Session, capability, or tenant revocation stops the corresponding subscription. Durable termination distinguishes Deleted, SessionRevoked, CapabilityRevoked, and TenantRevoked. Session-bound access continues to require an authorized operation rather than relying on possession of a subscription identifier.</p>
    <p>Restart restores definition, acknowledged position, unacknowledged deliveries, and continuity. Preserve the daemon store together with its tenant and session context. A corrupt or unavailable store refuses progress instead of silently resetting the cursor.</p>
    <h2>Separate notification from proof</h2>
    <p>An event may carry no receipt reference. A verified reference names its receipt and rung with Paxeer as the settlement domain. Event arrival alone does not establish an economic outcome. The Agent API reports <code>submitted</code>, <code>receipted</code> and <code>committed</code>; a verified receipt reference requires at least receipted. For settlement-sensitive automation, retrieve the evidence and require committed block evidence.</p>
    <p>Keep local approval and policy notifications distinguishable from protocol receipts. See <Link href="/agents">Agents on Paxeer X</Link> for the write lifecycle and <Link href="/agent-runtime">Agent runtime</Link> for capability, readiness, and recovery controls.</p>
    <div className="source-note"><strong>Reference:</strong> <code>services/agent/crates/layerx-agent-api/src/stream.rs</code> and <code>services/agent/crates/layerx-agentd/src/agent_rpc_adapters.rs</code>; <code>services/agent/crates/layerx-agentd/src/events/subscription.rs</code> in the monorepo.</div>
  </DocsLayout>
}
