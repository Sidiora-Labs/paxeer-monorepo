use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::{
    atomic::{AtomicBool, AtomicU64, Ordering},
    Arc,
};
use std::thread;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    header::{
        validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator,
    },
    proto::{encode_length_delimited, encode_varint, encode_varint_field},
    ProofError,
};
use layerx_indexer::ai_market::{
    verify_program_state, BlockCommitProof, FinalityAuthority, Observation as SnapshotObservation,
    ProjectionStore, ReadVerification, ViewError,
};
use layerx_indexer::ai_reputation::{
    Admission, FinalizedRead, HistoryRequest, ReputationError, ReputationProjector,
};
use layerx_indexer::{
    abi::AbiRegistry,
    backfill::Backfill,
    blockscout::{rpc_blocks, BlockscoutRange},
    chains::{chains_abi, ChainsConfig},
    codec::unhex,
    follow::{FollowPolicy, StepOutcome},
    paxeer::{AttributeEncoding, PaxeerIngester},
    store::Store,
    transport::{Endpoint, Security},
    IndexError,
};
use layerx_programs_ai_market::{
    codec::{self, decode_envelope, derive_market, derive_worker, encode_envelope, Envelope},
    dispatch,
    errors::ApplicationError,
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    queries::{read_state_chunk, QueryError, ReadProof, StateCapture},
    registry::{derive_rewards_account, F01_SECTION_CAP},
    registry_ops::{self, Outcome},
    reputation::{CompletedHistory, Observation, ReputationCurrent, SegmentKey, SECTION_CAP},
    reputation_codec::{
        decode_section, encode_current, encode_history, encode_section, reputation_root,
    },
    state::{self, Section, SharedState},
    types::{
        AssetId, Authentication, ChainDomain, Digest32, MarketId, Presence, PrincipalId, ProgramId,
        RequestId, RubricDigest, Score, StateDigest, Version, WorkerId,
    },
    MAX_EVENT_BYTES, MAX_STATE_BYTES,
};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const CHAIN: [u8; 32] = [10; 32];
const PROGRAM: [u8; 32] = [11; 32];
const OWNER: [u8; 32] = [12; 32];
const ASSET: [u8; 32] = [13; 32];
const REFUND: [u8; 32] = [14; 32];
const CHAIN_ID: &str = "ai-reputation-finality";
const CHUNK_RESPONSE_MAX: usize = 8_244;
const PUBLICATION_TIME_MS: u64 = 5_000;
const FINALIZED_RANK: u8 = 4;

#[derive(Debug)]
pub enum Failure {
    Application(ApplicationError),
    Query(QueryError),
    Reputation(ReputationError),
    Commit(ProofError),
    Io(std::io::Error),
    View(ViewError),
    Index(IndexError),
    Json(serde_json::Error),
    Unexpected(&'static str),
}
impl std::fmt::Display for Failure {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Application(e) => write!(f, "application: {e:?}"),
            Self::Query(e) => write!(f, "query: {e:?}"),
            Self::Reputation(e) => write!(f, "reputation: {e:?}"),
            Self::Commit(e) => write!(f, "commit: {e:?}"),
            Self::Io(e) => write!(f, "io: {e}"),
            Self::View(e) => write!(f, "view: {e:?}"),
            Self::Index(e) => write!(f, "index: {e}"),
            Self::Json(e) => write!(f, "JSON: {e}"),
            Self::Unexpected(e) => write!(f, "unexpected: {e}"),
        }
    }
}
impl std::error::Error for Failure {}
impl From<ApplicationError> for Failure {
    fn from(e: ApplicationError) -> Self {
        Self::Application(e)
    }
}
impl From<QueryError> for Failure {
    fn from(e: QueryError) -> Self {
        Self::Query(e)
    }
}
impl From<ReputationError> for Failure {
    fn from(e: ReputationError) -> Self {
        Self::Reputation(e)
    }
}
impl From<ProofError> for Failure {
    fn from(e: ProofError) -> Self {
        Self::Commit(e)
    }
}
impl From<std::io::Error> for Failure {
    fn from(e: std::io::Error) -> Self {
        Self::Io(e)
    }
}
impl From<ViewError> for Failure {
    fn from(e: ViewError) -> Self {
        Self::View(e)
    }
}
impl From<IndexError> for Failure {
    fn from(e: IndexError) -> Self {
        Self::Index(e)
    }
}
impl From<serde_json::Error> for Failure {
    fn from(e: serde_json::Error) -> Self {
        Self::Json(e)
    }
}
pub type Checked<T = ()> = Result<T, Failure>;
fn chain() -> Checked<ChainDomain> {
    Ok(ChainDomain::new(CHAIN)?)
}

fn program() -> Checked<ProgramId> {
    Ok(ProgramId::new(PROGRAM)?)
}

fn principal(bytes: [u8; 32]) -> Checked<PrincipalId> {
    Ok(PrincipalId::new(bytes)?)
}

fn digest(byte: u8) -> Checked<Digest32> {
    Ok(Digest32::new([byte; 32])?)
}

fn market() -> Checked<MarketId> {
    Ok(derive_market(chain()?, program()?)?)
}

fn worker(nonce: u8) -> Checked<WorkerId> {
    Ok(derive_worker(market()?, principal(OWNER)?, [nonce; 32])?)
}

fn policy() -> Checked<TaskPolicyV1> {
    Ok(TaskPolicyV1::bounded_default(
        1,
        1,
        PolicyCommitments {
            model_artifact: digest(1)?,
            dataset_artifact: [2; 32],
            benchmark_suite: digest(3)?,
            rubric: RubricDigest::new([4; 32])?,
            task_schema: digest(5)?,
            result_schema: digest(6)?,
            service_terms: digest(7)?,
        },
        100,
        1,
    )?)
}

fn encode_shared(shared: &SharedState<'_>) -> Checked<Vec<u8>> {
    let mut out = vec![0; shared.encoded_len()?];
    let mut scratch = vec![0; Section::Control.payload_cap()];
    let n = state::encode_shared_state(shared, &mut out, &mut scratch)?;
    out.truncate(n);
    Ok(out)
}

