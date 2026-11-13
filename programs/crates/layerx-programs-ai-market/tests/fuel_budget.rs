//! Meter every routed selector through the compiled production ABI-5 guest and compare
//! result, events and committed bytes with the native production router. The fixture uses
//! explicit release opt-level z and the deployment domain of Paxeer chain 125.
use layerx_ai_market_test_values::RequiredValue as _;
#[allow(dead_code)]
mod guest {
    include!("support/guest.rs");
}
use ed25519_dalek::{Signer, SigningKey};
use guest::CPU_FUEL as DEFAULT_CPU_FUEL;
use layerx_programs_ai_market::{
    admission::{
        admit_evaluator, Admission, AdmissionContext, AdmissionTable, ApprovalTerms,
        EvaluatorConsent, Participant, TABLE_MAX_BYTES,
    },
    aggregation_codec::{EpochAggregation, QualityStatus, WorkerAggregate},
    codec::{
        self, decode_envelope, derive_evaluator, derive_market, derive_task, derive_worker,
        encode_envelope, CommitScorePayload, Envelope, ReportBody, RevealScorePayload, ScoreVector,
    },
    commit_reveal::commitment,
    dispatch::{self, Buffers, Operation, Routed},
    epoch::{self, Frozen},
    errors::{ApplicationError, CodecResult, ARITHMETIC, NOT_FOUND},
    evaluators::{
        authority::{split_identity_section, EvaluatorRecord, EvaluatorRegion, LastRequest},
        model::{EvaluatorGrant, GrantTerms, SignedReport},
    },
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    registry::{derive_rewards_account, MarketHeader},
    registry_ops::{CallContext, PolicySection, PERMIT_METADATA, PERMIT_SUSPEND},
    reward_math::allocate,
    rewards::{decode_reward_state, FundRequest, FUNDING_POLICY_VERSION, REWARD_STATE_BYTES},
    state::{
        decode_shared_state, encode_shared_state, ActorSlot, Control, ReplayTable, Section,
        SharedState,
    },
    tasks::{self, SetBinding, TaskSet},
    types::{
        AccountId, AssetId, Authentication, ChainDomain, CommitmentDigest, Digest32,
        EvaluatorBinding, EvaluatorId, EvidenceRoot, FrozenBinding, MetadataDigest, Presence,
        PrincipalId, ProgramId, PublicKey32, ReportDigest, RequestDigest, RequestId, ResultDigest,
        RosterDigest, RubricDigest, Salt32, Signature64, Version, WorkerId, WorkerRosterEntry,
    },
    workers::{
        consent_digest, decode_manifest, WorkerCurrent, WorkerState, WorkerTable,
        WORKER_TABLE_MAX_BYTES,
    },
    MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
};
use layerx_programs_runtime::ValidatedModule;
use std::fmt;
use std::sync::Arc;
#[path = "fixtures/fund.rs"]
mod repair_478_fixture;

const CHAIN: [u8; 32] = layerx_programs_ai_market::MERGED_CHAIN_DOMAIN;
const PROGRAM: [u8; 32] = [11; 32];
const OWNER: [u8; 32] = [12; 32];
const ASSET: [u8; 32] = [13; 32];
const REFUND: [u8; 32] = [14; 32];
const KEEPER: u8 = 0x7f;
const REQUESTER: u8 = 0x30;
const CHALLENGER: u8 = 9;
const OPERATOR: u8 = 0x60;
const NOMINEE: u8 = 0x40;
const LAPSED: u8 = 0x41;
const SCHEDULED: u8 = 8;
const ORIGIN: u64 = 128;
const METADATA: [u8; 32] = [0x44; 32];
const ROLE_EXPIRY: u64 = 5000;
const SALT: [u8; 32] = [0x5a; 32];
/// Epoch 7 of the market at origin 128: T = 1024, commit [1088, 1104), reveal [1104, 1120).
const WORK_AT: u64 = 1030;
const COMMIT_AT: u64 = 1088;
const REVEAL_AT: u64 = 1104;
/// The two frozen workers of epoch 7 and the evaluators that commit and reveal.
const LOW: u8 = 0x20;
const HIGH: u8 = 0x21;
const REVEALERS: [u8; 3] = [2, 3, 4];
/// Every selector `dispatch::route` serves.
const ROUTED: [Operation; 72] = [
    dispatch::CREATE,
    dispatch::STAGE_POLICY,
    dispatch::CANCEL_POLICY,
    dispatch::SCHEDULE_ACTIVATION,
    dispatch::ADVANCE_ACTIVATION,
    dispatch::SUSPEND,
    dispatch::UPDATE_METADATA,
    dispatch::APPOINT_OPERATOR,
    dispatch::REVOKE_OPERATOR,
    dispatch::ADMIT_TASK,
    dispatch::ACCEPT_TASK,
    dispatch::CANCEL_TASK,
    dispatch::COMMIT_TASK_RESULT,
    dispatch::SEAL_TASK_SET,
    dispatch::OPEN_EPOCH,
    dispatch::EnrollWorker,
    dispatch::PublishMetadata,
    dispatch::SetDraining,
    dispatch::UndoDrain,
    dispatch::RotateDelegate,
    dispatch::RevokeDelegate,
    dispatch::RetireWorker,
    dispatch::AcceptEnrollment,
    dispatch::ExpireEnrollment,
    dispatch::ScheduleEvaluator,
    dispatch::RotateEvaluatorKey,
    dispatch::RevokeEvaluator,
    dispatch::ChallengeAssessment,
    dispatch::CommitScore,
    dispatch::RevealScore,
    dispatch::SealEvidence,
    dispatch::REQUEST_CLOSE,
    dispatch::ADVANCE_CLOSE,
    dispatch::BeginAggregation,
    dispatch::ProcessAggregation,
    dispatch::FinalizeAggregation,
    dispatch::FUND,
    dispatch::CLAIM,
    dispatch::EXPIRE_EPOCH_CLAIMS,
    dispatch::REFUND_FREE,
    dispatch::PRUNE_EPOCH,
    dispatch::ResetHistory,
    dispatch::SuspendHistory,
    dispatch::ResumeHistory,
    dispatch::ApproveAdmission,
    dispatch::RevokeAdmissionApproval,
    dispatch::AdmitWorker,
    dispatch::AdmitEvaluator,
    dispatch::Heartbeat,
    dispatch::RequestExit,
    dispatch::CancelExit,
    dispatch::PruneInactive,
    dispatch::AdministrativeRemove,
    dispatch::READ_HEADER,
    dispatch::READ_STATE_CHUNK,
    dispatch::OpenRoom,
    dispatch::PostAsk,
    dispatch::PostBid,
    dispatch::RecordDeal,
    dispatch::SettleLaps,
    dispatch::ChallengeLap,
    dispatch::ResolveChallenge,
    dispatch::SplitPayout,
    dispatch::LockBond,
    dispatch::SlashBond,
    dispatch::RefundBond,
    dispatch::CloseRoom,
    dispatch::RoomView,
    dispatch::DealView,
    dispatch::PeerView,
    dispatch::ReceiptBatchView,
    dispatch::BookView,
];

/// A failed journey step: an application refusal, a routed call that did not apply, or a
/// guest build or validation failure.
enum Failure {
    App(ApplicationError, &'static std::panic::Location<'static>),
    NotApplied(&'static str, String),
    Guest(String),
}
impl fmt::Debug for Failure {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::App(error, at) => write!(f, "application refusal {} at {at}", error_name(*error)),
            Self::NotApplied(op, routed) => write!(f, "{op} routed to {routed}"),
            Self::Guest(reason) => write!(f, "guest: {reason}"),
        }
    }
}
impl fmt::Display for Failure {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        fmt::Debug::fmt(self, f)
    }
}
impl std::error::Error for Failure {}
impl From<ApplicationError> for Failure {
    #[track_caller]
    fn from(error: ApplicationError) -> Self {
        Self::App(error, std::panic::Location::caller())
    }
}
fn error_name(error: ApplicationError) -> String {
    use layerx_programs_ai_market::errors::{APPLICATION_ERRORS, BAD_VERSION, CAPACITY};
    if error == BAD_VERSION {
        return "BAD_VERSION (0x0001)".into();
    }
    if error == CAPACITY {
        return "CAPACITY (0x0016)".into();
    }
    let name = APPLICATION_ERRORS
        .iter()
        .find(|entry| entry.code == error.code())
        .required_with("registered application error");
    format!("{}::{} (0x{:04x})", name.feature, name.name, error.code())
}
type Checked<T = ()> = Result<T, Failure>;

fn principal(n: u8) -> CodecResult<PrincipalId> {
    let mut bytes = [0x50; 32];
    bytes[0] = n;
    PrincipalId::new(bytes)
}
fn version() -> CodecResult<Version> {
    Version::new(1)
}
fn rubric() -> CodecResult<RubricDigest> {
    RubricDigest::new([4; 32])
}
fn policy(config: u64, minimum_evaluators: u8) -> CodecResult<TaskPolicyV1> {
    let mut policy = TaskPolicyV1::bounded_default(
        config,
        1,
        PolicyCommitments {
            model_artifact: Digest32::new([1; 32])?,
            dataset_artifact: [2; 32],
            benchmark_suite: Digest32::new([3; 32])?,
            rubric: rubric()?,
            task_schema: Digest32::new([5; 32])?,
            result_schema: Digest32::new([6; 32])?,
            service_terms: Digest32::new([7; 32])?,
        },
        100,
        1,
    )?;
    policy.minimum_evaluator_count = minimum_evaluators;
    Ok(policy)
}
fn policy_bytes(value: &TaskPolicyV1) -> CodecResult<Vec<u8>> {
    let mut bytes = vec![0; TASK_POLICY_BYTES];
    value.encode(&mut bytes)?;
    Ok(bytes)
}
/// The ed25519 delegate of worker `n`.
fn delegate_key(n: u8) -> SigningKey {
    SigningKey::from_bytes(&[0x90 + n; 32])
}
/// The frozen signing key of evaluator `n`.
fn evaluator_key(n: u8) -> SigningKey {
    SigningKey::from_bytes(&[n; 32])
}
/// The rotated signing key of evaluator or worker `n`.
fn rotated_key(n: u8) -> SigningKey {
    SigningKey::from_bytes(&[0xe0 ^ n; 32])
}
fn public(key: &SigningKey) -> PublicKey32 {
    PublicKey32(key.verifying_key().to_bytes())
}
/// The evidence root evaluator `n` seals in `epoch`.
fn root(n: u8, epoch: u64) -> [u8; 32] {
    let mut out = [0xe0 + n; 32];
    out[24..].copy_from_slice(&epoch.to_be_bytes());
    out
}
/// A request id unique per kind, actor and sequence.
fn tag(kind: u8, n: u8, sequence: u64) -> [u8; 32] {
    let mut out = [kind; 32];
    out[0] = n;
    out[1..9].copy_from_slice(&sequence.to_be_bytes());
    out
}
fn u64s(values: &[u64]) -> Vec<u8> {
    values.iter().flat_map(|v| v.to_be_bytes()).collect()
}

fn encode(state: &SharedState<'_>) -> CodecResult<Vec<u8>> {
    let mut out = vec![0; state.encoded_len()?];
    let mut scratch = vec![0; Section::Control.payload_cap()];
    let n = encode_shared_state(state, &mut out, &mut scratch)?;
    out.truncate(n);
    Ok(out)
}

/// One request envelope; `delegate` signs the request digest when present.
#[derive(Clone)]
struct Req {
    operation: Operation,
    actor: PrincipalId,
    epoch: u64,
    config: u64,
    roster: Presence<RosterDigest>,
    sequence: u64,
    request: [u8; 32],
    expiry: u64,
    delegate: Option<SigningKey>,
    payload: Vec<u8>,
}
fn req(operation: Operation, actor: PrincipalId, request: [u8; 32], payload: Vec<u8>) -> Req {
    Req {
        operation,
        actor,
        epoch: 0,
        config: u64::from(operation.metadata().boundary != dispatch::CallBoundary::ProgramRead),
        roster: Presence::Absent,
        sequence: 0,
        request,
        expiry: u64::MAX,
        delegate: None,
        payload,
    }
}
/// An envelope carrying the frozen binding `frozen`.
fn bound(
    operation: Operation,
    actor: PrincipalId,
    frozen: &FrozenBinding,
    request: [u8; 32],
    payload: Vec<u8>,
) -> Req {
    Req {
        epoch: frozen.epoch,
        config: frozen.config.get(),
        roster: Presence::Present(frozen.roster),
        ..req(operation, actor, request, payload)
    }
}
impl Req {
    fn encode(&self) -> CodecResult<Vec<u8>> {
        let chain = ChainDomain::new(CHAIN)?;
        let program = ProgramId::new(PROGRAM)?;
        let mut envelope = Envelope {
            operation: self.operation,
            chain,
            program,
            market: derive_market(chain, program)?,
            actor: self.actor,
            epoch: self.epoch,
            config: self.config,
            roster: self.roster,
            sequence: self.sequence,
            expiry: self.expiry,
            request: RequestId::new(self.request)?,
            payload: &self.payload,
            authentication: Authentication::Native,
        };
        let mut out = vec![0; MAX_RESULT_BYTES];
        if let Some(key) = &self.delegate {
            envelope.authentication = Authentication::Delegate {
                key: public(key),
                signature: Signature64([0; 64]),
            };
            let n = encode_envelope(&envelope, &mut out)?;
            let digest = decode_envelope(&out[..n])?.request_digest()?;
            envelope.authentication = Authentication::Delegate {
                key: public(key),
                signature: Signature64(key.sign(digest.as_bytes()).to_bytes()),
            };
        }
        let n = encode_envelope(&envelope, &mut out)?;
        out.truncate(n);
        Ok(out)
    }
    fn context(&self, at: u64) -> CodecResult<CallContext> {
        Ok(CallContext {
            chain: ChainDomain::new(CHAIN)?,
            program: ProgramId::new(PROGRAM)?,
            principal: self.actor,
            height: at,
        })
    }
}

/// Owned, decoded sections of one committed shared state value.
struct Parts {
    revision: u64,
    policy: Vec<u8>,
    workers: WorkerTable,
    region: Vec<u8>,
    reports: Vec<u8>,
    rewards: Vec<u8>,
    admission: AdmissionTable,
    reputation: Vec<u8>,
    replay: ReplayTable,
    features: Vec<u8>,
}
impl Parts {
    fn load(bytes: &[u8]) -> CodecResult<Self> {
        let state = decode_shared_state(bytes)?;
        let [policy, identity, reports, rewards, admission] = state.feature_sections;
        let (_, table) = layerx_programs_ai_market::reputation_transition::split_joint(admission)?;
        let reputation = admission[..admission.len() - table.len()].to_vec();
        let (workers, region) = split_identity_section(identity)?;
        Ok(Self {
            revision: state.revision,
            policy: policy.to_vec(),
            workers: WorkerTable::decode(workers)?,
            region: region.to_vec(),
            reports: reports.to_vec(),
            rewards: rewards.to_vec(),
            admission: AdmissionTable::decode(table)?,
            reputation,
            replay: state.control.replay,
            features: state.control.feature_bytes.to_vec(),
        })
    }
    fn market(&self) -> CodecResult<MarketHeader> {
        Ok(PolicySection::decode(&self.policy)?.header)
    }
    /// Commits at `self.revision`, keeping the F01 header revision equal to it.
    fn encode(&self) -> CodecResult<Vec<u8>> {
        let mut section = PolicySection::decode(&self.policy)?;
        section.header.state_revision = self.revision;
        let mut policy = vec![0; section.encoded_len()?];
        section.encode(&mut policy)?;
        let mut identity = vec![0; WORKER_TABLE_MAX_BYTES];
        let workers_len = self.workers.encode(&mut identity)?;
        identity.truncate(workers_len);
        identity.extend_from_slice(&self.region);
        let mut admission = vec![0; TABLE_MAX_BYTES];
        let admission_len = self.admission.encode(&mut admission)?;
        admission.truncate(admission_len);
        let admission = [self.reputation.as_slice(), admission.as_slice()].concat();
        encode(&SharedState {
            revision: self.revision,
            feature_sections: [&policy, &identity, &self.reports, &self.rewards, &admission],
            control: Control {
                replay: self.replay.clone(),
                feature_bytes: &self.features,
            },
        })
    }
    fn insert_grant(&mut self, grant: EvaluatorGrant, n: u8) -> CodecResult<()> {
        let mut region = EvaluatorRegion::decode(&self.region)?;
        region.insert(&EvaluatorRecord {
            grant,
            last: LastRequest {
                sequence: 1,
                request: RequestId::new([n; 32])?,
                digest: RequestDigest::new([0x33; 32])?,
                result: ResultDigest::new([0x34; 32])?,
            },
            rekey: None,
            revocation: None,
        })?;
        let mut out = vec![0; region.encoded_len()];
        let len = region.encode(&mut out)?;
        out.truncate(len);
        self.region = out;
        Ok(())
    }
}

fn admission_ctx(market: &MarketHeader, who: PrincipalId, at: u64) -> AdmissionContext<'_> {
    AdmissionContext {
        market,
        invoking_principal: who,
        height: at,
    }
}
/// Market-owner approval bound to the required effective epoch, expiring at its work end.
fn approve(
    table: &mut AdmissionTable,
    market: &MarketHeader,
    participant: Participant,
    owner: PrincipalId,
    delegate: PublicKey32,
    at: u64,
) -> CodecResult<(u64, Digest32)> {
    let effective = table.required_effective_epoch(&admission_ctx(market, owner, at))?;
    let terms = ApprovalTerms {
        participant,
        owner,
        enrollment_nonce_commitment: Digest32::new([7; 32])?,
        delegate,
        delegate_generation: 1,
        identity_commitment: Digest32::new([9; 32])?,
        effective_epoch: effective,
        config_version: 1,
        request: RequestId::new(participant.bytes())?,
        expiry_height: market.origin_height + effective * 128 + 64,
    };
    let digest = table.approve(&admission_ctx(market, market.owner_principal, at), &terms)?;
    Ok((effective, digest))
}
fn worker_record(
    worker: WorkerId,
    owner: PrincipalId,
    delegate: PublicKey32,
    slot: u8,
    at: u64,
) -> CodecResult<WorkerCurrent> {
    Ok(WorkerCurrent {
        worker,
        owner,
        delegate,
        metadata: MetadataDigest::new(METADATA)?,
        generation: 1,
        key_version: 1,
        metadata_revision: 1,
        valid_from: at,
        expiry: at + 4096,
        revocation_sequence: 0,
        effective_epoch: 0,
        last_sequence: 0,
        last_request_id: [0; 32],
        last_request_digest: [0; 32],
        last_result_digest: [0; 32],
        state: WorkerState::Enrolled,
        slot,
        last_metadata_height: at,
    })
}
fn roster_entry(record: &WorkerCurrent) -> CodecResult<WorkerRosterEntry> {
    Ok(WorkerRosterEntry {
        worker: record.worker,
        owner: record.owner,
        recipient: AccountId::new(record.owner.bytes())?,
        generation: version()?,
        key_version: version()?,
        public_key: record.delegate,
        metadata: record.metadata,
    })
}
fn grant(
    market: &MarketHeader,
    owner: PrincipalId,
    nonce: u8,
    signing_key: PublicKey32,
    effective: u64,
) -> CodecResult<EvaluatorGrant> {
    EvaluatorGrant::nominate(
        market.market_id,
        owner,
        [nonce; 32],
        GrantTerms {
            rubric: rubric()?,
            grant_version: version()?,
            key_version: version()?,
            signing_key,
            effective_epoch: effective,
            expiry_epoch_exclusive: effective + 32,
        },
    )
}
/// Canonical 36-byte score entries in the given order.
fn entries(pairs: &[(WorkerId, u32)]) -> Vec<u8> {
    let mut out = Vec::new();
    for (worker, score) in pairs {
        out.extend_from_slice(worker.as_bytes());
        out.extend_from_slice(&score.to_be_bytes());
    }
    out
}
/// The evaluator signature over the attestation digest of `body`.
fn sign<'a>(body: ReportBody<'a>, key: &SigningKey) -> CodecResult<SignedReport<'a>> {
    let digest = codec::attestation_digest(codec::report_digest(&body)?)?;
    Ok(SignedReport {
        body,
        signature: Signature64(key.sign(&digest.bytes()).to_bytes()),
    })
}
/// The canonical worker manifest of `record` at metadata `revision`.
fn manifest(
    market: &MarketHeader,
    record: &WorkerCurrent,
    revision: u64,
    valid_from: u64,
    expiry: u64,
) -> CodecResult<Vec<u8>> {
    let uri = b"https://worker.example/paxai/v1";
    let mut v = layerx_programs_ai_market::SCHEMA_VERSION
        .to_be_bytes()
        .to_vec();
    v.extend_from_slice(market.market_id.as_bytes());
    v.extend_from_slice(record.worker.as_bytes());
    v.extend_from_slice(record.owner.as_bytes());
    for n in [
        record.generation,
        record.key_version,
        revision,
        valid_from,
        expiry,
    ] {
        v.extend_from_slice(&n.to_be_bytes());
    }
    v.extend_from_slice(&[60; 32]);
    v.extend_from_slice(&1u16.to_be_bytes());
    v.push(1);
    v.push(1);
    v.extend_from_slice(&[50; 32]);
    for d in 0..4u8 {
        v.extend_from_slice(&[61 + d; 32]);
    }
    for n in [1024u32; 4] {
        v.extend_from_slice(&n.to_be_bytes());
    }
    v.push(1);
    v.extend_from_slice(&60_000u32.to_be_bytes());
    v.extend_from_slice(&4u16.to_be_bytes());
    v.push(1);
    v.extend_from_slice(&1u16.to_be_bytes());
    v.push(1);
    v.extend_from_slice(
        &u32::try_from(uri.len())
            .map_err(|_| ARITHMETIC)?
            .to_be_bytes(),
    );
    v.extend_from_slice(uri);
    v.extend_from_slice(&[70; 32]);
    v.push(1);
    v.extend_from_slice(&1u16.to_be_bytes());
    v.extend_from_slice(&1u16.to_be_bytes());
    v.extend_from_slice(&[71; 32]);
    v.extend_from_slice(&[72; 32]);
    v.extend_from_slice(&0u32.to_be_bytes());
    Ok(v)
}

fn guest_module() -> Checked<Arc<ValidatedModule>> {
    guest::module().map_err(|e| Failure::Guest(e.to_string()))
}

