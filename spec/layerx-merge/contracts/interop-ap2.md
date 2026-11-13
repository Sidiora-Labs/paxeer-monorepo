# Interop AP2 contract

The [AP2 adapter](../../../services/interop/crates/layerx-ap2/src/lib.rs) verifies signed mandates and their constraints, authorizes payments and produces signed payment and checkout evidence. Receipt verification uses independently pinned receipt keys.

## ap2-adapter-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-adapter-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub enum AdapterOutcome</code> |

## ap2-ap2-adapter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-adapter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub struct Ap2Adapter&lt;'a, R&gt;</code> |

## ap2-ap2-adapter-descriptor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-adapter-descriptor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/lib.rs](../../../services/interop/crates/layerx-ap2/src/lib.rs) | <code>pub fn ap2_adapter_descriptor(conformance: ConformanceSuite) -&gt; Result&lt;AdapterDescriptor, Ap2Error&gt;</code> |

## ap2-ap2-adapter-execute

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-adapter-execute</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub fn execute(&amp;self, gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, checkout_presentation: &amp;str, payment_presentation: &amp;str, verification: &amp;VerificationContext&lt;'_&gt;, binding: &amp;LayerXAssetBinding, plane: &amp;mut impl LayerXIntentPlane, payment_signer: &amp;mut impl ReceiptSigner, checkout_signer: &amp;mut impl ReceiptSigner, trace: &amp;TraceId, now: u64) -&gt; Result&lt;AdapterOutcome, Traced&lt;Ap2Error&gt;&gt;</code> |

## ap2-ap2-adapter-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-adapter-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub const fn new(resolver: &amp;'a R, receipt_keys: &amp;'a [SequencerAuthorization]) -&gt; Self</code> |

## ap2-ap2-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/error.rs](../../../services/interop/crates/layerx-ap2/src/error.rs) | <code>pub enum Ap2Error</code> |

## ap2-ap2-external-mandate-verifier

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-external-mandate-verifier</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/portable.rs](../../../services/interop/crates/layerx-ap2/src/portable.rs) | <code>pub struct Ap2ExternalMandateVerifier&lt;'a, R&gt;</code> |

## ap2-ap2-external-mandate-verifier-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-external-mandate-verifier-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/portable.rs](../../../services/interop/crates/layerx-ap2/src/portable.rs) | <code>pub fn new(resolver: &amp;'a R, conformance: ConformanceSuite) -&gt; Result&lt;Self, Ap2Error&gt;</code> |

## ap2-ap2-mandate-pair

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-mandate-pair</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/portable.rs](../../../services/interop/crates/layerx-ap2/src/portable.rs) | <code>pub struct Ap2MandatePair&lt;'a&gt;</code> |

## ap2-ap2-mandate-pair-media-type

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-mandate-pair-media-type</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/portable.rs](../../../services/interop/crates/layerx-ap2/src/portable.rs) | <code>pub const AP2_MANDATE_PAIR_MEDIA_TYPE: &amp;str</code> |

## ap2-ap2-mandate-pair-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-mandate-pair-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/portable.rs](../../../services/interop/crates/layerx-ap2/src/portable.rs) | <code>pub const fn new(checkout_mandate: &amp;'a str, payment_mandate: &amp;'a str) -&gt; Self</code> |

## ap2-ap2-mandate-pair-to-json

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-mandate-pair-to-json</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/portable.rs](../../../services/interop/crates/layerx-ap2/src/portable.rs) | <code>pub fn to_json(&amp;self) -&gt; Result&lt;Vec&lt;u8&gt;, Ap2Error&gt;</code> |

## ap2-ap2-spec-commit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-spec-commit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/lib.rs](../../../services/interop/crates/layerx-ap2/src/lib.rs) | <code>pub const AP2_SPEC_COMMIT: &amp;str</code> |

## ap2-ap2-spec-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-spec-sha256</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/lib.rs](../../../services/interop/crates/layerx-ap2/src/lib.rs) | <code>pub const AP2_SPEC_SHA256: [u8; 32]</code> |

## ap2-authorize-payment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-authorize-payment</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub fn authorize_payment(principal: &amp;PrincipalId, mandates: &amp;VerifiedMandates, verification: &amp;VerificationContext&lt;'_&gt;, binding: &amp;LayerXAssetBinding) -&gt; Result&lt;AuthorizedPayment, Ap2Error&gt;</code> |

