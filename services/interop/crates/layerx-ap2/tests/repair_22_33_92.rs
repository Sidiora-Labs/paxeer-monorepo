#[path = "support/source_receipt.rs"]
mod source_receipt;

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use layerx_ap2::{
    ap2_adapter_descriptor, authorize_payment, AdapterOutcome, Ap2Adapter, Ap2Error,
    Ap2ExternalMandateVerifier, Ap2MandatePair, AuthorizedPayment, KeyResolver, KeyUse,
    LayerXAssetBinding, MandateVerifier, Merchant, ProtectedHeader, ReceiptSigner,
    VerificationContext,
};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite};
use layerx_interop_gateway::error::GatewayError;
use layerx_interop_gateway::gateway::TranslationStatus;
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_portable::{ExternalEvidenceVerifier, PortableReceiptV2};
use p256::ecdsa::signature::{Signer as _, Verifier as _};
use p256::ecdsa::{Signature, SigningKey, VerifyingKey};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::io::Write as _;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

const NOW: u64 = 1_700_000_000;

struct MandateKeys;

fn signing_key(seed: u8) -> SigningKey {
    SigningKey::from_bytes((&[seed; 32]).into()).expect("test signing key")
}

impl KeyResolver for MandateKeys {
    fn resolve(&self, usage: KeyUse, header: &ProtectedHeader) -> Result<VerifyingKey, Ap2Error> {
        match (usage, header.key_id()) {
            (
                KeyUse::CheckoutMandateIssuer | KeyUse::PaymentMandateIssuer,
                Some("issuer" | "issuer-001"),
            ) => Ok(*signing_key(7).verifying_key()),
            (KeyUse::MerchantCheckout, Some("merchant" | "merchant-001")) => {
                Ok(*signing_key(11).verifying_key())
            }
            _ => Err(Ap2Error::KeyResolution),
        }
    }
}

fn jws(header: Value, payload: Value, seed: u8) -> String {
    let input = format!(
        "{}.{}",
        URL_SAFE_NO_PAD.encode(serde_json::to_vec(&header).expect("header")),
        URL_SAFE_NO_PAD.encode(serde_json::to_vec(&payload).expect("payload"))
    );
    let signature: Signature = signing_key(seed).sign(input.as_bytes());
    format!("{input}.{}", URL_SAFE_NO_PAD.encode(signature.to_bytes()))
}

fn mandates() -> (String, String) {
    mandates_for("issuer", "merchant", "Node credit")
}

fn mandates_for(issuer: &str, merchant: &str, title: &str) -> (String, String) {
    let checkout = jws(
        json!({"alg":"ES256","typ":"JWT","kid":merchant}),
        json!({
            "id":"checkout-12345","merchant":{"id":"merchant-001","name":"Test Merchant"},
            "line_items":[{"id":"line-001","item":{"id":"sku-001","title":title,"price":1000000},"quantity":1,"totals":[{"type":"total","amount":1000000}]}],
            "status":"completed","currency":"USD","totals":[{"type":"total","amount":1000000}],"links":[]
        }),
        11,
    );
    let reference = URL_SAFE_NO_PAD.encode(Sha256::digest(checkout.as_bytes()));
    let root = |mandate| {
        format!(
            "{}~",
            jws(
                json!({"alg":"ES256","typ":"dc+sd-jwt","kid":issuer}),
                json!({"iat":NOW-60,"exp":NOW+3600,"delegate_payload":[mandate]}),
                7
            )
        )
    };
    (
        root(
            json!({"vct":"mandate.checkout.1","checkout_jwt":checkout,"checkout_hash":reference,"iat":NOW-30,"exp":NOW+1800}),
        ),
        root(
            json!({"vct":"mandate.payment.1","transaction_id":reference,"payee":{"id":"merchant-001","name":"Test Merchant"},"payment_amount":{"amount":1000000,"currency":"USD"},"payment_instrument":{"id":"card-001","type":"card"},"iat":NOW-30,"exp":NOW+1800}),
        ),
    )
}

