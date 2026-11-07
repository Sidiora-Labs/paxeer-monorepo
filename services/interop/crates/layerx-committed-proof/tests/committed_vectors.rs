use std::collections::BTreeSet;

use ed25519_dalek::{Signature, VerifyingKey};
use layerx_committed_proof::{
    commit::{verify_commit, BlockIdFlag, Commit, CommitSig},
    header::{BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator},
    merkle::verify_proof,
    verify_committed, verify_header, ProofError,
};
use serde_json::Value;
use sha2::{Digest, Sha256};

const VECTORS: &str = include_str!("../../../specs/conformance/committed/vector-v4-committed.json");
const RECORDING: &[u8] =
    include_bytes!("../../../../../daemon/committed-fixture/vectors/vector-v4-committed.json");
const BATCH_DOMAIN: &[u8] = b"PAXEER/v1/sequence-batch\0";

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

fn recorded_base64(encoded: &str) -> Vec<u8> {
    const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    assert_eq!(encoded.len() % 4, 0);
    let mut decoded = Vec::new();
    let chunks = encoded.as_bytes().chunks_exact(4);
    let count = chunks.len();
    for (index, chunk) in chunks.enumerate() {
        let padding = chunk.iter().rev().take_while(|byte| **byte == b'=').count();
        assert!(padding <= 2);
        assert!(padding == 0 || index + 1 == count);
        let mut word = 0_u32;
        for (position, byte) in chunk.iter().enumerate() {
            let digit = if position >= 4 - padding {
                assert_eq!(*byte, b'=');
                0
            } else {
                ALPHABET
                    .iter()
                    .position(|candidate| candidate == byte)
                    .unwrap_or_else(|| panic!("invalid recorded base64")) as u32
            };
            word = (word << 6) | digit;
        }
        assert_eq!(word & ((1 << (padding * 8)) - 1), 0);
        decoded.extend_from_slice(&word.to_be_bytes()[1..4 - padding]);
    }
    decoded
}

fn recorded_timestamp(encoded: &str) -> Timestamp {
    assert!(encoded.is_ascii());
    let utc = encoded
        .strip_suffix('Z')
        .unwrap_or_else(|| panic!("non-UTC recording"));
    let (date, time) = utc
        .split_once('T')
        .unwrap_or_else(|| panic!("invalid recorded time"));
    let date: Vec<u32> = date.split('-').map(|part| part.parse().unwrap()).collect();
    assert_eq!(date.len(), 3);
    let (year, month, day) = (date[0], date[1], date[2]);
    assert!((1..=9999).contains(&year));
    assert!((1..=12).contains(&month));
    let leap = year % 4 == 0 && (year % 100 != 0 || year % 400 == 0);
    let months = [
        31,
        if leap { 29 } else { 28 },
        31,
        30,
        31,
        30,
        31,
        31,
        30,
        31,
        30,
        31,
    ];
    assert!((1..=months[month as usize - 1]).contains(&day));
    let (whole, fraction) = time.split_once('.').unwrap_or((time, ""));
    assert!(fraction.len() <= 9 && fraction.bytes().all(|byte| byte.is_ascii_digit()));
    let time: Vec<u32> = whole.split(':').map(|part| part.parse().unwrap()).collect();
    assert_eq!(time.len(), 3);
    assert!(time[0] < 24 && time[1] < 60 && time[2] < 60);
    let days_before_year = |year: u32| {
        let previous = i64::from(year - 1);
        365 * previous + previous / 4 - previous / 100 + previous / 400
    };
    let days = days_before_year(year) - days_before_year(1970)
        + i64::from(months[..month as usize - 1].iter().sum::<u32>())
        + i64::from(day - 1);
    Timestamp {
        seconds: days * 86_400 + i64::from(time[0] * 3600 + time[1] * 60 + time[2]),
        nanos: if fraction.is_empty() {
            0
        } else {
            fraction.parse::<u32>().unwrap() * 10_u32.pow(9 - fraction.len() as u32)
        },
    }
}

fn recorded_hex(value: &Value, field: &str) -> Vec<u8> {
    hex_bytes(&format!("0x{}", string(value, field)))
}