## ap2-authorized-payment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub struct AuthorizedPayment</code> |

## ap2-authorized-payment-amount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-amount</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub const fn amount(&amp;self) -&gt; u128</code> |

## ap2-authorized-payment-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-asset</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub const fn asset(&amp;self) -&gt; [u8; 32]</code> |

## ap2-authorized-payment-checkout-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-checkout-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub fn checkout_id(&amp;self) -&gt; &amp;str</code> |

## ap2-authorized-payment-execution-at

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-execution-at</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub const fn execution_at(&amp;self) -&gt; u64</code> |

## ap2-authorized-payment-idempotency-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-idempotency-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub const fn idempotency_key(&amp;self) -&gt; [u8; 32]</code> |

## ap2-authorized-payment-payee-receipt-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-payee-receipt-account</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub const fn payee_receipt_account(&amp;self) -&gt; [u8; 32]</code> |

## ap2-authorized-payment-payer-receipt-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-payer-receipt-account</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub const fn payer_receipt_account(&amp;self) -&gt; [u8; 32]</code> |

## ap2-authorized-payment-principal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-principal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub const fn principal(&amp;self) -&gt; &amp;PrincipalId</code> |

## ap2-authorized-payment-request-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-request-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub const fn request_digest(&amp;self) -&gt; [u8; 32]</code> |

## ap2-authorized-payment-transaction-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-transaction-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub fn transaction_id(&amp;self) -&gt; &amp;str</code> |

## ap2-executed-payment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-executed-payment</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub struct ExecutedPayment</code> |

## ap2-interop-ap2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-interop-ap2</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/lib.rs](../../../services/interop/crates/layerx-ap2/src/lib.rs) | <code>pub const fn interop_ap2() -&gt; &amp;'static str</code> |

## ap2-key-resolver

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-key-resolver</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/jose.rs](../../../services/interop/crates/layerx-ap2/src/jose.rs) | <code>pub trait KeyResolver</code> |

## ap2-key-use

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-key-use</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/jose.rs](../../../services/interop/crates/layerx-ap2/src/jose.rs) | <code>pub enum KeyUse</code> |

## ap2-layerx-asset-binding

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-layerx-asset-binding</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub struct LayerXAssetBinding</code> |

## ap2-layerx-intent-plane

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-layerx-intent-plane</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub trait LayerXIntentPlane</code> |

## ap2-mandate-mode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-mandate-mode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub enum MandateMode</code> |

## ap2-mandate-usage

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-mandate-usage</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub struct MandateUsage</code> |

## ap2-mandate-verifier

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-mandate-verifier</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub struct MandateVerifier&lt;'a, R&gt;</code> |

## ap2-mandate-verifier-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-mandate-verifier-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub const fn new(resolver: &amp;'a R) -&gt; Self</code> |

## ap2-mandate-verifier-verify

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-mandate-verifier-verify</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub fn verify(&amp;self, checkout_presentation: &amp;str, payment_presentation: &amp;str, context: &amp;VerificationContext&lt;'_&gt;) -&gt; Result&lt;VerifiedMandates, Ap2Error&gt;</code> |

## ap2-merchant

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-merchant</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>pub struct Merchant</code> |

## ap2-merchant-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-merchant-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>pub fn id(&amp;self) -&gt; &amp;str</code> |

## ap2-merchant-name

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-merchant-name</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>pub fn name(&amp;self) -&gt; &amp;str</code> |

## ap2-merchant-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-merchant-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>pub fn new(id: impl Into&lt;String&gt;, name: impl Into&lt;String&gt;, website: Option&lt;String&gt;) -&gt; Result&lt;Self, Ap2Error&gt;</code> |

## ap2-merchant-website

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-merchant-website</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>pub fn website(&amp;self) -&gt; Option&lt;&amp;str&gt;</code> |

## ap2-payment-amount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-payment-amount</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>pub struct PaymentAmount</code> |

## ap2-payment-amount-currency

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-payment-amount-currency</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>pub fn currency(&amp;self) -&gt; &amp;str</code> |

## ap2-payment-amount-minor-units

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-payment-amount-minor-units</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>pub const fn minor_units(&amp;self) -&gt; u128</code> |

## ap2-payment-instrument

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-payment-instrument</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>pub struct PaymentInstrument</code> |

## ap2-payment-instrument-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-payment-instrument-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>pub fn id(&amp;self) -&gt; &amp;str</code> |

## ap2-payment-instrument-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-payment-instrument-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>pub fn kind(&amp;self) -&gt; &amp;str</code> |

## ap2-plane-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-plane-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub enum PlaneOutcome</code> |

## ap2-portable-layerx-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-portable-layerx-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub type PortableLayerXEvidence = PortableReceipt</code> |

## ap2-protected-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-protected-header</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/jose.rs](../../../services/interop/crates/layerx-ap2/src/jose.rs) | <code>pub struct ProtectedHeader</code> |

## ap2-protected-header-algorithm

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-protected-header-algorithm</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/jose.rs](../../../services/interop/crates/layerx-ap2/src/jose.rs) | <code>pub fn algorithm(&amp;self) -&gt; &amp;str</code> |

