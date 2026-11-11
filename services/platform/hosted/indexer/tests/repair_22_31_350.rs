//! Task 22.31.350 repair record (audit 22.30.82).
//! tenant-backfill: tenant_history_survives_cutover_and_reopen.
//! backfill-content-authentication: unauthenticated_database_cannot_publish_changed_bodies.
//! snapshot-membership: snapshot_requires_exact_native_membership.
//! reputation-membership: reputation_refuses_changed_f07_across_reopen.
//! Prior attempt: tenant_history_survives_cutover_and_reopen and
//! unauthenticated_database_cannot_publish_changed_bodies failed on empty event history;
//! snapshot_requires_exact_native_membership and reputation_refuses_changed_f07_across_reopen passed.
//! No additional baseline gate: the explicit qualification order permits gates only after implementation.
//! Wiring request: modules/lxphost (node domain) must supply the native namespace head proof,
//! multistore proof and manifest in ProgramStateProof JSON for indexer finality promotion.
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
    ProjectionState, ProjectionStore, ReadVerification, ViewError, Viewer,
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
enum Failure {
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
        observed_sequence: 1000,
        execution_height: 1000,
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
            "repair-22-31-350-{}-{}-{name}",
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
        source_activity: digest(90)?,
        observed_at_ms: 4000,
    })
}

#[test]
fn ai_market_rejects_altered_membership_and_unsigned_reads_across_restart() -> Checked {
    let state = created_state()?;
    let material = certify_state(&state, PROGRAM, layerx_programs_ai_market::SHARED_STATE_KEY)?;
    let (_, _, facts) = capture_state(&state, &material)?;
    let viewer = Viewer::for_principal("ai-market-refusal-reader")?;
    let mut altered_proof = material.proof.clone();
    altered_proof.push(0);
    let altered_evidence = BlockCommitProof {
        store_proof: &altered_proof,
        ..material.evidence()
    };

    for (name, read, evidence, expected_error) in [
        (
            "altered-membership",
            ReadVerification::SequencerSigned,
            altered_evidence,
            ViewError::FinalityUnavailable,
        ),
        (
            "unsigned-read",
            ReadVerification::Unverified,
            material.evidence(),
            ViewError::BindingMismatch,
        ),
    ] {
        let dir = Scratch::new(name);
        let store = ProjectionStore::open(&dir.db())?;
        let mut observation = snapshot_observation(&state, facts)?;
        observation.read = read;
        let initial = store.observe(&observation)?;
        assert_eq!(initial.read, read);
        assert_eq!(
            store.promote(
                &initial.snapshot_id,
                &evidence,
                &authority(),
                PUBLICATION_TIME_MS,
            ),
            Err(expected_error)
        );
        let assert_unpublished = |store: &ProjectionStore| -> Checked {
            let snapshot = store
                .snapshot(&initial.snapshot_id)?
                .expect("saved capture");
            assert_eq!(snapshot, initial);
            assert_eq!(snapshot.rank, 0);
            assert_eq!(snapshot.binding, None);
            assert_eq!(snapshot.checkpoint, None);
            assert_eq!(snapshot.publication_time_ms, None);
            assert_eq!(
                snapshot.projection,
                match read {
                    ReadVerification::Unverified => ProjectionState::ObservedUnverified,
                    ReadVerification::SequencerSigned =>
                        ProjectionState::EvidenceVerifiedUnfinalized,
                }
            );
            let stream = store.stream(&market()?)?.expect("observed stream");
            assert_eq!(stream.finalized_snapshot, None);
            assert_eq!(stream.finalized_sequence, None);
            assert!(store.pending_events(&market()?, 10)?.is_empty());
            for selector in [Some(&initial.snapshot_id), None] {
                assert!(matches!(
                    store.market_view(&viewer, &market()?, selector, Some(1000)),
                    Err(ViewError::FinalityUnavailable)
                ));
            }
            Ok(())
        };
        assert_unpublished(&store)?;
        drop(store);
        let reopened = ProjectionStore::open(&dir.db())?;
        assert_unpublished(&reopened)?;
    }

    let dir = Scratch::new("valid-membership-and-signed-read");
    let store = ProjectionStore::open(&dir.db())?;
    let initial = store.observe(&snapshot_observation(&state, facts)?)?;
    assert_eq!(initial.rank, 0);
    assert_eq!(initial.read, ReadVerification::SequencerSigned);
    let finalized = store.promote(
        &initial.snapshot_id,
        &material.evidence(),
        &authority(),
        PUBLICATION_TIME_MS,
    )?;
    assert_eq!(finalized.snapshot_id, initial.snapshot_id);
    assert_eq!(finalized.rank, FINALIZED_RANK);
    assert_eq!(finalized.projection, ProjectionState::FinalizedPublishable);
    assert!(!finalized
        .binding
        .as_ref()
        .expect("finalized binding")
        .is_empty());
    assert_eq!(finalized.publication_time_ms, Some(PUBLICATION_TIME_MS));
    let view = store.market_view(&viewer, &market()?, None, Some(1000))?;
    assert_eq!(view.snapshot_id, finalized.snapshot_id);
    view.binding.require_finalized()?;
    let stream = store.stream(&market()?)?.expect("finalized stream");
    assert_eq!(stream.finalized_snapshot, Some(finalized.snapshot_id));
    assert_eq!(
        stream.finalized_sequence,
        Some(facts.proof.observed_sequence)
    );
    let events = store.pending_events(&market()?, 10)?;
    assert!(!events.is_empty());
    drop(store);
    let reopened = ProjectionStore::open(&dir.db())?;
    assert_eq!(reopened.snapshot(&finalized.snapshot_id)?, Some(finalized));
    assert_eq!(reopened.stream(&market()?)?, Some(stream));
    assert_eq!(reopened.pending_events(&market()?, 10)?, events);
    assert_eq!(
        reopened.market_view(&viewer, &market()?, None, Some(1000))?,
        view
    );
    Ok(())
}