fn created_state() -> Checked<Vec<u8>> {
    let mut encoded_policy = vec![0; TASK_POLICY_BYTES];
    policy()?.encode(&mut encoded_policy)?;
    let rewards = derive_rewards_account(program()?, AssetId::new(ASSET)?)?;
    let mut payload = OWNER.to_vec();
    payload.extend_from_slice(&ASSET);
    payload.extend_from_slice(rewards.as_bytes());
    payload.extend_from_slice(&REFUND);
    payload.push(0);
    payload.extend_from_slice(&encoded_policy);
    payload.extend_from_slice(&[16; 32]);
    let envelope = Envelope {
        operation: dispatch::CREATE,
        chain: chain()?,
        program: program()?,
        market: market()?,
        actor: principal(OWNER)?,
        epoch: 0,
        config: 1,
        roster: Presence::Absent,
        sequence: 1,
        expiry: 1_000_000,
        request: RequestId::new([1; 32])?,
        payload: &payload,
        authentication: Authentication::Native,
    };
    let mut encoded = vec![0; 16_384];
    let n = encode_envelope(&envelope, &mut encoded)?;
    let validated = decode_envelope(encoded.get(..n).ok_or(Failure::Unexpected("envelope"))?)?;
    let ctx = registry_ops::CallContext {
        chain: chain()?,
        program: program()?,
        principal: principal(OWNER)?,
        height: 1000,
    };
    let mut section = vec![0; F01_SECTION_CAP];
    let mut event = vec![0; MAX_EVENT_BYTES];
    match registry_ops::apply(&ctx, None, &validated, &mut section, &mut event)? {
        Outcome::Applied { state, .. } => encode_shared(&state),
        Outcome::AlreadyApplied(_) => Err(Failure::Unexpected("fresh create retried")),
    }
}

fn segment(worker: WorkerId) -> Checked<SegmentKey> {
    Ok(SegmentKey {
        market: market()?,
        worker,
        config: Version::new(1)?,
        policy: digest(1)?,
        model: digest(1)?,
        reset_generation: Version::new(1)?,
    })
}

fn unobserved(worker: WorkerId, height: u64) -> Checked<ReputationCurrent> {
    Ok(ReputationCurrent::bootstrap(
        segment(worker)?,
        principal(OWNER)?,
        height,
    )?)
}

fn observed(worker: WorkerId, epoch: u64, height: u64) -> Checked<ReputationCurrent> {
    Ok(ReputationCurrent {
        quality: Score::new(600_000)?,
        qualifying_count: 1,
        last_applied: Presence::Present(epoch),
        last_observed: Presence::Present(Observation { epoch, height }),
        last_transition_height: height,
        ..unobserved(worker, height)?
    })
}

fn completion(epoch: u64, height: u64) -> Checked<CompletedHistory> {
    let mut result = [0xEE; 32];
    result[..8].copy_from_slice(&epoch.to_be_bytes());
    let mut row_root = [0xDD; 32];
    row_root[..8].copy_from_slice(&epoch.to_be_bytes());
    Ok(CompletedHistory {
        epoch,
        execution_height: height,
        config: Version::new(1)?,
        result: Digest32::new(result)?,
        root: Digest32::new(row_root)?,
        observed_workers: 1,
        total_workers: 2,
        covered_workers: 1,
    })
}

fn frame(
    mut records: Vec<ReputationCurrent>,
    history: &[CompletedHistory],
) -> Checked<(Vec<u8>, Vec<ReputationCurrent>)> {
    records.sort_by_key(|record| record.worker);
    let mut out = b"RP07".to_vec();
    out.extend_from_slice(&1u16.to_be_bytes());
    out.push(u8::try_from(records.len()).map_err(|_| Failure::Unexpected("records"))?);
    out.push(u8::try_from(history.len()).map_err(|_| Failure::Unexpected("history"))?);
    out.extend_from_slice(market()?.as_bytes());
    let (presence, completed) = history.last().map_or((0, 0), |row| (1, row.epoch));
    out.push(presence);
    out.extend_from_slice(&completed.to_be_bytes());
    out.extend_from_slice(&[0; 15]);
    for record in &records {
        out.extend_from_slice(&encode_current(record)?);
    }
    for row in history {
        out.extend_from_slice(&encode_history(row)?);
    }
    let decoded = decode_section(&out)?;
    assert_eq!(decoded.records().copied().collect::<Vec<_>>(), records);
    assert_eq!(decoded.completed().copied().collect::<Vec<_>>(), history);
    let mut reencoded = vec![0; SECTION_CAP];
    let n = encode_section(&decoded, &mut reencoded)?;
    assert_eq!(reencoded.get(..n), Some(out.as_slice()));
    Ok((out, records))
}

fn state_with(section: &[u8]) -> Checked<Vec<u8>> {
    let base = created_state()?;
    let shared = state::decode_shared_state(&base)?;
    encode_shared(&shared.replace_section(Section::ReputationAdmission, section)?)
}

fn chunk_payload(
    revision: u64,
    pinned: Option<StateDigest>,
    offset: u32,
    requested: u16,
) -> Vec<u8> {
    let mut payload = revision.to_be_bytes().to_vec();
    payload.extend_from_slice(&pinned.map_or([0; 32], StateDigest::bytes));
    payload.extend_from_slice(&offset.to_be_bytes());
    payload.extend_from_slice(&requested.to_be_bytes());
    payload
}

fn read_all(state_bytes: &[u8]) -> Checked<Vec<Vec<u8>>> {
    let mut out = vec![0; CHUNK_RESPONSE_MAX];
    let n = read_state_chunk(state_bytes, &chunk_payload(0, None, 0, 8192), &mut out)?;
    let first = codec::decode_chunk_response(&out[..n])?;
    let (revision, pinned, total) = (first.revision, first.digest, first.total_bytes);
    let mut bodies = vec![out[..n].to_vec()];
    for offset in (8192..total).step_by(8192) {
        let payload = chunk_payload(revision, Some(pinned), offset, 8192);
        let n = read_state_chunk(state_bytes, &payload, &mut out)?;
        bodies.push(out[..n].to_vec());
    }
    Ok(bodies)
}

fn validators() -> Vec<Validator> {
    (1..=4)
        .map(|value| Validator {
            pub_key: SigningKey::from_bytes(&[value; 32])
                .verifying_key()
                .to_bytes(),
            voting_power: 10,
        })
        .collect()
}

fn authority() -> FinalityAuthority {
    FinalityAuthority {
        chain_id: CHAIN_ID.to_owned(),
        validators: validators(),
    }
}

fn signed(header: &Header, signers: &[(u8, bool)]) -> Commit {
    let mut commit = Commit {
        height: header.height as u64,
        round: 0,
        block_id: BlockId {
            hash: header.hash().to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![0xA5; 32],
            },
        },
        signatures: Vec::new(),
    };
    for value in 1..=4 {
        let Some((_, tamper)) = signers.iter().find(|(signer, _)| *signer == value) else {
            commit.signatures.push(CommitSig::absent());
            continue;
        };
        let key = SigningKey::from_bytes(&[value; 32]);
        let timestamp = Timestamp {
            seconds: 1_000,
            nanos: 123,
        };
        let mut vote = Vec::new();
        encode_varint_field(1, 2, &mut vote);
        vote.push(0x11);
        vote.extend_from_slice(&commit.height.to_le_bytes());
        let mut parts = Vec::new();
        encode_varint_field(
            1,
            u64::from(commit.block_id.part_set_header.total),
            &mut parts,
        );
        encode_length_delimited(2, &commit.block_id.part_set_header.hash, &mut parts);
        let mut block = Vec::new();
        encode_length_delimited(1, &commit.block_id.hash, &mut block);
        encode_length_delimited(2, &parts, &mut block);
        encode_length_delimited(4, &block, &mut vote);
        let mut time = Vec::new();
        encode_varint_field(1, timestamp.seconds as u64, &mut time);
        encode_varint_field(2, u64::from(timestamp.nanos), &mut time);
        encode_length_delimited(5, &time, &mut vote);
        encode_length_delimited(6, header.chain_id.as_bytes(), &mut vote);
        let mut sign_bytes = Vec::new();
        encode_varint(vote.len() as u64, &mut sign_bytes);
        sign_bytes.extend_from_slice(&vote);
        let mut signature = key.sign(&sign_bytes).to_bytes();
        if *tamper {
            signature[40] ^= 1;
        }
        commit.signatures.push(CommitSig {
            block_id_flag: BlockIdFlag::Commit,
            validator_address: Sha256::digest(key.verifying_key().as_bytes())[..20].to_vec(),
            timestamp,
            signature: Some(signature),
        });
    }
    commit
}

