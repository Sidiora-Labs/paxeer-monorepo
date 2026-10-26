# Interop ap2 contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## ap2-adapter-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-adapter-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:198</code> | <code>pub enum AdapterOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-adapter-outcome</code> | notes | enum layerx_ap2::adapter::AdapterOutcome; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:10; no reference outside the defining file found by name match |

## ap2-ap2-adapter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-adapter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:214</code> | <code>pub struct Ap2Adapter&lt;'a, R&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-adapter</code> | notes | struct layerx_ap2::adapter::Ap2Adapter; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:10; no reference outside the defining file found by name match |

## ap2-ap2-adapter-descriptor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-adapter-descriptor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/lib.rs:42</code> | <code>pub fn ap2_adapter_descriptor(conformance: ConformanceSuite) -&gt; Result&lt;AdapterDescriptor, Ap2Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-adapter-descriptor</code> | notes | fn layerx_ap2::ap2_adapter_descriptor |

## ap2-ap2-adapter-execute

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-adapter-execute</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:234</code> | <code>pub fn execute(&amp;self, gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, checkout_presentation: &amp;str, payment_presentation: &amp;str, verification: &amp;VerificationContext&lt;'_&gt;, binding: &amp;LayerXAssetBinding, plane: &amp;mut impl LayerXIntentPlane, payment_signer: &amp;mut impl ReceiptSigner, checkout_signer: &amp;mut impl ReceiptSigner, trace: &amp;TraceId, now: u64) -&gt; Result&lt;AdapterOutcome, Traced&lt;Ap2Error&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-adapter-execute</code> | notes | method layerx_ap2::adapter::Ap2Adapter::execute; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 7.5.18 (in_progress) replaces authorised batch verification (services/interop/crates/layerx-ap2/src/adapter.rs:10) with ProtocolReceipt v2 and a 0x101A log match |

## ap2-ap2-adapter-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-adapter-new</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:220</code> | <code>pub const fn new(resolver: &amp;'a R) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-adapter-new</code> | notes | method layerx_ap2::adapter::Ap2Adapter::new; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 7.5.18 (in_progress) replaces authorised batch verification (services/interop/crates/layerx-ap2/src/adapter.rs:10) with ProtocolReceipt v2 and a 0x101A log match |

## ap2-ap2-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/error.rs:8</code> | <code>pub enum Ap2Error</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-error</code> | notes | enum layerx_ap2::error::Ap2Error; 266 reference lines in 8 files; callers lists the first reference in each file; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:15 |

## ap2-ap2-external-mandate-verifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-external-mandate-verifier</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/portable.rs:55</code> | <code>pub struct Ap2ExternalMandateVerifier&lt;'a, R&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-external-mandate-verifier</code> | notes | struct layerx_ap2::portable::Ap2ExternalMandateVerifier; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:18; no reference outside the defining file found by name match |

## ap2-ap2-external-mandate-verifier-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-external-mandate-verifier-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/portable.rs:67</code> | <code>pub fn new(resolver: &amp;'a R, conformance: ConformanceSuite) -&gt; Result&lt;Self, Ap2Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-external-mandate-verifier-new</code> | notes | method layerx_ap2::portable::Ap2ExternalMandateVerifier::new; no reference outside the defining file found by name match |

## ap2-ap2-mandate-pair

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-mandate-pair</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/portable.rs:18</code> | <code>pub struct Ap2MandatePair&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-mandate-pair</code> | notes | struct layerx_ap2::portable::Ap2MandatePair; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:18; no reference outside the defining file found by name match |

## ap2-ap2-mandate-pair-media-type

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-mandate-pair-media-type</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/portable.rs:12</code> | <code>pub const AP2_MANDATE_PAIR_MEDIA_TYPE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-mandate-pair-media-type</code> | notes | const layerx_ap2::portable::AP2_MANDATE_PAIR_MEDIA_TYPE; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:18; no reference outside the defining file found by name match |

## ap2-ap2-mandate-pair-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-mandate-pair-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/portable.rs:25</code> | <code>pub const fn new(checkout_mandate: &amp;'a str, payment_mandate: &amp;'a str) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-mandate-pair-new</code> | notes | method layerx_ap2::portable::Ap2MandatePair::new; no reference outside the defining file found by name match |

## ap2-ap2-mandate-pair-to-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-mandate-pair-to-json</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/portable.rs:37</code> | <code>pub fn to_json(&amp;self) -&gt; Result&lt;Vec&lt;u8&gt;, Ap2Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-mandate-pair-to-json</code> | notes | method layerx_ap2::portable::Ap2MandatePair::to_json; no reference outside the defining file found by name match |

## ap2-ap2-spec-commit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-spec-commit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/lib.rs:28</code> | <code>pub const AP2_SPEC_COMMIT: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-spec-commit</code> | notes | const layerx_ap2::AP2_SPEC_COMMIT |

## ap2-ap2-spec-sha256

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-ap2-spec-sha256</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/lib.rs:30</code> | <code>pub const AP2_SPEC_SHA256: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-ap2-spec-sha256</code> | notes | const layerx_ap2::AP2_SPEC_SHA256 |

## ap2-authorize-payment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-authorize-payment</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:392</code> | <code>pub fn authorize_payment(principal: &amp;PrincipalId, mandates: &amp;VerifiedMandates, verification: &amp;VerificationContext&lt;'_&gt;, binding: &amp;LayerXAssetBinding) -&gt; Result&lt;AuthorizedPayment, Ap2Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-authorize-payment</code> | notes | fn layerx_ap2::adapter::authorize_payment; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:10 |

## ap2-authorized-payment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:60</code> | <code>pub struct AuthorizedPayment</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment</code> | notes | struct layerx_ap2::adapter::AuthorizedPayment; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:10 |

## ap2-authorized-payment-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-amount</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:105</code> | <code>pub const fn amount(&amp;self) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-amount</code> | notes | method layerx_ap2::adapter::AuthorizedPayment::amount |

## ap2-authorized-payment-asset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-asset</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:90</code> | <code>pub const fn asset(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-asset</code> | notes | method layerx_ap2::adapter::AuthorizedPayment::asset |

## ap2-authorized-payment-checkout-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-checkout-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:85</code> | <code>pub fn checkout_id(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-checkout-id</code> | notes | method layerx_ap2::adapter::AuthorizedPayment::checkout_id |

## ap2-authorized-payment-execution-at

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-execution-at</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:110</code> | <code>pub const fn execution_at(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-execution-at</code> | notes | method layerx_ap2::adapter::AuthorizedPayment::execution_at |

## ap2-authorized-payment-idempotency-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-idempotency-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:115</code> | <code>pub const fn idempotency_key(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-idempotency-key</code> | notes | method layerx_ap2::adapter::AuthorizedPayment::idempotency_key |

## ap2-authorized-payment-payee-receipt-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-payee-receipt-account</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:100</code> | <code>pub const fn payee_receipt_account(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-payee-receipt-account</code> | notes | method layerx_ap2::adapter::AuthorizedPayment::payee_receipt_account |

## ap2-authorized-payment-payer-receipt-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-payer-receipt-account</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:95</code> | <code>pub const fn payer_receipt_account(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-payer-receipt-account</code> | notes | method layerx_ap2::adapter::AuthorizedPayment::payer_receipt_account |

## ap2-authorized-payment-principal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-principal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:75</code> | <code>pub const fn principal(&amp;self) -&gt; &amp;PrincipalId</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-principal</code> | notes | method layerx_ap2::adapter::AuthorizedPayment::principal; no reference outside the defining file found by name match |

## ap2-authorized-payment-request-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-request-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:120</code> | <code>pub const fn request_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-request-digest</code> | notes | method layerx_ap2::adapter::AuthorizedPayment::request_digest; no reference outside the defining file found by name match |

## ap2-authorized-payment-transaction-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-transaction-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:80</code> | <code>pub fn transaction_id(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-authorized-payment-transaction-id</code> | notes | method layerx_ap2::adapter::AuthorizedPayment::transaction_id |

## ap2-executed-payment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-executed-payment</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:156</code> | <code>pub struct ExecutedPayment</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-executed-payment</code> | notes | struct layerx_ap2::adapter::ExecutedPayment; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:10; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 7.5.18 (in_progress) replaces authorised batch verification (services/interop/crates/layerx-ap2/src/adapter.rs:10) with ProtocolReceipt v2 and a 0x101A log match |

## ap2-interop-ap2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-interop-ap2</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/lib.rs:57</code> | <code>pub const fn interop_ap2() -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-interop-ap2</code> | notes | fn layerx_ap2::interop_ap2; no reference outside the defining file found by name match |

## ap2-key-resolver

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-key-resolver</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/jose.rs:102</code> | <code>pub trait KeyResolver</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-key-resolver</code> | notes | trait layerx_ap2::jose::KeyResolver; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:16 |

## ap2-key-use

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-key-use</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/jose.rs:22</code> | <code>pub enum KeyUse</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-key-use</code> | notes | enum layerx_ap2::jose::KeyUse; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:16 |

## ap2-layerx-asset-binding

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-layerx-asset-binding</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:30</code> | <code>pub struct LayerXAssetBinding</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-layerx-asset-binding</code> | notes | struct layerx_ap2::adapter::LayerXAssetBinding; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:10 |

## ap2-layerx-intent-plane

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-layerx-intent-plane</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:128</code> | <code>pub trait LayerXIntentPlane</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-layerx-intent-plane</code> | notes | trait layerx_ap2::adapter::LayerXIntentPlane; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:10; no reference outside the defining file found by name match |

## ap2-mandate-mode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-mandate-mode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:21</code> | <code>pub enum MandateMode</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-mandate-mode</code> | notes | enum layerx_ap2::verify::MandateMode; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:19 |

## ap2-mandate-usage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-mandate-usage</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:30</code> | <code>pub struct MandateUsage</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-mandate-usage</code> | notes | struct layerx_ap2::verify::MandateUsage; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:19; no reference outside the defining file found by name match |

## ap2-mandate-verifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-mandate-verifier</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:146</code> | <code>pub struct MandateVerifier&lt;'a, R&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-mandate-verifier</code> | notes | struct layerx_ap2::verify::MandateVerifier; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:19 |

## ap2-mandate-verifier-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-mandate-verifier-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:152</code> | <code>pub const fn new(resolver: &amp;'a R) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-mandate-verifier-new</code> | notes | method layerx_ap2::verify::MandateVerifier::new |

## ap2-mandate-verifier-verify

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-mandate-verifier-verify</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:165</code> | <code>pub fn verify(&amp;self, checkout_presentation: &amp;str, payment_presentation: &amp;str, context: &amp;VerificationContext&lt;'_&gt;) -&gt; Result&lt;VerifiedMandates, Ap2Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-mandate-verifier-verify</code> | notes | method layerx_ap2::verify::MandateVerifier::verify; no reference outside the defining file found by name match |

## ap2-merchant

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-merchant</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:12</code> | <code>pub struct Merchant</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-merchant</code> | notes | struct layerx_ap2::model::Merchant; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:17 |

## ap2-merchant-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-merchant-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:63</code> | <code>pub fn id(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-merchant-id</code> | notes | method layerx_ap2::model::Merchant::id; no reference outside the defining file found by name match |

## ap2-merchant-name

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-merchant-name</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:68</code> | <code>pub fn name(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-merchant-name</code> | notes | method layerx_ap2::model::Merchant::name; no reference outside the defining file found by name match |

## ap2-merchant-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-merchant-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:26</code> | <code>pub fn new(id: impl Into&lt;String&gt;, name: impl Into&lt;String&gt;, website: Option&lt;String&gt;) -&gt; Result&lt;Self, Ap2Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-merchant-new</code> | notes | method layerx_ap2::model::Merchant::new |

## ap2-merchant-website

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-merchant-website</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:73</code> | <code>pub fn website(&amp;self) -&gt; Option&lt;&amp;str&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-merchant-website</code> | notes | method layerx_ap2::model::Merchant::website; no reference outside the defining file found by name match |

## ap2-payment-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-payment-amount</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:79</code> | <code>pub struct PaymentAmount</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-payment-amount</code> | notes | struct layerx_ap2::model::PaymentAmount; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:17 |

## ap2-payment-amount-currency

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-payment-amount-currency</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:98</code> | <code>pub fn currency(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-payment-amount-currency</code> | notes | method layerx_ap2::model::PaymentAmount::currency; no reference outside the defining file found by name match |

## ap2-payment-amount-minor-units

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-payment-amount-minor-units</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:93</code> | <code>pub const fn minor_units(&amp;self) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-payment-amount-minor-units</code> | notes | method layerx_ap2::model::PaymentAmount::minor_units; no reference outside the defining file found by name match |

## ap2-payment-instrument

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-payment-instrument</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:104</code> | <code>pub struct PaymentInstrument</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-payment-instrument</code> | notes | struct layerx_ap2::model::PaymentInstrument; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:17 |

## ap2-payment-instrument-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-payment-instrument-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:127</code> | <code>pub fn id(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-payment-instrument-id</code> | notes | method layerx_ap2::model::PaymentInstrument::id; no reference outside the defining file found by name match |

## ap2-payment-instrument-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-payment-instrument-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:132</code> | <code>pub fn kind(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-payment-instrument-kind</code> | notes | method layerx_ap2::model::PaymentInstrument::kind; no reference outside the defining file found by name match |

## ap2-plane-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-plane-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:164</code> | <code>pub enum PlaneOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-plane-outcome</code> | notes | enum layerx_ap2::adapter::PlaneOutcome; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:10 |

## ap2-portable-layerx-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-portable-layerx-evidence</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:187</code> | <code>pub type PortableLayerXEvidence = PortableReceipt</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-portable-layerx-evidence</code> | notes | type alias layerx_ap2::adapter::PortableLayerXEvidence; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:10; no reference outside the defining file found by name match |
| <code>interface.11.rs-ap2-portable-layerx-evidence</code> | overridden_by | override.J47.6 |

## ap2-protected-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-protected-header</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/jose.rs:32</code> | <code>pub struct ProtectedHeader</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-protected-header</code> | notes | struct layerx_ap2::jose::ProtectedHeader; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:16 |

## ap2-protected-header-algorithm

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-protected-header-algorithm</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/jose.rs:48</code> | <code>pub fn algorithm(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-protected-header-algorithm</code> | notes | method layerx_ap2::jose::ProtectedHeader::algorithm; no reference outside the defining file found by name match |

## ap2-protected-header-certificate-chain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-protected-header-certificate-chain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/jose.rs:63</code> | <code>pub fn certificate_chain(&amp;self) -&gt; Option&lt;&amp;[String]&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-protected-header-certificate-chain</code> | notes | method layerx_ap2::jose::ProtectedHeader::certificate_chain |

## ap2-protected-header-key-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-protected-header-key-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/jose.rs:58</code> | <code>pub fn key_id(&amp;self) -&gt; Option&lt;&amp;str&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-protected-header-key-id</code> | notes | method layerx_ap2::jose::ProtectedHeader::key_id |

## ap2-protected-header-token-type

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-protected-header-token-type</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/jose.rs:53</code> | <code>pub fn token_type(&amp;self) -&gt; Option&lt;&amp;str&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-protected-header-token-type</code> | notes | method layerx_ap2::jose::ProtectedHeader::token_type; no reference outside the defining file found by name match |

## ap2-receipt-signer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-receipt-signer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:172</code> | <code>pub trait ReceiptSigner</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-receipt-signer</code> | notes | trait layerx_ap2::adapter::ReceiptSigner; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:10; no reference outside the defining file found by name match |

## ap2-sequencer-settlement

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.20.1</code> | req | 11020.1 |
| <code>acceptance.11.20.1</code> | scenario | Given a mandate matching the recorded payExact and a plane returning its receipted evidence, when execute runs, then the PaymentReceipt reference is lxp: followed by the recorded digest. |
| <code>acceptance.11.20.1</code> | test | services/interop/crates/layerx-ap2/tests/sequencer_settlement.rs::ap2_execute_settles_receipted_evidence (planned) |
| <code>acceptance.11.20.1</code> | qualification | UNRUN |
| <code>req.11020</code> | ac_1 | Ap2Adapter::execute settles the recorded receipt of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:109 and references lxp: and its receiptDigest (planned). |
| <code>acceptance.11.20.2</code> | req | 11020.2 |
| <code>acceptance.11.20.2</code> | scenario | Given a mandate whose payee differs from the recorded log recipient, when execute runs, then it returns the payee mismatch error and records nothing. |
| <code>acceptance.11.20.2</code> | test | services/interop/crates/layerx-ap2/tests/sequencer_settlement.rs::ap2_execute_refuses_a_payee_mismatch (planned) |
| <code>acceptance.11.20.2</code> | qualification | UNRUN |
| <code>req.11020</code> | ac_2 | A PaymentExact log paying another recipient is refused (planned) (services/interop/crates/layerx-ap2/src/adapter.rs:339). |
| <code>acceptance.11.31.1</code> | req | 11031.1 |
| <code>acceptance.11.31.1</code> | scenario | Given the recorded receipted evidence, when execute exports its portable evidence and from_json parses it, then the format is version 2 and the receipt bytes are the recorded signedReceipt. |
| <code>acceptance.11.31.1</code> | test | services/interop/crates/layerx-ap2/tests/sequencer_settlement.rs::ap2_portable_evidence_round_trips_as_v2 (planned) |
| <code>acceptance.11.31.1</code> | qualification | UNRUN |
| <code>req.11031</code> | ac_1 | The AP2 portable evidence is a version 2 document that from_json reads back (planned) (services/interop/crates/layerx-portable/src/receipt.rs:406). |
| <code>acceptance.11.57.1</code> | req | 11057.1 |
| <code>acceptance.11.57.1</code> | scenario | Given a mandate matching the recorded payExact, when execute runs, then the reference is lxp: and the recorded digest. |
| <code>acceptance.11.57.1</code> | test | services/interop/crates/layerx-ap2/tests/mandates.rs::mandate_payment_reference_is_the_recorded_digest (planned) |
| <code>acceptance.11.57.1</code> | qualification | UNRUN |
| <code>req.11057</code> | ac_1 | The AP2 mandate suite settles on the recorded receipt (planned) (services/interop/crates/layerx-ap2/src/adapter.rs:513). |
| <code>capability.11.7.1</code> | title | Settle the AP2 intent plane on SequencerEvidence |
| <code>capability.11.7.1</code> | reqs | 11020.1<br>11020.2 |
| <code>capability.11.7.2</code> | title | Export AP2 portable evidence as the version 2 portable receipt |
| <code>capability.11.7.2</code> | reqs | 11031.1 |
| <code>capability.11.7.3</code> | title | Move the AP2 mandate tests off AuthorizedBatch |
| <code>capability.11.7.3</code> | reqs | 11057.1 |

## ap2-signed-ap2-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-signed-ap2-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:190</code> | <code>pub struct SignedAp2Evidence</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-signed-ap2-evidence</code> | notes | struct layerx_ap2::adapter::SignedAp2Evidence; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:10; no reference outside the defining file found by name match |

## ap2-verification-context

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-verification-context</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:41</code> | <code>pub struct VerificationContext&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-verification-context</code> | notes | struct layerx_ap2::verify::VerificationContext; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:19 |

## ap2-verified-mandates

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:69</code> | <code>pub struct VerifiedMandates</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates</code> | notes | struct layerx_ap2::verify::VerifiedMandates; re-exported at services/interop/crates/layerx-ap2/src/lib.rs:19 |

## ap2-verified-mandates-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-amount</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:92</code> | <code>pub fn amount(&amp;self) -&gt; &amp;crate::model::PaymentAmount</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-amount</code> | notes | method layerx_ap2::verify::VerifiedMandates::amount |

## ap2-verified-mandates-checkout-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-checkout-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:107</code> | <code>pub fn checkout_id(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-checkout-id</code> | notes | method layerx_ap2::verify::VerifiedMandates::checkout_id |

## ap2-verified-mandates-checkout-receipt-reference

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-checkout-receipt-reference</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:122</code> | <code>pub fn checkout_receipt_reference(&amp;self) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-checkout-receipt-reference</code> | notes | method layerx_ap2::verify::VerifiedMandates::checkout_receipt_reference |

## ap2-verified-mandates-execution-at

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-execution-at</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:112</code> | <code>pub const fn execution_at(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-execution-at</code> | notes | method layerx_ap2::verify::VerifiedMandates::execution_at |

## ap2-verified-mandates-mode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-mode</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:82</code> | <code>pub const fn mode(&amp;self) -&gt; MandateMode</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-mode</code> | notes | method layerx_ap2::verify::VerifiedMandates::mode |

## ap2-verified-mandates-payee

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-payee</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:87</code> | <code>pub fn payee(&amp;self) -&gt; &amp;crate::model::Merchant</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-payee</code> | notes | method layerx_ap2::verify::VerifiedMandates::payee |

## ap2-verified-mandates-payment-instrument

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-payment-instrument</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:97</code> | <code>pub fn payment_instrument(&amp;self) -&gt; &amp;crate::model::PaymentInstrument</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-payment-instrument</code> | notes | method layerx_ap2::verify::VerifiedMandates::payment_instrument; no reference outside the defining file found by name match |

## ap2-verified-mandates-payment-receipt-reference

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-payment-receipt-reference</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:117</code> | <code>pub fn payment_receipt_reference(&amp;self) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-payment-receipt-reference</code> | notes | method layerx_ap2::verify::VerifiedMandates::payment_receipt_reference |

## ap2-verified-mandates-transaction-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-transaction-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/verify.rs:102</code> | <code>pub fn transaction_id(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ap2-verified-mandates-transaction-id</code> | notes | method layerx_ap2::verify::VerifiedMandates::transaction_id |

## fmt-ap2-agent-recurrence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-agent-recurrence</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:442</code> | <code>AgentRecurrence { frequency: String, max_occurrences?: Option&lt;u64&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-agent-recurrence</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-allowed-merchants

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-allowed-merchants</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:391</code> | <code>AllowedMerchants { allowed: Vec&lt;Merchant&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-allowed-merchants</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-allowed-payees

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-allowed-payees</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:422</code> | <code>AllowedPayees { allowed: Vec&lt;Merchant&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-allowed-payees</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-allowed-payment-instruments

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-allowed-payment-instruments</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:427</code> | <code>AllowedPaymentInstruments { allowed: Vec&lt;PaymentInstrument&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-allowed-payment-instruments</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-allowed-pisps

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-allowed-pisps</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:432</code> | <code>AllowedPisps { allowed: Vec&lt;Pisp&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-allowed-pisps</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-amount-range

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-amount-range</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:414</code> | <code>AmountRange { currency: String, max: u128, min?: Option&lt;u128&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-amount-range</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-ap2-mandate-pair

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-ap2-mandate-pair</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/portable.rs:18</code> | <code>Ap2MandatePair { checkoutMandate: &amp;'a str, paymentMandate: &amp;'a str }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-ap2-mandate-pair</code> | notes | AP2 mandate/receipt JSON via serde Serialize, rename_all camelCase |

## fmt-ap2-budget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-budget</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:449</code> | <code>Budget { max: Number, currency: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-budget</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-checkout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-checkout</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:286</code> | <code>Checkout { id: String, merchant?: Option&lt;Merchant&gt;, line_items: Vec&lt;CheckoutLineItem&gt;, status: String, currency: String, totals: Vec&lt;Total&gt;, links: Vec&lt;Value&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-checkout</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-checkout-line-item

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-checkout-line-item</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:339</code> | <code>CheckoutLineItem { id: String, item: Item, quantity: u64, totals: Vec&lt;Total&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-checkout-line-item</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-checkout-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-checkout-receipt</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:483</code> | <code>CheckoutReceipt { status: &amp;'static str, iss: String, iat: u64, reference: String, order_id: String, layerx_receipt_reference: String, layerx_evidence: PortableLayerXEvidence }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-checkout-receipt</code> | notes | AP2 mandate/receipt JSON via serde Serialize |

## fmt-ap2-closed-checkout-mandate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-closed-checkout-mandate</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:156</code> | <code>ClosedCheckoutMandate { vct: String, checkout_jwt: String, checkout_hash: String, iat?: Option&lt;u64&gt;, exp?: Option&lt;u64&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-closed-checkout-mandate</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-closed-payment-mandate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-closed-payment-mandate</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:175</code> | <code>ClosedPaymentMandate { vct: String, transaction_id: String, payee: Merchant, pisp?: Option&lt;Pisp&gt;, payment_amount: PaymentAmount, payment_instrument: PaymentInstrument, execution_date?: Option&lt;String&gt;, iat?: Option&lt;u64&gt;, exp?: Option&lt;u64&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-closed-payment-mandate</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-constraint-item

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-constraint-item</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:408</code> | <code>ConstraintItem { id: String, title: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-constraint-item</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-execution-date

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-execution-date</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:455</code> | <code>ExecutionDate { not_before?: Option&lt;String&gt;, not_after?: Option&lt;String&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-execution-date</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-item

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-item</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:362</code> | <code>Item { id: String, title: String, price: u128 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-item</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-line-item-requirement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-line-item-requirement</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:401</code> | <code>LineItemRequirement { id: String, acceptable_items: Vec&lt;ConstraintItem&gt;, quantity: u64 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-line-item-requirement</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-line-items-constraint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-line-items-constraint</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:396</code> | <code>LineItemsConstraint { items: Vec&lt;LineItemRequirement&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-line-items-constraint</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-merchant

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-merchant</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:12</code> | <code>Merchant { id: String, name: String, website?: Option&lt;String&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-merchant</code> | notes | AP2 mandate/receipt JSON via serde Clone, Debug, Eq, PartialEq, Deserialize |

## fmt-ap2-open-checkout-mandate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-open-checkout-mandate</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:213</code> | <code>OpenCheckoutMandate { vct: String, constraints: Vec&lt;Value&gt;, cnf: Value, iat?: Option&lt;u64&gt;, exp?: Option&lt;u64&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-open-checkout-mandate</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-open-payment-mandate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-open-payment-mandate</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:235</code> | <code>OpenPaymentMandate { vct: String, constraints: Vec&lt;Value&gt;, cnf: Value, payee?: Option&lt;Merchant&gt;, payment_amount?: Option&lt;PaymentAmount&gt;, payment_instrument?: Option&lt;PaymentInstrument&gt;, pisp?: Option&lt;Pisp&gt;, execution_date?: Option&lt;String&gt;, iat?: Option&lt;u64&gt;, exp?: Option&lt;u64&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-open-payment-mandate</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-owned-mandate-pair

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-owned-mandate-pair</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/portable.rs:47</code> | <code>OwnedMandatePair { checkoutMandate: String, paymentMandate: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-owned-mandate-pair</code> | notes | AP2 mandate/receipt JSON via serde Deserialize, deny_unknown_fields, rename_all camelCase |

## fmt-ap2-payment-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-payment-amount</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:79</code> | <code>PaymentAmount { amount: u128, currency: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-payment-amount</code> | notes | AP2 mandate/receipt JSON via serde Clone, Eq, PartialEq, Deserialize |

## fmt-ap2-payment-instrument

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-payment-instrument</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:104</code> | <code>PaymentInstrument { id: String, type: String, description?: Option&lt;String&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-payment-instrument</code> | notes | AP2 mandate/receipt JSON via serde Clone, Eq, PartialEq, Deserialize |

## fmt-ap2-payment-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-payment-receipt</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:469</code> | <code>PaymentReceipt { status: &amp;'static str, iss: String, iat: u64, reference: String, psp_confirmation_id?: Option&lt;String&gt;, network_confirmation_id?: Option&lt;String&gt;, layerx_receipt_reference: String, layerx_evidence: PortableLayerXEvidence }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-payment-receipt</code> | notes | AP2 mandate/receipt JSON via serde Serialize |

## fmt-ap2-payment-reference

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-payment-reference</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:437</code> | <code>PaymentReference { conditional_transaction_id: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-payment-reference</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |

## fmt-ap2-pisp

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-pisp</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:138</code> | <code>Pisp { legal_name: String, brand_name: String, domain_name: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-pisp</code> | notes | AP2 mandate/receipt JSON via serde Clone, Eq, PartialEq, Deserialize |

## fmt-ap2-protected-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-protected-header</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/jose.rs:32</code> | <code>ProtectedHeader { alg: String, typ?: Option&lt;String&gt;, kid?: Option&lt;String&gt;, x5c?: Option&lt;Vec&lt;String&gt;&gt;, crit?: Option&lt;Vec&lt;String&gt;&gt;, b64?: Option&lt;bool&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-protected-header</code> | notes | AP2 mandate/receipt JSON via serde Clone, Debug, Eq, PartialEq, Deserialize |

## fmt-ap2-receipt-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-receipt-header</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/adapter.rs:544</code> | <code>ReceiptHeader { alg: &amp;'static str, typ: &amp;'static str, kid: &amp;'a str }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-receipt-header</code> | notes | AP2 mandate/receipt JSON via serde Serialize |

## fmt-ap2-total

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-ap2-total</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-ap2/src/model.rs:378</code> | <code>Total { type: String, amount: i128 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-ap2-total</code> | notes | AP2 mandate/receipt JSON via serde Clone, Deserialize |
