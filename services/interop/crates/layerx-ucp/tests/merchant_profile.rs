use ed25519_dalek::{Signer as _, SigningKey};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite, PinnedSpec, SpecVersion};
use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::sequencer_logs::SequencerLog;
use layerx_ucp::{
    ucp_adapter_descriptor, Capability, CheckoutStatus, CheckoutSubmission, ExecutedUcpPayment,
    MerchantProfile, NegotiatedCapabilities, OrderMetadata, PaymentHandler, PlatformProfile,
    StoredOrder, UcpAdapter, UcpError, UcpIdempotencyKey, UCP_CHECKOUT_SPEC_SHA256,
};
use sha2::{Digest as _, Sha256};
use std::fmt::Write as _;

const UCP_VERSION: &str = "2026-04-08";
const CHECKOUT_CAPABILITY: &str = "dev.ucp.shopping.checkout";
const ORDER_CAPABILITY: &str = "dev.ucp.shopping.order";
const PAYMENT_HANDLER_ID: &str = "layerx-402";
const PAYMENT_HANDLER_VERSION: &str = UCP_VERSION;
const PAYMENT_HANDLER_SPEC: &str = "https://layerx.dev/2026-04-08/specification/402";
const PAYMENT_HANDLER_SCHEMA: &str = "https://layerx.dev/2026-04-08/schemas/402.json";

fn registered_gateway(trace: &TraceId) -> GatewayCore {
    let mut gateway = layerx_interop_gateway::interop_gateway_core();
    let adapter_id = AdapterId::new("ucp").unwrap_or_else(|error| panic!("adapter id: {error}"));
    let version = SpecVersion::parse("20260408").unwrap_or_else(|error| panic!("version: {error}"));
    let spec = PinnedSpec::new(adapter_id, version, UCP_CHECKOUT_SPEC_SHA256)
        .unwrap_or_else(|error| panic!("spec: {error}"));
    let suite_id = AdapterId::new("ucp-2026-04-08-vectors")
        .unwrap_or_else(|error| panic!("suite id: {error}"));
    let conformance = ConformanceSuite::new(suite_id, 3, [0xeb; 32])
        .unwrap_or_else(|error| panic!("conformance: {error}"));
    let descriptor = ucp_adapter_descriptor(spec, conformance)
        .unwrap_or_else(|error| panic!("descriptor: {error}"));
    gateway
        .register_adapter(descriptor, trace, 0)
        .unwrap_or_else(|error| panic!("register ucp: {error}"));
    gateway
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().fold(String::new(), |mut output, byte| {
        write!(&mut output, "{byte:02x}").unwrap_or_else(|error| panic!("hex encoding: {error}"));
        output
    })
}

fn receipt_keys(seed: [u8; 32]) -> [SequencerAuthorization; 1] {
    [SequencerAuthorization::new(
        [0x6b; 32],
        SigningKey::from_bytes(&seed).verifying_key().to_bytes(),
        1000,
        1999,
    )]
}

fn fixture_bytes(value: &serde_json::Value) -> Vec<u8> {
    let encoded = value
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .unwrap_or_else(|| panic!("missing fixture hex"));
    assert_eq!(encoded.len() % 2, 0);
    (0..encoded.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&encoded[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("fixture hex: {error}"))
        })
        .collect()
}

fn fixture_array<const N: usize>(value: &serde_json::Value) -> [u8; N] {
    fixture_bytes(value)
        .try_into()
        .unwrap_or_else(|_| panic!("expected {N} fixture bytes"))
}

fn replace_signed_field(unsigned: &mut [u8], before: [u8; 32], after: [u8; 32]) {
    let offsets: Vec<usize> = unsigned
        .windows(32)
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == before).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "one signed field to replace");
    unsigned[offsets[0]..offsets[0] + 32].copy_from_slice(&after);
}