/// What the in-process route decided for one call.
struct Expected<'a> {
    state: &'a [u8],
    topic: &'a [u8],
    event: &'a [u8],
    result: &'a [u8],
}
/// The application error a refusal frame carries, or its raw length.
fn describe(frame: &[u8]) -> String {
    codec::decode_result(frame).map_or_else(
        |_| format!("{} unframed bytes", frame.len()),
        |result| match result.error {
            Some(error) => error_name(error),
            None => "no application error".into(),
        },
    )
}
/// Runs the guest over `current` and compares its response, event and stored state with the
/// routed decision.
fn run_guest(
    module: &Arc<ValidatedModule>,
    ctx: &CallContext,
    current: Option<&[u8]>,
    envelope: &[u8],
    expected: &Expected<'_>,
) -> Checked<(Option<u64>, Result<(), String>)> {
    if envelope.get(8..10) == Some(&[0x05, 0x02]) {
        AGGREGATION_INPUT.with(|input| {
            let mut input = input.borrow_mut();
            if let Some(rows) = input.as_mut() {
                if rows.is_empty() {
                    rows.push((
                        *ctx,
                        current.required_with("aggregation state").to_vec(),
                        envelope.to_vec(),
                    ));
                }
            }
        });
    }
    let asset = match current {
        Some(bytes) => {
            PolicySection::decode(decode_shared_state(bytes)?.section(Section::PolicyLifecycle)?)?
                .header
                .funding_asset
                .bytes()
        }
        None => ASSET,
    };
    let mut host = guest::Guest::new(module.clone(), ctx.program.bytes(), asset)
        .map_err(|e| Failure::Guest(e.to_string()))?;
    host.ledger.balances.insert(ctx.principal.bytes(), 10_000);
    if let Some(bytes) = current {
        host.seed(bytes)
            .map_err(|e| Failure::Guest(e.to_string()))?;
        let state = decode_shared_state(bytes)?;
        if let Ok(rewards) = layerx_programs_ai_market::state::settlement_rewards(&state, None) {
            let ledger = rewards.ledger()?;
            host.ledger.balances.insert(
                host.account,
                ledger.free + ledger.reserved + ledger.liability,
            );
        }
    }
    if let Some(bytes) = current {
        let state = decode_shared_state(bytes)?;
        if let Ok(canaries) =
            layerx_programs_ai_market::canary::CanaryRegion::decode(state.control.feature_bytes)
        {
            for row in canaries.records() {
                let row = row?;
                if matches!(
                    row.status,
                    layerx_programs_ai_market::canary::ChallengeStatus::Open
                        | layerx_programs_ai_market::canary::ChallengeStatus::Miss
                ) {
                    *host.ledger.balances.entry(host.account).or_default() += row.deposit;
                }
            }
        }
    }
    if let Some(bytes) = current {
        let held = layerx_programs_ai_market::bonds::held_from_state(bytes)?;
        *host.ledger.balances.entry(host.account).or_default() += held;
    }
    let mut host = host.restart().map_err(|e| Failure::Guest(e.to_string()))?;
    let record = match host.call(ctx, envelope, Some(expected.state)) {
        Ok(r) => r,
        Err(e) => return Ok((None, Err(e.to_string()))),
    };
    let stored = host.state().map_err(|e| Failure::Guest(e.to_string()))?;
    let events: Vec<(&[u8], &[u8])> = record
        .events
        .iter()
        .map(|e| (e.topic.as_slice(), e.data.as_slice()))
        .collect();
    let expected_events = if expected.event.is_empty() {
        vec![]
    } else {
        vec![(expected.topic, expected.event)]
    };
    let verdict = if record.response != expected.result {
        Err("response differs".to_string())
    } else if events != expected_events {
        Err("events differ".to_string())
    } else if stored.as_deref() != Some(expected.state) {
        Err("stored state differs".to_string())
    } else {
        Ok(())
    };
    Ok((Some(record.fuel), verdict))
}

/// One measured guest call.
struct Row {
    operation: Operation,
    height: u64,
    state: usize,
    fuel: Option<u64>,
    verdict: Result<(), String>,
}

#[allow(dead_code)]
#[path = "fixtures/f11_journeys.rs"]
mod f11_journeys;
thread_local! {
    static F11_ROWS: std::cell::RefCell<Option<(Arc<ValidatedModule>, Vec<Row>)>> = const { std::cell::RefCell::new(None) };
    static AGGREGATION_INPUT: std::cell::RefCell<Option<Vec<AggregationSample>>> = const { std::cell::RefCell::new(None) };
}
fn f11_observe(
    ctx: &CallContext,
    current: &[u8],
    envelope: &[u8],
    next: &[u8],
    event: &[u8],
    result: &[u8],
    routed: Routed,
) {
    let selector = u16::from_be_bytes(envelope[8..10].try_into().required());
    let operation = Operation::decode(selector).required();
    if operation.metadata().feature != 11 || matches!(routed, Routed::Refused { .. }) {
        return;
    }
    F11_ROWS.with(|rows| {
        let mut rows = rows.borrow_mut();
        let Some((module, rows)) = rows.as_mut() else {
            return;
        };
        let state = if matches!(routed, Routed::Applied { .. }) {
            next
        } else {
            current
        };
        let mut topic = vec![0; 64];
        if event.is_empty() {
            topic.clear();
        } else {
            let n = codec::event_topic(operation, &mut topic).required();
            topic.truncate(n);
        }
        let expected = Expected {
            state,
            topic: &topic,
            event,
            result,
        };
        let (fuel, verdict) = run_guest(module, ctx, Some(current), envelope, &expected).required();
        rows.push(Row {
            operation,
            height: ctx.height,
            state: current.len(),
            fuel,
            verdict,
        });
    });
}
fn f11_journey(module: Arc<ValidatedModule>) -> Vec<Row> {
    F11_ROWS.with(|rows| *rows.borrow_mut() = Some((module, Vec::new())));
    f11_journeys::run();
    F11_ROWS.with(|rows| rows.borrow_mut().take().required().1)
}

/// The committed state of the journey's market, its next owner sequence and every measured
/// guest call.
struct Journey {
    module: Arc<ValidatedModule>,
    bytes: Vec<u8>,
    owner_sequence: u64,
    rows: Vec<Row>,
}

/// Routed calls: decided in-process, then measured in the guest.
impl Journey {
    /// Routes `call` at `at` over the committed bytes, requires `Applied`, runs the guest over
    /// the same bytes and commits the routed next state.
    fn send(&mut self, call: &Req, at: u64) -> Checked {
        self.send_step(call, at).map(|_| ())
    }
    fn send_step(&mut self, call: &Req, at: u64) -> Checked<Vec<u8>> {
        let mut prepared = call.clone();
        if matches!(call.operation.selector(), 0x0601 | 0x0701..=0x0703 | 0x0801..=0x0809)
            && !self.bytes.is_empty()
        {
            let parts = self.parts()?;
            if let Some(epoch) = parts.admission.current_epoch() {
                prepared.epoch = epoch;
                prepared.roster = Presence::Present(
                    layerx_programs_ai_market::state::settlement_rewards(
                        &decode_shared_state(&self.bytes)?,
                        None,
                    )?
                    .row(epoch)?
                    .roster,
                );
            }
        }
        let call = &prepared;
        let envelope = call.encode()?;
        let ctx = call.context(at)?;
        let current = (!self.bytes.is_empty()).then_some(self.bytes.as_slice());
        let mut next = vec![0; MAX_STATE_BYTES];
        let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
        let mut event = vec![0; MAX_EVENT_BYTES];
        let mut result = vec![0; MAX_RESULT_BYTES];
        let name = call.operation.metadata().name;
        println!(
            "native {name} at height {at}, state bytes {}",
            self.bytes.len()
        );
        let routed = dispatch::route(
            &ctx,
            &envelope,
            current,
            Buffers {
                next: &mut next,
                scratch: &mut scratch,
                event: &mut event,
                result: &mut result,
            },
        )
        .map_err(|e| Failure::NotApplied(name, format!("height {at}: {e:?}")))?;
        if let Routed::Unchanged { result_len } = routed {
            return self.verify_unchanged_call(call, at, &envelope, &result[..result_len]);
        }
        let Routed::Applied {
            operation,
            state_len,
            event_len,
            result_len,
            ..
        } = routed
        else {
            let detail = match routed {
                Routed::Refused { result_len } | Routed::Unchanged { result_len } => {
                    describe(&result[..result_len])
                }
                Routed::Applied { .. } => String::new(),
            };
            return Err(Failure::NotApplied(name, format!("{routed:?} {detail}")));
        };
        let mut topic = [0; 64];
        let topic_len = codec::event_topic(operation, &mut topic)?;
        let expected = Expected {
            state: &next[..state_len],
            topic: &topic[..topic_len],
            event: &event[..event_len],
            result: &result[..result_len],
        };
        Self::verify_applied_transition(operation, &ctx, current, &expected)?;
        let (fuel, verdict) = run_guest(&self.module, &ctx, current, &envelope, &expected)?;
        println!("guest {name} at height {at}: fuel {fuel:?}, {verdict:?}");
        self.rows.push(Row {
            operation,
            height: at,
            state: self.bytes.len(),
            fuel,
            verdict,
        });
        let payload = codec::decode_result(&result[..result_len])?
            .payload
            .to_vec();
        next.truncate(state_len);
        self.bytes = next;
        Ok(payload)
    }
    fn verify_applied_transition(
        operation: Operation,
        ctx: &CallContext,
        current: Option<&[u8]>,
        expected: &Expected<'_>,
    ) -> Checked {
        if matches!(
            operation,
            dispatch::OPEN_EPOCH
                | dispatch::RevealScore
                | dispatch::BeginAggregation
                | dispatch::ProcessAggregation
                | dispatch::FinalizeAggregation
        ) {
            let full = decode_shared_state(expected.state)?;
            layerx_programs_ai_market::state::settlement_rewards(&full, None)?;
            layerx_programs_ai_market::evaluators::admission::ReportRegion::decode(
                full.section(Section::CurrentReports)?,
            )?;
            let payload = codec::decode_result(expected.result)?.payload;
            let partial = operation == dispatch::OPEN_EPOCH
                && u16::from_be_bytes(
                    payload[payload.len() - 4..payload.len() - 2]
                        .try_into()
                        .required(),
                ) < u16::from_be_bytes(payload[payload.len() - 2..].try_into().required());
            if partial {
                assert!(epoch::captured_roster(&full)?.is_none());
                let header = layerx_programs_ai_market::queries::read_header(
                    expected.state,
                    ctx.chain,
                    ctx.program,
                )?;
                assert_eq!(header.epoch, Presence::Absent);
                assert_eq!(header.roster, Presence::Absent);
                let old = codec::decode_state(current.ok_or(NOT_FOUND)?)?;
                let new = codec::decode_state(expected.state)?;
                for section in [
                    Section::IdentityRoster,
                    Section::CurrentReports,
                    Section::SettlementClaims,
                    Section::ReputationAdmission,
                    Section::Control,
                ] {
                    assert_eq!(old.sections[section.index()], new.sections[section.index()]);
                }
            } else {
                assert!(epoch::captured_roster(&full)?.is_some());
            }
            if operation == dispatch::FinalizeAggregation {
                let cursor = u16::from_be_bytes(payload[payload.len() - 2..].try_into().required());
                let count = u16::from_be_bytes(payload[64..66].try_into().required());
                if cursor < count {
                    let old = codec::decode_state(current.ok_or(NOT_FOUND)?)?;
                    let new = codec::decode_state(expected.state)?;
                    assert_eq!(
                        old.sections[Section::ReputationAdmission.index()],
                        new.sections[Section::ReputationAdmission.index()]
                    );
                    assert_eq!(
                        &old.sections[Section::SettlementClaims.index()]
                            [..layerx_programs_ai_market::rewards::REWARD_STATE_BYTES],
                        &new.sections[Section::SettlementClaims.index()]
                            [..layerx_programs_ai_market::rewards::REWARD_STATE_BYTES]
                    );
                }
            }
        }
        Ok(())
    }
    fn verify_unchanged_call(
        &mut self,
        call: &Req,
        at: u64,
        envelope: &[u8],
        result: &[u8],
    ) -> Checked<Vec<u8>> {
        let ctx = call.context(at)?;
        let current = (!self.bytes.is_empty()).then_some(self.bytes.as_slice());
        let name = call.operation.metadata().name;
        let result_len = result.len();
        if matches!(
            call.operation,
            dispatch::READ_HEADER | dispatch::READ_STATE_CHUNK
        ) {
            let mut strict = vec![0; MAX_RESULT_BYTES];
            let len = if call.operation == dispatch::READ_HEADER {
                let header = layerx_programs_ai_market::queries::read_header(
                    &self.bytes,
                    ctx.chain,
                    ctx.program,
                )?;
                codec::encode_read_header(&header, &mut strict)?
            } else {
                layerx_programs_ai_market::queries::read_state_chunk(
                    &self.bytes,
                    &call.payload,
                    &mut strict,
                )?
            };
            let frame = codec::decode_result(&result[..result_len])?;
            assert_eq!(
                frame.payload,
                &strict[..len],
                "stored read differs from the strict reader"
            );
        }
        let expected = Expected {
            state: &self.bytes,
            topic: &[],
            event: &[],
            result: &result[..result_len],
        };
        let (fuel, verdict) = run_guest(&self.module, &ctx, current, envelope, &expected)?;
        println!("guest {name} at height {at}: fuel {fuel:?}, {verdict:?}");
        self.rows.push(Row {
            operation: call.operation,
            height: at,
            state: self.bytes.len(),
            fuel,
            verdict,
        });
        Ok(codec::decode_result(&result[..result_len])?
            .payload
            .to_vec())
    }

    fn parts(&self) -> CodecResult<Parts> {
        Parts::load(&self.bytes)
    }
    fn header(&self) -> CodecResult<MarketHeader> {
        self.parts()?.market()
    }
    fn revision(&self) -> CodecResult<u64> {
        Ok(decode_shared_state(&self.bytes)?.revision)
    }
    /// An owner-authorized F01 registry call carrying the expected revision first.
    fn owner(&mut self, operation: Operation, tail: &[u8], at: u64) -> Checked {
        let mut payload = self.revision()?.to_be_bytes().to_vec();
        payload.extend_from_slice(tail);
        let call = Req {
            config: self.header()?.active_config_version,
            sequence: self.owner_sequence,
            ..req(
                operation,
                PrincipalId::new(OWNER)?,
                tag(0x20, 0, self.owner_sequence),
                payload,
            )
        };
        self.send(&call, at)?;
        self.owner_sequence += 1;
        Ok(())
    }
    /// An owner-authorized F03 authority call bound to `frozen`.
    fn owner_bound(
        &mut self,
        operation: Operation,
        frozen: &FrozenBinding,
        payload: Vec<u8>,
        at: u64,
    ) -> Checked {
        let call = Req {
            sequence: self.owner_sequence,
            ..bound(
                operation,
                PrincipalId::new(OWNER)?,
                frozen,
                tag(0x30, 0, self.owner_sequence),
                payload,
            )
        };
        self.send(&call, at)?;
        self.owner_sequence += 1;
        Ok(())
    }
    /// A native owner F02 call at the owner sequence.
    fn owner_worker(&mut self, operation: Operation, payload: Vec<u8>, at: u64) -> Checked {
        let call = Req {
            sequence: self.owner_sequence,
            ..req(
                operation,
                PrincipalId::new(OWNER)?,
                tag(0x21, 0, self.owner_sequence),
                payload,
            )
        };
        self.send(&call, at)?;
        self.owner_sequence += 1;
        Ok(())
    }
    /// Permissionless `OPEN_EPOCH` of the clock epoch of `at`.
    fn open(&mut self, at: u64) -> Checked<Frozen> {
        let before = self.bytes.clone();
        let mut next = vec![0; MAX_STATE_BYTES];
        let mut scratch = vec![0; epoch::OPEN_SCRATCH_BYTES];
        let preview = epoch::preview_open(&self.bytes, at, &mut next, &mut scratch)
            .map_err(|e| Failure::NotApplied("preview_open", format!("height {at}: {e:?}")))?;
        let call = Req {
            epoch: preview.epoch,
            config: preview.config.get(),
            roster: Presence::Present(preview.roster),
            ..req(
                dispatch::OPEN_EPOCH,
                principal(KEEPER)?,
                tag(0x60, 0, preview.epoch),
                Vec::new(),
            )
        };
        let mut previous = 0;
        loop {
            let payload = self.send_step(&call, at)?;
            let cursor = u16::from_be_bytes(
                payload[payload.len() - 4..payload.len() - 2]
                    .try_into()
                    .required(),
            );
            let count = u16::from_be_bytes(payload[payload.len() - 2..].try_into().required());
            assert!(cursor > previous && cursor <= count);
            assert!(cursor - previous <= 8);
            if cursor == count {
                break;
            }
            previous = cursor;
        }
        // Compare the chunked producer with the strict, complete F07 transition.
        // This also checks records from earlier chunks and retained history.
        let old = decode_shared_state(&before)?;
        let after = decode_shared_state(&self.bytes)?;
        let (original, _) = layerx_programs_ai_market::reputation_transition::split_joint(
            old.section(Section::ReputationAdmission)?,
        )?;
        if let Some(original) = original {
            use layerx_programs_ai_market::reputation_transition::{
                open_segments, prune_released, split_joint, SegmentBinding,
            };
            let binding = |state: &layerx_programs_ai_market::state::SharedState<'_>| -> CodecResult<SegmentBinding> {
                let policy = PolicySection::decode(state.section(Section::PolicyLifecycle)?)?;
                Ok(SegmentBinding {
                    config: Version::new(policy.header.active_config_version)?,
                    policy: Digest32::new(policy.current.digest()?.bytes())?,
                    model: policy.current.commitments.model_artifact,
                })
            };
            let captured = epoch::captured_roster(&after)?.ok_or(NOT_FOUND)?;
            let workers = WorkerTable::decode(
                layerx_programs_ai_market::evaluators::authority::split_identity_section(
                    after.section(Section::IdentityRoster)?,
                )?
                .0,
            )?;
            let expected = open_segments(
                &original.state,
                binding(&old)?,
                binding(&after)?,
                captured.workers(),
                |worker| !matches!(workers.get(worker), Some(r) if r.state != WorkerState::Retired),
                at,
            )?;
            let rewards = layerx_programs_ai_market::state::settlement_rewards(&after, None)?;
            let expected = prune_released(&expected, |epoch| match rewards.row(epoch) {
                Ok(_) => Ok(true),
                Err(NOT_FOUND) => Ok(false),
                Err(error) => Err(error),
            })?;
            let (actual, _) = split_joint(after.section(Section::ReputationAdmission)?)?;
            assert_eq!(
                actual.ok_or(NOT_FOUND)?.state,
                expected,
                "chunked opening must match the strict full transition"
            );
        }
        Ok(preview)
    }
    /// Permissionless `ADVANCE_ACTIVATION`; the lifecycle becomes ACTIVE.
    fn advance(&mut self, at: u64) -> Checked {
        let header = self.header()?;
        let mut payload = self.revision()?.to_be_bytes().to_vec();
        payload.extend_from_slice(&header.activation_epoch.to_be_bytes());
        let call = Req {
            config: header.active_config_version,
            ..req(
                dispatch::ADVANCE_ACTIVATION,
                principal(KEEPER)?,
                [0x5f; 32],
                payload,
            )
        };
        self.send(&call, at)
    }
}

/// Producers of the prerequisites no routed operation writes.
impl Journey {
    /// Applies `change` to the decoded sections and commits it at the next revision.
    fn edit<T>(
        &mut self,
        change: impl FnOnce(&mut Parts, &MarketHeader) -> CodecResult<T>,
    ) -> CodecResult<T> {
        let mut parts = self.parts()?;
        let market = parts.market()?;
        let out = change(&mut parts, &market)?;
        parts.revision += 1;
        self.bytes = parts.encode()?;
        Ok(out)
    }
    /// F02 ENROLLED record with an ed25519 delegate, worker replay slot and F08 approval
    /// plus owner acceptance.
    fn enroll(&mut self, n: u8, at: u64) -> CodecResult<WorkerRosterEntry> {
        self.edit(|parts, market| {
            let owner = principal(n)?;
            let worker = derive_worker(market.market_id, owner, [n; 32])?;
            let slot = parts.workers.free_slot()?;
            let record = worker_record(worker, owner, public(&delegate_key(n)), slot, at)?;
            parts.workers.insert(&record)?;
            parts
                .replay
                .bind(ActorSlot::worker(usize::from(slot))?, owner, version()?)?;
            let participant = Participant::Worker(worker);
            let (effective, digest) = approve(
                &mut parts.admission,
                market,
                participant,
                owner,
                record.delegate,
                at,
            )?;
            parts.admission.admit(
                &admission_ctx(market, owner, at),
                &Admission {
                    participant,
                    delegate_generation: 1,
                    effective_epoch: effective,
                    config_version: 1,
                    approval_digest: digest,
                },
            )?;
            roster_entry(&record)
        })
    }
    /// F03 nomination accepted through the signed F08 evaluator consent of its key; the
    /// evaluator replay slot `n - 2` is bound to its owner.
    fn evaluator(&mut self, n: u8, at: u64) -> CodecResult<()> {
        self.edit(|parts, market| {
            let owner = principal(n)?;
            let key = evaluator_key(n);
            let signing_key = public(&key);
            let participant =
                Participant::Evaluator(derive_evaluator(market.market_id, owner, [n; 32])?);
            let (effective, digest) = approve(
                &mut parts.admission,
                market,
                participant,
                owner,
                signing_key,
                at,
            )?;
            let grant = grant(market, owner, n, signing_key, effective)?;
            let consent = EvaluatorConsent {
                chain: market.deployment_chain_domain,
                program: market.program_id,
                market: market.market_id,
                evaluator: grant.evaluator,
                owner,
                signing_key,
                enrollment_nonce: [n; 32],
                rubric: grant.rubric,
                approval_digest: digest,
                request: RequestId::new([n; 32])?,
                grant_version: 1,
                key_version: 1,
                effective_epoch: effective,
                config_version: 1,
                expiry_height: market.origin_height + effective * 128 + 64,
            };
            let mut signed = [0u8; 362];
            consent.encode(&mut signed)?;
            let mut payload = signed.to_vec();
            payload.extend_from_slice(&key.sign(consent.digest()?.as_bytes()).to_bytes());
            admit_evaluator(
                &mut parts.admission,
                &admission_ctx(market, owner, at),
                &grant,
                consent.request,
                &payload,
            )?;
            parts
                .replay
                .bind(ActorSlot::evaluator(usize::from(n - 2))?, owner, version()?)?;
            parts.insert_grant(grant, n)
        })
    }
    /// Owner FUND through the production router and transfer host.
    fn fund(&mut self, amount: u128, at: u64) -> Checked {
        let request = FundRequest {
            amount,
            refund_recipient: AccountId::new(REFUND)?,
            policy_version: FUNDING_POLICY_VERSION,
            consent: true,
        };
        let mut payload = vec![0; layerx_programs_ai_market::rewards::FUND_PAYLOAD_BYTES];
        request.encode(&mut payload)?;
        self.owner_raw(dispatch::FUND, payload, at)
    }
    /// F05 terminal result for every frozen worker, then `TerminalizeRewards`.
    fn terminalize(
        &mut self,
        frozen: &Frozen,
        roster: &[WorkerRosterEntry],
        at: u64,
    ) -> CodecResult<()> {
        self.edit(|parts, market| {
            let binding = FrozenBinding {
                chain: market.deployment_chain_domain,
                program: market.program_id,
                market: market.market_id,
                epoch: frozen.epoch,
                config: frozen.config,
                roster: frozen.roster,
            };
            let outputs = roster
                .iter()
                .map(|entry| {
                    WorkerAggregate::new(
                        entry.worker,
                        entry.generation,
                        3,
                        QualityStatus::ScoredPositive,
                        5,
                        5,
                    )
                })
                .collect::<CodecResult<Vec<_>>>()?;
            let allocation = allocate(frozen.budget, &outputs)?;
            let aggregation =
                EpochAggregation::structural(binding, Digest32::new([5; 32])?, roster, &outputs)?;
            let mut terminal = vec![0; REWARD_STATE_BYTES];
            let (head, tail) = parts
                .rewards
                .split_at_checked(REWARD_STATE_BYTES)
                .ok_or(NOT_FOUND)?;
            decode_reward_state(head)?.terminalize(
                &binding,
                aggregation.root(),
                &allocation,
                roster,
                at,
                &mut terminal,
            )?;
            terminal.extend_from_slice(tail);
            parts.rewards = terminal;
            Ok(())
        })
    }
}

