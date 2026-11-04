//! Task 22.31.165 repair record (audit 22.30.111, decision 2515).
//! Finding 1: finding_1_facilitator_evidence_crosses_every_transport.
//! Finding 2: finding_2_subscription_seller_capture_binds_grant_purpose_reference.
//! Finding 3: finding_3_identifiers_share_the_model_parser.
//! Finding 4: finding_4_consensus_requires_independent_authority_and_transitions.
//! Finding 8: finding_8_every_advertised_scheme_settles_native_output.
//! Baseline: not run; the mandatory implementation-first gate order forbids the
//! contradictory do_9 baseline run. None of these tests existed on the base.
//! Wiring request: services/interop/crates/layerx-interop-service/src (domain 11)
//! must provision independently authenticated per-network consensus checkpoints
//! and advance them with signed successors before using committed buyer capture.
//! Qualification revision, command, exit and private log are recorded by hf-done.

use std::collections::BTreeMap;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_committed_proof::{
    commit::{verify_commit, BlockIdFlag, Commit, CommitSig},
    committed::verify_committed,
    header::{
        validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator,
    },
    merkle,
    proto::{encode_length_delimited, encode_varint, encode_varint_field},
    CommittedProof,
};
use layerx_interop_gateway::{
    adapter::{AdapterId, ConformanceSuite},
    evidence::{canonical_receipt_digest, SequencerEvidence, SettlementRung},
    gateway::{TranslationKind, TranslationRequest, TranslationStatus},
    principal::PrincipalId,
    trace::TraceId,
    GatewayCore,
};
use layerx_proof::{
    inclusion::SequencerAuthorization,
    receipt::verify_sequencer_signature,
    sequencer_logs::{verify_logs, SequencerLog},
};
use layerx_wire::{encode::Encoder, limits::PROTOCOL_VERSION};
use layerx_x402::{
    buyer::{BuiltPayment, Buyer, SupportedKind, TrustedConsensusAnchor},
    facilitator::{
        Facilitator, FacilitatorCommitment, FacilitatorKind, FacilitatorRequest,
        SequencerFacilitatorPlane, SettlementIdentity, SettlementStep, SupportedResponse,
    },
    model::{
        account_identifiers, AtomicAmount, PaymentPayload, PaymentRequired, PaymentRequirements,
        ResourceInfo, SettlementResponse, X402Error, X402_VERSION,
    },
    seller::{CommittedSettlementProof, ExecutedPayment, Seller, SellerOutcome},
    transport::{
        decode_facilitator_request, decode_facilitator_settlement, decode_payment_payload,
        decode_settlement, encode_facilitator_request, encode_facilitator_settlement,
        encode_payment_payload, encode_settlement, TransportKind, TransportValue,
    },
    x402_adapter_descriptor,
};
use serde_json::json;
use sha2::{Digest as _, Sha256};

const ACCOUNT: &str = "agent:did:layerx:payee:main";
const ASSET: [u8; 32] = [0xab; 32];
const PAYER: [u8; 32] = [0x71; 32];
const REFERENCE: [u8; 32] = [0x74; 32];
const TRANSACTION: [u8; 32] = [0x61; 32];
const GRANT: [u8; 32] = [0x8a; 32];
const HEIGHT: u64 = 1000;
const CHAIN: &str = "paxeer-test-consensus";
const TRANSPORTS: [TransportKind; 3] =
    [TransportKind::Http, TransportKind::Mcp, TransportKind::A2a];

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn word(text: &str) -> [u8; 32] {
    assert_eq!(text.len(), 64);
    std::array::from_fn(|index| u8::from_str_radix(&text[index * 2..index * 2 + 2], 16).unwrap())
}

fn offer(scheme: &str) -> PaymentRequirements {
    let mut profile = json!({"account": ACCOUNT, "currency": "LXP"});
    if scheme != "exact" {
        profile["purposeHash"] = json!("7c".repeat(32));
    }
    PaymentRequirements {
        scheme: scheme.to_owned(),
        network: "eip155:125".to_owned(),
        amount: AtomicAmount::from_u128(1000),
        asset: hex(&ASSET),
        pay_to: hex(&account_identifiers(ACCOUNT).unwrap()[1]),
        max_timeout_seconds: 90,
        extra: Some(json!({"layerx": profile})),
    }
}

