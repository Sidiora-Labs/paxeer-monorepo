//! Proves: Header::hash/merkle::root/verify_commit/verify_receipted -> public CommittedProof/digest -> committed::verify_committed/CommittedBlock.
//!
//! Signed inputs are emitted by daemon/committed-fixture/vector_v4.go::recordV4
//! and rpc/sequencer_receipt_vectors_test.go::TestSequencerReceiptVectors, then
//! decoded into the public Rust types without replacing any verifier or transport.
//! This stateless boundary is crossed afresh twice, with refusals between reads.
//!
//! Repairs services/interop/crates/layerx-committed-proof/src/lib.rs::verify_header:
//! a malformed producer timestamp previously panicked inside Header::hash; public
//! verification must return ProofError::InvalidCommit. Header hashing and commit
//! verification now share Timestamp::is_valid so their encoding bounds agree.

use layerx_committed_proof::{
    commit::{verify_commit, BlockIdFlag, Commit, CommitSig},
    committed::verify_committed,
    header::{BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator},
    merkle::{root, verify_proof},
    verify_header, verify_receipted, CommittedBlock, CommittedProof, ProofError,
};
use serde_json::Value;
use sha2::{Digest, Sha256};

const VECTORS: &str = include_str!("../../../specs/conformance/committed/vector-v4-committed.json");
const RECORDING: &[u8] =
    include_bytes!("../../../../../daemon/committed-fixture/vectors/vector-v4-committed.json");
const RECEIPTS: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

fn string<'a>(value: &'a Value, field: &str) -> &'a str {
    value[field]
        .as_str()
        .unwrap_or_else(|| panic!("missing string {field}"))
}

fn number(value: &Value, field: &str) -> u64 {
    value[field]
        .as_u64()
        .unwrap_or_else(|| panic!("missing unsigned number {field}"))
}

fn signed(value: &Value, field: &str) -> i64 {
    value[field]
        .as_i64()
        .unwrap_or_else(|| panic!("missing signed number {field}"))
}

fn array<'a>(value: &'a Value, field: &str) -> &'a [Value] {
    value[field]
        .as_array()
        .unwrap_or_else(|| panic!("missing array {field}"))
}

fn hex_bytes(hex: &str) -> Vec<u8> {
    let hex = hex
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("missing hex prefix"));
    assert!(hex.is_ascii());
    assert_eq!(hex.len() % 2, 0);
    (0..hex.len())
        .step_by(2)
        .map(|i| {
            u8::from_str_radix(&hex[i..i + 2], 16)
                .unwrap_or_else(|error| panic!("invalid vector hex: {error}"))
        })
        .collect()
}

fn bytes(value: &Value, field: &str) -> Vec<u8> {
    hex_bytes(string(value, field))
}

fn fixed<const N: usize>(value: &Value, field: &str) -> [u8; N] {
    bytes(value, field)
        .try_into()
        .unwrap_or_else(|_| panic!("incorrect byte length for {field}"))
}

fn timestamp(value: &Value) -> Timestamp {
    Timestamp {
        seconds: signed(value, "seconds"),
        nanos: number(value, "nanos")
            .try_into()
            .unwrap_or_else(|_| panic!("timestamp nanos overflow")),
    }
}

fn block_id(value: &Value) -> BlockId {
    BlockId {
        hash: bytes(value, "hash"),
        part_set_header: PartSetHeader {
            total: number(&value["parts"], "total")
                .try_into()
                .unwrap_or_else(|_| panic!("part count overflow")),
            hash: bytes(&value["parts"], "hash"),
        },
    }
}

fn header(value: &Value) -> Header {
    Header {
        version: ConsensusVersion {
            block: number(&value["version"], "block"),
            app: number(&value["version"], "app"),
        },
        chain_id: string(value, "chain_id").to_owned(),
        height: signed(value, "height"),
        time: timestamp(&value["time"]),
        last_block_id: block_id(&value["last_block_id"]),
        last_commit_hash: bytes(value, "last_commit_hash"),
        data_hash: bytes(value, "data_hash"),
        validators_hash: fixed(value, "validators_hash"),
        next_validators_hash: bytes(value, "next_validators_hash"),
        consensus_hash: bytes(value, "consensus_hash"),
        app_hash: bytes(value, "app_hash"),
        last_results_hash: bytes(value, "last_results_hash"),
        evidence_hash: bytes(value, "evidence_hash"),
        proposer_address: bytes(value, "proposer_address"),
    }
}

fn commit(value: &Value) -> Commit {
    Commit {
        height: number(value, "height"),
        round: signed(value, "round")
            .try_into()
            .unwrap_or_else(|_| panic!("commit round overflow")),
        block_id: block_id(&value["block_id"]),
        signatures: array(value, "signatures")
            .iter()
            .map(|vote| CommitSig {
                block_id_flag: match number(vote, "block_id_flag") {
                    1 => BlockIdFlag::Absent,
                    2 => BlockIdFlag::Commit,
                    3 => BlockIdFlag::Nil,
                    flag => panic!("unknown block id flag {flag}"),
                },
                validator_address: bytes(vote, "validator_address"),
                timestamp: timestamp(&vote["timestamp"]),
                signature: if vote["signature"].is_null() {
                    None
                } else {
                    Some(fixed(vote, "signature"))
                },
            })
            .collect(),
    }
}

