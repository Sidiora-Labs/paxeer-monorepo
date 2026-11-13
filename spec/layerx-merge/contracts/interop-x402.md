# Interop x402 contract

## fmt-x402-extension

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-x402-extension</code> | <code>file_format</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct Extension { pub info: Value, pub schema: Value, }</code> |

## fmt-x402-facilitator-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-x402-facilitator-kind</code> | <code>file_format</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>Rust model: #[serde(deny_unknown_fields, rename_all = "camelCase")] pub struct FacilitatorKind { pub x402_version: u8, pub scheme: String, pub network: String, #[serde(default, skip_serializing_if = "Option::is_none")] pub extra: Option&lt;Value&gt;, }</code> |

## fmt-x402-facilitator-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-x402-facilitator-request</code> | <code>file_format</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>Rust model: #[serde(deny_unknown_fields, rename_all = "camelCase")] pub struct FacilitatorRequest { pub x402_version: u8, pub payment_payload: PaymentPayload, pub payment_requirements: PaymentRequirements, }</code> |

## fmt-x402-layerx-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-x402-layerx-evidence</code> | <code>file_format</code> | [services/interop/crates/layerx-x402/src/buyer.rs](../../../services/interop/crates/layerx-x402/src/buyer.rs) | <code>Rust model: #[serde(deny_unknown_fields, rename_all = "camelCase")] pub struct LayerXEvidence { receipt: String, receipt_digest: String, verification_level: String, logs: Vec&lt;EvidenceLog&gt;, #[serde(default)] block_number: Option&lt;String&gt;, #[serde(default)] block_hash: Option&lt;String&gt;, #[serde(default)] purpose_hash: Option&lt;String&gt;, #[serde(default)] proof: Option&lt;Value&gt;, }</code> |

## fmt-x402-payment-payload

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-x402-payment-payload</code> | <code>file_format</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>Rust model: #[serde(deny_unknown_fields, rename_all = "camelCase")] pub struct PaymentPayload { pub x402_version: u8, #[serde(default, skip_serializing_if = "Option::is_none")] pub resource: Option&lt;ResourceInfo&gt;, pub payload: Value, pub accepted: PaymentRequirements, #[serde(default, skip_serializing_if = "BTreeMap::is_empty")] pub extensions: Extensions, }</code> |

## fmt-x402-payment-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-x402-payment-required</code> | <code>file_format</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>Rust model: #[serde(deny_unknown_fields, rename_all = "camelCase")] pub struct PaymentRequired { pub x402_version: u8, #[serde(default, skip_serializing_if = "Option::is_none")] pub error: Option&lt;String&gt;, pub resource: ResourceInfo, pub accepts: Vec&lt;PaymentRequirements&gt;, #[serde(default, skip_serializing_if = "BTreeMap::is_empty")] pub extensions: Extensions, }</code> |

## fmt-x402-payment-requirements

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-x402-payment-requirements</code> | <code>file_format</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>Rust model: #[serde(deny_unknown_fields, rename_all = "camelCase")] pub struct PaymentRequirements { pub scheme: String, pub network: String, pub amount: AtomicAmount, pub asset: String, pub pay_to: String, pub max_timeout_seconds: u32, #[serde(default, skip_serializing_if = "Option::is_none")] pub extra: Option&lt;Value&gt;, }</code> |

## fmt-x402-resource-info

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-x402-resource-info</code> | <code>file_format</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>Rust model: #[serde(deny_unknown_fields, rename_all = "camelCase")] pub struct ResourceInfo { pub url: String, #[serde(default, skip_serializing_if = "Option::is_none")] pub description: Option&lt;String&gt;, #[serde(default, skip_serializing_if = "Option::is_none")] pub mime_type: Option&lt;String&gt;, #[serde(default, skip_serializing_if = "Option::is_none")] pub service_name: Option&lt;String&gt;, #[serde(default, skip_serializing_if = "Vec::is_empty")] pub tags: Vec&lt;String&gt;, #[serde(default, skip_serializing_if = "Option::is_none")] pub icon_url: Option&lt;String&gt;, }</code> |

