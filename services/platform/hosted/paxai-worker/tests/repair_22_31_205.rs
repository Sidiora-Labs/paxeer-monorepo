//! Audit 22.30.84 repairs through the worker binary, pinned TLS and native finalized snapshots.
#![allow(dead_code, unused_imports)]

#[path = "../src/main.rs"]
mod worker;

use ed25519_dalek::{Signer, SigningKey};
use layerx_client::px::{PxNetworkHead, PxPaxeerHead};
use layerx_paxai_worker::{
    auth::{
        acknowledgment_digest, admit, decode_acknowledgment, decode_service, encode_service,
        sign_metadata, verify_signed_metadata, Admission, JobReference, MetadataContext,
        MetadataPublication, Route, ServiceContext, ServiceError, ServiceOperation, ServiceRequest,
    },
    discovery::{
        AuthorityEvidence, FinalizedAuthority, IdentityEvidence, Readiness, VerifiedMetadata,
        MAX_FINALITY_LAG,
    },
    jobs::{
        decode_result, evidence_digest, result_manifest_digest, AdmissionView, Input, JobService,
        Lease, Outcome, QueryAnswer, ResultLocator, ServiceResult, LEASE_GRACE_MS,
    },
    metadata::{Capability, Endpoint, Manifest},
    runner::{
        processing_ms, Bounds, Completion, Dispatch, EvidenceFault, ProcessRunner, Progress,
        Report, Role, Segment, Usage, TOKENS, VECTOR_ELEMENTS,
    },
    store::{
        JobKey, JobState, JobStore, NewJob, Usage as QueueUsage, MAX_ENCRYPTED_BYTES, MAX_QUEUED,
        MAX_QUEUED_BYTES, MAX_RUNNING, RETENTION_MS, TOMBSTONE_DOMAIN,
    },
};
use layerx_programs_ai_market::{
    admission::{
        admit_evaluator, Admission as Membership, AdmissionContext, AdmissionTable, ApprovalTerms,
        EvaluatorConsent, Participant, TABLE_MAX_BYTES,
    },
    codec::{
        self, decode_envelope, derive_evaluator, derive_market, derive_worker, encode_envelope,
        encode_roster, Envelope, Roster,
    },
    dispatch::{self, Operation},
    epoch::{self, Frozen, Outcome as Opening, ADVANCE_SCRATCH_BYTES, OPEN_SCRATCH_BYTES},
    errors::{ApplicationError, CodecResult, ARITHMETIC, NON_CANONICAL},
    evaluators::{
        authority::{split_identity_section, EvaluatorRecord, EvaluatorRegion, LastRequest},
        model::{EvaluatorGrant, GrantTerms},
    },
    evidence::{
        chunk_count, encode_manifest, manifest_root, object_content_root, ArtifactContext,
        ArtifactError, ArtifactKind, ArtifactManifest, Items, Privacy, MAX_MANIFEST_BYTES,
    },
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    queries::{
        read_state_chunk, CaptureFacts, FinalityEvidence, QueryError, ReadProof, StateCapture,
    },
    registry::{derive_rewards_account, MarketHeader, F01_SECTION_CAP},
    registry_ops::{self, CallContext, Outcome as Registered, PolicySection},
    rewards::{
        decode_reward_state, FundReplay, FundRequest, FundingAuthority, FundingPhase, RewardLedger,
        RewardState, FUNDING_POLICY_VERSION, REWARD_STATE_BYTES,
    },
    state::{
        decode_shared_state, encode_shared_state, ActorSlot, Control, ReplayRequest, ReplayTable,
        Section, SharedState,
    },
    tasks::{TaskBinding, TaskStatus},
    types::{
        AccountId, AssetId, Authentication, ChainDomain, Digest32, EvaluatorRosterEntry, MarketId,
        PolicyDigest, Presence, PrincipalId, ProgramId, PublicKey32, RequestDigest, RequestId,
        ResultDigest, RosterDigest, RubricDigest, StateDigest, TaskId, Version, WorkerId,
        WorkerRosterEntry,
    },
    workers::{WorkerCurrent, WorkerState, WorkerTable, WORKER_TABLE_MAX_BYTES},
    MAX_EVENT_BYTES, MAX_STATE_BYTES,
};
use std::fs;
use std::io;
use std::path::{Path, PathBuf};
use std::sync::Barrier;
use std::thread;
use std::time::Duration;

const CHAIN: [u8; 32] = [10; 32];
const PROGRAM: [u8; 32] = [11; 32];
const OWNER: [u8; 32] = [12; 32];
const ASSET: [u8; 32] = [13; 32];
const REFUND: [u8; 32] = [14; 32];
const ROOT: [u8; 32] = [0xA1; 32];
const KEEPER: u8 = 0x7f;
const ORIGIN: u64 = 1000;
const DELEGATE_SEED: [u8; 32] = [0xD1; 32];
const CUSTOMER_SEED: [u8; 32] = [0xC9; 32];
const VALID_FROM: u64 = 1140;
const EXPIRY: u64 = 1180;
const HEIGHT: u64 = 1150;
const TASK_EXPIRY: u64 = 1172;
const CHUNK_RESPONSE_MAX: usize = 8_244;

const DEFAULT_URI: &str = "https://localhost/paxai/v1";
const NOW: u64 = 1_700_000_000_000;
const PAYLOAD_BYTES: usize = 1_024;
const RUNNER_ENV: &str = "PAXAI_RUNNER";

