use crate::read::ServedCheckpoint;
use layerx_client::sequencer::SequencerEndpoint;
use layerx_programs::hex;
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;
use serde::Deserialize;
use serde_json::Value;
use sha2::{Digest, Sha256};

pub const MAX_BUDGET_PROOF_BYTES: usize = 1_048_576;
const PAY: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1b,
];

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum BudgetProofError {
    Refused,
}

fn budget_verification_level(rank: u8) -> Result<VerificationLevel, BudgetProofError> {
    let level = VerificationLevel::from_wire_rank(rank).map_err(|_| BudgetProofError::Refused)?;
    require(level == VerificationLevel::COMMITTED)?;
    Ok(level)
}

pub struct BudgetProofTrust<'a> {
    pub tenant: &'a str,
    pub principal: &'a str,
    pub owner: &'a Did,
    pub protocol_version: u16,
    pub network_id: u32,
    pub checkpoint: &'a ServedCheckpoint,
    pub endpoint: &'a SequencerEndpoint,
    pub current_height: u64,
    pub now_ms: u64,
    pub maximum_age_seconds: u64,
}

pub struct VerifiedBudgetProof {
    budget_id: [u8; 32],
    asset: [u8; 32],
    source_account: [u8; 32],
    owner: String,
    observed_head_sequence: u64,
    remaining: u128,
    verification: VerificationLevel,
    evidence_digest: [u8; 32],
    receipt_digest: [u8; 32],
    checkpoint_digest: [u8; 32],
    age_sequences: u64,
    maximum_age_sequences: u64,
    digest: [u8; 32],
    canonical_export_bytes: Vec<u8>,
    committed_height: u64,
    get_budget_result: Vec<u8>,
}