fn recorded_block_id(value: &Value) -> BlockId {
    BlockId {
        hash: recorded_hex(value, "hash"),
        part_set_header: PartSetHeader {
            total: number(&value["parts"], "total").try_into().unwrap(),
            hash: recorded_hex(&value["parts"], "hash"),
        },
    }
}

fn recorded_commit(value: &Value) -> Commit {
    Commit {
        height: string(value, "height").parse().unwrap(),
        round: signed(value, "round").try_into().unwrap(),
        block_id: recorded_block_id(&value["block_id"]),
        signatures: array(value, "signatures")
            .iter()
            .map(|vote| CommitSig {
                block_id_flag: match number(vote, "block_id_flag") {
                    1 => BlockIdFlag::Absent,
                    2 => BlockIdFlag::Commit,
                    3 => BlockIdFlag::Nil,
                    flag => panic!("unknown recorded block id flag {flag}"),
                },
                validator_address: recorded_hex(vote, "validator_address"),
                timestamp: recorded_timestamp(string(vote, "timestamp")),
                signature: if vote["signature"].is_null() {
                    None
                } else {
                    Some(
                        recorded_base64(string(vote, "signature"))
                            .try_into()
                            .unwrap(),
                    )
                },
            })
            .collect(),
    }
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

fn inclusion(root: &[u8], proof: &Value) -> Result<(), ProofError> {
    assert_eq!(root, bytes(proof, "rootHash"));
    let data = bytes(proof, "data");
    let leaf = match string(proof, "leafEncoding") {
        "sha256-tx" => Sha256::digest(data).to_vec(),
        "exec-tx-result" => data,
        encoding => panic!("unknown proof leaf encoding {encoding}"),
    };
    let aunts: Vec<[u8; 32]> = array(proof, "aunts")
        .iter()
        .map(|aunt| {
            hex_bytes(aunt.as_str().unwrap_or_else(|| panic!("invalid aunt")))
                .try_into()
                .unwrap_or_else(|_| panic!("incorrect aunt length"))
        })
        .collect();
    verify_proof(
        root.try_into()
            .unwrap_or_else(|_| panic!("incorrect root length")),
        number(proof, "total"),
        number(proof, "index"),
        &leaf,
        &aunts,
    )
}

fn quantity(value: &Value, field: &str) -> u64 {
    u64::from_str_radix(
        string(value, field)
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("missing quantity prefix {field}")),
        16,
    )
    .unwrap_or_else(|error| panic!("invalid quantity {field}: {error}"))
}