fn payment(accepted: PaymentRequirements, reference: [u8; 32]) -> BuiltPayment {
    let payload = PaymentPayload {
        x402_version: X402_VERSION,
        resource: None,
        accepted,
        payload: json!({
            "transaction": format!("0x{}", hex(&TRANSACTION)),
            "idempotencyKey": hex(&reference), "grantId": hex(&GRANT),
        }),
        extensions: BTreeMap::new(),
    };
    payload.validate().unwrap();
    BuiltPayment {
        header: header_value(encode_payment_payload(TransportKind::Http, &payload).unwrap()),
        payload,
        idempotency_key: reference,
    }
}

fn payment_log(scheme: &str, reference: [u8; 32]) -> SequencerLog {
    let topic = word(if scheme == "exact" {
        "c4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1"
    } else {
        "13473d103a2d52172016c872eeae0153e254af3c21a1d987492ab737f1a34fe1"
    });
    let mut address = [0; 20];
    address[18..].copy_from_slice(&[0x10, 0x1a]);
    let mut data = ASSET.to_vec();
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(&1000_u128.to_be_bytes());
    if scheme != "exact" {
        data.extend_from_slice(&reference);
        // Native PaymentDrawn's last word is contextHash, not purposeHash.
        data.extend_from_slice(&[0x9b; 32]);
    }
    SequencerLog::new(
        address,
        vec![
            topic,
            if scheme == "exact" { reference } else { GRANT },
            PAYER,
            account_identifiers(ACCOUNT).unwrap()[1],
        ],
        data,
        0,
    )
}

fn sequence_hash() -> [u8; 32] {
    let mut hash = Sha256::new();
    hash.update(b"PAXEER/v1/sequence\0");
    hash.update([0x62; 32]);
    hash.update(9_u64.to_be_bytes());
    hash.update(TRANSACTION);
    hash.finalize().into()
}

