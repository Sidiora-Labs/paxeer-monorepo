use layerx_agentd::finality::{
    wait_for_rung, FinalityError, SubmissionProgress, VerificationProgress,
};
use layerx_types::rung::Rung;

pub mod fixtures {
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_agentd::finality::{validator_address, BlockCommit, CommitSignature, Validator};
    use layerx_client::sequencer::{ReceiptKeyEntry, SequencerBlock};
    use layerx_wire::encode::Encoder;
    use layerx_wire::hash::receipt_digest;
    use layerx_wire::limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION};
    use sha2::{Digest as _, Sha256};
    const SEQUENCER_MODULE_ID: u16 = 12;

    pub const TX_HASH: [u8; 32] = [3; 32];
    pub const IDEMPOTENCY_KEY: [u8; 32] = [9; 32];
    pub const CHAIN_ID: &str = "paxeer-test-125";

    pub fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
        result.unwrap_or_else(|error| panic!("{error:?}"))
    }
    #[derive(Clone)]
    struct Fields {
        version: u16,
        activity_id: [u8; 32],
        batch_id: [u8; 32],
        key_id: [u8; 32],
        module_id: u16,
        result_code: i32,
    }

    fn fields(height: u64, key_id: [u8; 32]) -> Fields {
        let mut batch_id = [0; 32];
        batch_id[24..].copy_from_slice(&height.to_be_bytes());
        Fields {
            version: PROTOCOL_VERSION,
            activity_id: [3; 32],
            batch_id,
            key_id,
            module_id: SEQUENCER_MODULE_ID,
            result_code: 0,
        }
    }

    fn encode_fields(fields: &Fields, signature: Option<[u8; 64]>) -> Vec<u8> {
        let previous = [4; 32];
        let sequence = 7_u64;
        let resulting: [u8; 32] = Sha256::digest(
            [
                b"PAXEER/v1/sequence\0".as_slice(),
                &previous,
                &sequence.to_be_bytes(),
                &fields.activity_id,
            ]
            .concat(),
        )
        .into();
        let empty_digest: [u8; 32] = Sha256::digest([]).into();
        let fee_asset: [u8; 32] = Sha256::digest(b"upax").into();
        let mut from = [0; 32];
        from[12..].fill(6);
        let mut to = [0; 32];
        to[12..].fill(7);
        let mut summary = Encoder::new(256);
        assert_eq!(summary.u64(21_000), Ok(()));
        assert_eq!(summary.u64(21_000), Ok(()));
        assert_eq!(summary.u64(1), Ok(()));
        assert_eq!(summary.u32(1), Ok(()));
        assert_eq!(summary.fixed(&[0; 20]), Ok(()));
        assert_eq!(summary.fixed(&[0; 20]), Ok(()));
        assert_eq!(summary.fixed(&empty_digest), Ok(()));
        assert_eq!(summary.u32(0), Ok(()));
        assert_eq!(summary.u32(0), Ok(()));
        assert_eq!(summary.u8(4), Ok(()));
        assert_eq!(summary.fixed(b"upax"), Ok(()));

        let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
        assert_eq!(
            encoder.structure_header_version(0x5201, fields.version),
            Ok(())
        );
        assert_eq!(encoder.u16(fields.version), Ok(()));
        assert_eq!(encoder.bytes(&fields.activity_id, 32), Ok(()));
        assert_eq!(encoder.u64(sequence), Ok(()));
        assert_eq!(encoder.bytes(&previous, 32), Ok(()));
        assert_eq!(encoder.bytes(&resulting, 32), Ok(()));
        assert_eq!(encoder.bytes(&empty_digest, 32), Ok(()));
        assert_eq!(encoder.i32(fields.result_code), Ok(()));
        assert_eq!(encoder.sequence_length(1, 512), Ok(()));
        assert_eq!(encoder.u16(fields.module_id), Ok(()));
        assert_eq!(encoder.u16(0), Ok(()));
        assert_eq!(encoder.u16(0), Ok(()));
        assert_eq!(encoder.u8(1), Ok(()));
        assert_eq!(encoder.u8(0), Ok(()));
        assert_eq!(encoder.bytes(&[0; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&summary.finish(), 256), Ok(()));
        assert_eq!(encoder.u128(21_000), Ok(()));
        assert_eq!(encoder.bytes(&fields.batch_id, 32), Ok(()));
        assert_eq!(encoder.u16(fields.module_id), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u32(0), Ok(()));
        assert_eq!(encoder.u8(2), Ok(()));
        assert_eq!(encoder.bytes(&fee_asset, 32), Ok(()));
        assert_eq!(encoder.u128(1), Ok(()));
        assert_eq!(encoder.bytes(&from, 32), Ok(()));
        assert_eq!(encoder.u128(21_002), Ok(()));
        assert_eq!(
            encoder.u128(if fields.result_code == 0 { 1 } else { 2 }),
            Ok(())
        );
        assert_eq!(encoder.u64(0), Ok(()));
        assert_eq!(encoder.bytes(&to, 32), Ok(()));
        assert_eq!(encoder.u128(0), Ok(()));
        assert_eq!(encoder.u128(u128::from(fields.result_code == 0)), Ok(()));
        assert_eq!(encoder.bytes(&[0; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&[0; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&fields.key_id, 32), Ok(()));
        assert_eq!(encoder.u64(1_000), Ok(()));
        assert_eq!(encoder.u8(u8::from(signature.is_some())), Ok(()));
        if let Some(signature) = signature {
            assert_eq!(encoder.bytes(&signature, 64), Ok(()));
        }
        encoder.finish()
    }

    fn sign(fields: &Fields, key: &SigningKey) -> Vec<u8> {
        let unsigned = encode_fields(fields, None);
        let digest = receipt_digest(&unsigned).unwrap_or_else(|error| panic!("{error:?}"));
        encode_fields(fields, Some(key.sign(&digest).to_bytes()))
    }

    pub fn receipt() -> (Vec<u8>, ReceiptKeyEntry) {
        let signer = SigningKey::from_bytes(&[8; 32]);
        let fields = fields(7, [1; 32]);
        (
            sign(&fields, &signer),
            ReceiptKeyEntry {
                key_id: fields.key_id,
                public_key: signer.verifying_key().to_bytes(),
                first_height: 7,
                last_height: 7,
            },
        )
    }

    pub fn block() -> SequencerBlock {
        SequencerBlock {
            number: 7,
            hash: [17; 32],
            parent_hash: [16; 32],
            timestamp: 1_000,
            transactions_root: [18; 32],
            receipts_root: [20; 32],
            transactions: Vec::new(),
        }
    }

    pub fn commit(powers: &[u64], signers: &[usize]) -> (BlockCommit, Vec<Validator>) {
        let keys: Vec<_> = powers
            .iter()
            .enumerate()
            .map(|(index, _)| {
                SigningKey::from_bytes(
                    &[u8::try_from(index + 50).unwrap_or_else(|_| panic!("key index")); 32],
                )
            })
            .collect();
        let validators = keys
            .iter()
            .zip(powers)
            .map(|(key, power)| Validator {
                public_key: key.verifying_key().to_bytes(),
                voting_power: *power,
            })
            .collect();
        let mut commit = BlockCommit {
            height: 7,
            round: 0,
            block_hash: block().hash,
            part_set_total: 1,
            part_set_hash: [19; 32],
            signatures: Vec::new(),
        };
        for index in signers {
            let timestamp_seconds =
                1_000 + i64::try_from(*index).unwrap_or_else(|_| panic!("timestamp"));
            let bytes = must(commit.vote_sign_bytes(CHAIN_ID, timestamp_seconds, 0));
            commit.signatures.push(CommitSignature {
                validator_address: validator_address(&keys[*index].verifying_key().to_bytes()),
                timestamp_seconds,
                timestamp_nanos: 0,
                signature: keys[*index].sign(&bytes).to_bytes(),
            });
        }
        (commit, validators)
    }
}

use fixtures::*;

#[test]
fn receipt_signed_by_unpublished_key_does_not_reach_receipted() {
    let (receipt, published) = receipt();
    let mut submission = SubmissionProgress::accepted(IDEMPOTENCY_KEY, TX_HASH);
    assert!(matches!(
        wait_for_rung(&mut submission, IDEMPOTENCY_KEY, Rung::Receipted, 2, 1, 1),
        Err(FinalityError::InvalidDeadline)
    ));
    assert!(matches!(
        wait_for_rung(&mut submission, IDEMPOTENCY_KEY, Rung::Receipted, 1, 2, 0),
        Err(FinalityError::InvalidDeadline)
    ));
    let mut unpublished = published;
    unpublished.public_key = ed25519_dalek::SigningKey::from_bytes(&[99; 32])
        .verifying_key()
        .to_bytes();
    assert!(matches!(
        submission.observe_receipt(&receipt, &[unpublished]),
        Err(FinalityError::InvalidReceipt)
    ));
    assert_eq!(
        must(submission.rung_at(IDEMPOTENCY_KEY, 0)),
        Rung::Submitted
    );
    let waited = must(wait_for_rung(
        &mut submission,
        IDEMPOTENCY_KEY,
        Rung::Receipted,
        0,
        10,
        3,
    ));
    assert!(waited.deadline_elapsed);
    assert_eq!(waited.reached, Rung::Submitted);
    assert_eq!(waited.observed_at_ms, 10);
    assert!(matches!(
        submission.observe_receipt(&receipt, &[]),
        Err(FinalityError::InvalidReceipt)
    ));
    let mut wrong_id = published;
    wrong_id.key_id = [2; 32];
    assert!(matches!(
        submission.observe_receipt(&receipt, &[wrong_id]),
        Err(FinalityError::InvalidReceipt)
    ));
    let mut expired = published;
    expired.last_height = 6;
    expired.first_height = 1;
    assert!(matches!(
        submission.observe_receipt(&receipt, &[expired]),
        Err(FinalityError::InvalidReceipt)
    ));
    assert_eq!(
        must(submission.rung_at(IDEMPOTENCY_KEY, 10)),
        Rung::Submitted
    );
    let mut other_transaction = SubmissionProgress::accepted(IDEMPOTENCY_KEY, [4; 32]);
    assert!(matches!(
        other_transaction.observe_receipt(&receipt, &[published]),
        Err(FinalityError::InvalidReceipt)
    ));
    assert_eq!(
        must(other_transaction.rung_at(IDEMPOTENCY_KEY, 10)),
        Rung::Submitted
    );
    must(submission.observe_receipt(&receipt, &[published]));
    assert_eq!(
        must(submission.rung_at(IDEMPOTENCY_KEY, 11)),
        Rung::Receipted
    );
    assert!(matches!(
        wait_for_rung(
            &mut submission,
            IDEMPOTENCY_KEY,
            Rung::Committed,
            u64::MAX - 1,
            u64::MAX,
            5,
        ),
        Err(FinalityError::Arithmetic)
    ));
    assert!(
        !must(wait_for_rung(
            &mut submission,
            IDEMPOTENCY_KEY,
            Rung::Receipted,
            11,
            11,
            1
        ))
        .deadline_elapsed
    );
}

#[test]
fn committed_requires_more_than_two_thirds_voting_power() {
    let (receipt, key) = receipt();
    for (powers, signers, accepted) in [
        (vec![1, 1, 1], vec![0, 1], false),
        (vec![1, 1, 1, 1], vec![0, 1, 2], true),
        (vec![1, 1, 8], vec![0, 1], false),
        (vec![1, 1, 8], vec![2], true),
        (vec![u64::MAX, u64::MAX, u64::MAX], vec![0, 1], false),
        (
            vec![u64::MAX, u64::MAX, u64::MAX, u64::MAX],
            vec![0, 1, 2],
            true,
        ),
    ] {
        let mut submission = SubmissionProgress::accepted(IDEMPOTENCY_KEY, TX_HASH);
        must(submission.observe_receipt(&receipt, &[key]));
        let (commit, validators) = commit(&powers, &signers);
        let result = submission.observe_block(&block(), &commit, CHAIN_ID, &validators);
        if accepted {
            must(result);
        } else {
            assert!(matches!(result, Err(FinalityError::InvalidCommit)));
        }
        let result = must(wait_for_rung(
            &mut submission,
            IDEMPOTENCY_KEY,
            Rung::Committed,
            0,
            10,
            2,
        ));
        assert_eq!(
            result.reached,
            if accepted {
                Rung::Committed
            } else {
                Rung::Receipted
            }
        );
        assert_eq!(result.deadline_elapsed, !accepted);
    }
    let mut submission = SubmissionProgress::accepted(IDEMPOTENCY_KEY, TX_HASH);
    must(submission.observe_receipt(&receipt, &[key]));
    let (mut forged, validators) = commit(&[1, 1, 1], &[0, 1]);
    forged.signatures.push(forged.signatures[0].clone());
    assert!(matches!(
        submission.observe_block(&block(), &forged, CHAIN_ID, &validators),
        Err(FinalityError::InvalidCommit)
    ));
    let (mut forged, validators) = commit(&[1, 1, 1, 1], &[0, 1, 2]);
    forged.signatures[0].signature[0] ^= 1;
    assert!(matches!(
        submission.observe_block(&block(), &forged, CHAIN_ID, &validators),
        Err(FinalityError::InvalidCommit)
    ));
    let (valid, validators) = commit(&[1, 1, 1, 1], &[0, 1, 2]);
    assert!(matches!(
        submission.observe_block(&block(), &valid, "other-chain", &validators),
        Err(FinalityError::InvalidCommit)
    ));
    let mut wrong_block = block();
    wrong_block.number = 6;
    assert!(matches!(
        submission.observe_block(&wrong_block, &valid, CHAIN_ID, &validators),
        Err(FinalityError::InvalidCommit)
    ));
    wrong_block = block();
    wrong_block.hash[0] ^= 1;
    assert!(matches!(
        submission.observe_block(&wrong_block, &valid, CHAIN_ID, &validators),
        Err(FinalityError::InvalidCommit)
    ));
    assert_eq!(
        must(submission.rung_at(IDEMPOTENCY_KEY, 0)),
        Rung::Receipted
    );
}