enum Failure {
    Application(ApplicationError),
    Service(ServiceError),
    Artifact(ArtifactError),
    Query(QueryError),
    Io(io::ErrorKind),
    Unexpected(&'static str),
}
impl std::fmt::Debug for Failure {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Application(error) => write!(f, "application refusal {error:?}"),
            Self::Service(error) => write!(f, "service refusal {error:?}"),
            Self::Artifact(error) => write!(f, "artifact refusal {error:?}"),
            Self::Query(error) => write!(f, "query refusal {error:?}"),
            Self::Io(kind) => write!(f, "io failure {kind:?}"),
            Self::Unexpected(what) => write!(f, "unexpected {what}"),
        }
    }
}
impl From<ApplicationError> for Failure {
    fn from(error: ApplicationError) -> Self {
        Self::Application(error)
    }
}
impl From<ServiceError> for Failure {
    fn from(error: ServiceError) -> Self {
        Self::Service(error)
    }
}
impl From<ArtifactError> for Failure {
    fn from(error: ArtifactError) -> Self {
        Self::Artifact(error)
    }
}
impl From<QueryError> for Failure {
    fn from(error: QueryError) -> Self {
        Self::Query(error)
    }
}
impl From<io::Error> for Failure {
    fn from(error: io::Error) -> Self {
        Self::Io(error.kind())
    }
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
fn epoch_height(origin: u64, epoch: u64, offset: u64) -> u64 {
    origin + epoch * 128 + offset
}
fn rubric() -> CodecResult<RubricDigest> {
    RubricDigest::new([4; 32])
}
fn task_policy() -> CodecResult<TaskPolicyV1> {
    let mut policy = TaskPolicyV1::bounded_default(
        1,
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
    policy.minimum_evaluator_count = 3;
    Ok(policy)
}
fn delegate() -> SigningKey {
    SigningKey::from_bytes(&DELEGATE_SEED)
}
fn customer() -> SigningKey {
    SigningKey::from_bytes(&CUSTOMER_SEED)
}
fn public(key: &SigningKey) -> PublicKey32 {
    PublicKey32(key.verifying_key().to_bytes())
}
/// Market, worker owner and worker of the journey.
fn identities() -> CodecResult<(MarketId, PrincipalId, WorkerId)> {
    let market = derive_market(ChainDomain::new(CHAIN)?, ProgramId::new(PROGRAM)?)?;
    let owner = principal(1)?;
    Ok((market, owner, derive_worker(market, owner, [1; 32])?))
}

fn encode_state(state: &SharedState<'_>) -> CodecResult<Vec<u8>> {
    let mut out = vec![0; state.encoded_len()?];
    let mut scratch = vec![0; Section::Control.payload_cap()];
    let n = encode_shared_state(state, &mut out, &mut scratch)?;
    out.truncate(n);
    Ok(out)
}

struct Call<'a> {
    operation: Operation,
    actor: PrincipalId,
    epoch: u64,
    config: u64,
    roster: Presence<RosterDigest>,
    sequence: u64,
    request: u8,
    payload: &'a [u8],
}
impl Call<'_> {
    fn encode(&self) -> CodecResult<Vec<u8>> {
        let chain = ChainDomain::new(CHAIN)?;
        let program = ProgramId::new(PROGRAM)?;
        let envelope = Envelope {
            operation: self.operation,
            chain,
            program,
            market: derive_market(chain, program)?,
            actor: self.actor,
            epoch: self.epoch,
            config: self.config,
            roster: self.roster,
            sequence: self.sequence,
            expiry: u64::MAX,
            request: RequestId::new([self.request; 32])?,
            payload: self.payload,
            authentication: Authentication::Native,
        };
        let mut encoded = vec![0; 32_768];
        let n = encode_envelope(&envelope, &mut encoded)?;
        encoded.truncate(n);
        Ok(encoded)
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

/// Committed state after the real F01 CREATE at `origin`.
fn create(origin: u64) -> CodecResult<Vec<u8>> {
    let program = ProgramId::new(PROGRAM)?;
    let mut policy_bytes = [0; TASK_POLICY_BYTES];
    task_policy()?.encode(&mut policy_bytes)?;
    let mut payload = OWNER.to_vec();
    payload.extend_from_slice(&ASSET);
    payload.extend_from_slice(derive_rewards_account(program, AssetId::new(ASSET)?)?.as_bytes());
    payload.extend_from_slice(&REFUND);
    payload.push(0);
    payload.extend_from_slice(&policy_bytes);
    payload.extend_from_slice(&[16; 32]);
    let call = Call {
        operation: dispatch::CREATE,
        actor: PrincipalId::new(OWNER)?,
        epoch: 0,
        config: 1,
        roster: Presence::Absent,
        sequence: 1,
        request: 1,
        payload: &payload,
    };
    let encoded = call.encode()?;
    let mut section = vec![0; F01_SECTION_CAP];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let Registered::Applied { state, .. } = registry_ops::apply(
        &call.context(origin)?,
        None,
        &decode_envelope(&encoded)?,
        &mut section,
        &mut event,
    )?
    else {
        return Err(NON_CANONICAL);
    };
    encode_state(&state)
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
    replay: ReplayTable,
    features: Vec<u8>,
}
impl Parts {
    fn load(bytes: &[u8]) -> CodecResult<Self> {
        let state = decode_shared_state(bytes)?;
        let [policy, identity, reports, rewards, admission] = state.feature_sections;
        let (workers, region) = split_identity_section(identity)?;
        Ok(Self {
            revision: state.revision,
            policy: policy.to_vec(),
            workers: WorkerTable::decode(workers)?,
            region: region.to_vec(),
            reports: reports.to_vec(),
            rewards: rewards.to_vec(),
            admission: AdmissionTable::decode(admission)?,
            features: state.control.feature_bytes.to_vec(),
            replay: state.control.replay,
        })
    }
    fn market(&self) -> CodecResult<MarketHeader> {
        Ok(PolicySection::decode(&self.policy)?.header)
    }
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
        encode_state(&SharedState {
            revision: self.revision,
            feature_sections: [
                &policy,
                &identity,
                &self.reports,
                &self.rewards,
                &admission[..admission_len],
            ],
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

fn admission_context(market: &MarketHeader, who: PrincipalId, at: u64) -> AdmissionContext<'_> {
    AdmissionContext {
        market,
        invoking_principal: who,
        height: at,
    }
}
/// Market-owner approval bound to the required effective epoch.
fn approve(
    table: &mut AdmissionTable,
    market: &MarketHeader,
    participant: Participant,
    owner: PrincipalId,
    delegate: PublicKey32,
    at: u64,
) -> CodecResult<(u64, Digest32)> {
    let effective = table.required_effective_epoch(&admission_context(market, owner, at))?;
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
        expiry_height: epoch_height(market.origin_height, effective, 64),
    };
    let digest = table.approve(
        &admission_context(market, market.owner_principal, at),
        &terms,
    )?;
    Ok((effective, digest))
}

fn evaluator_grant(
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

/// One market's committed shared state bytes and its next owner sequence.
#[derive(Clone)]
struct World {
    bytes: Vec<u8>,
    owner_sequence: u64,
}
impl World {
    fn create(origin: u64) -> CodecResult<Self> {
        Ok(Self {
            bytes: create(origin)?,
            owner_sequence: 2,
        })
    }
    fn parts(&self) -> CodecResult<Parts> {
        Parts::load(&self.bytes)
    }
    fn revision(&self) -> CodecResult<u64> {
        Ok(decode_shared_state(&self.bytes)?.revision)
    }
    fn header(&self) -> CodecResult<MarketHeader> {
        self.parts()?.market()
    }
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
    /// A real owner-authorized F01 registry operation.
    fn owner_op(&mut self, operation: Operation, payload: &[u8], at: u64) -> CodecResult<()> {
        let call = Call {
            operation,
            actor: PrincipalId::new(OWNER)?,
            epoch: 0,
            config: self.header()?.active_config_version,
            roster: Presence::Absent,
            sequence: self.owner_sequence,
            request: 0x20 + u8::try_from(self.owner_sequence).map_err(|_| ARITHMETIC)?,
            payload,
        };
        let encoded = call.encode()?;
        let current = decode_shared_state(&self.bytes)?;
        let mut section = vec![0; F01_SECTION_CAP];
        let mut event = vec![0; MAX_EVENT_BYTES];
        let Registered::Applied { state, .. } = registry_ops::apply(
            &call.context(at)?,
            Some(&current),
            &decode_envelope(&encoded)?,
            &mut section,
            &mut event,
        )?
        else {
            return Err(NON_CANONICAL);
        };
        self.bytes = encode_state(&state)?;
        self.owner_sequence += 1;
        Ok(())
    }
    fn schedule(&mut self, epoch: u64, at: u64) -> CodecResult<()> {
        let mut payload = self.revision()?.to_be_bytes().to_vec();
        payload.extend_from_slice(&epoch.to_be_bytes());
        self.owner_op(dispatch::SCHEDULE_ACTIVATION, &payload, at)
    }
}

/// Worker, evaluator and funding producers of the market journey.
impl World {
    /// F02 record, worker replay slot and F08 approval plus owner acceptance.
    fn enroll(&mut self, record: &WorkerCurrent, at: u64) -> CodecResult<WorkerRosterEntry> {
        self.edit(|parts, market| {
            let record = WorkerCurrent {
                slot: parts.workers.free_slot()?,
                ..*record
            };
            parts.workers.insert(&record)?;
            parts.replay.bind(
                ActorSlot::worker(usize::from(record.slot))?,
                record.owner,
                version()?,
            )?;
            let participant = Participant::Worker(record.worker);
            let (effective, digest) = approve(
                &mut parts.admission,
                market,
                participant,
                record.owner,
                record.delegate,
                at,
            )?;
            parts.admission.admit(
                &admission_context(market, record.owner, at),
                &Membership {
                    participant,
                    delegate_generation: 1,
                    effective_epoch: effective,
                    config_version: 1,
                    approval_digest: digest,
                },
            )?;
            Ok(WorkerRosterEntry {
                worker: record.worker,
                owner: record.owner,
                recipient: AccountId::new(record.owner.bytes())?,
                generation: version()?,
                key_version: version()?,
                public_key: record.delegate,
                metadata: record.metadata,
            })
        })
    }
    /// F03 nomination accepted through the signed F08 evaluator consent of its delegate.
    fn evaluator(&mut self, n: u8, at: u64) -> CodecResult<EvaluatorRosterEntry> {
        self.edit(|parts, market| {
            let owner = principal(n)?;
            let key = SigningKey::from_bytes(&[n; 32]);
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
            let grant = evaluator_grant(market, owner, n, signing_key, effective)?;
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
                expiry_height: epoch_height(market.origin_height, effective, 64),
            };
            let mut signed = [0u8; 362];
            consent.encode(&mut signed)?;
            let mut payload = signed.to_vec();
            payload.extend_from_slice(&key.sign(consent.digest()?.as_bytes()).to_bytes());
            admit_evaluator(
                &mut parts.admission,
                &admission_context(market, owner, at),
                &grant,
                consent.request,
                &payload,
            )?;
            parts.insert_grant(grant, n)?;
            Ok(EvaluatorRosterEntry {
                evaluator: grant.evaluator,
                owner: grant.principal,
                grant: grant.grant_version,
                key_version: grant.key_version,
                public_key: grant.signing_key,
                rubric: grant.rubric,
            })
        })
    }
    /// Real owner FUND of `amount` into the F06 reward state.
    fn fund(&mut self, amount: u128, at: u64) -> CodecResult<()> {
        let mut parts = self.parts()?;
        let market = parts.market()?;
        if parts.rewards.is_empty() {
            let ledger = RewardLedger::new(
                market.funding_asset,
                market.rewards_account,
                market.refund_recipient_account,
            )?;
            let mut initial = vec![0; REWARD_STATE_BYTES];
            RewardState::init(&ledger, &mut initial)?;
            parts.rewards = initial;
        }
        let tag = 0x40 + u8::try_from(self.owner_sequence).map_err(|_| ARITHMETIC)?;
        let request = ReplayRequest {
            slot: ActorSlot::OWNER,
            principal: market.owner_principal,
            authority_version: version()?,
            sequence: self.owner_sequence,
            request_id: RequestId::new([tag; 32])?,
            digest: RequestDigest::new([tag; 32])?,
            expiry_height: at + 1000,
        };
        let mut funded = vec![0; REWARD_STATE_BYTES];
        decode_reward_state(&parts.rewards)?.fund(
            &FundingAuthority {
                owner: market.owner_principal,
                treasury: Presence::Absent,
            },
            FundingPhase::Accepting,
            &FundRequest {
                amount,
                refund_recipient: market.refund_recipient_account,
                policy_version: FUNDING_POLICY_VERSION,
                consent: true,
            },
            &mut FundReplay {
                table: &mut parts.replay,
                request: &request,
                height: at,
                revision: &mut parts.revision,
                result: ResultDigest::new([tag; 32])?,
            },
            &mut funded,
        )?;
        parts.rewards = funded;
        self.bytes = parts.encode()?;
        self.owner_sequence += 1;
        Ok(())
    }
}

/// The permissionless `ADVANCE_ACTIVATION` and `OPEN_EPOCH` calls.
impl World {
    fn advance(&mut self, at: u64) -> CodecResult<()> {
        let header = self.header()?;
        let mut payload = self.revision()?.to_be_bytes().to_vec();
        payload.extend_from_slice(&header.activation_epoch.to_be_bytes());
        let call = Call {
            operation: dispatch::ADVANCE_ACTIVATION,
            actor: principal(KEEPER)?,
            epoch: 0,
            config: header.active_config_version,
            roster: Presence::Absent,
            sequence: 0,
            request: 0x61,
            payload: &payload,
        };
        let encoded = call.encode()?;
        let mut next = vec![0; MAX_STATE_BYTES];
        let mut scratch = vec![0; ADVANCE_SCRATCH_BYTES];
        let mut event = vec![0; MAX_EVENT_BYTES];
        let activated = epoch::advance_activation(
            &call.context(at)?,
            &decode_envelope(&encoded)?,
            &self.bytes,
            &mut next,
            &mut scratch,
            &mut event,
        )?;
        next.truncate(activated.state_len);
        self.bytes = next;
        Ok(())
    }
    /// Opens the clock epoch of `at` naming the previewed frozen config and roster.
    fn open(&mut self, at: u64) -> CodecResult<Frozen> {
        let mut next = vec![0; MAX_STATE_BYTES];
        let mut scratch = vec![0; OPEN_SCRATCH_BYTES];
        let preview = epoch::preview_open(&self.bytes, at, &mut next, &mut scratch)?;
        let call = Call {
            operation: dispatch::OPEN_EPOCH,
            actor: principal(KEEPER)?,
            epoch: preview.epoch,
            config: preview.config.get(),
            roster: Presence::Present(preview.roster),
            sequence: 0,
            request: 0x60,
            payload: &[],
        };
        let encoded = call.encode()?;
        let mut event = vec![0; MAX_EVENT_BYTES];
        match epoch::open_epoch(
            &call.context(at)?,
            &decode_envelope(&encoded)?,
            &self.bytes,
            &mut next,
            &mut scratch,
            &mut event,
        )? {
            Opening::Opened {
                state_len, frozen, ..
            } => {
                next.truncate(state_len);
                self.bytes = next;
                Ok(frozen)
            }
            Opening::AlreadyApplied { .. } => Err(NON_CANONICAL),
        }
    }
}

fn capability(model: u8) -> Capability {
    Capability {
        kind: 1,
        mode: 1,
        model: [model; 32],
        model_manifest: [0x32; 32],
        tokenizer: [0x33; 32],
        input_schema: [0x34; 32],
        output_schema: [0x35; 32],
        max_input_bytes: 65_536,
        max_output_bytes: 65_536,
        max_input_units: 4_096,
        max_output_units: 4_096,
        unit_kind: 1,
        latency_ms: 30_000,
        concurrency: 4,
        determinism: 1,
    }
}
fn endpoint(uri: &str, pin: [u8; 32]) -> Endpoint {
    Endpoint {
        id: 1,
        uri: uri.to_owned(),
        spki_sha256: pin,
    }
}
fn manifest(
    revision: u64,
    capabilities: Vec<Capability>,
    endpoints: Vec<Endpoint>,
) -> Checked<Manifest> {
    let (market, owner, worker) = identities()?;
    Ok(Manifest {
        market,
        worker,
        owner,
        generation: 1,
        key_version: 1,
        revision,
        valid_from: VALID_FROM,
        expiry: EXPIRY,
        deployment: Digest32::new([0xDE; 32])?,
        capabilities,
        endpoints,
        privacy_policy: Digest32::new([0x9A; 32])?,
        service_terms: Digest32::new([0x7E; 32])?,
    })
}
fn metadata_context() -> Checked<MetadataContext> {
    let (market, owner, worker) = identities()?;
    Ok(MetadataContext {
        chain: ChainDomain::new(CHAIN)?,
        program: ProgramId::new(PROGRAM)?,
        market,
        worker,
        owner,
        delegate: public(&delegate()),
    })
}
fn publication(manifest: &[u8], expected_revision: u64) -> Checked<MetadataPublication<'_>> {
    Ok(MetadataPublication {
        manifest,
        expected_revision,
        epoch: 0,
        config: 1,
        roster: Presence::Absent,
        sequence: expected_revision + 1,
        expiry: EXPIRY,
        request: RequestId::new([0x71; 32])?,
    })
}
fn sign(manifest: &Manifest, expected_revision: u64) -> Checked<Vec<u8>> {
    let bytes = manifest.encode()?;
    Ok(sign_metadata(
        &metadata_context()?,
        &publication(&bytes, expected_revision)?,
        &delegate(),
    )?)
}

/// The opened journey market and the evidence a discovery client holds about it.
struct Market {
    world: World,
    roster: Vec<u8>,
    signed: Vec<u8>,
}

/// Real `CREATE`, `SCHEDULE_ACTIVATION`, worker enrollment bound to the signed manifest digest,
/// three accepted evaluators, `FUND`, `ADVANCE_ACTIVATION` and `OPEN_EPOCH` 1, whose Work window
/// is 1128..1192.
fn opened(endpoints: Vec<Endpoint>) -> Checked<Market> {
    let (market, owner, worker) = identities()?;
    let manifest = manifest(1, vec![capability(0x31)], endpoints)?;
    let signed = sign(&manifest, 0)?;
    let (_, digest) = Manifest::decode(&manifest.encode()?)?;
    let mut world = World::create(ORIGIN)?;
    world.schedule(1, 1001)?;
    let entry = world.enroll(
        &WorkerCurrent {
            worker,
            owner,
            delegate: public(&delegate()),
            metadata: digest,
            generation: 1,
            key_version: 1,
            metadata_revision: 1,
            valid_from: VALID_FROM,
            expiry: EXPIRY,
            revocation_sequence: 0,
            effective_epoch: 0,
            last_sequence: 0,
            last_request_id: [0; 32],
            last_request_digest: [0; 32],
            last_result_digest: [0; 32],
            state: WorkerState::Enrolled,
            slot: 0,
            last_metadata_height: 1002,
        },
        1002,
    )?;
    let mut evaluators = Vec::new();
    for n in 2..=4u8 {
        evaluators.push(world.evaluator(n, 1004 + u64::from(n))?);
    }
    world.fund(500, 1128)?;
    world.advance(1128)?;
    world.open(1129)?;
    evaluators.sort_by_key(|e| e.evaluator);
    let mut roster = vec![0; 54 + 176 + 144 * evaluators.len()];
    let n = encode_roster(
        &Roster {
            market,
            epoch: 1,
            config: version()?,
            workers: &[entry],
            evaluators: &evaluators,
        },
        &mut roster,
    )?;
    roster.truncate(n);
    Ok(Market {
        world,
        roster,
        signed,
    })
}

fn chunk_payload(revision: u64, pinned: Option<StateDigest>, offset: u32) -> Vec<u8> {
    let mut payload = revision.to_be_bytes().to_vec();
    payload.extend_from_slice(&pinned.map_or([0; 32], StateDigest::bytes));
    payload.extend_from_slice(&offset.to_be_bytes());
    payload.extend_from_slice(&8_192u16.to_be_bytes());
    payload
}

/// One complete verified capture of `state` through the real chunked read path.
fn capture(state: &[u8], at: u64) -> Checked<(Vec<u8>, CaptureFacts)> {
    let proof = ReadProof {
        chain: ChainDomain::new(CHAIN)?,
        program: ProgramId::new(PROGRAM)?,
        native_state_root: Digest32::new(ROOT)?,
        observed_sequence: 77,
        execution_height: at,
        block_hash: Digest32::new([0xBB; 32])?,
    };
    let mut out = vec![0; CHUNK_RESPONSE_MAX];
    let n = read_state_chunk(state, &chunk_payload(0, None, 0), &mut out)?;
    let first = codec::decode_chunk_response(&out[..n])?;
    let (revision, pinned, total) = (first.revision, first.digest, first.total_bytes);
    let mut buffer = vec![0; MAX_STATE_BYTES];
    let mut capture = StateCapture::new(&mut buffer);
    capture.accept(&proof, &out[..n])?;
    for offset in (8_192..total).step_by(8_192) {
        let n = read_state_chunk(
            state,
            &chunk_payload(revision, Some(pinned), offset),
            &mut out,
        )?;
        capture.accept(&proof, &out[..n])?;
    }
    let (bytes, facts) = capture.finish()?;
    Ok((bytes.to_vec(), facts))
}

/// Evidence knobs of one finalized authority binding.
#[derive(Clone, Copy)]
struct Knobs {
    sealed: u64,
    rank: u8,
    frozen: bool,
    owner_height: u64,
    roster: bool,
    market: MarketId,
    worker: WorkerId,
}
struct View {
    bytes: Vec<u8>,
    facts: CaptureFacts,
    roster: Vec<u8>,
    at: u64,
}
fn view(state: &[u8], roster: &[u8], at: u64) -> Checked<View> {
    let (bytes, facts) = capture(state, at)?;
    Ok(View {
        bytes,
        facts,
        roster: roster.to_vec(),
        at,
    })
}
impl View {
    fn knobs(&self) -> CodecResult<Knobs> {
        let (market, _, worker) = identities()?;
        Ok(Knobs {
            sealed: self.at + MAX_FINALITY_LAG,
            rank: 4,
            frozen: false,
            owner_height: self.at,
            roster: true,
            market,
            worker,
        })
    }
    fn bind(&self, knobs: Knobs) -> Result<FinalizedAuthority, ServiceError> {
        let (_, owner, _) = identities()?;
        let finality = FinalityEvidence {
            native_state_root: Digest32::new(ROOT)?,
            block_height: self.at,
            block_hash: Digest32::new([0xBB; 32])?,
            app_hash: Digest32::new([0xC1; 32])?,
            store_proof_digest: Digest32::new([0xD3; 32])?,
            rank: knobs.rank,
        };
        FinalizedAuthority::bind(
            &AuthorityEvidence {
                state: &self.bytes,
                facts: &self.facts,
                finality: &finality,
                head: PxNetworkHead {
                    network_id: "paxeer".to_owned(),
                    paxeer: PxPaxeerHead {
                        chain_id: 125,
                        latest_block: knobs.sealed,
                    },
                    layerx: None,
                    latest_committed_height: knobs.sealed,
                },
                owner: IdentityEvidence {
                    principal: owner,
                    primary_key: PublicKey32([0x0E; 32]),
                    frozen: knobs.frozen,
                    execution_height: knobs.owner_height,
                },
                roster: knobs.roster.then_some(self.roster.as_slice()),
                observed_ms: 5_000,
            },
            ChainDomain::new(CHAIN)?,
            ProgramId::new(PROGRAM)?,
            knobs.market,
            knobs.worker,
        )
    }
    fn authority(&self) -> Checked<FinalizedAuthority> {
        Ok(self.bind(self.knobs()?)?)
    }
}
fn authority_at(market: &Market, at: u64) -> Checked<FinalizedAuthority> {
    view(&market.world.bytes, &market.roster, at)?.authority()
}
fn verified(authority: &FinalizedAuthority, signed: &[u8]) -> Checked<VerifiedMetadata> {
    Ok(authority.bind_metadata(verify_signed_metadata(
        signed,
        &authority.metadata_context(),
    )?)?)
}
fn customer_at(at: u64) -> Checked<IdentityEvidence> {
    Ok(IdentityEvidence {
        principal: principal(9)?,
        primary_key: public(&customer()),
        frozen: false,
        execution_height: at,
    })
}

/// A customer submit bound to `authority` and naming `capability` of the current revision.
#[derive(Clone, Copy)]
struct Submit {
    context: ServiceContext,
    request: ServiceRequest,
    task: TaskBinding,
}
fn submit(authority: &FinalizedAuthority, capability: &Capability) -> Checked<Submit> {
    let binding = authority.worker_binding();
    let request = ServiceRequest {
        receiver: binding.worker,
        task: TaskId::new([0x7A; 32])?,
        generation: 1,
        key_version: 1,
        metadata_revision: binding.metadata_revision,
        method: Route::Submit.code(),
        route: Route::Submit.code(),
        capability: capability.digest()?,
        workload_policy: authority.policy(),
        model: Digest32::new(capability.model)?,
        input_commitment: Digest32::new([0x1C; 32])?,
        payload_bytes: 1_024,
        max_output_bytes: 2_048,
        max_units: 256,
        deadline_ms: 0,
        task_expiry: TASK_EXPIRY,
        evaluation_access: Digest32::new([0xEA; 32])?,
        result_key: Some([0x4B; 32]),
    };
    let context = ServiceContext {
        chain: binding.chain,
        program: binding.program,
        market: binding.market,
        actor: principal(9)?,
        epoch: 1,
        config: authority.config(),
        roster: authority.roster(),
        sequence: 1,
        expiry: TASK_EXPIRY,
        request: RequestId::new([0x88; 32])?,
    };
    let task = TaskBinding {
        task: request.task,
        requester: context.actor,
        worker: binding.worker,
        input: request.input_commitment,
        deadline: TASK_EXPIRY,
        status: TaskStatus::Admitted,
        acknowledgement: None,
        result: None,
        admission: Digest32::new([0xAD; 32])?,
    };
    Ok(Submit {
        context,
        request,
        task,
    })
}
impl Submit {
    fn signed(&self, key: &SigningKey) -> Checked<Vec<u8>> {
        Ok(encode_service(
            ServiceOperation::SubmitJob,
            &self.context,
            &self.request.encode()?,
            key,
        )?)
    }
    fn admit(
        &self,
        authority: &FinalizedAuthority,
        evidence: &IdentityEvidence,
        metadata: &VerifiedMetadata,
    ) -> Checked<Result<Admission, ServiceError>> {
        let envelope = decode_service(&self.signed(&customer())?)?;
        Ok(admit(
            &envelope,
            &self.request,
            authority,
            evidence,
            metadata,
            &self.task,
        ))
    }
    fn with(&self, change: impl FnOnce(&mut Self)) -> Self {
        let mut changed = *self;
        change(&mut changed);
        changed
    }
}

/// The opened market at `HEIGHT` with its verified revision-1 metadata.
struct Admitting {
    market: Market,
    authority: FinalizedAuthority,
    metadata: VerifiedMetadata,
    submit: Submit,
}
fn admitting() -> Checked<Admitting> {
    let market = opened(vec![endpoint(DEFAULT_URI, [0xE1; 32])])?;
    let authority = authority_at(&market, HEIGHT)?;
    let metadata = verified(&authority, &market.signed)?;
    let submit = submit(&authority, &capability(0x31))?;
    Ok(Admitting {
        market,
        authority,
        metadata,
        submit,
    })
}

/// A private store directory removed when the test ends.
struct Scratch(PathBuf);
impl Scratch {
    fn new(name: &str) -> Checked<Self> {
        let path = std::env::temp_dir().join(format!(
            "paxai-repair-22-31-205-{}-{name}",
            std::process::id()
        ));
        if path.exists() {
            fs::remove_dir_all(&path)?;
        }
        fs::create_dir_all(&path)?;
        Ok(Self(path))
    }
    fn store(&self) -> PathBuf {
        self.0.join("store")
    }
    /// The configured runner path when no runner executable is installed.
    fn absent(&self) -> PathBuf {
        self.0.join("runner-not-installed")
    }
}
impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn service(root: &Path, runner: PathBuf) -> Checked<JobService> {
    Ok(JobService::new(
        JobStore::open(root)?,
        ProcessRunner::new(runner, Vec::new(), Duration::from_secs(5)),
        delegate(),
    ))
}

fn real_runner() -> Checked<PathBuf> {
    std::env::var_os(RUNNER_ENV)
        .map(PathBuf::from)
        .ok_or(Failure::Unexpected(
            "real model runner executable: PAXAI_RUNNER is not provisioned",
        ))
}

/// Encoded encrypted F09 manifest of `object` bound to `task` of epoch 1.
fn object_manifest(
    kind: ArtifactKind,
    publisher: PrincipalId,
    task: TaskId,
    policy: PolicyDigest,
    object: &[u8],
) -> Checked<Vec<u8>> {
    let byte_length = u64::try_from(object.len()).map_err(|_| ARITHMETIC)?;
    let chunks = chunk_count(byte_length)?;
    let mut scratch = vec![[0; 32]; usize::try_from(chunks).map_err(|_| ARITHMETIC)?];
    let content_root = object_content_root(object, &mut scratch)?;
    let (market, _, _) = identities()?;
    let manifest = ArtifactManifest {
        kind,
        privacy: Privacy::Encrypted,
        context: ArtifactContext {
            chain: ChainDomain::new(CHAIN)?,
            program: ProgramId::new(PROGRAM)?,
            market,
            policy,
        },
        epoch: 1,
        publisher,
        subject: task.bytes(),
        byte_length,
        chunk_count: chunks,
        content_root,
        parents: Items::Typed(&[]),
        declaration_root: [0; 32],
        reproduction_root: [0; 32],
        access_policy_root: [0xAC; 32],
        not_after_height: 4_000,
    };
    let mut out = vec![0; MAX_MANIFEST_BYTES];
    let n = encode_manifest(&manifest, &mut out)?;
    out.truncate(n);
    Ok(out)
}
fn root(manifest: &[u8]) -> Checked<Digest32> {
    Ok(Digest32::new(manifest_root(manifest)?.bytes())?)
}

/// One customer job: the signed submit, its encrypted input and its admitted F01 task.
struct Order {
    signed: Vec<u8>,
    manifest: Vec<u8>,
    payload: Vec<u8>,
    submit: Submit,
}
fn order(fixture: &Admitting, request: u8, sequence: u64, max_output_bytes: u32) -> Checked<Order> {
    let payload = vec![request ^ 0x5A; PAYLOAD_BYTES];
    let task = codec::derive_task(identities()?.0, 1, principal(9)?, [request; 32])?;
    let manifest = object_manifest(
        ArtifactKind::Input,
        principal(9)?,
        task,
        fixture.authority.policy(),
        &payload,
    )?;
    let input = root(&manifest)?;
    let id = RequestId::new([request; 32])?;
    let submit = fixture.submit.with(|s| {
        s.request.task = task;
        s.request.input_commitment = input;
        s.request.max_output_bytes = max_output_bytes;
        s.context.request = id;
        s.context.sequence = sequence;
        s.task.task = task;
        s.task.input = input;
    });
    Ok(Order {
        signed: submit.signed(&customer())?,
        manifest,
        payload,
        submit,
    })
}
impl Order {
    fn input(&self) -> Input<'_> {
        Input {
            manifest: &self.manifest,
            payload: &self.payload,
        }
    }
    fn view<'a>(
        &'a self,
        authority: &'a FinalizedAuthority,
        customer: &'a IdentityEvidence,
        metadata: &'a VerifiedMetadata,
    ) -> AdmissionView<'a> {
        AdmissionView {
            authority,
            customer,
            metadata,
            task: &self.submit.task,
        }
    }
    fn key(&self) -> Checked<JobKey> {
        Ok(JobKey::derive(
            self.submit.context.market,
            self.submit.context.actor,
            self.submit.context.request,
        )?)
    }
    fn accepted(&self, acknowledgment: &[u8]) -> Checked<TaskBinding> {
        Ok(TaskBinding {
            status: TaskStatus::Accepted,
            acknowledgement: Some(acknowledgment_digest(acknowledgment)?),
            ..self.submit.task
        })
    }
    /// A signed query or cancel of this job by `actor` under `key`.
    fn reference(
        &self,
        route: Route,
        actor: PrincipalId,
        sequence: u64,
        key: &SigningKey,
    ) -> Checked<Vec<u8>> {
        let payload = JobReference {
            receiver: self.submit.request.receiver,
            task: self.submit.request.task,
            generation: 1,
            key_version: 1,
            metadata_revision: 1,
            method: route.code(),
            route: route.code(),
            original: self.submit.context.request,
        }
        .encode();
        let context = ServiceContext {
            actor,
            sequence,
            ..self.submit.context
        };
        Ok(encode_service(route.operation(), &context, &payload, key)?)
    }
    /// The F09 RESULT object a worker returns for this job.
    fn output(&self, fixture: &Admitting, bytes: usize) -> Checked<(Vec<u8>, Vec<u8>)> {
        let output = vec![0x0F; bytes];
        let manifest = object_manifest(
            ArtifactKind::Result,
            principal(1)?,
            self.submit.request.task,
            fixture.authority.policy(),
            &output,
        )?;
        Ok((output, manifest))
    }
}

