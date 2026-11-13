use std::fs::{self, OpenOptions};
use std::io::Write as _;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite, PinnedSpec, SpecVersion};
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
use layerx_ucp::{
    ucp_adapter_descriptor, verify_sequencer_receipt_with_config, CheckoutStatus,
    CheckoutSubmission, ExecutedUcpPayment, MerchantCurrencyBinding, MerchantProfile,
    NegotiatedCapabilities, OrderMetadata, PaymentHandler, PlatformProfile, UcpAdapter, UcpError,
    UcpIdempotencyKey, UcpPaymentIntent, UcpPaymentPlane, UcpServiceConfig,
    UCP_CHECKOUT_SPEC_SHA256,
};
use serde_json::Value;
use sha2::{Digest as _, Sha256};

const PRODUCER: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../sdk/platform/conformance/fixtures/ucp-checkout-receipt-v1.json"
));

fn bytes(value: &Value) -> Vec<u8> {
    let text = value
        .as_str()
        .or_panic("expected Some value")
        .strip_prefix("0x")
        .or_panic("expected Some value");
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(&text[index..index + 2], 16).or_panic("expected Ok value"))
        .collect()
}

fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value).try_into().or_panic("expected Ok value")
}

static DIRECTORY_SEQUENCE: AtomicU64 = AtomicU64::new(0);

struct CheckoutDirectory(PathBuf);

impl CheckoutDirectory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "ucp-checkout-{}-{}",
            std::process::id(),
            DIRECTORY_SEQUENCE.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).or_panic("expected Ok value");
        Self(path)
    }
}

impl Drop for CheckoutDirectory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).or_panic("expected Ok value");
    }
}

// This peer replays a committed producer transcript; it never manufactures or
// signs receipts. The consumer is UcpAdapter and ExecutedUcpPayment below.
struct RecordedSubmission {
    directory: PathBuf,
}

impl RecordedSubmission {
    fn open(directory: &Path) -> Self {
        assert!(directory.is_dir());
        Self {
            directory: directory.to_owned(),
        }
    }

    fn submit(&self, source: &Value, native_calldata: &[u8]) {
        let path = self.directory.join("submission.json");
        let record = serde_json::json!({
            "producer": source,
            "native_calldata": native_calldata,
        });
        match OpenOptions::new().write(true).create_new(true).open(path) {
            Ok(mut file) => {
                file.write_all(&serde_json::to_vec(&record).or_panic("expected Ok value"))
                    .or_panic("expected Ok value");
                file.sync_all().or_panic("expected Ok value");
                fs::File::open(&self.directory)
                    .or_panic("expected Ok value")
                    .sync_all()
                    .or_panic("expected Ok value");
            }
            Err(error) if error.kind() == std::io::ErrorKind::AlreadyExists => {
                assert_eq!(self.read(), record, "retry must replay the same submission");
            }
            Err(error) => panic!("persist producer transcript: {error}"),
        }
    }

    fn read(&self) -> Value {
        serde_json::from_slice(
            &fs::read(self.directory.join("submission.json")).or_panic("expected Ok value"),
        )
        .or_panic("expected Ok value")
    }

    fn submission_count(&self) -> usize {
        fs::read_dir(&self.directory)
            .or_panic("expected Ok value")
            .count()
    }
}

fn gateway(trace: &TraceId) -> GatewayCore {
    let mut gateway = GatewayCore::new();
    let spec = PinnedSpec::new(
        AdapterId::new("ucp").or_panic("expected Ok value"),
        SpecVersion::parse("20260408").or_panic("expected Ok value"),
        UCP_CHECKOUT_SPEC_SHA256,
    )
    .or_panic("expected Ok value");
    let suite = ConformanceSuite::new(
        AdapterId::new("ucp-producer-checkout").or_panic("expected Ok value"),
        1,
        Sha256::digest(PRODUCER.as_bytes()).into(),
    )
    .or_panic("expected Ok value");
    gateway
        .register_adapter(
            ucp_adapter_descriptor(spec, suite).or_panic("expected Ok value"),
            trace,
            1,
        )
        .or_panic("expected Ok value");
    gateway
}

