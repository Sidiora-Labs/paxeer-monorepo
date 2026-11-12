#[path = "support/source_receipt.rs"]
pub mod source_receipt;

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use layerx_ap2::{
    ap2_adapter_descriptor, AdapterOutcome, Ap2Adapter, Ap2Error, AuthorizedPayment, KeyResolver,
    KeyUse, LayerXAssetBinding, LayerXIntentPlane, MandateMode, MandateVerifier, Merchant,
    PlaneOutcome, ProtectedHeader, ReceiptSigner, VerificationContext,
};
use layerx_intents::{compile, CompiledIntent, Intent, IntentKind, LxpSend};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite};
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
use layerx_interop_gateway::gateway::TranslationStatus;
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::SequencerLog;
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, NetworkId, ProtocolVersion, PublicKey, SendAuthorization,
    SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use p256::ecdsa::signature::{Signer as _, Verifier as _};
use p256::ecdsa::{Signature, SigningKey, VerifyingKey};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const NOW: u64 = 1_700_000_000;
const ISSUER_KID: &str = "issuer-001";
const MERCHANT_KID: &str = "merchant-001";
const ALTERNATE_ISSUER_KID: &str = "issuer-002";

type ExecutedPayment = layerx_ap2::ExecutedPayment;

fn recorded_hex(value: &Value) -> Vec<u8> {
    hex::decode(
        value
            .as_str()
            .or_panic("recorded hex string")
            .strip_prefix("0x")
            .or_panic("recorded hex prefix"),
    )
    .or_panic("recorded hex bytes")
}

fn recorded_array<const N: usize>(value: &Value) -> [u8; N] {
    recorded_hex(value)
        .try_into()
        .or_panic("recorded field width")
}

fn recorded_payment_topics(case: &Value) -> [[u8; 32]; 3] {
    let transaction = recorded_hex(&case["rawTransaction"]);
    let offsets: Vec<_> = transaction
        .windows(4)
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == [0x78, 0xea, 0x11, 0xdb]).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "one recorded payExact calldata selector");
    let start = offsets[0] + 4;
    [5, 0, 1].map(|word| {
        transaction[start + word * 32..start + (word + 1) * 32]
            .try_into()
            .or_panic("recorded payExact calldata word")
    })
}

fn recorded_payment(case: &Value) -> ExecutedPayment {
    ExecutedPayment {
        evidence: SequencerEvidence {
            transaction_hash: recorded_array(&case["receiptJson"]["txHash"]),
            signed_receipt: recorded_hex(&case["signedReceipt"]),
            logs: case["receiptJson"]["logs"]
                .as_array()
                .or_panic("recorded logs")
                .iter()
                .map(|log| {
                    SequencerLog::new(
                        recorded_array(&log["address"]),
                        log["topics"]
                            .as_array()
                            .or_panic("recorded topics")
                            .iter()
                            .map(recorded_array)
                            .collect(),
                        recorded_hex(&log["data"]),
                        u64::from_str_radix(
                            log["logIndex"]
                                .as_str()
                                .or_panic("recorded log index")
                                .strip_prefix("0x")
                                .or_panic("log index prefix"),
                            16,
                        )
                        .unwrap_or_else(|error| panic!("recorded log index quantity: {error:?}")),
                    )
                })
                .collect(),
            rung: SettlementRung::Receipted,
        },
        order_id: "order-12345".into(),
    }
}

use source_receipt::IntentKey;

fn assert_payment_reference(
    case: &Value,
    evidence: &SequencerEvidence,
    payment: &AuthorizedPayment,
) -> [u8; 32] {
    source_receipt::assert_mandate_reference(case, evidence, payment)
}

struct RecordedPaymentPlane {
    executed: ExecutedPayment,
    key: IntentKey,
}