fn header(height: u64, root: Digest32) -> Header {
    Header {
        chain_id: CHAIN_ID.to_owned(),
        height: height as i64,
        app_hash: root.bytes().to_vec(),
        version: ConsensusVersion { block: 11, app: 1 },
        time: Timestamp {
            seconds: 1_000,
            nanos: 0,
        },
        last_block_id: BlockId::default(),
        last_commit_hash: vec![1; 32],
        data_hash: vec![2; 32],
        validators_hash: validators_hash(&validators()),
        next_validators_hash: validators_hash(&validators()).to_vec(),
        consensus_hash: vec![3; 32],
        last_results_hash: vec![4; 32],
        evidence_hash: vec![5; 32],
        proposer_address: vec![6; 20],
    }
}

struct Material {
    header: Header,
    commit: Commit,
    proof: Vec<u8>,
}
impl Material {
    fn evidence(&self) -> BlockCommitProof<'_> {
        BlockCommitProof {
            header: &self.header,
            commit: &self.commit,
            store_proof: &self.proof,
        }
    }
}

// Canonical native fixture: the namespace head/manifest layout emitted by lxphost,
// with ICS23 leaf operations encoded by the production protobuf codec.
fn native_membership(state: &[u8], program: [u8; 32], cell: &[u8]) -> (Vec<u8>, [u8; 32]) {
    native_membership_version(state, program, cell, 1)
}

pub fn native_membership_version(
    state: &[u8],
    program: [u8; 32],
    cell: &[u8],
    version: u64,
) -> (Vec<u8>, [u8; 32]) {
    fn leaf(key: &[u8], value: &[u8], prefix: &[u8]) -> (Vec<u8>, [u8; 32]) {
        let mut operation = Vec::new();
        for (field, value) in [(1, 1), (2, 0), (3, 1), (4, 1)] {
            encode_varint_field(field, value, &mut operation);
        }
        encode_length_delimited(5, prefix, &mut operation);
        let mut proof = Vec::new();
        encode_length_delimited(1, key, &mut proof);
        encode_length_delimited(2, value, &mut proof);
        encode_length_delimited(3, &operation, &mut proof);
        let mut preimage = prefix.to_vec();
        encode_varint(key.len() as u64, &mut preimage);
        preimage.extend(key);
        encode_varint(32, &mut preimage);
        preimage.extend(Sha256::digest(value));
        (proof, Sha256::digest(preimage).into())
    }
    let mut manifest = vec![0, 1];
    manifest.extend(1u32.to_be_bytes());
    manifest.extend((cell.len() as u16).to_be_bytes());
    manifest.extend(cell);
    manifest.extend(Sha256::digest(state));
    manifest.extend((state.len() as u32).to_be_bytes());
    let mut head = vec![0, 1];
    head.extend(1u32.to_be_bytes());
    head.extend(Sha256::digest(&manifest));
    let mut key = b"\x01\x00\x09progstor".to_vec();
    key.extend(program);
    key.push(1);
    let mut prefix = vec![0, 2];
    encode_varint(version.checked_mul(2).expect("IAVL version"), &mut prefix);
    let (proof, store_root) = leaf(&key, &head, &prefix);
    let (outer, app_hash) = leaf(b"lxphost", &store_root, &[0]);
    let mut multistore = Vec::new();
    encode_length_delimited(1, &outer, &mut multistore);
    (serde_json::to_vec(&json!({"membership": {"proof": proof, "multistore": multistore}, "manifest": manifest})).expect("fixture JSON"), app_hash)
}

fn certify_state(state: &[u8], program: [u8; 32], cell: &[u8]) -> Checked<Material> {
    let (proof, root) = native_membership(state, program, cell);
    let header = header(1000, Digest32::new(root)?);
    let commit = signed(&header, &[(1, false), (2, false), (3, false), (4, false)]);
    Ok(Material {
        header,
        commit,
        proof,
    })
}

struct Scratch(PathBuf);
impl Scratch {
    fn new(name: &str) -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-1446-{}-{}-{name}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).expect("create scratch directory");
        Self(path)
    }
    fn db(&self) -> PathBuf {
        self.0.join("index.sqlite")
    }
}
impl Drop for Scratch {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove scratch directory");
    }
}

fn snapshot_observation<'a>(
    state: &'a [u8],
    facts: layerx_programs_ai_market::queries::CaptureFacts,
) -> Checked<SnapshotObservation<'a>> {
    Ok(SnapshotObservation {
        state,
        facts,
        read: ReadVerification::SequencerSigned,
        source_activity: facts.proof.block_hash,
        observed_at_ms: 4000,
    })
}

