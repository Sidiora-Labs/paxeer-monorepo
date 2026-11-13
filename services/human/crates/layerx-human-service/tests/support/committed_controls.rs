//! Exact paxd recording, decoded without constructing or signing replacement evidence.

use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_agentd::read::{
    AbciResultProof, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator,
    CommitVote,
};
use layerx_client::rpc::{commit::decode_validators, ChainProofVerifier};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_human_service::activity::{
    CommittedReceiptEvidence, ConsensusAuthority, EvidenceBundle, ReceiptFact, ReceiptHeaderBinding,
};
use layerx_human_service::notify::ActivityEntryId;
use layerx_proof::merkle::Proof;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const COMMITTED: &str = include_str!("../fixtures/repair_22_33_50/conformance-v4-committed.json");
const RECORDED: &str = include_str!("../fixtures/repair_22_33_50/paxd-v4-committed.json");
pub fn recorded_rpc_receipt() -> Value {
    let source: Value = serde_json::from_str(RECORDED).or_fail("paxd recorder output");
    let normalized: Value = serde_json::from_str(COMMITTED).or_fail("conformance recording");
    assert_eq!(
        source["settlement"]["receipt"],
        normalized["cases"][0]["receiptJson"]
    );
    source["settlement"]["receipt"].clone()
}

pub struct RecordedCommit {
    pub verifier: ChainProofVerifier,
    pub batch: AbciResultProof,
    pub batch_transaction: Vec<u8>,
    pub height: u64,
    pub replies: Vec<(&'static str, Value, Value)>,
}

pub fn recorded_commit() -> RecordedCommit {
    let source: Value = serde_json::from_str(RECORDED).or_fail("paxd recorder output");
    let normalized: Value = serde_json::from_str(COMMITTED).or_fail("conformance recording");
    assert_eq!(source["positive"], normalized["positive"]);
    let settlement = &source["settlement"];
    let case = &normalized["cases"][0];
    let height = case["executionHeader"]["height"].as_u64().or_fail("height");
    let validators: Vec<_> = settlement["validators"]
        .as_array()
        .or_fail("recorded validators")
        .iter()
        .map(|v| json!({"address":v["address"], "pubKey":v["pub_key"], "votingPower":v["voting_power"]}))
        .collect();
    let validators = json!({"height":height.to_string(), "total":validators.len().to_string(), "validators":validators});
    let verifier = ChainProofVerifier {
        chain_id: settlement["execution_header"]["chain_id"]
            .as_str()
            .or_fail("chain")
            .to_owned(),
        validators: decode_validators(&validators).or_fail("recorded validator pins"),
    };
    let recorded_proof = &settlement["sequence_batch_proof"];
    let decode = |v: &Value| {
        STANDARD
            .decode(v.as_str().or_fail("base64 field"))
            .or_fail("recorded bytes")
    };
    // Comet's transaction proof carries SHA256(transaction) as its leaf payload,
    // unlike an ABCI result proof. Retain the recorded leaf-hash check below.
    let batch_transaction = decode(&recorded_proof["data"]);
    let mut leaf = b"LXAR\x01".to_vec();
    let total: i64 = recorded_proof["proof"]["total"]
        .as_str()
        .or_fail("total")
        .parse()
        .or_fail("i64 total");
    let index: i64 = recorded_proof["proof"]["index"]
        .as_str()
        .or_fail("index")
        .parse()
        .or_fail("i64 index");
    leaf.extend_from_slice(&total.to_be_bytes());
    leaf.extend_from_slice(&index.to_be_bytes());
    leaf.extend_from_slice(&Sha256::digest(&batch_transaction));
    let mut fields = vec![leaf];
    fields.extend(
        recorded_proof["proof"]["aunts"]
            .as_array()
            .or_fail("aunts")
            .iter()
            .map(decode),
    );
    let batch = AbciResultProof::decode(&fields).or_fail("recorded batch inclusion");
    assert_eq!(
        batch.proof.leaf_hash.as_slice(),
        decode(&recorded_proof["proof"]["leaf_hash"])
    );
    assert_eq!(batch_transaction, bytes(&case["batchProof"]["data"]));
    let signed = json!({"height":height.to_string(), "signedHeader":{
        "header":settlement["execution_header"], "commit":settlement["execution_commit"]
    }});
    let hex = |v: &Value| {
        format!(
            "0x{}",
            v.as_str().or_fail("recorded hash").to_ascii_lowercase()
        )
    };
    let block_hash = hex(&settlement["execution_commit"]["block_id"]["hash"]);
    let block = json!({
        "number":format!("{height:#x}"), "hash":block_hash,
        "parentHash":hex(&settlement["execution_header"]["last_block_id"]["hash"]),
        "timestamp":format!("{:#x}", case["executionHeader"]["time"]["seconds"].as_u64().or_fail("timestamp")),
        "transactionsRoot":hex(&settlement["execution_header"]["data_hash"]),
        "receiptsRoot":hex(&settlement["header"]["last_results_hash"]),
        "transactions":[recorded_transaction(settlement, &block_hash, height)]
    });
    RecordedCommit {
        verifier,
        batch,
        batch_transaction,
        height,
        replies: vec![
            ("px_getCommit", json!([height.to_string()]), signed),
            ("px_getValidators", json!([height.to_string()]), validators),
            (
                "eth_getBlockByNumber",
                json!([format!("{height:#x}"), true]),
                block,
            ),
        ],
    }
}

// Decode the retained legacy RLP fields into the node's full-transaction JSON
// projection. The client re-encodes these same signed bytes when reading the block.
fn recorded_transaction(settlement: &Value, block_hash: &str, height: u64) -> Value {
    fn item<'a>(input: &mut &'a [u8]) -> &'a [u8] {
        let prefix = input[0];
        if prefix < 0x80 {
            let (value, rest) = input.split_at(1);
            *input = rest;
            return value;
        }
        let short = if prefix < 0xc0 {
            prefix - 0x80
        } else {
            prefix - 0xc0
        };
        let (offset, len) = if short < 56 {
            (1, usize::from(short))
        } else {
            let width = usize::from(short - 55);
            let length = input[1..=width]
                .iter()
                .fold(0_usize, |n, b| (n << 8) | usize::from(*b));
            (1 + width, length)
        };
        let value = &input[offset..offset + len];
        *input = &input[offset + len..];
        value
    }
    let raw = bytes(&settlement["raw_transaction"]);
    let mut rest = raw.as_slice();
    let mut fields = item(&mut rest);
    assert!(rest.is_empty());
    let values: Vec<_> = (0..9).map(|_| item(&mut fields)).collect();
    assert!(fields.is_empty());
    let hex = |data: &[u8]| format!("0x{}", layerx_human_service::hex::lower(data));
    let quantity = |data: &[u8]| {
        let encoded = hex(data);
        let digits = encoded[2..].trim_start_matches('0');
        format!("0x{}", if digits.is_empty() { "0" } else { digits })
    };
    let v = values[6]
        .iter()
        .fold(0_u64, |n, b| (n << 8) | u64::from(*b));
    assert!(v >= 35);
    let receipt = &settlement["receipt"];
    assert_eq!(
        hex(&layerx_crypto::evm_transaction::transaction_hash(&raw)),
        receipt["txHash"]
    );
    json!({
        "blockHash":block_hash, "blockNumber":format!("{height:#x}"),
        "from":receipt["from"], "hash":receipt["txHash"],
        "transactionIndex":"0x0", "type":"0x0", "chainId":format!("{:#x}", (v - 35) / 2),
        "nonce":quantity(values[0]), "gasPrice":quantity(values[1]), "gas":quantity(values[2]),
        "to":hex(values[3]), "value":quantity(values[4]), "input":hex(values[5]),
        "v":quantity(values[6]), "r":quantity(values[7]), "s":quantity(values[8])
    })
}

