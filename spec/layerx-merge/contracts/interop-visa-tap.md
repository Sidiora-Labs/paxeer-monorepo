# Interop Visa TAP contract

The [Visa TAP adapter](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) verifies trusted-agent credentials and intent bindings and binds payment evidence to authenticated transaction receipts.

## visa-agent-intent

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-agent-intent</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub enum AgentIntent</code> |

## visa-agent-public-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-agent-public-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub enum AgentPublicKey</code> |

## visa-bind-verified-agent

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-bind-verified-agent</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn bind_verified_agent(principal: &amp;PrincipalId, layerx_agent: [u8; 32], verified: &amp;VerifiedTrustedAgent, store: &amp;mut impl CredentialBindingStore, trace: &amp;TraceId) -&gt; Result&lt;CredentialBinding, TapError&gt;</code> |

## visa-canonical-tap-authority

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-canonical-tap-authority</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn canonical_tap_authority(value: &amp;str) -&gt; Result&lt;String, TapError&gt;</code> |

## visa-canonical-tap-path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-canonical-tap-path</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn canonical_tap_path(value: &amp;str) -&gt; Result&lt;String, TapError&gt;</code> |

## visa-credential-binding

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-credential-binding</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub struct CredentialBinding</code> |

## visa-credential-binding-store

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-credential-binding-store</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub trait CredentialBindingStore</code> |

## visa-interop-visa-tap

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-interop-visa-tap</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub const fn interop_visa_tap() -&gt; &amp;'static str</code> |

## visa-key-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-key-status</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub enum KeyStatus</code> |

## visa-layerx-intent-authority

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-layerx-intent-authority</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub trait LayerXIntentAuthority</code> |

## visa-max-clock-skew-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-max-clock-skew-seconds</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub const MAX_CLOCK_SKEW_SECONDS: u64</code> |

## visa-merchant-credential-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-merchant-credential-status</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub enum MerchantCredentialStatus</code> |

## visa-merchant-operation-result

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-merchant-operation-result</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub enum MerchantOperationResult</code> |

## visa-merchant-operation-result-from-receipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-merchant-operation-result-from-receipt</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn from_receipt( evidence: &amp;SequencerEvidence, receipt_keys: &amp;[SequencerAuthorization], payment_reference: [u8; 32], ) -&gt; Result&lt;Self, TapError&gt;</code> |

## visa-nonce-window

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-nonce-window</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub struct NonceWindow</code> |

## visa-nonce-window-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-nonce-window-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub const fn new() -&gt; Self</code> |

## visa-prepare-trusted-intent

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-prepare-trusted-intent</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn prepare_trusted_intent(principal: &amp;PrincipalId, layerx_agent: [u8; 32], verified: &amp;VerifiedTrustedAgent, store: &amp;mut impl CredentialBindingStore, trace: &amp;TraceId) -&gt; Result&lt;TrustedCommerceIntent, TapError&gt;</code> |

## visa-registered-agent-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-registered-agent-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub struct RegisteredAgentKey</code> |

## visa-signature-input

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-signature-input</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub struct SignatureInput</code> |

## visa-signature-input-created

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-signature-input-created</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub const fn created(&amp;self) -&gt; u64</code> |

## visa-signature-input-expires

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-signature-input-expires</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub const fn expires(&amp;self) -&gt; u64</code> |

## visa-signature-input-intent

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-signature-input-intent</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub const fn intent(&amp;self) -&gt; AgentIntent</code> |

## visa-signature-input-key-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-signature-input-key-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn key_id(&amp;self) -&gt; &amp;str</code> |

## visa-signature-input-nonce

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-signature-input-nonce</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn nonce(&amp;self) -&gt; &amp;str</code> |

## visa-signature-input-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-signature-input-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn parse(header: &amp;str) -&gt; Result&lt;Self, TapError&gt;</code> |

## visa-tap-algorithm

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-tap-algorithm</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub enum TapAlgorithm</code> |

## visa-tap-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-tap-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub enum TapError</code> |

## visa-tap-error-merchant-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-tap-error-merchant-status</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub const fn merchant_status(self) -&gt; MerchantCredentialStatus</code> |

## visa-tap-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-tap-request</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub struct TapRequest</code> |

## visa-tap-request-authority

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-tap-request-authority</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn authority(&amp;self) -&gt; &amp;str</code> |

## visa-tap-request-nonce

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-tap-request-nonce</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn nonce(&amp;self) -&gt; &amp;str</code> |

## visa-tap-request-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-tap-request-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn parse(authority: impl Into&lt;String&gt;, path: impl Into&lt;String&gt;, signature_input: &amp;str, signature: &amp;str) -&gt; Result&lt;Self, TapError&gt;</code> |

## visa-tap-request-path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-tap-request-path</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn path(&amp;self) -&gt; &amp;str</code> |

## visa-tap-verifier

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-tap-verifier</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub struct TapVerifier</code> |

## visa-tap-verifier-verify

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-tap-verifier-verify</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn verify(request: &amp;TapRequest, registry: &amp;impl TrustedAgentRegistry, nonces: &amp;mut NonceWindow, now: u64) -&gt; Result&lt;VerifiedTrustedAgent, TapError&gt;</code> |

## visa-tap-verifier-verify-credential

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-tap-verifier-verify-credential</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn verify_credential(request: &amp;TapRequest, registry: &amp;impl TrustedAgentRegistry, now: u64, clock_skew_seconds: u64) -&gt; Result&lt;VerifiedTrustedAgent, TapError&gt;</code> |

## visa-trusted-agent-registry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-trusted-agent-registry</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub trait TrustedAgentRegistry</code> |

## visa-trusted-commerce-intent

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-trusted-commerce-intent</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub struct TrustedCommerceIntent</code> |

## visa-verified-agent-binding

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-verified-agent-binding</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn verified_agent_binding(layerx_agent: [u8; 32], verified: &amp;VerifiedTrustedAgent) -&gt; Result&lt;CredentialBinding, TapError&gt;</code> |

## visa-verified-trusted-agent

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-verified-trusted-agent</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub struct VerifiedTrustedAgent</code> |

## visa-visa-tap-adapter-descriptor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-visa-tap-adapter-descriptor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub fn visa_tap_adapter_descriptor(spec: PinnedSpec, conformance: ConformanceSuite) -&gt; Result&lt;AdapterDescriptor, TapError&gt;</code> |

## visa-visa-tap-spec-commit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-visa-tap-spec-commit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub const VISA_TAP_SPEC_COMMIT: &amp;str</code> |

## visa-visa-tap-spec-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-visa-visa-tap-spec-sha256</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-visa-tap/src/lib.rs](../../../services/interop/crates/layerx-visa-tap/src/lib.rs) | <code>pub const VISA_TAP_SPEC_SHA256: [u8; 32]</code> |
