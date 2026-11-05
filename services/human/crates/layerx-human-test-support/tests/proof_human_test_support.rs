//! Proves: EvidenceNode::serve, raw_receipt_evidence, evidence_gate, AdmissionRecord -> HTTP JSON-RPC / public Rust evidence -> NodeRpc, Gate::readiness_gate, EvidenceAuthority::verify_receipt, AdmissionJournal::recover.
//!
//! The current consumers in agent parity, boundary and client sequencer tests use
//! JSON-RPC, replacing the indexed UnixListener/LNI serve interface. This proof
//! uses that current crossing and its actual decoders, without reviving LNI.
//! The committed receipt proof below uses the recorded paxd chain and reopens
//! the human evidence store; the locally constructed proofs cover codecs only.
//! The journey_faults, move_money and spend consumers construct raw receipt
//! evidence and verify it under the readiness gate's pinned authority identically.
//!
//! Repairs evidence_node.rs::AdmissionJournal::open: recovery discarded an invalid
//! tail logically but left it on disk, hiding every subsequent appended record.
//! The stop/reopen and second admission below prove truncation before append.
//! Repairs evidence_node.rs::EvidenceNode::result: px_getReceiptKeys must emit
//! the keys object with numeric height ranges required by the client's decoder.
//! lib.rs::serve_readiness_node now uses EvidenceNode::serve rather than a second
//! scripted HTTP implementation. No environment-provisioned producer is required.
//! A truncated producer header is refused by EvidenceAuthority::verify_receipt
//! at its canonical decoder, independently of the receipt-signature refusal.

use std::fs::{self, OpenOptions};
use std::io::Write as _;
use std::net::TcpListener;
use std::path::PathBuf;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::boot::{GateError, Status, WriteGateError};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::protocol_evidence::{ReceiptEvidenceError, VerifierPolicyError};
use layerx_client::rpc::submit::{submit_signed, Submission, SubmitError};
use layerx_client::sequencer::{SequencerEndpoint, SequencerReceipt};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_human_test_support::evidence_node::{AdmissionJournal, AdmissionRecord, EvidenceNode};
use layerx_human_test_support::{self as support, EVIDENCE_NETWORK_ID};
use layerx_intents::canonical::{decode_batch_header, decode_receipt};
use layerx_intents::vectors::{receipt, receipt_signing_digest};
use layerx_types::result::KnownResult;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;

// This is the boundary consumer's own function, also called by its conformance suite.
#[path = "../../../../agent/tests/boundary/cases.rs"]
mod boundary;
#[path = "../../../../agent/tests/boundary/gaps.rs"]
mod gaps;

struct Directory(PathBuf);
impl Directory {
    fn new(label: &str) -> Self {
        Self(support::directory(label))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("proof_human_test_support.rs: remove producer journal");
    }
}

fn listener() -> (TcpListener, NodeRpcConfig) {
    let listener = TcpListener::bind("127.0.0.1:0")
        .expect("evidence_node.rs::serve: local producer listener required");
    let config = NodeRpcConfig {
        url: format!(
            "http://{}/",
            listener.local_addr().expect("producer address")
        ),
        chain_id: 125,
        deadline: Duration::from_secs(2),
    };
    (listener, config)
}

fn signed_transaction(nonce: u64) -> Vec<u8> {
    Eip1559Call {
        chain_id: 125,
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100_000,
        to: [0x12; 20],
        data: Vec::new(),
    }
    .sign(&[0x52; 32])
    .expect("layerx-crypto::Eip1559Call::sign: real producer signature")
}

fn sender(raw: &[u8]) -> [u8; 20] {
    let (transaction, signature) = decode_signed(raw).expect("signed transaction producer");
    recover_sender(&transaction, &signature).expect("authenticated producer sender")
}

