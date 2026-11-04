//! Repair 22.33.1444: real producer codecs, native proofs, loopback transports and durable stores.
//! Finding/test mappings, baseline constraints and out-of-scope wiring requests are recorded
//! in src/repair_22_33_1444.kvx. No baseline test ran: the task's mandatory gate order permits
//! verification only after implementation, through hf-done.
use layerx_indexer::abi::{keccak, AbiType, LoadedAbi};
use layerx_indexer::ai_market::{
    CursorKeyring, PageRequest, ProjectionState, QueryLimiter, RateConfig, Viewer,
    AUTHORITY_FRESHNESS_HEIGHTS, CACHED_SNAPSHOTS, QUERY_IDLE_MS, QUERY_PRINCIPALS_MAX,
};
use layerx_indexer::ai_runtime::{encode_committed_read, AiRuntime, RuntimeConfig};
use layerx_indexer::ai_views::AiViews;
use layerx_indexer::codec::{hex, hex0x};
use layerx_programs_ai_market::types::{MetadataDigest, PublicKey32};
use layerx_programs_ai_market::workers::{
    WorkerCurrent, WorkerState, WorkerTable, WORKER_TABLE_MAX_BYTES,
};

use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
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
    BlockCommitProof, FinalityAuthority, Observation as SnapshotObservation, ProjectionStore,
    ReadVerification, ViewError,
};
use layerx_indexer::ai_reputation::{HistoryRequest, ReputationError};
use layerx_indexer::{
    abi::AbiRegistry,
    backfill::Backfill,
    blockscout::{rpc_blocks, BlockscoutRange},
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
    reputation_codec::{decode_section, encode_current, encode_history, encode_section},
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
enum Failure {
    Application(ApplicationError),
    Query(QueryError),
    Reputation(ReputationError),
    Commit(ProofError),
    Io(std::io::Error),
    View(ViewError),
    Index(IndexError),
    Json(serde_json::Error),
    Sql(rusqlite::Error),
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
            Self::Sql(e) => write!(f, "SQL: {e}"),
            Self::Unexpected(e) => write!(f, "unexpected: {e}"),
        }
    }
}
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
type Checked<T = ()> = Result<T, Failure>;
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
    let (proof, store_root) = leaf(&key, &head, &[0, 2, 2]);
    let (outer, app_hash) = leaf(b"lxphost", &store_root, &[0]);
    let mut multistore = Vec::new();
    encode_length_delimited(1, &outer, &mut multistore);
    (serde_json::to_vec(&json!({"membership": {"proof": proof, "multistore": multistore}, "manifest": manifest})).expect("fixture JSON"), app_hash)
}

fn certify_at(state: &[u8], height: u64) -> Checked<Material> {
    let program = PROGRAM;
    let cell = layerx_programs_ai_market::SHARED_STATE_KEY;
    let (proof, root) = native_membership(state, program, cell);
    let header = header(height, Digest32::new(root)?);
    let commit = signed(&header, &[(1, false), (2, false), (3, false), (4, false)]);
    Ok(Material {
        header,
        commit,
        proof,
    })
}

