use layerx_ai_market_test_values::RequiredValue as _;
#[path = "repair_22_33_260.rs"]
mod producers;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs_ai_market::{
    canary::{
        self, CanaryLogits, CanaryRegion, CanaryReveal, CanaryRoomProfile, CanaryTask,
        ChallengeLapPayload, ChallengeStatus, ResolveChallengePayload,
    },
    codec::{self, Envelope, ResultStatus},
    commit_reveal,
    dispatch::{self, Buffers, Operation, Routed},
    epoch,
    errors::{self, ApplicationError, CodecResult},
    policy::{PolicyCommitments, RoomProfile, TaskPolicyV1, TASK_POLICY_BYTES},
    registry::derive_rewards_account,
    registry_ops::CallContext,
    state::{decode_shared_state, encode_shared_state, ActorSlot, Section},
    types::*,
    value_adapter::ValueAction,
    MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
};

const ORIGIN: u64 = 128;
const START: u64 = ORIGIN + 6 * 128;
const COMMIT: u64 = START + 64;
const REVEAL: u64 = START + 80;
const EXPECTED: CanaryLogits = [
    524_288, 262_144, 65_536, 0, -65_536, -131_072, -262_144, -524_288,
];
const DEPOSIT: u128 = 7;

fn owner() -> CodecResult<PrincipalId> {
    PrincipalId::new([12; 32])
}
fn principal(n: u8) -> CodecResult<PrincipalId> {
    let mut bytes = [0x50; 32];
    bytes[0] = n;
    PrincipalId::new(bytes)
}
fn peer_key() -> SigningKey {
    SigningKey::from_bytes(&[0xb0; 32])
}