fn recorded_evidence(source: &Value) -> SequencerEvidence {
    let native = &source["receiptJson"];
    SequencerEvidence {
        transaction_hash: fixed(&native["txHash"]),
        signed_receipt: bytes(&source["signedReceipt"]),
        logs: native["logs"]
            .as_array()
            .or_panic("expected Some value")
            .iter()
            .map(|log| {
                SequencerLog::new(
                    fixed(&log["address"]),
                    log["topics"]
                        .as_array()
                        .or_panic("expected Some value")
                        .iter()
                        .map(fixed)
                        .collect(),
                    bytes(&log["data"]),
                    u64::from_str_radix(
                        log["logIndex"]
                            .as_str()
                            .or_panic("expected Some value")
                            .strip_prefix("0x")
                            .or_panic("expected Some value"),
                        16,
                    )
                    .or_panic("expected Ok value"),
                )
            })
            .collect(),
        rung: SettlementRung::Receipted,
    }
}

// The peer is a committed native producer transcript. Never alter its receipt,
// transaction, topics or signature to make it agree with the checkout.
/// Exercises a checkout against the recorded producer path.
///
/// # Panics
/// Fails when the producer checkout or its recorded fields differ.
pub fn producer_checkout(checkout_id: &str) {
    let recording: Value = serde_json::from_str(PRODUCER).or_panic("expected Ok value");
    let source = recording["cases"]
        .as_array()
        .or_panic("expected Some value")
        .iter()
        .find(|case| case["id"] == "V1-ucp-checkout-pay-exact")
        .or_panic("expected Some value");
    let (evidence, keys, calldata) = recorded_checkout_evidence(source);
    let payment_log = &evidence.logs[0];
    let (checkout, config, native_calldata) = checkout_payment(checkout_id, payment_log, &calldata);
    let directory = CheckoutDirectory::new();
    let store = RecordedSubmission::open(&directory.0);
    store.submit(source, &native_calldata);
    assert_eq!(store.submission_count(), 1);
    let trace = TraceId::mint([0x15; 16]);
    let principal = PrincipalId::new("producer-checkout-buyer").or_panic("expected Ok value");
    let mut gateway = gateway(&trace);
    let mut payment = ExecutedUcpPayment {
        metadata: OrderMetadata {
            order_id: format!("{checkout_id}-order"),
            permalink_url: format!("https://merchant.example/orders/{checkout_id}"),
        },
        evidence: SequencerEvidence {
            transaction_hash: evidence.transaction_hash,
            signed_receipt: Vec::new(),
            logs: Vec::new(),
            rung: SettlementRung::Submitted,
        },
    };
    assert_initial_submission(
        &mut gateway,
        &principal,
        &checkout,
        &config,
        &mut payment,
        &keys,
        &trace,
    );

    let metadata = payment.metadata.clone();
    drop(payment);
    drop(gateway);
    drop(store);
    let store = RecordedSubmission::open(&directory.0);
    let recovered = store.read();
    assert_eq!(recovered["producer"], *source);
    assert_eq!(
        recovered["native_calldata"],
        serde_json::json!(native_calldata.as_slice())
    );
    store.submit(&recovered["producer"], &native_calldata);
    assert_eq!(
        store.submission_count(),
        1,
        "restart must not create another submission"
    );
    let mut gateway = self::gateway(&trace);
    let mut payment = ExecutedUcpPayment {
        metadata,
        evidence: recorded_evidence(&recovered["producer"]),
    };
    let result = UcpAdapter::complete_checkout(
        &mut gateway,
        &principal,
        &checkout,
        &mut config.bind_plane(&mut payment),
        &keys,
        &trace,
        3,
    )
    .map_err(layerx_interop_gateway::trace::Traced::into_error);
    assert!(result.is_ok(), "{checkout_id}: unchanged native producer evidence must complete checkout: {result:?}; producer reference {:?}, checkout reference {:?}", payment_log.topics()[1], checkout.idempotency_key.gateway_key());
    let outcome = result.or_panic("expected Ok value");
    assert_eq!(outcome.status, CheckoutStatus::Completed);
    let order = outcome.order.or_panic("expected Some value");
    let stored = outcome.stored_order.or_panic("expected Some value");
    assert_recovered_order(
        &stored, &order, &evidence, &checkout, source, &keys, &config,
    );
    let retry = UcpAdapter::complete_checkout(
        &mut gateway,
        &principal,
        &checkout,
        &mut config.bind_plane(&mut payment),
        &keys,
        &trace,
        4,
    )
    .or_panic("expected Ok value");
    assert_eq!(retry.order, Some(order));
    assert_eq!(retry.stored_order, Some(stored));
    assert_eq!(payment.evidence, evidence);
    assert_eq!(store.submission_count(), 1);
    assert_eq!(
        &native_calldata[4..],
        &calldata[..192],
        "the signed producer transaction must contain exactly the encoded checkout payment"
    );
}

