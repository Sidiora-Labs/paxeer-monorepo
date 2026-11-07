use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use layerx_ap2::{
    ap2_adapter_descriptor, authorize_payment, Ap2Adapter, Ap2Error, Ap2ExternalMandateVerifier,
    Ap2MandatePair, AuthorizedPayment, KeyResolver, KeyUse, LayerXAssetBinding, MandateMode,
    MandateVerifier, Merchant, PortableLayerXEvidence, ProtectedHeader, VerificationContext,
};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite};
use layerx_interop_gateway::audit::AuditEventKind;
use layerx_interop_gateway::error::GatewayError;
use layerx_interop_gateway::gateway::{TranslationKind, TranslationRequest, TranslationStatus};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_portable::{ExternalEvidenceVerifier, PortableReceiptV2, PORTABLE_RECEIPT_FORMAT};
use layerx_proof::inclusion::SequencerAuthorization;
use p256::ecdsa::signature::Signer as _;
use p256::ecdsa::{Signature, SigningKey, VerifyingKey};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const NOW: u64 = 1_700_000_000;
const RECEIPTS: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

struct MandateKeys;

fn key(seed: u8) -> SigningKey {
    SigningKey::from_bytes((&[seed; 32]).into()).expect("fixture signing scalar")
}

impl KeyResolver for MandateKeys {
    fn resolve(&self, usage: KeyUse, header: &ProtectedHeader) -> Result<VerifyingKey, Ap2Error> {
        match (usage, header.key_id()) {
            (KeyUse::CheckoutMandateIssuer | KeyUse::PaymentMandateIssuer, Some("issuer")) => {
                Ok(*key(7).verifying_key())
            }
            (KeyUse::MerchantCheckout, Some("merchant")) => Ok(*key(11).verifying_key()),
            _ => Err(Ap2Error::KeyResolution),
        }
    }
}

fn jws(header: Value, payload: Value, key: &SigningKey) -> String {
    let input = format!(
        "{}.{}",
        URL_SAFE_NO_PAD.encode(serde_json::to_vec(&header).expect("header")),
        URL_SAFE_NO_PAD.encode(serde_json::to_vec(&payload).expect("payload"))
    );
    let signature: Signature = key.sign(input.as_bytes());
    format!("{input}.{}", URL_SAFE_NO_PAD.encode(signature.to_bytes()))
}

fn hash(value: &str) -> String {
    URL_SAFE_NO_PAD.encode(Sha256::digest(value.as_bytes()))
}

