# Interop x402 contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## fmt-x402-extension

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-x402-extension</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:144</code> | <code>Extension { info: Value, schema: Value }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-x402-extension</code> | notes | x402 v2 wire JSON via serde Clone, Debug, PartialEq, Serialize, Deserialize, deny_unknown_fields |

## fmt-x402-facilitator-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-x402-facilitator-kind</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:72</code> | <code>FacilitatorKind { x402Version: u8, scheme: String, network: String, extra?: Option&lt;Value&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-x402-facilitator-kind</code> | notes | x402 v2 wire JSON via serde Clone, Debug, PartialEq, Serialize, Deserialize, deny_unknown_fields, rename_all camelCase |

## fmt-x402-facilitator-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-x402-facilitator-request</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:34</code> | <code>FacilitatorRequest { x402Version: u8, paymentPayload: PaymentPayload, paymentRequirements: PaymentRequirements }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-x402-facilitator-request</code> | notes | x402 v2 wire JSON via serde Clone, Debug, PartialEq, Serialize, Deserialize, deny_unknown_fields, rename_all camelCase |

## fmt-x402-layerx-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-x402-layerx-evidence</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/crates/layerx-x402/src/buyer.rs:59</code> | <code>LayerXEvidence { receipt: String, receiptDigest: String, verificationLevel: String, purposeHash?: Option&lt;String&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-x402-layerx-evidence</code> | notes | x402 v2 wire JSON via serde Debug, Deserialize, deny_unknown_fields, rename_all camelCase |
| <code>interface.11.fmt-x402-layerx-evidence</code> | overridden_by | override.J47.5 |

## fmt-x402-payment-payload

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-x402-payment-payload</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:402</code> | <code>PaymentPayload { x402Version: u8, resource?: Option&lt;ResourceInfo&gt;, payload: Value, accepted: PaymentRequirements, extensions?: Extensions }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-x402-payment-payload</code> | notes | x402 v2 wire JSON via serde Clone, Debug, PartialEq, Serialize, Deserialize, deny_unknown_fields, rename_all camelCase; 39 reference lines in 11 files; callers lists the first reference in each file |

## fmt-x402-payment-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-x402-payment-required</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:368</code> | <code>PaymentRequired { x402Version: u8, error?: Option&lt;String&gt;, resource: ResourceInfo, accepts: Vec&lt;PaymentRequirements&gt;, extensions?: Extensions }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-x402-payment-required</code> | notes | x402 v2 wire JSON via serde Clone, Debug, PartialEq, Serialize, Deserialize, deny_unknown_fields, rename_all camelCase; 44 reference lines in 12 files; callers lists the first reference in each file |