fn proof(case: &Value) -> CommittedProof {
    let batch = &case["batchProof"];
    assert_eq!(string(batch, "leafEncoding"), "sha256-tx");
    assert_eq!(
        bytes(batch, "rootHash"),
        bytes(&case["executionHeader"], "data_hash")
    );
    CommittedProof {
        header: header(&case["header"]),
        commit: commit(&case["commit"]),
        execution_header: header(&case["executionHeader"]),
        batch_transaction: bytes(batch, "data"),
        batch_total: number(batch, "total"),
        batch_index: number(batch, "index"),
        batch_aunts: array(batch, "aunts")
            .iter()
            .map(|aunt| hex_bytes(aunt.as_str().unwrap()).try_into().unwrap())
            .collect(),
        signed_receipt: bytes(case, "signedReceipt"),
        activity_ids: vec![fixed(&case["receiptJson"], "txHash")],
    }
}

fn validators(case: &Value) -> Vec<Validator> {
    array(case, "validators")
        .iter()
        .map(|validator| Validator {
            pub_key: fixed(validator, "pub_key"),
            voting_power: signed(validator, "voting_power"),
        })
        .collect()
}

fn pins(case: &Value) -> [([u8; 32], [u8; 32], u64, u64); 1] {
    let pin = &case["pinnedKey"];
    [(
        fixed(pin, "keyId"),
        fixed(pin, "publicKey"),
        number(pin, "firstHeight"),
        number(pin, "lastHeight"),
    )]
}

fn committed_digest(case: &Value, proof: &CommittedProof) -> Result<[u8; 32], ProofError> {
    layerx_committed_proof::verify_committed(
        &bytes(case, "rawTransaction"),
        &proof.signed_receipt,
        &serde_json::to_vec(&case["receiptJson"]).unwrap(),
        fixed(case, "prevSeqHash"),
        &pins(case),
        proof.execution_header.hash(),
    )
}

#[test]
fn proof_rs_committed_proof_crate() {
    // Each iteration decodes new producer bytes and calls all public functions;
    // neither the authenticated digest nor the committed result is reused.
    for _ in 0..2 {
        let vectors: Value = serde_json::from_str(VECTORS).unwrap();
        let recording: Value = serde_json::from_slice(RECORDING).unwrap();
        let source_hash: [u8; 32] = Sha256::digest(RECORDING).into();
        let recorded_hash: [u8; 32] = hex_bytes(&format!(
            "0x{}",
            string(&vectors["provenance"], "sourceSha256")
        ))
        .try_into()
        .unwrap();
        assert_eq!(source_hash, recorded_hash);
        let case = array(&vectors, "cases")
            .iter()
            .find(|case| string(case, "id") == "committed-payExact")
            .unwrap();
        assert_eq!(case["receiptJson"], recording["settlement"]["receipt"]);
        assert_eq!(case["signedReceipt"], recording["positive"]["receipt"]);
        assert_eq!(
            case["rawTransaction"],
            recording["settlement"]["raw_transaction"]
        );

        let produced = proof(case);
        let set = validators(case);
        let chain = string(&vectors["provenance"], "chainId");
        let expected = CommittedBlock {
            number: number(&vectors["provenance"], "executionHeight"),
            hash: fixed(&case["receiptJson"], "blockHash"),
        };
        // The recorded block contains the sequence batch and its Cosmos-wrapped
        // EVM transaction. Hash the real transaction bytes before merkle::root.
        let batch_leaf: [u8; 32] = Sha256::digest(&produced.batch_transaction).into();
        let transaction_leaf: [u8; 32] =
            Sha256::digest(bytes(&case["transactionProof"], "data")).into();
        let data_root = root(&[&batch_leaf, &transaction_leaf]);
        assert_eq!(produced.batch_total, 2);
        assert_eq!(produced.execution_header.data_hash, data_root);
        assert_eq!(produced.execution_header.hash(), expected.hash);
        assert_eq!(
            verify_header(&produced.header, &set),
            Ok(produced.header.hash())
        );
        assert_eq!(
            verify_commit(
                chain,
                &set,
                produced.header.hash(),
                produced.header.height.try_into().unwrap(),
                &produced.commit
            ),
            Ok(number(&case["expected"], "power")),
        );
        assert_eq!(
            verify_proof(
                data_root,
                produced.batch_total,
                produced.batch_index,
                &batch_leaf,
                &produced.batch_aunts
            ),
            Ok(())
        );
        let digest = committed_digest(case, &produced).unwrap();
        // This exact producer return value crosses the public Rust API.
        assert_eq!(
            verify_committed(digest, &produced, &set, chain),
            Ok(expected)
        );

        let mut wrong_signer = produced.clone();
        wrong_signer.commit.signatures[0]
            .signature
            .as_mut()
            .unwrap()[0] ^= 1;
        assert_eq!(
            verify_committed(digest, &wrong_signer, &set, chain),
            Err(ProofError::InvalidCommitSignature)
        );
        let mut malformed = produced.clone();
        malformed.batch_aunts.clear();
        assert_eq!(
            verify_committed(digest, &malformed, &set, chain),
            Err(ProofError::InvalidProof)
        );
        let mut malformed = produced.clone();
        malformed.signed_receipt.truncate(68);
        assert_eq!(
            committed_digest(case, &malformed),
            Err(ProofError::InvalidReceipt)
        );
        assert_eq!(
            verify_committed(digest, &malformed, &set, chain),
            Err(ProofError::InvalidReceipt)
        );
        // A failed input never poisons the next complete verification.
        let digest = committed_digest(case, &produced).unwrap();
        assert_eq!(
            verify_committed(digest, &produced, &set, chain),
            Ok(expected)
        );
    }
}

