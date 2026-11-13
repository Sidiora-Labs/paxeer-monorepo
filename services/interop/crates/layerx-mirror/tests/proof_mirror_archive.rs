//! Proves: `CommittedBlock` -> `Archive::from_committed` / `Archive::bytes` -> `ArchiveData::decode` / `MirrorVerifier::committed_receipt`
//!
//! The crossing under proof is the public Rust archive boundary. Producer state
//! uses the signed receipts emitted by `rpc/sequencer_receipt_vectors_test.go`,
//! retained in `fixtures/committed_blocks.json`; that fixture adds block coordinates.
//! No RPC server, archive codec, observation decoder or verifier is replaced.
//! `CommittedBlockSource::acquire`'s public signature is also checked below; this
//! proof does not claim to exercise an external node or a chain publication.
//! Each iteration rebuilds the producer and decodes a fresh owned byte vector;
//! this boundary has no persistent state or cache to reopen.
//!
//! Repaired defect: src/committed.rs `CommittedBlock::validate_archive` and
//! src/publisher.rs `Archive::from_committed` now refuse incomplete public blocks:
//! previously a transaction list with missing or extra receipts could be archived.
//! src/runtime.rs `RuntimeConfig::committed_network_id` shares the checked source
//! identity conversion between runtime validation, acquisition and this consumer.

use layerx_mirror::committed::{
    CommittedBlock, CommittedBlockSource, CommittedHeader, CommittedRefusal,
};
use layerx_mirror::runtime::RuntimeConfig;
use layerx_mirror::source::{
    MirrorCanonicalPosition, MirrorObservation, MirrorSourceId, MirrorTargetIdentity,
};
use layerx_mirror::{
    archive_commitment, Archive, ArchiveData, ArchiveError, ArchivedCommittedBlock,
    CommittedCoordinate, MirrorEvidenceLevel, MirrorVerifier, MirrorVerifyError, NodeHead,
    SourceError,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::ReceiptCheck;
use layerx_wire::receipt::encode;
use serde_json::{json, Value};

fn fixture() -> Value {
    serde_json::from_str(include_str!("fixtures/committed_blocks.json"))
        .unwrap_or_else(|error| panic!("producer receipt vectors: {error}"))
}

fn bytes(value: &Value) -> Vec<u8> {
    let digits = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .unwrap_or_else(|| panic!("producer hex bytes"));
    assert_eq!(digits.len() % 2, 0);
    (0..digits.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&digits[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("producer hex: {error}"))
        })
        .collect()
}

fn config_document() -> Value {
    let mut value: Value =
        serde_json::from_str(include_str!("../../../deploy/mirror/config.example.json"))
            .unwrap_or_else(|error| panic!("publisher configuration: {error}"));
    let producer = fixture();
    value["first_height"] = json!(1000);
    value["paxeer"]["genesis_hash_hex"] = producer["genesis"]["hash"].clone();
    value
}

fn config() -> RuntimeConfig {
    serde_json::from_value(config_document())
        .unwrap_or_else(|error| panic!("runtime consumer config: {error}"))
}

fn producer_block() -> CommittedBlock {
    let producer = fixture();
    CommittedBlock {
        header: CommittedHeader::from_block_json(&producer["committedBlock"])
            .unwrap_or_else(|error| panic!("producer header: {error:?}")),
        receipts: producer["receipts"]
            .as_array()
            .unwrap_or_else(|| panic!("producer receipts"))
            .iter()
            .map(|receipt| bytes(&receipt["receipt"]))
            .collect(),
    }
}

fn head(block: &CommittedBlock) -> NodeHead {
    NodeHead {
        latest_sealed_batch: block.header.height,
        latest_committed: Some(CommittedCoordinate {
            height: block.header.height,
            block_hash: block.header.block_hash,
        }),
    }
}

fn archive(block: &CommittedBlock) -> Archive {
    Archive::from_committed(
        block,
        config()
            .committed_network_id()
            .unwrap_or_else(|error| panic!("source identity: {error:?}")),
        head(block),
    )
    .unwrap_or_else(|error| panic!("real archive producer: {error:?}"))
}

fn receipt_keys(config: &RuntimeConfig) -> Vec<SequencerAuthorization> {
    config
        .paxeer
        .receipt_keys
        .iter()
        .map(|key| {
            SequencerAuthorization::from_config(
                &key.sequencer_id_hex,
                &key.public_key_hex,
                &key.first_height.to_string(),
                &key.last_height.to_string(),
            )
            .unwrap_or_else(|field| panic!("independent receipt authority: {field}"))
        })
        .collect()
}

