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
            .expect("recorded hex string")
            .strip_prefix("0x")
            .expect("recorded hex prefix"),
    )
    .expect("recorded hex bytes")
}

fn recorded_array<const N: usize>(value: &Value) -> [u8; N] {
    recorded_hex(value)
        .try_into()
        .expect("recorded field width")
}

fn recorded_payment(case: &Value) -> ExecutedPayment {
    ExecutedPayment {
        evidence: SequencerEvidence {
            transaction_hash: recorded_array(&case["receiptJson"]["txHash"]),
            signed_receipt: recorded_hex(&case["signedReceipt"]),
            logs: case["receiptJson"]["logs"]
                .as_array()
                .expect("recorded logs")
                .iter()
                .map(|log| {
                    SequencerLog::new(
                        recorded_array(&log["address"]),
                        log["topics"]
                            .as_array()
                            .expect("recorded topics")
                            .iter()
                            .map(recorded_array)
                            .collect(),
                        recorded_hex(&log["data"]),
                        u64::from_str_radix(
                            log["logIndex"]
                                .as_str()
                                .expect("recorded log index")
                                .strip_prefix("0x")
                                .expect("log index prefix"),
                            16,
                        )
                        .expect("recorded log index quantity"),
                    )
                })
                .collect(),
            rung: SettlementRung::Receipted,
        },
        order_id: "order-12345".into(),
    }
}

struct IntentKey {
    directory: std::path::PathBuf,
    public_key: [u8; 32],
}

impl IntentKey {
    fn new() -> Self {
        let directory =
            std::env::temp_dir().join(format!("ap2-mandate-intent-{}", std::process::id()));
        std::fs::create_dir(&directory).expect("intent key directory");
        let mut der =
            hex::decode("302e020100300506032b657004220420").expect("Ed25519 PKCS8 prefix");
        der.extend_from_slice(&[0x35; 32]);
        std::fs::write(directory.join("key.der"), der).expect("intent signing key");
        let output = std::process::Command::new("openssl")
            .args(["pkey", "-inform", "DER", "-in"])
            .arg(directory.join("key.der"))
            .args(["-pubout", "-outform", "DER"])
            .output()
            .expect("Ed25519 public key command");
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        assert_eq!(output.stdout.len(), 44);
        Self {
            directory,
            public_key: output.stdout[12..].try_into().expect("Ed25519 public key"),
        }
    }

    fn sign(&self, message: &[u8]) -> [u8; 64] {
        std::fs::write(self.directory.join("message"), message).expect("intent signing input");
        let output = std::process::Command::new("openssl")
            .args(["pkeyutl", "-sign", "-rawin", "-keyform", "DER", "-inkey"])
            .arg(self.directory.join("key.der"))
            .arg("-in")
            .arg(self.directory.join("message"))
            .output()
            .expect("Ed25519 intent signer");
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        output.stdout.try_into().expect("Ed25519 intent signature")
    }
}

impl Drop for IntentKey {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.directory).expect("remove intent key directory");
    }
}

fn sign_payment_reference(
    evidence: &mut SequencerEvidence,
    payment: &AuthorizedPayment,
    key: &IntentKey,
) -> [u8; 32] {
    assert_eq!(evidence.logs.len(), 1, "one recorded PaymentExact log");
    let log = &evidence.logs[0];
    let old_reference = log.topics()[3];
    let mut topics = log.topics().to_vec();
    topics[3] = payment.idempotency_key();
    let positions: Vec<_> = evidence
        .signed_receipt
        .windows(32)
        .enumerate()
        .filter_map(|(index, bytes)| (bytes == old_reference).then_some(index))
        .collect();
    assert_eq!(positions.len(), 1, "one signed PaymentExact reference");
    evidence.signed_receipt[positions[0]..positions[0] + 32]
        .copy_from_slice(&payment.idempotency_key());
    evidence.logs[0] = SequencerLog::new(log.address(), topics, log.data().to_vec(), 0);
    let unsigned_end = evidence.signed_receipt.len() - 69;
    let mut hash = Sha256::new();
    hash.update(b"LXP/v1/receipt\0");
    hash.update(&evidence.signed_receipt[..unsigned_end]);
    hash.update([0]);
    let digest: [u8; 32] = hash.finalize().into();
    evidence.signed_receipt[unsigned_end + 5..].copy_from_slice(&key.sign(&digest));
    digest
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
            AccountId::parse("agent:buyer:main").expect("payer account"),
            AccountId::parse("agent:merchant:main").expect("payee account"),
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
            NetworkId::new(125).expect("network"),
            ProtocolVersion::new(2).expect("protocol"),
        )
        .map_err(|_| Ap2Error::IntentCompilation)?;
        let activity = ActivityType::new(ModuleId::Asset, 5).expect("send activity");
        let registry =
            ModuleRegistry::new(&[
                ModuleRegistration::new(ModuleId::Asset, &[activity]).expect("asset registration")
            ])
            .expect("module registry");
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
            ActivityType::new(ModuleId::Asset, 5).expect("send")
        );
        let log = &self.executed.evidence.logs[0];
        assert_eq!(payment.payer_receipt_account(), log.topics()[1]);
        assert_eq!(payment.payee_receipt_account(), log.topics()[2]);
        assert_eq!(payment.idempotency_key(), log.topics()[3]);
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
    fn issuer(&self) -> &str {
        "test-merchant-api"
    }

    fn key_id(&self) -> &str {
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
    SigningKey::from_bytes((&[7_u8; 32]).into())
        .unwrap_or_else(|error| panic!("valid issuer scalar: {error:?}"))
}
fn merchant_key() -> SigningKey {
    SigningKey::from_bytes((&[11_u8; 32]).into())
        .unwrap_or_else(|error| panic!("valid merchant scalar: {error:?}"))
}
fn agent_key() -> SigningKey {
    SigningKey::from_bytes((&[19_u8; 32]).into())
        .unwrap_or_else(|error| panic!("valid agent scalar: {error:?}"))
}
fn alternate_issuer_key() -> SigningKey {
    SigningKey::from_bytes((&[23_u8; 32]).into())
        .unwrap_or_else(|error| panic!("valid alternate issuer scalar: {error:?}"))
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
    let value: Value = serde_json::from_str(source)
        .unwrap_or_else(|error| panic!("golden vector specification is valid JSON: {error:?}"));
    assert_eq!(value["generator"], expected_generator);
    assert!(!source.to_ascii_lowercase().contains("placeholder"));
    value
}

