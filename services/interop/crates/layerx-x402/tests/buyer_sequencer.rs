use std::collections::BTreeMap;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    header::{
        validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator,
    },
    merkle,
    proto::{encode_length_delimited, encode_varint, encode_varint_field},
    CommittedProof,
};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite};
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
use layerx_interop_gateway::gateway::{TranslationKind, TranslationRequest, TranslationStatus};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::{TraceId, Traced};
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_wire::hash::receipt_digest;
use layerx_wire::receipt::encode_unsigned;
use layerx_x402::buyer::{BuiltPayment, Buyer, CapturedSettlement, SupportedKind};
use layerx_x402::model::{
    account_identifiers, AtomicAmount, PaymentPayload, PaymentRequirements, SettlementResponse,
    X402Error, X402_VERSION,
};
use layerx_x402::seller::{CommittedSettlementProof, ExecutedPayment, Seller, SellerOutcome};
use layerx_x402::x402_adapter_descriptor;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

type TestResult<T = ()> = Result<T, Box<dyn std::error::Error>>;

fn historical_fixture(id: &str) -> Value {
    let document: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap_or_else(|error| panic!("receipt fixture: {error}"));
    document["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("fixture cases"))
        .iter()
        .find(|case| case["id"] == id)
        .unwrap_or_else(|| panic!("missing fixture {id}"))
        .clone()
}

fn fixture(id: &str) -> Value {
    let mut case = historical_fixture(id);
    let mut topics = case["receiptJson"]["logs"][0]["topics"]
        .as_array()
        .unwrap_or_else(|| panic!("fixture topics"))
        .iter()
        .map(|topic| word(text(topic)))
        .collect::<Vec<_>>();
    let recorded_topics = topics.concat();
    topics[1..].rotate_right(1);
    let mut receipt = bytes(text(&case["signedReceipt"]));
    let positions: Vec<_> = receipt
        .windows(recorded_topics.len())
        .enumerate()
        .filter_map(|(index, value)| (value == recorded_topics).then_some(index))
        .collect();
    assert_eq!(positions.len(), 1);
    receipt[positions[0]..positions[0] + recorded_topics.len()].copy_from_slice(&topics.concat());
    let digest = canonical_receipt_digest(&receipt)
        .unwrap_or_else(|error| panic!("native receipt digest: {error}"));
    let signer = SigningKey::from_bytes(&[0x72; 32]);
    let signature_start = receipt.len() - 64;
    receipt[signature_start..].copy_from_slice(&signer.sign(&digest).to_bytes());
    case["signedReceipt"] = json!(format!("0x{}", hex(&receipt)));
    case["receiptDigest"] = json!(format!("0x{}", hex(&digest)));
    case["pinnedKey"]["publicKey"] =
        json!(format!("0x{}", hex(&signer.verifying_key().to_bytes())));
    case["receiptJson"]["logs"][0]["topics"] = json!(topics
        .iter()
        .map(|topic| format!("0x{}", hex(topic)))
        .collect::<Vec<_>>());
    case
}

fn text(value: &Value) -> &str {
    value.as_str().unwrap_or_else(|| panic!("expected text"))
}

fn bytes(value: &str) -> Vec<u8> {
    let digits = value
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("hex prefix"));
    assert_eq!(digits.len() % 2, 0);
    (0..digits.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&digits[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("hex: {error}"))
        })
        .collect()
}

fn word(value: &str) -> [u8; 32] {
    bytes(value)
        .try_into()
        .unwrap_or_else(|_| panic!("expected word"))
}