## ap2-protected-header-certificate-chain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-protected-header-certificate-chain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/jose.rs](../../../services/interop/crates/layerx-ap2/src/jose.rs) | <code>pub fn certificate_chain(&amp;self) -&gt; Option&lt;&amp;[String]&gt;</code> |

## ap2-protected-header-key-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-protected-header-key-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/jose.rs](../../../services/interop/crates/layerx-ap2/src/jose.rs) | <code>pub fn key_id(&amp;self) -&gt; Option&lt;&amp;str&gt;</code> |

## ap2-protected-header-token-type

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-protected-header-token-type</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/jose.rs](../../../services/interop/crates/layerx-ap2/src/jose.rs) | <code>pub fn token_type(&amp;self) -&gt; Option&lt;&amp;str&gt;</code> |

## ap2-receipt-signer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-receipt-signer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub trait ReceiptSigner</code> |

## ap2-signed-ap2-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-signed-ap2-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>pub struct SignedAp2Evidence</code> |

## ap2-verification-context

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-verification-context</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub struct VerificationContext&lt;'a&gt;</code> |

## ap2-verified-mandates

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub struct VerifiedMandates</code> |

## ap2-verified-mandates-amount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-amount</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub fn amount(&amp;self) -&gt; &amp;crate::model::PaymentAmount</code> |

## ap2-verified-mandates-checkout-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-checkout-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub fn checkout_id(&amp;self) -&gt; &amp;str</code> |

## ap2-verified-mandates-checkout-receipt-reference

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-checkout-receipt-reference</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub fn checkout_receipt_reference(&amp;self) -&gt; String</code> |

## ap2-verified-mandates-execution-at

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-execution-at</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub const fn execution_at(&amp;self) -&gt; u64</code> |

## ap2-verified-mandates-mode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-mode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub const fn mode(&amp;self) -&gt; MandateMode</code> |

## ap2-verified-mandates-payee

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-payee</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub fn payee(&amp;self) -&gt; &amp;crate::model::Merchant</code> |

## ap2-verified-mandates-payment-instrument

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-payment-instrument</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub fn payment_instrument(&amp;self) -&gt; &amp;crate::model::PaymentInstrument</code> |

## ap2-verified-mandates-payment-receipt-reference

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-payment-receipt-reference</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub fn payment_receipt_reference(&amp;self) -&gt; String</code> |

## ap2-verified-mandates-transaction-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-transaction-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ap2/src/verify.rs](../../../services/interop/crates/layerx-ap2/src/verify.rs) | <code>pub fn transaction_id(&amp;self) -&gt; &amp;str</code> |

## fmt-ap2-agent-recurrence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-agent-recurrence</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct AgentRecurrence { pub(crate) frequency: String, #[serde(default)] pub(crate) max_occurrences: Option&lt;u64&gt;, }</code> |

## fmt-ap2-allowed-merchants

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-allowed-merchants</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct AllowedMerchants { pub(crate) allowed: Vec&lt;Merchant&gt;, }</code> |

## fmt-ap2-allowed-payees

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-allowed-payees</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct AllowedPayees { pub(crate) allowed: Vec&lt;Merchant&gt;, }</code> |