fn observation(archive: &Archive) -> MirrorObservation {
    // The public consumer accepts an independently supplied source observation.
    // from_archive performs the actual commitment, network and height checks.
    MirrorObservation::from_archive(
        MirrorSourceId::new("archive-proof")
            .unwrap_or_else(|error| panic!("source identifier: {error:?}")),
        MirrorTargetIdentity::Ethereum {
            chain_id: 1,
            genesis_hash: [1; 32],
            contract: [2; 20],
            code_hash: [3; 32],
            publisher: [4; 20],
        },
        archive.bytes(),
        archive.data().network_id,
        archive.data().batch_number,
        archive.commitment(),
        MirrorCanonicalPosition::Ethereum {
            block_number: 50,
            block_hash: [5; 32],
            reference_head_number: 62,
            reference_head_hash: [6; 32],
        },
    )
    .unwrap_or_else(|error| panic!("real observation consumer: {error:?}"))
}

#[test]
fn proof_mirror_archive() {
    let runtime = config();
    assert_eq!(runtime.first_height, 1000);
    assert_eq!(runtime.freshness_budget_blocks, 64);
    assert_eq!(runtime.paxeer.rpc.endpoints.len(), 3);
    assert_eq!(runtime.paxeer.rpc.quorum, 2);
    assert_eq!(runtime.paxeer.chain_id, 125);
    assert_eq!(
        runtime.paxeer.genesis_hash_hex,
        fixture()["genesis"]["hash"]
    );
    let keys = receipt_keys(&runtime);
    let mut previous_bytes = None;

    // Compile against the exact public method that real acquisition callers use.
    let _: fn(&CommittedBlockSource, u64) -> Result<CommittedBlock, CommittedRefusal> =
        CommittedBlockSource::acquire;

    for _ in 0..2 {
        let block = producer_block();
        assert_eq!(block.validate_archive(), Ok(()));
        let produced = archive(&block);
        let observed = observation(&produced);
        let transported = produced.bytes().to_vec();
        let expected = ArchivedCommittedBlock {
            block_hash: block.header.block_hash,
            parent_hash: block.header.parent_hash,
            app_hash: block.header.app_hash,
            timestamp: block.header.timestamp,
            receipts: block.receipts.clone(),
        };
        let expected_coordinate = head(&block).latest_committed;
        let transactions = block.header.transactions.clone();
        drop(produced);
        drop(block);

        let decoded = ArchiveData::decode(&transported)
            .unwrap_or_else(|error| panic!("real archive consumer: {error:?}"));
        assert_eq!(decoded.network_id, 125);
        assert_eq!(decoded.batch_number, runtime.first_height);
        assert_eq!(decoded.committed, expected);
        assert_eq!(decoded.committed_coordinate(), expected_coordinate);
        let consumer = MirrorVerifier::admit_committed(&transported, observed)
            .unwrap_or_else(|error| panic!("real verifier admission: {error:?}"));
        assert_eq!(consumer.committed().ok(), expected_coordinate);
        for (index, transaction) in transactions.iter().enumerate() {
            let result = consumer
                .committed_receipt(index, &keys)
                .unwrap_or_else(|error| panic!("receipt consumer: {error:?}"));
            assert_eq!(result.level(), MirrorEvidenceLevel::Committed);
            assert_eq!(result.batch_number(), runtime.first_height);
            assert_eq!(
                result.freshness().latest_committed_mirrored,
                expected_coordinate
            );
            assert_eq!(
                result
                    .value()
                    .protocol()
                    .map(layerx_wire::receipt::ProtocolReceipt::activity_id),
                Some(*transaction)
            );
            assert_eq!(
                encode(result.value()).ok(),
                Some(expected.receipts[index].clone())
            );
        }
        assert_eq!(
            consumer.committed_receipt(transactions.len(), &keys),
            Err(MirrorVerifyError::ReceiptMissing)
        );
        let wrong_signer = [SequencerAuthorization::new(
            keys[0].sequencer_id(),
            [9; 32],
            keys[0].first_batch_number(),
            keys[0].last_batch_number(),
        )];
        assert_eq!(
            consumer.committed_receipt(0, &wrong_signer),
            Err(MirrorVerifyError::Receipt(ReceiptCheck::SequencerSignature))
        );
        if let Some(previous) = previous_bytes {
            assert_eq!(transported, previous);
        }
        previous_bytes = Some(transported);
    }
}