fn context(now: u64, skew: u64) -> VerificationContext<'static> {
    VerificationContext {
        now,
        clock_skew_seconds: skew,
        expected_audience: "merchant-api",
        expected_nonce: "checkout-nonce",
        currency_minor_exponent: 2,
        usage: None,
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum Token {
    CheckoutRoot,
    PaymentRoot,
    CheckoutOpen,
    PaymentOpen,
    CheckoutBinding,
    PaymentBinding,
}

const TOKENS: [Token; 6] = [
    Token::CheckoutRoot,
    Token::PaymentRoot,
    Token::CheckoutOpen,
    Token::PaymentOpen,
    Token::CheckoutBinding,
    Token::PaymentBinding,
];

fn times() -> Value {
    json!({"iat":NOW-60,"exp":NOW+3600})
}

fn root(mandate: Value, mut claims: Value) -> String {
    claims["delegate_payload"] = json!([mandate]);
    format!(
        "{}~",
        jws(
            json!({"alg":"ES256","typ":"dc+sd-jwt","kid":"issuer"}),
            claims,
            &key(7),
        )
    )
}

fn bound(open: &str, mandate: Value, mut claims: Value) -> String {
    claims["aud"] = json!("merchant-api");
    claims["nonce"] = json!("checkout-nonce");
    claims["sd_hash"] = json!(hash(open));
    claims["delegate_payload"] = json!([mandate]);
    format!(
        "{}~",
        jws(json!({"alg":"ES256","typ":"kb+sd-jwt"}), claims, &key(19),)
    )
}

fn pair_with_payment(
    target: Token,
    claims: &Value,
    execution_date: Option<&str>,
) -> (String, String) {
    let checkout = jws(
        json!({"alg":"ES256","typ":"JWT","kid":"merchant"}),
        json!({
            "id":"checkout-12345","merchant":{"id":"merchant-001","name":"Test Merchant"},
            "line_items":[{"id":"line-001","item":{"id":"sku-001","title":"Node credit","price":100},"quantity":1,"totals":[{"type":"total","amount":100}]}],
            "status":"completed","currency":"USD","totals":[{"type":"total","amount":100}],"links":[]
        }),
        &key(11),
    );
    let closed_checkout = json!({
        "vct":"mandate.checkout.1","checkout_jwt":checkout,"checkout_hash":hash(&checkout)
    });
    let mut closed_payment = json!({
        "vct":"mandate.payment.1","transaction_id":hash(&checkout),
        "payee":{"id":"merchant-001","name":"Test Merchant"},
        "payment_amount":{"amount":100,"currency":"USD"},
        "payment_instrument":{"id":"card-001","type":"card"}
    });
    if let Some(date) = execution_date {
        closed_payment["execution_date"] = json!(date);
    }
    let at = |location| {
        if target == location {
            claims.clone()
        } else {
            times()
        }
    };
    if matches!(target, Token::CheckoutRoot | Token::PaymentRoot) {
        return (
            root(closed_checkout, at(Token::CheckoutRoot)),
            root(closed_payment, at(Token::PaymentRoot)),
        );
    }
    let agent = key(19);
    let point = agent.verifying_key().to_encoded_point(false);
    let confirmation = json!({"jwk":{
        "kty":"EC","crv":"P-256",
        "x":URL_SAFE_NO_PAD.encode(point.x().expect("x")),
        "y":URL_SAFE_NO_PAD.encode(point.y().expect("y"))
    }});
    let open_checkout = root(
        json!({"vct":"mandate.checkout.open.1","cnf":confirmation,"constraints":[{
            "type":"checkout.line_items","items":[{"id":"credit","acceptable_items":[{"id":"sku-001","title":"Node credit"}],"quantity":1}]
        }]}),
        at(Token::CheckoutOpen),
    );
    let open_payment = root(
        json!({"vct":"mandate.payment.open.1","cnf":confirmation,"constraints":[
            {"type":"payment.reference","conditional_transaction_id":hash(&open_checkout)},
            {"type":"payment.amount_range","currency":"USD","min":1,"max":100}
        ]}),
        at(Token::PaymentOpen),
    );
    let checkout_binding = bound(&open_checkout, closed_checkout, at(Token::CheckoutBinding));
    let payment_binding = bound(&open_payment, closed_payment, at(Token::PaymentBinding));
    (
        format!("{open_checkout}~~{checkout_binding}"),
        format!("{open_payment}~~{payment_binding}"),
    )
}

fn pair(target: Token, claims: &Value) -> (String, String) {
    pair_with_payment(target, claims, None)
}

fn suite() -> ConformanceSuite {
    let corpus = Sha256::digest(RECEIPTS.as_bytes()).into();
    ConformanceSuite::new(
        AdapterId::new("ap2-repair-vectors").expect("suite ID"),
        1,
        corpus,
    )
    .expect("conformance suite")
}

fn binding() -> LayerXAssetBinding {
    LayerXAssetBinding {
        currency: "USD".into(),
        minor_unit_exponent: 2,
        atomic_units_per_minor_unit: 10,
        asset: [1; 32],
        payer_receipt_account: [2; 32],
        payee_receipt_account: [3; 32],
        payee_merchant: Merchant::new("merchant-001", "Test Merchant", None).expect("merchant"),
    }
}

fn request(payment: &AuthorizedPayment) -> TranslationRequest {
    TranslationRequest::new(
        AdapterId::new("ap2").expect("adapter ID"),
        TranslationKind::StateChanging,
        payment.idempotency_key(),
        payment.request_digest(),
    )
    .expect("real gateway request")
}

struct SavedMandates(PathBuf);

impl SavedMandates {
    fn write(bytes: &[u8]) -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "ap2-repair-22-33-88-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        let mut file = std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&path)
            .expect("create saved mandate pair");
        std::io::Write::write_all(&mut file, bytes).expect("save exact signed pair");
        file.sync_all().expect("persist signed pair before closing");
        Self(path)
    }

    fn reopen(&self) -> Vec<u8> {
        std::fs::read(&self.0).expect("reopen saved signed pair")
    }
}

