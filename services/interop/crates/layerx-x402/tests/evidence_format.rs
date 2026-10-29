use std::collections::BTreeMap;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite};
use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_interop_gateway::gateway::{TranslationKind, TranslationRequest, TranslationStatus};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::{TraceId, Traced};
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_wire::encode::Encoder;
use layerx_wire::limits::PROTOCOL_VERSION;
use layerx_x402::buyer::{BuiltPayment, Buyer, LayerXEvidence};
use layerx_x402::model::{
    account_identifiers, AtomicAmount, PaymentPayload, PaymentRequirements, SettlementResponse,
    X402Error, X402_VERSION,
};
use layerx_x402::seller::{ExecutedPayment, Seller, SellerOutcome};
use layerx_x402::x402_adapter_descriptor;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const ACCOUNT: &str = "agent:did:layerx:evidence-payee:main";
const ASSET: [u8; 32] = [0xab; 32];
const PAYER: [u8; 32] = [0x71; 32];
const REFERENCE: [u8; 32] = [0x74; 32];
const TRANSACTION: [u8; 32] = [0x61; 32];
const GRANT: [u8; 32] = [0x8a; 32];

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn offer(scheme: &str) -> PaymentRequirements {
    let mut terms = json!({"account": ACCOUNT, "currency": "LXP", "commitment": "receipted"});
    if scheme != "exact" {
        terms["purposeHash"] = json!("7c".repeat(32));
    }
    PaymentRequirements {
        scheme: scheme.to_owned(),
        network: "eip155:125".to_owned(),
        amount: AtomicAmount::from_u128(1000),
        asset: hex(&ASSET),
        pay_to: hex(&account_identifiers(ACCOUNT).unwrap()[0]),
        max_timeout_seconds: 90,
        extra: Some(json!({"layerx": terms})),
    }
}

fn payment_log(scheme: &str) -> SequencerLog {
    let signature = if scheme == "exact" {
        "c4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1"
    } else {
        "13473d103a2d52172016c872eeae0153e254af3c21a1d987492ab737f1a34fe1"
    };
    let topic = std::array::from_fn(|index| {
        u8::from_str_radix(&signature[index * 2..index * 2 + 2], 16).unwrap()
    });
    let mut address = [0; 20];
    address[18..].copy_from_slice(&[0x10, 0x1a]);
    let mut data = ASSET.to_vec();
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(&1000_u128.to_be_bytes());
    if scheme != "exact" {
        data.extend_from_slice(&REFERENCE);
        data.extend_from_slice(&[0x9b; 32]);
    }
    SequencerLog::new(
        address,
        vec![
            topic,
            if scheme == "exact" { REFERENCE } else { GRANT },
            PAYER,
            account_identifiers(ACCOUNT).unwrap()[0],
        ],
        data,
        0,
    )
}

