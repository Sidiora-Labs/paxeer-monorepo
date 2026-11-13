import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import { metadataFor } from '@/lib/seo'
import Link from 'next/link'

export const metadata = metadataFor('/paid-apis')

export default function PaidApisPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Paid APIs and x402</h1><p className="page-description">Use x402 version 2 offers, EVM payments, and verified receipts to release paid HTTP resources on Paxeer mainnet-beta.</p></div>
    <p>A paid endpoint advertises its price through x402 version 2. The buyer selects an accepted offer, authorizes payment on the named EVM network, and retries with payment evidence. The seller releases the resource after verifying the receipt and required commitment. Exact, metered, and subscription schemes use this evidence-bound lifecycle.</p>
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
      <tr><td><code>scheme</code>, <code>network</code></td><td><code>exact</code>, <code>metered</code>, or <code>subscription</code>, with an <code>eip155:</code> network identifier. Paxeer mainnet-beta uses <code>eip155:125</code>.</td></tr>
      <tr><td><code>amount</code></td><td>Positive canonical decimal string, represented exactly as an unsigned 128-bit integer.</td></tr>
      <tr><td><code>asset</code>, <code>payTo</code></td><td>32-byte hexadecimal Asset and recipient identifiers.</td></tr>
      <tr><td><code>maxTimeoutSeconds</code></td><td>Positive integer request timeout, at most <code>2^32 - 1</code> seconds.</td></tr>
      <tr><td><code>extra.layerx</code></td><td>Commitment, account reference, currency, and scheme-specific grant terms.</td></tr>
    </tbody></table>
    <p>The network is an <code>eip155:</code> chain identifier with a positive chain number that fits unsigned 32-bit range. For the typed money-movement buyer flow, <code>extra.layerx.account</code> names the destination account, such as <code>agent:&lt;did&gt;:main</code>, and <code>currency</code> names its money currency. The buyer checks that <code>payTo</code> corresponds to the offered account or its asset account before preparing payment.</p>
    <h2>Add seller middleware</h2>
    <p>The Node package <code>@sidiora/layerx-seller-middleware</code> lives at <code>services/platform/middleware/seller/</code>. It exports <code>SellerMiddleware</code>, <code>ReceiptPayloadAuthority</code>, and payment-header codecs. Configure trusted receipt keys, a payment authority, payment requirements, and a durable fulfillment repository. This handler follows <code>services/platform/examples/paid-api/index.mjs</code>; <code>middleware</code> is the configured seller instance.</p>
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
    <p>The example serves <code>GET /paid</code>. Its configuration selects network, scheme, resource URL, port, and the environment-variable names for price, asset, recipient, account, currency, and receipt keys. Its offer requests <code>receipted</code> assurance. Set the resource URL to the endpoint the customer requests.</p>
    <h3>Persist fulfillment</h3>
    <p>A fulfillment record binds payment identity to the request digest, transaction hash, canonical receipt evidence, optional committed-block evidence, and returned resource. Matching retries return the stored resource; conflicting reuse is refused. The example durably records a claim before resource release, then synchronizes and publishes the result. An unresolved claim remains an unknown outcome until reconciled, so an uncertain release is not repeated.</p>
    <h2>Integrate the buyer</h2>
    <p><code>BuyerMiddleware</code> in <code>services/platform/middleware/buyer/</code> accepts a production client, source account, supported scheme/network pairs, trusted receipt keys, an optional commitment resolver, and retry policy. <code>prepare(paymentRequiredHeader, callerIdempotencyKey)</code> prepares an exact payment through the typed quote workflow. <code>fetch</code> drives the HTTP exchange, while <code>captureSettlement</code> verifies the result. Use <code>fetchGrant</code> and <code>captureGrantSettlement</code> for grant-backed draw transactions.</p>
    <p>The buyer validates offered account and currency terms before quoting. Required extensions are echoed unchanged. Choose a stable caller idempotency key for one purchase and retain it with the accepted offer and activity identifier until the outcome is resolved.</p>
    <h2>Metered requests and subscriptions</h2>
    <p>A metered or subscription offer requires nonzero 32-byte hexadecimal <code>payer</code> and <code>purposeHash</code> terms. Subscription offers require a positive decimal-string <code>windowSeconds</code> below <code>2^64</code>; metered offers omit that field.</p>
    <p>Grant-backed payments use a signed EIP-1559 transaction calling the ledger precompile’s <code>draw</code> method. Its 228-byte calldata encodes the grant ID, recipient, unsigned 128-bit amount, idempotency key, unsigned 64-bit sequence, purpose hash, and context hash. Each metered request or subscription payment carries its own authorized draw. The seller requires the grant to be active and bound to the signed executor, payer, recipient, and asset.</p>
    <pre><code>{`{
  "rawTransaction": "<signed EIP-1559 draw transaction as hex>",
  "idempotencyKey": "<64-hex key>"
}`}</code></pre>
    <p>The grant payment authority checks the transaction’s chain ID, ledger destination, zero native value, calldata, amount, recipient, purpose, idempotency key, and executor. Receipt verification requires the matching <code>PaymentDrawn</code> event, including grant ID, payer, asset, amount, and context. A request with changed payment facts is refused.</p>
    <h2>Select settlement assurance</h2>
    <table><thead><tr><th>Commitment</th><th>Required evidence</th></tr></thead><tbody>
      <tr><td><code>receipted</code></td><td>An authorized signed successful receipt bound to the raw EVM transaction and matching payment event.</td></tr>
      <tr><td><code>committed</code></td><td>Receipted evidence plus verified transaction and result inclusion, block headers, validator sets, and BFT commits.</td></tr>
      <tr><td>Trust configuration</td><td>Receipt keys are configured locally; committed verification also resolves the trusted chain identity and validator-set hash.</td></tr>
    </tbody></table>
    <p>Set <code>extra.layerx.commitment</code> to <code>receipted</code> or <code>committed</code>. Committed verification requires a commitment resolver and a trusted validator resolver. A receipt is final when its block is committed; a signed receipt alone does not supply that proof. Missing evidence cannot lower the requested assurance.</p>
    <h3>Receipt verification and capture</h3>
    <p>The seller verifies canonical receipt encoding, the raw transaction and its chain ID, sender and destination binding, successful result, authorized receipt signature, receipt digest, and the accepted payment facts in ledger log effects. Successful settlement returns the verified EVM transaction hash in <code>transaction</code>.</p>
    <p>The buyer checks <code>success</code>, network, amount, payer, transaction hash, and the <code>layerx</code> extension. Portable evidence uses <code>format: "layerx-receipt-proof-v2"</code> and <code>verificationLevel: "receipted"</code> or <code>"committed"</code>, with raw transaction, signed receipt, receipt digest, and receipt-key metadata. The buyer verifies these against its prepared payment and configured trust. HTTP 200 alone does not establish payment.</p>
    <h2>Recover uncertain payment outcomes</h2>
    <p>Preserve the original raw transaction, its EVM hash, the accepted offer, and purchase idempotency key when a payment outcome is uncertain. The node’s <code>pax_sequencerReceipt</code> method supplies receipt data for the transaction; the hosted gateway also accepts that method. Verify recovered material before using it as payment evidence.</p>
    <p>Recover the original payment through the SDK or buyer-middleware settlement capture. Committed assurance requires recovered evidence to verify against the chain’s block headers, BFT commits, inclusion proofs, and trusted validator set. The seller’s durable fulfillment record resolves matching retries to the same resource. A replacement debit does not resolve the original purchase.</p>
    <table><thead><tr><th>Seller outcome</th><th>HTTP</th><th>Application action</th></tr></thead><tbody>
      <tr><td>Payment required</td><td>402</td><td>Select and authorize an offered payment.</td></tr>
      <tr><td>Pending</td><td>202</td><td>Honor retry guidance and recover the same activity.</td></tr>
      <tr><td>Refused or conflicting reuse</td><td>402 or 409</td><td>Inspect the refusal reason and preserve the original payment identity.</td></tr>
      <tr><td>Released</td><td>200</td><td>Verify PAYMENT-RESPONSE and consume the stored resource.</td></tr>
    </tbody></table>
    <h2>Facilitators and transport interoperability</h2>
    <p>The interoperability service in <code>services/interop/</code> supplies x402 facilitator operations for supported offers, verification, and settlement. Verification validates the signed payment without submitting it; settlement uses the chain relay and verified receipt evidence. HTTP, MCP, and A2A bindings share payment semantics. Reuse the same settlement identity for recovery and reject a conflicting receipt for that identity.</p>
    <p>Use <Link href="/services">service agreements</Link> for work that needs deadlines, delivered artifacts, review, and disputes. Continue with the <Link href="/sdk">SDK</Link> and <Link href="/modules">native protocol modules</Link> for account, Asset, and activity integration.</p>
  </DocsLayout>
}