// The authority inputs go through the production canonical receipt codec and
// cryptographic verifier. Settlement messages are emitted only by Seller or
// Facilitator; there is no replacement implementation of either component.
fn encoded_receipt(logs: &[SequencerLog], signature: Option<[u8; 64]>) -> Vec<u8> {
    let mut summary = vec![0; 8 * 3 + 4 + 20 * 2];
    summary.extend_from_slice(&Sha256::digest(b""));
    summary.extend_from_slice(&0_u32.to_be_bytes());
    summary.extend_from_slice(&u32::try_from(logs.len()).unwrap().to_be_bytes());
    summary.push(4);
    summary.extend_from_slice(b"uhpx");
    let mut encoder = Encoder::new(16384);
    encoder
        .structure_header_version(0x5201, PROTOCOL_VERSION)
        .unwrap();
    encoder.u16(PROTOCOL_VERSION).unwrap();
    encoder.bytes(&TRANSACTION, 32).unwrap();
    encoder.u64(9).unwrap();
    for value in [[0x62; 32], sequence_hash(), [0x64; 32]] {
        encoder.bytes(&value, 32).unwrap();
    }
    encoder.i32(0).unwrap();
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
    encoder.sequence_length(effects.len(), 4096).unwrap();
    for (ordinal, (kind, body)) in effects.into_iter().enumerate() {
        encoder.u16(12).unwrap();
        encoder.u16(u16::try_from(ordinal).unwrap()).unwrap();
        encoder.u16(0).unwrap();
        encoder.u8(kind).unwrap();
        encoder.u8(0).unwrap();
        encoder.bytes(&[0; 32], 32).unwrap();
        encoder.bytes(&body, 256).unwrap();
    }
    let mut height = [0; 32];
    height[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    encoder.u128(1).unwrap();
    encoder.bytes(&height, 32).unwrap();
    encoder.u16(12).unwrap();
    encoder.u32(1).unwrap();
    encoder.u32(0).unwrap();
    encoder.u8(2).unwrap();
    encoder.bytes(&ASSET, 32).unwrap();
    encoder.u128(1000).unwrap();
    encoder.bytes(&PAYER, 32).unwrap();
    encoder.u128(5000).unwrap();
    encoder.u128(4000).unwrap();
    encoder.u64(1).unwrap();
    encoder
        .bytes(&account_identifiers(ACCOUNT).unwrap()[1], 32)
        .unwrap();
    encoder.u128(0).unwrap();
    encoder.u128(1000).unwrap();
    for value in [0x66, 0x67, 0x68] {
        encoder.bytes(&[value; 32], 32).unwrap();
    }
    encoder.u64(HEIGHT).unwrap();
    encoder.u8(u8::from(signature.is_some())).unwrap();
    if let Some(signature) = signature {
        encoder.bytes(&signature, 64).unwrap();
    }
    encoder.finish()
}

fn receipt_keys() -> Vec<SequencerAuthorization> {
    vec![SequencerAuthorization::new(
        [0x68; 32],
        SigningKey::from_bytes(&[0x72; 32])
            .verifying_key()
            .to_bytes(),
        HEIGHT,
        HEIGHT,
    )]
}

fn evidence(scheme: &str, reference: [u8; 32]) -> SequencerEvidence {
    let logs = vec![payment_log(scheme, reference)];
    let unsigned = encoded_receipt(&logs, None);
    let digest = layerx_wire::hash::receipt_digest(&unsigned).unwrap();
    let signature = SigningKey::from_bytes(&[0x72; 32]).sign(&digest).to_bytes();
    let signed_receipt = encoded_receipt(&logs, Some(signature));
    let decoded = layerx_wire::receipt::decode(&signed_receipt).unwrap();
    assert_eq!(
        layerx_wire::receipt::encode(&decoded).unwrap(),
        signed_receipt
    );
    let keys = receipt_keys();
    let receipt = verify_sequencer_signature(&signed_receipt, keys.as_slice()).unwrap();
    assert_eq!(verify_logs(&receipt, logs.clone()).unwrap().logs(), logs);
    SequencerEvidence {
        transaction_hash: TRANSACTION,
        signed_receipt,
        logs,
        rung: SettlementRung::Receipted,
    }
}

fn gateway() -> (GatewayCore, PrincipalId, TraceId) {
    let trace = TraceId::mint([0x73; 16]);
    let principal = PrincipalId::new("repair-buyer").unwrap();
    let suite = ConformanceSuite::new(AdapterId::new("x402-v2").unwrap(), 1, [0x75; 32]).unwrap();
    let mut gateway = GatewayCore::new();
    gateway
        .register_adapter(x402_adapter_descriptor(suite).unwrap(), &trace, 0)
        .unwrap();
    (gateway, principal, trace)
}

fn seller_settlement(
    payment: &BuiltPayment,
    evidence: SequencerEvidence,
    commitment: Option<FacilitatorCommitment>,
) -> SettlementResponse {
    let required = PaymentRequired {
        x402_version: X402_VERSION,
        error: None,
        resource: ResourceInfo {
            url: "https://merchant.example/item".to_owned(),
            description: None,
            mime_type: None,
            service_name: None,
            tags: vec![],
            icon_url: None,
        },
        accepts: vec![payment.payload.accepted.clone()],
        extensions: BTreeMap::new(),
    };
    let seller = Seller::new(required)
        .unwrap()
        .with_receipt_keys(receipt_keys());
    assert_eq!(
        seller.payment_required().unwrap().body.accepts[0],
        payment.payload.accepted
    );
    let (mut gateway, principal, trace) = gateway();
    let translation = TranslationRequest::new(
        AdapterId::new("x402").unwrap(),
        TranslationKind::StateChanging,
        payment.idempotency_key,
        [0x75; 32],
    )
    .unwrap();
    gateway
        .begin_translation(&principal, &translation, &trace, 0)
        .unwrap();
    let executed = ExecutedPayment {
        evidence,
        committed: commitment.map(|value| CommittedSettlementProof {
            proof: value.proof,
            validators: value.validators,
            chain_id: value.chain_id,
        }),
    };
    let outcome = Seller::settled(
        &mut gateway,
        &principal,
        payment.idempotency_key,
        &payment.payload.accepted,
        &payment.payload.payload,
        &executed,
        &receipt_keys(),
        &trace,
        1,
    )
    .unwrap();
    let SellerOutcome::Settled {
        response,
        receipt_digest,
        ..
    } = outcome
    else {
        panic!("settled")
    };
    assert_eq!(
        gateway.translation(&principal, payment.idempotency_key),
        Some(TranslationStatus::ReceiptVerified { receipt_digest })
    );
    response
}

fn header_value(wire: TransportValue) -> String {
    match wire {
        TransportValue::HttpHeader { value, .. } => value,
        TransportValue::Json(_) => panic!("HTTP payment messages require headers"),
    }
}

fn received(transport: TransportKind, response: &SettlementResponse) -> String {
    let wire = encode_settlement(transport, response).unwrap();
    let response = decode_settlement(transport, &wire).unwrap();
    header_value(encode_settlement(TransportKind::Http, &response).unwrap())
}

fn validators(key: &SigningKey) -> Vec<Validator> {
    vec![Validator {
        pub_key: key.verifying_key().to_bytes(),
        voting_power: 10,
    }]
}

fn header(
    chain: &str,
    height: u64,
    set: &[Validator],
    next: &[Validator],
    parent: Vec<u8>,
) -> Header {
    Header {
        version: ConsensusVersion { block: 11, app: 0 },
        chain_id: chain.to_owned(),
        height: height as i64,
        time: Timestamp {
            seconds: 1700000000 + height as i64,
            nanos: 0,
        },
        last_block_id: BlockId {
            hash: parent,
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![0x41; 32],
            },
        },
        last_commit_hash: vec![0x42; 32],
        data_hash: vec![0x43; 32],
        validators_hash: validators_hash(set),
        next_validators_hash: validators_hash(next).to_vec(),
        consensus_hash: vec![0x44; 32],
        app_hash: vec![0x45; 32],
        last_results_hash: vec![0x46; 32],
        evidence_hash: vec![0x47; 32],
        proposer_address: Sha256::digest(set[0].pub_key)[..20].to_vec(),
    }
}

