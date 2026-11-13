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
      <p>Paxeer X settles native Asset payments on one BFT chain. Applications quote a move, commit the accepted quote under a stable idempotency key, and track a journey whose outcome is backed by evidence. The native wallet prepares a ledger precompile call, discloses its terms, signs an EIP-1559 transaction, submits it to the node and verifies receipt progress.</p>
      <ol>
        <li><strong>Quote:</strong> identify payer, recipient, Asset or currency, amount, mechanism, fee ceiling, expected arrival, and irreversible legs.</li>
        <li><strong>Approve and commit:</strong> bind approval to those terms. An expired quote returns <code>quote-expired</code> rather than silently changing the payment.</li>
        <li><strong>Track:</strong> retain the journey or activity identifier and idempotency key through interruptions.</li>
        <li><strong>Verify and fulfil:</strong> validate canonical evidence and payment facts before delivering goods or releasing a protected resource.</li>
      </ol>
      <h2>Choose a payment experience</h2>
      <table><thead><tr><th>Experience</th><th>Flow</th><th>Settlement rule</th></tr></thead><tbody>
        <tr><td>Wallet or application transfer</td><td>Quote and commit a move, or sign a ledger payment transaction</td><td>Verify the receipt and selected commitment</td></tr>
        <tr><td>Paid HTTP resource</td><td>x402 exact offer and receipt-backed retry</td><td>Release only after the offer is paid and verified</td></tr>
        <tr><td>Merchant cart</td><td>Catalog quote, persistent order, payment, fulfilment</td><td>Bind order and settlement to the same request digest</td></tr>
        <tr><td>Metered usage</td><td>Ledger grant and authorized executor draws</td><td>Validate purpose, grant limits, expiry, and each draw</td></tr>
        <tr><td>Subscription</td><td>Recurring grant with a declared billing window</td><td>Signed draw and distinct period key for every renewal</td></tr>
      </tbody></table>
      <h2>Journey states</h2>
      <table><thead><tr><th>State</th><th>Application behavior</th></tr></thead><tbody>
        <tr><td><code>getting-ready</code></td><td>The payment is accepted and its legs are being prepared.</td></tr>
        <tr><td><code>sending</code></td><td>At least one leg is in flight.</td></tr>
        <tr><td><code>waiting-for-you</code></td><td>Request the required human decision before its deterministic expiry.</td></tr>
        <tr><td><code>processing</code></td><td>The payment is submitted and settling.</td></tr>
        <tr><td><code>still-checking</code></td><td>The outcome is unknown. Keep duplicate-capable controls locked and resolve the original payment.</td></tr>
        <tr><td><code>done</code></td><td>Settlement is backed by a verified receipt or finality proof.</td></tr>
        <tr><td><code>done-finalised</code></td><td>The human API uses this label for receipt-backed completion with committed evidence. Inspect the evidence as well as the journey state; a <code>done</code> journey can also carry committed evidence.</td></tr>
        <tr><td><code>refused</code></td><td>Inspect <code>refused_by</code> and <code>money_left</code> before reporting the result.</td></tr>
        <tr><td><code>awaiting-approval</code></td><td>The native engine exposes its awaiting-approval leg phase while the journey awaits approval. It is not receipt-backed completion.</td></tr>
        <tr><td><code>closed</code></td><td>A closed journey is read-only and cannot resume signing or execution. Inspect its evidence to determine the movement outcome.</td></tr>
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
      <p>A metered or subscription challenge binds <code>extra.layerx.payer</code>, <code>purposeHash</code>, and <code>commitment</code>; subscriptions also bind <code>windowSeconds</code>. A payer authorizes a ledger grant for a specific executor. The executor signs an EIP-1559 <code>draw</code> call to <code>0x101A</code> for each charge. The seller checks the active grant, executor, source account, recipient, Asset, amount, purpose and network against the offer, then verifies the receipt and payment log.</p>
      <p>Grant limits and expiry are enforced by the ledger. Keep one persistent idempotency key for each usage charge or subscription period. The middleware binds that key to the signed draw. A pending payment returns HTTP 202 while the original transaction is tracked; release the resource after the required evidence is verified.</p>
      <h2>Commitments and evidence</h2>
      <table><thead><tr><th>Commitment</th><th>Required evidence</th></tr></thead><tbody>
        <tr><td><code>submitted</code></td><td>An accepted transaction hash; execution is not yet established.</td></tr>
        <tr><td><code>receipted</code></td><td>A verified signed protocol receipt bound to the transaction, successful result and expected payment log.</td></tr>
        <tr><td><code>committed</code></td><td>Receipt inclusion in a committed block, its signed consensus commit and a validator set matched to an independently trusted validator hash.</td></tr>
      </tbody></table>
      <p>x402 accepts <code>receipted</code> or <code>committed</code> in <code>extra.layerx.commitment</code>; it defaults to <code>receipted</code> when the LayerX terms are absent. Only committed block evidence establishes finality. Successful settlement uses the <code>lxp:&lt;receipt_digest&gt;</code> reference. The middleware verifies the signed raw transaction, receipt signature, receipt digest and payment facts, plus consensus evidence when committed settlement is requested.</p>
      <h2>Retries, sequences, and unknown outcomes</h2>
      <p>Choose one stable key per purchase or order. Repeating a human-plane mutation under that key returns its original journey; a different body under the same key is <code>idempotency-conflict</code>. Native wallet sends consume the EVM sender nonce and bind debit authorization to payment account state. Retain the signed transaction and its hash when a response is lost. Recover its receipt and journaled submission before creating a new payment.</p>
      <p>On the human plane, retriabilities are <code>retriable</code>, <code>retriable-after</code>, <code>structural</code>, and <code>final</code>. The SDK <code>ProductionClient</code> reports its own retry classes: <code>never</code>, <code>safe</code>, <code>after</code>, and <code>unknown-outcome</code>. Map between the two vocabularies explicitly rather than treating them as one list. Respect carried rate-limit timing. An <code>unknown-outcome</code> result or human-plane <code>still-checking</code> requires tracking and evidence recovery, with duplicate controls locked.</p>
      <h2>Fees and refusal</h2>
      <p>EVM payment transactions pay gas in the native asset. The CLI queries gas prices and estimates the exact ledger call at pending state, then checks the estimate against <code>--fee-limit</code> in wei. Keep enough native balance for gas as well as enough Asset balance for the transfer.</p>
      <p>A transaction hash or pending response does not establish a successful debit. Inspect the execution result, payment event and requested receipt rung. An executed EVM transaction can consume its nonce and gas even when its call reverts. Program fees and occupancy charges have their own accounting rules; see the fee guide.</p>
      <h2>Continue building</h2>
      <ul><li><Link href="/payments-quickstart">Make your first payment</Link></li><li><Link href="/assets">Assets and token accounts</Link></li><li><Link href="/platform-api">Public payment API</Link></li><li><Link href="/paid-apis">x402 transport</Link></li><li><Link href="/fees">Fee schedules</Link></li></ul>
    </DocsLayout>
  )
}
