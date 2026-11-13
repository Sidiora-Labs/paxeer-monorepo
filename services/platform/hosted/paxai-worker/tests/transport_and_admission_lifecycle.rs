//! Audit 22.30.84 medium repairs through pinned TLS, durable admissions and real processes.
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

fn hex_bytes(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut text = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        text.push(char::from(DIGITS[usize::from(byte >> 4)]));
        text.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    text
}

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
const EXPIRY: u64 = 1180;
const VALID_FROM: u64 = EXPIRY - layerx_programs_ai_market::workers::DEFAULT_PROPOSAL_LIFETIME;
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

/// Only native dispatch writes market state; readers never reconstruct its sections.
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
    fn revision(&self) -> CodecResult<u64> {
        Ok(decode_shared_state(&self.bytes)?.revision)
    }
    fn header(&self) -> CodecResult<MarketHeader> {
        let state = decode_shared_state(&self.bytes)?;
        Ok(PolicySection::decode(state.section(Section::PolicyLifecycle)?)?.header)
    }
    fn apply(&mut self, call: &Call<'_>, at: u64) -> CodecResult<()> {
        let mut next = vec![0; MAX_STATE_BYTES];
        let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
        let mut event = vec![0; MAX_EVENT_BYTES];
        let mut result = vec![0; layerx_programs_ai_market::MAX_RESULT_BYTES];
        let routed = dispatch::route(
            &call.context(at)?,
            &call.encode()?,
            Some(&self.bytes),
            dispatch::Buffers {
                next: &mut next,
                scratch: &mut scratch,
                event: &mut event,
                result: &mut result,
            },
        )?;
        let dispatch::Routed::Applied { state_len, .. } = routed else {
            let (dispatch::Routed::Refused { result_len }
            | dispatch::Routed::Unchanged { result_len }) = routed
            else {
                unreachable!();
            };
            panic!(
                "native {:?} at {at} refused: {:?}",
                call.operation,
                codec::decode_result(&result[..result_len])
            );
        };
        next.truncate(state_len);
        self.bytes = next;
        Ok(())
    }
    fn owner_op(&mut self, operation: Operation, payload: &[u8], at: u64) -> CodecResult<()> {
        self.apply(
            &Call {
                operation,
                actor: PrincipalId::new(OWNER)?,
                epoch: 0,
                config: self.header()?.active_config_version,
                roster: Presence::Absent,
                sequence: self.owner_sequence,
                request: 0x20 + u8::try_from(self.owner_sequence).map_err(|_| ARITHMETIC)?,
                payload,
            },
            at,
        )?;
        self.owner_sequence += 1;
        Ok(())
    }
    fn participant_op(
        &mut self,
        operation: Operation,
        actor: PrincipalId,
        sequence: u64,
        request: u8,
        payload: &[u8],
        at: u64,
    ) -> CodecResult<()> {
        self.apply(
            &Call {
                operation,
                actor,
                epoch: 0,
                config: self.header()?.active_config_version,
                roster: Presence::Absent,
                sequence,
                request,
                payload,
            },
            at,
        )
    }
    fn schedule(&mut self, epoch: u64, at: u64) -> CodecResult<()> {
        let mut payload = self.revision()?.to_be_bytes().to_vec();
        payload.extend_from_slice(&epoch.to_be_bytes());
        self.owner_op(dispatch::SCHEDULE_ACTIVATION, &payload, at)
    }
    fn approve(
        &mut self,
        participant: Participant,
        owner: PrincipalId,
        delegate: PublicKey32,
        identity: Digest32,
        at: u64,
    ) -> CodecResult<ApprovalTerms> {
        let market = self.header()?;
        let terms = ApprovalTerms {
            participant,
            owner,
            delegate,
            delegate_generation: 1,
            enrollment_nonce_commitment: Digest32::new([7; 32])?,
            identity_commitment: identity,
            effective_epoch: 0,
            config_version: market.active_config_version,
            request: RequestId::new(
                [0x20 + u8::try_from(self.owner_sequence).map_err(|_| ARITHMETIC)?; 32],
            )?,
            expiry_height: epoch_height(market.origin_height, 0, 64),
        };
        let mut payload = vec![participant.role() as u8];
        payload.extend_from_slice(&participant.bytes());
        payload.extend_from_slice(owner.as_bytes());
        payload.extend_from_slice(terms.enrollment_nonce_commitment.as_bytes());
        payload.extend_from_slice(&delegate.0);
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(identity.as_bytes());
        payload.extend_from_slice(&terms.effective_epoch.to_be_bytes());
        payload.extend_from_slice(&terms.config_version.to_be_bytes());
        payload.extend_from_slice(&terms.expiry_height.to_be_bytes());
        self.owner_op(dispatch::ApproveAdmission, &payload, at)?;
        Ok(terms)
    }
    fn enroll(
        &mut self,
        metadata: layerx_programs_ai_market::types::MetadataDigest,
        at: u64,
    ) -> CodecResult<WorkerRosterEntry> {
        let (_, owner, worker) = identities()?;
        let mut payload = owner.bytes().to_vec();
        payload.extend_from_slice(&[1; 32]);
        payload.extend_from_slice(&public(&delegate()).0);
        payload.extend_from_slice(metadata.as_bytes());
        payload.extend_from_slice(&EXPIRY.to_be_bytes());
        self.owner_op(dispatch::EnrollWorker, &payload, at)?;
        let header = self.header()?;
        let consent = layerx_programs_ai_market::workers::consent_digest(
            &header,
            worker,
            owner,
            public(&delegate()),
            1,
            1,
            metadata,
            EXPIRY,
        )?;
        let mut payload = worker.bytes().to_vec();
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(metadata.as_bytes());
        payload.extend_from_slice(&delegate().sign(consent.as_bytes()).to_bytes());
        self.participant_op(dispatch::AcceptEnrollment, owner, 1, 0x51, &payload, at)?;
        let state = decode_shared_state(&self.bytes)?;
        let workers = split_identity_section(state.section(Section::IdentityRoster)?)?.0;
        let record = WorkerTable::decode(workers)?
            .get(worker)
            .ok_or(NON_CANONICAL)?;
        let terms = self.approve(
            Participant::Worker(worker),
            owner,
            public(&delegate()),
            record.proposal_digest(&header)?,
            at,
        )?;
        let mut payload = worker.bytes().to_vec();
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(&terms.effective_epoch.to_be_bytes());
        payload.extend_from_slice(&terms.config_version.to_be_bytes());
        payload.extend_from_slice(terms.digest(&header)?.as_bytes());
        self.participant_op(dispatch::AdmitWorker, owner, 2, 0x52, &payload, at)?;
        Ok(WorkerRosterEntry {
            worker,
            owner,
            recipient: AccountId::new(owner.bytes())?,
            generation: version()?,
            key_version: version()?,
            public_key: public(&delegate()),
            metadata,
        })
    }
    fn evaluator(&mut self, n: u8, at: u64) -> CodecResult<EvaluatorRosterEntry> {
        let owner = principal(n)?;
        let key = SigningKey::from_bytes(&[n; 32]);
        let signing_key = public(&key);
        let header = self.header()?;
        let mut payload = owner.bytes().to_vec();
        payload.extend_from_slice(&[n; 32]);
        payload.extend_from_slice(rubric()?.as_bytes());
        payload.extend_from_slice(&signing_key.0);
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(&0u64.to_be_bytes());
        payload.extend_from_slice(&32u64.to_be_bytes());
        self.owner_op(dispatch::ScheduleEvaluator, &payload, at)?;
        let state = decode_shared_state(&self.bytes)?;
        let region = split_identity_section(state.section(Section::IdentityRoster)?)?.1;
        let grant = EvaluatorRegion::decode(region)?.stored_grant(derive_evaluator(
            header.market_id,
            owner,
            [n; 32],
        )?)?;
        let mut encoded = [0; layerx_programs_ai_market::evaluators::model::GRANT_BYTES];
        let len = layerx_programs_ai_market::evaluators::codec::encode_grant(&grant, &mut encoded)?;
        let identity = codec::domain_hash("PAXAI/evaluator-grant/v1", &encoded[..len])?;
        let terms = self.approve(
            Participant::Evaluator(grant.evaluator),
            owner,
            signing_key,
            identity,
            at,
        )?;
        let consent = EvaluatorConsent {
            chain: header.deployment_chain_domain,
            program: header.program_id,
            market: header.market_id,
            evaluator: grant.evaluator,
            owner,
            signing_key,
            enrollment_nonce: [n; 32],
            rubric: grant.rubric,
            approval_digest: terms.digest(&header)?,
            request: RequestId::new([n; 32])?,
            grant_version: 1,
            key_version: 1,
            effective_epoch: terms.effective_epoch,
            config_version: terms.config_version,
            expiry_height: terms.expiry_height,
        };
        let mut encoded = [0; 362];
        consent.encode(&mut encoded)?;
        let mut payload = encoded.to_vec();
        payload.extend_from_slice(&key.sign(consent.digest()?.as_bytes()).to_bytes());
        self.participant_op(dispatch::AdmitEvaluator, owner, 1, n, &payload, at)?;
        Ok(EvaluatorRosterEntry {
            evaluator: grant.evaluator,
            owner,
            grant: grant.grant_version,
            key_version: grant.key_version,
            public_key: signing_key,
            rubric: grant.rubric,
        })
    }
    fn fund(&mut self, amount: u128, at: u64) -> CodecResult<()> {
        let mut payload = [0; layerx_programs_ai_market::rewards::FUND_PAYLOAD_BYTES];
        let len = FundRequest {
            amount,
            refund_recipient: self.header()?.refund_recipient_account,
            policy_version: FUNDING_POLICY_VERSION,
            consent: true,
        }
        .encode(&mut payload)?;
        self.owner_op(dispatch::FUND, &payload[..len], at)
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
    let (market, _, _) = identities()?;
    let manifest = manifest(1, vec![capability(0x31)], endpoints)?;
    let signed = sign(&manifest, 0)?;
    let (_, digest) = Manifest::decode(&manifest.encode()?)?;
    let mut world = World::create(ORIGIN)?;
    world.schedule(1, 1001)?;
    let entry = world.enroll(digest, VALID_FROM)?;
    let mut evaluators = Vec::new();
    for n in 2..=4u8 {
        evaluators.push(world.evaluator(n, VALID_FROM + u64::from(n))?);
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
            "paxai-repair-22-33-1464-{}-{name}",
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
        metrics: layerx_paxai_worker::metrics::Metrics::default(),
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
    service
        .runtime
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}: {}", "runtime lock"))
        .observe(&evidence, &market.signed, vec![customer_at(height)?])?;
    Ok(())
}

