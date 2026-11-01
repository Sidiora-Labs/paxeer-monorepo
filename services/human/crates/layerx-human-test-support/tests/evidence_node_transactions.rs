use ed25519_dalek::SigningKey;
use layerx_client::lni::handshake::decode_node_info;
use layerx_client::lni::schema::{decode_envelope, encode_envelope, Envelope, Version};
use layerx_client::lni::transport::{FrameTransport, Uds};
use layerx_crypto::evm_transaction::{decode_signed, transaction_hash, Eip1559Call};
use layerx_human_test_support::evidence_node::{AdmissionJournal, AdmissionRecord, EvidenceNode};
use layerx_human_test_support::{
    connect_evidence_node, evidence_node, evidence_registry, serve_evidence_node,
    EVIDENCE_NETWORK_ID,
};
use layerx_types::result::KnownResult;

fn call() -> Eip1559Call {
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1_000_000,
        max_fee_per_gas: 100_000_000,
        gas_limit: 100_000,
        to: [0x12; 20],
        data: vec![0x01, 0x02, 0x03, 0x04],
    }
}

fn exchange(client: &mut Uds, tag: u16, correlation_id: u64, payload: &[u8]) -> Vec<u8> {
    let request = encode_envelope(Envelope {
        version: Version::V1_3,
        message_tag: tag,
        correlation_id,
        canonical_payload: payload,
        proof_material: &[],
    })
    .unwrap_or_else(|error| panic!("encode request: {error:?}"));
    client
        .send(&request)
        .unwrap_or_else(|error| panic!("send request: {error:?}"));
    client
        .receive()
        .unwrap_or_else(|error| panic!("receive response: {error:?}"))
}

fn handshake(client: &mut Uds, signer: &SigningKey) {
    let response = exchange(client, 1, 0, &[]);
    let envelope =
        decode_envelope(&response).unwrap_or_else(|error| panic!("decode handshake: {error:?}"));
    assert_eq!(envelope.message_tag, 2);
    assert_eq!(envelope.correlation_id, 0);
    assert!(envelope.proof_material.is_empty());
    let info = decode_node_info(envelope.canonical_payload)
        .unwrap_or_else(|error| panic!("decode node info: {error:?}"));
    assert_eq!(info.network_id, EVIDENCE_NETWORK_ID);
    assert_eq!(
        info.authorised_sequencer_key,
        signer.verifying_key().to_bytes()
    );
}

fn assert_admitted(client: &mut Uds, correlation_id: u64, raw: &[u8], tx_hash: &[u8; 32]) {
    let response = exchange(client, 3, correlation_id, raw);
    let envelope = decode_envelope(&response)
        .unwrap_or_else(|error| panic!("decode submission response: {error:?}"));
    assert_eq!(envelope.version, Version::V1_3);
    assert_eq!(envelope.message_tag, 4);
    assert_eq!(envelope.correlation_id, correlation_id);
    assert_eq!(envelope.canonical_payload, raw);
    assert_eq!(envelope.proof_material, tx_hash);
}