impl LayerXIntentPlane for RecordedPaymentPlane {
    fn compile(
        &mut self,
        payment: &AuthorizedPayment,
        _: &TraceId,
    ) -> Result<CompiledIntent, Ap2Error> {
        let send = LxpSend::new(
            AccountId::parse("agent:buyer:main").or_panic("payer account"),
            AccountId::parse("agent:merchant:main").or_panic("payee account"),
            AssetId::new(payment.asset()),
            Amount::from_u128(payment.amount()),
            Sequence::from_u64(0),
            IdempotencyKey::new(payment.idempotency_key()),
            TimestampSeconds::from_u64(NOW + 1800),
            ContextHash::new(payment.request_digest()),
            SendAuthorization::new(
                SendAuthorizationKind::Owner,
                PublicKey::new(self.key.public_key),
                AuthorizationSignature::new(self.key.sign(&payment.request_digest())),
            ),
            NetworkId::new(125).or_panic("network"),
            ProtocolVersion::new(2).or_panic("protocol"),
        )
        .map_err(|_| Ap2Error::IntentCompilation)?;
        let activity = ActivityType::new(ModuleId::Asset, 5).or_panic("send activity");
        let registry =
            ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Asset, &[activity])
                .or_panic("asset registration")])
            .or_panic("module registry");
        compile(&Intent::v1(IntentKind::LxpSend(send)), &registry)
            .map_err(|_| Ap2Error::IntentCompilation)
    }

    fn execute(
        &mut self,
        payment: &AuthorizedPayment,
        compiled: CompiledIntent,
        _: &TraceId,
    ) -> Result<PlaneOutcome, Ap2Error> {
        assert_eq!(
            compiled.activity_type(),
            ActivityType::new(ModuleId::Asset, 5).or_panic("send")
        );
        let log = source_receipt::payment_log(&self.executed.evidence);
        assert_eq!(payment.idempotency_key(), log.topics()[1]);
        assert_eq!(payment.payer_receipt_account(), log.topics()[2]);
        assert_eq!(payment.payee_receipt_account(), log.topics()[3]);
        assert_eq!(payment.asset().as_slice(), &log.data()[..32]);
        assert_eq!(payment.amount().to_be_bytes().as_slice(), &log.data()[48..]);
        Ok(PlaneOutcome::Executed(Box::new(ExecutedPayment {
            evidence: self.executed.evidence.clone(),
            order_id: self.executed.order_id.clone(),
        })))
    }
}

struct MandateReceiptSigner(SigningKey);

impl ReceiptSigner for MandateReceiptSigner {
    fn issuer(&self) -> &'static str {
        "test-merchant-api"
    }

    fn key_id(&self) -> &'static str {
        "receipt-001"
    }

    fn verifying_key(&self) -> &VerifyingKey {
        self.0.verifying_key()
    }

    fn sign_es256(&mut self, input: &[u8]) -> Result<[u8; 64], Ap2Error> {
        let signature: Signature = self.0.sign(input);
        Ok(signature.to_bytes().into())
    }
}

struct TestKeyResolver {
    issuer: VerifyingKey,
    merchant: VerifyingKey,
}

impl TestKeyResolver {
    fn authentic() -> Self {
        Self {
            issuer: *issuer_key().verifying_key(),
            merchant: *merchant_key().verifying_key(),
        }
    }
}

impl KeyResolver for TestKeyResolver {
    fn resolve(&self, usage: KeyUse, header: &ProtectedHeader) -> Result<VerifyingKey, Ap2Error> {
        match (usage, header.key_id()) {
            (KeyUse::CheckoutMandateIssuer | KeyUse::PaymentMandateIssuer, Some(ISSUER_KID)) => {
                Ok(self.issuer)
            }
            (KeyUse::PaymentMandateIssuer, Some(ALTERNATE_ISSUER_KID)) => {
                Ok(*alternate_issuer_key().verifying_key())
            }
            (KeyUse::MerchantCheckout, Some(MERCHANT_KID)) => Ok(self.merchant),
            _ => Err(Ap2Error::KeyResolution),
        }
    }
}

fn issuer_key() -> SigningKey {
    SigningKey::from_bytes((&[7_u8; 32]).into()).or_panic("valid issuer scalar")
}
fn merchant_key() -> SigningKey {
    SigningKey::from_bytes((&[11_u8; 32]).into()).or_panic("valid merchant scalar")
}
fn agent_key() -> SigningKey {
    SigningKey::from_bytes((&[19_u8; 32]).into()).or_panic("valid agent scalar")
}
fn alternate_issuer_key() -> SigningKey {
    SigningKey::from_bytes((&[23_u8; 32]).into()).or_panic("valid alternate issuer scalar")
}

fn context() -> VerificationContext<'static> {
    VerificationContext {
        now: NOW,
        clock_skew_seconds: 300,
        expected_audience: "test-merchant-api",
        expected_nonce: "nonce-abc123xyz",
        currency_minor_exponent: 2,
        usage: None,
    }
}