fn signed_receipt(
    idempotency_key: [u8; 32],
    amount: u128,
    asset: [u8; 32],
    recipient: [u8; 32],
    sequencer_seed: [u8; 32],
) -> SequencerEvidence {
    let fixture: serde_json::Value = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )))
    .unwrap_or_else(|error| panic!("sequencer fixture: {error}"));
    let case = fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("missing cases"))
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .unwrap_or_else(|| panic!("missing pay-exact fixture"));
    let log = &case["receiptJson"]["logs"][0];
    let mut topics: Vec<[u8; 32]> = log["topics"]
        .as_array()
        .unwrap_or_else(|| panic!("missing topics"))
        .iter()
        .map(fixture_array)
        .collect();
    let signed = fixture_bytes(&case["signedReceipt"]);
    let mut unsigned = signed[..signed.len() - 69].to_vec();
    replace_signed_field(&mut unsigned, topics[2], recipient);
    replace_signed_field(&mut unsigned, topics[3], idempotency_key);
    topics[2] = recipient;
    topics[3] = idempotency_key;
    let mut data = vec![0; 64];
    data[..32].copy_from_slice(&asset);
    data[48..].copy_from_slice(&amount.to_be_bytes());
    replace_signed_field(
        &mut unsigned,
        Sha256::digest(fixture_bytes(&log["data"])).into(),
        Sha256::digest(&data).into(),
    );
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(&unsigned);
    digest.update([0]);
    let signature = SigningKey::from_bytes(&sequencer_seed)
        .sign(&digest.finalize())
        .to_bytes();
    unsigned.extend_from_slice(&[1, 0, 0, 0, 64]);
    unsigned.extend_from_slice(&signature);
    SequencerEvidence {
        transaction_hash: fixture_array(&case["receiptJson"]["txHash"]),
        signed_receipt: unsigned,
        logs: vec![SequencerLog::new(
            fixture_array(&log["address"]),
            topics,
            data,
            0,
        )],
        rung: SettlementRung::Receipted,
    }
}

fn checkout_payment(submission: &CheckoutSubmission, seed: [u8; 32]) -> ExecutedUcpPayment {
    let key = submission.idempotency_key.gateway_key();
    ExecutedUcpPayment {
        metadata: OrderMetadata {
            order_id: format!("ord_{}", hex(&key[..8])),
            permalink_url: format!("https://merchant.example/orders/{}", hex(&key[..8])),
        },
        evidence: signed_receipt(
            key,
            submission.total_minor,
            submission.layerx_asset,
            submission.layerx_recipient,
            seed,
        ),
    }
}

#[test]
fn merchant_profile_publishes_checkout_and_order_capabilities() {
    let handler = PaymentHandler::new(
        PAYMENT_HANDLER_ID,
        PAYMENT_HANDLER_VERSION,
        PAYMENT_HANDLER_SPEC,
        PAYMENT_HANDLER_SCHEMA,
    )
    .unwrap_or_else(|error| panic!("payment handler: {error}"));

    let profile = MerchantProfile::layerx("https://merchant.example/ucp", handler)
        .unwrap_or_else(|error| panic!("merchant profile: {error}"));

    assert_eq!(profile.version(), UCP_VERSION);
    assert_eq!(profile.rest().endpoint(), "https://merchant.example/ucp");
    assert_eq!(profile.capabilities().len(), 2);

    let checkout = profile
        .capabilities()
        .iter()
        .find(|c| c.name() == CHECKOUT_CAPABILITY)
        .unwrap_or_else(|| panic!("checkout capability missing"));
    assert_eq!(checkout.version(), UCP_VERSION);
    assert!(checkout.spec().starts_with("https://ucp.dev/"));
    assert!(checkout.schema().starts_with("https://ucp.dev/"));

    let order = profile
        .capabilities()
        .iter()
        .find(|c| c.name() == ORDER_CAPABILITY)
        .unwrap_or_else(|| panic!("order capability missing"));
    assert_eq!(order.version(), UCP_VERSION);
}

#[test]
fn capability_negotiation_requires_exact_checkout_match() {
    let merchant_handler = PaymentHandler::new(
        PAYMENT_HANDLER_ID,
        PAYMENT_HANDLER_VERSION,
        PAYMENT_HANDLER_SPEC,
        PAYMENT_HANDLER_SCHEMA,
    )
    .unwrap_or_else(|error| panic!("merchant handler: {error}"));

    let platform = PlatformProfile {
        profile_url: "https://platform.example/profile".to_owned(),
        capabilities: vec![Capability::new(
            CHECKOUT_CAPABILITY,
            UCP_VERSION,
            "https://ucp.dev/checkout",
            "https://ucp.dev/checkout.json",
        )
        .unwrap_or_else(|error| panic!("checkout capability: {error}"))],
        payment_handlers: vec![PaymentHandler::new(
            PAYMENT_HANDLER_ID,
            PAYMENT_HANDLER_VERSION,
            PAYMENT_HANDLER_SPEC,
            PAYMENT_HANDLER_SCHEMA,
        )
        .unwrap_or_else(|error| panic!("platform handler: {error}"))],
    };

    let negotiated = NegotiatedCapabilities::negotiate(&platform, &merchant_handler)
        .unwrap_or_else(|error| panic!("negotiation: {error}"));
    assert!(negotiated.checkout());
    assert!(!negotiated.order());

    let unsupported_version = PlatformProfile {
        profile_url: "https://platform.example/profile".to_owned(),
        capabilities: vec![Capability::new(
            CHECKOUT_CAPABILITY,
            "2025-01-01",
            "https://ucp.dev/old",
            "https://ucp.dev/old.json",
        )
        .unwrap_or_else(|error| panic!("old capability: {error}"))],
        payment_handlers: vec![merchant_handler.clone()],
    };

    let refused = NegotiatedCapabilities::negotiate(&unsupported_version, &merchant_handler);
    assert!(matches!(refused, Err(UcpError::CapabilityUnavailable)));
}