fn assert_recovered_order(
    stored: &layerx_ucp::StoredOrder,
    order: &layerx_ucp::UcpOrder,
    evidence: &SequencerEvidence,
    checkout: &CheckoutSubmission,
    source: &Value,
    keys: &[SequencerAuthorization],
    config: &UcpServiceConfig,
) {
    assert_eq!(stored.signed_receipt, evidence.signed_receipt);
    assert_eq!(stored.logs, evidence.logs);
    assert_eq!(order.total_minor, checkout.total_minor);
    assert_eq!(order.transaction_hash, evidence.transaction_hash);
    assert_eq!(
        Ok(order.receipt_digest),
        canonical_receipt_digest(&bytes(&source["signedReceipt"]))
    );
    assert_eq!(
        UcpAdapter::read_order_with_config(stored, keys, config),
        Ok(order.clone())
    );
}

fn assert_initial_submission(
    gateway: &mut GatewayCore,
    principal: &PrincipalId,
    checkout: &CheckoutSubmission,
    config: &UcpServiceConfig,
    payment: &mut ExecutedUcpPayment,
    keys: &[SequencerAuthorization],
    trace: &TraceId,
) {
    let submitted = UcpAdapter::complete_checkout(
        gateway,
        principal,
        checkout,
        &mut config.bind_plane(payment),
        keys,
        trace,
        2,
    )
    .or_panic("expected Ok value");
    assert_eq!(submitted.status, CheckoutStatus::CompleteInProgress);
    assert!(submitted.order.is_none());
    assert!(submitted.stored_order.is_none());
    assert_eq!(
        gateway.translation(principal, checkout.idempotency_key.gateway_key()),
        Some(TranslationStatus::Pending)
    );
}

fn recorded_checkout_evidence(
    source: &Value,
) -> (SequencerEvidence, [SequencerAuthorization; 1], Vec<u8>) {
    let native = &source["receiptJson"];
    let key = &source["pinnedKey"];
    let keys = [SequencerAuthorization::new(
        fixed(&key["keyId"]),
        fixed(&key["publicKey"]),
        key["firstHeight"].as_u64().or_panic("expected Some value"),
        key["lastHeight"].as_u64().or_panic("expected Some value"),
    )];
    let evidence = recorded_evidence(source);
    assert_eq!(evidence.signed_receipt, bytes(&native["receipt"]));
    let verified =
        verify_sequencer_signature(&evidence.signed_receipt, &keys).or_panic("expected Ok value");
    assert_eq!(
        verified
            .protocol()
            .or_panic("expected Some value")
            .activity_id(),
        evidence.transaction_hash
    );
    verify_logs(&verified, evidence.logs.clone()).or_panic("expected Ok value");
    assert_eq!(evidence.logs.len(), 1);
    let payment_log = &evidence.logs[0];
    assert_eq!(payment_log.topics().len(), 4);
    let raw = bytes(&source["rawTransaction"]);
    let selectors: Vec<_> = raw
        .windows(4)
        .enumerate()
        .filter_map(|(index, word)| (word == [0x78, 0xea, 0x11, 0xdb]).then_some(index))
        .collect();
    assert_eq!(selectors.len(), 1);
    let calldata = raw[selectors[0] + 4..].to_vec();
    for (word, topic) in [(5, 1), (0, 2), (1, 3)] {
        assert_eq!(
            &calldata[word * 32..(word + 1) * 32],
            payment_log.topics()[topic]
        );
    }
    (evidence, keys, calldata)
}

