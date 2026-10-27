use std::fs;
use std::io::ErrorKind;
use std::path::{Path, PathBuf};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    header::{
        validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator,
    },
    proto::{encode_length_delimited, encode_varint, encode_varint_field},
    ProofError,
};
use layerx_indexer::ai_market::{BlockCommitProof, FinalityAuthority};
use layerx_indexer::ai_reputation::{
    freshness, Admission, ArchiveLocator, FinalizedRead, Freshness, HistoryRequest,
    ReputationError, ReputationProjector, RootConflict,
};
use layerx_programs_ai_market::{
    codec::{self, decode_envelope, derive_market, derive_worker, encode_envelope, Envelope},
    dispatch,
    errors::{
        ApplicationError, F07_BINDING_MISMATCH, F07_EPOCH_NOT_SEALED, F07_FINALITY_UNAVAILABLE,
        F07_UNKNOWN_WORKER, WRONG_DOMAIN,
    },
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    queries::{
        bind_snapshot, read_state_chunk, FinalityEvidence, QueryError, ReadProof, StateCapture,
        BINDING_MAX_BYTES,
    },
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

enum Failure {
    Application(ApplicationError),
    Query(QueryError),
    Reputation(ReputationError),
    Commit(ProofError),
    Io(std::io::Error),
    Unexpected(&'static str),
}
impl From<ApplicationError> for Failure {
    fn from(error: ApplicationError) -> Self {
        Self::Application(error)
    }
}
impl From<QueryError> for Failure {
    fn from(error: QueryError) -> Self {
        Self::Query(error)
    }
}
impl From<ReputationError> for Failure {
    fn from(error: ReputationError) -> Self {
        Self::Reputation(error)
    }
}
impl From<ProofError> for Failure {
    fn from(error: ProofError) -> Self {
        Self::Commit(error)
    }
}
impl From<std::io::Error> for Failure {
    fn from(error: std::io::Error) -> Self {
        Self::Io(error)
    }
}
impl core::fmt::Debug for Failure {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        match self {
            Self::Application(error) => write!(f, "application refusal {error:?}"),
            Self::Query(error) => write!(f, "query refusal {error:?}"),
            Self::Reputation(error) => write!(f, "reputation refusal {error:?}"),
            Self::Commit(error) => write!(f, "commit refusal {error:?}"),
            Self::Io(error) => write!(f, "io failure {error}"),
            Self::Unexpected(what) => write!(f, "unexpected {what}"),
        }
    }
}
type Checked<T = ()> = Result<T, Failure>;
#[derive(Clone)]
struct CommitMaterial {
    header: Header,
    commit: Commit,
    store_proof: Vec<u8>,
}
/// Block commit material with its store proof digest.
type Certified = (CommitMaterial, [u8; 32]);
/// State bytes with the read proof and block commit offered for them.
type Offered = (Vec<u8>, ReadProof, Certified);

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
/// A nonzero digest distinct per (height, variant).
fn root(height: u64, variant: u8) -> Checked<Digest32> {
    let mut bytes = [variant; 32];
    bytes[..8].copy_from_slice(&height.to_be_bytes());
    Ok(Digest32::new(bytes)?)
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

/// Real F01 CREATE through `registry_ops::apply`.
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

/// Frames an `RP07` section byte for byte and checks it against the real codec both ways.
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

/// Discovery read, then pinned 8192-byte reads; returns the encoded chunk bodies.
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

fn proof(height: u64, native_state_root: Digest32) -> Checked<ReadProof> {
    Ok(ReadProof {
        chain: chain()?,
        program: program()?,
        native_state_root,
        observed_sequence: height,
        execution_height: height,
        block_hash: Digest32::new(header(height, native_state_root).hash())?,
    })
}

/// Independently recomputed snapshot identity and binding bytes of an accepted capture.
fn expected_binding(
    state_bytes: &[u8],
    read: ReadProof,
    checkpoint: [u8; 32],
) -> Checked<(Digest32, Vec<u8>)> {
    let mut buffer = vec![0; MAX_STATE_BYTES];
    let mut capture = StateCapture::new(&mut buffer);
    for body in read_all(state_bytes)? {
        capture.accept(&read, &body)?;
    }
    let (bytes, facts) = capture.finish()?;
    let binding = bind_snapshot(
        bytes,
        &facts,
        &FinalityEvidence {
            native_state_root: read.native_state_root,
            block_height: read.execution_height,
            block_hash: read.block_hash,
            app_hash: read.native_state_root,
            store_proof_digest: Digest32::new(checkpoint)?,
            rank: FINALIZED_RANK,
        },
        PUBLICATION_TIME_MS,
    )?;
    let mut encoded = [0; BINDING_MAX_BYTES];
    let n = binding.encode(&mut encoded)?;
    Ok((binding.snapshot_id()?, encoded[..n].to_vec()))
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

fn store_proof(height: u64, root: Digest32) -> Vec<u8> {
    let mut proof = root.bytes().to_vec();
    proof.extend_from_slice(&height.to_be_bytes());
    proof
}

fn certify_by(height: u64, root: Digest32, signers: &[(u8, bool)]) -> Checked<Certified> {
    let header = header(height, root);
    let commit = signed(&header, signers);
    let store_proof = store_proof(height, root);
    let id = Sha256::digest(&store_proof).into();
    Ok((
        CommitMaterial {
            header,
            commit,
            store_proof,
        },
        id,
    ))
}

fn certify(height: u64, root: Digest32) -> Checked<Certified> {
    certify_by(height, root, &[(1, false), (2, false), (3, false)])
}

fn offer(
    projector: &mut ReputationProjector,
    state_bytes: &[u8],
    read: ReadProof,
    certified: &Certified,
) -> Checked<Result<Admission, ReputationError>> {
    let chunks = read_all(state_bytes)?;
    Ok(projector.admit(&FinalizedRead {
        proof: read,
        chunks: &chunks,
        commit: BlockCommitProof {
            header: &certified.0.header,
            commit: &certified.0.commit,
            store_proof: &certified.0.store_proof,
        },
        publication_time_ms: PUBLICATION_TIME_MS,
    }))
}

fn fresh_store(name: &str) -> Checked<PathBuf> {
    let path = std::env::temp_dir().join(format!(
        "layerx-ai-reputation-{name}-{}.sqlite",
        std::process::id()
    ));
    for suffix in ["", "-wal", "-shm"] {
        let mut file = path.clone().into_os_string();
        file.push(suffix);
        match fs::remove_file(&file) {
            Ok(()) => {}
            Err(error) if error.kind() == ErrorKind::NotFound => {}
            Err(error) => return Err(error.into()),
        }
    }
    Ok(path)
}

fn open(path: &Path) -> Checked<ReputationProjector> {
    Ok(ReputationProjector::open(
        path,
        authority(),
        chain()?,
        program()?,
    )?)
}

fn request(worker: WorkerId, epoch: Option<u64>) -> Checked<HistoryRequest> {
    Ok(HistoryRequest {
        market: market()?,
        worker,
        epoch,
        minimum_finalized_height: 0,
    })
}

struct A09 {
    path: PathBuf,
    observed: WorkerId,
    unobserved: WorkerId,
    records: Vec<ReputationCurrent>,
    history: Vec<CompletedHistory>,
    state_bytes: Vec<u8>,
    projector: ReputationProjector,
}

/// Worker `observed` last observed at height 10000 (epoch 3), worker `unobserved` never.
fn a09(name: &str) -> Checked<A09> {
    let path = fresh_store(name)?;
    let (a, b) = (worker(1)?, worker(2)?);
    let history = (1..=3)
        .map(|epoch| completion(epoch, 7_000 + epoch * 1_000))
        .collect::<Checked<Vec<_>>>()?;
    let (section, records) = frame(
        vec![observed(a, 3, 10_000)?, unobserved(b, 5_000)?],
        &history,
    )?;
    let projector = open(&path)?;
    Ok(A09 {
        path,
        observed: a,
        unobserved: b,
        records,
        history,
        state_bytes: state_with(&section)?,
        projector,
    })
}

fn advance(fixture: &mut A09, height: u64) -> Checked<(Digest32, Certified)> {
    let native_root = root(height, 1)?;
    let certified = certify(height, native_root)?;
    assert_eq!(
        offer(
            &mut fixture.projector,
            &fixture.state_bytes,
            proof(height, native_root)?,
            &certified
        )?,
        Ok(Admission::Advanced { height })
    );
    Ok((native_root, certified))
}

#[test]
fn a09_freshness_is_measured_at_the_authenticated_finalized_height() -> Checked {
    let mut fixture = a09("a09-freshness")?;
    let a = fixture.observed;
    assert_eq!(
        fixture.projector.read_history(&request(a, None)?),
        Err(ReputationError::Refused(F07_FINALITY_UNAVAILABLE))
    );

    let (r14095, c14095) = advance(&mut fixture, 14_095)?;
    let (snapshot, binding) =
        expected_binding(&fixture.state_bytes, proof(14_095, r14095)?, c14095.1)?;
    let view = fixture.projector.read_history(&request(a, None)?)?;
    assert_eq!(view.finalized.height, 14_095);
    assert_eq!(view.finalized.native_state_root, r14095);
    assert_eq!(view.finalized.checkpoint_id, c14095.1);
    assert_eq!(view.finalized.snapshot_id, snapshot);
    assert_eq!(view.finalized.binding, binding);
    assert_eq!(view.finalized.rank, FINALIZED_RANK);
    assert_eq!(
        view.finalized.reputation_root,
        Presence::Present(reputation_root(market()?, 3, &fixture.records)?)
    );
    assert_eq!(view.current, observed(a, 3, 10_000)?);
    assert_eq!(view.freshness, Freshness::Fresh { age: 4_095 });
    assert_eq!(view.history, fixture.history);
    let never = fixture
        .projector
        .read_history(&request(fixture.unobserved, None)?)?;
    assert_eq!(never.current, unobserved(fixture.unobserved, 5_000)?);
    assert_eq!(never.freshness, Freshness::Unobserved);

    advance(&mut fixture, 14_096)?;
    let stale = fixture.projector.read_history(&request(a, None)?)?;
    assert_eq!(stale.finalized.height, 14_096);
    assert_eq!(stale.freshness, Freshness::Stale { age: 4_096 });
    assert_eq!(stale.current, observed(a, 3, 10_000)?);
    assert_eq!(stale.history, fixture.history);

    let r9999 = root(9_999, 1)?;
    let early = ReputationError::InvalidProof {
        finalized_height: 9_999,
        committed_height: 10_000,
    };
    assert_eq!(
        offer(
            &mut fixture.projector,
            &fixture.state_bytes,
            proof(9_999, r9999)?,
            &certify(9_999, r9999)?
        )?,
        Err(early.clone())
    );
    assert_eq!(freshness(9_999, &observed(a, 3, 10_000)?), Err(early));
    assert_eq!(
        fixture
            .projector
            .read_history(&request(a, None)?)?
            .finalized
            .height,
        14_096
    );
    Ok(())
}

#[test]
fn a09_cursor_is_monotonic_and_recovers_from_its_persisted_point() -> Checked {
    let mut fixture = a09("a09-cursor")?;
    let a = fixture.observed;
    let (r14096, c14096) = advance(&mut fixture, 14_096)?;
    let (r15000, c15000) = advance(&mut fixture, 15_000)?;
    assert_eq!(
        offer(
            &mut fixture.projector,
            &fixture.state_bytes,
            proof(15_000, r15000)?,
            &c15000
        )?,
        Ok(Admission::AlreadyAccepted { height: 15_000 })
    );
    let rollback = ReputationError::StaleProof {
        accepted: 15_000,
        offered: 14_096,
    };
    assert_eq!(
        offer(
            &mut fixture.projector,
            &fixture.state_bytes,
            proof(14_096, r14096)?,
            &c14096
        )?,
        Err(rollback.clone())
    );
    assert_eq!(
        fixture.projector.read_history(&HistoryRequest {
            minimum_finalized_height: 15_001,
            ..request(a, None)?
        }),
        Err(ReputationError::Refused(F07_FINALITY_UNAVAILABLE))
    );

    drop(fixture.projector);
    fixture.projector = open(&fixture.path)?;
    let recovered = fixture.projector.read_history(&HistoryRequest {
        minimum_finalized_height: 15_000,
        ..request(a, None)?
    })?;
    assert_eq!(recovered.finalized.height, 15_000);
    assert_eq!(recovered.finalized.native_state_root, r15000);
    assert_eq!(recovered.finalized.checkpoint_id, c15000.1);
    assert_eq!(recovered.freshness, Freshness::Stale { age: 5_000 });
    assert_eq!(
        offer(
            &mut fixture.projector,
            &fixture.state_bytes,
            proof(14_096, r14096)?,
            &c14096
        )?,
        Err(rollback)
    );
    advance(&mut fixture, 15_001)?;
    Ok(())
}

#[test]
fn a09_invalid_certificates_never_move_the_cursor() -> Checked {
    let mut fixture = a09("a09-invalid")?;
    let (_, c15000) = advance(&mut fixture, 15_000)?;
    let r16000 = root(16_000, 1)?;
    let valid = certify(16_000, r16000)?;
    let mut unpinned = valid.clone();
    let mut other_validators = validators();
    other_validators[3].pub_key = SigningKey::from_bytes(&[5; 32]).verifying_key().to_bytes();
    unpinned.0.header.validators_hash = validators_hash(&other_validators);
    unpinned.0.commit = signed(&unpinned.0.header, &[(1, false), (2, false), (3, false)]);
    let mut mismatched = valid.clone();
    mismatched.0.header = c15000.0.header.clone();
    let mut other_chain = valid.clone();
    other_chain.0.header.chain_id = "another-chain".to_owned();
    other_chain.0.commit = signed(&other_chain.0.header, &[(1, false), (2, false), (3, false)]);
    let invalid = [
        (
            proof(16_000, r16000)?,
            certify_by(16_000, r16000, &[(1, false), (2, true), (3, false)])?,
            ReputationError::Commit(ProofError::InvalidCommitSignature),
        ),
        (
            proof(16_000, r16000)?,
            certify_by(16_000, r16000, &[(1, false), (2, false)])?,
            ReputationError::Commit(ProofError::InsufficientVotingPower),
        ),
        (
            proof(16_000, r16000)?,
            unpinned,
            ReputationError::Commit(ProofError::ValidatorsHashMismatch),
        ),
        (
            proof(16_000, r16000)?,
            mismatched,
            ReputationError::Commit(ProofError::InvalidCommit),
        ),
        (
            proof(16_000, r16000)?,
            certify(16_000, root(16_000, 2)?)?,
            ReputationError::Refused(F07_BINDING_MISMATCH),
        ),
        (
            proof(16_000, r16000)?,
            certify(16_001, r16000)?,
            ReputationError::Refused(F07_BINDING_MISMATCH),
        ),
        (
            ReadProof {
                program: ProgramId::new([99; 32])?,
                ..proof(16_000, r16000)?
            },
            valid.clone(),
            ReputationError::Refused(WRONG_DOMAIN),
        ),
        (
            proof(16_000, r16000)?,
            other_chain,
            ReputationError::Refused(WRONG_DOMAIN),
        ),
    ];
    for (read, certified, refusal) in invalid {
        assert_eq!(
            offer(
                &mut fixture.projector,
                &fixture.state_bytes,
                read,
                &certified
            )?,
            Err(refusal)
        );
        assert_eq!(
            fixture
                .projector
                .read_history(&request(fixture.observed, None)?)?
                .finalized
                .height,
            15_000
        );
    }
    assert_eq!(
        offer(
            &mut fixture.projector,
            &fixture.state_bytes,
            proof(16_000, r16000)?,
            &valid
        )?,
        Ok(Admission::Advanced { height: 16_000 })
    );
    Ok(())
}

#[test]
fn a09_same_height_root_conflict_halts_the_projection_durably() -> Checked {
    let mut fixture = a09("a09-conflict")?;
    let a = fixture.observed;
    let (r15000, _) = advance(&mut fixture, 15_000)?;
    let other = root(15_000, 2)?;
    let conflicting = certify(15_000, other)?;
    let conflict = ReputationError::Conflict(Box::new(RootConflict {
        market: market()?,
        height: 15_000,
        accepted_root: r15000,
        conflicting_root: other,
        conflicting_checkpoint: conflicting.1,
    }));
    assert_eq!(
        offer(
            &mut fixture.projector,
            &fixture.state_bytes,
            proof(15_000, other)?,
            &conflicting
        )?,
        Err(conflict.clone())
    );
    let r16000 = root(16_000, 1)?;
    assert_eq!(
        offer(
            &mut fixture.projector,
            &fixture.state_bytes,
            proof(16_000, r16000)?,
            &certify(16_000, r16000)?
        )?,
        Err(conflict.clone())
    );
    assert_eq!(
        fixture.projector.read_history(&request(a, None)?),
        Err(conflict.clone())
    );
    drop(fixture.projector);
    let projector = open(&fixture.path)?;
    assert_eq!(projector.read_history(&request(a, None)?), Err(conflict));
    Ok(())
}

/// Two certified states of a full ring: epochs 1..=32 at 20000, then 2..=33 at 20100.
struct A11 {
    observer: WorkerId,
    reader: WorkerId,
    retained: Vec<CompletedHistory>,
    first: Offered,
    second: Offered,
}

fn a11() -> Checked<A11> {
    let observer = worker(1)?;
    let ring = |epochs: std::ops::RangeInclusive<u64>| {
        epochs
            .map(|epoch| completion(epoch, 19_000 + epoch * 10))
            .collect::<Checked<Vec<_>>>()
    };
    let records = |observed_epoch| -> Checked<Vec<ReputationCurrent>> {
        let mut records = (2..=32)
            .map(|nonce| unobserved(worker(nonce)?, 1_000))
            .collect::<Checked<Vec<_>>>()?;
        records.push(observed(
            observer,
            observed_epoch,
            19_000 + observed_epoch * 10,
        )?);
        Ok(records)
    };
    let (first_section, _) = frame(records(32)?, &ring(1..=32)?)?;
    assert_eq!(first_section.len(), 8_896);
    let retained = ring(2..=33)?;
    let (second_section, _) = frame(records(33)?, &retained)?;
    let certified = |height, section: &[u8]| -> Checked<Offered> {
        let native_root = root(height, 1)?;
        Ok((
            state_with(section)?,
            proof(height, native_root)?,
            certify(height, native_root)?,
        ))
    };
    Ok(A11 {
        observer,
        reader: worker(2)?,
        retained,
        first: certified(20_000, &first_section)?,
        second: certified(20_100, &second_section)?,
    })
}

fn admit_all(projector: &mut ReputationProjector, states: &[&Offered]) -> Checked {
    for (state_bytes, read, certified) in states {
        assert_eq!(
            offer(projector, state_bytes, *read, certified)?,
            Ok(Admission::Advanced {
                height: read.execution_height
            })
        );
    }
    Ok(())
}

#[test]
fn a11_finalized_eviction_reports_history_outside_retention_with_archive_locator() -> Checked {
    let fixture = a11()?;
    let reader = fixture.reader;
    let path = fresh_store("a11")?;
    let mut projector = open(&path)?;
    admit_all(&mut projector, &[&fixture.first, &fixture.second])?;

    let (first_state, first_read, (_, first_id)) = &fixture.first;
    let (first_snapshot, _) = expected_binding(first_state, *first_read, *first_id)?;
    let evicted = ReputationError::HistoryOutsideRetention {
        oldest_retained: Some(2),
        archive: Some(Box::new(ArchiveLocator {
            epoch: 1,
            finalized_height: 20_000,
            native_state_root: first_read.native_state_root,
            checkpoint_id: *first_id,
            snapshot_id: first_snapshot,
        })),
    };
    assert_eq!(
        projector.read_history(&request(reader, Some(1))?),
        Err(evicted.clone())
    );
    let retained = projector.read_history(&request(reader, Some(2))?)?;
    assert_eq!(retained.history, vec![completion(2, 19_020)?]);
    assert_eq!(retained.finalized.height, 20_100);
    assert_eq!(retained.current, unobserved(reader, 1_000)?);
    assert_eq!(retained.freshness, Freshness::Unobserved);
    assert_eq!(
        projector.read_history(&request(reader, Some(33))?)?.history,
        vec![completion(33, 19_330)?]
    );
    let all = projector.read_history(&request(fixture.observer, None)?)?;
    assert_eq!(all.history, fixture.retained);
    assert_eq!(all.freshness, Freshness::Fresh { age: 770 });
    assert_eq!(
        projector.read_history(&request(reader, Some(34))?),
        Err(ReputationError::Refused(F07_EPOCH_NOT_SEALED))
    );
    assert_eq!(
        projector.read_history(&request(worker(99)?, None)?),
        Err(ReputationError::Refused(F07_UNKNOWN_WORKER))
    );

    drop(projector);
    let projector = open(&path)?;
    assert_eq!(
        projector.read_history(&request(reader, Some(1))?),
        Err(evicted)
    );
    Ok(())
}

#[test]
fn a11_eviction_never_seen_by_this_projection_has_no_archive_locator() -> Checked {
    let fixture = a11()?;
    let mut late = open(&fresh_store("a11-late")?)?;
    admit_all(&mut late, &[&fixture.second])?;
    assert_eq!(
        late.read_history(&request(fixture.reader, Some(1))?),
        Err(ReputationError::HistoryOutsideRetention {
            oldest_retained: Some(2),
            archive: None,
        })
    );
    assert_eq!(
        late.read_history(&request(fixture.reader, None)?)?.history,
        fixture.retained
    );
    Ok(())
}