fn context(now: u64) -> VerificationContext<'static> {
    VerificationContext {
        now,
        clock_skew_seconds: 300,
        expected_audience: "merchant-api",
        expected_nonce: "nonce",
        currency_minor_exponent: 2,
        usage: None,
    }
}

fn authorized(pair: &(String, String), now: u64) -> AuthorizedPayment {
    let context = context(now);
    let verified = MandateVerifier::new(&MandateKeys)
        .verify(&pair.0, &pair.1, &context)
        .expect("authentic mandates at the actual verification time");
    let words = source_receipt::calldata(&source_receipt::case());
    authorize_payment(
        &PrincipalId::new("buyer").expect("principal"),
        &verified,
        &context,
        &LayerXAssetBinding {
            currency: "USD".into(),
            minor_unit_exponent: 2,
            atomic_units_per_minor_unit: 1,
            asset: words[2],
            payer_receipt_account: words[0],
            payee_receipt_account: words[1],
            payee_merchant: Merchant::new("merchant-001", "Test Merchant", None).expect("merchant"),
        },
    )
    .expect("production mandate authorization")
}

#[test]
fn node_receipt_keeps_transaction_logs_and_sequence() {
    for name in ["mandates", "settlement"] {
        let case = source_receipt::named_case(name);
        source_receipt::assert_source(&case, &source_receipt::evidence(&case));
        let mut changed_transaction = source_receipt::bytes(&case["rawTransaction"]);
        let last = changed_transaction.len() - 1;
        changed_transaction[last] ^= 1;
        assert!(
            PortableReceiptV2::from_sequencer_receipt(
                &case["receiptJson"],
                &[source_receipt::pin(&case)],
                Some(&changed_transaction),
            )
            .is_err(),
            "the original signed receipt must refuse a different payer transaction"
        );
    }
}

#[test]
fn mandate_fixture_rewritten_receipt() {
    let case = source_receipt::named_case("mandates");
    let pair = mandates_for("issuer-001", "merchant-001", "LayerX node credit");
    assert_eq!(pair.0, case["checkoutMandate"]);
    assert_eq!(pair.1, case["paymentMandate"]);
    let payment = authorized(&pair, NOW);
    let digest =
        source_receipt::assert_mandate_reference(&case, &source_receipt::evidence(&case), &payment);
    let pins = [source_receipt::pin(&case)];
    let trace = TraceId::mint([0x93; 16]);
    let mut gateway = new_gateway(&trace, NOW);
    let mut plane = source_receipt::RecordedPlane::new(case);
    let mut payment_signer = Ap2Signer(signing_key(23));
    let mut checkout_signer = Ap2Signer(signing_key(29));
    let result = Ap2Adapter::new(&MandateKeys, &pins)
        .execute(
            &mut gateway,
            payment.principal(),
            &pair.0,
            &pair.1,
            &context(NOW),
            &binding(&plane.case),
            &mut plane,
            &mut payment_signer,
            &mut checkout_signer,
            &trace,
            NOW + 1,
        )
        .expect("production mandate compilation and recorded node settlement");
    let AdapterOutcome::Settled(settled) = result else {
        panic!("expected settlement: {result:?}")
    };
    assert_eq!(settled.receipt_digest, digest);
    assert_eq!(
        settled
            .layerx
            .verify_v2(&pins)
            .expect("portable settlement evidence")
            .receipt_digest(),
        digest
    );
    assert_eq!(plane.executions, 1);
    verify_ap2_jwts(
        &serde_json::to_vec(&json!({
            "payment": settled.payment_receipt_jwt,
            "checkout": settled.checkout_receipt_jwt,
        }))
        .expect("original signed AP2 JWTs"),
        &pins,
        digest,
        &plane.case,
    );
}