fn checkout_payment(
    checkout_id: &str,
    payment_log: &SequencerLog,
    calldata: &[u8],
) -> (CheckoutSubmission, UcpServiceConfig, [u8; 196]) {
    let asset: [u8; 32] = payment_log.data()[..32]
        .try_into()
        .or_panic("expected Ok value");
    let atomic = u128::from_be_bytes(
        payment_log.data()[48..64]
            .try_into()
            .or_panic("expected Ok value"),
    );
    let checkout = CheckoutSubmission {
        checkout_id: checkout_id.into(),
        currency: *b"USD",
        total_minor: 100,
        layerx_asset: asset,
        layerx_recipient: payment_log.topics()[3],
        idempotency_key: UcpIdempotencyKey::parse("123e4567-e89b-12d3-a456-426614174000")
            .or_panic("expected Ok value"),
        negotiated: negotiated_checkout_capabilities(),
    };
    let config = UcpServiceConfig::new(
        MerchantCurrencyBinding::new(*b"USD", asset, 2, 6).or_panic("expected Ok value"),
    );
    assert_eq!(config.atomic_amount(&checkout), Ok(atomic));
    assert_ne!(atomic, checkout.total_minor);
    let intent = checkout
        .payment_intent(&config)
        .or_panic("expected Ok value");
    let expires_at =
        u64::from_be_bytes(calldata[152..160].try_into().or_panic("expected Ok value"));
    let native_calldata = intent
        .encode_pay_exact(payment_log.topics()[2], expires_at)
        .or_panic("expected Ok value");
    assert_eq!(
        &native_calldata[164..],
        checkout.idempotency_key.gateway_key()
    );
    assert_eq!(&native_calldata[116..132], atomic.to_be_bytes());
    (checkout, config, native_calldata)
}

/// Checks that a submission remains available after reopening its store.
///
/// # Panics
/// Fails when the reopened store changes or duplicates the submission.
pub fn durable_submission_recovery() {
    let recording: Value = serde_json::from_str(PRODUCER).or_panic("expected Ok value");
    let source = recording["cases"]
        .as_array()
        .or_panic("expected Some value")
        .iter()
        .find(|case| case["id"] == "V1-ucp-checkout-pay-exact")
        .or_panic("expected Some value");
    let directory = CheckoutDirectory::new();
    let store = RecordedSubmission::open(&directory.0);
    let native = bytes(&source["rawTransaction"]);
    store.submit(source, &native);
    let original = store.read();
    drop(store);
    let reopened = RecordedSubmission::open(&directory.0);
    assert_eq!(reopened.read(), original);
    reopened.submit(source, &native);
    assert_eq!(reopened.submission_count(), 1);
    assert_eq!(
        reopened.read()["producer"]["signedReceipt"],
        source["signedReceipt"]
    );
}

/// Compares native checkout encoding with the producer transaction.
///
/// # Panics
/// Fails when encoded calldata differs from the producer transaction.
pub fn native_codec_matches_producer() {
    let recording: Value = serde_json::from_str(PRODUCER).or_panic("expected Ok value");
    let source = recording["cases"]
        .as_array()
        .or_panic("expected Some value")
        .iter()
        .find(|case| case["id"] == "V1-ucp-checkout-pay-exact")
        .or_panic("expected Some value");
    let evidence = recorded_evidence(source);
    let log = &evidence.logs[0];
    let transaction = bytes(&source["rawTransaction"]);
    let selectors: Vec<_> = transaction
        .windows(4)
        .enumerate()
        .filter_map(|(offset, word)| (word == [0x78, 0xea, 0x11, 0xdb]).then_some(offset))
        .collect();
    assert_eq!(selectors.len(), 1);
    let calldata = &transaction[selectors[0]..selectors[0] + 196];
    let intent = UcpPaymentIntent {
        checkout_id: "native-codec-producer-recording".into(),
        currency: *b"USD",
        amount: u128::from_be_bytes(log.data()[48..64].try_into().or_panic("expected Ok value")),
        asset: log.data()[..32].try_into().or_panic("expected Ok value"),
        recipient: log.topics()[3],
        idempotency_key: log.topics()[1],
    };
    let encoded = intent
        .encode_pay_exact(
            log.topics()[2],
            u64::from_be_bytes(calldata[156..164].try_into().or_panic("expected Ok value")),
        )
        .or_panic("expected Ok value");
    assert_eq!(
        encoded.as_slice(),
        calldata,
        "codec must match all producer calldata bytes"
    );
    assert_eq!(
        evidence.signed_receipt,
        bytes(&source["receiptJson"]["receipt"])
    );
}