fn bytes(value: &Value) -> Vec<u8> {
    let text = value
        .as_str()
        .or_fail("hex string")
        .strip_prefix("0x")
        .or_fail("0x prefix");
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).or_fail("hex byte"))
        .collect()
}
fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value).try_into().or_fail("fixed-width field")
}
fn time(value: &Value) -> CommitTimestamp {
    CommitTimestamp {
        seconds: value["seconds"].as_i64().or_fail("seconds"),
        nanos: value["nanos"]
            .as_u64()
            .or_fail("nanos")
            .try_into()
            .or_fail("u32 nanos"),
    }
}
fn block_id(value: &Value) -> BlockId {
    BlockId {
        hash: fixed(&value["hash"]),
        part_set_total: value["parts"]["total"]
            .as_u64()
            .or_fail("parts")
            .try_into()
            .or_fail("u32 parts"),
        part_set_hash: fixed(&value["parts"]["hash"]),
    }
}
fn header(v: &Value) -> CheckpointHeader {
    let optional = |key: &str| {
        if bytes(&v[key]).is_empty() {
            None
        } else {
            Some(fixed(&v[key]))
        }
    };
    CheckpointHeader {
        version_block: v["version"]["block"].as_u64().or_fail("version"),
        version_app: v["version"]["app"].as_u64().or_fail("app version"),
        chain_id: v["chain_id"].as_str().or_fail("chain id").to_owned(),
        height: v["height"].as_u64().or_fail("height"),
        time: time(&v["time"]),
        last_block_id: Some(block_id(&v["last_block_id"])),
        last_commit_hash: optional("last_commit_hash"),
        data_hash: optional("data_hash"),
        validators_hash: fixed(&v["validators_hash"]),
        next_validators_hash: fixed(&v["next_validators_hash"]),
        consensus_hash: fixed(&v["consensus_hash"]),
        app_hash: fixed(&v["app_hash"]),
        last_results_hash: optional("last_results_hash"),
        evidence_hash: optional("evidence_hash"),
        proposer_address: fixed(&v["proposer_address"]),
    }
}