fn read_request(stream: &mut TcpStream) -> Option<Vec<u8>> {
    let mut bytes = Vec::new();
    let mut chunk = [0_u8; 4096];
    let header_end = loop {
        let count = stream.read(&mut chunk).ok()?;
        if count == 0 {
            return None;
        }
        bytes.extend_from_slice(&chunk[..count]);
        if let Some(position) = bytes.windows(4).position(|window| window == b"\r\n\r\n") {
            break position + 4;
        }
    };
    let head = String::from_utf8(bytes[..header_end].to_vec()).ok()?;
    let length = head
        .split("\r\n")
        .filter_map(|line| line.split_once(':'))
        .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
        .and_then(|(_, value)| value.trim().parse::<usize>().ok())
        .unwrap_or(0);
    while bytes.len() < header_end + length {
        let count = stream.read(&mut chunk).ok()?;
        if count == 0 {
            return None;
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
    Some(bytes[header_end..header_end + length].to_vec())
}

fn within(range: &BlockscoutRange, from: u64, to: u64, absent: &[u64]) -> BlockscoutRange {
    let keep = |height: u64| (from..=to).contains(&height) && !absent.contains(&height);
    BlockscoutRange {
        blocks: range
            .blocks
            .iter()
            .filter(|row| keep(row.number))
            .cloned()
            .collect(),
        transactions: range
            .transactions
            .iter()
            .filter(|row| keep(row.block_number))
            .cloned()
            .collect(),
        logs: range
            .logs
            .iter()
            .filter(|row| keep(row.block_number))
            .cloned()
            .collect(),
        token_transfers: range
            .token_transfers
            .iter()
            .filter(|row| keep(row.block_number))
            .cloned()
            .collect(),
        internal_transactions: range
            .internal_transactions
            .iter()
            .filter(|row| keep(row.block_number))
            .cloned()
            .collect(),
        tokens: range.tokens.clone(),
    }
}

// Use a current registry emission in the historical transaction slot occupied
// by a retired custody log in the older transcript. Both peers carry the exact
// recorded address, topics and data; only the containing transaction differs.
fn registry_log() -> Value {
    let fixture: Value = serde_json::from_str(include_str!("../fixtures/chains_blocks.json"))
        .expect("recorded registry emission");
    fixture["scope"]["receipts"]
        ["0x00000000000000000000000000000000000000000000000000000000000007d1"]["logs"][0]
        .clone()
}

fn recorded_chain() -> Value {
    let mut chain: Value = serde_json::from_str(include_str!("../../fixtures/paxeer_blocks.json"))
        .expect("recorded node chain");
    let transaction = chain["canonical"]["blocks"]["2"]["transactions"][2]["hash"]
        .as_str()
        .expect("registry transaction")
        .to_owned();
    let log = &mut chain["receipts"][transaction]["logs"][0];
    let recorded = registry_log();
    for field in ["address", "topics", "data"] {
        log[field] = recorded[field].clone();
    }
    chain
}
fn database_rows() -> BlockscoutRange {
    let mut rows: BlockscoutRange =
        serde_json::from_str(include_str!("../fixtures/backfill_blockscout.json"))
            .expect("recorded database rows");
    let recorded = registry_log();
    let bytes = |value: &Value| unhex(value.as_str().expect("recorded bytes")).expect("hex");
    let log = &mut rows.logs[1];
    log.address_hash = bytes(&recorded["address"]);
    log.first_topic = Some(bytes(&recorded["topics"][0]));
    log.second_topic = Some(bytes(&recorded["topics"][1]));
    log.third_topic = None;
    log.fourth_topic = None;
    log.data = bytes(&recorded["data"]);
    rows
}

struct Node {
    url: String,
    stop: Arc<AtomicBool>,
    thread: Option<thread::JoinHandle<()>>,
    full_blocks: Arc<AtomicU64>,
    receipts: Arc<AtomicU64>,
}
impl Node {
    fn new(tenant: bool, missing_receipts: bool) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("bind node fixture");
        let address = listener.local_addr().expect("node fixture address");
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let full_blocks = Arc::new(AtomicU64::new(0));
        let full_count = Arc::clone(&full_blocks);
        let receipts = Arc::new(AtomicU64::new(0));
        let receipt_count = Arc::clone(&receipts);
        let mut chain = recorded_chain();
        if tenant {
            for block in chain["canonical"]["blocks"]
                .as_object_mut()
                .expect("blocks")
                .values_mut()
            {
                for tx in block["transactions"].as_array_mut().expect("transactions") {
                    tx["chainId"] = json!(format!("0x{:x}", 125_000_001u64));
                }
            }
        }
        let thread = thread::spawn(move || {
            for incoming in listener.incoming() {
                if stopped.load(Ordering::Acquire) {
                    break;
                }
                let mut stream = incoming.expect("node fixture connection");
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("read timeout");
                let body = read_request(&mut stream).expect("JSON-RPC HTTP request");
                let request: Value = serde_json::from_slice(&body).expect("JSON-RPC request");
                let params = &request["params"];
                let result = match request["method"].as_str().expect("method") {
                    "eth_chainId" => chain["chain_id"].clone(),
                    "eth_blockNumber" => chain["canonical"]["head"].clone(),
                    "eth_getBlockByNumber" => {
                        let height = u64::from_str_radix(
                            params[0].as_str().expect("height").trim_start_matches("0x"),
                            16,
                        )
                        .expect("quantity");
                        let mut block = chain["canonical"]["blocks"][height.to_string()].clone();
                        if params[1] == true {
                            full_count.fetch_add(1, Ordering::Relaxed);
                        } else if let Some(txs) = block["transactions"].as_array_mut() {
                            for tx in txs {
                                *tx = tx["hash"].clone();
                            }
                        }
                        block
                    }
                    "eth_getTransactionReceipt" => {
                        receipt_count.fetch_add(1, Ordering::Relaxed);
                        if missing_receipts {
                            Value::Null
                        } else {
                            chain["receipts"][params[0].as_str().expect("hash")].clone()
                        }
                    }
                    other => panic!("unexpected node method {other}"),
                };
                let response =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
                write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}",response.len()).expect("node response");
            }
        });
        Self {
            url: format!("http://{address}"),
            stop,
            thread: Some(thread),
            full_blocks,
            receipts,
        }
    }
    fn ingester(&self) -> Checked<PaxeerIngester> {
        let mut registry = AbiRegistry::load_dir(
            &Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../precompiles"),
        )?;
        for abi in chains_abi(&ChainsConfig::default()) {
            registry.insert(abi);
        }
        Ok(PaxeerIngester::new(
            Endpoint::parse(
                &self.url,
                Security::Plaintext {
                    allow_remote: false,
                },
                Duration::from_secs(5),
            )?,
            None,
            registry,
            FollowPolicy {
                finality_depth: 2,
                max_units_per_step: 16,
            },
            1,
            Some(0xe5),
            AttributeEncoding::Base64,
        ))
    }
}
impl Drop for Node {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.url.trim_start_matches("http://"));
        self.thread
            .take()
            .expect("fixture thread")
            .join()
            .expect("node fixture joined");
    }
}

// The registry inventory is a persisted peer of transaction ingestion; a tenant
// must already be registered before its historical transactions are attributed.
fn register_tenant(store: &Store) -> Checked {
    store.insert_chain(&layerx_indexer::chains::ChainRecord {
        id: 1,
        name: "tenant-history".into(),
        operator: "0xa7dcef9aef26202fce82a7c7d6672afb3a149db2".into(),
        status: "active".into(),
        chain_id: 125_000_001,
        created_height: 0,
        bond: "100000000000000000000".into(),
        gas_allowance: 500000,
        byte_allowance: 65536,
        fee_share_bps: 100,
    })?;
    Ok(())
}

fn tenant_history(store: &Store) -> Checked<(Vec<Value>, Vec<Value>, Value)> {
    Ok((
        store.transfers("pax:1")?,
        store.events("pax:1")?,
        store.chain_summary(1)?,
    ))
}

fn recorded_namespace() -> Checked<Material> {
    let transcript: Value = serde_json::from_str(include_str!(
        "../../../../../../programs/tests/vectors/ics23-market-namespaces-app-hash.json"
    ))?;
    let namespace = &transcript["entries"][0]["namespace"];
    let bytes = |field: &str| unhex(namespace[field].as_str().expect("recorded hex"));
    let header = header(
        1000,
        Digest32::new(bytes("app_hash")?.try_into().expect("app hash"))?,
    );
    Ok(Material {
        commit: signed(&header, &[(1, false), (2, false), (3, false)]),
        header,
        proof: serde_json::to_vec(&json!({
            "membership": {"proof":bytes("proof")?, "multistore":bytes("multistore_proof")?},
            "manifest":bytes("manifest")?
        }))?,
    })
}

