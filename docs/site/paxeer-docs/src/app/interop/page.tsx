import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/interop')

export default function InteroperabilityPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Interoperability</h1><p className="page-description">Connect agent payments, commerce mandates, card and bank rails, and external chains to Paxeer X through evidence-backed adapters.</p></div>
    <h2>One execution ledger, many entry points</h2>
    <p>Interoperability adapters translate foreign protocols into evidence that LayerX execution can verify. They preserve the original protocol&apos;s authorization and settlement facts while routing state changes through a receipt-verified LayerX operation. Paxeer X retains custody and withdrawal guarantees.</p>
    <p>The interoperability gateway provides HTTP, MCP, and A2A ingress. Transport selects how a request arrives; the adapter selects how its evidence is validated. Applications keep the same operation identity and reconciliation process across these interfaces.</p>
    <h2>Connector catalogue</h2>
    <table><thead><tr><th>Adapter</th><th>Application journey</th><th>Evidence boundary</th></tr></thead><tbody>
      <tr><td><code>x402</code></td><td>Buyer builds payment, seller offers a resource, facilitator verifies and settles</td><td>Payment evidence plus verified LayerX receipt; HTTP, MCP, and A2A transports</td></tr>
      <tr><td><code>ap2</code></td><td>Verify an agent commerce mandate and execute within its authority</td><td>Verified mandate and LayerX receipt</td></tr>
      <tr><td><code>ucp</code></td><td>Complete a commerce checkout</td><td>Checkout evidence and receipt-gated execution</td></tr>
      <tr><td><code>visa-tap</code></td><td>Verify a trusted agent intent and execute its authorized action</td><td>Trusted agent credential, canonical target, and policy checks</td></tr>
      <tr><td><code>fiat</code></td><td>Process card, bank, and real-time-payment callbacks</td><td>External settlement evidence and LayerX receipt</td></tr>
      <tr><td><code>migration-v2</code></td><td>Import verified account, asset, and history state</td><td>Source-chain verification and migration rules</td></tr>
    </tbody></table>
    <h2>Gateway discovery and operation tracking</h2>
    <p><code>GET /v1/adapters</code> exposes adapter metadata. Track an operation with <code>GET /v1/operations/&#123;id&#125;</code>, where <code>id</code> is exactly 64 hexadecimal characters. The service also exposes <code>GET /livez</code> and <code>GET /readyz</code> for process and readiness checks. These are gateway paths relative to the configured gateway origin.</p>
    <pre><code>{`# Use the gateway origin assigned to your environment.
curl "$INTEROP_GATEWAY/v1/adapters"
curl "$INTEROP_GATEWAY/v1/operations/$OPERATION_ID"`}</code></pre>
    <p>Build the request body from the selected adapter&apos;s schema. Preserve operation identifiers and evidence digests through retries. A successful HTTP exchange reports the gateway exchange; the operation&apos;s receipt-backed outcome establishes the state change.</p>
    <h3>Evidence policies</h3>
    <table><thead><tr><th>Policy</th><th>Required basis</th></tr></thead><tbody><tr><td><code>layerx-receipt</code></td><td>A verified LayerX execution receipt</td></tr><tr><td><code>verified-mandate+layerx-receipt</code></td><td>Accepted mandate authority and a verified execution receipt</td></tr><tr><td><code>trusted-agent-credential</code></td><td>Agent credential accepted by the configured trust policy</td></tr><tr><td><code>external-settlement+layerx-receipt</code></td><td>External rail settlement evidence paired with a verified execution receipt</td></tr></tbody></table>
    <h2>x402 over HTTP, MCP, and A2A</h2>
    <p>x402 supports buyer, seller, and facilitator roles. The same route family accepts <code>http</code>, <code>mcp</code>, or <code>a2a</code> as its transport segment. Capability discovery comes before constructing a payment, so clients can select the supported scheme and network.</p>
    <table><thead><tr><th>Method</th><th>Path</th><th>Purpose</th></tr></thead><tbody>
      <tr><td>GET</td><td><code>/v1/&#123;transport&#125;/x402/supported</code></td><td>Discover payment support</td></tr>
      <tr><td>GET</td><td><code>/v1/&#123;transport&#125;/x402/facilitator/supported</code></td><td>Discover facilitator support</td></tr>
      <tr><td>POST</td><td><code>/v1/&#123;transport&#125;/x402/buyer/build</code></td><td>Build buyer evidence</td></tr>
      <tr><td>POST</td><td><code>/v1/&#123;transport&#125;/x402/seller/offer</code></td><td>Produce a seller offer</td></tr>
      <tr><td>POST</td><td><code>/v1/&#123;transport&#125;/x402/verify</code></td><td>Verify payment evidence</td></tr>
      <tr><td>POST</td><td><code>/v1/&#123;transport&#125;/x402/settle</code></td><td>Settle through LayerX execution</td></tr>
    </tbody></table>
    <p>Verification also has the <code>/facilitator/verify</code> variant; settlement has <code>/seller/settle</code> and <code>/facilitator/settle</code> variants beneath the same x402 prefix. Commitment extras and metered or subscription grant draws let applications bind payments to a resource and consume an authorized allowance over time.</p>
    <h3>Paid resource workflow</h3>
    <ol><li>The seller publishes an offer identifying the resource and payment terms.</li><li>The buyer discovers support and builds evidence for the selected terms.</li><li>The seller or facilitator verifies the evidence before accepting the purchase.</li><li>Settlement submits the LayerX operation and verifies its receipt.</li><li>The application associates resource delivery with the accepted payment operation and retains its evidence for reconciliation.</li></ol>
    <h2>Agent commerce and fiat rails</h2>
    <table><thead><tr><th>Method</th><th>HTTP route</th><th>Action</th></tr></thead><tbody>
      <tr><td>POST</td><td><code>/v1/http/ap2/mandates/verify</code></td><td>Verify mandate</td></tr><tr><td>POST</td><td><code>/v1/http/ap2/execute</code></td><td>Execute authorized mandate</td></tr>
      <tr><td>POST</td><td><code>/v1/http/ucp/checkouts/complete</code></td><td>Complete checkout</td></tr>
      <tr><td>POST</td><td><code>/v1/http/visa-tap/intents/verify</code></td><td>Verify trusted-agent intent</td></tr><tr><td>POST</td><td><code>/v1/http/visa-tap/intents/execute</code></td><td>Execute accepted intent</td></tr>
      <tr><td>POST</td><td><code>/v1/http/fiat/card/callbacks</code></td><td>Accept card-rail evidence</td></tr><tr><td>POST</td><td><code>/v1/http/fiat/bank/callbacks</code></td><td>Accept bank-rail evidence</td></tr><tr><td>POST</td><td><code>/v1/http/fiat/rtp/callbacks</code></td><td>Accept real-time-payment evidence</td></tr>
    </tbody></table>
    <p>Unlike x402, these adapters are HTTP-only: there is no MCP or A2A variant of the routes above. Mandate verification establishes what an agent may do before execution changes state. TAP policy binds the trusted agent and canonical request target, with clock-skew limits owned by the service. Fiat callbacks bind external settlement to the LayerX operation; repeating a provider notification must reconcile the original operation rather than credit it twice.</p>
    <h2>Migration imports</h2>
    <p>The hosted migration adapter is <code>migration-v2</code>. It is served under <code>/v2</code>, not the <code>/v1</code> adapter prefix, and every route is a POST. Other methods, a <code>/v1</code> prefix, or a trailing slash are unknown routes.</p>
    <table><thead><tr><th>Method</th><th>Route</th><th>Action</th></tr></thead><tbody>
      <tr><td>POST</td><td><code>/v2/migration/accounts</code></td><td>Import accounts</td></tr>
      <tr><td>POST</td><td><code>/v2/migration/assets</code></td><td>Import assets</td></tr>
      <tr><td>POST</td><td><code>/v2/migration/history</code></td><td>Import history</td></tr>
      <tr><td>POST</td><td><code>/v2/migration/history/read</code></td><td>Read imported history</td></tr>
    </tbody></table>
    <h2>Portable receipts and mirrors</h2>
    <p>Portable receipt verification checks LayerX evidence without requiring a live node connection. Ethereum and Solana mirrors publish batch archives and commitments so independent consumers can verify committed LayerX evidence; see <Link href="/mirrors">Ethereum &amp; Solana Mirrors</Link>. Mirrors carry verification data; custody remains on Paxeer X.</p>
    <p>Choose migration for a source-state transition, a mirror for independent archive verification, and the <Link href="/bridge">bridge</Link> for a custody-backed asset transfer. Preserve the source chain identity and proof material in each case.</p>
    <h2>Admission and receipts</h2>
    <p>Adapters ultimately submit operations to LayerX execution. An admission acknowledgement precedes execution and publication, so it does not by itself establish a state change. Receipt lookup is the step that establishes the operation&apos;s outcome. Integrations keep gateway exchange, admission, execution receipt, and settlement evidence as separate records, keyed by the operation identifier.</p>
    <h2>Continue building</h2><p>Explore <Link href="/bridge">bridging and custody</Link>, <Link href="/paxeer-vs-layerx">execution and settlement architecture</Link>, and <Link href="/rest-grpc">native chain APIs</Link> to connect protocol evidence to chain state.</p>
  </DocsLayout>
}
