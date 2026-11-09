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
fn reward_finality_authorization_requires_verified_snapshot() -> Checked {
    use std::cell::Cell;

    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_client::ai_market::{Approval, OperationEffect, VerifiedProgramState};
    use layerx_programs_ai_market::{
        admission::{AdmissionTable, EvaluatorConsent, Participant},
        codec::{self, ROSTER_MAX_BYTES},
        epoch,
        evaluators::{authority, codec::encode_grant, model::CONSENT_BYTES},
        registry_ops::PolicySection,
        reputation_transition::split_joint,
        state::ActorSlot,
        types::{AccountId, MetadataDigest, PublicKey32},
        workers::{self, WorkerTable},
        MAX_RESULT_BYTES, MAX_STATE_BYTES, MERGED_CHAIN_DOMAIN,
    };
    use layerx_programs_registry::chain_proof::{decode::ProofOp, VerifiedChainHead};
    use sha2::{Digest as _, Sha256};

    struct Guest {
        state: Vec<u8>,
        request: u64,
    }

    impl Guest {
        fn header(&self) -> Checked<layerx_programs_ai_market::registry::MarketHeader> {
            let shared = state::decode_shared_state(&self.state)?;
            Ok(PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?.header)
        }

        fn request_id(&self) -> Checked<RequestId> {
            let mut bytes = [0x71; 32];
            bytes[..8].copy_from_slice(&(self.request + 1).to_be_bytes());
            Ok(RequestId::new(bytes)?)
        }

        fn call(
            &mut self,
            actor: PrincipalId,
            operation: dispatch::Operation,
            payload: &[u8],
            height: u64,
        ) -> Checked {
            let chain = ChainDomain::new(MERGED_CHAIN_DOMAIN)?;
            let mut sequence = 1;
            let (epoch, roster) = if self.state.is_empty() {
                (0, Presence::Absent)
            } else {
                let shared = state::decode_shared_state(&self.state)?;
                for index in 0..43 {
                    if let Some(role) = shared.control.replay.actor(ActorSlot::from_index(index)?) {
                        if role.principal == actor {
                            sequence = role.last.map_or(1, |last| last.sequence + 1);
                        }
                    }
                }
                if operation == dispatch::OPEN_EPOCH {
                    let frozen = epoch::preview_open(
                        &self.state,
                        height,
                        &mut vec![0; MAX_STATE_BYTES],
                        &mut vec![0; epoch::OPEN_SCRATCH_BYTES],
                    )?;
                    (frozen.epoch, Presence::Present(frozen.roster))
                } else {
                    epoch::captured_roster(&shared)?.map_or((0, Presence::Absent), |roster| {
                        (
                            roster.binding.epoch,
                            Presence::Present(roster.binding.roster),
                        )
                    })
                }
            };
            if operation.metadata().sequence == dispatch::SequencePolicy::ObjectLocal {
                sequence = 0;
            }
            let envelope = Envelope {
                operation,
                chain,
                program: program()?,
                market: derive_market(chain, program()?)?,
                actor,
                epoch,
                config: 1,
                roster,
                sequence,
                expiry: 200,
                request: self.request_id()?,
                payload,
                authentication: Authentication::Native,
            };
            let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
            let length = encode_envelope(&envelope, &mut encoded)?;
            self.request += 1;
            let mut next = vec![0; MAX_STATE_BYTES];
            let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
            let mut event = vec![0; MAX_EVENT_BYTES];
            let mut result = vec![0; MAX_RESULT_BYTES];
            let routed = dispatch::route(
                &registry_ops::CallContext {
                    chain,
                    program: program()?,
                    principal: actor,
                    height,
                },
                &encoded[..length],
                if self.state.is_empty() {
                    None
                } else {
                    Some(&self.state)
                },
                dispatch::Buffers {
                    next: &mut next,
                    scratch: &mut scratch,
                    event: &mut event,
                    result: &mut result,
                },
            )?;
            match routed {
                dispatch::Routed::Applied {
                    state_len,
                    result_len,
                    ..
                } => {
                    assert_eq!(codec::decode_result(&result[..result_len])?.error, None);
                    next.truncate(state_len);
                    self.state = next;
                    Ok(())
                }
                dispatch::Routed::Refused { result_len } => Err(Failure::Setup(format!(
                    "{}: {:?}",
                    operation.metadata().name,
                    codec::decode_result(&result[..result_len])?.error
                ))),
                dispatch::Routed::Unchanged { .. } => {
                    Err(Failure::Setup("fresh guest operation unchanged".to_owned()))
                }
            }
        }

        fn approve_participant(
            &mut self,
            participant: Participant,
            owner: PrincipalId,
            key: PublicKey32,
            nonce: u8,
            identity: Digest32,
        ) -> Checked<Digest32> {
            let mut payload = vec![participant.role() as u8];
            payload.extend_from_slice(&participant.bytes());
            payload.extend_from_slice(owner.as_bytes());
            payload.extend_from_slice(
                codec::domain_hash("PAXAI/enrollment-nonce/v1", &[nonce; 32])?.as_bytes(),
            );
            payload.extend_from_slice(&key.0);
            payload.extend_from_slice(&1_u64.to_be_bytes());
            payload.extend_from_slice(identity.as_bytes());
            payload.extend_from_slice(&0_u64.to_be_bytes());
            payload.extend_from_slice(&1_u64.to_be_bytes());
            payload.extend_from_slice(&64_u64.to_be_bytes());
            self.call(principal(OWNER)?, dispatch::ApproveAdmission, &payload, 2)?;
            let shared = state::decode_shared_state(&self.state)?;
            let table = AdmissionTable::decode(
                split_joint(shared.section(Section::ReputationAdmission)?)?.1,
            )?;
            table
                .get(participant)
                .and_then(|member| member.approval)
                .map(|approval| approval.digest)
                .ok_or_else(|| Failure::Setup("missing produced admission approval".to_owned()))
        }
    }

    let mut guest = Guest {
        state: Vec::new(),
        request: 0,
    };
    let mut policy = vec![0; TASK_POLICY_BYTES];
    policy_with(4)?.encode(&mut policy)?;
    let mut create = OWNER.to_vec();
    create.extend_from_slice(&ASSET);
    create.extend_from_slice(derive_rewards_account(program()?, AssetId::new(ASSET)?)?.as_bytes());
    create.extend_from_slice(&REFUND);
    create.push(0);
    create.extend_from_slice(&policy);
    create.extend_from_slice(&[16; 32]);
    guest.call(principal(OWNER)?, dispatch::CREATE, &create, ORIGIN)?;

    let worker_owner = principal([51; 32])?;
    let delegate = SigningKey::from_bytes(&[52; 32]);
    let public = PublicKey32(delegate.verifying_key().to_bytes());
    let header = guest.header()?;
    let worker = codec::derive_worker(header.market_id, worker_owner, [51; 32])?;
    let metadata = MetadataDigest::new([53; 32])?;
    let mut enroll = worker_owner.bytes().to_vec();
    enroll.extend_from_slice(&[51; 32]);
    enroll.extend_from_slice(&public.0);
    enroll.extend_from_slice(metadata.as_bytes());
    enroll.extend_from_slice(&120_u64.to_be_bytes());
    guest.call(principal(OWNER)?, dispatch::EnrollWorker, &enroll, 2)?;
    let consent =
        workers::consent_digest(&header, worker, worker_owner, public, 1, 1, metadata, 120)?;
    let mut accept = worker.bytes().to_vec();
    accept.extend_from_slice(&1_u64.to_be_bytes());
    accept.extend_from_slice(&1_u64.to_be_bytes());
    accept.extend_from_slice(metadata.as_bytes());
    accept.extend_from_slice(&delegate.sign(consent.as_bytes()).to_bytes());
    guest.call(worker_owner, dispatch::AcceptEnrollment, &accept, 2)?;
    let identity = {
        let shared = state::decode_shared_state(&guest.state)?;
        let table = WorkerTable::decode(
            authority::split_identity_section(shared.section(Section::IdentityRoster)?)?.0,
        )?;
        table
            .get(worker)
            .ok_or_else(|| Failure::Setup("missing produced worker".to_owned()))?
            .proposal_digest(&guest.header()?)?
    };
    let approval = guest.approve_participant(
        Participant::Worker(worker),
        worker_owner,
        public,
        51,
        identity,
    )?;
    let mut admit = worker.bytes().to_vec();
    for value in [1_u64, 0, 1] {
        admit.extend_from_slice(&value.to_be_bytes());
    }
    admit.extend_from_slice(approval.as_bytes());
    guest.call(worker_owner, dispatch::AdmitWorker, &admit, 2)?;

    for nonce in 54..=56 {
        let owner = principal([nonce; 32])?;
        let key = SigningKey::from_bytes(&[nonce; 32]);
        let public = PublicKey32(key.verifying_key().to_bytes());
        let mut schedule = owner.bytes().to_vec();
        schedule.extend_from_slice(&[nonce; 32]);
        schedule.extend_from_slice(&[4; 32]);
        schedule.extend_from_slice(&public.0);
        for value in [1_u64, 1, 0, 32] {
            schedule.extend_from_slice(&value.to_be_bytes());
        }
        guest.call(principal(OWNER)?, dispatch::ScheduleEvaluator, &schedule, 2)?;
        let grant = {
            let shared = state::decode_shared_state(&guest.state)?;
            let region = authority::evaluator_region(shared.section(Section::IdentityRoster)?)?;
            region
                .get(codec::derive_evaluator(
                    header.market_id,
                    owner,
                    [nonce; 32],
                )?)
                .ok_or_else(|| Failure::Setup("missing produced evaluator".to_owned()))?
                .grant
        };
        let mut encoded_grant = [0; 256];
        let length = encode_grant(&grant, &mut encoded_grant)?;
        let identity = codec::domain_hash("PAXAI/evaluator-grant/v1", &encoded_grant[..length])?;
        let approval = guest.approve_participant(
            Participant::Evaluator(grant.evaluator),
            owner,
            public,
            nonce,
            identity,
        )?;
        let consent = EvaluatorConsent {
            chain: header.deployment_chain_domain,
            program: header.program_id,
            market: header.market_id,
            evaluator: grant.evaluator,
            owner,
            signing_key: public,
            enrollment_nonce: [nonce; 32],
            rubric: grant.rubric,
            approval_digest: approval,
            request: guest.request_id()?,
            grant_version: 1,
            key_version: 1,
            effective_epoch: 0,
            config_version: 1,
            expiry_height: 64,
        };
        let mut encoded_consent = [0; CONSENT_BYTES];
        consent.encode(&mut encoded_consent)?;
        let mut admit = encoded_consent.to_vec();
        admit.extend_from_slice(&key.sign(consent.digest()?.as_bytes()).to_bytes());
        guest.call(owner, dispatch::AdmitEvaluator, &admit, 2)?;
    }
    let mut fund = vec![0; 57];
    let length = FundRequest {
        amount: 100,
        refund_recipient: AccountId::new(REFUND)?,
        policy_version: FUNDING_POLICY_VERSION,
        consent: true,
    }
    .encode(&mut fund)?;
    guest.call(principal(OWNER)?, dispatch::FUND, &fund[..length], 2)?;
    guest.call(principal(OWNER)?, dispatch::OPEN_EPOCH, &[], 10)?;
    let shared = state::decode_shared_state(&guest.state)?;
    let captured = epoch::captured_roster(&shared)?
        .ok_or_else(|| Failure::Setup("missing produced roster".to_owned()))?;
    assert_eq!(captured.workers().len(), 1);
    assert_eq!(captured.evaluators().len(), 3);
    assert_eq!(captured.workers()[0].worker, worker);
    let mut roster = vec![0; ROSTER_MAX_BYTES];
    let length = codec::encode_roster(&captured.view(), &mut roster)?;
    roster.truncate(length);
    let mut payload = worker.bytes().to_vec();
    payload.extend_from_slice(worker_owner.as_bytes());
    payload.extend_from_slice(&100_u128.to_be_bytes());
    assert_eq!(payload.len(), 80);

    let (namespace_head, manifest, ops, root, app_hash) = repair::namespace_proof(&guest.state);
    let node = repair::node_commit(11, app_hash, [0x66; 32]);
    let registry = registry()?;
    let signer = owner_signer()?;
    let from = signer
        .evm_address()
        .ok_or_else(|| Failure::Setup("missing EVM key".to_owned()))?;
    let request = OperationRequest {
        operation: dispatch::CLAIM,
        payload: &payload,
        actor: principal(OWNER)?,
        roster: Some(&roster),
        sequence: 0,
        expiry: 200,
        request: RequestId::new([34; 32])?,
        required_rank: FINALIZED_RANK,
    };
    let terms = NativeTerms {
        network_id: NETWORK,
        actor_did: b"did:layerx:paxai-owner",
        owner_public_key: signer.public_key(),
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
    };
    enum Finality {
        Verified,
        Missing,
        Asserted,
    }
    let prepared_count = Cell::new(0);
    let approved_count = Cell::new(0);
    let authorize = |head: &VerifiedChainHead,
                     value: &[u8],
                     proof_ops: &[ProofOp],
                     block: [u8; 32],
                     evidence: Finality|
     -> Result<(PreparedOperation, Approval), AiClientError> {
        let witness = VerifiedProgramState::verify(
            head,
            &node.verifier.chain_id,
            header.program_id,
            &namespace_head,
            &manifest,
            value,
            proof_ops,
        )?;
        let verified = CommitFinality::from_verified_program(head, &witness)?;
        let asserted = CommitFinality::new(
            verified.block_height(),
            verified.block_hash().bytes(),
            verified.app_hash().bytes(),
            verified.store_proof_digest().bytes(),
        )?;
        let finality = match evidence {
            Finality::Verified => Some(&verified),
            Finality::Missing => None,
            Finality::Asserted => Some(&asserted),
        };
        let snapshot = repair::capture_market(value, root, block, 10).finalize(finality, 5000)?;
        let prepared = snapshot.prepare(&registry, 11, &request, &terms)?;
        prepared_count.set(prepared_count.get() + 1);
        assert_eq!(prepared.binding(), snapshot.binding());
        assert_eq!(prepared.snapshot_id(), snapshot.snapshot_id());
        let review = prepared.clone().review(&registry, from)?;
        let approved_terms = review.terms().clone();
        assert_eq!(approved_terms.snapshot, prepared.snapshot_id());
        let approval = review.approve(&approved_terms, signer.public_key())?;
        approved_count.set(approved_count.get() + 1);
        Ok((prepared, approval))
    };
    let mut modified_value = guest.state.clone();
    let last = modified_value.len() - 1;
    modified_value[last] ^= 1;
    let mut modified_key = ops.clone();
    modified_key[0].key[0] ^= 1;
    let mut modified_block = node.head.header_hash();
    modified_block[0] ^= 1;
    let mut modified_app = app_hash;
    modified_app[0] ^= 1;
    let foreign_node = repair::node_commit(11, modified_app, [0x66; 32]);
    for (head, value, proof_ops, block, evidence, expected) in [
        (
            &node.head,
            modified_value.as_slice(),
            ops.as_slice(),
            node.head.header_hash(),
            Finality::Verified,
            AiClientError::FinalityMismatch,
        ),
        (
            &node.head,
            guest.state.as_slice(),
            modified_key.as_slice(),
            node.head.header_hash(),
            Finality::Verified,
            AiClientError::FinalityMismatch,
        ),
        (
            &node.head,
            guest.state.as_slice(),
            ops.as_slice(),
            modified_block,
            Finality::Verified,
            AiClientError::Query(QueryError::BindingMismatch),
        ),
        (
            &foreign_node.head,
            guest.state.as_slice(),
            ops.as_slice(),
            node.head.header_hash(),
            Finality::Verified,
            AiClientError::FinalityMismatch,
        ),
        (
            &node.head,
            guest.state.as_slice(),
            ops.as_slice(),
            node.head.header_hash(),
            Finality::Missing,
            AiClientError::Query(QueryError::FinalityUnavailable),
        ),
        (
            &node.head,
            guest.state.as_slice(),
            ops.as_slice(),
            node.head.header_hash(),
            Finality::Asserted,
            AiClientError::Query(QueryError::FinalityUnavailable),
        ),
    ] {
        assert_eq!(
            authorize(head, value, proof_ops, block, evidence),
            Err(expected)
        );
        assert_eq!(prepared_count.get(), 0);
        assert_eq!(approved_count.get(), 0);
    }
    let (prepared, _approval) = authorize(
        &node.head,
        &guest.state,
        &ops,
        node.head.header_hash(),
        Finality::Verified,
    )?;
    assert_eq!(prepared_count.get(), 1);
    assert_eq!(approved_count.get(), 1);
    assert_eq!(prepared.binding().block_height, 11);
    assert_eq!(prepared.binding().execution_height, 10);
    assert_eq!(
        prepared.binding().block_hash.bytes(),
        node.head.header_hash()
    );
    assert_eq!(prepared.binding().app_hash.bytes(), app_hash);
    assert_eq!(prepared.binding().native_state_root.bytes(), root);
    let mut proof_hash = Sha256::new();
    for op in &ops {
        proof_hash.update((op.data.len() as u64).to_be_bytes());
        proof_hash.update(&op.data);
    }
    let proof_digest: [u8; 32] = proof_hash.finalize().into();
    assert_eq!(prepared.binding().store_proof_digest.bytes(), proof_digest);
    assert_eq!(
        prepared.binding().state_digest,
        codec::state_digest(&guest.state)?
    );
    assert_eq!(prepared.binding().rank, FINALIZED_RANK);
    assert_eq!(prepared.binding().snapshot_id()?, prepared.snapshot_id());
    assert_eq!(
        prepared.effect(),
        OperationEffect::Claim {
            worker,
            recipient: AccountId::new(worker_owner.bytes())?,
            amount: 100,
            asset: AssetId::new(ASSET)?
        }
    );
    let call = evm_transaction::decode_unsigned(prepared.canonical_bytes())
        .map_err(setup("claim transaction"))?;
    let disclosure = bind_call(&call, from).map_err(setup("claim disclosure"))?;
    let Some(DisclosedNativeOperation::ProgramCall(native)) = disclosure.native_operation else {
        return Err(Failure::Setup("missing disclosed claim".to_owned()));
    };
    let envelope = decode_envelope(&native.calldata)?.envelope;
    assert_eq!(envelope.operation, dispatch::CLAIM);
    assert_eq!(envelope.payload, payload);
    assert_eq!(envelope.roster, Presence::Present(captured.binding.roster));
    assert_eq!(envelope.epoch, captured.binding.epoch);
    assert_eq!(native.program_id.bytes(), PROGRAM);
    assert_eq!(native.guest_abi, GUEST_ABI);
    assert_eq!(native.entrypoint, ENTRYPOINT);
    Ok(())
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
