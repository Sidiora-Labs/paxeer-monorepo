#![forbid(unsafe_code)]
extern crate self as layerx_programs_ai_market;
pub use layerx_client::ai_market::market_program::*;

mod signal {
    include!("../src/inference_market/signal.rs");
    pub mod deals {
        include!("../src/inference_market/deals.rs");
        pub mod laps {
            include!("../src/inference_market/laps.rs");
        }
    }
}

#[allow(dead_code)]
mod producers {
    use ed25519_dalek::{Signer, SigningKey};
    use layerx_programs_ai_market::{
        admission::{
            AdmissionContext, AdmissionTable, ApprovalTerms, EvaluatorConsent, Participant,
        },
        aggregation::{self, Progress},
        aggregation_codec::AggregationPhase,
        codec::{self, decode_envelope, decode_result, encode_envelope, Envelope, ResultStatus},
        dispatch::{self, Buffers, Operation, Routed},
        epoch::{self, Frozen},
        errors::{CodecResult, NON_CANONICAL, NOT_FOUND},
        evaluators::{
            authority,
            codec::encode_grant,
            model::{EvaluatorGrant, GrantTerms},
        },
        policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
        registry::{derive_rewards_account, MarketHeader},
        registry_ops::{CallContext, PolicySection},
        reputation_transition::split_joint,
        rewards::{decode_reward_state, REWARD_STATE_BYTES},
        state::{decode_shared_state, ActorSlot, Section},
        tasks::{self, SetBinding, TaskSet},
        types::*,
        workers::{self, WorkerTable},
        MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
    };

    /// Keep the original application error while identifying the refusing fixture boundary.
    fn fixture_step<T>(
        step: &str,
        operation: &str,
        run: impl FnOnce() -> CodecResult<T>,
    ) -> CodecResult<T> {
        run().map_err(|error| {
            eprintln!("fixture step={step} operation={operation}: {error:?}");
            error
        })
    }