fn stage_input(service: &worker::Service, order: &Order) -> Checked {
    let name = hex_bytes(&order.submit.request.input_commitment.bytes());
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

use layerx_paxai_worker::auth::{StatusChallenge, STATUS_PATH, SUBMIT_PATH};
use layerx_paxai_worker::discovery::{spki_sha256, EndpointClient, NetworkProfile, TransportError};
use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer, ServerName};
use rustls::{
    ClientConfig, ClientConnection, RootCertStore, ServerConfig, ServerConnection, StreamOwned,
};
use std::io::{Read, Write};
use std::net::{IpAddr, Ipv4Addr, TcpListener, TcpStream};
use std::process::{Command, Stdio};
use std::sync::Arc;
use std::time::Instant;

struct Tls {
    roots: Arc<RootCertStore>,
    server: Arc<ServerConfig>,
    pin: [u8; 32],
}
impl Tls {
    fn new() -> Self {
        let key =
            rcgen::KeyPair::generate().unwrap_or_else(|error| panic!("{error:?}: {}", "CA key"));
        let mut params = rcgen::CertificateParams::new(Vec::<String>::new())
            .unwrap_or_else(|error| panic!("{error:?}: {}", "CA params"));
        params.is_ca = rcgen::IsCa::Ca(rcgen::BasicConstraints::Unconstrained);
        let ca = params
            .self_signed(&key)
            .unwrap_or_else(|error| panic!("{error:?}: {}", "CA certificate"));
        let issuer = rcgen::Issuer::from_params(&params, &key);
        let key =
            rcgen::KeyPair::generate().unwrap_or_else(|error| panic!("{error:?}: {}", "leaf key"));
        let leaf = rcgen::CertificateParams::new(vec!["localhost".into()])
            .unwrap_or_else(|error| panic!("{error:?}: {}", "leaf params"))
            .signed_by(&key, &issuer)
            .unwrap_or_else(|error| panic!("{error:?}: {}", "leaf certificate"));
        let mut roots = RootCertStore::empty();
        roots
            .add(ca.der().clone())
            .unwrap_or_else(|error| panic!("{error:?}: {}", "trust CA"));
        let server =
            ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
                .with_protocol_versions(&[&rustls::version::TLS13])
                .unwrap_or_else(|error| panic!("{error:?}: {}", "TLS 1.3"))
                .with_no_client_auth()
                .with_single_cert(
                    vec![leaf.der().clone()],
                    PrivateKeyDer::from(PrivatePkcs8KeyDer::from(key.serialize_der())),
                )
                .unwrap_or_else(|error| panic!("{error:?}: {}", "server config"));
        Self {
            roots: Arc::new(roots),
            server: Arc::new(server),
            pin: spki_sha256(leaf.der()).unwrap_or_else(|error| panic!("{error:?}: {}", "SPKI")),
        }
    }
    fn client(&self, timeout: Duration) -> EndpointClient {
        EndpointClient {
            roots: self.roots.clone(),
            profile: NetworkProfile::Private {
                allowlist: vec![IpAddr::V4(Ipv4Addr::LOCALHOST)],
            },
            timeout,
        }
    }
    fn endpoint(&self, listener: &TcpListener) -> Endpoint {
        endpoint(
            &format!(
                "https://localhost:{}/paxai/v1",
                listener
                    .local_addr()
                    .unwrap_or_else(|error| panic!("{error:?}: {}", "address"))
                    .port()
            ),
            self.pin,
        )
    }
    fn connect(&self, listener: &TcpListener) -> StreamOwned<ClientConnection, TcpStream> {
        let config =
            ClientConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
                .with_protocol_versions(&[&rustls::version::TLS13])
                .unwrap_or_else(|error| panic!("{error:?}: {}", "TLS 1.3"))
                .with_root_certificates((*self.roots).clone())
                .with_no_client_auth();
        let conn = ClientConnection::new(
            Arc::new(config),
            ServerName::try_from("localhost")
                .unwrap_or_else(|error| panic!("{error:?}: {}", "server name")),
        )
        .unwrap_or_else(|error| panic!("{error:?}: {}", "client connection"));
        let socket = TcpStream::connect(
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("{error:?}: {}", "address")),
        )
        .unwrap_or_else(|error| panic!("{error:?}: {}", "connect"));
        socket
            .set_read_timeout(Some(Duration::from_secs(3)))
            .unwrap_or_else(|error| panic!("{error:?}: {}", "read timeout"));
        socket
            .set_write_timeout(Some(Duration::from_secs(3)))
            .unwrap_or_else(|error| panic!("{error:?}: {}", "write timeout"));
        StreamOwned::new(conn, socket)
    }
}