#[test]
fn proof_human_test_support_admission_crosses_rpc_and_recovers_after_torn_tail() {
    let directory = Directory::new("proof-admission");
    let signer = SigningKey::from_bytes(&[0x84; 32]);
    let first = signed_transaction(1);
    let first_hash = transaction_hash(&first);
    let mut producer = EvidenceNode::new(&directory.0, EVIDENCE_NETWORK_ID, &signer, 9);
    producer.register_sender(sender(&first));
    let (tcp, config) = listener();
    let server = producer.serve(tcp);
    let endpoint = SequencerEndpoint::new(&config.url, &[]).expect("real sequencer consumer");
    // Each iteration reaches the producer twice: submit and receipt lookup.
    for _ in 0..2 {
        assert_eq!(
            submit_signed(&endpoint, &first),
            Ok(Submission::Submitted {
                tx_hash: first_hash,
                rung: Rung::Submitted,
            })
        );
        assert_eq!(
            endpoint.sequencer_receipt(&first_hash),
            Ok(Some(SequencerReceipt::Submitted {
                tx_hash: first_hash,
                nonce: 1,
            }))
        );
    }
    boundary::verify_human_test_support_refusals(&endpoint)
        .expect("real boundary refusal consumer");
    let stranger = Eip1559Call {
        chain_id: 125,
        nonce: 2,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100_000,
        to: [0x12; 20],
        data: Vec::new(),
    }
    .sign(&[0x53; 32])
    .expect("foreign signer");
    assert!(
        matches!(submit_signed(&endpoint, &stranger), Err(SubmitError::Refused { result, .. })
        if result.raw() == KnownResult::UnknownDid.raw())
    );
    let producer = server.join().expect("evidence_node.rs::serve must finish");
    assert_eq!(producer.served().len(), 9);
    assert_eq!(
        producer
            .served()
            .iter()
            .filter(|method| *method == "eth_sendRawTransaction")
            .count(),
        5
    );
    let expected = AdmissionRecord {
        tx_hash: first_hash,
        raw_transaction: first.clone(),
    };
    assert_eq!(producer.journal().records(), &[expected.clone()]);
    drop(producer);
    assert_eq!(
        AdmissionJournal::recover(&directory.0, EVIDENCE_NETWORK_ID),
        vec![expected.clone()]
    );

    // An interrupted next record must never hide later authenticated admissions.
    let mut file = OpenOptions::new()
        .append(true)
        .open(directory.0.join("admission.journal"))
        .expect("producer journal");
    file.write_all(b"LXAR\0\0\0")
        .expect("interrupt producer append");
    file.sync_data().expect("persist torn producer tail");
    drop(file);
    assert_eq!(
        AdmissionJournal::recover(&directory.0, EVIDENCE_NETWORK_ID),
        vec![expected.clone()]
    );
    let second = signed_transaction(2);
    let second_hash = transaction_hash(&second);
    let mut reopened = EvidenceNode::new(&directory.0, EVIDENCE_NETWORK_ID, &signer, 4);
    reopened.register_sender(sender(&second));
    let (tcp, config) = listener();
    let server = reopened.serve(tcp);
    let endpoint = SequencerEndpoint::new(&config.url, &[]).expect("reopened sequencer consumer");
    assert_eq!(
        endpoint.sequencer_receipt(&first_hash),
        Ok(Some(SequencerReceipt::Submitted {
            tx_hash: first_hash,
            nonce: 1
        }))
    );
    assert_eq!(
        submit_signed(&endpoint, &first),
        Ok(Submission::Submitted {
            tx_hash: first_hash,
            rung: Rung::Submitted
        })
    );
    assert_eq!(
        submit_signed(&endpoint, &second),
        Ok(Submission::Submitted {
            tx_hash: second_hash,
            rung: Rung::Submitted
        })
    );
    assert_eq!(
        endpoint.sequencer_receipt(&second_hash),
        Ok(Some(SequencerReceipt::Submitted {
            tx_hash: second_hash,
            nonce: 2
        }))
    );
    let reopened = server.join().expect("reopened producer must finish");
    assert_eq!(reopened.journal().records().len(), 2);
    drop(reopened);
    let records = AdmissionJournal::recover(&directory.0, EVIDENCE_NETWORK_ID);
    assert_eq!(
        records,
        vec![
            expected.clone(),
            AdmissionRecord {
                tx_hash: second_hash,
                raw_transaction: second
            }
        ]
    );
    let reopened = AdmissionJournal::open(&directory.0, EVIDENCE_NETWORK_ID);
    assert!(reopened.contains(&first_hash));
    assert!(reopened.contains(&second_hash));
    drop(reopened);
    let journal_path = directory.0.join("admission.journal");
    let mut malformed_record = fs::read(&journal_path).expect("real persisted producer records");
    *malformed_record
        .last_mut()
        .expect("producer record checksum") ^= 1;
    fs::write(&journal_path, malformed_record).expect("corrupt producer checksum");
    assert_eq!(
        AdmissionJournal::recover(&directory.0, EVIDENCE_NETWORK_ID),
        vec![expected]
    );
}

fn signed_receipt(signer: &SigningKey) -> Vec<u8> {
    // The repository's canonical receipt encoder is the producer; only its
    // committed batch identifier is bound to this execution before signing.
    let encode = |signature| {
        let mut bytes = receipt(signature).expect("layerx-intents::vectors::receipt producer");
        let decoded = decode_receipt(&bytes).expect("canonical producer receipt");
        let protocol = decoded.protocol().expect("protocol receipt");
        let batch = support::committed_execution_batch_id(
            protocol.previous_state_root(),
            protocol.activity_root(),
            protocol.global_sequence(),
        );
        let positions: Vec<_> = bytes
            .windows(32)
            .enumerate()
            .filter_map(|(index, field)| (field == protocol.batch_id()).then_some(index))
            .collect();
        assert_eq!(positions.len(), 1);
        bytes[positions[0]..positions[0] + 32].copy_from_slice(&batch);
        bytes
    };
    let unsigned = encode(None);
    let digest = receipt_signing_digest(&unsigned).expect("real receipt signing digest");
    encode(Some(signer.sign(&digest).to_bytes()))
}

