import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/human-interface')

export default function HumanInterface() {
  return <DocsLayout>
    <div className="page-header">
      <h1 className="page-title">Human interface</h1>
      <p className="page-description">Manage your Paxeer identity, balances, agents, approvals, and recovery through receipt-backed journeys.</p>
    </div>
    <DocArt name="intent-builder" />
    <h2>An account interface for Paxeer</h2>
    <p>The Human interface presents the Paxeer network through accounts and journeys: authenticate with a passkey, move funds, manage agents, review approvals, and inspect receipts. Home combines balances, managed agents, pending approvals, and recent activity.</p>
    <p>Self-custody is the default posture; custodial holding requires explicit opt-in. Account authentication and protocol authority have separate roles. The key-export ceremony transfers the primary key once and stops subsequent service signing for that identity. Current money movements use <code>move</code> journeys on the same network.</p>
    <table><thead><tr><th>Identity</th><th>Purpose</th><th>Authority</th></tr></thead><tbody>
      <tr><td>Application identity</td><td>Passkey authentication and device sessions</td><td>Opens the Human account experience</td></tr>
      <tr><td>Protocol identity</td><td>DID and account operations</td><td>Self-custody or explicitly selected custodial authority</td></tr>
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
    <h3>Protocol ownership</h3>
    <p>Onboarding binds protocol identity through the address precompile and verifies the resulting receipt. Passkeys authorize application access; protocol authority controls signed operations. See <Link href="/identity">identity</Link> for these roles.</p>
    <h2>Create and govern managed agents</h2>
    <p>Agent creation asks for a name, purpose, and monthly spending limit. Each limit identifies whether enforcement is <code>protocol</code> or <code>app</code>. A separate native fee budget sets maximum fees per activity, total fees, period length, and maximum fees per period. Networks enforcing fee-authority version 2 require a finite <code>native_fee_budget</code>; changing or rotating a key does not replenish it.</p>
    <pre><code>{`POST /v1/agents
Idempotency-Key: 9f3c1e6a0b8d4f72a5c9e1d3b7a2c4e6

{
  "name": "Research Runner",
  "purpose": "Buys datasets and pays per-use research services",
  "monthly_limit": { "amount": "250000000", "currency": "LXP" },
  "native_fee_budget": {
    "asset_id": "<asset_id-from-fee-policy>",
    "maximum_per_activity": "1000000000000000",
    "maximum_total": "100000000000000000",
    "period_length_ms": "86400000",
    "maximum_per_period": "10000000000000000"
  }
}`}</code></pre>
    <p>This example follows <code>services/human/schema/human-api/agents.kvx</code>. Substitute <code>asset_id</code> from <code>GET /v1/sessions/fee-policy</code>. Amounts are integer strings in the declared asset units. The positive per-activity limit cannot exceed the total; a nonzero period requires a period limit at least as large as the per-activity limit. Creation returns an <code>agent-create</code> journey.</p>
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
    <p>Journeys expose stages, evidence references, timestamps, and refusals. States include getting-ready, sending, processing, done, done-finalised, still-checking, refused, waiting-for-you, closed, and awaiting-approval. A journey held for approval has no completed movement. A refusal identifies its authority, explanatory copy, whether money left, and an optional path to change the request.</p>
    <table><thead><tr><th>Verification</th><th>Meaning</th></tr></thead><tbody>
      <tr><td><code>unverified</code></td><td>Submitted; no stronger authenticated execution evidence is attached</td></tr>
      <tr><td><code>receipt-verified</code></td><td>Receipted; the execution receipt is verified</td></tr>
      <tr><td><code>checkpoint-finalised</code></td><td>Committed; inclusion in a committed block is verified</td></tr>
    </tbody></table>
    <p>Read <code>GET /v1/journeys</code>, <code>GET /v1/journeys/&#123;journey_id&#125;</code>, and <code>GET /v1/evidence/&#123;evidence_id&#125;</code>. Evidence material includes its class, verification, content type, base64 bytes, and optional progress rung and inclusion height. Verify the actual material and its trusted key bindings. Live updates use <code>POST /v1/stream</code> and <code>GET /v1/stream/&#123;cursor&#125;</code>.</p>
    <h2>Recovery, sessions, and support</h2>
    <p>Security settings manage passkeys, sessions, authenticators, backup codes, and recovery evidence. Sensitive actions require a server disclosure digest and matching step-up evidence. Recovery evidence has a remask time and copy policy. Primary-key export uses <code>POST /v1/security/key-export/begin</code> and <code>POST /v1/security/key-export/finish</code>; it returns the key once and leaves the service unable to sign for that identity.</p>
    <p>Activity search uses <code>POST /v1/activity/query</code>; detail uses <code>GET /v1/activity/&#123;entry_id&#125;</code>. Support conversations accept deposit, withdrawal, agents, account, and report topics through <code>POST /v1/support/conversations</code>.</p>
    <p>Continue with <Link href="/funding">funding and money movement</Link> or explore <Link href="/agents">agent capabilities</Link>.</p>
  </DocsLayout>
}