fn admit_chain_task(fixture: &mut Admitting, order: &Order, nonce: u8) -> Checked {
    let authority = fixture.authority;
    let mut payload = 1u64.to_be_bytes().to_vec();
    payload.extend_from_slice(&authority.config().to_be_bytes());
    payload.extend_from_slice(authority.policy().as_bytes());
    let Presence::Present(roster) = authority.roster() else {
        return Err(Failure::Unexpected("roster missing"));
    };
    payload.extend_from_slice(roster.as_bytes());
    payload.extend_from_slice(principal(9)?.as_bytes());
    payload.extend_from_slice(authority.record().worker.as_bytes());
    payload.extend_from_slice(fixture.metadata.digest().as_bytes());
    payload.extend_from_slice(&[nonce; 32]);
    payload.extend_from_slice(order.submit.request.input_commitment.as_bytes());
    payload.extend_from_slice(&TASK_EXPIRY.to_be_bytes());
    let call = Call {
        operation: dispatch::ADMIT_TASK,
        actor: principal(9)?,
        epoch: 1,
        config: authority.config(),
        roster: authority.roster(),
        sequence: 0,
        request: nonce,
        payload: &payload,
    };
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let mut result = vec![0; layerx_programs_ai_market::MAX_RESULT_BYTES];
    let routed = dispatch::route(
        &call.context(HEIGHT)?,
        &call.encode()?,
        Some(&fixture.market.world.bytes),
        dispatch::Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?;
    let dispatch::Routed::Applied { state_len, .. } = routed else {
        let result_len = match routed {
            dispatch::Routed::Refused { result_len }
            | dispatch::Routed::Unchanged { result_len } => result_len,
            dispatch::Routed::Applied { .. } => unreachable!(),
        };
        panic!(
            "real ADMIT_TASK did not apply: {:?}",
            codec::decode_result(&result[..result_len])
        );
    };
    next.truncate(state_len);
    fixture.market.world.bytes = next;
    Ok(())
}

fn worker_service(directory: &Scratch) -> Checked<worker::Service> {
    let binding = admitting()?.authority.worker_binding();
    let membership = layerx_paxai_worker::membership::MembershipObserver::open(
        &directory.0,
        layerx_paxai_worker::membership::Subject {
            chain: binding.chain,
            program: binding.program,
            market: binding.market,
            worker: binding.worker,
        },
    )
    .map_err(|_| Failure::Unexpected("membership store unavailable"))?;
    let input_dir = directory.0.join("inputs");
    fs::create_dir_all(&input_dir)?;
    Ok(worker::Service {
        binding,
        key: delegate(),
        metrics: Default::default(),
        jobs: service(
            &directory.store(),
            PathBuf::from(env!("CARGO_BIN_EXE_layerx-paxai-runner")),
        )?,
        runtime: std::sync::Mutex::new(worker::Runtime {
            membership,
            current: None,
            refreshed: None,
        }),
        state_dir: directory.0.clone(),
        input_dir,
    })
}

fn feed(service: &worker::Service, market: &Market, height: u64) -> Checked {
    let view = view(&market.world.bytes, &market.roster, height)?;
    let finality = FinalityEvidence {
        native_state_root: view.facts.proof.native_state_root,
        block_height: height,
        block_hash: view.facts.proof.block_hash,
        app_hash: Digest32::new([0xC1; 32])?,
        store_proof_digest: Digest32::new([0xD3; 32])?,
        rank: 4,
    };
    let evidence = AuthorityEvidence {
        state: &view.bytes,
        facts: &view.facts,
        finality: &finality,
        head: PxNetworkHead {
            network_id: "paxeer".to_owned(),
            paxeer: PxPaxeerHead {
                chain_id: 125,
                latest_block: height,
            },
            layerx: None,
            latest_committed_height: height,
        },
        owner: IdentityEvidence {
            principal: identities()?.1,
            primary_key: PublicKey32([0x0E; 32]),
            frozen: false,
            execution_height: height,
        },
        roster: Some(&market.roster),
        observed_ms: NOW,
    };
    service.runtime.lock().expect("runtime lock").observe(
        &evidence,
        &market.signed,
        vec![customer_at(height)?],
    )?;
    Ok(())
}

fn stage_input(service: &worker::Service, order: &Order) -> Checked {
    let name = order
        .submit
        .request
        .input_commitment
        .bytes()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<String>();
    fs::write(
        service.input_dir.join(format!("{name}.manifest")),
        &order.manifest,
    )?;
    fs::write(
        service.input_dir.join(format!("{name}.payload")),
        &order.payload,
    )?;
    Ok(())
}

use layerx_paxai_worker::auth::{
    EvaluationAccess, StatusChallenge, CANCEL_PATH, QUERY_PATH, STATUS_PATH, SUBMIT_PATH,
};
use layerx_paxai_worker::discovery::{spki_sha256, EndpointClient, NetworkProfile, TransportError};
use std::io::{BufRead, BufReader};
use std::net::{IpAddr, Ipv4Addr, TcpListener};
use std::os::unix::fs::PermissionsExt;
use std::process::{Child, Command, Stdio};
use std::sync::Arc;
use std::time::{Instant, SystemTime, UNIX_EPOCH};

fn millis() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("wall clock")
        .as_millis()
        .try_into()
        .expect("milliseconds")
}