#[test]
fn checkout_completion_requires_receipt_verification() {
    let trace = TraceId::mint([0xcc; 16]);
    let mut gateway = registered_gateway(&trace);
    let principal =
        PrincipalId::new("merchant").unwrap_or_else(|error| panic!("principal: {error}"));

    let asset = [0xc1; 32];
    let recipient = [0xd1; 32];
    let idempotency_key = UcpIdempotencyKey::parse("12345678-1234-5678-1234-567812345678")
        .unwrap_or_else(|error| panic!("idempotency key: {error}"));

    let handler = PaymentHandler::new(
        PAYMENT_HANDLER_ID,
        PAYMENT_HANDLER_VERSION,
        PAYMENT_HANDLER_SPEC,
        PAYMENT_HANDLER_SCHEMA,
    )
    .unwrap_or_else(|error| panic!("handler: {error}"));

    let platform = PlatformProfile {
        profile_url: "https://platform.example/profile".to_owned(),
        capabilities: vec![Capability::new(
            CHECKOUT_CAPABILITY,
            UCP_VERSION,
            "https://ucp.dev/checkout",
            "https://ucp.dev/checkout.json",
        )
        .unwrap_or_else(|error| panic!("checkout: {error}"))],
        payment_handlers: vec![handler.clone()],
    };

    let negotiated = NegotiatedCapabilities::negotiate(&platform, &handler)
        .unwrap_or_else(|error| panic!("negotiation: {error}"));

    let submission = CheckoutSubmission {
        checkout_id: "chk_test123".to_owned(),
        currency: *b"USD",
        total_minor: 9999,
        layerx_asset: asset,
        layerx_recipient: recipient,
        idempotency_key,
        negotiated,
    };

    let sequencer_seed = [0x55; 32];
    let mut plane = checkout_payment(&submission, sequencer_seed);

    let outcome = UcpAdapter::complete_checkout(
        &mut gateway,
        &principal,
        &submission,
        &mut plane,
        &receipt_keys(sequencer_seed),
        &trace,
        10,
    )
    .unwrap_or_else(|error| panic!("checkout completion: {error}"));

    assert_eq!(outcome.status, CheckoutStatus::Completed);
    assert!(outcome.order.is_some());

    let order = outcome
        .order
        .unwrap_or_else(|| panic!("completed checkout order is missing"));
    assert_eq!(order.checkout_id, "chk_test123");
    assert_eq!(order.currency, *b"USD");
    assert_eq!(order.total_minor, 9999);
    assert_ne!(order.receipt_digest, [0; 32]);
}

#[test]
fn order_state_remains_receipt_backed_or_honestly_pending() {
    let trace = TraceId::mint([0xdd; 16]);
    let mut gateway = registered_gateway(&trace);
    let principal =
        PrincipalId::new("merchant").unwrap_or_else(|error| panic!("principal: {error}"));

    let asset = [0xc2; 32];
    let recipient = [0xd2; 32];
    let idempotency_key = UcpIdempotencyKey::parse("87654321-4321-8765-4321-876543218765")
        .unwrap_or_else(|error| panic!("idempotency key: {error}"));

    let handler = PaymentHandler::new(
        PAYMENT_HANDLER_ID,
        PAYMENT_HANDLER_VERSION,
        PAYMENT_HANDLER_SPEC,
        PAYMENT_HANDLER_SCHEMA,
    )
    .unwrap_or_else(|error| panic!("handler: {error}"));

    let platform = PlatformProfile {
        profile_url: "https://platform.example/profile".to_owned(),
        capabilities: vec![Capability::new(
            CHECKOUT_CAPABILITY,
            UCP_VERSION,
            "https://ucp.dev/checkout",
            "https://ucp.dev/checkout.json",
        )
        .unwrap_or_else(|error| panic!("checkout: {error}"))],
        payment_handlers: vec![handler.clone()],
    };

    let negotiated = NegotiatedCapabilities::negotiate(&platform, &handler)
        .unwrap_or_else(|error| panic!("negotiation: {error}"));

    let submission = CheckoutSubmission {
        checkout_id: "chk_pending".to_owned(),
        currency: *b"EUR",
        total_minor: 5000,
        layerx_asset: asset,
        layerx_recipient: recipient,
        idempotency_key,
        negotiated,
    };

    let mut plane = checkout_payment(&submission, [0x55; 32]);
    plane.evidence.rung = SettlementRung::Submitted;

    let outcome = UcpAdapter::complete_checkout(
        &mut gateway,
        &principal,
        &submission,
        &mut plane,
        &[],
        &trace,
        20,
    )
    .unwrap_or_else(|error| panic!("checkout pending: {error}"));

    assert_eq!(outcome.status, CheckoutStatus::CompleteInProgress);
    assert!(
        outcome.order.is_none(),
        "pending checkout must not return an order"
    );
}