fn vector(source: &str, expected_generator: &str) -> Value {
    let value: Value =
        serde_json::from_str(source).or_panic("golden vector specification is valid JSON");
    assert_eq!(value["generator"], expected_generator);
    assert!(!source.to_ascii_lowercase().contains("placeholder"));
    value
}

fn jws(header: &Value, payload: &Value, key: &SigningKey) -> String {
    let header = URL_SAFE_NO_PAD.encode(serde_json::to_vec(header).or_panic("serializable header"));
    let payload =
        URL_SAFE_NO_PAD.encode(serde_json::to_vec(payload).or_panic("serializable payload"));
    let signing_input = format!("{header}.{payload}");
    let signature: Signature = key.sign(signing_input.as_bytes());
    format!(
        "{signing_input}.{}",
        URL_SAFE_NO_PAD.encode(signature.to_bytes())
    )
}

fn root(mandate: Value) -> String {
    let mut payload = json!({"iat": NOW - 60, "exp": NOW + 3_600});
    payload["delegate_payload"] = Value::Array(vec![mandate]);
    format!(
        "{}~",
        jws(
            &json!({"alg":"ES256","typ":"dc+sd-jwt","kid":ISSUER_KID}),
            &payload,
            &issuer_key()
        )
    )
}

fn alternate_root(mandate: Value) -> String {
    let mut payload = json!({"iat": NOW - 60, "exp": NOW + 3_600});
    payload["delegate_payload"] = Value::Array(vec![mandate]);
    format!(
        "{}~",
        jws(
            &json!({"alg":"ES256","typ":"dc+sd-jwt","kid":ALTERNATE_ISSUER_KID}),
            &payload,
            &alternate_issuer_key()
        )
    )
}

fn agent_jwk() -> Value {
    let key = agent_key();
    let point = key.verifying_key().to_encoded_point(false);
    json!({"kty":"EC","crv":"P-256","x":URL_SAFE_NO_PAD.encode(point.x().or_panic("x coordinate")),"y":URL_SAFE_NO_PAD.encode(point.y().or_panic("y coordinate"))})
}

fn key_bound(open: &str, mandate: Value) -> String {
    let mut payload = json!({"iat":NOW-30,"exp":NOW+1_800,"aud":"test-merchant-api","nonce":"nonce-abc123xyz","sd_hash":URL_SAFE_NO_PAD.encode(Sha256::digest(open.as_bytes()))});
    payload["delegate_payload"] = Value::Array(vec![mandate]);
    format!(
        "{}~",
        jws(
            &json!({"alg":"ES256","typ":"kb+sd-jwt"}),
            &payload,
            &agent_key()
        )
    )
}

fn merchant_checkout(amount: u128, id: &str) -> String {
    jws(
        &json!({"alg":"ES256","typ":"JWT","kid":MERCHANT_KID}),
        &json!({
            "id":id,"merchant":{"id":"merchant-001","name":"Test Merchant"},
            "line_items":[{"id":"line-001","item":{"id":"sku-001","title":"LayerX node credit","price":amount},"quantity":1,"totals":[{"type":"total","amount":amount}]}],
            "status":"completed","currency":"USD","totals":[{"type":"total","amount":amount}],"links":[]
        }),
        &merchant_key(),
    )
}

fn closed_checkout(checkout_jwt: &str) -> Value {
    json!({"vct":"mandate.checkout.1","checkout_jwt":checkout_jwt,"checkout_hash":URL_SAFE_NO_PAD.encode(Sha256::digest(checkout_jwt.as_bytes())),"iat":NOW-30,"exp":NOW+1_800})
}

fn closed_payment(checkout_jwt: &str, amount: u128) -> Value {
    json!({"vct":"mandate.payment.1","transaction_id":URL_SAFE_NO_PAD.encode(Sha256::digest(checkout_jwt.as_bytes())),"payee":{"id":"merchant-001","name":"Test Merchant"},"payment_amount":{"amount":amount,"currency":"USD"},"payment_instrument":{"id":"card-001","type":"card"},"iat":NOW-30,"exp":NOW+1_800})
}

fn direct_pair(amount: u128) -> (String, String) {
    let checkout_jwt = merchant_checkout(amount, "checkout-12345");
    (
        root(closed_checkout(&checkout_jwt)),
        root(closed_payment(&checkout_jwt, amount)),
    )
}

