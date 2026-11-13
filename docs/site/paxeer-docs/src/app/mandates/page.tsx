import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/mandates')

export default function Mandates() {
  return <DocsLayout>
    <div className="page-header">
      <h1 className="page-title">Payment mandates and trusted agents</h1>
      <p className="page-description">Authorize direct and autonomous purchases with signed AP2 mandates, exact asset bindings, and trusted-agent request verification.</p>
    </div>
    <h2>Separate the agent, the authorization, and the payment</h2>
    <p>Paxeer X verifies who presents a commerce request, what the payer authorizes, and what payment executes. Visa Trusted Agent Protocol supplies signed agent credentials and intent tags. AP2 supplies signed mandate constraints and checkout bindings. The interoperability service binds verified authorization to a payer-signed EVM payment and returns verified settlement evidence. Each step retains its own verification boundary.</p>
    <h2>AP2 v1 mandate verification</h2>
    <p>The Rust AP2 verifier in services/interop/crates/layerx-ap2 verifies SD-JWT and JOSE mandate material and evaluates its constraints before preparing payment. Direct mode handles direct authorization; autonomous mode evaluates delegated purchasing authorization, including the applicable line-item constraints. Verification binds payment and checkout identities, merchant details, amount, currency, payment instrument, audience, nonce, and validity conditions.</p>
    <table><thead><tr><th>Verification input</th><th>Purpose</th></tr></thead><tbody>
      <tr><td><code>now</code> and <code>clock_skew_seconds</code></td><td>Evaluate validity windows with an explicit clock and permitted skew.</td></tr>
      <tr><td><code>expected_audience</code></td><td>Require the mandate to target the expected audience.</td></tr>
      <tr><td><code>expected_nonce</code></td><td>Bind verification to the expected authorization context.</td></tr>
      <tr><td><code>currency_minor_exponent</code></td><td>Interpret the payment currency at its correct minor-unit precision.</td></tr>
      <tr><td><code>usage</code></td><td>Evaluate an open mandate against caller-supplied prior-use facts in the Rust verifier.</td></tr>
    </tbody></table>
    <h3>Bounded autonomous authorization</h3>
    <p>For an open payment mandate, retain a usage record with its exact mandate reference, frequency, previous occurrence count, amount already spent in minor units, and next eligible execution time. The reference is the AP2 <code>sd_hash</code> of that specific mandate, preventing usage facts from being applied to a different authorization. Supply these facts explicitly to the Rust verifier for the next purchase. The hosted HTTP handler supplies no usage record and therefore cannot verify constraints that require prior-use facts.</p>
    <p>The verification context requires a positive timestamp, nonempty audience and nonce, a currency minor exponent no greater than 18, and clock skew no greater than 3,600 seconds. These are verifier input bounds enforced before constraint evaluation; the mandate still has to pass its signed constraints. The adapter profile is AP2 v1 (<code>ap2-v1-signature-constraint-receipt-verified</code>).</p>
    <h2>Bind authorization to actual settlement</h2>
    <p>A verified mandate becomes an authorized payment only after its merchant and currency match the configured LayerX asset binding. The binding identifies the asset, payer and payee receipt accounts, minor-unit exponent, and atomic units per currency minor unit. Amount conversion uses checked multiplication and rejects overflow or a zero result.</p>
    <pre><code>{`asset atomic amount = mandate minor units × atomic units per minor unit
idempotency identity = principal + stable payment reference
request binding = mandate material + currency + asset
                + payer account + payee account + amount`}</code></pre>
    <p>The idempotency identity remains stable across retries of the same principal and payment reference. The request digest commits to the signed authorization material and settlement binding. An explicit execution date is already bound by the signed presentations; a locally resolved execution time does not change the digest on retry.</p>
    <h2>Interoperability service routes</h2>
    <table><thead><tr><th>HTTP route</th><th>Operation</th></tr></thead><tbody>
      <tr><td><code>POST /v1/http/ap2/mandates/verify</code></td><td>Verify signed mandate material and its constraint context.</td></tr>
      <tr><td><code>POST /v1/http/ap2/execute</code></td><td>Validate and relay the payer-signed payment, then export signed receipt evidence.</td></tr>
      <tr><td><code>POST /v1/http/visa-tap/intents/verify</code></td><td>Verify a trusted-agent request.</td></tr>
      <tr><td><code>POST /v1/http/visa-tap/intents/execute</code></td><td>Verify the trusted-agent request and relay its authorized payment.</td></tr>
    </tbody></table>
    <ol>
      <li>Send checkout_presentation, payment_presentation, and nonce. The HTTP handler uses its server clock, configured audience and currency precision, and zero clock skew to verify the mandate pair.</li>
      <li>Evaluate the signed merchant, amount, checkout, and applicable autonomous constraints.</li>
      <li>Apply the asset and receipt-account binding configured for the authenticated principal.</li>
      <li>For execution, include activity as hex-encoded, payer-signed EIP-1559 transaction bytes. The service checks the payment accounts, asset, amount, reference and expiry before relay.</li>
      <li>A pending relay returns HTTP 202. Once execution evidence verifies, the configured receipt signers export payment_receipt_jwt and checkout_receipt_jwt.</li>
    </ol>
    <h2>Visa Trusted Agent Protocol</h2>
    <p>The TAP adapter verifies RFC 9421-style <code>sig2</code> request signatures against a trusted-agent registry. The <code>agent-browser-auth</code> tag identifies browsing intent and <code>agent-payer-auth</code> identifies payment intent. Verification checks the validity window, registered key status, signature algorithm, signed request target, cryptographic signature, and nonce uniqueness.</p>
    <h3>Replay protection and credential binding</h3>
    <p>A nonce window consumes the nonce together with its key identifier and expiry. A verified credential can be associated with a LayerX agent through the credential-binding store. That association records identity context; payment authority still requires the matching payer-signed transaction. A valid agent signature does not by itself supply the payer&apos;s mandate or authorize an arbitrary transfer.</p>
    <p>Maintain the registry&apos;s key status and validity information, use canonical authority and path representations for signature verification, and preserve nonce state across the merchant request lifecycle. Browse and pay operations retain distinct intent tags so an operation is evaluated in its declared context.</p>
    <h2>Portable purchase evidence</h2>
    <p>Completed AP2 execution returns signed evidence containing portable LayerX receipt evidence. Payment and checkout receipt references preserve the exact verified authorization bindings. The portable mandate-pair format carries the external evidence needed by <code>Ap2ExternalMandateVerifier</code>, allowing verification to travel with the purchase evidence.</p>
    <p>Retain the signed mandate pair, signed payment transaction, canonical execution receipt, independent receipt-key trust configuration, merchant checkout identity, and applicable mandate usage facts. Keep raw payment credentials out of operational logs. Use <Link href="/commerce">agent commerce</Link> to connect this authorization flow to merchant checkout, order reads, and paid-resource fulfillment.</p>
  </DocsLayout>
}
