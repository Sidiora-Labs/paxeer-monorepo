# Interop fiat contract

The [fiat adapter](../../../services/interop/crates/layerx-fiat/src/lib.rs) validates provider evidence, binds it to a token reference and applies verified outcomes through the payment plane.

## fiat-evidence-class

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-evidence-class</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub enum EvidenceClass</code> |

## fiat-executed-fiat-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-executed-fiat-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub struct ExecutedFiatOutcome</code> |

## fiat-external-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-external-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub struct ExternalId(String)</code> |

## fiat-external-id-as-str

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-external-id-as-str</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

## fiat-external-id-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-external-id-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub fn new(value: impl Into&lt;String&gt;) -&gt; Result&lt;Self, FiatError&gt;</code> |

## fiat-fiat-adapter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub struct FiatAdapter</code> |

## fiat-fiat-adapter-apply

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter-apply</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub fn apply(gateway: &amp;mut GatewayCore, principal: &amp;PrincipalId, token: &amp;TokenReference, evidence: &amp;ProviderEvidence, verifier: &amp;impl ProviderVerifier, plane: &amp;mut impl FiatPlane, trace: &amp;TraceId, now: u64) -&gt; Result&lt;FiatJourneyState, Traced&lt;FiatError&gt;&gt;</code> |

## fiat-fiat-adapter-descriptor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter-descriptor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub fn fiat_adapter_descriptor(spec: PinnedSpec, conformance: ConformanceSuite) -&gt; Result&lt;AdapterDescriptor, FiatError&gt;</code> |

## fiat-fiat-adapter-idempotency-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter-idempotency-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub fn idempotency_key(facts: &amp;VerifiedProviderFacts) -&gt; [u8; 32]</code> |

## fiat-fiat-adapter-verify-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-adapter-verify-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub fn verify_evidence(token: &amp;TokenReference, evidence: &amp;ProviderEvidence, verifier: &amp;impl ProviderVerifier, trace: &amp;TraceId) -&gt; Result&lt;VerifiedProviderFacts, Traced&lt;FiatError&gt;&gt;</code> |

## fiat-fiat-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub enum FiatError</code> |

## fiat-fiat-intent

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-intent</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub struct FiatIntent</code> |

## fiat-fiat-intent-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-intent-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub enum FiatIntentKind</code> |

## fiat-fiat-journey-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-journey-state</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub enum FiatJourneyState</code> |

## fiat-fiat-plane

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-plane</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub trait FiatPlane</code> |

## fiat-fiat-plane-result

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-plane-result</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub enum FiatPlaneResult</code> |

## fiat-fiat-rail

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-fiat-rail</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub enum FiatRail</code> |

## fiat-interop-fiat-adapters

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-interop-fiat-adapters</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub const fn interop_fiat_adapters() -&gt; &amp;'static str</code> |

## fiat-plane-fiat-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-plane-fiat-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub enum PlaneFiatOutcome</code> |

## fiat-provider-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-provider-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub struct ProviderEvidence</code> |

## fiat-provider-evidence-canonical

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-provider-evidence-canonical</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub fn canonical(&amp;self) -&gt; &amp;[u8]</code> |

## fiat-provider-evidence-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-provider-evidence-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub const fn digest(&amp;self) -&gt; [u8; 32]</code> |

## fiat-provider-evidence-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-provider-evidence-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub fn new(canonical: Vec&lt;u8&gt;) -&gt; Result&lt;Self, FiatError&gt;</code> |

## fiat-provider-verifier

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-provider-verifier</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub trait ProviderVerifier</code> |

## fiat-token-reference

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-token-reference</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub struct TokenReference(Zeroizing&lt;Vec&lt;u8&gt;&gt;)</code> |

## fiat-token-reference-as-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-token-reference-as-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub fn as_bytes(&amp;self) -&gt; &amp;[u8]</code> |

## fiat-token-reference-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-token-reference-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub fn new(value: Vec&lt;u8&gt;) -&gt; Result&lt;Self, FiatError&gt;</code> |

## fiat-verified-provider-facts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-fiat-verified-provider-facts</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-fiat/src/lib.rs](../../../services/interop/crates/layerx-fiat/src/lib.rs) | <code>pub struct VerifiedProviderFacts</code> |