fn autonomous_pair(amount: u128, maximum: u128) -> (String, String) {
    let open_checkout = root(
        json!({"vct":"mandate.checkout.open.1","constraints":[{"type":"checkout.line_items","items":[{"id":"requested-credit","acceptable_items":[{"id":"sku-001","title":"LayerX node credit"}],"quantity":1}]}],"cnf":{"jwk":agent_jwk()},"iat":NOW-60,"exp":NOW+3_600}),
    );
    let reference = URL_SAFE_NO_PAD.encode(Sha256::digest(open_checkout.as_bytes()));
    let open_payment = root(
        json!({"vct":"mandate.payment.open.1","constraints":[{"type":"payment.reference","conditional_transaction_id":reference},{"type":"payment.amount_range","currency":"USD","min":1,"max":maximum}],"cnf":{"jwk":agent_jwk()},"iat":NOW-60,"exp":NOW+3_600}),
    );
    let checkout_jwt = merchant_checkout(amount, "checkout-67890");
    let checkout = key_bound(&open_checkout, closed_checkout(&checkout_jwt));
    let payment = key_bound(&open_payment, closed_payment(&checkout_jwt, amount));
    (
        format!("{open_checkout}~~{checkout}"),
        format!("{open_payment}~~{payment}"),
    )
}

fn corrupt_signature(presentation: &str) -> String {
    let first = presentation.find('.').or_panic("header separator");
    let start = presentation[first + 1..]
        .find('.')
        .map_or_else(|| panic!("payload separator"), |next| first + next + 2);
    let mut bytes = presentation.as_bytes().to_vec();
    bytes[start] = if bytes[start] == b'A' { b'B' } else { b'A' };
    String::from_utf8(bytes).or_panic("base64url mutation is UTF-8")
}

#[test]
fn authentic_direct_vector_verifies_expected_values() {
    let spec = vector(
        include_str!("../../../specs/conformance/ap2/direct/001-minimal-valid.json"),
        "mandates::direct_pair",
    );
    let amount = u128::from(
        spec["expected_values"]["amount_minor_units"]
            .as_u64()
            .or_panic("minor-unit amount"),
    );
    let (checkout, payment) = direct_pair(amount);
    let resolver = TestKeyResolver::authentic();
    let verified = MandateVerifier::new(&resolver)
        .verify(&checkout, &payment, &context())
        .or_panic("authentic direct pair verifies");
    assert_eq!(verified.mode(), MandateMode::Direct);
    assert_eq!(
        verified.checkout_id(),
        spec["expected_values"]["checkout_id"]
            .as_str()
            .or_panic("checkout id")
    );
    assert_eq!(
        verified.payee().id(),
        spec["expected_values"]["payee_id"]
            .as_str()
            .or_panic("payee id")
    );
    assert_eq!(verified.amount().minor_units(), amount);
    assert_eq!(
        verified.amount().currency(),
        spec["expected_values"]["currency"]
            .as_str()
            .or_panic("currency")
    );
}

#[test]
fn authentic_autonomous_vector_verifies_key_binding_and_constraints() {
    let _spec = vector(
        include_str!("../../../specs/conformance/ap2/autonomous/001-line-items.json"),
        "mandates::autonomous_pair",
    );
    let (checkout, payment) = autonomous_pair(15_000, 20_000);
    let resolver = TestKeyResolver::authentic();
    let verified = MandateVerifier::new(&resolver)
        .verify(&checkout, &payment, &context())
        .or_panic("authentic autonomous pair verifies");
    assert_eq!(verified.mode(), MandateMode::Autonomous);
    assert_eq!(verified.checkout_id(), "checkout-67890");
    assert_eq!(verified.amount().minor_units(), 15_000);
}

#[test]
fn authentic_signature_corruption_is_refused_cryptographically() {
    let _spec = vector(
        include_str!("../../../specs/conformance/ap2/refusals/002-invalid-signature.json"),
        "mandates::direct_pair",
    );
    let (checkout, payment) = direct_pair(10_000);
    let resolver = TestKeyResolver::authentic();
    let result =
        MandateVerifier::new(&resolver).verify(&corrupt_signature(&checkout), &payment, &context());
    assert!(matches!(result, Err(Ap2Error::InvalidSignature)));
}