#[test]
fn committed_malformed() {
    let complete = producer_block();
    let mut missing = complete.clone();
    missing.receipts.pop();
    assert_eq!(missing.validate_archive(), Err(CommittedRefusal::Malformed));
    assert_eq!(
        Archive::from_committed(&missing, 125, head(&missing)),
        Err(SourceError::Committed)
    );
    let mut extra = complete.clone();
    extra.receipts.push(complete.receipts[0].clone());
    assert_eq!(
        Archive::from_committed(&extra, 125, head(&extra)),
        Err(SourceError::Committed)
    );
    let mut empty = complete.clone();
    empty.receipts[0].clear();
    assert_eq!(
        Archive::from_committed(&empty, 125, head(&empty)),
        Err(SourceError::Committed)
    );
    let mut unidentified = complete.clone();
    unidentified.header.block_hash = [0; 32];
    assert_eq!(
        Archive::from_committed(&unidentified, 125, head(&unidentified)),
        Err(SourceError::Committed)
    );

    // The archive framing can contain a nonempty malformed receipt. The real
    // receipt consumer must refuse it at decoding, even with a valid commitment.
    let mut malformed_receipt = complete.clone();
    malformed_receipt.receipts[0].truncate(1);
    let produced = archive(&malformed_receipt);
    let consumer = MirrorVerifier::admit_committed(produced.bytes(), observation(&produced))
        .unwrap_or_else(|error| panic!("framed malformed receipt: {error:?}"));
    assert_eq!(
        consumer.committed_receipt(0, &receipt_keys(&config())),
        Err(MirrorVerifyError::Receipt(ReceiptCheck::Decode))
    );

    let produced = archive(&complete);
    let valid = produced.bytes();
    for length in [0, 8, 9, 21, valid.len() - 1] {
        let malformed = &valid[..length];
        assert!(ArchiveData::decode(malformed).is_err());
        let mut observed = observation(&produced);
        // Match the malformed payload's commitment so the consumer must decode it.
        observed.commitment = archive_commitment(malformed);
        assert!(matches!(
            MirrorVerifier::admit_committed(malformed, observed),
            Err(MirrorVerifyError::Archive(_))
        ));
    }
    let mut unsupported = valid.to_vec();
    unsupported[8..10].copy_from_slice(&3_u16.to_be_bytes());
    assert_eq!(
        ArchiveData::decode(&unsupported),
        Err(ArchiveError::Version)
    );
    let mut tampered = valid.to_vec();
    tampered[22] ^= 1;
    assert!(matches!(
        MirrorVerifier::admit_committed(&tampered, observation(&produced)),
        Err(MirrorVerifyError::SourceCommitment)
    ));

    let mut malformed_header = fixture()["committedBlock"].clone();
    malformed_header["number"] = json!("0x03e8");
    let refusal = CommittedHeader::from_block_json(&malformed_header)
        .err()
        .unwrap_or_else(|| panic!("noncanonical header accepted"));
    assert_eq!(refusal, CommittedRefusal::Malformed);
    assert_eq!(refusal.code(), "committed_malformed");
    for (refusal, code) in [
        (CommittedRefusal::Unavailable, "committed_unavailable"),
        (CommittedRefusal::ChainMismatch, "committed_chain_mismatch"),
        (
            CommittedRefusal::GenesisMismatch,
            "committed_genesis_mismatch",
        ),
    ] {
        assert_eq!(refusal.code(), code);
    }
    let mut invalid_source = config().paxeer;
    invalid_source.genesis_hash_hex = "invalid".to_owned();
    assert!(matches!(
        CommittedBlockSource::open(&invalid_source),
        Err(CommittedRefusal::Malformed)
    ));
}

#[test]
fn u64() {
    for field in ["first_height", "freshness_budget_blocks"] {
        for number in [1, u64::MAX] {
            let mut document = config_document();
            document[field] = json!(number);
            let decoded: RuntimeConfig = serde_json::from_value(document)
                .unwrap_or_else(|error| panic!("u64 configuration: {error}"));
            assert_eq!(
                if field == "first_height" {
                    decoded.first_height
                } else {
                    decoded.freshness_budget_blocks
                },
                number
            );
        }
        for invalid in [json!(-1), json!(1.5), json!("1"), Value::Null] {
            let mut document = config_document();
            document[field] = invalid;
            assert!(serde_json::from_value::<RuntimeConfig>(document).is_err());
        }
        let mut document = config_document();
        document[field] = json!("OVERFLOW");
        let overflow = serde_json::to_string(&document)
            .unwrap_or_else(|error| panic!("serialize config: {error}"))
            .replace("\"OVERFLOW\"", "18446744073709551616");
        assert!(serde_json::from_str::<RuntimeConfig>(&overflow).is_err());
    }
    let mut runtime = config();
    for chain_id in [0, u64::from(u32::MAX) + 1, u64::MAX] {
        runtime.paxeer.chain_id = chain_id;
        assert_eq!(
            runtime.committed_network_id(),
            Err(CommittedRefusal::ChainMismatch)
        );
    }
    runtime.paxeer.chain_id = u64::from(u32::MAX);
    assert_eq!(runtime.committed_network_id(), Ok(u32::MAX));
}