fn signed_commit(header: &Header, key: &SigningKey) -> Commit {
    let block_id = BlockId {
        hash: header.hash().to_vec(),
        part_set_header: PartSetHeader {
            total: 1,
            hash: vec![0x48; 32],
        },
    };
    let mut parts = Vec::new();
    encode_varint_field(1, 1, &mut parts);
    encode_length_delimited(2, &block_id.part_set_header.hash, &mut parts);
    let mut id = Vec::new();
    encode_length_delimited(1, &block_id.hash, &mut id);
    encode_length_delimited(2, &parts, &mut id);
    let mut time = Vec::new();
    encode_varint_field(1, header.time.seconds as u64, &mut time);
    let mut vote = Vec::new();
    encode_varint_field(1, 2, &mut vote);
    vote.push(0x11);
    vote.extend_from_slice(&(header.height as u64).to_le_bytes());
    encode_length_delimited(4, &id, &mut vote);
    encode_length_delimited(5, &time, &mut vote);
    encode_length_delimited(6, header.chain_id.as_bytes(), &mut vote);
    let mut sign_bytes = Vec::new();
    encode_varint(vote.len() as u64, &mut sign_bytes);
    sign_bytes.extend(vote);
    let commit = Commit {
        height: header.height as u64,
        round: 0,
        block_id,
        signatures: vec![CommitSig {
            block_id_flag: BlockIdFlag::Commit,
            validator_address: Sha256::digest(key.verifying_key().to_bytes())[..20].to_vec(),
            timestamp: header.time,
            signature: Some(key.sign(&sign_bytes).to_bytes()),
        }],
    };
    assert!(verify_commit(
        &header.chain_id,
        &validators(key),
        header.hash(),
        header.height as u64,
        &commit
    )
    .is_ok());
    commit
}

fn commitment(
    evidence: &mut SequencerEvidence,
    chain: &str,
    key: &SigningKey,
) -> FacilitatorCommitment {
    let set = validators(key);
    let mut batch = b"PAXEER/v1/sequence-batch\0".to_vec();
    for value in [HEIGHT, 9, 9] {
        batch.extend_from_slice(&value.to_be_bytes());
    }
    for value in [
        [0x62; 32],
        sequence_hash(),
        Sha256::digest(TRANSACTION).into(),
    ] {
        batch.extend_from_slice(&value);
    }
    batch.extend_from_slice(&[0; 96]);
    let batch_digest = Sha256::digest(&batch);
    let tx_digest = Sha256::digest(TRANSACTION);
    let mut execution = header(chain, HEIGHT, &set, &set, vec![0x49; 32]);
    execution.data_hash = merkle::root(&[&batch_digest[..], &tx_digest[..]]).to_vec();
    let next = header(chain, HEIGHT + 1, &set, &set, execution.hash().to_vec());
    let commit = signed_commit(&next, key);
    let proof = CommittedProof {
        header: next,
        commit,
        execution_header: execution,
        batch_transaction: batch,
        batch_total: 2,
        batch_index: 0,
        batch_aunts: vec![merkle::root(&[&tx_digest[..]])],
        signed_receipt: evidence.signed_receipt.clone(),
        activity_ids: vec![TRANSACTION],
    };
    let digest = canonical_receipt_digest(&evidence.signed_receipt).unwrap();
    let block = verify_committed(digest, &proof, &set, chain).unwrap();
    evidence.rung = SettlementRung::Committed {
        block_number: block.number,
        block_hash: block.hash,
    };
    FacilitatorCommitment {
        proof,
        validators: set,
        chain_id: chain.to_owned(),
    }
}

fn facilitator(schemes: &[&str]) -> Facilitator {
    Facilitator::new(SupportedResponse {
        kinds: schemes
            .iter()
            .map(|scheme| FacilitatorKind {
                x402_version: X402_VERSION,
                scheme: (*scheme).to_owned(),
                network: "eip155:125".to_owned(),
                extra: None,
            })
            .collect(),
        extensions: vec![],
        signers: BTreeMap::new(),
    })
    .unwrap()
}