#[test]
fn authentic_payment_binding_corruption_is_refused_after_signature_verification() {
    let _spec = vector(
        include_str!("../../../specs/conformance/ap2/refusals/003-binding-mismatch.json"),
        "mandates::direct_pair",
    );
    let checkout_jwt = merchant_checkout(10_000, "checkout-12345");
    let checkout = root(closed_checkout(&checkout_jwt));
    let mut mandate = closed_payment(&checkout_jwt, 10_000);
    mandate["transaction_id"] = Value::String(URL_SAFE_NO_PAD.encode([0_u8; 32]));
    let resolver = TestKeyResolver::authentic();
    let result = MandateVerifier::new(&resolver).verify(&checkout, &root(mandate), &context());
    assert!(matches!(result, Err(Ap2Error::PaymentBindingMismatch)));
}

#[test]
fn authentic_expired_vector_is_refused_after_signature_verification() {
    let _spec = vector(
        include_str!("../../../specs/conformance/ap2/refusals/001-expired.json"),
        "mandates::root",
    );
    let checkout_jwt = merchant_checkout(10_000, "checkout-12345");
    let mut mandate = closed_checkout(&checkout_jwt);
    mandate["iat"] = json!(NOW - 7_200);
    mandate["exp"] = json!(NOW - 3_600);
    let resolver = TestKeyResolver::authentic();
    let result = MandateVerifier::new(&resolver).verify(
        &root(mandate),
        &root(closed_payment(&checkout_jwt, 10_000)),
        &context(),
    );
    assert!(matches!(result, Err(Ap2Error::Expired)));
}

#[test]
fn authentic_amount_violation_reaches_constraint_evaluation() {
    let spec = vector(
        include_str!("../../../specs/conformance/ap2/constraints/001-amount-range-exceeded.json"),
        "mandates::autonomous_pair",
    );
    let amount = u128::from(
        spec["authentic_amount_minor_units"]
            .as_u64()
            .or_panic("authentic amount"),
    );
    let maximum = u128::from(
        spec["constraint_max_minor_units"]
            .as_u64()
            .or_panic("constraint maximum"),
    );
    let (checkout, payment) = autonomous_pair(amount, maximum);
    let resolver = TestKeyResolver::authentic();
    let result = MandateVerifier::new(&resolver).verify(&checkout, &payment, &context());
    assert!(matches!(
        result,
        Err(Ap2Error::ConstraintViolated("payment.amount_range"))
    ));
}

#[test]
fn presentation_bounds_are_enforced_before_parsing() {
    let resolver = TestKeyResolver::authentic();
    let oversized = "a".repeat(300_000);
    let result = MandateVerifier::new(&resolver).verify(&oversized, &oversized, &context());
    assert!(matches!(result, Err(Ap2Error::Bounds)));
}

#[test]
fn empty_presentations_are_refused_at_the_bound() {
    let resolver = TestKeyResolver::authentic();
    let result = MandateVerifier::new(&resolver).verify("", "", &context());
    assert!(matches!(result, Err(Ap2Error::Bounds)));
}

#[test]
fn authentic_mismatched_issuers_are_refused() {
    let checkout_jwt = merchant_checkout(10_000, "checkout-12345");
    let checkout = root(closed_checkout(&checkout_jwt));
    let payment = alternate_root(closed_payment(&checkout_jwt, 10_000));
    let resolver = TestKeyResolver::authentic();
    let result = MandateVerifier::new(&resolver).verify(&checkout, &payment, &context());
    assert!(matches!(result, Err(Ap2Error::InvalidSignature)));
}

#[test]
fn authentic_not_yet_valid_mandate_is_refused() {
    let checkout_jwt = merchant_checkout(10_000, "checkout-12345");
    let mut mandate = closed_checkout(&checkout_jwt);
    mandate["iat"] = json!(NOW + 3_600);
    mandate["exp"] = json!(NOW + 7_200);
    let resolver = TestKeyResolver::authentic();
    let result = MandateVerifier::new(&resolver).verify(
        &root(mandate),
        &root(closed_payment(&checkout_jwt, 10_000)),
        &context(),
    );
    assert!(matches!(result, Err(Ap2Error::NotYetValid)));
}

#[test]
fn authentic_autonomous_audience_mismatch_is_refused() {
    let (checkout, payment) = autonomous_pair(15_000, 20_000);
    let resolver = TestKeyResolver::authentic();
    let mut wrong_context = context();
    wrong_context.expected_audience = "other-merchant-api";
    let result = MandateVerifier::new(&resolver).verify(&checkout, &payment, &wrong_context);
    assert!(matches!(result, Err(Ap2Error::AudienceMismatch)));
}