## fmt-x402-payment-requirements

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-x402-payment-requirements</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:153</code> | <code>PaymentRequirements { scheme: String, network: String, amount: AtomicAmount, asset: String, payTo: String, maxTimeoutSeconds: u32, extra?: Option&lt;Value&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-x402-payment-requirements</code> | notes | x402 v2 wire JSON via serde Clone, Debug, PartialEq, Serialize, Deserialize, deny_unknown_fields, rename_all camelCase; 72 reference lines in 12 files; callers lists the first reference in each file |

## fmt-x402-resource-info

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-x402-resource-info</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:96</code> | <code>ResourceInfo { url: String, description?: Option&lt;String&gt;, mimeType?: Option&lt;String&gt;, serviceName?: Option&lt;String&gt;, tags?: Vec&lt;String&gt;, iconUrl?: Option&lt;String&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-x402-resource-info</code> | notes | x402 v2 wire JSON via serde Clone, Debug, PartialEq, Serialize, Deserialize, deny_unknown_fields, rename_all camelCase |

## fmt-x402-settlement-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-x402-settlement-response</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:430</code> | <code>SettlementResponse { success: bool, errorReason?: Option&lt;String&gt;, payer?: Option&lt;String&gt;, transaction: String, network: String, amount?: Option&lt;AtomicAmount&gt;, extensions?: BTreeMap&lt;String, Value&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-x402-settlement-response</code> | notes | x402 v2 wire JSON via serde Clone, Debug, PartialEq, Serialize, Deserialize, deny_unknown_fields, rename_all camelCase; 47 reference lines in 11 files; callers lists the first reference in each file |

## fmt-x402-supported-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-x402-supported-response</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:83</code> | <code>SupportedResponse { kinds: Vec&lt;FacilitatorKind&gt;, extensions: Vec&lt;String&gt;, signers: BTreeMap&lt;String, Vec&lt;String&gt;&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-x402-supported-response</code> | notes | x402 v2 wire JSON via serde Clone, Debug, PartialEq, Serialize, Deserialize, deny_unknown_fields |

## fmt-x402-verify-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-x402-verify-response</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:59</code> | <code>VerifyResponse { isValid: bool, invalidReason?: Option&lt;String&gt;, payer?: Option&lt;String&gt;, extra?: Option&lt;Value&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-x402-verify-response</code> | notes | x402 v2 wire JSON via serde Clone, Debug, PartialEq, Serialize, Deserialize, deny_unknown_fields, rename_all camelCase |

## rs-x402-error-transaction-form

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-error-transaction-form</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-x402/src/model.rs</code> (planned) | <code>X402Error::TransactionForm</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-error-transaction-form</code> | notes | stale task 0.101 do_3: returned for an lxp: transaction value instead of EvidenceMismatch |

Contract context <code>contract_context.11.x402-sequencer-settlement</code>:

| Field | Frozen rule |
|---|---|
| interface | x402 seller, facilitator and buyer settle and capture on SequencerEvidence with transaction set to the transaction hash. |
| storage | None beyond the gateway records. |
| errors | X402Error::EvidenceMismatch, X402Error::TransactionForm. |
| events | None. |
| gas | Paid by the payer-signed transaction. |
| activation | Wave 2; committed rung in wave 5. |
| invariants | transaction is a 32-byte hash; committed only with a verified proof. |
| pause | Not pausable. |

## x402-account-identifiers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-account-identifiers</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:248</code> | <code>pub fn account_identifiers(account: &amp;str) -&gt; Result&lt;[[u8; 32]; 2], X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-account-identifiers</code> | notes | fn layerx_x402::model::account_identifiers; re-exported at services/interop/crates/layerx-x402/src/lib.rs:12 |

## x402-atomic-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-atomic-amount</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:42</code> | <code>pub struct AtomicAmount(u128)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-atomic-amount</code> | notes | struct layerx_x402::model::AtomicAmount |

## x402-atomic-amount-from-u128

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-atomic-amount-from-u128</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:65</code> | <code>pub const fn from_u128(value: u128) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-atomic-amount-from-u128</code> | notes | method layerx_x402::model::AtomicAmount::from_u128 |

## x402-atomic-amount-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-atomic-amount-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:50</code> | <code>pub fn parse(value: &amp;str) -&gt; Result&lt;Self, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-atomic-amount-parse</code> | notes | method layerx_x402::model::AtomicAmount::parse |

## x402-atomic-amount-value

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-atomic-amount-value</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:70</code> | <code>pub const fn value(self) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-atomic-amount-value</code> | notes | method layerx_x402::model::AtomicAmount::value; no reference outside the defining file found by name match |

## x402-built-payment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-built-payment</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/buyer.rs:44</code> | <code>pub struct BuiltPayment</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-built-payment</code> | notes | struct layerx_x402::buyer::BuiltPayment |

## x402-buyer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-buyer</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-x402/src/buyer.rs:69</code> | <code>pub struct Buyer</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-buyer</code> | notes | struct layerx_x402::buyer::Buyer; re-exported at services/interop/crates/layerx-x402/src/lib.rs:10; spec/layerx-merge/spec.kvx task 0.101 replaces the buyer AuthorizedBatch verification (services/interop/crates/layerx-x402/src/buyer.rs:8, :165) with the signed sequencer receipt |

## x402-buyer-build-payment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-buyer-build-payment</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/buyer.rs:102</code> | <code>pub fn build_payment(&amp;self, required_header: &amp;str, idempotency_key: [u8; 32], plane: &amp;mut impl BuyerPaymentPlane, trace: &amp;TraceId) -&gt; Result&lt;BuiltPayment, Traced&lt;X402Error&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-buyer-build-payment</code> | notes | method layerx_x402::buyer::Buyer::build_payment |

## x402-buyer-capture-settlement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-buyer-capture-settlement</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-x402/src/buyer.rs:162</code> | <code>pub fn capture_settlement(response_header: &amp;str, expected: &amp;BuiltPayment, authorised_batch: &amp;AuthorizedBatch, trace: &amp;TraceId) -&gt; Result&lt;CapturedSettlement, Traced&lt;X402Error&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-buyer-capture-settlement</code> | notes | method layerx_x402::buyer::Buyer::capture_settlement; spec/layerx-merge/spec.kvx task 0.101 replaces the buyer AuthorizedBatch verification (services/interop/crates/layerx-x402/src/buyer.rs:8, :165) with the signed sequencer receipt |

## x402-buyer-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-buyer-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/buyer.rs:79</code> | <code>pub fn new(supported: Vec&lt;SupportedKind&gt;) -&gt; Result&lt;Self, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-buyer-new</code> | notes | method layerx_x402::buyer::Buyer::new |

## x402-buyer-payment-plane

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-buyer-payment-plane</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/buyer.rs:34</code> | <code>pub trait BuyerPaymentPlane</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-buyer-payment-plane</code> | notes | trait layerx_x402::buyer::BuyerPaymentPlane |

## x402-captured-settlement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-captured-settlement</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-x402/src/buyer.rs:51</code> | <code>pub struct CapturedSettlement</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-captured-settlement</code> | notes | struct layerx_x402::buyer::CapturedSettlement; spec/layerx-merge/spec.kvx task 0.101 replaces the buyer AuthorizedBatch verification (services/interop/crates/layerx-x402/src/buyer.rs:8, :165) with the signed sequencer receipt |

## x402-decode-facilitator-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-decode-facilitator-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:169</code> | <code>pub fn decode_facilitator_request(_transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;FacilitatorRequest, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-decode-facilitator-request</code> | notes | fn layerx_x402::transport::decode_facilitator_request |

## x402-decode-facilitator-settlement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-decode-facilitator-settlement</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:223</code> | <code>pub fn decode_facilitator_settlement(_transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;SettlementResponse, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-decode-facilitator-settlement</code> | notes | fn layerx_x402::transport::decode_facilitator_settlement |

## x402-decode-payment-payload

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-decode-payment-payload</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:114</code> | <code>pub fn decode_payment_payload(transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;PaymentPayload, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-decode-payment-payload</code> | notes | fn layerx_x402::transport::decode_payment_payload |

## x402-decode-payment-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-decode-payment-required</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:87</code> | <code>pub fn decode_payment_required(transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;PaymentRequired, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-decode-payment-required</code> | notes | fn layerx_x402::transport::decode_payment_required |

## x402-decode-settlement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-decode-settlement</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:141</code> | <code>pub fn decode_settlement(transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;SettlementResponse, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-decode-settlement</code> | notes | fn layerx_x402::transport::decode_settlement |

## x402-decode-supported-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-decode-supported-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:250</code> | <code>pub fn decode_supported_response(_transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;SupportedResponse, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-decode-supported-response</code> | notes | fn layerx_x402::transport::decode_supported_response |

## x402-decode-verify-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-decode-verify-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:196</code> | <code>pub fn decode_verify_response(_transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;VerifyResponse, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-decode-verify-response</code> | notes | fn layerx_x402::transport::decode_verify_response |

## x402-encode-facilitator-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-encode-facilitator-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:156</code> | <code>pub fn encode_facilitator_request(_transport: TransportKind, value: &amp;FacilitatorRequest) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-encode-facilitator-request</code> | notes | fn layerx_x402::transport::encode_facilitator_request |

## x402-encode-facilitator-settlement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-encode-facilitator-settlement</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:210</code> | <code>pub fn encode_facilitator_settlement(_transport: TransportKind, value: &amp;SettlementResponse) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-encode-facilitator-settlement</code> | notes | fn layerx_x402::transport::encode_facilitator_settlement |

## x402-encode-payment-payload

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-encode-payment-payload</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:101</code> | <code>pub fn encode_payment_payload(transport: TransportKind, value: &amp;PaymentPayload) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-encode-payment-payload</code> | notes | fn layerx_x402::transport::encode_payment_payload |

## x402-encode-payment-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-encode-payment-required</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:74</code> | <code>pub fn encode_payment_required(transport: TransportKind, value: &amp;PaymentRequired) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-encode-payment-required</code> | notes | fn layerx_x402::transport::encode_payment_required |

## x402-encode-settlement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-encode-settlement</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:128</code> | <code>pub fn encode_settlement(transport: TransportKind, value: &amp;SettlementResponse) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-encode-settlement</code> | notes | fn layerx_x402::transport::encode_settlement |

## x402-encode-supported-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-encode-supported-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:237</code> | <code>pub fn encode_supported_response(_transport: TransportKind, value: &amp;SupportedResponse) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-encode-supported-response</code> | notes | fn layerx_x402::transport::encode_supported_response |

## x402-encode-verify-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-encode-verify-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:183</code> | <code>pub fn encode_verify_response(_transport: TransportKind, value: &amp;VerifyResponse) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-encode-verify-response</code> | notes | fn layerx_x402::transport::encode_verify_response |

## x402-executed-payment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-executed-payment</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-x402/src/seller.rs:47</code> | <code>pub struct ExecutedPayment</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-executed-payment</code> | notes | struct layerx_x402::seller::ExecutedPayment; spec/layerx-merge/spec.kvx task 7.5.17 (in_progress) replaces authorised batch verification (services/interop/crates/layerx-x402/src/seller.rs:243) with ProtocolReceipt v2 against ReceiptKeys and a 0x101A PaymentExact/PaymentDrawn match |

## x402-extension

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-extension</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:144</code> | <code>pub struct Extension</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-extension</code> | notes | struct layerx_x402::model::Extension |

## x402-extensions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-extensions</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:149</code> | <code>pub type Extensions = BTreeMap&lt;String, Extension&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-extensions</code> | notes | type alias layerx_x402::model::Extensions; no reference outside the defining file found by name match |

## x402-facilitator

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-facilitator</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:213</code> | <code>pub struct Facilitator</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-facilitator</code> | notes | struct layerx_x402::facilitator::Facilitator; re-exported at services/interop/crates/layerx-x402/src/lib.rs:11 |

## x402-facilitator-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:72</code> | <code>pub struct FacilitatorKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-facilitator-kind</code> | notes | struct layerx_x402::facilitator::FacilitatorKind |

## x402-facilitator-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:223</code> | <code>pub fn new(supported: SupportedResponse) -&gt; Result&lt;Self, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-facilitator-new</code> | notes | method layerx_x402::facilitator::Facilitator::new |

## x402-facilitator-payment-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-payment-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:111</code> | <code>pub struct FacilitatorPaymentRequest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-facilitator-payment-request</code> | notes | struct layerx_x402::facilitator::FacilitatorPaymentRequest |

## x402-facilitator-plane

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-plane</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:187</code> | <code>pub trait FacilitatorPlane</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-facilitator-plane</code> | notes | trait layerx_x402::facilitator::FacilitatorPlane |

## x402-facilitator-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:34</code> | <code>pub struct FacilitatorRequest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-facilitator-request</code> | notes | struct layerx_x402::facilitator::FacilitatorRequest |

## x402-facilitator-settle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-settle</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:289</code> | <code>pub fn settle(&amp;self, gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, request: &amp;FacilitatorRequest, stable_identity: [u8; 32], step: SettlementStep, plane: &amp;mut impl FacilitatorPlane, trace: &amp;TraceId, now: u64) -&gt; Result&lt;SettlementResponse, Traced&lt;X402Error&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-facilitator-settle</code> | notes | method layerx_x402::facilitator::Facilitator::settle; spec/layerx-merge/spec.kvx task 0.101 moves the facilitator settle path (services/interop/crates/layerx-x402/src/facilitator.rs:242, :291, :444) onto GatewayCore sequencer-receipt settlement with transaction = txHash |

## x402-facilitator-settlement-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-settlement-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:173</code> | <code>pub enum FacilitatorSettlementOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-facilitator-settlement-outcome</code> | notes | enum layerx_x402::facilitator::FacilitatorSettlementOutcome |

## x402-facilitator-supported

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-supported</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:230</code> | <code>pub fn supported(&amp;self) -&gt; SupportedResponse</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-facilitator-supported</code> | notes | method layerx_x402::facilitator::Facilitator::supported |

## x402-facilitator-verify

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-verify</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:240</code> | <code>pub fn verify(&amp;self, gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, request: &amp;FacilitatorRequest, plane: &amp;mut impl FacilitatorPlane, trace: &amp;TraceId, now: u64) -&gt; Result&lt;VerifyResponse, Traced&lt;X402Error&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-facilitator-verify</code> | notes | method layerx_x402::facilitator::Facilitator::verify; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 0.101 moves the facilitator settle path (services/interop/crates/layerx-x402/src/facilitator.rs:242, :291, :444) onto GatewayCore sequencer-receipt settlement with transaction = txHash |

## x402-interop-x402-buyer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-interop-x402-buyer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/lib.rs:63</code> | <code>pub const fn interop_x402_buyer() -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-interop-x402-buyer</code> | notes | fn layerx_x402::interop_x402_buyer; no reference outside the defining file found by name match |

## x402-interop-x402-facilitator

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-interop-x402-facilitator</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/lib.rs:69</code> | <code>pub const fn interop_x402_facilitator() -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-interop-x402-facilitator</code> | notes | fn layerx_x402::interop_x402_facilitator; no reference outside the defining file found by name match |

## x402-interop-x402-seller

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-interop-x402-seller</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/lib.rs:57</code> | <code>pub const fn interop_x402_seller() -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-interop-x402-seller</code> | notes | fn layerx_x402::interop_x402_seller; no reference outside the defining file found by name match |

## x402-layerx-network-namespace

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-layerx-network-namespace</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:19</code> | <code>pub const LAYERX_NETWORK_NAMESPACE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-layerx-network-namespace</code> | notes | const layerx_x402::model::LAYERX_NETWORK_NAMESPACE; no reference outside the defining file found by name match |

## x402-layerx-payment-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-layerx-payment-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/seller.rs:31</code> | <code>pub struct LayerXPaymentRequest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-layerx-payment-request</code> | notes | struct layerx_x402::seller::LayerXPaymentRequest |

## x402-layerx-terms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-layerx-terms</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:234</code> | <code>pub struct LayerXTerms</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-layerx-terms</code> | notes | struct layerx_x402::model::LayerXTerms; re-exported at services/interop/crates/layerx-x402/src/lib.rs:12 |

## x402-max-account-did-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-max-account-did-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:23</code> | <code>pub const MAX_ACCOUNT_DID_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-max-account-did-bytes</code> | notes | const layerx_x402::model::MAX_ACCOUNT_DID_BYTES; no reference outside the defining file found by name match |

## x402-max-account-reference-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-max-account-reference-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:21</code> | <code>pub const MAX_ACCOUNT_REFERENCE_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-max-account-reference-bytes</code> | notes | const layerx_x402::model::MAX_ACCOUNT_REFERENCE_BYTES; no reference outside the defining file found by name match |

## x402-max-currency-code-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-max-currency-code-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:25</code> | <code>pub const MAX_CURRENCY_CODE_BYTES: usize</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-max-currency-code-bytes</code> | notes | const layerx_x402::model::MAX_CURRENCY_CODE_BYTES; no reference outside the defining file found by name match |

## x402-payment-build-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-build-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/buyer.rs:26</code> | <code>pub struct PaymentBuildRequest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-build-request</code> | notes | struct layerx_x402::buyer::PaymentBuildRequest |

## x402-payment-payload

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-payload</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:402</code> | <code>pub struct PaymentPayload</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-payload</code> | notes | struct layerx_x402::model::PaymentPayload; re-exported at services/interop/crates/layerx-x402/src/lib.rs:12 |

## x402-payment-payload-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-payload-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:415</code> | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-payload-validate</code> | notes | method layerx_x402::model::PaymentPayload::validate; no reference outside the defining file found by name match |

## x402-payment-plane

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-plane</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/seller.rs:62</code> | <code>pub trait PaymentPlane</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-plane</code> | notes | trait layerx_x402::seller::PaymentPlane |

## x402-payment-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-required</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:368</code> | <code>pub struct PaymentRequired</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-required</code> | notes | struct layerx_x402::model::PaymentRequired; re-exported at services/interop/crates/layerx-x402/src/lib.rs:12 |

## x402-payment-required-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-required-header</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:14</code> | <code>pub const PAYMENT_REQUIRED_HEADER: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-required-header</code> | notes | const layerx_x402::model::PAYMENT_REQUIRED_HEADER |

## x402-payment-required-signal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-required-signal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/seller.rs:76</code> | <code>pub struct PaymentRequiredSignal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-required-signal</code> | notes | struct layerx_x402::seller::PaymentRequiredSignal; no reference outside the defining file found by name match |

## x402-payment-required-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-required-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:381</code> | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-required-validate</code> | notes | method layerx_x402::model::PaymentRequired::validate; no reference outside the defining file found by name match |

## x402-payment-requirements

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-requirements</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:153</code> | <code>pub struct PaymentRequirements</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-requirements</code> | notes | struct layerx_x402::model::PaymentRequirements; 59 reference lines in 11 files; callers lists the first reference in each file; re-exported at services/interop/crates/layerx-x402/src/lib.rs:12 |

## x402-payment-requirements-is-layerx-network

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-requirements-is-layerx-network</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:189</code> | <code>pub fn is_layerx_network(&amp;self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-requirements-is-layerx-network</code> | notes | method layerx_x402::model::PaymentRequirements::is_layerx_network; no reference outside the defining file found by name match |

## x402-payment-requirements-layerx-facts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-requirements-layerx-facts</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:224</code> | <code>pub fn layerx_facts(&amp;self) -&gt; Result&lt;([u8; 32], [u8; 32]), X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-requirements-layerx-facts</code> | notes | method layerx_x402::model::PaymentRequirements::layerx_facts |

## x402-payment-requirements-layerx-terms

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-requirements-layerx-terms</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:208</code> | <code>pub fn layerx_terms(&amp;self) -&gt; Result&lt;LayerXTerms, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-requirements-layerx-terms</code> | notes | method layerx_x402::model::PaymentRequirements::layerx_terms |

## x402-payment-requirements-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-requirements-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:169</code> | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-requirements-validate</code> | notes | method layerx_x402::model::PaymentRequirements::validate; no reference outside the defining file found by name match |

## x402-payment-response-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-response-header</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:16</code> | <code>pub const PAYMENT_RESPONSE_HEADER: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-response-header</code> | notes | const layerx_x402::model::PAYMENT_RESPONSE_HEADER |

## x402-payment-signature-header

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-payment-signature-header</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:15</code> | <code>pub const PAYMENT_SIGNATURE_HEADER: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-payment-signature-header</code> | notes | const layerx_x402::model::PAYMENT_SIGNATURE_HEADER |

## x402-plane-payment-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-plane-payment-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/seller.rs:55</code> | <code>pub enum PlanePaymentOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-plane-payment-outcome</code> | notes | enum layerx_x402::seller::PlanePaymentOutcome |

## x402-plane-verify-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-plane-verify-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:160</code> | <code>pub enum PlaneVerifyOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-plane-verify-outcome</code> | notes | enum layerx_x402::facilitator::PlaneVerifyOutcome |

## x402-resource-info

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-resource-info</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:96</code> | <code>pub struct ResourceInfo</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-resource-info</code> | notes | struct layerx_x402::model::ResourceInfo |

## x402-resource-info-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-resource-info-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:113</code> | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-resource-info-validate</code> | notes | method layerx_x402::model::ResourceInfo::validate; no reference outside the defining file found by name match |

## x402-seller

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-seller</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/seller.rs:98</code> | <code>pub struct Seller</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-seller</code> | notes | struct layerx_x402::seller::Seller; re-exported at services/interop/crates/layerx-x402/src/lib.rs:16 |

## x402-seller-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-seller-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/seller.rs:108</code> | <code>pub fn new(required: PaymentRequired) -&gt; Result&lt;Self, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-seller-new</code> | notes | method layerx_x402::seller::Seller::new |

## x402-seller-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-seller-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/seller.rs:83</code> | <code>pub enum SellerOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-seller-outcome</code> | notes | enum layerx_x402::seller::SellerOutcome |

## x402-seller-payment-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-seller-payment-required</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/seller.rs:122</code> | <code>pub fn payment_required(&amp;self) -&gt; Result&lt;PaymentRequiredSignal, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-seller-payment-required</code> | notes | method layerx_x402::seller::Seller::payment_required |

## x402-seller-settle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-seller-settle</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-x402/src/seller.rs:138</code> | <code>pub fn settle(&amp;self, gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, payment_header: &amp;str, plane: &amp;mut impl PaymentPlane, trace: &amp;TraceId, now: u64) -&gt; Result&lt;SellerOutcome, Traced&lt;X402Error&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-seller-settle</code> | notes | method layerx_x402::seller::Seller::settle; spec/layerx-merge/spec.kvx task 7.5.17 (in_progress) replaces authorised batch verification (services/interop/crates/layerx-x402/src/seller.rs:243) with ProtocolReceipt v2 against ReceiptKeys and a 0x101A PaymentExact/PaymentDrawn match |

## x402-sequencer-settlement

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.11.1</code> | req | 11011.1 |
| <code>acceptance.11.11.1</code> | scenario | Given a PAYMENT-RESPONSE whose layerx evidence says committed and carries no blockHash, when capture_settlement parses it, then it returns EvidenceMismatch. |
| <code>acceptance.11.11.1</code> | test | services/interop/crates/layerx-x402/tests/layerx_evidence.rs::committed_evidence_without_block_fields_is_refused (planned) |
| <code>acceptance.11.11.1</code> | qualification | UNRUN |
| <code>req.11011</code> | ac_1 | A committed verificationLevel without blockNumber and blockHash is refused with EvidenceMismatch (planned) (services/interop/crates/layerx-x402/src/buyer.rs:59). |
| <code>acceptance.11.11.2</code> | req | 11011.2 |
| <code>acceptance.11.11.2</code> | scenario | Given a receipted evidence object that also carries blockNumber 0x10 and a blockHash, when capture_settlement parses it, then it returns EvidenceMismatch. |
| <code>acceptance.11.11.2</code> | test | services/interop/crates/layerx-x402/tests/layerx_evidence.rs::receipted_evidence_with_a_block_hash_is_refused (planned) |
| <code>acceptance.11.11.2</code> | qualification | UNRUN |
| <code>req.11011</code> | ac_2 | A receipted verificationLevel carrying blockHash is refused with EvidenceMismatch (planned) (services/interop/crates/layerx-x402/src/buyer.rs:59). |
| <code>acceptance.11.17.1</code> | req | 11017.1 |
| <code>acceptance.11.17.1</code> | scenario | Given an offer matching the recorded payExact and its receipted evidence, when Seller::settle runs, then the response transaction is the recorded txHash and the level is receipted. |
| <code>acceptance.11.17.1</code> | test | services/interop/crates/layerx-x402/tests/seller.rs::seller_settles_receipted_evidence_with_the_transaction_hash (planned) |
| <code>acceptance.11.17.1</code> | qualification | UNRUN |
| <code>req.11017</code> | ac_1 | Seller::settle reports the recorded txHash of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:109 as transaction with verificationLevel receipted (planned). |
| <code>acceptance.11.17.2</code> | req | 11017.2 |
| <code>acceptance.11.17.2</code> | scenario | Given the same offer and evidence at the submitted rung, when Seller::settle runs, then the outcome is pending with the recorded txHash and no settlement is recorded. |
| <code>acceptance.11.17.2</code> | test | services/interop/crates/layerx-x402/tests/seller.rs::seller_reports_a_submitted_rung_as_pending (planned) |
| <code>acceptance.11.17.2</code> | qualification | UNRUN |
| <code>req.11017</code> | ac_2 | Submitted evidence yields a pending outcome carrying the transaction hash (planned) (services/interop/crates/layerx-x402/src/seller.rs:55). |
| <code>acceptance.11.18.1</code> | req | 11018.1 |
| <code>acceptance.11.18.1</code> | scenario | Given a plane returning the recorded receipted evidence, when Facilitator::settle runs, then the response is success with transaction the recorded txHash and level receipted. |
| <code>acceptance.11.18.1</code> | test | services/interop/crates/layerx-x402/tests/facilitator_evidence.rs::facilitator_settles_receipted_evidence (planned) |
| <code>acceptance.11.18.1</code> | qualification | UNRUN |
| <code>req.11018</code> | ac_1 | Facilitator::settle answers success with the recorded txHash of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:109 at the receipted rung (planned). |
| <code>acceptance.11.18.2</code> | req | 11018.2 |
| <code>acceptance.11.18.2</code> | scenario | Given a plane returning a submitted rung for the recorded hash, when Facilitator::settle runs, then the response is pending with that hash. |
| <code>acceptance.11.18.2</code> | test | services/interop/crates/layerx-x402/tests/facilitator_evidence.rs::facilitator_pending_reports_the_relayed_hash (planned) |
| <code>acceptance.11.18.2</code> | qualification | UNRUN |
| <code>req.11018</code> | ac_2 | render_pending carries the relayed transaction hash with level submitted (planned) (services/interop/crates/layerx-x402/src/facilitator.rs:408). |
| <code>acceptance.11.19.1</code> | req | 11019.1 |
| <code>acceptance.11.19.1</code> | scenario | Given a PAYMENT-RESPONSE carrying the recorded signedReceipt, logs and receiptDigest, when capture_settlement runs with the pinned key, then it returns the recorded txHash at rung receipted. |
| <code>acceptance.11.19.1</code> | test | services/interop/crates/layerx-x402/tests/buyer_sequencer.rs::buyer_captures_the_recorded_receipted_settlement (planned) |
| <code>acceptance.11.19.1</code> | qualification | UNRUN |
| <code>req.11019</code> | ac_1 | capture_settlement accepts the recorded receipt of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:109 under its pinnedKey and returns its txHash (planned). |
| <code>acceptance.11.19.2</code> | req | 11019.2 |
| <code>acceptance.11.19.2</code> | scenario | Given the signedReceipt and logs of case V3-tampered-log, when capture_settlement runs, then it returns EvidenceMismatch. |
| <code>acceptance.11.19.2</code> | test | services/interop/crates/layerx-x402/tests/buyer_sequencer.rs::buyer_refuses_a_tampered_log (planned) |
| <code>acceptance.11.19.2</code> | qualification | UNRUN |
| <code>req.11019</code> | ac_2 | capture_settlement refuses the tampered-log case of the receipt fixture with EvidenceMismatch (planned) (services/agent/crates/layerx-proof/src/sequencer_logs.rs:410). |
| <code>acceptance.11.19.3</code> | req | 11019.3 |
| <code>acceptance.11.19.3</code> | scenario | Given the recorded evidence with receiptDigest replaced by SHA-256 of the signedReceipt bytes, when capture_settlement runs, then it returns EvidenceMismatch. |
| <code>acceptance.11.19.3</code> | test | services/interop/crates/layerx-x402/tests/buyer_sequencer.rs::buyer_refuses_a_digest_over_the_signed_bytes (planned) |
| <code>acceptance.11.19.3</code> | qualification | UNRUN |
| <code>req.11019</code> | ac_3 | A receiptDigest computed over the whole signed bytes is refused (planned) (services/interop/crates/layerx-x402/src/buyer.rs:59). |
| <code>acceptance.11.2.1</code> | req | 11002.1 |
| <code>acceptance.11.2.1</code> | scenario | Given X402Error::TransactionForm, when it is formatted with Display, then the text contains 0x-prefixed 32-byte transaction hash and differs from the EvidenceMismatch text. |
| <code>acceptance.11.2.1</code> | test | services/interop/crates/layerx-x402/tests/errors.rs::transaction_form_error_names_the_tx_hash_form (planned) |
| <code>acceptance.11.2.1</code> | qualification | UNRUN |
| <code>req.11002</code> | ac_1 | X402Error::TransactionForm renders a message naming the 0x-prefixed 32-byte transaction hash form (planned) beside the variants of services/interop/crates/layerx-x402/src/model.rs:485. |
| <code>acceptance.11.2.2</code> | req | 11002.2 |
| <code>acceptance.11.2.2</code> | scenario | Given settlement transactions lxp:00, 0x followed by 63 hex digits and 0x followed by 64 zero digits, when validate_transaction checks each, then each is refused with X402Error::TransactionForm. |
| <code>acceptance.11.2.2</code> | test | services/interop/crates/layerx-x402/src/seller.rs::lxp_and_short_transactions_are_refused_as_transaction_form (planned) |
| <code>acceptance.11.2.2</code> | qualification | UNRUN |
| <code>req.11002</code> | ac_2 | validate_transaction refuses an lxp: transaction identifier, a 63-digit hash and the all-zero hash with TransactionForm (planned) (services/interop/crates/layerx-x402/src/seller.rs:312). |
| <code>acceptance.11.30.1</code> | req | 11030.1 |
| <code>acceptance.11.30.1</code> | scenario | Given an offer for 2000000 and the recorded evidence paying 1000000, when Seller::settle runs, then it returns EvidenceMismatch and records nothing. |
| <code>acceptance.11.30.1</code> | test | services/interop/crates/layerx-x402/tests/seller.rs::seller_refuses_a_log_for_another_amount (planned) |
| <code>acceptance.11.30.1</code> | qualification | UNRUN |
| <code>req.11030</code> | ac_1 | A PaymentExact log for another amount is refused with EvidenceMismatch (planned) (services/interop/crates/layerx-x402/src/model.rs:501). |
| <code>acceptance.11.30.2</code> | req | 11030.2 |
| <code>acceptance.11.30.2</code> | scenario | Given the recorded PAX transfer evidence of case V1-pax-transfer, when Seller::settle runs, then it returns EvidenceMismatch. |
| <code>acceptance.11.30.2</code> | test | services/interop/crates/layerx-x402/tests/seller.rs::seller_refuses_evidence_without_a_payment_log (planned) |
| <code>acceptance.11.30.2</code> | qualification | UNRUN |
| <code>req.11030</code> | ac_2 | Evidence without a PaymentExact log is refused (planned) (services/interop/crates/layerx-x402/src/seller.rs:236). |
| <code>acceptance.11.41.1</code> | req | 11041.1 |
| <code>acceptance.11.41.1</code> | scenario | Given an offer and evidence carrying one matching PaymentDrawn log for the named grant, when Seller::settle runs, then it settles and refuses the same evidence for another grant id. |
| <code>acceptance.11.41.1</code> | test | services/interop/crates/layerx-x402/tests/seller.rs::seller_settles_a_drawn_payment (planned) |
| <code>acceptance.11.41.1</code> | qualification | UNRUN |
| <code>req.11041</code> | ac_1 | A PaymentDrawn log matching the offer settles (planned) (services/agent/crates/layerx-proof/src/sequencer_logs.rs:113). |
| <code>acceptance.11.42.1</code> | req | 11042.1 |
| <code>acceptance.11.42.1</code> | scenario | Given a SettlementResponse with transaction lxp:abc, when validate_wire runs, then it returns the TransactionForm error, and the recorded txHash passes. |
| <code>acceptance.11.42.1</code> | test | services/interop/crates/layerx-x402/tests/errors.rs::settlement_response_refuses_a_non_hash_transaction (planned) |
| <code>acceptance.11.42.1</code> | qualification | UNRUN |
| <code>req.11042</code> | ac_1 | validate_wire refuses a settlement whose transaction is not a 32-byte hash (planned) (services/interop/crates/layerx-x402/src/model.rs:447). |
| <code>acceptance.11.47.1</code> | req | 11047.1 |
| <code>acceptance.11.47.1</code> | scenario | Given a plane returning a committed rung without proof, when settle runs, then the level is receipted; with the recorded committed proof it is committed with the block fields. |
| <code>acceptance.11.47.1</code> | test | services/interop/crates/layerx-x402/tests/facilitator_evidence.rs::facilitator_reports_committed_only_with_a_proof (planned) |
| <code>acceptance.11.47.1</code> | qualification | UNRUN |
| <code>req.11047</code> | ac_1 | A committed rung without a proof is rendered receipted (planned) (services/interop/crates/layerx-x402/src/facilitator.rs:484). |
| <code>acceptance.11.48.1</code> | req | 11048.1 |
| <code>acceptance.11.48.1</code> | scenario | Given the recorded committed vector with blockHash altered, when capture_settlement runs, then it returns EvidenceMismatch, and the unaltered vector is captured as committed. |
| <code>acceptance.11.48.1</code> | test | services/interop/crates/layerx-x402/tests/buyer_sequencer.rs::buyer_refuses_a_committed_rung_with_a_wrong_block_hash (planned) |
| <code>acceptance.11.48.1</code> | qualification | UNRUN |
| <code>req.11048</code> | ac_1 | A committed rung whose proof names another block hash is refused (planned) (services/interop/crates/layerx-x402/src/buyer.rs:162). |
| <code>acceptance.11.56.1</code> | req | 11056.1 |
| <code>acceptance.11.56.1</code> | scenario | Given the rewritten signed_settlement, when capture_settlement runs with the pinned key, then it returns the recorded txHash. |
| <code>acceptance.11.56.1</code> | test | services/interop/crates/layerx-x402/tests/buyer.rs::buyer_tests_capture_on_the_recorded_receipt (planned) |
| <code>acceptance.11.56.1</code> | qualification | UNRUN |
| <code>req.11056</code> | ac_1 | The buyer suite captures the recorded receipt without AuthorizedBatch (planned) (services/interop/crates/layerx-x402/tests/buyer.rs:692). |
| <code>capability.11.3.1</code> | title | Settle the x402 seller plane on SequencerEvidence and report the transaction hash |
| <code>capability.11.3.1</code> | reqs | 11017.1<br>11017.2 |
| <code>capability.11.3.2</code> | title | Match the x402 seller offer against the PaymentExact log instead of protocol facts |
| <code>capability.11.3.2</code> | reqs | 11030.1<br>11030.2 |
| <code>capability.11.3.3</code> | title | Accept PaymentDrawn logs for x402 sellers paid from a standing grant |
| <code>capability.11.3.3</code> | reqs | 11041.1 |
| <code>capability.11.4.1</code> | title | Settle the x402 facilitator plane on SequencerEvidence |
| <code>capability.11.4.1</code> | reqs | 11018.1<br>11018.2 |
| <code>capability.11.4.2</code> | title | Report the committed rung from the x402 facilitator only with a committed proof |
| <code>capability.11.4.2</code> | reqs | 11047.1 |
| <code>capability.11.5.1</code> | title | Verify the signed receipt and PaymentExact log when the x402 buyer captures |
| <code>capability.11.5.1</code> | reqs | 11019.1<br>11019.2<br>11019.3 |
| <code>capability.11.5.2</code> | title | Accept a committed rung at the x402 buyer only with a verifying proof |
| <code>capability.11.5.2</code> | reqs | 11048.1 |
| <code>capability.11.5.3</code> | title | Move the x402 buyer tests off AuthorizedBatch |
| <code>capability.11.5.3</code> | reqs | 11056.1 |
| <code>capability.11.6.1</code> | title | Add the x402 TransactionForm error for a settlement transaction that is not a tx hash |
| <code>capability.11.6.1</code> | reqs | 11002.1<br>11002.2 |
| <code>capability.11.6.2</code> | title | Carry blockNumber and blockHash in the x402 layerx evidence for the committed rung |
| <code>capability.11.6.2</code> | reqs | 11011.1<br>11011.2 |
| <code>capability.11.6.3</code> | title | Validate the x402 settlement response transaction against TransactionForm |
| <code>capability.11.6.3</code> | reqs | 11042.1 |
| <code>resolution.17.881</code> | title | decision.1104 binds: Committed is reported only with a verified committed proof |
| <code>resolution.17.881</code> | applies_to | decision.1104<br>task.11.4.2<br>task.11.5.2 |
| <code>resolution.17.881</code> | source | decision.1104 in fragments/J47/decision.kvx; tasks from affects |
| <code>resolution.17.882</code> | title | decision.1105 binds: A pending x402 settlement names the transaction hash |
| <code>resolution.17.882</code> | applies_to | decision.1105<br>task.11.4.1<br>task.11.16.2 |
| <code>resolution.17.882</code> | source | decision.1105 in fragments/J47/decision.kvx; tasks from affects |
| <code>resolution.17.883</code> | title | decision.1106 binds: AP2 keeps the lxp: digest reference |
| <code>resolution.17.883</code> | applies_to | decision.1106<br>task.11.6.1<br>task.11.7.1 |
| <code>resolution.17.883</code> | source | decision.1106 in fragments/J47/decision.kvx; tasks from affects |

## x402-settlement-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-settlement-identity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:123</code> | <code>pub struct SettlementIdentity</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-settlement-identity</code> | notes | struct layerx_x402::facilitator::SettlementIdentity |

## x402-settlement-identity-derive

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-settlement-identity-derive</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:136</code> | <code>pub fn derive(principal: &amp;PrincipalId, request: &amp;FacilitatorRequest, stable_identity: [u8; 32], step: SettlementStep) -&gt; Result&lt;Self, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-settlement-identity-derive</code> | notes | method layerx_x402::facilitator::SettlementIdentity::derive |

## x402-settlement-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-settlement-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:430</code> | <code>pub struct SettlementResponse</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-settlement-response</code> | notes | struct layerx_x402::model::SettlementResponse; re-exported at services/interop/crates/layerx-x402/src/lib.rs:12 |

## x402-settlement-response-validate-wire

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-settlement-response-validate-wire</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:447</code> | <code>pub fn validate_wire(&amp;self) -&gt; Result&lt;(), X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-settlement-response-validate-wire</code> | notes | method layerx_x402::model::SettlementResponse::validate_wire |

## x402-settlement-step

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-settlement-step</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:93</code> | <code>pub enum SettlementStep</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-settlement-step</code> | notes | enum layerx_x402::facilitator::SettlementStep |

## x402-supported-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-supported-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/buyer.rs:20</code> | <code>pub struct SupportedKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-supported-kind</code> | notes | struct layerx_x402::buyer::SupportedKind |

## x402-supported-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-supported-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:83</code> | <code>pub struct SupportedResponse</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-supported-response</code> | notes | struct layerx_x402::facilitator::SupportedResponse |

## x402-transport-capability

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-transport-capability</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:38</code> | <code>pub struct TransportCapability</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-transport-capability</code> | notes | struct layerx_x402::transport::TransportCapability; no reference outside the defining file found by name match |

## x402-transport-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-transport-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:23</code> | <code>pub enum TransportKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-transport-kind</code> | notes | enum layerx_x402::transport::TransportKind |

## x402-transport-matrix

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-transport-matrix</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:48</code> | <code>pub const TRANSPORT_MATRIX: [TransportCapability; 3]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-transport-matrix</code> | notes | const layerx_x402::transport::TRANSPORT_MATRIX |

## x402-transport-value

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-transport-value</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/transport.rs:31</code> | <code>pub enum TransportValue</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-transport-value</code> | notes | enum layerx_x402::transport::TransportValue |

## x402-verify-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-verify-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/facilitator.rs:59</code> | <code>pub struct VerifyResponse</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-verify-response</code> | notes | struct layerx_x402::facilitator::VerifyResponse |

## x402-x402-adapter-descriptor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-x402-adapter-descriptor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/lib.rs:42</code> | <code>pub fn x402_adapter_descriptor(conformance: ConformanceSuite) -&gt; Result&lt;AdapterDescriptor, X402Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-x402-adapter-descriptor</code> | notes | fn layerx_x402::x402_adapter_descriptor |

## x402-x402-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-x402-error</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-x402/src/model.rs:485</code> | <code>pub enum X402Error</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-x402-error</code> | notes | enum layerx_x402::model::X402Error; 238 reference lines in 14 files; callers lists the first reference in each file; spec/layerx-merge/spec.kvx task 0.101 do_3 adds X402Error::TransactionForm for an lxp: transaction value |

## x402-x402-spec-commit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-x402-spec-commit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/lib.rs:25</code> | <code>pub const X402_SPEC_COMMIT: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-x402-spec-commit</code> | notes | const layerx_x402::X402_SPEC_COMMIT |

## x402-x402-spec-sha256

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-x402-spec-sha256</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/lib.rs:27</code> | <code>pub const X402_SPEC_SHA256: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-x402-spec-sha256</code> | notes | const layerx_x402::X402_SPEC_SHA256 |

## x402-x402-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-x402-x402-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-x402/src/model.rs:13</code> | <code>pub const X402_VERSION: u8</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-x402-x402-version</code> | notes | const layerx_x402::model::X402_VERSION |