fn receipt(logs: &[SequencerLog], signature: Option<[u8; 64]>) -> Vec<u8> {
    let mut summary = vec![0; 8 * 3 + 4 + 20 * 2];
    summary.extend_from_slice(&Sha256::digest(b""));
    summary.extend_from_slice(&0_u32.to_be_bytes());
    summary.extend_from_slice(&u32::try_from(logs.len()).unwrap().to_be_bytes());
    summary.push(4);
    summary.extend_from_slice(b"uhpx");
    let mut encoder = Encoder::new(4096);
    assert_eq!(
        encoder.structure_header_version(0x5201, PROTOCOL_VERSION),
        Ok(())
    );
    assert_eq!(encoder.u16(PROTOCOL_VERSION), Ok(()));
    assert_eq!(encoder.bytes(&TRANSACTION, 32), Ok(()));
    assert_eq!(encoder.u64(9), Ok(()));
    for value in [0x62, 0x63, 0x64] {
        assert_eq!(encoder.bytes(&[value; 32], 32), Ok(()));
    }
    assert_eq!(encoder.i32(0), Ok(()));
    let mut effects = vec![(1, summary)];
    for log in logs {
        let mut body = log.address().to_vec();
        body.push(u8::try_from(log.topics().len()).unwrap());
        for topic in log.topics() {
            body.extend_from_slice(topic);
        }
        body.extend_from_slice(&Sha256::digest(log.data()));
        effects.push((3, body));
    }
    assert_eq!(encoder.sequence_length(effects.len(), 4096), Ok(()));
    for (ordinal, (kind, body)) in effects.into_iter().enumerate() {
        assert_eq!(encoder.u16(12), Ok(()));
        assert_eq!(encoder.u16(u16::try_from(ordinal).unwrap()), Ok(()));
        assert_eq!(encoder.u16(0), Ok(()));
        assert_eq!(encoder.u8(kind), Ok(()));
        assert_eq!(encoder.u8(0), Ok(()));
        assert_eq!(encoder.bytes(&[0; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&body, 256), Ok(()));
    }
    let mut height = [0; 32];
    height[24..].copy_from_slice(&1000_u64.to_be_bytes());
    assert_eq!(encoder.u128(1), Ok(()));
    assert_eq!(encoder.bytes(&height, 32), Ok(()));
    assert_eq!(encoder.u16(12), Ok(()));
    assert_eq!(encoder.u32(1), Ok(()));
    assert_eq!(encoder.u32(0), Ok(()));
    assert_eq!(encoder.u8(2), Ok(()));
    assert_eq!(encoder.bytes(&ASSET, 32), Ok(()));
    assert_eq!(encoder.u128(1000), Ok(()));
    assert_eq!(encoder.bytes(&PAYER, 32), Ok(()));
    assert_eq!(encoder.u128(5000), Ok(()));
    assert_eq!(encoder.u128(4000), Ok(()));
    assert_eq!(encoder.u64(1), Ok(()));
    assert_eq!(
        encoder.bytes(&account_identifiers(ACCOUNT).unwrap()[0], 32),
        Ok(())
    );
    assert_eq!(encoder.u128(0), Ok(()));
    assert_eq!(encoder.u128(1000), Ok(()));
    for value in [0x66, 0x67, 0x68] {
        assert_eq!(encoder.bytes(&[value; 32], 32), Ok(()));
    }
    assert_eq!(encoder.u64(1000), Ok(()));
    assert_eq!(encoder.u8(u8::from(signature.is_some())), Ok(()));
    if let Some(signature) = signature {
        assert_eq!(encoder.bytes(&signature, 64), Ok(()));
    }
    encoder.finish()
}

fn keys() -> Vec<SequencerAuthorization> {
    vec![SequencerAuthorization::new(
        [0x68; 32],
        SigningKey::from_bytes(&[0x72; 32])
            .verifying_key()
            .to_bytes(),
        1000,
        1000,
    )]
}

fn settle(accepted: &PaymentRequirements) -> (String, SettlementResponse, Vec<SequencerLog>) {
    let trace = TraceId::mint([0x73; 16]);
    let principal = PrincipalId::new("evidence-seller").unwrap();
    let mut gateway = GatewayCore::new();
    let suite = ConformanceSuite::new(AdapterId::new("x402-v2").unwrap(), 1, [0x75; 32]).unwrap();
    gateway
        .register_adapter(x402_adapter_descriptor(suite).unwrap(), &trace, 0)
        .unwrap();
    let request = TranslationRequest::new(
        AdapterId::new("x402").unwrap(),
        TranslationKind::StateChanging,
        REFERENCE,
        [0x76; 32],
    )
    .unwrap();
    assert_eq!(
        gateway
            .begin_translation(&principal, &request, &trace, 0)
            .unwrap(),
        TranslationStatus::Pending
    );
    let logs = vec![
        payment_log(&accepted.scheme),
        SequencerLog::new([0x45; 20], vec![[0x46; 32]], vec![], 1),
    ];
    let signer = SigningKey::from_bytes(&[0x72; 32]);
    let digest = layerx_wire::hash::receipt_digest(&receipt(&logs, None)).unwrap();
    let executed = ExecutedPayment {
        committed: None,
        evidence: SequencerEvidence {
            transaction_hash: TRANSACTION,
            signed_receipt: receipt(&logs, Some(signer.sign(&digest).to_bytes())),
            logs: logs.clone(),
            rung: SettlementRung::Receipted,
        },
    };
    let outcome = Seller::settled(
        &mut gateway,
        &principal,
        REFERENCE,
        accepted,
        &json!({"idempotencyKey": hex(&REFERENCE), "grantId": hex(&GRANT)}),
        &executed,
        &keys(),
        &trace,
        1,
    )
    .unwrap();
    let SellerOutcome::Settled {
        header,
        response,
        receipt_digest,
    } = outcome
    else {
        panic!("verified receipt must settle");
    };
    assert_eq!(
        gateway.translation(&principal, REFERENCE),
        Some(TranslationStatus::ReceiptVerified { receipt_digest })
    );
    (header, response, logs)
}

fn built(accepted: PaymentRequirements) -> BuiltPayment {
    let payload = PaymentPayload {
        x402_version: X402_VERSION,
        resource: None,
        accepted,
        payload: json!({"idempotencyKey": hex(&REFERENCE)}),
        extensions: BTreeMap::new(),
    };
    payload.validate().unwrap();
    BuiltPayment {
        header: STANDARD.encode(serde_json::to_vec(&payload).unwrap()),
        payload,
        idempotency_key: REFERENCE,
    }
}

#[test]
fn buyer_accepts_the_model_asset_format() {
    for asset_prefix in ["", "0x"] {
        for payee_prefix in ["", "0x"] {
            let mut accepted = offer("exact");
            accepted.asset.insert_str(0, asset_prefix);
            accepted.pay_to.insert_str(0, payee_prefix);
            accepted.validate().unwrap();
            accepted.layerx_terms().unwrap();
            let (header, response, _) = settle(&accepted);
            let payment = built(accepted);
            let trace = TraceId::mint([0x77; 16]);
            let captured = Buyer::capture_settlement(&header, &payment, &keys(), &trace).unwrap();
            assert_eq!(captured.response, response);
            assert_eq!(captured.transaction_hash, TRANSACTION);
            for field in ["asset", "payTo"] {
                let mut changed = payment.clone();
                if field == "asset" {
                    changed.payload.accepted.asset = hex(&[0xcd; 32]);
                } else {
                    changed.payload.accepted.pay_to = hex(&[0xcd; 32]);
                }
                assert_eq!(
                    Buyer::capture_settlement(&header, &changed, &keys(), &trace)
                        .map_err(Traced::into_error),
                    Err(X402Error::EvidenceMismatch)
                );
            }
        }
    }
}

#[test]
fn seller_evidence_carries_settlement_logs() {
    for scheme in ["exact", "metered", "subscription"] {
        let accepted = offer(scheme);
        let (header, response, logs) = settle(&accepted);
        let decoded: SettlementResponse =
            serde_json::from_slice(&STANDARD.decode(&header).unwrap()).unwrap();
        assert_eq!(decoded, response);
        let evidence = &decoded.extensions["layerx"];
        let _: LayerXEvidence = serde_json::from_value(evidence.clone()).unwrap();
        let served = evidence["logs"].as_array().unwrap();
        assert!(!served.is_empty());
        assert_eq!(served.len(), logs.len());
        let decoded_logs = served
            .iter()
            .zip(&logs)
            .map(|(wire, log)| {
                assert_eq!(wire["address"], format!("0x{}", hex(&log.address())));
                assert_eq!(
                    wire["topics"],
                    json!(log
                        .topics()
                        .iter()
                        .map(|topic| format!("0x{}", hex(topic)))
                        .collect::<Vec<_>>())
                );
                assert_eq!(wire["data"], format!("0x{}", hex(log.data())));
                assert_eq!(wire["logIndex"], format!("0x{:x}", log.log_index()));
                log.clone()
            })
            .collect::<Vec<_>>();
        let signed = STANDARD
            .decode(evidence["receipt"].as_str().unwrap())
            .unwrap();
        let receipt = verify_sequencer_signature(&signed, keys().as_slice()).unwrap();
        let verified = verify_logs(&receipt, decoded_logs).unwrap();
        assert_eq!(verified.logs(), logs);
        if scheme == "exact" {
            let captured = Buyer::capture_settlement(
                &header,
                &built(accepted),
                &keys(),
                &TraceId::mint([0x78; 16]),
            )
            .unwrap();
            assert_eq!(captured.response, response);
        } else {
            assert_eq!(evidence["purposeHash"], "7c".repeat(32));
        }
        let mut missing_logs = evidence.clone();
        missing_logs.as_object_mut().unwrap().remove("logs");
        assert!(serde_json::from_value::<LayerXEvidence>(missing_logs).is_err());
        assert!(evidence["logs"]
            .as_array()
            .unwrap()
            .iter()
            .all(Value::is_object));
    }
}