/// The opened epoch's task, evidence, score and challenge calls.
impl Journey {
    fn frozen_binding(&self, frozen: &Frozen) -> CodecResult<FrozenBinding> {
        let header = self.header()?;
        Ok(FrozenBinding {
            chain: header.deployment_chain_domain,
            program: header.program_id,
            market: header.market_id,
            epoch: frozen.epoch,
            config: frozen.config,
            roster: frozen.roster,
        })
    }
    fn evaluator_id(&self, n: u8) -> CodecResult<EvaluatorId> {
        derive_evaluator(self.header()?.market_id, principal(n)?, [n; 32])
    }
    /// Keeper `SEAL_TASK_SET` of the opened epoch's task region, then `SealEvidence` of each
    /// of `evaluators` under evaluator sequence 1.
    fn seal(&mut self, frozen: &Frozen, evaluators: &[u8], at: u64) -> Checked {
        let binding = self.frozen_binding(frozen)?;
        let set = SetBinding {
            market: binding.market,
            epoch: binding.epoch,
            config: binding.config,
            policy: frozen.policy,
            roster: binding.roster,
        };
        let section = PolicySection::decode(&self.parts()?.policy)?
            .task_region
            .to_vec();
        let digest = tasks::task_set_digest(&set, &TaskSet::decode(&section)?)?;
        let mut payload = u64s(&[binding.epoch, binding.config.get()]);
        payload.extend_from_slice(digest.as_bytes());
        let call = bound(
            dispatch::SEAL_TASK_SET,
            principal(KEEPER)?,
            &binding,
            tag(0x61, 0, binding.epoch),
            payload,
        );
        self.send(&call, at)?;
        let sealed = tasks::sealed_task_set(&decode_shared_state(&self.bytes)?, binding.epoch)?;
        assert_eq!(
            sealed, digest,
            "evidence must consume the routed task-set seal"
        );
        for &n in evaluators {
            let mut claim = layerx_programs_ai_market::evidence::VERSION
                .to_be_bytes()
                .to_vec();
            claim.extend_from_slice(&root(n, binding.epoch));
            claim.extend_from_slice(frozen.policy.as_bytes());
            claim.extend_from_slice(sealed.as_bytes());
            claim.extend_from_slice(rubric()?.as_bytes());
            claim.push(1);
            let call = Req {
                sequence: 1,
                expiry: ROLE_EXPIRY,
                ..bound(
                    dispatch::SealEvidence,
                    principal(n)?,
                    &binding,
                    tag(0xc5, n, 1),
                    claim,
                )
            };
            self.send(&call, at)?;
        }
        Ok(())
    }
    /// `ADMIT_TASK` of `worker` by the requester under `nonce`; returns the task id.
    fn admit_task(
        &mut self,
        frozen: &Frozen,
        worker: WorkerId,
        nonce: u8,
        at: u64,
    ) -> Checked<[u8; 32]> {
        let binding = self.frozen_binding(frozen)?;
        let requester = principal(REQUESTER)?;
        let mut payload = u64s(&[binding.epoch, binding.config.get()]);
        payload.extend_from_slice(frozen.policy.as_bytes());
        payload.extend_from_slice(binding.roster.as_bytes());
        payload.extend_from_slice(requester.as_bytes());
        payload.extend_from_slice(worker.as_bytes());
        payload.extend_from_slice(&METADATA);
        payload.extend_from_slice(&[nonce; 32]);
        payload.extend_from_slice(&[0xa0 ^ nonce; 32]);
        payload.extend_from_slice(&(at + 24).to_be_bytes());
        let call = bound(
            dispatch::ADMIT_TASK,
            requester,
            &binding,
            [0xb0 ^ nonce; 32],
            payload,
        );
        self.send(&call, at)?;
        Ok(derive_task(binding.market, binding.epoch, requester, [nonce; 32])?.bytes())
    }
    /// `ACCEPT_TASK` or `COMMIT_TASK_RESULT` of `task` by the worker owner at `sequence`.
    fn worker_step(
        &mut self,
        frozen: &Frozen,
        operation: Operation,
        owner: PrincipalId,
        task: [u8; 32],
        sequence: u64,
        at: u64,
    ) -> Checked {
        let binding = self.frozen_binding(frozen)?;
        let mut payload = self.revision()?.to_be_bytes().to_vec();
        payload.extend_from_slice(&task);
        payload.extend_from_slice(&[0x60 + u8::try_from(sequence).map_err(|_| ARITHMETIC)?; 32]);
        let call = Req {
            sequence,
            ..bound(
                operation,
                owner,
                &binding,
                tag(0x70, owner.as_bytes()[0], sequence),
                payload,
            )
        };
        self.send(&call, at)
    }
    /// The frozen binding B of evaluator `n` at grant and key version 1.
    fn evaluator_binding(&self, frozen: &Frozen, n: u8) -> CodecResult<EvaluatorBinding> {
        Ok(EvaluatorBinding {
            frozen: self.frozen_binding(frozen)?,
            evaluator: self.evaluator_id(n)?,
            grant: version()?,
            key_version: version()?,
        })
    }
    /// `CommitScore` (sequence 2) of evaluator `n` over its signed report of `scores`.
    fn commit_score(&mut self, frozen: &Frozen, n: u8, scores: &[u8], at: u64) -> Checked {
        let binding = self.evaluator_binding(frozen, n)?;
        let body = ReportBody {
            binding,
            evidence: EvidenceRoot::new(root(n, frozen.epoch))?,
            scores: ScoreVector::Encoded(scores),
        };
        let digest: CommitmentDigest =
            codec::commitment_digest(&binding, codec::report_digest(&body)?, Salt32::new(SALT)?)?;
        let mut payload = vec![0; commitment::COMMIT_SCORE_BYTES];
        codec::encode_commit_score(
            &CommitScorePayload {
                binding,
                commitment: digest,
            },
            &mut payload,
        )?;
        let call = Req {
            sequence: 2,
            expiry: at + 16,
            ..bound(
                dispatch::CommitScore,
                principal(n)?,
                &binding.frozen,
                tag(0xa1, n, 2),
                payload,
            )
        };
        self.send(&call, at)
    }
    /// `RevealScore` (sequence 3) of evaluator `n`; returns the report digest.
    fn reveal_score(
        &mut self,
        frozen: &Frozen,
        n: u8,
        scores: &[u8],
        at: u64,
    ) -> Checked<ReportDigest> {
        let binding = self.evaluator_binding(frozen, n)?;
        let body = ReportBody {
            binding,
            evidence: EvidenceRoot::new(root(n, frozen.epoch))?,
            scores: ScoreVector::Encoded(scores),
        };
        let report = sign(body, &evaluator_key(n))?;
        let mut payload = vec![0; commitment::REVEAL_MAX_BYTES];
        let len = codec::encode_reveal_score(
            &RevealScorePayload {
                report: report.body,
                signature: report.signature,
                salt: Salt32::new(SALT)?,
            },
            &mut payload,
        )?;
        payload.truncate(len);
        let call = Req {
            sequence: 3,
            expiry: ROLE_EXPIRY,
            ..bound(
                dispatch::RevealScore,
                principal(n)?,
                &binding.frozen,
                tag(0xa2, n, 3),
                payload,
            )
        };
        self.send(&call, at)?;
        Ok(codec::report_digest(&report.body)?)
    }
    /// Permissionless `ChallengeAssessment` of `report` by `requester` under the derived id
    /// `H('PAXAI/evaluator-challenge/v1', chain || program || market || epoch || payload ||
    /// requester)`.
    fn challenge(
        &mut self,
        frozen: &Frozen,
        evaluator: EvaluatorId,
        report: ReportDigest,
        at: u64,
    ) -> Checked {
        let binding = self.frozen_binding(frozen)?;
        let requester = principal(CHALLENGER)?;
        let mut payload = evaluator.as_bytes().to_vec();
        payload.extend_from_slice(report.as_bytes());
        payload.push(3);
        payload.extend_from_slice(&[0x77; 32]);
        let mut preimage = CHAIN.to_vec();
        preimage.extend_from_slice(&PROGRAM);
        preimage.extend_from_slice(binding.market.as_bytes());
        preimage.extend_from_slice(&binding.epoch.to_be_bytes());
        preimage.extend_from_slice(&payload);
        preimage.extend_from_slice(requester.as_bytes());
        let id = codec::domain_hash("PAXAI/evaluator-challenge/v1", &preimage)?;
        let call = bound(
            dispatch::ChallengeAssessment,
            requester,
            &binding,
            id.bytes(),
            payload,
        );
        self.send(&call, at)
    }
}

/// The F02 lifecycle of one nominee and the expiry of another, after the epoch's reveals.
impl Journey {
    /// A native or delegate-signed F02 call of nominee `n` at its worker `sequence`.
    fn nominee(
        &mut self,
        operation: Operation,
        n: u8,
        sequence: u64,
        delegate: Option<SigningKey>,
        payload: Vec<u8>,
        at: u64,
    ) -> Checked {
        let call = Req {
            sequence,
            delegate,
            ..req(operation, principal(n)?, tag(0x80, n, sequence), payload)
        };
        self.send(&call, at)
    }
    fn worker_of(&self, n: u8) -> CodecResult<WorkerCurrent> {
        let state = decode_shared_state(&self.bytes)?;
        let (workers, _) = split_identity_section(state.section(Section::IdentityRoster)?)?;
        let worker = derive_worker(self.header()?.market_id, principal(n)?, [n; 32])?;
        WorkerTable::decode(workers)?.get(worker).ok_or(NOT_FOUND)
    }
    /// Owner `EnrollWorker` of nominee `n` whose consent expires at `expiry`.
    fn enroll_worker(&mut self, n: u8, expiry: u64, at: u64) -> Checked {
        let mut payload = principal(n)?.as_bytes().to_vec();
        payload.extend_from_slice(&[n; 32]);
        payload.extend_from_slice(&public(&delegate_key(n)).0);
        payload.extend_from_slice(&[0x45; 32]);
        payload.extend_from_slice(&expiry.to_be_bytes());
        self.owner_worker(dispatch::EnrollWorker, payload, at)
    }
    /// Enrollment, acceptance, metadata, drain, undo, revocation, rotation and retirement of
    /// one nominee; returns the height of its last call.
    fn worker_lifecycle(&mut self, at: u64) -> Checked<u64> {
        let header = self.header()?;
        let key = delegate_key(NOMINEE);
        self.enroll_worker(NOMINEE, at + 100, at)?;
        let record = self.worker_of(NOMINEE)?;
        let consent = consent_digest(
            &header,
            record.worker,
            record.owner,
            record.delegate,
            1,
            1,
            record.metadata,
            record.expiry,
        )?;
        let mut accept = record.worker.as_bytes().to_vec();
        accept.extend_from_slice(&u64s(&[1, 1]));
        accept.extend_from_slice(record.metadata.as_bytes());
        accept.extend_from_slice(&key.sign(consent.as_bytes()).to_bytes());
        self.nominee(dispatch::AcceptEnrollment, NOMINEE, 1, None, accept, at)?;

        let published = at + 8;
        let record = self.worker_of(NOMINEE)?;
        let bytes = manifest(&header, &record, 2, published, published + 200)?;
        let summary = decode_manifest(&bytes)?;
        let mut publish = record.worker.as_bytes().to_vec();
        publish.extend_from_slice(&u64s(&[1, 2]));
        publish.extend_from_slice(summary.digest.as_bytes());
        publish.extend_from_slice(&u64s(&[published, published + 200]));
        publish.extend_from_slice(
            &u32::try_from(bytes.len())
                .map_err(|_| ARITHMETIC)?
                .to_be_bytes(),
        );
        publish.extend_from_slice(&bytes);
        self.nominee(
            dispatch::PublishMetadata,
            NOMINEE,
            2,
            Some(key),
            publish,
            published,
        )?;

        let worker = record.worker.as_bytes().to_vec();
        self.nominee(
            dispatch::SetDraining,
            NOMINEE,
            3,
            None,
            worker.clone(),
            published,
        )?;
        self.nominee(
            dispatch::UndoDrain,
            NOMINEE,
            4,
            None,
            worker.clone(),
            published,
        )?;
        let mut revoke = worker.clone();
        revoke.extend_from_slice(&1u64.to_be_bytes());
        revoke.push(1);
        revoke.extend_from_slice(&1u64.to_be_bytes());
        self.nominee(
            dispatch::RevokeDelegate,
            NOMINEE,
            5,
            None,
            revoke,
            published,
        )?;

        let rotated = rotated_key(NOMINEE);
        let metadata = MetadataDigest::new([0x46; 32])?;
        let consent_expiry = published + 50;
        let consent = consent_digest(
            &header,
            record.worker,
            record.owner,
            public(&rotated),
            2,
            2,
            metadata,
            consent_expiry,
        )?;
        let mut rotate = worker.clone();
        rotate.extend_from_slice(&u64s(&[1, 1]));
        rotate.extend_from_slice(&public(&rotated).0);
        rotate.extend_from_slice(metadata.as_bytes());
        rotate.extend_from_slice(&consent_expiry.to_be_bytes());
        rotate.extend_from_slice(&rotated.sign(consent.as_bytes()).to_bytes());
        self.nominee(
            dispatch::RotateDelegate,
            NOMINEE,
            6,
            None,
            rotate,
            published,
        )?;
        self.nominee(dispatch::RetireWorker, NOMINEE, 7, None, worker, published)?;
        Ok(published)
    }
    /// Owner enrollment of a second nominee at `lapse` that is never accepted, then its
    /// expiry once the consent window closes.
    fn expire_enrollment(&mut self, lapse: u64) -> Checked {
        let header = self.header()?;
        self.enroll_worker(LAPSED, lapse + 1, lapse)?;
        let candidate = self.worker_of(LAPSED)?;
        let mut expire = candidate.worker.as_bytes().to_vec();
        expire.extend_from_slice(candidate.proposal_digest(&header)?.as_bytes());
        expire.extend_from_slice(&candidate.expiry.to_be_bytes());
        self.owner_worker(dispatch::ExpireEnrollment, expire, lapse + 1)
    }
}

/// The F01 registry calls of the fresh market and the F03 authority calls of the owner.
impl Journey {
    /// `CREATE` at the origin, then operator appointment, metadata update and a staged policy
    /// the owner cancels.
    fn registry(&mut self) -> Checked {
        let owner = PrincipalId::new(OWNER)?;
        let mut create = OWNER.to_vec();
        create.extend_from_slice(&ASSET);
        create.extend_from_slice(
            derive_rewards_account(ProgramId::new(PROGRAM)?, AssetId::new(ASSET)?)?.as_bytes(),
        );
        create.extend_from_slice(&REFUND);
        create.push(0);
        create.extend_from_slice(&policy_bytes(&policy(1, 3)?)?);
        create.extend_from_slice(&[16; 32]);
        self.send(
            &Req {
                sequence: 1,
                ..req(dispatch::CREATE, owner, [1; 32], create)
            },
            ORIGIN,
        )?;
        let mut appoint = principal(OPERATOR)?.as_bytes().to_vec();
        appoint.push(PERMIT_METADATA | PERMIT_SUSPEND);
        appoint.extend_from_slice(&0u64.to_be_bytes());
        self.owner(dispatch::APPOINT_OPERATOR, &appoint, ORIGIN + 1)?;
        self.owner(dispatch::UPDATE_METADATA, &[0x17; 32], ORIGIN + 1)?;
        let mut staged = policy_bytes(&policy(2, 3)?)?;
        staged.extend_from_slice(&10u64.to_be_bytes());
        self.owner(dispatch::STAGE_POLICY, &staged, ORIGIN + 1)?;
        self.owner(dispatch::CANCEL_POLICY, &2u64.to_be_bytes(), ORIGIN + 1)?;
        Ok(())
    }
    /// Owner scheduling of a new evaluator, key rotation of evaluator 6 and revocation of
    /// evaluator 7, bound to the opened epoch.
    fn authority(&mut self, binding: &FrozenBinding, at: u64) -> Checked {
        let mut schedule = principal(SCHEDULED)?.as_bytes().to_vec();
        schedule.extend_from_slice(&[SCHEDULED; 32]);
        schedule.extend_from_slice(rubric()?.as_bytes());
        schedule.extend_from_slice(&public(&evaluator_key(SCHEDULED)).0);
        schedule.extend_from_slice(&u64s(&[1, 1, binding.epoch + 1, binding.epoch + 33]));
        self.owner_bound(dispatch::ScheduleEvaluator, binding, schedule, at)?;
        let mut rotate = self.evaluator_id(6)?.as_bytes().to_vec();
        rotate.extend_from_slice(&public(&rotated_key(6)).0);
        rotate.extend_from_slice(&u64s(&[2, 1, binding.epoch + 1]));
        self.owner_bound(dispatch::RotateEvaluatorKey, binding, rotate, at)?;
        let mut revoke = self.evaluator_id(7)?.as_bytes().to_vec();
        revoke.extend_from_slice(&1u64.to_be_bytes());
        revoke.extend_from_slice(&1u16.to_be_bytes());
        revoke.extend_from_slice(&[0x5e; 32]);
        self.owner_bound(dispatch::RevokeEvaluator, binding, revoke, at)?;
        Ok(())
    }
}

/// The journey from a fresh market: the registry, epoch 6 opening and activation, epoch 7
/// opening, its tasks, seals, scores and challenge, the F03 authority calls, the F02 worker
/// lifecycle, and finally the operator revocation and suspension.
fn journey(module: Arc<ValidatedModule>) -> Checked<Journey> {
    let mut j = Journey {
        module,
        bytes: Vec::new(),
        owner_sequence: 2,
        rows: Vec::new(),
    };
    j.registry()?;

    let low = j.enroll(LOW, 130)?;
    for n in 2..=4 {
        j.evaluator(n, 129 + u64::from(n))?;
    }
    j.owner(dispatch::SCHEDULE_ACTIVATION, &6u64.to_be_bytes(), 134)?;
    j.fund(500, 135)?;
    let first = j.open(896)?;
    assert_eq!((first.epoch, first.workers, first.evaluators), (6, 1, 3));
    j.advance(897)?;
    let high = j.enroll(HIGH, 900)?;
    for n in 5..=7 {
        j.evaluator(n, 896 + u64::from(n))?;
    }
    j.seal(&first, &[], 960)?;
    j.terminalize(&first, &[low], 1008)
        .map_err(|e| Failure::NotApplied("terminalize prerequisite", format!("{e:?}")))?;

    let opened = j.open(1024)?;
    assert_eq!((opened.epoch, opened.workers, opened.evaluators), (7, 2, 6));
    let mut workers = [low, high];
    workers.sort_by_key(|w| w.worker);
    let [first_worker, second_worker] = workers;
    let task = j.admit_task(&opened, first_worker.worker, 1, WORK_AT)?;
    j.worker_step(
        &opened,
        dispatch::ACCEPT_TASK,
        first_worker.owner,
        task,
        1,
        WORK_AT + 1,
    )?;
    j.worker_step(
        &opened,
        dispatch::COMMIT_TASK_RESULT,
        first_worker.owner,
        task,
        2,
        WORK_AT + 2,
    )?;
    let cancelled = j.admit_task(&opened, first_worker.worker, 2, WORK_AT + 3)?;
    let binding = j.frozen_binding(&opened)?;
    j.send(
        &bound(
            dispatch::CANCEL_TASK,
            principal(REQUESTER)?,
            &binding,
            [0xbe; 32],
            cancelled.to_vec(),
        ),
        WORK_AT + 4,
    )?;
    j.seal(&opened, &[2, 3, 4, 5, 6, 7], COMMIT_AT)?;

    let scores = entries(&[
        (first_worker.worker, 500_000),
        (second_worker.worker, 600_000),
    ]);
    for n in REVEALERS {
        j.commit_score(&opened, n, &scores, COMMIT_AT + 2)?;
    }
    let mut reports = Vec::new();
    for n in REVEALERS {
        reports.push(j.reveal_score(&opened, n, &scores, REVEAL_AT)?);
    }
    let challenged = j.evaluator_id(REVEALERS[0])?;
    j.challenge(&opened, challenged, reports[0], REVEAL_AT + 2)?;

    j.authority(&binding, REVEAL_AT + 3)?;
    let mut extra = j.fork();
    extra.finish_epoch(&opened, 1120)?;
    extra.history_calls(first_worker.worker, 1121)?;
    extra.settlement_calls(&opened, 1122)?;
    j.rows.extend(extra.rows);
    let retired = j.worker_lifecycle(REVEAL_AT + 4)?;
    j.expire_enrollment(retired + 1)?;
    j.owner(
        dispatch::REVOKE_OPERATOR,
        &1u64.to_be_bytes(),
        REVEAL_AT + 15,
    )?;
    j.owner(dispatch::SUSPEND, &[0x50; 32], REVEAL_AT + 15)?;
    Ok(j)
}

fn committed_fund_row() -> Checked<Row> {
    let fixture = repair_478_fixture::fixture(OWNER, ASSET)?;
    let envelope = decode_envelope(&fixture.fund)?;
    let ctx = CallContext {
        chain: envelope.envelope.chain,
        program: envelope.envelope.program,
        principal: envelope.envelope.actor,
        height: repair_478_fixture::HEIGHT,
    };
    let mut topic = [0; 64];
    let topic_len = codec::event_topic(dispatch::FUND, &mut topic)?;
    let expected = Expected {
        state: &fixture.next,
        topic: &topic[..topic_len],
        event: &fixture.event,
        result: &fixture.result,
    };
    let (fuel, verdict) = run_guest(
        &guest_module()?,
        &ctx,
        Some(&fixture.current),
        &fixture.fund,
        &expected,
    )?;
    Ok(Row {
        operation: dispatch::FUND,
        height: ctx.height,
        state: fixture.current.len(),
        fuel,
        verdict,
    })
}

#[test]
fn fund_fits_the_protocol_cpu_fuel_cap() -> Checked {
    let row = committed_fund_row()?;
    println!(
        "{:<22} {:>6} {:>7} {:>10}  verdict (cap {DEFAULT_CPU_FUEL})",
        "operation", "height", "state", "fuel"
    );
    println!(
        "{:<22} {:>6} {:>7} {:>10}  {}",
        row.operation.metadata().name,
        row.height,
        row.state,
        row.fuel.required_with("Fund must report measured CPU fuel"),
        row.verdict
            .as_ref()
            .map_or_else(Clone::clone, |()| "ok".to_string())
    );
    let fuel = row.fuel.required_with("Fund must report measured CPU fuel");
    assert!(fuel > 0);
    assert!(fuel < 999_968);
    assert!(fuel <= DEFAULT_CPU_FUEL);
    assert_eq!(row.verdict, Ok(()));
    Ok(())
}