## fmt-x402-settlement-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-x402-settlement-response</code> | <code>file_format</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>Rust model: #[serde(deny_unknown_fields, rename_all = "camelCase")] pub struct SettlementResponse { pub success: bool, #[serde(default, skip_serializing_if = "Option::is_none")] pub error_reason: Option&lt;String&gt;, #[serde(default, skip_serializing_if = "Option::is_none")] pub payer: Option&lt;String&gt;, pub transaction: String, pub network: String, #[serde(default, skip_serializing_if = "Option::is_none")] pub amount: Option&lt;AtomicAmount&gt;, #[serde(default, skip_serializing_if = "BTreeMap::is_empty")] pub extensions: BTreeMap&lt;String, Value&gt;, }</code> |

## fmt-x402-supported-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-x402-supported-response</code> | <code>file_format</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct SupportedResponse { pub kinds: Vec&lt;FacilitatorKind&gt;, pub extensions: Vec&lt;String&gt;, pub signers: BTreeMap&lt;String, Vec&lt;String&gt;&gt;, }</code> |

## fmt-x402-verify-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-x402-verify-response</code> | <code>file_format</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>Rust model: #[serde(deny_unknown_fields, rename_all = "camelCase")] pub struct VerifyResponse { pub is_valid: bool, #[serde(default, skip_serializing_if = "Option::is_none")] pub invalid_reason: Option&lt;String&gt;, #[serde(default, skip_serializing_if = "Option::is_none")] pub payer: Option&lt;String&gt;, #[serde(default, skip_serializing_if = "Option::is_none")] pub extra: Option&lt;Value&gt;, }</code> |

## rs-x402-error-transaction-form

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-error-transaction-form</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>X402Error::TransactionForm</code> |

| Field | Rule |
|---|---|
| interface | x402 seller, facilitator and buyer settle and capture on SequencerEvidence with transaction set to the transaction hash. |
| storage | None beyond the gateway records. |
| errors | X402Error::EvidenceMismatch, X402Error::TransactionForm. |
| events | None. |
| gas | Paid by the payer-signed transaction. |
| invariants | transaction is a 32-byte hash; committed only with a verified proof. |
| pause | Not pausable. |

## x402-account-identifiers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-account-identifiers</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub fn account_identifiers(account: &amp;str) -&gt; Result&lt;[[u8; 32]; 2], X402Error&gt;</code> |

## x402-atomic-amount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-atomic-amount</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub struct AtomicAmount(u128)</code> |

## x402-atomic-amount-from-u128

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-atomic-amount-from-u128</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub const fn from_u128(value: u128) -&gt; Self</code> |

## x402-atomic-amount-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-atomic-amount-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub fn parse(value: &amp;str) -&gt; Result&lt;Self, X402Error&gt;</code> |

## x402-atomic-amount-value

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-atomic-amount-value</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub const fn value(self) -&gt; u128</code> |

## x402-built-payment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-built-payment</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/buyer.rs](../../../services/interop/crates/layerx-x402/src/buyer.rs) | <code>pub struct BuiltPayment</code> |

## x402-buyer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-buyer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/buyer.rs](../../../services/interop/crates/layerx-x402/src/buyer.rs) | <code>pub struct Buyer</code> |

## x402-buyer-build-payment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-buyer-build-payment</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/buyer.rs](../../../services/interop/crates/layerx-x402/src/buyer.rs) | <code>pub fn build_payment(&amp;self, required_header: &amp;str, idempotency_key: [u8; 32], plane: &amp;mut impl BuyerPaymentPlane, trace: &amp;TraceId) -&gt; Result&lt;BuiltPayment, Traced&lt;X402Error&gt;&gt;</code> |

## x402-buyer-capture-settlement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-buyer-capture-settlement</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/buyer.rs](../../../services/interop/crates/layerx-x402/src/buyer.rs) | <code>pub fn capture_settlement( response_header: &amp;str, expected: &amp;BuiltPayment, receipt_keys: &amp;[SequencerAuthorization], trace: &amp;TraceId, ) -&gt; Result&lt;CapturedSettlement, Traced&lt;X402Error&gt;&gt;</code> |