struct TlsWorker {
    directory: Scratch,
    config: PathBuf,
    endpoint: Endpoint,
    client: EndpointClient,
    process: Option<Child>,
}

impl Drop for TlsWorker {
    fn drop(&mut self) {
        self.stop();
    }
}

impl TlsWorker {
    fn new(name: &str) -> Checked<(Self, Admitting)> {
        let directory = Scratch::new(name)?;
        let ca_key = rcgen::KeyPair::generate().expect("CA key");
        let mut ca_params =
            rcgen::CertificateParams::new(Vec::<String>::new()).expect("CA parameters");
        ca_params.is_ca = rcgen::IsCa::Ca(rcgen::BasicConstraints::Unconstrained);
        let ca = ca_params.self_signed(&ca_key).expect("CA certificate");
        let issuer = rcgen::Issuer::from_params(&ca_params, &ca_key);
        let leaf_key = rcgen::KeyPair::generate().expect("TLS key");
        let leaf = rcgen::CertificateParams::new(vec!["localhost".into()])
            .expect("leaf parameters")
            .signed_by(&leaf_key, &issuer)
            .expect("TLS certificate");
        let mut roots = rustls::RootCertStore::empty();
        roots.add(ca.der().clone()).expect("trusted root");
        let port = TcpListener::bind("127.0.0.1:0")?.local_addr()?.port();
        let endpoint = endpoint(
            &format!("https://localhost:{port}/paxai/v1"),
            spki_sha256(leaf.der()).expect("SPKI"),
        );
        let market = opened(vec![endpoint.clone()])?;
        let authority = authority_at(&market, HEIGHT)?;
        let metadata = verified(&authority, &market.signed)?;
        let submit = submit(&authority, &capability(0x31))?;
        let fixture = Admitting {
            market,
            authority,
            metadata,
            submit,
        };
        let file = |name: &str| directory.0.join(name);
        fs::write(file("chain.pem"), format!("{}{}", leaf.pem(), ca.pem()))?;
        fs::write(file("key.pem"), leaf_key.serialize_pem())?;
        fs::write(
            file("delegate.hex"),
            DELEGATE_SEED
                .iter()
                .map(|n| format!("{n:02x}"))
                .collect::<String>(),
        )?;
        fs::write(file("metadata.bin"), &fixture.market.signed)?;
        fs::create_dir_all(file("inputs"))?;
        // The external authority adapter reads a captured native snapshot. It performs no
        // service logic: the binary must decode all chunks and bind finalized authority.
        let reader = file("authority-reader");
        fs::write(
            &reader,
            format!(
                "#!/bin/sh\ncat >/dev/null\nexec cat '{}'\n",
                file("observation.json").display()
            ),
        )?;
        fs::set_permissions(&reader, fs::Permissions::from_mode(0o700))?;
        let text = |bytes: &[u8]| bytes.iter().map(|n| format!("{n:02x}")).collect::<String>();
        let (market, owner, worker) = identities()?;
        let config = file("worker.json");
        let value = serde_json::json!({
            "listen": format!("127.0.0.1:{port}"),
            "certificate_chain": file("chain.pem"), "private_key": file("key.pem"),
            "delegate_seed": file("delegate.hex"), "signed_metadata": file("metadata.bin"),
            "chain": text(&CHAIN), "program": text(&PROGRAM), "market": text(market.as_bytes()),
            "worker": text(worker.as_bytes()), "owner": text(owner.as_bytes()),
            "state_dir": file("state"), "input_dir": file("inputs"),
            "runner": env!("CARGO_BIN_EXE_layerx-paxai-runner"),
            "authority_reader": reader, "sponsored_sender": file("sender-unavailable"),
            "chain_config": file("chain.json"), "capabilities": [1], "access_declaration": [1],
            "response_capacity": 16384, "budget": [1, 1, 1, 1, 1, 1, 1], "fee_limit": 1
        });
        fs::write(
            &config,
            serde_json::to_vec(&value).expect("configuration JSON"),
        )?;
        let harness = Self {
            directory,
            config,
            endpoint,
            client: EndpointClient {
                profile: NetworkProfile::Private {
                    allowlist: vec![IpAddr::V4(Ipv4Addr::LOCALHOST)],
                },
                roots: Arc::new(roots),
                timeout: Duration::from_secs(3),
            },
            process: None,
        };
        harness.observe(&fixture.market, HEIGHT)?;
        Ok((harness, fixture))
    }

