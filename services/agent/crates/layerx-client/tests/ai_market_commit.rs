#[path = "support/market_22_33_1546.rs"]
mod repair;

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
    OperationRecord, OperationRequest, OperationState, PreparedOperation, ReviewField, ENTRYPOINT,
    GUEST_ABI, PROGRAM_CALL_ORDINAL,
};
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::disclosure::DisclosedNativeOperation;
use layerx_crypto::evm_transaction::{self, Eip1559Call};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::payments::bind_call;
use layerx_crypto::signer::{sign_disclosed_call, Signer};
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
const NETWORK: u32 = 125;
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

fn owner_signer() -> Checked<LocalSigner> {
    let account = derive_from_seed(&OWNER_SEED, 0).map_err(setup("owner derivation"))?;
    Ok(LocalSigner::from_derived_account(&account))
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

fn prepared_operation(registry: &ModuleRegistry) -> Checked<PreparedOperation> {
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
    Ok(snapshot.prepare(
        registry,
        9,
        &OperationRequest {
            operation: dispatch::FUND,
            payload: &payload,
            actor: principal(OWNER)?,
            roster: None,
            sequence: 1,
            expiry: 200,
            request: RequestId::new([34; 32])?,
            required_rank: FINALIZED_RANK,
        },
        &NativeTerms {
            network_id: NETWORK,
            actor_did: b"did:layerx:paxai-owner",
            owner_public_key: owner_signer()?.public_key(),
            account_sequence: 1,
            idempotency_key: [34; 32],
            not_before: 0,
            not_after: 200,
            fee_limit: 1000,
            capabilities: &[0, 0],
            access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
            response_capacity: 16,
            resources: Resources([1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096]),
            gas_limit: 100_000,
            max_fee_per_gas: 30,
            max_priority_fee_per_gas: 2,
        },
    )?)
}

fn signed_operation(registry: &ModuleRegistry) -> Checked<OperationRecord> {
    let signer = owner_signer()?;
    let from = signer
        .evm_address()
        .ok_or_else(|| Failure::Setup("missing EVM key".to_owned()))?;
    let prepared = prepared_operation(registry)?;
    let call = evm_transaction::decode_unsigned(prepared.canonical_bytes())
        .map_err(setup("unsigned transaction"))?;
    let review = prepared.review_call(registry, call, from)?;
    let terms = review.terms().clone();
    let approval = review.approve(&terms, signer.public_key())?;
    Ok(ready(approval.sign(&signer, registry, 9))?)
}

#[test]
fn prepared_operation_signs_as_eip1559_program_call() -> Checked {
    let registry = registry()?;
    let prepared = prepared_operation(&registry)?;
    let intent = prepared.intent().bytes();
    let call = evm_transaction::decode_unsigned(prepared.canonical_bytes())
        .map_err(setup("unsigned transaction"))?;
    assert_eq!(prepared.canonical_bytes(), call.encode_unsigned());
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 1);
    assert_eq!(call.gas_limit, 100_000);
    assert_eq!(call.max_fee_per_gas, 30);
    assert_eq!(call.max_priority_fee_per_gas, 2);
    assert_eq!(&call.to[..18], &[0; 18]);
    assert_eq!(&call.to[18..], &[0x10, 0x1d]);
    assert_eq!(&call.data[..4], &[0x54, 0xbd, 0x2b, 0x48]);
    let signer = owner_signer()?;
    let from = signer
        .evm_address()
        .ok_or_else(|| Failure::Setup("missing EVM key".to_owned()))?;
    for changed in [
        Eip1559Call {
            gas_limit: call.gas_limit + 1,
            ..call.clone()
        },
        Eip1559Call {
            nonce: call.nonce + 1,
            ..call.clone()
        },
        Eip1559Call {
            max_fee_per_gas: call.max_fee_per_gas + 1,
            ..call.clone()
        },
        Eip1559Call {
            max_priority_fee_per_gas: call.max_priority_fee_per_gas + 1,
            ..call.clone()
        },
    ] {
        assert_eq!(
            prepared.clone().review_call(&registry, changed, from),
            Err(AiClientError::ReviewMismatch(ReviewField::Transaction))
        );
    }
    let disclosure = bind_call(&call, from).map_err(setup("program call disclosure"))?;
    assert_eq!(disclosure.fee_limit, 3_000_000);
    assert_eq!(disclosure.idempotency_key, [34; 32]);
    assert_eq!(disclosure.expiry.not_before, 0);
    assert_eq!(disclosure.expiry.not_after, 200);
    assert_eq!(disclosure.expiry.payload_expires_at, 200);
    let Some(DisclosedNativeOperation::ProgramCall(native)) = &disclosure.native_operation else {
        return Err(Failure::Setup("missing disclosed program call".to_owned()));
    };
    assert_eq!(native.program_id.bytes(), PROGRAM);
    assert_eq!(native.guest_abi, GUEST_ABI);
    assert_eq!(native.entrypoint, ENTRYPOINT);
    assert_eq!(
        decode_envelope(&native.calldata)?.request_digest()?.bytes(),
        intent
    );
    let review = prepared.clone().review(&registry, from)?;
    assert_eq!(review.terms().snapshot, prepared.snapshot_id());
    assert_eq!(review.terms().fee_limit, 1000);
    assert_eq!(review.terms().not_before, 0);
    assert_eq!(review.terms().not_after, 200);
    assert_eq!(review.terms().idempotency_key, [34; 32]);
    let approved = review.terms().clone();
    assert_eq!(
        review.clone().approve(&approved, [0; 32]),
        Err(AiClientError::UnauthorizedKey)
    );
    let mut changed = approved.clone();
    changed
        .transaction
        .as_mut()
        .ok_or_else(|| Failure::Setup("missing reviewed transaction".to_owned()))?
        .0
        .gas_limit += 1;
    assert_eq!(
        review.clone().approve(&changed, signer.public_key()),
        Err(AiClientError::ReviewMismatch(ReviewField::Transaction))
    );
    let signature = ready(sign_disclosed_call(&signer, &call, &disclosure, from))
        .map_err(setup("disclosed signing"))?;
    let raw = evm_transaction::assemble(&call, &signature);
    let (signed_call, signed_signature) =
        evm_transaction::decode_signed(&raw).map_err(setup("signed transaction"))?;
    assert_eq!(signed_call, call);
    assert_eq!(
        evm_transaction::recover_sender(&signed_call, &signed_signature)
            .map_err(setup("sender"))?,
        from
    );
    let record = OperationRecord::from_signed_transaction(raw.clone(), intent, approved.not_after)?;
    assert_eq!(record.intent(), intent);
    assert_eq!(record.signed_bytes(), raw);
    assert_eq!(record.protocol_version(), 2);
    assert_eq!(record.network_id(), NETWORK);
    assert_eq!(
        record.activity_id(),
        evm_transaction::transaction_hash(&raw)
    );
    assert_eq!(record.idempotency_key(), record.activity_id());
    assert_eq!(
        OperationRecord::decode(&record.encode()?, &registry)?,
        record
    );
    let approval = review.approve(&approved, signer.public_key())?;
    assert_eq!(ready(approval.sign(&signer, &registry, 9))?, record);
    Ok(())
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
    repair::receipt_sequence_and_height_survive_reopen();
    Ok(())
}