fn facilitator_settlement(
    transport: TransportKind,
    scheme: &str,
    committed: bool,
) -> (
    BuiltPayment,
    SettlementResponse,
    Option<TrustedConsensusAnchor>,
) {
    let facilitator = facilitator(&[scheme]);
    let (mut gateway, principal, trace) = gateway();
    let mut request = FacilitatorRequest {
        x402_version: X402_VERSION,
        payment_payload: payment(offer(scheme), REFERENCE).payload,
        payment_requirements: offer(scheme),
    };
    let stable = [0x42; 32];
    let identity =
        SettlementIdentity::derive(&principal, &request, stable, SettlementStep::Single).unwrap();
    request.payment_payload.payload["idempotencyKey"] = json!(hex(&identity.idempotency_key));
    let request = decode_facilitator_request(
        transport,
        &encode_facilitator_request(transport, &request).unwrap(),
    )
    .unwrap();
    let mut evidence = evidence(scheme, identity.idempotency_key);
    let mut plane_commitment = None;
    let mut anchor = None;
    if committed {
        let proof = commitment(&mut evidence, CHAIN, &SigningKey::from_bytes(&[0x51; 32]));
        anchor = Some(
            TrustedConsensusAnchor::new(
                "eip155:125".to_owned(),
                proof.proof.execution_header.clone(),
                proof.validators.clone(),
            )
            .unwrap(),
        );
        plane_commitment = Some(proof);
    }
    let mut plane = SequencerFacilitatorPlane::new(evidence.clone(), receipt_keys());
    if let Some(proof) = plane_commitment {
        plane = plane.with_committed_proof(proof);
    }
    let response = facilitator
        .settle(
            &mut gateway,
            &principal,
            &request,
            stable,
            SettlementStep::Single,
            &mut plane,
            &trace,
            1,
        )
        .unwrap();
    assert!(response.success);
    assert_eq!(
        gateway.translation(&principal, identity.idempotency_key),
        Some(TranslationStatus::ReceiptVerified {
            receipt_digest: canonical_receipt_digest(&evidence.signed_receipt).unwrap()
        })
    );
    let wire = encode_facilitator_settlement(transport, &response).unwrap();
    let response = decode_facilitator_settlement(transport, &wire).unwrap();
    let payload = decode_payment_payload(
        transport,
        &encode_payment_payload(transport, &request.payment_payload).unwrap(),
    )
    .unwrap();
    let payment = BuiltPayment {
        header: header_value(encode_payment_payload(TransportKind::Http, &payload).unwrap()),
        payload,
        idempotency_key: identity.idempotency_key,
    };
    (payment, response, anchor)
}

#[test]
fn finding_1_facilitator_evidence_crosses_every_transport() {
    let trace = TraceId::mint([0x77; 16]);
    for transport in TRANSPORTS {
        for committed in [false, true] {
            let (payment, response, anchor) = facilitator_settlement(transport, "exact", committed);
            let header = received(transport, &response);
            let anchors: Vec<_> = anchor.into_iter().collect();
            let captured = Buyer::capture_settlement_with_consensus(
                &header,
                &payment,
                &receipt_keys(),
                &anchors,
                &trace,
            )
            .unwrap();
            assert_eq!(captured.response, response);
            assert_eq!(captured.transaction_hash, TRANSACTION);
            assert_eq!(
                captured.rung,
                if committed { "committed" } else { "receipted" }
            );
            assert_eq!(
                captured.receipt_digest,
                canonical_receipt_digest(&captured.canonical_receipt).unwrap()
            );
            assert_eq!(
                response.extensions["layerx"]["logs"]
                    .as_array()
                    .unwrap()
                    .len(),
                1
            );
            for field in if committed {
                vec!["logs", "proof"]
            } else {
                vec!["logs"]
            } {
                let mut missing = response.clone();
                missing
                    .extensions
                    .get_mut("layerx")
                    .unwrap()
                    .as_object_mut()
                    .unwrap()
                    .remove(field);
                assert!(Buyer::capture_settlement_with_consensus(
                    &received(transport, &missing),
                    &payment,
                    &receipt_keys(),
                    &anchors,
                    &trace
                )
                .is_err());
            }
            let mut tampered = response.clone();
            tampered.extensions.get_mut("layerx").unwrap()["logs"][0]["data"] =
                json!(format!("0x{}", "00".repeat(64)));
            assert!(Buyer::capture_settlement_with_consensus(
                &received(transport, &tampered),
                &payment,
                &receipt_keys(),
                &anchors,
                &trace
            )
            .is_err());
        }
    }
}

