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
use std::collections::HashMap;
use std::fmt::Write as _;

const UCP_VERSION: &str = "2026-04-08";
const CHECKOUT_CAPABILITY: &str = "dev.ucp.shopping.checkout";
const ORDER_CAPABILITY: &str = "dev.ucp.shopping.order";

fn registered_gateway(trace: &TraceId) -> Result<GatewayCore, UcpError> {
    let mut gateway = layerx_interop_gateway::interop_gateway_core();
    let adapter_id = AdapterId::new("ucp").map_err(|error| UcpError::Gateway(error.into()))?;
    let version =
        SpecVersion::parse("20260408").map_err(|error| UcpError::Gateway(error.into()))?;
    let spec = PinnedSpec::new(adapter_id, version, UCP_CHECKOUT_SPEC_SHA256)
        .map_err(|error| UcpError::Gateway(error.into()))?;
    let suite_id = AdapterId::new("ucp-2026-04-08-vectors")
        .map_err(|error| UcpError::Gateway(error.into()))?;
    let conformance = ConformanceSuite::new(suite_id, 3, [0xeb; 32])
        .map_err(|error| UcpError::Gateway(error.into()))?;
    let descriptor = ucp_adapter_descriptor(spec, conformance)?;
    gateway
        .register_adapter(descriptor, trace, 0)
        .map_err(|error| UcpError::Gateway(error.into_error()))?;
    Ok(gateway)
}

struct UcpClient {
    profile_url: String,
    capabilities: Vec<Capability>,
    payment_handlers: Vec<PaymentHandler>,
}

impl UcpClient {
    fn new() -> Result<Self, UcpError> {
        Ok(Self {
            profile_url: "https://buyer-platform.example/ucp-profile".to_owned(),
            capabilities: vec![
                Capability::new(
                    CHECKOUT_CAPABILITY,
                    UCP_VERSION,
                    "https://ucp.dev/2026-04-08/specification/checkout",
                    "https://ucp.dev/2026-04-08/schemas/shopping/checkout.json",
                )?,
                Capability::new(
                    ORDER_CAPABILITY,
                    UCP_VERSION,
                    "https://ucp.dev/2026-04-08/specification/order",
                    "https://ucp.dev/2026-04-08/schemas/shopping/order.json",
                )?,
            ],
            payment_handlers: vec![PaymentHandler::new(
                "layerx-402",
                UCP_VERSION,
                "https://layerx.dev/2026-04-08/specification/402",
                "https://layerx.dev/2026-04-08/schemas/402.json",
            )?],
        })
    }

    fn platform_profile(&self) -> PlatformProfile {
        PlatformProfile {
            profile_url: self.profile_url.clone(),
            capabilities: self.capabilities.clone(),
            payment_handlers: self.payment_handlers.clone(),
        }
    }
}

struct LayerXSeller {
    gateway: GatewayCore,
    principal: PrincipalId,
    merchant_profile: MerchantProfile,
    orders: HashMap<String, StoredOrder>,
}

impl LayerXSeller {
    fn new() -> Result<Self, UcpError> {
        let handler = PaymentHandler::new(
            "layerx-402",
            UCP_VERSION,
            "https://layerx.dev/2026-04-08/specification/402",
            "https://layerx.dev/2026-04-08/schemas/402.json",
        )?;

        let merchant_profile =
            MerchantProfile::layerx("https://merchant.example/ucp-rest", handler)?;

        let principal =
            PrincipalId::new("seller-merchant").map_err(|_| UcpError::InvalidProfile)?;

        let trace = TraceId::mint([0xcc; 16]);
        Ok(Self {
            gateway: registered_gateway(&trace)?,
            principal,
            merchant_profile,
            orders: HashMap::new(),
        })
    }

    fn process_checkout(
        &mut self,
        submission: CheckoutSubmission,
        payment: &mut ExecutedUcpPayment,
        trace: &TraceId,
        now: u64,
    ) -> Result<CheckoutStatus, UcpError> {
        let outcome = UcpAdapter::complete_checkout(
            &mut self.gateway,
            &self.principal,
            &submission,
            payment,
            &receipt_keys([0x88; 32]),
            trace,
            now,
        )
        .map_err(layerx_interop_gateway::trace::Traced::into_error)?;

        if let Some(stored) = outcome.stored_order {
            self.orders.insert(stored.metadata.order_id.clone(), stored);
        }

        Ok(outcome.status)
    }