#[test]
/// # Errors
/// Returns an error when producing or applying the market state fails.
/// # Panics
/// Panics when market state or execution does not satisfy the asserted conditions.
#[allow(
    private_interfaces,
    reason = "The test entry point preserves its existing checked journey result."
)]
pub fn every_routed_operation_fits_the_protocol_cpu_fuel_cap() -> Checked {
    let module = guest_module()?;
    // Drive each independent real state trajectory even if another one refuses, so one
    // fixture error cannot hide failures at the protocol's maximum bounded state.
    let small = journey(module.clone());
    let admission = admission_journey(module.clone());
    let mut f11 = f11_journey(module.clone());
    let maximum = maximum_journey(module);
    println!("small journey error: {:?}", small.as_ref().err());
    println!("admission journey error: {:?}", admission.as_ref().err());
    println!("maximum journey error: {:?}", maximum.as_ref().err());
    let mut journey = small?;
    let mut admission = admission?;
    let mut maximum = maximum?;
    for (index, row) in maximum.rows.iter().enumerate() {
        println!(
            "maximum row={index} operation={} height={} state_bytes={} fuel={:?} verdict={:?}",
            row.operation.metadata().name,
            row.height,
            row.state,
            row.fuel,
            row.verdict
        );
    }
    journey.rows.append(&mut admission.rows);
    journey.rows.append(&mut maximum.rows);
    journey.rows.append(&mut f11);
    journey.rows.push(committed_fund_row()?);
    println!(
        "{:<22} {:>6} {:>7} {:>10}  verdict (cap {DEFAULT_CPU_FUEL})",
        "operation", "height", "state", "fuel"
    );
    for row in &journey.rows {
        println!(
            "{:<22} {:>6} {:>7} {:>10?}  {}",
            row.operation.metadata().name,
            row.height,
            row.state,
            row.fuel,
            row.verdict
                .as_ref()
                .map_or_else(Clone::clone, |()| "ok".to_string())
        );
    }
    let covered: std::collections::BTreeSet<u16> = ROUTED.iter().map(|op| op.selector()).collect();
    let metadata: std::collections::BTreeSet<u16> = dispatch::OPERATIONS
        .iter()
        .map(|m| m.operation.selector())
        .collect();
    assert_eq!(
        covered, metadata,
        "fuel inventory must track routing metadata"
    );
    let missing: Vec<&str> = ROUTED
        .iter()
        .filter(|op| !journey.rows.iter().any(|row| row.operation == **op))
        .map(|op| op.metadata().name)
        .collect();
    assert_eq!(missing, Vec::<&str>::new());
    let over: Vec<(&str, u64)> = journey
        .rows
        .iter()
        .filter_map(|row| {
            row.fuel
                .filter(|fuel| *fuel > DEFAULT_CPU_FUEL)
                .map(|fuel| (row.operation.metadata().name, fuel))
        })
        .collect();
    participant_call_totals(&journey.rows, "all_selectors", 64, 32, 8);
    assert_eq!(over, Vec::<(&str, u64)>::new());
    assert!(
        journey.rows.iter().all(|row| row.fuel.is_some()),
        "every routed guest execution must produce measured fuel"
    );
    let failed: Vec<(&str, u64, &str)> = journey
        .rows
        .iter()
        .filter_map(|row| {
            row.verdict
                .as_ref()
                .err()
                .map(|reason| (row.operation.metadata().name, row.height, reason.as_str()))
        })
        .collect();
    assert_eq!(failed, Vec::<(&str, u64, &str)>::new());
    Ok(())
}

#[test]
fn fund_joint_sections_match_the_carried_transition_and_replay() {
    use layerx_programs_ai_market::{
        epoch, rewards,
        state::{decode_shared_state, Section},
        MERGED_CHAIN_DOMAIN,
    };
    use repair_478_fixture::{fixture, HEIGHT, PROGRAM};
    let owner = [12; 32];
    let f = fixture(owner, [13; 32]).required();
    let ctx = CallContext {
        chain: ChainDomain::new(MERGED_CHAIN_DOMAIN).required(),
        program: ProgramId::new(PROGRAM).required(),
        principal: PrincipalId::new(owner).required(),
        height: HEIGHT,
    };
    let envelope = codec::decode_envelope(&f.fund).required();
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; rewards::SCRATCH_BYTES];
    let mut carry = vec![0; epoch::CARRY_SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let (outcome, written) =
        epoch::carry_reputation(&f.current, &mut next, &mut carry, |current, next| {
            let outcome = rewards::apply(&ctx, &envelope, current, next, &mut scratch, &mut event)?;
            let rewards::Outcome::Applied { state_len, .. } = outcome else {
                panic!("first Fund must apply");
            };
            Ok((outcome, Some(state_len)))
        })
        .required();
    assert_eq!(&next[..written.required()], f.next);
    let rewards::Outcome::Applied {
        event_len,
        response,
        ..
    } = outcome
    else {
        panic!("first Fund must apply");
    };
    assert_eq!(&event[..event_len], f.event);
    assert_eq!(
        response.as_bytes(),
        codec::decode_result(&f.result).required().payload
    );
    let before = decode_shared_state(&f.current).required();
    let after = decode_shared_state(&f.next).required();
    let joint = Section::ReputationAdmission.index();
    assert_eq!(&before.feature_sections[joint][..4], b"RP07");
    for section in [
        Section::IdentityRoster,
        Section::CurrentReports,
        Section::ReputationAdmission,
    ] {
        assert_eq!(
            before.feature_sections[section.index()],
            after.feature_sections[section.index()]
        );
    }
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    next.fill(0xa5);
    let retry = dispatch::route(
        &ctx,
        &f.fund,
        Some(&f.next),
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )
    .required();
    let Routed::Unchanged { result_len } = retry else {
        panic!("Fund replay must return the retained result");
    };
    assert_eq!(
        codec::decode_result(&result[..result_len])
            .required()
            .revision,
        after.revision
    );
    assert!(next.iter().all(|byte| *byte == 0xa5));

    verify_corrupt_joint_funding(&f, &ctx, &before);
}
fn verify_corrupt_joint_funding(
    f: &repair_478_fixture::Fixture,
    ctx: &CallContext,
    before: &layerx_programs_ai_market::state::SharedState<'_>,
) {
    let joint = Section::ReputationAdmission.index();
    let mut next = vec![0xa5; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    let mut corrupt_joint = before.feature_sections[joint].to_vec();
    corrupt_joint[4] ^= 0x80;
    let corrupt_state = before
        .replace_section(Section::ReputationAdmission, &corrupt_joint)
        .required();
    let mut corrupt = vec![0; MAX_STATE_BYTES];
    let mut control = vec![0; Section::Control.payload_cap()];
    let len = layerx_programs_ai_market::state::encode_shared_state(
        &corrupt_state,
        &mut corrupt,
        &mut control,
    )
    .required();
    let refused = dispatch::route(
        ctx,
        &f.fund,
        Some(&corrupt[..len]),
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )
    .required();
    let Routed::Refused { result_len } = refused else {
        panic!("Fund must validate the joint reputation region");
    };
    assert!(codec::decode_result(&result[..result_len])
        .required()
        .error
        .is_some());
    assert!(next.iter().all(|byte| *byte == 0xa5));
}

#[test]
fn fund_entry_gas_for_committed_receipt_fixture_is_bounded() {
    measured_fund_cpu_fuel();
}

fn measured_fund_cpu_fuel() -> u64 {
    use repair_478_fixture::{fixture, AMOUNT, HEIGHT, PROGRAM};
    let f = fixture(OWNER, ASSET).required();
    let mut host = guest::Guest::new(guest_module().required(), PROGRAM, ASSET).required();
    host.ledger.balances.insert(OWNER, AMOUNT);
    let ctx = CallContext {
        chain: ChainDomain::new(CHAIN).required(),
        program: ProgramId::new(PROGRAM).required(),
        principal: PrincipalId::new(OWNER).required(),
        height: HEIGHT,
    };
    let created = host.call(&ctx, &f.create, Some(&f.current)).required();
    assert_eq!(created.transfers, 0);
    assert_eq!(
        host.state().required().as_deref(),
        Some(f.current.as_slice())
    );
    let record = host.call(&ctx, &f.fund, Some(&f.next)).required();
    assert_eq!(record.program.bytes(), PROGRAM);
    assert_eq!(record.abi_version, 5);
    assert_eq!(record.response, f.result);
    assert_eq!(
        codec::decode_result(&record.response).required().error,
        None
    );
    assert_eq!(record.events.len(), 1);
    assert_eq!(record.events[0].data, f.event);
    assert_eq!(host.state().required().as_deref(), Some(f.next.as_slice()));
    assert_eq!(record.transfers, 1);
    assert_eq!(host.ledger.balance(OWNER), 0);
    assert_eq!(host.ledger.balance(f.rewards), AMOUNT);
    assert_eq!(host.ledger.total(), AMOUNT);
    let usage = record.usage;
    assert!(usage.cpu_fuel < 999_968);
    assert!(usage.cpu_fuel > 0);
    assert!(usage.cpu_fuel <= layerx_programs_runtime::meter::DEFAULT_CPU_FUEL);
    assert!(usage.cpu_fuel < 10_000_000);
    let shared_state_key_bytes =
        u64::try_from(layerx_programs_ai_market::SHARED_STATE_KEY.len()).required();
    let shared_state_value_bytes = u64::try_from(f.current.len()).required();
    assert_eq!(shared_state_key_bytes, 14);
    assert_eq!(
        usage.storage_read_bytes,
        shared_state_key_bytes + shared_state_value_bytes
    );
    assert_eq!(
        usage.storage_write_bytes,
        shared_state_key_bytes + u64::try_from(f.next.len()).required()
    );
    println!(
        "Fund committed fixture CPU fuel: {} (kernel allowance 10000000)",
        usage.cpu_fuel
    );
    usage.cpu_fuel
}

impl Journey {
    fn fork(&self) -> Self {
        Self {
            module: self.module.clone(),
            bytes: self.bytes.clone(),
            owner_sequence: self.owner_sequence,
            rows: vec![],
        }
    }
    fn owner_raw(&mut self, operation: Operation, payload: Vec<u8>, at: u64) -> Checked {
        let call = Req {
            sequence: self.owner_sequence,
            config: self.header()?.active_config_version,
            ..req(
                operation,
                PrincipalId::new(OWNER)?,
                tag(0xd0, 0, self.owner_sequence),
                payload,
            )
        };
        self.send(&call, at)?;
        self.owner_sequence += 1;
        Ok(())
    }
    fn object(
        &mut self,
        operation: Operation,
        frozen: &Frozen,
        payload: Vec<u8>,
        at: u64,
    ) -> Checked<Vec<u8>> {
        let binding = self.frozen_binding(frozen)?;
        let call = bound(
            operation,
            principal(KEEPER)?,
            &binding,
            tag(
                0xd1,
                operation.selector().to_le_bytes()[0],
                self.revision()?,
            ),
            payload,
        );
        self.send_step(&call, at)
    }
    fn finish_epoch(&mut self, frozen: &Frozen, at: u64) -> Checked {
        self.object(dispatch::BeginAggregation, frozen, vec![], at)?;
        loop {
            let progress = layerx_programs_ai_market::aggregation::progress(&self.bytes)?;
            let Presence::Present(input) = progress.input else {
                return Err(Failure::Guest("missing aggregation input".into()));
            };
            if progress.cursor == progress.worker_count {
                let mut previous = 0;
                loop {
                    let payload = self.object(
                        dispatch::FinalizeAggregation,
                        frozen,
                        input.bytes().to_vec(),
                        at,
                    )?;
                    let cursor =
                        u16::from_be_bytes(payload[payload.len() - 2..].try_into().required());
                    assert!(cursor > previous && cursor <= progress.worker_count);
                    assert!(cursor - previous <= 8);
                    if cursor == progress.worker_count {
                        break;
                    }
                    previous = cursor;
                }
                assert_eq!(
                    layerx_programs_ai_market::aggregation::progress(&self.bytes)?.phase,
                    layerx_programs_ai_market::aggregation_codec::AggregationPhase::Terminal
                );
                break;
            }
            let mut payload = input.bytes().to_vec();
            payload.extend_from_slice(&progress.cursor.to_be_bytes());
            self.object(dispatch::ProcessAggregation, frozen, payload, at)?;
        }
        Ok(())
    }
    fn history_calls(&mut self, worker: WorkerId, at: u64) -> Checked {
        for operation in [
            dispatch::SuspendHistory,
            dispatch::ResumeHistory,
            dispatch::ResetHistory,
        ] {
            let parts = self.parts()?;
            let (region, _) =
                layerx_programs_ai_market::reputation_transition::split_joint(&parts.reputation)?;
            let region = region.ok_or(NOT_FOUND)?;
            let record = *region
                .state
                .records()
                .find(|r| r.worker == worker)
                .ok_or(NOT_FOUND)?;
            let mut payload = worker.bytes().to_vec();
            payload.extend_from_slice(record.segment.as_bytes());
            payload.extend_from_slice(&record.reset_generation.get().to_be_bytes());
            payload.push(1);
            if operation == dispatch::ResetHistory {
                let w = parts.workers.get(worker).ok_or(NOT_FOUND)?;
                let slot = ActorSlot::worker(usize::from(w.slot))?;
                let sequence = parts
                    .replay
                    .actor(slot)
                    .ok_or(NOT_FOUND)?
                    .last
                    .map_or(1, |last| last.sequence + 1);
                self.send(
                    &Req {
                        sequence,
                        ..req(operation, w.owner, tag(0xd2, 1, sequence), payload)
                    },
                    at,
                )?;
            } else {
                self.owner_raw(operation, payload, at)?;
            }
        }
        Ok(())
    }
    fn settlement_calls(&mut self, frozen: &Frozen, at: u64) -> Checked {
        let parts = self.parts()?;
        let state = decode_shared_state(&self.bytes)?;
        let rewards = layerx_programs_ai_market::state::settlement_rewards(&state, None)?;
        let row = rewards.row(frozen.epoch)?;
        let dictionary = rewards.dictionary();
        let entry = row
            .entries()
            .iter()
            .find(|e| e.entitlement > 0)
            .ok_or(NOT_FOUND)?;
        let slot = dictionary.slot(entry.slot)?;
        let claim = layerx_programs_ai_market::rewards::ClaimRequest {
            worker: slot.worker,
            recipient: slot.recipient,
            amount: entry.entitlement,
        };
        let mut payload = vec![0; 80];
        claim.encode(&mut payload)?;
        self.object(dispatch::CLAIM, frozen, payload, at)?;
        let expiry = row.expiry_height;
        self.object(dispatch::EXPIRE_EPOCH_CLAIMS, frozen, vec![], expiry)?;
        self.owner(dispatch::REQUEST_CLOSE, &[0x52; 32], expiry + 1)?;
        let ledger = layerx_programs_ai_market::state::settlement_rewards(
            &decode_shared_state(&self.bytes)?,
            None,
        )?
        .ledger()?;
        let refund = layerx_programs_ai_market::rewards::RefundRequest {
            expected_refunded: ledger.tracked_refunds,
            amount: ledger.free,
            recipient: ledger.refund_recipient,
        };
        let mut payload = vec![0; 64];
        refund.encode(&mut payload)?;
        self.object(dispatch::REFUND_FREE, frozen, payload, expiry + 2)?;
        // The previous completed epoch is prunable while the newest remains captured.
        let first = layerx_programs_ai_market::state::settlement_rewards(
            &decode_shared_state(&self.bytes)?,
            None,
        )?
        .row(6)?;
        let earlier = Frozen {
            epoch: 6,
            config: frozen.config,
            roster: first.roster,
            ..*frozen
        };
        self.object(dispatch::EXPIRE_EPOCH_CLAIMS, &earlier, vec![], expiry + 3)?;
        self.object(dispatch::PRUNE_EPOCH, &earlier, vec![], expiry + 3)?;
        for _ in 0..10 {
            let header = self.header()?;
            if header.lifecycle == layerx_programs_ai_market::registry_ops::CLOSED {
                break;
            }
            let mut payload = self.revision()?.to_be_bytes().to_vec();
            payload.push(header.close_phase);
            payload.extend_from_slice(&header.close_cursor.to_be_bytes());
            self.send(
                &req(
                    dispatch::ADVANCE_CLOSE,
                    principal(KEEPER)?,
                    tag(0xd3, header.close_phase, self.revision()?),
                    payload,
                ),
                expiry + 4,
            )?;
        }
        assert_eq!(
            self.header()?.lifecycle,
            layerx_programs_ai_market::registry_ops::CLOSED
        );
        self.read_closed_state(expiry)?;
        let _ = parts;
        Ok(())
    }
    fn read_closed_state(&mut self, expiry: u64) -> Checked {
        self.send(
            &req(
                dispatch::READ_HEADER,
                principal(KEEPER)?,
                [0xd4; 32],
                vec![],
            ),
            expiry + 5,
        )?;
        let mut payload = vec![0; 64];
        let n = codec::encode_chunk_request(
            &codec::ChunkRequest {
                revision: self.revision()?,
                digest: Presence::Present(codec::state_digest(&self.bytes)?),
                offset: 0,
                requested: u16::try_from(layerx_programs_ai_market::MAX_CHUNK_BYTES).required(),
            },
            &mut payload,
        )?;
        payload.truncate(n);
        self.send(
            &req(
                dispatch::READ_STATE_CHUNK,
                principal(KEEPER)?,
                [0xd5; 32],
                payload,
            ),
            expiry + 5,
        )?;
        Ok(())
    }
}

fn participant_payload(participant: Participant) -> Vec<u8> {
    let mut payload = vec![match participant {
        Participant::Worker(_) => 1,
        Participant::Evaluator(_) => 2,
    }];
    payload.extend_from_slice(&participant.bytes());
    payload
}
impl Journey {
    fn approval(&mut self, participant: Participant, at: u64) -> Checked<Digest32> {
        let parts = self.parts()?;
        let market = parts.market()?;
        let effective = parts.admission.required_effective_epoch(&admission_ctx(
            &market,
            market.owner_principal,
            at,
        ))?;
        let (owner, delegate, identity) = match participant {
            Participant::Worker(worker) => {
                let w = parts.workers.get(worker).ok_or(NOT_FOUND)?;
                (w.owner, w.delegate, w.proposal_digest(&market)?)
            }
            Participant::Evaluator(evaluator) => {
                let region = EvaluatorRegion::decode(&parts.region)?;
                let g = region.stored_grant(evaluator)?;
                let mut bytes = vec![0; layerx_programs_ai_market::evaluators::model::GRANT_BYTES];
                let n = layerx_programs_ai_market::evaluators::codec::encode_grant(&g, &mut bytes)?;
                (
                    g.principal,
                    g.signing_key,
                    codec::domain_hash("PAXAI/evaluator-grant/v1", &bytes[..n])?,
                )
            }
        };
        let terms = ApprovalTerms {
            participant,
            owner,
            enrollment_nonce_commitment: Digest32::new([7; 32])?,
            delegate,
            delegate_generation: 1,
            identity_commitment: identity,
            effective_epoch: effective,
            config_version: 1,
            request: RequestId::new(tag(0xd0, 0, self.owner_sequence))?,
            expiry_height: ORIGIN + effective * 128 + 64,
        };
        let mut payload = participant_payload(participant);
        payload.extend_from_slice(owner.as_bytes());
        payload.extend_from_slice(terms.enrollment_nonce_commitment.as_bytes());
        payload.extend_from_slice(&delegate.0);
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(identity.as_bytes());
        payload.extend_from_slice(&u64s(&[effective, 1, terms.expiry_height]));
        self.owner_raw(dispatch::ApproveAdmission, payload, at)?;
        Ok(terms.digest(&market)?)
    }
}
fn admission_journey(module: Arc<ValidatedModule>) -> Checked<Journey> {
    let mut j = Journey {
        module,
        bytes: vec![],
        owner_sequence: 2,
        rows: vec![],
    };
    j.registry()?;
    let (participant, evaluator) = initial_journey_admissions(&mut j)?;
    let other = j.edit(|parts, market| {
        let owner = principal(HIGH)?;
        let worker = derive_worker(market.market_id, owner, [HIGH; 32])?;
        parts.workers.insert(&worker_record(
            worker,
            owner,
            public(&delegate_key(HIGH)),
            1,
            132,
        )?)?;
        parts
            .replay
            .bind(ActorSlot::worker(1)?, owner, version()?)?;
        Ok(worker)
    })?;
    j.approval(Participant::Worker(other), 132)?;
    j.owner_raw(
        dispatch::RevokeAdmissionApproval,
        participant_payload(Participant::Worker(other)),
        133,
    )?;
    // Two further real admitted evaluators supply the minimum activation quorum.
    j.evaluator(3, 133)?;
    j.evaluator(4, 134)?;
    j.fund(1000, 135)?;
    j.owner(dispatch::SCHEDULE_ACTIVATION, &6u64.to_be_bytes(), 136)?;
    let frozen = j.open(896)?;
    j.advance(897)?;
    let mut payload = participant_payload(participant);
    payload.extend_from_slice(&u64s(&[1, 1, 6]));
    j.send(
        &Req {
            sequence: 2,
            delegate: Some(delegate_key(LOW)),
            ..req(dispatch::Heartbeat, principal(LOW)?, [0xe3; 32], payload)
        },
        898,
    )?;
    let mut payload = participant_payload(participant);
    payload.extend_from_slice(&u64s(&[1, 7]));
    payload.push(1);
    j.send(
        &Req {
            sequence: 3,
            ..req(dispatch::RequestExit, principal(LOW)?, [0xe4; 32], payload)
        },
        899,
    )?;
    let mut payload = participant_payload(participant);
    payload.extend_from_slice(&1u64.to_be_bytes());
    j.send(
        &Req {
            sequence: 4,
            ..req(dispatch::CancelExit, principal(LOW)?, [0xe5; 32], payload)
        },
        900,
    )?;
    let mut payload = participant_payload(Participant::Evaluator(evaluator));
    payload.extend_from_slice(&1u64.to_be_bytes());
    payload.push(1);
    j.owner_raw(dispatch::AdministrativeRemove, payload, 901)?;
    // The removed evaluator is replaced by a real admission effective in epoch 7.
    j.evaluator(5, 902)?;
    j.seal(&frozen, &[], 960)?;
    j.finish_epoch(&frozen, 1008)?;
    let _ = j.open(1024)?;
    let mut payload = participant_payload(participant);
    payload.extend_from_slice(&j.revision()?.to_be_bytes());
    j.send(
        &Req {
            sequence: 0,
            epoch: 7,
            ..req(
                dispatch::PruneInactive,
                principal(LOW)?,
                [0xe6; 32],
                payload,
            )
        },
        1030,
    )?;
    Ok(j)
}
fn initial_journey_admissions(j: &mut Journey) -> Checked<(Participant, EvaluatorId)> {
    let worker = j.edit(|parts, market| {
        let owner = principal(LOW)?;
        let id = derive_worker(market.market_id, owner, [LOW; 32])?;
        let record = worker_record(id, owner, public(&delegate_key(LOW)), 0, 130)?;
        parts.workers.insert(&record)?;
        parts
            .replay
            .bind(ActorSlot::worker(0)?, owner, version()?)?;
        let g = grant(market, principal(2)?, 2, public(&evaluator_key(2)), 0)?;
        parts.insert_grant(g, 2)?;
        Ok(id)
    })?;
    let participant = Participant::Worker(worker);
    let digest = j.approval(participant, 130)?;
    let mut payload = worker.bytes().to_vec();
    payload.extend_from_slice(&u64s(&[1, 0, 1]));
    payload.extend_from_slice(digest.as_bytes());
    j.send(
        &Req {
            sequence: 1,
            ..req(dispatch::AdmitWorker, principal(LOW)?, [0xe1; 32], payload)
        },
        131,
    )?;
    let evaluator = j.evaluator_id(2)?;
    let digest = j.approval(Participant::Evaluator(evaluator), 131)?;
    let market = j.header()?;
    let consent = EvaluatorConsent {
        chain: market.deployment_chain_domain,
        program: market.program_id,
        market: market.market_id,
        evaluator,
        owner: principal(2)?,
        signing_key: public(&evaluator_key(2)),
        enrollment_nonce: [2; 32],
        rubric: rubric()?,
        approval_digest: digest,
        request: RequestId::new([0xe2; 32])?,
        grant_version: 1,
        key_version: 1,
        effective_epoch: 0,
        config_version: 1,
        expiry_height: 192,
    };
    let mut encoded = [0; 362];
    consent.encode(&mut encoded)?;
    let mut payload = encoded.to_vec();
    payload.extend_from_slice(
        &evaluator_key(2)
            .sign(consent.digest()?.as_bytes())
            .to_bytes(),
    );
    j.send(
        &Req {
            sequence: 1,
            ..req(dispatch::AdmitEvaluator, principal(2)?, [0xe2; 32], payload)
        },
        132,
    )?;
    Ok((participant, evaluator))
}

fn maximum_journey(module: Arc<ValidatedModule>) -> Checked<Journey> {
    maximum_journey_with_tasks(module, None)
}

fn maximum_journey_with_tasks(
    module: Arc<ValidatedModule>,
    task_count: Option<usize>,
) -> Checked<Journey> {
    maximum_journey_with_roster(module, task_count, 32, 8)
}

fn maximum_journey_with_roster(
    module: Arc<ValidatedModule>,
    task_count: Option<usize>,
    worker_count: u8,
    evaluator_count: u8,
) -> Checked<Journey> {
    let evaluators = (2..2 + evaluator_count).collect::<Vec<_>>();
    let mut j = Journey {
        module,
        bytes: vec![],
        owner_sequence: 2,
        rows: vec![],
    };
    j.registry()?;
    j.fund(10_000, 129)?;
    j.edit(|parts, market| install_maximum_roster(parts, market, worker_count, evaluator_count))?;
    j.owner(dispatch::SCHEDULE_ACTIVATION, &10u64.to_be_bytes(), 1407)?;
    if evaluator_count < 3 {
        verify_missing_quorum(&mut j, worker_count, evaluator_count)?;
        return Ok(j);
    }
    let frozen = j.open(1408)?;
    assert_eq!(
        (frozen.workers, frozen.evaluators),
        (worker_count, evaluator_count)
    );
    j.advance(1409)?;
    let state = decode_shared_state(&j.bytes)?;
    let roster = epoch::captured_roster(&state)?.ok_or(NOT_FOUND)?;
    let workers = roster.workers().to_vec();
    let policy_section = PolicySection::decode(&j.parts()?.policy)?.encoded_len()?;
    // The empty region already carries its count/presence header; sealing adds a digest.
    let available = Section::PolicyLifecycle.payload_cap() - policy_section - 32;
    let max_tasks = layerx_programs_ai_market::MAX_TASKS.min(available / tasks::TASK_BINDING_BYTES);
    assert!(max_tasks >= workers.len());
    let task_count = task_count.unwrap_or(max_tasks);
    assert!(task_count > 0 && task_count <= max_tasks);
    for nonce in 1..=u8::try_from(task_count).map_err(|_| ARITHMETIC)? {
        // Cancelled tasks fill the available F01 capacity and leave the set sealable.
        let task = j.admit_task(
            &frozen,
            workers[usize::from(nonce - 1) % usize::from(worker_count)].worker,
            nonce,
            1410,
        )?;
        let binding = j.frozen_binding(&frozen)?;
        j.send(
            &bound(
                dispatch::CANCEL_TASK,
                principal(REQUESTER)?,
                &binding,
                tag(0xf0, nonce, 1),
                task.to_vec(),
            ),
            1411,
        )?;
    }
    j.seal(&frozen, &evaluators, 1472)?;
    let scores = entries(
        &workers
            .iter()
            .map(|w| (w.worker, 500_000))
            .collect::<Vec<_>>(),
    );
    // The commit and reveal helpers deliberately use the same production bindings while
    // the heights follow this maximum roster's epoch rather than the small journey's epoch.
    for &n in &evaluators {
        j.commit_score(&frozen, n, &scores, 1472)?;
    }
    for &n in &evaluators {
        j.reveal_score(&frozen, n, &scores, 1488)?;
    }
    j.finish_epoch(&frozen, 1504)?;
    let progress = layerx_programs_ai_market::aggregation::progress(&j.bytes)?;
    assert_eq!(progress.worker_count, u16::from(worker_count));
    assert_eq!(progress.cursor, u16::from(worker_count));
    maximum_roster_reads(&mut j)?;
    Ok(j)
}
fn install_maximum_roster(
    parts: &mut Parts,
    market: &MarketHeader,
    worker_count: u8,
    evaluator_count: u8,
) -> CodecResult<()> {
    // Build the full 32/8 roster with the real approval/admission producers. Rollover
    // resets the four-admissions-per-opened-epoch quota, just as live enrollment does.
    let mut epoch = 0;
    for index in 0..worker_count + evaluator_count {
        if index % 4 == 0 && index > 0 {
            epoch += 1;
            layerx_programs_ai_market::roster::open_epoch(&mut parts.admission, epoch, |_| false)?;
        }
        let at = ORIGIN + epoch * 128 + 1;
        if index < worker_count {
            let n = 0x20 + index;
            let owner = principal(n)?;
            let worker = derive_worker(market.market_id, owner, [n; 32])?;
            let record = worker_record(worker, owner, public(&delegate_key(n)), index, ORIGIN)?;
            parts.workers.insert(&record)?;
            parts
                .replay
                .bind(ActorSlot::worker(usize::from(index))?, owner, version()?)?;
            let participant = Participant::Worker(worker);
            let (effective, digest) = approve(
                &mut parts.admission,
                market,
                participant,
                owner,
                record.delegate,
                at,
            )?;
            parts.admission.admit(
                &admission_ctx(market, owner, at),
                &Admission {
                    participant,
                    delegate_generation: 1,
                    effective_epoch: effective,
                    config_version: 1,
                    approval_digest: digest,
                },
            )?;
        } else {
            let n = index - worker_count + 2;
            let owner = principal(n)?;
            let signing_key = public(&evaluator_key(n));
            let participant =
                Participant::Evaluator(derive_evaluator(market.market_id, owner, [n; 32])?);
            let (effective, digest) = approve(
                &mut parts.admission,
                market,
                participant,
                owner,
                signing_key,
                at,
            )?;
            let grant = grant(market, owner, n, signing_key, effective)?;
            let consent = EvaluatorConsent {
                chain: market.deployment_chain_domain,
                program: market.program_id,
                market: market.market_id,
                evaluator: grant.evaluator,
                owner,
                signing_key,
                enrollment_nonce: [n; 32],
                rubric: grant.rubric,
                approval_digest: digest,
                request: RequestId::new([n; 32])?,
                grant_version: 1,
                key_version: 1,
                effective_epoch: effective,
                config_version: 1,
                expiry_height: ORIGIN + effective * 128 + 64,
            };
            let mut encoded = [0; 362];
            consent.encode(&mut encoded)?;
            let mut payload = encoded.to_vec();
            payload.extend_from_slice(
                &evaluator_key(n)
                    .sign(consent.digest()?.as_bytes())
                    .to_bytes(),
            );
            admit_evaluator(
                &mut parts.admission,
                &admission_ctx(market, owner, at),
                &grant,
                consent.request,
                &payload,
            )?;
            parts
                .replay
                .bind(ActorSlot::evaluator(usize::from(n - 2))?, owner, version()?)?;
            parts.insert_grant(grant, n)?;
        }
    }
    Ok(())
}
fn verify_missing_quorum(j: &mut Journey, worker_count: u8, evaluator_count: u8) -> Checked {
    let call = Req {
        epoch: 10,
        config: 1,
        roster: Presence::Present(RosterDigest::new([0x60; 32])?),
        ..req(
            dispatch::OPEN_EPOCH,
            principal(KEEPER)?,
            tag(0x60, 0, 10),
            Vec::new(),
        )
    };
    let envelope = call.encode()?;
    let ctx = call.context(1408)?;
    let mut host = guest::Guest::new(j.module.clone(), PROGRAM, ASSET)
        .map_err(|e| Failure::Guest(e.to_string()))?;
    host.seed(&j.bytes)
        .map_err(|e| Failure::Guest(e.to_string()))?;
    let observed = host
        .call_refusal(
            &ctx,
            &envelope,
            layerx_programs_ai_market::errors::READINESS_BLOCKED,
        )
        .map_err(|e| Failure::Guest(e.to_string()))?;
    j.rows.push(Row {
        operation: dispatch::OPEN_EPOCH,
        height: 1408,
        state: j.bytes.len(),
        fuel: Some(observed.fuel),
        verdict: Ok(()),
    });
    println!(
            "roster_sweep workers={worker_count} evaluators={evaluator_count} expected=READINESS_BLOCKED reveals=unreachable aggregation=unreachable"
        );
    Ok(())
}
fn maximum_roster_reads(j: &mut Journey) -> Checked {
    j.send(
        &req(
            dispatch::READ_HEADER,
            principal(KEEPER)?,
            [0xf1; 32],
            vec![],
        ),
        1505,
    )?;
    let mut payload = vec![0; 64];
    let n = codec::encode_chunk_request(
        &codec::ChunkRequest {
            revision: j.revision()?,
            digest: Presence::Present(codec::state_digest(&j.bytes)?),
            offset: 0,
            requested: u16::try_from(layerx_programs_ai_market::MAX_CHUNK_BYTES).required(),
        },
        &mut payload,
    )?;
    payload.truncate(n);
    j.send(
        &req(
            dispatch::READ_STATE_CHUNK,
            principal(KEEPER)?,
            [0xf2; 32],
            payload,
        ),
        1505,
    )?;
    Ok(())
}

// Before this repair, candidate seven at 32/8 used OPEN_EPOCH fuel
// [833545, 872662, 908311, 943960, refused at attempted 1000046] and
// FinalizeAggregation [515041, 620550, 621383, 622216, 941581] (16 tasks).
// Candidate six also refused OPEN_EPOCH at attempted 1000003. These capped
// attempts are stopping positions, not completed costs. At candidate six after
// this repair, OPEN_EPOCH uses [720967, 734254, 746850, 759448, 772046, 896924]
// and FinalizeAggregation [491195, 595081, 595795, 596509, 597223, 893640].
// The middle opening calls grow by about 12600 fuel per six participants,
// versus about 35649 per seven before; the last call also commits the roster.
// Every participant_call row below records the other rosters and task counts;
// counts, totals and the unchanged 900000 margin are asserted together.
fn participant_call_totals(rows: &[Row], sweep: &str, tasks: usize, workers: u8, evaluators: u8) {
    let mut calls = std::collections::BTreeMap::<(u16, u64), Vec<&Row>>::new();
    for row in rows.iter().filter(|r| {
        matches!(
            r.operation,
            dispatch::OPEN_EPOCH | dispatch::FinalizeAggregation
        )
    }) {
        calls
            .entry((row.operation.selector(), row.height))
            .or_default()
            .push(row);
    }
    for ((selector, height), rows) in calls {
        let total: Option<u64> = rows.iter().map(|row| row.fuel).sum();
        println!(
            "participant_calls sweep={sweep} tasks={tasks} workers={workers} evaluators={evaluators} selector={selector:04x} height={height} calls={} total_fuel={total:?}",
            rows.len()
        );
        for (call, row) in rows.iter().enumerate() {
            println!(
                "participant_call sweep={sweep} tasks={tasks} workers={workers} evaluators={evaluators} selector={selector:04x} height={height} call={call} fuel={:?} verdict={:?}",
                row.fuel, row.verdict
            );
        }
    }
    if workers == 32 && evaluators == 8 {
        for row in rows.iter().filter(|row| {
            matches!(
                row.operation,
                dispatch::OPEN_EPOCH | dispatch::FinalizeAggregation
            )
        }) {
            assert!(
                row.fuel.is_some_and(|fuel| fuel > 0 && fuel <= 900_000),
                "each participant chunk at 32/8 must retain fuel margin: {:?}",
                row.fuel
            );
        }
    }
}

#[derive(Clone, Copy)]
struct FuelRatio {
    numerator: i128,
    denominator: u8,
}

impl FuelRatio {
    fn delta(now: u64, old: u64, units: u8) -> Self {
        assert!(units.is_power_of_two());
        Self {
            numerator: i128::from(now) - i128::from(old),
            denominator: units,
        }
    }

    fn project(self, fuel: u64, units: u8) -> Self {
        Self {
            numerator: i128::from(fuel) * i128::from(self.denominator)
                + self.numerator * i128::from(units),
            denominator: self.denominator,
        }
    }

    fn format(&self, f: &mut fmt::Formatter<'_>, integral_suffix: &str) -> fmt::Result {
        let numerator = self.numerator.unsigned_abs();
        let denominator = u128::from(self.denominator);
        if self.numerator < 0 {
            f.write_str("-")?;
        }
        write!(f, "{}", numerator / denominator)?;
        let mut remainder = numerator % denominator;
        if remainder == 0 {
            return f.write_str(integral_suffix);
        }
        f.write_str(".")?;
        while remainder != 0 {
            remainder *= 10;
            write!(f, "{}", remainder / denominator)?;
            remainder %= denominator;
        }
        Ok(())
    }
}

impl fmt::Debug for FuelRatio {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        self.format(f, ".0")
    }
}

