use std::fmt;
use std::fs;
use std::future::Future;
use std::path::PathBuf;
use std::pin::pin;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};

use layerx_client::ai_market::{
    AiClientError, CommitFinality, NativeTerms, ObservedSnapshot, OperationJournal,
    OperationRecord, OperationRequest, OperationState, PROGRAM_CALL_ORDINAL,
};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::signer::Signer;
use layerx_programs_ai_market::codec::{decode_envelope, derive_market, encode_envelope, Envelope};
use layerx_programs_ai_market::dispatch;
use layerx_programs_ai_market::errors::ApplicationError;
use layerx_programs_ai_market::policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES};
use layerx_programs_ai_market::queries::{read_state_chunk, QueryError, ReadProof, FINALIZED_RANK};
use layerx_programs_ai_market::registry::{derive_rewards_account, F01_SECTION_CAP};
use layerx_programs_ai_market::registry_ops::{self, Outcome};
use layerx_programs_ai_market::rewards::{FundRequest, FUNDING_POLICY_VERSION};
use layerx_programs_ai_market::state::{self, Section, SharedState};
use layerx_programs_ai_market::types::{
    AssetId, Authentication, ChainDomain, Digest32, MarketId, Presence, PrincipalId, ProgramId,
    RequestId, RubricDigest,
};
use layerx_programs_ai_market::{MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::program_call::Resources;

const CHAIN: [u8; 32] = [10; 32];
const PROGRAM: [u8; 32] = [11; 32];
const OWNER: [u8; 32] = [12; 32];
const ASSET: [u8; 32] = [13; 32];
const REFUND: [u8; 32] = [14; 32];
const OWNER_SEED: [u8; 32] = [0x41; 32];
const ORIGIN: u64 = 1;
const NETWORK: u32 = 42;
static NEXT_JOURNAL: AtomicU64 = AtomicU64::new(1);

enum Failure {
    Application(ApplicationError),
    Query(QueryError),
    Client(AiClientError),
    Setup(String),
}

impl fmt::Debug for Failure {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Application(error) => write!(formatter, "application refusal {error:?}"),
            Self::Query(error) => write!(formatter, "query refusal {error:?}"),
            Self::Client(error) => write!(formatter, "client refusal {error:?}"),
            Self::Setup(detail) => write!(formatter, "setup failure {detail}"),
        }
    }
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

impl From<AiClientError> for Failure {
    fn from(error: AiClientError) -> Self {
        Self::Client(error)
    }
}

type Checked<T = ()> = Result<T, Failure>;

fn setup<E: fmt::Debug>(what: &'static str) -> impl FnOnce(E) -> Failure {
    move |error| Failure::Setup(format!("{what}: {error:?}"))
}

fn registry() -> Checked<ModuleRegistry> {
    let call = ActivityType::new(ModuleId::Programs, PROGRAM_CALL_ORDINAL)
        .map_err(setup("activity type"))?;
    let registration =
        ModuleRegistration::new(ModuleId::Programs, &[call]).map_err(setup("registration"))?;
    ModuleRegistry::new(&[registration]).map_err(setup("registry"))
}

fn chain() -> Checked<ChainDomain> {
    Ok(ChainDomain::new(CHAIN)?)
}

fn program() -> Checked<ProgramId> {
    Ok(ProgramId::new(PROGRAM)?)
}

fn market() -> Checked<MarketId> {
    Ok(derive_market(chain()?, program()?)?)
}

fn principal(bytes: [u8; 32]) -> Checked<PrincipalId> {
    Ok(PrincipalId::new(bytes)?)
}

fn digest(byte: u8) -> Checked<Digest32> {
    Ok(Digest32::new([byte; 32])?)
}

fn public_key(seed: [u8; 32]) -> [u8; 32] {
    LocalSigner::new(seed).public_key()
}