fn loopback(host: &str, _: u16) -> io::Result<Vec<IpAddr>> {
    if host != "localhost" {
        return Err(io::Error::from(io::ErrorKind::InvalidInput));
    }
    Ok(vec![IpAddr::V4(Ipv4Addr::LOCALHOST)])
}

fn receive_request(stream: &mut impl Read) -> Vec<u8> {
    let mut received = Vec::new();
    let mut byte = [0];
    while !received.ends_with(b"\r\n\r\n") {
        stream
            .read_exact(&mut byte)
            .unwrap_or_else(|error| panic!("{error:?}: {}", "request head"));
        received.push(byte[0]);
    }
    let text =
        std::str::from_utf8(&received).unwrap_or_else(|error| panic!("{error:?}: {}", "HTTP head"));
    let length = text
        .split("\r\n")
        .find_map(|line| line.strip_prefix("Content-Length: "))
        .unwrap_or_else(|| panic!("request length"))
        .parse::<usize>()
        .unwrap_or_else(|error| panic!("{error:?}: {}", "decimal length"));
    let start = received.len();
    received.resize(start + length, 0);
    stream
        .read_exact(&mut received[start..])
        .unwrap_or_else(|error| panic!("{error:?}: {}", "request body"));
    received
}

#[test]
fn worker_request_target_injection_sends_no_bytes() {
    let tls = Tls::new();
    let listener = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("{error:?}: {}", "listener"));
    listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("{error:?}: {}", "nonblocking listener"));
    let endpoint = tls.endpoint(&listener);
    for path in [
        "/paxai/v1/status\r\nInjected: yes",
        "/paxai/v1/status extra",
        "/paxai/v1/status\t",
        "/paxai/v1/../status",
        "/paxai/v1/%73tatus",
        "/paxai/v1/status?x=1",
        "/paxai/v1/unknown",
    ] {
        let result = tls
            .client(Duration::from_secs(1))
            .post(&endpoint, path, b"signed", |_, _| {
                panic!("invalid target reached resolution")
            });
        assert_eq!(result, Err(TransportError::UnsafeEndpoint), "{path:?}");
        assert_eq!(
            listener
                .accept()
                .err()
                .unwrap_or_else(|| panic!("invalid target connected"))
                .kind(),
            io::ErrorKind::WouldBlock
        );
    }
}

