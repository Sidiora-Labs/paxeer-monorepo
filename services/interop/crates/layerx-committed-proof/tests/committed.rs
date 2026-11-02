use std::collections::BTreeSet;

use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    committed::verify_committed,
    header::{BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator},
    CommittedBlock, CommittedProof, ProofError,
};
use serde_json::Value;

const VECTORS: &str = include_str!("../../../specs/conformance/committed/vector-v4-committed.json");

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

fn receipt_digest(case: &Value) -> [u8; 32] {
    let pin = &case["pinnedKey"];
    layerx_committed_proof::verify_committed(
        &bytes(case, "rawTransaction"),
        &bytes(case, "signedReceipt"),
        &serde_json::to_vec(&case["receiptJson"]).unwrap(),
        fixed(case, "prevSeqHash"),
        &[(
            fixed(pin, "keyId"),
            fixed(pin, "publicKey"),
            number(pin, "firstHeight"),
            number(pin, "lastHeight"),
        )],
        fixed(&case["receiptJson"], "blockHash"),
    )
    .unwrap_or_else(|error| {
        panic!(
            "{}: receipt authentication failed: {error}",
            string(case, "id")
        )
    })
}

#[test]
fn committed_vectors_verify_end_to_end() {
    let vectors: Value = serde_json::from_str(VECTORS).unwrap();
    assert_eq!(string(&vectors, "name"), "vector-v4-committed");
    assert_eq!(number(&vectors, "version"), 4);
    let chain_id = string(&vectors["provenance"], "chainId");
    let cases = array(&vectors, "cases");
    let mut seen = BTreeSet::new();
    let mut accepted = 0;
    let mut refused = 0;
    for case in cases {
        let id = string(case, "id");
        assert!(seen.insert(id), "duplicate case {id}");
        let proof = proof(case);
        let validators: Vec<_> = array(case, "validators")
            .iter()
            .map(|validator| Validator {
                pub_key: fixed(validator, "pub_key"),
                voting_power: signed(validator, "voting_power"),
            })
            .collect();
        let digest = receipt_digest(case);
        let actual = verify_committed(digest, &proof, &validators, chain_id);
        let expected = &case["expected"];
        match string(expected, "rung") {
            "committed" => {
                accepted += 1;
                let number = u64::from_str_radix(
                    string(&case["receiptJson"], "blockNumber")
                        .strip_prefix("0x")
                        .unwrap(),
                    16,
                )
                .unwrap();
                assert_eq!(
                    actual,
                    Ok(CommittedBlock {
                        number,
                        hash: fixed(&case["receiptJson"], "blockHash"),
                    }),
                    "{id}"
                );
                assert_eq!(number, proof.execution_header.height as u64, "{id}");
                assert_eq!(actual.unwrap().hash, proof.execution_header.hash(), "{id}");
            }
            "refused" => {
                refused += 1;
                let error = match string(expected, "error") {
                    "InsufficientVotingPower" => ProofError::InsufficientVotingPower,
                    "ValidatorsHashMismatch" => ProofError::ValidatorsHashMismatch,
                    "RootMismatch" => ProofError::RootMismatch,
                    error => panic!("unhandled vector error {error}"),
                };
                assert_eq!(actual, Err(error), "{id}");
            }
            rung => panic!("unknown vector rung {rung}"),
        }
    }
    assert_eq!((accepted, refused), (1, 3));
    assert_eq!(
        seen,
        BTreeSet::from([
            "committed-payExact",
            "commit-under-two-thirds",
            "wrong-validators-hash",
            "flipped-proof-aunt",
        ])
    );

    let positive = cases
        .iter()
        .find(|case| string(case, "id") == "committed-payExact")
        .unwrap();
    let valid = proof(positive);
    let digest = receipt_digest(positive);
    let validators: Vec<_> = array(positive, "validators")
        .iter()
        .map(|validator| Validator {
            pub_key: fixed(validator, "pub_key"),
            voting_power: signed(validator, "voting_power"),
        })
        .collect();
    let assert_refused = |proof: &CommittedProof, error| {
        assert_eq!(
            verify_committed(digest, proof, &validators, chain_id),
            Err(error)
        );
    };

    let mut changed_digest = digest;
    changed_digest[0] ^= 1;
    assert_eq!(
        verify_committed(changed_digest, &valid, &validators, chain_id),
        Err(ProofError::InvalidReceipt)
    );
    assert_eq!(
        verify_committed(digest, &valid, &validators, "another-chain"),
        Err(ProofError::InvalidCommit)
    );
    assert_eq!(
        verify_committed(digest, &valid, &validators, ""),
        Err(ProofError::InvalidCommit)
    );

    let mut changed = valid.clone();
    changed.execution_header.data_hash[0] ^= 1;
    assert_refused(&changed, ProofError::InvalidCommit);
    let mut changed = valid.clone();
    changed.execution_header.height -= 1;
    assert_refused(&changed, ProofError::InvalidCommit);
    let mut changed = valid.clone();
    changed.execution_header.chain_id.push('x');
    assert_refused(&changed, ProofError::InvalidCommit);
    let mut changed = valid.clone();
    changed.execution_header.next_validators_hash[0] ^= 1;
    assert_refused(&changed, ProofError::InvalidCommit);
    let mut changed = valid.clone();
    changed.header.time.nanos = 1_000_000_000;
    assert_refused(&changed, ProofError::InvalidCommit);
    let mut changed = valid.clone();
    changed.execution_header.time.seconds = 253_402_300_800;
    assert_refused(&changed, ProofError::InvalidCommit);
    let mut changed = valid.clone();
    changed.commit.signatures[0].signature.as_mut().unwrap()[0] ^= 1;
    assert_refused(&changed, ProofError::InvalidCommitSignature);

    let mut changed = valid.clone();
    changed.batch_transaction[0] ^= 1;
    assert_refused(&changed, ProofError::RootMismatch);
    let mut changed = valid.clone();
    changed.batch_aunts.clear();
    assert_refused(&changed, ProofError::InvalidProof);
    let mut changed = valid.clone();
    changed.batch_aunts.push([0; 32]);
    assert_refused(&changed, ProofError::InvalidProof);
    let mut changed = valid.clone();
    changed.batch_index = 1;
    assert_refused(&changed, ProofError::InvalidProof);
    let mut changed = valid.clone();
    changed.batch_total = 0;
    assert_refused(&changed, ProofError::InvalidProof);
    let mut changed = valid.clone();
    changed.batch_total = u64::MAX;
    assert_refused(&changed, ProofError::InvalidProof);
    let mut changed = valid.clone();
    changed.activity_ids[0][0] ^= 1;
    assert_refused(&changed, ProofError::ReceiptSequenceMismatch);
    let mut changed = valid.clone();
    changed.activity_ids.clear();
    assert_refused(&changed, ProofError::InvalidProof);
    let mut changed = valid.clone();
    changed.signed_receipt[10] ^= 1;
    assert_refused(&changed, ProofError::InvalidReceipt);
    let mut changed = valid.clone();
    changed.signed_receipt.truncate(68);
    assert_refused(&changed, ProofError::InvalidReceipt);
    let mut changed = valid.clone();
    let trailer = changed.signed_receipt.len() - 69;
    changed.signed_receipt[trailer] = 0;
    assert_refused(&changed, ProofError::InvalidReceipt);
}