#[test]
fn order_read_verifies_stored_receipt_on_every_access() {
    let asset = [0xc3; 32];
    let recipient = [0xd3; 32];
    let idempotency_key = UcpIdempotencyKey::parse("abcdef01-2345-6789-abcd-ef0123456789")
        .unwrap_or_else(|error| panic!("idempotency key: {error}"));

    let handler = PaymentHandler::new(
        PAYMENT_HANDLER_ID,
        PAYMENT_HANDLER_VERSION,
        PAYMENT_HANDLER_SPEC,
        PAYMENT_HANDLER_SCHEMA,
    )
    .unwrap_or_else(|error| panic!("handler: {error}"));

    let platform = PlatformProfile {
        profile_url: "https://platform.example/profile".to_owned(),
        capabilities: vec![
            Capability::new(
                CHECKOUT_CAPABILITY,
                UCP_VERSION,
                "https://ucp.dev/checkout",
                "https://ucp.dev/checkout.json",
            )
            .unwrap_or_else(|error| panic!("checkout: {error}")),
            Capability::new(
                ORDER_CAPABILITY,
                UCP_VERSION,
                "https://ucp.dev/order",
                "https://ucp.dev/order.json",
            )
            .unwrap_or_else(|error| panic!("order: {error}")),
        ],
        payment_handlers: vec![handler.clone()],
    };

    let negotiated = NegotiatedCapabilities::negotiate(&platform, &handler)
        .unwrap_or_else(|error| panic!("negotiation: {error}"));

    let submission = CheckoutSubmission {
        checkout_id: "chk_order_read".to_owned(),
        currency: *b"GBP",
        total_minor: 12500,
        layerx_asset: asset,
        layerx_recipient: recipient,
        idempotency_key,
        negotiated,
    };

    let sequencer_seed = [0x77; 32];
    let evidence = signed_receipt(
        idempotency_key.gateway_key(),
        submission.total_minor,
        asset,
        recipient,
        sequencer_seed,
    );

    let metadata = OrderMetadata {
        order_id: "ord_read_test".to_owned(),
        permalink_url: "https://merchant.example/orders/ord_read_test".to_owned(),
    };

    let stored = StoredOrder {
        submission,
        metadata,
        transaction_hash: evidence.transaction_hash,
        signed_receipt: evidence.signed_receipt,
        logs: evidence.logs,
        rung: evidence.rung,
    };

    let order = UcpAdapter::read_order(&stored, &receipt_keys(sequencer_seed))
        .unwrap_or_else(|error| panic!("order read: {error}"));

    assert_eq!(order.id, "ord_read_test");
    assert_eq!(order.checkout_id, "chk_order_read");
    assert_eq!(order.currency, *b"GBP");
    assert_eq!(order.total_minor, 12500);
    assert_ne!(order.receipt_digest, [0; 32]);
}

#[test]
fn payment_handler_mismatch_refuses_negotiation() {
    let merchant_handler = PaymentHandler::new(
        PAYMENT_HANDLER_ID,
        PAYMENT_HANDLER_VERSION,
        PAYMENT_HANDLER_SPEC,
        PAYMENT_HANDLER_SCHEMA,
    )
    .unwrap_or_else(|error| panic!("merchant handler: {error}"));

    let different_handler = PaymentHandler::new(
        "different-handler",
        PAYMENT_HANDLER_VERSION,
        PAYMENT_HANDLER_SPEC,
        PAYMENT_HANDLER_SCHEMA,
    )
    .unwrap_or_else(|error| panic!("different handler: {error}"));

    let platform = PlatformProfile {
        profile_url: "https://platform.example/profile".to_owned(),
        capabilities: vec![Capability::new(
            CHECKOUT_CAPABILITY,
            UCP_VERSION,
            "https://ucp.dev/checkout",
            "https://ucp.dev/checkout.json",
        )
        .unwrap_or_else(|error| panic!("checkout: {error}"))],
        payment_handlers: vec![different_handler],
    };

    let refused = NegotiatedCapabilities::negotiate(&platform, &merchant_handler);
    assert!(matches!(refused, Err(UcpError::PaymentHandlerUnavailable)));
}
