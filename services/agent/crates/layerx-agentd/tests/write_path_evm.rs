use layerx_agent_api::error::RequestId;
use layerx_agent_api::verify::RungStatus;
use layerx_agentd::agent_rpc_dispatch::{
    decode_write_answer, track_write_answer, wait_write_answer,
};
use layerx_agentd::finality::{wait_for_rung, SubmissionProgress, VerifiedReceipt};
use layerx_agentd::human_runtime::{accept_raw_write, write_submission};
use layerx_agentd::outbox::{Outbox, SubmissionState, SubmissionStatus};
use layerx_agentd::store::{Store, TenantId};
use layerx_crypto::evm_transaction::{decode_signed, transaction_hash, Eip1559Call};
use layerx_types::rung::Rung;

struct DurableWrite {
    root: std::path::PathBuf,
    store: Store,
    outbox: Outbox,
    hash: [u8; 32],
}

impl DurableWrite {
    fn queued(name: &str) -> Self {
        let root =
            std::env::temp_dir().join(format!("write-path-evm-{}-{name}", std::process::id()));
        std::fs::create_dir(&root).unwrap();
        let mut store = Store::open(&root).unwrap();
        let mut outbox = Outbox::default();
        let call = Eip1559Call {
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
            to: [0x1a; 20],
            data: vec![1, 2, 3, 4],
        };
        let raw = call.sign(&[0xa5; 32]).unwrap();
        let (_, signature) = decode_signed(&raw).unwrap();
        assert_eq!(signature.as_bytes().len(), 65);
        let expected = transaction_hash(&raw);
        let hash = outbox
            .enqueue(&mut store, TenantId::new("write-tenant").unwrap(), raw)
            .unwrap();
        assert_eq!(hash, expected);
        Self {
            root,
            store,
            outbox,
            hash,
        }
    }
}

impl Drop for DurableWrite {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.root).unwrap();
    }
}

#[test]
fn submit_returns_the_transaction_hash_and_submitted_state() {
    let mut write = DurableWrite::queued("submit");
    assert!(accept_raw_write(&mut write.outbox, &mut write.store, write.hash, [0x55; 32]).is_err());
    assert_eq!(
        write.outbox.status(write.hash).unwrap().state,
        SubmissionState::Queued
    );
    let status =
        accept_raw_write(&mut write.outbox, &mut write.store, write.hash, write.hash).unwrap();
    let (answer, rung) =
        decode_write_answer(RequestId(1), write_submission(&status, None).unwrap()).unwrap();
    assert_eq!(answer["tx_hash"], hex::encode(write.hash));
    assert_eq!(answer["submission_ref"], hex::encode(write.hash));
    assert_eq!(answer["state"], "Submitted");
    assert_eq!(answer["rung"], "submitted");
    assert_eq!(rung, Rung::Submitted);
    assert_eq!(write.outbox.status(write.hash).unwrap(), &status);
    assert_eq!(answer["evidence"], serde_json::json!([]));
    assert_eq!(answer["transitions"], serde_json::json!([]));
}

#[test]
fn wait_for_committed_reports_the_achieved_rung() {
    let (receipt, key) = fixtures::receipt();
    let verified = VerifiedReceipt::verify(fixtures::TX_HASH, &receipt, &[key]).unwrap();
    let status = SubmissionStatus {
        submission_id: fixtures::TX_HASH,
        activity_id: fixtures::TX_HASH,
        state: SubmissionState::Submitted,
        evidence: None,
        transitions: Vec::new(),
    };
    let (submission, rung) = decode_write_answer(
        RequestId(2),
        write_submission(&status, Some(&verified)).unwrap(),
    )
    .unwrap();
    assert_eq!(rung, Rung::Receipted);
    assert_eq!(submission["state"], "Executed");
    let mut progress = SubmissionProgress::accepted(fixtures::IDEMPOTENCY_KEY, fixtures::TX_HASH);
    progress.observe_receipt(&receipt, &[key]).unwrap();
    let waited = wait_for_rung(
        &mut progress,
        fixtures::IDEMPOTENCY_KEY,
        Rung::Committed,
        100,
        105,
        1,
    )
    .unwrap();
    let (answer, verification) = wait_write_answer(submission, waited, RequestId(2)).unwrap();
    assert_eq!(answer["actual_rung"], "receipted");
    assert_eq!(answer["deadline_elapsed"], true);
    assert_eq!(
        answer["submission"]["tx_hash"],
        hex::encode(fixtures::TX_HASH)
    );
    assert!(matches!(
        verification,
        RungStatus::Below {
            requested: Rung::Committed,
            achieved: Rung::Receipted,
            ..
        }
    ));
    let mut corrupt = receipt;
    *corrupt.last_mut().unwrap() ^= 1;
    assert!(VerifiedReceipt::verify(fixtures::TX_HASH, &corrupt, &[key]).is_err());
}

#[test]
fn track_reports_state_by_transaction_hash() {
    let mut write = DurableWrite::queued("track");
    accept_raw_write(&mut write.outbox, &mut write.store, write.hash, write.hash).unwrap();
    let reference = hex::encode(write.hash);
    let (answer, rung) = track_write_answer(RequestId(3), &write.outbox, &reference).unwrap();
    assert_eq!(answer["tx_hash"], reference);
    assert_eq!(answer["state"], "Submitted");
    assert_eq!(rung, Rung::Submitted);
    let refusal = track_write_answer(RequestId(4), &write.outbox, &"ab".repeat(31)).unwrap_err();
    assert_eq!(refusal.reason, "invalid_request");
    assert_eq!(refusal.request_id, RequestId(4));
    assert!(track_write_answer(RequestId(5), &write.outbox, &"ab".repeat(32)).is_err());
    let mut restored = Outbox::default();
    restored
        .restore(
            &write.store,
            TenantId::new("write-tenant").unwrap(),
            write.hash,
        )
        .unwrap();
    let (restored_answer, _) = track_write_answer(RequestId(6), &restored, &reference).unwrap();
    assert_eq!(restored_answer, answer);
}

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