fn transcript_response(
    response: Vec<u8>,
    delay: Option<Duration>,
) -> Result<Vec<u8>, TransportError> {
    let tls = Tls::new();
    let listener = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("{error:?}: {}", "listener"));
    let endpoint = tls.endpoint(&listener);
    let config = tls.server.clone();
    let peer = thread::spawn(move || {
        let (socket, _) = listener
            .accept()
            .unwrap_or_else(|error| panic!("{error:?}: {}", "accept TLS peer"));
        socket
            .set_read_timeout(Some(Duration::from_secs(2)))
            .unwrap_or_else(|error| panic!("{error:?}: {}", "read timeout"));
        socket
            .set_write_timeout(Some(Duration::from_secs(2)))
            .unwrap_or_else(|error| panic!("{error:?}: {}", "write timeout"));
        let mut stream = StreamOwned::new(
            ServerConnection::new(config)
                .unwrap_or_else(|error| panic!("{error:?}: {}", "server connection")),
            socket,
        );
        let received = receive_request(&mut stream);
        assert!(received.starts_with(b"POST /paxai/v1/status HTTP/1.1\r\n"));
        assert!(received.ends_with(b"signed"));
        for byte in response {
            if let Some(delay) = delay {
                thread::sleep(delay);
            }
            if stream
                .write_all(&[byte])
                .and_then(|()| stream.flush())
                .is_err()
            {
                break;
            }
        }
        stream.conn.send_close_notify();
        let _ = stream.flush();
    });
    let result =
        tls.client(Duration::from_millis(350))
            .post(&endpoint, STATUS_PATH, b"signed", loopback);
    peer.join()
        .unwrap_or_else(|error| panic!("{error:?}: {}", "peer thread"));
    result
}