fn material_at(state: &[u8], height: u64) -> Checked<Material> {
    let mut material = certify_state(state, PROGRAM, layerx_programs_ai_market::SHARED_STATE_KEY)?;
    material.header.height = height as i64;
    material.commit = signed(&material.header, &[(1, false), (2, false), (3, false)]);
    Ok(material)
}

fn captured(
    state: &[u8],
    material: &Material,
) -> Checked<(
    ReadProof,
    Vec<Vec<u8>>,
    layerx_programs_ai_market::queries::CaptureFacts,
)> {
    let proof = ReadProof {
        chain: chain()?,
        program: program()?,
        native_state_root: Digest32::new(
            material
                .header
                .app_hash
                .as_slice()
                .try_into()
                .expect("app hash"),
        )?,
        observed_sequence: material.header.height as u64,
        execution_height: material.header.height as u64,
        block_hash: Digest32::new(material.header.hash())?,
    };
    let chunks = read_all(state)?;
    let mut bytes = vec![0; MAX_STATE_BYTES];
    let mut capture = StateCapture::new(&mut bytes);
    for chunk in &chunks {
        capture.accept(&proof, chunk)?;
    }
    let (assembled, facts) = capture.finish()?;
    assert_eq!(assembled, state);
    Ok((proof, chunks, facts))
}

fn admit(
    projector: &mut ReputationProjector,
    state: &[u8],
    material: &Material,
) -> Result<Admission, Failure> {
    let (proof, chunks, _) = captured(state, material)?;
    Ok(projector.admit(&FinalizedRead {
        proof,
        chunks: &chunks,
        commit: material.evidence(),
        publication_time_ms: PUBLICATION_TIME_MS,
    })?)
}

pub fn reputation_committed_membership_rollback_conflict_and_reopen() -> Checked {
    let participant = worker(1)?;
    let original = observed(participant, 1, 999)?;
    let (section, records) = frame(vec![original], &[completion(1, 999)?])?;
    let state = state_with(&section)?;
    let mut changed_record = original;
    changed_record.quality = Score::new(900_000)?;
    let (altered, _) = frame(vec![changed_record], &[completion(1, 999)?])?;
    let changed = state_with(&altered)?;
    let committed = material_at(&state, 1000)?;
    let dir = Scratch::new("reputation");
    let mut projector = ReputationProjector::open(&dir.db(), authority(), chain()?, program()?)?;
    assert_eq!(
        admit(&mut projector, &state, &committed)?,
        Admission::Advanced { height: 1000 }
    );
    let request = HistoryRequest {
        market: market()?,
        worker: participant,
        epoch: Some(1),
        minimum_finalized_height: 1000,
    };
    let accepted = projector.read_history(&request)?;
    assert_eq!(accepted.current, original);
    assert_eq!(accepted.history, vec![completion(1, 999)?]);
    assert_eq!(
        accepted.finalized.reputation_root,
        Presence::Present(reputation_root(market()?, 1, &records)?)
    );
    assert_eq!(accepted.finalized.rank, FINALIZED_RANK);
    assert_eq!(
        accepted.finalized.checkpoint_id,
        <[u8; 32]>::from(Sha256::digest(&committed.proof))
    );

    let mut malformed = material_at(&state, 1000)?;
    malformed.proof = b"a nonempty non-membership proof".to_vec();
    let wrong_program = certify_state(
        &state,
        [77; 32],
        layerx_programs_ai_market::SHARED_STATE_KEY,
    )?;
    let wrong_cell = certify_state(&state, PROGRAM, b"unrelated/key")?;
    let mut altered_leaf = material_at(&state, 1000)?;
    let mut document: Value = serde_json::from_slice(&altered_leaf.proof)?;
    let value = document["membership"]["proof"]
        .as_array_mut()
        .expect("leaf bytes");
    let byte = value.last_mut().expect("nonempty leaf");
    *byte = json!(byte.as_u64().expect("byte") ^ 1);
    altered_leaf.proof = serde_json::to_vec(&document)?;
    let recorded = recorded_namespace()?;
    for invalid in [
        &malformed,
        &wrong_program,
        &wrong_cell,
        &altered_leaf,
        &recorded,
    ] {
        // Consensus still verifies: rejection must come from state membership.
        layerx_indexer::ai_market::verify_block_commit(&invalid.evidence(), &authority())?;
        assert!(matches!(
            admit(&mut projector, &state, invalid),
            Err(Failure::Reputation(ReputationError::Commit(
                ProofError::InvalidProof
            )))
        ));
        assert_eq!(projector.read_history(&request)?, accepted);
    }
    assert!(matches!(
        admit(&mut projector, &changed, &committed),
        Err(Failure::Reputation(ReputationError::Commit(
            ProofError::InvalidProof
        )))
    ));
    assert_eq!(projector.read_history(&request)?, accepted);
    let later = material_at(&state, 1001)?;
    assert_eq!(
        admit(&mut projector, &state, &later)?,
        Admission::Advanced { height: 1001 }
    );
    let latest = projector.read_history(&request)?;
    assert!(matches!(
        admit(&mut projector, &state, &committed),
        Err(Failure::Reputation(ReputationError::StaleProof {
            accepted: 1001,
            offered: 1000
        }))
    ));
    drop(projector);
    let mut projector = ReputationProjector::open(&dir.db(), authority(), chain()?, program()?)?;
    assert_eq!(projector.read_history(&request)?, latest);
    assert_eq!(
        admit(&mut projector, &state, &later)?,
        Admission::AlreadyAccepted { height: 1001 }
    );
    assert!(matches!(
        admit(&mut projector, &changed, &later),
        Err(Failure::Reputation(ReputationError::Commit(
            ProofError::InvalidProof
        )))
    ));
    let conflict = material_at(&changed, 1001)?;
    let conflict_error = projector
        .admit(&FinalizedRead {
            proof: captured(&changed, &conflict)?.0,
            chunks: &read_all(&changed)?,
            commit: conflict.evidence(),
            publication_time_ms: PUBLICATION_TIME_MS,
        })
        .expect_err("two authenticated roots at one height must halt");
    assert!(matches!(&conflict_error, ReputationError::Conflict(_)));
    assert_eq!(
        projector.read_history(&request),
        Err(conflict_error.clone())
    );
    drop(projector);
    let mut reopened = ReputationProjector::open(&dir.db(), authority(), chain()?, program()?)?;
    assert_eq!(reopened.read_history(&request), Err(conflict_error.clone()));
    assert!(
        matches!(admit(&mut reopened, &state, &later), Err(Failure::Reputation(error)) if error == conflict_error)
    );
    Ok(())
}