fn committed_votes(commit: &Value, validators: &[CommitValidator]) -> Vec<CommitVote> {
    commit["signatures"]
        .as_array()
        .or_fail("votes")
        .iter()
        .map(|vote| {
            assert_eq!(vote["block_id_flag"], 2);
            let address = fixed(&vote["validator_address"]);
            CommitVote {
                validator_address: address,
                voting_power: validators
                    .iter()
                    .find(|v| v.address == address)
                    .or_fail("pinned validator")
                    .voting_power,
                timestamp: time(&vote["timestamp"]),
                signature: fixed(&vote["signature"]),
            }
        })
        .collect()
}

fn recorded_header_binding(binding: &Value) -> ReceiptHeaderBinding {
    ReceiptHeaderBinding {
        canonical_header_bytes: bytes(&binding["header"]),
        header_signature: fixed(&binding["header_signature"]),
        receipt_proof: Proof::new(
            binding["proof"]["LeafIndex"]
                .as_u64()
                .or_fail("leaf index")
                .try_into()
                .or_fail("index u32"),
            binding["proof"]["LeafCount"]
                .as_u64()
                .or_fail("count")
                .try_into()
                .or_fail("count u32"),
            binding["proof"]["Siblings"]
                .as_array()
                .or_fail("siblings")
                .iter()
                .map(|s| serde_json::from_value(s.clone()).or_fail("sibling digest"))
                .collect(),
        )
        .or_fail("real receipt Merkle proof"),
    }
}

pub fn recorded_receipt() -> (
    ActivityEntryId,
    ReceiptFact,
    Vec<ReceiptKeyEntry>,
    Vec<ConsensusAuthority>,
) {
    let v: Value = serde_json::from_str(COMMITTED).or_fail("Go node conformance recording");
    assert_eq!(
        v["provenance"]["sourceSha256"],
        format!("{:x}", Sha256::digest(RECORDED.as_bytes()))
    );
    let c = &v["cases"][0];
    assert_eq!(c["id"], "committed-payExact");
    assert_eq!(c["receiptJson"]["rung"], "committed");
    let raw: Value = serde_json::from_str(RECORDED).or_fail("original Go recorder output");
    // Pin the recording's provenance and retain the actual tx-zero bytes.
    assert_eq!(c["signedReceipt"], raw["positive"]["receipt"]);
    assert_eq!(
        bytes(&c["batchProof"]["data"]),
        STANDARD
            .decode(
                raw["settlement"]["sequence_batch_proof"]["data"]
                    .as_str()
                    .or_fail("batch base64")
            )
            .or_fail("recorded batch")
    );
    let key = &c["pinnedKey"];
    let keys = vec![ReceiptKeyEntry {
        key_id: fixed(&key["keyId"]),
        public_key: fixed(&key["publicKey"]),
        first_height: key["firstHeight"].as_u64().or_fail("first"),
        last_height: key["lastHeight"].as_u64().or_fail("last"),
    }];
    let validators: Vec<_> = c["validators"]
        .as_array()
        .or_fail("validators")
        .iter()
        .map(|v| {
            let public_key = fixed(&v["pub_key"]);
            CommitValidator {
                address: CommitValidator::address_for_key(&public_key),
                public_key,
                voting_power: v["voting_power"].as_u64().or_fail("power"),
            }
        })
        .collect();
    let h = header(&c["header"]);
    let commit = &c["commit"];
    let votes = committed_votes(commit, &validators);
    let id = ActivityEntryId::new("act_repair2233350").or_fail("entry");
    let mut fact = EvidenceBundle::receipt_fact(&id, bytes(&c["signedReceipt"]), Rung::Committed)
        .or_fail("real receipt codec");
    fact.committed = Some(CommittedReceiptEvidence {
        header: h.clone(),
        execution_header: header(&c["executionHeader"]),
        commit: BlockCommit {
            height: commit["height"].as_u64().or_fail("commit height"),
            round: commit["round"]
                .as_u64()
                .or_fail("round")
                .try_into()
                .or_fail("round u32"),
            block_id: block_id(&commit["block_id"]),
            votes,
        },
        batch_transaction: bytes(&c["batchProof"]["data"]),
        batch_total: c["batchProof"]["total"].as_u64().or_fail("total"),
        batch_aunts: c["batchProof"]["aunts"]
            .as_array()
            .or_fail("aunts")
            .iter()
            .map(fixed)
            .collect(),
        activity_ids: vec![fact.tx_hash],
    });
    fact.header_binding = Some(recorded_header_binding(&v["positive"]));
    (
        id,
        fact,
        keys,
        vec![ConsensusAuthority {
            chain_id: h.chain_id,
            first_height: h.height,
            last_height: h.height,
            validators,
        }],
    )
}

use crate::require_value::RequireValue as _;