#[test]
fn worker_client_framing_rejects_ambiguous_and_truncated_tls_responses() {
    for response in [
        "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nContent-Length: 2\r\n\r\nok",
        "HTTP/1.1 200 OK\r\nContent-Length: 8\r\ncontent-length: 2\r\n\r\nok",
        "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\nContent-Length: 2\r\n\r\nok",
        "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n",
        "HTTP/1.1 200 OK\r\n\r\n",
        "HTTP/1.1 200 OK\r\nContent-Length: 8\r\n\r\nok",
        "HTTP/1.1 200junk OK\r\nContent-Length: 2\r\n\r\nok",
        "HTTP/1.1 200\r\nContent-Length: 2\r\n\r\nok",
        "HTTP/1.1 099 OK\r\nContent-Length: 2\r\n\r\nok",
    ] {
        assert_eq!(
            transcript_response(response.as_bytes().to_vec(), None),
            Err(TransportError::Malformed),
            "{response:?}"
        );
    }
    assert_eq!(
        transcript_response(
            b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok".to_vec(),
            None
        ),
        Ok(b"ok".to_vec())
    );
}

#[test]
fn worker_client_total_deadline_bounds_a_peer_making_read_progress() {
    let started = Instant::now();
    assert_eq!(
        transcript_response(
            b"HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n".to_vec(),
            Some(Duration::from_millis(70))
        ),
        Err(TransportError::Unavailable)
    );
    assert!(
        started.elapsed() < Duration::from_secs(2),
        "trickling reads escaped deadline"
    );
}

fn assert_probe_framing(
    tls: &Tls,
    service: &Arc<worker::Service>,
    signed_probe: &[u8],
    authority: &FinalizedAuthority,
    challenge: &StatusChallenge,
) -> Checked {
    for head in [
        "POST /paxai/v1/status HTTP/1.1 extra\r\nContent-Length: 32\r\n\r\n",
        "POST /paxai/v1/status HTTP/1.1\r\nContent-Length: 32\r\nContent-Length: 1\r\n\r\n",
        "POST /paxai/v1/status HTTP/1.1\r\nTransfer-Encoding: chunked\r\nContent-Length: 32\r\n\r\n",
        "POST /paxai/v1/status HTTP/1.1\r\n\r\n",
    ] {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let mut stream = tls.connect(&listener);
        let config = tls.server.clone(); let service = service.clone();
        let server = thread::spawn(move || {
            let (socket, _) = listener.accept().unwrap_or_else(|error| panic!("{error:?}: {}", "accept"));
            worker::serve(&service, config, socket, Instant::now() + Duration::from_secs(2))
        });
        stream.write_all(head.as_bytes())?; stream.write_all(signed_probe)?; stream.flush()?;
        let mut bytes = Vec::new(); stream.read_to_end(&mut bytes)?;
        assert!(bytes.starts_with(b"HTTP/1.1 400 Bad Request\r\n"));
        assert!(bytes.ends_with(&ServiceError::NonCanonical.body(Presence::Absent)));
        drop(stream); server.join().unwrap_or_else(|error| panic!("{error:?}: {}", "server thread"))?;
    }
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let endpoint = tls.endpoint(&listener);
    let config = tls.server.clone();
    let service2 = service.clone();
    let server = thread::spawn(move || {
        let (socket, _) = listener
            .accept()
            .unwrap_or_else(|error| panic!("{error:?}: {}", "accept"));
        worker::serve(
            &service2,
            config,
            socket,
            Instant::now() + Duration::from_secs(2),
        )
    });
    let signed = tls
        .client(Duration::from_secs(2))
        .post(&endpoint, STATUS_PATH, signed_probe, loopback)
        .unwrap_or_else(|error| panic!("{error:?}: {}", "valid probe through strict TLS listener"));
    layerx_paxai_worker::auth::verify_status(&signed, &authority.worker_binding(), challenge)?;
    server
        .join()
        .unwrap_or_else(|error| panic!("{error:?}: {}", "server thread"))?;
    Ok(())
}