fn observe_state(
    store: &ProjectionStore,
    state: &[u8],
    material: &Material,
) -> Checked<layerx_indexer::ai_market::SnapshotRecord> {
    let (_, _, facts) = captured(state, material)?;
    Ok(store.observe(&snapshot_observation(state, facts)?)?)
}

// The archive peer reads its response from disk on every request. Dropping it
// joins the server; a new listener after reopen cannot retain the old buffer.
struct Archive {
    endpoint: Endpoint,
    address: std::net::SocketAddr,
    stop: Arc<AtomicBool>,
    thread: Option<thread::JoinHandle<()>>,
}
impl Archive {
    fn open(path: PathBuf, digest: StateDigest) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("archive bind");
        let address = listener.local_addr().expect("archive address");
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let expected = format!("GET /{} ", layerx_indexer::codec::hex(digest.as_bytes()));
        let thread = thread::spawn(move || {
            for incoming in listener.incoming() {
                if stopped.load(Ordering::Acquire) {
                    break;
                }
                let mut stream = incoming.expect("archive connection");
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("archive timeout");
                let mut request = Vec::new();
                loop {
                    let mut bytes = [0; 1024];
                    let n = stream.read(&mut bytes).expect("archive request");
                    assert!(n > 0);
                    request.extend_from_slice(&bytes[..n]);
                    if request.windows(4).any(|w| w == b"\r\n\r\n") {
                        break;
                    }
                }
                assert!(String::from_utf8_lossy(&request).starts_with(&expected));
                let bytes = fs::read(&path).expect("durable archive bytes");
                write!(
                    stream,
                    "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    bytes.len()
                )
                .expect("archive headers");
                stream.write_all(&bytes).expect("archive body");
            }
        });
        Self {
            endpoint: Endpoint::parse(
                &format!("http://{address}"),
                Security::Plaintext {
                    allow_remote: false,
                },
                Duration::from_secs(5),
            )
            .expect("archive endpoint"),
            address,
            stop,
            thread: Some(thread),
        }
    }
}
impl Drop for Archive {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        self.thread
            .take()
            .expect("archive thread")
            .join()
            .expect("archive stopped");
    }
}

pub fn snapshot_membership_archive_transport_and_restart() -> Checked {
    use layerx_indexer::ai_market::{ProjectionState, Viewer};
    let dir = Scratch::new("snapshot-archive");
    let state = created_state()?;
    let old_material = material_at(&state, 1000)?;
    let store = ProjectionStore::open(&dir.db())?;
    let provisional = observe_state(&store, &state, &old_material)?;
    assert_eq!(provisional.rank, 0);
    assert!(store.pending_events(&market()?, 16)?.is_empty());
    let mut bad_signature = old_material.commit.clone();
    bad_signature.signatures[0]
        .signature
        .as_mut()
        .expect("signature")[0] ^= 1;
    assert_eq!(
        store.promote(
            &provisional.snapshot_id,
            &BlockCommitProof {
                header: &old_material.header,
                commit: &bad_signature,
                store_proof: &old_material.proof
            },
            &authority(),
            PUBLICATION_TIME_MS
        ),
        Err(ViewError::FinalityUnavailable)
    );
    let insufficient = signed(&old_material.header, &[(1, false), (2, false)]);
    assert_eq!(
        store.promote(
            &provisional.snapshot_id,
            &BlockCommitProof {
                header: &old_material.header,
                commit: &insufficient,
                store_proof: &old_material.proof
            },
            &authority(),
            PUBLICATION_TIME_MS
        ),
        Err(ViewError::FinalityUnavailable)
    );
    let old = store.promote(
        &provisional.snapshot_id,
        &old_material.evidence(),
        &authority(),
        PUBLICATION_TIME_MS,
    )?;
    assert_eq!(old.projection, ProjectionState::FinalizedPublishable);
    assert_eq!(old.rank, FINALIZED_RANK);
    let old_binding = old.binding.clone().expect("finalized binding");
    let pending = store.pending_events(&market()?, 16)?;
    assert_eq!(pending.len(), 1);
    assert_eq!(pending[0].binding, old_binding);
    assert_eq!(pending[0].snapshot_id, old.snapshot_id);

    let (section, _) = frame(vec![observed(worker(1)?, 1, 999)?], &[completion(1, 999)?])?;
    let changed = state_with(&section)?;
    let bad_store = ProjectionStore::open_in_memory()?;
    let changed_record = observe_state(&bad_store, &changed, &old_material)?;
    assert_eq!(
        bad_store.promote(
            &changed_record.snapshot_id,
            &old_material.evidence(),
            &authority(),
            PUBLICATION_TIME_MS
        ),
        Err(ViewError::FinalityUnavailable)
    );
    assert_eq!(
        bad_store
            .snapshot(&changed_record.snapshot_id)?
            .expect("provisional")
            .rank,
        0
    );
    assert!(bad_store.pending_events(&market()?, 16)?.is_empty());
    let mut altered_leaf = material_at(&state, 1000)?;
    let mut document: Value = serde_json::from_slice(&altered_leaf.proof)?;
    let last = document["manifest"]
        .as_array_mut()
        .expect("manifest")
        .last_mut()
        .expect("manifest byte");
    *last = json!(last.as_u64().expect("byte") ^ 1);
    altered_leaf.proof = serde_json::to_vec(&document)?;
    let wrong_key = certify_state(&state, PROGRAM, b"unrelated/state")?;
    let wrong_program = certify_state(
        &state,
        [77; 32],
        layerx_programs_ai_market::SHARED_STATE_KEY,
    )?;
    let recorded = recorded_namespace()?;
    for invalid in [&altered_leaf, &wrong_key, &wrong_program, &recorded] {
        layerx_indexer::ai_market::verify_block_commit(&invalid.evidence(), &authority())?;
        let store = ProjectionStore::open_in_memory()?;
        let observed = observe_state(&store, &state, invalid)?;
        assert_eq!(
            store.promote(
                &observed.snapshot_id,
                &invalid.evidence(),
                &authority(),
                PUBLICATION_TIME_MS
            ),
            Err(ViewError::FinalityUnavailable)
        );
        assert_eq!(
            store
                .snapshot(&observed.snapshot_id)?
                .expect("unpromoted")
                .rank,
            0
        );
        assert!(store.pending_events(&market()?, 16)?.is_empty());
    }
    let next_material = material_at(&changed, 1001)?;
    let next = observe_state(&store, &changed, &next_material)?;
    let next = store.promote(
        &next.snapshot_id,
        &next_material.evidence(),
        &authority(),
        PUBLICATION_TIME_MS,
    )?;
    assert_eq!(next.rank, FINALIZED_RANK);
    let archive_path = dir.0.join("state.archive");
    fs::write(&archive_path, &state)?;
    let archive = Archive::open(archive_path.clone(), old.state_digest);
    let path = format!(
        "/{}",
        layerx_indexer::codec::hex(old.state_digest.as_bytes())
    );
    let (status, before) = archive.endpoint.get(&path)?;
    assert_eq!(status, 200);
    assert_eq!(before, state);
    store.archive(&old.snapshot_id, PUBLICATION_TIME_MS + 1)?;
    assert_eq!(
        store
            .snapshot(&old.snapshot_id)?
            .expect("archived")
            .projection,
        ProjectionState::Archived
    );
    drop(archive);
    drop(store);
    drop(before);
    drop(state);
    // Restart both sides before retrieving any recovery bytes.
    let store = ProjectionStore::open(&dir.db())?;
    let archived = store
        .snapshot(&old.snapshot_id)?
        .expect("recovered archive header");
    assert_eq!(archived.binding, Some(old_binding));
    assert!(!archived.retained);
    let archive = Archive::open(archive_path, old.state_digest);
    let (status, recovered) = archive.endpoint.get(&path)?;
    assert_eq!(status, 200);
    assert_eq!(codec::state_digest(&recovered)?, old.state_digest);
    let mut altered = recovered.clone();
    *altered.last_mut().expect("archive bytes") ^= 1;
    assert_eq!(
        store.restore_archived(&old.snapshot_id, &altered),
        Err(ViewError::IntegrityFailure)
    );
    assert_eq!(store.snapshot(&old.snapshot_id)?, Some(archived));
    store.restore_archived(&old.snapshot_id, &recovered)?;
    let viewer = Viewer::for_principal("repair-archive")?;
    let restored = store.market_view(&viewer, &market()?, Some(&old.snapshot_id), None)?;
    assert_eq!(
        restored.binding,
        layerx_programs_ai_market::queries::SnapshotBinding::decode(
            &old.binding.expect("old binding")
        )?
    );
    assert_eq!(store.pending_events(&market()?, 16)?.len(), 2);
    drop(archive);
    drop(store);
    let reopened = ProjectionStore::open(&dir.db())?;
    assert_eq!(
        reopened.market_view(&viewer, &market()?, Some(&old.snapshot_id), None)?,
        restored
    );
    Ok(())
}

