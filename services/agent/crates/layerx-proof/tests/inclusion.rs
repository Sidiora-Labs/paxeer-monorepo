use ed25519_dalek::{Signer as _, SigningKey};
use layerx_proof::inclusion::{
    verify_activity, verify_receipt, verify_state, InclusionError, SequencerAuthorization,
};
use layerx_proof::level::achieved_rung;
use layerx_proof::merkle::{build_proof, MerkleError, Proof};
use layerx_types::{rung::Rung, verify::VerificationLevel};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::batch_header_digest;
use layerx_wire::limits::PROTOCOL_VERSION;

fn header_bytes(
    batch_number: u64,
    resulting_state_root: [u8; 32],
    activity_merkle_root: [u8; 32],
    sequencer_id: [u8; 32],
) -> Vec<u8> {
    header_bytes_with_receipt_root(
        batch_number,
        resulting_state_root,
        activity_merkle_root,
        [2; 32],
        sequencer_id,
    )
}

fn header_bytes_with_receipt_root(
    batch_number: u64,
    resulting_state_root: [u8; 32],
    activity_merkle_root: [u8; 32],
    receipt_merkle_root: [u8; 32],
    sequencer_id: [u8; 32],
) -> Vec<u8> {
    let mut encoder = Encoder::new(354);
    assert_eq!(
        encoder.structure_header_version(0x1701, PROTOCOL_VERSION),
        Ok(())
    );
    assert_eq!(encoder.u8(15), Ok(()));
    let fields: [(u8, Vec<u8>); 15] = [
        (1, PROTOCOL_VERSION.to_be_bytes().to_vec()),
        (2, 42_u32.to_be_bytes().to_vec()),
        (3, 2_u64.to_be_bytes().to_vec()),
        (4, batch_number.to_be_bytes().to_vec()),
        (5, 9_u64.to_be_bytes().to_vec()),
        (6, 10_u64.to_be_bytes().to_vec()),
        (7, [1; 32].to_vec()),
        (8, resulting_state_root.to_vec()),
        (9, activity_merkle_root.to_vec()),
        (10, receipt_merkle_root.to_vec()),
        (11, [3; 32].to_vec()),
        (12, [4; 32].to_vec()),
        (13, [5; 32].to_vec()),
        (14, 1_000_u64.to_be_bytes().to_vec()),
        (15, sequencer_id.to_vec()),
    ];
    for (field, value) in fields {
        assert_eq!(encoder.tag(field, 15), Ok(()));
        match field {
            1 => assert_eq!(
                encoder.u16(u16::from_be_bytes([value[0], value[1]])),
                Ok(())
            ),
            2 => assert_eq!(
                encoder.u32(u32::from_be_bytes([value[0], value[1], value[2], value[3]])),
                Ok(())
            ),
            3..=6 | 14 => assert_eq!(
                encoder.u64(u64::from_be_bytes(
                    value
                        .as_slice()
                        .try_into()
                        .unwrap_or_else(|_| panic!("invalid u64 test field")),
                )),
                Ok(())
            ),
            _ => assert_eq!(encoder.bytes(&value, 32), Ok(())),
        }
    }
    let bytes = encoder.finish();
    assert_eq!(bytes.len(), 354);
    bytes
}

fn sign_header(bytes: &[u8], key: &SigningKey) -> [u8; 64] {
    let digest =
        batch_header_digest(bytes).unwrap_or_else(|error| panic!("header hash failed: {error:?}"));
    key.sign(&digest).to_bytes()
}

