#[path = "support/agent_runtime_peer.rs"]
mod agent_runtime_peer;

use agent_runtime_peer::{answer, bytes, checked, frame, request, Peer, TRACE};
use layerx_agent_api::track::{
    EvidenceRef, ReceiptRef, SubmissionRef, SubmissionState, TrackRequest, TrackedSubmission,
};
use layerx_agentd::outbox::SubmissionState as OutboxState;
use layerx_human_service::journeys::{
    AgentBoundary, AgentBoundaryError, ReceiptKeyEntry, ReceiptMaterial,
};
use layerx_human_service::server::agent_runtime::TraceContext;
use layerx_human_test_support::daemon_track_observation;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_sdk::Client;
use layerx_types::result::ResultCode;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use layerx_wire::receipt::{decode, encode};
use sha2::{Digest as _, Sha256};

const SUBMISSION_ID: [u8; 32] = [0xab; 32];
const SUBMISSION_HEX: &str = "abababababababababababababababababababababababababababababababab";
const ACTIVITY_ID: [u8; 32] = [0xcd; 32];

#[test]
fn repair_22_31_579_decodes_daemon_states_without_receipts() {
    let cases = [
        (OutboxState::Prepared, SubmissionState::Prepared),
        (OutboxState::Signed, SubmissionState::Signed),
        (OutboxState::Queued, SubmissionState::Queued),
        (OutboxState::Submitted, SubmissionState::Submitted),
        (OutboxState::Unknown, SubmissionState::Unknown),
        (OutboxState::Expired, SubmissionState::Expired),
        (
            OutboxState::Failed,
            SubmissionState::Failed {
                result: ResultCode::from_raw(-1),
            },
        ),
    ];
    let mut body = Vec::new();
    bytes(&mut body, SUBMISSION_HEX.as_bytes());
    let exchanges = cases
        .iter()
        .map(|(state, _)| {
            let observation = checked(daemon_track_observation(SUBMISSION_ID, ACTIVITY_ID, *state));
            (request(3, &body), answer(&observation))
        })
        .collect();
    let (peer, mut runtime) = Peer::serve(exchanges);
    let _trace = checked(TraceContext::enter(TRACE));
    let client = checked(Client::daemon(
        "/run/layerx-agentd.sock",
        layerx_agent_api::agent_api_schema_v1().version,
    ));
    let call = client.track(TrackRequest {
        submission_ref: checked(SubmissionRef::new(SUBMISSION_HEX)),
    });
    for (status, state) in cases {
        let observation = checked(runtime.track(&call));
        assert_eq!(observation.activity_id, ACTIVITY_ID, "{status:?}");
        assert_eq!(observation.receipt, None, "{status:?}");
        assert_eq!(
            observation.submission,
            TrackedSubmission {
                submission_ref: checked(SubmissionRef::new(SUBMISSION_HEX)),
                state,
                rung: Rung::Submitted,
                evidence: Vec::new(),
                transitions: Vec::new(),
            },
            "{status:?}"
        );
    }
    peer.finish();
}

