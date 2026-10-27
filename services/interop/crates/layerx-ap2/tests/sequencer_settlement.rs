use std::path::PathBuf;
use std::process::Command;
use std::sync::atomic::{AtomicU64, Ordering};

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use layerx_ap2::{
    ap2_adapter_descriptor, authorize_payment, AdapterOutcome, Ap2Adapter, Ap2Error,
    AuthorizedPayment, ExecutedPayment, KeyResolver, KeyUse, LayerXAssetBinding, LayerXIntentPlane,
    MandateVerifier, Merchant, PlaneOutcome, ProtectedHeader, ReceiptSigner, VerificationContext,
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
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
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
const FIXTURE: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
));

fn recorded_case() -> Value {
    let fixture: Value = serde_json::from_str(FIXTURE).expect("recorded sequencer vectors");
    fixture["cases"]
        .as_array()
        .expect("cases")
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .expect("recorded PaymentExact")
        .clone()
}

fn bytes(value: &Value) -> Vec<u8> {
    hex::decode(
        value
            .as_str()
            .expect("hex string")
            .strip_prefix("0x")
            .expect("hex prefix"),
    )
    .expect("hex bytes")
}

fn array<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value).try_into().expect("fixed-width field")
}

fn recorded_evidence(case: &Value) -> SequencerEvidence {
    SequencerEvidence {
        transaction_hash: array(&case["receiptJson"]["txHash"]),
        signed_receipt: bytes(&case["signedReceipt"]),
        logs: case["receiptJson"]["logs"]
            .as_array()
            .expect("logs")
            .iter()
            .map(|log| {
                SequencerLog::new(
                    array(&log["address"]),
                    log["topics"]
                        .as_array()
                        .expect("topics")
                        .iter()
                        .map(array)
                        .collect(),
                    bytes(&log["data"]),
                    u64::from_str_radix(
                        log["logIndex"]
                            .as_str()
                            .expect("log index")
                            .trim_start_matches("0x"),
                        16,
                    )
                    .expect("index"),
                )
            })
            .collect(),
        rung: SettlementRung::Receipted,
    }
}

fn pinned_key(case: &Value) -> SequencerAuthorization {
    let key = &case["pinnedKey"];
    SequencerAuthorization::new(
        array(&key["keyId"]),
        array(&key["publicKey"]),
        key["firstHeight"].as_u64().expect("first height"),
        key["lastHeight"].as_u64().expect("last height"),
    )
}

struct ReceiptKey {
    directory: PathBuf,
    public_key: [u8; 32],
}

impl ReceiptKey {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let directory = std::env::temp_dir().join(format!(
            "ap2-sequencer-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::create_dir(&directory).expect("private test key directory");
        let mut der =
            hex::decode("302e020100300506032b657004220420").expect("Ed25519 PKCS8 prefix");
        der.extend_from_slice(&[0x35; 32]);
        std::fs::write(directory.join("key.der"), der).expect("test-only key");
        let output = Command::new("openssl")
            .args(["pkey", "-inform", "DER", "-in"])
            .arg(directory.join("key.der"))
            .args(["-pubout", "-outform", "DER"])
            .output()
            .expect("openssl public key");
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        assert_eq!(output.stdout.len(), 44);
        let public_key = output.stdout[12..].try_into().expect("Ed25519 public key");
        Self {
            directory,
            public_key,
        }
    }

    fn sign(&self, message: &[u8]) -> [u8; 64] {
        std::fs::write(self.directory.join("message"), message).expect("message to sign");
        let output = Command::new("openssl")
            .args(["pkeyutl", "-sign", "-rawin", "-keyform", "DER", "-inkey"])
            .arg(self.directory.join("key.der"))
            .arg("-in")
            .arg(self.directory.join("message"))
            .output()
            .expect("openssl Ed25519 signer");
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        output.stdout.try_into().expect("Ed25519 signature")
    }
}

impl Drop for ReceiptKey {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.directory).expect("remove test key directory");
    }
}

struct MandateKeys;

fn es256_key(seed: u8) -> SigningKey {
    SigningKey::from_bytes((&[seed; 32]).into()).expect("valid ES256 test key")
}