#[test]
fn signed_payment_amount_mismatch_is_refused() {
    let checkout_jwt = merchant_checkout(10_000, "checkout-12345");
    let resolver = TestKeyResolver::authentic();
    let result = MandateVerifier::new(&resolver).verify(
        &root(closed_checkout(&checkout_jwt)),
        &root(closed_payment(&checkout_jwt, 20_000)),
        &context(),
    );
    assert!(matches!(result, Err(Ap2Error::PaymentBindingMismatch)));
}

#[test]
fn unsupported_algorithm_is_refused_before_key_resolution() {
    let token = format!(
        "{}~",
        jws(
            &json!({"alg":"RS256","kid":ISSUER_KID}),
            &json!({"delegate_payload":[]}),
            &issuer_key()
        )
    );
    let resolver = TestKeyResolver::authentic();
    let result = MandateVerifier::new(&resolver).verify(&token, &token, &context());
    assert!(matches!(result, Err(Ap2Error::UnsupportedAlgorithm)));
}

#[test]
fn mandate_payment_reference_is_the_recorded_digest() {
    let fixture: Value = serde_json::from_str(include_str!("fixtures/ap2-mandate-pay-exact.json"))
        .or_panic("recorded sequencer receipt vectors");
    let case = fixture["cases"]
        .as_array()
        .or_panic("recorded cases")
        .iter()
        .find(|case| case["id"] == "mandates")
        .or_panic("recorded payExact receipt");
    let executed = recorded_payment(case);
    let pins = recorded_pins_and_digest(case, &executed);
    let log = source_receipt::payment_log(&executed.evidence);
    let amount = u128::from_be_bytes(log.data()[48..].try_into().or_panic("recorded amount"));
    let binding = LayerXAssetBinding {
        currency: "USD".into(),
        minor_unit_exponent: 2,
        atomic_units_per_minor_unit: 1,
        asset: log.data()[..32].try_into().or_panic("recorded asset"),
        payer_receipt_account: recorded_payment_topics(case)[1],
        payee_receipt_account: recorded_payment_topics(case)[2],
        payee_merchant: Merchant::new("merchant-001", "Test Merchant", None).or_panic("merchant"),
    };
    let (checkout, payment) = direct_pair(amount);
    let resolver = TestKeyResolver::authentic();
    let verification = context();
    let verified = MandateVerifier::new(&resolver)
        .verify(&checkout, &payment, &verification)
        .or_panic("authentic recorded-amount mandates");
    let principal = PrincipalId::new("buyer").or_panic("principal");
    let authorized = layerx_ap2::authorize_payment(&principal, &verified, &verification, &binding)
        .or_panic("authorized recorded payment");
    let digest = assert_payment_reference(case, &executed.evidence, &authorized);
    let receipt_key = IntentKey::new(case);
    assert_eq!(
        canonical_receipt_digest(&executed.evidence.signed_receipt),
        Ok(digest)
    );
    assert_eq!(digest, recorded_array::<32>(&case["receiptDigest"]));
    assert_receipt_protocol(
        &executed,
        &pins,
        "mandate receipt is signed by its height-pinned sequencer",
        "mandate protocol receipt",
    );
    let signed_receipt = executed.evidence.signed_receipt.clone();
    let trace = TraceId::mint([0x57; 16]);
    let suite = recorded_mandate_suite(&fixture);
    let mut gateway = GatewayCore::new();
    gateway
        .register_adapter(
            ap2_adapter_descriptor(suite).or_panic("AP2 descriptor"),
            &trace,
            NOW,
        )
        .or_panic("register AP2");
    let mut plane = RecordedPaymentPlane {
        executed,
        key: receipt_key,
    };
    let mut payment_signer = MandateReceiptSigner(issuer_key());
    let mut checkout_signer = MandateReceiptSigner(merchant_key());
    let outcome = Ap2Adapter::new(&resolver, &pins)
        .execute(
            &mut gateway,
            &principal,
            &checkout,
            &payment,
            &verification,
            &binding,
            &mut plane,
            &mut payment_signer,
            &mut checkout_signer,
            &trace,
            NOW,
        )
        .unwrap_or_else(|error| panic!("AP2 settles the recorded payExact receipt: {error:?}"));
    let AdapterOutcome::Settled(settled) = outcome else {
        panic!("expected recorded payment settlement: {outcome:?}");
    };
    assert_eq!(settled.receipt_digest, digest);
    assert_eq!(
        gateway.translation(&principal, authorized.idempotency_key()),
        Some(TranslationStatus::ReceiptVerified {
            receipt_digest: digest
        })
    );
    let portable = settled
        .layerx
        .verify_v2(&pins)
        .or_panic("portable receipt verifies");
    assert_eq!(portable.receipt_digest(), digest);
    assert_eq!(portable.signed_receipt(), signed_receipt);
    assert_payment_jwt(
        &settled.payment_receipt_jwt,
        &payment_signer,
        &verified.payment_receipt_reference(),
        digest,
    );
}