fn policy_with(rubric: u8) -> Checked<TaskPolicyV1> {
    Ok(TaskPolicyV1::bounded_default(
        1,
        1,
        PolicyCommitments {
            model_artifact: digest(1)?,
            dataset_artifact: [2; 32],
            benchmark_suite: digest(3)?,
            rubric: RubricDigest::new([rubric; 32])?,
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
    let length = state::encode_shared_state(shared, &mut out, &mut scratch)?;
    out.truncate(length);
    Ok(out)
}

/// Real F01 CREATE at the market origin through `registry_ops::apply`.
fn created_state() -> Checked<Vec<u8>> {
    let mut encoded_policy = vec![0; TASK_POLICY_BYTES];
    policy_with(4)?.encode(&mut encoded_policy)?;
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
    let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
    let length = encode_envelope(&envelope, &mut encoded)?;
    let validated = decode_envelope(&encoded[..length])?;
    let context = registry_ops::CallContext {
        chain: chain()?,
        program: program()?,
        principal: principal(OWNER)?,
        height: ORIGIN,
    };
    let mut section = vec![0; F01_SECTION_CAP];
    let mut event = vec![0; MAX_EVENT_BYTES];
    match registry_ops::apply(&context, None, &validated, &mut section, &mut event)? {
        Outcome::Applied { state, .. } => encode_shared(&state),
        Outcome::AlreadyApplied(_) => Err(Failure::Setup("fresh create retried".to_owned())),
    }
}

fn observed() -> Checked<ObservedSnapshot> {
    let state = created_state()?;
    let proof = ReadProof {
        chain: chain()?,
        program: program()?,
        native_state_root: digest(21)?,
        observed_sequence: 91,
        execution_height: 9,
        block_hash: digest(22)?,
    };
    let mut chunks = Vec::new();
    let mut offset = 0_u32;
    let mut revision = 0_u64;
    let mut pinned = [0; 32];
    loop {
        let mut request = revision.to_be_bytes().to_vec();
        request.extend_from_slice(&pinned);
        request.extend_from_slice(&offset.to_be_bytes());
        request.extend_from_slice(&8192_u16.to_be_bytes());
        let mut out = vec![0; 8244];
        let length = read_state_chunk(&state, &request, &mut out)?;
        out.truncate(length);
        let chunk = layerx_programs_ai_market::codec::decode_chunk_response(&out)?;
        revision = chunk.revision;
        pinned = chunk.digest.bytes();
        let total = chunk.total_bytes;
        chunks.push((proof, out));
        offset += 8192;
        if offset >= total {
            break;
        }
    }
    Ok(ObservedSnapshot::capture(&chunks)?)
}

#[test]
fn finalize_keeps_the_block_commit() -> Checked {
    let commit = CommitFinality::new(40, [31; 32], [32; 32], [33; 32])?;
    let snapshot = observed()?.finalize(Some(&commit), 5000)?;
    assert_eq!(snapshot.commit(), &commit);
    assert_eq!(snapshot.commit().block_height(), 40);
    assert_eq!(snapshot.commit().block_hash().bytes(), [31; 32]);
    assert_eq!(snapshot.commit().app_hash().bytes(), [32; 32]);
    assert_eq!(snapshot.commit().store_proof_digest().bytes(), [33; 32]);
    assert_eq!(snapshot.binding().block_height, 40);
    assert_eq!(snapshot.binding().block_hash, commit.block_hash());
    assert_eq!(snapshot.binding().app_hash, commit.app_hash());
    assert_eq!(
        snapshot.binding().store_proof_digest,
        commit.store_proof_digest()
    );
    assert_eq!(snapshot.binding().rank, FINALIZED_RANK);
    assert_eq!(snapshot.snapshot_id(), snapshot.binding().snapshot_id()?);
    let same_height = CommitFinality::new(9, [31; 32], [32; 32], [33; 32])?;
    assert_eq!(
        observed()?.finalize(Some(&same_height), 5000)?.commit(),
        &same_height
    );
    Ok(())
}

#[test]
fn finalize_refuses_a_zero_app_hash_or_an_older_commit() -> Checked {
    let unavailable = AiClientError::Query(QueryError::FinalityUnavailable);
    assert_eq!(
        CommitFinality::new(40, [31; 32], [0; 32], [33; 32]),
        Err(unavailable.clone())
    );
    assert_eq!(
        CommitFinality::new(40, [0; 32], [32; 32], [33; 32]),
        Err(unavailable.clone())
    );
    assert_eq!(observed()?.finalize(None, 5000), Err(unavailable));
    for height in [1, 8] {
        let older = CommitFinality::new(height, [31; 32], [32; 32], [33; 32])?;
        assert_eq!(
            observed()?.finalize(Some(&older), 5000),
            Err(AiClientError::Query(QueryError::BindingMismatch))
        );
    }
    Ok(())
}

fn signed_operation(registry: &ModuleRegistry) -> Checked<OperationRecord> {
    let commit = CommitFinality::new(9, [31; 32], [32; 32], [33; 32])?;
    let snapshot = observed()?.finalize(Some(&commit), 5000)?;
    let mut payload = vec![0; 57];
    let length = FundRequest {
        amount: 25,
        refund_recipient: layerx_programs_ai_market::types::AccountId::new(REFUND)?,
        policy_version: FUNDING_POLICY_VERSION,
        consent: true,
    }
    .encode(&mut payload)?;
    payload.truncate(length);
    let prepared = snapshot.prepare(
        registry,
        9,
        &OperationRequest {
            operation: dispatch::FUND,
            payload: &payload,
            actor: principal(OWNER)?,
            roster: None,
            sequence: 1,
            expiry: 1000000,
            request: RequestId::new([34; 32])?,
            required_rank: FINALIZED_RANK,
        },
        &NativeTerms {
            network_id: NETWORK,
            actor_did: b"did:layerx:paxai-owner",
            owner_public_key: public_key(OWNER_SEED),
            account_sequence: 1,
            idempotency_key: [35; 32],
            not_before: 100,
            not_after: 200,
            fee_limit: 1000,
            capabilities: &[0, 0],
            access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
            response_capacity: 16,
            resources: Resources([1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096]),
        },
    )?;
    let review = prepared.review(registry)?;
    let terms = review.terms().clone();
    let approval = review.approve(&terms, public_key(OWNER_SEED))?;
    Ok(ready(approval.sign(
        &LocalSigner::new(OWNER_SEED),
        registry,
        9,
    ))?)
}

fn ready<F: Future>(future: F) -> F::Output {
    struct Idle;
    impl Wake for Idle {
        fn wake(self: Arc<Self>) {}
    }
    let waker = Waker::from(Arc::new(Idle));
    let mut context = Context::from_waker(&waker);
    let mut future = pin!(future);
    match future.as_mut().poll(&mut context) {
        Poll::Ready(output) => output,
        Poll::Pending => panic!("signing future did not complete"),
    }
}

struct JournalDirectory(PathBuf);

impl Drop for JournalDirectory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

#[test]
fn journal_finalize_records_the_block_hash() -> Checked {
    let registry = registry()?;
    let signed = signed_operation(&registry)?;
    let mut encoded = signed.encode()?;
    encoded[8] = OperationState::Executed.code();
    encoded[152..156].copy_from_slice(&1_u32.to_be_bytes());
    let signed_body = encoded.split_off(159);
    encoded.truncate(156);
    encoded.push(1);
    encoded.extend_from_slice(&0_i32.to_be_bytes());
    encoded.push(1);
    encoded.extend_from_slice(&12_u64.to_be_bytes());
    encoded.push(0);
    encoded.extend_from_slice(&signed_body);
    let mut record = OperationRecord::decode(&encoded, &registry)?;
    assert_eq!(record.state(), OperationState::Executed);
    assert_eq!(record.global_sequence(), Some(12));
    let directory = JournalDirectory(std::env::temp_dir().join(format!(
        "layerx-ai-market-commit-{}-{}",
        std::process::id(),
        NEXT_JOURNAL.fetch_add(1, Ordering::Relaxed)
    )));
    let journal = OperationJournal::open(&directory.0)?;
    journal.persist(&record)?;
    let original = record.clone();
    let older = CommitFinality::new(11, [41; 32], [42; 32], [43; 32])?;
    assert_eq!(
        journal.finalize(&mut record, &older),
        Err(AiClientError::FinalityMismatch)
    );
    assert_eq!(record, original);
    assert_eq!(journal.load(&record.activity_id(), &registry)?, original);
    let commit = CommitFinality::new(12, [41; 32], [42; 32], [43; 32])?;
    assert_eq!(
        journal.finalize(&mut record, &commit)?,
        OperationState::Finalized
    );
    assert_eq!(record.checkpoint(), Some(commit.block_hash().bytes()));
    let reopened = OperationJournal::open(&directory.0)?;
    let persisted = reopened.load(&record.activity_id(), &registry)?;
    assert_eq!(persisted, record);
    assert_eq!(persisted.state(), OperationState::Finalized);
    assert_eq!(persisted.checkpoint(), Some([41; 32]));
    assert_eq!(persisted.signed_bytes(), signed.signed_bytes());
    assert_eq!(
        reopened.finalize(&mut record, &commit),
        Err(AiClientError::InvalidTransition {
            from: OperationState::Finalized
        })
    );
    Ok(())
}