    pub struct Reply {
        pub routed: Routed,
        pub state: Vec<u8>,
        pub event: Vec<u8>,
        pub result: Vec<u8>,
    }
    pub fn header(bytes: &[u8]) -> CodecResult<MarketHeader> {
        fixture_step(
            "header",
            "decode_shared_state -> PolicySection::decode",
            || {
                let state =
                    fixture_step("header", "decode_shared_state (codec.rs:schema)", || {
                        decode_shared_state(bytes)
                    })?;
                let section = state.feature_sections[Section::PolicyLifecycle.index()];
                fixture_step("header", "PolicySection::decode", || {
                    PolicySection::decode(section)
                })
                .map(|section| section.header)
            },
        )
    }
    pub fn table(bytes: &[u8]) -> CodecResult<AdmissionTable> {
        fixture_step(
            "table",
            "decode_shared_state -> split_joint -> AdmissionTable::decode (admission.rs:660)",
            || {
                let state = fixture_step("table", "decode_shared_state (codec.rs:schema)", || {
                    decode_shared_state(bytes)
                })?;
                let joint = state.feature_sections[Section::ReputationAdmission.index()];
                let (_, table) = fixture_step("table", "split_joint", || split_joint(joint))?;
                AdmissionTable::decode(table).map_err(|error| {
        eprintln!("fixture step=table operation=AdmissionTable::decode decoder=admission.rs:660 prefix={:?} length={}", table.get(..2), table.len());
        error
    })
            },
        )
    }
    pub fn next_sequence(bytes: &[u8], actor: PrincipalId) -> CodecResult<u64> {
        fixture_step(
            "next_sequence",
            "decode_shared_state -> replay.actor",
            || {
                let state = decode_shared_state(bytes)?;
                for index in 0..43 {
                    let slot = ActorSlot::from_index(index)?;
                    if let Some(role) = state.control.replay.actor(slot) {
                        if role.principal == actor {
                            return Ok(role.last.map_or(1, |r| r.sequence + 1));
                        }
                    }
                }
                Ok(1)
            },
        )
    }
    pub fn envelope(
        bytes: &[u8],
        operation: Operation,
        actor: PrincipalId,
        payload: &[u8],
        sequence: u64,
    ) -> CodecResult<Vec<u8>> {
        fixture_step(
            "envelope",
            "captured_roster -> encode_envelope (codec.rs:schema)",
            || {
                let market = header(bytes)?;
                let frozen = epoch::captured_roster(&decode_shared_state(bytes)?)?;
                let (epoch, roster) = frozen.map_or((0, Presence::Absent), |r| {
                    (r.binding.epoch, Presence::Present(r.binding.roster))
                });
                let mut id = actor.bytes();
                id[..2].copy_from_slice(&operation.selector().to_be_bytes());
                id[2..10].copy_from_slice(&sequence.to_be_bytes());
                id[10..18].copy_from_slice(&decode_shared_state(bytes)?.revision.to_be_bytes());
                let e = Envelope {
                    operation,
                    chain: market.deployment_chain_domain,
                    program: market.program_id,
                    market: market.market_id,
                    actor,
                    epoch,
                    config: market.active_config_version,
                    roster,
                    sequence,
                    expiry: u64::MAX,
                    request: RequestId::new(id)?,
                    payload,
                    authentication: Authentication::Native,
                };
                let mut out = vec![0; MAX_ENVELOPE_BYTES];
                let len = encode_envelope(&e, &mut out)?;
                out.truncate(len);
                Ok(out)
            },
        )
    }
    pub fn compose(
        bytes: &[u8],
        encoded: &[u8],
        actor: PrincipalId,
        at: u64,
    ) -> CodecResult<Reply> {
        fixture_step(
            "compose",
            "decode header -> dispatch::route -> frame result",
            || {
                let market = header(bytes)?;
                let mut next = vec![0; MAX_STATE_BYTES];
                let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
                let mut event = vec![0; MAX_EVENT_BYTES];
                let mut result = vec![0; MAX_RESULT_BYTES];
                let routed = dispatch::route(
                    &CallContext {
                        chain: market.deployment_chain_domain,
                        program: market.program_id,
                        principal: actor,
                        height: at,
                    },
                    encoded,
                    Some(bytes),
                    Buffers {
                        next: &mut next,
                        scratch: &mut scratch,
                        event: &mut event,
                        result: &mut result,
                    },
                )?;
                match routed {
                    Routed::Applied {
                        state_len,
                        event_len,
                        result_len,
                        ..
                    } => {
                        next.truncate(state_len);
                        event.truncate(event_len);
                        result.truncate(result_len);
                    }
                    Routed::Unchanged { result_len } | Routed::Refused { result_len } => {
                        next.clear();
                        assert!(event.iter().all(|b| *b == 0));
                        event.clear();
                        result.truncate(result_len);
                    }
                }
                Ok(Reply {
                    routed,
                    state: next,
                    event,
                    result,
                })
            },
        )
    }
    pub fn apply(
        bytes: &mut Vec<u8>,
        encoded: &[u8],
        actor: PrincipalId,
        at: u64,
    ) -> CodecResult<Reply> {
        let operation = decode_envelope(encoded)?.envelope.operation;
        let reply = fixture_step("apply", operation.metadata().name, || {
            compose(bytes, encoded, actor, at)
        })?;
        match reply.routed {
            Routed::Applied { .. } => *bytes = reply.state.clone(),
            Routed::Refused { .. } => {
                let error = decode_result(&reply.result)?.error.ok_or(NON_CANONICAL)?;
                eprintln!(
                    "{} at {at} refused: {error:?}",
                    decode_envelope(encoded)?.envelope.operation.metadata().name
                );
                return Err(error);
            }
            Routed::Unchanged { .. } => return Err(NON_CANONICAL),
        }
        Ok(reply)
    }
    pub fn call(
        bytes: &mut Vec<u8>,
        operation: Operation,
        actor: PrincipalId,
        payload: &[u8],
        at: u64,
    ) -> CodecResult<Reply> {
        fixture_step("call", operation.metadata().name, || {
            let sequence = if operation.metadata().sequence == dispatch::SequencePolicy::ObjectLocal
            {
                0
            } else {
                next_sequence(bytes, actor)?
            };
            let encoded = envelope(bytes, operation, actor, payload, sequence)?;
            apply(bytes, &encoded, actor, at)
        })
    }
    fn approval(
        bytes: &mut Vec<u8>,
        participant: Participant,
        owner: PrincipalId,
        key: PublicKey32,
        nonce: u8,
        identity: Digest32,
        at: u64,
    ) -> CodecResult<(u64, Digest32)> {
        fixture_step("approval", "ApproveAdmission", || {
            let market = header(bytes)?;
            let effective = table(bytes)?.required_effective_epoch(&AdmissionContext {
                market: &market,
                invoking_principal: owner,
                height: at,
            })?;
            let mut terms = ApprovalTerms {
                participant,
                owner,
                enrollment_nonce_commitment: codec::domain_hash(
                    "PAXAI/enrollment-nonce/v1",
                    &[nonce; 32],
                )?,
                delegate: key,
                delegate_generation: 1,
                identity_commitment: identity,
                effective_epoch: effective,
                config_version: market.active_config_version,
                request: RequestId::new([1; 32])?,
                expiry_height: market.origin_height + effective * 128 + 64,
            };
            let mut payload = vec![participant.role() as u8];
            payload.extend_from_slice(&participant.bytes());
            payload.extend_from_slice(owner.as_bytes());
            payload.extend_from_slice(terms.enrollment_nonce_commitment.as_bytes());
            payload.extend_from_slice(&key.0);
            payload.extend_from_slice(&1u64.to_be_bytes());
            payload.extend_from_slice(identity.as_bytes());
            payload.extend_from_slice(&effective.to_be_bytes());
            payload.extend_from_slice(&market.active_config_version.to_be_bytes());
            payload.extend_from_slice(&terms.expiry_height.to_be_bytes());
            let encoded = envelope(
                bytes,
                dispatch::ApproveAdmission,
                market.owner_principal,
                &payload,
                next_sequence(bytes, market.owner_principal)?,
            )?;
            terms.request = decode_envelope(&encoded)?.envelope.request;
            let digest = terms.digest(&market)?;
            apply(bytes, &encoded, market.owner_principal, at)?;
            Ok((effective, digest))
        })
    }
    pub fn enroll(
        bytes: &mut Vec<u8>,
        owner_sequence: &mut u64,
        owner: PrincipalId,
        nonce: u8,
        key: &SigningKey,
        metadata: MetadataDigest,
        at: u64,
    ) -> CodecResult<WorkerRosterEntry> {
        fixture_step(
            "enroll",
            "EnrollWorker -> AcceptEnrollment -> ApproveAdmission -> AdmitWorker",
            || {
                let market = header(bytes)?;
                let public = PublicKey32(key.verifying_key().to_bytes());
                let worker = codec::derive_worker(market.market_id, owner, [nonce; 32])?;
                let expiry = at + workers::DEFAULT_PROPOSAL_LIFETIME;
                let mut payload = owner.bytes().to_vec();
                payload.extend_from_slice(&[nonce; 32]);
                payload.extend_from_slice(&public.0);
                payload.extend_from_slice(metadata.as_bytes());
                payload.extend_from_slice(&expiry.to_be_bytes());
                call(
                    bytes,
                    dispatch::EnrollWorker,
                    market.owner_principal,
                    &payload,
                    at,
                )?;
                let consent = workers::consent_digest(
                    &market, worker, owner, public, 1, 1, metadata, expiry,
                )?;
                let mut payload = worker.bytes().to_vec();
                payload.extend_from_slice(&1u64.to_be_bytes());
                payload.extend_from_slice(&1u64.to_be_bytes());
                payload.extend_from_slice(metadata.as_bytes());
                payload.extend_from_slice(&key.sign(consent.as_bytes()).to_bytes());
                call(bytes, dispatch::AcceptEnrollment, owner, &payload, at)?;
                let identity = {
                    let state = decode_shared_state(bytes)?;
                    let w = WorkerTable::decode(
                        authority::split_identity_section(
                            state.feature_sections[Section::IdentityRoster.index()],
                        )?
                        .0,
                    )?;
                    w.get(worker)
                        .ok_or(NOT_FOUND)?
                        .proposal_digest(&header(bytes)?)?
                };
                let (effective, digest) = approval(
                    bytes,
                    Participant::Worker(worker),
                    owner,
                    public,
                    nonce,
                    identity,
                    at,
                )?;
                let mut payload = worker.bytes().to_vec();
                payload.extend_from_slice(&1u64.to_be_bytes());
                payload.extend_from_slice(&effective.to_be_bytes());
                payload.extend_from_slice(&market.active_config_version.to_be_bytes());
                payload.extend_from_slice(digest.as_bytes());
                call(bytes, dispatch::AdmitWorker, owner, &payload, at)?;
                *owner_sequence = next_sequence(bytes, market.owner_principal)?;
                Ok(WorkerRosterEntry {
                    worker,
                    owner,
                    recipient: AccountId::new(owner.bytes())?,
                    generation: Version::new(1)?,
                    key_version: Version::new(1)?,
                    public_key: public,
                    metadata,
                })
            },
        )
    }
    pub fn evaluator(
        bytes: &mut Vec<u8>,
        owner_sequence: &mut u64,
        owner: PrincipalId,
        nonce: u8,
        key: &SigningKey,
        at: u64,
    ) -> CodecResult<()> {
        fixture_step(
            "evaluator",
            "ScheduleEvaluator -> ApproveAdmission -> AdmitEvaluator (admission.rs:1148)",
            || {
                let market = header(bytes)?;
                let public = PublicKey32(key.verifying_key().to_bytes());
                let effective = table(bytes)?.required_effective_epoch(&AdmissionContext {
                    market: &market,
                    invoking_principal: owner,
                    height: at,
                })?;
                let rubric = PolicySection::decode(
                    decode_shared_state(bytes)?.feature_sections[Section::PolicyLifecycle.index()],
                )?
                .current
                .commitments
                .rubric;
                let grant = EvaluatorGrant::nominate(
                    market.market_id,
                    owner,
                    [nonce; 32],
                    GrantTerms {
                        rubric,
                        grant_version: Version::new(1)?,
                        key_version: Version::new(1)?,
                        signing_key: public,
                        effective_epoch: effective,
                        expiry_epoch_exclusive: effective + 32,
                    },
                )?;
                let mut payload = owner.bytes().to_vec();
                payload.extend_from_slice(&[nonce; 32]);
                payload.extend_from_slice(rubric.as_bytes());
                payload.extend_from_slice(&public.0);
                for n in [1u64, 1, effective, effective + 32] {
                    payload.extend_from_slice(&n.to_be_bytes());
                }
                call(
                    bytes,
                    dispatch::ScheduleEvaluator,
                    market.owner_principal,
                    &payload,
                    at,
                )?;
                let mut encoded_grant = [0; 256];
                let len = encode_grant(&grant, &mut encoded_grant)?;
                let identity =
                    codec::domain_hash("PAXAI/evaluator-grant/v1", &encoded_grant[..len])?;
                let (_, digest) = approval(
                    bytes,
                    Participant::Evaluator(grant.evaluator),
                    owner,
                    public,
                    nonce,
                    identity,
                    at,
                )?;
                let request = RequestId::new([nonce; 32])?;
                let consent = EvaluatorConsent {
                    chain: market.deployment_chain_domain,
                    program: market.program_id,
                    market: market.market_id,
                    evaluator: grant.evaluator,
                    owner,
                    signing_key: public,
                    enrollment_nonce: [nonce; 32],
                    rubric,
                    approval_digest: digest,
                    request,
                    grant_version: 1,
                    key_version: 1,
                    effective_epoch: effective,
                    config_version: market.active_config_version,
                    expiry_height: market.origin_height + effective * 128 + 64,
                };
                let mut body = [0; 362];
                consent.encode(&mut body)?;
                let mut payload = body.to_vec();
                payload.extend_from_slice(&key.sign(consent.digest()?.as_bytes()).to_bytes());
                // The signed consent binds the exact request id carried by the outer envelope.
                let mut encoded = envelope(
                    bytes,
                    dispatch::AdmitEvaluator,
                    owner,
                    &payload,
                    next_sequence(bytes, owner)?,
                )?;
                let mut e = decode_envelope(&encoded)?.envelope;
                e.request = request;
                let mut out = vec![0; MAX_ENVELOPE_BYTES];
                let len = encode_envelope(&e, &mut out)?;
                out.truncate(len);
                encoded = out;
                apply(bytes, &encoded, owner, at)?;
                *owner_sequence = next_sequence(bytes, market.owner_principal)?;
                Ok(())
            },
        )
    }
    /// Refresh each expiring worker through its delegate-signed metadata producer before
    /// opening; enrollment consent and service metadata have separate bounded lifetimes.
    fn fixture_delegate(n: u8) -> SigningKey {
        SigningKey::from_bytes(&[0x90 + n; 32])
    }
    pub fn refresh_workers(bytes: &mut Vec<u8>, opening: u64) -> CodecResult<()> {
        refresh_workers_with(bytes, opening, |record| {
            [20_u8, 21]
                .into_iter()
                .map(fixture_delegate)
                .find(|key| key.verifying_key().to_bytes() == record.delegate.0)
                .unwrap_or_else(|| panic!("worker has no enrolled fixture delegate"))
        })
    }

