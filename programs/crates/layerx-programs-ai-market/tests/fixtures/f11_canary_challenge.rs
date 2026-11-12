use super::producers;
use layerx_ai_market_test_values::RequiredValue as _;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs_ai_market::{
    canary::{
        self, CanaryLogits, CanaryReveal, CanaryRoomProfile, CanaryTask, ChallengeLapPayload,
        ChallengeStatus, ResolveChallengePayload,
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
    let chain = ChainDomain::new(layerx_programs_ai_market::MERGED_CHAIN_DOMAIN)?;
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

pub fn recorded_journey() {
    commit_challenge_reveal_resolve_hit_returns_deposit_after_restart().required();
}