#[path = "../../../../../programs/crates/layerx-programs-ai-market/tests/repair_22_33_233.rs"]
mod consumer_fixture;

fn transcript_hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn transcript_has_current_sources(recorded: &Value) -> bool {
    [
        (
            "consumer_source_sha256",
            include_bytes!("../src/ai_market.rs").as_slice(),
        ),
        (
            "membership_source_sha256",
            include_bytes!("../src/ai_market_membership.rs").as_slice(),
        ),
    ]
    .into_iter()
    .all(|(field, source)| {
        recorded[field]
            .as_str()
            .and_then(|text| unhex(text).ok())
            .is_some_and(|digest| digest.as_slice() == Sha256::digest(source).as_slice())
    })
}

fn transcript_binding(
    binding: &layerx_programs_ai_market::queries::SnapshotBinding,
) -> Checked<String> {
    let mut bytes = vec![0; layerx_programs_ai_market::queries::BINDING_MAX_BYTES];
    let n = binding.encode(&mut bytes)?;
    Ok(transcript_hex(&bytes[..n]))
}

fn live_consumer_transcript() -> Checked<Value> {
    let state = consumer_fixture::producer_state();
    let valid = consumer_fixture::Material::new(&state, consumer_fixture::PROGRAM);
    let authority = FinalityAuthority {
        chain_id: consumer_fixture::CHAIN_ID.to_owned(),
        validators: consumer_fixture::authority(),
    };
    let viewer = Viewer::for_principal("ai-market-transcript-reader")?;
    let evidence = |material: &consumer_fixture::Material| Material {
        header: material.header.clone(),
        commit: material.commit.clone(),
        proof: material.proof.clone(),
    };
    let dir = Scratch::new("consumer-transcript-successes");
    let mut successes = Vec::new();
    for sequence in [77u64, 78] {
        let (captured, facts) = consumer_fixture::capture(&state, &valid, sequence);
        let material = evidence(&valid);
        let authenticated = verify_program_state(
            &material.evidence(),
            &authority,
            facts.proof.chain,
            facts.proof.program,
            &captured,
        )?;
        let store = ProjectionStore::open(&dir.db())?;
        let initial = store.observe(&SnapshotObservation {
            state: &captured,
            facts,
            read: ReadVerification::SequencerSigned,
            source_activity: digest(sequence as u8)?,
            observed_at_ms: sequence * 100,
        })?;
        assert_eq!(initial.rank, 0);
        let finalized = store.promote(
            &initial.snapshot_id,
            &material.evidence(),
            &authority,
            sequence * 100 + 1,
        )?;
        assert_eq!(finalized.snapshot_id, initial.snapshot_id);
        assert_eq!(finalized.rank, FINALIZED_RANK);
        let before = store.market_view(
            &viewer,
            &finalized.market,
            Some(&finalized.snapshot_id),
            Some(consumer_fixture::HEIGHT),
        )?;
        let pending = store.pending_events(&finalized.market, 10)?;
        assert_eq!(pending.len(), (sequence - 76) as usize);
        let event = pending.last().expect("finalized event");
        assert_eq!(event.snapshot_id, finalized.snapshot_id);
        assert_eq!(Some(&event.binding), finalized.binding.as_ref());
        assert_eq!(event.rank, FINALIZED_RANK);
        assert!(!event.published);
        let cursor = store.stream(&finalized.market)?;
        drop(store);
        let database_sha256 = transcript_hex(&Sha256::digest(fs::read(dir.db())?));
        let reopened = ProjectionStore::open(&dir.db())?;
        let recovered = reopened
            .snapshot(&finalized.snapshot_id)?
            .expect("recovered capture");
        assert_eq!(recovered, finalized);
        assert_eq!(reopened.stream(&finalized.market)?, cursor);
        assert_eq!(reopened.pending_events(&finalized.market, 10)?, pending);
        let recovered_view = reopened.market_view(
            &viewer,
            &finalized.market,
            Some(&finalized.snapshot_id),
            Some(consumer_fixture::HEIGHT),
        )?;
        assert_eq!(recovered_view, before);
        recovered_view.binding.verify_capture(&captured, &facts)?;
        assert_eq!(recovered.state_digest, facts.digest);
        successes.push(json!({
            "authenticated_chain": transcript_hex(authenticated.chain().as_bytes()),
            "authenticated_program": transcript_hex(authenticated.program().as_bytes()),
            "before_binding": transcript_binding(&before.binding)?,
            "block_hash": transcript_hex(facts.proof.block_hash.as_bytes()),
            "chunks": facts.chunks,
            "database_sha256": database_sha256,
            "event_binding": transcript_hex(&event.binding),
            "event_snapshot_id": transcript_hex(event.snapshot_id.as_bytes()),
            "freshness": format!("{:?}", before.freshness),
            "native_root": transcript_hex(facts.proof.native_state_root.as_bytes()),
            "observed_rank": initial.rank,
            "pending_count": pending.len(),
            "projection": format!("{:?}", finalized.projection),
            "proof_sha256": transcript_hex(&Sha256::digest(&material.proof)),
            "rank": finalized.rank,
            "record_binding": finalized.binding.as_ref().map(|bytes| transcript_hex(bytes)),
            "record_equal_after_reopen": recovered == finalized,
            "recovered_binding": recovered.binding.as_ref().map(|bytes| transcript_hex(bytes)),
            "recovered_projection": format!("{:?}", recovered_view.projection),
            "recovered_snapshot_id": transcript_hex(recovered_view.snapshot_id.as_bytes()),
            "recovered_source_activity": transcript_hex(recovered_view.source_activity.as_bytes()),
            "sequence": sequence,
            "snapshot_id": transcript_hex(finalized.snapshot_id.as_bytes()),
            "source_activity": transcript_hex(before.source_activity.as_bytes()),
            "state_digest": transcript_hex(facts.digest.as_bytes()),
            "state_sha256": transcript_hex(&Sha256::digest(&captured)),
            "total_bytes": facts.total_bytes,
        }));
    }

    let (captured, facts) = consumer_fixture::capture(&state, &valid, 77);
    let mut refusals = Vec::new();
    for case in 0..5 {
        let mut material = evidence(&valid);
        let mut offered = facts;
        let mut read = ReadVerification::SequencerSigned;
        match case {
            0 => {
                material.commit.signatures[0]
                    .signature
                    .as_mut()
                    .expect("signature")[0] ^= 1
            }
            1 => material = evidence(&consumer_fixture::Material::new(&state, [99; 32])),
            2 => material.proof.push(0),
            3 => offered.proof.block_hash = digest(99)?,
            4 => read = ReadVerification::Unverified,
            _ => unreachable!(),
        }
        let dir = Scratch::new("consumer-transcript-refusal");
        let store = ProjectionStore::open(&dir.db())?;
        let initial = store.observe(&SnapshotObservation {
            state: &captured,
            facts: offered,
            read,
            source_activity: digest(77)?,
            observed_at_ms: 7700,
        })?;
        let error = store
            .promote(&initial.snapshot_id, &material.evidence(), &authority, 2000)
            .expect_err("unauthenticated promotion must fail");
        assert_eq!(
            error,
            if case >= 3 {
                ViewError::BindingMismatch
            } else {
                ViewError::FinalityUnavailable
            }
        );
        assert_eq!(store.snapshot(&initial.snapshot_id)?, Some(initial.clone()));
        let cursor = store.stream(&initial.market)?.expect("observed stream");
        assert_eq!(cursor.finalized_snapshot, None);
        assert_eq!(cursor.finalized_sequence, None);
        assert!(store.pending_events(&initial.market, 10)?.is_empty());
        drop(store);
        let reopened = ProjectionStore::open(&dir.db())?;
        let recovered = reopened
            .snapshot(&initial.snapshot_id)?
            .expect("recovered refusal");
        assert_eq!(recovered, initial);
        assert_eq!(recovered.rank, 0);
        assert_eq!(recovered.binding, None);
        assert_eq!(recovered.checkpoint, None);
        assert_eq!(recovered.publication_time_ms, None);
        assert!(!recovered.projection.finalized());
        assert_eq!(reopened.stream(&initial.market)?, Some(cursor));
        let pending = reopened.pending_events(&initial.market, 10)?;
        assert!(pending.is_empty());
        let view_error = reopened
            .market_view(
                &viewer,
                &initial.market,
                Some(&initial.snapshot_id),
                Some(consumer_fixture::HEIGHT),
            )
            .expect_err("refused capture cannot be published");
        assert_eq!(view_error, ViewError::FinalityUnavailable);
        assert_eq!(
            reopened.market_view(
                &viewer,
                &initial.market,
                None,
                Some(consumer_fixture::HEIGHT)
            ),
            Err(ViewError::FinalityUnavailable)
        );
        refusals.push(json!({
            "block_hash": transcript_hex(&material.header.hash()),
            "case": case,
            "error": format!("{error:?}"),
            "native_root": transcript_hex(&material.header.app_hash),
            "pending_count": pending.len(),
            "proof_sha256": transcript_hex(&Sha256::digest(&material.proof)),
            "read_block_hash": transcript_hex(offered.proof.block_hash.as_bytes()),
            "read_verification": read as u8,
            "recovered_binding": recovered.binding.as_ref().map(|bytes| transcript_hex(bytes)),
            "recovered_projection_finalized": recovered.projection.finalized(),
            "recovered_rank": recovered.rank,
            "recovered_snapshot_id": transcript_hex(recovered.snapshot_id.as_bytes()),
            "recovered_state_digest": transcript_hex(recovered.state_digest.as_bytes()),
            "signature": transcript_hex(&material.commit.signatures[0].signature.expect("signature")),
            "snapshot_id": transcript_hex(initial.snapshot_id.as_bytes()),
            "view_error": format!("{view_error:?}"),
        }));
    }
    Ok(json!({
        "consumer": "services/platform/hosted/indexer/src/ai_market.rs::ProjectionStore",
        "consumer_source_sha256": transcript_hex(&Sha256::digest(include_bytes!("../src/ai_market.rs"))),
        "format": 1,
        "membership_source_sha256": transcript_hex(&Sha256::digest(include_bytes!("../src/ai_market_membership.rs"))),
        "refusals": refusals,
        "source_revision": "116ae09395ecb90e9547e536cc7d40168f788fa9",
        "successes": successes,
    }))
}

