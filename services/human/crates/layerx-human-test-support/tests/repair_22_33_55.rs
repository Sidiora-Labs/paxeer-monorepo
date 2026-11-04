use std::fs::{self, OpenOptions};
use std::io::Write as _;
use std::net::TcpListener;
use std::path::PathBuf;
use std::time::Duration;

use ed25519_dalek::SigningKey;
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_client::sequencer::SequencerReceipt;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_human_test_support::evidence_node::{AdmissionJournal, AdmissionRecord, EvidenceNode};
use layerx_human_test_support::{self as support, EVIDENCE_NETWORK_ID};

#[path = "support/committed_evidence.rs"]
mod committed_evidence;

const README: &str = include_str!("../README.md");

struct Directory(PathBuf);

impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove admission fixture directory");
    }
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
    .expect("production EIP-1559 transaction encoder")
}

fn listener() -> (TcpListener, NodeRpcConfig) {
    let listener = TcpListener::bind("127.0.0.1:0").expect("loopback HTTP fixture");
    let config = NodeRpcConfig {
        url: format!(
            "http://{}/",
            listener.local_addr().expect("fixture address")
        ),
        chain_id: 125,
        deadline: Duration::from_secs(2),
    };
    (listener, config)
}

#[test]
fn finding18_documented_http_admission_stays_submitted_after_journal_recovery() {
    for description in [
        "HTTP JSON-RPC",
        "Submitted-only receipt replies",
        "journal recovery",
        "rung: \"submitted\"",
        "truncates",
        "admission.journal",
    ] {
        assert!(
            README.contains(description),
            "missing documentation: {description}"
        );
    }
    assert!(!README.contains("Unix"), "retired transport description");

    let directory = Directory(support::directory("repair223355-admission"));
    let signer = SigningKey::from_bytes(&[0x84; 32]);
    let first = signed_transaction(1);
    let first_hash = transaction_hash(&first);
    let (transaction, signature) = decode_signed(&first).expect("real signed transaction");
    let sender = recover_sender(&transaction, &signature).expect("real recovered sender");
    let mut producer = EvidenceNode::new(&directory.0, EVIDENCE_NETWORK_ID, &signer, 7);
    producer.register_sender(sender);
    let (tcp, config) = listener();
    let server = producer.serve(tcp);
    let node = NodeRpc::connect(&config).expect("production HTTP JSON-RPC consumer");
    let keys = node.receipt_keys().expect("fixture receipt keys");
    assert_eq!(keys.len(), 1);
    assert_eq!(keys[0].public_key, signer.verifying_key().to_bytes());
    assert_eq!(node.sequencer_receipt(&first_hash), Ok(None));
    for _ in 0..2 {
        assert_eq!(node.send_raw_transaction(&first), Ok(first_hash));
        assert_eq!(
            node.sequencer_receipt(&first_hash),
            Ok(Some(SequencerReceipt::Submitted {
                tx_hash: first_hash,
                nonce: 1,
            }))
        );
    }
    drop(node);
    let producer = server.join().expect("stop bounded admission server");
    assert_eq!(
        producer.served(),
        &[
            "eth_chainId",
            "px_getReceiptKeys",
            "pax_sequencerReceipt",
            "eth_sendRawTransaction",
            "pax_sequencerReceipt",
            "eth_sendRawTransaction",
            "pax_sequencerReceipt",
        ]
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

    let path = directory.0.join("admission.journal");
    let valid_length = fs::metadata(&path).expect("persisted journal").len();
    let mut interrupted = OpenOptions::new()
        .append(true)
        .open(&path)
        .expect("journal tail");
    interrupted
        .write_all(b"LXAR\0\0\0")
        .expect("torn next record");
    interrupted.sync_data().expect("persist interrupted append");
    drop(interrupted);

    let second = signed_transaction(2);
    let second_hash = transaction_hash(&second);
    let mut reopened = EvidenceNode::new(&directory.0, EVIDENCE_NETWORK_ID, &signer, 5);
    assert_eq!(
        fs::metadata(&path).expect("recovered journal").len(),
        valid_length
    );
    assert!(reopened.journal().contains(&first_hash));
    assert_eq!(reopened.journal().records(), &[expected.clone()]);
    reopened.register_sender(sender);
    let (tcp, config) = listener();
    let server = reopened.serve(tcp);
    let node = NodeRpc::connect(&config).expect("reopened real HTTP fixture");
    assert_eq!(
        node.sequencer_receipt(&first_hash),
        Ok(Some(SequencerReceipt::Submitted {
            tx_hash: first_hash,
            nonce: 1,
        }))
    );
    assert_eq!(node.send_raw_transaction(&first), Ok(first_hash));
    assert_eq!(node.send_raw_transaction(&second), Ok(second_hash));
    assert_eq!(
        node.sequencer_receipt(&second_hash),
        Ok(Some(SequencerReceipt::Submitted {
            tx_hash: second_hash,
            nonce: 2,
        }))
    );
    drop(node);
    let reopened = server.join().expect("stop reopened server");
    let expected_records = vec![
        expected,
        AdmissionRecord {
            tx_hash: second_hash,
            raw_transaction: second,
        },
    ];
    assert_eq!(reopened.journal().records(), expected_records);
    drop(reopened);
    assert_eq!(
        AdmissionJournal::recover(&directory.0, EVIDENCE_NETWORK_ID),
        expected_records
    );
}

#[test]
fn finding18_documented_committed_proof_uses_separate_real_node_recording() {
    for description in [
        "separate real-node proof",
        "proof_human_test_support_real_committed_receipt_recovers_after_human_restart",
        "tests/support/committed_evidence.rs",
        "recorded `paxd` receipt",
        "produce `Committed` evidence",
    ] {
        assert!(
            README.contains(description),
            "missing evidence limit: {description}"
        );
    }
    committed_evidence::committed_receipt_crosses_rpc_and_recovers();
    committed_evidence::recorded_malformed_and_foreign_evidence_is_refused();
}
