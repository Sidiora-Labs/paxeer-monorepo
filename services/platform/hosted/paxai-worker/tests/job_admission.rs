//! Signed worker admission against a real opened market and its finalized task records.
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
        let path =
            std::env::temp_dir().join(format!("paxai-job-recovery-{}-{name}", std::process::id()));
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

#[test]
fn job_admitted_after_finalized_observation() -> Checked {
    let directory = Scratch::new("admission-observation")?;
    let service = worker_service(&directory)?;
    let mut fixture = admitting()?;
    let order = order(&fixture, 0x41, 1, 128)?;
    admit_chain_task(&mut fixture, &order, 0x41).expect("native task admission");
    stage_input(&service, &order)?;
    let before = service.dispatch(
        "POST",
        layerx_paxai_worker::auth::SUBMIT_PATH,
        &order.signed,
    );
    assert_eq!(before.status, 503);
    assert_eq!(
        before.body,
        ServiceError::StaleAuthority.body(Presence::Present(order.submit.context.request))
    );
    assert_eq!(service.jobs.store().usage()?.queued, 0);
    feed(&service, &fixture.market, HEIGHT).expect("bind finalized observation");
    let response = service.dispatch(
        "POST",
        layerx_paxai_worker::auth::SUBMIT_PATH,
        &order.signed,
    );
    assert_eq!(response.status, 200);
    let (ack, envelope) = decode_acknowledgment(&response.body, public(&delegate()))?;
    assert_eq!(ack.task, order.submit.request.task);
    assert_eq!(envelope.context.request, order.submit.context.request);
    let saved = service.jobs.store().job(order.key()?)?;
    assert_eq!(response.body, saved.record.acknowledgment);
    assert_eq!(saved.state.state, JobState::Accepted);
    assert_eq!(service.jobs.runner().invocations(), 0);
    let duplicate = service.dispatch(
        "POST",
        layerx_paxai_worker::auth::SUBMIT_PATH,
        &order.signed,
    );
    assert_eq!(duplicate.status, 200);
    assert_eq!(duplicate.body, response.body);
    assert_eq!(service.jobs.store().usage()?.queued, 1);
    service.runtime.lock().expect("runtime lock").refreshed = Some(
        std::time::Instant::now()
            - Duration::from_millis(layerx_paxai_worker::discovery::READINESS_TTL_MS),
    );
    assert_eq!(
        service
            .dispatch(
                "POST",
                layerx_paxai_worker::auth::SUBMIT_PATH,
                &order.signed
            )
            .status,
        503
    );
    Ok(())
}

#[test]
fn job_refused_after_membership_revoked() -> Checked {
    let directory = Scratch::new("admission-revoked")?;
    let service = worker_service(&directory)?;
    let mut fixture = admitting()?;
    let order = order(&fixture, 0x42, 1, 128)?;
    admit_chain_task(&mut fixture, &order, 0x42).expect("native task admission");
    stage_input(&service, &order)?;
    feed(&service, &fixture.market, HEIGHT).expect("bind finalized observation");
    assert!(service
        .runtime
        .lock()
        .expect("runtime lock")
        .membership
        .new_work()
        .is_ok());
    fixture.market.world.edit(|parts, market| {
        parts.admission.administrative_remove(
            &admission_context(market, market.owner_principal, HEIGHT + 1),
            Participant::Worker(service.binding.worker),
            1,
            layerx_programs_ai_market::admission::RemovalReason::Security,
        )?;
        let record = parts
            .workers
            .get(service.binding.worker)
            .ok_or(NON_CANONICAL)?;
        parts.workers.replace(&WorkerCurrent {
            state: WorkerState::Revoked,
            ..record
        })
    })?;
    feed(&service, &fixture.market, HEIGHT + 1)?;
    let error = service
        .runtime
        .lock()
        .expect("runtime lock")
        .membership
        .new_work()
        .expect_err("revoked membership refused");
    assert_eq!(error, ServiceError::DelegateRevoked);
    let response = service.dispatch(
        "POST",
        layerx_paxai_worker::auth::SUBMIT_PATH,
        &order.signed,
    );
    assert_eq!(response.status, 400);
    assert_eq!(
        response.body,
        error.body(Presence::Present(order.submit.context.request))
    );
    assert_eq!(service.jobs.store().usage()?.queued, 0);
    assert_eq!(service.jobs.runner().invocations(), 0);
    Ok(())
}

#[test]
fn program_operations_preserve_delegate_and_abi_bindings() -> Checked {
    let directory = Scratch::new("program-operations")?;
    let service = worker_service(&directory)?;
    let fixture = admitting()?;
    let config: worker::Config = serde_json::from_value(serde_json::json!({
        "listen": "127.0.0.1:0", "certificate_chain": "certificate.pem", "private_key": "tls.key",
        "delegate_seed": "delegate.key", "signed_metadata": "metadata", "chain": "", "program": "",
        "market": "", "worker": "", "owner": "", "state_dir": directory.0,
        "input_dir": service.input_dir, "runner": env!("CARGO_BIN_EXE_layerx-paxai-runner"),
        "authority_reader": "authority-reader", "sponsored_sender": "sponsored-sender",
        "chain_config": "chain.json", "capabilities": [1, 2], "access_declaration": [3, 4],
        "response_capacity": 16384, "budget": [1, 2, 3, 4, 5, 6, 7], "fee_limit": 100
    }))
    .expect("valid call configuration");
    for operation in [
        dispatch::ACCEPT_TASK,
        dispatch::COMMIT_TASK_RESULT,
        dispatch::Heartbeat,
    ] {
        let payload = vec![0x51; 72];
        let pending =
            worker::signed_operation(&service, &fixture.authority, operation, 3, &payload)?;
        let decoded = codec::decode_envelope(&pending.envelope)?;
        assert_eq!(decoded.envelope.operation, operation);
        assert_eq!(decoded.envelope.actor, service.binding.owner);
        assert_eq!(decoded.envelope.sequence, 3);
        assert_eq!(decoded.envelope.request.bytes(), pending.request);
        assert_eq!(decoded.envelope.payload, payload);
        let Authentication::Delegate { key, signature } = decoded.envelope.authentication else {
            return Err(Failure::Unexpected("missing delegate authentication"));
        };
        assert_eq!(key, public(&delegate()));
        delegate()
            .verifying_key()
            .verify_strict(
                decoded.request_digest()?.as_bytes(),
                &ed25519_dalek::Signature::from_bytes(&signature.0),
            )
            .expect("genuine delegate signature");
        let call = worker::programs_call(&config, service.binding.program, &pending.envelope);
        assert_eq!(&call[..4], &[0x54, 0xbd, 0x2b, 0x48]);
        assert_eq!(&call[4..36], service.binding.program.as_bytes());
        let offset = usize::try_from(u128::from_be_bytes(
            call[116..132].try_into().expect("ABI offset"),
        ))
        .expect("offset fits");
        let length = usize::try_from(u128::from_be_bytes(
            call[4 + offset + 16..4 + offset + 32]
                .try_into()
                .expect("ABI length"),
        ))
        .expect("length fits");
        assert_eq!(
            &call[4 + offset + 32..4 + offset + 32 + length],
            pending.envelope
        );
    }
    Ok(())
}