impl fmt::Display for FuelRatio {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        self.format(f, "")
    }
}

#[test]
fn maximum_state_fuel_scaling() -> Checked {
    let module = guest_module()?;
    let mut previous: Option<(usize, Option<u64>, Option<u64>)> = None;
    let mut failures = Vec::new();
    for tasks in [16, 32, 48, 64] {
        let measured = maximum_journey_with_tasks(module.clone(), Some(tasks))?;
        participant_call_totals(&measured.rows, "maximum", tasks, 32, 8);
        let opening = measured
            .rows
            .iter()
            .find(|row| row.operation == dispatch::OPEN_EPOCH && row.height == 1408)
            .required_with("maximum roster opening must execute");
        let admission = measured
            .rows
            .iter()
            .rev()
            .find(|row| row.operation == dispatch::ADMIT_TASK && row.height == 1410)
            .required_with("last task admission must execute");
        let slope = |now: Option<u64>, old: Option<u64>, count: usize| {
            now.zip(old)
                .map(|(now, old)| FuelRatio::delta(now, old, u8::try_from(count).required()))
        };
        let (open_per_task, admit_per_task) = match previous {
            Some((count, open, admit)) => (
                slope(opening.fuel, open, tasks - count),
                slope(admission.fuel, admit, tasks - count),
            ),
            None => (None, None),
        };
        println!(
            "sweep tasks={tasks} workers=32 evaluators=8 open_height=1408 open_fuel={:?} admit_height=1410 admit_fuel={:?} open_per_added_task={open_per_task:?} admit_per_added_task={admit_per_task:?}",
            opening.fuel, admission.fuel
        );
        for (index, row) in measured.rows.iter().enumerate() {
            println!(
                "sweep tasks={tasks} row={index} operation={} height={} state_bytes={} fuel={:?} verdict={:?}",
                row.operation.metadata().name,
                row.height,
                row.state,
                row.fuel,
                row.verdict
            );
            if row
                .fuel
                .is_none_or(|fuel| fuel == 0 || fuel > DEFAULT_CPU_FUEL)
                || row.verdict.is_err()
            {
                failures.push((
                    tasks,
                    index,
                    row.operation.metadata().name,
                    row.fuel,
                    row.verdict.clone(),
                ));
            }
        }
        previous = Some((tasks, opening.fuel, admission.fuel));
    }
    assert!(
        failures.is_empty(),
        "maximum-state sweep failures: {failures:?}"
    );
    Ok(())
}

#[test]
fn fixed_roster_task_fuel_scaling() -> Checked {
    let module = guest_module()?;
    let mut previous = std::collections::BTreeMap::new();
    let mut failures = Vec::new();
    for tasks in [16, 32, 48, 64] {
        let mut measured = maximum_journey_with_roster(module.clone(), Some(tasks), 16, 4)?;
        let state = decode_shared_state(&measured.bytes)?;
        let policy = PolicySection::decode(state.section(Section::PolicyLifecycle)?)?;
        let set = TaskSet::decode(policy.task_region)?;
        assert_eq!(set.len(), tasks);
        assert!(set.seal().is_some());
        let initial_opening = measured
            .rows
            .iter()
            .find(|row| row.operation == dispatch::OPEN_EPOCH)
            .required_with("initial opening must execute");
        println!(
            "fixed_roster_opening_control tasks={tasks} workers=16 evaluators=4 admitted_tasks_at_opening=0 height={} state_bytes={} fuel={:?} verdict={:?}",
            initial_opening.height,
            initial_opening.state,
            initial_opening.fuel,
            initial_opening.verdict
        );
        // The first opening precedes admission. Open the following epoch so its
        // input actually contains the task rows produced by this journey.
        let opening_state = measured.bytes.len();
        let opened = measured.open(1536)?;
        assert_eq!((opened.workers, opened.evaluators), (16, 4));
        participant_call_totals(&measured.rows, "fixed_roster", tasks, 16, 4);
        for operation in [
            dispatch::OPEN_EPOCH,
            dispatch::BeginAggregation,
            dispatch::FinalizeAggregation,
        ] {
            let row = measured
                .rows
                .iter()
                .rev()
                .find(|row| row.operation == operation)
                .required_with("fixed-roster handler must execute");
            let slope = row.fuel.and_then(|fuel| {
                previous
                    .insert(operation.selector(), (tasks, fuel))
                    .map(|(old_tasks, old_fuel)| {
                        FuelRatio::delta(fuel, old_fuel, u8::try_from(tasks - old_tasks).required())
                    })
            });
            println!(
                "fixed_roster_task_sweep tasks={tasks} workers=16 evaluators=4 operation={} height={} state_bytes={} opening_input_bytes={opening_state} fuel={:?} fuel_per_added_task={slope:?} verdict={:?}",
                row.operation.metadata().name,
                row.height,
                row.state,
                row.fuel,
                row.verdict
            );
            if row
                .fuel
                .is_none_or(|fuel| fuel == 0 || fuel > DEFAULT_CPU_FUEL)
                || row.verdict.is_err()
            {
                failures.push((
                    tasks,
                    operation.metadata().name,
                    row.fuel,
                    row.verdict.clone(),
                ));
            }
        }
    }
    assert!(
        failures.is_empty(),
        "fixed-roster task sweep failures: {failures:?}"
    );
    Ok(())
}