fn evidence(bytes: Vec<u8>, signer: &SigningKey) -> support::RawReceiptEvidence {
    let decoded = decode_receipt(&bytes).expect("producer canonical receipt");
    let protocol = decoded.protocol().expect("producer protocol receipt");
    let previous = protocol.previous_state_root();
    let resulting = protocol.resulting_state_root();
    let sequence = protocol.global_sequence();
    support::raw_receipt_evidence(
        bytes,
        signer.verifying_key().to_bytes(),
        previous,
        resulting,
        sequence,
        signer,
    )
}

#[test]
fn proof_human_test_support_gate_and_receipts_are_reobserved_and_refuse_foreign_or_malformed_results(
) {
    let directory = Directory::new("proof-readiness");
    let signer = SigningKey::from_bytes(&[0x84; 32]);
    let foreign = SigningKey::from_bytes(&[0x85; 32]);
    let mut gate = support::evidence_gate(&signer);
    assert_eq!(gate.status(), &Status::NotReady);
    assert_eq!(gate.guard_write(|| ()), Err(WriteGateError::NotReady));
    // Both readiness generations decode a new result from the real producer.
    let producer = EvidenceNode::new(&directory.0, EVIDENCE_NETWORK_ID, &signer, 3);
    let (tcp, config) = listener();
    let server = producer.serve(tcp);
    let node = NodeRpc::connect(&config).expect("NodeRpc must connect to in-process producer");
    for generation in 1..=2 {
        let ready = gate.readiness_gate(&node).expect("real readiness consumer");
        assert_eq!(ready.generation, generation);
        assert_eq!(ready.chain_id, 125);
        assert_eq!(ready.receipt_keys.len(), 1);
        assert_eq!(ready.receipt_keys[0].first_height, 7);
        assert_eq!(ready.receipt_keys[0].last_height, 7);
        assert_eq!(
            ready.receipt_keys[0].public_key,
            signer.verifying_key().to_bytes()
        );
        assert_eq!(gate.guard_write(|| generation), Ok(generation));
    }
    let producer = server.join().expect("readiness producer must finish");
    assert_eq!(
        producer.served(),
        &["eth_chainId", "px_getReceiptKeys", "px_getReceiptKeys"]
    );
    drop(producer);
    let authority = gate
        .evidence_authority()
        .expect("gated evidence authority")
        .clone();
    for _ in 0..2 {
        let raw = evidence(signed_receipt(&signer), &signer);
        let verified = authority
            .verify_receipt(&raw)
            .expect("real service receipt consumer");
        assert_eq!(verified.activity_id(), [1; 32]);
        assert_eq!(verified.global_sequence(), 9);
        assert_eq!(verified.amount(), 25);
        assert_eq!(verified.result_code(), 0);
        assert_eq!(verified.level(), VerificationLevel::BATCH_INCLUDED);
        assert_eq!(verified.canonical_receipt(), raw.canonical_receipt());
        let header = decode_batch_header(raw.canonical_header()).expect("consumer header decode");
        assert_eq!(header.sequencer_id(), signer.verifying_key().to_bytes());
    }
    let raw = evidence(signed_receipt(&foreign), &foreign);
    assert_eq!(
        authority.verify_receipt(&raw),
        Err(ReceiptEvidenceError::Policy(
            VerifierPolicyError::UnknownSequencer
        ))
    );
    let mut malformed = signed_receipt(&signer);
    *malformed.last_mut().expect("producer receipt signature") ^= 1;
    // The public producer emits a correctly signed inclusion proof over the
    // damaged receipt; the actual receipt verifier must decode and refuse it.
    let malformed = evidence(malformed, &signer);
    assert!(matches!(
        authority.verify_receipt(&malformed),
        Err(ReceiptEvidenceError::Receipt(_))
    ));
    let raw = evidence(signed_receipt(&signer), &signer);
    let mut truncated_header = raw.canonical_header().to_vec();
    truncated_header.pop().expect("producer header bytes");
    let malformed = support::RawReceiptEvidence::new(
        raw.canonical_receipt().to_vec(),
        raw.proof().clone(),
        truncated_header,
        raw.header_signature(),
    );
    assert_eq!(
        authority.verify_receipt(&malformed),
        Err(ReceiptEvidenceError::Policy(
            VerifierPolicyError::HeaderDecode
        ))
    );
    gate.disconnected();
    assert_eq!(gate.guard_write(|| ()), Err(WriteGateError::NotReady));
    let (config, server) = support::serve_readiness_node(&foreign);
    let foreign_node = NodeRpc::connect(&config).expect("foreign in-process producer");
    assert_eq!(
        gate.readiness_gate(&foreign_node),
        Err(GateError::Evidence(VerifierPolicyError::HandshakeKey))
    );
    assert_eq!(gate.status(), &Status::NotReady);
    assert_eq!(gate.guard_write(|| ()), Err(WriteGateError::NotReady));
    server.join().expect("foreign producer must finish");
}

#[path = "support/committed_evidence.rs"]
mod committed_evidence;

#[test]
fn proof_human_test_support_real_committed_receipt_recovers_after_human_restart() {
    committed_evidence::committed_receipt_crosses_rpc_and_recovers();
    committed_evidence::recorded_malformed_and_foreign_evidence_is_refused();
}