#[test]
fn raises_only_the_level_each_proof_establishes() {
    let key = SigningKey::from_bytes(&[7; 32]);
    let sequencer_id = key.verifying_key().to_bytes();
    let activities: [&[u8]; 3] = [b"activity-a", b"activity-b", b"activity-c"];
    let states: [&[u8]; 2] = [b"state-a", b"state-b"];
    let (activity_proof, activity_root) = build_proof(&activities, 1)
        .unwrap_or_else(|error| panic!("activity proof failed: {error:?}"));
    let (state_proof, state_root) =
        build_proof(&states, 0).unwrap_or_else(|error| panic!("state proof failed: {error:?}"));
    let header = header_bytes(7, state_root, activity_root, sequencer_id);
    let signature = sign_header(&header, &key);
    let authorization = SequencerAuthorization::new(sequencer_id, sequencer_id, 7, 9);

    let activity = verify_activity(
        activities[1],
        &activity_proof,
        &header,
        &signature,
        &authorization,
    )
    .unwrap_or_else(|error| panic!("activity inclusion failed: {error:?}"));
    assert_eq!(activity.level(), VerificationLevel::BATCH_INCLUDED);

    let state = verify_state(
        states[0],
        &state_proof,
        &state_root,
        &header,
        &signature,
        &authorization,
    )
    .unwrap_or_else(|error| panic!("state inclusion failed: {error:?}"));
    assert_eq!(state.level(), VerificationLevel::STATE_PROVEN);
}

#[test]
fn proof_evidence_never_reaches_the_committed_rung() {
    use layerx_proof::receipt::{verify_outcome, AuthorizedBatch};
    use layerx_wire::receipt::{decode, decode_batch_header, decode_merkle_proof};

    let receipt_bytes =
        include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/receipt");
    let activity_bytes =
        include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/activity.lxa");
    let header_bytes =
        include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/header");
    let signature = include_bytes!(
        "../../../../../tests/fixtures/asset/bound-native-withdrawal/header.signature"
    );
    let public_key = *include_bytes!(
        "../../../../../tests/fixtures/asset/bound-native-withdrawal/sequencer.public"
    );
    let header = decode_batch_header(header_bytes)
        .unwrap_or_else(|error| panic!("native header: {error:?}"));
    let authorization = SequencerAuthorization::new(
        header.sequencer_id(),
        public_key,
        header.batch_number(),
        header.batch_number(),
    );
    let decoded = decode(receipt_bytes).unwrap_or_else(|error| panic!("native receipt: {error:?}"));
    let Some(protocol) = decoded.protocol() else {
        panic!("native protocol receipt");
    };
    let batch = AuthorizedBatch::new(
        protocol.batch_id(),
        protocol.asset(),
        protocol.previous_state_root(),
        protocol.resulting_state_root(),
        public_key,
    );
    let receipt = verify_outcome(receipt_bytes, &batch)
        .unwrap_or_else(|error| panic!("receipt verification: {error:?}"));
    assert_eq!(receipt.canonical_bytes(), receipt_bytes);
    assert_eq!(receipt.rung(), Rung::Submitted);

    let (activity_proof, activity_root) = build_proof(&[activity_bytes.as_slice()], 0)
        .unwrap_or_else(|error| panic!("activity proof: {error:?}"));
    assert_eq!(activity_root, header.activity_merkle_root());
    let activity = verify_activity(
        activity_bytes,
        &activity_proof,
        header_bytes,
        signature,
        &authorization,
    )
    .unwrap_or_else(|error| panic!("native activity inclusion: {error:?}"));
    let captured_proof = decode_merkle_proof(include_bytes!(
        "../../../../../tests/fixtures/asset/bound-native-withdrawal/receipt.proof"
    ))
    .unwrap_or_else(|error| panic!("native receipt path: {error:?}"));
    let receipt_proof = Proof::new(
        captured_proof.leaf_index(),
        captured_proof.leaf_count(),
        captured_proof.siblings().to_vec(),
    )
    .unwrap_or_else(|error| panic!("native receipt proof: {error:?}"));
    let included_receipt = verify_receipt(
        receipt_bytes,
        &receipt_proof,
        header_bytes,
        signature,
        &authorization,
    )
    .unwrap_or_else(|error| panic!("native receipt inclusion: {error:?}"));

    let (state, state_root) =
        verified_state_for_roots(header.activity_merkle_root(), header.receipt_merkle_root());
    assert_uncommitted_proof_rungs(
        receipt.evidence(),
        activity.evidence(),
        included_receipt.evidence(),
        state.evidence(),
    );
    for included in [&activity, &included_receipt, &state] {
        assert_eq!(included.rung(), Rung::Receipted);
        assert_ne!(included.rung(), Rung::Committed);
    }
    assert_eq!(activity.evidence().proof_root(), Some(activity_root));
    assert_eq!(
        included_receipt.evidence().proof_root(),
        Some(header.receipt_merkle_root())
    );
    assert_eq!(state.evidence().proof_root(), Some(state_root));
}