fn receipt_and_batch(case: &Value, execution: &Header, block_hash: [u8; 32]) {
    let receipt = &case["receiptJson"];
    assert_eq!(string(receipt, "rung"), "committed");
    assert_eq!(bytes(receipt, "receipt"), bytes(case, "signedReceipt"));
    assert_eq!(quantity(receipt, "blockNumber"), execution.height as u64);
    assert_eq!(fixed::<32>(receipt, "blockHash"), execution.hash());
    assert_eq!(execution.hash(), block_hash);
    assert_eq!(quantity(receipt, "height"), execution.height as u64);

    let pin = &case["pinnedKey"];
    assert_eq!(fixed::<32>(pin, "publicKey"), fixed(receipt, "signingKey"));
    assert_eq!(fixed::<32>(pin, "keyId"), fixed(receipt, "keyId"));
    let signed_receipt = bytes(case, "signedReceipt");
    let digest = verify_committed(
        &bytes(case, "rawTransaction"),
        &signed_receipt,
        &serde_json::to_vec(receipt)
            .unwrap_or_else(|error| panic!("invalid receipt JSON: {error}")),
        fixed(case, "prevSeqHash"),
        &[(
            fixed(pin, "keyId"),
            fixed(pin, "publicKey"),
            number(pin, "firstHeight"),
            number(pin, "lastHeight"),
        )],
        block_hash,
    )
    .unwrap_or_else(|error| {
        panic!(
            "{}: verbatim recorder committed receipt refused: {error:?}",
            string(case, "id")
        )
    });
    let receipt_key = VerifyingKey::from_bytes(&fixed(pin, "publicKey"))
        .unwrap_or_else(|error| panic!("invalid receipt key: {error}"));
    let receipt_signature = Signature::from_slice(&signed_receipt[signed_receipt.len() - 64..])
        .unwrap_or_else(|error| panic!("invalid recorded receipt signature: {error}"));
    assert!(
        receipt_key
            .verify_strict(&digest, &receipt_signature)
            .is_ok(),
        "{}: returned digest must authenticate the recorded signature",
        string(case, "id"),
    );

    let batch_proof = &case["batchProof"];
    let batch = bytes(batch_proof, "data");
    assert_eq!(batch.len(), BATCH_DOMAIN.len() + 216);
    assert_eq!(&batch[..BATCH_DOMAIN.len()], BATCH_DOMAIN);
    let body = &batch[BATCH_DOMAIN.len()..];
    let integer = |start| {
        u64::from_be_bytes(
            body[start..start + 8]
                .try_into()
                .unwrap_or_else(|_| panic!("invalid batch integer")),
        )
    };
    assert_eq!(integer(0), execution.height as u64);
    // This recording contains one payment in its sequence batch.
    assert_eq!(integer(8), quantity(receipt, "seq"));
    assert_eq!(integer(16), quantity(receipt, "seq"));
    assert_eq!(&body[24..56], bytes(case, "prevSeqHash"));
    assert_eq!(
        &body[88..120],
        &Sha256::digest(bytes(receipt, "txHash"))[..]
    );
    assert_eq!(&body[120..152], bytes(pin, "keyId"));
    let mut preimage = BATCH_DOMAIN.to_vec();
    preimage.extend_from_slice(execution.chain_id.as_bytes());
    preimage.extend_from_slice(&body[..120]);
    let key = VerifyingKey::from_bytes(&fixed(pin, "publicKey"))
        .unwrap_or_else(|error| panic!("invalid batch key: {error}"));
    let signature = Signature::from_slice(&body[152..])
        .unwrap_or_else(|error| panic!("invalid batch signature: {error}"));
    assert!(key
        .verify_strict(&Sha256::digest(preimage), &signature)
        .is_ok());
    assert_eq!(number(batch_proof, "index"), 0);
    assert_eq!(number(batch_proof, "total"), 2);
    let transaction = &case["transactionProof"];
    let result = &case["execTxResultProof"];
    assert_eq!(number(transaction, "index"), quantity(receipt, "index"));
    assert_eq!(number(transaction, "index"), 1);
    assert_eq!(number(result, "index"), number(transaction, "index"));
    assert_eq!(number(transaction, "total"), number(batch_proof, "total"));
    assert_eq!(number(result, "total"), number(transaction, "total"));
    assert_eq!(inclusion(&execution.data_hash, transaction), Ok(()));
}