fn recorded_mandate_suite(fixture: &Value) -> ConformanceSuite {
    let suite = ConformanceSuite::new(
        AdapterId::new("ap2-mandate-receipt-vectors").or_panic("suite ID"),
        u64::try_from(fixture["cases"].as_array().or_panic("recorded cases").len())
            .or_panic("vector count"),
        Sha256::digest(include_bytes!("fixtures/ap2-mandate-pay-exact.json")).into(),
    )
    .or_panic("conformance suite");
    suite
}

fn recorded_pins_and_digest(
    case: &Value,
    executed: &ExecutedPayment,
) -> [SequencerAuthorization; 1] {
    let digest: [u8; 32] = recorded_array(&case["receiptDigest"]);
    assert_eq!(
        canonical_receipt_digest(&executed.evidence.signed_receipt),
        Ok(digest)
    );
    let key = &case["pinnedKey"];
    let pins = [SequencerAuthorization::new(
        recorded_array(&key["keyId"]),
        recorded_array(&key["publicKey"]),
        key["firstHeight"].as_u64().or_panic("first signing height"),
        key["lastHeight"].as_u64().or_panic("last signing height"),
    )];
    assert!(!executed.order_id.is_empty());
    assert!(executed
        .order_id
        .bytes()
        .all(|byte| byte.is_ascii_graphic()));
    assert_receipt_protocol(
        executed,
        &pins,
        "recorded receipt is signed by its height-pinned sequencer",
        "recorded protocol receipt",
    );
    pins
}

fn assert_receipt_protocol(
    executed: &ExecutedPayment,
    pins: &[SequencerAuthorization; 1],
    signature_context: &str,
    protocol_context: &str,
) {
    let receipt = verify_sequencer_signature(&executed.evidence.signed_receipt, pins)
        .unwrap_or_else(|error| panic!("{signature_context}: {error:?}"));
    let protocol = receipt.protocol().or_panic(protocol_context);
    assert_eq!(protocol.result_code(), 0);
    assert_eq!(protocol.activity_id(), executed.evidence.transaction_hash);
}

fn assert_payment_jwt(
    jwt: &str,
    payment_signer: &MandateReceiptSigner,
    expected_reference: &str,
    digest: [u8; 32],
) {
    let (input, signature) = jwt.rsplit_once('.').or_panic("signed payment JWT");
    let signature = Signature::from_slice(
        &URL_SAFE_NO_PAD
            .decode(signature)
            .or_panic("signature bytes"),
    )
    .or_panic("ES256 signature");
    payment_signer
        .verifying_key()
        .verify(input.as_bytes(), &signature)
        .or_panic("authentic payment receipt signature");
    let payload: Value = serde_json::from_slice(
        &URL_SAFE_NO_PAD
            .decode(input.split_once('.').or_panic("JWT payload").1)
            .or_panic("payload bytes"),
    )
    .or_panic("payment receipt payload");
    assert_eq!(payload["status"], "Success");
    assert_eq!(payload["reference"], expected_reference);
    let reference = payload["layerx_receipt_reference"]
        .as_str()
        .or_panic("LayerX reference");
    assert!(reference.starts_with("lxp:"));
    assert_eq!(reference, format!("lxp:{}", URL_SAFE_NO_PAD.encode(digest)));
    assert_eq!(
        payload["layerx_evidence"]["receiptDigest"],
        format!("0x{}", hex::encode(digest))
    );
}

trait OrPanic {
    type Output;

    fn or_panic(self, context: &str) -> Self::Output;
}

impl<T, E: std::fmt::Debug> OrPanic for std::result::Result<T, E> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> OrPanic for std::option::Option<T> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}
