# Interop ucp contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## ucp-capability

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-capability</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:70</code> | <code>pub struct Capability</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-capability</code> | notes | struct layerx_ucp::Capability |

## ucp-capability-name

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-capability-name</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:106</code> | <code>pub fn name(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-capability-name</code> | notes | method layerx_ucp::Capability::name |

## ucp-capability-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-capability-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:83</code> | <code>pub fn new(name: impl Into&lt;String&gt;, version: impl Into&lt;String&gt;, spec: impl Into&lt;String&gt;, schema: impl Into&lt;String&gt;) -&gt; Result&lt;Self, UcpError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-capability-new</code> | notes | method layerx_ucp::Capability::new |

## ucp-capability-schema

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-capability-schema</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:121</code> | <code>pub fn schema(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-capability-schema</code> | notes | method layerx_ucp::Capability::schema |

## ucp-capability-spec

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-capability-spec</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:116</code> | <code>pub fn spec(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-capability-spec</code> | notes | method layerx_ucp::Capability::spec |

## ucp-capability-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-capability-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:111</code> | <code>pub fn version(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-capability-version</code> | notes | method layerx_ucp::Capability::version; no reference outside the defining file found by name match |

## ucp-checkout-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-checkout-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:516</code> | <code>pub struct CheckoutOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-checkout-outcome</code> | notes | struct layerx_ucp::CheckoutOutcome |

## ucp-checkout-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-checkout-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:405</code> | <code>pub enum CheckoutStatus</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-checkout-status</code> | notes | enum layerx_ucp::CheckoutStatus |

## ucp-checkout-submission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-checkout-submission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:416</code> | <code>pub struct CheckoutSubmission</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-checkout-submission</code> | notes | struct layerx_ucp::CheckoutSubmission |

## ucp-executed-ucp-payment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-executed-ucp-payment</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:481</code> | <code>pub struct ExecutedUcpPayment</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-executed-ucp-payment</code> | notes | struct layerx_ucp::ExecutedUcpPayment |
| <code>interface.11.rs-ucp-executed-ucp-payment</code> | overridden_by | override.J47.7 |

## ucp-interop-ucp

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-interop-ucp</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:977</code> | <code>pub const fn interop_ucp() -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-interop-ucp</code> | notes | fn layerx_ucp::interop_ucp |

## ucp-merchant-profile

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:231</code> | <code>pub struct MerchantProfile</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile</code> | notes | struct layerx_ucp::MerchantProfile |

## ucp-merchant-profile-capabilities

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-capabilities</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:275</code> | <code>pub fn capabilities(&amp;self) -&gt; &amp;[Capability]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-capabilities</code> | notes | method layerx_ucp::MerchantProfile::capabilities |

## ucp-merchant-profile-layerx

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-layerx</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:244</code> | <code>pub fn layerx(rest_endpoint: impl Into&lt;String&gt;, payment_handler: PaymentHandler) -&gt; Result&lt;Self, UcpError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-layerx</code> | notes | method layerx_ucp::MerchantProfile::layerx |

## ucp-merchant-profile-payment-handlers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-payment-handlers</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:280</code> | <code>pub fn payment_handlers(&amp;self) -&gt; &amp;[PaymentHandler]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-payment-handlers</code> | notes | method layerx_ucp::MerchantProfile::payment_handlers |

## ucp-merchant-profile-rest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-rest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:270</code> | <code>pub const fn rest(&amp;self) -&gt; &amp;RestService</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-rest</code> | notes | method layerx_ucp::MerchantProfile::rest |

## ucp-merchant-profile-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:265</code> | <code>pub fn version(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-version</code> | notes | method layerx_ucp::MerchantProfile::version; no reference outside the defining file found by name match |

## ucp-negotiated-capabilities

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:295</code> | <code>pub struct NegotiatedCapabilities</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities</code> | notes | struct layerx_ucp::NegotiatedCapabilities |

## ucp-negotiated-capabilities-checkout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities-checkout</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:342</code> | <code>pub const fn checkout(self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities-checkout</code> | notes | method layerx_ucp::NegotiatedCapabilities::checkout |

## ucp-negotiated-capabilities-negotiate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities-negotiate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:308</code> | <code>pub fn negotiate(platform: &amp;PlatformProfile, merchant_handler: &amp;PaymentHandler) -&gt; Result&lt;Self, UcpError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities-negotiate</code> | notes | method layerx_ucp::NegotiatedCapabilities::negotiate |

## ucp-negotiated-capabilities-order

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities-order</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:347</code> | <code>pub const fn order(self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities-order</code> | notes | method layerx_ucp::NegotiatedCapabilities::order |

## ucp-negotiated-capabilities-payment-handler-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities-payment-handler-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:352</code> | <code>pub const fn payment_handler_digest(self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities-payment-handler-digest</code> | notes | method layerx_ucp::NegotiatedCapabilities::payment_handler_digest |

## ucp-order-metadata

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-order-metadata</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:445</code> | <code>pub struct OrderMetadata</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-order-metadata</code> | notes | struct layerx_ucp::OrderMetadata |

## ucp-payment-handler

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-payment-handler</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:138</code> | <code>pub struct PaymentHandler</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-payment-handler</code> | notes | struct layerx_ucp::PaymentHandler |

## ucp-payment-handler-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-payment-handler-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:174</code> | <code>pub fn id(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-payment-handler-id</code> | notes | method layerx_ucp::PaymentHandler::id; no reference outside the defining file found by name match |

## ucp-payment-handler-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-payment-handler-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:151</code> | <code>pub fn new(id: impl Into&lt;String&gt;, version: impl Into&lt;String&gt;, spec: impl Into&lt;String&gt;, schema: impl Into&lt;String&gt;) -&gt; Result&lt;Self, UcpError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-payment-handler-new</code> | notes | method layerx_ucp::PaymentHandler::new |

## ucp-payment-handler-payment-handler-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-payment-handler-payment-handler-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:184</code> | <code>pub fn payment_handler_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-payment-handler-payment-handler-digest</code> | notes | method layerx_ucp::PaymentHandler::payment_handler_digest |

## ucp-payment-handler-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-payment-handler-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:179</code> | <code>pub fn version(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-payment-handler-version</code> | notes | method layerx_ucp::PaymentHandler::version; no reference outside the defining file found by name match |

## ucp-platform-profile

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-platform-profile</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:287</code> | <code>pub struct PlatformProfile</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-platform-profile</code> | notes | struct layerx_ucp::PlatformProfile |

## ucp-rest-service

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-rest-service</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:129</code> | <code>pub struct RestService</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-rest-service</code> | notes | struct layerx_ucp::RestService; no reference outside the defining file found by name match |

## ucp-rest-service-endpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-rest-service-endpoint</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:219</code> | <code>pub fn endpoint(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-rest-service-endpoint</code> | notes | method layerx_ucp::RestService::endpoint; no reference outside the defining file found by name match |

## ucp-rest-service-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-rest-service-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:207</code> | <code>pub fn new(endpoint: impl Into&lt;String&gt;) -&gt; Result&lt;Self, UcpError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-rest-service-new</code> | notes | method layerx_ucp::RestService::new; no reference outside the defining file found by name match |

## ucp-rest-service-schema

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-rest-service-schema</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:224</code> | <code>pub fn schema(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-rest-service-schema</code> | notes | method layerx_ucp::RestService::schema; no reference outside the defining file found by name match |

## ucp-sequencer-settlement

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.22.1</code> | req | 11022.1 |
| <code>acceptance.11.22.1</code> | scenario | Given an open checkout and a plane returning the recorded receipted evidence, when complete_checkout runs, then the stored order carries the recorded txHash and rung receipted. |
| <code>acceptance.11.22.1</code> | test | services/interop/crates/layerx-ucp/tests/sequencer_checkout.rs::complete_checkout_stores_the_transaction_hash (planned) |
| <code>acceptance.11.22.1</code> | qualification | UNRUN |
| <code>req.11022</code> | ac_1 | complete_checkout stores the recorded txHash of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:109 in the order (planned) (services/interop/crates/layerx-ucp/src/lib.rs:524). |
| <code>acceptance.11.22.2</code> | req | 11022.2 |
| <code>acceptance.11.22.2</code> | scenario | Given the same checkout and a submitted rung, when complete_checkout runs, then the answer is pending and no order is stored. |
| <code>acceptance.11.22.2</code> | test | services/interop/crates/layerx-ucp/tests/sequencer_checkout.rs::complete_checkout_on_a_submitted_rung_stays_open (planned) |
| <code>acceptance.11.22.2</code> | qualification | UNRUN |
| <code>req.11022</code> | ac_2 | A submitted rung leaves the checkout incomplete (planned) (services/interop/crates/layerx-ucp/src/lib.rs:542). |
| <code>acceptance.11.33.1</code> | req | 11033.1 |
| <code>acceptance.11.33.1</code> | scenario | Given the recorded receipted evidence, when complete_checkout stores the order, then the order digest is the recorded receiptDigest. |
| <code>acceptance.11.33.1</code> | test | services/interop/crates/layerx-ucp/tests/sequencer_checkout.rs::ucp_order_digest_is_the_recorded_receipt_digest (planned) |
| <code>acceptance.11.33.1</code> | qualification | UNRUN |
| <code>req.11033</code> | ac_1 | The UCP order digest equals receiptDigest at sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:150 (planned). |
| <code>acceptance.11.43.1</code> | req | 11043.1 |
| <code>acceptance.11.43.1</code> | scenario | Given a stored order settled on the recorded receipt and an adapter pinning another key, when read_order runs, then it returns the receipt refusal. |
| <code>acceptance.11.43.1</code> | test | services/interop/crates/layerx-ucp/tests/sequencer_checkout.rs::read_order_refuses_an_unpinned_receipt (planned) |
| <code>acceptance.11.43.1</code> | qualification | UNRUN |
| <code>req.11043</code> | ac_1 | read_order refuses an order whose receipt key was unpinned (planned) (services/interop/crates/layerx-ucp/src/lib.rs:629). |
| <code>acceptance.11.59.1</code> | req | 11059.1 |
| <code>acceptance.11.59.1</code> | scenario | Given the rewritten seller, when the client completes a checkout, then the order carries the recorded txHash. |
| <code>acceptance.11.59.1</code> | test | services/interop/crates/layerx-ucp/tests/e2e_client_seller.rs::client_and_seller_complete_on_the_recorded_receipt (planned) |
| <code>acceptance.11.59.1</code> | qualification | UNRUN |
| <code>req.11059</code> | ac_1 | The UCP end-to-end flow completes on the recorded receipt (planned) (services/interop/crates/layerx-ucp/tests/e2e_client_seller.rs:393). |
| <code>capability.11.9.1</code> | title | Complete UCP checkouts on SequencerEvidence and store the transaction hash |
| <code>capability.11.9.1</code> | reqs | 11022.1<br>11022.2 |
| <code>capability.11.9.2</code> | title | Use the gateway canonical receipt digest in UCP instead of the private copy |
| <code>capability.11.9.2</code> | reqs | 11033.1 |
| <code>capability.11.9.3</code> | title | Re-verify the stored receipt when a UCP order is read |
| <code>capability.11.9.3</code> | reqs | 11043.1 |
| <code>capability.11.9.4</code> | title | Move the UCP client and seller end-to-end test off AuthorizedBatch |
| <code>capability.11.9.4</code> | reqs | 11059.1 |

## ucp-stored-order

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-stored-order</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:524</code> | <code>pub struct StoredOrder</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-stored-order</code> | notes | struct layerx_ucp::StoredOrder |
| <code>interface.11.rs-ucp-stored-order</code> | overridden_by | override.J47.8 |

## ucp-ucp-adapter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-adapter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:533</code> | <code>pub struct UcpAdapter</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-adapter</code> | notes | struct layerx_ucp::UcpAdapter |

## ucp-ucp-adapter-complete-checkout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-adapter-complete-checkout</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:542</code> | <code>pub fn complete_checkout(gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, submission: &amp;CheckoutSubmission, plane: &amp;mut impl UcpPaymentPlane, trace: &amp;TraceId, now: u64) -&gt; Result&lt;CheckoutOutcome, Traced&lt;UcpError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-adapter-complete-checkout</code> | notes | method layerx_ucp::UcpAdapter::complete_checkout |
| <code>interface.11.rs-ucp-ucp-adapter-complete-checkout</code> | overridden_by | override.J47.9 |

## ucp-ucp-adapter-descriptor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-adapter-descriptor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:876</code> | <code>pub fn ucp_adapter_descriptor(spec: PinnedSpec, conformance: ConformanceSuite) -&gt; Result&lt;AdapterDescriptor, UcpError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-adapter-descriptor</code> | notes | fn layerx_ucp::ucp_adapter_descriptor |

## ucp-ucp-adapter-read-order

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-adapter-read-order</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:629</code> | <code>pub fn read_order(stored: &amp;StoredOrder) -&gt; Result&lt;UcpOrder, UcpError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-adapter-read-order</code> | notes | method layerx_ucp::UcpAdapter::read_order |
| <code>interface.11.rs-ucp-ucp-adapter-read-order</code> | overridden_by | override.J47.10 |

## ucp-ucp-checkout-schema-sha256

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-checkout-schema-sha256</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:51</code> | <code>pub const UCP_CHECKOUT_SCHEMA_SHA256: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-checkout-schema-sha256</code> | notes | const layerx_ucp::UCP_CHECKOUT_SCHEMA_SHA256 |

## ucp-ucp-checkout-spec-sha256

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-checkout-spec-sha256</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:40</code> | <code>pub const UCP_CHECKOUT_SPEC_SHA256: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-checkout-spec-sha256</code> | notes | const layerx_ucp::UCP_CHECKOUT_SPEC_SHA256 |

## ucp-ucp-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:929</code> | <code>pub enum UcpError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-error</code> | notes | enum layerx_ucp::UcpError; 46 reference lines in 4 files; callers lists the first reference in each file |

## ucp-ucp-idempotency-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-idempotency-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:359</code> | <code>pub struct UcpIdempotencyKey([u8; 16])</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-idempotency-key</code> | notes | struct layerx_ucp::UcpIdempotencyKey |

## ucp-ucp-idempotency-key-gateway-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-idempotency-key-gateway-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:395</code> | <code>pub fn gateway_key(self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-idempotency-key-gateway-key</code> | notes | method layerx_ucp::UcpIdempotencyKey::gateway_key |

## ucp-ucp-idempotency-key-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-idempotency-key-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:367</code> | <code>pub fn parse(value: &amp;str) -&gt; Result&lt;Self, UcpError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-idempotency-key-parse</code> | notes | method layerx_ucp::UcpIdempotencyKey::parse |

## ucp-ucp-order

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-order</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:505</code> | <code>pub struct UcpOrder</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-order</code> | notes | struct layerx_ucp::UcpOrder |

## ucp-ucp-order-schema-sha256

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-order-schema-sha256</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:56</code> | <code>pub const UCP_ORDER_SCHEMA_SHA256: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-order-schema-sha256</code> | notes | const layerx_ucp::UCP_ORDER_SCHEMA_SHA256 |

## ucp-ucp-order-spec-sha256

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-order-spec-sha256</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:46</code> | <code>pub const UCP_ORDER_SPEC_SHA256: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-order-spec-sha256</code> | notes | const layerx_ucp::UCP_ORDER_SPEC_SHA256 |

## ucp-ucp-payment-intent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-payment-intent</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:462</code> | <code>pub struct UcpPaymentIntent</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-payment-intent</code> | notes | struct layerx_ucp::UcpPaymentIntent |

## ucp-ucp-payment-plane

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-payment-plane</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:488</code> | <code>pub trait UcpPaymentPlane</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-payment-plane</code> | notes | trait layerx_ucp::UcpPaymentPlane |

## ucp-ucp-plane-result

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-plane-result</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:473</code> | <code>pub enum UcpPlaneResult</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-plane-result</code> | notes | enum layerx_ucp::UcpPlaneResult |

## ucp-ucp-rest-schema-sha256

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-rest-schema-sha256</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:61</code> | <code>pub const UCP_REST_SCHEMA_SHA256: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-ucp-rest-schema-sha256</code> | notes | const layerx_ucp::UCP_REST_SCHEMA_SHA256 |

## ucp-verify-sequencer-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-ucp-verify-sequencer-receipt</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-ucp/src/lib.rs:664</code> | <code>pub fn verify_sequencer_receipt(submission: &amp;CheckoutSubmission, canonical_receipt: &amp;[u8], receipt_keys: &amp;[SequencerAuthorization]) -&gt; Result&lt;[u8; 32], UcpError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-ucp-verify-sequencer-receipt</code> | notes | fn layerx_ucp::verify_sequencer_receipt |