#[test]
fn committed_vector_commits_verify() {
    let vectors: Value = serde_json::from_str(VECTORS)
        .unwrap_or_else(|error| panic!("invalid committed vectors: {error}"));
    assert_eq!(string(&vectors, "name"), "vector-v4-committed");
    assert_eq!(number(&vectors, "version"), 4);
    let provenance = &vectors["provenance"];
    assert_eq!(
        hex_bytes(&format!("0x{}", string(provenance, "sourceSha256"))),
        &Sha256::digest(RECORDING)[..]
    );
    let cases = array(&vectors, "cases");
    assert_eq!(cases.len(), 4);
    let positive = &cases[0];
    assert_eq!(string(positive, "id"), "committed-payExact");
    let recording: Value = serde_json::from_slice(RECORDING)
        .unwrap_or_else(|error| panic!("invalid source recording: {error}"));
    let settlement = &recording["settlement"];
    assert_eq!(vectors["authorization"], recording["authorization"]);
    assert_eq!(vectors["positive"], recording["positive"]);
    assert_eq!(vectors["settlement"], *settlement);
    assert_eq!(positive["receiptHeader"], recording["positive"]["header"]);
    assert_eq!(
        positive["receiptHeaderSignature"],
        recording["positive"]["header_signature"]
    );
    assert_eq!(positive["receiptProof"], recording["positive"]["proof"]);
    assert_eq!(
        commit(&positive["commit"]),
        recorded_commit(&settlement["commit"])
    );
    assert_eq!(
        commit(&cases[1]["commit"]),
        recorded_commit(&settlement["commit_at_most_two_thirds"])
    );
    assert_eq!(
        array(positive, "validators").len(),
        array(settlement, "validators").len()
    );
    for (validator, recorded) in array(positive, "validators")
        .iter()
        .zip(array(settlement, "validators"))
    {
        assert_eq!(
            bytes(validator, "pub_key"),
            recorded_base64(string(&recorded["pub_key"], "value"))
        );
        assert_eq!(
            signed(validator, "voting_power").to_string(),
            string(recorded, "voting_power")
        );
    }
    for (field, source) in [
        ("batchProof", "sequence_batch_proof"),
        ("transactionProof", "transaction_proof"),
    ] {
        let proof = &positive[field];
        let recorded = &settlement[source];
        assert_eq!(
            bytes(proof, "rootHash"),
            recorded_hex(recorded, "root_hash")
        );
        assert_eq!(
            bytes(proof, "data"),
            recorded_base64(string(recorded, "data"))
        );
        for integer in ["total", "index"] {
            assert_eq!(
                number(proof, integer).to_string(),
                string(&recorded["proof"], integer)
            );
        }
        assert_eq!(
            array(proof, "aunts").len(),
            array(&recorded["proof"], "aunts").len()
        );
        for (aunt, recorded) in array(proof, "aunts")
            .iter()
            .zip(array(&recorded["proof"], "aunts"))
        {
            assert_eq!(
                hex_bytes(aunt.as_str().unwrap()),
                recorded_base64(recorded.as_str().unwrap())
            );
        }
    }
    assert_eq!(positive["receiptJson"], settlement["receipt"]);
    assert_eq!(positive["rawTransaction"], settlement["raw_transaction"]);
    assert_eq!(positive["signedReceipt"], recording["positive"]["receipt"]);
    assert_eq!(
        positive["execTxResultProof"]["data"],
        settlement["exec_tx_result_bytes"]
    );
    for (field, source, height_field) in [
        ("executionHeader", "execution_header", "executionHeight"),
        ("header", "header", "commitHeight"),
    ] {
        assert_eq!(
            number(provenance, height_field),
            number(&positive[field], "height")
        );
        assert_eq!(
            number(provenance, height_field).to_string(),
            string(&settlement[source], "height")
        );
        assert_eq!(
            string(&positive[field], "chain_id"),
            string(&settlement[source], "chain_id")
        );
        assert_eq!(
            timestamp(&positive[field]["time"]),
            recorded_timestamp(string(&settlement[source], "time"))
        );
        assert_eq!(
            block_id(&positive[field]["last_block_id"]),
            recorded_block_id(&settlement[source]["last_block_id"])
        );
        for version in ["block", "app"] {
            assert_eq!(
                number(&positive[field]["version"], version).to_string(),
                string(&settlement[source]["version"], version)
            );
        }
        for hash in [
            "last_commit_hash",
            "data_hash",
            "validators_hash",
            "next_validators_hash",
            "consensus_hash",
            "app_hash",
            "last_results_hash",
            "evidence_hash",
            "proposer_address",
        ] {
            assert_eq!(
                bytes(&positive[field], hash),
                hex_bytes(&format!("0x{}", string(&settlement[source], hash))),
                "{field}.{hash}: recorded header evidence"
            );
        }
    }
    let mut seen = BTreeSet::new();
    for case in cases {
        let id = string(case, "id");
        assert!(seen.insert(id), "duplicate case {id}");
        let mut mutation = positive.clone();
        mutation["id"] = case["id"].clone();
        mutation["expected"] = case["expected"].clone();
        match id {
            "committed-payExact" => {}
            "commit-under-two-thirds" => {
                let slots = mutation["commit"]["signatures"]
                    .as_array_mut()
                    .unwrap_or_else(|| panic!("missing commit signatures"));
                for slot in slots {
                    *slot = serde_json::json!({
                        "block_id_flag": 1,
                        "validator_address": "0x",
                        "timestamp": {"seconds": -62_135_596_800_i64, "nanos": 0},
                        "signature": null,
                    });
                }
            }
            "wrong-validators-hash" => {
                mutation["validators"][0]["voting_power"] =
                    (signed(&positive["validators"][0], "voting_power") + 1).into();
            }
            "flipped-proof-aunt" => {
                let mut aunt = hex_bytes(
                    positive["batchProof"]["aunts"][0]
                        .as_str()
                        .unwrap_or_else(|| panic!("missing batch proof aunt")),
                );
                aunt[0] ^= 1;
                let hex: String = aunt.iter().map(|byte| format!("{byte:02x}")).collect();
                mutation["batchProof"]["aunts"][0] = format!("0x{hex}").into();
            }
            _ => panic!("unexpected committed vector {id}"),
        }
        assert_eq!(&mutation, case, "{id}: only the named evidence may change");
        let header = header(&case["header"]);
        let execution = self::header(&case["executionHeader"]);
        let set: Vec<Validator> = array(case, "validators")
            .iter()
            .map(|validator| Validator {
                pub_key: fixed(validator, "pub_key"),
                voting_power: signed(validator, "voting_power"),
            })
            .collect();
        let commit = commit(&case["commit"]);
        assert_eq!(header.chain_id, string(provenance, "chainId"));
        assert_eq!(header.chain_id, execution.chain_id);
        assert_eq!(header.height, execution.height + 1);
        assert_eq!(header.last_block_id.hash, execution.hash());
        assert_eq!(
            verify_header(&execution, &set).is_ok(),
            id != "wrong-validators-hash"
        );
        assert_eq!(
            inclusion(&header.last_results_hash, &case["execTxResultProof"]),
            Ok(()),
            "{id}"
        );

        let header_result = verify_header(&header, &set);
        let commit_result = verify_commit(
            &header.chain_id,
            &set,
            header.hash(),
            header
                .height
                .try_into()
                .unwrap_or_else(|_| panic!("invalid height")),
            &commit,
        );
        let proof_result = inclusion(&execution.data_hash, &case["batchProof"]);
        let expected = &case["expected"];
        let expected_error = match id {
            "committed-payExact" => {
                assert_eq!(commit_result, Ok(number(expected, "power")), "{id}");
                None
            }
            "commit-under-two-thirds" => {
                assert_eq!(
                    commit_result,
                    Err(ProofError::InsufficientVotingPower),
                    "{id}"
                );
                assert_eq!(string(expected, "commitError"), "InsufficientVotingPower");
                Some(ProofError::InsufficientVotingPower)
            }
            "wrong-validators-hash" => {
                assert_eq!(
                    header_result,
                    Err(ProofError::ValidatorsHashMismatch),
                    "{id}"
                );
                Some(ProofError::ValidatorsHashMismatch)
            }
            "flipped-proof-aunt" => {
                assert_eq!(proof_result, Err(ProofError::RootMismatch), "{id}");
                Some(ProofError::RootMismatch)
            }
            _ => panic!("unexpected committed vector {id}"),
        };
        if id != "commit-under-two-thirds" {
            assert_eq!(commit_result, Ok(number(expected, "commitPower")), "{id}");
        }
        if id != "wrong-validators-hash" {
            assert_eq!(header_result, Ok(header.hash()), "{id}");
        }
        if id != "flipped-proof-aunt" {
            assert_eq!(proof_result, Ok(()), "{id}");
        }
        let verified = header_result
            .and_then(|_| commit_result)
            .and_then(|power| proof_result.map(|()| power));
        match expected_error {
            None => {
                assert_eq!(string(expected, "rung"), "committed");
                assert_eq!(verified, Ok(number(expected, "power")), "{id}");
            }
            Some(error) => {
                assert_eq!(string(expected, "rung"), "refused");
                assert_eq!(string(expected, "error"), format!("{error:?}"));
                assert_eq!(verified, Err(error), "{id}");
            }
        }
    }
    assert_eq!(
        seen,
        BTreeSet::from([
            "committed-payExact",
            "commit-under-two-thirds",
            "wrong-validators-hash",
            "flipped-proof-aunt"
        ])
    );
    let set: Vec<Validator> = array(positive, "validators")
        .iter()
        .map(|validator| Validator {
            pub_key: fixed(validator, "pub_key"),
            voting_power: signed(validator, "voting_power"),
        })
        .collect();
    let block_hash = verify_header(&header(&positive["executionHeader"]), &set)
        .unwrap_or_else(|error| panic!("recorded execution header refused: {error:?}"));
    for case in cases {
        receipt_and_batch(case, &header(&case["executionHeader"]), block_hash);
    }
}
