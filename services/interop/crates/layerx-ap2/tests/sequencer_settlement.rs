#[path = "support/source_receipt.rs"]
pub mod source_receipt;

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
use layerx_portable::{PortableReceiptV2, PORTABLE_RECEIPT_FORMAT};
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
    "/tests/fixtures/ap2-mandate-pay-exact.json"
));

fn recorded_case() -> Value {
    let fixture: Value = serde_json::from_str(FIXTURE).or_panic("recorded sequencer vectors");
    fixture["cases"]
        .as_array()
        .or_panic("cases")
        .iter()
        .find(|case| case["id"] == "settlement")
        .or_panic("recorded PaymentExact")
        .clone()
}

fn bytes(value: &Value) -> Vec<u8> {
    hex::decode(
        value
            .as_str()
            .or_panic("hex string")
            .strip_prefix("0x")
            .or_panic("hex prefix"),
    )
    .or_panic("hex bytes")
}

fn array<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value).try_into().or_panic("fixed-width field")
}

fn recorded_payment_topics(case: &Value) -> [[u8; 32]; 3] {
    let transaction = bytes(&case["rawTransaction"]);
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

fn recorded_evidence(case: &Value) -> SequencerEvidence {
    SequencerEvidence {
        transaction_hash: array(&case["receiptJson"]["txHash"]),
        signed_receipt: bytes(&case["signedReceipt"]),
        logs: case["receiptJson"]["logs"]
            .as_array()
            .or_panic("logs")
            .iter()
            .map(|log| {
                SequencerLog::new(
                    array(&log["address"]),
                    log["topics"]
                        .as_array()
                        .or_panic("topics")
                        .iter()
                        .map(array)
                        .collect(),
                    bytes(&log["data"]),
                    u64::from_str_radix(
                        log["logIndex"]
                            .as_str()
                            .or_panic("log index")
                            .trim_start_matches("0x"),
                        16,
                    )
                    .or_panic("index"),
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
        key["firstHeight"].as_u64().or_panic("first height"),
        key["lastHeight"].as_u64().or_panic("last height"),
    )
}

use source_receipt::IntentKey as ReceiptKey;

struct MandateKeys;

fn es256_key(seed: u8) -> SigningKey {
    SigningKey::from_bytes((&[seed; 32]).into()).or_panic("valid ES256 test key")
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

fn jws(header: &Value, payload: &Value, key: &SigningKey) -> String {
    let input = format!(
        "{}.{}",
        URL_SAFE_NO_PAD.encode(serde_json::to_vec(header).or_panic("header")),
        URL_SAFE_NO_PAD.encode(serde_json::to_vec(payload).or_panic("payload"))
    );
    let signature: Signature = key.sign(input.as_bytes());
    format!("{input}.{}", URL_SAFE_NO_PAD.encode(signature.to_bytes()))
}

fn mandates() -> (String, String) {
    let checkout = jws(
        &json!({"alg":"ES256","typ":"JWT","kid":"merchant"}),
        &json!({
            "id":"checkout-12345","merchant":{"id":"merchant-001","name":"Test Merchant"},
            "line_items":[{"id":"line-001","item":{"id":"sku-001","title":"Node credit","price":1_000_000},"quantity":1,"totals":[{"type":"total","amount":1_000_000}]}],
            "status":"completed","currency":"USD","totals":[{"type":"total","amount":1_000_000}],"links":[]
        }),
        &es256_key(11),
    );
    let reference = URL_SAFE_NO_PAD.encode(Sha256::digest(checkout.as_bytes()));
    let root = |mandate| {
        format!(
            "{}~",
            jws(
                &json!({"alg":"ES256","typ":"dc+sd-jwt","kid":"issuer"}),
                &json!({"iat":NOW-60,"exp":NOW+3600,"delegate_payload":[mandate]}),
                &es256_key(7)
            )
        )
    };
    (
        root(
            json!({"vct":"mandate.checkout.1","checkout_jwt":checkout,"checkout_hash":reference,"iat":NOW-30,"exp":NOW+1800}),
        ),
        root(
            json!({"vct":"mandate.payment.1","transaction_id":reference,"payee":{"id":"merchant-001","name":"Test Merchant"},"payment_amount":{"amount":1_000_000,"currency":"USD"},"payment_instrument":{"id":"card-001","type":"card"},"iat":NOW-30,"exp":NOW+1800}),
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

fn binding(case: &Value, evidence: &SequencerEvidence) -> LayerXAssetBinding {
    let log = source_receipt::payment_log(evidence);
    LayerXAssetBinding {
        currency: "USD".into(),
        minor_unit_exponent: 2,
        atomic_units_per_minor_unit: 1,
        asset: log.data()[..32].try_into().or_panic("asset"),
        payer_receipt_account: recorded_payment_topics(case)[1],
        payee_receipt_account: recorded_payment_topics(case)[2],
        payee_merchant: Merchant::new("merchant-001", "Test Merchant", None).or_panic("merchant"),
    }
}

struct Ap2Signer(SigningKey);

impl ReceiptSigner for Ap2Signer {
    fn issuer(&self) -> &'static str {
        "merchant-api"
    }
    fn key_id(&self) -> &'static str {
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
            .or_panic("registry");
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
            ActivityType::new(ModuleId::Asset, 5).or_panic("send")
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
        AdapterId::new("ap2-settlement-vectors").or_panic("suite ID"),
        2,
        [0x41; 32],
    )
    .or_panic("suite");
    let mut gateway = GatewayCore::new();
    gateway
        .register_adapter(
            ap2_adapter_descriptor(suite).or_panic("descriptor"),
            trace,
            NOW,
        )
        .or_panic("register AP2");
    gateway
}

fn authorized(
    principal: &PrincipalId,
    pair: &(String, String),
    binding: &LayerXAssetBinding,
) -> AuthorizedPayment {
    let mandates = MandateVerifier::new(&MandateKeys)
        .verify(&pair.0, &pair.1, &context())
        .or_panic("authentic mandates");
    authorize_payment(principal, &mandates, &context(), binding).or_panic("authorized payment")
}

fn signed_payload(jwt: &str, key: &VerifyingKey) -> Value {
    let (input, signature) = jwt.rsplit_once('.').or_panic("signed JWT");
    let signature = Signature::from_slice(
        &URL_SAFE_NO_PAD
            .decode(signature)
            .or_panic("signature bytes"),
    )
    .or_panic("ES256 signature");
    key.verify(input.as_bytes(), &signature)
        .or_panic("authentic AP2 receipt signature");
    serde_json::from_slice(
        &URL_SAFE_NO_PAD
            .decode(input.split_once('.').or_panic("JWT payload").1)
            .or_panic("payload bytes"),
    )
    .or_panic("receipt payload")
}

fn assert_payment_reference(
    case: &Value,
    evidence: &SequencerEvidence,
    payment: &AuthorizedPayment,
) -> [u8; 32] {
    source_receipt::assert_mandate_reference(case, evidence, payment)
}

fn assert_signed_settlement_payloads(
    payment_jwt: &str,
    checkout_jwt: &str,
    payment_signer: &Ap2Signer,
    checkout_signer: &Ap2Signer,
    digest: [u8; 32],
) {
    for (jwt, signer) in [
        (payment_jwt, payment_signer),
        (checkout_jwt, checkout_signer),
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
}

fn assert_recorded_evidence(case: &Value, evidence: &SequencerEvidence) {
    let pins = [pinned_key(case)];
    let receipt = verify_sequencer_signature(&evidence.signed_receipt, pins.as_slice())
        .or_panic("recorded signature");
    verify_logs(&receipt, evidence.logs.clone()).or_panic("recorded logs");
    assert_eq!(
        canonical_receipt_digest(&evidence.signed_receipt),
        Ok(array(&case["receiptDigest"]))
    );
}

fn assert_pending_payment(
    gateway: &GatewayCore,
    principal: &PrincipalId,
    payment: &AuthorizedPayment,
) {
    assert_eq!(
        gateway.translation(principal, payment.idempotency_key()),
        Some(TranslationStatus::Pending)
    );
    assert_eq!(
        gateway
            .principal_audit(principal)
            .or_panic("audit")
            .entries()
            .len(),
        1
    );
}

#[test]
fn ap2_execute_settles_receipted_evidence() {
    let case = recorded_case();
    let evidence = recorded_evidence(&case);
    let binding = binding(&case, &evidence);
    let pair = mandates();
    let principal = PrincipalId::new("buyer").or_panic("principal");
    let payment = authorized(&principal, &pair, &binding);
    let digest = assert_payment_reference(&case, &evidence, &payment);
    let key = ReceiptKey::new(&case);
    assert_eq!(
        &source_receipt::payment_log(&evidence).topics()[1..],
        &[
            payment.idempotency_key(),
            binding.payer_receipt_account,
            binding.payee_receipt_account,
        ]
    );
    assert_ne!(payment.idempotency_key(), binding.payer_receipt_account);
    assert_ne!(payment.idempotency_key(), binding.payee_receipt_account);
    assert_ne!(binding.payer_receipt_account, binding.payee_receipt_account);
    let pins = verified_receipt_pins(&case, &evidence, digest);
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
        .or_panic("AP2 settlement");
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
        .unwrap_or_else(|error| panic!("independently verified portable evidence: {error:?}"));
    assert_eq!(portable.receipt_digest(), digest);
    assert_eq!(portable.signed_receipt(), plane.evidence.signed_receipt);
    assert_signed_settlement_payloads(
        &settled.payment_receipt_jwt,
        &settled.checkout_receipt_jwt,
        &payment_signer,
        &checkout_signer,
        digest,
    );
    let replay = adapter
        .execute(
            &mut gateway,
            &principal,
            &pair.0,
            &pair.1,
            &VerificationContext {
                now: NOW + 1,
                ..context()
            },
            &binding,
            &mut plane,
            &mut payment_signer,
            &mut checkout_signer,
            &trace,
            NOW + 1,
        )
        .or_panic("idempotent replay");
    assert_eq!(
        replay,
        AdapterOutcome::AlreadySettled {
            receipt_digest: digest
        }
    );
    assert_eq!(plane.executions, 1);
}

fn verified_receipt_pins(
    case: &Value,
    evidence: &SequencerEvidence,
    digest: [u8; 32],
) -> [SequencerAuthorization; 1] {
    let recorded_pin = pinned_key(case);
    let pins = [SequencerAuthorization::new(
        recorded_pin.sequencer_id(),
        recorded_pin.public_key(),
        recorded_pin.first_batch_number(),
        recorded_pin.last_batch_number(),
    )];
    let receipt = verify_sequencer_signature(&evidence.signed_receipt, pins.as_slice())
        .or_panic("original node-signed receipt");
    verify_logs(&receipt, evidence.logs.clone()).or_panic("signed PaymentExact log");
    assert_eq!(
        canonical_receipt_digest(&evidence.signed_receipt),
        Ok(digest)
    );
    pins
}

#[test]
fn ap2_portable_evidence_round_trips_as_v2() {
    let case = recorded_case();
    let evidence = recorded_evidence(&case);
    let binding = binding(&case, &evidence);
    let pair = mandates();
    let principal = PrincipalId::new("buyer").or_panic("principal");
    let payment = authorized(&principal, &pair, &binding);
    let digest = assert_payment_reference(&case, &evidence, &payment);
    let key = ReceiptKey::new(&case);
    let recorded_pin = pinned_key(&case);
    let pins = [SequencerAuthorization::new(
        recorded_pin.sequencer_id(),
        recorded_pin.public_key(),
        recorded_pin.first_batch_number(),
        recorded_pin.last_batch_number(),
    )];
    let receipt = verify_sequencer_signature(&evidence.signed_receipt, pins.as_slice())
        .or_panic("authentic sequencer receipt");
    let protocol = receipt.protocol().or_panic("protocol receipt");
    let height = u64::from_be_bytes(
        protocol.batch_id()[24..]
            .try_into()
            .or_panic("receipt height"),
    );
    verify_logs(&receipt, evidence.logs.clone()).or_panic("signed PaymentExact log");
    let mut plane = ReceiptedPlane {
        evidence,
        key,
        executions: 0,
    };
    let trace = TraceId::mint([0x53; 16]);
    let mut gateway = gateway(&trace);
    let mut payment_signer = Ap2Signer(es256_key(23));
    let mut checkout_signer = Ap2Signer(es256_key(29));
    let outcome = Ap2Adapter::new(&MandateKeys, &pins)
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
        .or_panic("AP2 settlement");
    let AdapterOutcome::Settled(settled) = outcome else {
        panic!("expected settled evidence: {outcome:?}");
    };
    let json = settled.layerx.to_json().or_panic("portable JSON export");
    let parsed = PortableReceiptV2::from_json(&json).or_panic("version 2 portable receipt");
    assert_eq!(parsed, settled.layerx);
    assert_eq!(parsed.format(), PORTABLE_RECEIPT_FORMAT);
    assert_eq!(parsed.format(), "layerx-receipt-proof-v2");
    let verified = parsed
        .verify_v2(&pins)
        .or_panic("independent pinned verification");
    assert_eq!(verified.signed_receipt(), plane.evidence.signed_receipt);
    assert_eq!(verified.receipt_digest(), digest);
    assert_eq!(verified.receipt_key(), &pins[0]);
    assert_eq!(verified.height(), height);
    assert_eq!(verified.inclusion_height(), height);
    assert_eq!(settled.receipt_digest, digest);
    assert_eq!(
        gateway.translation(&principal, payment.idempotency_key()),
        Some(TranslationStatus::ReceiptVerified {
            receipt_digest: digest
        })
    );
    for (jwt, signer) in [
        (&settled.payment_receipt_jwt, &payment_signer),
        (&settled.checkout_receipt_jwt, &checkout_signer),
    ] {
        let payload = signed_payload(jwt, signer.verifying_key());
        let json = serde_json::to_vec(&payload["layerx_evidence"])
            .or_panic("embedded portable evidence JSON");
        let embedded = PortableReceiptV2::from_json(&json).or_panic("embedded version 2 evidence");
        assert_eq!(embedded, parsed);
        assert_eq!(
            embedded
                .verify_v2(&pins)
                .or_panic("authentic embedded evidence"),
            verified
        );
    }
    assert_eq!(plane.executions, 1);
}

#[test]
fn ap2_execute_refuses_a_payee_mismatch() {
    let case = recorded_case();
    let evidence = recorded_evidence(&case);
    assert_recorded_evidence(&case, &evidence);
    let pins = [pinned_key(&case)];
    let mut binding = binding(&case, &evidence);
    binding.payee_receipt_account = [0x99; 32];
    assert_ne!(
        binding.payee_receipt_account,
        recorded_payment_topics(&case)[2]
    );
    assert_eq!(
        binding.payer_receipt_account,
        recorded_payment_topics(&case)[1]
    );
    let pair = mandates();
    let principal = PrincipalId::new("buyer").or_panic("principal");
    let payment = authorized(&principal, &pair, &binding);
    let mut plane = ReceiptedPlane {
        evidence,
        key: ReceiptKey::new(&case),
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
        .err()
        .or_panic("wrong payee must be refused");
    assert_eq!(error.error(), &Ap2Error::EvidenceMismatch);
    assert_eq!(error.trace(), &trace);
    assert_pending_payment(&gateway, &principal, &payment);
    let digest = assert_payment_reference(&case, &plane.evidence, &payment);
    assert_eq!(
        source_receipt::payment_log(&plane.evidence).topics()[1],
        payment.idempotency_key()
    );
    let test_pins = [SequencerAuthorization::new(
        pins[0].sequencer_id(),
        pins[0].public_key(),
        pins[0].first_batch_number(),
        pins[0].last_batch_number(),
    )];
    let receipt = verify_sequencer_signature(&plane.evidence.signed_receipt, test_pins.as_slice())
        .or_panic("reference-matched signature");
    verify_logs(&receipt, plane.evidence.logs.clone()).or_panic("reference-matched log");
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
        .err()
        .unwrap_or_else(|| {
            panic!(
                "{}",
                "payee mismatch with matching reference must be refused"
            )
        });
    assert_eq!(error.error(), &Ap2Error::EvidenceMismatch);
    assert_pending_payment(&gateway, &principal, &payment);
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