#[test]
fn finding_2_subscription_seller_capture_binds_grant_purpose_reference() {
    let trace = TraceId::mint([0x78; 16]);
    for scheme in ["metered", "subscription"] {
        let payment = payment(offer(scheme), REFERENCE);
        let response = seller_settlement(&payment, evidence(scheme, REFERENCE), None);
        for transport in TRANSPORTS {
            let header = received(transport, &response);
            let captured =
                Buyer::capture_settlement(&header, &payment, &receipt_keys(), &trace).unwrap();
            assert_eq!(captured.response, response);
            assert_eq!(
                captured.response.extensions["layerx"]["purposeHash"],
                "7c".repeat(32)
            );
            for field in ["grant", "reference", "purpose", "scheme"] {
                let mut changed = payment.clone();
                match field {
                    "grant" => changed.payload.payload["grantId"] = json!(hex(&[0x8b; 32])),
                    "reference" => changed.idempotency_key = [0x75; 32],
                    "purpose" => {
                        changed.payload.accepted.extra.as_mut().unwrap()["layerx"]["purposeHash"] =
                            json!("7d".repeat(32))
                    }
                    "scheme" => changed.payload.accepted.scheme = "unsupported".to_owned(),
                    _ => unreachable!(),
                }
                assert!(
                    Buyer::capture_settlement(&header, &changed, &receipt_keys(), &trace).is_err(),
                    "{field}"
                );
            }
            let mut changed = response.clone();
            changed.extensions.get_mut("layerx").unwrap()["purposeHash"] = json!("7d".repeat(32));
            assert!(Buyer::capture_settlement(
                &received(transport, &changed),
                &payment,
                &receipt_keys(),
                &trace
            )
            .is_err());
            changed
                .extensions
                .get_mut("layerx")
                .unwrap()
                .as_object_mut()
                .unwrap()
                .remove("purposeHash");
            assert!(Buyer::capture_settlement(
                &received(transport, &changed),
                &payment,
                &receipt_keys(),
                &trace
            )
            .is_err());
        }
    }
}

#[test]
fn finding_3_identifiers_share_the_model_parser() {
    let trace = TraceId::mint([0x79; 16]);
    for asset_upper in [false, true] {
        for payee_upper in [false, true] {
            for asset_prefix in ["", "0x"] {
                for payee_prefix in ["", "0x"] {
                    let mut accepted = offer("exact");
                    if asset_upper {
                        accepted.asset = accepted.asset.to_ascii_uppercase();
                    }
                    if payee_upper {
                        accepted.pay_to = accepted.pay_to.to_ascii_uppercase();
                    }
                    accepted.asset.insert_str(0, asset_prefix);
                    accepted.pay_to.insert_str(0, payee_prefix);
                    accepted.validate().unwrap();
                    accepted.layerx_terms().unwrap();
                    assert_eq!(
                        accepted.layerx_facts().unwrap(),
                        (ASSET, account_identifiers(ACCOUNT).unwrap()[1])
                    );
                    let payment = payment(accepted, REFERENCE);
                    let response = seller_settlement(&payment, evidence("exact", REFERENCE), None);
                    for transport in TRANSPORTS {
                        let captured = Buyer::capture_settlement(
                            &received(transport, &response),
                            &payment,
                            &receipt_keys(),
                            &trace,
                        )
                        .unwrap();
                        assert_eq!(captured.response, response);
                    }
                }
            }
        }
    }
    for invalid in [
        "a".repeat(63),
        "a".repeat(65),
        "gg".repeat(32),
        format!("0X{}", "aa".repeat(32)),
        "é".repeat(32),
    ] {
        let mut accepted = offer("exact");
        accepted.asset = invalid;
        assert!(accepted.layerx_facts().is_err());
        let payment = payment(offer("exact"), REFERENCE);
        let response = seller_settlement(&payment, evidence("exact", REFERENCE), None);
        let mut changed = payment.clone();
        changed.payload.accepted.asset = accepted.asset;
        assert!(Buyer::capture_settlement(
            &received(TransportKind::Http, &response),
            &changed,
            &receipt_keys(),
            &trace
        )
        .is_err());
    }
}