fn verified_state_for_roots(
    activity_root: [u8; 32],
    receipt_root: [u8; 32],
) -> (layerx_proof::inclusion::InclusionEvidence, [u8; 32]) {
    let key = SigningKey::from_bytes(&[7; 32]);
    let sequencer_id = key.verifying_key().to_bytes();
    let states: [&[u8]; 2] = [b"state-a", b"state-b"];
    let (state_proof, state_root) =
        build_proof(&states, 0).unwrap_or_else(|error| panic!("state proof: {error:?}"));
    let state_header =
        header_bytes_with_receipt_root(7, state_root, activity_root, receipt_root, sequencer_id);
    let state_signature = sign_header(&state_header, &key);
    let state_authorization = SequencerAuthorization::new(sequencer_id, sequencer_id, 7, 7);
    let state = verify_state(
        states[0],
        &state_proof,
        &state_root,
        &state_header,
        &state_signature,
        &state_authorization,
    )
    .unwrap_or_else(|error| panic!("state inclusion: {error:?}"));

    (state, state_root)
}

fn assert_uncommitted_proof_rungs(
    receipt: &layerx_proof::evidence::Evidence,
    activity: &layerx_proof::evidence::Evidence,
    included_receipt: &layerx_proof::evidence::Evidence,
    state: &layerx_proof::evidence::Evidence,
) {
    use layerx_proof::level::achieved;

    for (evidence, level, rung) in [
        (
            receipt,
            VerificationLevel::SEQUENCER_SIGNED,
            Rung::Submitted,
        ),
        (activity, VerificationLevel::BATCH_INCLUDED, Rung::Receipted),
        (
            included_receipt,
            VerificationLevel::BATCH_INCLUDED,
            Rung::Receipted,
        ),
        (state, VerificationLevel::STATE_PROVEN, Rung::Receipted),
    ] {
        assert_eq!(evidence.level(), level);
        assert_eq!(achieved(evidence), level);
        assert_eq!(evidence.rung(), rung);
        assert_eq!(achieved_rung(evidence), rung);
        assert!(evidence.rung() < Rung::Committed);
        assert!(achieved_rung(evidence) < Rung::Committed);
    }
}