#[test]
fn repair_22_31_579_decodes_executed_and_committed_receipt_digests() {
    let height = 7_u64;
    let mut canonical = 1_u16.to_be_bytes().to_vec();
    canonical.extend_from_slice(&height.to_be_bytes());
    canonical.extend_from_slice(&ACTIVITY_ID);
    canonical.extend_from_slice(&[0x73; 32]);
    canonical.extend_from_slice(&[0x74; 32]);
    let canonical_bytes = checked(encode(&checked(decode(&canonical))));
    let digest: [u8; 32] = Sha256::digest(&canonical_bytes).into();
    let receipt_hex: String = layerx_human_service::hex::lower((digest).as_ref());
    let material = ReceiptMaterial {
        signed_receipt: canonical_bytes.clone(),
        canonical_bytes,
        authorised_batch: AuthorizedBatch::new(
            [0x71; 32], [0x72; 32], [0x73; 32], [0x74; 32], [0x75; 32],
        ),
        verification_level: VerificationLevel::COMMITTED,
        tx_hash: ACTIVITY_ID,
        receipt_hash: digest,
        block_height: height,
        rung: Rung::Receipted,
        receipt_key: ReceiptKeyEntry {
            key_id: [0x94; 32],
            public_key: [0x75; 32],
            first_height: 1,
            last_height: u64::MAX,
        },
        inclusion_height: height,
    };
    let mut body = Vec::new();
    bytes(&mut body, SUBMISSION_HEX.as_bytes());
    let cases = [(6, Rung::Receipted), (9, Rung::Committed)];
    let exchanges = cases
        .iter()
        .map(|(code, rung)| {
            let mut response = ACTIVITY_ID.to_vec();
            response.extend_from_slice(&SUBMISSION_ID);
            response.push(*code);
            response.extend_from_slice(&digest);
            response.extend_from_slice(&[rung.wire_rank(), 1]);
            bytes(&mut response, b"sequencer_receipt");
            response.extend_from_slice(&digest);
            response.push(0);
            response.push(u8::from(*code == 6));
            if *code == 6 {
                response.extend_from_slice(&checked(frame(&material).encode()));
            }
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
        submission_ref: checked(SubmissionRef::new(SUBMISSION_HEX)),
    });
    for (code, rung) in cases {
        let observation = checked(runtime.track(&call));
        assert_eq!(observation.activity_id, ACTIVITY_ID);
        assert_eq!(
            observation.receipt,
            if code == 6 {
                Some(material.clone())
            } else {
                None
            }
        );
        let receipt_ref = checked(ReceiptRef::new(receipt_hex.clone()));
        assert_eq!(
            observation.submission,
            TrackedSubmission {
                submission_ref: checked(SubmissionRef::new(SUBMISSION_HEX)),
                state: if code == 6 {
                    SubmissionState::Executed { receipt_ref }
                } else {
                    SubmissionState::Committed { receipt_ref }
                },
                rung,
                evidence: vec![EvidenceRef {
                    kind: String::from("sequencer_receipt"),
                    digest,
                }],
                transitions: Vec::new(),
            }
        );
    }
    peer.finish();
}

#[test]
fn repair_22_31_579_refuses_length_prefixed_text_references() {
    let mut body = Vec::new();
    bytes(&mut body, SUBMISSION_HEX.as_bytes());
    let mut response = ACTIVITY_ID.to_vec();
    bytes(&mut response, SUBMISSION_HEX.as_bytes());
    response.extend_from_slice(&[3, 0, 0, 0, 0]);
    let (peer, mut runtime) = Peer::serve(vec![(request(3, &body), answer(&response))]);
    let _trace = checked(TraceContext::enter(TRACE));
    let client = checked(Client::daemon(
        "/run/layerx-agentd.sock",
        layerx_agent_api::agent_api_schema_v1().version,
    ));
    let call = client.track(TrackRequest {
        submission_ref: checked(SubmissionRef::new(SUBMISSION_HEX)),
    });
    assert_eq!(
        runtime.track(&call),
        Err(AgentBoundaryError::CorruptResponse)
    );
    peer.finish();
}

#[test]
fn repair_22_31_579_refuses_undefined_state_codes() {
    let mut body = Vec::new();
    bytes(&mut body, SUBMISSION_HEX.as_bytes());
    let codes = (0..=u8::MAX).filter(|code| !matches!(code, 0..=3 | 5..=9));
    let exchanges = codes
        .clone()
        .map(|code| {
            let mut response = ACTIVITY_ID.to_vec();
            response.extend_from_slice(&SUBMISSION_ID);
            response.extend_from_slice(&[code, 0, 0, 0, 0]);
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
        submission_ref: checked(SubmissionRef::new(SUBMISSION_HEX)),
    });
    for code in codes {
        assert_eq!(
            runtime.track(&call),
            Err(AgentBoundaryError::CorruptResponse),
            "state code {code}"
        );
    }
    peer.finish();
}