    fn jobs(&self) -> PathBuf {
        self.directory.0.join("state/jobs")
    }

    fn observe(&self, market: &Market, height: u64) -> Checked {
        let mut out = vec![0; CHUNK_RESPONSE_MAX];
        let n = read_state_chunk(&market.world.bytes, &chunk_payload(0, None, 0), &mut out)?;
        let first = codec::decode_chunk_response(&out[..n])?;
        let (revision, digest, total) = (first.revision, first.digest, first.total_bytes);
        let mut chunks = vec![out[..n].to_vec()];
        for offset in (8192..total).step_by(8192) {
            let n = read_state_chunk(
                &market.world.bytes,
                &chunk_payload(revision, Some(digest), offset),
                &mut out,
            )?;
            chunks.push(out[..n].to_vec());
        }
        let identity = |n: u8, key: PublicKey32| {
            serde_json::json!({
                "principal": principal(n).expect("principal").bytes(), "primary_key": key.0,
                "frozen": false, "execution_height": height,
            })
        };
        let observation = serde_json::json!({
            "chunks": chunks, "native_state_root": ROOT, "observed_sequence": 77,
            "execution_height": height, "block_height": height, "block_hash": ([0xBB; 32]),
            "app_hash": ([0xC1; 32]), "store_proof_digest": ([0xD3; 32]), "sealed_height": height,
            "owner": identity(1, PublicKey32([0x0E; 32])),
            "identities": [identity(9, public(&customer())), identity(8, public(&SigningKey::from_bytes(&[0xC8; 32]))),
                identity(2, public(&SigningKey::from_bytes(&[2; 32]))), identity(10, public(&SigningKey::from_bytes(&[10; 32])))],
            "roster": market.roster, "signed_metadata": market.signed,
        });
        let path = self.directory.0.join("observation.json");
        let pending = path.with_extension("pending");
        fs::write(
            &pending,
            serde_json::to_vec(&observation).expect("snapshot JSON"),
        )?;
        fs::rename(pending, path)?;
        Ok(())
    }