#[test]
fn aggregation_receipt_index_preserves_routed_refusals() -> Checked {
    let module = guest_module()?;
    AGGREGATION_INPUT.with(|input| *input.borrow_mut() = Some(Vec::new()));
    let _journey = maximum_journey_with_roster(module.clone(), Some(16), 16, 4)?;
    let inputs = AGGREGATION_INPUT.with(|input| input.borrow_mut().take().required());
    let (ctx, state, envelope) = inputs.first().required_with("real first aggregation chunk");
    let frame = codec::decode_state(state)?;
    let reports = frame.sections[Section::CurrentReports.index()];
    assert_eq!(reports[8], 4);
    let first = 9;
    let signed = first + 82;
    let signed_len = usize::from(u16::from_be_bytes(
        reports[first + 80..signed].try_into().required(),
    ));
    let second = signed + signed_len;
    let cases = invalid_aggregation_receipts(reports, first, signed, second);
    for (name, reports, expected) in cases {
        let mut frame = frame;
        frame.sections[Section::CurrentReports.index()] = &reports;
        let mut corrupted = vec![0; MAX_STATE_BYTES];
        let len = codec::encode_state(&frame, &mut corrupted)?;
        corrupted.truncate(len);
        let mut next = vec![0; MAX_STATE_BYTES];
        let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
        let mut event = vec![0; MAX_EVENT_BYTES];
        let mut result = vec![0; MAX_RESULT_BYTES];
        let outcome = dispatch::route(
            ctx,
            envelope,
            Some(&corrupted),
            dispatch::Buffers {
                next: &mut next,
                scratch: &mut scratch,
                event: &mut event,
                result: &mut result,
            },
        )?;
        let Routed::Refused { result_len } = outcome else {
            panic!("{name} was admitted: {outcome:?}")
        };
        assert_eq!(
            codec::decode_result(&result[..result_len])?.error,
            Some(expected),
            "{name}"
        );
        let mut compiled = guest::Guest::new(module.clone(), PROGRAM, ASSET)
            .map_err(|e| Failure::Guest(e.to_string()))?;
        compiled
            .seed(&corrupted)
            .map_err(|e| Failure::Guest(e.to_string()))?;
        let observed = compiled
            .call_refusal(ctx, envelope, expected)
            .map_err(|e| Failure::Guest(format!("{name}: {e}")))?;
        assert!(
            observed.fuel > 0 && observed.fuel <= DEFAULT_CPU_FUEL,
            "{name}"
        );
        assert_eq!(observed.transfers, 0, "{name}");
        assert!(observed.events.is_empty(), "{name}");
        assert_eq!(observed.response, result[..result_len], "{name}");
    }
    Ok(())
}
type InvalidReceipt = (&'static str, Vec<u8>, ApplicationError);
fn invalid_aggregation_receipts(
    reports: &[u8],
    first: usize,
    signed: usize,
    second: usize,
) -> Vec<InvalidReceipt> {
    use layerx_programs_ai_market::errors::{
        F03_SCORE_RANGE, F05_REPORT_INVARIANT, NON_CANONICAL, WRONG_ROSTER,
    };
    let mut cases = Vec::new();
    let mut duplicate = reports.to_vec();
    duplicate[second..second + 32].copy_from_slice(&reports[first..first + 32]);
    cases.push(("duplicate evaluator", duplicate, NON_CANONICAL));
    let mut zero = reports.to_vec();
    zero[first..first + 32].fill(0);
    cases.push(("zero receipt evaluator", zero, NON_CANONICAL));
    let mut short = reports.to_vec();
    short[first + 80..signed].fill(0);
    cases.push(("invalid signed length", short, NON_CANONICAL));
    let mut count = reports.to_vec();
    count[8] = 9;
    cases.push(("receipt capacity", count, NON_CANONICAL));
    for (name, start, end) in [
        ("zero report grant version", 178, 186),
        ("zero report key version", 186, 194),
        ("zero report evidence", 194, 226),
    ] {
        let mut invalid = reports.to_vec();
        invalid[signed + start..signed + end].fill(0);
        cases.push((name, invalid, NON_CANONICAL));
    }
    let mut identity = reports.to_vec();
    identity[signed + 228..signed + 260].fill(0);
    cases.push(("zero chunk worker", identity, NON_CANONICAL));
    let mut repeated_identity = reports.to_vec();
    repeated_identity[second + 82 + 228..second + 82 + 260].fill(0);
    cases.push((
        "second report chunk worker",
        repeated_identity,
        NON_CANONICAL,
    ));
    let mut repeated_range = reports.to_vec();
    repeated_range[second + 82 + 260..second + 82 + 264]
        .copy_from_slice(&1_000_001u32.to_be_bytes());
    cases.push(("second report score range", repeated_range, F03_SCORE_RANGE));
    let mut range = reports.to_vec();
    range[signed + 260..signed + 264].copy_from_slice(&1_000_001u32.to_be_bytes());
    cases.push(("score range", range, F03_SCORE_RANGE));
    let mut evaluator = reports.to_vec();
    evaluator[signed + 146] ^= 1;
    cases.push(("signed evaluator mismatch", evaluator, F05_REPORT_INVARIANT));
    let mut roster = reports.to_vec();
    roster[signed + 114] ^= 1;
    cases.push(("frozen roster mismatch", roster, WRONG_ROSTER));
    cases
}

fn state_decode_baseline(
    module: &Arc<ValidatedModule>,
    ctx: &CallContext,
    current: &[u8],
) -> Checked<u64> {
    let shared = decode_shared_state(current)?;
    let workers =
        WorkerTable::decode(split_identity_section(shared.section(Section::IdentityRoster)?)?.0)?;
    let peer = workers.iter().next().ok_or(NOT_FOUND)?;
    let mut chunk = vec![0; 64];
    let n = codec::encode_chunk_request(
        &codec::ChunkRequest {
            revision: shared.revision,
            digest: Presence::Present(codec::state_digest(current)?),
            offset: 0,
            requested: u16::try_from(layerx_programs_ai_market::MAX_CHUNK_BYTES).required(),
        },
        &mut chunk,
    )?;
    chunk.truncate(n);
    let mut reads = Vec::new();
    // The F08 enrollment view in f11_views reads the worker owner's key.
    // Keep the first peer and also check the last peer on this same snapshot.
    for peer in [peer, workers.iter().last().ok_or(NOT_FOUND)?] {
        let mut envelope = vec![0; MAX_ENVELOPE_BYTES];
        let n = layerx_programs_ai_market::queries::encode_view_envelope(
            &Envelope {
                operation: dispatch::PeerView,
                chain: ctx.chain,
                program: ctx.program,
                market: derive_market(ctx.chain, ctx.program)?,
                actor: ctx.principal,
                epoch: 0,
                config: 0,
                roster: Presence::Absent,
                sequence: 0,
                expiry: u64::MAX,
                request: RequestId::new([0xd6; 32])?,
                payload: peer.owner.as_bytes(),
                authentication: Authentication::Native,
            },
            &mut envelope,
        )?;
        envelope.truncate(n);
        reads.push((dispatch::PeerView, envelope));
    }
    // READ_HEADER has an empty request; encode_read_header encodes its response.
    reads.push((
        dispatch::READ_HEADER,
        req(dispatch::READ_HEADER, ctx.principal, [0xd4; 32], vec![]).encode()?,
    ));
    reads.push((
        dispatch::READ_STATE_CHUNK,
        req(dispatch::READ_STATE_CHUNK, ctx.principal, [0xd5; 32], chunk).encode()?,
    ));
    let mut cheapest = u64::MAX;
    for (operation, envelope) in reads {
        if let Some(fuel) = measure_baseline_read(module, ctx, current, operation, &envelope)? {
            cheapest = cheapest.min(fuel);
        }
    }
    assert_ne!(cheapest, u64::MAX, "at least one successful baseline read");
    Ok(cheapest)
}
fn measure_baseline_read(
    module: &Arc<ValidatedModule>,
    ctx: &CallContext,
    current: &[u8],
    operation: Operation,
    envelope: &[u8],
) -> Checked<Option<u64>> {
    let shared = decode_shared_state(current)?;
    let workers =
        WorkerTable::decode(split_identity_section(shared.section(Section::IdentityRoster)?)?.0)?;

    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    let routed = dispatch::route(
        ctx,
        envelope,
        Some(current),
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?;
    let result_len = match routed {
        Routed::Unchanged { result_len } => result_len,
        Routed::Refused { result_len } => {
            let frame = codec::decode_result(&result[..result_len])?;
            let error = frame
                .error
                .required_with("baseline refusal must name its error");
            println!(
                "state_decode_baseline_refused operation={} height={} state_bytes={} error={}",
                operation.metadata().name,
                ctx.height,
                current.len(),
                describe(&result[..result_len]),
            );
            let mut compiled = guest::Guest::new(module.clone(), PROGRAM, ASSET)
                .map_err(|e| Failure::Guest(e.to_string()))?;
            compiled
                .seed(current)
                .map_err(|e| Failure::Guest(e.to_string()))?;
            let observed = compiled
                .call_refusal(ctx, envelope, error)
                .map_err(|e| Failure::Guest(e.to_string()))?;
            assert_eq!(observed.response, result[..result_len]);
            assert!(observed.fuel > 0 && observed.fuel <= DEFAULT_CPU_FUEL);
            assert_eq!(observed.transfers, 0);
            assert!(observed.events.is_empty());
            assert_eq!(
                compiled
                    .state()
                    .map_err(|e| Failure::Guest(e.to_string()))?
                    .as_deref(),
                Some(current)
            );
            return Ok(None);
        }
        Routed::Applied { .. } => panic!(
            "baseline {} must be read-only: {routed:?}",
            operation.metadata().name
        ),
    };
    let frame = codec::decode_result(&result[..result_len])?;
    assert_eq!(frame.status, codec::ResultStatus::Ok);
    assert_eq!(frame.error, None);
    assert_eq!(frame.revision, shared.revision);
    verify_baseline_payload(ctx, current, operation, envelope, &frame, &workers)?;
    let (fuel, verdict) = run_guest(
        module,
        ctx,
        Some(current),
        envelope,
        &Expected {
            state: current,
            topic: &[],
            event: &[],
            result: &result[..result_len],
        },
    )?;
    assert!(verdict.is_ok(), "baseline: {verdict:?}");
    let fuel = fuel.required_with("read-only baseline must report measured fuel");
    assert!(fuel > 0 && fuel <= DEFAULT_CPU_FUEL);
    println!(
        "state_decode_baseline operation={} height={} state_bytes={} fuel={fuel}",
        operation.metadata().name,
        ctx.height,
        current.len()
    );
    Ok(Some(fuel))
}
fn verify_baseline_payload(
    ctx: &CallContext,
    current: &[u8],
    operation: Operation,
    envelope: &[u8],
    frame: &codec::ApplicationResult<'_>,
    workers: &WorkerTable,
) -> CodecResult<()> {
    if operation == dispatch::PeerView {
        let view = layerx_programs_ai_market::queries::PeerView::decode(frame.payload)?;
        let request = layerx_programs_ai_market::queries::decode_view_envelope(envelope)?;
        let peer = workers
            .iter()
            .find(|peer| peer.owner.as_bytes() == request.envelope.payload)
            .required_with("F08 view owner from the real worker table");
        assert_eq!(view.peer_key, peer.owner);
        assert_eq!(view.worker, peer.worker);
        assert_eq!(view.delegate, peer.delegate);
        assert_eq!(view.metadata, peer.metadata);
        assert_eq!(view.generation, peer.generation);
        assert_eq!(view.key_version, peer.key_version);
        assert_eq!(view.metadata_revision, peer.metadata_revision);
        assert_eq!(view.valid_from, peer.valid_from);
        assert_eq!(view.expiry, peer.expiry);
        assert_eq!(view.state, peer.state);
        assert!(view.enrolled);
    } else {
        let mut strict = vec![0; MAX_RESULT_BYTES];
        let n = if operation == dispatch::READ_HEADER {
            let header =
                layerx_programs_ai_market::queries::read_header(current, ctx.chain, ctx.program)?;
            codec::encode_read_header(&header, &mut strict)?
        } else {
            let request = codec::decode_envelope(envelope)?;
            layerx_programs_ai_market::queries::read_state_chunk(
                current,
                request.envelope.payload,
                &mut strict,
            )?
        };
        assert_eq!(frame.payload, &strict[..n]);
    }
    Ok(())
}

#[test]
fn roster_state_fuel_scaling() -> Checked {
    let module = guest_module()?;
    let baseline =
        guest::aggregation_baseline_module().map_err(|e| Failure::Guest(e.to_string()))?;
    let mut measurements = RosterMeasurements::default();
    let original_rosters = [(8, 2), (16, 4), (24, 6), (32, 8)];
    for (workers, evaluators) in original_rosters.into_iter().chain([(16, 8), (32, 4)]) {
        measurements.measure(&module, &baseline, workers, evaluators, &original_rosters)?;
    }
    let RosterMeasurements {
        pass_measurements,
        failures,
        handler_fuel,
        first_chunks,
        read_baselines,
    } = measurements;
    // The separate baseline guest measures the three candidates on exactly the
    // first ProcessAggregation input at 16/4 and 32/8; aggregation_passes prints
    // their pairs plus the chunk reader, frame copy and control comparison.
    // Baseline pairs measured at e1722222b475 (16/4, 32/8): frame decode
    // (3572, 3572), replay decode/encode (38478, 65466), full roster load
    // (115850, 181783). The actual chunk roster read is (18005, 18005).
    // Process phase pairs: votes (66026, 130658), median (14459, 22651),
    // commitments (6671, 8919); frame copy (4227, 4633). The remaining growth
    // is in report-cell projection, including equal 32-byte identity ordering,
    // rather than a replay or roster rescan. Checked borrowed score cells and
    // wordwise lexicographic ordering retain every identity/range/order check.
    // ProcessAggregation itself already preserves replay bytes (zero replay
    // decodes/encodes) and reads only eight snapshot workers. The full-roster
    // measurement is the Begin/Finalize reader, not an extra Process pass.
    // Attempt 22's first repaired pair at d57ed245750a was 118828/148707
    // (16/4, 32/8), down from 175338/251901 but still outside ten percent.
    // Its remaining per-report work parsed receipts twice and reconstructed
    // sealed commitments. Borrow those checked fields and encoded commitments,
    // project the contiguous eight cells, and preserve median comparisons.
    // Attempt 22 projects complete reports by the eight chunk assignments;
    // sparse reports search only those assignments with checked borrowed cells.
    // The new 16/4 and 32/8 pair is recorded as chunk_projection_pair in this
    // case's private hf-done evidence, alongside the per-row roster measurements.
    // Attempt 23 starts from 109008/130979 (16/4, 32/8). Index each
    // committed receipt once and project the same eight checked score columns.
    // At 3702c3b02066, the index-only pair was 108399/129062. Its actual
    // 16/4 and 32/8 process phase pairs were: envelope 3490/3490, roster
    // 11892/11892, commitments 4519/5927, votes 11235/21611, median
    // 6068/9428, settlement 9854/9858, commit 9619/10025; receipt indexing
    // 22068/23944, control comparison 3406/6331. Roster loading is flat;
    // the remaining work repeats per evaluator, not per roster worker.
    // Accumulate the same insertion median while projecting each score column,
    // borrow evaluator identities, and compare checked words directly so each
    // identity is loaded once. Every original count, score, ordering, binding
    // and comparison-count bound remains. The final pair and all phases are
    // emitted below and into the bounded gate's private evidence log.
    // Decision 3175 compares equal evaluator counts across roster sizes and
    // equal participant counts across evaluator counts. Read-only baselines
    // remain diagnostics: they cost more than ProcessAggregation itself.
    let chunk_16_4 = *first_chunks.get(&(16, 4)).required_with("16/4 first chunk");
    let chunk_16_8 = *first_chunks.get(&(16, 8)).required_with("16/8 first chunk");
    let chunk_32_4 = *first_chunks.get(&(32, 4)).required_with("32/4 first chunk");
    let chunk_32_8 = *first_chunks.get(&(32, 8)).required_with("32/8 first chunk");
    println!(
        "chunk_projection_pair workers16_evaluators4={chunk_16_4} workers16_evaluators8={chunk_16_8} workers32_evaluators4={chunk_32_4} workers32_evaluators8={chunk_32_8} read_baselines={read_baselines:?}"
    );
    // The frozen roster digest is checked above, and every semantic comparison
    // and the ten-percent/900000 assertions below remain in force.
    assert_eq!(pass_measurements.len(), 4);
    for (selector, rows) in &handler_fuel {
        let mut previous = std::collections::BTreeMap::new();
        let mut roster = 0;
        let mut chunk = 0;
        for &(workers, fuel) in rows {
            if workers != roster {
                roster = workers;
                chunk = 0;
            }
            if let Some((old_workers, old_fuel)) = previous.insert(chunk, (workers, fuel)) {
                let slope = FuelRatio::delta(fuel, old_fuel, workers - old_workers);
                println!(
                    "handler_slope selector={selector:04x} chunk={chunk} workers={workers} fuel={fuel} fuel_per_added_worker={} implied_32={}",
                    slope,
                    slope.project(fuel, 32 - workers)
                );
            }
            chunk += 1;
        }
    }
    assert!(
        failures.is_empty(),
        "roster-state sweep failures: {failures:?}"
    );
    for operation in [
        dispatch::OPEN_EPOCH,
        dispatch::BeginAggregation,
        dispatch::FinalizeAggregation,
    ] {
        let full = handler_fuel
            .get(&operation.selector())
            .and_then(|rows| rows.iter().find(|(workers, _)| *workers == 32))
            .required_with("full roster handler measurement");
        assert!(
            full.1 <= 900_000,
            "{} at 32/8 must retain fuel margin: {}",
            operation.metadata().name,
            full.1
        );
    }
    assert!(
        chunk_32_8 * 100 <= chunk_16_8 * 110,
        "at eight evaluators, the first chunk must remain within ten percent across rosters: 16/4={chunk_16_4}, 16/8={chunk_16_8}, 32/4={chunk_32_4}, 32/8={chunk_32_8}"
    );
    assert!(
        chunk_16_8 <= chunk_16_4 * 2,
        "at sixteen participants, doubling evaluators must cost at most twice the first chunk: 16/4={chunk_16_4}, 16/8={chunk_16_8}, 32/4={chunk_32_4}, 32/8={chunk_32_8}"
    );
    Ok(())
}
type AggregationSample = (CallContext, Vec<u8>, Vec<u8>);
type RosterFailure = (u8, u8, usize, &'static str, Option<u64>, Result<(), String>);
#[derive(Default)]
struct RosterMeasurements {
    pass_measurements: Vec<(u8, Vec<u64>)>,
    failures: Vec<RosterFailure>,
    handler_fuel: std::collections::BTreeMap<u16, Vec<(u8, u64)>>,
    first_chunks: std::collections::BTreeMap<(u8, u8), u64>,
    read_baselines: std::collections::BTreeMap<(u8, u8), u64>,
}
impl RosterMeasurements {
    fn measure(
        &mut self,
        module: &Arc<ValidatedModule>,
        baseline: &Arc<ValidatedModule>,
        workers: u8,
        evaluators: u8,
        original_rosters: &[(u8, u8)],
    ) -> Checked {
        AGGREGATION_INPUT.with(|input| *input.borrow_mut() = Some(Vec::new()));
        let measured =
            maximum_journey_with_roster(Arc::clone(module), Some(16), workers, evaluators)?;
        let inputs = AGGREGATION_INPUT.with(|input| input.borrow_mut().take().required());
        self.measure_read_baseline(module, &inputs, &measured, workers, evaluators)?;
        if workers == 16 || workers == 32 {
            let values = aggregation_probe(baseline, &inputs, workers, evaluators)?;
            self.pass_measurements.push((workers, values));
        }
        participant_call_totals(&measured.rows, "roster", 16, workers, evaluators);
        let mut selected = 0;
        for (index, row) in measured.rows.iter().enumerate() {
            if !matches!(
                row.operation,
                dispatch::OPEN_EPOCH
                    | dispatch::RevealScore
                    | dispatch::BeginAggregation
                    | dispatch::ProcessAggregation
                    | dispatch::FinalizeAggregation
            ) {
                continue;
            }
            selected += 1;
            if row.operation != dispatch::RevealScore {
                if let Some(fuel) = row.fuel {
                    if original_rosters.contains(&(workers, evaluators)) {
                        self.handler_fuel
                            .entry(row.operation.selector())
                            .or_default()
                            .push((workers, fuel));
                    }
                    if row.operation == dispatch::ProcessAggregation {
                        self.first_chunks
                            .entry((workers, evaluators))
                            .or_insert(fuel);
                    }
                }
            }
            println!(
                "roster_sweep tasks=16 workers={workers} evaluators={evaluators} row={index} operation={} height={} state_bytes={} fuel={:?} verdict={:?}",
                row.operation.metadata().name,
                row.height,
                row.state,
                row.fuel,
                row.verdict
            );
            if row
                .fuel
                .is_none_or(|fuel| fuel == 0 || fuel > DEFAULT_CPU_FUEL)
                || row.verdict.is_err()
            {
                self.failures.push((
                    workers,
                    evaluators,
                    index,
                    row.operation.metadata().name,
                    row.fuel,
                    row.verdict.clone(),
                ));
            }
        }
        assert_eq!(
            selected,
            if evaluators < 3 {
                1
            } else {
                usize::from(evaluators)
                    + 1
                    + usize::from(workers).div_ceil(8)
                    + 2 * usize::from(workers).div_ceil(
                        layerx_programs_ai_market::aggregation::participant_chunk_size(
                            usize::from(workers),
                        ),
                    )
            }
        );
        Ok(())
    }
    fn measure_read_baseline(
        &mut self,
        module: &Arc<ValidatedModule>,
        inputs: &[AggregationSample],
        measured: &Journey,
        workers: u8,
        evaluators: u8,
    ) -> Checked {
        let (read_ctx, read_state) = inputs.first().map_or_else(
            || {
                (
                    CallContext {
                        chain: ChainDomain::new(CHAIN).required_with("chain"),
                        program: ProgramId::new(PROGRAM).required_with("program"),
                        principal: principal(KEEPER).required_with("reader"),
                        height: measured
                            .rows
                            .last()
                            .required_with("roster journey row")
                            .height,
                    },
                    measured.bytes.as_slice(),
                )
            },
            |(ctx, state, _)| (*ctx, state.as_slice()),
        );
        let read_fuel = state_decode_baseline(module, &read_ctx, read_state)?;
        println!(
            "roster_state_decode_baseline workers={workers} evaluators={evaluators} height={} state_bytes={} fuel={read_fuel}",
            read_ctx.height,
            read_state.len()
        );
        self.read_baselines.insert((workers, evaluators), read_fuel);
        Ok(())
    }
}
fn aggregation_probe(
    baseline: &Arc<ValidatedModule>,
    inputs: &[AggregationSample],
    workers: u8,
    evaluators: u8,
) -> Checked<Vec<u64>> {
    let (ctx, state, envelope) = inputs
        .first()
        .required_with("first aggregation chunk input");
    let mut probe = guest::Guest::new(baseline.clone(), PROGRAM, ASSET)
        .map_err(|e| Failure::Guest(e.to_string()))?;
    probe
        .seed(state)
        .map_err(|e| Failure::Guest(e.to_string()))?;
    let observed = probe
        .call(ctx, envelope, None)
        .map_err(|e| Failure::Guest(e.to_string()))?;
    assert_eq!(observed.transfers, 0);
    assert!(observed.events.is_empty());
    assert_eq!(
        probe
            .state()
            .map_err(|e| Failure::Guest(e.to_string()))?
            .as_deref(),
        Some(state.as_slice())
    );
    assert_eq!(observed.response.len(), 144);
    let values: Vec<u64> = observed.response[..48]
        .chunks_exact(8)
        .map(|value| u64::from_be_bytes(value.try_into().required()))
        .collect();
    assert!(values.iter().all(|fuel| *fuel > 0));
    let envelope = codec::decode_envelope(envelope)?;
    let Presence::Present(roster) = envelope.envelope.roster else {
        panic!("frozen roster")
    };
    assert_eq!(&observed.response[104..136], roster.as_bytes());
    let phases: Vec<u64> = observed.response[48..104]
        .chunks_exact(8)
        .map(|value| u64::from_be_bytes(value.try_into().required()))
        .collect();
    println!(
                "aggregation_passes workers={workers} evaluators={evaluators} frame_decode={} replay_decode_encode={} full_roster={} chunk_roster={} frame_copy={} control_compare={}",
                values[0], values[1], values[2], values[3], values[4], values[5]
            );
    println!(
                "aggregation_phases workers={workers} evaluators={evaluators} envelope={} roster={} commitments={} votes={} median={} settlement={} commit={}",
                phases[0], phases[1], phases[2], phases[3], phases[4], phases[5], phases[6]
            );
    let index_fuel = u64::from_be_bytes(observed.response[136..144].try_into().required());
    assert!(index_fuel > 0);
    println!(
        "aggregation_receipt_index workers={workers} evaluators={evaluators} fuel={index_fuel}"
    );
    Ok(values)
}

#[test]
fn opening_preserves_control_and_retires_only_departing_worker() -> Checked {
    let mut j = Journey {
        module: guest_module()?,
        bytes: Vec::new(),
        owner_sequence: 2,
        rows: Vec::new(),
    };
    j.registry()?;
    let low = j.enroll(LOW, 130)?;
    for n in 2..=4 {
        j.evaluator(n, 129 + u64::from(n))?;
    }
    j.owner(dispatch::SCHEDULE_ACTIVATION, &6u64.to_be_bytes(), 134)?;
    j.fund(500, 135)?;
    let before = codec::decode_state(&j.bytes)?.sections[Section::Control.index()].to_vec();
    let frozen = j.open(896)?;
    assert_eq!(
        codec::decode_state(&j.bytes)?.sections[Section::Control.index()],
        before
    );
    j.advance(897)?;
    let high = j.enroll(HIGH, 900)?;
    let mut payload = participant_payload(Participant::Worker(low.worker));
    payload.extend_from_slice(&u64s(&[1, 7]));
    payload.push(1);
    j.send(
        &Req {
            sequence: 1,
            expiry: 950,
            ..req(dispatch::RequestExit, low.owner, [0xe7; 32], payload)
        },
        901,
    )?;
    j.seal(&frozen, &[], 960)?;
    j.finish_epoch(&frozen, 1008)?;
    let before = decode_shared_state(&j.bytes)?;
    let survivor = before.control.replay.actor(ActorSlot::worker(1)?);
    assert!(before.control.replay.actor(ActorSlot::worker(0)?).is_some());
    let next = j.open(1024)?;
    assert_eq!(next.workers, 1);
    let after = decode_shared_state(&j.bytes)?;
    assert!(after.control.replay.actor(ActorSlot::worker(0)?).is_none());
    assert_eq!(after.control.replay.actor(ActorSlot::worker(1)?), survivor);
    let captured = epoch::captured_roster(&after)?.ok_or(NOT_FOUND)?;
    assert_eq!(captured.workers()[0].worker, high.worker);
    for row in &j.rows {
        assert!(
            row.verdict.is_ok(),
            "{}: {:?}",
            row.operation.metadata().name,
            row.verdict
        );
        assert!(row
            .fuel
            .is_some_and(|fuel| fuel > 0 && fuel <= DEFAULT_CPU_FUEL));
    }
    Ok(())
}

#[test]
fn aggregation_seal_preserves_suspended_history_bytes() -> Checked {
    use layerx_programs_ai_market::{
        reputation::{HistoryStatus, CURRENT_BYTES, HEADER_BYTES, HISTORY_BYTES},
        reputation_transition::split_joint,
        Score,
    };
    let mut j = Journey {
        module: guest_module()?,
        bytes: Vec::new(),
        owner_sequence: 2,
        rows: Vec::new(),
    };
    j.registry()?;
    let worker = j.enroll(LOW, 130)?;
    for n in 2..=4 {
        j.evaluator(n, 129 + u64::from(n))?;
    }
    j.owner(dispatch::SCHEDULE_ACTIVATION, &6u64.to_be_bytes(), 134)?;
    j.fund(500, 135)?;
    let frozen = j.open(896)?;
    j.advance(897)?;
    let state = decode_shared_state(&j.bytes)?;
    let (region, _) = split_joint(state.section(Section::ReputationAdmission)?)?;
    let region = region.ok_or(NOT_FOUND)?;
    let record = *region
        .state
        .records()
        .find(|r| r.worker == worker.worker)
        .ok_or(NOT_FOUND)?;
    let mut payload = worker.worker.bytes().to_vec();
    payload.extend_from_slice(record.segment.as_bytes());
    payload.extend_from_slice(&record.reset_generation.get().to_be_bytes());
    payload.push(1);
    j.owner_raw(dispatch::SuspendHistory, payload, 898)?;
    j.seal(&frozen, &[2, 3, 4], 960)?;
    let scores = entries(&[(worker.worker, 500_000)]);
    for n in 2..=4 {
        j.commit_score(&frozen, n, &scores, 960)?;
    }
    for n in 2..=4 {
        j.reveal_score(&frozen, n, &scores, 976)?;
    }
    let before = decode_shared_state(&j.bytes)?;
    let before_joint = before.section(Section::ReputationAdmission)?.to_vec();
    let (before_region, before_table) = split_joint(&before_joint)?;
    let before_region = before_region.ok_or(NOT_FOUND)?;
    let base_len = HEADER_BYTES
        + before_region.state.records().count() * CURRENT_BYTES
        + before_region.state.completed().count() * HISTORY_BYTES;
    assert_eq!(base_len, before_joint.len() - before_table.len());
    j.object(dispatch::BeginAggregation, &frozen, vec![], 1008)?;
    let after = decode_shared_state(&j.bytes)?;
    let joint = after.section(Section::ReputationAdmission)?;
    assert_eq!(&joint[..base_len], &before_joint[..base_len]);
    let (sealed, table) = split_joint(joint)?;
    assert_eq!(table, before_table);
    let sealed = sealed.ok_or(NOT_FOUND)?;
    let Presence::Present(seal) = sealed.seal else {
        panic!("begin must append a history seal");
    };
    assert_eq!(seal.eligible, 3);
    assert!(!seal.allows(0));
    // The exact Begin retry, remaining chunk and terminal transition all travel
    // through the native router and the same compiled ABI-5 guest.
    j.finish_epoch(&frozen, 1008)?;
    let terminal = decode_shared_state(&j.bytes)?;
    let (region, _) = split_joint(terminal.section(Section::ReputationAdmission)?)?;
    let region = region.ok_or(NOT_FOUND)?;
    assert_eq!(region.seal, Presence::Absent);
    let current = region
        .state
        .records()
        .find(|r| r.worker == worker.worker)
        .ok_or(NOT_FOUND)?;
    assert_eq!(current.status, HistoryStatus::Suspended);
    assert_eq!(current.segment, record.segment);
    assert_eq!(current.qualifying_count, 0);
    assert_eq!(current.quality.get(), 0);
    assert_eq!(current.last_observed, Presence::Absent);
    assert_eq!(current.last_applied, Presence::Present(frozen.epoch));
    assert!(current.coverage == Presence::Present(Score::new(1_000_000)?));
    assert_eq!(
        region
            .state
            .completed()
            .last()
            .ok_or(NOT_FOUND)?
            .observed_workers,
        0
    );
    for row in &j.rows {
        assert!(
            row.verdict.is_ok(),
            "{}: {:?}",
            row.operation.metadata().name,
            row.verdict
        );
        assert!(row
            .fuel
            .is_some_and(|fuel| fuel > 0 && fuel <= DEFAULT_CPU_FUEL));
    }
    Ok(())
}

#[allow(
    dead_code,
    reason = "The shared retry proof is consumed by the participant retry suite."
)]
pub(crate) fn resumable_participant_retry_proof() -> Result<(), Box<dyn std::error::Error>> {
    fn resumable_participant_retry_inner() -> Checked {
        let mut j = maximum_journey_with_roster(guest_module()?, Some(16), 16, 4)?;
        let state = decode_shared_state(&j.bytes)?;
        let captured = epoch::captured_roster(&state)?.ok_or(NOT_FOUND)?;
        let binding = captured.binding;
        let progress = layerx_programs_ai_market::aggregation::progress(&j.bytes)?;
        let Presence::Present(input) = progress.input else {
            return Err(NOT_FOUND.into());
        };
        let before = j.bytes.clone();
        for (operation, payload) in [
            (dispatch::FinalizeAggregation, input.bytes().to_vec()),
            (dispatch::OPEN_EPOCH, vec![]),
        ] {
            let call = bound(operation, principal(KEEPER)?, &binding, [0xed; 32], payload);
            let at = if operation == dispatch::OPEN_EPOCH {
                1410
            } else {
                1504
            };
            let response = j.send_step(&call, at)?;
            assert_eq!(
                u16::from_be_bytes(
                    response[response.len() - 2..]
                        .try_into()
                        .required_with("cursor is two bytes")
                ),
                16
            );
            assert_eq!(
                j.bytes, before,
                "completed exact retries must not mutate state"
            );
        }
        assert_eq!(
            j.rows
                .iter()
                .filter(|r| r.operation == dispatch::OPEN_EPOCH && r.height == 1408)
                .count(),
            2
        );
        assert_eq!(
            j.rows
                .iter()
                .filter(|r| r.operation == dispatch::FinalizeAggregation && r.height == 1504)
                .count(),
            3
        );
        for row in j.rows {
            assert!(row.verdict.is_ok());
            assert!(row.fuel.is_some_and(|f| f > 0 && f <= DEFAULT_CPU_FUEL));
        }
        Ok(())
    }

    resumable_participant_retry_inner().map_err(|e| Box::new(e) as Box<dyn std::error::Error>)
}

#[test]
fn current_room_profile_uses_the_full_wire_capacity() {
    f11_journeys::room_profile_wire_round_trip();
}

#[path = "repair_22_33_260.rs"]
mod f11_producers;

#[test]
fn f11_fuel_table() {
    f11_budget::run();
}

mod f11_budget {
    // OpenEpoch and FinalizeAggregation publish their participant tables when
    // their returned cursors reach the count. Producers complete both through
    // dispatch before using the captured roster.
    // All selectors use 16 rooms, 64 asks and 128 bids at maximum payload;
    // SettleLaps uses 64 laps and 32 peers. SplitPayout uses a separate deal
    // settled through RecordDeal and SettleLaps: 64 laps and ten peer credits,
    // plus the host and protocol credits, in one room with ten bids and one ask.
    use super::{f11_producers as producers, run_guest, Expected, ValidatedModule};
    use ed25519_dalek::{Signer, SigningKey};
    use layerx_ai_market_test_values::RequiredValue as _;
    use layerx_programs_ai_market::{
        codec::{self, Envelope, ResultStatus},
        dispatch::{self, Operation, Routed},
        epoch,
        errors::*,
        evidence::*,
        laps::*,
        policy::{PolicyCommitments, RoomProfile, TaskPolicyV1, TASK_POLICY_BYTES},
        queries,
        registry::derive_rewards_account,
        registry_ops::{CallContext, PolicySection},
        rooms::*,
        roster,
        state::{decode_shared_state, Section},
        types::*,
        workers::{WorkerState, WorkerTable},
        MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
        MERGED_CHAIN_DOMAIN,
    };
    use std::sync::Arc;
    use std::{
        cell::Cell,
        collections::BTreeSet,
        io::{BufRead, BufReader, Write},
        path::PathBuf,
        process::{Child, ChildStdin, ChildStdout, Command, Stdio},
    };
    const ORIGIN: u64 = 128;
    const EPOCH: u64 = 10;
    const START: u64 = ORIGIN + EPOCH * 128;
    const AT: u64 = START + 64;
    thread_local! { static IDENTITIES: Cell<[[u8; 32]; 4]> = const { Cell::new([[0; 32]; 4]) }; }
    fn owner() -> PrincipalId {
        actor(12)
    }
    fn actor(n: u8) -> PrincipalId {
        PrincipalId::new(IDENTITIES.with(|ids| match n {
            12 | 121 | 127 => ids.get()[1],
            32 => ids.get()[2],
            33 | 30 | 120 => ids.get()[3],
            _ => [n; 32],
        }))
        .required()
    }
    fn digest(n: u8) -> Digest32 {
        Digest32::new([n; 32]).required()
    }
    fn encoded(size: usize, f: impl FnOnce(&mut [u8]) -> CodecResult<usize>) -> Vec<u8> {
        let mut out = vec![0; size];
        let n = f(&mut out).required();
        out.truncate(n);
        out
    }
    fn open_epoch(bytes: &mut Vec<u8>, at: u64) {
        producers::refresh_workers_with(bytes, at, |w| {
            let n = (32..64).find(|n| actor(*n) == w.owner).required();
            SigningKey::from_bytes(&[0x90 + n; 32])
        })
        .required();
        let frozen = producers::open(bytes, at).required();
        let captured = epoch::captured_roster(&decode_shared_state(bytes).required())
            .required()
            .required();
        assert_eq!(captured.binding.epoch, frozen.epoch);
        assert_eq!(captured.binding.roster, frozen.roster);
    }
    fn hex(bytes: &[u8]) -> String {
        bytes.iter().fold(String::new(), |mut text, b| {
            use core::fmt::Write as _;
            write!(text, "{b:02x}").unwrap_or_else(|error| panic!("hex formatting: {error}"));
            text
        })
    }
    fn unhex(s: &str) -> Vec<u8> {
        if s == "-" {
            return Vec::new();
        }
        assert!(s.len().is_multiple_of(2));
        (0..s.len())
            .step_by(2)
            .map(|i| u8::from_str_radix(&s[i..i + 2], 16).required())
            .collect()
    }
    struct Native {
        module: Arc<ValidatedModule>,
        child: Child,
        input: Option<ChildStdin>,
        output: BufReader<ChildStdout>,
    }
    impl Drop for Native {
        fn drop(&mut self) {
            if self.input.is_some() {
                let _ = self.child.kill();
                let _ = self.child.wait();
            }
        }
    }
    impl Native {
        fn start() -> Self {
            let path = std::env::var_os("PAXAI_HOST_BOUNDARY_MANIFEST").map_or_else(
                || {
                    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
                        .join("../../../build/paxai-host-boundary/manifest")
                },
                PathBuf::from,
            );
            let manifest = std::fs::read_to_string(&path).required();
            let lines: Vec<_> = manifest.lines().collect();
            let [wasm, binary, chain] = lines[..] else {
                panic!("invalid native manifest");
            };
            assert_eq!(unhex(chain), MERGED_CHAIN_DOMAIN);
            let resolve = |name: &str| {
                let p = PathBuf::from(name);
                if p.is_absolute() {
                    p
                } else {
                    path.parent().required().join(p)
                }
            };
            let wasm_bytes = std::fs::read(resolve(wasm)).required();
            assert!(
                wasm_bytes.len() + super::guest::DEPLOYMENT_ENVELOPE_BYTES
                    <= super::guest::DEPLOYMENT_LIMIT
            );
            let module = Arc::new(
                layerx_programs_runtime::WasmEngine::declared()
                    .required()
                    .validate_versioned_metered(
                        layerx_programs_runtime::ABI_V5_VERSION,
                        &wasm_bytes,
                        layerx_programs_runtime::FuelSchedule::WASMI_0_31_2,
                    )
                    .required(),
            );
            let mut child = Command::new(resolve(binary))
                .arg(resolve(wasm))
                .stdin(Stdio::piped())
                .stdout(Stdio::piped())
                .stderr(Stdio::inherit())
                .spawn()
                .required();
            let input = child.stdin.take().required();
            let output = BufReader::new(child.stdout.take().required());
            let mut native = Self {
                module,
                child,
                input: Some(input),
                output,
            };
            let ready = native.line();
            let tokens: Vec<_> = ready.split_whitespace().collect();
            assert_eq!(tokens.len(), 9, "native bring-up: {ready}");
            assert_eq!(tokens[0], "ready");
            IDENTITIES.with(|ids| {
                ids.set(std::array::from_fn(|i| {
                    unhex(tokens[i + 1]).try_into().required()
                }));
            });
            native
        }
        fn line(&mut self) -> String {
            let mut s = String::new();
            assert!(
                self.output.read_line(&mut s).required() > 0,
                "native harness closed stdout"
            );
            s
        }
        fn check(
            &mut self,
            before: &[u8],
            envelope: &[u8],
            who: PrincipalId,
            at: u64,
            expected: &producers::Reply,
        ) {
            let op = u16::from_be_bytes(envelope[8..10].try_into().required());
            let header = producers::header(before).required();
            let operation = Operation::decode(op).required();
            let mut topic = [0; 64];
            let topic_len = if expected.event.is_empty() {
                0
            } else {
                codec::event_topic(operation, &mut topic).required()
            };
            let (in_process_fuel, verdict) = run_guest(
                &self.module,
                &CallContext {
                    chain: header.deployment_chain_domain,
                    program: header.program_id,
                    principal: who,
                    height: at,
                },
                Some(before),
                envelope,
                &Expected {
                    state: if expected.state.is_empty() {
                        before
                    } else {
                        &expected.state
                    },
                    topic: &topic[..topic_len],
                    event: &expected.event,
                    result: &expected.result,
                },
            )
            .required();
            println!(
                "F11 {op:#06x} state={} in_process_fuel={in_process_fuel:?} verdict={verdict:?}",
                before.len()
            );
            let actor_index = (0..3)
                .find(|i| IDENTITIES.with(|ids| ids.get()[i + 1] == who.bytes()))
                .required();
            let refund_amount = if operation == dispatch::RefundBond {
                let ledger = |bytes: &[u8]| {
                    let shared = decode_shared_state(bytes).required();
                    layerx_programs_ai_market::rewards::decode_reward_state(
                        &shared.room_market().required().0
                            [..layerx_programs_ai_market::rewards::REWARD_STATE_BYTES],
                    )
                    .required()
                    .ledger()
                    .required()
                };
                ledger(&expected.state)
                    .total_claimed
                    .checked_sub(ledger(before).total_claimed)
                    .required()
            } else {
                0
            };
            writeln!(
                self.input.as_mut().required(),
                "f11 {actor_index} {at} {} {} {}",
                hex(&refund_amount.to_be_bytes()),
                hex(before),
                hex(envelope)
            )
            .required();
            self.input.as_mut().required().flush().required();
            let line = self.line();
            let t: Vec<_> = line.split_whitespace().collect();
            assert_eq!(t[0], "call");
            assert_eq!(t[1], "0", "selector {op:#06x}: {line}");
            assert_eq!(t[2], "0", "selector {op:#06x}: {line}");
            let usage = Self::verify_native_receipt(
                &t,
                &header,
                before,
                expected,
                who,
                &topic[..topic_len],
            );
            let metrics = self.line();
            let m: Vec<_> = metrics.split_whitespace().collect();
            assert_eq!(m.len(), 4);
            assert_eq!(m[0], "f11");
            let fuel: u64 = m[1].parse().required();
            let boundaries: u32 = m[2].parse().required();
            let bytes: usize = m[3].parse().required();
            assert_eq!(verdict, Ok(()), "in-process selector {op:#06x}");
            let in_process_fuel = in_process_fuel.required_with("measured in-process F11 fuel");
            assert!(in_process_fuel > 0 && in_process_fuel < 1_000_000);
            assert_eq!(fuel, usage.cpu_fuel);
            assert!(fuel > 0 && fuel < 1_000_000);
            assert_eq!(boundaries, 0, "plain F11 call has no replay record");
            assert_eq!(bytes, 0, "plain F11 call has no replay witness");
            assert_eq!(bytes, t[12].parse::<usize>().required());
            println!(
                "F11 {op:#06x} state={} fuel={fuel} in_process_fuel={in_process_fuel} boundaries={boundaries} replay_bytes={bytes}",
                before.len()
            );
        }
        fn verify_native_receipt(
            t: &[&str],
            header: &layerx_programs_ai_market::registry::MarketHeader,
            before: &[u8],
            expected: &producers::Reply,
            who: PrincipalId,
            topic: &[u8],
        ) -> layerx_programs_runtime::meter::MeteredUsage {
            use layerx_programs_runtime::terminal::{
                CandidateTerminalOutcome, ExecutionTerminal, TerminalDetail,
            };
            let effects = layerx_programs_runtime::abi::AbiEffects::decode_program_event_envelope(
                &unhex(t[17]),
            )
            .required();
            assert_eq!(
                effects.events.len(),
                usize::from(!expected.event.is_empty())
            );
            if let Some(event) = effects.events.first() {
                assert_eq!(event.program.bytes(), header.program_id.bytes());
                assert_eq!(event.principal.bytes(), who.bytes());
                assert_eq!(event.topic, topic);
                assert_eq!(event.data, expected.event);
            }
            let terminal = unhex(t[5]);
            let detail = layerx_wire::receipt::decode_applied_terminal(&terminal)
                .required()
                .0;
            let decoded = layerx_programs_runtime::terminal::decode_terminal_payload(
                t[3].parse().required(),
                t[4].parse().required(),
                detail,
            )
            .required();
            let TerminalDetail::Execution(ExecutionTerminal::CandidateV4 {
                program,
                outcome,
                usage,
                ..
            }) = decoded.detail
            else {
                panic!("missing execution terminal");
            };
            assert_eq!(program, IDENTITIES.with(|ids| ids.get()[0]));
            assert_eq!(
                outcome,
                CandidateTerminalOutcome::Success {
                    code: 0,
                    response: expected.result.clone()
                }
            );
            assert_eq!(
                unhex(t[6]),
                if expected.state.is_empty() {
                    before
                } else {
                    &expected.state
                }
            );
            usage
        }

        fn check_whole_record_refusal(
            &mut self,
            before: &[u8],
            envelope: &[u8],
            who: PrincipalId,
            at: u64,
        ) {
            let actor_index = (0..3)
                .find(|i| IDENTITIES.with(|ids| ids.get()[i + 1] == who.bytes()))
                .required();
            writeln!(
                self.input.as_mut().required(),
                "f11-record {actor_index} {at} {} {} {}",
                hex(&0u128.to_be_bytes()),
                hex(before),
                hex(envelope)
            )
            .required();
            self.input.as_mut().required().flush().required();
            let line = self.line();
            let t: Vec<_> = line.split_whitespace().collect();
            assert_eq!(t[0], "call");
            assert_eq!(t[1], "-3", "record bounds refusal: {line}");
            assert_eq!(t[2], "0", "record bounds receipt: {line}");
            assert_eq!(unhex(t[6]), before, "record refusal must retain state");
            assert_eq!(t[12], "0", "record refusal has no witness");
            let metrics = self.line();
            let m: Vec<_> = metrics.split_whitespace().collect();
            assert_eq!(m[0], "f11-refused");
            assert_eq!(m[1], "0");
            let fee: u64 = m[2].parse().required();
            let fuel: u64 = m[3].parse().required();
            assert_eq!(fee, t[15].parse::<u64>().required());
            assert_eq!(fee, 0, "a preexecution bounds refusal charges no fee");
            assert_eq!(fuel, 0, "the refused call has no program outcome");
            println!(
                "F11 whole record refused state={} receipt_fee={fee} receipt_fuel={fuel}",
                before.len()
            );
        }
        fn finish(mut self) {
            drop(self.input.take());
            assert!(self.child.wait().required().success());
        }
    }
    struct Market {
        bytes: Vec<u8>,
        roster: RosterDigest,
        native: Native,
        measuring: bool,
        measured: BTreeSet<u16>,
    }
    impl Market {
        fn new() -> Self {
            let native = Native::start();
            let mut bytes = Self::create_budget_state();
            let roster = Self::activate_budget_roster(&mut bytes);
            Self {
                bytes,
                roster,
                native,
                measuring: false,
                measured: BTreeSet::new(),
            }
        }
        fn create_budget_state() -> Vec<u8> {
            let chain = ChainDomain::new(MERGED_CHAIN_DOMAIN).required();
            let program = ProgramId::new(IDENTITIES.with(|ids| ids.get()[0])).required();
            let mut asset_bytes = [0; 32];
            asset_bytes[0] = 9;
            let asset = AssetId::new(asset_bytes).required();
            let policy = TaskPolicyV1::bounded_default(
                1,
                1,
                PolicyCommitments {
                    model_artifact: digest(1),
                    dataset_artifact: [2; 32],
                    benchmark_suite: digest(3),
                    rubric: RubricDigest::new([4; 32]).required(),
                    task_schema: digest(5),
                    result_schema: digest(6),
                    service_terms: digest(7),
                },
                1000,
                1,
            )
            .required();
            let mut payload = owner().bytes().to_vec();
            payload.extend_from_slice(asset.as_bytes());
            payload.extend_from_slice(derive_rewards_account(program, asset).required().as_bytes());
            payload.extend_from_slice(&[14; 32]);
            payload.push(1);
            payload.extend_from_slice(actor(15).as_bytes());
            payload.extend_from_slice(&encoded(TASK_POLICY_BYTES, |out| policy.encode(out)));
            payload.extend_from_slice(&[16; 32]);
            let envelope = encoded(MAX_ENVELOPE_BYTES, |out| {
                codec::encode_envelope(
                    &Envelope {
                        operation: dispatch::CREATE,
                        chain,
                        program,
                        market: codec::derive_market(chain, program)?,
                        actor: owner(),
                        epoch: 0,
                        config: 1,
                        roster: Presence::Absent,
                        sequence: 1,
                        expiry: u64::MAX,
                        request: RequestId::new([1; 32])?,
                        payload: &payload,
                        authentication: Authentication::Native,
                    },
                    out,
                )
            });
            let mut bytes = vec![0; MAX_STATE_BYTES];
            let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
            let mut event = vec![0; MAX_EVENT_BYTES];
            let mut result = vec![0; MAX_RESULT_BYTES];
            let routed = dispatch::route(
                &CallContext {
                    chain,
                    program,
                    principal: owner(),
                    height: ORIGIN,
                },
                &envelope,
                None,
                dispatch::Buffers {
                    next: &mut bytes,
                    scratch: &mut scratch,
                    event: &mut event,
                    result: &mut result,
                },
            )
            .required();
            let Routed::Applied { state_len, .. } = routed else {
                panic!("Create refused");
            };
            bytes.truncate(state_len);
            bytes
        }
        fn activate_budget_roster(bytes: &mut Vec<u8>) -> RosterDigest {
            let mut sequence = 2;
            producers::enroll(
                bytes,
                &mut sequence,
                actor(32),
                32,
                &SigningKey::from_bytes(&[0x90 + 32; 32]),
                MetadataDigest::new([32; 32]).required(),
                ORIGIN + 2,
            )
            .required();
            for n in [2, 3, 4] {
                producers::evaluator(
                    bytes,
                    &mut sequence,
                    actor(n),
                    n,
                    &SigningKey::from_bytes(&[n; 32]),
                    ORIGIN + 2,
                )
                .required();
            }
            let mut fund = 1_000_000u128.to_be_bytes().to_vec();
            fund.extend_from_slice(&[14; 32]);
            fund.extend_from_slice(&1u64.to_be_bytes());
            fund.push(1);
            producers::call(bytes, dispatch::FUND, owner(), &fund, ORIGIN + 7).required();
            let pending: Vec<_> = (33u8..64).collect();
            for epoch in 1..EPOCH {
                let start = ORIGIN + epoch * 128;
                open_epoch(bytes, start);
                producers::heartbeat(bytes, start + 1).required();
                for &n in pending
                    .iter()
                    .skip(usize::try_from((epoch - 1) * 4).required())
                    .take(4)
                {
                    producers::enroll(
                        bytes,
                        &mut sequence,
                        actor(n),
                        n,
                        &SigningKey::from_bytes(&[0x90 + n; 32]),
                        MetadataDigest::new([n; 32]).required(),
                        start + 2,
                    )
                    .required();
                }
                producers::settle(bytes, start + 96).required();
            }
            let mut schedule = decode_shared_state(bytes)
                .required()
                .revision
                .to_be_bytes()
                .to_vec();
            schedule.extend_from_slice(&EPOCH.to_be_bytes());
            producers::call(
                bytes,
                dispatch::SCHEDULE_ACTIVATION,
                owner(),
                &schedule,
                START - 32,
            )
            .required();
            open_epoch(bytes, START);
            let mut advance = decode_shared_state(bytes)
                .required()
                .revision
                .to_be_bytes()
                .to_vec();
            advance.extend_from_slice(&EPOCH.to_be_bytes());
            producers::call(
                bytes,
                dispatch::ADVANCE_ACTIVATION,
                owner(),
                &advance,
                START + 1,
            )
            .required();
            let roster = RosterDigest::new(
                roster::roster_digest(&producers::table(bytes).required(), EPOCH)
                    .required()
                    .bytes(),
            )
            .required();
            assert_eq!(
                epoch::captured_roster(&decode_shared_state(bytes).required())
                    .required()
                    .required()
                    .workers()
                    .len(),
                32
            );
            roster
        }

        fn envelope(&self, op: Operation, who: PrincipalId, payload: &[u8]) -> Vec<u8> {
            let sequence = if op.metadata().sequence == dispatch::SequencePolicy::ObjectLocal {
                0
            } else {
                producers::next_sequence(&self.bytes, who).required()
            };
            let bytes = producers::envelope(&self.bytes, op, who, payload, sequence).required();
            if op.metadata().feature != 11
                || matches!(op, dispatch::ChallengeLap | dispatch::ResolveChallenge)
            {
                return bytes;
            }
            let mut e = codec::decode_envelope(&bytes).required().envelope;
            e.roster = Presence::Present(self.roster);
            encoded(MAX_ENVELOPE_BYTES, |out| codec::encode_envelope(&e, out))
        }
        fn call(&mut self, op: Operation, who: PrincipalId, payload: &[u8], at: u64) -> Vec<u8> {
            let e = self.envelope(op, who, payload);
            let expected = producers::compose(&self.bytes, &e, who, at).required();
            assert!(
                matches!(expected.routed, Routed::Applied { .. }),
                "{op:?}: {:?}",
                codec::decode_result(&expected.result).required()
            );
            if self.measuring && op.metadata().feature == 11 {
                if op != dispatch::SettleLaps {
                    assert_eq!(payload.len(), op.metadata().payload_max);
                }
                self.native.check(&self.bytes, &e, who, at, &expected);
                self.measured.insert(op.selector());
            }
            self.bytes = expected.state;
            e
        }
        fn manifest(&self, batch: &ReceiptBatch) -> Vec<u8> {
            let state = decode_shared_state(&self.bytes).required();
            let section = PolicySection::decode(state.section(Section::PolicyLifecycle).required())
                .required();
            let binding = epoch::captured_roster(&state).required().required().binding;
            let mut parents: Vec<_> = (0..MAX_PARENTS - 1)
                .map(|n| ParentRef {
                    purpose: ParentPurpose::Execution,
                    root: digest(150 + u8::try_from(n).required()).bytes(),
                })
                .collect();
            parents.push(ParentRef {
                purpose: ParentPurpose::Execution,
                root: batch.summary_digest().required().bytes(),
            });
            parents.sort_by_key(|p| (p.purpose as u8, p.root));
            encoded(MAX_MANIFEST_BYTES, |out| {
                encode_manifest(
                    &ArtifactManifest {
                        kind: ArtifactKind::EvaluationEvidence,
                        privacy: Privacy::Public,
                        context: ArtifactContext {
                            chain: binding.chain,
                            program: binding.program,
                            market: binding.market,
                            policy: section.current.digest().required(),
                        },
                        epoch: binding.epoch,
                        publisher: owner(),
                        subject: producers::sealed_task_set(&self.bytes, EPOCH)
                            .required()
                            .bytes(),
                        byte_length: 1,
                        chunk_count: 1,
                        content_root: content_root(1, 1, &batch.batch_digest.bytes()),
                        parents: Items::Typed(&parents),
                        declaration_root: [0; 32],
                        reproduction_root: [0; 32],
                        access_policy_root: [0; 32],
                        not_after_height: u64::MAX,
                    },
                    out,
                )
                .map_err(|_| NON_CANONICAL)
            })
        }
        fn seal(&mut self, manifest: &[u8], voter: u8) {
            let state = decode_shared_state(&self.bytes).required();
            let policy = PolicySection::decode(state.section(Section::PolicyLifecycle).required())
                .required()
                .current;
            let mut payload = 1u16.to_be_bytes().to_vec();
            payload.extend_from_slice(&manifest_root(manifest).required().bytes());
            payload.extend_from_slice(policy.digest().required().as_bytes());
            payload.extend_from_slice(
                producers::sealed_task_set(&self.bytes, EPOCH)
                    .required()
                    .as_bytes(),
            );
            payload.extend_from_slice(policy.commitments.rubric.as_bytes());
            payload.push(policy.assessment_mode);
            self.call(dispatch::SealEvidence, actor(voter), &payload, AT);
        }
        fn settle_payload(batch: ReceiptBatch, manifest: &[u8]) -> Vec<u8> {
            encoded(5000, |out| {
                SettleLapsRequest {
                    batch,
                    artifact_manifest: manifest,
                }
                .encode(out)
            })
        }
    }
    fn batch(first: u64, count: usize, peers: usize, bitmap: u32) -> ReceiptBatch {
        let ranges: Vec<_> = (0..peers)
            .map(|i| digest(80 + u8::try_from(i).required()))
            .collect();
        let key = SigningKey::from_bytes(&[12; 32]);
        let receipts: Vec<_> = (0..count)
            .map(|i| {
                let mut receipt = LapReceipt {
                    deal_id: digest(62),
                    lap_index: first + i as u64,
                    prompt_tokens: 10,
                    generated_tokens: 90,
                    cumulative_tokens: (first + i as u64 + 1) * 100,
                    layer_range_digests: &ranges,
                    host_signature: Signature64([1; 64]),
                    peer_countersignatures: bitmap,
                };
                receipt.host_signature = Signature64(
                    key.sign(receipt.signing_digest().required().as_bytes())
                        .to_bytes(),
                );
                receipt
            })
            .collect();
        ReceiptBatch::from_receipts(&receipts).required()
    }

    use layerx_programs_ai_market::{
        bonds::{LockBondRequest, RefundBondRequest, SlashBondRequest},
        canary::{
            self, CanaryReveal, CanaryRoomProfile, CanaryTask, ChallengeLapPayload,
            ResolveChallengePayload,
        },
        commit_reveal,
        rewards::SplitPayoutRequest,
    };

    impl Market {
        fn peer(&self, n: u8) -> WorkerRosterEntry {
            let state = decode_shared_state(&self.bytes).required();
            *epoch::captured_roster(&state)
                .required()
                .required()
                .workers()
                .iter()
                .find(|w| w.owner == actor(n))
                .required()
        }
        fn score_envelope(
            &self,
            operation: dispatch::Operation,
            payload: &[u8],
            binding: FrozenBinding,
        ) -> Vec<u8> {
            let sequence = producers::next_sequence(&self.bytes, owner()).required();
            let mut request = [0x77; 32];
            request[..8].copy_from_slice(&sequence.to_be_bytes());
            request[8..10].copy_from_slice(&operation.selector().to_be_bytes());
            let e = Envelope {
                operation,
                chain: binding.chain,
                program: binding.program,
                market: binding.market,
                actor: owner(),
                epoch: binding.epoch,
                config: binding.config.get(),
                roster: Presence::Present(binding.roster),
                sequence,
                expiry: u64::MAX,
                request: RequestId::new(request).required(),
                payload,
                authentication: Authentication::Native,
            };
            encoded(MAX_ENVELOPE_BYTES, |out| {
                commit_reveal::encode_canary_envelope(&e, out)
            })
        }
        fn score(
            &mut self,
            operation: dispatch::Operation,
            payload: &[u8],
            binding: FrozenBinding,
            height: u64,
        ) {
            let envelope = self.score_envelope(operation, payload, binding);
            let reply = producers::compose(&self.bytes, &envelope, owner(), height).required();
            assert!(
                matches!(reply.routed, Routed::Applied { .. }),
                "{:?}",
                codec::decode_result(&reply.result).required()
            );
            self.bytes = reply.state;
        }
        fn canary(&mut self, lap: u64) -> (CanaryTask, Digest32) {
            let binding = epoch::captured_roster(&decode_shared_state(&self.bytes).required())
                .required()
                .required()
                .binding;
            let room = RoomBook::decode(
                decode_shared_state(&self.bytes)
                    .required()
                    .room_market()
                    .required()
                    .1,
            )
            .required()
            .room(digest(60))
            .required()
            .required();
            let answer = [200_000; 8];
            let mut task = CanaryTask {
                deal: digest(62),
                lap,
                peer: self.peer(32).worker,
                prompt_digest: digest(90),
                expected_logits_digest: canary::logits_digest(&[100_000; 8]).required(),
                answer_digest: canary::logits_digest(&answer).required(),
                profile: CanaryRoomProfile { room: room.profile },
                peer_signature: Signature64([0; 64]),
            };
            task.peer_signature = Signature64(
                SigningKey::from_bytes(&[0x90 + 32; 32])
                    .sign(task.answer_attestation(&binding).required().as_bytes())
                    .to_bytes(),
            );
            let id = task.commitment(&binding, owner()).required();
            let payload = task.encode().required();
            self.score(dispatch::CommitScore, &payload, binding, AT);
            self.call(
                dispatch::ChallengeLap,
                actor(120),
                &ChallengeLapPayload {
                    deal: task.deal,
                    lap,
                    commitment: id,
                    deposit: room.profile.minimum_bond,
                }
                .encode()
                .required(),
                AT + 1,
            );
            (task, id)
        }
        fn resolve_fault(&mut self, task: &CanaryTask, id: Digest32) {
            let binding = epoch::captured_roster(&decode_shared_state(&self.bytes).required())
                .required()
                .required()
                .binding;
            let payload = CanaryReveal {
                commitment: id,
                prompt_digest: task.prompt_digest,
                logits: [100_000; 8],
            }
            .encode()
            .required();
            self.score(dispatch::RevealScore, &payload, binding, START + 80);
            self.call(
                dispatch::ResolveChallenge,
                actor(121),
                &ResolveChallengePayload {
                    deal: task.deal,
                    lap: task.lap,
                    commitment: id,
                    answer: [200_000; 8],
                }
                .encode()
                .required(),
                START + 81,
            );
            assert!(canary::commitment_from_state(&self.bytes, id)
                .required()
                .peer_at_fault());
        }
    }

    fn room_profile() -> RoomProfile {
        RoomProfile {
            minimum_bond: 100,
            canary_ratio_bps: 250,
            lap_size_tokens: 100,
            challenge_window_blocks: 16,
            slash_fraction_bps: 1000,
            host_share_bps: 1000,
            protocol_share_bps: 1000,
            settlement_account: AccountId::new([80; 32]).required(),
            max_generated_tokens: 8192,
            queue_depth: layerx_programs_ai_market::policy::DEFAULT_QUEUE_DEPTH,
            liveness_timeout_ms: 30_000,
            acknowledgement_window_ms: 5000,
            canary_relative_tolerance_bps: 100,
        }
    }
    impl Market {
        fn add_room(&mut self, i: usize) {
            self.call(
                dispatch::OpenRoom,
                owner(),
                &encoded(OPEN_ROOM_REQUEST_BYTES, |out| {
                    OpenRoomRequest {
                        room_id: if i == 0 {
                            digest(60)
                        } else {
                            digest(99 + u8::try_from(i).required())
                        },
                        model_digest: if i == 1 { digest(2) } else { digest(1) },
                        runtime_digest: digest(51),
                        profile: room_profile(),
                    }
                    .encode(out)
                }),
                START + 2,
            );
        }
        fn add_ask(&mut self, i: usize) {
            self.call(
                dispatch::PostAsk,
                actor(30),
                &encoded(128, |out| {
                    PostAskRequest {
                        room_id: digest(60),
                        ask_id: digest(1 + u8::try_from(i).required()),
                        model_digest: digest(1),
                        maximum_price_per_thousand_tokens: 2000,
                        stream_id: 1 + i as u128,
                    }
                    .encode(out)
                }),
                START + 2,
            );
        }
        fn add_bid(&mut self, i: usize) {
            let n = if i < 32 {
                32 + u8::try_from(i).required()
            } else {
                32
            };
            let who = actor(n);
            let room = if i < 64 { digest(60) } else { digest(100) };
            let bid = digest(1 + u8::try_from(i).required());
            self.call(
                dispatch::LockBond,
                who,
                &LockBondRequest {
                    room,
                    bid,
                    amount: 100,
                }
                .encode()
                .required(),
                START + 2,
            );
            let state = decode_shared_state(&self.bytes).required();
            let workers = WorkerTable::decode(
                layerx_programs_ai_market::evaluators::authority::split_identity_section(
                    state.section(Section::IdentityRoster).required(),
                )
                .required()
                .0,
            )
            .required();
            let worker = workers.iter().find(|w| w.owner == who).required();
            assert!(matches!(
                worker.state,
                WorkerState::Available | WorkerState::Enrolled
            ));
            let payload = encoded(130, |out| {
                PostBidRequest {
                    room_id: room,
                    bid_id: bid,
                    capability_digest: Digest32::new(worker.metadata.bytes())?,
                    minimum_price_per_thousand_tokens: 5,
                    bond_amount: 100,
                    layer_capacity: 16,
                }
                .encode(out)
            });
            self.call(dispatch::PostBid, who, &payload, START + 2);
        }
        fn full_book(&mut self) {
            for i in 0..ROOM_CAPACITY - 1 {
                self.add_room(i);
            }
            for i in 0..ASK_CAPACITY - 1 {
                self.add_ask(i);
            }
            for i in 0..BID_CAPACITY - 1 {
                self.add_bid(i);
            }
            let base = self.bytes.clone();
            self.add_ask(ASK_CAPACITY - 1);
            self.add_bid(BID_CAPACITY - 1);
            self.measuring = true;
            self.add_room(ROOM_CAPACITY - 1);
            self.measuring = false;
            self.bytes = base.clone();
            self.add_room(ROOM_CAPACITY - 1);
            self.add_bid(BID_CAPACITY - 1);
            self.measuring = true;
            self.add_ask(ASK_CAPACITY - 1);
            self.measuring = false;
            self.bytes = base;
            self.add_room(ROOM_CAPACITY - 1);
            self.add_ask(ASK_CAPACITY - 1);
            self.measuring = true;
            self.add_bid(BID_CAPACITY - 1);
            assert_eq!(
                RoomBook::decode(
                    decode_shared_state(&self.bytes)
                        .required()
                        .room_market()
                        .required()
                        .1
                )
                .required()
                .counts(),
                [16, 64, 128, 0]
            );
            assert!(self.bytes.len() <= MAX_STATE_BYTES);
        }
        fn read(&mut self, op: Operation, payload: &[u8], at: u64) {
            assert_eq!(payload.len(), op.metadata().payload_max);
            let h = producers::header(&self.bytes).required();
            let envelope = Envelope {
                operation: op,
                chain: h.deployment_chain_domain,
                program: h.program_id,
                market: h.market_id,
                actor: owner(),
                epoch: 0,
                config: 0,
                roster: Presence::Absent,
                sequence: 0,
                expiry: u64::MAX,
                request: RequestId::new([99; 32]).required(),
                payload,
                authentication: Authentication::Native,
            };
            let e = encoded(MAX_ENVELOPE_BYTES, |out| {
                if op == dispatch::BookView {
                    layerx_programs_ai_market::placement::encode_book_envelope(&envelope, out)
                } else {
                    queries::encode_view_envelope(&envelope, out)
                }
            });
            let expected = producers::compose(&self.bytes, &e, owner(), at).required();
            assert!(matches!(expected.routed, Routed::Unchanged { .. }));
            let result = codec::decode_result(&expected.result).required();
            assert_eq!(result.status, ResultStatus::Ok);
            assert_eq!(result.error, None);
            self.native.check(&self.bytes, &e, owner(), at, &expected);
            self.measured.insert(op.selector());
        }
    }
    pub(super) fn run() {
        let mut m = Market::new();
        let payout_base = m.bytes.clone();
        m.full_book();
        m.read(dispatch::RoomView, digest(60).as_bytes(), START + 2);
        m.read(dispatch::BookView, digest(1).as_bytes(), START + 2);
        let layers = std::array::from_fn(|i| {
            Some(LayerSplit {
                bid_id: digest(1 + u8::try_from(i).required()),
                first_layer: u16::try_from(i).required(),
                end_layer: u16::try_from(i + 1).required(),
            })
        });
        let deal = RecordDealRequest {
            room_id: digest(60),
            deal_id: digest(62),
            ask_id: digest(61),
            agreed_price_per_thousand_tokens: 1000,
            roster_digest: m.roster,
            layers,
        };
        let payload = encoded(1297, |out| deal.encode(out));
        assert_eq!(payload.len(), dispatch::RecordDeal.metadata().payload_max);
        m.call(dispatch::RecordDeal, owner(), &payload, START + 2);
        producers::seal_tasks(&mut m.bytes, AT).required();
        let b = batch(0, LAP_BATCH_MAX, MAX_DEAL_PEERS, u32::MAX);
        assert_eq!(b.last_lap_index, 63);
        let manifest = m.manifest(&b);
        assert_eq!(
            manifest.len(),
            MANIFEST_FIXED_BYTES + MAX_PARENTS * PARENT_BYTES
        );
        m.seal(&manifest, 2);
        let payload = Market::settle_payload(b, &manifest);
        assert_eq!(
            payload.len(),
            SETTLE_FIXED_BYTES + MANIFEST_FIXED_BYTES + MAX_PARENTS * PARENT_BYTES
        );
        m.call(dispatch::SettleLaps, owner(), &payload, AT);
        m.read(dispatch::DealView, digest(62).as_bytes(), AT);
        let mut batch_key = [0; 64];
        batch_key[..32].copy_from_slice(digest(62).as_bytes());
        batch_key[32..].copy_from_slice(b.batch_digest.as_bytes());
        m.read(dispatch::ReceiptBatchView, &batch_key, AT);
        m.read(dispatch::PeerView, m.peer(32).owner.as_bytes(), AT);
        let (task, id) = m.canary(0);
        m.resolve_fault(&task, id);
        m.call(
            dispatch::SlashBond,
            actor(120),
            &SlashBondRequest {
                deal: digest(62),
                bid: digest(1),
                lap: 0,
                evidence: id,
            }
            .encode()
            .required(),
            START + 82,
        );
        m.measuring = false;
        let settled = settlement_from_state(&m.bytes, digest(62))
            .required()
            .required();
        m.call(
            dispatch::SplitPayout,
            owner(),
            &SplitPayoutRequest {
                deal: digest(62),
                batch: settled.batch.batch_digest,
                amount: settled.settled_amount_owed,
            }
            .encode()
            .required(),
            START + 82,
        );
        m.measuring = true;
        m.call(
            dispatch::CloseRoom,
            owner(),
            digest(60).as_bytes(),
            START + 83,
        );
        m.call(
            dispatch::RefundBond,
            actor(32),
            &RefundBondRequest {
                room: digest(60),
                bid: digest(1),
            }
            .encode()
            .required(),
            START + 112,
        );
        measure_split_payout(&mut m, payout_base, &deal);
        let expected: BTreeSet<_> = (0x0b01..=0x0b0c).chain(0x0b10..=0x0b14).collect();
        assert_eq!(m.measured, expected);
        m.native.finish();
    }
    fn measure_split_payout(m: &mut Market, payout_base: Vec<u8>, deal: &RecordDealRequest) {
        const PAYOUT_PEERS: usize = 10;
        m.measuring = false;
        m.bytes = payout_base;
        m.add_room(0);
        m.add_ask(60);
        for i in 0..PAYOUT_PEERS {
            m.add_bid(i);
        }
        let payout_deal = RecordDealRequest {
            layers: std::array::from_fn(|i| {
                (i < PAYOUT_PEERS).then(|| LayerSplit {
                    bid_id: digest(1 + u8::try_from(i).required()),
                    first_layer: u16::try_from(i).required(),
                    end_layer: u16::try_from(i + 1).required(),
                })
            }),
            ..*deal
        };
        m.call(
            dispatch::RecordDeal,
            owner(),
            &encoded(1297, |out| payout_deal.encode(out)),
            START + 2,
        );
        producers::seal_tasks(&mut m.bytes, AT).required();
        let payout_batch = batch(0, LAP_BATCH_MAX, PAYOUT_PEERS, (1 << PAYOUT_PEERS) - 1);
        assert_eq!(
            payout_batch.last_lap_index - payout_batch.first_lap_index + 1,
            64
        );
        let manifest = m.manifest(&payout_batch);
        m.seal(&manifest, 2);
        m.call(
            dispatch::SettleLaps,
            owner(),
            &Market::settle_payload(payout_batch, &manifest),
            AT,
        );
        let state = decode_shared_state(&m.bytes).required();
        let book = RoomBook::decode(state.room_market().required().1).required();
        assert_eq!(book.counts(), [1, 1, 10, 1]);
        assert_eq!(
            book.deal(digest(62))
                .required()
                .required()
                .layers
                .iter()
                .flatten()
                .count(),
            PAYOUT_PEERS
        );
        m.measuring = true;
        let settled = settlement_from_state(&m.bytes, digest(62))
            .required()
            .required();
        m.call(
            dispatch::SplitPayout,
            owner(),
            &SplitPayoutRequest {
                deal: digest(62),
                batch: settled.batch.batch_digest,
                amount: settled.settled_amount_owed,
            }
            .encode()
            .required(),
            START + 82,
        );
        let state = decode_shared_state(&m.bytes).required();
        let rewards = layerx_programs_ai_market::rewards::decode_reward_state(
            &state.room_market().required().0
                [..layerx_programs_ai_market::rewards::REWARD_STATE_BYTES],
        )
        .required();
        let credited = (32..64)
            .filter(|&n| rewards.payout_balance(EPOCH, m.peer(n).worker).required() > 0)
            .count();
        assert_eq!(credited, PAYOUT_PEERS);
    }

    pub(super) fn whole_record_refusal() {
        let mut m = Market::new();
        for i in 0..ROOM_CAPACITY - 1 {
            m.add_room(i);
        }
        for i in 0..ASK_CAPACITY {
            m.add_ask(i);
        }
        for i in 0..BID_CAPACITY {
            m.add_bid(i);
        }
        let payload = encoded(OPEN_ROOM_REQUEST_BYTES, |out| {
            OpenRoomRequest {
                room_id: digest(99 + u8::try_from(ROOM_CAPACITY - 1).required()),
                model_digest: digest(1),
                runtime_digest: digest(51),
                profile: room_profile(),
            }
            .encode(out)
        });
        assert_eq!(payload.len(), dispatch::OpenRoom.metadata().payload_max);
        let envelope = m.envelope(dispatch::OpenRoom, owner(), &payload);
        let expected = producers::compose(&m.bytes, &envelope, owner(), START + 2).required();
        assert!(matches!(expected.routed, Routed::Applied { .. }));
        m.native
            .check_whole_record_refusal(&m.bytes, &envelope, owner(), START + 2);
        m.native.finish();
    }
}

#[test]
fn f11_whole_record_is_refused_on_the_full_state() {
    f11_budget::whole_record_refusal();
}
