# Interop fiat contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## fiat-evidence-class

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-evidence-class</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:44</code> | <code>pub enum EvidenceClass</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-evidence-class</code> | notes | enum layerx_fiat::EvidenceClass |

## fiat-executed-fiat-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-executed-fiat-outcome</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:264</code> | <code>pub struct ExecutedFiatOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-executed-fiat-outcome</code> | notes | struct layerx_fiat::ExecutedFiatOutcome; spec/layerx-merge/spec.kvx task 7.5.21 uses GatewayCore::settle_with_sequencer_receipt and extends ExecutedFiatOutcome (services/interop/crates/layerx-fiat/src/lib.rs:264) with signed receipt bytes and logs |

## fiat-external-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-external-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:66</code> | <code>pub struct ExternalId(String)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-external-id</code> | notes | struct layerx_fiat::ExternalId |

## fiat-external-id-as-str

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-external-id-as-str</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:86</code> | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-external-id-as-str</code> | notes | method layerx_fiat::ExternalId::as_str; no reference outside the defining file found by name match |

## fiat-external-id-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-external-id-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:74</code> | <code>pub fn new(value: impl Into&lt;String&gt;) -&gt; Result&lt;Self, FiatError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-external-id-new</code> | notes | method layerx_fiat::ExternalId::new |

## fiat-fiat-adapter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:306</code> | <code>pub struct FiatAdapter</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter</code> | notes | struct layerx_fiat::FiatAdapter |

## fiat-fiat-adapter-apply

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter-apply</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:344</code> | <code>pub fn apply(gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, token: &amp;TokenReference, evidence: &amp;ProviderEvidence, verifier: &amp;impl ProviderVerifier, plane: &amp;mut impl FiatPlane, trace: &amp;TraceId, now: u64) -&gt; Result&lt;FiatJourneyState, Traced&lt;FiatError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter-apply</code> | notes | method layerx_fiat::FiatAdapter::apply; spec/layerx-merge/spec.kvx task 7.5.21 uses GatewayCore::settle_with_sequencer_receipt and extends ExecutedFiatOutcome (services/interop/crates/layerx-fiat/src/lib.rs:264) with signed receipt bytes and logs |

## fiat-fiat-adapter-descriptor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter-descriptor</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:611</code> | <code>pub fn fiat_adapter_descriptor(spec: PinnedSpec, conformance: ConformanceSuite) -&gt; Result&lt;AdapterDescriptor, FiatError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter-descriptor</code> | notes | fn layerx_fiat::fiat_adapter_descriptor |

## fiat-fiat-adapter-idempotency-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter-idempotency-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:313</code> | <code>pub fn idempotency_key(facts: &amp;VerifiedProviderFacts) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter-idempotency-key</code> | notes | method layerx_fiat::FiatAdapter::idempotency_key |

## fiat-fiat-adapter-verify-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter-verify-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:323</code> | <code>pub fn verify_evidence(token: &amp;TokenReference, evidence: &amp;ProviderEvidence, verifier: &amp;impl ProviderVerifier, trace: &amp;TraceId) -&gt; Result&lt;VerifiedProviderFacts, Traced&lt;FiatError&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter-verify-evidence</code> | notes | method layerx_fiat::FiatAdapter::verify_evidence |

## fiat-fiat-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:488</code> | <code>pub enum FiatError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-fiat-error</code> | notes | enum layerx_fiat::FiatError |

## fiat-fiat-intent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-intent</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:246</code> | <code>pub struct FiatIntent</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-fiat-intent</code> | notes | struct layerx_fiat::FiatIntent |

## fiat-fiat-intent-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-intent-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:239</code> | <code>pub enum FiatIntentKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-fiat-intent-kind</code> | notes | enum layerx_fiat::FiatIntentKind; no reference outside the defining file found by name match |

## fiat-fiat-journey-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-journey-state</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:293</code> | <code>pub enum FiatJourneyState</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-fiat-journey-state</code> | notes | enum layerx_fiat::FiatJourneyState |

## fiat-fiat-plane

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-plane</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:277</code> | <code>pub trait FiatPlane</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-fiat-plane</code> | notes | trait layerx_fiat::FiatPlane; spec/layerx-merge/spec.kvx task 7.5.21 uses GatewayCore::settle_with_sequencer_receipt and extends ExecutedFiatOutcome (services/interop/crates/layerx-fiat/src/lib.rs:264) with signed receipt bytes and logs |

## fiat-fiat-plane-result

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-plane-result</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:270</code> | <code>pub enum FiatPlaneResult</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-fiat-plane-result</code> | notes | enum layerx_fiat::FiatPlaneResult |

## fiat-fiat-rail

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-rail</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:25</code> | <code>pub enum FiatRail</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-fiat-rail</code> | notes | enum layerx_fiat::FiatRail |

## fiat-interop-fiat-adapters

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-interop-fiat-adapters</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:620</code> | <code>pub const fn interop_fiat_adapters() -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-interop-fiat-adapters</code> | notes | fn layerx_fiat::interop_fiat_adapters; no reference outside the defining file found by name match |

## fiat-plane-fiat-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-plane-fiat-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:258</code> | <code>pub enum PlaneFiatOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-plane-fiat-outcome</code> | notes | enum layerx_fiat::PlaneFiatOutcome |

## fiat-provider-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-provider-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:127</code> | <code>pub struct ProviderEvidence</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-provider-evidence</code> | notes | struct layerx_fiat::ProviderEvidence |