fn jws(header: &Value, payload: &Value, key: &SigningKey) -> String {
    let header = URL_SAFE_NO_PAD.encode(
        serde_json::to_vec(header).unwrap_or_else(|error| panic!("serializable header: {error:?}")),
    );
    let payload = URL_SAFE_NO_PAD.encode(
        serde_json::to_vec(payload)
            .unwrap_or_else(|error| panic!("serializable payload: {error:?}")),
    );
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
    json!({"kty":"EC","crv":"P-256","x":URL_SAFE_NO_PAD.encode(point.x().unwrap_or_else(|| panic!("x coordinate"))),"y":URL_SAFE_NO_PAD.encode(point.y().unwrap_or_else(|| panic!("y coordinate")))})
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
    let first = presentation
        .find('.')
        .unwrap_or_else(|| panic!("header separator"));
    let start = presentation[first + 1..]
        .find('.')
        .map_or_else(|| panic!("payload separator"), |next| first + next + 2);
    let mut bytes = presentation.as_bytes().to_vec();
    bytes[start] = if bytes[start] == b'A' { b'B' } else { b'A' };
    String::from_utf8(bytes)
        .unwrap_or_else(|error| panic!("base64url mutation is UTF-8: {error:?}"))
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
            .unwrap_or_else(|| panic!("minor-unit amount")),
    );
    let (checkout, payment) = direct_pair(amount);
    let resolver = TestKeyResolver::authentic();
    let verified = MandateVerifier::new(&resolver)
        .verify(&checkout, &payment, &context())
        .unwrap_or_else(|error| panic!("authentic direct pair verifies: {error:?}"));
    assert_eq!(verified.mode(), MandateMode::Direct);
    assert_eq!(
        verified.checkout_id(),
        spec["expected_values"]["checkout_id"]
            .as_str()
            .unwrap_or_else(|| panic!("checkout id"))
    );
    assert_eq!(
        verified.payee().id(),
        spec["expected_values"]["payee_id"]
            .as_str()
            .unwrap_or_else(|| panic!("payee id"))
    );
    assert_eq!(verified.amount().minor_units(), amount);
    assert_eq!(
        verified.amount().currency(),
        spec["expected_values"]["currency"]
            .as_str()
            .unwrap_or_else(|| panic!("currency"))
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
        .unwrap_or_else(|error| panic!("authentic autonomous pair verifies: {error:?}"));
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
            .unwrap_or_else(|| panic!("authentic amount")),
    );
    let maximum = u128::from(
        spec["constraint_max_minor_units"]
            .as_u64()
            .unwrap_or_else(|| panic!("constraint maximum")),
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
    let fixture: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .expect("recorded sequencer receipt vectors");
    let case = fixture["cases"]
        .as_array()
        .expect("recorded cases")
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .expect("recorded payExact receipt");
    let mut executed = recorded_payment(case);
    let digest: [u8; 32] = recorded_array(&case["receiptDigest"]);
    assert_eq!(
        canonical_receipt_digest(&executed.evidence.signed_receipt),
        Ok(digest)
    );
    let key = &case["pinnedKey"];
    let pins = [SequencerAuthorization::new(
        recorded_array(&key["keyId"]),
        recorded_array(&key["publicKey"]),
        key["firstHeight"].as_u64().expect("first signing height"),
        key["lastHeight"].as_u64().expect("last signing height"),
    )];
    assert!(!executed.order_id.is_empty());
    assert!(executed
        .order_id
        .bytes()
        .all(|byte| byte.is_ascii_graphic()));
    let receipt = verify_sequencer_signature(&executed.evidence.signed_receipt, &pins)
        .expect("recorded receipt is signed by its height-pinned sequencer");
    let protocol = receipt.protocol().expect("recorded protocol receipt");
    assert_eq!(protocol.result_code(), 0);
    assert_eq!(protocol.activity_id(), executed.evidence.transaction_hash);
    let log = &executed.evidence.logs[0];
    let amount = u128::from_be_bytes(log.data()[48..].try_into().expect("recorded amount"));
    let binding = LayerXAssetBinding {
        currency: "USD".into(),
        minor_unit_exponent: 2,
        atomic_units_per_minor_unit: 1,
        asset: log.data()[..32].try_into().expect("recorded asset"),
        payer_receipt_account: log.topics()[1],
        payee_receipt_account: log.topics()[2],
        payee_merchant: Merchant::new("merchant-001", "Test Merchant", None).expect("merchant"),
    };
    let (checkout, payment) = direct_pair(amount);
    let resolver = TestKeyResolver::authentic();
    let verification = context();
    let verified = MandateVerifier::new(&resolver)
        .verify(&checkout, &payment, &verification)
        .expect("authentic recorded-amount mandates");
    let principal = PrincipalId::new("buyer").expect("principal");
    let authorized = layerx_ap2::authorize_payment(&principal, &verified, &verification, &binding)
        .expect("authorized recorded payment");
    let receipt_key = IntentKey::new();
    let digest = sign_payment_reference(&mut executed.evidence, &authorized, &receipt_key);
    assert_eq!(
        canonical_receipt_digest(&executed.evidence.signed_receipt),
        Ok(digest)
    );
    assert_ne!(digest, recorded_array::<32>(&case["receiptDigest"]));
    let pins = [SequencerAuthorization::new(
        recorded_array(&key["keyId"]),
        receipt_key.public_key,
        key["firstHeight"].as_u64().expect("first signing height"),
        key["lastHeight"].as_u64().expect("last signing height"),
    )];
    let receipt = verify_sequencer_signature(&executed.evidence.signed_receipt, &pins)
        .expect("mandate receipt is signed by its height-pinned sequencer");
    let protocol = receipt.protocol().expect("mandate protocol receipt");
    assert_eq!(protocol.result_code(), 0);
    assert_eq!(protocol.activity_id(), executed.evidence.transaction_hash);
    let signed_receipt = executed.evidence.signed_receipt.clone();
    let trace = TraceId::mint([0x57; 16]);
    let suite = ConformanceSuite::new(
        AdapterId::new("ap2-mandate-receipt-vectors").expect("suite ID"),
        u64::try_from(fixture["cases"].as_array().expect("recorded cases").len())
            .expect("vector count"),
        Sha256::digest(include_bytes!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))
        .into(),
    )
    .expect("conformance suite");
    let mut gateway = GatewayCore::new();
    gateway
        .register_adapter(
            ap2_adapter_descriptor(suite).expect("AP2 descriptor"),
            &trace,
            NOW,
        )
        .expect("register AP2");
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
        .expect("AP2 settles the recorded payExact receipt");
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
        .expect("portable receipt verifies");
    assert_eq!(portable.receipt_digest(), digest);
    assert_eq!(portable.signed_receipt(), signed_receipt);
    let (input, signature) = settled
        .payment_receipt_jwt
        .rsplit_once('.')
        .expect("signed payment JWT");
    let signature =
        Signature::from_slice(&URL_SAFE_NO_PAD.decode(signature).expect("signature bytes"))
            .expect("ES256 signature");
    payment_signer
        .verifying_key()
        .verify(input.as_bytes(), &signature)
        .expect("authentic payment receipt signature");
    let payload: Value = serde_json::from_slice(
        &URL_SAFE_NO_PAD
            .decode(input.split_once('.').expect("JWT payload").1)
            .expect("payload bytes"),
    )
    .expect("payment receipt payload");
    assert_eq!(payload["status"], "Success");
    assert_eq!(payload["reference"], verified.payment_receipt_reference());
    let reference = payload["layerx_receipt_reference"]
        .as_str()
        .expect("LayerX reference");
    assert!(reference.starts_with("lxp:"));
    assert_eq!(reference, format!("lxp:{}", URL_SAFE_NO_PAD.encode(digest)));
    assert_eq!(
        payload["layerx_evidence"]["receiptDigest"],
        format!("0x{}", hex::encode(digest))
    );
}