fn assert_reopened_admission(
    service: Arc<worker::Service>,
    directory: &Scratch,
    fixture: &Admitting,
    tls: &Tls,
    order: &Order,
    acknowledgment: &[u8],
) -> Checked {
    drop(service);
    let reopened = Arc::new(worker_service(directory)?);
    feed(&reopened, &fixture.market, HEIGHT)?;
    assert_eq!(
        reopened
            .jobs
            .store()
            .job(order.key()?)?
            .record
            .acknowledgment,
        acknowledgment.to_vec()
    );
    let query = order.reference(Route::Query, principal(9)?, 2, &customer())?;
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let endpoint = tls.endpoint(&listener);
    let config = tls.server.clone();
    let server = thread::spawn(move || {
        let (socket, _) = listener
            .accept()
            .unwrap_or_else(|error| panic!("{error:?}: {}", "accept reopened service"));
        worker::serve(
            &reopened,
            config,
            socket,
            Instant::now() + Duration::from_secs(2),
        )
    });
    let recovered = tls
        .client(Duration::from_secs(2))
        .post(
            &endpoint,
            layerx_paxai_worker::auth::QUERY_PATH,
            &query,
            loopback,
        )
        .unwrap_or_else(|error| {
            panic!(
                "{error:?}: {}",
                "query writer-produced admission through reopened TLS listener"
            )
        });
    assert_eq!(recovered.as_slice(), acknowledgment);
    server
        .join()
        .unwrap_or_else(|error| panic!("{error:?}: {}", "reopened server thread"))?;
    Ok(())
}

#[test]
fn worker_server_framing_rejects_conflicts_and_accepts_a_signed_tls_probe() -> Checked {
    let tls = Tls::new();
    let directory = Scratch::new("server-framing")?;
    let service = Arc::new(worker_service(&directory)?);
    let mut fixture = admitting()?;
    let order = order(&fixture, 0xA1, 1, 512)?;
    admit_chain_task(&mut fixture, &order, 0xA1)?;
    stage_input(&service, &order)?;
    feed(&service, &fixture.market, HEIGHT)?;
    let challenge = StatusChallenge {
        epoch: fixture
            .authority
            .current_epoch()
            .unwrap_or_else(|| panic!("epoch")),
        config: fixture.authority.config(),
        roster: fixture.authority.roster(),
        expiry: TASK_EXPIRY,
        challenge: RequestId::new([0x66; 32])?,
    };
    let signed_probe = challenge.encode();
    assert_probe_framing(
        &tls,
        &service,
        &signed_probe,
        &fixture.authority,
        &challenge,
    )?;
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let endpoint = tls.endpoint(&listener);
    let config = tls.server.clone();
    let service2 = service.clone();
    let server = thread::spawn(move || {
        let (socket, _) = listener
            .accept()
            .unwrap_or_else(|error| panic!("{error:?}: {}", "accept"));
        worker::serve(
            &service2,
            config,
            socket,
            Instant::now() + Duration::from_secs(2),
        )
    });
    let acknowledgment = tls
        .client(Duration::from_secs(2))
        .post(&endpoint, SUBMIT_PATH, &order.signed, loopback)
        .unwrap_or_else(|error| {
            panic!(
                "{error:?}: {}",
                "valid signed submit through strict TLS listener"
            )
        });
    let (admitted, _) = decode_acknowledgment(&acknowledgment, public(&delegate()))?;
    assert_eq!(admitted.task, order.submit.request.task);
    assert_eq!(
        service
            .jobs
            .store()
            .job(order.key()?)?
            .record
            .acknowledgment,
        acknowledgment
    );
    server
        .join()
        .unwrap_or_else(|error| panic!("{error:?}: {}", "server thread"))?;
    assert_reopened_admission(service, &directory, &fixture, &tls, &order, &acknowledgment)?;
    Ok(())
}

fn peer_args(mode: &str) -> Vec<std::ffi::OsString> {
    [
        "--exact",
        "worker_process_peer",
        "--nocapture",
        &format!("--skip=mode={mode}"),
        "--skip",
    ]
    .into_iter()
    .map(Into::into)
    .collect()
}