#[test]
fn ap2_settlement_rewritten_receipt() {
    let pair = mandates();
    let first = authorized(&pair, NOW);
    let later = authorized(&pair, NOW + 1);
    assert_eq!(first.idempotency_key(), later.idempotency_key());
    let trace = TraceId::mint([0x92; 16]);
    let mut gateway = GatewayCore::new();
    gateway
        .register_adapter(
            ap2_adapter_descriptor(
                ConformanceSuite::new(
                    AdapterId::new("ap2-replay-regression").expect("suite ID"),
                    3,
                    [0x92; 32],
                )
                .expect("suite"),
            )
            .expect("AP2 descriptor"),
            &trace,
            NOW,
        )
        .expect("register AP2");
    let request = |payment: &AuthorizedPayment| {
        payment
            .translation_request()
            .expect("stable AP2 gateway request")
    };
    assert_eq!(
        gateway
            .begin_translation(first.principal(), &request(&first), &trace, NOW)
            .expect("initial translation"),
        TranslationStatus::Pending
    );
    assert_eq!(
        gateway
            .begin_translation(later.principal(), &request(&later), &trace, NOW + 1)
            .expect(
                "fresh verification must replay the same request without an idempotency conflict"
            ),
        TranslationStatus::Pending
    );
    assert_eq!(first.request_digest(), later.request_digest());
    assert_eq!(
        gateway
            .principal_audit(first.principal())
            .expect("audit")
            .entries()
            .len(),
        1
    );

    let case = source_receipt::case();
    assert_eq!(pair.0, case["checkoutMandate"]);
    assert_eq!(pair.1, case["paymentMandate"]);
    let digest =
        source_receipt::assert_mandate_reference(&case, &source_receipt::evidence(&case), &first);
    let pins = [source_receipt::pin(&case)];
    let mut plane = source_receipt::RecordedPlane::new(case.clone());
    let mut payment_signer = Ap2Signer(signing_key(23));
    let mut checkout_signer = Ap2Signer(signing_key(29));
    let adapter = Ap2Adapter::new(&MandateKeys, &pins);
    let result = adapter
        .execute(
            &mut gateway,
            first.principal(),
            &pair.0,
            &pair.1,
            &context(NOW + 1),
            &binding(&case),
            &mut plane,
            &mut payment_signer,
            &mut checkout_signer,
            &trace,
            NOW + 1,
        )
        .expect("real AP2 compilation and settlement of the recorded producer result");
    let AdapterOutcome::Settled(settled) = result else {
        panic!("expected settlement: {result:?}")
    };
    assert_eq!(settled.receipt_digest, digest);
    assert_eq!(plane.executions, 1);
    assert_eq!(
        gateway.translation(first.principal(), first.idempotency_key()),
        Some(TranslationStatus::ReceiptVerified {
            receipt_digest: digest
        })
    );
    let portable = PortableReceiptV2::from_sequencer_receipt(
        &case["receiptJson"],
        &pins,
        Some(&source_receipt::bytes(&case["rawTransaction"])),
    )
    .expect("production portable encoder binds the payer transaction");
    let mut archive = SavedState::new();
    archive.save(
        "mandates.json",
        &Ap2MandatePair::new(&pair.0, &pair.1)
            .to_json()
            .expect("mandate transport encoder"),
    );
    archive.save(
        "portable.json",
        &portable.to_json().expect("portable transport encoder"),
    );
    archive.save(
        "execution.json",
        &serde_json::to_vec(&case).expect("unchanged recorded execution"),
    );
    archive.save(
        "receipts.json",
        &serde_json::to_vec(&json!({
            "payment": settled.payment_receipt_jwt, "checkout": settled.checkout_receipt_jwt,
        }))
        .expect("original signed AP2 receipts"),
    );
    verify_ap2_jwts(&archive.load("receipts.json"), &pins, digest, &case);
    drop(settled);
    drop(gateway);
    drop(adapter);
    drop(plane);
    drop(pair);
    drop(payment_signer);
    drop(checkout_signer);

    let encoded = archive.load("mandates.json");
    let recovered_pair: Value =
        serde_json::from_slice(&encoded).expect("reopened exact presentations");
    let recovered_case: Value = serde_json::from_slice(&archive.load("execution.json"))
        .expect("reopened original node execution");
    assert_eq!(recovered_case, case);
    let verifier =
        Ap2ExternalMandateVerifier::new(&MandateKeys, suite()).expect("new verifier after restart");
    let verified = verifier
        .verify(&encoded, &context(NOW + 2))
        .expect("production transport consumer after reopen");
    let recovered = authorize_payment(
        first.principal(),
        &verified,
        &context(NOW + 2),
        &binding(&case),
    )
    .expect("recover mandate authorization");
    assert_eq!(recovered.execution_at(), NOW + 2);
    assert_eq!(recovered.request_digest(), first.request_digest());
    assert_eq!(recovered.idempotency_key(), first.idempotency_key());
    let portable = PortableReceiptV2::from_json(&archive.load("portable.json"))
        .expect("reopened portable evidence");
    let proof = portable
        .verify_v2(&pins)
        .expect("independent pinned verification after reopen");
    assert_eq!(proof.receipt_digest(), digest);
    assert_eq!(
        proof.raw_transaction(),
        Some(source_receipt::bytes(&case["rawTransaction"]).as_slice())
    );
    assert_eq!(
        proof.signed_receipt(),
        source_receipt::bytes(&case["signedReceipt"])
    );
    let evidence = source_receipt::evidence(&recovered_case);
    source_receipt::assert_mandate_reference(&recovered_case, &evidence, &recovered);
    let mut gateway = new_gateway(&trace, NOW + 2);
    assert_eq!(
        gateway
            .begin_translation(recovered.principal(), &request(&recovered), &trace, NOW + 2)
            .expect("restore original authorized request"),
        TranslationStatus::Pending
    );
    assert_eq!(
        gateway
            .settle_with_sequencer_evidence(
                recovered.principal(),
                recovered.idempotency_key(),
                &evidence,
                &pins,
                &trace,
                NOW + 2
            )
            .expect("restore gateway state only through original verified evidence"),
        TranslationStatus::ReceiptVerified {
            receipt_digest: digest
        }
    );
    let before = gateway
        .principal_audit(recovered.principal())
        .expect("restored audit")
        .entries()
        .len();
    let mut plane = source_receipt::RecordedPlane::new(recovered_case);
    let mut payment_signer = Ap2Signer(signing_key(23));
    let mut checkout_signer = Ap2Signer(signing_key(29));
    for now in [NOW + 3, NOW + 17] {
        let result = Ap2Adapter::new(&MandateKeys, &pins)
            .execute(
                &mut gateway,
                recovered.principal(),
                recovered_pair["checkoutMandate"]
                    .as_str()
                    .expect("checkout presentation"),
                recovered_pair["paymentMandate"]
                    .as_str()
                    .expect("payment presentation"),
                &context(now),
                &binding(&case),
                &mut plane,
                &mut payment_signer,
                &mut checkout_signer,
                &trace,
                now,
            )
            .expect("fresh verification must recover the settled result after reopening");
        assert_eq!(
            result,
            AdapterOutcome::AlreadySettled {
                receipt_digest: digest
            }
        );
    }
    assert_eq!(
        plane.executions, 0,
        "recovery never submits a second payment"
    );
    assert_eq!(
        gateway
            .principal_audit(recovered.principal())
            .expect("replay audit")
            .entries()
            .len(),
        before
    );
    verify_ap2_jwts(&archive.load("receipts.json"), &pins, digest, &case);
    let mut changed_binding = binding(&case);
    changed_binding.atomic_units_per_minor_unit = 2;
    let changed = authorize_payment(
        recovered.principal(),
        &verified,
        &context(NOW + 18),
        &changed_binding,
    )
    .expect("changed policy");
    assert_eq!(changed.idempotency_key(), recovered.idempotency_key());
    assert_ne!(changed.request_digest(), recovered.request_digest());
    assert_eq!(
        gateway
            .begin_translation(changed.principal(), &request(&changed), &trace, NOW + 18)
            .expect_err("changed economics must still conflict")
            .error(),
        &GatewayError::IdempotencyConflict
    );
}