    pub fn refresh_workers_with(
        bytes: &mut Vec<u8>,
        opening: u64,
        delegate: impl Fn(&workers::WorkerCurrent) -> SigningKey,
    ) -> CodecResult<()> {
        fixture_step(
            "refresh_workers",
            "decode_manifest -> PublishMetadata",
            || {
                let market = header(bytes)?;
                let state = decode_shared_state(bytes)?;
                let table = WorkerTable::decode(
                    authority::split_identity_section(
                        state.feature_sections[Section::IdentityRoster.index()],
                    )?
                    .0,
                )?;
                for record in table.iter() {
                    if !matches!(
                        record.state,
                        workers::WorkerState::Enrolled | workers::WorkerState::Available
                    ) || record.expiry > opening + 96
                    {
                        continue;
                    }
                    let at = opening
                        .checked_sub(workers::METADATA_COOLDOWN)
                        .ok_or(NON_CANONICAL)?;
                    let key = delegate(record);
                    assert_eq!(PublicKey32(key.verifying_key().to_bytes()), record.delegate);
                    let mut manifest = 1u16.to_be_bytes().to_vec();
                    manifest.extend_from_slice(market.market_id.as_bytes());
                    manifest.extend_from_slice(record.worker.as_bytes());
                    manifest.extend_from_slice(record.owner.as_bytes());
                    for value in [
                        record.generation,
                        record.key_version,
                        record.metadata_revision + 1,
                        at,
                        at + workers::MAX_METADATA_LIFETIME,
                    ] {
                        manifest.extend_from_slice(&value.to_be_bytes());
                    }
                    manifest.extend_from_slice(&[60; 32]);
                    manifest.extend_from_slice(&1u16.to_be_bytes());
                    manifest.extend_from_slice(&[1, 1]);
                    for byte in 61..66 {
                        manifest.extend_from_slice(&[byte; 32]);
                    }
                    for _ in 0..4 {
                        manifest.extend_from_slice(&1024u32.to_be_bytes());
                    }
                    manifest.push(1);
                    manifest.extend_from_slice(&60_000u32.to_be_bytes());
                    manifest.extend_from_slice(&4u16.to_be_bytes());
                    manifest.push(1);
                    manifest.extend_from_slice(&1u16.to_be_bytes());
                    manifest.push(1);
                    let uri = b"https://worker.example/paxai/v1";
                    manifest.extend_from_slice(&(uri.len() as u32).to_be_bytes());
                    manifest.extend_from_slice(uri);
                    manifest.extend_from_slice(&[70; 32]);
                    manifest.push(1);
                    manifest.extend_from_slice(&1u16.to_be_bytes());
                    manifest.extend_from_slice(&1u16.to_be_bytes());
                    manifest.extend_from_slice(&[71; 32]);
                    manifest.extend_from_slice(&[72; 32]);
                    manifest.extend_from_slice(&0u32.to_be_bytes());
                    let summary = workers::decode_manifest(&manifest)?;
                    let mut payload = record.worker.bytes().to_vec();
                    for value in [record.metadata_revision, summary.revision] {
                        payload.extend_from_slice(&value.to_be_bytes());
                    }
                    payload.extend_from_slice(summary.digest.as_bytes());
                    payload.extend_from_slice(&summary.valid_from.to_be_bytes());
                    payload.extend_from_slice(&summary.expiry.to_be_bytes());
                    payload.extend_from_slice(&(manifest.len() as u32).to_be_bytes());
                    payload.extend_from_slice(&manifest);
                    let encoded = envelope(
                        bytes,
                        dispatch::PublishMetadata,
                        record.owner,
                        &payload,
                        next_sequence(bytes, record.owner)?,
                    )?;
                    let mut call = decode_envelope(&encoded)?.envelope;
                    call.expiry = opening;
                    call.authentication = Authentication::Delegate {
                        key: record.delegate,
                        signature: Signature64([0; 64]),
                    };
                    let mut signed = vec![0; MAX_ENVELOPE_BYTES];
                    let len = encode_envelope(&call, &mut signed)?;
                    let digest = decode_envelope(&signed[..len])?.request_digest()?;
                    call.authentication = Authentication::Delegate {
                        key: record.delegate,
                        signature: Signature64(key.sign(digest.as_bytes()).to_bytes()),
                    };
                    let len = encode_envelope(&call, &mut signed)?;
                    apply(bytes, &signed[..len], record.owner, at)?;
                }
                Ok(())
            },
        )
    }
    pub fn open(bytes: &mut Vec<u8>, at: u64) -> CodecResult<Frozen> {
        fixture_step(
            "open",
            "refresh_workers -> preview_open -> OpenEpoch",
            || {
                refresh_workers(bytes, at)?;
                let mut next = vec![0; MAX_STATE_BYTES];
                let mut scratch = vec![0; epoch::OPEN_SCRATCH_BYTES];
                let frozen = fixture_step("open", "epoch::preview_open", || {
                    epoch::preview_open(bytes, at, &mut next, &mut scratch)
                })?;
                let encoded = envelope(
                    bytes,
                    dispatch::OPEN_EPOCH,
                    PrincipalId::new([0x7f; 32])?,
                    &[],
                    0,
                )?;
                let mut e = decode_envelope(&encoded)?.envelope;
                e.epoch = frozen.epoch;
                e.config = frozen.config.get();
                e.roster = Presence::Present(frozen.roster);
                let actor = e.actor;
                let mut out = vec![0; MAX_ENVELOPE_BYTES];
                let len = encode_envelope(&e, &mut out)?;
                apply(bytes, &out[..len], actor, at)?;
                Ok(frozen)
            },
        )
    }
    /// The legacy task/report readers take the F08-only view used by the real dispatcher.
    /// Carry through the production adapter; a read cannot write or replace the joint state.
    fn read_admission_view<T>(
        bytes: &[u8],
        read: impl FnOnce(&layerx_programs_ai_market::state::SharedState<'_>) -> CodecResult<T>,
    ) -> CodecResult<T> {
        let mut next = vec![0; MAX_STATE_BYTES];
        let mut scratch = vec![0; epoch::CARRY_SCRATCH_BYTES];
        let (value, written) =
            epoch::carry_reputation(bytes, &mut next, &mut scratch, |current, _| {
                Ok((read(&decode_shared_state(current)?)?, None))
            })?;
        assert_eq!(written, None);
        assert!(next.iter().all(|byte| *byte == 0));
        Ok(value)
    }

    pub fn sealed_task_set(bytes: &[u8], epoch: u64) -> CodecResult<Digest32> {
        fixture_step(
            "sealed_task_set",
            "tasks::sealed_task_set via F08 view (admission.rs:660)",
            || read_admission_view(bytes, |state| tasks::sealed_task_set(state, epoch)),
        )
    }

    pub fn seal_tasks(bytes: &mut Vec<u8>, at: u64) -> CodecResult<()> {
        fixture_step("seal_tasks", "sealed_task_set -> SealTaskSet", || {
            let state = decode_shared_state(bytes)?;
            let market = header(bytes)?;
            let frozen = epoch::captured_roster(&state)?.ok_or(NOT_FOUND)?.binding;
            match sealed_task_set(bytes, frozen.epoch) {
                Ok(_) => return Ok(()),
                Err(layerx_programs_ai_market::errors::F09_EVIDENCE_TASK_SET_UNSEALED) => {}
                Err(error) => return Err(error),
            }
            let section =
                PolicySection::decode(state.feature_sections[Section::PolicyLifecycle.index()])?;
            let set = SetBinding {
                market: market.market_id,
                epoch: frozen.epoch,
                config: frozen.config,
                policy: section.current.digest()?,
                roster: frozen.roster,
            };
            let digest = tasks::task_set_digest(&set, &TaskSet::decode(section.task_region)?)?;
            let mut payload = frozen.epoch.to_be_bytes().to_vec();
            payload.extend_from_slice(&frozen.config.get().to_be_bytes());
            payload.extend_from_slice(digest.as_bytes());
            call(
                bytes,
                dispatch::SEAL_TASK_SET,
                PrincipalId::new([0x7f; 32])?,
                &payload,
                at,
            )?;
            Ok(())
        })
    }
    pub fn settle(bytes: &mut Vec<u8>, at: u64) -> CodecResult<()> {
        fixture_step(
            "settle",
            "BeginAggregation -> ProcessAggregation -> FinalizeAggregation",
            || {
                seal_tasks(bytes, at)?;
                let keeper = PrincipalId::new([0x7f; 32])?;
                if aggregation::progress(bytes)?.phase == AggregationPhase::Unsealed {
                    call(bytes, dispatch::BeginAggregation, keeper, &[], at)?;
                }
                loop {
                    let p = aggregation::progress(bytes)?;
                    if p.cursor == p.worker_count {
                        break;
                    }
                    let Presence::Present(input) = p.input else {
                        return Err(NOT_FOUND);
                    };
                    let mut payload = input.bytes().to_vec();
                    payload.extend_from_slice(&p.cursor.to_be_bytes());
                    call(bytes, dispatch::ProcessAggregation, keeper, &payload, at)?;
                }
                let Presence::Present(input) = aggregation::progress(bytes)?.input else {
                    return Err(NOT_FOUND);
                };
                call(
                    bytes,
                    dispatch::FinalizeAggregation,
                    keeper,
                    input.as_bytes(),
                    at,
                )?;
                Ok(())
            },
        )
    }
    /// Refresh every included identity using its own authenticated owner and replay slot.
    pub fn heartbeat(bytes: &mut Vec<u8>, at: u64) -> CodecResult<()> {
        fixture_step("heartbeat", "Heartbeat", || {
            let table = table(bytes)?;
            for member in table.iter().filter(|m| m.admitted_epoch.is_some()) {
                let mut payload = vec![member.participant.role() as u8];
                payload.extend_from_slice(&member.participant.bytes());
                payload.extend_from_slice(&member.membership_generation.to_be_bytes());
                payload.extend_from_slice(&member.delegate_generation.to_be_bytes());
                payload.extend_from_slice(&table.current_epoch().ok_or(NOT_FOUND)?.to_be_bytes());
                call(bytes, dispatch::Heartbeat, member.owner, &payload, at)?;
            }
            Ok(())
        })
    }
}

mod proof {
    use super::*;
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_client::ai_market::{inference_market as client, sign_market_envelope, NativeTerms};
    use layerx_crypto::account_derivation::derive_from_seed;
    use layerx_crypto::signer::{LocalSigner, Signer as _};
    use layerx_crypto::{disclosure::call::decode_program_call, evm_transaction};
    use layerx_programs_ai_market::{
        codec::{self, Envelope},
        dispatch::{self, Routed},
        epoch,
        errors::*,
        evidence::*,
        laps::*,
        policy::{PolicyCommitments, RoomProfile, TaskPolicyV1, TASK_POLICY_BYTES},
        registry::derive_rewards_account,
        registry_ops::{CallContext, PolicySection},
        rooms::*,
        roster,
        state::{decode_shared_state, Section},
        types::*,
        MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
        MERGED_CHAIN_DOMAIN,
    };
    use layerx_types::program_call::Resources;
    use sha2::{Digest as _, Sha256};
    use signal::deals::laps::{
        receive_countersignature, send_lap, LapAcknowledgement, LapEngine, LapError, LapFrame,
        LapServerFrame, LapTransport, MeterReport,
    };
    use signal::deals::{DealFrame, LayerPlan, PlannedLayers};
    use std::fs;
    use std::io::{Read, Write};
    use std::net::{TcpListener, TcpStream};
    use std::path::PathBuf;
    use std::sync::atomic::{AtomicU64, Ordering};
    use std::sync::{Arc, Mutex};
    use std::time::Duration;
    use tungstenite::{Message, WebSocket};

    fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
        value.unwrap_or_else(|e| panic!("{e:?}"))
    }
    fn native_signer(n: u8) -> LocalSigner {
        LocalSigner::from_derived_account(&must(derive_from_seed(&[n; 32], 0)))
    }
    fn terms(signer: &LocalSigner, request: [u8; 32], nonce: u64) -> NativeTerms<'_> {
        NativeTerms {
            network_id: 125,
            actor_did: b"did:layerx:lap-test",
            owner_public_key: signer.public_key(),
            account_sequence: nonce,
            idempotency_key: request,
            not_before: 0,
            not_after: u64::MAX,
            fee_limit: 1_000_000,
            capabilities: &[],
            access_declaration: &[],
            response_capacity: 16_384,
            resources: Resources([10_000_000; 7]),
            gas_limit: 100_000,
            max_fee_per_gas: 10,
            max_priority_fee_per_gas: 1,
        }
    }
    fn ready<F: std::future::Future>(future: F) -> F::Output {
        struct WakeThread(std::thread::Thread);
        impl std::task::Wake for WakeThread {
            fn wake(self: Arc<Self>) {
                self.0.unpark();
            }
        }
        let waker = std::task::Waker::from(Arc::new(WakeThread(std::thread::current())));
        let mut context = std::task::Context::from_waker(&waker);
        let mut future = std::pin::pin!(future);
        loop {
            match future.as_mut().poll(&mut context) {
                std::task::Poll::Ready(v) => return v,
                std::task::Poll::Pending => std::thread::park(),
            }
        }
    }
    struct Directory(PathBuf);
    impl Directory {
        fn new() -> Self {
            static NEXT: AtomicU64 = AtomicU64::new(0);
            let path = std::env::temp_dir().join(format!(
                "inference-laps-{}-{}",
                std::process::id(),
                NEXT.fetch_add(1, Ordering::Relaxed)
            ));
            must(fs::create_dir(&path));
            Self(path)
        }
    }
    impl Drop for Directory {
        fn drop(&mut self) {
            let _ = fs::remove_dir_all(&self.0);
        }
    }
    const ORIGIN: u64 = 128;
    const START: u64 = ORIGIN + 6 * 128;
    const AT: u64 = START + 64;
    fn owner() -> PrincipalId {
        actor(12)
    }
    fn actor(n: u8) -> PrincipalId {
        let signer = native_signer(n);
        let mut bytes = [0; 32];
        bytes[12..].copy_from_slice(&signer.evm_address().unwrap());
        PrincipalId::new(bytes).unwrap()
    }
    fn digest(n: u8) -> Digest32 {
        Digest32::new([n; 32]).unwrap()
    }
    fn encoded(size: usize, f: impl FnOnce(&mut [u8]) -> CodecResult<usize>) -> Vec<u8> {
        let mut out = vec![0; size];
        let n = f(&mut out).unwrap();
        out.truncate(n);
        out
    }

    struct Market {
        bytes: Vec<u8>,
        roster: RosterDigest,
    }
    impl Market {
        fn new() -> Self {
            let chain = ChainDomain::new(MERGED_CHAIN_DOMAIN).unwrap();
            let program = ProgramId::new([11; 32]).unwrap();
            let asset = AssetId::new([13; 32]).unwrap();
            let policy = TaskPolicyV1::bounded_default(
                1,
                1,
                PolicyCommitments {
                    model_artifact: digest(1),
                    dataset_artifact: [2; 32],
                    benchmark_suite: digest(3),
                    rubric: RubricDigest::new([4; 32]).unwrap(),
                    task_schema: digest(5),
                    result_schema: digest(6),
                    service_terms: digest(7),
                },
                1000,
                1,
            )
            .unwrap();
            let mut payload = owner().bytes().to_vec();
            payload.extend_from_slice(asset.as_bytes());
            payload.extend_from_slice(derive_rewards_account(program, asset).unwrap().as_bytes());
            payload.extend_from_slice(&[14; 32]);
            payload.push(0);
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
            .unwrap();
            let Routed::Applied { state_len, .. } = routed else {
                panic!("Create refused");
            };
            bytes.truncate(state_len);
            let mut sequence = 2;
            for n in [20] {
                producers::enroll(
                    &mut bytes,
                    &mut sequence,
                    actor(n),
                    n,
                    &SigningKey::from_bytes(&[0x90 + n; 32]),
                    MetadataDigest::new([n; 32]).unwrap(),
                    ORIGIN + 2,
                )
                .unwrap();
            }
            for n in [2, 3, 4] {
                producers::evaluator(
                    &mut bytes,
                    &mut sequence,
                    actor(n),
                    n,
                    &SigningKey::from_bytes(&[n; 32]),
                    ORIGIN + 2,
                )
                .unwrap();
            }
            let mut fund = 1000u128.to_be_bytes().to_vec();
            fund.extend_from_slice(&[14; 32]);
            fund.extend_from_slice(&1u64.to_be_bytes());
            fund.push(1);
            producers::call(&mut bytes, dispatch::FUND, owner(), &fund, ORIGIN + 7).unwrap();
            producers::open(&mut bytes, ORIGIN + 128).unwrap();
            producers::heartbeat(&mut bytes, ORIGIN + 129).unwrap();
            producers::enroll(
                &mut bytes,
                &mut sequence,
                actor(21),
                21,
                &SigningKey::from_bytes(&[0x90 + 21; 32]),
                MetadataDigest::new([21; 32]).unwrap(),
                ORIGIN + 130,
            )
            .unwrap();
            producers::settle(&mut bytes, ORIGIN + 224).unwrap();
            let mut schedule = decode_shared_state(&bytes)
                .unwrap()
                .revision
                .to_be_bytes()
                .to_vec();
            schedule.extend_from_slice(&6u64.to_be_bytes());
            producers::call(
                &mut bytes,
                dispatch::SCHEDULE_ACTIVATION,
                owner(),
                &schedule,
                START - 32,
            )
            .unwrap();
            producers::open(&mut bytes, START).unwrap();
            let mut advance = decode_shared_state(&bytes)
                .unwrap()
                .revision
                .to_be_bytes()
                .to_vec();
            advance.extend_from_slice(&6u64.to_be_bytes());
            producers::call(
                &mut bytes,
                dispatch::ADVANCE_ACTIVATION,
                actor(127),
                &advance,
                START + 1,
            )
            .unwrap();
            let roster = RosterDigest::new(
                roster::roster_digest(&producers::table(&bytes).unwrap(), 6)
                    .unwrap()
                    .bytes(),
            )
            .unwrap();
            let mut market = Self { bytes, roster };
            let profile = RoomProfile {
                minimum_bond: 100,
                canary_ratio_bps: 250,
                lap_size_tokens: 32,
                challenge_window_blocks: 16,
                slash_fraction_bps: 1000,
                host_share_bps: 1000,
                protocol_share_bps: 100,
                settlement_account: AccountId::new([80; 32]).unwrap(),
                max_generated_tokens: 4096,
                queue_depth: layerx_programs_ai_market::policy::DEFAULT_QUEUE_DEPTH,
                liveness_timeout_ms: 30_000,
                acknowledgement_window_ms: 5_000,
                canary_relative_tolerance_bps: 100,
            };
            market.call(
                dispatch::OpenRoom,
                owner(),
                &encoded(256, |out| {
                    OpenRoomRequest {
                        room_id: digest(60),
                        model_digest: digest(1),
                        runtime_digest: digest(51),
                        profile,
                    }
                    .encode(out)
                }),
                START + 2,
            );
            market.call(
                dispatch::PostAsk,
                actor(30),
                &encoded(256, |out| {
                    PostAskRequest {
                        room_id: digest(60),
                        ask_id: digest(61),
                        model_digest: digest(1),
                        maximum_price_per_thousand_tokens: 2000,
                        stream_id: 123,
                    }
                    .encode(out)
                }),
                START + 2,
            );
            let state = decode_shared_state(&market.bytes).unwrap();
            let workers = layerx_programs_ai_market::workers::WorkerTable::decode(
                layerx_programs_ai_market::evaluators::authority::split_identity_section(
                    state.section(Section::IdentityRoster).unwrap(),
                )
                .unwrap()
                .0,
            )
            .unwrap();
            for (n, bid) in [(20, 70), (21, 71)] {
                let worker = workers.iter().find(|w| w.owner == actor(n)).unwrap();
                market.call(
                    dispatch::PostBid,
                    actor(n),
                    &encoded(256, |out| {
                        PostBidRequest {
                            room_id: digest(60),
                            bid_id: digest(bid),
                            capability_digest: Digest32::new(worker.metadata.bytes())?,
                            minimum_price_per_thousand_tokens: 5,
                            bond_amount: 100,
                            layer_capacity: 16,
                        }
                        .encode(out)
                    }),
                    START + 2,
                );
            }
            let mut layers = [None; MAX_DEAL_PEERS];
            layers[0] = Some(LayerSplit {
                bid_id: digest(70),
                first_layer: 0,
                end_layer: 8,
            });
            layers[1] = Some(LayerSplit {
                bid_id: digest(71),
                first_layer: 8,
                end_layer: 16,
            });
            market.call(
                dispatch::RecordDeal,
                owner(),
                &encoded(1400, |out| {
                    RecordDealRequest {
                        room_id: digest(60),
                        deal_id: digest(62),
                        ask_id: digest(61),
                        agreed_price_per_thousand_tokens: 1001,
                        roster_digest: roster,
                        layers,
                    }
                    .encode(out)
                }),
                START + 2,
            );
            producers::seal_tasks(&mut market.bytes, AT).unwrap();
            market
        }
        fn envelope(&self, op: dispatch::Operation, who: PrincipalId, payload: &[u8]) -> Vec<u8> {
            let sequence = if op.metadata().sequence == dispatch::SequencePolicy::ObjectLocal {
                0
            } else {
                producers::next_sequence(&self.bytes, who).unwrap()
            };
            let bytes = producers::envelope(&self.bytes, op, who, payload, sequence).unwrap();
            if op.metadata().feature != 11 {
                return bytes;
            }
            let mut e = codec::decode_envelope(&bytes).unwrap().envelope;
            e.roster = Presence::Present(self.roster);
            encoded(MAX_ENVELOPE_BYTES, |out| codec::encode_envelope(&e, out))
        }
        fn call(
            &mut self,
            op: dispatch::Operation,
            who: PrincipalId,
            payload: &[u8],
            at: u64,
        ) -> Vec<u8> {
            if op == dispatch::PostBid {
                let mut r = codec::Reader::new(payload);
                let room = r.digest32().unwrap();
                let bid = r.digest32().unwrap();
                r.take(48).unwrap();
                let amount = r.u128().unwrap();
                if layerx_programs_ai_market::bonds::bond_from_state(&self.bytes, bid)
                    .unwrap()
                    .is_none()
                {
                    self.call(
                        dispatch::LockBond,
                        who,
                        &layerx_programs_ai_market::bonds::LockBondRequest { room, bid, amount }
                            .encode()
                            .unwrap(),
                        at,
                    );
                }
            }
            let e = self.envelope(op, who, payload);
            producers::apply(&mut self.bytes, &e, who, at).unwrap();
            e
        }
    }

    fn frame(market: &Market) -> DealFrame {
        let shared = must(decode_shared_state(&market.bytes));
        let policy = must(PolicySection::decode(must(
            shared.section(Section::PolicyLifecycle),
        )));
        let book = must(RoomBook::decode(must(shared.room_market()).1));
        let workers = must(workers::WorkerTable::decode(
            must(evaluators::authority::split_identity_section(must(
                shared.section(Section::IdentityRoster),
            )))
            .0,
        ));
        let layers = [(20, 70, 0, 8), (21, 71, 8, 16)]
            .into_iter()
            .map(|(n, bid, first_layer, end_layer)| {
                let worker = workers
                    .iter()
                    .find(|worker| worker.owner == actor(n))
                    .unwrap();
                PlannedLayers {
                    peer_key: worker.delegate.0,
                    owner: actor(n).bytes(),
                    bid_id: digest(bid).bytes(),
                    manifest_digest: worker.metadata.bytes(),
                    first_layer,
                    end_layer,
                }
            })
            .collect();
        let deal = must(book.deal(digest(62))).unwrap();
        DealFrame {
            chain: policy.header.deployment_chain_domain.bytes(),
            program: policy.header.program_id.bytes(),
            market: policy.header.market_id.bytes(),
            room_id: digest(60).bytes(),
            model_digest: digest(1).bytes(),
            runtime_digest: digest(51).bytes(),
            deal_id: digest(62).bytes(),
            ask_id: digest(61).bytes(),
            roster_digest: deal.roster_digest.bytes(),
            epoch: 6,
            config: 1,
            round: 1,
            price: deal.agreed_price_per_thousand_tokens.to_string(),
            plan: LayerPlan {
                embedding_host: owner().bytes(),
                head_host: owner().bytes(),
                layer_count: 16,
                layers,
            },
        }
    }

    struct WirePeer {
        key: client::HostMeterKey,
        socket: WebSocket<TcpStream>,
        reader: TcpStream,
        writer: Arc<Mutex<TcpStream>>,
    }
    impl WirePeer {
        fn new(n: u8) -> Self {
            let listener = must(TcpListener::bind("127.0.0.1:0"));
            let address = must(listener.local_addr());
            let server = std::thread::spawn(move || {
                let (stream, _) = must(listener.accept());
                must(stream.set_read_timeout(Some(Duration::from_secs(3))));
                let socket = must(tungstenite::accept(stream));
                must(socket.get_ref().try_clone())
            });
            let stream = must(TcpStream::connect(address));
            must(stream.set_read_timeout(Some(Duration::from_secs(3))));
            let (socket, _) = must(tungstenite::client(format!("ws://{address}/"), stream));
            let reader = must(server.join());
            let writer = Arc::new(Mutex::new(must(reader.try_clone())));
            Self {
                key: client::HostMeterKey::from_seed(&[0x90 + n; 32]),
                socket,
                reader,
                writer,
            }
        }
        fn acknowledge(
            &mut self,
            issued: &LapFrame,
            engine: &mut LapEngine,
            now: u64,
        ) -> signal::deals::laps::LapCountersignature {
            must(send_lap(&self.writer, issued));
            let Message::Text(text) = must(self.socket.read()) else {
                panic!("receipt must be a text frame");
            };
            let wire: serde_json::Value = must(serde_json::from_str(text.as_str()));
            assert_eq!(wire["type"], "lap_receipt");
            let mut unknown = wire.clone();
            unknown["unknown"] = serde_json::json!(true);
            assert!(serde_json::from_value::<LapServerFrame>(unknown).is_err());
            let mut unknown = wire;
            unknown["lap"]["receipt"]["unknown"] = serde_json::json!(true);
            assert!(serde_json::from_value::<LapServerFrame>(unknown).is_err());
            let LapServerFrame::LapReceipt { lap } = must(serde_json::from_str(text.as_str()));
            assert_eq!(lap.receipt, issued.receipt);
            assert_eq!(lap.host_meter_key, issued.host_meter_key);
            must(lap.receipt.verify(issued.host_meter_key));
            let signed = must(lap.countersign(&self.key));
            let acknowledgement = must(serde_json::to_value(LapAcknowledgement::LapCountersign(
                signed.clone(),
            )));
            assert_eq!(acknowledgement["type"], "lap_countersign");
            let mut unknown = acknowledgement.clone();
            unknown["unknown"] = serde_json::json!(true);
            assert!(serde_json::from_value::<LapAcknowledgement>(unknown).is_err());
            must(self.socket.send(Message::Text(
                must(serde_json::to_string(&acknowledgement)).into(),
            )));
            let received = must(receive_countersignature(&mut self.reader, &self.writer));
            assert_eq!(received.digest, signed.digest);
            must(engine.countersign(self.key.public_key(), &received, now));
            signed
        }
    }

    struct RoutedMarket {
        market: Market,
        pending: Vec<Vec<u8>>,
        operations: Vec<dispatch::Operation>,
        meter_submissions: Vec<Vec<u8>>,
    }
    impl RoutedMarket {
        fn new() -> Self {
            Self {
                market: Market::new(),
                pending: Vec::new(),
                operations: Vec::new(),
                meter_submissions: Vec::new(),
            }
        }
        fn commit(&mut self) {
            assert!(!self.pending.is_empty());
            for raw in std::mem::take(&mut self.pending) {
                let (call, signature) = must(evm_transaction::decode_signed(&raw));
                let sender = must(evm_transaction::recover_sender(&call, &signature));
                let mut principal = [0; 32];
                principal[12..].copy_from_slice(&sender);
                let (native, _) = must(decode_program_call(&call.data));
                let reply = must(producers::compose(
                    &self.market.bytes,
                    &native.calldata,
                    must(PrincipalId::new(principal)),
                    AT,
                ));
                match reply.routed {
                    Routed::Applied { .. } => self.market.bytes = reply.state,
                    _ => panic!(
                        "real program refused: {:?}",
                        must(codec::decode_result(&reply.result)).error
                    ),
                }
            }
        }
    }
    impl LapTransport for RoutedMarket {
        fn state(&mut self) -> Result<Vec<u8>, LapError> {
            Ok(self.market.bytes.clone())
        }
        fn submit(&mut self, operation: &client::MarketOperation) -> Result<(), LapError> {
            let signer = if operation.operation() == dispatch::SealEvidence {
                native_signer(2)
            } else {
                native_signer(12)
            };
            let caller = if operation.operation() == dispatch::SealEvidence {
                actor(2)
            } else {
                owner()
            };
            let envelope = self
                .market
                .envelope(operation.operation(), caller, operation.payload());
            let request = must(codec::decode_envelope(&envelope))
                .envelope
                .request
                .bytes();
            let signed = ready(sign_market_envelope(
                &envelope,
                &terms(&signer, request, self.operations.len() as u64),
                &signer,
            ))?;
            self.pending.push(signed.signed_bytes().to_vec());
            self.operations.push(operation.operation());
            Ok(())
        }
        fn meter(&mut self, report: &MeterReport) -> Result<bool, LapError> {
            let settled = market_settlement(&self.market.bytes);
            assert_eq!(settled.batch.cumulative_tokens, report.cumulative_tokens);
            let calldata = report.calldata()?;
            let signer = native_signer(12);
            let request: [u8; 32] = Sha256::digest(&calldata).into();
            let signed = ready(client::sign_stream_meter(
                &calldata,
                &terms(&signer, request, 2),
                &signer,
            ))?;
            let (call, signature) = must(evm_transaction::decode_signed(signed.signed_bytes()));
            assert_eq!(
                must(evm_transaction::recover_sender(&call, &signature)),
                signer.evm_address().unwrap()
            );
            let mut target = [0; 20];
            target[18..].copy_from_slice(&[0x10, 0x1b]);
            assert_eq!(call.to, target);
            assert_eq!(call.data, calldata);
            self.meter_submissions.push(signed.signed_bytes().to_vec());
            // A signed submission alone is pending; this harness does not invent a pay receipt.
            Ok(false)
        }
    }
    fn market_settlement(bytes: &[u8]) -> LapSettlement {
        must(settlement_from_state(bytes, digest(62))).unwrap()
    }
    fn open_engine(
        directory: &Directory,
        market: &Market,
        host: &client::HostMeterKey,
        batch_size: usize,
    ) -> LapEngine {
        must(LapEngine::open(
            &directory.0.join("laps.json"),
            &market.bytes,
            frame(market),
            host.public_key(),
            batch_size,
        ))
    }
    fn ranges() -> Vec<[u8; 32]> {
        vec![digest(90).bytes(), digest(91).bytes()]
    }

    #[test]
    fn receipt_issue_and_countersign_over_the_signalling_channel() {
        let market = Market::new();
        let directory = Directory::new();
        let host = client::HostMeterKey::from_seed(&[12; 32]);
        let mut engine = open_engine(&directory, &market, &host, 4);
        let mut peers = [WirePeer::new(20), WirePeer::new(21)];
        let issued = must(engine.issue_lap(10, 32, ranges(), false, 100, &host));
        assert_eq!(issued.receipt.lap_index, 0);
        assert_eq!(issued.receipt.cumulative_tokens, 42);
        assert_eq!(issued.receipt.peer_countersignatures, 0);
        assert!(engine
            .issue_lap(0, 32, ranges(), false, 101, &host)
            .is_err());
        peers[0].acknowledge(&issued, &mut engine, 101);
        assert!(!must(engine.finish_lap(102, &host)));
        peers[1].acknowledge(&issued, &mut engine, 102);
        assert!(must(engine.finish_lap(103, &host)));
        assert_eq!(engine.receipts()[0].peer_countersignatures, 3);
        must(engine.receipts()[0].verify(host.public_key()));
        assert!(!engine.ready_to_settle());
        must(engine.end_deal());
        assert!(engine.ready_to_settle());
    }

    #[test]
    fn batch_merkle_root_matches_the_on_chain_seal_and_settle_precedes_meter() {
        let mut transport = RoutedMarket::new();
        let directory = Directory::new();
        let host = client::HostMeterKey::from_seed(&[12; 32]);
        let mut engine = open_engine(&directory, &transport.market, &host, 3);
        let shared = must(decode_shared_state(&transport.market.bytes));
        let frozen = must(epoch::captured_roster(&shared)).unwrap().binding;
        assert_ne!(frozen.roster, transport.market.roster);
        assert_eq!(frozen.epoch, 6);
        let mut peers = [WirePeer::new(20), WirePeer::new(21)];
        for index in 0..3 {
            let issued = must(engine.issue_lap(
                if index == 0 { 10 } else { 0 },
                32,
                ranges(),
                false,
                index * 10,
                &host,
            ));
            for peer in &mut peers {
                peer.acknowledge(&issued, &mut engine, index * 10 + 1);
            }
            assert!(must(engine.finish_lap(index * 10 + 2, &host)));
        }
        let batch = must(engine.batch());
        assert_eq!(batch.cumulative_tokens, 106);
        let mut nodes = engine
            .receipts()
            .iter()
            .map(|receipt| {
                let digests = receipt
                    .layer_range_digests
                    .iter()
                    .map(|d| must(Digest32::new(*d)))
                    .collect::<Vec<_>>();
                must(must(receipt.receipt(&digests)).digest()).bytes()
            })
            .collect::<Vec<_>>();
        while nodes.len() > 1 {
            nodes = nodes
                .chunks(2)
                .map(|pair| {
                    let mut bytes = pair[0].to_vec();
                    bytes.extend_from_slice(pair.get(1).unwrap_or(&pair[0]));
                    must(codec::domain_hash("PAXAI/lap-node/v1", &bytes)).bytes()
                })
                .collect();
        }
        assert_eq!(batch.batch_digest.bytes(), nodes[0]);
        assert!(engine.report_meter(&mut transport, &host).is_err());
        assert!(!must(engine.seal_and_settle(&mut transport, &host)));
        assert_eq!(transport.operations, vec![dispatch::SealEvidence]);
        assert!(transport.meter_submissions.is_empty());
        transport.commit();
        assert!(!must(engine.seal_and_settle(&mut transport, &host)));
        assert_eq!(
            transport.operations,
            vec![dispatch::SealEvidence, dispatch::SettleLaps]
        );
        assert!(transport.meter_submissions.is_empty());
        transport.commit();
        assert_eq!(market_settlement(&transport.market.bytes).batch, batch);
        let sealed_state = transport.market.bytes.clone();
        let shared = must(decode_shared_state(&sealed_state));
        let seal = must(SealRegion::decode(shared.control.feature_bytes))
            .seals(6)
            .next()
            .map(must)
            .unwrap();
        assert!(!must(engine.seal_and_settle(&mut transport, &host)));
        let archive: serde_json::Value = must(serde_json::from_slice(&must(fs::read(
            directory.0.join("laps.batch-0.json"),
        ))));
        let manifest_bytes = archive["artifact_manifest"]
            .as_array()
            .unwrap()
            .iter()
            .map(|v| v.as_u64().unwrap() as u8)
            .collect::<Vec<_>>();
        assert_eq!(
            must(manifest_root(&manifest_bytes)).bytes(),
            seal.root.bytes()
        );
        let manifest = must(decode_manifest(&manifest_bytes));
        assert_eq!(
            manifest.content_root,
            content_root(
                manifest.byte_length,
                manifest.chunk_count,
                &batch.batch_digest.bytes()
            )
        );
        assert_eq!(
            must(require_sealed_artifact_chunk_root(
                &shared,
                6,
                &manifest_bytes,
                &batch
            )),
            market_settlement(&transport.market.bytes).task_set
        );
        assert_eq!(transport.meter_submissions.len(), 1);
        assert_eq!(
            engine.receipts().len(),
            3,
            "pending meter must retain the batch"
        );
        let (call, _) = must(evm_transaction::decode_signed(
            &transport.meter_submissions[0],
        ));
        assert_eq!(
            &call.data[..4],
            &evm_transaction::transaction_hash(b"streamMeter(bytes32,uint64,bytes32,bytes)")[..4]
        );
        assert_eq!(&call.data[4..36], &manifest_stream(123));
        assert_eq!(&call.data[60..68], &106_u64.to_be_bytes());
        assert_eq!(&call.data[68..100], &host.public_key());
        let signature = must(call.data[164..228].try_into());
        must(layerx_crypto::ed25519::verify_digest(
            &host.public_key(),
            &signature,
            &client::meter_digest(manifest_stream(123), 106, host.public_key()),
        ));
        drop(engine);
        let reopened = open_engine(&directory, &transport.market, &host, 3);
        assert_eq!(must(reopened.batch()), batch);
        assert!(reopened.ready_to_settle());
        assert_eq!(transport.operations.len(), 2);
    }
    fn manifest_stream(id: u128) -> [u8; 32] {
        let mut stream = [0; 32];
        stream[16..].copy_from_slice(&id.to_be_bytes());
        stream
    }

    #[test]
    fn two_missing_countersign_deadlines_in_one_batch_mark_and_drop_the_peer() {
        let market = Market::new();
        let directory = Directory::new();
        let host = client::HostMeterKey::from_seed(&[12; 32]);
        let mut engine = open_engine(&directory, &market, &host, 4);
        let mut peer = WirePeer::new(20);
        let missing = client::HostMeterKey::from_seed(&[0x90 + 21; 32]);
        let first = must(engine.issue_lap(3, 32, ranges(), false, 0, &host));
        let valid = peer.acknowledge(&first, &mut engine, 1);
        assert!(!must(engine.finish_lap(4999, &host)));
        assert!(engine
            .countersign(
                missing.public_key(),
                &must(first.countersign(&missing)),
                5000
            )
            .is_err());
        assert!(must(engine.finish_lap(5000, &host)));
        assert!(engine.penalties().is_empty());
        assert!(!engine.dropped(&missing.public_key()));
        let second = must(engine.issue_lap(0, 32, ranges(), false, 5001, &host));
        assert!(engine
            .countersign(peer.key.public_key(), &valid, 5002)
            .is_err());
        peer.acknowledge(&second, &mut engine, 5002);
        assert!(must(engine.finish_lap(10001, &host)));
        assert_eq!(engine.penalties().len(), 1);
        assert_eq!(engine.penalties()[0].peer_key, missing.public_key());
        assert_eq!(engine.penalties()[0].bid_id, digest(71).bytes());
        assert_eq!(engine.penalties()[0].batch_first_lap, 0);
        assert_eq!(engine.penalties()[0].missed_laps, vec![0, 1]);
        assert!(engine.dropped(&missing.public_key()));
        assert_eq!(must(engine.batch()).missing_countersigners, 2);
        assert_eq!(must(engine.batch()).countersign_quorum, 1);
        assert!(!must(engine.finish_lap(20000, &host)));
        drop(engine);
        let reopened = open_engine(&directory, &market, &host, 4);
        assert!(reopened.dropped(&missing.public_key()));
        assert_eq!(reopened.penalties().len(), 1);
    }

    #[test]
    fn pending_receipt_and_verified_countersignatures_survive_restart_and_forgery_is_rejected() {
        let market = Market::new();
        let directory = Directory::new();
        let host = client::HostMeterKey::from_seed(&[12; 32]);
        let mut engine = open_engine(&directory, &market, &host, 4);
        let mut peer = WirePeer::new(20);
        let first = must(engine.issue_lap(10, 32, ranges(), false, 100, &host));
        let ack = peer.acknowledge(&first, &mut engine, 101);
        let other = client::HostMeterKey::from_seed(&[0x90 + 21; 32]);
        assert!(engine.countersign(other.public_key(), &ack, 102).is_err());
        let mut forged = must(first.countersign(&other));
        forged.signature[0] ^= 1;
        assert!(engine
            .countersign(other.public_key(), &forged, 102)
            .is_err());
        drop(engine);
        let mut reopened = open_engine(&directory, &market, &host, 4);
        assert!(!must(reopened.finish_lap(102, &host)));
        must(reopened.countersign(other.public_key(), &must(first.countersign(&other)), 103));
        assert!(must(reopened.finish_lap(104, &host)));
        assert_eq!(reopened.receipts()[0].peer_countersignatures, 3);
        assert!(reopened.penalties().is_empty());
    }

    #[test]
    fn partial_deal_end_flushes_and_invalid_lap_boundaries_are_rejected() {
        let market = Market::new();
        let directory = Directory::new();
        let host = client::HostMeterKey::from_seed(&[12; 32]);
        let mut engine = open_engine(&directory, &market, &host, 64);
        assert!(engine.issue_lap(2, 31, ranges(), false, 0, &host).is_err());
        assert!(engine.issue_lap(2, 33, ranges(), true, 0, &host).is_err());
        assert!(engine
            .issue_lap(u64::MAX, 7, ranges(), true, 0, &host)
            .is_err());
        assert!(engine
            .issue_lap(2, 7, vec![[0; 32]; 2], true, 0, &host)
            .is_err());
        let issued = must(engine.issue_lap(2, 7, ranges(), true, 0, &host));
        for n in [20, 21] {
            let key = client::HostMeterKey::from_seed(&[0x90 + n; 32]);
            must(engine.countersign(key.public_key(), &must(issued.countersign(&key)), 1));
        }
        assert!(must(engine.finish_lap(2, &host)));
        assert!(engine.ready_to_settle());
        let batch = must(engine.batch());
        assert_eq!(
            (
                batch.first_lap_index,
                batch.last_lap_index,
                batch.cumulative_tokens
            ),
            (0, 0, 9)
        );
        assert!(engine.issue_lap(0, 32, ranges(), false, 3, &host).is_err());
    }
}