    fn start(&mut self, authority: &FinalizedAuthority) -> Checked {
        assert!(self.process.is_none());
        let log = fs::File::create(self.directory.0.join("worker.stderr"))?;
        let mut process = Command::new(env!("CARGO_BIN_EXE_layerx-paxai-worker"))
            .arg("--config")
            .arg(&self.config)
            .stdout(Stdio::piped())
            .stderr(log)
            .spawn()?;
        let mut first = String::new();
        BufReader::new(process.stdout.take().expect("worker stdout")).read_line(&mut first)?;
        self.process = Some(process);
        assert!(
            first.starts_with("listening "),
            "worker startup: {first}; {}",
            fs::read_to_string(self.directory.0.join("worker.stderr"))?
        );
        self.wait_current(authority);
        Ok(())
    }

    fn stop(&mut self) {
        if let Some(mut process) = self.process.take() {
            process.kill().expect("stop worker process");
            process.wait().expect("reap worker process");
        }
    }

    fn challenge(authority: &FinalizedAuthority) -> StatusChallenge {
        StatusChallenge {
            epoch: authority.current_epoch().expect("epoch"),
            config: authority.config(),
            roster: authority.roster(),
            expiry: TASK_EXPIRY,
            challenge: RequestId::new([0x5C; 32]).expect("challenge"),
        }
    }

    fn wait_current(&self, authority: &FinalizedAuthority) {
        let challenge = Self::challenge(authority);
        let until = Instant::now() + Duration::from_secs(10);
        loop {
            let response = self.post(STATUS_PATH, &challenge.encode());
            if let Ok(bytes) = &response {
                if layerx_paxai_worker::auth::verify_status(
                    bytes,
                    &authority.worker_binding(),
                    &challenge,
                )
                .is_ok()
                {
                    return;
                }
            }
            assert!(
                Instant::now() < until,
                "current signed status unavailable: {response:?}; {}",
                fs::read_to_string(self.directory.0.join("worker.stderr"))
                    .expect("worker diagnostics")
            );
            thread::sleep(Duration::from_millis(50));
        }
    }

    fn post(&self, path: &str, body: &[u8]) -> Result<Vec<u8>, TransportError> {
        self.client.post(&self.endpoint, path, body, |_, _| {
            Ok(vec![IpAddr::V4(Ipv4Addr::LOCALHOST)])
        })
    }

    fn stage(&self, order: &Order) -> Checked {
        let name = order
            .submit
            .request
            .input_commitment
            .bytes()
            .iter()
            .map(|n| format!("{n:02x}"))
            .collect::<String>();
        fs::write(
            self.directory.0.join(format!("inputs/{name}.manifest")),
            &order.manifest,
        )?;
        fs::write(
            self.directory.0.join(format!("inputs/{name}.payload")),
            &order.payload,
        )?;
        Ok(())
    }

    fn stage_access(&self, access: &EvaluationAccess) -> Checked {
        let name = access
            .commitment()?
            .bytes()
            .iter()
            .map(|n| format!("{n:02x}"))
            .collect::<String>();
        fs::write(
            self.directory.0.join(format!("inputs/{name}.access")),
            access.encode()?,
        )?;
        Ok(())
    }
}