/// Committed process peer: spawn the same compiled test executable, never a shell or toolchain.
#[test]
fn worker_process_peer() {
    let mode = std::env::args().find_map(|arg| arg.strip_prefix("--skip=mode=").map(str::to_owned));
    let Some(mode) = mode else {
        assert!(std::env::current_exe()
            .unwrap_or_else(|error| panic!("{error:?}: {}", "test executable"))
            .is_file());
        return;
    };
    if mode == "descendant" {
        thread::sleep(Duration::from_secs(4));
        std::process::exit(0);
    }
    let args = std::env::args().collect::<Vec<_>>();
    if args.last().is_some_and(|arg| arg == "describe") {
        // This peer provides no runner idempotency/fencing guarantee.
        std::process::exit(1);
    }
    let mut input = Vec::new();
    io::stdin()
        .read_to_end(&mut input)
        .unwrap_or_else(|error| panic!("{error:?}: {}", "peer request"));
    let descendant = Command::new(
        std::env::current_exe().unwrap_or_else(|error| panic!("{error:?}: {}", "test executable")),
    )
    .args(peer_args("descendant"))
    .arg("unused")
    .stdin(Stdio::null())
    .stdout(Stdio::inherit())
    .stderr(Stdio::null())
    .spawn()
    .unwrap_or_else(|error| panic!("{error:?}: {}", "descendant retaining stdout"));
    if mode == "retain-exit" {
        std::process::exit(0);
    }
    assert_eq!(mode, "retain-timeout");
    let mut descendant = descendant;
    let _ = descendant.wait();
    std::process::exit(0);
}

#[test]
fn worker_adapter_deadline_includes_descendant_pipe_completion() {
    for mode in ["retain-exit", "retain-timeout"] {
        let mut command = Command::new(
            std::env::current_exe()
                .unwrap_or_else(|error| panic!("{error:?}: {}", "test executable")),
        );
        command.args(peer_args(mode)).arg("adapter");
        let started = Instant::now();
        assert_eq!(
            worker::adapter_call(
                &mut command,
                &serde_json::json!({"request": 1}),
                Duration::from_millis(200)
            ),
            Err(ServiceError::ExecutionUnavailable)
        );
        assert!(
            started.elapsed() < Duration::from_secs(2),
            "adapter retained a pipe after deadline"
        );
    }
}

#[test]
fn worker_runner_deadline_bounds_retained_pipes_and_preserves_ambiguity() -> Checked {
    for mode in ["retain-exit", "retain-timeout"] {
        let runner = ProcessRunner::new(
            std::env::current_exe()?,
            peer_args(mode),
            Duration::from_millis(200),
        );
        let started = Instant::now();
        assert_eq!(
            runner.lookup(JobKey::from_digest(Digest32::new([0x77; 32])?)),
            Err(layerx_paxai_worker::runner::RunnerError::Ambiguous)
        );
        assert!(started.elapsed() < Duration::from_secs(2));
        assert_eq!(runner.invocations(), 1);
    }
    Ok(())
}

fn assert_legacy_reopen(
    directory: &Scratch,
    fixture: &Admitting,
    service: worker::Service,
    orders: &[Order],
    lease1: &Lease,
    future: u64,
) -> Checked {
    // Legacy tracking can contain unvalidated keys and expired fences from an earlier worker.
    fs::write(
        directory.0.join("chain-jobs.json"),
        serde_json::to_vec(&vec![[0xFFu8; 32]])
            .unwrap_or_else(|error| panic!("{error:?}: {}", "legacy tracking JSON")),
    )?;
    fs::write(
        directory.0.join("expired-attempts.json"),
        serde_json::to_vec(&vec![(lease1.key.digest().bytes(), lease1.fence)])
            .unwrap_or_else(|error| panic!("{error:?}: {}", "legacy fence JSON")),
    )?;
    drop(service);
    let mut reopened = worker_service(directory)?;
    reopened.jobs = JobService::new(
        JobStore::open(&directory.store())?,
        ProcessRunner::new(directory.absent(), Vec::new(), Duration::from_millis(100)),
        delegate(),
    );
    assert_eq!(
        reopened.maintain_fences(future),
        Err(ServiceError::ExecutionUnavailable),
        "unavailable cancellation remains an explicit error after durable migration"
    );
    assert_eq!(reopened.jobs.store().tracked()?.len(), 3);
    assert_eq!(
        reopened.jobs.store().job(orders[3].key()?)?.state.state,
        JobState::Accepted
    );
    assert_eq!(
        reopened.jobs.store().job(orders[2].key()?)?.state.state,
        JobState::Ended(Outcome::UnknownExecution)
    );
    let recovered = reopened.jobs.store().job(lease1.key)?.state;
    assert_eq!(recovered.state, JobState::Running);
    assert_eq!(recovered.fence, lease1.fence);
    assert!(recovered.late, "expired fence must survive reopen");
    let marker = Digest32::new([0xE1; 32])?;
    reopened
        .jobs
        .store()
        .mark(marker, Digest32::new([0xE2; 32])?, HEIGHT, NOW)?;
    reopened.cleanup_retention(&fixture.market.world.bytes, future, TASK_EXPIRY + 1)?;
    // An expired replay marker was removed: a different digest can be recorded only now.
    reopened
        .jobs
        .store()
        .mark(marker, Digest32::new([0xE3; 32])?, TASK_EXPIRY + 2, future)?;
    assert!(!directory.0.join("chain-jobs.json").exists());
    assert!(!directory.0.join("expired-attempts.json").exists());
    Ok(())
}

