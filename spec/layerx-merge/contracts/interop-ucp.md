# Interop UCP contract

The [UCP adapter](../../../services/interop/crates/layerx-ucp/src/lib.rs) completes checkouts and verifies stored orders using authenticated payment receipts and independently supplied receipt-key pins.

## ucp-capability

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-capability</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct Capability</code> |

## ucp-capability-name

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-capability-name</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn name(&amp;self) -&gt; &amp;str</code> |

## ucp-capability-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-capability-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn new(name: impl Into&lt;String&gt;, version: impl Into&lt;String&gt;, spec: impl Into&lt;String&gt;, schema: impl Into&lt;String&gt;) -&gt; Result&lt;Self, UcpError&gt;</code> |

## ucp-capability-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-capability-schema</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn schema(&amp;self) -&gt; &amp;str</code> |

## ucp-capability-spec

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-capability-spec</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn spec(&amp;self) -&gt; &amp;str</code> |

## ucp-capability-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-capability-version</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn version(&amp;self) -&gt; &amp;str</code> |

## ucp-checkout-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-checkout-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct CheckoutOutcome</code> |

## ucp-checkout-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-checkout-status</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub enum CheckoutStatus</code> |

## ucp-checkout-submission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-checkout-submission</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct CheckoutSubmission</code> |

## ucp-executed-ucp-payment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-executed-ucp-payment</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct ExecutedUcpPayment</code> |

## ucp-interop-ucp

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-interop-ucp</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub const fn interop_ucp() -&gt; &amp;'static str</code> |

## ucp-merchant-profile

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct MerchantProfile</code> |

## ucp-merchant-profile-capabilities

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-capabilities</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn capabilities(&amp;self) -&gt; &amp;[Capability]</code> |

## ucp-merchant-profile-layerx

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-layerx</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn layerx(rest_endpoint: impl Into&lt;String&gt;, payment_handler: PaymentHandler) -&gt; Result&lt;Self, UcpError&gt;</code> |

## ucp-merchant-profile-payment-handlers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-payment-handlers</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn payment_handlers(&amp;self) -&gt; &amp;[PaymentHandler]</code> |

## ucp-merchant-profile-rest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-rest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub const fn rest(&amp;self) -&gt; &amp;RestService</code> |

## ucp-merchant-profile-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-merchant-profile-version</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn version(&amp;self) -&gt; &amp;str</code> |

## ucp-negotiated-capabilities

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct NegotiatedCapabilities</code> |

## ucp-negotiated-capabilities-checkout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities-checkout</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub const fn checkout(self) -&gt; bool</code> |

## ucp-negotiated-capabilities-negotiate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities-negotiate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn negotiate(platform: &amp;PlatformProfile, merchant_handler: &amp;PaymentHandler) -&gt; Result&lt;Self, UcpError&gt;</code> |

## ucp-negotiated-capabilities-order

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities-order</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub const fn order(self) -&gt; bool</code> |

## ucp-negotiated-capabilities-payment-handler-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-negotiated-capabilities-payment-handler-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub const fn payment_handler_digest(self) -&gt; [u8; 32]</code> |

## ucp-order-metadata

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-order-metadata</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct OrderMetadata</code> |

## ucp-payment-handler

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-payment-handler</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct PaymentHandler</code> |

## ucp-payment-handler-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-payment-handler-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn id(&amp;self) -&gt; &amp;str</code> |

## ucp-payment-handler-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-payment-handler-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn new(id: impl Into&lt;String&gt;, version: impl Into&lt;String&gt;, spec: impl Into&lt;String&gt;, schema: impl Into&lt;String&gt;) -&gt; Result&lt;Self, UcpError&gt;</code> |

## ucp-payment-handler-payment-handler-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-payment-handler-payment-handler-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn payment_handler_digest(&amp;self) -&gt; [u8; 32]</code> |

## ucp-payment-handler-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-payment-handler-version</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn version(&amp;self) -&gt; &amp;str</code> |

## ucp-platform-profile

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-platform-profile</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct PlatformProfile</code> |

## ucp-rest-service

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-rest-service</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct RestService</code> |

## ucp-rest-service-endpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-rest-service-endpoint</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn endpoint(&amp;self) -&gt; &amp;str</code> |

## ucp-rest-service-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-rest-service-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn new(endpoint: impl Into&lt;String&gt;) -&gt; Result&lt;Self, UcpError&gt;</code> |

## ucp-rest-service-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-rest-service-schema</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn schema(&amp;self) -&gt; &amp;str</code> |

## ucp-stored-order

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-stored-order</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct StoredOrder</code> |

## ucp-ucp-adapter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-adapter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct UcpAdapter</code> |

## ucp-ucp-adapter-complete-checkout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-adapter-complete-checkout</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn complete_checkout( gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, submission: &amp;CheckoutSubmission, plane: &amp;mut impl UcpPaymentPlane, receipt_keys: &amp;[SequencerAuthorization], trace: &amp;TraceId, now: u64, ) -&gt; Result&lt;CheckoutOutcome, Traced&lt;UcpError&gt;&gt;</code> |

## ucp-ucp-adapter-descriptor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-adapter-descriptor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn ucp_adapter_descriptor(spec: PinnedSpec, conformance: ConformanceSuite) -&gt; Result&lt;AdapterDescriptor, UcpError&gt;</code> |

## ucp-ucp-adapter-read-order

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-adapter-read-order</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn read_order( stored: &amp;StoredOrder, receipt_keys: &amp;[SequencerAuthorization], ) -&gt; Result&lt;UcpOrder, UcpError&gt;</code> |

## ucp-ucp-checkout-schema-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-checkout-schema-sha256</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub const UCP_CHECKOUT_SCHEMA_SHA256: [u8; 32]</code> |

## ucp-ucp-checkout-spec-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-checkout-spec-sha256</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub const UCP_CHECKOUT_SPEC_SHA256: [u8; 32]</code> |

## ucp-ucp-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub enum UcpError</code> |

## ucp-ucp-idempotency-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-idempotency-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct UcpIdempotencyKey([u8; 16])</code> |

## ucp-ucp-idempotency-key-gateway-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-idempotency-key-gateway-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn gateway_key(self) -&gt; [u8; 32]</code> |

## ucp-ucp-idempotency-key-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-idempotency-key-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn parse(value: &amp;str) -&gt; Result&lt;Self, UcpError&gt;</code> |

## ucp-ucp-order

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-order</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct UcpOrder</code> |

## ucp-ucp-order-schema-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-order-schema-sha256</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub const UCP_ORDER_SCHEMA_SHA256: [u8; 32]</code> |

## ucp-ucp-order-spec-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-order-spec-sha256</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub const UCP_ORDER_SPEC_SHA256: [u8; 32]</code> |

## ucp-ucp-payment-intent

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-payment-intent</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub struct UcpPaymentIntent</code> |

## ucp-ucp-payment-plane

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-payment-plane</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub trait UcpPaymentPlane</code> |

## ucp-ucp-plane-result

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-plane-result</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub enum UcpPlaneResult</code> |

## ucp-ucp-rest-schema-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-ucp-rest-schema-sha256</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub const UCP_REST_SCHEMA_SHA256: [u8; 32]</code> |

## ucp-verify-sequencer-receipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-ucp-verify-sequencer-receipt</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-ucp/src/lib.rs](../../../services/interop/crates/layerx-ucp/src/lib.rs) | <code>pub fn verify_sequencer_receipt(submission: &amp;CheckoutSubmission, canonical_receipt: &amp;[u8], receipt_keys: &amp;[SequencerAuthorization]) -&gt; Result&lt;[u8; 32], UcpError&gt;</code> |
