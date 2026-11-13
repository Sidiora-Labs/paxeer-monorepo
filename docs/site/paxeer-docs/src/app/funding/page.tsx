import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/funding')

export default function Funding() {
  return <DocsLayout>
    <div className="page-header">
      <h1 className="page-title">Funding and money movement</h1>
      <p className="page-description">Move balances, fund agents, and connect external payment rails on the merged network. Former custody-boundary deposit, withdraw, and exit journeys are retired.</p>
    </div>
    <h2>Understand the boundary</h2>
    <p>Current money movement on the merged network is the internal <code>move</code> journey: fund, allocate, return, or transfer between LayerX accounts, including the EVM intent path. The former custody-boundary journeys — deposit, withdraw, exit, and wallet-binding — are retired kinds. They are retained only for decoding stored history and cannot start new journeys. Retained history of those kinds reads as <code>closed</code>, cannot resume signing or execution, and does not assert a successful movement.</p>
    <table><thead><tr><th>Journey</th><th>Status</th><th>Authorization and evidence</th></tr></thead><tbody>
      <tr><td>Move</td><td>Current</td><td>Quote, commit, execution receipt</td></tr>
      <tr><td>Deposit</td><td>Retired — history only</td><td>Formerly wallet transaction and custody proof</td></tr>
      <tr><td>Withdraw</td><td>Retired — history only</td><td>Formerly withdrawal authorization and claim signature</td></tr>
      <tr><td>Exit</td><td>Retired — history only</td><td>Formerly eligibility, explicit confirmation, wallet claim</td></tr>
    </tbody></table>
    <p>The <code>settlement_domain</code> field is likewise retired; internal moves do not select a settlement domain. Money-moving mutations require an <code>Idempotency-Key</code>. Retain the key when retrying the same intended operation, and reconcile its journey before starting another movement.</p>
    <h2>Retired custody-boundary journeys</h2>
    <p><code>POST /v1/deposits</code> (and its <code>/confirm</code> step), <code>POST /v1/withdrawals</code> (and its <code>/claim</code> step), <code>POST /v1/exit</code>, and <code>GET /v1/exit/eligibility</code> remain in the Human API schema as retired operations. Do not build new deposit, withdrawal, or exit flows on them. The on-chain custody path they relied on is also retired: the custody keeper refuses deposits and the custody precompile reverts. New integrations fund and move balances with the move flow below, on accounts that already hold spendable balances on the merged network.</p>
    <h2>Move money and fund an agent</h2>
    <p>Quote the source, destination, and amount first. The resolver selects <code>fund</code>, <code>allocate</code>, <code>return</code>, or <code>transfer</code> from the account relationship. Users choose what they intend to move; they do not choose a lower-level mechanism to bypass those relationships.</p>
    <pre><code>{`POST /v1/moves/quote

{
  "source": "acct_01j2gxq0aab1c2d3e4f5g6h7j8",
  "destination": "acct_01j2gxq0aak9m0n1p2q3r4s5t6",
  "money": { "amount": "500000", "currency": "LXP" }
}

POST /v1/moves
Idempotency-Key: <unique-key-for-this-move>

{ "quote_id": "<quote_id-from-response>" }`}</code></pre>
    <p>These request fields follow the Human API golden wire example. Amount is an integer string in the asset&apos;s precision, not a floating-point number. Obtain the quote ID from the response; the placeholder is not a real quote.</p>
    <table><thead><tr><th>Quote field</th><th>What to review</th></tr></thead><tbody>
      <tr><td><code>money</code> and <code>mechanism</code></td><td>The exact amount, currency, and resolved movement</td></tr>
      <tr><td><code>fee_estimate</code> and <code>fee_ceiling</code></td><td>Expected charge and maximum disclosed charge</td></tr>
      <tr><td><code>arrival_estimate</code> and <code>expires_at</code></td><td>Expected timing and quote validity</td></tr>
      <tr><td><code>irreversibility_copy_key</code></td><td>Any irreversible-action disclosure</td></tr>
    </tbody></table>
    <p>Commit returns a journey. A refused movement reports the refusing authority, explanatory copy, remaining money, and optional change path. Agent funding uses this same quote-and-commit surface. Reclaim uses <code>POST /v1/agents/&#123;agent_id&#125;/reclaim</code> with <code>money</code> and returns a move with return semantics.</p>
    <h3>Spending and fee budgets</h3>
    <p>Agent monthly limits govern spending. Native fee budgets independently constrain per-activity fees, aggregate fees, and period fees. Read <code>GET /v1/sessions/fee-policy</code> for the native fee asset, currency, and decimals. Present the principal movement and the execution fee separately so an agent&apos;s funding amount is not confused with its ability to pay fees.</p>
    <h2>Verification levels</h2>
    <p>Human verification advances through three current rungs as evidence arrives: submitted (<code>unverified</code>), receipted (<code>receipt-verified</code>), and committed (<code>checkpoint-finalised</code>). <code>checkpoint-finalised</code> is the retained label for committed and is the current top rung. <code>paxeer-finalised</code> and <code>settlement-anchored</code> are retired decoding values; they do not define additional current rungs, so do not wait for them or present them as a stronger finality state.</p>
    <h2>Card, bank, and real-time payment rails</h2>
    <p>The fiat interop adapter connects provider callbacks for card, bank, and real-time payments. Provider evidence distinguishes authorised, clearing, settled, reversed, and chargeback events. Credit and corrective movements require accepted provider evidence and a receipt-gated LayerX result.</p>
    <table><thead><tr><th>Rail</th><th>Provider callback route</th></tr></thead><tbody>
      <tr><td>Card</td><td><code>POST /v1/http/fiat/card/callbacks</code></td></tr>
      <tr><td>Bank</td><td><code>POST /v1/http/fiat/bank/callbacks</code></td></tr>
      <tr><td>Real-time payment</td><td><code>POST /v1/http/fiat/rtp/callbacks</code></td></tr>
    </tbody></table>
    <p>These are provider integration endpoints, not consumer deposit or card-entry endpoints. Callbacks carry token references only; PAN-shaped input is refused. A provider&apos;s authorisation is distinct from settlement, and reversal or chargeback is represented as its own verified event.</p>
    <h3>Independent market-maker ramps</h3>
    <p>Market-maker ramps let ordinary LayerX principals offer external conversion through quote terms and receipt-gated orders. Their toolkit compiles payer grant draws and operator sends, verifies order receipts, and journals orders. External custody is explicitly labelled so users can distinguish a ramp operator&apos;s custody from the Human account custody boundary. This ramp role is separate from the fiat callback adapter.</p>
    <h2>Reconcile the result</h2>
    <p>Read balances through <code>GET /v1/account/balance</code>, journey status through <code>GET /v1/journeys/&#123;journey_id&#125;</code>, and proof material through <code>GET /v1/evidence/&#123;evidence_id&#125;</code>. Journey states preserve still-checking, refused, and waiting-for-you alongside completion. The API response envelope is <code>{'{ ok, result?, error?, trace }'}</code>; retain trace and evidence references for support.</p>
    <p>See <Link href="/human-interface">the Human interface</Link> for passkeys, wallet binding, approvals, and agent controls, and <Link href="/interop">interop</Link> for external system boundaries.</p>
  </DocsLayout>
}