impl Drop for SavedMandates {
    fn drop(&mut self) {
        std::fs::remove_file(&self.0).expect("remove saved mandate pair");
    }
}

#[test]
fn ap2_replay_clock_digest_reopens_signed_state() {
    for target in [Token::PaymentRoot, Token::PaymentBinding] {
        let pair = pair(target, &times());
        let principal = PrincipalId::new("buyer").expect("principal");
        let trace = TraceId::mint([0x88; 16]);
        let policy = binding();
        let encoded = Ap2MandatePair::new(&pair.0, &pair.1)
            .to_json()
            .expect("producer codec");
        let saved = SavedMandates::write(&encoded);
        let mut gateway = GatewayCore::new();
        gateway
            .register_adapter(
                ap2_adapter_descriptor(suite()).expect("descriptor"),
                &trace,
                NOW,
            )
            .expect("registered AP2");
        let first = {
            let verifier =
                Ap2ExternalMandateVerifier::new(&MandateKeys, suite()).expect("verifier");
            let verified = verifier
                .verify(&encoded, &context(NOW, 0))
                .expect("transport consumer");
            let first = authorize_payment(&principal, &verified, &context(NOW, 0), &policy)
                .expect("first authorized request");
            assert_eq!(first.execution_at(), NOW);
            assert_eq!(
                gateway
                    .begin_translation(&principal, &request(&first), &trace, NOW)
                    .expect("first submission"),
                TranslationStatus::Pending
            );
            let later = verifier
                .verify(&encoded, &context(NOW + 1, 0))
                .expect("fresh verification");
            let later = authorize_payment(&principal, &later, &context(NOW + 1, 0), &policy)
                .expect("fresh authorization");
            assert_eq!(later.execution_at(), NOW + 1);
            assert_eq!(later.idempotency_key(), first.idempotency_key());
            assert_eq!(later.request_digest(), first.request_digest());
            assert_eq!(
                gateway
                    .begin_translation(&principal, &request(&later), &trace, NOW + 1)
                    .expect("retry pending request"),
                TranslationStatus::Pending
            );
            assert_eq!(
                gateway
                    .refuse_translation(&principal, first.idempotency_key(), &trace, NOW + 1)
                    .expect("record terminal refusal"),
                TranslationStatus::Refused
            );
            first
        };
        // The AP2 verifier and its decoded state have stopped. Reopen only
        // the producer's saved bytes; the independent gateway store survives.
        drop(pair);
        let reopened = saved.reopen();
        assert_eq!(reopened, encoded);
        let verifier =
            Ap2ExternalMandateVerifier::new(&MandateKeys, suite()).expect("reopened verifier");
        let recovered = verifier
            .verify(&reopened, &context(NOW + 2, 0))
            .expect("recover signed state");
        let recovered = authorize_payment(&principal, &recovered, &context(NOW + 2, 0), &policy)
            .expect("recover authorized request");
        assert_eq!(recovered.execution_at(), NOW + 2);
        assert_eq!(recovered.idempotency_key(), first.idempotency_key());
        assert_eq!(recovered.request_digest(), first.request_digest());
        assert_eq!(
            gateway
                .begin_translation(&principal, &request(&recovered), &trace, NOW + 2)
                .expect("recover original result"),
            TranslationStatus::Refused
        );
        assert_eq!(
            gateway.translation(&principal, recovered.idempotency_key()),
            Some(TranslationStatus::Refused)
        );
        let audit = gateway
            .principal_audit(&principal)
            .expect("production audit chain");
        assert_eq!(
            audit
                .entries()
                .iter()
                .filter(|entry| entry.kind() == AuditEventKind::TranslationBegun)
                .count(),
            1,
            "one submission across time changes and AP2 state reopen"
        );
        assert_eq!(audit.entries().len(), 2, "one begin and one refusal");

        let mut changed_policy = policy.clone();
        changed_policy.atomic_units_per_minor_unit += 1;
        let verified = verifier
            .verify(&reopened, &context(NOW + 3, 0))
            .expect("signed state");
        let changed =
            authorize_payment(&principal, &verified, &context(NOW + 3, 0), &changed_policy)
                .expect("changed execution policy");
        assert_eq!(changed.idempotency_key(), first.idempotency_key());
        assert_ne!(changed.request_digest(), first.request_digest());
        let error = gateway
            .begin_translation(&principal, &request(&changed), &trace, NOW + 3)
            .expect_err("changed economic request still conflicts");
        assert_eq!(error.error(), &GatewayError::IdempotencyConflict);
    }
}