#[test]
fn finding_4_consensus_requires_independent_authority_and_transitions() {
    let payment = payment(offer("exact"), REFERENCE);
    let trace = TraceId::mint([0x7a; 16]);
    let good_key = SigningKey::from_bytes(&[0x51; 32]);
    let mut good_evidence = evidence("exact", REFERENCE);
    let good_commitment = commitment(&mut good_evidence, CHAIN, &good_key);
    let anchor = TrustedConsensusAnchor::new(
        "eip155:125".to_owned(),
        good_commitment.proof.execution_header.clone(),
        validators(&good_key),
    )
    .unwrap();
    let response = seller_settlement(
        &payment,
        good_evidence.clone(),
        Some(good_commitment.clone()),
    );
    let buyer = Buyer::with_receipt_keys(
        vec![SupportedKind {
            scheme: "exact".to_owned(),
            network: "eip155:125".to_owned(),
        }],
        receipt_keys(),
    )
    .unwrap()
    .with_consensus_anchors(vec![anchor.clone()])
    .unwrap();
    for transport in TRANSPORTS {
        let header = received(transport, &response);
        assert_eq!(
            buyer.capture(&header, &payment, &trace).unwrap().rung,
            "committed"
        );
        assert!(Buyer::capture_settlement(&header, &payment, &receipt_keys(), &trace).is_err());
        let wrong_network = TrustedConsensusAnchor::new(
            "eip155:126".to_owned(),
            good_commitment.proof.execution_header.clone(),
            validators(&good_key),
        )
        .unwrap();
        assert!(Buyer::capture_settlement_with_consensus(
            &header,
            &payment,
            &receipt_keys(),
            &[wrong_network],
            &trace
        )
        .is_err());
        for (chain, signer) in [
            ("response-selected-chain", good_key.clone()),
            (CHAIN, SigningKey::from_bytes(&[0x52; 32])),
        ] {
            let mut hostile_evidence = evidence("exact", REFERENCE);
            let hostile_commitment = commitment(&mut hostile_evidence, chain, &signer);
            assert_eq!(
                hostile_evidence.signed_receipt,
                good_evidence.signed_receipt
            );
            let hostile_response =
                seller_settlement(&payment, hostile_evidence, Some(hostile_commitment));
            assert!(hostile_response.success);
            assert!(buyer
                .capture(&received(transport, &hostile_response), &payment, &trace)
                .is_err());
        }
        for field in ["signature", "inclusion", "coordinates"] {
            let mut changed = response.clone();
            let evidence = changed.extensions.get_mut("layerx").unwrap();
            match field {
                "signature" => {
                    evidence["proof"]["commit"]["signatures"][0]["signature"] =
                        json!(format!("0x{}", "00".repeat(64)))
                }
                "inclusion" => {
                    evidence["proof"]["batchProof"]["aunts"][0] =
                        json!(format!("0x{}", "00".repeat(32)))
                }
                "coordinates" => evidence["blockNumber"] = json!("0x3e9"),
                _ => unreachable!(),
            }
            assert!(
                buyer
                    .capture(&received(transport, &changed), &payment, &trace)
                    .is_err(),
                "{field}"
            );
        }
    }

    // Two independently signed validator changes lead from the trusted root
    // to the actual execution header. No response can choose that root.
    let old = SigningKey::from_bytes(&[0x53; 32]);
    let middle = SigningKey::from_bytes(&[0x54; 32]);
    let root = header(
        CHAIN,
        HEIGHT - 2,
        &validators(&old),
        &validators(&middle),
        vec![0x55; 32],
    );
    let intermediate = header(
        CHAIN,
        HEIGHT - 1,
        &validators(&middle),
        &validators(&good_key),
        root.hash().to_vec(),
    );
    let mut execution = good_commitment.proof.execution_header.clone();
    execution.last_block_id.hash = intermediate.hash().to_vec();
    let successor = header(
        CHAIN,
        HEIGHT + 1,
        &validators(&good_key),
        &validators(&good_key),
        execution.hash().to_vec(),
    );
    let mut moved_commitment = good_commitment;
    moved_commitment.proof.execution_header = execution.clone();
    moved_commitment.proof.header = successor.clone();
    moved_commitment.proof.commit = signed_commit(&successor, &good_key);
    let mut moved_evidence = good_evidence;
    moved_evidence.rung = SettlementRung::Committed {
        block_number: HEIGHT,
        block_hash: execution.hash(),
    };
    let moved_response = seller_settlement(&payment, moved_evidence, Some(moved_commitment));
    let mut checkpoint =
        TrustedConsensusAnchor::new("eip155:125".to_owned(), root.clone(), validators(&old))
            .unwrap();
    let capture = |checkpoint: &TrustedConsensusAnchor| {
        Buyer::capture_settlement_with_consensus(
            &received(TransportKind::Http, &moved_response),
            &payment,
            &receipt_keys(),
            &[checkpoint.clone()],
            &trace,
        )
    };
    assert!(capture(&checkpoint).is_err());
    assert!(checkpoint
        .advance(
            execution.clone(),
            &signed_commit(&execution, &good_key),
            validators(&good_key)
        )
        .is_err());
    let rogue_key = SigningKey::from_bytes(&[0x56; 32]);
    let rogue = header(
        CHAIN,
        HEIGHT - 1,
        &validators(&rogue_key),
        &validators(&good_key),
        root.hash().to_vec(),
    );
    assert!(checkpoint
        .advance(
            rogue.clone(),
            &signed_commit(&rogue, &rogue_key),
            validators(&rogue_key)
        )
        .is_err());
    let mut bad_commit = signed_commit(&intermediate, &middle);
    bad_commit.signatures[0].signature.as_mut().unwrap()[0] ^= 1;
    assert!(checkpoint
        .advance(intermediate.clone(), &bad_commit, validators(&middle))
        .is_err());
    checkpoint
        .advance(
            intermediate.clone(),
            &signed_commit(&intermediate, &middle),
            validators(&middle),
        )
        .unwrap();
    assert!(capture(&checkpoint).is_err());
    checkpoint
        .advance(
            execution.clone(),
            &signed_commit(&execution, &good_key),
            validators(&good_key),
        )
        .unwrap();
    assert_eq!(capture(&checkpoint).unwrap().rung, "committed");
    checkpoint
        .advance(
            successor.clone(),
            &signed_commit(&successor, &good_key),
            validators(&good_key),
        )
        .unwrap();
    assert_eq!(capture(&checkpoint).unwrap().rung, "committed");
}