#[test]
fn ai_market_transcript_tracks_current_sources() -> Checked {
    let recorded = consumer_fixture::transcript();
    assert!(transcript_has_current_sources(&recorded));
    for field in ["consumer_source_sha256", "membership_source_sha256"] {
        let mut changed = recorded.clone();
        let mut digest = unhex(recorded[field].as_str().expect("source digest"))?;
        assert_eq!(digest.len(), 32);
        digest[0] ^= 1;
        changed[field] = json!(transcript_hex(&digest));
        assert!(
            !transcript_has_current_sources(&changed),
            "changed {field} accepted"
        );
    }
    let live = live_consumer_transcript()?;
    assert!(transcript_has_current_sources(&live));
    assert_eq!(
        recorded,
        live,
        "current production transcript: {}",
        serde_json::to_string(&live)?
    );
    consumer_fixture::ai_real_consumer_recovers_fresh_guest_captures();
    consumer_fixture::ai_real_consumer_refuses_unauthenticated_evidence_across_restart();
    Ok(())
}

#[test]
fn snapshot_requires_exact_native_membership() -> Checked {
    let state = created_state()?;
    let material = certify_state(&state, PROGRAM, layerx_programs_ai_market::SHARED_STATE_KEY)?;
    let authenticated = verify_program_state(
        &material.evidence(),
        &authority(),
        chain()?,
        program()?,
        &state,
    )?;
    assert_eq!(authenticated.state(), state);
    assert_eq!(authenticated.chain(), chain()?);
    assert_eq!(authenticated.program(), program()?);
    assert_eq!(authenticated.state_digest(), codec::state_digest(&state)?);
    assert_eq!(
        authenticated.commit().app_hash.as_slice(),
        material.header.app_hash
    );
    let dir = Scratch::new("snapshot");
    let store = ProjectionStore::open(&dir.db())?;
    let (_, _, facts) = capture_state(&state, &material)?;
    let initial = store.observe(&snapshot_observation(&state, facts)?)?;
    assert_eq!(initial.rank, 0);
    let finalized = store.promote(
        &initial.snapshot_id,
        &material.evidence(),
        &authority(),
        PUBLICATION_TIME_MS,
    )?;
    assert_eq!(finalized.rank, FINALIZED_RANK);
    let id = finalized.snapshot_id;
    drop(store);
    let reopened = ProjectionStore::open(&dir.db())?;
    assert_eq!(reopened.snapshot(&id)?, Some(finalized));
    drop(reopened);

    let mut malformed =
        certify_state(&state, PROGRAM, layerx_programs_ai_market::SHARED_STATE_KEY)?;
    malformed.proof = b"nonempty but not a membership proof".to_vec();
    let unrelated_program = certify_state(
        &state,
        [77; 32],
        layerx_programs_ai_market::SHARED_STATE_KEY,
    )?;
    let unrelated_cell = certify_state(&state, PROGRAM, b"other/state")?;
    // This transcript was emitted by the real keeper's IAVL/multistore producer.
    // It has valid membership under its committed root, for an unrelated program.
    let transcript: Value = serde_json::from_str(include_str!(
        "../../../../../programs/tests/vectors/ics23-market-namespaces-app-hash.json"
    ))?;
    let namespace = &transcript["entries"][0]["namespace"];
    let recorded = |field: &str| {
        layerx_indexer::codec::unhex(namespace[field].as_str().expect("recorded hex"))
    };
    let recorded_header = header(
        1000,
        Digest32::new(recorded("app_hash")?.try_into().expect("recorded root"))?,
    );
    let unrelated_recorded = Material {
        commit: signed(
            &recorded_header,
            &[(1, false), (2, false), (3, false), (4, false)],
        ),
        header: recorded_header,
        proof: serde_json::to_vec(
            &json!({"membership": {"proof": recorded("proof")?, "multistore": recorded("multistore_proof")?}, "manifest": recorded("manifest")?}),
        )?,
    };
    let mut bad_manifest =
        certify_state(&state, PROGRAM, layerx_programs_ai_market::SHARED_STATE_KEY)?;
    let mut document: Value = serde_json::from_slice(&bad_manifest.proof)?;
    document["manifest"][10] = json!(255);
    bad_manifest.proof = serde_json::to_vec(&document)?;
    for evidence in [
        &malformed,
        &unrelated_program,
        &unrelated_cell,
        &unrelated_recorded,
        &bad_manifest,
    ] {
        let store = ProjectionStore::open_in_memory()?;
        let (_, _, facts) = capture_state(&state, evidence)?;
        let observed = store.observe(&snapshot_observation(&state, facts)?)?;
        assert_eq!(
            store.promote(
                &observed.snapshot_id,
                &evidence.evidence(),
                &authority(),
                PUBLICATION_TIME_MS
            ),
            Err(ViewError::FinalityUnavailable)
        );
        assert_eq!(
            store
                .snapshot(&observed.snapshot_id)?
                .expect("observed snapshot")
                .rank,
            0
        );
    }
    let mut other_authority = authority();
    other_authority.chain_id = "another-consensus-chain".into();
    assert!(verify_program_state(
        &material.evidence(),
        &other_authority,
        chain()?,
        program()?,
        &state
    )
    .is_err());
    assert!(verify_program_state(
        &material.evidence(),
        &authority(),
        ChainDomain::new([78; 32])?,
        program()?,
        &state
    )
    .is_err());

    let (section, _) = frame(vec![observed(worker(1)?, 1, 999)?], &[completion(1, 999)?])?;
    let changed = state_with(&section)?;
    let store = ProjectionStore::open_in_memory()?;
    let (_, _, facts) = capture_state(&changed, &material)?;
    let offered = store.observe(&snapshot_observation(&changed, facts)?)?;
    assert_eq!(
        store.promote(
            &offered.snapshot_id,
            &material.evidence(),
            &authority(),
            PUBLICATION_TIME_MS
        ),
        Err(ViewError::FinalityUnavailable)
    );
    assert_eq!(
        store
            .snapshot(&offered.snapshot_id)?
            .expect("provisional snapshot")
            .rank,
        0
    );
    Ok(())
}