/// Checks that a different producer reference cannot complete checkout.
///
/// # Panics
/// Fails when the unrelated reference is accepted.
pub fn unrelated_producer_reference_is_refused() {
    let recording: Value = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )))
    .or_panic("expected Ok value");
    let source = recording["cases"]
        .as_array()
        .or_panic("expected Some value")
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .or_panic("expected Some value");
    let evidence = recorded_evidence(source);
    let key = &source["pinnedKey"];
    let keys = [SequencerAuthorization::new(
        fixed(&key["keyId"]),
        fixed(&key["publicKey"]),
        key["firstHeight"].as_u64().or_panic("expected Some value"),
        key["lastHeight"].as_u64().or_panic("expected Some value"),
    )];
    let verified =
        verify_sequencer_signature(&evidence.signed_receipt, &keys).or_panic("expected Ok value");
    verify_logs(&verified, evidence.logs.clone()).or_panic("expected Ok value");
    let log = &evidence.logs[0];
    let mut checkout = CheckoutSubmission {
        checkout_id: "unrelated-producer-reference".into(),
        currency: *b"USD",
        total_minor: 100,
        layerx_asset: log.data()[..32].try_into().or_panic("expected Ok value"),
        layerx_recipient: log.topics()[3],
        idempotency_key: UcpIdempotencyKey::parse("123e4567-e89b-12d3-a456-426614174000")
            .or_panic("expected Ok value"),
        negotiated: negotiated_checkout_capabilities(),
    };
    let config = UcpServiceConfig::new(
        MerchantCurrencyBinding::new(*b"USD", checkout.layerx_asset, 2, 6)
            .or_panic("expected Ok value"),
    );
    assert_eq!(
        config
            .atomic_amount(&checkout)
            .or_panic("expected Ok value"),
        u128::from_be_bytes(log.data()[48..64].try_into().or_panic("expected Ok value"))
    );
    let mut payment = ExecutedUcpPayment {
        metadata: OrderMetadata {
            order_id: "unrelated-producer-order".into(),
            permalink_url: "https://merchant.example/orders/unrelated-producer-order".into(),
        },
        evidence,
    };
    let trace = TraceId::mint([0x16; 16]);
    for uuid in [
        "123e4567-e89b-12d3-a456-426614174000",
        "123e4567-e89b-12d3-a456-426614174001",
    ] {
        checkout.idempotency_key = UcpIdempotencyKey::parse(uuid).or_panic("expected Ok value");
        let intent = checkout
            .payment_intent(&config)
            .or_panic("expected Ok value");
        assert_ne!(intent.idempotency_key, payment.evidence.logs[0].topics()[1]);
        assert_eq!(
            payment
                .execute(&intent, &trace)
                .err()
                .or_panic("expected Err value"),
            UcpError::ReceiptMismatch,
            "the payment plane must reject an unrelated producer reference"
        );
        assert_eq!(
            verify_sequencer_receipt_with_config(
                &checkout,
                &payment.evidence.signed_receipt,
                &keys,
                &config,
            ),
            Err(UcpError::ReceiptMismatch),
            "the signed-receipt consumer must independently enforce checkout binding"
        );
    }
}

fn negotiated_checkout_capabilities() -> NegotiatedCapabilities {
    let handler = PaymentHandler::new(
        "layerx-402",
        "2026-04-08",
        "https://layerx.dev/2026-04-08/specification/402",
        "https://layerx.dev/2026-04-08/schemas/402.json",
    )
    .or_panic("expected Ok value");
    let merchant = MerchantProfile::layerx("https://merchant.example/ucp", handler.clone())
        .or_panic("expected Ok value");
    let platform = PlatformProfile {
        profile_url: "https://buyer.example/profile".into(),
        capabilities: merchant.capabilities().to_vec(),
        payment_handlers: vec![handler.clone()],
    };
    NegotiatedCapabilities::negotiate(&platform, &handler).or_panic("expected Ok value")
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
