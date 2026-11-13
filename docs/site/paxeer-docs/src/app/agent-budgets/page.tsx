import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/agent-budgets')

export default function AgentBudgetsPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Agent Budgets and Approvals</h1><p className="page-description">Fund bounded agent activity, delegate spending authority, review exact actions and reconcile verified results.</p></div>
    <h2>A funded ceiling for autonomous work</h2>
    <p>Paxeer X budgets give an owner a bounded way to fund delegated work. A budget binds an account, asset, purpose, funding amount, period allowance and expiry. Delegates receive spending authority within that ceiling. The owner can change policy, remove delegates or close the budget as the assignment changes.</p>
    <p>A budget is a funded ceiling over spending. Account balances, capability permissions, approval holds and budget allowances remain distinct controls. An agent must satisfy every applicable control before its action proceeds.</p>
    <h2>Choose the enforcement boundary</h2>
    <table><thead><tr><th>Control</th><th>Enforcement</th><th>Use</th></tr></thead><tbody>
      <tr><td><code>ProtocolBudget</code></td><td>Native protocol validation.</td><td>Funded spending ceilings and delegated authority checked during execution.</td></tr>
      <tr><td><code>DaemonLimit</code></td><td>The agent daemon.</td><td>Local automation limits for activity routed through that daemon.</td></tr>
      <tr><td>Capability</td><td>Scoped agent authorization.</td><td>Activity, asset, counterparty, amount, rate, purpose and expiry restrictions.</td></tr>
      <tr><td>Approval hold</td><td>The daemon preparation workflow.</td><td>Human review of a particular disclosed activity before release.</td></tr>
    </tbody></table>
    <p>Route all relevant traffic through the daemon when relying on a daemon limit or approval hold: bypassing that daemon bypasses those controls. Approval releases a prepared action and grants no additional protocol authority.</p>
    <h2>Configure a budget</h2>
    <table><thead><tr><th>Field</th><th>Meaning</th></tr></thead><tbody>
      <tr><td><code>budget_id</code>, <code>budget_account</code></td><td>32-byte budget identity and account binding.</td></tr>
      <tr><td><code>asset_id</code>, <code>purpose_hash</code></td><td>The funded asset and committed purpose reference.</td></tr>
      <tr><td><code>amount</code></td><td>Initial funding amount.</td></tr>
      <tr><td><code>per_period_limit</code></td><td>Maximum permitted spend in a budget period.</td></tr>
      <tr><td><code>period_length</code>, <code>period_start</code></td><td>Period duration and initial period boundary.</td></tr>
      <tr><td><code>rollover_policy</code>, <code>carry_cap</code></td><td>No rollover, or capped carry-forward of allowance.</td></tr>
      <tr><td><code>expiry</code>, <code>revocation_sequence</code></td><td>Lifetime and revocation generation.</td></tr>
      <tr><td><code>source_account</code>, <code>source_sequence</code></td><td>Version 2 funding source and its replay-protected debit sequence.</td></tr>
    </tbody></table>
    <p>Amounts and limits use unsigned 128-bit values at the protocol boundary. Agent API amounts, sequences, limits and timestamps travel as decimal strings, preserving exact values across clients. Native budgets support up to 16 delegates. The rollover constants are <code>LX_BUDGET_ROLLOVER_NONE = 1</code> and <code>LX_BUDGET_ROLLOVER_CAPPED = 2</code>.</p>
    <h3>Example policy</h3>
    <p>Consider a procurement agent with 1,000 asset units funded, a 200-unit period ceiling, no rollover and a fixed expiry. A 60-unit purchase consumes both funding and current-period allowance. Adding funding increases available funds; it does not remove the period ceiling. A second delegate shares the budget ceiling rather than receiving an independent copy of it.</p>
    <p>Choose asset-unit scaling before encoding values. With six decimal places, 60 units encodes as <code>60000000</code>. The example describes policy values; obtain the asset precision from its registered metadata.</p>
    <h2>Native budget activities</h2>
    <p>Budget is module <code>3</code>. Its ten activity types occupy <code>0x0003xxxx</code>. Canonical payloads begin with a big-endian two-byte version prefix.</p>
    <table><thead><tr><th>Activity</th><th>Type</th><th>Purpose</th></tr></thead><tbody>
      <tr><td><code>LX_BUDGET_CREATE</code></td><td><code>0x00030001</code></td><td>Create a funded budget and its period policy.</td></tr>
      <tr><td><code>LX_BUDGET_FUND</code></td><td><code>0x00030002</code></td><td>Add funds, preserving policy limits.</td></tr>
      <tr><td><code>LX_BUDGET_AMEND</code></td><td><code>0x00030003</code></td><td>Amend period limit, carry cap, expiry and rollover.</td></tr>
      <tr><td><code>LX_BUDGET_DELEGATE_ADD</code></td><td><code>0x00030004</code></td><td>Add a permitted delegate.</td></tr>
      <tr><td><code>LX_BUDGET_DELEGATE_REMOVE</code></td><td><code>0x00030005</code></td><td>Remove a delegate.</td></tr>
      <tr><td><code>LX_BUDGET_SPEND</code></td><td><code>0x00030006</code></td><td>Transfer the specified amount to a recipient within the budget.</td></tr>
      <tr><td><code>LX_BUDGET_CLOSE</code></td><td><code>0x00030007</code></td><td>Close using the required revocation sequence.</td></tr>
      <tr><td><code>LX_BUDGET_DEFUND</code></td><td><code>0x00030008</code></td><td>Withdraw funding from a budget. Payload: 50 bytes.</td></tr>
      <tr><td><code>LX_BUDGET_REVOKE</code></td><td><code>0x00030009</code></td><td>Revoke a budget. Payload: 42 bytes.</td></tr>
      <tr><td><code>LX_BUDGET_DELEGATE_SPEND</code></td><td><code>0x0003000a</code></td><td>Spend as a delegate under a per-spend and total delegate capability. Payload: 114 bytes (version, budget, recipient, amount, and a 32-byte idempotency key).</td></tr>
    </tbody></table>
    <h3>Canonical spend payload</h3>
    <pre><code>{`version:u16be = 1
budget_id:bytes32
recipient:bytes32
amount:u128be

Total: 82 bytes`}</code></pre>
    <p>Create supports version 1 at 211 bytes and version 2 at 251 bytes. Fund supports version 1 at 50 bytes and version 2 at 58 bytes. Version 2 create appends the source account and source sequence; version 2 fund appends the source sequence. Period maintenance uses the protocol batch timestamp to roll due periods.</p>
    <h2>Agent API workflow</h2>
    <ol><li>Use <code>budget.create</code> with the intended enforcement mode and establish the permitted agent capability. A <code>ProtocolBudget</code> create requires the signed <code>BudgetAuthorization</code>; that authorization is refused for a <code>DaemonLimit</code>.</li><li>Use <code>budget.fund</code> to add funds to a protocol budget. Funding requires the signed authorization carrying its preparation reference and signature; funding a <code>DaemonLimit</code> is refused, because a daemon limit holds no funds.</li><li>Inspect state with <code>budget.reconciliation</code> before scheduling new spending. Note that the served <code>budget.list</code> adapter currently answers a bare array rather than <code>AuthorityResponse&lt;BudgetRecords&gt;</code>, which SDKs refuse as a decode violation; do not rely on it until it answers the wrapped shape.</li><li>Prepare the action with actor, authority, account sequence, timestamp bound, idempotency key, fee limit, payload and payload hash.</li><li>Review the returned disclosure, then sign and submit the exact preparation.</li><li>Track the submission and reconcile its verified receipt with the budget and account state.</li><li>Use <code>budget.revoke</code> and capability revocation when the assignment ends.</li></ol>
    <p>Protocol spending uses the native budget activity and protocol validation. Daemon budget objects carry their enforcement mode explicitly. Keep receipt evidence attached to reconciled spending so local scheduling decisions follow committed results.</p>
    <h2>Approve the exact action</h2>
    <p>An approval record contains <code>approval_id</code>, tenant, the held structured activity disclosure, canonical bytes digest, hold reason, creation time, expiry and state. Review actor, authority, counterparties, amounts, asset, fee limit, expiry and idempotency key alongside the canonical digest.</p>
    <table><thead><tr><th>Operation</th><th>Behavior</th></tr></thead><tbody>
      <tr><td><code>approval.list</code></td><td>Find approval records in the authorized scope.</td></tr>
      <tr><td><code>approval.get</code></td><td>Inspect one held action and its disclosure.</td></tr>
      <tr><td><code>approval.approve</code></td><td>Release the exact preparation bound to the reviewed digest.</td></tr>
      <tr><td><code>approval.reject</code></td><td>Reject the held action.</td></tr>
    </tbody></table>
    <p>Decision operations carry an <code>idempotency_key</code>. Approval states are <code>Held</code>, <code>Granted</code>, <code>Rejected</code>, <code>Expired</code> and <code>Defective</code>; all except Held are terminal. A modified activity requires its own preparation and review. Approval does not override a missing delegate grant, expired capability or exhausted budget.</p>
    <h2>Reconcile before retrying</h2>
    <p>A submission progresses through Prepared, Signed, Queued, and Submitted before its final result; there is no Acknowledged state. <code>Unknown</code> is a pending state requiring reconciliation. Executed and Committed results carry a receipt reference. Use <code>track</code> or <code>wait</code> to inspect evidence and the rung actually reached (<code>submitted</code>, <code>receipted</code>, or <code>committed</code>) before making a new economic attempt.</p>
    <h3>Common refusals</h3>
    <table><thead><tr><th>Refusal</th><th>Response</th></tr></thead><tbody>
      <tr><td><code>LXP_ERR_BUDGET_PERIOD_CAP</code>, <code>LXP_ERR_BUDGET_ALLOWANCE_EXCEEDED</code></td><td>Inspect period policy and remaining allowance; amend only with owner authority.</td></tr>
      <tr><td><code>LXP_ERR_INSUFFICIENT_BUDGET_FUNDS</code></td><td>Reconcile funds and fund the budget if appropriate.</td></tr>
      <tr><td><code>LXP_ERR_UNAUTHORIZED_DELEGATE</code>, <code>LXP_ERR_UNAUTHORIZED_DEBIT</code></td><td>Check delegate, owner and funding-source bindings.</td></tr>
      <tr><td><code>LXP_ERR_BUDGET_REVOKED</code>, <code>LXP_ERR_EXPIRED</code></td><td>Stop spending under that authority.</td></tr>
      <tr><td><code>LXP_ERR_STALE_REVOCATION</code>, <code>LXP_ERR_SEQUENCE_REUSED</code>, <code>LXP_ERR_SEQUENCE_GAP</code></td><td>Refresh verified revocation and sequence state before preparing again.</td></tr>
    </tbody></table>
    <div className="source-note"><strong>Technical references:</strong> <code>include/layerx/lx_budget.h</code>; <code>src/modules/budget/</code>; <code>services/agent/schema/agent-api/</code> budget, write, identity and approval contracts.</div>
    <p>See <Link href="/identity">identity and delegated credentials</Link> for owner authority, session revocation and recovery.</p>
  </DocsLayout>
}