## fiat-provider-evidence-canonical

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-provider-evidence-canonical</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:150</code> | <code>pub fn canonical(&amp;self) -&gt; &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-provider-evidence-canonical</code> | notes | method layerx_fiat::ProviderEvidence::canonical |

## fiat-provider-evidence-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-provider-evidence-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:155</code> | <code>pub const fn digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-provider-evidence-digest</code> | notes | method layerx_fiat::ProviderEvidence::digest; no reference outside the defining file found by name match |

## fiat-provider-evidence-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-provider-evidence-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:138</code> | <code>pub fn new(canonical: Vec&lt;u8&gt;) -&gt; Result&lt;Self, FiatError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-provider-evidence-new</code> | notes | method layerx_fiat::ProviderEvidence::new |

## fiat-provider-verifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-provider-verifier</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:224</code> | <code>pub trait ProviderVerifier</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-provider-verifier</code> | notes | trait layerx_fiat::ProviderVerifier |

## fiat-sequencer-settlement

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.21.1</code> | req | 11021.1 |
| <code>acceptance.11.21.1</code> | scenario | Given a card journey whose capture plane returns the recorded receipted evidence, when apply runs, then the journey state is settled with the recorded receiptDigest. |
| <code>acceptance.11.21.1</code> | test | services/interop/crates/layerx-fiat/tests/sequencer_settlement.rs::fiat_card_capture_settles_receipted_evidence (planned) |
| <code>acceptance.11.21.1</code> | qualification | UNRUN |
| <code>req.11021</code> | ac_1 | A card capture settles the recorded receipt of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:109 and advances the journey (planned) (services/interop/crates/layerx-fiat/src/lib.rs:293). |
| <code>acceptance.11.21.2</code> | req | 11021.2 |
| <code>acceptance.11.21.2</code> | scenario | Given the same journey and a plane returning a submitted rung, when apply runs, then the journey state is unchanged and the answer is pending. |
| <code>acceptance.11.21.2</code> | test | services/interop/crates/layerx-fiat/tests/sequencer_settlement.rs::fiat_submitted_rung_keeps_the_journey_waiting (planned) |
| <code>acceptance.11.21.2</code> | qualification | UNRUN |
| <code>req.11021</code> | ac_2 | A submitted rung leaves the fiat journey state unchanged (planned) (services/interop/crates/layerx-fiat/src/lib.rs:344). |
| <code>acceptance.11.32.1</code> | req | 11032.1 |
| <code>acceptance.11.32.1</code> | scenario | Given a settled card journey and a reversal whose evidence repeats the capture txHash, when apply runs, then it returns the reversal refusal and the journey stays settled. |
| <code>acceptance.11.32.1</code> | test | services/interop/crates/layerx-fiat/tests/sequencer_settlement.rs::fiat_reversal_on_the_capture_receipt_is_refused (planned) |
| <code>acceptance.11.32.1</code> | qualification | UNRUN |
| <code>req.11032</code> | ac_1 | A reversal settled on the capture's own transaction hash is refused (planned) (services/interop/crates/layerx-fiat/src/lib.rs:456). |
| <code>acceptance.11.58.1</code> | req | 11058.1 |
| <code>acceptance.11.58.1</code> | scenario | Given the rewritten fixture, when a card journey is applied, then it settles with the recorded receiptDigest. |
| <code>acceptance.11.58.1</code> | test | services/interop/crates/layerx-fiat/tests/adapter.rs::fiat_adapter_suite_settles_on_the_recorded_receipt (planned) |
| <code>acceptance.11.58.1</code> | qualification | UNRUN |
| <code>req.11058</code> | ac_1 | The fiat adapter suite settles a card journey on the recorded receipt (planned) (services/interop/crates/layerx-fiat/tests/adapter.rs:485). |
| <code>capability.11.8.1</code> | title | Settle the fiat plane on SequencerEvidence |
| <code>capability.11.8.1</code> | reqs | 11021.1<br>11021.2 |
| <code>capability.11.8.2</code> | title | Settle fiat reversal and chargeback legs on their own sequencer receipts |
| <code>capability.11.8.2</code> | reqs | 11032.1 |
| <code>capability.11.8.3</code> | title | Move the fiat adapter tests off AuthorizedBatch |
| <code>capability.11.8.3</code> | reqs | 11058.1 |

## fiat-token-reference

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-token-reference</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:93</code> | <code>pub struct TokenReference(Zeroizing&lt;Vec&lt;u8&gt;&gt;)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-token-reference</code> | notes | struct layerx_fiat::TokenReference |

## fiat-token-reference-as-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-token-reference-as-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:114</code> | <code>pub fn as_bytes(&amp;self) -&gt; &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-token-reference-as-bytes</code> | notes | method layerx_fiat::TokenReference::as_bytes; no reference outside the defining file found by name match |

## fiat-token-reference-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-token-reference-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:101</code> | <code>pub fn new(value: Vec&lt;u8&gt;) -&gt; Result&lt;Self, FiatError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-token-reference-new</code> | notes | method layerx_fiat::TokenReference::new |

## fiat-verified-provider-facts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-fiat-verified-provider-facts</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-fiat/src/lib.rs:174</code> | <code>pub struct VerifiedProviderFacts</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-fiat-verified-provider-facts</code> | notes | struct layerx_fiat::VerifiedProviderFacts |