#[test]
fn rejects_truncated_swapped_and_unauthorised_evidence() {
    let key = SigningKey::from_bytes(&[7; 32]);
    let sequencer_id = key.verifying_key().to_bytes();
    let activities: [&[u8]; 3] = [b"activity-a", b"activity-b", b"activity-c"];
    let states: [&[u8]; 2] = [b"state-a", b"state-b"];
    let (proof, activity_root) = build_proof(&activities, 1)
        .unwrap_or_else(|error| panic!("activity proof failed: {error:?}"));
    let (_, state_root) =
        build_proof(&states, 0).unwrap_or_else(|error| panic!("state proof failed: {error:?}"));
    let header = header_bytes(7, state_root, activity_root, sequencer_id);
    let signature = sign_header(&header, &key);
    let authorization = SequencerAuthorization::new(sequencer_id, sequencer_id, 7, 9);

    let mut truncated = proof.siblings().to_vec();
    let _ = truncated.pop();
    assert_eq!(
        Proof::new(proof.leaf_index(), proof.leaf_count(), truncated),
        Err(MerkleError::PathLength {
            expected: proof.siblings().len(),
            actual: proof.siblings().len() - 1,
        })
    );

    let swapped = header_bytes(7, state_root, [99; 32], sequencer_id);
    let swapped_signature = sign_header(&swapped, &key);
    assert!(matches!(
        verify_activity(
            activities[1],
            &proof,
            &swapped,
            &swapped_signature,
            &authorization,
        ),
        Err(InclusionError::Merkle(MerkleError::RootMismatch))
    ));

    let wrong_batch = SequencerAuthorization::new(sequencer_id, sequencer_id, 8, 9);
    assert_eq!(
        verify_activity(activities[1], &proof, &header, &signature, &wrong_batch),
        Err(InclusionError::BatchNumber)
    );

    let wrong_identity = SequencerAuthorization::new([55; 32], sequencer_id, 7, 9);
    assert_eq!(
        verify_activity(activities[1], &proof, &header, &signature, &wrong_identity,),
        Err(InclusionError::SequencerIdentity)
    );
}

#[test]
fn inclusion_evidence_reports_the_receipted_rung() {
    let key = SigningKey::from_bytes(&[7; 32]);
    let sequencer_id = key.verifying_key().to_bytes();
    let activities: [&[u8]; 3] = [b"activity-a", b"activity-b", b"activity-c"];
    let receipts: [&[u8]; 2] = [b"receipt-a", b"receipt-b"];
    let states: [&[u8]; 2] = [b"state-a", b"state-b"];
    let (activity_proof, activity_root) = build_proof(&activities, 1)
        .unwrap_or_else(|error| panic!("activity proof failed: {error:?}"));
    let (receipt_proof, receipt_root) =
        build_proof(&receipts, 0).unwrap_or_else(|error| panic!("receipt proof failed: {error:?}"));
    let (state_proof, state_root) =
        build_proof(&states, 0).unwrap_or_else(|error| panic!("state proof failed: {error:?}"));
    let header =
        header_bytes_with_receipt_root(7, state_root, activity_root, receipt_root, sequencer_id);
    let signature = sign_header(&header, &key);
    let authorization = SequencerAuthorization::new(sequencer_id, sequencer_id, 7, 9);
    let activity = verify_activity(
        activities[1],
        &activity_proof,
        &header,
        &signature,
        &authorization,
    )
    .unwrap_or_else(|error| panic!("activity inclusion failed: {error:?}"));
    let receipt = verify_receipt(
        receipts[0],
        &receipt_proof,
        &header,
        &signature,
        &authorization,
    )
    .unwrap_or_else(|error| panic!("receipt inclusion failed: {error:?}"));
    let state = verify_state(
        states[0],
        &state_proof,
        &state_root,
        &header,
        &signature,
        &authorization,
    )
    .unwrap_or_else(|error| panic!("state inclusion failed: {error:?}"));
    for (included, level, root) in [
        (activity, VerificationLevel::BATCH_INCLUDED, activity_root),
        (receipt, VerificationLevel::BATCH_INCLUDED, receipt_root),
        (state, VerificationLevel::STATE_PROVEN, state_root),
    ] {
        assert_eq!(included.rung(), Rung::Receipted);
        assert_eq!(included.level(), level);
        assert_eq!(included.evidence().level(), level);
        assert_eq!(included.evidence().rung(), Rung::Receipted);
        assert_eq!(achieved_rung(included.evidence()), Rung::Receipted);
        assert_eq!(included.evidence().proof_root(), Some(root));
        assert_eq!(
            included.evidence().header_digest(),
            Some(
                batch_header_digest(&header)
                    .unwrap_or_else(|error| panic!("header hash failed: {error:?}"))
            )
        );
    }
}