pub fn binding_consumes_authenticated_store_proof_and_refuses_wrong_keys() -> Checked {
    use layerx_programs_ai_market::queries::{bind_snapshot, SnapshotBinding, BINDING_MAX_BYTES};
    let state = created_state()?;
    let material = material_at(&state, 1000)?;
    let authenticated = verify_program_state(
        &material.evidence(),
        &authority(),
        chain()?,
        program()?,
        &state,
    )?;
    assert_eq!(authenticated.state(), state);
    let evidence = authenticated
        .commit()
        .finality_evidence(captured(&state, &material)?.0.native_state_root)?;
    let facts = captured(&state, &material)?.2;
    let binding = bind_snapshot(
        authenticated.state(),
        &facts,
        &evidence,
        PUBLICATION_TIME_MS,
    )?;
    binding.require_finalized()?;
    assert_eq!(binding.block_height, 1000);
    assert_eq!(binding.block_hash.bytes(), material.header.hash());
    assert_eq!(
        binding.app_hash.as_bytes().as_slice(),
        material.header.app_hash
    );
    assert_eq!(
        binding.store_proof_digest.bytes(),
        <[u8; 32]>::from(Sha256::digest(&material.proof))
    );
    let mut encoded = [0; BINDING_MAX_BYTES];
    let n = binding.encode(&mut encoded)?;
    assert_eq!(n, BINDING_MAX_BYTES - 8 - 32);
    assert_eq!(SnapshotBinding::decode(&encoded[..n])?, binding);
    let store = ProjectionStore::open_in_memory()?;
    let observed = observe_state(&store, &state, &material)?;
    let published = store.promote(
        &observed.snapshot_id,
        &material.evidence(),
        &authority(),
        PUBLICATION_TIME_MS,
    )?;
    assert_eq!(published.binding, Some(encoded[..n].to_vec()));
    let events = store.pending_events(&market()?, 16)?;
    assert_eq!(events.len(), 1);
    assert_eq!(SnapshotBinding::decode(&events[0].binding)?, binding);
    assert_eq!(events[0].snapshot_id, binding.snapshot_id()?);
    let mut malformed = material_at(&state, 1000)?;
    malformed.proof = b"snapshot-binding-v2 store proof bytes".to_vec();
    let wrong_program = certify_state(
        &state,
        [77; 32],
        layerx_programs_ai_market::SHARED_STATE_KEY,
    )?;
    let wrong_key = certify_state(&state, PROGRAM, b"not/the/shared/state")?;
    let recorded = recorded_namespace()?;
    for invalid in [&malformed, &wrong_program, &wrong_key, &recorded] {
        layerx_indexer::ai_market::verify_block_commit(&invalid.evidence(), &authority())?;
        assert!(matches!(
            verify_program_state(
                &invalid.evidence(),
                &authority(),
                chain()?,
                program()?,
                &state
            ),
            Err(ProofError::InvalidProof)
        ));
        let store = ProjectionStore::open_in_memory()?;
        let provisional = observe_state(&store, &state, invalid)?;
        assert_eq!(
            store.promote(
                &provisional.snapshot_id,
                &invalid.evidence(),
                &authority(),
                PUBLICATION_TIME_MS
            ),
            Err(ViewError::FinalityUnavailable)
        );
        assert!(store.pending_events(&market()?, 16)?.is_empty());
    }
    Ok(())
}