#[test]
fn finding_8_every_advertised_scheme_settles_native_output() {
    let trace = TraceId::mint([0x7b; 16]);
    let supported = facilitator(&["exact", "metered", "subscription"]).supported();
    assert_eq!(supported.kinds.len(), 3);
    for kind in supported.kinds {
        for transport in TRANSPORTS {
            for committed in [false, true] {
                let (payment, response, anchor) =
                    facilitator_settlement(transport, &kind.scheme, committed);
                let anchors: Vec<_> = anchor.into_iter().collect();
                let captured = Buyer::capture_settlement_with_consensus(
                    &received(transport, &response),
                    &payment,
                    &receipt_keys(),
                    &anchors,
                    &trace,
                )
                .unwrap();
                assert_eq!(captured.response, response);
                if kind.scheme != "exact" {
                    assert_eq!(
                        response.extensions["layerx"]["purposeHash"],
                        "7c".repeat(32)
                    );
                }
            }
        }
    }
    let unsupported = SupportedResponse {
        kinds: vec![FacilitatorKind {
            x402_version: X402_VERSION,
            scheme: "escrow".to_owned(),
            network: "eip155:125".to_owned(),
            extra: None,
        }],
        extensions: vec![],
        signers: BTreeMap::new(),
    };
    assert_eq!(
        Facilitator::new(unsupported).err(),
        Some(X402Error::UnsupportedOffer)
    );

    // Real relay output with a different grant or reference must be refused
    // before the gateway records a verified settlement.
    for mismatch in ["grant", "reference", "event", "missing-proof"] {
        let (mut gateway, principal, trace) = gateway();
        let facilitator = facilitator(&["subscription"]);
        let mut request = FacilitatorRequest {
            x402_version: X402_VERSION,
            payment_payload: payment(offer("subscription"), REFERENCE).payload,
            payment_requirements: offer("subscription"),
        };
        let identity =
            SettlementIdentity::derive(&principal, &request, REFERENCE, SettlementStep::Single)
                .unwrap();
        let mut produced = evidence(
            if mismatch == "event" {
                "exact"
            } else {
                "subscription"
            },
            if mismatch == "reference" {
                [0x76; 32]
            } else {
                identity.idempotency_key
            },
        );
        if mismatch == "grant" {
            request.payment_payload.payload["grantId"] = json!(hex(&[0x8b; 32]));
        }
        if mismatch == "missing-proof" {
            commitment(&mut produced, CHAIN, &SigningKey::from_bytes(&[0x51; 32]));
        }
        let mut plane = SequencerFacilitatorPlane::new(produced, receipt_keys());
        assert!(
            facilitator
                .settle(
                    &mut gateway,
                    &principal,
                    &request,
                    REFERENCE,
                    SettlementStep::Single,
                    &mut plane,
                    &trace,
                    1
                )
                .is_err(),
            "{mismatch}"
        );
        assert_eq!(
            gateway.translation(&principal, identity.idempotency_key),
            Some(TranslationStatus::Pending)
        );
    }
}
