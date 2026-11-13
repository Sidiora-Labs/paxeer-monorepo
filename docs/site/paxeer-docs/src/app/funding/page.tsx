import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/funding')

export default function Funding() {
  return <DocsLayout>
    <div className="page-header">
      <h1 className="page-title">Funding and money movement</h1>
      <p className="page-description">Quote and commit money movements, fund managed agents, and inspect receipt evidence on Paxeer.</p>
    </div>
    <h2>Movement journeys</h2>
    <p>The Human API represents money movement as a <code>move</code> journey on one network. The resolver vocabulary is <code>fund</code>, <code>allocate</code>, <code>return</code>, and <code>transfer</code>. Each journey reports its own stages and evidence.</p>
    <table><thead><tr><th>Movement</th><th>Meaning</th><th>Authorization and evidence</th></tr></thead><tbody>
      <tr><td>Fund</td><td>Fund a managed budget</td><td>Resolved route and verified receipts</td></tr>
      <tr><td>Allocate</td><td>Move funds from a budget to an agent</td><td>Resolved route and verified receipts</td></tr>
      <tr><td>Return</td><td>Return funds to the owner</td><td>Resolved route and verified receipts</td></tr>
      <tr><td>Transfer</td><td>Send funds to a recipient</td><td>Quote, commit, execution receipt</td></tr>
    </tbody></table>
    <p>Money-moving mutations require an <code>Idempotency-Key</code>. Reuse the key when retrying the same intended action and reconcile the existing journey before requesting another movement.</p>
    <h2>Production transfer path</h2>
    <p><code>POST /v1/moves/quote</code> prepares a movement and <code>POST /v1/moves</code> commits that quote. In the production service, a quote builds an EIP-1559 bank transfer using the source owner&apos;s bound EVM account and pending nonce. The source must belong to the authenticated owner; the destination must resolve to a different, nonzero recipient.</p>
    <h2>Move money and fund an agent</h2>
    <p>Quote the source, destination, and amount first. The resolver defines <code>fund</code>, <code>allocate</code>, <code>return</code>, and <code>transfer</code> according to the account relationship. The production bank quote returns <code>transfer</code> and preserves the selected source, recipient, amount, and fees for commitment.</p>
    <pre><code>{`POST /v1/moves/quote

{
  "source": "<authenticated-source-account>",
  "destination": "<recipient-account>",
  "money": { "amount": "500000", "currency": "<currency-from-balance>" }
}

POST /v1/moves
Idempotency-Key: <unique-key-for-this-move>

{ "quote_id": "<quote_id-from-response>" }`}</code></pre>
    <p>Substitute the authenticated source account, a resolvable recipient account, and the currency returned by the balance read. The production service accepts the owner&apos;s canonical source account or its returned account identifier; a destination may be a canonical account or EVM address. Amounts are positive integer strings in the asset&apos;s base units. Obtain <code>quote_id</code> from the quote response.</p>
    <table><thead><tr><th>Quote field</th><th>What to review</th></tr></thead><tbody>
      <tr><td><code>money</code> and <code>mechanism</code></td><td>The exact amount, currency, and resolved movement</td></tr>
      <tr><td><code>fee_estimate</code> and <code>fee_ceiling</code></td><td>Expected charge and maximum disclosed charge</td></tr>
      <tr><td><code>arrival_estimate</code> and <code>expires_at</code></td><td>Expected timing and quote validity</td></tr>
      <tr><td><code>irreversibility_copy_key</code></td><td>Any irreversible-action disclosure</td></tr>
    </tbody></table>
    <p>Commit returns a journey. A refused movement reports the refusing authority, explanatory copy, remaining money, and optional change path. Funding a recipient account uses the quote-and-commit surface. Reclaim uses <code>POST /v1/agents/&#123;agent_id&#125;/reclaim</code> with <code>money</code> and returns a move with return semantics.</p>
    <h3>Spending and fee budgets</h3>
    <p>Agent monthly limits carry an enforcement label: <code>protocol</code> or <code>app</code>. Native fee budgets separately constrain per-activity, total, and period fees. Read <code>GET /v1/sessions/fee-policy</code> for the configured fee asset, currency, and decimals, and present the principal amount separately from execution fees.</p>
    <h2>Verification levels</h2>
    <p>Human evidence uses <code>unverified</code> for submitted, <code>receipt-verified</code> for receipted, and <code>checkpoint-finalised</code> for committed. Committed means the verified outcome is included in a committed block on this network. The journey&apos;s completion label alone is insufficient to establish that evidence.</p>
    <h2>Card, bank, and real-time payment rails</h2>
    <p>The fiat interop adapter connects provider callbacks for card, bank, and real-time payments. Provider evidence distinguishes authorised, clearing, settled, reversed, and chargeback events. Credit and corrective movements require accepted provider evidence and a receipt-gated LayerX result.</p>
    <table><thead><tr><th>Rail</th><th>Provider callback route</th></tr></thead><tbody>
      <tr><td>Card</td><td><code>POST /v1/http/fiat/card/callbacks</code></td></tr>
      <tr><td>Bank</td><td><code>POST /v1/http/fiat/bank/callbacks</code></td></tr>
      <tr><td>Real-time payment</td><td><code>POST /v1/http/fiat/rtp/callbacks</code></td></tr>
    </tbody></table>
    <p>These are provider integration endpoints, not consumer deposit or card-entry endpoints. Callbacks carry token references only; PAN-shaped input is refused. A provider&apos;s authorisation is distinct from settlement, and reversal or chargeback is represented as its own verified event.</p>
    <h3>Independent market-maker ramps</h3>
    <p>Market-maker ramps in <code>services/platform/ramps/toolkit</code> compile grant draws and operator sends, verify order receipts, and journal orders. Their quote terms identify the external custody role. Inspect those terms before authorizing a conversion.</p>
    <h2>Reconcile the result</h2>
    <p>Read balances through <code>GET /v1/account/balance</code>, journey status through <code>GET /v1/journeys/&#123;journey_id&#125;</code>, and proof material through <code>GET /v1/evidence/&#123;evidence_id&#125;</code>. Journey states preserve still-checking, refused, and waiting-for-you alongside completion. The API response envelope is <code>{'{ ok, result?, error?, trace }'}</code>; retain trace and evidence references for support.</p>
    <p>See <Link href="/human-interface">the Human interface</Link> for passkeys, ownership, approvals, and agent controls, and <Link href="/interop">interop</Link> for external system boundaries.</p>
  </DocsLayout>
}