impl VerifiedBudgetProof {
    pub const fn committed_height(&self) -> u64 {
        self.committed_height
    }
    pub fn get_budget_result(&self) -> &[u8] {
        &self.get_budget_result
    }
    pub const fn budget_id(&self) -> [u8; 32] {
        self.budget_id
    }
    pub const fn asset(&self) -> [u8; 32] {
        self.asset
    }
    pub const fn source_account(&self) -> [u8; 32] {
        self.source_account
    }
    pub fn owner(&self) -> &str {
        &self.owner
    }
    pub const fn observed_head_sequence(&self) -> u64 {
        self.observed_head_sequence
    }
    pub const fn remaining(&self) -> u128 {
        self.remaining
    }
    pub const fn verification(&self) -> VerificationLevel {
        self.verification
    }
    pub const fn evidence_digest(&self) -> [u8; 32] {
        self.evidence_digest
    }
    pub const fn receipt_digest(&self) -> [u8; 32] {
        self.receipt_digest
    }
    pub const fn checkpoint_digest(&self) -> [u8; 32] {
        self.checkpoint_digest
    }
    pub const fn age_sequences(&self) -> u64 {
        self.age_sequences
    }
    pub const fn maximum_age_sequences(&self) -> u64 {
        self.maximum_age_sequences
    }
    pub const fn digest(&self) -> [u8; 32] {
        self.digest
    }
    pub fn canonical_export_bytes(&self) -> &[u8] {
        &self.canonical_export_bytes
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Export {
    schema: String,
    tenant: String,
    principal: String,
    protocol_version: u16,
    network_id: u32,
    maximum_age_seconds: u64,
    head: ExportHead,
    budget_state: State,
    accounts: Accounts,
    identity_key: String,
    proofs: Proofs,
    finality: Finality,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct ExportHead {
    committed_height: u64,
    block_hash: String,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Accounts {
    owner: String,
    budget: String,
    source: String,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct State {
    budget_id: String,
    asset: String,
    revocation_sequence: u64,
    observed_head_sequence: u64,
    verification: u8,
    evidence_digest: String,
    receipt_digest: String,
    checkpoint_digest: String,
    age_sequences: u64,
    maximum_age_sequences: u64,
    remaining: String,
    closed: bool,
    revoked: bool,
    canonical_core_bytes: String,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Finality {
    height: u64,
    block_hash: String,
    app_hash: String,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Proofs {
    budget_call: Call,
    remaining_call: Call,
    account_join: Value,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Call {
    to: String,
    data: String,
    result: String,
    height: u64,
}

fn require(value: bool) -> Result<(), BudgetProofError> {
    if value {
        Ok(())
    } else {
        Err(BudgetProofError::Refused)
    }
}
fn bytes(value: &str) -> Result<Vec<u8>, BudgetProofError> {
    require(!value.is_empty() && value.len() <= MAX_BUDGET_PROOF_BYTES && value.len() % 2 == 0)?;
    let decoded = hex::decode(value).map_err(|_| BudgetProofError::Refused)?;
    require(hex::encode(&decoded) == value)?;
    Ok(decoded)
}
fn id(value: &str) -> Result<[u8; 32], BudgetProofError> {
    bytes(value)?
        .try_into()
        .map_err(|_| BudgetProofError::Refused)
}
fn calldata(signature: &[u8], budget_id: [u8; 32]) -> Vec<u8> {
    let mut data = layerx_crypto::evm_transaction::transaction_hash(signature)[..4].to_vec();
    data.extend_from_slice(&budget_id);
    data
}
fn replay_call(
    call: &Call,
    signature: &[u8],
    budget_id: [u8; 32],
    trust: &BudgetProofTrust<'_>,
) -> Result<Vec<u8>, BudgetProofError> {
    let data = calldata(signature, budget_id);
    require(
        call.height == trust.checkpoint.height
            && bytes(&call.to)? == PAY
            && bytes(&call.data)? == data,
    )?;
    let result = bytes(&call.result)?;
    require(
        trust
            .endpoint
            .call_at(call.height, &PAY, &data)
            .map_err(|_| BudgetProofError::Refused)?
            == result,
    )?;
    Ok(result)
}

pub fn budget_proof_committed_head(export: &[u8]) -> Result<(u64, [u8; 32]), BudgetProofError> {
    require(!export.is_empty() && export.len() <= MAX_BUDGET_PROOF_BYTES)?;
    let value: Export = serde_json::from_slice(export).map_err(|_| BudgetProofError::Refused)?;
    require(value.head.committed_height > 0)?;
    Ok((value.head.committed_height, id(&value.head.block_hash)?))
}

fn verify_native_budget_proof(
    export: &[u8],
    budget_id: [u8; 32],
    trust: &BudgetProofTrust<'_>,
) -> Result<VerifiedBudgetProof, BudgetProofError> {
    require(!export.is_empty() && export.len() <= MAX_BUDGET_PROOF_BYTES && budget_id != [0; 32])?;
    let value: Export = serde_json::from_slice(export).map_err(|_| BudgetProofError::Refused)?;
    let checkpoint = trust.checkpoint;
    require(
        value.schema == "layerx.human.budget-proof.v2"
            && value.tenant == trust.tenant
            && value.principal == trust.principal
            && value.protocol_version == trust.protocol_version
            && value.network_id == trust.network_id
            && checkpoint.header.chain_id == trust.network_id.to_string()
            && value.head.committed_height > 0
            && value.head.committed_height == checkpoint.height
            && value.finality.height == checkpoint.height
            && id(&value.head.block_hash)? == checkpoint.block_hash
            && id(&value.finality.block_hash)? == checkpoint.block_hash
            && id(&value.finality.app_hash)? == checkpoint.app_hash,
    )?;
    // Revalidate the supplied checkpoint rather than trusting its public fields.
    require(
        crate::read::checkpoint(
            checkpoint.height,
            &trust.network_id.to_string(),
            &checkpoint.header,
            &checkpoint.commit,
            &checkpoint.validators,
        )
        .map_err(|_| BudgetProofError::Refused)?
            == *checkpoint,
    )?;
    let age = trust
        .current_height
        .checked_sub(checkpoint.height)
        .ok_or(BudgetProofError::Refused)?;
    let state = &value.budget_state;
    let verification = budget_verification_level(state.verification)?;
    require(
        state.maximum_age_sequences > 0
            && age <= state.maximum_age_sequences
            && trust.maximum_age_seconds > 0
            && value.maximum_age_seconds > 0
            && value.maximum_age_seconds <= trust.maximum_age_seconds,
    )?;
    let timestamp_ms = u64::try_from(checkpoint.header.time.seconds)
        .map_err(|_| BudgetProofError::Refused)?
        .checked_mul(1000)
        .and_then(|ms| ms.checked_add(u64::from(checkpoint.header.time.nanos / 1_000_000)))
        .ok_or(BudgetProofError::Refused)?;
    require(
        trust
            .now_ms
            .checked_sub(timestamp_ms)
            .ok_or(BudgetProofError::Refused)?
            <= value
                .maximum_age_seconds
                .checked_mul(1000)
                .ok_or(BudgetProofError::Refused)?,
    )?;
    let budget_call = replay_call(
        &value.proofs.budget_call,
        b"getBudget(bytes32)",
        budget_id,
        trust,
    )?;
    let remaining_call = replay_call(
        &value.proofs.remaining_call,
        b"budgetRemaining(bytes32)",
        budget_id,
        trust,
    )?;
    let committed =
        super::ProtocolBudgetState::decode_get_budget(budget_id, checkpoint.height, &budget_call)
            .map_err(|_| BudgetProofError::Refused)?;
    let record = committed.record();
    let source = record
        .source_account
        .filter(|source| *source != record.owner)
        .unwrap_or(record.owner);
    require(
        id(&value.accounts.owner)? == record.owner
            && record.owner != [0; 32]
            && id(&value.accounts.budget)? == record.budget_account
            && id(&value.accounts.source)? == source,
    )?;
    let owner_did =
        std::str::from_utf8(trust.owner.as_bytes()).map_err(|_| BudgetProofError::Refused)?;
    let mut identity = b"LXP/v1/did-id\0".to_vec();
    identity.extend_from_slice(
        &u16::try_from(owner_did.len())
            .map_err(|_| BudgetProofError::Refused)?
            .to_be_bytes(),
    );
    identity.extend_from_slice(owner_did.as_bytes());
    require(id(&value.identity_key)? == <[u8; 32]>::from(Sha256::digest(identity)))?;
    let join = &value.proofs.account_join;
    require(join["account"]["layerx_did"].as_str() == Some(owner_did))?;
    let rows = join["layerx"]["accounts"]
        .as_array()
        .ok_or(BudgetProofError::Refused)?;
    let matches = rows
        .iter()
        .filter(|row| {
            row["bound"] == true
                && row["layerx_account"]
                    .as_str()
                    .and_then(|text| id(text.strip_prefix("0x").unwrap_or(text)).ok())
                    == Some(record.owner)
                && row["evm_address"]
                    .as_str()
                    .and_then(|text| text.strip_prefix("0x"))
                    .and_then(|text| hex::decode(text).ok())
                    .is_some_and(|bytes| bytes.len() == 20 && bytes != [0; 20])
        })
        .count();
    require(matches == 1)?;
    require(remaining_call.len() == 32 && remaining_call[..16] == [0; 16])?;
    let remaining = u128::from_be_bytes(
        remaining_call[16..]
            .try_into()
            .map_err(|_| BudgetProofError::Refused)?,
    );
    require(
        remaining <= record.remaining()
            && (!(record.closed
                || record.revoked
                || timestamp_ms < record.period_start
                || timestamp_ms >= record.expiry)
                || remaining == 0),
    )?;
    let join_bytes = serde_json::to_vec(join).map_err(|_| BudgetProofError::Refused)?;
    let mut evidence = Sha256::new();
    evidence.update(b"layerx-human/rpc-budget-evidence/v2\0");
    evidence.update(checkpoint.block_hash);
    for material in [&budget_call, &remaining_call, &join_bytes] {
        evidence.update(
            u64::try_from(material.len())
                .map_err(|_| BudgetProofError::Refused)?
                .to_be_bytes(),
        );
        evidence.update(material);
    }
    let evidence_digest: [u8; 32] = evidence.finalize().into();
    let receipt_digest: [u8; 32] = Sha256::digest(&budget_call).into();
    require(
        id(&state.budget_id)? == budget_id
            && id(&state.asset)? == record.asset_id
            && state.revocation_sequence == record.revocation_sequence
            && state.revocation_sequence > 0
            && state.revocation_sequence <= checkpoint.height
            && state.observed_head_sequence == checkpoint.height
            && id(&state.evidence_digest)? == evidence_digest
            && id(&state.receipt_digest)? == receipt_digest
            && id(&state.checkpoint_digest)? == checkpoint.block_hash
            && state.age_sequences == 0
            && state.remaining == remaining.to_string()
            && state.closed == record.closed
            && state.revoked == record.revoked
            && bytes(&state.canonical_core_bytes)? == budget_call,
    )?;
    Ok(VerifiedBudgetProof {
        budget_id,
        asset: record.asset_id,
        source_account: source,
        owner: owner_did.to_owned(),
        observed_head_sequence: checkpoint.height,
        remaining,
        verification,
        evidence_digest,
        receipt_digest,
        checkpoint_digest: checkpoint.block_hash,
        age_sequences: age,
        maximum_age_sequences: state.maximum_age_sequences,
        digest: Sha256::digest(export).into(),
        canonical_export_bytes: export.to_vec(),
        committed_height: checkpoint.height,
        get_budget_result: budget_call,
    })
}

pub trait BudgetProofVerifier {
    type Verified;
    fn verify(
        &self,
        export: &[u8],
        budget_id: [u8; 32],
    ) -> Result<Self::Verified, BudgetProofError>;
}
pub fn verify_budget_proof<T: BudgetProofVerifier>(
    export: &[u8],
    budget_id: [u8; 32],
    trust: &T,
) -> Result<T::Verified, BudgetProofError> {
    trust.verify(export, budget_id)
}
impl BudgetProofVerifier for BudgetProofTrust<'_> {
    type Verified = VerifiedBudgetProof;
    fn verify(
        &self,
        export: &[u8],
        budget_id: [u8; 32],
    ) -> Result<Self::Verified, BudgetProofError> {
        verify_native_budget_proof(export, budget_id, self)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::read::{
        validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp,
        CommitValidator, CommitVote,
    };
    use ed25519_dalek::{Signer as _, SigningKey};
    use serde_json::json;
    use std::io::{Read as _, Write as _};
    use std::sync::{
        atomic::{AtomicBool, Ordering},
        Arc,
    };

    #[test]
    fn repair_22_31_535_budget_verification_requires_committed() {
        assert_eq!(
            budget_verification_level(VerificationLevel::COMMITTED.wire_rank()),
            Ok(VerificationLevel::COMMITTED)
        );
        for rank in [0, 1, 2, 3, 4, 5, 7, u8::MAX] {
            assert_eq!(
                budget_verification_level(rank),
                Err(BudgetProofError::Refused),
                "rank {rank}"
            );
        }
    }

    const OWNER: &str =
        "did:layerx:0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a";
    const HEIGHT: u64 = 8;
    fn budget_id() -> [u8; 32] {
        let mut id = [0; 32];
        id[31] = 7;
        id
    }
    fn fixture_path() -> std::path::PathBuf {
        std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../../../../tests/vectors/budget-proof-v2/node-answers.json")
    }
    fn b64(bytes: &[u8]) -> String {
        const ALPHABET: &[u8; 64] =
            b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
        let mut encoded = String::new();
        for chunk in bytes.chunks(3) {
            let word = (u32::from(chunk[0]) << 16)
                | (u32::from(*chunk.get(1).unwrap_or(&0)) << 8)
                | u32::from(*chunk.get(2).unwrap_or(&0));
            encoded.push(ALPHABET[(word >> 18) as usize] as char);
            encoded.push(ALPHABET[((word >> 12) & 63) as usize] as char);
            encoded.push(if chunk.len() > 1 {
                ALPHABET[((word >> 6) & 63) as usize] as char
            } else {
                '='
            });
            encoded.push(if chunk.len() > 2 {
                ALPHABET[(word & 63) as usize] as char
            } else {
                '='
            });
        }
        encoded
    }
    fn generated_answers() -> Value {
        let key = SigningKey::from_bytes(&[1; 32]);
        let public_key = key.verifying_key().to_bytes();
        let validator = CommitValidator {
            address: CommitValidator::address_for_key(&public_key),
            public_key,
            voting_power: 10,
        };
        let timestamp = CommitTimestamp {
            seconds: 1_700_000_000,
            nanos: 0,
        };
        let header = CheckpointHeader {
            version_block: 11,
            version_app: 1,
            chain_id: "125".into(),
            height: HEIGHT,
            time: timestamp,
            last_block_id: Some(BlockId {
                hash: [1; 32],
                part_set_total: 1,
                part_set_hash: [2; 32],
            }),
            last_commit_hash: Some([3; 32]),
            data_hash: Some([4; 32]),
            validators_hash: validator_set_hash(&[validator]),
            next_validators_hash: validator_set_hash(&[validator]),
            consensus_hash: [5; 32],
            app_hash: [6; 32],
            last_results_hash: Some([7; 32]),
            evidence_hash: Some([8; 32]),
            proposer_address: validator.address,
        };
        let mut commit = BlockCommit {
            height: HEIGHT,
            round: 1,
            block_id: BlockId {
                hash: header.block_hash(),
                part_set_total: 1,
                part_set_hash: [9; 32],
            },
            votes: Vec::new(),
        };
        let signature = key
            .sign(&commit.vote_sign_bytes("125", timestamp))
            .to_bytes();
        commit.votes.push(CommitVote {
            validator_address: validator.address,
            voting_power: 10,
            timestamp,
            signature,
        });
        crate::read::checkpoint(HEIGHT, "125", &header, &commit, &[validator])
            .expect("signed checkpoint");
        let h = |bytes: &[u8]| hex::encode(bytes);
        let prefixed = |bytes: &[u8]| format!("0x{}", h(bytes));
        let block_id = |id: BlockId| json!({"hash": h(&id.hash), "parts": {"total": id.part_set_total, "hash": h(&id.part_set_hash)}});
        let signed = json!({"header": {
            "version": {"block": "11", "app": "1"}, "chain_id": "125", "height": "8", "time": "2023-11-14T22:13:20Z",
            "last_block_id": block_id(header.last_block_id.expect("parent")), "last_commit_hash": h(&[3;32]), "data_hash": h(&[4;32]),
            "validators_hash": h(&header.validators_hash), "next_validators_hash": h(&header.next_validators_hash), "consensus_hash": h(&[5;32]),
            "app_hash": h(&[6;32]), "last_results_hash": h(&[7;32]), "evidence_hash": h(&[8;32]), "proposer_address": h(&validator.address)
        }, "commit": {"height":"8", "round":1, "block_id":block_id(commit.block_id), "signatures":[{
            "block_id_flag":2, "validator_address":h(&validator.address), "timestamp":"2023-11-14T22:13:20Z", "signature":b64(&signature)
        }]}});
        let mut words = vec![[0; 32]; 20];
        words[1] = [2; 32];
        words[2] = [3; 32];
        words[3] = [4; 32];
        for (index, number) in [
            (0, 32_u64),
            (5, 100),
            (6, 100),
            (8, 23),
            (10, 60_000),
            (11, 1_699_999_990_000),
            (12, 1_700_000_060_000),
            (13, 1),
            (14, 1),
            (17, 576),
        ] {
            words[index][24..].copy_from_slice(&number.to_be_bytes());
        }
        let mut remaining = [0; 32];
        remaining[31] = 77;
        let identity = |id: [u8; 32]| json!({"evm_address":prefixed(&[0x11;20]), "pax_address":null, "layerx_did":OWNER, "layerx_account":h(&id), "bound":true});
        let join = json!({"account":identity([2;32]), "paxeer":null, "layerx":{"accounts":[identity([2;32])]}});
        let balance = |id: [u8; 32]| json!({"account":identity(id), "balances":[{"asset_id":h(&[4;32]), "denom":null, "custody":null, "paxeer":null, "layerx":{"amount":"100"}}], "joined_limit":100});
        let mut answers = json!([
            {"method":"eth_blockNumber", "params":[], "result":"0x8"},
            {"method":"eth_getBlockByNumber", "params":["0x8",true], "result":{
                "number":"0x8", "hash":prefixed(&header.block_hash()), "parentHash":prefixed(&[1;32]), "timestamp":"0x6553f100",
                "transactionsRoot":prefixed(&[4;32]), "receiptsRoot":prefixed(&[7;32]), "stateRoot":prefixed(&header.app_hash), "transactions":[]}},
            {"method":"px_getCommit", "params":["8"], "result":{"height":"8", "signedHeader":signed}},
            {"method":"px_getValidators", "params":["8"], "result":{"height":"8", "total":"1", "validators":[{
                "address":h(&validator.address), "pubKey":{"type":"tendermint/PubKeyEd25519", "value":b64(&public_key)}, "votingPower":"10"}]}},
            {"method":"eth_call", "params":[{"to":prefixed(&PAY), "data":prefixed(&calldata(b"getBudget(bytes32)",budget_id()))},"0x8"], "result":prefixed(&words.concat())},
            {"method":"eth_call", "params":[{"to":prefixed(&PAY), "data":prefixed(&calldata(b"budgetRemaining(bytes32)",budget_id()))},"0x8"], "result":prefixed(&remaining)},
            {"method":"px_getAccount", "params":[OWNER], "result":join},
            {"method":"px_getBalances", "params":[h(&[2;32])], "result":balance([2;32])},
            {"method":"px_getBalances", "params":[h(&[3;32])], "result":balance([3;32])}
        ]);
        let mut state_root = answers[1].clone();
        state_root["params"][1] = json!(false);
        answers.as_array_mut().expect("answers").push(state_root);
        answers
    }
    #[test]
    fn budget_proof_v2_node_answers_fixture_is_current() {
        fn assert_layerx_dids(value: &Value) {
            match value {
                Value::Object(fields) => {
                    for (name, value) in fields {
                        if name == "layerx_did" {
                            let suffix = value
                                .as_str()
                                .expect("layerx_did string")
                                .strip_prefix("did:layerx:")
                                .expect("layerx_did prefix");
                            assert_eq!(suffix.len(), 64, "layerx_did hex length");
                            assert!(
                                suffix
                                    .bytes()
                                    .all(|byte| byte.is_ascii_hexdigit()
                                        && !byte.is_ascii_uppercase()),
                                "layerx_did must contain lowercase ASCII hexadecimal characters"
                            );
                        }
                        assert_layerx_dids(value);
                    }
                }
                Value::Array(values) => {
                    for value in values {
                        assert_layerx_dids(value);
                    }
                }
                _ => {}
            }
        }
        let answers = generated_answers();
        assert_layerx_dids(&answers);
        let mut generated = serde_json::to_vec_pretty(&answers).expect("answers JSON");
        generated.push(b'\n');
        if std::env::var("LAYERX_WRITE_VECTORS").as_deref() == Ok("1") {
            std::fs::create_dir_all(fixture_path().parent().expect("fixture directory"))
                .expect("directory");
            std::fs::write(fixture_path(), &generated).expect("write answers");
        }
        assert_eq!(
            std::fs::read(fixture_path()).expect("committed answers"),
            generated
        );
    }
    struct Replay {
        endpoint: SequencerEndpoint,
        stop: Arc<AtomicBool>,
        worker: Option<std::thread::JoinHandle<usize>>,
    }
    impl Replay {
        fn new(answers: Value) -> Self {
            let listener = std::net::TcpListener::bind("127.0.0.1:0").expect("bind RPC");
            listener
                .set_nonblocking(true)
                .expect("nonblocking listener");
            let address = listener.local_addr().expect("RPC address");
            let stop = Arc::new(AtomicBool::new(false));
            let stopped = Arc::clone(&stop);
            let worker = std::thread::spawn(move || {
                let mut calls = 0;
                while !stopped.load(Ordering::SeqCst) {
                    let mut stream = match listener.accept() {
                        Ok((stream, _)) => stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            std::thread::sleep(std::time::Duration::from_millis(1));
                            continue;
                        }
                        Err(error) => panic!("RPC accept: {error}"),
                    };
                    stream
                        .set_read_timeout(Some(std::time::Duration::from_secs(5)))
                        .expect("read deadline");
                    let mut headers = Vec::new();
                    while !headers.ends_with(b"\r\n\r\n") {
                        let mut byte = [0];
                        stream.read_exact(&mut byte).expect("HTTP header");
                        headers.push(byte[0]);
                    }
                    let headers = String::from_utf8(headers).expect("headers");
                    let length: usize = headers
                        .lines()
                        .find_map(|line| {
                            let (name, value) = line.split_once(':')?;
                            name.eq_ignore_ascii_case("content-length")
                                .then(|| value.trim().parse().expect("length"))
                        })
                        .expect("content length");
                    let mut body = vec![0; length];
                    stream.read_exact(&mut body).expect("RPC body");
                    let request: Value = serde_json::from_slice(&body).expect("RPC JSON");
                    let answer = answers
                        .as_array()
                        .expect("answers")
                        .iter()
                        .find(|answer| {
                            answer["method"] == request["method"]
                                && answer["params"] == request["params"]
                        })
                        .expect("recorded method and params");
                    if request["method"] == "eth_call" {
                        calls += 1;
                    }
                    let body =
                        json!({"jsonrpc":"2.0", "id":request["id"], "result":answer["result"]})
                            .to_string();
                    write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",body.len(),body).expect("RPC reply");
                }
                calls
            });
            Self {
                endpoint: SequencerEndpoint::new(&format!("http://{address}"), &[])
                    .expect("endpoint"),
                stop,
                worker: Some(worker),
            }
        }
        fn finish(mut self) -> usize {
            self.stop.store(true, Ordering::SeqCst);
            self.worker
                .take()
                .expect("worker")
                .join()
                .expect("RPC replay")
        }
    }
    impl Drop for Replay {
        fn drop(&mut self) {
            self.stop.store(true, Ordering::SeqCst);
            if let Some(worker) = self.worker.take() {
                let _ = worker.join();
            }
        }
    }
    #[test]
    fn budget_proof_v2_verifies_the_fixture_export_and_refuses_tampering() {
        let answers: Value =
            serde_json::from_slice(&std::fs::read(fixture_path()).expect("answers"))
                .expect("fixture JSON");
        let export = std::fs::read(fixture_path().with_file_name("export.json"))
            .expect("authority's committed export from task 8.98.18");
        let mut document: Value = serde_json::from_slice(&export).expect("authority export JSON");
        document["budget_state"]["verification"] = json!(VerificationLevel::COMMITTED.wire_rank());
        let export = serde_json::to_vec(&document).expect("current committed export label");
        let replay = Replay::new(answers);
        let hash = id(document["head"]["block_hash"].as_str().expect("hash")).expect("block hash");
        assert_eq!(
            replay.endpoint.block_number().expect("current block"),
            HEIGHT
        );
        let checkpoint = ServedCheckpoint::acquire_at_committed_block(
            &replay.endpoint,
            HEIGHT,
            Some(hash),
            "125",
        )
        .expect("acquire signed checkpoint");
        let owner = Did::new(OWNER.as_bytes()).expect("owner DID");
        let mut trust = BudgetProofTrust {
            tenant: "tenant",
            principal: "operator",
            owner: &owner,
            protocol_version: 3,
            network_id: 125,
            checkpoint: &checkpoint,
            endpoint: &replay.endpoint,
            current_height: HEIGHT,
            now_ms: 1_700_000_000_000,
            maximum_age_seconds: 60,
        };
        let verify = |document: &Value, trust: &BudgetProofTrust<'_>| {
            verify_native_budget_proof(
                &serde_json::to_vec(document).expect("export"),
                budget_id(),
                trust,
            )
        };
        let verified = verify_native_budget_proof(&export, budget_id(), &trust)
            .expect("authority's committed v2 proof must verify without adjustment");
        assert_eq!(verified.canonical_export_bytes(), export.as_slice());
        assert_eq!(verified.remaining(), 77);
        assert_eq!(verified.committed_height(), HEIGHT);
        assert_eq!(verified.checkpoint_digest(), checkpoint.block_hash);
        assert_eq!(verified.source_account(), [2; 32]);
        assert_eq!(verified.verification(), VerificationLevel::COMMITTED);
        for rank in [0, 1, 2, 3, 4, 5, 7, u8::MAX] {
            let mut retired = document.clone();
            retired["budget_state"]["verification"] = json!(rank);
            assert!(
                matches!(verify(&retired, &trust), Err(BudgetProofError::Refused)),
                "rank {rank}"
            );
        }
        let wrong_owner = Did::new(
            b"did:layerx:0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b",
        )
        .expect("different owner DID");
        let wrong_owner_trust = BudgetProofTrust {
            owner: &wrong_owner,
            ..trust
        };
        assert!(matches!(
            verify_budget_proof(
                &serde_json::to_vec(&document).expect("export"),
                budget_id(),
                &wrong_owner_trust,
            ),
            Err(BudgetProofError::Refused)
        ));
        for name in ["budget_call", "remaining_call"] {
            let mut changed = document.clone();
            let mut result =
                bytes(changed["proofs"][name]["result"].as_str().expect("result")).expect("bytes");
            result[31] ^= 1;
            changed["proofs"][name]["result"] = json!(hex::encode(&result));
            assert!(verify(&changed, &trust).is_err(), "changed {name} result");
        }
        for field in ["block_hash", "app_hash"] {
            let mut changed = document.clone();
            changed["finality"][field] = json!(hex::encode(&[0xaa; 32]));
            assert!(
                verify(&changed, &trust).is_err(),
                "changed finality {field}"
            );
        }
        let mut changed = document.clone();
        changed["head"]["committed_height"] = json!(HEIGHT + 1);
        assert!(verify(&changed, &trust).is_err(), "substituted height");
        trust.current_height = HEIGHT + 2;
        assert_eq!(
            verify(&document, &trust)
                .expect("maximum permitted age")
                .age_sequences(),
            2
        );
        trust.current_height = HEIGHT + 3;
        assert!(verify(&document, &trust).is_err(), "stale height");
        trust.current_height = HEIGHT - 1;
        assert!(verify(&document, &trust).is_err(), "future height");
        trust.current_height = HEIGHT;
        trust.now_ms += 60_001;
        assert!(verify(&document, &trust).is_err(), "stale time");
        assert_eq!(
            replay.finish(),
            9,
            "valid proof, owner refusal and altered results must execute eth_call"
        );
    }
}
