# Interop visa tap contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## visa-agent-intent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-agent-intent</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:56</code> | <code>pub enum AgentIntent</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-agent-intent</code> | notes | enum layerx_visa_tap::AgentIntent |

## visa-agent-public-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-agent-public-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:303</code> | <code>pub enum AgentPublicKey</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-agent-public-key</code> | notes | enum layerx_visa_tap::AgentPublicKey |

## visa-bind-verified-agent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-bind-verified-agent</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:758</code> | <code>pub fn bind_verified_agent(principal: &amp;PrincipalId, layerx_agent: [u8; 32], verified: &amp;VerifiedTrustedAgent, store: &amp;mut impl CredentialBindingStore, trace: &amp;TraceId) -&gt; Result&lt;CredentialBinding, TapError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-bind-verified-agent</code> | notes | fn layerx_visa_tap::bind_verified_agent |

## visa-canonical-tap-authority

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-canonical-tap-authority</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:912</code> | <code>pub fn canonical_tap_authority(value: &amp;str) -&gt; Result&lt;String, TapError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-canonical-tap-authority</code> | notes | fn layerx_visa_tap::canonical_tap_authority |

## visa-canonical-tap-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-canonical-tap-path</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:965</code> | <code>pub fn canonical_tap_path(value: &amp;str) -&gt; Result&lt;String, TapError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-canonical-tap-path</code> | notes | fn layerx_visa_tap::canonical_tap_path |

## visa-credential-binding

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-credential-binding</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:729</code> | <code>pub struct CredentialBinding</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-credential-binding</code> | notes | struct layerx_visa_tap::CredentialBinding |

## visa-credential-binding-store

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-credential-binding-store</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:738</code> | <code>pub trait CredentialBindingStore</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-credential-binding-store</code> | notes | trait layerx_visa_tap::CredentialBindingStore |

## visa-interop-visa-tap

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-interop-visa-tap</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:1088</code> | <code>pub const fn interop_visa_tap() -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-interop-visa-tap</code> | notes | fn layerx_visa_tap::interop_visa_tap; no reference outside the defining file found by name match |

## visa-key-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-key-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:296</code> | <code>pub enum KeyStatus</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-key-status</code> | notes | enum layerx_visa_tap::KeyStatus |

## visa-layerx-intent-authority

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-layerx-intent-authority</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:423</code> | <code>pub trait LayerXIntentAuthority</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-layerx-intent-authority</code> | notes | trait layerx_visa_tap::LayerXIntentAuthority |

## visa-max-clock-skew-seconds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-max-clock-skew-seconds</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:28</code> | <code>pub const MAX_CLOCK_SKEW_SECONDS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-max-clock-skew-seconds</code> | notes | const layerx_visa_tap::MAX_CLOCK_SKEW_SECONDS; chain calls: 0x101a at :36 |

## visa-merchant-credential-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-merchant-credential-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:826</code> | <code>pub enum MerchantCredentialStatus</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-merchant-credential-status</code> | notes | enum layerx_visa_tap::MerchantCredentialStatus |

## visa-merchant-operation-result

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-merchant-operation-result</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:441</code> | <code>pub enum MerchantOperationResult</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-merchant-operation-result</code> | notes | enum layerx_visa_tap::MerchantOperationResult |

## visa-merchant-operation-result-from-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-merchant-operation-result-from-receipt</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:455</code> | <code>pub fn from_receipt(canonical_receipt: &amp;[u8], receipt_keys: &amp;[SequencerAuthorization], payment_reference: [u8; 32], from: [u8; 32], to: [u8; 32], asset: [u8; 32], amount: u128) -&gt; Result&lt;Self, TapError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-merchant-operation-result-from-receipt</code> | notes | method layerx_visa_tap::MerchantOperationResult::from_receipt |
| <code>interface.11.rs-visa-merchant-operation-result-from-receipt</code> | overridden_by | override.J47.11 |

## visa-nonce-window

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-nonce-window</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:365</code> | <code>pub struct NonceWindow</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-nonce-window</code> | notes | struct layerx_visa_tap::NonceWindow |

## visa-nonce-window-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-nonce-window-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:371</code> | <code>pub const fn new() -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-nonce-window-new</code> | notes | method layerx_visa_tap::NonceWindow::new |

## visa-prepare-trusted-intent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-prepare-trusted-intent</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:807</code> | <code>pub fn prepare_trusted_intent(principal: &amp;PrincipalId, layerx_agent: [u8; 32], verified: &amp;VerifiedTrustedAgent, store: &amp;mut impl CredentialBindingStore, trace: &amp;TraceId) -&gt; Result&lt;TrustedCommerceIntent, TapError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-prepare-trusted-intent</code> | notes | fn layerx_visa_tap::prepare_trusted_intent |

## visa-registered-agent-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-registered-agent-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:319</code> | <code>pub struct RegisteredAgentKey</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-registered-agent-key</code> | notes | struct layerx_visa_tap::RegisteredAgentKey |

