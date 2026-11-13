import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import { metadataFor } from '@/lib/seo'
import Link from 'next/link'

export const metadata = metadataFor('/paid-apis')

export default function PaidApisPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Paid APIs and x402</h1><p className="page-description">Monetize HTTP resources, agent tools, and recurring access with payment-required offers and receipt-verified fulfillment on Paxeer X.</p></div>
    <p>A Paxeer X paid endpoint advertises its price through x402 version 2. The buyer selects an accepted offer, authorizes payment, and retries with payment evidence. The seller releases the resource after verifying the receipt and required commitment. Exact, metered, and subscription schemes share this evidence-bound lifecycle.</p>
    <DocArt name="service-payment" />
    <h2>The HTTP payment exchange</h2>
    <ol>
      <li>The buyer requests the resource without payment.</li>
      <li>The seller returns HTTP <code>402</code> with a <code>PAYMENT-REQUIRED</code> header and matching JSON body.</li>
      <li>The buyer selects a supported scheme and network, verifies payee terms, and constructs a payment payload.</li>
      <li>The buyer retries with <code>PAYMENT-SIGNATURE</code>, preserving the accepted offer and required extensions.</li>
      <li>The seller verifies settlement and records fulfillment before returning the resource with <code>PAYMENT-RESPONSE</code>.</li>
      <li>The buyer independently verifies the response receipt and settlement reference.</li>
    </ol>
    <p>HTTP headers carry standard-base64 JSON. MCP and A2A bindings carry the corresponding JSON values. Settlement identity stays consistent across transports, so redelivering the same payment through another binding retains the same economic identity.</p>
    <h2>Define the offer</h2>
    <table><thead><tr><th>Field</th><th>Contract</th></tr></thead><tbody>
      <tr><td><code>x402Version</code></td><td><code>2</code>.</td></tr>
      <tr><td><code>resource</code></td><td>HTTP(S) URL, with optional description, MIME type, service name, tags, and icon URL.</td></tr>
      <tr><td><code>accepts</code></td><td>One to 32 offers; buyer selection follows seller order among supported scheme/network pairs.</td></tr>
      <tr><td><code>scheme</code>, <code>network</code></td><td><code>exact</code>, <code>metered</code>, or <code>subscription</code>, with the intended network identifier.</td></tr>
      <tr><td><code>amount</code></td><td>Positive canonical decimal string, represented exactly as an unsigned 128-bit integer.</td></tr>
      <tr><td><code>asset</code>, <code>payTo</code></td><td>32-byte hexadecimal Asset and recipient identifiers.</td></tr>
      <tr><td><code>maxTimeoutSeconds</code></td><td>Positive request timeout bound.</td></tr>
      <tr><td><code>extra.layerx</code></td><td>Commitment, account reference, currency, and scheme-specific grant terms.</td></tr>
    </tbody></table>
    <p>For a <code>layerx:*</code> network, <code>extra.layerx.account</code> names the destination account, such as <code>agent:&lt;did&gt;:main</code>, and <code>currency</code> names its money currency. The offered <code>payTo</code> must derive from that account under a supported account-ID derivation. Buyers refuse a mismatched recipient.</p>
    <h2>Add seller middleware</h2>
    <p>The Node package <code>@sidiora/layerx-seller-middleware</code> exports <code>SellerMiddleware</code>, <code>ReceiptPayloadAuthority</code>, and the payment-header codecs. Supply a trusted authority resolver, payment requirements, and a durable fulfillment repository. The following handler is the resource-release pattern used by the paid API example; <code>middleware</code> is the configured seller instance.</p>
    <pre><code>{`import { PAYMENT_SIGNATURE_HEADER } from "@sidiora/layerx-seller-middleware";

const header = request.headers[PAYMENT_SIGNATURE_HEADER.toLowerCase()];
const paymentHeader = Array.isArray(header) ? undefined : header;
const decision = await middleware.handle(
  "public-paid-api",
  paymentHeader,
  async () => resourceBody,
);
const headers = decision.kind === "payment-required"
  || decision.kind === "refused"
  || decision.kind === "released"
  ? decision.headers
  : { "retry-after": "1" };
response.writeHead(decision.status, {
  "content-type": "application/json", ...headers,
});
response.end(JSON.stringify(
  decision.kind === "payment-required" ? decision.body
    : decision.kind === "released" ? decision.resource
    : { state: decision.kind },
));`}</code></pre>
    <p>The example serves <code>GET /paid</code>. Its configuration selects protocol version, network, price, Asset, recipient, account, currency, receipt authority, and fulfillment directory. Keep credentials in the configured environment variables. Set the resource URL to the externally reachable endpoint your customer requests.</p>
    <h3>Persist fulfillment</h3>
    <p>A fulfillment record binds the caller&apos;s idempotency key to the request digest, canonical receipt, authorized batch, and returned resource. Persist it durably and return the stored resource for a matching retry. Refuse reuse of the same key for a different request. The example writes an exclusive file, synchronizes it and its directory, and checks the existing record after a concurrent write.</p>
    <h2>Integrate the buyer</h2>
    <p><code>BuyerMiddleware</code> accepts a production client, source account, protocol selection, supported scheme/network pairs, configured receipt authorities, and retry policy. <code>prepare(paymentRequiredHeader, callerIdempotencyKey)</code> parses the offer and prepares payment through the typed quote workflow. <code>fetch</code> drives the HTTP exchange, while <code>captureSettlement</code> verifies the result. Use <code>fetchGrant</code> and <code>captureGrantSettlement</code> for grant-backed receives.</p>
    <p>The buyer validates offered account and currency terms before quoting. Required extensions are echoed unchanged. Choose a stable caller idempotency key for one purchase and retain it with the accepted offer and activity identifier until the outcome is resolved.</p>
    <h2>Metered requests and subscriptions</h2>
    <p>A metered or subscription offer additionally commits to a nonzero payer and purpose hash. Subscription offers require a positive decimal-string <code>windowSeconds</code> below <code>2^64</code>; metered offers omit the field.</p>
    <p>The payer signs a canonical 346-byte Asset ordinal-7 grant. The receiver submits a canonical 733-byte Asset ordinal-6 receive that embeds the grant and receiver authorization. Every metered request is one draw. Every subscription renewal uses a distinct signed receive and idempotency key; a recurring allowance alone does not establish one renewal per period.</p>
    <pre><code>{`{
  "receive": "<733-byte canonical receive as hex>",
  "idempotencyKey": "<64-hex key>"
}`}</code></pre>
    <p>Validation binds the payer, recipient, Asset, amount, purpose, network, grant identity, allowance, per-draw maximum, expiry, controller, and signed context. Metered grants are non-recurring with a zero window. Unknown, paused, or unregistered Assets and changed payment facts are refused.</p>
    <h2>Select settlement assurance</h2>
    <table><thead><tr><th>Commitment</th><th>Required evidence</th></tr></thead><tbody>
      <tr><td><code>executed</code></td><td>An authorized sequencer-signed successful receipt for the exact activity.</td></tr>
      <tr><td><code>batched</code></td><td>Executed evidence plus receipt inclusion in the authorized signed batch header.</td></tr>
      <tr><td><code>finalised</code></td><td>Batched evidence plus the guarantor checkpoint certificate for that batch.</td></tr>
    </tbody></table>
    <p>Set <code>extra.layerx.commitment</code> to the assurance your resource requires. Authority keys come from verifier configuration. Missing evidence keeps settlement pending or refused; it never silently lowers the requested assurance.</p>
    <h3>Receipt verification and capture</h3>
    <p>The seller verifies canonical receipt encoding, activity binding, successful result, protocol body, authorized sequencer signature, and the accepted Asset, amount, and recipient. The gateway records a receipt-verified settlement. Success returns <code>transaction: "lxp:&lt;receipt_digest&gt;"</code>.</p>
    <p>The buyer checks <code>success</code>, the <code>layerx</code> extension, <code>verificationLevel: "sequencer-signed"</code>, canonical receipt bytes, payer, network, Asset, recipient, and amount. It recomputes the receipt digest and requires both the declared digest and <code>lxp:</code> reference to match. An HTTP 200 or acknowledgement by itself does not establish settlement.</p>
    <h2>Recover uncertain payment outcomes</h2>
    <p>There is no public <code>lx_*</code> JSON-RPC transport in this checkout. The hosted gateway refuses every <code>lx_*</code> method name with JSON-RPC <code>-32601</code> (<code>Method not found</code>), including <code>lx_sendActivity</code>, <code>lx_getActivityStatus</code>, <code>lx_getReceipt</code>, and <code>lx_getProof</code>. Do not build submission or recovery flows on those names.</p>
    <p>Recover through the identity you already hold: the purchase idempotency key, the accepted offer, and the original activity or transaction reference. Re-resolve the original payment through the SDK or buyer-middleware settlement capture, and use published hosted reads such as <code>pax_sequencerReceipt</code> where they apply. For batched assurance, require the recovered receipt evidence to verify against the authorized signed batch header before release. Verify recovered evidence before releasing or capturing the resource. Preserve payment identity throughout recovery; a replacement debit does not resolve the original purchase.</p>
    <table><thead><tr><th>Seller outcome</th><th>HTTP</th><th>Application action</th></tr></thead><tbody>
      <tr><td>Payment required</td><td>402</td><td>Select and authorize an offered payment.</td></tr>
      <tr><td>Pending</td><td>202</td><td>Honor retry guidance and recover the same activity.</td></tr>
      <tr><td>Refused</td><td>402</td><td>Read settlement reason; correct the failed condition.</td></tr>
      <tr><td>Released</td><td>200</td><td>Verify PAYMENT-RESPONSE and consume the stored resource.</td></tr>
    </tbody></table>
    <h2>Facilitators and transport interoperability</h2>
    <p>The x402 facilitator exposes <code>/supported</code>, <code>/verify</code>, and <code>/settle</code>. Verification is read-only; settlement is state-changing and completes against verified canonical receipts. HTTP, MCP, and A2A share the same message semantics. An identical settlement identity and receipt digest recover the existing settlement; a swapped receipt for that identity is an idempotency conflict.</p>
    <p>Use <Link href="/services">service agreements</Link> for work that needs deadlines, delivered artifacts, review, and disputes. Continue with the <Link href="/sdk">SDK</Link> and <Link href="/modules">native protocol modules</Link> for account, Asset, and activity integration.</p>
  </DocsLayout>
}
