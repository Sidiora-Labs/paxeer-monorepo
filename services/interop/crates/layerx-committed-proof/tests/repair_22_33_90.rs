//! Task 22.33.90, audit 22.30.110, finding proof_namespace_registry.
//! Proving tests: proof_namespace_registry_consumes_both_public_verifiers and
//! proof_namespace_registry_preserves_consensus_refusals.
//! Wiring request: domain 11 (fm-interop), spec/layerx-merge/interfaces.kvx,
//! interface 11.rs-committed-proof-verify-committed-j47: qualify the four-argument
//! path as layerx_committed_proof::committed::verify_committed and document the
//! separate six-argument layerx_committed_proof::verify_committed receipt-view helper.
//! Baseline: no earlier gate evidence exists; no unchanged-tree verify was run
//! because the task's explicit gate order requires complete implementation first.
//! Qualification revision, command, exit and private log are recorded by hf-done.

use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    header::{BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator},
    CommittedBlock, CommittedProof, ProofError,
};
use serde_json::Value;
use sha2::{Digest, Sha256};

const VECTORS: &str = include_str!("../../../specs/conformance/committed/vector-v4-committed.json");
const RECORDING: &[u8] =
    include_bytes!("../../../../../daemon/committed-fixture/vectors/vector-v4-committed.json");

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

fn receipt_view_digest(case: &Value, view: &Value) -> Result<[u8; 32], ProofError> {
    let pin = &case["pinnedKey"];
    layerx_committed_proof::verify_committed(
        &bytes(case, "rawTransaction"),
        &bytes(case, "signedReceipt"),
        &serde_json::to_vec(view).unwrap(),
        fixed(case, "prevSeqHash"),
        &[(
            fixed(pin, "keyId"),
            fixed(pin, "publicKey"),
            number(pin, "firstHeight"),
            number(pin, "lastHeight"),
        )],
        fixed(&case["receiptJson"], "blockHash"),
    )
}

#[test]
fn proof_namespace_registry_consumes_both_public_verifiers() {
    let vectors: Value = serde_json::from_str(VECTORS).unwrap();
    assert_eq!(number(&vectors, "version"), 4);
    let source_digest = hex_bytes(&format!(
        "0x{}",
        string(&vectors["provenance"], "sourceSha256")
    ));
    assert_eq!(source_digest, Sha256::digest(RECORDING).as_slice());
    let case = array(&vectors, "cases")
        .iter()
        .find(|case| string(case, "id") == "committed-payExact")
        .unwrap();
    let recording: Value = serde_json::from_slice(RECORDING).unwrap();
    assert_eq!(case["signedReceipt"], recording["positive"]["receipt"]);
    let proof = proof(case);
    let set = validators(case);
    let chain_id = string(&vectors["provenance"], "chainId");

    // A downstream crate consumes both public paths, with their distinct outputs.
    let digest: [u8; 32] = receipt_view_digest(case, &case["receiptJson"]).unwrap();
    let signed = bytes(case, "signedReceipt");
    let mut expected_digest = Sha256::new();
    expected_digest.update(b"LXP/v1/receipt\0");
    expected_digest.update(&signed[..signed.len() - 69]);
    expected_digest.update([0]);
    assert_eq!(digest, <[u8; 32]>::from(expected_digest.finalize()));
    let block: CommittedBlock =
        layerx_committed_proof::committed::verify_committed(digest, &proof, &set, chain_id)
            .unwrap();
    assert_eq!(
        block,
        CommittedBlock {
            number: proof.execution_header.height.try_into().unwrap(),
            hash: fixed(&case["receiptJson"], "blockHash"),
        }
    );
    assert_eq!(block.hash, proof.execution_header.hash());

    let mut wrong_view = case["receiptJson"].clone();
    wrong_view["blockNumber"] = "0x0".into();
    assert_eq!(
        receipt_view_digest(case, &wrong_view),
        Err(ProofError::ReceiptViewMismatch)
    );
    let mut wrong_digest = digest;
    wrong_digest[0] ^= 1;
    assert_eq!(
        layerx_committed_proof::committed::verify_committed(wrong_digest, &proof, &set, chain_id),
        Err(ProofError::InvalidReceipt),
    );
}

#[test]
fn proof_namespace_registry_preserves_consensus_refusals() {
    let vectors: Value = serde_json::from_str(VECTORS).unwrap();
    let chain_id = string(&vectors["provenance"], "chainId");
    for (id, expected) in [
        (
            "commit-under-two-thirds",
            ProofError::InsufficientVotingPower,
        ),
        ("wrong-validators-hash", ProofError::ValidatorsHashMismatch),
        ("flipped-proof-aunt", ProofError::RootMismatch),
    ] {
        let case = array(&vectors, "cases")
            .iter()
            .find(|case| string(case, "id") == id)
            .unwrap();
        let digest = receipt_view_digest(case, &case["receiptJson"]).unwrap();
        assert_eq!(
            layerx_committed_proof::committed::verify_committed(
                digest,
                &proof(case),
                &validators(case),
                chain_id,
            ),
            Err(expected),
            "{id}",
        );
    }
}