fn create() -> CodecResult<Vec<u8>> {
    let chain = ChainDomain::new([10; 32])?;
    let program = ProgramId::new([11; 32])?;
    let asset = AssetId::new([13; 32])?;
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
        100,
        1,
    )?;
    let mut task_policy = [0; TASK_POLICY_BYTES];
    policy.encode(&mut task_policy)?;
    let mut payload = owner()?.bytes().to_vec();
    payload.extend_from_slice(asset.as_bytes());
    payload.extend_from_slice(derive_rewards_account(program, asset)?.as_bytes());
    payload.extend_from_slice(&[14; 32]);
    payload.push(0);
    payload.extend_from_slice(&task_policy);
    payload.extend_from_slice(&[16; 32]);
    let e = Envelope {
        operation: dispatch::CREATE,
        chain,
        program,
        market: codec::derive_market(chain, program)?,
        actor: owner()?,
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
    let n = codec::encode_envelope(&e, &mut encoded)?;
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    let routed = dispatch::route(
        &CallContext {
            chain,
            program,
            principal: owner()?,
            height: ORIGIN,
        },
        &encoded[..n],
        None,
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?;
    let Routed::Applied { state_len, .. } = routed else {
        return Err(errors::NON_CANONICAL);
    };
    next.truncate(state_len);
    Ok(next)
}

struct Market {
    bytes: Vec<u8>,
    peer: WorkerId,
}
impl Market {
    fn new() -> CodecResult<Self> {
        let mut bytes = create()?;
        let mut sequence = 2;
        let peer = producers::enroll(
            &mut bytes,
            &mut sequence,
            principal(0x20)?,
            0x20,
            &peer_key(),
            MetadataDigest::new([0x44; 32])?,
            ORIGIN + 2,
        )?
        .worker;
        for n in 2..=4 {
            producers::evaluator(
                &mut bytes,
                &mut sequence,
                principal(n)?,
                n,
                &SigningKey::from_bytes(&[n; 32]),
                ORIGIN + 2,
            )?;
        }
        let mut fund = 500u128.to_be_bytes().to_vec();
        fund.extend_from_slice(&[14; 32]);
        fund.extend_from_slice(&1u64.to_be_bytes());
        fund.push(1);
        producers::call(&mut bytes, dispatch::FUND, owner()?, &fund, ORIGIN + 7)?;
        let mut schedule = decode_shared_state(&bytes)?.revision.to_be_bytes().to_vec();
        schedule.extend_from_slice(&6u64.to_be_bytes());
        producers::call(
            &mut bytes,
            dispatch::SCHEDULE_ACTIVATION,
            owner()?,
            &schedule,
            START - 32,
        )?;
        producers::open(&mut bytes, START)?;
        let mut advance = decode_shared_state(&bytes)?.revision.to_be_bytes().to_vec();
        advance.extend_from_slice(&6u64.to_be_bytes());
        producers::call(
            &mut bytes,
            dispatch::ADVANCE_ACTIVATION,
            principal(0x7f)?,
            &advance,
            START + 1,
        )?;
        Ok(Self { bytes, peer })
    }
    fn binding(&self) -> CodecResult<FrozenBinding> {
        Ok(epoch::captured_roster(&decode_shared_state(&self.bytes)?)?
            .ok_or(errors::WRONG_EPOCH)?
            .binding)
    }
    fn task(&self, bps: u16, answer: &CanaryLogits, lap: u64) -> CodecResult<CanaryTask> {
        let mut task = CanaryTask {
            deal: Digest32::new([42; 32])?,
            lap,
            peer: self.peer,
            prompt_digest: codec::domain_hash(
                "PAXAI/canary-prompt/v1",
                b"canary prompt position zero",
            )?,
            expected_logits_digest: canary::logits_digest(&EXPECTED)?,
            answer_digest: canary::logits_digest(answer)?,
            profile: CanaryRoomProfile {
                room: RoomProfile {
                    minimum_bond: DEPOSIT,
                    canary_ratio_bps: 250,
                    lap_size_tokens: 256,
                    challenge_window_blocks: 10,
                    slash_fraction_bps: 2500,
                    host_share_bps: 1000,
                    protocol_share_bps: 500,
                    settlement_account: AccountId::new([8; 32])?,
                    max_generated_tokens: 4096,
                    queue_depth: layerx_programs_ai_market::policy::DEFAULT_QUEUE_DEPTH,
                    liveness_timeout_ms: 30_000,
                    acknowledgement_window_ms: 5_000,
                    canary_relative_tolerance_bps: bps,
                },
            },
            peer_signature: Signature64([0; 64]),
        };
        task.peer_signature = Signature64(
            peer_key()
                .sign(task.answer_attestation(&self.binding()?)?.as_bytes())
                .to_bytes(),
        );
        Ok(task)
    }
    fn score_envelope(
        &self,
        operation: Operation,
        payload: &[u8],
        actor: PrincipalId,
    ) -> CodecResult<Vec<u8>> {
        let b = self.binding()?;
        let sequence = producers::next_sequence(&self.bytes, owner()?)?;
        let mut request = [0x77; 32];
        request[..8].copy_from_slice(&sequence.to_be_bytes());
        request[8..10].copy_from_slice(&operation.selector().to_be_bytes());
        let e = Envelope {
            operation,
            chain: b.chain,
            program: b.program,
            market: b.market,
            actor,
            epoch: b.epoch,
            config: b.config.get(),
            roster: Presence::Present(b.roster),
            sequence,
            expiry: u64::MAX,
            request: RequestId::new(request)?,
            payload,
            authentication: Authentication::Native,
        };
        let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
        let n = commit_reveal::encode_canary_envelope(&e, &mut encoded)?;
        encoded.truncate(n);
        Ok(encoded)
    }
    fn send(
        &mut self,
        encoded: &[u8],
        actor: PrincipalId,
        at: u64,
    ) -> CodecResult<producers::Reply> {
        let reply = producers::compose(&self.bytes, encoded, actor, at)?;
        if let Routed::Applied { .. } = reply.routed {
            self.bytes.clone_from(&reply.state);
        }
        Ok(reply)
    }
    fn commit(&mut self, task: &CanaryTask) -> CodecResult<Digest32> {
        let encoded = self.score_envelope(dispatch::CommitScore, &task.encode()?, owner()?)?;
        let id = task.commitment(&self.binding()?, owner()?)?;
        let reply = self.send(&encoded, owner()?, COMMIT)?;
        assert_applied(&reply)?;
        let event = codec::decode_commit_event(&reply.event)?;
        assert_eq!(event.commitment.bytes(), id.bytes());
        assert_eq!(event.accepted_height, COMMIT);
        let record = canary::commitment_from_state(&self.bytes, id)?;
        assert_eq!(record.task, *task);
        assert_eq!(record.revealed_height, None);
        assert_eq!(record.expected_logits, [0; 8]);
        assert_eq!(record.status, ChallengeStatus::Unchallenged);
        Ok(id)
    }
    fn challenge_bytes(
        &self,
        task: &CanaryTask,
        id: Digest32,
        deposit: u128,
    ) -> CodecResult<Vec<u8>> {
        producers::envelope(
            &self.bytes,
            dispatch::ChallengeLap,
            principal(0x70)?,
            &ChallengeLapPayload {
                deal: task.deal,
                lap: task.lap,
                commitment: id,
                deposit,
            }
            .encode()?,
            0,
        )
    }
    fn challenge(&mut self, task: &CanaryTask, id: Digest32) -> CodecResult<()> {
        let encoded = self.challenge_bytes(task, id, DEPOSIT)?;
        let before = self.bytes.clone();
        let reply = self.send(&encoded, principal(0x70)?, COMMIT + 1)?;
        assert_applied(&reply)?;
        assert_eq!(
            canary::value_action(dispatch::ChallengeLap, &before, &self.bytes)?,
            ValueAction::Fund { amount: DEPOSIT }
        );
        let r = canary::commitment_from_state(&self.bytes, id)?;
        assert_eq!(r.status, ChallengeStatus::Open);
        assert_eq!(r.challenger, Some(principal(0x70)?));
        assert_eq!(r.deposit, DEPOSIT);
        Ok(())
    }
    fn reveal_bytes(
        &self,
        id: Digest32,
        task: &CanaryTask,
        logits: CanaryLogits,
    ) -> CodecResult<Vec<u8>> {
        self.score_envelope(
            dispatch::RevealScore,
            &CanaryReveal {
                commitment: id,
                prompt_digest: task.prompt_digest,
                logits,
            }
            .encode()?,
            owner()?,
        )
    }
    fn reveal(&mut self, task: &CanaryTask, id: Digest32) -> CodecResult<()> {
        let encoded = self.reveal_bytes(id, task, EXPECTED)?;
        let reply = self.send(&encoded, owner()?, REVEAL)?;
        assert_applied(&reply)?;
        let event = codec::decode_reveal_event(&reply.event)?;
        assert_eq!(event.vector_count, 8);
        assert_eq!(event.admitted_height, REVEAL);
        let record = canary::commitment_from_state(&self.bytes, id)?;
        assert_eq!(record.expected_logits, EXPECTED);
        assert_eq!(record.revealed_height, Some(REVEAL));
        Ok(())
    }
    fn resolve_bytes(
        &self,
        task: &CanaryTask,
        id: Digest32,
        answer: CanaryLogits,
    ) -> CodecResult<Vec<u8>> {
        producers::envelope(
            &self.bytes,
            dispatch::ResolveChallenge,
            principal(0x71)?,
            &ResolveChallengePayload {
                deal: task.deal,
                lap: task.lap,
                commitment: id,
                answer,
            }
            .encode()?,
            0,
        )
    }
    fn restart(self) -> CodecResult<Self> {
        let decoded = decode_shared_state(&self.bytes)?;
        let mut recovered = vec![0; MAX_STATE_BYTES];
        let mut control = vec![0; Section::Control.payload_cap()];
        let n = encode_shared_state(&decoded, &mut recovered, &mut control)?;
        recovered.truncate(n);
        assert_eq!(recovered, self.bytes);
        Ok(Self {
            bytes: recovered,
            peer: self.peer,
        })
    }
}
fn assert_applied(reply: &producers::Reply) -> CodecResult<()> {
    assert!(
        matches!(reply.routed, Routed::Applied { .. }),
        "{:?}",
        codec::decode_result(&reply.result)?
    );
    assert_eq!(
        codec::decode_result(&reply.result)?.status,
        ResultStatus::Ok
    );
    Ok(())
}
fn assert_refused(reply: &producers::Reply, error: ApplicationError) -> CodecResult<()> {
    assert!(matches!(reply.routed, Routed::Refused { .. }));
    assert_eq!(codec::decode_result(&reply.result)?.error, Some(error));
    assert!(reply.state.is_empty());
    assert!(reply.event.is_empty());
    Ok(())
}

#[test]
fn commit_challenge_reveal_resolve_hit_returns_deposit_after_restart() -> CodecResult<()> {
    let mut m = Market::new()?;
    let mut answer = EXPECTED;
    answer[0] += 5242;
    answer[4] -= 655;
    let task = m.task(100, &answer, 9)?;
    let before = m.bytes.clone();
    let id = m.commit(&task)?;
    m.challenge(&task, id)?;
    m.reveal(&task, id)?;
    let mut m = m.restart()?;
    let encoded = m.resolve_bytes(&task, id, answer)?;
    let unresolved = m.bytes.clone();
    let reply = m.send(&encoded, principal(0x71)?, REVEAL + 1)?;
    assert_applied(&reply)?;
    let record = canary::commitment_from_state(&m.bytes, id)?;
    assert_eq!(record.status, ChallengeStatus::Hit);
    assert!(!record.peer_at_fault());
    assert_eq!(record.refundable_deposit(), DEPOSIT);
    assert_eq!(
        canary::value_action(dispatch::ResolveChallenge, &unresolved, &m.bytes)?,
        ValueAction::Pay {
            recipient: AccountId::new(principal(0x70)?.bytes())?,
            amount: DEPOSIT
        }
    );
    let old = decode_shared_state(&before)?;
    let new = decode_shared_state(&m.bytes)?;
    for section in [
        Section::IdentityRoster,
        Section::CurrentReports,
        Section::SettlementClaims,
        Section::ReputationAdmission,
    ] {
        assert_eq!(
            old.feature_sections[section.index()],
            new.feature_sections[section.index()]
        );
    }
    assert_eq!(new.revision, old.revision + 4);
    assert_eq!(
        new.control
            .replay
            .actor(ActorSlot::OWNER)
            .required()
            .last
            .required()
            .sequence,
        old.control
            .replay
            .actor(ActorSlot::OWNER)
            .required()
            .last
            .required()
            .sequence
            + 2
    );
    Ok(())
}

#[test]
fn resolve_miss_retains_deposit_and_persists_peer_lap_fault() -> CodecResult<()> {
    let mut m = Market::new()?;
    let mut answer = EXPECTED;
    answer[4] -= 656;
    let task = m.task(100, &answer, 10)?;
    let id = m.commit(&task)?;
    m.challenge(&task, id)?;
    m.reveal(&task, id)?;
    let encoded = m.resolve_bytes(&task, id, answer)?;
    let before = m.bytes.clone();
    let reply = m.send(&encoded, principal(0x71)?, REVEAL + 1)?;
    assert_applied(&reply)?;
    assert_eq!(
        canary::value_action(dispatch::ResolveChallenge, &before, &m.bytes)?,
        ValueAction::None
    );
    let m = m.restart()?;
    let fault = canary::commitment_from_state(&m.bytes, id)?;
    assert!(fault.peer_at_fault());
    assert_eq!(fault.task.peer, m.peer);
    assert_eq!((fault.task.deal, fault.task.lap), (task.deal, 10));
    assert_eq!(fault.status, ChallengeStatus::Miss);
    assert_eq!(fault.refundable_deposit(), 0);
    assert_eq!(fault.deposit, DEPOSIT);
    assert_eq!(fault.resolved_height, REVEAL + 1);
    Ok(())
}

#[test]
fn challenge_outside_window_refuses_at_both_boundaries() -> CodecResult<()> {
    let mut m = Market::new()?;
    let task = m.task(100, &EXPECTED, 11)?;
    let id = m.commit(&task)?;
    let encoded = m.challenge_bytes(&task, id, DEPOSIT)?;
    let record = canary::commitment_from_state(&m.bytes, id)?;
    let window = canary::challenge_window(ORIGIN, &record)?;
    assert_eq!(window.start, COMMIT);
    assert_eq!(window.end, START + 96 + 10);
    let unchanged = m.bytes.clone();
    for at in [COMMIT - 1, window.end, window.end + 1] {
        let reply = m.send(&encoded, principal(0x70)?, at)?;
        assert_refused(&reply, errors::F11_CHALLENGE_WINDOW_CLOSED)?;
        assert_eq!(m.bytes, unchanged);
    }
    let reply = m.send(&encoded, principal(0x70)?, window.end - 1)?;
    assert_applied(&reply)?;
    Ok(())
}

#[test]
fn zero_basis_points_requires_equality_through_resolution() -> CodecResult<()> {
    for delta in [0, 1] {
        let mut m = Market::new()?;
        let mut answer = EXPECTED;
        answer[2] += delta;
        let task = m.task(0, &answer, 12)?;
        let id = m.commit(&task)?;
        m.challenge(&task, id)?;
        m.reveal(&task, id)?;
        let encoded = m.resolve_bytes(&task, id, answer)?;
        let reply = m.send(&encoded, principal(0x71)?, REVEAL)?;
        assert_applied(&reply)?;
        assert_eq!(
            canary::commitment_from_state(&m.bytes, id)?.status,
            if delta == 0 {
                ChallengeStatus::Hit
            } else {
                ChallengeStatus::Miss
            }
        );
    }
    Ok(())
}

#[test]
fn fixed_point_tolerance_has_exact_bounds_zero_and_full_i32_range() -> CodecResult<()> {
    assert_eq!(canary::LOGIT_FRACTION_BITS, 16);
    assert_eq!(
        canary::decode_logits(&canary::encode_logits(&[i32::MIN; 8]))?,
        [i32::MIN; 8]
    );
    assert_eq!(
        canary::decode_logits(&canary::encode_logits(&[i32::MAX; 8]))?,
        [i32::MAX; 8]
    );
    for e in [-65_536, 65_536] {
        assert!(canary::relative_tolerance_hit(&[e; 8], &[e + 655; 8], 100)?);
        assert!(!canary::relative_tolerance_hit(
            &[e; 8],
            &[e + 656; 8],
            100
        )?);
    }
    assert!(canary::relative_tolerance_hit(
        &[10_000; 8],
        &[10_001; 8],
        1
    )?);
    assert!(!canary::relative_tolerance_hit(&[0; 8], &[1; 8], 10_000)?);
    assert!(!canary::relative_tolerance_hit(
        &[i32::MIN; 8],
        &[i32::MAX; 8],
        10_000
    )?);
    assert!(canary::relative_tolerance_hit(
        &[i32::MIN; 8],
        &[0; 8],
        10_000
    )?);
    assert!(canary::relative_tolerance_hit(
        &[i32::MAX; 8],
        &[i32::MAX; 8],
        0
    )?);
    assert!(!canary::relative_tolerance_hit(
        &[i32::MIN; 8],
        &[i32::MIN + 1; 8],
        0
    )?);
    assert_eq!(
        canary::relative_tolerance_hit(&EXPECTED, &EXPECTED, 10_001),
        Err(errors::NON_CANONICAL)
    );
    Ok(())
}

#[test]
fn reveal_checks_commitment_prompt_and_existing_f04_windows() -> CodecResult<()> {
    let mut m = Market::new()?;
    let task = m.task(100, &EXPECTED, 13)?;
    let commit = m.score_envelope(dispatch::CommitScore, &task.encode()?, owner()?)?;
    for at in [COMMIT - 1, REVEAL] {
        let reply = m.send(&commit, owner()?, at)?;
        assert_refused(&reply, errors::WRONG_PHASE)?;
    }
    let id = m.commit(&task)?;
    m.challenge(&task, id)?;
    let early_resolve = m.resolve_bytes(&task, id, EXPECTED)?;
    let reply = m.send(&early_resolve, principal(0x71)?, COMMIT + 2)?;
    assert_refused(&reply, errors::NOT_FOUND)?;
    let reveal = m.reveal_bytes(id, &task, EXPECTED)?;
    for at in [REVEAL - 1, START + 96] {
        let reply = m.send(&reveal, owner()?, at)?;
        assert_refused(&reply, errors::WRONG_PHASE)?;
    }
    let mut wrong = EXPECTED;
    wrong[0] += 1;
    let bad = m.reveal_bytes(id, &task, wrong)?;
    let reply = m.send(&bad, owner()?, REVEAL)?;
    assert_refused(&reply, errors::F04_COMMIT_MISMATCH)?;
    let mut bad_task = task;
    bad_task.prompt_digest = Digest32::new([99; 32])?;
    let bad = m.reveal_bytes(id, &bad_task, EXPECTED)?;
    let reply = m.send(&bad, owner()?, REVEAL)?;
    assert_refused(&reply, errors::F04_COMMIT_MISMATCH)?;
    m.reveal(&task, id)?;
    Ok(())
}

#[test]
fn host_authority_peer_signature_and_answer_binding_refuse_fabricated_faults() -> CodecResult<()> {
    let mut m = Market::new()?;
    let task = m.task(100, &EXPECTED, 14)?;
    let encoded = m.score_envelope(dispatch::CommitScore, &task.encode()?, principal(0x70)?)?;
    let reply = m.send(&encoded, principal(0x70)?, COMMIT)?;
    assert_refused(&reply, errors::UNAUTHORIZED)?;
    let encoded = m.score_envelope(dispatch::CommitScore, &task.encode()?, owner()?)?;
    let reply = m.send(&encoded, principal(0x70)?, COMMIT)?;
    assert_refused(&reply, errors::UNAUTHORIZED)?;
    let mut forged = task;
    forged.peer_signature.0[0] ^= 1;
    let encoded = m.score_envelope(dispatch::CommitScore, &forged.encode()?, owner()?)?;
    let reply = m.send(&encoded, owner()?, COMMIT)?;
    assert_refused(&reply, errors::BAD_SIGNATURE)?;
    let id = m.commit(&task)?;
    m.challenge(&task, id)?;
    m.reveal(&task, id)?;
    let mut fabricated = EXPECTED;
    fabricated[0] *= -1;
    let encoded = m.resolve_bytes(&task, id, fabricated)?;
    let before = m.bytes.clone();
    let reply = m.send(&encoded, principal(0x71)?, REVEAL)?;
    assert_refused(&reply, errors::F04_COMMIT_MISMATCH)?;
    assert_eq!(m.bytes, before);
    assert!(!canary::commitment_from_state(&m.bytes, id)?.peer_at_fault());
    Ok(())
}

#[test]
fn exact_retries_preserve_state_and_never_repeat_deposit_or_refund() -> CodecResult<()> {
    let mut m = Market::new()?;
    let task = m.task(100, &EXPECTED, 15)?;
    let commit = m.score_envelope(dispatch::CommitScore, &task.encode()?, owner()?)?;
    let id = m.commit(&task)?;
    let before = m.bytes.clone();
    let retry = m.send(&commit, owner()?, COMMIT + 1)?;
    assert!(matches!(retry.routed, Routed::Unchanged { .. }));
    assert_eq!(m.bytes, before);
    let challenge = m.challenge_bytes(&task, id, DEPOSIT)?;
    m.challenge(&task, id)?;
    let before = m.bytes.clone();
    let retry = m.send(&challenge, principal(0x70)?, REVEAL + 100)?;
    assert!(matches!(retry.routed, Routed::Unchanged { .. }));
    assert_eq!(m.bytes, before);
    let reveal = m.reveal_bytes(id, &task, EXPECTED)?;
    m.reveal(&task, id)?;
    let before = m.bytes.clone();
    let retry = m.send(&reveal, owner()?, REVEAL + 1)?;
    assert!(matches!(retry.routed, Routed::Unchanged { .. }));
    assert_eq!(m.bytes, before);
    let resolve = m.resolve_bytes(&task, id, EXPECTED)?;
    let reply = m.send(&resolve, principal(0x71)?, REVEAL + 1)?;
    assert_applied(&reply)?;
    let before = m.bytes.clone();
    let retry = m.send(&resolve, principal(0x71)?, REVEAL + 2)?;
    assert!(matches!(retry.routed, Routed::Unchanged { .. }));
    assert_eq!(m.bytes, before);
    Ok(())
}

#[test]
fn challenge_binds_deal_lap_commitment_and_exact_deposit() -> CodecResult<()> {
    let mut m = Market::new()?;
    let task = m.task(100, &EXPECTED, 16)?;
    let id = m.commit(&task)?;
    for deposit in [0, DEPOSIT - 1, DEPOSIT + 1] {
        let encoded = m.challenge_bytes(&task, id, deposit)?;
        let reply = m.send(&encoded, principal(0x70)?, COMMIT + 1)?;
        assert_refused(&reply, errors::F11_BOND_INSUFFICIENT)?;
    }
    for change_deal in [false, true] {
        let mut wrong = task;
        if change_deal {
            wrong.deal = Digest32::new([43; 32])?;
        } else {
            wrong.lap += 1;
        }
        let encoded = m.challenge_bytes(&wrong, id, DEPOSIT)?;
        let reply = m.send(&encoded, principal(0x70)?, COMMIT + 1)?;
        assert_refused(&reply, errors::CONFLICT)?;
    }
    let encoded = m.challenge_bytes(&task, Digest32::new([99; 32])?, DEPOSIT)?;
    let reply = m.send(&encoded, principal(0x70)?, COMMIT + 1)?;
    assert_refused(&reply, errors::NOT_FOUND)?;
    Ok(())
}

#[test]
fn canary_codecs_reject_truncation_trailing_bytes_and_noncanonical_authentication(
) -> CodecResult<()> {
    let mut m = Market::new()?;
    let task = m.task(100, &EXPECTED, 17)?;
    let payload = task.encode()?;
    assert_eq!(
        payload.len(),
        240 + layerx_programs_ai_market::policy::ROOM_PROFILE_BYTES
    );
    let mut unsupported = payload;
    unsupported[..8].copy_from_slice(b"CANARY1\0");
    assert_eq!(CanaryTask::decode(&unsupported), Err(errors::BAD_VERSION));
    let mut trailing = payload.to_vec();
    trailing.push(0);
    assert_eq!(CanaryTask::decode(&trailing), Err(errors::NON_CANONICAL));
    for n in 0..payload.len() {
        assert!(CanaryTask::decode(&payload[..n]).is_err());
    }
    let encoded = m.score_envelope(dispatch::CommitScore, &payload, owner()?)?;
    for n in 0..encoded.len() {
        assert!(commit_reveal::decode_canary_envelope(&encoded[..n]).is_err());
    }
    let mut trailing = encoded.clone();
    trailing.push(0);
    assert!(commit_reveal::decode_canary_envelope(&trailing).is_err());
    let mut delegated = encoded.clone();
    *delegated.last_mut().required() = 1;
    assert!(commit_reveal::decode_canary_envelope(&delegated).is_err());
    let id = m.commit(&task)?;
    let payload = CanaryReveal {
        commitment: id,
        prompt_digest: task.prompt_digest,
        logits: EXPECTED,
    }
    .encode()?;
    for n in 0..payload.len() {
        assert!(CanaryReveal::decode(&payload[..n]).is_err());
    }
    let state = decode_shared_state(&m.bytes)?;
    assert_eq!(
        CanaryRegion::decode(state.control.feature_bytes)?
            .records()
            .count(),
        1
    );
    let mut old_region = state.control.feature_bytes.to_vec();
    let at = old_region
        .windows(8)
        .position(|bytes| bytes == b"CANARYS2")
        .required();
    old_region[at..at + 8].copy_from_slice(b"CANARYS1");
    assert!(matches!(
        CanaryRegion::decode(&old_region),
        Err(errors::BAD_VERSION)
    ));
    assert!(m.bytes.len() <= MAX_STATE_BYTES);
    Ok(())
}

#[test]
fn room_terms_are_frozen_and_canary_state_enforces_its_capacity() -> CodecResult<()> {
    let mut m = Market::new()?;
    let task = m.task(100, &EXPECTED, 0)?;
    m.commit(&task)?;
    let changed = m.task(101, &EXPECTED, 1)?;
    let encoded = m.score_envelope(dispatch::CommitScore, &changed.encode()?, owner()?)?;
    let reply = m.send(&encoded, owner()?, COMMIT)?;
    assert_refused(&reply, errors::CONFLICT)?;
    for lap in 1..canary::CANARY_CAPACITY {
        let task = m.task(100, &EXPECTED, lap as u64)?;
        m.commit(&task)?;
    }
    let task = m.task(100, &EXPECTED, canary::CANARY_CAPACITY as u64)?;
    let encoded = m.score_envelope(dispatch::CommitScore, &task.encode()?, owner()?)?;
    let before = m.bytes.clone();
    let reply = m.send(&encoded, owner()?, COMMIT)?;
    assert_refused(&reply, errors::CAPACITY)?;
    assert_eq!(m.bytes, before);
    let state = decode_shared_state(&m.bytes)?;
    assert_eq!(
        CanaryRegion::decode(state.control.feature_bytes)?
            .records()
            .count(),
        canary::CANARY_CAPACITY
    );
    assert!(m.bytes.len() <= MAX_STATE_BYTES);
    Ok(())
}

#[test]
fn canary_uses_decoded_room_profile_tolerance_after_restart() -> CodecResult<()> {
    use layerx_programs_ai_market::{
        policy::{MarketPolicy, MARKET_POLICY_BYTES},
        registry_ops::PolicySection,
    };

    for (bps, delta, expected_status) in [
        (0, 0, ChallengeStatus::Hit),
        (0, 1, ChallengeStatus::Miss),
        (1, 1, ChallengeStatus::Hit),
    ] {
        let mut m = Market::new()?;
        let mut answer = EXPECTED;
        answer[2] += delta;
        let mut task = m.task(bps, &answer, 18)?;
        let policy = MarketPolicy {
            task_policy: PolicySection::decode(
                decode_shared_state(&m.bytes)?.section(Section::PolicyLifecycle)?,
            )?
            .current,
            room_profile: task.profile.room,
        };
        let mut encoded = [0; MARKET_POLICY_BYTES];
        policy.encode(&mut encoded)?;
        let decoded = MarketPolicy::decode(&encoded)?;
        assert_eq!(decoded.room_profile.canary_relative_tolerance_bps, bps);
        task.profile = CanaryRoomProfile {
            room: decoded.room_profile,
        };
        let id = m.commit(&task)?;
        let mut m = m.restart()?;
        m.challenge(&task, id)?;
        let mut m = m.restart()?;
        m.reveal(&task, id)?;
        let mut m = m.restart()?;
        let persisted = canary::commitment_from_state(&m.bytes, id)?;
        assert_eq!(persisted.task.profile.room, decoded.room_profile);
        let request = m.resolve_bytes(&task, id, answer)?;
        let before = m.bytes.clone();
        let reply = m.send(&request, principal(0x71)?, REVEAL + 1)?;
        assert_applied(&reply)?;
        let m = m.restart()?;
        let resolved = canary::commitment_from_state(&m.bytes, id)?;
        assert_eq!(resolved.status, expected_status, "bps={bps}, delta={delta}");
        assert_eq!(resolved.task.profile.room, decoded.room_profile);
        assert_eq!(
            resolved.peer_at_fault(),
            expected_status == ChallengeStatus::Miss
        );
        assert_eq!(
            resolved.refundable_deposit(),
            if expected_status == ChallengeStatus::Hit {
                DEPOSIT
            } else {
                0
            },
        );
        assert_eq!(
            canary::value_action(dispatch::ResolveChallenge, &before, &m.bytes)?,
            if expected_status == ChallengeStatus::Hit {
                ValueAction::Pay {
                    recipient: AccountId::new(principal(0x70)?.bytes())?,
                    amount: DEPOSIT,
                }
            } else {
                ValueAction::None
            },
        );
    }
    Ok(())
}
