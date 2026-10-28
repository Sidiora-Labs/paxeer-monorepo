use std::fmt::Debug;
use std::os::unix::net::UnixListener;
use std::path::PathBuf;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use k256::ecdsa::SigningKey;
use layerx_agent_api::error::RequestId;
use layerx_agent_api::idempotency::{BodyDigest, IdempotentMutation, Key};
use layerx_agent_api::submit::{PreparationRef, SignatureBytes, SubmitRequest};
use layerx_agent_api::track::{SubmissionRef, SubmissionState, TrackRequest};
use layerx_human_service::frame::{read_frame, write_frame, Limits};
use layerx_human_service::journeys::{
    AgentBoundary, AgentBoundaryError, ReceiptLookup, ReceiptMaterial,
};
use layerx_human_service::server::agent_runtime::{AgentRuntime, TraceContext};
use layerx_proof::receipt::AuthorizedBatch;
use layerx_sdk::Client;
use layerx_types::payload::ModuleRegistry;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use layerx_wire::receipt::{decode, encode};

const TRACE: &str = "trc_0123456789abcdef0123456789abcdef";
const MAX_FRAME: usize = 4096;

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("agent runtime peer: {error:?}"))
}

struct Peer {
    directory: PathBuf,
    server: Option<JoinHandle<()>>,
}

impl Peer {
    fn serve(exchanges: Vec<(Vec<u8>, Vec<u8>)>) -> (Self, AgentRuntime) {
        let mut entropy = [0; 16];
        checked(getrandom::fill(&mut entropy));
        let directory =
            std::env::temp_dir().join(format!("human-rungs-{:032x}", u128::from_be_bytes(entropy)));
        checked(std::fs::create_dir(&directory));
        let socket = directory.join("peer.sock");
        let listener = checked(UnixListener::bind(&socket));
        checked(listener.set_nonblocking(true));
        let server = thread::spawn(move || {
            for (expected, response) in exchanges {
                let deadline = Instant::now() + Duration::from_secs(5);
                let mut stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(Instant::now() < deadline, "peer request deadline");
                            thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => panic!("peer accept: {error}"),
                    }
                };
                checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
                checked(stream.set_write_timeout(Some(Duration::from_secs(5))));
                assert_eq!(checked(read_frame(&mut stream, MAX_FRAME)), expected);
                checked(write_frame(&mut stream, &response, MAX_FRAME));
            }
        });
        let runtime = checked(AgentRuntime::new(
            &socket,
            Limits {
                maximum_frame_bytes: MAX_FRAME,
                maximum_connections: 1,
                maximum_streams: 1,
                maximum_queued_bytes: MAX_FRAME,
                deadline: Duration::from_secs(5),
            },
            checked(ModuleRegistry::new(&[])),
        ));
        (
            Self {
                directory,
                server: Some(server),
            },
            runtime,
        )
    }

    fn finish(mut self) {
        if let Some(server) = self.server.take() {
            checked(server.join());
        }
    }
}

impl Drop for Peer {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.directory);
    }
}

fn bytes(target: &mut Vec<u8>, value: &[u8]) {
    target.extend_from_slice(&checked(u32::try_from(value.len())).to_be_bytes());
    target.extend_from_slice(value);
}

fn traced(inner: &[u8]) -> Vec<u8> {
    let mut frame = b"LXHAGT02".to_vec();
    frame.extend_from_slice(TRACE.as_bytes());
    bytes(&mut frame, inner);
    frame
}

fn request(opcode: u8, body: &[u8]) -> Vec<u8> {
    let mut inner = b"LXHAGT01".to_vec();
    inner.push(opcode);
    inner.extend_from_slice(body);
    traced(&inner)
}

fn answer(body: &[u8]) -> Vec<u8> {
    let mut inner = b"LXHAGT01\0".to_vec();
    inner.extend_from_slice(body);
    traced(&inner)
}

#[test]
fn runtime_decodes_rung_bytes_zero_one_two_and_refuses_three() {
    let reference = "submission_rungs";
    let mut body = Vec::new();
    bytes(&mut body, reference.as_bytes());
    let ranks = 0..=u8::MAX;
    let exchanges = ranks
        .clone()
        .map(|rung| {
            let mut response = vec![0x11; 32];
            bytes(&mut response, reference.as_bytes());
            response.extend_from_slice(&[3, rung, 0, 0, 0]);
            (request(3, &body), answer(&response))
        })
        .collect();
    let (peer, mut runtime) = Peer::serve(exchanges);
    let _trace = checked(TraceContext::enter(TRACE));
    let client = checked(Client::daemon(
        "/run/layerx-agentd.sock",
        layerx_agent_api::agent_api_schema_v1().version,
    ));
    let call = client.track(TrackRequest {
        submission_ref: checked(SubmissionRef::new(reference)),
    });
    for rank in ranks {
        let result = runtime.track(&call);
        match rank {
            0..=2 => {
                let observation = checked(result);
                let expected = [Rung::Submitted, Rung::Receipted, Rung::Committed];
                assert_eq!(observation.submission.rung, expected[usize::from(rank)]);
                assert_eq!(observation.activity_id, [0x11; 32]);
                assert_eq!(observation.submission.submission_ref.as_str(), reference);
            }
            _ => assert_eq!(
                result,
                Err(AgentBoundaryError::CorruptResponse),
                "rung {rank}"
            ),
        }
    }
    peer.finish();
}