fn binding(case: &Value) -> LayerXAssetBinding {
    let words = source_receipt::calldata(case);
    LayerXAssetBinding {
        currency: "USD".into(),
        minor_unit_exponent: 2,
        atomic_units_per_minor_unit: 1,
        asset: words[2],
        payer_receipt_account: words[0],
        payee_receipt_account: words[1],
        payee_merchant: Merchant::new("merchant-001", "Test Merchant", None).expect("merchant"),
    }
}

fn suite() -> ConformanceSuite {
    ConformanceSuite::new(
        AdapterId::new("ap2-repair-recovery").expect("suite ID"),
        2,
        Sha256::digest(include_bytes!("fixtures/ap2-mandate-pay-exact.json")).into(),
    )
    .expect("fixture suite")
}

fn new_gateway(trace: &TraceId, now: u64) -> GatewayCore {
    let mut gateway = GatewayCore::new();
    gateway
        .register_adapter(
            ap2_adapter_descriptor(suite()).expect("descriptor"),
            trace,
            now,
        )
        .expect("register recovered AP2 gateway");
    gateway
}

struct Ap2Signer(SigningKey);
impl ReceiptSigner for Ap2Signer {
    fn issuer(&self) -> &str {
        "merchant-api"
    }
    fn key_id(&self) -> &str {
        "receipt"
    }
    fn verifying_key(&self) -> &VerifyingKey {
        self.0.verifying_key()
    }
    fn sign_es256(&mut self, input: &[u8]) -> Result<[u8; 64], Ap2Error> {
        let signature: Signature = self.0.sign(input);
        Ok(signature.to_bytes().into())
    }
}