#[test]
fn proof_rs_receipted_producer_digest_crosses_public_boundary_twice() {
    for _ in 0..2 {
        let receipts: Value = serde_json::from_str(RECEIPTS).unwrap();
        let case = array(&receipts, "cases")
            .iter()
            .find(|case| string(case, "id") == "V1-layerxledger-pay-exact")
            .unwrap();
        let raw = bytes(case, "rawTransaction");
        let receipt = bytes(case, "signedReceipt");
        let view = serde_json::to_vec(&case["receiptJson"]).unwrap();
        let previous = fixed(case, "prevSeqHash");
        let keys = pins(case);
        let authenticate =
            |receipt: &[u8], view: &[u8]| verify_receipted(&raw, receipt, view, previous, &keys);
        let digest = authenticate(&receipt, &view).unwrap();
        assert_eq!(digest, fixed(case, "receiptDigest"));
        // A digest returned by the public receipt verifier is usable directly
        // by the public Merkle producer and inclusion consumer.
        let commitment = root(&[&digest]);
        assert_eq!(verify_proof(commitment, 1, 0, &digest, &[]), Ok(()));
        let mut wrong_digest = digest;
        wrong_digest[0] ^= 1;
        assert_eq!(
            verify_proof(commitment, 1, 0, &wrong_digest, &[]),
            Err(ProofError::RootMismatch)
        );
        let mut wrong_signer = keys;
        wrong_signer[0].1 = [0; 32];
        assert_eq!(
            verify_receipted(&raw, &receipt, &view, previous, &wrong_signer),
            Err(ProofError::UnpinnedReceiptSignature)
        );
        assert_eq!(
            authenticate(&receipt[..receipt.len() - 1], &view),
            Err(ProofError::InvalidReceipt)
        );
        assert_eq!(
            authenticate(&receipt, b"{"),
            Err(ProofError::ReceiptViewMismatch)
        );
        assert_eq!(authenticate(&receipt, &view), Ok(digest));
    }
}

#[test]
fn proof_rs_malformed_producer_timestamps_are_refused_without_panicking() {
    let vectors: Value = serde_json::from_str(VECTORS).unwrap();
    let case = array(&vectors, "cases")
        .iter()
        .find(|case| string(case, "id") == "committed-payExact")
        .unwrap();
    let valid = proof(case);
    let set = validators(case);
    let chain = string(&vectors["provenance"], "chainId");
    let digest = committed_digest(case, &valid).unwrap();
    for time in [
        Timestamp {
            seconds: -62_135_596_801,
            nanos: 0,
        },
        Timestamp {
            seconds: 253_402_300_800,
            nanos: 0,
        },
        Timestamp {
            seconds: valid.header.time.seconds,
            nanos: 1_000_000_000,
        },
    ] {
        let mut malformed = valid.clone();
        malformed.header.time = time;
        assert_eq!(
            verify_header(&malformed.header, &set),
            Err(ProofError::InvalidCommit)
        );
        assert_eq!(
            verify_committed(digest, &malformed, &set, chain),
            Err(ProofError::InvalidCommit)
        );
        let mut malformed = valid.clone();
        malformed.execution_header.time = time;
        assert_eq!(
            verify_header(&malformed.execution_header, &set),
            Err(ProofError::InvalidCommit)
        );
        assert_eq!(
            verify_committed(digest, &malformed, &set, chain),
            Err(ProofError::InvalidCommit)
        );
        let mut malformed = valid.commit.clone();
        malformed.signatures[0].timestamp = time;
        assert_eq!(
            verify_commit(
                chain,
                &set,
                valid.header.hash(),
                valid.header.height.try_into().unwrap(),
                &malformed
            ),
            Err(ProofError::InvalidCommit)
        );
    }
    // Protobuf permits both endpoints, including negative seconds.
    for seconds in [-62_135_596_800, 253_402_300_799] {
        let mut boundary = valid.header.clone();
        boundary.time = Timestamp {
            seconds,
            nanos: 999_999_999,
        };
        assert_eq!(verify_header(&boundary, &set), Ok(boundary.hash()));
    }
    assert_eq!(verify_header(&valid.header, &set), Ok(valid.header.hash()));
}