## x402-buyer-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-buyer-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/buyer.rs](../../../services/interop/crates/layerx-x402/src/buyer.rs) | <code>pub fn new(supported: Vec&lt;SupportedKind&gt;) -&gt; Result&lt;Self, X402Error&gt;</code> |

## x402-buyer-payment-plane

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-buyer-payment-plane</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/buyer.rs](../../../services/interop/crates/layerx-x402/src/buyer.rs) | <code>pub trait BuyerPaymentPlane</code> |

## x402-captured-settlement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-captured-settlement</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/buyer.rs](../../../services/interop/crates/layerx-x402/src/buyer.rs) | <code>pub struct CapturedSettlement</code> |

## x402-decode-facilitator-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-decode-facilitator-request</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn decode_facilitator_request(_transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;FacilitatorRequest, X402Error&gt;</code> |

## x402-decode-facilitator-settlement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-decode-facilitator-settlement</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn decode_facilitator_settlement(_transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;SettlementResponse, X402Error&gt;</code> |

## x402-decode-payment-payload

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-decode-payment-payload</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn decode_payment_payload(transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;PaymentPayload, X402Error&gt;</code> |

## x402-decode-payment-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-decode-payment-required</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn decode_payment_required(transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;PaymentRequired, X402Error&gt;</code> |

## x402-decode-settlement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-decode-settlement</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn decode_settlement(transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;SettlementResponse, X402Error&gt;</code> |

## x402-decode-supported-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-decode-supported-response</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn decode_supported_response(_transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;SupportedResponse, X402Error&gt;</code> |

## x402-decode-verify-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-decode-verify-response</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn decode_verify_response(_transport: TransportKind, value: &amp;TransportValue) -&gt; Result&lt;VerifyResponse, X402Error&gt;</code> |

## x402-encode-facilitator-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-encode-facilitator-request</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn encode_facilitator_request(_transport: TransportKind, value: &amp;FacilitatorRequest) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

## x402-encode-facilitator-settlement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-encode-facilitator-settlement</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn encode_facilitator_settlement(_transport: TransportKind, value: &amp;SettlementResponse) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

## x402-encode-payment-payload

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-encode-payment-payload</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn encode_payment_payload(transport: TransportKind, value: &amp;PaymentPayload) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

## x402-encode-payment-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-encode-payment-required</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn encode_payment_required(transport: TransportKind, value: &amp;PaymentRequired) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

## x402-encode-settlement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-encode-settlement</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn encode_settlement(transport: TransportKind, value: &amp;SettlementResponse) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

## x402-encode-supported-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-encode-supported-response</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn encode_supported_response(_transport: TransportKind, value: &amp;SupportedResponse) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

## x402-encode-verify-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-encode-verify-response</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub fn encode_verify_response(_transport: TransportKind, value: &amp;VerifyResponse) -&gt; Result&lt;TransportValue, X402Error&gt;</code> |

## x402-executed-payment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-executed-payment</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/seller.rs](../../../services/interop/crates/layerx-x402/src/seller.rs) | <code>pub struct ExecutedPayment</code> |

## x402-extension

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-extension</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub struct Extension</code> |

## x402-extensions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-extensions</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub type Extensions = BTreeMap&lt;String, Extension&gt;</code> |

## x402-facilitator

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-facilitator</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub struct Facilitator</code> |

## x402-facilitator-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub struct FacilitatorKind</code> |

## x402-facilitator-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub fn new(supported: SupportedResponse) -&gt; Result&lt;Self, X402Error&gt;</code> |

## x402-facilitator-payment-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-payment-request</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub struct FacilitatorPaymentRequest</code> |

## x402-facilitator-plane

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-plane</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub trait FacilitatorPlane</code> |

## x402-facilitator-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-request</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub struct FacilitatorRequest</code> |

## x402-facilitator-settle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-settle</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub fn settle(&amp;self, gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, request: &amp;FacilitatorRequest, stable_identity: [u8; 32], step: SettlementStep, plane: &amp;mut impl FacilitatorPlane, trace: &amp;TraceId, now: u64) -&gt; Result&lt;SettlementResponse, Traced&lt;X402Error&gt;&gt;</code> |

## x402-facilitator-settlement-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-settlement-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub enum FacilitatorSettlementOutcome</code> |

## x402-facilitator-supported

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-supported</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub fn supported(&amp;self) -&gt; SupportedResponse</code> |

## x402-facilitator-verify

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-facilitator-verify</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub fn verify(&amp;self, gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, request: &amp;FacilitatorRequest, plane: &amp;mut impl FacilitatorPlane, trace: &amp;TraceId, now: u64) -&gt; Result&lt;VerifyResponse, Traced&lt;X402Error&gt;&gt;</code> |

## x402-interop-x402-buyer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-interop-x402-buyer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/lib.rs](../../../services/interop/crates/layerx-x402/src/lib.rs) | <code>pub const fn interop_x402_buyer() -&gt; &amp;'static str</code> |

## x402-interop-x402-facilitator

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-interop-x402-facilitator</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/lib.rs](../../../services/interop/crates/layerx-x402/src/lib.rs) | <code>pub const fn interop_x402_facilitator() -&gt; &amp;'static str</code> |