fn apply_chain(market: &mut Market, signed: &[u8], actor: PrincipalId, at: u64) -> Checked {
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let mut result = vec![0; layerx_programs_ai_market::MAX_RESULT_BYTES];
    let context = CallContext {
        chain: ChainDomain::new(CHAIN)?,
        program: ProgramId::new(PROGRAM)?,
        principal: actor,
        height: at,
    };
    let routed = dispatch::route(
        &context,
        signed,
        Some(&market.world.bytes),
        dispatch::Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?;
    let dispatch::Routed::Applied { state_len, .. } = routed else {
        let result_len = match routed {
            dispatch::Routed::Refused { result_len }
            | dispatch::Routed::Unchanged { result_len } => result_len,
            _ => unreachable!(),
        };
        panic!(
            "native transition refused: {:?}",
            codec::decode_result(&result[..result_len])
        );
    };
    next.truncate(state_len);
    market.world.bytes = next;
    Ok(())
}

fn accept_chain(
    market: &mut Market,
    order: &Order,
    ack: &[u8],
    sequence: u64,
) -> Checked<TaskBinding> {
    let authority = authority_at(market, HEIGHT + 2)?;
    let mut payload = market.world.revision()?.to_be_bytes().to_vec();
    payload.extend_from_slice(order.submit.request.task.as_bytes());
    payload.extend_from_slice(acknowledgment_digest(ack)?.as_bytes());
    let call = Call {
        operation: dispatch::ACCEPT_TASK,
        actor: principal(1)?,
        epoch: 1,
        config: authority.config(),
        roster: authority.roster(),
        sequence,
        request: 0xE0 + u8::try_from(sequence).expect("sequence"),
        payload: &payload,
    };
    apply_chain(market, &call.encode()?, principal(1)?, HEIGHT + 2)?;
    order.accepted(ack)
}

fn drain(market: &mut Market) -> Checked {
    let (_, owner, worker) = identities()?;
    market.world.edit(|parts, header| {
        parts
            .admission
            .request_exit(
                &admission_context(header, owner, HEIGHT),
                Participant::Worker(worker),
                1,
                2,
                layerx_programs_ai_market::admission::ExitReason::Voluntary,
            )
            .map(|_| ())
    })?;
    Ok(())
}

fn publish(market: &mut Market, manifest: &Manifest, key: &SigningKey, sequence: u64) -> Checked {
    let mut context = metadata_context()?;
    context.delegate = public(key);
    let bytes = manifest.encode()?;
    let mut publication = publication(&bytes, manifest.revision - 1)?;
    publication.sequence = sequence;
    publication.request = RequestId::new([0x70 + u8::try_from(sequence).expect("sequence"); 32])?;
    let signed = sign_metadata(&context, &publication, key)?;
    apply_chain(market, &signed, principal(1)?, HEIGHT + 1)?;
    market.signed = signed;
    Ok(())
}

fn order_for(fixture: &Admitting, request: u8, actor: u8, key: &SigningKey) -> Checked<Order> {
    let mut order = order(fixture, request, 1, 512)?;
    let task = codec::derive_task(identities()?.0, 1, principal(actor)?, [request; 32])?;
    order.manifest = object_manifest(
        ArtifactKind::Input,
        principal(actor)?,
        task,
        fixture.authority.policy(),
        &order.payload,
    )?;
    order.submit.context.actor = principal(actor)?;
    order.submit.request.task = task;
    order.submit.request.input_commitment = root(&order.manifest)?;
    order.submit.task.task = task;
    order.submit.task.requester = principal(actor)?;
    order.submit.task.input = order.submit.request.input_commitment;
    order.signed = order.submit.signed(key)?;
    Ok(order)
}

fn admit_order(fixture: &mut Admitting, order: &Order, nonce: u8) -> Checked {
    let authority = fixture.authority;
    let mut payload = 1u64.to_be_bytes().to_vec();
    payload.extend_from_slice(&authority.config().to_be_bytes());
    payload.extend_from_slice(authority.policy().as_bytes());
    let Presence::Present(roster) = authority.roster() else {
        panic!("frozen roster");
    };
    payload.extend_from_slice(roster.as_bytes());
    payload.extend_from_slice(order.submit.context.actor.as_bytes());
    payload.extend_from_slice(authority.record().worker.as_bytes());
    payload.extend_from_slice(fixture.metadata.digest().as_bytes());
    payload.extend_from_slice(&[nonce; 32]);
    payload.extend_from_slice(order.submit.request.input_commitment.as_bytes());
    payload.extend_from_slice(&TASK_EXPIRY.to_be_bytes());
    let call = Call {
        operation: dispatch::ADMIT_TASK,
        actor: order.submit.context.actor,
        epoch: 1,
        config: authority.config(),
        roster: authority.roster(),
        sequence: 0,
        request: nonce,
        payload: &payload,
    };
    apply_chain(&mut fixture.market, &call.encode()?, call.actor, HEIGHT)
}

#[test]
fn worker_drain_preserves_tls_redelivery_query_cancel_and_admitted_execution_after_restart(
) -> Checked {
    let (mut transport, mut fixture) = TlsWorker::new("drain")?;
    let first = order(&fixture, 0x81, 1, 512)?;
    let other = order(&fixture, 0x82, 2, 512)?;
    let new = order(&fixture, 0x83, 3, 512)?;
    for (order, nonce) in [(&first, 0x81), (&other, 0x82), (&new, 0x83)] {
        admit_order(&mut fixture, order, nonce)?;
        transport.stage(order)?;
    }
    transport.observe(&fixture.market, HEIGHT)?;
    transport.start(&fixture.authority)?;
    let ack = transport
        .post(SUBMIT_PATH, &first.signed)
        .expect("first admission through TLS");
    let other_ack = transport
        .post(SUBMIT_PATH, &other.signed)
        .expect("second admission through TLS");
    transport.stop();
    drain(&mut fixture.market)?;
    let drained = authority_at(&fixture.market, HEIGHT + 1)?;
    assert_eq!(
        drained.admission_gate(),
        Err(ServiceError::AdmissionNotEffective)
    );
    transport.observe(&fixture.market, HEIGHT + 1)?;
    fs::remove_dir_all(transport.directory.0.join("inputs"))?;
    transport.start(&drained)?;
    assert_eq!(
        transport
            .post(SUBMIT_PATH, &first.signed)
            .expect("saved acknowledgment during drain"),
        ack
    );
    let query = first.reference(Route::Query, principal(9)?, 10, &customer())?;
    assert_eq!(
        transport
            .post(QUERY_PATH, &query)
            .expect("customer query during drain"),
        ack
    );
    assert_eq!(
        transport.post(SUBMIT_PATH, &new.signed),
        Err(TransportError::Service(ServiceError::AdmissionNotEffective))
    );
    let cancel = first.reference(Route::Cancel, principal(9)?, 11, &customer())?;
    assert_eq!(
        transport
            .post(CANCEL_PATH, &cancel)
            .expect("cancel during drain"),
        ack
    );
    let result = transport
        .post(
            QUERY_PATH,
            &first.reference(Route::Query, principal(9)?, 12, &customer())?,
        )
        .expect("cancel result");
    assert_eq!(
        decode_result(&result, public(&delegate()))?.0.outcome,
        Outcome::CancelledBeforeStart
    );
    transport.stop();
    let accepted = accept_chain(&mut fixture.market, &other, &other_ack, 1)?;
    // Drive the same admission/claim boundary used by maintenance on the reopened durable job.
    let service = worker_service(&transport.directory)?;
    let service = worker::Service {
        jobs: service_jobs(&transport, delegate())?,
        ..service
    };
    feed(&service, &fixture.market, HEIGHT + 2)?;
    let runtime = service.runtime.lock().expect("runtime");
    assert_eq!(
        runtime.membership.new_work(),
        Err(ServiceError::AdmissionNotEffective)
    );
    let job = service.jobs.store().job(other.key()?)?;
    let lease = worker::claim_admitted(
        &service,
        &runtime.membership,
        other.key()?,
        &accepted,
        job.record.accepted_ms + 1,
    )?;
    assert_eq!(
        service.jobs.store().job(lease.key)?.state.state,
        JobState::Running
    );
    Ok(())
}

fn service_jobs(transport: &TlsWorker, key: SigningKey) -> Checked<JobService> {
    Ok(JobService::new(
        JobStore::open(&transport.jobs())?,
        ProcessRunner::new(
            PathBuf::from(env!("CARGO_BIN_EXE_layerx-paxai-runner")),
            Vec::new(),
            Duration::from_secs(5),
        ),
        key,
    ))
}

#[test]
fn worker_current_metadata_refresh_accepts_tls_submit_and_signs_current_status() -> Checked {
    let (mut transport, mut fixture) = TlsWorker::new("metadata")?;
    let old = order(&fixture, 0x84, 1, 512)?;
    admit_order(&mut fixture, &old, 0x84)?;
    // F01 tasks are admitted against the epoch's frozen metadata before its refresh.
    let next_task = order(&fixture, 0x85, 2, 512)?;
    admit_order(&mut fixture, &next_task, 0x85)?;
    transport.stage(&old)?;
    transport.observe(&fixture.market, HEIGHT)?;
    transport.start(&fixture.authority)?;
    let old_ack = transport
        .post(SUBMIT_PATH, &old.signed)
        .expect("revision one admission");
    let replacement = manifest(2, vec![capability(0x41)], vec![transport.endpoint.clone()])?;
    publish(&mut fixture.market, &replacement, &delegate(), 1)?;
    fixture.authority = authority_at(&fixture.market, HEIGHT + 1)?;
    fixture.metadata = verified(&fixture.authority, &fixture.market.signed)?;
    fixture.submit = submit(&fixture.authority, &capability(0x41))?;
    let new = order(&fixture, 0x85, 2, 512)?;
    assert_eq!(new.submit.request.task, next_task.submit.request.task);
    assert_eq!(
        new.submit.request.input_commitment,
        next_task.submit.request.input_commitment
    );
    transport.stage(&new)?;
    transport.observe(&fixture.market, HEIGHT + 1)?;
    transport.wait_current(&fixture.authority);
    let ack = transport
        .post(SUBMIT_PATH, &new.signed)
        .expect("current revision admission");
    let (current, _) = decode_acknowledgment(&ack, public(&delegate()))?;
    assert_eq!(current.metadata, fixture.metadata.digest());
    assert_eq!(current.model, Digest32::new([0x41; 32])?);
    assert_eq!(
        transport
            .post(SUBMIT_PATH, &old.signed)
            .expect("old revision replay"),
        old_ack
    );
    let (saved, _) = decode_acknowledgment(&old_ack, public(&delegate()))?;
    assert_ne!(saved.metadata, current.metadata);
    let stale = new.submit.with(|s| {
        s.context.request = RequestId::new([0x86; 32]).expect("request");
        s.request.metadata_revision = 1;
    });
    assert_eq!(
        transport.post(SUBMIT_PATH, &stale.signed(&customer())?),
        Err(TransportError::Service(ServiceError::WrongRevision))
    );
    Ok(())
}

#[test]
fn worker_verified_evaluator_access_reaches_tls_result_consumer_after_restart() -> Checked {
    let (mut transport, mut fixture) = TlsWorker::new("access")?;
    let mut order = order(&fixture, 0x87, 1, 512)?;
    let access = EvaluationAccess::new(vec![principal(2)?])?;
    order.submit.request.evaluation_access = access.commitment()?;
    order.signed = order.submit.signed(&customer())?;
    admit_order(&mut fixture, &order, 0x87)?;
    transport.stage(&order)?;
    transport.stage_access(&access)?;
    transport.observe(&fixture.market, HEIGHT)?;
    transport.start(&fixture.authority)?;
    let ack = transport
        .post(SUBMIT_PATH, &order.signed)
        .expect("grant-bound admission");
    let evaluator = SigningKey::from_bytes(&[2; 32]);
    let outsider = SigningKey::from_bytes(&[10; 32]);
    assert_eq!(
        transport
            .post(
                QUERY_PATH,
                &order.reference(Route::Query, principal(2)?, 1, &evaluator)?
            )
            .expect("granted query"),
        ack
    );
    assert_eq!(
        transport.post(
            QUERY_PATH,
            &order.reference(Route::Query, principal(10)?, 1, &outsider)?
        ),
        Err(TransportError::Service(ServiceError::AccessDenied))
    );
    assert_eq!(
        transport.post(
            CANCEL_PATH,
            &order.reference(Route::Cancel, principal(2)?, 2, &evaluator)?
        ),
        Err(TransportError::Service(ServiceError::AccessDenied))
    );
    transport
        .post(
            CANCEL_PATH,
            &order.reference(Route::Cancel, principal(9)?, 2, &customer())?,
        )
        .expect("customer cancellation");
    let customer_result = transport
        .post(
            QUERY_PATH,
            &order.reference(Route::Query, principal(9)?, 3, &customer())?,
        )
        .expect("customer result");
    assert_eq!(
        decode_result(&customer_result, public(&delegate()))?
            .0
            .outcome,
        Outcome::CancelledBeforeStart
    );
    transport.stop();
    fs::remove_dir_all(transport.directory.0.join("inputs"))?;
    transport.start(&fixture.authority)?;
    assert_eq!(
        transport
            .post(
                QUERY_PATH,
                &order.reference(Route::Query, principal(2)?, 3, &evaluator)?
            )
            .expect("recovered evaluator grant"),
        customer_result
    );
    assert_eq!(
        transport.post(
            QUERY_PATH,
            &order.reference(Route::Query, principal(10)?, 2, &outsider)?
        ),
        Err(TransportError::Service(ServiceError::AccessDenied))
    );
    // The grant commitment is checked on every release, including after storage damage.
    let name = access
        .commitment()?
        .bytes()
        .iter()
        .map(|n| format!("{n:02x}"))
        .collect::<String>();
    let forged = EvaluationAccess::new(vec![principal(10)?])?;
    fs::write(
        transport.jobs().join(format!("evaluation-access/{name}")),
        forged.encode()?,
    )?;
    assert_eq!(
        transport.post(
            QUERY_PATH,
            &order.reference(Route::Query, principal(10)?, 3, &outsider)?
        ),
        Err(TransportError::Service(
            ServiceError::MetadataIntegrityFailure
        ))
    );
    Ok(())
}

#[test]
fn worker_cancel_returns_exact_customer_acknowledgment_for_shared_request_id() -> Checked {
    let (mut transport, mut fixture) = TlsWorker::new("cancel")?;
    let first = order_for(&fixture, 0x89, 9, &customer())?;
    let second_key = SigningKey::from_bytes(&[0xC8; 32]);
    let second = order_for(&fixture, 0x89, 8, &second_key)?;
    assert_eq!(first.submit.context.request, second.submit.context.request);
    assert_ne!(first.key()?, second.key()?);
    admit_order(&mut fixture, &first, 0x89)?;
    admit_order(&mut fixture, &second, 0x89)?;
    transport.stage(&first)?;
    transport.stage(&second)?;
    transport.observe(&fixture.market, HEIGHT)?;
    transport.start(&fixture.authority)?;
    let first_ack = transport
        .post(SUBMIT_PATH, &first.signed)
        .expect("customer one admission");
    let second_ack = transport
        .post(SUBMIT_PATH, &second.signed)
        .expect("customer two admission");
    assert_ne!(first_ack, second_ack);
    let cancel = second.reference(Route::Cancel, principal(8)?, 2, &second_key)?;
    assert_eq!(
        transport
            .post(CANCEL_PATH, &cancel)
            .expect("customer two cancellation"),
        second_ack
    );
    let store = JobStore::open(&transport.jobs())?;
    assert_eq!(store.job(first.key()?)?.state.state, JobState::Accepted);
    assert_eq!(
        store.job(second.key()?)?.state.state,
        JobState::Ended(Outcome::CancelledBeforeStart)
    );
    assert_eq!(
        transport
            .post(
                CANCEL_PATH,
                &first.reference(Route::Cancel, principal(9)?, 2, &customer())?
            )
            .expect("customer one cancellation"),
        first_ack
    );
    Ok(())
}

#[test]
fn worker_delegate_value_rotation_recovers_old_job_completion_under_original_identity() -> Checked {
    let (mut transport, mut fixture) = TlsWorker::new("rotation")?;
    let old = order(&fixture, 0x8A, 1, 512)?;
    let expiring = order(&fixture, 0x8B, 2, 512)?;
    admit_order(&mut fixture, &old, 0x8A)?;
    admit_order(&mut fixture, &expiring, 0x8B)?;
    transport.stage(&old)?;
    transport.stage(&expiring)?;
    transport.observe(&fixture.market, HEIGHT)?;
    transport.start(&fixture.authority)?;
    let ack = transport
        .post(SUBMIT_PATH, &old.signed)
        .expect("old admission");
    transport
        .post(SUBMIT_PATH, &expiring.signed)
        .expect("second old admission");
    let rotated = SigningKey::from_bytes(&[0xD2; 32]);
    let mut replacement = manifest(1, vec![capability(0x31)], vec![transport.endpoint.clone()])?;
    replacement.generation = 2;
    replacement.key_version = 2;
    let (_, digest) = Manifest::decode(&replacement.encode()?)?;
    let record = *fixture.authority.record();
    let header = fixture.market.world.header()?;
    let consent = layerx_programs_ai_market::workers::consent_digest(
        &header,
        record.worker,
        record.owner,
        public(&rotated),
        2,
        2,
        digest,
        EXPIRY,
    )?;
    let mut payload = record.worker.as_bytes().to_vec();
    payload.extend_from_slice(&1u64.to_be_bytes());
    payload.extend_from_slice(&1u64.to_be_bytes());
    payload.extend_from_slice(&public(&rotated).0);
    payload.extend_from_slice(digest.as_bytes());
    payload.extend_from_slice(&EXPIRY.to_be_bytes());
    payload.extend_from_slice(&rotated.sign(consent.as_bytes()).to_bytes());
    let call = Call {
        operation: dispatch::RotateDelegate,
        actor: record.owner,
        epoch: 1,
        config: fixture.authority.config(),
        roster: fixture.authority.roster(),
        sequence: 1,
        request: 0xE7,
        payload: &payload,
    };
    apply_chain(
        &mut fixture.market,
        &call.encode()?,
        record.owner,
        HEIGHT + 1,
    )?;
    let mut context = metadata_context()?;
    context.delegate = public(&rotated);
    let bytes = replacement.encode()?;
    fixture.market.signed = sign_metadata(&context, &publication(&bytes, 0)?, &rotated)?;
    fixture.authority = authority_at(&fixture.market, HEIGHT + 1)?;
    assert_ne!(
        public(&delegate()),
        fixture.authority.worker_binding().delegate
    );
    fs::write(
        transport.directory.0.join("delegate.hex"),
        [0xD2u8; 32]
            .iter()
            .map(|n| format!("{n:02x}"))
            .collect::<String>(),
    )?;
    fs::write(
        transport.directory.0.join("metadata.bin"),
        &fixture.market.signed,
    )?;
    transport.observe(&fixture.market, HEIGHT + 1)?;
    transport.wait_current(&fixture.authority);
    // Restart the actual worker with a different configured secret, not just new versions.
    transport.stop();
    fs::remove_dir_all(transport.directory.0.join("inputs"))?;
    transport.start(&fixture.authority)?;
    assert_eq!(
        transport
            .post(SUBMIT_PATH, &old.signed)
            .expect("old-generation replay"),
        ack
    );
    assert_eq!(
        transport
            .post(
                CANCEL_PATH,
                &old.reference(Route::Cancel, principal(9)?, 10, &customer())?
            )
            .expect("old-generation completion after restart"),
        ack
    );
    let saved = transport
        .post(
            QUERY_PATH,
            &old.reference(Route::Query, principal(9)?, 11, &customer())?,
        )
        .expect("old result retrieval");
    let (result, _) = decode_result(&saved, public(&delegate()))?;
    assert_eq!(
        (result.generation, result.key_version, result.outcome),
        (1, 1, Outcome::CancelledBeforeStart)
    );
    assert_eq!(
        decode_result(&saved, public(&rotated)),
        Err(ServiceError::BadSignature)
    );
    transport.stop();
    let reopened = service_jobs(&transport, rotated)?;
    let job = reopened.store().job(expiring.key()?)?;
    assert_eq!(
        reopened.expire(
            &fixture.authority,
            job.record.accepted_ms + u64::from(job.record.admission.deadline_ms)
        )?,
        1
    );
    let result = reopened
        .store()
        .job(expiring.key()?)?
        .result
        .expect("old admission settled");
    assert_eq!(
        decode_result(&result, public(&delegate()))?.0.outcome,
        Outcome::ExpiredBeforeStart
    );
    let private = fs::metadata(transport.jobs().join("delegates"))?;
    assert_eq!(private.permissions().mode() & 0o077, 0);
    for entry in fs::read_dir(transport.jobs().join("delegates"))? {
        assert_eq!(entry?.metadata()?.permissions().mode() & 0o077, 0);
    }
    Ok(())
}