## visa-signature-input

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-signature-input</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:92</code> | <code>pub struct SignatureInput</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-signature-input</code> | notes | struct layerx_visa_tap::SignatureInput; no reference outside the defining file found by name match |

## visa-signature-input-created

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-signature-input-created</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:191</code> | <code>pub const fn created(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-signature-input-created</code> | notes | method layerx_visa_tap::SignatureInput::created; no reference outside the defining file found by name match |

## visa-signature-input-expires

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-signature-input-expires</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:196</code> | <code>pub const fn expires(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-signature-input-expires</code> | notes | method layerx_visa_tap::SignatureInput::expires; no reference outside the defining file found by name match |

## visa-signature-input-intent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-signature-input-intent</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:211</code> | <code>pub const fn intent(&amp;self) -&gt; AgentIntent</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-signature-input-intent</code> | notes | method layerx_visa_tap::SignatureInput::intent; no reference outside the defining file found by name match |

## visa-signature-input-key-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-signature-input-key-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:201</code> | <code>pub fn key_id(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-signature-input-key-id</code> | notes | method layerx_visa_tap::SignatureInput::key_id; no reference outside the defining file found by name match |

## visa-signature-input-nonce

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-signature-input-nonce</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:206</code> | <code>pub fn nonce(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-signature-input-nonce</code> | notes | method layerx_visa_tap::SignatureInput::nonce; no reference outside the defining file found by name match |

## visa-signature-input-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-signature-input-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:108</code> | <code>pub fn parse(header: &amp;str) -&gt; Result&lt;Self, TapError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-signature-input-parse</code> | notes | method layerx_visa_tap::SignatureInput::parse; no reference outside the defining file found by name match |

## visa-tap-algorithm

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-tap-algorithm</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:74</code> | <code>pub enum TapAlgorithm</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-tap-algorithm</code> | notes | enum layerx_visa_tap::TapAlgorithm; no reference outside the defining file found by name match |

## visa-tap-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-tap-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:1017</code> | <code>pub enum TapError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-tap-error</code> | notes | enum layerx_visa_tap::TapError; 81 reference lines in 3 files; callers lists the first reference in each file |

## visa-tap-error-merchant-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-tap-error-merchant-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:837</code> | <code>pub const fn merchant_status(self) -&gt; MerchantCredentialStatus</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-tap-error-merchant-status</code> | notes | method layerx_visa_tap::TapError::merchant_status |

## visa-tap-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-tap-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:218</code> | <code>pub struct TapRequest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-tap-request</code> | notes | struct layerx_visa_tap::TapRequest |

## visa-tap-request-authority

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-tap-request-authority</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:283</code> | <code>pub fn authority(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-tap-request-authority</code> | notes | method layerx_visa_tap::TapRequest::authority |

## visa-tap-request-nonce

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-tap-request-nonce</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:277</code> | <code>pub fn nonce(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-tap-request-nonce</code> | notes | method layerx_visa_tap::TapRequest::nonce |

## visa-tap-request-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-tap-request-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:232</code> | <code>pub fn parse(authority: impl Into&lt;String&gt;, path: impl Into&lt;String&gt;, signature_input: &amp;str, signature: &amp;str) -&gt; Result&lt;Self, TapError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-tap-request-parse</code> | notes | method layerx_visa_tap::TapRequest::parse |

## visa-tap-request-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-tap-request-path</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:289</code> | <code>pub fn path(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-tap-request-path</code> | notes | method layerx_visa_tap::TapRequest::path; no reference outside the defining file found by name match |

## visa-tap-sequencer-settlement

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.23.1</code> | req | 11023.1 |
| <code>acceptance.11.23.1</code> | scenario | Given the recorded receipted evidence and its reference, when from_receipt runs, then the result carries the recorded payer, recipient, asset and amount 1000000. |
| <code>acceptance.11.23.1</code> | test | services/interop/crates/layerx-visa-tap/tests/sequencer_result.rs::from_receipt_reads_the_pay_exact_log (planned) |
| <code>acceptance.11.23.1</code> | qualification | UNRUN |
| <code>req.11023</code> | ac_1 | from_receipt reads payer, recipient, asset and amount from the recorded PaymentExact log of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:109 (planned). |
| <code>acceptance.11.23.2</code> | req | 11023.2 |
| <code>acceptance.11.23.2</code> | scenario | Given the recorded evidence and a different payment_reference, when from_receipt runs, then it returns the reference mismatch error. |
| <code>acceptance.11.23.2</code> | test | services/interop/crates/layerx-visa-tap/tests/sequencer_result.rs::from_receipt_refuses_another_reference (planned) |
| <code>acceptance.11.23.2</code> | qualification | UNRUN |
| <code>req.11023</code> | ac_2 | A PaymentExact log with another reference is refused (planned) (services/interop/crates/layerx-visa-tap/src/lib.rs:455). |
| <code>acceptance.11.34.1</code> | req | 11034.1 |
| <code>acceptance.11.34.1</code> | scenario | Given the recorded receipted evidence, when from_receipt runs, then the result digest is the recorded receiptDigest. |
| <code>acceptance.11.34.1</code> | test | services/interop/crates/layerx-visa-tap/tests/sequencer_result.rs::visa_result_digest_is_the_recorded_receipt_digest (planned) |
| <code>acceptance.11.34.1</code> | qualification | UNRUN |
| <code>req.11034</code> | ac_1 | The Visa operation digest equals receiptDigest at sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:150 (planned). |
| <code>acceptance.11.60.1</code> | req | 11060.1 |
| <code>acceptance.11.60.1</code> | scenario | Given the rewritten fixture, when the middleware executes an intent, then the result carries the recorded amount and digest. |
| <code>acceptance.11.60.1</code> | test | services/interop/crates/layerx-visa-tap/tests/middleware.rs::middleware_operation_result_uses_the_recorded_receipt (planned) |
| <code>acceptance.11.60.1</code> | qualification | UNRUN |
| <code>req.11060</code> | ac_1 | The middleware suite builds its operation result from the recorded receipt (planned) (services/interop/crates/layerx-visa-tap/src/lib.rs:455). |
| <code>capability.11.17.1</code> | title | Build the Visa TAP operation result from SequencerEvidence and verified logs |
| <code>capability.11.17.1</code> | reqs | 11023.1<br>11023.2 |
| <code>capability.11.17.2</code> | title | Use the gateway canonical receipt digest in Visa TAP instead of the private copy |
| <code>capability.11.17.2</code> | reqs | 11034.1 |
| <code>capability.11.17.3</code> | title | Move the Visa TAP middleware tests onto SequencerEvidence |
| <code>capability.11.17.3</code> | reqs | 11060.1 |

## visa-tap-verifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-tap-verifier</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:608</code> | <code>pub struct TapVerifier</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-tap-verifier</code> | notes | struct layerx_visa_tap::TapVerifier |

## visa-tap-verifier-verify

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-tap-verifier-verify</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:652</code> | <code>pub fn verify(request: &amp;TapRequest, registry: &amp;impl TrustedAgentRegistry, nonces: &amp;mut NonceWindow, now: u64) -&gt; Result&lt;VerifiedTrustedAgent, TapError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-tap-verifier-verify</code> | notes | method layerx_visa_tap::TapVerifier::verify |

## visa-tap-verifier-verify-credential

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-tap-verifier-verify-credential</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:621</code> | <code>pub fn verify_credential(request: &amp;TapRequest, registry: &amp;impl TrustedAgentRegistry, now: u64, clock_skew_seconds: u64) -&gt; Result&lt;VerifiedTrustedAgent, TapError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-tap-verifier-verify-credential</code> | notes | method layerx_visa_tap::TapVerifier::verify_credential |

## visa-trusted-agent-registry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-trusted-agent-registry</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:354</code> | <code>pub trait TrustedAgentRegistry</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-trusted-agent-registry</code> | notes | trait layerx_visa_tap::TrustedAgentRegistry |

## visa-trusted-commerce-intent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-trusted-commerce-intent</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:412</code> | <code>pub struct TrustedCommerceIntent</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-trusted-commerce-intent</code> | notes | struct layerx_visa_tap::TrustedCommerceIntent |

## visa-verified-agent-binding

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-verified-agent-binding</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:773</code> | <code>pub fn verified_agent_binding(layerx_agent: [u8; 32], verified: &amp;VerifiedTrustedAgent) -&gt; Result&lt;CredentialBinding, TapError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-verified-agent-binding</code> | notes | fn layerx_visa_tap::verified_agent_binding |

## visa-verified-trusted-agent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-verified-trusted-agent</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:397</code> | <code>pub struct VerifiedTrustedAgent</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-verified-trusted-agent</code> | notes | struct layerx_visa_tap::VerifiedTrustedAgent |

## visa-visa-tap-adapter-descriptor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-visa-tap-adapter-descriptor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:1006</code> | <code>pub fn visa_tap_adapter_descriptor(spec: PinnedSpec, conformance: ConformanceSuite) -&gt; Result&lt;AdapterDescriptor, TapError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-visa-tap-adapter-descriptor</code> | notes | fn layerx_visa_tap::visa_tap_adapter_descriptor |

## visa-visa-tap-spec-commit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-visa-tap-spec-commit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:46</code> | <code>pub const VISA_TAP_SPEC_COMMIT: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-visa-tap-spec-commit</code> | notes | const layerx_visa_tap::VISA_TAP_SPEC_COMMIT |

## visa-visa-tap-spec-sha256

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-visa-visa-tap-spec-sha256</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-visa-tap/src/lib.rs:48</code> | <code>pub const VISA_TAP_SPEC_SHA256: [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-visa-visa-tap-spec-sha256</code> | notes | const layerx_visa_tap::VISA_TAP_SPEC_SHA256 |
