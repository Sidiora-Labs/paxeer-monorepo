import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/human-interface')

export default function HumanInterface() {
  return <DocsLayout>
    <div className="page-header">
      <h1 className="page-title">Human interface</h1>
      <p className="page-description">Manage your Paxeer X identity, balances, agents, approvals, and recovery through evidence-backed journeys.</p>
    </div>
    <DocArt name="intent-builder" />
    <h2>A human control plane for Paxeer X</h2>
    <p>The Human interface brings the LayerX execution system into an account experience: sign in with a passkey, add funds, create agents, authorize sensitive work, and follow every movement through receipts. Home combines your balance, managed agents, pending approvals, and recent activity.</p>
    <p>Human is custodial by default. Account authentication and protocol authority have separate roles. This separation lets you manage agents without exposing protocol signing keys to the browser. On the merged network, money moves through <code>move</code> journeys; the former wallet-binding, deposit, withdraw, and exit journey kinds are retired and remain readable only as closed history.</p>
    <table><thead><tr><th>Identity</th><th>Purpose</th><th>Authority</th></tr></thead><tbody>
      <tr><td>Application identity</td><td>Passkey authentication and device sessions</td><td>Opens the Human account experience</td></tr>
      <tr><td>Protocol identity</td><td>LayerX DID and account operations</td><td>Custody service holds the primary signing key</td></tr>
    </tbody></table>
    <h2>Onboard and secure an account</h2>
    <ol><li>Create an account with an email and display name.</li><li>Register a passkey using the browser credential ceremony.</li><li>Follow the protocol identity and recovery stages.</li><li>Open the account when its protocol identity has a verified LayerX receipt.</li></ol>
    <p>Onboarding progresses through creating the account, adding the passkey, setting up the protocol identity, and putting recovery in place. Resume continues the existing journey. A local progress indicator alone does not activate an account.</p>
    <table><thead><tr><th>Action</th><th>Human API</th></tr></thead><tbody>
      <tr><td>Create account</td><td><code>POST /v1/accounts</code></td></tr>
      <tr><td>Register passkey</td><td><code>POST /v1/passkeys/registrations</code>, then <code>POST /v1/passkeys/registrations/&#123;registration_id&#125;</code></td></tr>
      <tr><td>Read or resume onboarding</td><td><code>GET /v1/onboarding</code>, <code>POST /v1/onboarding/resume</code></td></tr>
      <tr><td>Sign in</td><td><code>POST /v1/passkeys/assertions</code>, finish the assertion, then <code>POST /v1/sessions</code></td></tr>
      <tr><td>Inspect or refresh sessions</td><td><code>GET /v1/sessions</code>, <code>POST /v1/sessions/refresh</code></td></tr>
    </tbody></table>
    <h3>Wallet binding is retired</h3>
    <p>The former wallet-binding flow (<code>POST /v1/wallet-binding/statement</code>, <code>POST</code> and <code>GET /v1/wallet-binding</code>, and rebind) is retired. Every binding type and operation is marked retired, and stored wallet-binding journeys read back only as closed history; they cannot resume signing or execution. There is no wallet to connect and no binding state to poll. An owner EVM address is bound to the DID during registration instead; see <Link href="/identity">identity</Link>.</p>
    <h2>Create and govern managed agents</h2>
    <p>An agent has a name, purpose, and monthly spending limit. Creation establishes the agent, its protection, and first funding. A separate native fee budget sets maximum fees per activity, total fees, period length, and maximum fees per period; a spending allowance and a fee allowance serve different purposes. On networks enforcing native fee authority version 2, the finite <code>native_fee_budget</code> must be supplied at creation: omitting it refuses creation, and changing or rotating a key never silently replenishes it.</p>
    <pre><code>{`POST /v1/agents
Idempotency-Key: 9f3c1e6a0b8d4f72a5c9e1d3b7a2c4e6

{
  "name": "Research Runner",
  "purpose": "Buys datasets and pays per-use research services",
  "monthly_limit": { "amount": "250000000", "currency": "LXP" },
  "native_fee_budget": { "...": "finite fee budget, required on fee-authority-v2 networks" }
}`}</code></pre>
    <p>The example follows the Human API wire format. Amounts are integer strings; present human-readable values using the asset&apos;s declared precision. Creation returns an <code>agent-create</code> journey.</p>
    <table><thead><tr><th>Control</th><th>Operation</th><th>Result</th></tr></thead><tbody>
      <tr><td>Browse agents</td><td><code>GET /v1/agents</code>; <code>GET /v1/agents/&#123;agent_id&#125;</code></td><td>Agent details and status</td></tr>
      <tr><td>Pause or resume</td><td><code>POST /v1/agents/&#123;agent_id&#125;/pause</code> or <code>/resume</code></td><td>Updated Agent</td></tr>
      <tr><td>Change monthly limit</td><td><code>POST /v1/agents/&#123;agent_id&#125;/limit</code></td><td>Updated Agent</td></tr>
      <tr><td>Fund</td><td>Move quote and commit</td><td>Receipt-backed movement journey</td></tr>
      <tr><td>Reclaim funds</td><td><code>POST /v1/agents/&#123;agent_id&#125;/reclaim</code></td><td>Move journey using return semantics</td></tr>
      <tr><td>Rotate or recover keys</td><td><code>POST /v1/agents/&#123;agent_id&#125;/rotate</code> or <code>/recover</code></td><td>Key challenge</td></tr>
      <tr><td>Archive</td><td><code>POST /v1/agents/&#123;agent_id&#125;/archive</code> with <code>confirm_name</code></td><td>Agent retirement journey</td></tr>
    </tbody></table>
    <p>Owner key rotation adds a disclosure and step-up before starting the rotation window. Retirement stops work and closes the agent. Pause returns the current Agent directly; it does not create a separate journey response.</p>
    <h2>Review approvals before money moves</h2>
    <p>Held activities appear as <code>waiting-for-you</code>. Open the approval to inspect the amount, counterparty, asset, fees, expiry, remaining budget after approval, and supporting evidence. A step-up ceremony confirms the disclosed action before approval. Rejecting resolves the request without approving its movement.</p>
    <ol><li>List pending requests with <code>GET /v1/approvals</code>.</li><li>Read <code>GET /v1/approvals/&#123;approval_id&#125;</code> and review its facts.</li><li>Begin step-up with <code>POST /v1/step-up</code> and finish it with <code>POST /v1/step-up/&#123;challenge_id&#125;</code> using the supplied confirmation digest.</li><li>Submit <code>step_up_evidence</code> to <code>POST /v1/approvals/&#123;approval_id&#125;/approve</code>, or use <code>/reject</code>.</li><li>Read the decision&apos;s <code>money_moved</code> and evidence instead of inferring success from the button press. At decision time <code>money_moved</code> is false; movement is established by the resulting journey and receipt.</li></ol>
    <p>Approval states are pending, approved, rejected, expired, and defective. Expiry and defective evidence prevent a request from being treated as an approved payment.</p>
    <h2>Read progress and proof</h2>
    <p>Journeys expose stages, evidence references, timestamps, and refusals. States are getting-ready, sending, processing, done, done-finalised, still-checking, refused, waiting-for-you, closed, and awaiting-approval. Closed is how retired wallet-binding, deposit, withdraw, and exit history reads back; awaiting-approval marks a journey held for approval and is not receipt-backed completion. A refusal identifies who refused, explanatory copy, money left, and an optional path to change the request.</p>
    <table><thead><tr><th>Verification</th><th>Meaning</th></tr></thead><tbody>
      <tr><td><code>unverified</code></td><td>Submitted; no stronger authenticated execution evidence is attached</td></tr>
      <tr><td><code>receipt-verified</code></td><td>Receipted; the LayerX execution receipt is verified</td></tr>
      <tr><td><code>checkpoint-finalised</code></td><td>Committed; inclusion in a committed block is verified</td></tr>
    </tbody></table>
    <p>The former <code>paxeer-finalised</code> label is retired and defines no further rung. Use <code>GET /v1/journeys</code>, <code>GET /v1/journeys/&#123;journey_id&#125;</code>, and <code>GET /v1/evidence/&#123;evidence_id&#125;</code>. Evidence material includes its class, verification, content type, and base64 bytes. Live updates use <code>POST /v1/stream</code> and <code>GET /v1/stream/&#123;cursor&#125;</code>. These Human verification labels are distinct from RPC commitment names.</p>
    <h2>Recovery, sessions, and support</h2>
    <p>Security settings manage passkeys, individual or all sessions, authenticator enrollment, backup-code rotation, and recovery evidence. Sensitive actions use a disclosure digest and step-up authorization. Revealed recovery evidence is a timed secret with a remask timestamp and copy policy. Protocol keys remain in custody; the single exception is the <code>export-primary-key</code> security action, the only route by which a DID primary key leaves the service.</p>
    <p>Activity search uses <code>POST /v1/activity/query</code>; detail uses <code>GET /v1/activity/&#123;entry_id&#125;</code>. Support conversations accept deposit, withdrawal, agents, account, and report topics through <code>POST /v1/support/conversations</code>.</p>
    <p>Continue with <Link href="/funding">funding, money movement, and withdrawals</Link> or explore <Link href="/agents">agent capabilities</Link>.</p>
  </DocsLayout>
}