#[test]
fn reputation_refuses_changed_f07_across_reopen() -> Checked {
    let participant = worker(1)?;
    let original = observed(participant, 1, 999)?;
    let (section, records) = frame(vec![original], &[completion(1, 999)?])?;
    let state = state_with(&section)?;
    let mut changed_record = original;
    changed_record.quality = Score::new(900_000)?;
    let (changed_section, _) = frame(vec![changed_record], &[completion(1, 999)?])?;
    let changed = state_with(&changed_section)?;
    assert_ne!(codec::state_digest(&state)?, codec::state_digest(&changed)?);
    let material = certify_state(&state, PROGRAM, layerx_programs_ai_market::SHARED_STATE_KEY)?;
    let (proof, chunks, _) = capture_state(&state, &material)?;
    let (changed_proof, changed_chunks, _) = capture_state(&changed, &material)?;
    let request = HistoryRequest {
        market: market()?,
        worker: participant,
        epoch: Some(1),
        minimum_finalized_height: 1000,
    };
    let dir = Scratch::new("reputation");
    let mut projector = ReputationProjector::open(&dir.db(), authority(), chain()?, program()?)?;
    let forged = FinalizedRead {
        proof: changed_proof,
        chunks: &changed_chunks,
        commit: material.evidence(),
        publication_time_ms: PUBLICATION_TIME_MS,
    };
    assert!(matches!(
        projector.admit(&forged),
        Err(ReputationError::Commit(_))
    ));
    assert!(projector.read_history(&request).is_err());
    assert_eq!(
        projector.admit(&FinalizedRead {
            proof,
            chunks: &chunks,
            commit: material.evidence(),
            publication_time_ms: PUBLICATION_TIME_MS
        })?,
        Admission::Advanced { height: 1000 }
    );
    let accepted = projector.read_history(&request)?;
    assert_eq!(accepted.current, original);
    assert_eq!(accepted.finalized.rank, FINALIZED_RANK);
    assert_eq!(
        accepted.finalized.reputation_root,
        Presence::Present(reputation_root(market()?, 1, &records)?)
    );
    assert!(matches!(
        projector.admit(&forged),
        Err(ReputationError::Commit(_))
    ));
    assert_eq!(projector.read_history(&request)?, accepted);
    drop(projector);
    let mut reopened = ReputationProjector::open(&dir.db(), authority(), chain()?, program()?)?;
    assert_eq!(reopened.read_history(&request)?, accepted);
    assert!(matches!(
        reopened.admit(&forged),
        Err(ReputationError::Commit(_))
    ));
    assert_eq!(reopened.read_history(&request)?, accepted);
    assert_eq!(
        reopened.admit(&FinalizedRead {
            proof,
            chunks: &chunks,
            commit: material.evidence(),
            publication_time_ms: PUBLICATION_TIME_MS
        })?,
        Admission::AlreadyAccepted { height: 1000 }
    );
    Ok(())
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
    let fixture: Value = serde_json::from_str(include_str!("fixtures/chains_blocks.json"))
        .expect("recorded registry emission");
    fixture["scope"]["receipts"]
        ["0x00000000000000000000000000000000000000000000000000000000000007d1"]["logs"][0]
        .clone()
}