#[test]
fn ap2_replay_clock_digest_binds_signed_execution_date() {
    let principal = PrincipalId::new("buyer").expect("principal");
    let policy = binding();
    let mut digests = Vec::new();
    for date in [
        None,
        Some("2023-11-14T22:13:20Z"),
        Some("2023-11-14T22:13:21Z"),
    ] {
        let pair = pair_with_payment(Token::PaymentRoot, &times(), date);
        let mut digest = None;
        for now in [NOW, NOW + 1] {
            let verification = context(now, 5);
            let verified = MandateVerifier::new(&MandateKeys)
                .verify(&pair.0, &pair.1, &verification)
                .expect("signed execution policy");
            let payment = authorize_payment(&principal, &verified, &verification, &policy)
                .expect("authorized execution policy");
            if let Some(previous) = digest {
                assert_eq!(payment.request_digest(), previous);
            }
            digest = Some(payment.request_digest());
        }
        digests.push(digest.expect("request digest"));
    }
    assert_ne!(
        digests[0], digests[1],
        "explicit signed execution date stays bound"
    );
    assert_ne!(
        digests[1], digests[2],
        "different signed dates stay distinct"
    );
}

#[test]
fn outer_jwt_time_types_rejects_present_invalid_root_and_binding_claims() {
    let invalid = [
        Value::Null,
        json!("1700000000"),
        json!(-1),
        json!(1.5),
        json!(1.0),
        json!(true),
        json!([]),
        json!({}),
        serde_json::from_str("18446744073709551616").expect("out-of-range integer"),
    ];
    for target in TOKENS {
        for claim in ["iat", "exp"] {
            for value in &invalid {
                let mut claims = times();
                claims[claim] = value.clone();
                let pair = pair(target, &claims);
                let result =
                    MandateVerifier::new(&MandateKeys).verify(&pair.0, &pair.1, &context(NOW, 0));
                assert!(
                    matches!(result, Err(Ap2Error::Malformed("JWT NumericDate"))),
                    "{target:?} must reject present {claim}={value}"
                );
            }
        }
    }
}

#[test]
fn outer_jwt_time_types_missing_and_unsigned_integer_claims_are_supported() {
    for target in TOKENS {
        for claims in [
            json!({}),
            json!({"iat":0}),
            json!({"exp":u64::MAX}),
            times(),
        ] {
            let pair = pair(target, &claims);
            let verified = MandateVerifier::new(&MandateKeys)
                .verify(&pair.0, &pair.1, &context(NOW, 0))
                .unwrap_or_else(|error| panic!("{target:?} claims {claims}: {error:?}"));
            assert_eq!(verified.amount().minor_units(), 100);
            assert_eq!(
                verified.mode(),
                if matches!(target, Token::CheckoutRoot | Token::PaymentRoot) {
                    MandateMode::Direct
                } else {
                    MandateMode::Autonomous
                }
            );
        }
    }
}

