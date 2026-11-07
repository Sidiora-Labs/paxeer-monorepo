use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_agentd::read::{
    BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator, CommitVote,
};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_human_service::activity::{
    CommittedReceiptEvidence, ConsensusAuthority, EvidenceBundle, ReceiptFact, ReceiptHeaderBinding,
};
use layerx_human_service::notify::ActivityEntryId;
use layerx_proof::merkle::Proof;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const COMMITTED: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../interop/specs/conformance/committed/vector-v4-committed.json"
));
pub const RECORDED: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../daemon/committed-fixture/vectors/vector-v4-committed.json"
));
pub fn bytes(value: &Value) -> Vec<u8> {
    let text = value
        .as_str()
        .expect("hex string")
        .strip_prefix("0x")
        .expect("0x prefix");
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).expect("hex byte"))
        .collect()
}
pub fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value).try_into().expect("fixed-width field")
}
fn time(value: &Value) -> CommitTimestamp {
    CommitTimestamp {
        seconds: value["seconds"].as_i64().expect("seconds"),
        nanos: value["nanos"]
            .as_u64()
            .expect("nanos")
            .try_into()
            .expect("u32 nanos"),
    }
}
fn block_id(value: &Value) -> BlockId {
    BlockId {
        hash: fixed(&value["hash"]),
        part_set_total: value["parts"]["total"]
            .as_u64()
            .expect("parts")
            .try_into()
            .expect("u32 parts"),
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
        version_block: v["version"]["block"].as_u64().expect("version"),
        version_app: v["version"]["app"].as_u64().expect("app version"),
        chain_id: v["chain_id"].as_str().expect("chain id").to_owned(),
        height: v["height"].as_u64().expect("height"),
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

pub fn recorded_receipt() -> (
    ActivityEntryId,
    ReceiptFact,
    Vec<ReceiptKeyEntry>,
    Vec<ConsensusAuthority>,
) {
    let v: Value = serde_json::from_str(COMMITTED).expect("Go node conformance recording");
    let c = &v["cases"][0];
    let raw: Value = serde_json::from_str(RECORDED).expect("original Go recorder output");
    assert_eq!(
        v["provenance"]["sourceSha256"],
        hex(&Sha256::digest(RECORDED.as_bytes()))
    );
    // Pin the recording's provenance and retain the actual tx-zero bytes.
    assert_eq!(c["signedReceipt"], raw["positive"]["receipt"]);
    assert_eq!(
        bytes(&c["batchProof"]["data"]),
        STANDARD
            .decode(
                raw["settlement"]["sequence_batch_proof"]["data"]
                    .as_str()
                    .expect("batch base64")
            )
            .expect("recorded batch")
    );
    let key = &c["pinnedKey"];
    let keys = vec![ReceiptKeyEntry {
        key_id: fixed(&key["keyId"]),
        public_key: fixed(&key["publicKey"]),
        first_height: key["firstHeight"].as_u64().expect("first"),
        last_height: key["lastHeight"].as_u64().expect("last"),
    }];
    let validators: Vec<_> = c["validators"]
        .as_array()
        .expect("validators")
        .iter()
        .map(|v| {
            let public_key = fixed(&v["pub_key"]);
            CommitValidator {
                address: CommitValidator::address_for_key(&public_key),
                public_key,
                voting_power: v["voting_power"].as_u64().expect("power"),
            }
        })
        .collect();
    let h = header(&c["header"]);
    let commit = &c["commit"];
    let votes = commit["signatures"]
        .as_array()
        .expect("votes")
        .iter()
        .filter(|vote| {
            if vote["block_id_flag"] == 1 {
                assert_eq!(vote["validator_address"], "0x");
                assert!(vote["signature"].is_null());
                assert_eq!(
                    vote["timestamp"],
                    json!({"seconds": -62_135_596_800_i64, "nanos": 0})
                );
                false
            } else {
                true
            }
        })
        .map(|vote| {
            assert_eq!(vote["block_id_flag"], 2);
            let address = fixed(&vote["validator_address"]);
            CommitVote {
                validator_address: address,
                voting_power: validators
                    .iter()
                    .find(|v| v.address == address)
                    .expect("pinned validator")
                    .voting_power,
                timestamp: time(&vote["timestamp"]),
                signature: fixed(&vote["signature"]),
            }
        })
        .collect();
    let id = ActivityEntryId::new("act_repair2231156").expect("entry");
    let mut fact = EvidenceBundle::receipt_fact(&id, bytes(&c["signedReceipt"]), Rung::Committed)
        .expect("real receipt codec");
    fact.committed = Some(CommittedReceiptEvidence {
        header: h.clone(),
        execution_header: header(&c["executionHeader"]),
        commit: BlockCommit {
            height: commit["height"].as_u64().expect("commit height"),
            round: commit["round"]
                .as_u64()
                .expect("round")
                .try_into()
                .expect("round u32"),
            block_id: block_id(&commit["block_id"]),
            votes,
        },
        batch_transaction: bytes(&c["batchProof"]["data"]),
        batch_total: c["batchProof"]["total"].as_u64().expect("total"),
        batch_aunts: c["batchProof"]["aunts"]
            .as_array()
            .expect("aunts")
            .iter()
            .map(fixed)
            .collect(),
        activity_ids: vec![fact.tx_hash],
    });
    let binding = &v["positive"];
    fact.header_binding = Some(ReceiptHeaderBinding {
        canonical_header_bytes: bytes(&binding["header"]),
        header_signature: fixed(&binding["header_signature"]),
        receipt_proof: Proof::new(
            binding["proof"]["LeafIndex"]
                .as_u64()
                .expect("leaf index")
                .try_into()
                .expect("index u32"),
            binding["proof"]["LeafCount"]
                .as_u64()
                .expect("count")
                .try_into()
                .expect("count u32"),
            binding["proof"]["Siblings"]
                .as_array()
                .expect("siblings")
                .iter()
                .map(|s| serde_json::from_value(s.clone()).expect("sibling digest"))
                .collect(),
        )
        .expect("real receipt Merkle proof"),
    });
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

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}