#[test]
fn evidence_node_admits_signed_type2_transaction() {
    let signer = SigningKey::from_bytes(&[0x86; 32]);
    let call = call();
    let raw_transaction = call
        .sign(&[0x37; 32])
        .unwrap_or_else(|error| panic!("sign transaction: {error}"));
    assert_eq!(raw_transaction[0], 2);
    let (decoded, _) = decode_signed(&raw_transaction)
        .unwrap_or_else(|error| panic!("decode signed transaction: {error}"));
    assert_eq!(decoded, call);
    let tx_hash = transaction_hash(&raw_transaction);
    assert_ne!(tx_hash, call.signing_hash());
    let expected = AdmissionRecord {
        tx_hash,
        raw_transaction: raw_transaction.clone(),
    };
    let node = evidence_node(&signer, "transactions-signed-journal", 1);
    let journal_directory = node.journal().directory().to_path_buf();
    let (socket_path, server) = serve_evidence_node(node, "transactions-signed");
    let mut client = connect_evidence_node(&socket_path);
    handshake(&mut client, &signer);
    assert_admitted(&mut client, 1, &raw_transaction, &tx_hash);
    assert_admitted(&mut client, 2, &raw_transaction, &tx_hash);
    drop(client);
    let node = server
        .join()
        .unwrap_or_else(|_| panic!("evidence node worker panicked"));
    assert!(!node.fail_stopped());
    assert_eq!(node.authentication_refusals(), 0);
    assert_eq!(node.queued(), vec![tx_hash]);
    assert!(node.journal().contains(&tx_hash));
    assert!(!node.journal().contains(&call.signing_hash()));
    assert_eq!(node.journal().records(), std::slice::from_ref(&expected));
    drop(node);
    std::fs::remove_file(socket_path).unwrap_or_else(|error| panic!("remove socket: {error}"));

    let restarted = EvidenceNode::new(
        signer.verifying_key().to_bytes(),
        EVIDENCE_NETWORK_ID,
        evidence_registry(),
        &journal_directory,
        1,
    );
    assert!(restarted.journal().contains(&tx_hash));
    assert_eq!(
        restarted.journal().records(),
        std::slice::from_ref(&expected)
    );
    let (socket_path, server) = serve_evidence_node(restarted, "transactions-restarted");
    let mut client = connect_evidence_node(&socket_path);
    handshake(&mut client, &signer);
    assert_admitted(&mut client, 3, &raw_transaction, &tx_hash);
    drop(client);
    let node = server
        .join()
        .unwrap_or_else(|_| panic!("restarted evidence node worker panicked"));
    assert!(!node.fail_stopped());
    assert!(node.queued().is_empty());
    assert_eq!(node.journal().records(), std::slice::from_ref(&expected));
    drop(node);
    assert_eq!(
        AdmissionJournal::recover(&journal_directory, EVIDENCE_NETWORK_ID),
        vec![expected]
    );
    std::fs::remove_file(socket_path).unwrap_or_else(|error| panic!("remove socket: {error}"));
    std::fs::remove_dir_all(journal_directory)
        .unwrap_or_else(|error| panic!("remove journal: {error}"));
}

#[test]
fn evidence_node_refuses_unsigned_transaction() {
    let signer = SigningKey::from_bytes(&[0x87; 32]);
    let call = call();
    let signed = call
        .sign(&[0x38; 32])
        .unwrap_or_else(|error| panic!("sign transaction: {error}"));
    let mut trailing = signed.clone();
    trailing.push(0);
    let mut wrong_type = signed.clone();
    wrong_type[0] = 1;
    let invalid = [
        call.encode_unsigned(),
        signed[..signed.len() - 1].to_vec(),
        trailing,
        wrong_type,
    ];
    let node = evidence_node(&signer, "transactions-unsigned-journal", 1);
    let journal_directory = node.journal().directory().to_path_buf();
    let (socket_path, server) = serve_evidence_node(node, "transactions-unsigned");
    let mut client = connect_evidence_node(&socket_path);
    handshake(&mut client, &signer);
    for (index, raw) in invalid.iter().enumerate() {
        assert!(decode_signed(raw).is_err());
        let correlation_id =
            u64::try_from(index + 1).unwrap_or_else(|error| panic!("correlation id: {error}"));
        let response = exchange(&mut client, 3, correlation_id, raw);
        let envelope =
            decode_envelope(&response).unwrap_or_else(|error| panic!("decode refusal: {error:?}"));
        assert_eq!(envelope.message_tag, 25);
        assert_eq!(envelope.correlation_id, correlation_id);
        let mut refusal = vec![4];
        refusal.extend_from_slice(&KnownResult::MalformedEnvelope.raw().to_be_bytes());
        assert_eq!(envelope.canonical_payload, refusal);
        assert!(envelope.proof_material.is_empty());
    }
    drop(client);
    let node = server
        .join()
        .unwrap_or_else(|_| panic!("evidence node worker panicked"));
    assert!(!node.fail_stopped());
    assert!(node.queued().is_empty());
    assert!(node.journal().records().is_empty());
    for raw in &invalid {
        assert!(!node.journal().contains(&transaction_hash(raw)));
    }
    drop(node);
    assert!(AdmissionJournal::recover(&journal_directory, EVIDENCE_NETWORK_ID).is_empty());
    let restarted = EvidenceNode::new(
        signer.verifying_key().to_bytes(),
        EVIDENCE_NETWORK_ID,
        evidence_registry(),
        &journal_directory,
        1,
    );
    assert!(restarted.journal().records().is_empty());
    assert!(restarted.queued().is_empty());
    drop(restarted);
    std::fs::remove_file(socket_path).unwrap_or_else(|error| panic!("remove socket: {error}"));
    std::fs::remove_dir_all(journal_directory)
        .unwrap_or_else(|error| panic!("remove journal: {error}"));
}
