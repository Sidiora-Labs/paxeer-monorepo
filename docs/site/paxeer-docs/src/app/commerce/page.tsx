import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/commerce')

export default function Commerce() {
  return <DocsLayout>
    <div className="page-header">
      <h1 className="page-title">Agent commerce</h1>
      <p className="page-description">Connect merchant checkout, trusted shopping agents, delegated authorization, and receipt-backed payment through Paxeer X.</p>
    </div>
    <h2>One commerce journey, explicit authority at every step</h2>
    <p>Paxeer X brings the LayerX commerce plane into a single workflow: a merchant advertises its capabilities, an agent identifies the intended operation, the payer supplies bounded authorization, and the payment plane produces verifiable settlement evidence. Checkout and order experiences use that evidence to report a completed purchase.</p>
    <table><thead><tr><th>Protocol or integration</th><th>Role</th><th>Result</th></tr></thead><tbody>
      <tr><td>UCP</td><td>Merchant profiles, capability negotiation, checkout completion, and order reads.</td><td>A checkout outcome and an order tied to verified payment evidence.</td></tr>
      <tr><td>AP2</td><td>Signed payment and checkout mandates with evaluated constraints.</td><td>An authorized payment mapped to a typed LayerX intent.</td></tr>
      <tr><td>Visa Trusted Agent Protocol</td><td>Signed agent identity and browse or pay intent verification.</td><td>A verified agent credential handed to the merchant authority path.</td></tr>
      <tr><td>Express and FastAPI</td><td>Paid-resource release, fulfillment persistence, and webhook processing.</td><td>A resource associated with its canonical receipt and request digest.</td></tr>
    </tbody></table>
    <h2>Merchant discovery and capability negotiation</h2>
    <p>The UCP adapter uses revision <code>2026-04-08</code>. A merchant profile declares REST services, payment handlers, and supported capabilities. A platform profile is negotiated against it before checkout completion. The checkout capability is <code>dev.ucp.shopping.checkout</code>; order reads use <code>dev.ucp.shopping.order</code>. Negotiation binds the selected payment handler by digest, so checkout completion carries the payment configuration that the participants agreed to use.</p>
    <p>Publish a merchant profile with the supported protocol revision and HTTPS service endpoint. Have the buying platform negotiate its capabilities before it submits a payment. Preserve the negotiated result alongside the checkout for subsequent order verification.</p>
    <h2>Complete a checkout</h2>
    <ol>
      <li>Prepare the checkout identifier, uppercase three-letter currency, and positive total in ISO-4217 minor units.</li>
      <li>Bind the checkout to the LayerX asset and recipient, negotiated payment handler, and a stable UCP idempotency key.</li>
      <li>Submit completion through <code>POST /v1/http/ucp/checkouts/complete</code>.</li>
      <li>The payment plane executes the typed payment intent. Pending or refused execution produces the corresponding checkout outcome.</li>
      <li>For executed payment, verify the canonical receipt against the authorized batch and checkout binding. Return the completed order only after settlement evidence passes verification.</li>
    </ol>
    <p>At the adapter boundary, the completion request has the following fields. This is the Rust submission type, rather than an HTTP JSON example:</p>
    <pre><code>{`CheckoutSubmission {
    checkout_id,
    currency,            // [u8; 3], ISO-4217 code
    total_minor,         // u128, positive minor-unit amount
    layerx_asset,        // [u8; 32]
    layerx_recipient,    // [u8; 32]
    idempotency_key,     // UcpIdempotencyKey
    negotiated,          // NegotiatedCapabilities
}`}</code></pre>
    <p>Submission validation also requires a nonzero LayerX asset, a nonzero recipient, and a nonzero negotiated payment-handler digest. A submission that fails validation is refused before payment execution.</p>
    <h3>Retries and completion states</h3>
    <p>The gateway binds an idempotency key to a digest of the submitted request. Repeated attempts retain their payment identity. An execution already settled by receipt must present consistent order evidence on a retry; a pending response cannot replace previously verified settlement.</p>
    <table><thead><tr><th>Checkout status vocabulary</th><th>Meaning in the purchase journey</th></tr></thead><tbody>
      <tr><td><code>Incomplete</code></td><td>The checkout still requires information.</td></tr>
      <tr><td><code>RequiresEscalation</code></td><td>The flow requires escalation before completion.</td></tr>
      <tr><td><code>ReadyForComplete</code></td><td>The checkout is ready for completion submission.</td></tr>
      <tr><td><code>CompleteInProgress</code></td><td>Completion awaits a payment result.</td></tr>
      <tr><td><code>Completed</code></td><td>Verified payment evidence supports the order.</td></tr>
      <tr><td><code>Canceled</code></td><td>The checkout is canceled.</td></tr>
    </tbody></table>
    <h2>Read an order with its evidence</h2>
    <p>An order combines merchant-owned metadata, including its order identifier and HTTPS permalink, with verified settlement evidence. <code>UcpAdapter::read_order</code> checks the negotiated order capability, validates metadata, and re-verifies the stored canonical receipt against its authorized batch and original checkout. Reading an order is a read operation; it does not debit balances or execute a second payment.</p>
    <h2>Release paid resources</h2>
    <p>The Express and FastAPI integrations connect settlement to merchant fulfillment. Their fulfillment repository receives a proposed record and a resource-release function. Persist the idempotency key, request digest, canonical receipt, and returned resource together. On repeat delivery, return the existing record only when its request digest matches; a different digest produces a fulfillment conflict.</p>
    <p>The seller fulfillment store creates records with exclusive file creation (<code>wx</code>) so a concurrent duplicate cannot overwrite the first record. Webhook delivery tracking is a separate responsibility: use a durable delivery store for a merchant service that runs across restarts or multiple workers. Keep fulfillment identity consistent with the payment operation so retries return the same purchased resource.</p>
    <h2>Plan the merchant integration</h2>
    <ol>
      <li>Define the merchant profile, payment handler, asset mapping, and recipient.</li>
      <li>Choose the buying-agent credential and <Link href="/mandates">mandate authorization</Link> policy.</li>
      <li>Persist checkout submissions, idempotency keys, and merchant order metadata.</li>
      <li>Store canonical receipts and authorized batches for checkout completion and order reads.</li>
      <li>Connect fulfillment and webhook delivery to durable application records.</li>
    </ol>
    <p>Use <Link href="/evm">EVM applications</Link> for contract-facing experiences and <Link href="/sdk">the SDK</Link> for network integration. Commerce adapters preserve protocol-specific identities while sending payment execution through the typed LayerX plane.</p>
  </DocsLayout>
}
