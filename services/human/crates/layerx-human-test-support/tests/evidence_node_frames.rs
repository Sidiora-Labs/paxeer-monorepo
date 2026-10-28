use std::io::{Read as _, Write as _};
use std::os::unix::net::UnixStream;
use std::time::Duration;

use ed25519_dalek::SigningKey;
use layerx_client::lni::handshake::{decode_node_info, NodeRole};
use layerx_client::lni::schema::{decode_envelope, encode_envelope, Envelope, Version};
use layerx_client::lni::transport::{FrameTransport, Uds};
use layerx_human_service::frame::{read_frame, write_frame};
use layerx_human_test_support::evidence_node::FRAME_BYTES;
use layerx_human_test_support::{
    connect_evidence_node, evidence_node, serve_evidence_node, EVIDENCE_NETWORK_ID,
};
use layerx_intents::canonical::PROTOCOL_VERSION;
use layerx_types::result::KnownResult;

fn handshake_request() -> Vec<u8> {
    encode_envelope(Envelope {
        version: Version::V1_3,
        message_tag: 1,
        correlation_id: 0,
        canonical_payload: &[],
        proof_material: &[],
    })
    .unwrap_or_else(|error| panic!("encode handshake: {error:?}"))
}

fn assert_handshake(response: &[u8], signer: &SigningKey) {
    let envelope = decode_envelope(response)
        .unwrap_or_else(|error| panic!("decode handshake envelope: {error:?}"));
    assert_eq!(envelope.version, Version::V1_3);
    assert_eq!(envelope.message_tag, 2);
    assert_eq!(envelope.correlation_id, 0);
    assert!(envelope.proof_material.is_empty());
    let info = decode_node_info(envelope.canonical_payload)
        .unwrap_or_else(|error| panic!("decode node information: {error:?}"));
    assert_eq!(info.interface_version, Version::V1_3);
    assert_eq!(info.protocol_version, PROTOCOL_VERSION);
    assert_eq!(info.network_id, EVIDENCE_NETWORK_ID);
    assert_eq!(info.role, NodeRole::Sequencer);
    assert_eq!(info.chain_head_sequence, 50);
    assert_eq!(info.latest_sealed_batch, 7);
    assert_eq!(info.latest_finalised_checkpoint, [0x91; 32]);
    assert_eq!(
        info.authorised_sequencer_key,
        signer.verifying_key().to_bytes()
    );
    assert_eq!(
        info.advertised_capabilities,
        ["authenticated_durable_submit", "node_info", "submit"]
    );
}

fn handshake(client: &mut Uds, signer: &SigningKey) {
    client
        .send(&handshake_request())
        .unwrap_or_else(|error| panic!("send handshake: {error:?}"));
    let response = client
        .receive()
        .unwrap_or_else(|error| panic!("receive handshake: {error:?}"));
    assert_handshake(&response, signer);
}

#[test]
fn evidence_node_handshakes_over_human_frames() {
    let signer = SigningKey::from_bytes(&[0x84; 32]);
    let node = evidence_node(&signer, "frames-handshake-journal", 1);
    let journal_directory = node.journal().directory().to_path_buf();
    let (socket_path, server) = serve_evidence_node(node, "frames-handshake");
    let mut client = connect_evidence_node(&socket_path);
    handshake(&mut client, &signer);
    drop(client);
    let node = server
        .join()
        .unwrap_or_else(|_| panic!("evidence node worker panicked"));
    assert!(!node.fail_stopped());
    assert!(node.queued().is_empty());
    assert!(node.journal().records().is_empty());
    drop(node);
    std::fs::remove_file(socket_path).unwrap_or_else(|error| panic!("remove socket: {error}"));
    std::fs::remove_dir_all(journal_directory)
        .unwrap_or_else(|error| panic!("remove journal: {error}"));
}

#[test]
fn evidence_node_refuses_oversized_frame() {
    let signer = SigningKey::from_bytes(&[0x85; 32]);
    let node = evidence_node(&signer, "frames-oversized-journal", 1);
    let journal_directory = node.journal().directory().to_path_buf();
    let (socket_path, server) = serve_evidence_node(node, "frames-oversized");
    for after_handshake in [false, true] {
        let mut stream = UnixStream::connect(&socket_path)
            .unwrap_or_else(|error| panic!("connect oversized sender: {error}"));
        stream
            .set_read_timeout(Some(Duration::from_secs(2)))
            .unwrap_or_else(|error| panic!("set read deadline: {error}"));
        stream
            .set_write_timeout(Some(Duration::from_secs(2)))
            .unwrap_or_else(|error| panic!("set write deadline: {error}"));
        if after_handshake {
            write_frame(&mut stream, &handshake_request(), FRAME_BYTES)
                .unwrap_or_else(|error| panic!("send initial handshake: {error:?}"));
            let response = read_frame(&mut stream, FRAME_BYTES)
                .unwrap_or_else(|error| panic!("receive initial handshake: {error:?}"));
            assert_handshake(&response, &signer);
        }
        let oversized = u32::try_from(FRAME_BYTES + 1)
            .unwrap_or_else(|error| panic!("oversized length: {error}"));
        stream
            .write_all(&oversized.to_be_bytes())
            .unwrap_or_else(|error| panic!("write oversized prefix: {error}"));
        let mut response = [0_u8; 1];
        assert_eq!(
            stream
                .read(&mut response)
                .unwrap_or_else(|error| panic!("read refused connection: {error}")),
            0,
            "the server must close an oversized frame without waiting for its body"
        );
    }
    let mut client = connect_evidence_node(&socket_path);
    handshake(&mut client, &signer);
    let payload = vec![0_u8; FRAME_BYTES - 22];
    let request = encode_envelope(Envelope {
        version: Version::V1_3,
        message_tag: 1,
        correlation_id: 17,
        canonical_payload: &payload,
        proof_material: &[],
    })
    .unwrap_or_else(|error| panic!("encode maximum frame: {error:?}"));
    assert_eq!(request.len(), FRAME_BYTES);
    client
        .send(&request)
        .unwrap_or_else(|error| panic!("send maximum frame: {error:?}"));
    let response = client
        .receive()
        .unwrap_or_else(|error| panic!("receive maximum frame response: {error:?}"));
    let envelope = decode_envelope(&response)
        .unwrap_or_else(|error| panic!("decode maximum frame response: {error:?}"));
    assert_eq!(envelope.message_tag, 25);
    assert_eq!(envelope.correlation_id, 17);
    let mut refusal = vec![1];
    refusal.extend_from_slice(&KnownResult::MalformedEnvelope.raw().to_be_bytes());
    assert_eq!(envelope.canonical_payload, refusal);
    assert!(envelope.proof_material.is_empty());
    drop(client);
    let node = server
        .join()
        .unwrap_or_else(|_| panic!("evidence node worker panicked"));
    assert!(!node.fail_stopped());
    assert!(node.queued().is_empty());
    assert!(node.journal().records().is_empty());
    assert_eq!(node.authentication_refusals(), 0);
    drop(node);
    std::fs::remove_file(socket_path).unwrap_or_else(|error| panic!("remove socket: {error}"));
    std::fs::remove_dir_all(journal_directory)
        .unwrap_or_else(|error| panic!("remove journal: {error}"));
}