## fmt-ap2-allowed-payment-instruments

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-allowed-payment-instruments</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct AllowedPaymentInstruments { pub(crate) allowed: Vec&lt;PaymentInstrument&gt;, }</code> |

## fmt-ap2-allowed-pisps

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-allowed-pisps</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct AllowedPisps { pub(crate) allowed: Vec&lt;Pisp&gt;, }</code> |

## fmt-ap2-amount-range

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-amount-range</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct AmountRange { pub(crate) currency: String, pub(crate) max: u128, #[serde(default)] pub(crate) min: Option&lt;u128&gt;, }</code> |

## fmt-ap2-ap2-mandate-pair

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-ap2-mandate-pair</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/portable.rs](../../../services/interop/crates/layerx-ap2/src/portable.rs) | <code>Rust model: #[serde(rename_all = "camelCase")] pub struct Ap2MandatePair&lt;'a&gt; { checkout_mandate: &amp;'a str, payment_mandate: &amp;'a str, }</code> |

## fmt-ap2-budget

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-budget</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct Budget { pub(crate) max: Number, pub(crate) currency: String, }</code> |

## fmt-ap2-checkout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-checkout</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct Checkout { pub(crate) id: String, #[serde(default)] pub(crate) merchant: Option&lt;Merchant&gt;, pub(crate) line_items: Vec&lt;CheckoutLineItem&gt;, pub(crate) status: String, pub(crate) currency: String, pub(crate) totals: Vec&lt;Total&gt;, pub(crate) links: Vec&lt;Value&gt;, }</code> |

## fmt-ap2-checkout-line-item

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-checkout-line-item</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct CheckoutLineItem { pub(crate) id: String, pub(crate) item: Item, pub(crate) quantity: u64, pub(crate) totals: Vec&lt;Total&gt;, }</code> |

## fmt-ap2-checkout-receipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-checkout-receipt</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>Rust model: struct CheckoutReceipt { status: &amp;'static str, iss: String, iat: u64, reference: String, order_id: String, layerx_receipt_reference: String, layerx_evidence: PortableLayerXEvidence, }</code> |

## fmt-ap2-closed-checkout-mandate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-closed-checkout-mandate</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct ClosedCheckoutMandate { pub(crate) vct: String, pub(crate) checkout_jwt: String, pub(crate) checkout_hash: String, #[serde(default)] pub(crate) iat: Option&lt;u64&gt;, #[serde(default)] pub(crate) exp: Option&lt;u64&gt;, }</code> |

## fmt-ap2-closed-payment-mandate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-closed-payment-mandate</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct ClosedPaymentMandate { pub(crate) vct: String, pub(crate) transaction_id: String, pub(crate) payee: Merchant, #[serde(default)] pub(crate) pisp: Option&lt;Pisp&gt;, pub(crate) payment_amount: PaymentAmount, pub(crate) payment_instrument: PaymentInstrument, #[serde(default)] pub(crate) execution_date: Option&lt;String&gt;, #[serde(default)] pub(crate) iat: Option&lt;u64&gt;, #[serde(default)] pub(crate) exp: Option&lt;u64&gt;, }</code> |

## fmt-ap2-constraint-item

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-constraint-item</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct ConstraintItem { pub(crate) id: String, pub(crate) title: String, }</code> |

