//! Market state producers and persistence assertions.

// This integration target is also included by the existing suites for its shared, real
// producer fixtures. It has no subprocess, artifact or environment dependency.
#![allow(dead_code)]
use ed25519_dalek::{Signer, SigningKey};
use layerx_ai_market_test_values::RequiredValue as _;
use layerx_programs_ai_market::{
    admission::{AdmissionContext, AdmissionTable, ApprovalTerms, EvaluatorConsent, Participant},
    aggregation::{self},
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
    run().inspect_err(|&error| {
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
            let state = fixture_step("header", "decode_shared_state (codec.rs:schema)", || {
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
            AdmissionTable::decode(table).inspect_err(|_| {
        eprintln!("fixture step=table operation=AdmissionTable::decode decoder=admission.rs:660 prefix={:?} length={}", table.get(..2), table.len());
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
pub fn compose(bytes: &[u8], encoded: &[u8], actor: PrincipalId, at: u64) -> CodecResult<Reply> {
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
            super::observe(
                &CallContext {
                    chain: market.deployment_chain_domain,
                    program: market.program_id,
                    principal: actor,
                    height: at,
                },
                bytes,
                encoded,
                &next,
                &event,
                &result,
                routed,
            );
            Ok(Reply {
                routed,
                state: next,
                event,
                result,
            })
        },
    )
}
fn crate_error_name(error: layerx_programs_ai_market::errors::ApplicationError) -> &'static str {
    if error == layerx_programs_ai_market::errors::F11_BOND_INSUFFICIENT {
        return "F11_BOND_INSUFFICIENT";
    }
    layerx_programs_ai_market::errors::APPLICATION_ERRORS
        .iter()
        .find(|entry| entry.code == error.code())
        .required_with("registered application error")
        .name
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
            let name = crate_error_name(error);
            eprintln!(
                "{} at {at} refused: {name} ({} / 0x{:04x})",
                operation.metadata().name,
                error.code(),
                error.code(),
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
        let sequence = if operation.metadata().sequence == dispatch::SequencePolicy::ObjectLocal {
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
            let consent =
                workers::consent_digest(&market, worker, owner, public, 1, 1, metadata, expiry)?;
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
            let identity = codec::domain_hash("PAXAI/evaluator-grant/v1", &encoded_grant[..len])?;
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
pub fn refresh_workers(bytes: &mut Vec<u8>, opening: u64) -> CodecResult<()> {
    refresh_workers_with(bytes, opening, |record| {
        fixture_delegate(record.owner.bytes()[0])
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
                manifest.extend_from_slice(&(u32::try_from(uri.len()).required()).to_be_bytes());
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
                payload
                    .extend_from_slice(&(u32::try_from(manifest.len()).required()).to_be_bytes());
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
            let mut cursor = 0;
            let mut count = None;
            loop {
                if cursor != 0 {
                    assert!(epoch::captured_roster(&decode_shared_state(bytes)?)?.is_none());
                }
                let reply = apply(bytes, &out[..len], actor, at)?;
                let result = decode_result(&reply.result)?;
                assert_eq!(result.status, ResultStatus::Ok);
                assert_eq!(result.payload.len(), epoch::OPEN_SUFFIX_BYTES);
                let mut progress = codec::Reader::new(&result.payload[100..]);
                let next_cursor = progress.u16()?;
                let participant_count = progress.u16()?;
                assert!(next_cursor > cursor || (cursor == 0 && participant_count == 0));
                assert!(next_cursor <= participant_count);
                if let Some(count) = count {
                    assert_eq!(participant_count, count);
                }
                count = Some(participant_count);
                cursor = next_cursor;
                let captured = epoch::captured_roster(&decode_shared_state(bytes)?)?;
                if cursor == participant_count {
                    let captured =
                        captured.required_with("the final opening call publishes the roster");
                    assert_eq!(captured.binding.epoch, frozen.epoch);
                    assert_eq!(captured.binding.config, frozen.config);
                    assert_eq!(captured.binding.roster, frozen.roster);
                    assert_eq!(captured.workers().len(), usize::from(frozen.workers));
                    assert_eq!(captured.evaluators().len(), usize::from(frozen.evaluators));
                    break;
                }
                assert!(
                    captured.is_none(),
                    "a partial opening cannot publish a roster"
                );
            }
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
        match sealed_task_set(bytes, frozen.epoch) {
            Ok(retained) => {
                assert_eq!(retained, digest);
                return Ok(());
            }
            Err(layerx_programs_ai_market::errors::F09_EVIDENCE_TASK_SET_UNSEALED) => {}
            Err(error) => return Err(error),
        }
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
        assert_eq!(sealed_task_set(bytes, frozen.epoch)?, digest);
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
            let mut cursor = 0;
            let count = aggregation::progress(bytes)?.worker_count;
            loop {
                let reply = call(
                    bytes,
                    dispatch::FinalizeAggregation,
                    keeper,
                    input.as_bytes(),
                    at,
                )?;
                let result = decode_result(&reply.result)?;
                assert_eq!(result.status, ResultStatus::Ok);
                assert_eq!(result.payload.len(), aggregation::FINALIZE_RESPONSE_BYTES);
                let mut progress = codec::Reader::new(&result.payload[64..]);
                let worker_count = progress.u16()?;
                let weight = progress.u64()?;
                let next_cursor = progress.u16()?;
                assert_eq!(worker_count, count);
                assert!(next_cursor > cursor || (cursor == 0 && count == 0));
                assert!(next_cursor <= count);
                cursor = next_cursor;
                let committed = aggregation::progress(bytes)?;
                assert_eq!(committed.worker_count, count);
                assert_eq!(committed.running_weight, weight);
                assert_eq!(
                    committed.phase == AggregationPhase::Terminal,
                    cursor == count
                );
                if cursor == count {
                    break;
                }
            }
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
/// Four admissions per opened epoch, with real terminal steps and heartbeats in between.
/// Returns the height at which the final roster can be opened (epoch 12 for 32 workers).
#[allow(
    clippy::too_many_arguments,
    reason = "The shared producer API preserves each explicit identity and roster input."
)]
pub fn grow(
    bytes: &mut Vec<u8>,
    owner_sequence: &mut u64,
    first: WorkerRosterEntry,
    workers: u8,
    principal: fn(u8) -> CodecResult<PrincipalId>,
    delegate: fn(u8) -> SigningKey,
    evaluator_key: fn(u8) -> SigningKey,
    metadata: MetadataDigest,
) -> CodecResult<(Vec<WorkerRosterEntry>, u64)> {
    fixture_step(
        "grow",
        "Fund -> OpenEpoch -> Heartbeat -> admissions -> aggregation",
        || {
            let origin = header(bytes)?.origin_height;
            let market_owner = header(bytes)?.owner_principal;
            let mut funding = 500u128.to_be_bytes().to_vec();
            funding.extend_from_slice(header(bytes)?.refund_recipient_account.as_bytes());
            funding.extend_from_slice(&1u64.to_be_bytes());
            funding.push(1);
            call(bytes, dispatch::FUND, market_owner, &funding, origin + 8)?;
            let mut roster = vec![first];
            let mut pending = Vec::new();
            for n in 5..=9 {
                pending.push((false, n));
            }
            for n in 0x21..0x20 + workers {
                pending.push((true, n));
            }
            let mut epoch = 1;
            for batch in pending.chunks(4) {
                let start = origin + 128 * epoch;
                open(bytes, start)?;
                heartbeat(bytes, start + 1)?;
                for &(worker, n) in batch {
                    if worker {
                        roster.push(enroll(
                            bytes,
                            owner_sequence,
                            principal(n)?,
                            n,
                            &delegate(n),
                            metadata,
                            start + 2,
                        )?);
                    } else {
                        evaluator(
                            bytes,
                            owner_sequence,
                            principal(n)?,
                            n,
                            &evaluator_key(n),
                            start + 2,
                        )?;
                    }
                }
                settle(bytes, start + 96)?;
                epoch += 1;
            }
            *owner_sequence = next_sequence(bytes, market_owner)?;
            Ok((roster, origin + 128 * epoch.max(6)))
        },
    )
}

pub fn settlement_bytes(bytes: &[u8]) -> CodecResult<Vec<u8>> {
    Ok(decode_shared_state(bytes)?.feature_sections[Section::SettlementClaims.index()].to_vec())
}
/// Produce the scores consumed by the reward tests through registered evidence, commit
/// and reveal operations. The three independent frozen evaluators sign the same ballot.
pub fn prepare_scores(
    bytes: &mut Vec<u8>,
    weights: &[(WorkerRosterEntry, u32)],
    key: fn(u8) -> SigningKey,
) -> CodecResult<()> {
    fixture_step(
        "prepare_scores",
        "sealed_task_set -> admitted_report -> SealEvidence -> CommitScore -> RevealScore",
        || {
            let market = header(bytes)?;
            let state = decode_shared_state(bytes)?;
            let captured = epoch::captured_roster(&state)?.ok_or(NOT_FOUND)?;
            let frozen = captured.binding;
            let start = market.origin_height + 128 * frozen.epoch;
            let voters = captured
                .evaluators()
                .iter()
                .take(3)
                .copied()
                .collect::<Vec<_>>();
            seal_tasks(bytes, start + 64)?;
            if weights.iter().all(|(_, weight)| *weight == 0) {
                return Ok(());
            }
            let scores = prepared_scores(weights);
            let policy = PolicySection::decode(
                decode_shared_state(bytes)?.feature_sections[Section::PolicyLifecycle.index()],
            )?
            .current;
            let task_set = sealed_task_set(bytes, frozen.epoch)?;
            let salt = Salt32::new([0x5a; 32])?;
            for voter in voters {
                // Reentry for a prepared late-settlement fixture reads the actual producer row.
                if read_admission_view(bytes, |state| {
                    layerx_programs_ai_market::evaluators::admission::admitted_report(
                        state,
                        frozen.epoch,
                        voter.evaluator,
                    )
                    .map(|report| report.is_some())
                })? {
                    continue;
                }
                let owner = voter.owner;
                let n = owner.bytes()[0];
                let evidence = EvidenceRoot::new([n; 32])?;
                let mut claim = 1u16.to_be_bytes().to_vec();
                claim.extend_from_slice(evidence.as_bytes());
                claim.extend_from_slice(policy.digest()?.as_bytes());
                claim.extend_from_slice(task_set.as_bytes());
                claim.extend_from_slice(policy.commitments.rubric.as_bytes());
                claim.push(1);
                call(bytes, dispatch::SealEvidence, owner, &claim, start + 64)?;
                let binding = EvaluatorBinding {
                    frozen,
                    evaluator: voter.evaluator,
                    grant: voter.grant,
                    key_version: voter.key_version,
                };
                let body = codec::ReportBody {
                    binding,
                    evidence,
                    scores: codec::ScoreVector::Encoded(&scores),
                };
                let digest = codec::report_digest(&body)?;
                let signature = Signature64(
                    key(n)
                        .sign(codec::attestation_digest(digest)?.as_bytes())
                        .to_bytes(),
                );
                let commitment = codec::commitment_digest(&binding, digest, salt)?;
                let mut payload = vec![
                        0;
                        layerx_programs_ai_market::commit_reveal::commitment::COMMIT_SCORE_BYTES
                    ];
                codec::encode_commit_score(
                    &codec::CommitScorePayload {
                        binding,
                        commitment,
                    },
                    &mut payload,
                )?;
                call(bytes, dispatch::CommitScore, owner, &payload, start + 64)?;
                let mut payload =
                    vec![0; layerx_programs_ai_market::commit_reveal::commitment::REVEAL_MAX_BYTES];
                let len = codec::encode_reveal_score(
                    &codec::RevealScorePayload {
                        report: body,
                        signature,
                        salt,
                    },
                    &mut payload,
                )?;
                call(
                    bytes,
                    dispatch::RevealScore,
                    owner,
                    &payload[..len],
                    start + 80,
                )?;
            }
            Ok(())
        },
    )
}
fn prepared_scores(weights: &[(WorkerRosterEntry, u32)]) -> Vec<u8> {
    let mut ballot = weights
        .iter()
        .map(|(w, score)| (w.worker, *score))
        .collect::<Vec<_>>();
    ballot.sort_by_key(|(w, _)| *w);
    let mut scores = Vec::new();
    for (worker, score) in ballot {
        scores.extend_from_slice(worker.as_bytes());
        scores.extend_from_slice(&score.to_be_bytes());
    }
    scores
}

fn fixture_principal(n: u8) -> CodecResult<PrincipalId> {
    let mut p = [0x50; 32];
    p[0] = n;
    PrincipalId::new(p)
}
fn fixture_delegate(n: u8) -> SigningKey {
    SigningKey::from_bytes(&[0x90 + n; 32])
}
fn fixture_evaluator(n: u8) -> SigningKey {
    SigningKey::from_bytes(&[n; 32])
}
fn create_fixture() -> CodecResult<Vec<u8>> {
    fixture_step("create_fixture", "Create", || {
        let chain = ChainDomain::new([10; 32])?;
        let program = ProgramId::new([11; 32])?;
        let owner = PrincipalId::new([12; 32])?;
        let policy = TaskPolicyV1::bounded_default(
            1,
            1,
            PolicyCommitments {
                model_artifact: Digest32::new([1; 32])?,
                dataset_artifact: [2; 32],
                benchmark_suite: Digest32::new([3; 32])?,
                rubric: RubricDigest::new([4; 32])?,
                task_schema: Digest32::new([5; 32])?,
                result_schema: Digest32::new([6; 32])?,
                service_terms: Digest32::new([7; 32])?,
            },
            101,
            1,
        )?;
        let mut policy_bytes = [0; TASK_POLICY_BYTES];
        policy.encode(&mut policy_bytes)?;
        let mut payload = owner.bytes().to_vec();
        payload.extend_from_slice(&[13; 32]);
        payload.extend_from_slice(
            derive_rewards_account(program, AssetId::new([13; 32])?)?.as_bytes(),
        );
        payload.extend_from_slice(&[14; 32]);
        payload.push(0);
        payload.extend_from_slice(&policy_bytes);
        payload.extend_from_slice(&[16; 32]);
        let e = Envelope {
            operation: dispatch::CREATE,
            chain,
            program,
            market: codec::derive_market(chain, program)?,
            actor: owner,
            epoch: 0,
            config: 1,
            roster: Presence::Absent,
            sequence: 1,
            expiry: u64::MAX,
            request: RequestId::new([1; 32])?,
            payload: &payload,
            authentication: Authentication::Native,
        };
        let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
        let len = encode_envelope(&e, &mut encoded)?;
        let mut next = vec![0; MAX_STATE_BYTES];
        let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
        let mut event = vec![0; MAX_EVENT_BYTES];
        let mut result = vec![0; MAX_RESULT_BYTES];
        let routed = dispatch::route(
            &CallContext {
                chain,
                program,
                principal: owner,
                height: 128,
            },
            &encoded[..len],
            None,
            Buffers {
                next: &mut next,
                scratch: &mut scratch,
                event: &mut event,
                result: &mut result,
            },
        )?;
        let Routed::Applied { state_len, .. } = routed else {
            return Err(NON_CANONICAL);
        };
        next.truncate(state_len);
        Ok(next)
    })
}
fn small_fixture() -> CodecResult<(Vec<u8>, WorkerRosterEntry, u64)> {
    let mut bytes = create_fixture()?;
    let mut seq = 2;
    let first = enroll(
        &mut bytes,
        &mut seq,
        fixture_principal(0x20)?,
        0x20,
        &fixture_delegate(0x20),
        MetadataDigest::new([0x44; 32])?,
        130,
    )?;
    for n in 2..=4 {
        evaluator(
            &mut bytes,
            &mut seq,
            fixture_principal(n)?,
            n,
            &fixture_evaluator(n),
            130,
        )?;
    }
    let state = decode_shared_state(&bytes)?;
    let joint = state.feature_sections[Section::ReputationAdmission.index()];
    assert_eq!(joint.get(..4), Some(b"RP07".as_slice()));
    assert!(
        matches!(
            AdmissionTable::decode(joint),
            Err(layerx_programs_ai_market::errors::BAD_VERSION)
        ),
        "admission.rs:660 refuses the RP07 prefix, which is not the F08 schema"
    );
    assert_eq!(table(&bytes)?.len(), 4);
    Ok((bytes, first, seq))
}
fn fund_fixture(bytes: &mut Vec<u8>, at: u64) -> CodecResult<()> {
    let mut fund = 500u128.to_be_bytes().to_vec();
    fund.extend_from_slice(&[14; 32]);
    fund.extend_from_slice(&1u64.to_be_bytes());
    fund.push(1);
    call(
        bytes,
        dispatch::FUND,
        PrincipalId::new([12; 32])?,
        &fund,
        at,
    )?;
    Ok(())
}
fn activate_fixture(bytes: &mut Vec<u8>, epoch: u64, start: u64) -> CodecResult<Frozen> {
    fixture_step(
        "activate_fixture",
        "ScheduleActivation -> OpenEpoch -> AdvanceActivation",
        || {
            let owner = PrincipalId::new([12; 32])?;
            let keeper = fixture_principal(0x7f)?;
            let mut schedule = decode_shared_state(bytes)?.revision.to_be_bytes().to_vec();
            schedule.extend_from_slice(&epoch.to_be_bytes());
            call(
                bytes,
                dispatch::SCHEDULE_ACTIVATION,
                owner,
                &schedule,
                start - 32,
            )?;
            let frozen = open(bytes, start)?;
            let mut advance = decode_shared_state(bytes)?.revision.to_be_bytes().to_vec();
            advance.extend_from_slice(&epoch.to_be_bytes());
            call(
                bytes,
                dispatch::ADVANCE_ACTIVATION,
                keeper,
                &advance,
                start + 1,
            )?;
            Ok(frozen)
        },
    )
}
fn prove_admissions() -> CodecResult<()> {
    let (mut bytes, first, mut seq) = small_fixture()?;
    let (workers, opening) = grow(
        &mut bytes,
        &mut seq,
        first,
        32,
        fixture_principal,
        fixture_delegate,
        fixture_evaluator,
        MetadataDigest::new([0x44; 32])?,
    )?;
    assert_eq!(opening, 1408); // Ten real openings are needed at the four-admission limit.
    let frozen = activate_fixture(&mut bytes, 10, opening)?;
    assert_eq!(
        (frozen.epoch, frozen.workers, frozen.evaluators),
        (10, 32, 8)
    );
    let admissions = table(&bytes)?;
    assert_eq!(admissions.len(), 40);
    assert_eq!(workers.len(), 32);
    let mut heights = std::collections::BTreeMap::new();
    for member in admissions.iter() {
        assert!(member.admitted());
        assert!(member.approval.is_none());
        assert_eq!(member.membership_generation, 1);
        let at = member.admission_height.ok_or(NOT_FOUND)?;
        *heights.entry((at - 128) / 128).or_insert(0usize) += 1;
        assert!(member.admitted_epoch.is_some());
        match member.participant {
            Participant::Worker(id) => assert_eq!(
                id,
                codec::derive_worker(
                    header(&bytes)?.market_id,
                    member.owner,
                    [member.owner.bytes()[0]; 32]
                )?
            ),
            Participant::Evaluator(id) => assert_eq!(
                id,
                codec::derive_evaluator(
                    header(&bytes)?.market_id,
                    member.owner,
                    [member.owner.bytes()[0]; 32]
                )?
            ),
        }
    }
    assert!(heights.values().all(|n| *n <= 4));
    assert_eq!(heights.len(), 10);
    assert_eq!(
        header(&bytes)?.lifecycle,
        layerx_programs_ai_market::registry_ops::ACTIVE
    );
    // A different native principal cannot consume another participant's heartbeat/role.
    let member = admissions
        .get(Participant::Worker(first.worker))
        .ok_or(NOT_FOUND)?;
    let mut payload = vec![1];
    payload.extend_from_slice(first.worker.as_bytes());
    for n in [member.membership_generation, member.delegate_generation, 10] {
        payload.extend_from_slice(&n.to_be_bytes());
    }
    let encoded = envelope(
        &bytes,
        dispatch::Heartbeat,
        first.owner,
        &payload,
        next_sequence(&bytes, first.owner)?,
    )?;
    let before = bytes.clone();
    let denied = compose(&bytes, &encoded, fixture_principal(0x60)?, opening + 32)?;
    assert!(matches!(denied.routed, Routed::Refused { .. }));
    assert_eq!(
        decode_result(&denied.result)?.error,
        Some(layerx_programs_ai_market::errors::UNAUTHORIZED)
    );
    assert!(denied.event.is_empty());
    assert_eq!(bytes, before);
    Ok(())
}

pub fn lap_batch(
    deal: Digest32,
    first: u64,
    count: usize,
    countersigners: u32,
) -> CodecResult<layerx_programs_ai_market::laps::ReceiptBatch> {
    use layerx_programs_ai_market::laps::{LapReceipt, ReceiptBatch};

    let ranges = [Digest32::new([80; 32])?, Digest32::new([81; 32])?];
    let key = SigningKey::from_bytes(&[12; 32]);
    let receipts = (0..count)
        .map(|index| {
            let mut receipt = LapReceipt {
                deal_id: deal,
                lap_index: first + index as u64,
                prompt_tokens: 10,
                generated_tokens: 90,
                cumulative_tokens: (first + index as u64 + 1) * 100,
                layer_range_digests: &ranges,
                host_signature: Signature64([1; 64]),
                peer_countersignatures: countersigners,
            };
            receipt.host_signature =
                Signature64(key.sign(receipt.signing_digest()?.as_bytes()).to_bytes());
            Ok(receipt)
        })
        .collect::<CodecResult<Vec<_>>>()?;
    ReceiptBatch::from_receipts(&receipts)
}