fn recorded_chain() -> Value {
    let mut chain: Value = serde_json::from_str(include_str!("../fixtures/paxeer_blocks.json"))
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
        serde_json::from_str(include_str!("fixtures/backfill_blockscout.json"))
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

#[test]
fn tenant_history_survives_cutover_and_reopen() -> Checked {
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
    assert!(!expected.1.is_empty());
    assert_eq!(expected.1.len(), 1);
    assert_eq!(expected.1[0]["name"], "ChainStatusChanged");
    assert_eq!(expected.1[0]["decoded"]["args"]["id"], "1");
    assert_eq!(expected.1[0]["decoded"]["args"]["status"], "2");
    assert_eq!(live.chain(1)?.expect("registered tenant").status, "paused");
    assert_eq!(expected.2["tx_count"], 5);
    assert!(expected.2["gas_used"].as_u64().expect("gas") > 0);
    assert!(live.transfers("paxeer")?.is_empty());

    let mut rows = database_rows();
    for row in &mut rows.transactions {
        row.chain_id = Some("125000001".into());
    }
    let converted = rpc_blocks(&rows)?;
    let through_codec = Store::open_in_memory()?;
    register_tenant(&through_codec)?;
    for rpc in converted.values() {
        for tx in rpc.block["transactions"].as_array().expect("transactions") {
            assert_eq!(tx["chainId"], format!("0x{:x}", 125_000_001u64));
        }
        let unit = node.decode(&rpc.block, &rpc.receipts, &[], &|_| Ok(false))?;
        through_codec.commit_backfill(&unit, 2)?;
    }
    through_codec.finish_backfill("paxeer", 3, 2)?;
    assert_eq!(tenant_history(&through_codec)?, expected);
    let dir = Scratch::new("tenant");
    let store = Store::open(&dir.db())?;
    register_tenant(&store)?;
    assert_eq!(
        Backfill::new(&node, 2, 2).run(&store, |from, to| Ok(within(&rows, from, to, &[])))?,
        2
    );
    assert_eq!(store.cursor("paxeer")?.expect("cutover cursor").position, 2);
    assert_eq!(store.chain_summary(1)?["tx_count"], 4);
    let at_cutover = tenant_history(&store)?;
    drop(store);
    let reopened = Store::open(&dir.db())?;
    assert_eq!(tenant_history(&reopened)?, at_cutover);
    assert_eq!(
        reopened
            .cursor("paxeer")?
            .expect("recovered cutover")
            .position,
        2
    );
    assert_eq!(
        node.step(&reopened)?,
        StepOutcome::Advanced {
            units: 1,
            position: 3
        }
    );
    assert_eq!(tenant_history(&reopened)?, expected);
    drop(reopened);
    let reopened = Store::open(&dir.db())?;
    assert_eq!(tenant_history(&reopened)?, expected);
    Ok(())
}

#[test]
fn unauthenticated_database_cannot_publish_changed_bodies() -> Checked {
    // These are precisely the untrusted rows admitted by PaxscanTls::Unauthenticated;
    // publication uses the same Backfill consumer for either database TLS policy.
    let server = Node::new(true, false);
    let node = server.ingester()?;
    let live = Store::open_in_memory()?;
    register_tenant(&live)?;
    node.step(&live)?;
    let expected = tenant_history(&live)?;
    assert!(!expected.0.is_empty());
    assert!(!expected.1.is_empty());
    let original = database_rows();
    let original_hashes: Vec<_> = original
        .transactions
        .iter()
        .map(|tx| tx.hash.clone())
        .collect();
    let mut altered = original.clone();
    altered.transactions[0].value = "999999999999999999999".into();
    altered.transactions[0].from_address_hash = vec![0x44; 20];
    for tx in &mut altered.transactions {
        tx.chain_id = Some("125000999".into());
    }
    // Change an ERC20 log and its redundant database row together, preserving
    // every transaction/block identifier and internal database consistency.
    altered.logs[0].data = vec![0; 32];
    altered.logs[0].data[31] = 7;
    altered.token_transfers[0].amount = Some("7".into());
    altered.logs[1].data[31] = 1; // Claim active instead of the committed paused status.
    assert_eq!(
        altered
            .transactions
            .iter()
            .map(|tx| tx.hash.clone())
            .collect::<Vec<_>>(),
        original_hashes
    );
    assert_eq!(altered.blocks, original.blocks);
    rpc_blocks(&altered)?;
    let imported = Store::open_in_memory()?;
    register_tenant(&imported)?;
    assert_eq!(
        Backfill::new(&node, 3, 3).run(&imported, |_, _| Ok(altered.clone()))?,
        3
    );
    assert_eq!(tenant_history(&imported)?, expected);
    assert_eq!(imported.chain(1)?, live.chain(1)?);
    assert_eq!(
        imported.chain(1)?.expect("authenticated tenant").status,
        "paused"
    );
    assert!(imported.transfers("pax:999")?.is_empty());
    assert!(server.full_blocks.load(Ordering::Relaxed) >= 6);
    assert!(server.receipts.load(Ordering::Relaxed) >= 10);

    let unavailable = Node::new(true, true);
    let unavailable_node = unavailable.ingester()?;
    let refused = Store::open_in_memory()?;
    register_tenant(&refused)?;
    assert!(matches!(
        Backfill::new(&unavailable_node, 3, 3).apply_range(&refused, 1, 3, &original),
        Err(IndexError::Source(_))
    ));
    assert!(refused.cursor("paxeer")?.is_none());
    assert!(refused.backfill_cursor("paxeer")?.is_none());
    assert!(refused.transfers("pax:1")?.is_empty());
    assert!(refused.events("pax:1")?.is_empty());
    assert_eq!(refused.chain_summary(1)?["tx_count"], 0);
    Ok(())
}
