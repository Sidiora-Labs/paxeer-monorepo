import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/payments')

export default function Payments() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Payments</h1>
        <p className="page-description">Build transfers, paid APIs, merchant checkout, metered usage, and subscriptions on Paxeer X with explicit approval, bounded fees, and verifiable settlement.</p>
      </div>
      <h2>One payment lifecycle</h2>
      <p>Paxeer X combines native Asset settlement with the LayerX payment platform. Applications quote a move, commit the accepted quote under a stable idempotency key, and track a journey whose outcome is backed by evidence. Native integrations prepare canonical activities, disclose their exact terms, sign, submit, and verify the requested commitment.</p>
      <ol>
        <li><strong>Quote:</strong> identify payer, recipient, Asset or currency, amount, mechanism, fee ceiling, expected arrival, and irreversible legs.</li>
        <li><strong>Approve and commit:</strong> bind approval to those terms. An expired quote returns <code>quote-expired</code> rather than silently changing the payment.</li>
        <li><strong>Track:</strong> retain the journey or activity identifier and idempotency key through interruptions.</li>
        <li><strong>Verify and fulfil:</strong> validate canonical evidence and payment facts before delivering goods or releasing a protected resource.</li>
      </ol>
      <h2>Choose a payment experience</h2>
      <table><thead><tr><th>Experience</th><th>Flow</th><th>Settlement rule</th></tr></thead><tbody>
        <tr><td>Wallet or application transfer</td><td>Quote and commit a move, or sign a native Asset SEND</td><td>Verify the receipt and selected commitment</td></tr>
        <tr><td>Paid HTTP resource</td><td>x402 exact offer and receipt-backed retry</td><td>Release only after the offer is paid and verified</td></tr>
        <tr><td>Merchant cart</td><td>Catalog quote, persistent order, payment, fulfilment</td><td>Bind order and settlement to the same request digest</td></tr>
        <tr><td>Metered usage</td><td>Non-recurring grant and authorized receiver draws</td><td>Validate purpose, allowance, expiry, and each receive</td></tr>
        <tr><td>Subscription</td><td>Recurring grant with a declared billing window</td><td>New receive and distinct period key for every renewal</td></tr>
      </tbody></table>
      <h2>Journey states</h2>
      <table><thead><tr><th>State</th><th>Application behavior</th></tr></thead><tbody>
        <tr><td><code>getting-ready</code></td><td>The payment is accepted and its legs are being prepared.</td></tr>
        <tr><td><code>sending</code></td><td>At least one leg is in flight.</td></tr>
        <tr><td><code>waiting-for-you</code></td><td>Request the required human decision before its deterministic expiry.</td></tr>
        <tr><td><code>processing</code></td><td>The payment is submitted and settling.</td></tr>
        <tr><td><code>still-checking</code></td><td>The outcome is unknown. Keep duplicate-capable controls locked and resolve the original payment.</td></tr>
        <tr><td><code>done</code></td><td>Settlement is backed by a verified receipt or finality proof.</td></tr>
        <tr><td><code>done-finalised</code></td><td>The receipt-backed outcome has committed evidence, represented by <code>checkpoint-finalised</code> verification. This retained label is not required for every committed journey: EVM-intent and native journey projections render <code>done</code> and expose evidence separately.</td></tr>
        <tr><td><code>refused</code></td><td>Inspect <code>refused_by</code> and <code>money_left</code> before reporting the result.</td></tr>
        <tr><td><code>awaiting-approval</code></td><td>The native engine exposes its awaiting-approval leg phase while the journey awaits approval. It is not receipt-backed completion.</td></tr>
        <tr><td><code>closed</code></td><td>Retained wallet-binding, deposit, withdraw, and exit history reads as closed. Closed history cannot resume signing or execution and does not assert a successful movement.</td></tr>
      </tbody></table>
      <h2>Merchant checkout</h2>
      <p><code>@sidiora/layerx-merchant-middleware</code> prices catalog lines and creates a <code>PAYMENT-REQUIRED</code> offer. A cart contains at most 256 lines, with unique SKUs and positive integer quantities. All lines share one Asset, recipient, scheme, and network. Amount multiplication and summation use integer arithmetic within the 128-bit amount range.</p>
      <p>The order store opens the order before the seller decision. The quote digest binds its lines, total, Asset, recipient, scheme, network, and timeout. Repeating an unchanged checkout resolves the same order; a catalog price change produces a different digest. Store responses are checked for matching <code>checkoutKey</code> and <code>requestDigest</code>, with mismatches refused as <code>order-conflict</code>.</p>
      <table><thead><tr><th>Checkout result</th><th>Action</th></tr></thead><tbody>
        <tr><td><code>payment-required</code></td><td>Return HTTP 402 with the decision headers.</td></tr>
        <tr><td><code>pending</code></td><td>Preserve the order while settlement completes.</td></tr>
        <tr><td><code>refused</code></td><td>Record the refused order and expose its refusal.</td></tr>
        <tr><td><code>paid</code></td><td>The order is <code>paid-verified</code> and carries the receipt digest and transaction.</td></tr>
      </tbody></table>
      <p>Late settlement uses verified webhooks. The merchant checks signature, freshness, replay protection, order identifier and request digest, then resolves and verifies canonical receipt evidence. It recomputes the receipt digest before marking the order paid.</p>
      <h2>Buyer and HTTP payment flow</h2>
      <ol>
        <li>The seller responds with HTTP 402 and <code>PAYMENT-REQUIRED</code>.</li>
        <li>The buyer selects an offered requirement unchanged and checks supported scheme, network, Asset, recipient, amount, and commitment.</li>
        <li>The buyer pays under the purchase idempotency key and retries with <code>PAYMENT-SIGNATURE</code>.</li>
        <li>The seller verifies the payment before release and supplies <code>PAYMENT-RESPONSE</code>.</li>
        <li>The buyer verifies captured settlement against receipt evidence.</li>
      </ol>
      <p><code>@sidiora/layerx-buyer-middleware</code> provides <code>fetch(url, init)</code> for the complete loop, plus <code>parseOffer</code>, <code>prepare</code>, and <code>captureSettlement</code> for custom workflows. Unsupported requirements fail before payment. The human-plane transport uses HTTPS, protects bearer tokens with <code>SecretBytes</code>, and carries <code>Idempotency-Key</code> on mutations.</p>
      <h2>Metered payments and subscriptions</h2>
      <p>A metered or subscription challenge binds <code>extra.layerx.payer</code>, <code>purposeHash</code>, and <code>commitment</code>; subscriptions also bind <code>windowSeconds</code>. The payer signs an ordinal-7 grant. Each draw uses a receiver-signed ordinal-6 receive with its own idempotency key. Before submission, the seller checks payer, recipient, Asset, amount, purpose, grant limits, expiry, network, and receiver authority.</p>
      <p>Metered grants are non-recurring with a zero window. Subscription grants recur with the exact offered window. Every renewal uses current sequences and a distinct period key: an allowance alone does not establish one renewal per period. Pending draws return HTTP 202 without resource release; persistent draw recovery resolves the original activity identifier.</p>
      <h2>Commitments and evidence</h2>
      <table><thead><tr><th>Commitment</th><th>Required evidence</th></tr></thead><tbody>
        <tr><td><code>executed</code></td><td>Verified canonical receipt for the submitted activity.</td></tr>
        <tr><td><code>batched</code></td><td>Execution plus an authenticated receipt proof whose canonical value equals the receipt byte for byte.</td></tr>
        <tr><td><code>finalised</code></td><td>Batch evidence plus finalised checkpoint evidence whose canonical header equals the proof&apos;s signed batch header.</td></tr>
      </tbody></table>
      <p>A stronger requested commitment is never downgraded to a weaker success. HTTP 202, admission acknowledgements, and activity identifiers alone do not establish execution. Successful x402 settlement references <code>lxp:&lt;receipt_digest&gt;</code>, with the digest computed from the canonical receipt and the protocol Merkle-leaf domain. Verify configured authority, protocol/network, receipt facts, and commitment evidence together.</p>
      <h2>Retries, sequences, and unknown outcomes</h2>
      <p>Choose one stable key per purchase or order. Repeating a human-plane mutation under that key returns its original journey; a different body under the same key is <code>idempotency-conflict</code>. Native activities also consume identity and account sequences, preventing replay. Retain signed bytes and the activity identifier when a response is lost. Resolve the original through receipt and status reads instead of generating another debit.</p>
      <p>On the human plane, retriabilities are <code>retriable</code>, <code>retriable-after</code>, <code>structural</code>, and <code>final</code>. The SDK <code>ProductionClient</code> reports its own retry classes: <code>never</code>, <code>safe</code>, <code>after</code>, and <code>unknown-outcome</code>. Map between the two vocabularies explicitly rather than treating them as one list. Respect carried rate-limit timing. An <code>unknown-outcome</code> result or human-plane <code>still-checking</code> requires tracking and evidence recovery, with duplicate controls locked.</p>
      <h2>Fees and refusal</h2>
      <p>The committed schedule prices activity type, encoded bytes, execution, storage, Asset operations, and module operations, then applies its basis-point multiplier with ceiling rounding. Fee estimates expose their schedule and snapshot but reserve nothing. Set an integer <code>fee_limit</code> and maintain spendable fee balance for that limit.</p>
      <p>An admission refusal consumes no sequence and charges no fee. An admitted activity that fails consumes its sequence, emits a failure receipt, rolls back module effects, and may charge up to the fee limit. Protocol fees go to <code>system:fees</code>; Programs occupancy rent is a separate batch settlement.</p>
      <h2>Continue building</h2>
      <ul><li><Link href="/payments-quickstart">Make your first payment</Link></li><li><Link href="/assets">Assets and token accounts</Link></li><li><Link href="/platform-api">Public payment API</Link></li><li><Link href="/paid-apis">x402 transport</Link></li><li><Link href="/fees">Fee schedules</Link></li></ul>
    </DocsLayout>
  )
}