#[test]
fn runtime_decodes_finalization_evidence_with_a_65_byte_signature() {
    let tx_hash = [0x31; 32];
    let receipt_hash = [0x42; 32];
    let observed_height = 0x0102_0304_0506_0708_u64;
    let key = checked(SigningKey::from_bytes((&[0x53; 32]).into()));
    let (signature, recovery) = checked(key.sign_prehash_recoverable(&tx_hash));
    let mut signature_bytes = [0; 65];
    signature_bytes[..64].copy_from_slice(&signature.to_bytes());
    signature_bytes[64] = recovery.to_byte();
    let mut replay = 1_u16.to_be_bytes().to_vec();
    replay.extend_from_slice(&observed_height.to_be_bytes());
    replay.extend_from_slice(&tx_hash);
    replay.extend_from_slice(&[0x73; 32]);
    replay.extend_from_slice(&[0x74; 32]);
    let canonical_bytes = checked(encode(&checked(decode(&replay))));
    let batch_fields = [[0x71; 32], [0x72; 32], [0x73; 32], [0x74; 32], [0x75; 32]];
    let expected = ReceiptMaterial {
        canonical_bytes: canonical_bytes.clone(),
        authorised_batch: AuthorizedBatch::new(
            batch_fields[0],
            batch_fields[1],
            batch_fields[2],
            batch_fields[3],
            batch_fields[4],
        ),
        verification_level: VerificationLevel::CHECKPOINT_FINALISED,
        tx_hash,
        receipt_hash,
        block_height: observed_height,
        rung: Rung::Committed,
    };
    let mut receipt = Vec::new();
    bytes(&mut receipt, &canonical_bytes);
    for field in batch_fields {
        receipt.extend_from_slice(&field);
    }
    receipt.push(4);
    receipt.extend_from_slice(&tx_hash);
    receipt.extend_from_slice(&receipt_hash);
    receipt.extend_from_slice(&observed_height.to_be_bytes());
    receipt.push(2);

    let client = checked(Client::daemon(
        "/run/layerx-agentd.sock",
        layerx_agent_api::agent_api_schema_v1().version,
    ));
    let call = client.submit(IdempotentMutation {
        request_id: RequestId(9),
        key: checked(Key::new([0x64; 32])),
        body_digest: BodyDigest([0x65; 32]),
        operation: SubmitRequest {
            preparation_ref: checked(PreparationRef::new("preparation_rungs")),
            signature: checked(SignatureBytes::new(signature_bytes.to_vec())),
            approval_release_ref: None,
        },
    });
    let signer_public_key = [0x66; 32];
    let mut submit = 9_u64.to_be_bytes().to_vec();
    submit.extend_from_slice(&[0x64; 32]);
    submit.extend_from_slice(&[0x65; 32]);
    bytes(&mut submit, b"preparation_rungs");
    bytes(&mut submit, &signature_bytes);
    submit.extend_from_slice(&signer_public_key);
    submit.push(0);
    let mut observation = tx_hash.to_vec();
    bytes(&mut observation, b"submission_rungs");
    observation.push(6);
    bytes(&mut observation, b"receipt_rungs");
    observation.extend_from_slice(&[2, 0, 0, 1]);
    observation.extend_from_slice(&receipt);
    let mut response = vec![1];
    response.extend_from_slice(&receipt);
    let mut coordinates = vec![0x64; 32];
    coordinates.extend_from_slice(&tx_hash);
    let mut truncated = response.clone();
    truncated.pop();
    let mut trailing = response.clone();
    trailing.push(0);
    let mut retired_rung = response.clone();
    *checked(retired_rung.last_mut().ok_or("missing rung")) = 3;
    let mut exchanges = vec![(request(2, &submit), answer(&observation))];
    exchanges.extend(
        [response, truncated, trailing, retired_rung]
            .into_iter()
            .map(|response| (request(8, &coordinates), answer(&response))),
    );
    let (peer, mut runtime) = Peer::serve(exchanges);
    let _trace = checked(TraceContext::enter(TRACE));
    let submitted = checked(runtime.submit(&call, signer_public_key));
    assert_eq!(submitted.activity_id, tx_hash);
    assert_eq!(submitted.submission.rung, Rung::Committed);
    assert!(matches!(
        submitted.submission.state,
        SubmissionState::Executed { .. }
    ));
    assert_eq!(submitted.receipt, Some(expected.clone()));
    let ReceiptLookup::Found(evidence) = checked(runtime.evidence([0x64; 32], tx_hash)) else {
        panic!("missing finalization evidence");
    };
    assert_eq!(evidence, expected);
    for _ in 0..3 {
        assert_eq!(
            runtime.evidence([0x64; 32], tx_hash),
            Err(AgentBoundaryError::CorruptResponse)
        );
    }
    assert!(SignatureBytes::new(signature_bytes[..64].to_vec()).is_err());
    let mut long_signature = signature_bytes.to_vec();
    long_signature.push(0);
    assert!(SignatureBytes::new(long_signature).is_err());
    peer.finish();
}