#[test]
fn outer_jwt_time_types_expiration_and_issuance_boundaries() {
    for target in TOKENS {
        for (skew, exp, expected) in [
            (0, NOW - 1, Err(Ap2Error::Expired)),
            (0, NOW, Err(Ap2Error::Expired)),
            (0, NOW + 1, Ok(())),
            (5, NOW - 6, Err(Ap2Error::Expired)),
            (5, NOW - 5, Err(Ap2Error::Expired)),
            (5, NOW - 4, Ok(())),
        ] {
            let pair = pair(target, &json!({"iat":NOW-60,"exp":exp}));
            let result =
                MandateVerifier::new(&MandateKeys).verify(&pair.0, &pair.1, &context(NOW, skew));
            assert_eq!(
                result.map(|_| ()),
                expected,
                "{target:?}: exp={exp}, skew={skew}"
            );
        }
        for (iat, expected) in [(NOW + 5, Ok(())), (NOW + 6, Err(Ap2Error::NotYetValid))] {
            let pair = pair(target, &json!({"iat":iat,"exp":NOW+60}));
            let result =
                MandateVerifier::new(&MandateKeys).verify(&pair.0, &pair.1, &context(NOW, 5));
            assert_eq!(result.map(|_| ()), expected, "{target:?}: iat={iat}");
        }
        let pair = pair(target, &json!({"iat":NOW+1,"exp":NOW+1}));
        let result = MandateVerifier::new(&MandateKeys).verify(&pair.0, &pair.1, &context(NOW, 5));
        assert_eq!(
            result.map(|_| ()),
            Err(Ap2Error::Malformed("mandate time window")),
            "{target:?}"
        );
    }
}

fn fixture_bytes(value: &Value) -> Vec<u8> {
    hex::decode(
        value
            .as_str()
            .expect("hex field")
            .strip_prefix("0x")
            .expect("hex prefix"),
    )
    .expect("recorded bytes")
}

#[test]
fn ap2_public_api_registry_constructor_and_v2_export_consumer() {
    let fixture: Value =
        serde_json::from_str(RECEIPTS).expect("recorded sequencer producer corpus");
    let case = fixture["cases"]
        .as_array()
        .expect("cases")
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .expect("recorded payment");
    let pin = &case["pinnedKey"];
    let pins = [SequencerAuthorization::new(
        fixture_bytes(&pin["keyId"]).try_into().expect("key ID"),
        fixture_bytes(&pin["publicKey"])
            .try_into()
            .expect("public key"),
        pin["firstHeight"].as_u64().expect("first height"),
        pin["lastHeight"].as_u64().expect("last height"),
    )];
    let _adapter: Ap2Adapter<'_, MandateKeys> = Ap2Adapter::new(&MandateKeys, &pins);
    let signed = fixture_bytes(&case["signedReceipt"]);
    let height = u64::from_str_radix(
        case["receiptJson"]["height"]
            .as_str()
            .expect("height")
            .strip_prefix("0x")
            .expect("quantity prefix"),
        16,
    )
    .expect("height quantity");
    let exported: PortableLayerXEvidence = PortableReceiptV2::export(&signed, &pins[0], height)
        .expect("real producer receipt exports through the public AP2 alias");
    let transported = exported.to_json().expect("v2 wire document");
    let consumed: PortableReceiptV2 =
        PortableLayerXEvidence::from_json(&transported).expect("downstream v2 consumer");
    assert_eq!(consumed, exported);
    assert_eq!(consumed.format(), PORTABLE_RECEIPT_FORMAT);
    assert_eq!(consumed.format(), "layerx-receipt-proof-v2");
    let verified = consumed
        .verify_v2(&pins)
        .expect("downstream pinned signature verification");
    assert_eq!(verified.signed_receipt(), signed);
    assert_eq!(
        verified.receipt_digest().as_slice(),
        fixture_bytes(&case["receiptDigest"])
    );
    assert_eq!(verified.receipt_key(), &pins[0]);
    assert_eq!(verified.height(), height);
    assert_eq!(verified.result_code(), 0);
}
