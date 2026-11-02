#[path = "agent_runtime_rungs.rs"]
mod rungs;

use layerx_agentd::human_receipt_frame::ReceiptLookupAnswer;
use layerx_human_service::journeys::{
    AgentBoundary, AgentBoundaryError, ReceiptKeyEntry, ReceiptLookup, ReceiptMaterial,
};
use layerx_human_service::server::agent_runtime::TraceContext;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::{rung::Rung, verify::VerificationLevel};
use layerx_wire::receipt::{decode, encode};
use sha2::{Digest, Sha256};

fn material(rung: Rung) -> ReceiptMaterial {
    let height = 150_u64;
    let tx_hash = [0x31; 32];
    let mut replay = 1_u16.to_be_bytes().to_vec();
    replay.extend_from_slice(&height.to_be_bytes());
    replay.extend_from_slice(&tx_hash);
    replay.extend_from_slice(&[0x73; 32]);
    replay.extend_from_slice(&[0x74; 32]);
    let canonical_bytes = encode(&decode(&replay).expect("receipt")).expect("canonical receipt");
    ReceiptMaterial {
        receipt_hash: Sha256::digest(&canonical_bytes).into(),
        signed_receipt: canonical_bytes.clone(),
        canonical_bytes,
        authorised_batch: AuthorizedBatch::new(
            [0x71; 32], [0x72; 32], [0x73; 32], [0x74; 32], [0x75; 32],
        ),
        verification_level: VerificationLevel::SEQUENCER_SIGNED,
        tx_hash,
        block_height: height,
        rung,
        receipt_key: ReceiptKeyEntry {
            key_id: [0x94; 32],
            public_key: [0x75; 32],
            first_height: 100,
            last_height: 199,
        },
        inclusion_height: height,
    }
}

#[test]
fn runtime_decodes_shared_owner_receipt_answers_and_refuses_corrupt_evidence() {
    let receipted = material(Rung::Receipted);
    let mut committed = material(Rung::Committed);
    committed.verification_level = VerificationLevel::CHECKPOINT_FINALISED;
    let submitted = ReceiptLookupAnswer::Submitted {
        tx_hash: [0x31; 32],
        nonce: 41,
    };
    let valid = [
        (
            ReceiptLookupAnswer::Found(rungs::frame(&receipted)),
            ReceiptLookup::Found(receipted.clone()),
        ),
        (
            ReceiptLookupAnswer::Found(rungs::frame(&committed)),
            ReceiptLookup::Found(committed),
        ),
        (ReceiptLookupAnswer::Absent, ReceiptLookup::Absent),
        (
            submitted,
            ReceiptLookup::Submitted {
                tx_hash: [0x31; 32],
                nonce: 41,
            },
        ),
    ];
    let mut responses = valid
        .iter()
        .map(|(answer, _)| answer.encode().expect("shared answer"))
        .collect::<Vec<_>>();
    let good = responses[0].clone();
    let mut truncated = good.clone();
    truncated.truncate(1 + 4 + receipted.canonical_bytes.len() + 5 * 32);
    let mut excluded = rungs::frame(&receipted);
    excluded.first_height = receipted.inclusion_height + 1;
    let mut submitted_frame = rungs::frame(&receipted);
    submitted_frame.rung = Rung::Submitted;
    let mut wrong_signed = rungs::frame(&receipted);
    wrong_signed.signed_receipt[0] ^= 1;
    let mut zero_height = rungs::frame(&receipted);
    zero_height.inclusion_height = 0;
    let mut wrong_height = rungs::frame(&receipted);
    wrong_height.inclusion_height += 1;
    let mut zero_key = rungs::frame(&receipted);
    zero_key.receipt_key_id = [0; 32];
    let mut wrong_key = rungs::frame(&receipted);
    wrong_key.public_key[0] ^= 1;
    let mut expired = rungs::frame(&receipted);
    expired.last_height = receipted.inclusion_height - 1;
    responses.push(truncated);
    for frame in [
        excluded,
        submitted_frame,
        wrong_signed,
        zero_height,
        wrong_height,
        zero_key,
        wrong_key,
        expired,
    ] {
        responses.push(
            ReceiptLookupAnswer::Found(frame)
                .encode()
                .expect("malformed evidence answer"),
        );
    }
    let mut trailing = good.clone();
    trailing.push(0);
    responses.push(trailing);
    responses.push(vec![3]);
    let mut invalid_level = good.clone();
    invalid_level[1 + 4 + receipted.canonical_bytes.len() + 5 * 32] = 0;
    responses.push(invalid_level);
    let mut coordinates = [0x64; 32].to_vec();
    coordinates.extend_from_slice(&receipted.tx_hash);
    let count = responses.len();
    let exchanges = responses
        .into_iter()
        .map(|response| (rungs::request(4, &coordinates), rungs::answer(&response)))
        .collect();
    let (peer, mut runtime) = rungs::Peer::serve(exchanges);
    let _trace = TraceContext::enter(rungs::TRACE).expect("trace");
    for (_, expected) in valid {
        assert_eq!(
            runtime.receipt_by_idempotency_key([0x64; 32], receipted.tx_hash),
            Ok(expected)
        );
    }
    for _ in 4..count {
        assert_eq!(
            runtime.receipt_by_idempotency_key([0x64; 32], receipted.tx_hash),
            Err(AgentBoundaryError::CorruptResponse)
        );
    }
    peer.finish();
}