    fn get_order(&self, order_id: &str) -> Result<layerx_ucp::UcpOrder, UcpError> {
        let stored = self.orders.get(order_id).ok_or(UcpError::InvalidOrder)?;

        UcpAdapter::read_order(stored, &receipt_keys([0x88; 32]))
    }
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
fn ucp_client_completes_checkout_with_layerx_seller() {
    let client = UcpClient::new().unwrap_or_else(|error| panic!("client setup: {error}"));
    let mut seller = LayerXSeller::new().unwrap_or_else(|error| panic!("seller setup: {error}"));

    let merchant_handler = seller.merchant_profile.payment_handlers()[0].clone();
    let negotiated =
        NegotiatedCapabilities::negotiate(&client.platform_profile(), &merchant_handler)
            .unwrap_or_else(|error| panic!("capability negotiation: {error}"));

    assert!(
        negotiated.checkout(),
        "client and seller must agree on checkout capability"
    );

    let asset = [0xe1; 32];
    let recipient = [0xe2; 32];
    let idempotency_key = UcpIdempotencyKey::parse("aaaabbbb-cccc-dddd-eeee-ffff00001111")
        .unwrap_or_else(|error| panic!("idempotency key: {error}"));

    let submission = CheckoutSubmission {
        checkout_id: "chk_e2e_test".to_owned(),
        currency: *b"USD",
        total_minor: 25000,
        layerx_asset: asset,
        layerx_recipient: recipient,
        idempotency_key,
        negotiated,
    };

    let mut payment = checkout_payment(&submission, [0x88; 32]);
    payment.evidence.rung = SettlementRung::Submitted;
    let trace = TraceId::mint([0xe3; 16]);

    let first_attempt = seller
        .process_checkout(submission.clone(), &mut payment, &trace, 100)
        .unwrap_or_else(|error| panic!("first checkout attempt: {error}"));
    assert_eq!(
        first_attempt,
        CheckoutStatus::CompleteInProgress,
        "first attempt must be pending"
    );

    payment.evidence.rung = SettlementRung::Receipted;
    let second_attempt = seller
        .process_checkout(submission.clone(), &mut payment, &trace, 101)
        .unwrap_or_else(|error| panic!("second checkout attempt: {error}"));
    assert_eq!(
        second_attempt,
        CheckoutStatus::Completed,
        "second attempt must complete with receipt"
    );
}

#[test]
fn ucp_client_retrieves_order_with_verified_receipt() {
    let client = UcpClient::new().unwrap_or_else(|error| panic!("client setup: {error}"));
    let mut seller = LayerXSeller::new().unwrap_or_else(|error| panic!("seller setup: {error}"));

    let merchant_handler = seller.merchant_profile.payment_handlers()[0].clone();
    let negotiated =
        NegotiatedCapabilities::negotiate(&client.platform_profile(), &merchant_handler)
            .unwrap_or_else(|error| panic!("negotiation: {error}"));

    let asset = [0xf1; 32];
    let recipient = [0xf2; 32];
    let idempotency_key = UcpIdempotencyKey::parse("11112222-3333-4444-5555-666677778888")
        .unwrap_or_else(|error| panic!("idempotency key: {error}"));

    let submission = CheckoutSubmission {
        checkout_id: "chk_order_retrieve".to_owned(),
        currency: *b"EUR",
        total_minor: 15000,
        layerx_asset: asset,
        layerx_recipient: recipient,
        idempotency_key,
        negotiated,
    };

    let mut payment = checkout_payment(&submission, [0x88; 32]);
    payment.evidence.rung = SettlementRung::Submitted;
    let trace = TraceId::mint([0xf3; 16]);

    seller
        .process_checkout(submission.clone(), &mut payment, &trace, 200)
        .unwrap_or_else(|error| panic!("first attempt: {error}"));

    payment.evidence.rung = SettlementRung::Receipted;
    let status = seller
        .process_checkout(submission.clone(), &mut payment, &trace, 201)
        .unwrap_or_else(|error| panic!("second attempt: {error}"));
    assert_eq!(status, CheckoutStatus::Completed);

    let order_id = format!("ord_{}", hex(&idempotency_key.gateway_key()[..8]));
    let order = seller
        .get_order(&order_id)
        .unwrap_or_else(|error| panic!("order retrieval: {error}"));

    assert_eq!(order.checkout_id, "chk_order_retrieve");
    assert_eq!(order.currency, *b"EUR");
    assert_eq!(order.total_minor, 15000);
    assert_ne!(
        order.receipt_digest, [0; 32],
        "order must carry verified receipt digest"
    );
}

#[test]
fn ucp_client_and_seller_maintain_idempotency_across_retries() {
    let client = UcpClient::new().unwrap_or_else(|error| panic!("client setup: {error}"));
    let mut seller = LayerXSeller::new().unwrap_or_else(|error| panic!("seller setup: {error}"));

    let merchant_handler = seller.merchant_profile.payment_handlers()[0].clone();
    let negotiated =
        NegotiatedCapabilities::negotiate(&client.platform_profile(), &merchant_handler)
            .unwrap_or_else(|error| panic!("negotiation: {error}"));

    let asset = [0xb1; 32];
    let recipient = [0xb2; 32];
    let idempotency_key = UcpIdempotencyKey::parse("deadbeef-1234-5678-9abc-def012345678")
        .unwrap_or_else(|error| panic!("idempotency key: {error}"));

    let submission = CheckoutSubmission {
        checkout_id: "chk_idempotency".to_owned(),
        currency: *b"GBP",
        total_minor: 7500,
        layerx_asset: asset,
        layerx_recipient: recipient,
        idempotency_key,
        negotiated,
    };

    let mut payment = checkout_payment(&submission, [0x88; 32]);
    payment.evidence.rung = SettlementRung::Submitted;
    let trace = TraceId::mint([0xb3; 16]);

    seller
        .process_checkout(submission.clone(), &mut payment, &trace, 300)
        .unwrap_or_else(|error| panic!("attempt 1: {error}"));

    payment.evidence.rung = SettlementRung::Receipted;
    let completed = seller
        .process_checkout(submission.clone(), &mut payment, &trace, 301)
        .unwrap_or_else(|error| panic!("attempt 2: {error}"));
    assert_eq!(completed, CheckoutStatus::Completed);

    let replay = seller
        .process_checkout(submission.clone(), &mut payment, &trace, 302)
        .unwrap_or_else(|error| panic!("replay: {error}"));
    assert_eq!(
        replay,
        CheckoutStatus::Completed,
        "idempotent replay must return same completed status"
    );
}
