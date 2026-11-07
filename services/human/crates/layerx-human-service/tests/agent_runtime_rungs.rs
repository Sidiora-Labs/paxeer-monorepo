#[path = "support/agent_runtime_peer.rs"]
mod agent_runtime_peer;

pub use agent_runtime_peer::{answer, frame, request, Peer, TRACE};
use agent_runtime_peer::{bytes, checked};
use k256::ecdsa::SigningKey;
use layerx_agent_api::error::RequestId;
use layerx_agent_api::idempotency::{BodyDigest, IdempotentMutation, Key};
use layerx_agent_api::submit::{PreparationRef, SignatureBytes, SubmitRequest};
use layerx_agent_api::track::{SubmissionRef, SubmissionState, TrackRequest};
use layerx_agentd::human_receipt_frame::ReceiptLookupAnswer;
use layerx_human_service::journeys::{
    AgentBoundary, AgentBoundaryError, ReceiptKeyEntry, ReceiptLookup, ReceiptMaterial,
};
use layerx_human_service::server::agent_runtime::TraceContext;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_sdk::Client;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use layerx_wire::receipt::{decode, encode};
use sha2::{Digest as _, Sha256};

#[test]
fn runtime_decodes_rung_bytes_zero_one_two_and_refuses_three() {
    let reference = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789";
    let submission_id = [0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89].repeat(4);
    let mut body = Vec::new();
    bytes(&mut body, reference.as_bytes());
    let ranks = 0..=u8::MAX;
    let exchanges = ranks
        .clone()
        .map(|rung| {
            let mut response = vec![0x11; 32];
            response.extend_from_slice(&submission_id);
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
    let receipt_hash = Sha256::digest(&canonical_bytes).into();
    let batch_fields = [[0x71; 32], [0x72; 32], [0x73; 32], [0x74; 32], [0x75; 32]];
    let expected = ReceiptMaterial {
        canonical_bytes: canonical_bytes.clone(),
        signed_receipt: canonical_bytes.clone(),
        receipt_key: ReceiptKeyEntry {
            key_id: [0x94; 32],
            public_key: batch_fields[4],
            first_height: 1,
            last_height: u64::MAX,
        },
        inclusion_height: observed_height,
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
    let receipt = checked(frame(&expected).encode());
    let rung_offset = 4 + canonical_bytes.len() + 5 * 32 + 1 + 32 + 32 + 8;

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
    observation.extend_from_slice(&[0x61; 32]);
    observation.push(6);
    observation.extend_from_slice(&receipt_hash);
    observation.extend_from_slice(&[2, 0, 0, 1]);
    observation.extend_from_slice(&receipt);
    let response = checked(ReceiptLookupAnswer::Found(frame(&expected)).encode());
    let mut coordinates = vec![0x64; 32];
    coordinates.extend_from_slice(&tx_hash);
    let mut truncated = response.clone();
    truncated.pop();
    let mut trailing = response.clone();
    trailing.push(0);
    let mut retired_rung = response.clone();
    retired_rung[1 + rung_offset] = 3;
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