fn assert_ambiguous_execution(service: &worker::Service, lease2: Lease) -> Checked {
    assert_eq!(
        service.jobs.run(lease2, NOW + 1)?,
        JobState::Ended(Outcome::UnknownExecution)
    );
    let calls = service.jobs.runner().invocations();
    assert!(
        service.jobs.run(lease2, NOW + 2).is_err(),
        "ambiguous execution cannot redispatch"
    );
    assert_eq!(service.jobs.runner().invocations(), calls);
    service.jobs.reconcile(NOW + 3)?;
    assert_eq!(
        service.jobs.store().job(lease2.key)?.state.state,
        JobState::Ended(Outcome::UnknownExecution),
        "a peer with no retry guarantee cannot authorize another dispatch"
    );
    Ok(())
}

#[test]
fn worker_retention_preserves_unresolved_admissions_and_expired_fences_after_reopen() -> Checked {
    let directory = Scratch::new("retention")?;
    let mut fixture = admitting()?;
    let orders = (0..4)
        .map(|i| {
            let mut order = order(&fixture, 0xB0 + i, u64::from(i) + 1, 512)?;
            if i == 2 {
                order.submit.request.deadline_ms = 100;
                order.signed = order.submit.signed(&customer())?;
            }
            Ok(order)
        })
        .collect::<Checked<Vec<_>>>()?;
    for (i, order) in orders.iter().enumerate() {
        admit_chain_task(
            &mut fixture,
            order,
            0xB0 + u8::try_from(i).unwrap_or_else(|error| panic!("{error:?}: {}", "nonce")),
        )?;
    }
    let mut service = worker_service(&directory)?;
    // A real process peer with no retry guarantee produces an ambiguous dispatch.
    service.jobs = JobService::new(
        JobStore::open(&directory.store())?,
        ProcessRunner::new(
            std::env::current_exe()?,
            peer_args("retain-exit"),
            Duration::from_millis(100),
        ),
        delegate(),
    );
    feed(&service, &fixture.market, HEIGHT)?;
    let bad = service.dispatch("POST", SUBMIT_PATH, &orders[0].signed);
    assert_ne!(
        bad.status, 200,
        "missing input must refuse before admission"
    );
    assert!(service.jobs.store().tracked()?.is_empty());
    let mut acknowledgments = Vec::new();
    for order in &orders {
        stage_input(&service, order)?;
        let response = service.dispatch("POST", SUBMIT_PATH, &order.signed);
        assert_eq!(response.status, 200, "durable admission");
        acknowledgments.push(response.body);
    }
    assert_eq!(service.jobs.store().tracked()?.len(), orders.len());
    // Job 0 is locally final but still awaiting chain settlement before its chain deadline.
    let cancel = orders[0].reference(Route::Cancel, principal(9)?, 11, &customer())?;
    assert_eq!(
        service
            .dispatch("POST", layerx_paxai_worker::auth::CANCEL_PATH, &cancel)
            .status,
        200
    );
    // Job 1 is running; job 2 is ambiguous; job 3 remains accepted.
    let lease1 = service.jobs.claim(
        orders[1].key()?,
        &orders[1].accepted(&acknowledgments[1])?,
        &fixture.authority,
        NOW,
    )?;
    let lease2 = service.jobs.claim(
        orders[2].key()?,
        &orders[2].accepted(&acknowledgments[2])?,
        &fixture.authority,
        NOW,
    )?;
    assert_ambiguous_execution(&service, lease2)?;
    let running = service.jobs.store().job(lease1.key)?.state;
    service
        .jobs
        .fence_expired(lease1.key, lease1.fence, running.lease_until_ms)?;
    assert!(service.jobs.store().job(lease1.key)?.state.late);
    let current_ms = u64::try_from(
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_else(|error| panic!("{error:?}: {}", "wall clock"))
            .as_millis(),
    )
    .unwrap_or_else(|error| panic!("{error:?}: {}", "milliseconds"));
    let future = current_ms + RETENTION_MS * 2;
    assert_eq!(
        service.cleanup_retention(&fixture.market.world.bytes, future, HEIGHT)?,
        0,
        "locally final job still owes chain settlement"
    );
    assert_eq!(
        service.cleanup_retention(&fixture.market.world.bytes, future, TASK_EXPIRY + 1)?,
        1
    );
    assert!(service.jobs.store().tombstone(orders[0].key()?)?.is_some());
    assert_eq!(service.jobs.store().tracked()?.len(), 3);
    assert_legacy_reopen(&directory, &fixture, service, &orders, &lease1, future)?;
    Ok(())
}
