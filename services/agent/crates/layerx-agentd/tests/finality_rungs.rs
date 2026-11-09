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

    pub const TX_HASH: [u8; 32] = [
        0xe4, 0xdd, 0x02, 0x96, 0x01, 0x28, 0xef, 0x55, 0x00, 0x3e, 0x3e, 0x04, 0xe7, 0x8e, 0x49,
        0x3c, 0x66, 0xc6, 0x93, 0x5e, 0x17, 0x8a, 0xb4, 0xe8, 0xf4, 0x7d, 0xaa, 0x23, 0x33, 0x45,
        0x9e, 0x38,
    ];
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
            activity_id: TX_HASH,
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
        let payment_denom = "upax";
        let fee_asset: [u8; 32] = Sha256::digest(payment_denom.as_bytes()).into();
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

    pub fn transaction() -> Vec<u8> {
        let raw = must(
            layerx_crypto::evm_transaction::Eip1559Call {
                chain_id: 125,
                nonce: 7,
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 2,
                gas_limit: 21_000,
                to: [7; 20],
                data: Vec::new(),
            }
            .sign(&[6; 32]),
        );
        assert_eq!(
            layerx_crypto::evm_transaction::transaction_hash(&raw),
            TX_HASH
        );
        raw
    }

    pub fn header(validators: &[Validator]) -> layerx_agentd::read::CheckpointHeader {
        use layerx_agentd::read::{
            validator_set_hash, CheckpointHeader, CommitTimestamp, CommitValidator,
        };
        let validators: Vec<_> = validators
            .iter()
            .map(|validator| CommitValidator {
                address: validator_address(&validator.public_key),
                public_key: validator.public_key,
                voting_power: validator.voting_power,
            })
            .collect();
        CheckpointHeader {
            version_block: 11,
            version_app: 1,
            chain_id: CHAIN_ID.to_owned(),
            height: 7,
            time: CommitTimestamp {
                seconds: 1000,
                nanos: 0,
            },
            last_block_id: None,
            last_commit_hash: None,
            data_hash: Some(layerx_programs::chain_proof::merkle::hash_from_byte_slices(
                &[&Sha256::digest(transaction())],
            )),
            validators_hash: validator_set_hash(&validators),
            next_validators_hash: validator_set_hash(&validators),
            consensus_hash: [2; 32],
            app_hash: [18; 32],
            last_results_hash: Some([20; 32]),
            evidence_hash: None,
            proposer_address: validators[0].address,
        }
    }

    pub fn block_for(validators: &[Validator]) -> SequencerBlock {
        use layerx_client::sequencer::SequencerEndpoint;
        use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
        use serde_json::{json, Value};
        use std::io::{BufRead, BufReader, Read, Write};
        use std::net::TcpListener;
        use std::time::Duration;

        let header = header(validators);
        let hash = header.block_hash();
        let raw = transaction();
        let (call, signature) = must(decode_signed(&raw));
        let quantity = |bytes: &[u8]| {
            let text = hex::encode(bytes);
            format!("0x{}", text.trim_start_matches('0'))
        };
        let parity = signature.as_bytes()[64] - 27;
        let response = json!({
            "number": "0x7", "hash": format!("0x{}", hex::encode(hash)),
            "parentHash": format!("0x{}", hex::encode([16; 32])), "timestamp": "0x3e8",
            "transactionsRoot": format!("0x{}", hex::encode(header.data_hash.unwrap())),
            "receiptsRoot": format!("0x{}", hex::encode([20; 32])),
            "transactions": [{
                "blockHash": format!("0x{}", hex::encode(hash)), "blockNumber": "0x7",
                "from": format!("0x{}", hex::encode(must(recover_sender(&call, &signature)))),
                "gas": format!("{:#x}", call.gas_limit), "gasPrice": "0x1",
                "hash": format!("0x{}", hex::encode(TX_HASH)), "input": "0x",
                "nonce": format!("{:#x}", call.nonce), "to": format!("0x{}", hex::encode(call.to)),
                "transactionIndex": "0x0", "value": "0x0", "type": "0x2",
                "v": format!("{parity:#x}"), "yParity": format!("{parity:#x}"),
                "r": quantity(&signature.as_bytes()[..32]), "s": quantity(&signature.as_bytes()[32..64]),
                "accessList": [], "chainId": format!("{:#x}", call.chain_id),
                "maxFeePerGas": format!("{:#x}", call.max_fee_per_gas),
                "maxPriorityFeePerGas": format!("{:#x}", call.max_priority_fee_per_gas)
            }]
        });
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let endpoint = must(SequencerEndpoint::new(
            &format!("http://{}/", must(listener.local_addr())),
            &[],
        ));
        let server = std::thread::spawn(move || {
            let (stream, _) = must(listener.accept());
            must(stream.set_read_timeout(Some(Duration::from_secs(5))));
            must(stream.set_write_timeout(Some(Duration::from_secs(5))));
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(must(value.trim().parse::<usize>()));
                }
            }
            let length = length.unwrap();
            assert!(length <= 64 * 1024);
            let mut body = vec![0; length];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["method"], "eth_getBlockByNumber");
            assert_eq!(request["params"], json!(["0x7", true]));
            let body =
                json!({"jsonrpc":"2.0", "id": request["id"], "result": response}).to_string();
            let mut stream = reader.into_inner();
            must(write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                body.len()
            ));
            must(stream.flush());
        });
        let block = must(endpoint.block_by_number(header.height)).unwrap();
        must(server.join());
        assert_eq!(block.number, header.height);
        assert_eq!(block.hash, hash);
        assert_eq!(block.transactions, vec![raw]);
        block
    }

    pub fn assert_fixture_binding(
        header: &layerx_agentd::read::CheckpointHeader,
        commit: &BlockCommit,
        block: &SequencerBlock,
        validators: &[Validator],
    ) {
        use layerx_agentd::read::{validator_set_hash, CommitTimestamp, CommitValidator};
        assert_eq!(header.height, block.number);
        assert_eq!(commit.height, block.number);
        assert_eq!(header.chain_id, CHAIN_ID);
        assert_eq!(header.version_block, 11);
        let trusted: Vec<_> = validators
            .iter()
            .map(|validator| CommitValidator {
                address: validator_address(&validator.public_key),
                public_key: validator.public_key,
                voting_power: validator.voting_power,
            })
            .collect();
        assert_eq!(header.validators_hash, validator_set_hash(&trusted));
        assert_eq!(header.block_hash(), commit.block_hash);
        assert_eq!(block.hash, commit.block_hash);
        assert!(!block.transactions.is_empty());
        assert_eq!(block.transactions, vec![transaction()]);
        let leaves: Vec<_> = block
            .transactions
            .iter()
            .map(|raw| Sha256::digest(raw).to_vec())
            .collect();
        let leaves: Vec<_> = leaves.iter().map(Vec::as_slice).collect();
        assert_eq!(
            header.data_hash,
            Some(layerx_programs::chain_proof::merkle::hash_from_byte_slices(
                &leaves
            ))
        );
        let native = layerx_agentd::read::BlockCommit {
            height: commit.height,
            round: commit.round,
            block_id: layerx_agentd::read::BlockId {
                hash: commit.block_hash,
                part_set_total: commit.part_set_total,
                part_set_hash: commit.part_set_hash,
            },
            votes: Vec::new(),
        };
        for signature in &commit.signatures {
            let timestamp = CommitTimestamp {
                seconds: signature.timestamp_seconds,
                nanos: signature.timestamp_nanos,
            };
            assert_eq!(
                must(commit.vote_sign_bytes(CHAIN_ID, timestamp.seconds, timestamp.nanos)),
                native.vote_sign_bytes(CHAIN_ID, timestamp),
            );
        }
    }

    pub fn block() -> SequencerBlock {
        let (_, validators) = commit(&[1, 1, 1, 1], &[0, 1, 2]);
        block_for(&validators)
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
        let validators: Vec<Validator> = keys
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
            block_hash: header(&validators).block_hash(),
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
            false,
        ),
    ] {
        let mut submission = SubmissionProgress::accepted(IDEMPOTENCY_KEY, TX_HASH);
        must(submission.observe_receipt(&receipt, &[key]));
        let (commit, validators) = commit(&powers, &signers);
        let block = block_for(&validators);
        let header = header(&validators);
        assert_fixture_binding(&header, &commit, &block, &validators);
        let result =
            submission.observe_block_with_header(&block, &commit, &header, CHAIN_ID, &validators);
        if accepted {
            result.unwrap_or_else(|error| panic!("quorum powers {powers:?}: {error:?}"));
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
    for (powers, signers, quorum) in [
        (vec![u64::MAX; 3], vec![0, 1], false),
        (vec![u64::MAX; 4], vec![0, 1, 2], true),
    ] {
        let (commit, validators) = commit(&powers, &signers);
        let block = block_for(&validators);
        let verified =
            layerx_agentd::finality::VerifiedBlock::verify(&block, &commit, CHAIN_ID, &validators);
        if quorum {
            must(verified);
        } else {
            assert!(matches!(verified, Err(FinalityError::InvalidCommit)));
        }
        let mut submission = SubmissionProgress::accepted(IDEMPOTENCY_KEY, TX_HASH);
        must(submission.observe_receipt(&receipt, &[key]));
        assert!(matches!(
            submission.observe_block(&block, &commit, CHAIN_ID, &validators),
            Err(FinalityError::InvalidCommit),
        ));
        assert_eq!(
            must(submission.rung_at(IDEMPOTENCY_KEY, 0)),
            Rung::Receipted
        );
    }
    let mut submission = SubmissionProgress::accepted(IDEMPOTENCY_KEY, TX_HASH);
    must(submission.observe_receipt(&receipt, &[key]));
    let (mut forged, validators) = commit(&[1, 1, 1], &[0, 1]);
    forged.signatures.push(forged.signatures[0].clone());
    assert!(matches!(
        submission.observe_block_with_header(
            &block_for(&validators),
            &forged,
            &header(&validators),
            CHAIN_ID,
            &validators
        ),
        Err(FinalityError::InvalidCommit)
    ));
    let (mut forged, validators) = commit(&[1, 1, 1, 1], &[0, 1, 2]);
    forged.signatures[0].signature[0] ^= 1;
    assert!(matches!(
        submission.observe_block_with_header(
            &block_for(&validators),
            &forged,
            &header(&validators),
            CHAIN_ID,
            &validators
        ),
        Err(FinalityError::InvalidCommit)
    ));
    let (valid, validators) = commit(&[1, 1, 1, 1], &[0, 1, 2]);
    assert!(matches!(
        submission.observe_block_with_header(
            &block_for(&validators),
            &valid,
            &header(&validators),
            "other-chain",
            &validators
        ),
        Err(FinalityError::InvalidCommit)
    ));
    let mut wrong_block = block();
    wrong_block.number = 6;
    assert!(matches!(
        submission.observe_block_with_header(
            &wrong_block,
            &valid,
            &header(&validators),
            CHAIN_ID,
            &validators
        ),
        Err(FinalityError::InvalidCommit)
    ));
    wrong_block = block();
    wrong_block.hash[0] ^= 1;
    assert!(matches!(
        submission.observe_block_with_header(
            &wrong_block,
            &valid,
            &header(&validators),
            CHAIN_ID,
            &validators
        ),
        Err(FinalityError::InvalidCommit)
    ));
    assert_eq!(
        must(submission.rung_at(IDEMPOTENCY_KEY, 0)),
        Rung::Receipted
    );
}

#[test]
fn empty_or_unrelated_block_cannot_promote_a_receipt() {
    let (receipt, key) = receipt();
    let (commit, validators) = commit(&[1, 1, 1, 1], &[0, 1, 2]);
    let header = header(&validators);
    for mutation in ["empty", "foreign", "missing-header"] {
        let mut progress = SubmissionProgress::accepted(IDEMPOTENCY_KEY, TX_HASH);
        must(progress.observe_receipt(&receipt, &[key]));
        let mut block = block_for(&validators);
        let result = match mutation {
            "empty" => {
                block.transactions.clear();
                progress.observe_block_with_header(&block, &commit, &header, CHAIN_ID, &validators)
            }
            "foreign" => {
                block.transactions[0] = must(
                    layerx_crypto::evm_transaction::Eip1559Call {
                        chain_id: 125,
                        nonce: 8,
                        max_priority_fee_per_gas: 1,
                        max_fee_per_gas: 2,
                        gas_limit: 21_000,
                        to: [7; 20],
                        data: vec![],
                    }
                    .sign(&[6; 32]),
                );
                progress.observe_block_with_header(&block, &commit, &header, CHAIN_ID, &validators)
            }
            _ => progress.observe_block(&block, &commit, CHAIN_ID, &validators),
        };
        assert!(matches!(result, Err(FinalityError::InvalidCommit)));
        assert_eq!(must(progress.rung_at(IDEMPOTENCY_KEY, 0)), Rung::Receipted);
    }
}