## fmt-ap2-execution-date

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-execution-date</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct ExecutionDate { #[serde(default)] pub(crate) not_before: Option&lt;String&gt;, #[serde(default)] pub(crate) not_after: Option&lt;String&gt;, }</code> |

## fmt-ap2-item

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-item</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct Item { pub(crate) id: String, pub(crate) title: String, pub(crate) price: u128, }</code> |

## fmt-ap2-line-item-requirement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-line-item-requirement</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct LineItemRequirement { pub(crate) id: String, pub(crate) acceptable_items: Vec&lt;ConstraintItem&gt;, pub(crate) quantity: u64, }</code> |

## fmt-ap2-line-items-constraint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-line-items-constraint</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct LineItemsConstraint { pub(crate) items: Vec&lt;LineItemRequirement&gt;, }</code> |

## fmt-ap2-merchant

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-merchant</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub struct Merchant { pub(crate) id: String, pub(crate) name: String, #[serde(default)] pub(crate) website: Option&lt;String&gt;, }</code> |

## fmt-ap2-open-checkout-mandate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-open-checkout-mandate</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct OpenCheckoutMandate { pub(crate) vct: String, pub(crate) constraints: Vec&lt;Value&gt;, pub(crate) cnf: Value, #[serde(default)] pub(crate) iat: Option&lt;u64&gt;, #[serde(default)] pub(crate) exp: Option&lt;u64&gt;, }</code> |

## fmt-ap2-open-payment-mandate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-open-payment-mandate</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct OpenPaymentMandate { pub(crate) vct: String, pub(crate) constraints: Vec&lt;Value&gt;, pub(crate) cnf: Value, #[serde(default)] pub(crate) payee: Option&lt;Merchant&gt;, #[serde(default)] pub(crate) payment_amount: Option&lt;PaymentAmount&gt;, #[serde(default)] pub(crate) payment_instrument: Option&lt;PaymentInstrument&gt;, #[serde(default)] pub(crate) pisp: Option&lt;Pisp&gt;, #[serde(default)] pub(crate) execution_date: Option&lt;String&gt;, #[serde(default)] pub(crate) iat: Option&lt;u64&gt;, #[serde(default)] pub(crate) exp: Option&lt;u64&gt;, }</code> |

## fmt-ap2-owned-mandate-pair

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-owned-mandate-pair</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/portable.rs](../../../services/interop/crates/layerx-ap2/src/portable.rs) | <code>Rust model: #[serde(rename_all = "camelCase", deny_unknown_fields)] struct OwnedMandatePair { checkout_mandate: String, payment_mandate: String, }</code> |

## fmt-ap2-payment-amount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-payment-amount</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub struct PaymentAmount { pub(crate) amount: u128, pub(crate) currency: String, }</code> |

## fmt-ap2-payment-instrument

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-payment-instrument</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub struct PaymentInstrument { pub(crate) id: String, #[serde(rename = "type")] pub(crate) kind: String, #[serde(default)] pub(crate) description: Option&lt;String&gt;, }</code> |

## fmt-ap2-payment-receipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-payment-receipt</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>Rust model: struct PaymentReceipt { status: &amp;'static str, iss: String, iat: u64, reference: String, #[serde(skip_serializing_if = "Option::is_none")] psp_confirmation_id: Option&lt;String&gt;, #[serde(skip_serializing_if = "Option::is_none")] network_confirmation_id: Option&lt;String&gt;, layerx_receipt_reference: String, layerx_evidence: PortableLayerXEvidence, }</code> |

## fmt-ap2-payment-reference

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-payment-reference</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct PaymentReference { pub(crate) conditional_transaction_id: String, }</code> |

## fmt-ap2-pisp

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-pisp</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub struct Pisp { #[serde(rename = "legal_name")] pub(crate) legal: String, #[serde(rename = "brand_name")] pub(crate) brand: String, #[serde(rename = "domain_name")] pub(crate) domain: String, }</code> |

## fmt-ap2-protected-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-protected-header</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/jose.rs](../../../services/interop/crates/layerx-ap2/src/jose.rs) | <code>Rust model: pub struct ProtectedHeader { alg: String, #[serde(default)] typ: Option&lt;String&gt;, #[serde(default)] kid: Option&lt;String&gt;, #[serde(default)] x5c: Option&lt;Vec&lt;String&gt;&gt;, #[serde(default)] crit: Option&lt;Vec&lt;String&gt;&gt;, #[serde(default)] b64: Option&lt;bool&gt;, }</code> |

## fmt-ap2-receipt-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-receipt-header</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/adapter.rs](../../../services/interop/crates/layerx-ap2/src/adapter.rs) | <code>Rust model: struct ReceiptHeader&lt;'a&gt; { alg: &amp;'static str, typ: &amp;'static str, kid: &amp;'a str, }</code> |

## fmt-ap2-total

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-ap2-total</code> | <code>file_format</code> | [services/interop/crates/layerx-ap2/src/model.rs](../../../services/interop/crates/layerx-ap2/src/model.rs) | <code>Rust model: pub(crate) struct Total { #[serde(rename = "type")] pub(crate) kind: String, pub(crate) amount: i128, }</code> |