pub fn tenant_backfill_preserves_chain_id_history_events_and_summary() -> Checked {
    let server = Node::new(true, false);
    let node = server.ingester()?;
    let live = Store::open_in_memory()?;
    register_tenant(&live)?;
    assert_eq!(
        node.step(&live)?,
        StepOutcome::Advanced {
            units: 3,
            position: 3
        }
    );
    let expected = tenant_history(&live)?;
    assert!(!expected.0.is_empty());
    assert_eq!(expected.1.len(), 1);
    assert_eq!(expected.1[0]["name"], "ChainStatusChanged");
    assert_eq!(expected.1[0]["decoded"]["args"]["id"], "1");
    assert_eq!(expected.2["tx_count"], 5);
    assert!(expected.2["gas_used"].as_u64().expect("gas") > 0);
    assert!(live.transfers("paxeer")?.is_empty());
    let mut rows = database_rows();
    for row in &mut rows.transactions {
        row.chain_id = Some("125000001".into());
    }
    let converted = rpc_blocks(&rows)?;
    let chain = {
        let mut chain = recorded_chain();
        for block in chain["canonical"]["blocks"]
            .as_object_mut()
            .expect("blocks")
            .values_mut()
        {
            for tx in block["transactions"].as_array_mut().expect("transactions") {
                tx["chainId"] = json!("0x7735941");
            }
        }
        chain
    };
    for (height, block) in &converted {
        let live_txs = chain["canonical"]["blocks"][height.to_string()]["transactions"]
            .as_array()
            .expect("live transactions");
        let db_txs = block.block["transactions"]
            .as_array()
            .expect("database transactions");
        assert_eq!(db_txs.len(), live_txs.len());
        for (database, live) in db_txs.iter().zip(live_txs) {
            for field in [
                "hash",
                "blockHash",
                "blockNumber",
                "transactionIndex",
                "from",
                "to",
                "value",
                "input",
                "chainId",
            ] {
                assert_eq!(database[field], live[field], "transaction field {field}");
            }
            assert_eq!(database["chainId"], "0x7735941");
        }
    }
    let dir = Scratch::new("tenant-cutover");
    let store = Store::open(&dir.db())?;
    register_tenant(&store)?;
    let mut asked = Vec::new();
    assert_eq!(
        Backfill::new(&node, 2, 1).run(&store, |from, to| {
            asked.push((from, to));
            Ok(within(&rows, from, to, &[]))
        })?,
        2
    );
    assert_eq!(asked, vec![(1, 1), (2, 2)]);
    assert_eq!(store.cursor("paxeer")?.expect("cutover").position, 2);
    assert_eq!(store.chain_summary(1)?["tx_count"], 4);
    let at_cutover = tenant_history(&store)?;
    drop(store);
    drop(node);
    drop(server);
    let server = Node::new(true, false);
    let node = server.ingester()?;
    let store = Store::open(&dir.db())?;
    assert_eq!(tenant_history(&store)?, at_cutover);
    assert_eq!(
        store.cursor("paxeer")?.expect("recovered cursor").position,
        2
    );
    assert_eq!(
        node.step(&store)?,
        StepOutcome::Advanced {
            units: 1,
            position: 3
        }
    );
    assert_eq!(tenant_history(&store)?, expected);
    assert!(store.transfers("paxeer")?.is_empty());
    assert_eq!(store.chain(1)?.expect("tenant").status, "paused");
    drop(store);
    let reopened = Store::open(&dir.db())?;
    assert_eq!(tenant_history(&reopened)?, expected);
    assert_eq!(reopened.cursor("paxeer")?.expect("live cursor").position, 3);
    Ok(())
}

pub fn daemon_observation_order_allows_disabled_index_evm_fallback() -> Checked {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let address = listener.local_addr()?;
    let stopped = Arc::new(AtomicBool::new(false));
    let stop = Arc::clone(&stopped);
    let calls = Arc::new(std::sync::Mutex::new(Vec::<String>::new()));
    let requests = Arc::clone(&calls);
    let chain = recorded_chain();
    let server = thread::spawn(move || {
        for incoming in listener.incoming() {
            if stopped.load(Ordering::Acquire) {
                break;
            }
            let mut stream = incoming.expect("node connection");
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .expect("timeout");
            let body = read_request(&mut stream).expect("request");
            let request: Value = serde_json::from_slice(&body).expect("JSON-RPC request");
            let method = request["method"].as_str().expect("method");
            requests.lock().expect("calls").push(method.into());
            let result = match method {
                "status" => {
                    json!({"node_info":{"other":{"tx_index":"off"}}, "sync_info":{"latest_block_height":"3", "catching_up":false}})
                }
                "eth_chainId" => chain["chain_id"].clone(),
                "eth_blockNumber" => chain["canonical"]["head"].clone(),
                "eth_getBlockByNumber" => {
                    let height = u64::from_str_radix(
                        request["params"][0]
                            .as_str()
                            .expect("height")
                            .trim_start_matches("0x"),
                        16,
                    )
                    .expect("quantity");
                    chain["canonical"]["blocks"][height.to_string()].clone()
                }
                "eth_getTransactionReceipt" => {
                    chain["receipts"][request["params"][0].as_str().expect("hash")].clone()
                }
                "tx_search" => {
                    let response = json!({"jsonrpc":"2.0", "id":request["id"], "error":{"code":-32603,"message":"Internal error","data":layerx_indexer::paxeer::TX_INDEX_DISABLED}}).to_string();
                    write!(stream,"HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}",response.len()).expect("error response");
                    continue;
                }
                _ => panic!("unexpected node method {method}"),
            };
            let response =
                json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
            write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}",
                response.len()
            )
            .expect("response");
        }
    });
    let endpoint = Endpoint::parse(
        &format!("http://{address}"),
        Security::Plaintext {
            allow_remote: false,
        },
        Duration::from_secs(5),
    )?;
    let mut registry = AbiRegistry::load_dir(
        &Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../precompiles"),
    )?;
    for abi in chains_abi(&ChainsConfig::default()) {
        registry.insert(abi);
    }
    let node = PaxeerIngester::new(
        endpoint.clone(),
        Some(endpoint),
        registry,
        FollowPolicy {
            finality_depth: 2,
            max_units_per_step: 16,
        },
        1,
        Some(0xe5),
        AttributeEncoding::Base64,
    );
    let dir = Scratch::new("disabled-index");
    let store = Store::open(&dir.db())?;
    register_tenant(&store)?;
    store.start_source("paxeer")?;
    store.start_source("comet")?;
    let outcome = node.daemon_step(&store);
    stop.store(true, Ordering::Release);
    let _ = TcpStream::connect(address);
    server.join().expect("node stopped");
    assert_eq!(
        calls.lock().expect("calls").first().map(String::as_str),
        Some("status")
    );
    assert_eq!(
        &calls.lock().expect("calls")[..4],
        ["status", "eth_chainId", "eth_blockNumber", "tx_search"]
    );
    assert_eq!(
        outcome?,
        StepOutcome::Advanced {
            units: 3,
            position: 3
        }
    );
    assert_eq!(
        store.cursor("paxeer")?.expect("fallback cursor").position,
        3
    );
    assert!(!store.transfers("paxeer")?.is_empty());
    assert_eq!(store.chain(1)?.expect("registered tenant").status, "paused");
    let comet = store
        .source_observation("comet")?
        .expect("Comet observation");
    assert_eq!(comet.source_head, Some(3));
    assert_eq!(comet.last_error.as_deref(), Some("tx_index_disabled"));
    assert!(comet.last_success_at.is_none());
    let readiness = layerx_indexer::api::Readiness {
        sources: vec!["comet"],
        stall_after_secs: 30,
        started_at: 0,
        explorer: None,
    };
    let document = layerx_indexer::api::readiness_document(&store, &readiness)?;
    assert_eq!(document.status, "unavailable");
    assert_ne!(document.sources[0].state, "ready");
    let transfers = store.transfers("paxeer")?;
    let events = store.events("paxeer")?;
    drop(store);
    let reopened = Store::open(&dir.db())?;
    assert_eq!(
        reopened
            .cursor("paxeer")?
            .expect("recovered cursor")
            .position,
        3
    );
    assert_eq!(reopened.transfers("paxeer")?, transfers);
    assert_eq!(reopened.events("paxeer")?, events);
    assert_eq!(
        reopened.chain(1)?.expect("recovered tenant").status,
        "paused"
    );
    let comet = reopened
        .source_observation("comet")?
        .expect("recovered Comet observation");
    assert_eq!(comet.last_error.as_deref(), Some("tx_index_disabled"));
    assert!(comet.last_success_at.is_none());
    Ok(())
}