fn hex(value: &[u8]) -> String {
    value.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn recorded(id: &str) -> TestResult<(Buyer, BuiltPayment, SettlementResponse, Value)> {
    let case = fixture(id);
    let exact = fixture("V1-layerxledger-pay-exact");
    let log = &exact["receiptJson"]["logs"][0];
    let data = bytes(text(&log["data"]));
    let amount = AtomicAmount::from_u128(1_000_000);
    let key = &case["pinnedKey"];
    let authorization = SequencerAuthorization::new(
        word(text(&key["keyId"])),
        word(text(&key["publicKey"])),
        key["firstHeight"].as_u64().ok_or("first height")?,
        key["lastHeight"].as_u64().ok_or("last height")?,
    );
    let buyer = Buyer::with_receipt_keys(
        vec![SupportedKind {
            scheme: "exact".to_owned(),
            network: "eip155:125".to_owned(),
        }],
        vec![authorization],
    )?;
    let payment = BuiltPayment {
        header: String::new(),
        payload: PaymentPayload {
            x402_version: X402_VERSION,
            resource: None,
            accepted: PaymentRequirements {
                scheme: "exact".to_owned(),
                network: "eip155:125".to_owned(),
                amount,
                asset: format!("0x{}", hex(&data[..32])),
                pay_to: text(&log["topics"][3]).to_owned(),
                max_timeout_seconds: 90,
                extra: Some(
                    json!({"layerx": {"account": "agent:did:layerx:payee:main", "currency": "LXP"}}),
                ),
            },
            payload: json!({}),
            extensions: BTreeMap::new(),
        },
        idempotency_key: word(text(&log["topics"][1])),
    };
    let response = SettlementResponse {
        success: true,
        error_reason: None,
        payer: Some(text(&log["topics"][2]).trim_start_matches("0x").to_owned()),
        transaction: text(&case["receiptJson"]["txHash"]).to_owned(),
        network: "eip155:125".to_owned(),
        amount: Some(amount),
        extensions: BTreeMap::from([(
            "layerx".to_owned(),
            json!({
                "receipt": STANDARD.encode(bytes(text(&case["signedReceipt"]))),
                "receiptDigest": text(&case["receiptDigest"]).trim_start_matches("0x"),
                "verificationLevel": "receipted",
                "logs": case["receiptJson"]["logs"],
            }),
        )]),
    };
    Ok((buyer, payment, response, case))
}

fn capture(
    buyer: &Buyer,
    payment: &BuiltPayment,
    response: &SettlementResponse,
) -> Result<CapturedSettlement, X402Error> {
    let header = STANDARD.encode(serde_json::to_vec(response).map_err(|_| X402Error::Encode)?);
    Buyer::capture_settlement(
        &header,
        payment,
        buyer.receipt_keys(),
        &TraceId::mint([0xab; 16]),
    )
    .map_err(Traced::into_error)
}

#[test]
fn buyer_captures_the_recorded_receipted_settlement() -> TestResult {
    let (buyer, payment, response, case) = recorded("V1-layerxledger-pay-exact")?;
    let captured = capture(&buyer, &payment, &response)?;
    assert_eq!(
        captured.transaction_hash,
        word(text(&case["receiptJson"]["txHash"]))
    );
    assert_eq!(captured.response, response);
    assert_eq!(captured.rung, "receipted");
    assert_eq!(
        captured.canonical_receipt,
        bytes(text(&case["signedReceipt"]))
    );
    assert_eq!(captured.receipt_digest, word(text(&case["receiptDigest"])));
    let decoded = verify_sequencer_signature(&captured.canonical_receipt, buyer.receipt_keys())
        .map_err(|error| format!("signature: {error:?}"))?;
    let canonical_digest =
        receipt_digest(&encode_unsigned(&decoded).map_err(|error| format!("encode: {error:?}"))?)
            .map_err(|error| format!("digest: {error:?}"))?;
    assert_eq!(captured.receipt_digest, canonical_digest);

    let mut wrong_reference = payment.clone();
    wrong_reference.idempotency_key[0] ^= 1;
    assert_eq!(
        capture(&buyer, &wrong_reference, &response),
        Err(X402Error::EvidenceMismatch)
    );
    let mut wrong_amount = payment.clone();
    wrong_amount.payload.accepted.amount = AtomicAmount::from_u128(999_999);
    assert_eq!(
        capture(&buyer, &wrong_amount, &response),
        Err(X402Error::EvidenceMismatch)
    );
    for field in ["asset", "payTo"] {
        let mut wrong_payment = payment.clone();
        let value = format!("0x{}", "ff".repeat(32));
        if field == "asset" {
            wrong_payment.payload.accepted.asset = value;
        } else {
            wrong_payment.payload.accepted.pay_to = value;
        }
        assert_eq!(
            capture(&buyer, &wrong_payment, &response),
            Err(X402Error::EvidenceMismatch)
        );
    }
    let mut wrong_payer = response.clone();
    wrong_payer.payer = Some("ff".repeat(32));
    assert_eq!(
        capture(&buyer, &payment, &wrong_payer),
        Err(X402Error::EvidenceMismatch)
    );
    let unpinned = Buyer::new(vec![SupportedKind {
        scheme: "exact".to_owned(),
        network: "eip155:125".to_owned(),
    }])?;
    assert_eq!(
        capture(&unpinned, &payment, &response),
        Err(X402Error::EvidenceMismatch)
    );
    Ok(())
}

#[test]
fn buyer_refuses_legacy_payment_exact_topic_order() -> TestResult {
    let (_, payment, mut response, _) = recorded("V1-layerxledger-pay-exact")?;
    let case = historical_fixture("V1-layerxledger-pay-exact");
    let key = &case["pinnedKey"];
    let authorization = SequencerAuthorization::new(
        word(text(&key["keyId"])),
        word(text(&key["publicKey"])),
        key["firstHeight"].as_u64().ok_or("first height")?,
        key["lastHeight"].as_u64().ok_or("last height")?,
    );
    let buyer = Buyer::with_receipt_keys(
        vec![SupportedKind {
            scheme: "exact".to_owned(),
            network: "eip155:125".to_owned(),
        }],
        vec![authorization],
    )?;
    let evidence = response.extensions.get_mut("layerx").ok_or("layerx")?;
    evidence["receipt"] = json!(STANDARD.encode(bytes(text(&case["signedReceipt"]))));
    evidence["receiptDigest"] = json!(text(&case["receiptDigest"]).trim_start_matches("0x"));
    evidence["logs"] = case["receiptJson"]["logs"].clone();
    assert_eq!(
        capture(&buyer, &payment, &response),
        Err(X402Error::EvidenceMismatch)
    );
    Ok(())
}

#[test]
fn buyer_refuses_a_tampered_log() -> TestResult {
    let (buyer, payment, response, case) = recorded("V3-tampered-log")?;
    let signed = bytes(text(&case["signedReceipt"]));
    let receipt = verify_sequencer_signature(&signed, buyer.receipt_keys())
        .map_err(|error| format!("signature: {error:?}"))?;
    let log = &case["receiptJson"]["logs"][0];
    let served = SequencerLog::new(
        bytes(text(&log["address"]))
            .try_into()
            .map_err(|_| "address")?,
        log["topics"]
            .as_array()
            .ok_or("topics")?
            .iter()
            .map(|topic| word(text(topic)))
            .collect(),
        bytes(text(&log["data"])),
        0,
    );
    assert!(verify_logs(&receipt, vec![served]).is_err());
    assert_eq!(
        capture(&buyer, &payment, &response),
        Err(X402Error::EvidenceMismatch)
    );

    let (buyer, payment, mut response, _) = recorded("V1-layerxledger-pay-exact")?;
    capture(&buyer, &payment, &response)?;
    let original = text(&response.extensions["layerx"]["logs"][0]["data"]).to_owned();
    let mut tampered = bytes(&original);
    *tampered.last_mut().ok_or("log data")? ^= 1;
    response.extensions.get_mut("layerx").ok_or("layerx")?["logs"][0]["data"] =
        json!(format!("0x{}", hex(&tampered)));
    assert_eq!(
        capture(&buyer, &payment, &response),
        Err(X402Error::EvidenceMismatch)
    );
    Ok(())
}

#[test]
fn buyer_refuses_a_digest_over_the_signed_bytes() -> TestResult {
    let (buyer, payment, mut response, case) = recorded("V1-layerxledger-pay-exact")?;
    capture(&buyer, &payment, &response)?;
    let signed = bytes(text(&case["signedReceipt"]));
    let signed_digest: [u8; 32] = Sha256::digest(&signed).into();
    assert_ne!(signed_digest, word(text(&case["receiptDigest"])));
    response.extensions.get_mut("layerx").ok_or("layerx")?["receiptDigest"] =
        json!(hex(&signed_digest));
    assert_eq!(
        capture(&buyer, &payment, &response),
        Err(X402Error::EvidenceMismatch)
    );
    let domain_signed_digest =
        receipt_digest(&signed).map_err(|error| format!("digest: {error:?}"))?;
    assert_ne!(domain_signed_digest, word(text(&case["receiptDigest"])));
    response.extensions.get_mut("layerx").ok_or("layerx")?["receiptDigest"] =
        json!(hex(&domain_signed_digest));
    assert_eq!(
        capture(&buyer, &payment, &response),
        Err(X402Error::EvidenceMismatch)
    );
    Ok(())
}

fn proof_header_json(header: &Header) -> Value {
    json!({
        "version": {"block": header.version.block, "app": header.version.app},
        "chain_id": header.chain_id,
        "height": header.height,
        "time": {"seconds": header.time.seconds, "nanos": header.time.nanos},
        "last_block_id": {
            "hash": format!("0x{}", hex(&header.last_block_id.hash)),
            "parts": {
                "total": header.last_block_id.part_set_header.total,
                "hash": format!("0x{}", hex(&header.last_block_id.part_set_header.hash)),
            },
        },
        "last_commit_hash": format!("0x{}", hex(&header.last_commit_hash)),
        "data_hash": format!("0x{}", hex(&header.data_hash)),
        "validators_hash": format!("0x{}", hex(&header.validators_hash)),
        "next_validators_hash": format!("0x{}", hex(&header.next_validators_hash)),
        "consensus_hash": format!("0x{}", hex(&header.consensus_hash)),
        "app_hash": format!("0x{}", hex(&header.app_hash)),
        "last_results_hash": format!("0x{}", hex(&header.last_results_hash)),
        "evidence_hash": format!("0x{}", hex(&header.evidence_hash)),
        "proposer_address": format!("0x{}", hex(&header.proposer_address)),
    })
}

fn committed_settlement_proof(
    buyer: &Buyer,
    response: &SettlementResponse,
    case: &Value,
) -> TestResult<CommittedSettlementProof> {
    let signed = STANDARD.decode(text(&response.extensions["layerx"]["receipt"]))?;
    let receipt = verify_sequencer_signature(&signed, buyer.receipt_keys())
        .map_err(|error| format!("signature: {error:?}"))?;
    let receipt = receipt.protocol().ok_or("protocol receipt")?;
    let height = u64::from_be_bytes(receipt.batch_id()[24..].try_into()?);
    let mut batch = b"PAXEER/v1/sequence-batch\0".to_vec();
    batch.extend_from_slice(&height.to_be_bytes());
    batch.extend_from_slice(&receipt.global_sequence().to_be_bytes());
    batch.extend_from_slice(&receipt.global_sequence().to_be_bytes());
    batch.extend_from_slice(&receipt.previous_state_root());
    batch.extend_from_slice(&receipt.resulting_state_root());
    batch.extend_from_slice(&Sha256::digest(receipt.activity_id()));
    batch.extend_from_slice(&buyer.receipt_keys()[0].sequencer_id());
    let batch_signature = SigningKey::from_bytes(&[0x72; 32]).sign(&batch);
    batch.extend_from_slice(&batch_signature.to_bytes());
    let batch_hash = Sha256::digest(&batch);
    let transaction_hash = Sha256::digest(bytes(text(&case["rawTransaction"])));
    let data_hash = merkle::root(&[&batch_hash, &transaction_hash]);
    let aunt = merkle::root(&[&transaction_hash]);

    let validator = SigningKey::from_bytes(&[0x39; 32]);
    let public_key = validator.verifying_key().to_bytes();
    let validators = [Validator {
        pub_key: public_key,
        voting_power: 10,
    }];
    let set_hash = validators_hash(&validators);
    let address = Sha256::digest(public_key)[..20].to_vec();
    let execution = Header {
        version: ConsensusVersion { block: 11, app: 0 },
        chain_id: "x402-buyer-proof".to_owned(),
        height: height.try_into()?,
        time: Timestamp {
            seconds: 1_700_000_000,
            nanos: 0,
        },
        last_block_id: BlockId::default(),
        last_commit_hash: Vec::new(),
        data_hash: data_hash.to_vec(),
        validators_hash: set_hash,
        next_validators_hash: set_hash.to_vec(),
        consensus_hash: Vec::new(),
        app_hash: Vec::new(),
        last_results_hash: Vec::new(),
        evidence_hash: Vec::new(),
        proposer_address: address.clone(),
    };
    let block_hash = execution.hash();
    let mut successor = execution.clone();
    successor.height += 1;
    successor.last_block_id.hash = block_hash.to_vec();
    let header_hash = successor.hash();
    let parts_hash = Sha256::digest(header_hash);

    let mut parts = Vec::new();
    encode_varint_field(1, 1, &mut parts);
    encode_length_delimited(2, &parts_hash, &mut parts);
    let mut block_id = Vec::new();
    encode_length_delimited(1, &header_hash, &mut block_id);
    encode_length_delimited(2, &parts, &mut block_id);
    let mut timestamp = Vec::new();
    encode_varint_field(1, successor.time.seconds as u64, &mut timestamp);
    let mut vote = vec![0x08, 0x02, 0x11];
    vote.extend_from_slice(&(successor.height as u64).to_le_bytes());
    encode_length_delimited(4, &block_id, &mut vote);
    encode_length_delimited(5, &timestamp, &mut vote);
    encode_length_delimited(6, successor.chain_id.as_bytes(), &mut vote);
    let mut sign_bytes = Vec::new();
    encode_varint(vote.len() as u64, &mut sign_bytes);
    sign_bytes.extend_from_slice(&vote);
    let signature = validator.sign(&sign_bytes).to_bytes();

    Ok(CommittedSettlementProof {
        chain_id: successor.chain_id.clone(),
        validators: validators.to_vec(),
        proof: CommittedProof {
            header: successor.clone(),
            execution_header: execution,
            commit: Commit {
                height: successor.height.try_into()?,
                round: 0,
                block_id: BlockId {
                    hash: header_hash.to_vec(),
                    part_set_header: PartSetHeader {
                        total: 1,
                        hash: parts_hash.to_vec(),
                    },
                },
                signatures: vec![CommitSig {
                    block_id_flag: BlockIdFlag::Commit,
                    validator_address: address,
                    timestamp: successor.time,
                    signature: Some(signature),
                }],
            },
            batch_transaction: batch,
            batch_total: 2,
            batch_index: 0,
            batch_aunts: vec![aunt],
            activity_ids: vec![receipt.activity_id()],
            signed_receipt: signed,
        },
    })
}

fn committed_response(
    buyer: &Buyer,
    mut response: SettlementResponse,
    case: &Value,
) -> TestResult<SettlementResponse> {
    let committed = committed_settlement_proof(buyer, &response, case)?;
    let proof = &committed.proof;
    let execution = &proof.execution_header;
    let successor = &proof.header;
    let height = execution.height;
    let block_hash = execution.hash();
    let header_hash = &proof.commit.block_id.hash;
    let parts_hash = &proof.commit.block_id.part_set_header.hash;
    let public_key = committed.validators[0].pub_key;
    let address = &proof.commit.signatures[0].validator_address;
    let signature = proof.commit.signatures[0]
        .signature
        .ok_or("commit signature")?;
    let data_hash = &execution.data_hash;
    let batch = &proof.batch_transaction;
    let aunt = proof.batch_aunts[0];
    let evidence = response.extensions.get_mut("layerx").ok_or("layerx")?;
    evidence["verificationLevel"] = json!("committed");
    evidence["blockNumber"] = json!(format!("0x{height:x}"));
    evidence["blockHash"] = json!(format!("0x{}", hex(&block_hash)));
    evidence["proof"] = json!({
        "chainId": successor.chain_id,
        "header": proof_header_json(&successor),
        "executionHeader": proof_header_json(&execution),
        "validators": [{"pub_key": format!("0x{}", hex(&public_key)), "voting_power": 10}],
        "commit": {
            "height": successor.height, "round": 0,
            "block_id": {"hash": format!("0x{}", hex(&header_hash)),
                "parts": {"total": 1, "hash": format!("0x{}", hex(&parts_hash))}},
            "signatures": [{"block_id_flag": 2,
                "validator_address": format!("0x{}", hex(&address)),
                "timestamp": {"seconds": successor.time.seconds, "nanos": 0},
                "signature": format!("0x{}", hex(&signature))}],
        },
        "batchProof": {
            "leafEncoding": "sha256-tx", "rootHash": format!("0x{}", hex(&data_hash)),
            "data": format!("0x{}", hex(&batch)), "total": 2, "index": 0,
            "aunts": [format!("0x{}", hex(&aunt))],
        },
        "activityIds": [format!("0x{}", hex(&proof.activity_ids[0]))],
    });
    Ok(response)
}

fn recorded_for_seller() -> TestResult<(Buyer, BuiltPayment, SettlementResponse, Value)> {
    let (buyer, mut payment, mut response, case) = recorded("V1-layerxledger-pay-exact")?;
    let payee = account_identifiers("agent:did:layerx:payee:main")?[0];
    let original_payee = word(&payment.payload.accepted.pay_to);
    let evidence = response.extensions.get_mut("layerx").ok_or("layerx")?;
    let mut signed = STANDARD.decode(text(&evidence["receipt"]))?;
    let unsigned_length = signed.len() - 69;
    let positions: Vec<_> = signed[..unsigned_length]
        .windows(original_payee.len())
        .enumerate()
        .filter_map(|(index, value)| (value == original_payee).then_some(index))
        .collect();
    assert!(!positions.is_empty());
    for index in positions {
        signed[index..index + payee.len()].copy_from_slice(&payee);
    }
    let digest = canonical_receipt_digest(&signed)?;
    let signature = SigningKey::from_bytes(&[0x72; 32]).sign(&digest).to_bytes();
    let signature_start = signed.len() - signature.len();
    signed[signature_start..].copy_from_slice(&signature);
    evidence["receipt"] = json!(STANDARD.encode(&signed));
    evidence["receiptDigest"] = json!(hex(&digest));
    evidence["logs"][0]["topics"][3] = json!(format!("0x{}", hex(&payee)));
    payment.payload.accepted.pay_to = format!("0x{}", hex(&payee));
    capture(&buyer, &payment, &response)?;
    Ok((buyer, payment, response, case))
}

#[test]
fn buyer_refuses_a_committed_rung_with_a_wrong_block_hash() -> TestResult {
    let (buyer, payment, response, case) = recorded_for_seller()?;
    let response = committed_response(&buyer, response, &case)?;
    let captured = capture(&buyer, &payment, &response)?;
    assert_eq!(captured.rung, "committed");
    assert_eq!(captured.response, response);

    let mut wrong_hash = response.clone();
    wrong_hash.extensions.get_mut("layerx").ok_or("layerx")?["blockHash"] =
        json!(format!("0x{}", "ff".repeat(32)));
    assert_eq!(
        capture(&buyer, &payment, &wrong_hash),
        Err(X402Error::EvidenceMismatch)
    );

    let mut wrong_number = response.clone();
    wrong_number.extensions.get_mut("layerx").ok_or("layerx")?["blockNumber"] = json!("0x1");
    assert_eq!(
        capture(&buyer, &payment, &wrong_number),
        Err(X402Error::EvidenceMismatch)
    );

    let mut missing = response.clone();
    missing
        .extensions
        .get_mut("layerx")
        .ok_or("layerx")?
        .as_object_mut()
        .ok_or("evidence object")?
        .remove("proof");
    assert_eq!(
        capture(&buyer, &payment, &missing),
        Err(X402Error::EvidenceMismatch)
    );

    for path in [
        "/proof/executionHeader/data_hash",
        "/proof/commit/signatures/0/signature",
        "/proof/batchProof/aunts/0",
        "/proof/activityIds/0",
    ] {
        let mut changed = response.clone();
        let field = changed
            .extensions
            .get_mut("layerx")
            .ok_or("layerx")?
            .pointer_mut(path)
            .ok_or("proof field")?;
        let mut corrupted = bytes(text(field));
        corrupted[0] ^= 1;
        *field = json!(format!("0x{}", hex(&corrupted)));
        assert_eq!(
            capture(&buyer, &payment, &changed),
            Err(X402Error::EvidenceMismatch),
            "{path}"
        );
    }
    Ok(())
}

fn opened_seller_gateway(
    payment: &BuiltPayment,
) -> TestResult<(GatewayCore, PrincipalId, TraceId)> {
    let mut gateway = GatewayCore::new();
    let principal = PrincipalId::new("committed-seller")?;
    let trace = TraceId::mint([0x73; 16]);
    let suite = ConformanceSuite::new(AdapterId::new("x402-v2")?, 1, [0x75; 32])?;
    gateway
        .register_adapter(x402_adapter_descriptor(suite)?, &trace, 0)
        .map_err(Traced::into_error)?;
    let request = TranslationRequest::new(
        AdapterId::new("x402")?,
        TranslationKind::StateChanging,
        payment.idempotency_key,
        [0x76; 32],
    )?;
    assert_eq!(
        gateway
            .begin_translation(&principal, &request, &trace, 0)
            .map_err(Traced::into_error)?,
        TranslationStatus::Pending
    );
    Ok((gateway, principal, trace))
}

fn executed_committed_payment(
    buyer: &Buyer,
    response: &SettlementResponse,
    case: &Value,
) -> TestResult<ExecutedPayment> {
    let committed = committed_settlement_proof(buyer, response, case)?;
    let log = &response.extensions["layerx"]["logs"][0];
    let evidence = SequencerEvidence {
        transaction_hash: word(&response.transaction),
        signed_receipt: committed.proof.signed_receipt.clone(),
        logs: vec![SequencerLog::new(
            bytes(text(&log["address"]))
                .try_into()
                .map_err(|_| "log address")?,
            log["topics"]
                .as_array()
                .ok_or("log topics")?
                .iter()
                .map(|topic| word(text(topic)))
                .collect(),
            bytes(text(&log["data"])),
            0,
        )],
        rung: SettlementRung::Committed {
            block_number: committed.proof.execution_header.height.try_into()?,
            block_hash: committed.proof.execution_header.hash(),
        },
    };
    Ok(ExecutedPayment {
        evidence,
        committed: Some(committed),
    })
}

#[test]
fn seller_committed_settlement_is_captured_by_the_buyer() -> TestResult {
    let (buyer, payment, response, case) = recorded_for_seller()?;
    let executed = executed_committed_payment(&buyer, &response, &case)?;
    let expected = committed_response(&buyer, response, &case)?;
    let (mut gateway, principal, trace) = opened_seller_gateway(&payment)?;
    let outcome = Seller::settled(
        &mut gateway,
        &principal,
        payment.idempotency_key,
        &payment.payload.accepted,
        &payment.payload.payload,
        &executed,
        buyer.receipt_keys(),
        &trace,
        1,
    )?;
    let SellerOutcome::Settled {
        header,
        response,
        receipt_digest,
    } = outcome
    else {
        return Err("committed payment must settle".into());
    };
    assert_eq!(
        response.extensions["layerx"]["proof"],
        expected.extensions["layerx"]["proof"]
    );
    let captured = Buyer::capture_settlement(&header, &payment, buyer.receipt_keys(), &trace)
        .map_err(Traced::into_error)?;
    assert_eq!(captured.rung, "committed");
    assert_eq!(
        captured.transaction_hash,
        executed.evidence.transaction_hash
    );
    assert_eq!(captured.response, response);
    assert_eq!(captured.receipt_digest, receipt_digest);
    assert_eq!(
        gateway.translation(&principal, payment.idempotency_key),
        Some(TranslationStatus::ReceiptVerified { receipt_digest })
    );

    let mut receipted = executed;
    receipted.evidence.rung = SettlementRung::Receipted;
    let outcome = Seller::settled(
        &mut gateway,
        &principal,
        payment.idempotency_key,
        &payment.payload.accepted,
        &payment.payload.payload,
        &receipted,
        buyer.receipt_keys(),
        &trace,
        2,
    )?;
    let SellerOutcome::Settled { response, .. } = outcome else {
        return Err("receipted payment must settle".into());
    };
    assert!(response.extensions["layerx"].get("proof").is_none());
    Ok(())
}

#[test]
fn seller_refuses_committed_evidence_without_a_proof() -> TestResult {
    let (buyer, payment, response, case) = recorded_for_seller()?;
    for tamper in [
        "missing",
        "receipt",
        "signature",
        "number",
        "hash",
        "batch",
        "validators",
        "chain",
    ] {
        let mut executed = executed_committed_payment(&buyer, &response, &case)?;
        if tamper == "missing" {
            executed.committed = None;
        } else {
            let committed = executed.committed.as_mut().ok_or("committed proof")?;
            match tamper {
                "receipt" => committed.proof.signed_receipt[0] ^= 1,
                "signature" => {
                    committed.proof.commit.signatures[0]
                        .signature
                        .as_mut()
                        .ok_or("commit signature")?[0] ^= 1
                }
                "number" => {
                    let SettlementRung::Committed { block_number, .. } =
                        &mut executed.evidence.rung
                    else {
                        return Err("committed rung".into());
                    };
                    *block_number += 1;
                }
                "hash" => {
                    let SettlementRung::Committed { block_hash, .. } = &mut executed.evidence.rung
                    else {
                        return Err("committed rung".into());
                    };
                    block_hash[0] ^= 1;
                }
                "batch" => committed.proof.batch_aunts[0][0] ^= 1,
                "validators" => committed.validators.clear(),
                "chain" => committed.chain_id.push('x'),
                _ => return Err("unknown tamper".into()),
            }
        }
        let (mut gateway, principal, trace) = opened_seller_gateway(&payment)?;
        assert_eq!(
            Seller::settled(
                &mut gateway,
                &principal,
                payment.idempotency_key,
                &payment.payload.accepted,
                &payment.payload.payload,
                &executed,
                buyer.receipt_keys(),
                &trace,
                1,
            ),
            Err(X402Error::EvidenceMismatch),
            "{tamper}"
        );
        assert_eq!(
            gateway.translation(&principal, payment.idempotency_key),
            Some(TranslationStatus::Pending),
            "{tamper}"
        );
    }
    Ok(())
}