impl KeyResolver for MandateKeys {
    fn resolve(&self, usage: KeyUse, header: &ProtectedHeader) -> Result<VerifyingKey, Ap2Error> {
        match (usage, header.key_id()) {
            (KeyUse::CheckoutMandateIssuer | KeyUse::PaymentMandateIssuer, Some("issuer")) => {
                Ok(*es256_key(7).verifying_key())
            }
            (KeyUse::MerchantCheckout, Some("merchant")) => Ok(*es256_key(11).verifying_key()),
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

fn mandates() -> (String, String) {
    let checkout = jws(
        json!({"alg":"ES256","typ":"JWT","kid":"merchant"}),
        json!({
            "id":"checkout-12345","merchant":{"id":"merchant-001","name":"Test Merchant"},
            "line_items":[{"id":"line-001","item":{"id":"sku-001","title":"Node credit","price":1000000},"quantity":1,"totals":[{"type":"total","amount":1000000}]}],
            "status":"completed","currency":"USD","totals":[{"type":"total","amount":1000000}],"links":[]
        }),
        &es256_key(11),
    );
    let reference = URL_SAFE_NO_PAD.encode(Sha256::digest(checkout.as_bytes()));
    let root = |mandate| {
        format!(
            "{}~",
            jws(
                json!({"alg":"ES256","typ":"dc+sd-jwt","kid":"issuer"}),
                json!({"iat":NOW-60,"exp":NOW+3600,"delegate_payload":[mandate]}),
                &es256_key(7)
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

fn context() -> VerificationContext<'static> {
    VerificationContext {
        now: NOW,
        clock_skew_seconds: 300,
        expected_audience: "merchant-api",
        expected_nonce: "nonce",
        currency_minor_exponent: 2,
        usage: None,
    }
}

fn binding(evidence: &SequencerEvidence) -> LayerXAssetBinding {
    let log = &evidence.logs[0];
    LayerXAssetBinding {
        currency: "USD".into(),
        minor_unit_exponent: 2,
        atomic_units_per_minor_unit: 1,
        asset: log.data()[..32].try_into().expect("asset"),
        payer_receipt_account: log.topics()[1],
        payee_receipt_account: log.topics()[2],
        payee_merchant: Merchant::new("merchant-001", "Test Merchant", None).expect("merchant"),
    }
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

struct ReceiptedPlane {
    evidence: SequencerEvidence,
    key: ReceiptKey,
    executions: usize,
}

impl LayerXIntentPlane for ReceiptedPlane {
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
            .expect("registry");
        compile(&Intent::v1(IntentKind::LxpSend(send)), &registry)
            .map_err(|_| Ap2Error::IntentCompilation)
    }

    fn execute(
        &mut self,
        _: &AuthorizedPayment,
        compiled: CompiledIntent,
        _: &TraceId,
    ) -> Result<PlaneOutcome, Ap2Error> {
        assert_eq!(
            compiled.activity_type(),
            ActivityType::new(ModuleId::Asset, 5).expect("send")
        );
        self.executions += 1;
        Ok(PlaneOutcome::Executed(Box::new(ExecutedPayment {
            evidence: self.evidence.clone(),
            order_id: "order-12345".into(),
        })))
    }
}

fn gateway(trace: &TraceId) -> GatewayCore {
    let suite = ConformanceSuite::new(
        AdapterId::new("ap2-settlement-vectors").expect("suite ID"),
        2,
        [0x41; 32],
    )
    .expect("suite");
    let mut gateway = GatewayCore::new();
    gateway
        .register_adapter(
            ap2_adapter_descriptor(suite).expect("descriptor"),
            trace,
            NOW,
        )
        .expect("register AP2");
    gateway
}

fn authorized(
    principal: &PrincipalId,
    pair: &(String, String),
    binding: &LayerXAssetBinding,
) -> AuthorizedPayment {
    let mandates = MandateVerifier::new(&MandateKeys)
        .verify(&pair.0, &pair.1, &context())
        .expect("authentic mandates");
    authorize_payment(principal, &mandates, &context(), binding).expect("authorized payment")
}

fn signed_payload(jwt: &str, key: &VerifyingKey) -> Value {
    let (input, signature) = jwt.rsplit_once('.').expect("signed JWT");
    let signature =
        Signature::from_slice(&URL_SAFE_NO_PAD.decode(signature).expect("signature bytes"))
            .expect("ES256 signature");
    key.verify(input.as_bytes(), &signature)
        .expect("authentic AP2 receipt signature");
    serde_json::from_slice(
        &URL_SAFE_NO_PAD
            .decode(input.split_once('.').expect("JWT payload").1)
            .expect("payload bytes"),
    )
    .expect("receipt payload")
}

fn sign_payment_reference(
    evidence: &mut SequencerEvidence,
    payment: &AuthorizedPayment,
    key: &ReceiptKey,
) -> [u8; 32] {
    let old_reference = evidence.logs[0].topics()[3];
    let mut topics = evidence.logs[0].topics().to_vec();
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
    evidence.logs[0] = SequencerLog::new(
        evidence.logs[0].address(),
        topics,
        evidence.logs[0].data().to_vec(),
        0,
    );
    let unsigned_end = evidence.signed_receipt.len() - 69;
    let mut hash = Sha256::new();
    hash.update(b"LXP/v1/receipt\0");
    hash.update(&evidence.signed_receipt[..unsigned_end]);
    hash.update([0]);
    let digest: [u8; 32] = hash.finalize().into();
    evidence.signed_receipt[unsigned_end + 5..].copy_from_slice(&key.sign(&digest));
    digest
}

#[test]
fn ap2_execute_settles_receipted_evidence() {
    let case = recorded_case();
    let mut evidence = recorded_evidence(&case);
    let binding = binding(&evidence);
    let pair = mandates();
    let principal = PrincipalId::new("buyer").expect("principal");
    let payment = authorized(&principal, &pair, &binding);
    let key = ReceiptKey::new();
    let digest = sign_payment_reference(&mut evidence, &payment, &key);
    let recorded_pin = pinned_key(&case);
    let pins = [SequencerAuthorization::new(
        recorded_pin.sequencer_id(),
        key.public_key,
        recorded_pin.first_batch_number(),
        recorded_pin.last_batch_number(),
    )];
    let receipt = verify_sequencer_signature(&evidence.signed_receipt, pins.as_slice())
        .expect("self-signed receipt");
    verify_logs(&receipt, evidence.logs.clone()).expect("signed PaymentExact log");
    assert_eq!(
        canonical_receipt_digest(&evidence.signed_receipt),
        Ok(digest)
    );
    let mut plane = ReceiptedPlane {
        evidence,
        key,
        executions: 0,
    };
    let trace = TraceId::mint([0x51; 16]);
    let mut gateway = gateway(&trace);
    let adapter = Ap2Adapter::new(&MandateKeys, &pins);
    let mut payment_signer = Ap2Signer(es256_key(23));
    let mut checkout_signer = Ap2Signer(es256_key(29));
    let outcome = adapter
        .execute(
            &mut gateway,
            &principal,
            &pair.0,
            &pair.1,
            &context(),
            &binding,
            &mut plane,
            &mut payment_signer,
            &mut checkout_signer,
            &trace,
            NOW,
        )
        .expect("AP2 settlement");
    let AdapterOutcome::Settled(settled) = outcome else {
        panic!("expected settled evidence: {outcome:?}");
    };
    assert_eq!(settled.receipt_digest, digest);
    assert_eq!(
        gateway.translation(&principal, payment.idempotency_key()),
        Some(TranslationStatus::ReceiptVerified {
            receipt_digest: digest
        })
    );
    let portable = settled
        .layerx
        .verify_v2(&pins)
        .expect("independently verified portable evidence");
    assert_eq!(portable.receipt_digest(), digest);
    assert_eq!(portable.signed_receipt(), plane.evidence.signed_receipt);
    for (jwt, signer) in [
        (&settled.payment_receipt_jwt, &payment_signer),
        (&settled.checkout_receipt_jwt, &checkout_signer),
    ] {
        let payload = signed_payload(jwt, signer.verifying_key());
        assert_eq!(payload["status"], "Success");
        assert_eq!(
            payload["layerx_receipt_reference"],
            format!("lxp:{}", URL_SAFE_NO_PAD.encode(digest))
        );
        assert_eq!(
            payload["layerx_evidence"]["receiptDigest"],
            format!("0x{}", hex::encode(digest))
        );
    }
    let replay = adapter
        .execute(
            &mut gateway,
            &principal,
            &pair.0,
            &pair.1,
            &context(),
            &binding,
            &mut plane,
            &mut payment_signer,
            &mut checkout_signer,
            &trace,
            NOW + 1,
        )
        .expect("idempotent replay");
    assert_eq!(
        replay,
        AdapterOutcome::AlreadySettled {
            receipt_digest: digest
        }
    );
    assert_eq!(plane.executions, 1);
}

#[test]
fn ap2_execute_refuses_a_payee_mismatch() {
    let case = recorded_case();
    let evidence = recorded_evidence(&case);
    let pins = [pinned_key(&case)];
    let receipt = verify_sequencer_signature(&evidence.signed_receipt, pins.as_slice())
        .expect("recorded signature");
    verify_logs(&receipt, evidence.logs.clone()).expect("recorded logs");
    assert_eq!(
        canonical_receipt_digest(&evidence.signed_receipt),
        Ok(array(&case["receiptDigest"]))
    );
    let mut binding = binding(&evidence);
    binding.payee_receipt_account = [0x99; 32];
    assert_ne!(binding.payee_receipt_account, evidence.logs[0].topics()[2]);
    assert_eq!(binding.payer_receipt_account, evidence.logs[0].topics()[1]);
    let pair = mandates();
    let principal = PrincipalId::new("buyer").expect("principal");
    let payment = authorized(&principal, &pair, &binding);
    let mut plane = ReceiptedPlane {
        evidence,
        key: ReceiptKey::new(),
        executions: 0,
    };
    let trace = TraceId::mint([0x52; 16]);
    let mut gateway = gateway(&trace);
    let mut payment_signer = Ap2Signer(es256_key(23));
    let mut checkout_signer = Ap2Signer(es256_key(29));
    let error = Ap2Adapter::new(&MandateKeys, &pins)
        .execute(
            &mut gateway,
            &principal,
            &pair.0,
            &pair.1,
            &context(),
            &binding,
            &mut plane,
            &mut payment_signer,
            &mut checkout_signer,
            &trace,
            NOW,
        )
        .expect_err("wrong payee must be refused");
    assert_eq!(error.error(), &Ap2Error::EvidenceMismatch);
    assert_eq!(error.trace(), &trace);
    assert_eq!(
        gateway.translation(&principal, payment.idempotency_key()),
        Some(TranslationStatus::Pending)
    );
    assert_eq!(
        gateway
            .principal_audit(&principal)
            .expect("audit")
            .entries()
            .len(),
        1
    );
    let digest = sign_payment_reference(&mut plane.evidence, &payment, &plane.key);
    assert_eq!(
        plane.evidence.logs[0].topics()[3],
        payment.idempotency_key()
    );
    let test_pins = [SequencerAuthorization::new(
        pins[0].sequencer_id(),
        plane.key.public_key,
        pins[0].first_batch_number(),
        pins[0].last_batch_number(),
    )];
    let receipt = verify_sequencer_signature(&plane.evidence.signed_receipt, test_pins.as_slice())
        .expect("reference-matched signature");
    verify_logs(&receipt, plane.evidence.logs.clone()).expect("reference-matched log");
    assert_eq!(
        canonical_receipt_digest(&plane.evidence.signed_receipt),
        Ok(digest)
    );
    let error = Ap2Adapter::new(&MandateKeys, &test_pins)
        .execute(
            &mut gateway,
            &principal,
            &pair.0,
            &pair.1,
            &context(),
            &binding,
            &mut plane,
            &mut payment_signer,
            &mut checkout_signer,
            &trace,
            NOW + 1,
        )
        .expect_err("payee mismatch with matching reference must be refused");
    assert_eq!(error.error(), &Ap2Error::EvidenceMismatch);
    assert_eq!(
        gateway.translation(&principal, payment.idempotency_key()),
        Some(TranslationStatus::Pending)
    );
    assert_eq!(
        gateway
            .principal_audit(&principal)
            .expect("audit")
            .entries()
            .len(),
        1
    );
}
