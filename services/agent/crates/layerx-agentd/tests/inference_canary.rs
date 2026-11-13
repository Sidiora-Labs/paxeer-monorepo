#![forbid(unsafe_code)]
extern crate self as layerx_programs_ai_market;
pub use layerx_client::ai_market::market_program::*;

#[allow(
    dead_code,
    reason = "The host uses the full signal listener; this proof exercises lap receipts."
)]
mod signal {
    include!("../src/inference_market/signal.rs");
    pub mod deals {
        include!("../src/inference_market/deals.rs");
        pub mod laps {
            include!("../src/inference_market/laps.rs");
        }
    }
}
#[allow(
    dead_code,
    reason = "CanaryLapRequest is decoded by the host HTTP route; this proof uses the engine directly."
)]
mod canary_engine {
    include!("../src/inference_market/canary.rs");
}

mod producers {
    use ed25519_dalek::{Signer, SigningKey};
    use layerx_programs_ai_market::{
        admission::{
            AdmissionContext, AdmissionTable, ApprovalTerms, EvaluatorConsent, Participant,
        },
        aggregation,
        aggregation_codec::AggregationPhase,
        codec::{self, decode_envelope, decode_result, encode_envelope, Envelope},
        dispatch::{self, Buffers, Operation, Routed},
        epoch::{self, Frozen},
        errors::{CodecResult, NON_CANONICAL, NOT_FOUND},
        evaluators::{
            authority,
            codec::encode_grant,
            model::{EvaluatorGrant, GrantTerms},
        },
        registry::MarketHeader,
        registry_ops::{CallContext, PolicySection},
        reputation_transition::split_joint,
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
        run().inspect_err(|error| {
            eprintln!("fixture step={step} operation={operation}: {error:?}");
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
                AdmissionTable::decode(table).inspect_err(|error| {
        eprintln!("fixture step=table operation=AdmissionTable::decode decoder=admission.rs:660 prefix={:?} length={}: {error:?}", table.get(..2), table.len());
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
            Routed::Applied { .. } => bytes.clone_from(&reply.state),
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

    fn worker_manifest(
        market: &MarketHeader,
        record: &workers::WorkerCurrent,
        at: u64,
    ) -> CodecResult<Vec<u8>> {
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
        manifest.extend_from_slice(
            &u32::try_from(uri.len())
                .map_err(|_| NON_CANONICAL)?
                .to_be_bytes(),
        );
        manifest.extend_from_slice(uri);
        manifest.extend_from_slice(&[70; 32]);
        manifest.push(1);
        manifest.extend_from_slice(&1u16.to_be_bytes());
        manifest.extend_from_slice(&1u16.to_be_bytes());
        manifest.extend_from_slice(&[71; 32]);
        manifest.extend_from_slice(&[72; 32]);
        manifest.extend_from_slice(&0u32.to_be_bytes());
        Ok(manifest)
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
                    let manifest = worker_manifest(&market, record, at)?;
                    let summary = workers::decode_manifest(&manifest)?;
                    let mut payload = record.worker.bytes().to_vec();
                    for value in [record.metadata_revision, summary.revision] {
                        payload.extend_from_slice(&value.to_be_bytes());
                    }
                    payload.extend_from_slice(summary.digest.as_bytes());
                    payload.extend_from_slice(&summary.valid_from.to_be_bytes());
                    payload.extend_from_slice(&summary.expiry.to_be_bytes());
                    payload.extend_from_slice(
                        &u32::try_from(manifest.len())
                            .map_err(|_| NON_CANONICAL)?
                            .to_be_bytes(),
                    );
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
    use canary_engine::{
        canary_positions, CanaryAnswer, CanaryEngine, CanaryError, CanaryExample, CanaryPlan,
        CanaryTable, CanaryTransport, CanaryView,
    };
    use ed25519_dalek::SigningKey;
    use layerx_client::ai_market::{
        inference_market as client, market_envelope, sign_market_envelope, NativeTerms,
    };
    use layerx_crypto::account_derivation::derive_from_seed;
    use layerx_crypto::signer::{LocalSigner, Signer as _};
    use layerx_crypto::{disclosure::call::decode_program_call, evm_transaction};
    use layerx_programs_ai_market::{
        canary::{self, ChallengeStatus},
        codec::{self, Envelope},
        dispatch::{self, Routed},
        epoch,
        policy::{PolicyCommitments, RoomProfile, TaskPolicyV1, TASK_POLICY_BYTES},
        registry::derive_rewards_account,
        registry_ops::{CallContext, PolicySection},
        rooms::*,
        roster,
        state::{decode_shared_state, Section},
        workers, MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
        MERGED_CHAIN_DOMAIN,
    };
    use layerx_types::program_call::Resources;
    use signal::deals::laps::LapEngine;
    use signal::deals::{DealFrame, LayerPlan, PlannedLayers};
    use std::fs;
    use std::io::{Read, Write};
    use std::net::{SocketAddr, TcpListener, TcpStream};
    use std::path::{Path, PathBuf};
    use std::sync::{
        atomic::{AtomicU64, Ordering},
        Arc,
    };
    use std::thread::JoinHandle;
    use std::time::Duration;

    fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
        value.unwrap_or_else(|error| panic!("{error:?}"))
    }
    fn native_signer(n: u8) -> LocalSigner {
        LocalSigner::from_derived_account(&must(derive_from_seed(&[n; 32], 0)))
    }
    fn terms(signer: &LocalSigner, request: [u8; 32], nonce: u64) -> NativeTerms<'_> {
        NativeTerms {
            network_id: 125,
            actor_did: b"did:layerx:canary-test",
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
                std::task::Poll::Ready(value) => return value,
                std::task::Poll::Pending => std::thread::park(),
            }
        }
    }
    struct Directory(PathBuf);
    impl Directory {
        fn new() -> Self {
            static NEXT: AtomicU64 = AtomicU64::new(0);
            let path = std::env::temp_dir().join(format!(
                "inference-canary-{}-{}",
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
    const REVEAL: u64 = START + 80;
    const EXPECTED: [i32; 8] = [
        524_288, 262_144, 65_536, 0, -65_536, -131_072, -262_144, -524_288,
    ];
    fn owner() -> PrincipalId {
        actor(12)
    }
    fn actor(n: u8) -> PrincipalId {
        let signer = native_signer(n);
        let mut bytes = [0; 32];
        bytes[12..].copy_from_slice(
            &signer
                .evm_address()
                .unwrap_or_else(|| panic!("test signer has no EVM address")),
        );
        must(PrincipalId::new(bytes))
    }
    fn digest(n: u8) -> Digest32 {
        must(Digest32::new([n; 32]))
    }
    fn encoded(size: usize, f: impl FnOnce(&mut [u8]) -> CodecResult<usize>) -> Vec<u8> {
        let mut out = vec![0; size];
        let n = must(f(&mut out));
        out.truncate(n);
        out
    }
    struct Market {
        bytes: Vec<u8>,
        roster: RosterDigest,
    }
    impl Market {
        fn create_state() -> Vec<u8> {
            let chain =
                ChainDomain::new(MERGED_CHAIN_DOMAIN).unwrap_or_else(|error| panic!("{error:?}"));
            let program = ProgramId::new([11; 32]).unwrap_or_else(|error| panic!("{error:?}"));
            let asset = AssetId::new([13; 32]).unwrap_or_else(|error| panic!("{error:?}"));
            let policy = TaskPolicyV1::bounded_default(
                1,
                1,
                PolicyCommitments {
                    model_artifact: digest(1),
                    dataset_artifact: [2; 32],
                    benchmark_suite: digest(3),
                    rubric: RubricDigest::new([4; 32]).unwrap_or_else(|error| panic!("{error:?}")),
                    task_schema: digest(5),
                    result_schema: digest(6),
                    service_terms: digest(7),
                },
                1000,
                1,
            )
            .unwrap_or_else(|error| panic!("{error:?}"));
            let mut payload = owner().bytes().to_vec();
            payload.extend_from_slice(asset.as_bytes());
            payload.extend_from_slice(
                derive_rewards_account(program, asset)
                    .unwrap_or_else(|error| panic!("{error:?}"))
                    .as_bytes(),
            );
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
            .unwrap_or_else(|error| panic!("{error:?}"));
            let Routed::Applied { state_len, .. } = routed else {
                panic!("Create refused");
            };
            bytes.truncate(state_len);
            bytes
        }

        fn activate(bytes: &mut Vec<u8>) -> RosterDigest {
            let mut sequence = 2;
            producers::enroll(
                bytes,
                &mut sequence,
                actor(20),
                20,
                &SigningKey::from_bytes(&[0x90 + 20; 32]),
                MetadataDigest::new([20; 32]).unwrap_or_else(|error| panic!("{error:?}")),
                ORIGIN + 2,
            )
            .unwrap_or_else(|error| panic!("{error:?}"));
            for n in [2, 3, 4] {
                producers::evaluator(
                    bytes,
                    &mut sequence,
                    actor(n),
                    n,
                    &SigningKey::from_bytes(&[n; 32]),
                    ORIGIN + 2,
                )
                .unwrap_or_else(|error| panic!("{error:?}"));
            }
            let mut fund = 1000u128.to_be_bytes().to_vec();
            fund.extend_from_slice(&[14; 32]);
            fund.extend_from_slice(&1u64.to_be_bytes());
            fund.push(1);
            producers::call(bytes, dispatch::FUND, owner(), &fund, ORIGIN + 7)
                .unwrap_or_else(|error| panic!("{error:?}"));
            producers::open(bytes, ORIGIN + 128).unwrap_or_else(|error| panic!("{error:?}"));
            producers::heartbeat(bytes, ORIGIN + 129).unwrap_or_else(|error| panic!("{error:?}"));
            producers::enroll(
                bytes,
                &mut sequence,
                actor(21),
                21,
                &SigningKey::from_bytes(&[0x90 + 21; 32]),
                MetadataDigest::new([21; 32]).unwrap_or_else(|error| panic!("{error:?}")),
                ORIGIN + 130,
            )
            .unwrap_or_else(|error| panic!("{error:?}"));
            producers::settle(bytes, ORIGIN + 224).unwrap_or_else(|error| panic!("{error:?}"));
            let mut schedule = decode_shared_state(bytes)
                .unwrap_or_else(|error| panic!("{error:?}"))
                .revision
                .to_be_bytes()
                .to_vec();
            schedule.extend_from_slice(&6u64.to_be_bytes());
            producers::call(
                bytes,
                dispatch::SCHEDULE_ACTIVATION,
                owner(),
                &schedule,
                START - 32,
            )
            .unwrap_or_else(|error| panic!("{error:?}"));
            producers::open(bytes, START).unwrap_or_else(|error| panic!("{error:?}"));
            let mut advance = decode_shared_state(bytes)
                .unwrap_or_else(|error| panic!("{error:?}"))
                .revision
                .to_be_bytes()
                .to_vec();
            advance.extend_from_slice(&6u64.to_be_bytes());
            producers::call(
                bytes,
                dispatch::ADVANCE_ACTIVATION,
                actor(127),
                &advance,
                START + 1,
            )
            .unwrap_or_else(|error| panic!("{error:?}"));
            let roster = RosterDigest::new(
                roster::roster_digest(
                    &producers::table(bytes).unwrap_or_else(|error| panic!("{error:?}")),
                    6,
                )
                .unwrap_or_else(|error| panic!("{error:?}"))
                .bytes(),
            )
            .unwrap_or_else(|error| panic!("{error:?}"));
            roster
        }

        fn new(ratio_bps: u16) -> Self {
            let mut bytes = Self::create_state();
            let roster = Self::activate(&mut bytes);
            let mut market = Self { bytes, roster };
            let profile = RoomProfile {
                minimum_bond: 100,
                canary_ratio_bps: ratio_bps,
                lap_size_tokens: 32,
                challenge_window_blocks: 16,
                slash_fraction_bps: 1000,
                host_share_bps: 1000,
                protocol_share_bps: 100,
                settlement_account: AccountId::new([80; 32])
                    .unwrap_or_else(|error| panic!("{error:?}")),
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
            market.post_bids();
            market.record_deal();
            producers::seal_tasks(&mut market.bytes, AT)
                .unwrap_or_else(|error| panic!("{error:?}"));
            market
        }

        fn post_bids(&mut self) {
            let state =
                decode_shared_state(&self.bytes).unwrap_or_else(|error| panic!("{error:?}"));
            let workers = layerx_programs_ai_market::workers::WorkerTable::decode(
                layerx_programs_ai_market::evaluators::authority::split_identity_section(
                    state
                        .section(Section::IdentityRoster)
                        .unwrap_or_else(|error| panic!("{error:?}")),
                )
                .unwrap_or_else(|error| panic!("{error:?}"))
                .0,
            )
            .unwrap_or_else(|error| panic!("{error:?}"));
            for (n, bid) in [(20, 70), (21, 71)] {
                let worker = workers
                    .iter()
                    .find(|w| w.owner == actor(n))
                    .unwrap_or_else(|| panic!("enrolled worker missing"));
                self.call(
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
        }

        fn record_deal(&mut self) {
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
            self.call(
                dispatch::RecordDeal,
                owner(),
                &encoded(1400, |out| {
                    RecordDealRequest {
                        room_id: digest(60),
                        deal_id: digest(62),
                        ask_id: digest(61),
                        agreed_price_per_thousand_tokens: 1001,
                        roster_digest: self.roster,
                        layers,
                    }
                    .encode(out)
                }),
                START + 2,
            );
        }

        fn envelope(&self, op: dispatch::Operation, who: PrincipalId, payload: &[u8]) -> Vec<u8> {
            let sequence = if op.metadata().sequence == dispatch::SequencePolicy::ObjectLocal {
                0
            } else {
                producers::next_sequence(&self.bytes, who)
                    .unwrap_or_else(|error| panic!("{error:?}"))
            };
            let bytes = producers::envelope(&self.bytes, op, who, payload, sequence)
                .unwrap_or_else(|error| panic!("{error:?}"));
            if op.metadata().feature != 11 {
                return bytes;
            }
            let mut e = codec::decode_envelope(&bytes)
                .unwrap_or_else(|error| panic!("{error:?}"))
                .envelope;
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
                let room = r.digest32().unwrap_or_else(|error| panic!("{error:?}"));
                let bid = r.digest32().unwrap_or_else(|error| panic!("{error:?}"));
                r.take(48).unwrap_or_else(|error| panic!("{error:?}"));
                let amount = r.u128().unwrap_or_else(|error| panic!("{error:?}"));
                if layerx_programs_ai_market::bonds::bond_from_state(&self.bytes, bid)
                    .unwrap_or_else(|error| panic!("{error:?}"))
                    .is_none()
                {
                    self.call(
                        dispatch::LockBond,
                        who,
                        &layerx_programs_ai_market::bonds::LockBondRequest { room, bid, amount }
                            .encode()
                            .unwrap_or_else(|error| panic!("{error:?}")),
                        at,
                    );
                }
            }
            let e = self.envelope(op, who, payload);
            producers::apply(&mut self.bytes, &e, who, at)
                .unwrap_or_else(|error| panic!("{error:?}"));
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
                    .unwrap_or_else(|| panic!("enrolled worker missing"));
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
        let deal = must(book.deal(digest(62))).unwrap_or_else(|| panic!("recorded deal missing"));
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

    // A TCP host for the native program. Every submission is a signed client
    // transaction; the consumer recovers its caller and executes dispatch::route.
    // Both produced state and the accepted-operation journal survive a restart.
    struct ProgramNode {
        address: SocketAddr,
        worker: Option<JoinHandle<()>>,
    }
    fn write_packet(stream: &mut TcpStream, bytes: &[u8]) {
        must(stream.write_all(&must(u32::try_from(bytes.len())).to_be_bytes()));
        must(stream.write_all(bytes));
    }
    fn read_packet(stream: &mut TcpStream) -> Vec<u8> {
        let mut length = [0; 4];
        must(stream.read_exact(&mut length));
        let length = u32::from_be_bytes(length) as usize;
        assert!(length <= MAX_STATE_BYTES + MAX_ENVELOPE_BYTES + 4096);
        let mut bytes = vec![0; length];
        must(stream.read_exact(&mut bytes));
        bytes
    }
    fn exchange(address: SocketAddr, kind: u8, bytes: &[u8]) -> Vec<u8> {
        let mut stream = must(TcpStream::connect(address));
        must(stream.set_read_timeout(Some(Duration::from_secs(10))));
        must(stream.set_write_timeout(Some(Duration::from_secs(10))));
        must(stream.write_all(&[kind]));
        write_packet(&mut stream, bytes);
        read_packet(&mut stream)
    }
    impl ProgramNode {
        fn start(directory: &Path, initial: &[u8]) -> Self {
            let state_path = directory.join("program-state");
            let height_path = directory.join("program-height");
            let operations_path = directory.join("program-operations");
            if !state_path.exists() {
                must(fs::write(&state_path, initial));
                must(fs::write(&height_path, AT.to_be_bytes()));
                must(fs::write(&operations_path, []));
            }
            let mut state = must(fs::read(&state_path));
            must(decode_shared_state(&state));
            let height_bytes = must(fs::read(&height_path));
            let mut height = u64::from_be_bytes(must(height_bytes.as_slice().try_into()));
            let mut operations = must(fs::read(&operations_path));
            let listener = must(TcpListener::bind("127.0.0.1:0"));
            let address = must(listener.local_addr());
            let worker = std::thread::spawn(move || loop {
                let (mut stream, _) = must(listener.accept());
                must(stream.set_read_timeout(Some(Duration::from_secs(10))));
                must(stream.set_write_timeout(Some(Duration::from_secs(10))));
                let mut kind = [0];
                must(stream.read_exact(&mut kind));
                let bytes = read_packet(&mut stream);
                let response = match kind[0] {
                    0 => {
                        write_packet(&mut stream, &[]);
                        break;
                    }
                    1 => {
                        let mut out = height.to_be_bytes().to_vec();
                        out.extend_from_slice(&state);
                        out
                    }
                    2 => {
                        let (call, signature) = must(evm_transaction::decode_signed(&bytes));
                        assert_eq!(call.chain_id, 125);
                        let mut target = [0; 20];
                        target[18..].copy_from_slice(&[0x10, 0x1d]);
                        assert_eq!(call.to, target);
                        let sender = must(evm_transaction::recover_sender(&call, &signature));
                        let mut caller = [0; 32];
                        caller[12..].copy_from_slice(&sender);
                        let caller = must(PrincipalId::new(caller));
                        let (native, fee) = must(decode_program_call(&call.data));
                        assert_eq!(native.guest_abi, 5);
                        assert_eq!(native.entrypoint, b"layerx_call");
                        assert_eq!(fee, 1_000_000);
                        let (envelope, _) =
                            must(client::decode_operation_envelope(&native.calldata));
                        assert_eq!(native.program_id.bytes(), envelope.program.bytes());
                        assert_eq!(envelope.actor, caller);
                        let reply =
                            must(producers::compose(&state, &native.calldata, caller, height));
                        assert!(
                            matches!(reply.routed, Routed::Applied { .. }),
                            "program refused {:?}",
                            must(codec::decode_result(&reply.result)).error
                        );
                        let action = must(canary::value_action(
                            envelope.operation,
                            &state,
                            &reply.state,
                        ));
                        if envelope.operation == dispatch::ChallengeLap {
                            assert_eq!(action, value_adapter::ValueAction::Fund { amount: 100 });
                        }
                        state = reply.state;
                        operations.extend_from_slice(&envelope.operation.selector().to_be_bytes());
                        must(fs::write(state_path.with_extension("next"), &state));
                        must(fs::rename(state_path.with_extension("next"), &state_path));
                        must(fs::write(&operations_path, &operations));
                        reply.event
                    }
                    3 => {
                        let next = u64::from_be_bytes(must(bytes.as_slice().try_into()));
                        assert!(next >= height);
                        height = next;
                        must(fs::write(&height_path, height.to_be_bytes()));
                        Vec::new()
                    }
                    4 => operations.clone(),
                    _ => panic!("unknown native host operation"),
                };
                write_packet(&mut stream, &response);
            });
            Self {
                address,
                worker: Some(worker),
            }
        }
        fn height(&self, height: u64) {
            assert!(exchange(self.address, 3, &height.to_be_bytes()).is_empty());
        }
        fn operations(&self) -> Vec<dispatch::Operation> {
            exchange(self.address, 4, &[])
                .chunks_exact(2)
                .map(|bytes| {
                    must(dispatch::Operation::decode(u16::from_be_bytes(must(
                        bytes.try_into(),
                    ))))
                })
                .collect()
        }
    }
    impl Drop for ProgramNode {
        fn drop(&mut self) {
            if let Some(worker) = self.worker.take() {
                let _ = exchange(self.address, 0, &[]);
                must(worker.join());
            }
        }
    }
    struct NativeTransport {
        address: SocketAddr,
        nonce: u64,
    }
    impl CanaryTransport for NativeTransport {
        fn view(&mut self) -> Result<CanaryView, CanaryError> {
            let bytes = exchange(self.address, 1, &[]);
            let height = u64::from_be_bytes(must(bytes[..8].try_into()));
            Ok(CanaryView {
                state: bytes[8..].to_vec(),
                height,
            })
        }
        fn submit(&mut self, operation: &client::MarketOperation) -> Result<(), CanaryError> {
            let view = self.view()?;
            let shared = decode_shared_state(&view.state)?;
            let binding = must(epoch::captured_roster(&shared))
                .unwrap_or_else(|| panic!("frozen roster missing"))
                .binding;
            let sequence = if operation.operation().metadata().sequence
                == dispatch::SequencePolicy::ObjectLocal
            {
                0
            } else {
                producers::next_sequence(&view.state, owner())?
            };
            let mut request = [0x77; 32];
            request[..8].copy_from_slice(&shared.revision.to_be_bytes());
            request[8..10].copy_from_slice(&operation.operation().selector().to_be_bytes());
            let envelope = Envelope {
                operation: operation.operation(),
                chain: binding.chain,
                program: binding.program,
                market: binding.market,
                actor: owner(),
                epoch: binding.epoch,
                config: binding.config.get(),
                roster: Presence::Present(binding.roster),
                sequence,
                expiry: u64::MAX,
                request: RequestId::new(request)?,
                payload: operation.payload(),
                authentication: Authentication::Native,
            };
            let mut bytes = vec![0; MAX_ENVELOPE_BYTES];
            let n = client::encode_operation_envelope(&envelope, &mut bytes)?;
            bytes.truncate(n);
            let signer = native_signer(12);
            let operation_record = ready(sign_market_envelope(
                &bytes,
                &terms(&signer, request, self.nonce),
                &signer,
            ))?;
            assert_eq!(market_envelope(&operation_record)?, bytes);
            let event = exchange(self.address, 2, operation_record.signed_bytes());
            assert!(!event.is_empty());
            self.nonce += 1;
            Ok(())
        }
    }
    fn table() -> CanaryTable {
        CanaryTable {
            model_digest: digest(1).bytes(),
            entries: vec![CanaryExample {
                prompt_tokens: vec![41, 17, 93],
                expected_logits: EXPECTED,
            }],
        }
    }
    fn engine(directory: &Directory, host: &Arc<client::HostMeterKey>) -> CanaryEngine {
        must(CanaryEngine::open(&directory.0, Arc::clone(host), table()))
    }
    fn peer(plan: &CanaryPlan) -> client::HostMeterKey {
        [20_u8, 21]
            .into_iter()
            .map(|n| client::HostMeterKey::from_seed(&[0x90 + n; 32]))
            .find(|peer| Some(peer.public_key()) == plan.peer_key)
            .unwrap_or_else(|| panic!("selected peer missing"))
    }
    fn record(transport: &mut NativeTransport) -> canary::CanaryCommitment {
        let view = must(transport.view());
        let shared = must(decode_shared_state(&view.state));
        let region = must(canary::CanaryRegion::decode(shared.control.feature_bytes));
        let records = region.records().map(must).collect::<Vec<_>>();
        assert_eq!(records.len(), 1);
        records[0]
    }
    fn finish_lap(
        market: &Market,
        state: &[u8],
        directory: &Directory,
        host: &client::HostMeterKey,
    ) {
        let mut laps = must(LapEngine::open(
            &directory.0.join("laps.json"),
            state,
            frame(market),
            host.public_key(),
            1,
        ));
        let issued = must(laps.issue_lap(10, 32, vec![[90; 32], [91; 32]], true, 100, host));
        must(issued.receipt.verify(host.public_key()));
        for n in [20_u8, 21] {
            let key = client::HostMeterKey::from_seed(&[0x90 + n; 32]);
            must(laps.countersign(key.public_key(), &must(issued.countersign(&key)), 101));
        }
        assert!(must(laps.finish_lap(102, host)));
        assert_eq!(laps.receipts()[0].peer_countersignatures, 3);
    }

    #[test]
    fn sampling_key_is_secret_and_deterministic_per_deal_and_lap() {
        let host = client::HostMeterKey::from_seed(&[12; 32]);
        let key = client::canary_sampling_key(&host, [62; 32], 19);
        assert_eq!(key, client::canary_sampling_key(&host, [62; 32], 19));
        assert_ne!(key, client::canary_sampling_key(&host, [63; 32], 19));
        assert_ne!(key, client::canary_sampling_key(&host, [62; 32], 20));
        assert_ne!(
            key,
            client::canary_sampling_key(&client::HostMeterKey::from_seed(&[13; 32]), [62; 32], 19)
        );
        let positions = must(canary_positions(&key, 256, 10_000));
        assert_eq!(positions, must(canary_positions(&key, 256, 10_000)));
        assert_eq!(positions.len(), 1);
        assert!(positions[0] < 256);
        assert_eq!(must(canary_positions(&key, 1, 10_000)), vec![0]);
        assert!(must(canary_positions(&key, 256, 0)).is_empty());
        assert!(canary_positions(&key, 0, 250).is_err());
        assert!(canary_positions(&key, 256, 10_001).is_err());
        let selected = (0_u64..10_000)
            .filter(|index| {
                let mut key = [0; 32];
                key[..8].copy_from_slice(&index.to_be_bytes());
                !must(canary_positions(&key, 256, 250)).is_empty()
            })
            .count();
        assert!(
            (200..=300).contains(&selected),
            "room-ratio selected {selected} laps"
        );
    }

    #[test]
    fn correct_answer_leaves_no_challenge_after_a_real_lap() {
        let market = Market::new(10_000);
        let directory = Directory::new();
        let host = Arc::new(client::HostMeterKey::from_seed(&[12; 32]));
        let node = ProgramNode::start(&directory.0, &market.bytes);
        let mut transport = NativeTransport {
            address: node.address,
            nonce: 0,
        };
        let engine = engine(&directory, &host);
        let plan = must(engine.prepare(&market.bytes, [62; 32], 0, 32));
        let mut prompts = vec![vec![7]; 32];
        must(plan.inject(&mut prompts));
        assert_eq!(prompts.len(), 33);
        assert_eq!(
            prompts[must(usize::try_from(
                plan.position
                    .unwrap_or_else(|| panic!("canary position missing"))
            ))],
            table().entries[0].prompt_tokens
        );
        assert_eq!(
            prompts.iter().filter(|prompt| **prompt == vec![7]).count(),
            32
        );
        assert!(engine
            .require_committed(&market.bytes, [62; 32], 0, 32)
            .is_err());
        assert!(engine.audit_answer(&market.bytes, [62; 32], 0).is_err());
        let answer = must(CanaryAnswer::sign(&plan, EXPECTED, &peer(&plan)));
        must(engine.accept_answer(answer.clone()));
        must(engine.accept_answer(answer));
        assert!(!must(engine.advance(&mut transport, [62; 32], 0)));
        let committed = must(transport.view()).state;
        must(engine.require_committed(&committed, [62; 32], 0, 32));
        assert_eq!(record(&mut transport).status, ChallengeStatus::Unchallenged);
        finish_lap(&market, &committed, &directory, &host);
        assert!(must(engine.audit_answer(&committed, [62; 32], 0)));
        assert!(must(engine.advance(&mut transport, [62; 32], 0)));
        let final_record = record(&mut transport);
        assert_eq!(final_record.status, ChallengeStatus::Unchallenged);
        assert_eq!(final_record.challenger, None);
        assert_eq!(final_record.revealed_height, None);
        assert!(!final_record.peer_at_fault());
        assert_eq!(node.operations(), vec![dispatch::CommitScore]);
    }

    #[test]
    fn wrong_answer_commits_challenges_reveals_and_resolves_across_restart() {
        let market = Market::new(10_000);
        let directory = Directory::new();
        let host = Arc::new(client::HostMeterKey::from_seed(&[12; 32]));
        let mut node = ProgramNode::start(&directory.0, &market.bytes);
        let mut transport = NativeTransport {
            address: node.address,
            nonce: 0,
        };
        let initial = engine(&directory, &host);
        let plan = must(initial.prepare(&market.bytes, [62; 32], 0, 32));
        let mut wrong = EXPECTED;
        wrong[3] = 1;
        let answer = must(CanaryAnswer::sign(&plan, wrong, &peer(&plan)));
        let mut forged = answer.clone();
        forged.signature[0] ^= 1;
        assert!(initial.accept_answer(forged).is_err());
        must(initial.accept_answer(answer.clone()));
        drop(initial);
        let recovered = engine(&directory, &host);
        assert!(!must(recovered.advance(&mut transport, [62; 32], 0)));
        assert_eq!(node.operations(), vec![dispatch::CommitScore]);
        let committed = must(transport.view()).state;
        must(recovered.require_committed(&committed, [62; 32], 0, 32));
        finish_lap(&market, &committed, &directory, &host);
        assert!(!must(recovered.audit_answer(&committed, [62; 32], 0)));
        let conflicting = must(CanaryAnswer::sign(&plan, EXPECTED, &peer(&plan)));
        assert!(recovered.accept_answer(conflicting).is_err());
        assert!(!must(recovered.advance(&mut transport, [62; 32], 0)));
        assert_eq!(record(&mut transport).status, ChallengeStatus::Open);
        assert_eq!(
            node.operations(),
            vec![dispatch::CommitScore, dispatch::ChallengeLap]
        );
        assert!(!must(recovered.advance(&mut transport, [62; 32], 0)));
        assert_eq!(node.operations().len(), 2);
        node.height(REVEAL);
        assert!(!must(recovered.advance(&mut transport, [62; 32], 0)));
        assert_eq!(
            node.operations(),
            vec![
                dispatch::CommitScore,
                dispatch::ChallengeLap,
                dispatch::RevealScore
            ]
        );
        assert_eq!(record(&mut transport).expected_logits, EXPECTED);
        drop(recovered);
        drop(node);
        node = ProgramNode::start(&directory.0, &[]);
        transport.address = node.address;
        let recovered = engine(&directory, &host);
        must(recovered.accept_answer(answer));
        assert!(!must(recovered.advance(&mut transport, [62; 32], 0)));
        assert!(must(recovered.advance(&mut transport, [62; 32], 0)));
        let final_record = record(&mut transport);
        assert_eq!(final_record.status, ChallengeStatus::Miss);
        assert!(final_record.peer_at_fault());
        assert_eq!(
            final_record.task.answer_digest,
            must(canary::logits_digest(&wrong))
        );
        assert_eq!(
            node.operations(),
            vec![
                dispatch::CommitScore,
                dispatch::ChallengeLap,
                dispatch::RevealScore,
                dispatch::ResolveChallenge
            ]
        );
    }

    #[test]
    fn room_tolerance_and_zero_ratio_are_enforced_without_challenges() {
        for ratio in [0, 10_000] {
            let market = Market::new(ratio);
            let directory = Directory::new();
            let host = Arc::new(client::HostMeterKey::from_seed(&[12; 32]));
            let node = ProgramNode::start(&directory.0, &market.bytes);
            let mut transport = NativeTransport {
                address: node.address,
                nonce: 0,
            };
            let engine = engine(&directory, &host);
            let plan = must(engine.prepare(&market.bytes, [62; 32], 0, 32));
            if ratio == 0 {
                assert_eq!(plan.position, None);
                assert!(plan.prompt_tokens.is_empty());
                assert!(must(engine.advance(&mut transport, [62; 32], 0)));
            } else {
                let mut close = EXPECTED;
                close[0] += 5242;
                must(engine.accept_answer(must(CanaryAnswer::sign(&plan, close, &peer(&plan)))));
                assert!(!must(engine.advance(&mut transport, [62; 32], 0)));
            }
            let committed = must(transport.view()).state;
            must(engine.require_committed(&committed, [62; 32], 0, 32));
            finish_lap(&market, &committed, &directory, &host);
            assert!(must(engine.audit_answer(&committed, [62; 32], 0)));
            must(engine.tick(&mut transport));
            assert_eq!(
                node.operations(),
                if ratio == 0 {
                    vec![]
                } else {
                    vec![dispatch::CommitScore]
                }
            );
        }
    }

    #[test]
    fn model_and_journal_bindings_refuse_substitution() {
        let market = Market::new(10_000);
        let directory = Directory::new();
        let host = Arc::new(client::HostMeterKey::from_seed(&[12; 32]));
        let engine = engine(&directory, &host);
        let plan = must(engine.prepare(&market.bytes, [62; 32], 0, 32));
        assert!(engine.prepare(&market.bytes, [62; 32], 0, 31).is_err());
        let mut changed = table();
        changed.model_digest = [2; 32];
        let other = must(CanaryEngine::open(&directory.0, Arc::clone(&host), changed));
        assert!(other.prepare(&market.bytes, [62; 32], 0, 32).is_err());
        let mut answer = must(CanaryAnswer::sign(&plan, EXPECTED, &peer(&plan)));
        answer.lap_index = 1;
        assert!(engine.accept_answer(answer).is_err());
        let mut wrong_length = vec![vec![7]; 31];
        assert!(plan.inject(&mut wrong_length).is_err());
        must(engine.accept_answer(must(CanaryAnswer::sign(&plan, EXPECTED, &peer(&plan)))));
        let journal = directory
            .0
            .join(format!("canary-{}-0.json", hex::encode([62; 32])));
        let mut value: serde_json::Value = must(serde_json::from_slice(&must(fs::read(&journal))));
        value["expected"][0] = serde_json::json!(EXPECTED[0] + 1);
        must(fs::write(&journal, must(serde_json::to_vec(&value))));
        assert!(engine.prepare(&market.bytes, [62; 32], 0, 32).is_err());
    }
}