fn verify_ap2_jwts(
    encoded: &[u8],
    pins: &[layerx_proof::inclusion::SequencerAuthorization],
    digest: [u8; 32],
    case: &Value,
) {
    let saved: Value = serde_json::from_slice(encoded).expect("saved original JWTs");
    for (name, seed) in [("payment", 23), ("checkout", 29)] {
        let (input, signature) = saved[name]
            .as_str()
            .expect("JWT")
            .rsplit_once('.')
            .expect("compact JWT");
        let signature =
            Signature::from_slice(&URL_SAFE_NO_PAD.decode(signature).expect("signature bytes"))
                .expect("ES256 signature");
        signing_key(seed)
            .verifying_key()
            .verify(input.as_bytes(), &signature)
            .expect("original AP2 JWT signature");
        let payload: Value = serde_json::from_slice(
            &URL_SAFE_NO_PAD
                .decode(input.split_once('.').expect("payload").1)
                .expect("payload bytes"),
        )
        .expect("JWT payload");
        assert_eq!(payload["status"], "Success");
        assert_eq!(payload["iss"], "merchant-api");
        let field = if name == "payment" {
            "paymentMandate"
        } else {
            "checkoutMandate"
        };
        let issuer_jwt = case[field]
            .as_str()
            .expect("recorded presentation")
            .split('~')
            .next()
            .expect("issuer JWT");
        assert_eq!(
            payload["reference"],
            URL_SAFE_NO_PAD.encode(Sha256::digest(issuer_jwt.as_bytes()))
        );
        assert_eq!(payload["iat"], NOW + 1);
        assert_eq!(
            payload["layerx_receipt_reference"],
            format!("lxp:{}", URL_SAFE_NO_PAD.encode(digest))
        );
        let portable = PortableReceiptV2::from_json(
            &serde_json::to_vec(&payload["layerx_evidence"]).expect("embedded bytes"),
        )
        .expect("embedded production portable receipt");
        assert_eq!(
            portable
                .verify_v2(pins)
                .expect("embedded proof after recovery")
                .receipt_digest(),
            digest
        );
    }
}

struct SavedState(PathBuf);
impl SavedState {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "ap2-22-33-92-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::create_dir(&path).expect("private recovery directory");
        Self(path)
    }
    fn save(&mut self, name: &str, bytes: &[u8]) {
        let mut file = std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(self.0.join(name))
            .expect("create recovery evidence");
        file.write_all(bytes).expect("persist exact evidence");
        file.sync_all().expect("sync before component stops");
    }
    fn load(&self, name: &str) -> Vec<u8> {
        std::fs::read(self.0.join(name)).expect("reopen evidence")
    }
}
impl Drop for SavedState {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.0).expect("remove recovery directory");
    }
}