fn capture_state(
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
                .expect("root"),
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

struct Scratch(PathBuf);
impl Scratch {
    fn new(name: &str) -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-1444-{}-{}-{name}",
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

impl From<rusqlite::Error> for Failure {
    fn from(e: rusqlite::Error) -> Self {
        Self::Sql(e)
    }
}
fn worker_row(slot: u8, nonce: u8, state: WorkerState) -> Checked<WorkerCurrent> {
    Ok(WorkerCurrent {
        worker: derive_worker(market()?, principal([20; 32])?, [nonce; 32])?,
        owner: principal([20; 32])?,
        delegate: PublicKey32([slot + 50; 32]),
        metadata: MetadataDigest::new([30; 32])?,
        generation: 2,
        key_version: 2,
        metadata_revision: 4,
        valid_from: 200,
        expiry: 400,
        revocation_sequence: 0,
        effective_epoch: 1,
        last_sequence: 0,
        last_request_id: [0; 32],
        last_request_digest: [0; 32],
        last_result_digest: [0; 32],
        state,
        slot,
        last_metadata_height: 200,
    })
}

/// Created market plus an F02 worker table; even slots available, odd slots retired. Seat
/// `slot` is held by the worker derived from nonce `slot + nonce_base`.
fn market_with_workers(count: u8, nonce_base: u8) -> Checked<Vec<u8>> {
    let base = created_state()?;
    let shared = state::decode_shared_state(&base)?;
    let mut table = WorkerTable::new();
    for slot in 0..count {
        let status = if slot % 2 == 0 {
            WorkerState::Available
        } else {
            WorkerState::Retired
        };
        table.insert(&worker_row(slot, slot + nonce_base, status)?)?;
    }
    let mut section = vec![0; WORKER_TABLE_MAX_BYTES];
    let n = table.encode(&mut section)?;
    let identity = section
        .get(..n)
        .ok_or(Failure::Unexpected("worker table"))?;
    encode_shared(&shared.replace_section(Section::IdentityRoster, identity)?)
}

#[test]
fn abi_bytes24_function_topics_and_nested_arrays() -> Checked {
    let bytes = AbiType::from_param(&json!({"type":"bytes24"}))?;
    let function = AbiType::from_param(&json!({"type":"function"}))?;
    assert_ne!(bytes, function);
    assert_eq!(bytes.canonical(), "bytes24");
    assert_eq!(function.canonical(), "function");
    let mut registry = AbiRegistry::default();
    for kind in ["bytes24", "function"] {
        let events = AbiRegistry::parse_events(
            "Producer",
            &json!([
                {"type":"event","name":"Value","inputs":[{"name":"value","type":kind,"indexed":true}]},
                {"type":"event","name":"Nested","inputs":[{"name":"values","type":format!("{kind}[2][]"),"indexed":false}]}
            ]),
        )?;
        assert_eq!(events[0].signature, format!("Value({kind})"));
        assert_eq!(events[0].topic, keccak(format!("Value({kind})").as_bytes()));
        assert_eq!(events[1].signature, format!("Nested({kind}[2][])"));
        registry.insert(LoadedAbi {
            contract: "Producer".into(),
            address: None,
            events,
        });
    }
    assert_ne!(keccak(b"Value(bytes24)"), keccak(b"Value(function)"));
    let mut word = [0u8; 32];
    word[..24].fill(0x42);
    for kind in ["bytes24", "function"] {
        let topic = keccak(format!("Value({kind})").as_bytes());
        let (_, decoded) = registry
            .decode_log([1; 20], &[topic, word], &[])?
            .expect("canonical topic decoded");
        assert_eq!(decoded.args["value"], hex0x(&[0x42; 24]));
        let mut body = vec![0u8; 64];
        body[31] = 32;
        body[63] = 1;
        body.extend(word);
        body.extend(word);
        let (_, decoded) = registry
            .decode_log(
                [1; 20],
                &[keccak(format!("Nested({kind}[2][])").as_bytes())],
                &body,
            )?
            .expect("nested producer event");
        assert_eq!(
            decoded.args["values"],
            json!([[hex0x(&[0x42; 24]), hex0x(&[0x42; 24])]])
        );
        let tuple = AbiType::from_param(&json!({"type":"tuple[][2]", "components":[
            {"type":"bytes24[3]"}, {"type":"function[]"}
        ]}))?;
        assert_eq!(tuple.canonical(), "(bytes24[3],function[])[][2]");
        let mut bad = word;
        bad[31] = 1;
        assert!(registry.decode_log([1; 20], &[topic, bad], &[]).is_err());
    }
    Ok(())
}

#[test]
fn limiter_bounds_churn_preserves_permits_and_live_quotas() -> Checked {
    let config = RateConfig {
        per_minute: 1,
        burst: 2,
        concurrent: 2,
    };
    let limiter = QueryLimiter::with_capacity(Some(config), 2);
    let a = Viewer::for_principal("principal-a")?;
    let b = Viewer::for_principal("principal-b")?;
    let c = Viewer::for_principal("principal-c")?;
    let held = limiter.admit(&a, 0)?;
    drop(limiter.admit(&a, 0)?);
    drop(limiter.admit(&b, 0)?);
    assert!(matches!(limiter.admit(&a, 1), Err(ViewError::RateLimited)));
    assert!(matches!(
        limiter.admit(&c, 1),
        Err(ViewError::CapacityExceeded)
    ));
    assert_eq!(limiter.retained_principals()?, 2);
    // A long running permit is never removed, even after every idle bucket expires.
    let c_permit = limiter.admit(&c, QUERY_IDLE_MS + 1)?;
    let other_a = limiter.admit(&a, QUERY_IDLE_MS + 1)?;
    assert!(matches!(
        limiter.admit(&a, QUERY_IDLE_MS + 1),
        Err(ViewError::RateLimited)
    ));
    drop(other_a);
    drop(held);
    drop(c_permit);
    // The last admissions were at QUERY_IDLE_MS + 1: at twice the idle
    // interval these buckets are still one millisecond short of expiry.
    assert!(matches!(
        limiter.admit(&b, 2 * QUERY_IDLE_MS),
        Err(ViewError::CapacityExceeded)
    ));
    for n in 2..514 {
        let viewer = Viewer::for_principal(&format!("churn-{n}"))?;
        drop(limiter.admit(&viewer, n * QUERY_IDLE_MS + 1)?);
        assert!(limiter.retained_principals()? <= 2);
    }
    // A five minute idle timeout cannot reset a quota with a longer refill period.
    let slow = QueryLimiter::with_capacity(
        Some(RateConfig {
            per_minute: 1,
            burst: 100,
            concurrent: 1,
        }),
        1,
    );
    for _ in 0..100 {
        drop(slow.admit(&a, 0)?);
    }
    assert!(matches!(
        slow.admit(&b, QUERY_IDLE_MS),
        Err(ViewError::CapacityExceeded)
    ));
    assert!(matches!(slow.admit(&a, 0), Err(ViewError::RateLimited)));
    drop(slow.admit(&b, 100 * 60_000)?);
    assert_eq!(slow.retained_principals()?, 1);
    assert!(matches!(
        QueryLimiter::new(None).admit(&a, 0),
        Err(ViewError::EndpointDisabled)
    ));
    let hard = QueryLimiter::with_capacity(Some(config), usize::MAX);
    for n in 0..QUERY_PRINCIPALS_MAX {
        drop(hard.admit(&Viewer::for_principal(&format!("hard-{n}"))?, 0)?);
    }
    assert!(matches!(
        hard.admit(&a, 0),
        Err(ViewError::CapacityExceeded)
    ));
    assert_eq!(hard.retained_principals()?, QUERY_PRINCIPALS_MAX);
    Ok(())
}

fn publish(
    store: &ProjectionStore,
    state: &[u8],
    height: u64,
) -> Checked<layerx_indexer::ai_market::SnapshotRecord> {
    let material = certify_at(state, height)?;
    let (_, _, facts) = capture_state(state, &material)?;
    let record = store.observe(&SnapshotObservation {
        state,
        facts,
        read: ReadVerification::SequencerSigned,
        source_activity: Digest32::new(material.header.hash())?,
        observed_at_ms: height,
    })?;
    Ok(store.promote(
        &record.snapshot_id,
        &material.evidence(),
        &authority(),
        PUBLICATION_TIME_MS,
    )?)
}

#[test]
fn snapshot_archive_recovers_without_caller_bytes_after_reopen() -> Checked {
    let dir = Scratch::new("archive");
    let state = market_with_workers(3, 1)?;
    let store = ProjectionStore::open(&dir.db())?;
    let first = publish(&store, &state, 1000)?;
    let next = publish(&store, &state, 1001)?;
    let rows = store.committed_rows(&first.snapshot_id)?;
    let viewer = Viewer::for_principal("archive-reader")?;
    let request = PageRequest::parse(
        &hex(market()?.as_bytes()),
        Some(&format!(
            "snapshot={}&limit=1",
            hex(first.snapshot_id.as_bytes())
        )),
    )?;
    let page = store.participants(&viewer, &CursorKeyring::new(1, [42; 32]), &request, 1)?;
    assert!(page.cursor.is_some());
    assert_eq!(
        store.archive(&first.snapshot_id, 2),
        Err(ViewError::CapacityExceeded)
    );
    assert_eq!(
        store.archive(&next.snapshot_id, u64::MAX),
        Err(ViewError::ProjectionUnavailable)
    );
    let after_lease = layerx_programs_ai_market::queries::CURSOR_LIFETIME_MS + 2;
    store.archive(&first.snapshot_id, after_lease)?;
    assert_eq!(
        store
            .snapshot(&first.snapshot_id)?
            .expect("saved header")
            .projection,
        ProjectionState::Archived
    );
    assert!(matches!(
        store.market_view(&viewer, &market()?, Some(&first.snapshot_id), None),
        Err(ViewError::SnapshotPruned { .. })
    ));
    drop(store);
    drop(state);
    let reopened = ProjectionStore::open(&dir.db())?;
    reopened.recover_archived(&first.snapshot_id, after_lease)?;
    let recovered =
        reopened.market_view(&viewer, &market()?, Some(&first.snapshot_id), Some(1000))?;
    assert_eq!(recovered.snapshot_id, first.snapshot_id);
    let mut encoded_binding = vec![0; layerx_programs_ai_market::queries::BINDING_MAX_BYTES];
    let n = recovered.binding.encode(&mut encoded_binding)?;
    encoded_binding.truncate(n);
    assert_eq!(Some(encoded_binding), first.binding);
    assert_eq!(reopened.committed_rows(&first.snapshot_id)?, rows);
    assert_eq!(
        reopened
            .stream(&market()?)?
            .expect("stream")
            .finalized_snapshot,
        Some(next.snapshot_id)
    );
    let recovered_page = reopened.participants(
        &viewer,
        &CursorKeyring::new(1, [42; 32]),
        &request,
        after_lease,
    )?;
    assert_eq!(recovered_page.rows, page.rows);
    Ok(())
}

#[test]
fn automatic_snapshot_eviction_persists_archive_and_recovers() -> Checked {
    let dir = Scratch::new("automatic-archive");
    let state = created_state()?;
    let store = ProjectionStore::open(&dir.db())?;
    let first = publish(&store, &state, 1000)?;
    for n in 1..=CACHED_SNAPSHOTS {
        publish(&store, &state, 1000 + n as u64)?;
    }
    assert_eq!(
        store
            .snapshot(&first.snapshot_id)?
            .expect("evicted header")
            .projection,
        ProjectionState::Archived
    );
    let sql = rusqlite::Connection::open(dir.db())?;
    assert_eq!(
        sql.query_row(
            "SELECT COUNT(*) FROM ai_snapshot WHERE state IS NOT NULL",
            [],
            |r| r.get::<_, usize>(0)
        )?,
        CACHED_SNAPSHOTS
    );
    assert_eq!(
        sql.query_row(
            "SELECT COUNT(*) FROM ai_snapshot_archive WHERE snapshot_id=?1",
            [first.snapshot_id.as_bytes().as_slice()],
            |r| r.get::<_, usize>(0)
        )?,
        1
    );
    drop(sql);
    drop(store);
    drop(state);
    let reopened = ProjectionStore::open(&dir.db())?;
    reopened.recover_archived(&first.snapshot_id, 1_000_000)?;
    assert_eq!(
        reopened
            .snapshot(&first.snapshot_id)?
            .expect("restored")
            .projection,
        ProjectionState::FinalizedPublishable
    );
    assert_eq!(
        reopened
            .market_view(
                &Viewer::for_principal("recovery")?,
                &market()?,
                Some(&first.snapshot_id),
                None
            )?
            .snapshot_id,
        first.snapshot_id
    );
    let sql = rusqlite::Connection::open(dir.db())?;
    assert_eq!(
        sql.query_row(
            "SELECT COUNT(*) FROM ai_snapshot WHERE state IS NOT NULL",
            [],
            |r| r.get::<_, usize>(0)
        )?,
        CACHED_SNAPSHOTS
    );
    Ok(())
}

#[test]
fn pre_archive_discarded_snapshot_is_explicitly_pruned_on_upgrade() -> Checked {
    let dir = Scratch::new("old-archive");
    let store = ProjectionStore::open(&dir.db())?;
    let state = created_state()?;
    let first = publish(&store, &state, 1000)?;
    publish(&store, &state, 1001)?;
    drop(store);
    let sql = rusqlite::Connection::open(dir.db())?;
    sql.execute(
        "UPDATE ai_snapshot SET state=NULL,projection=4 WHERE snapshot_id=?1",
        [first.snapshot_id.as_bytes().as_slice()],
    )?;
    drop(sql);
    let upgraded = ProjectionStore::open(&dir.db())?;
    assert_eq!(
        upgraded
            .snapshot(&first.snapshot_id)?
            .expect("legacy header")
            .projection,
        ProjectionState::Pruned
    );
    assert_eq!(
        upgraded.recover_archived(&first.snapshot_id, 0),
        Err(ViewError::ProjectionUnavailable)
    );
    assert!(matches!(
        upgraded.market_view(
            &Viewer::for_principal("reader")?,
            &market()?,
            Some(&first.snapshot_id),
            None
        ),
        Err(ViewError::SnapshotPruned { .. })
    ));
    Ok(())
}

#[test]
fn authority_regression_refuses_freshness_and_signing_admission() -> Checked {
    let store = ProjectionStore::open_in_memory()?;
    publish(&store, &created_state()?, 1000)?;
    let viewer = Viewer::for_principal("signer")?;
    for height in [0, 1, 999] {
        assert_eq!(
            store.market_view(&viewer, &market()?, None, Some(height)),
            Err(ViewError::AuthorityRegression)
        );
        assert_eq!(
            store.require_fresh_authority(&market()?, height),
            Err(ViewError::AuthorityRegression)
        );
    }
    for height in [1000, 1000 + AUTHORITY_FRESHNESS_HEIGHTS] {
        assert_eq!(
            store
                .market_view(&viewer, &market()?, None, Some(height))?
                .freshness,
            layerx_indexer::ai_market::Freshness::Current
        );
        assert!(store.require_fresh_authority(&market()?, height).is_ok());
    }
    let lag = AUTHORITY_FRESHNESS_HEIGHTS + 1;
    assert_eq!(
        store
            .market_view(&viewer, &market()?, None, Some(1000 + lag))?
            .freshness,
        layerx_indexer::ai_market::Freshness::Stale { lag }
    );
    assert_eq!(
        store.require_fresh_authority(&market()?, 1000 + lag),
        Err(ViewError::AuthorityStale { lag })
    );
    Ok(())
}

struct Server {
    url: String,
    stopped: Arc<AtomicBool>,
    thread: Option<thread::JoinHandle<()>>,
}
impl Server {
    fn new(handler: impl Fn(&str, &[u8]) -> (u16, Vec<u8>) + Send + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("bind loopback peer");
        let address = listener.local_addr().expect("peer address");
        let stopped = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&stopped);
        let thread = thread::spawn(move || {
            for stream in listener.incoming() {
                if stop.load(Ordering::Acquire) {
                    break;
                }
                let mut stream = stream.expect("accept peer request");
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("peer timeout");
                let (head, body) = read_http_request(&mut stream);
                let (status, response) = handler(&head, &body);
                write!(stream,"HTTP/1.1 {status} Response\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",response.len()).expect("response head");
                stream.write_all(&response).expect("response body");
            }
        });
        Self {
            url: format!("http://{address}"),
            stopped,
            thread: Some(thread),
        }
    }
    fn endpoint(&self) -> Checked<Endpoint> {
        Ok(Endpoint::parse(
            &self.url,
            Security::Plaintext {
                allow_remote: false,
            },
            Duration::from_secs(5),
        )?)
    }
}
impl Drop for Server {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.url.trim_start_matches("http://"));
        self.thread
            .take()
            .expect("peer worker")
            .join()
            .expect("peer served real requests");
    }
}
fn read_http_request(stream: &mut TcpStream) -> (String, Vec<u8>) {
    let mut bytes = Vec::new();
    let mut chunk = [0u8; 4096];
    let end = loop {
        let n = stream.read(&mut chunk).expect("read request");
        assert!(n > 0, "request ended before headers");
        bytes.extend_from_slice(&chunk[..n]);
        if let Some(end) = bytes.windows(4).position(|w| w == b"\r\n\r\n") {
            break end + 4;
        }
    };
    let head = String::from_utf8(bytes[..end].to_vec()).expect("HTTP request head");
    let length = head
        .lines()
        .filter_map(|l| l.split_once(':'))
        .find(|(k, _)| k.eq_ignore_ascii_case("content-length"))
        .map(|(_, v)| v.trim().parse::<usize>().expect("content length"))
        .unwrap_or(0);
    while bytes.len() < end + length {
        let n = stream.read(&mut chunk).expect("request body");
        assert!(n > 0, "request ended before body");
        bytes.extend_from_slice(&chunk[..n]);
    }
    (head, bytes[end..end + length].to_vec())
}