## x402-interop-x402-seller

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-interop-x402-seller</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/lib.rs](../../../services/interop/crates/layerx-x402/src/lib.rs) | <code>pub const fn interop_x402_seller() -&gt; &amp;'static str</code> |

## x402-layerx-payment-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-layerx-payment-request</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/seller.rs](../../../services/interop/crates/layerx-x402/src/seller.rs) | <code>pub struct LayerXPaymentRequest</code> |

## x402-layerx-terms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-layerx-terms</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub struct LayerXTerms</code> |

## x402-max-account-did-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-max-account-did-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub const MAX_ACCOUNT_DID_BYTES: usize</code> |

## x402-max-account-reference-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-max-account-reference-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub const MAX_ACCOUNT_REFERENCE_BYTES: usize</code> |

## x402-max-currency-code-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-max-currency-code-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub const MAX_CURRENCY_CODE_BYTES: usize</code> |

## x402-payment-build-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-build-request</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/buyer.rs](../../../services/interop/crates/layerx-x402/src/buyer.rs) | <code>pub struct PaymentBuildRequest</code> |

## x402-payment-payload

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-payload</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub struct PaymentPayload</code> |

## x402-payment-payload-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-payload-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), X402Error&gt;</code> |

## x402-payment-plane

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-plane</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/seller.rs](../../../services/interop/crates/layerx-x402/src/seller.rs) | <code>pub trait PaymentPlane</code> |

## x402-payment-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-required</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub struct PaymentRequired</code> |

## x402-payment-required-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-required-header</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub const PAYMENT_REQUIRED_HEADER: &amp;str</code> |

## x402-payment-required-signal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-required-signal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/seller.rs](../../../services/interop/crates/layerx-x402/src/seller.rs) | <code>pub struct PaymentRequiredSignal</code> |

## x402-payment-required-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-required-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), X402Error&gt;</code> |

## x402-payment-requirements

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-requirements</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub struct PaymentRequirements</code> |

## x402-payment-requirements-is-layerx-network

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-requirements-is-layerx-network</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub fn is_layerx_network(&amp;self) -&gt; bool</code> |

## x402-payment-requirements-layerx-facts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-requirements-layerx-facts</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub fn layerx_facts(&amp;self) -&gt; Result&lt;([u8; 32], [u8; 32]), X402Error&gt;</code> |

## x402-payment-requirements-layerx-terms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-requirements-layerx-terms</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub fn layerx_terms(&amp;self) -&gt; Result&lt;LayerXTerms, X402Error&gt;</code> |

## x402-payment-requirements-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-requirements-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), X402Error&gt;</code> |

## x402-payment-response-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-response-header</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub const PAYMENT_RESPONSE_HEADER: &amp;str</code> |

## x402-payment-signature-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-payment-signature-header</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub const PAYMENT_SIGNATURE_HEADER: &amp;str</code> |

## x402-plane-payment-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-plane-payment-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/seller.rs](../../../services/interop/crates/layerx-x402/src/seller.rs) | <code>pub enum PlanePaymentOutcome</code> |