// The peer serves the repository's recorded EVM transcript. It can deliberately
// omit chainId in the block response while keeping the transaction lookup intact.
fn node_peer(
    tenant: bool,
    omit_block_domain: bool,
    unresolved: bool,
    index_status: &'static str,
    requests: Arc<std::sync::Mutex<Vec<String>>>,
) -> Server {
    let mut fixture: Value = serde_json::from_str(include_str!("../fixtures/paxeer_blocks.json"))
        .expect("recorded EVM transcript");
    if tenant {
        for block in fixture["canonical"]["blocks"]
            .as_object_mut()
            .expect("blocks")
            .values_mut()
        {
            for transaction in block["transactions"].as_array_mut().expect("transactions") {
                transaction["chainId"] = json!(format!("0x{:x}", 125_000_001u64));
            }
        }
    }
    Server::new(move |_, body| {
        let request: Value = serde_json::from_slice(body).expect("JSON-RPC request");
        let method = request["method"].as_str().expect("method");
        requests
            .lock()
            .expect("request ledger")
            .push(method.to_owned());
        let params = &request["params"];
        let result = match method {
            "status" => json!({"node_info":{"other":{"tx_index":index_status}},
                "sync_info":{"latest_block_height":"3","catching_up":false}}),
            "tx_search" => {
                return (
                    200,
                    serde_json::to_vec(&json!({"jsonrpc":"2.0","id":request["id"],
                "error":{"code":-32603,"data":layerx_indexer::paxeer::TX_INDEX_DISABLED}}))
                    .expect("disabled index transcript"),
                )
            }
            "eth_chainId" => fixture["chain_id"].clone(),
            "eth_blockNumber" => fixture["canonical"]["head"].clone(),
            "eth_getBlockByNumber" => {
                let height = u64::from_str_radix(
                    params[0].as_str().expect("height").trim_start_matches("0x"),
                    16,
                )
                .expect("height quantity");
                let mut block = fixture["canonical"]["blocks"][height.to_string()].clone();
                if omit_block_domain {
                    for transaction in block["transactions"].as_array_mut().expect("transactions") {
                        transaction
                            .as_object_mut()
                            .expect("transaction object")
                            .remove("chainId");
                    }
                }
                block
            }
            "eth_getTransactionByHash" => {
                let mut found = None;
                for block in fixture["canonical"]["blocks"]
                    .as_object()
                    .expect("blocks")
                    .values()
                {
                    for tx in block["transactions"].as_array().expect("transactions") {
                        if tx["hash"] == params[0] {
                            found = Some(tx.clone());
                        }
                    }
                }
                let mut transaction = found.expect("recorded transaction lookup");
                if unresolved {
                    transaction
                        .as_object_mut()
                        .expect("transaction")
                        .remove("chainId");
                }
                transaction
            }
            "eth_getTransactionReceipt" => {
                fixture["receipts"][params[0].as_str().expect("hash")].clone()
            }
            other => panic!("unexpected node request {other}"),
        };
        (
            200,
            serde_json::to_vec(&json!({"jsonrpc":"2.0","id":request["id"],"result":result}))
                .expect("RPC transcript"),
        )
    })
}
fn ingester(server: &Server, comet: bool) -> Checked<PaxeerIngester> {
    Ok(PaxeerIngester::new(
        server.endpoint()?,
        if comet {
            Some(server.endpoint()?)
        } else {
            None
        },
        AbiRegistry::default(),
        FollowPolicy {
            finality_depth: 2,
            max_units_per_step: 16,
        },
        1,
        Some(0xe5),
        AttributeEncoding::Base64,
    ))
}
fn tenant(store: &Store) -> Checked {
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

#[test]
fn daemon_callback_reaches_evm_fallback_and_keeps_comet_unready() -> Checked {
    let calls = Arc::new(std::sync::Mutex::new(Vec::new()));
    let server = node_peer(false, false, false, "off", Arc::clone(&calls));
    let node = ingester(&server, true)?;
    let store = Store::open_in_memory()?;
    store.start_source("comet")?;
    store.start_source("paxeer")?;
    assert_eq!(
        node.daemon_step(&store)?,
        StepOutcome::Advanced {
            units: 3,
            position: 3
        }
    );
    assert_eq!(store.cursor("paxeer")?.expect("EVM progress").position, 3);
    assert!(!store.transfers("paxeer")?.is_empty());
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
    let calls = calls.lock().expect("callback order");
    assert_eq!(
        &calls[..4],
        ["status", "eth_chainId", "eth_blockNumber", "tx_search"]
    );
    assert!(calls.iter().any(|m| m == "eth_getTransactionReceipt"));
    drop(calls);
    let invalid = node_peer(
        false,
        false,
        false,
        "unknown",
        Arc::new(std::sync::Mutex::new(Vec::new())),
    );
    let refused = Store::open_in_memory()?;
    assert!(ingester(&invalid, true)?.daemon_step(&refused).is_err());
    assert!(refused.cursor("paxeer")?.is_none());
    Ok(())
}

#[test]
fn tenant_query_authenticates_database_domains_and_falls_back_to_node() -> Checked {
    let calls = Arc::new(std::sync::Mutex::new(Vec::new()));
    let server = node_peer(true, true, false, "off", Arc::clone(&calls));
    let node = ingester(&server, false)?;
    let mut rows: BlockscoutRange =
        serde_json::from_str(include_str!("fixtures/backfill_blockscout.json"))?;
    assert!(rows.transactions.iter().all(|t| t.chain_id.is_none()));
    // Even a populated domain hint is untrusted database content.
    rows.transactions[0].chain_id = Some("125000999".into());
    layerx_indexer::paxscan::resolve_transaction_domains(&mut rows, &node)?;
    assert!(rows
        .transactions
        .iter()
        .all(|t| t.chain_id.as_deref() == Some("125000001")));
    let direct = Store::open_in_memory()?;
    tenant(&direct)?;
    for block in rpc_blocks(&rows)?.values() {
        let unit = node.decode(&block.block, &block.receipts, &[], &|_| Ok(false))?;
        direct.commit_backfill(&unit, 2)?;
    }
    assert!(!direct.transfers("pax:1")?.is_empty());
    assert!(direct.transfers("paxeer")?.is_empty());
    let dir = Scratch::new("tenant-query");
    let imported = Store::open(&dir.db())?;
    tenant(&imported)?;
    Backfill::new(&node, 3, 3).apply_range(&imported, 1, 3, &rows)?;
    assert_eq!(imported.transfers("pax:1")?, direct.transfers("pax:1")?);
    assert!(imported.transfers("paxeer")?.is_empty());
    assert!(imported.transfers("pax:999")?.is_empty());
    assert!(
        calls
            .lock()
            .expect("lookup requests")
            .iter()
            .filter(|m| m.as_str() == "eth_getTransactionByHash")
            .count()
            >= rows.transactions.len() * 2
    );
    let expected = imported.transfers("pax:1")?;
    drop(imported);
    assert_eq!(Store::open(&dir.db())?.transfers("pax:1")?, expected);
    Ok(())
}

#[test]
fn unresolved_transaction_domains_refuse_entire_backfill_range() -> Checked {
    let server = node_peer(
        true,
        true,
        true,
        "off",
        Arc::new(std::sync::Mutex::new(Vec::new())),
    );
    let node = ingester(&server, false)?;
    let mut rows: BlockscoutRange =
        serde_json::from_str(include_str!("fixtures/backfill_blockscout.json"))?;
    let original = rows.clone();
    assert!(layerx_indexer::paxscan::resolve_transaction_domains(&mut rows, &node).is_err());
    assert_eq!(rows, original, "domain lookup is all or nothing");
    let store = Store::open_in_memory()?;
    tenant(&store)?;
    assert!(Backfill::new(&node, 3, 3)
        .apply_range(&store, 1, 3, &rows)
        .is_err());
    assert!(store.backfill_cursor("paxeer")?.is_none());
    assert!(store.cursor("paxeer")?.is_none());
    assert!(store.transfers("paxeer")?.is_empty());
    assert!(store.transfers("pax:1")?.is_empty());
    Ok(())
}

#[test]
fn legacy_settlement_upgrade_preserves_history_without_retired_authority() -> Checked {
    let server = node_peer(
        false,
        false,
        false,
        "off",
        Arc::new(std::sync::Mutex::new(Vec::new())),
    );
    let dir = Scratch::new("legacy-settlement");
    let store = Store::open(&dir.db())?;
    ingester(&server, false)?.step(&store)?;
    let expected = store.transfers("paxeer")?;
    assert!(!expected.is_empty());
    drop(store);
    let sql = rusqlite::Connection::open(dir.db())?;
    sql.execute("DROP TABLE retired_settlement_evidence", [])?;
    sql.execute(
        "DELETE FROM store_migrations WHERE name='retire-settlement-output-v1'",
        [],
    )?;
    let document=json!({"level":"settled","source":"retired-checkpoint","checkpoint_id":"retired-authority"}).to_string();
    sql.execute("INSERT INTO settlement_evidence(chain,activity_id,sequence,document,last_attempt) VALUES(?1,?2,?3,?4,?5)",
        rusqlite::params!["paxeer",expected[0]["tx_id"].as_str().expect("transaction"),expected[0]["height_or_seq"].as_str().expect("height").parse::<i64>().expect("position"),document,7])?;
    drop(sql);
    let upgraded = Store::open(&dir.db())?;
    assert_eq!(upgraded.transfers("paxeer")?, expected);
    let account = expected[0]["account"].as_str().expect("history account");
    let history = upgraded.history(account, None, 100, None)?;
    assert!(!history.items.is_empty());
    for row in &history.items {
        assert_eq!(row["final_basis"], "block_commit");
        assert_eq!(row["settlement"]["level"], "unverified");
        assert!(row["settlement"]["source"].is_null());
        assert!(!row.to_string().contains("retired-authority"));
    }
    let sql = rusqlite::Connection::open(dir.db())?;
    assert_eq!(
        sql.query_row("SELECT COUNT(*) FROM settlement_evidence", [], |r| r
            .get::<_, usize>(0))?,
        0
    );
    assert_eq!(
        sql.query_row(
            "SELECT document FROM retired_settlement_evidence",
            [],
            |r| r.get::<_, String>(0)
        )?,
        document
    );
    // Output policy remains safe even if an old writer repopulates the retired input table.
    sql.execute("INSERT INTO settlement_evidence SELECT chain,activity_id,sequence,document,last_attempt FROM retired_settlement_evidence",[])?;
    assert_eq!(upgraded.transfers("paxeer")?, expected);
    drop(sql);
    drop(upgraded);
    let reopened = Store::open(&dir.db())?;
    assert_eq!(reopened.transfers("paxeer")?, expected);
    let sql = rusqlite::Connection::open(dir.db())?;
    assert_eq!(
        sql.query_row(
            "SELECT COUNT(*) FROM retired_settlement_evidence",
            [],
            |r| r.get::<_, usize>(0)
        )?,
        1
    );
    Ok(())
}

fn runtime_settings(dir: &Scratch, server: &Server) -> Checked<PathBuf> {
    let path = dir.0.join("ai-config.json");
    let configured: Vec<_> = validators()
        .iter()
        .map(|v| json!({"pub_key":v.pub_key,"voting_power":v.voting_power}))
        .collect();
    fs::write(
        &path,
        serde_json::to_vec(&json!({"chain":CHAIN,"program":PROGRAM,
        "consensus_chain_id":CHAIN_ID,"validators":configured,"source":server.url,
        "ca_der":null,"allow_remote_plaintext":false}))?,
    )?;
    Ok(path)
}

// An authenticated identity fixture is an independently signed challenge. The shared
// consumer invokes its authentication peer before admission, just as the gateway invokes
// its durable key lookup; no production projection, codec or transport is replaced.
fn authenticated_fixture_principal(
    key: &ed25519_dalek::VerifyingKey,
    signature: &ed25519_dalek::Signature,
) -> Checked<String> {
    key.verify_strict(b"AI view request fixture", signature)
        .map_err(|_| Failure::Unexpected("credential refused"))?;
    Ok(hex(&Sha256::digest(key.as_bytes())))
}

#[test]
fn ai_runtime_authenticated_publication_and_reads_survive_restart() -> Checked {
    let participant = worker(1)?;
    let original = observed(participant, 1, 950)?;
    let (section, _) = frame(vec![original], &[completion(1, 950)?])?;
    let base = market_with_workers(3, 1)?;
    let shared = state::decode_shared_state(&base)?;
    let state = encode_shared(&shared.replace_section(Section::ReputationAdmission, &section)?)?;
    let material = certify_at(&state, 1000)?;
    let chunks = read_all(&state)?;
    let encoded = encode_committed_read(chain()?, program()?, 1000, &material.evidence(), &chunks)?;
    let calls = Arc::new(std::sync::Mutex::new(Vec::new()));
    let seen = Arc::clone(&calls);
    let body = encoded.clone();
    let program_hex = hex(&PROGRAM);
    let server = Server::new(move |head, request_body| {
        assert!(request_body.is_empty());
        let path = head
            .lines()
            .next()
            .expect("GET request")
            .split_whitespace()
            .nth(1)
            .expect("target");
        seen.lock().expect("source requests").push(path.to_owned());
        if path == format!("/v1/ai/programs/{program_hex}/committed-read?after=0") {
            (200, body.clone())
        } else {
            assert_eq!(
                path,
                format!("/v1/ai/programs/{program_hex}/committed-read?after=1000")
            );
            (204, Vec::new())
        }
    });
    let dir = Scratch::new("ai-runtime");
    let config = runtime_settings(&dir, &server)?;
    let (mut runtime, endpoint) = RuntimeConfig::read(&config)?.open(&dir.db())?;
    let observations = Store::open(&dir.db())?;
    observations.start_source("ai")?;
    assert!(runtime.projections.stream(&market()?)?.is_none());
    let accepted = runtime
        .daemon_poll(&observations, &endpoint, PUBLICATION_TIME_MS)?
        .expect("committed program read");
    assert_eq!(accepted.projection, ProjectionState::FinalizedPublishable);
    assert_eq!(accepted.rank, FINALIZED_RANK);
    let readiness = layerx_indexer::api::Readiness {
        sources: vec!["ai"],
        stall_after_secs: 30,
        started_at: 0,
        explorer: None,
    };
    assert_eq!(
        layerx_indexer::api::readiness_document(&observations, &readiness)?.status,
        "ready"
    );
    let request = HistoryRequest {
        market: market()?,
        worker: participant,
        epoch: Some(1),
        minimum_finalized_height: 1000,
    };
    let reputation = runtime.reputation.read_history(&request)?;
    assert_eq!(reputation.current, original);
    let views = AiViews::open(&dir.db(), 1, [42; 32])?;
    let key = SigningKey::from_bytes(&[77; 32]);
    let credential = key.sign(b"AI view request fixture");
    let bad = key.sign(b"wrong request");
    assert!(views
        .authenticated_query(
            || authenticated_fixture_principal(&key.verifying_key(), &bad),
            "snapshot",
            &hex(market()?.as_bytes()),
            None,
            Some(1000),
            6000
        )
        .is_err());
    assert_eq!(
        views.limiter.retained_principals()?,
        0,
        "failed authentication precedes all query admission"
    );
    let snapshot = views.authenticated_query(
        || authenticated_fixture_principal(&key.verifying_key(), &credential),
        "snapshot",
        &hex(market()?.as_bytes()),
        None,
        Some(1000),
        6000,
    )??;
    assert_eq!(snapshot["committed"], true);
    assert_eq!(
        snapshot["snapshot_id"],
        hex0x(accepted.snapshot_id.as_bytes())
    );
    assert_eq!(snapshot["components"]["F07"], "available");
    assert_eq!(snapshot["freshness"]["label"], "current");
    let page = views.authenticated_query(
        || authenticated_fixture_principal(&key.verifying_key(), &credential),
        "participants",
        &hex(market()?.as_bytes()),
        Some("limit=1"),
        Some(1000),
        6001,
    )??;
    assert_eq!(page["rows"].as_array().expect("participant rows").len(), 1);
    let cursor = page["cursor"].as_str().expect("next page").to_owned();
    let events = runtime.projections.pending_events(&market()?, 10)?;
    assert_eq!(events.len(), 1);
    assert!(events[0].rank >= FINALIZED_RANK);
    drop(views);
    drop(runtime);
    drop(observations);
    drop(state);
    drop(material);
    drop(chunks);
    let (mut reopened, endpoint) = RuntimeConfig::read(&config)?.open(&dir.db())?;
    let observations = Store::open(&dir.db())?;
    observations.start_source("ai")?;
    assert_eq!(
        layerx_indexer::api::readiness_document(&observations, &readiness)?.status,
        "unavailable"
    );
    assert!(reopened
        .daemon_poll(&observations, &endpoint, 7000)?
        .is_none());
    assert_eq!(
        layerx_indexer::api::readiness_document(&observations, &readiness)?.status,
        "ready"
    );
    assert_eq!(reopened.reputation.read_history(&request)?, reputation);
    assert_eq!(reopened.projections.pending_events(&market()?, 10)?, events);
    assert_eq!(
        reopened.ingest(&encoded, 7001)?,
        accepted,
        "producer replay cannot duplicate publication"
    );
    let views = AiViews::open(&dir.db(), 1, [42; 32])?;
    let recovered = views.authenticated_query(
        || authenticated_fixture_principal(&key.verifying_key(), &credential),
        "snapshot",
        &hex(market()?.as_bytes()),
        None,
        Some(1000),
        7000,
    )??;
    assert_eq!(recovered, snapshot);
    let next = views.authenticated_query(
        || authenticated_fixture_principal(&key.verifying_key(), &credential),
        "participants",
        &hex(market()?.as_bytes()),
        Some(&format!("limit=1&cursor={cursor}")),
        Some(1000),
        7001,
    )??;
    assert_eq!(next["snapshot_id"], page["snapshot_id"]);
    assert_ne!(next["rows"], page["rows"]);
    let mut forged: Value = serde_json::from_slice(&encoded)?;
    forged["commit"]["signatures"][0]["signature"][0] = json!(255);
    assert!(reopened
        .ingest(&serde_json::to_vec(&forged)?, 7002)
        .is_err());
    assert_eq!(reopened.projections.pending_events(&market()?, 10)?, events);
    // Credential revocation/failure on a later request still blocks a recovered cursor.
    let principals = views.limiter.retained_principals()?;
    assert!(views
        .authenticated_query(
            || authenticated_fixture_principal(&key.verifying_key(), &bad),
            "participants",
            &hex(market()?.as_bytes()),
            Some(&format!("limit=1&cursor={cursor}")),
            Some(1000),
            7002
        )
        .is_err());
    assert_eq!(views.limiter.retained_principals()?, principals);
    assert_eq!(calls.lock().expect("polls").len(), 2);
    Ok(())
}

#[test]
fn ai_runtime_rejects_unbound_captures_before_any_publication() -> Checked {
    let state = created_state()?;
    let material = certify_at(&state, 1000)?;
    let chunks = read_all(&state)?;
    let encoded = encode_committed_read(chain()?, program()?, 1000, &material.evidence(), &chunks)?;
    let pristine: Value = serde_json::from_slice(&encoded)?;
    let mut bad_signature = pristine.clone();
    bad_signature["commit"]["signatures"][0]["signature"][0] = json!(255);
    let mut bad_domain = pristine.clone();
    bad_domain["program"] = json!(vec![88u8; 32]);
    let mut bad_membership = pristine.clone();
    bad_membership["store_proof"] = json!([1, 2, 3]);
    let mut incomplete = pristine.clone();
    incomplete["chunks"] = json!([]);
    let mut bad_time = pristine.clone();
    bad_time["header"]["time"] = json!([1000, 1_000_000_000]);
    for (n, refused) in [
        bad_signature,
        bad_domain,
        bad_membership,
        incomplete,
        bad_time,
    ]
    .iter()
    .enumerate()
    {
        let dir = Scratch::new(&format!("rejected-{n}"));
        let mut runtime = AiRuntime::open(&dir.db(), authority(), chain()?, program()?)?;
        assert!(runtime.ingest(&serde_json::to_vec(refused)?, 6000).is_err());
        assert!(runtime.projections.stream(&market()?)?.is_none());
        assert!(runtime
            .projections
            .pending_events(&market()?, 10)?
            .is_empty());
        let sql = rusqlite::Connection::open(dir.db())?;
        assert_eq!(
            sql.query_row("SELECT COUNT(*) FROM ai_reputation_cursor", [], |r| r
                .get::<_, usize>(0))?,
            0
        );
    }
    Ok(())
}

#[test]
fn archive_recovery_refuses_corruption_without_losing_locator() -> Checked {
    let dir = Scratch::new("archive-integrity");
    let store = ProjectionStore::open(&dir.db())?;
    let first = publish(&store, &created_state()?, 1000)?;
    publish(&store, &created_state()?, 1001)?;
    store.archive(&first.snapshot_id, 0)?;
    drop(store);
    let sql = rusqlite::Connection::open(dir.db())?;
    sql.execute("UPDATE ai_archive SET state=x'010203'", [])?;
    drop(sql);
    let reopened = ProjectionStore::open(&dir.db())?;
    assert_eq!(
        reopened.recover_archived(&first.snapshot_id, 1),
        Err(ViewError::IntegrityFailure)
    );
    let header = reopened
        .snapshot(&first.snapshot_id)?
        .expect("archive binding");
    assert_eq!(header.projection, ProjectionState::Archived);
    assert!(!header.retained);
    assert_eq!(
        reopened.alerts()?.last().expect("integrity alert").category,
        "integrity-failure"
    );
    let sql = rusqlite::Connection::open(dir.db())?;
    assert_eq!(
        sql.query_row(
            "SELECT COUNT(*) FROM ai_snapshot_archive WHERE snapshot_id=?1",
            [first.snapshot_id.as_bytes().as_slice()],
            |r| r.get::<_, usize>(0)
        )?,
        1
    );
    Ok(())
}

#[test]
fn ai_runtime_idle_before_first_projection_is_not_ready() -> Checked {
    let server = Server::new(|head, body| {
        assert!(head.starts_with("GET /v1/ai/programs/"));
        assert!(body.is_empty());
        (204, Vec::new())
    });
    let dir = Scratch::new("idle-ai");
    let config = runtime_settings(&dir, &server)?;
    let (mut runtime, endpoint) = RuntimeConfig::read(&config)?.open(&dir.db())?;
    let observations = Store::open(&dir.db())?;
    observations.start_source("ai")?;
    assert!(runtime.daemon_poll(&observations, &endpoint, 1000).is_err());
    let readiness = layerx_indexer::api::Readiness {
        sources: vec!["ai"],
        stall_after_secs: 30,
        started_at: 0,
        explorer: None,
    };
    assert_eq!(
        layerx_indexer::api::readiness_document(&observations, &readiness)?.status,
        "unavailable"
    );
    assert!(runtime.projections.stream(&market()?)?.is_none());
    Ok(())
}