## x402-plane-verify-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-plane-verify-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub enum PlaneVerifyOutcome</code> |

## x402-resource-info

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-resource-info</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub struct ResourceInfo</code> |

## x402-resource-info-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-resource-info-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), X402Error&gt;</code> |

## x402-seller

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-seller</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/seller.rs](../../../services/interop/crates/layerx-x402/src/seller.rs) | <code>pub struct Seller</code> |

## x402-seller-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-seller-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/seller.rs](../../../services/interop/crates/layerx-x402/src/seller.rs) | <code>pub fn new(required: PaymentRequired) -&gt; Result&lt;Self, X402Error&gt;</code> |

## x402-seller-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-seller-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/seller.rs](../../../services/interop/crates/layerx-x402/src/seller.rs) | <code>pub enum SellerOutcome</code> |

## x402-seller-payment-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-seller-payment-required</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/seller.rs](../../../services/interop/crates/layerx-x402/src/seller.rs) | <code>pub fn payment_required(&amp;self) -&gt; Result&lt;PaymentRequiredSignal, X402Error&gt;</code> |

## x402-seller-settle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-seller-settle</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/seller.rs](../../../services/interop/crates/layerx-x402/src/seller.rs) | <code>pub fn settle(&amp;self, gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, payment_header: &amp;str, plane: &amp;mut impl PaymentPlane, trace: &amp;TraceId, now: u64) -&gt; Result&lt;SellerOutcome, Traced&lt;X402Error&gt;&gt;</code> |

## x402-settlement-identity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-settlement-identity</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub struct SettlementIdentity</code> |

## x402-settlement-identity-derive

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-settlement-identity-derive</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub fn derive(principal: &amp;PrincipalId, request: &amp;FacilitatorRequest, stable_identity: [u8; 32], step: SettlementStep) -&gt; Result&lt;Self, X402Error&gt;</code> |

## x402-settlement-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-settlement-response</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub struct SettlementResponse</code> |

## x402-settlement-response-validate-wire

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-settlement-response-validate-wire</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub fn validate_wire(&amp;self) -&gt; Result&lt;(), X402Error&gt;</code> |

## x402-settlement-step

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-settlement-step</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub enum SettlementStep</code> |

## x402-supported-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-supported-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/buyer.rs](../../../services/interop/crates/layerx-x402/src/buyer.rs) | <code>pub struct SupportedKind</code> |

## x402-supported-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-supported-response</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub struct SupportedResponse</code> |

## x402-transport-capability

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-transport-capability</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub struct TransportCapability</code> |

## x402-transport-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-transport-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub enum TransportKind</code> |

## x402-transport-matrix

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-transport-matrix</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub const TRANSPORT_MATRIX: [TransportCapability; 3]</code> |

## x402-transport-value

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-transport-value</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/transport.rs](../../../services/interop/crates/layerx-x402/src/transport.rs) | <code>pub enum TransportValue</code> |

## x402-verify-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-verify-response</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/facilitator.rs](../../../services/interop/crates/layerx-x402/src/facilitator.rs) | <code>pub struct VerifyResponse</code> |

## x402-x402-adapter-descriptor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-x402-adapter-descriptor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/lib.rs](../../../services/interop/crates/layerx-x402/src/lib.rs) | <code>pub fn x402_adapter_descriptor(conformance: ConformanceSuite) -&gt; Result&lt;AdapterDescriptor, X402Error&gt;</code> |

## x402-x402-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-x402-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub enum X402Error</code> |

## x402-x402-spec-commit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-x402-spec-commit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/lib.rs](../../../services/interop/crates/layerx-x402/src/lib.rs) | <code>pub const X402_SPEC_COMMIT: &amp;str</code> |

## x402-x402-spec-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-x402-spec-sha256</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/lib.rs](../../../services/interop/crates/layerx-x402/src/lib.rs) | <code>pub const X402_SPEC_SHA256: [u8; 32]</code> |

## x402-x402-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-x402-x402-version</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-x402/src/model.rs](../../../services/interop/crates/layerx-x402/src/model.rs) | <code>pub const X402_VERSION: u8</code> |
