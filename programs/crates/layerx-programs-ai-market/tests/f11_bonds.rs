use layerx_ai_market_test_values::RequiredValue as _;
#[path = "repair_22_33_260.rs"]
mod producers;

use ed25519_dalek::{Signer, SigningKey};
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
    MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES, MERGED_CHAIN_DOMAIN,
};
const ORIGIN: u64 = 128;
const START: u64 = ORIGIN + 6 * 128;
const AT: u64 = START + 64;
fn owner() -> PrincipalId {
    PrincipalId::new([12; 32]).required()
}
fn actor(n: u8) -> PrincipalId {
    PrincipalId::new([n; 32]).required()
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

struct Market {
    bytes: Vec<u8>,
    roster: RosterDigest,
}
impl Market {
    fn new() -> Self {
        let mut bytes = Self::create_state();
        let roster = Self::activate_state(&mut bytes);
        let mut market = Self { bytes, roster };
        market.open_room_and_ask();
        market.post_deal();
        producers::seal_tasks(&mut market.bytes, AT).required();
        market
    }
    fn create_state() -> Vec<u8> {
        let chain = ChainDomain::new(MERGED_CHAIN_DOMAIN).required();
        let program = ProgramId::new([11; 32]).required();
        let asset = AssetId::new([13; 32]).required();
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

    fn activate_state(bytes: &mut Vec<u8>) -> RosterDigest {
        let mut sequence = 2;
        {
            let n = 20;
            producers::enroll(
                bytes,
                &mut sequence,
                actor(n),
                n,
                &SigningKey::from_bytes(&[0x90 + n; 32]),
                MetadataDigest::new([n; 32]).required(),
                ORIGIN + 2,
            )
            .required();
        }
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
        let mut fund = 1000u128.to_be_bytes().to_vec();
        fund.extend_from_slice(&[14; 32]);
        fund.extend_from_slice(&1u64.to_be_bytes());
        fund.push(1);
        producers::call(bytes, dispatch::FUND, owner(), &fund, ORIGIN + 7).required();
        producers::open(bytes, ORIGIN + 128).required();
        producers::heartbeat(bytes, ORIGIN + 129).required();
        producers::enroll(
            bytes,
            &mut sequence,
            actor(21),
            21,
            &SigningKey::from_bytes(&[0x90 + 21; 32]),
            MetadataDigest::new([21; 32]).required(),
            ORIGIN + 130,
        )
        .required();
        producers::settle(bytes, ORIGIN + 224).required();
        let mut schedule = decode_shared_state(bytes)
            .required()
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
        .required();
        producers::open(bytes, START).required();
        let mut advance = decode_shared_state(bytes)
            .required()
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
        .required();

        RosterDigest::new(
            roster::roster_digest(&producers::table(bytes).required(), 6)
                .required()
                .bytes(),
        )
        .required()
    }

    fn open_room_and_ask(&mut self) {
        let market = self;
        let profile = RoomProfile {
            minimum_bond: 100,
            canary_ratio_bps: 250,
            lap_size_tokens: 32,
            challenge_window_blocks: 16,
            slash_fraction_bps: 1000,
            host_share_bps: 1000,
            protocol_share_bps: 1000,
            settlement_account: AccountId::new([80; 32]).required(),
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
    }

    fn post_deal(&mut self) {
        let market = self;
        let roster = market.roster;
        let state = decode_shared_state(&market.bytes).required();
        let workers = layerx_programs_ai_market::workers::WorkerTable::decode(
            layerx_programs_ai_market::evaluators::authority::split_identity_section(
                state.section(Section::IdentityRoster).required(),
            )
            .required()
            .0,
        )
        .required();
        for (n, bid) in [(20, 70), (21, 71)] {
            let worker = workers.iter().find(|w| w.owner == actor(n)).required();
            market.call(
                dispatch::LockBond,
                actor(n),
                &LockBondRequest {
                    room: digest(60),
                    bid: digest(bid),
                    amount: 100,
                }
                .encode()
                .required(),
                START + 2,
            );
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
            end_layer: 4,
        });
        layers[1] = Some(LayerSplit {
            bid_id: digest(71),
            first_layer: 4,
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
                    agreed_price_per_thousand_tokens: 1000,
                    roster_digest: roster,
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
    fn call(
        &mut self,
        op: dispatch::Operation,
        who: PrincipalId,
        payload: &[u8],
        at: u64,
    ) -> Vec<u8> {
        let e = self.envelope(op, who, payload);
        producers::apply(&mut self.bytes, &e, who, at).required();
        e
    }
    fn manifest(&self, batch: &ReceiptBatch) -> Vec<u8> {
        let state = decode_shared_state(&self.bytes).required();
        let section =
            PolicySection::decode(state.section(Section::PolicyLifecycle).required()).required();
        let binding = epoch::captured_roster(&state).required().required().binding;
        let parents = [ParentRef {
            purpose: ParentPurpose::Execution,
            root: batch.summary_digest().required().bytes(),
        }];
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
                    subject: producers::sealed_task_set(&self.bytes, 6)
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
            producers::sealed_task_set(&self.bytes, 6)
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
fn batch(first: u64, count: usize, bitmap: u32) -> ReceiptBatch {
    producers::lap_batch(digest(62), first, count, bitmap).required()
}

use layerx_programs_ai_market::{
    bonds::{self, BondRecord, LockBondRequest, RefundBondRequest, SlashBondRequest},
    canary::{
        self, CanaryReveal, CanaryRoomProfile, CanaryTask, ChallengeLapPayload,
        ResolveChallengePayload,
    },
    commit_reveal,
    rewards::{self, SplitPayoutRequest},
    value_adapter::{self, RewardsAccount, ValueAction},
};

impl Market {
    fn settle_batch(&mut self, batch: ReceiptBatch) {
        producers::seal_tasks(&mut self.bytes, AT).required();
        let manifest = self.manifest(&batch);
        self.seal(&manifest, if batch.first_lap_index == 0 { 2 } else { 3 });
        self.call(
            dispatch::SettleLaps,
            owner(),
            &Market::settle_payload(batch, &manifest),
            AT,
        );
    }
    fn split_request(&self, amount: u128) -> SplitPayoutRequest {
        let settled = settlement_from_state(&self.bytes, digest(62))
            .required()
            .required();
        SplitPayoutRequest {
            deal: digest(62),
            batch: settled.batch.batch_digest,
            amount,
        }
    }
    fn reward_state(&self) -> rewards::RewardState<'_> {
        layerx_programs_ai_market::state::settlement_rewards(
            &decode_shared_state(&self.bytes).required(),
            None,
        )
        .required()
    }
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
    fn balance(&self, worker: WorkerId) -> u128 {
        self.reward_state().payout_balance(6, worker).required()
    }
    fn split(&mut self, amount: u128, who: PrincipalId, height: u64) -> Vec<u8> {
        self.call(
            dispatch::SplitPayout,
            who,
            &self.split_request(amount).encode().required(),
            height,
        )
    }
    fn refused_call(
        &self,
        op: dispatch::Operation,
        who: PrincipalId,
        payload: &[u8],
        height: u64,
        error: ApplicationError,
    ) {
        let bytes = self.bytes.clone();
        let e = self.envelope(op, who, payload);
        let reply = producers::compose(&bytes, &e, who, height).required();
        assert!(
            matches!(reply.routed, Routed::Refused { .. }),
            "{:?}",
            codec::decode_result(&reply.result).required()
        );
        assert_eq!(
            codec::decode_result(&reply.result).required().error,
            Some(error)
        );
        assert!(reply.state.is_empty());
        assert!(reply.event.is_empty());
        assert_eq!(self.bytes, bytes);
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
            peer: self.peer(20).worker,
            prompt_digest: digest(90),
            expected_logits_digest: canary::logits_digest(&[100_000; 8]).required(),
            answer_digest: canary::logits_digest(&answer).required(),
            profile: CanaryRoomProfile { room: room.profile },
            peer_signature: Signature64([0; 64]),
        };
        task.peer_signature = Signature64(
            SigningKey::from_bytes(&[0x90 + 20; 32])
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

impl Market {
    fn bond(&self, bid: u8) -> BondRecord {
        bonds::bond_from_state(&self.bytes, digest(bid))
            .required()
            .required()
    }
    fn slash_request(evidence: Digest32, lap: u64) -> SlashBondRequest {
        SlashBondRequest {
            deal: digest(62),
            bid: digest(70),
            lap,
            evidence,
        }
    }
    fn close(&mut self, height: u64) {
        self.call(dispatch::CloseRoom, owner(), digest(60).as_bytes(), height);
    }
    fn refund_request() -> RefundBondRequest {
        RefundBondRequest {
            room: digest(60),
            bid: digest(70),
        }
    }
    fn post_bid(&self, id: u8) -> Vec<u8> {
        let state = decode_shared_state(&self.bytes).required();
        let workers = layerx_programs_ai_market::workers::WorkerTable::decode(
            layerx_programs_ai_market::evaluators::authority::split_identity_section(
                state.section(Section::IdentityRoster).required(),
            )
            .required()
            .0,
        )
        .required();
        let worker = workers.iter().find(|w| w.owner == actor(20)).required();
        encoded(130, |out| {
            PostBidRequest {
                room_id: digest(60),
                bid_id: digest(id),
                capability_digest: Digest32::new(worker.metadata.bytes())?,
                minimum_price_per_thousand_tokens: 5,
                bond_amount: 100,
                layer_capacity: 16,
            }
            .encode(out)
        })
    }
}

#[test]
fn room_record_capacity_includes_refund_close_height() {
    let m = Market::new();
    let state = decode_shared_state(&m.bytes).required();
    let mut room = RoomBook::decode(state.room_market().required().1)
        .required()
        .room(digest(60))
        .required()
        .required();
    assert_eq!(room.closed_height, 0);
    room.state = RoomState::Closed;
    room.closed_height = START + 83;
    let mut bytes = [0; ROOM_RECORD_BYTES];
    assert_eq!(room.encode(&mut bytes).required(), bytes.len());
    assert_eq!(RoomRecord::decode(&bytes).required(), room);
    assert_eq!(
        room.encode(&mut bytes[..ROOM_RECORD_BYTES - 1]),
        Err(CAPACITY)
    );
}

#[test]
fn lock_then_bid_passes_and_lock_replay_does_not_deposit_twice() {
    let mut m = Market::new();
    let request = LockBondRequest {
        room: digest(60),
        bid: digest(72),
        amount: 100,
    }
    .encode()
    .required();
    let before = m.reward_state().ledger().required();
    let envelope = m.call(dispatch::LockBond, actor(20), &request, START + 3);
    let locked = m.bytes.clone();
    let replay = producers::compose(&locked, &envelope, actor(20), START + 3).required();
    assert!(matches!(replay.routed, Routed::Unchanged { .. }));
    assert!(replay.state.is_empty());
    assert!(replay.event.is_empty());
    assert_eq!(m.reward_state().ledger().required(), before);
    assert_eq!(m.bond(72).remaining().required(), 100);
    m.call(dispatch::PostBid, actor(20), &m.post_bid(72), START + 3);
    let state = decode_shared_state(&m.bytes).required();
    let bid = RoomBook::decode(state.room_market().required().1)
        .required()
        .bid(digest(72))
        .required()
        .required();
    assert_eq!(bid.bond_amount, m.bond(72).amount);
    assert_eq!(bid.peer_key, actor(20));
    assert_eq!(m.bond(72).locked_height, START + 3);
    assert_eq!(m.bond(70).locked_height, START + 2);
    assert_eq!(m.bond(72).worker, m.bond(70).worker);
}

#[test]
fn bid_without_lock_refuses_and_foreign_or_insufficient_locks_refuse() {
    let mut m = Market::new();
    m.refused_call(
        dispatch::PostBid,
        actor(20),
        &m.post_bid(72),
        START + 3,
        F11_BOND_INSUFFICIENT,
    );
    m.refused_call(
        dispatch::LockBond,
        actor(20),
        &LockBondRequest {
            room: digest(60),
            bid: digest(72),
            amount: 99,
        }
        .encode()
        .required(),
        START + 3,
        F11_BOND_INSUFFICIENT,
    );
    m.call(
        dispatch::LockBond,
        actor(21),
        &LockBondRequest {
            room: digest(60),
            bid: digest(72),
            amount: 100,
        }
        .encode()
        .required(),
        START + 3,
    );
    m.refused_call(
        dispatch::PostBid,
        actor(20),
        &m.post_bid(72),
        START + 3,
        UNAUTHORIZED,
    );
    m.refused_call(
        dispatch::LockBond,
        actor(20),
        &LockBondRequest {
            room: digest(60),
            bid: digest(72),
            amount: 100,
        }
        .encode()
        .required(),
        START + 3,
        F11_BOND_LOCKED,
    );
}

#[test]
fn slash_after_resolved_miss_moves_profile_fraction_into_other_peers_rewards() {
    let mut m = Market::new();
    m.settle_batch(batch(0, 1, 3));
    let (task, id) = m.canary(0);
    let payload = Market::slash_request(id, 0).encode().required();
    m.refused_call(
        dispatch::SlashBond,
        actor(120),
        &payload,
        AT + 2,
        UNAUTHORIZED,
    );
    m.resolve_fault(&task, id);
    let before = m.reward_state().ledger().required();
    let honest = m.peer(21).worker;
    let balance = m.balance(honest);
    m.call(dispatch::SlashBond, actor(120), &payload, START + 82);
    let after = m.reward_state().ledger().required();
    assert_eq!(m.bond(70).slashed, 10);
    assert_eq!(m.bond(70).remaining().required(), 90);
    assert_eq!(m.balance(honest) - balance, 10);
    assert_eq!(m.balance(m.peer(20).worker), 0);
    assert_eq!(after.tracked_deposits - before.tracked_deposits, 10);
    assert_eq!(after.liability - before.liability, 10);
    assert_eq!(after.free, before.free);
    after.validate().required();
    let bytes = m.bytes.clone();
    m.bytes = decode_shared_state(&bytes)
        .map(|_| bytes.clone())
        .required();
    assert_eq!(m.bond(70).remaining().required(), 90);
    assert_eq!(m.balance(honest) - balance, 10);
    m.refused_call(
        dispatch::SlashBond,
        actor(120),
        &Market::slash_request(id, 1).encode().required(),
        START + 82,
        UNAUTHORIZED,
    );
}

#[test]
fn double_slash_for_one_lap_refuses_even_with_another_request_or_fault_kind() {
    let mut m = Market::new();
    m.settle_batch(batch(0, 1, 3));
    let (task, id) = m.canary(0);
    m.resolve_fault(&task, id);
    let payload = Market::slash_request(id, 0).encode().required();
    m.call(dispatch::SlashBond, actor(120), &payload, START + 82);
    assert_eq!(
        settlement_from_state(&m.bytes, digest(62))
            .required()
            .required()
            .slashed_laps[0],
        1
    );
    m.refused_call(
        dispatch::SlashBond,
        actor(121),
        &payload,
        START + 83,
        CONFLICT,
    );
    let another = Market::slash_request(digest(99), 0).encode().required();
    m.refused_call(dispatch::SlashBond, owner(), &another, START + 83, CONFLICT);
    assert_eq!(m.bond(70).slashed, 10);
    assert_eq!(m.balance(m.peer(21).worker), 10);
}

#[test]
fn missed_countersign_deadline_requires_native_host_and_recorded_incomplete_quorum() {
    let mut m = Market::new();
    let batch = batch(0, 1, 2);
    m.settle_batch(batch);
    let committed = settlement_from_state(&m.bytes, digest(62))
        .required()
        .required();
    assert_eq!(committed.batch.missing_countersigners, 1);
    assert_eq!(committed.batch.countersign_quorum, 1);
    let honest_bid = SlashBondRequest {
        bid: digest(71),
        ..Market::slash_request(batch.batch_digest, 0)
    }
    .encode()
    .required();
    m.refused_call(
        dispatch::SlashBond,
        owner(),
        &honest_bid,
        START + 96,
        UNAUTHORIZED,
    );
    let payload = Market::slash_request(batch.batch_digest, 0)
        .encode()
        .required();
    m.refused_call(
        dispatch::SlashBond,
        owner(),
        &payload,
        START + 95,
        UNAUTHORIZED,
    );
    m.refused_call(
        dispatch::SlashBond,
        actor(120),
        &payload,
        START + 96,
        UNAUTHORIZED,
    );
    m.call(dispatch::SlashBond, owner(), &payload, START + 96);
    assert_eq!(
        settlement_from_state(&m.bytes, digest(62))
            .required()
            .required()
            .slashed_laps[0],
        1
    );
    assert_eq!(m.bond(70).slashed, 10);
    assert_eq!(m.balance(m.peer(21).worker), 10);
}

#[test]
fn complete_quorum_cannot_be_used_as_a_missed_deadline() {
    let mut m = Market::new();
    let batch = batch(0, 1, 3);
    m.settle_batch(batch);
    let payload = Market::slash_request(batch.batch_digest, 0)
        .encode()
        .required();
    m.refused_call(
        dispatch::SlashBond,
        owner(),
        &payload,
        START + 96,
        UNAUTHORIZED,
    );
    assert_eq!(m.bond(70).slashed, 0);
}

#[test]
fn refund_before_last_deal_closes_or_before_challenge_window_refuses() {
    let mut m = Market::new();
    let refund = Market::refund_request().encode().required();
    m.refused_call(
        dispatch::RefundBond,
        actor(20),
        &refund,
        AT,
        F11_BOND_LOCKED,
    );
    m.refused_call(
        dispatch::CloseRoom,
        owner(),
        digest(60).as_bytes(),
        AT,
        F11_BOND_LOCKED,
    );
    m.settle_batch(batch(0, 1, 3));
    m.refused_call(
        dispatch::CloseRoom,
        owner(),
        digest(60).as_bytes(),
        START + 81,
        F11_BOND_LOCKED,
    );
    m.split(100, owner(), START + 81);
    m.close(START + 82);
    m.refused_call(
        dispatch::RefundBond,
        actor(20),
        &refund,
        START + 111,
        F11_BOND_LOCKED,
    );
    assert_eq!(m.bond(70).remaining().required(), 100);
    let state = decode_shared_state(&m.bytes).required();
    assert_eq!(
        RoomBook::decode(state.room_market().required().1)
            .required()
            .deal(digest(62))
            .required()
            .required()
            .state,
        DealState::Settled
    );
}

#[test]
fn refund_after_window_returns_remainder_once_and_preserves_reward_claims() {
    let mut m = Market::new();
    m.settle_batch(batch(0, 1, 3));
    let (task, id) = m.canary(0);
    m.resolve_fault(&task, id);
    m.call(
        dispatch::SlashBond,
        actor(120),
        &Market::slash_request(id, 0).encode().required(),
        START + 82,
    );
    m.split(100, owner(), START + 82);
    m.close(START + 83);
    let before = m.reward_state().ledger().required();
    let honest = m.balance(m.peer(21).worker);
    let refund = Market::refund_request().encode().required();
    m.refused_call(
        dispatch::RefundBond,
        actor(21),
        &refund,
        START + 112,
        UNAUTHORIZED,
    );
    let envelope = m.call(dispatch::RefundBond, actor(20), &refund, START + 112);
    assert!(m.bond(70).refunded);
    assert_eq!(m.bond(70).remaining().required(), 0);
    let after = m.reward_state().ledger().required();
    assert_eq!(after.total_claimed - before.total_claimed, 90);
    assert_eq!(after.tracked_deposits - before.tracked_deposits, 90);
    assert_eq!(after.liability, before.liability);
    assert_eq!(m.balance(m.peer(21).worker), honest);
    let bound = RewardsAccount::for_ledger(ProgramId::new([11; 32]).required(), &before).required();
    let mut covered_before = before;
    covered_before.tracked_deposits += 190;
    covered_before.free += 190;
    let mut covered_after = after;
    covered_after.tracked_deposits += 100;
    covered_after.free += 100;
    assert_eq!(
        value_adapter::plan(
            &bound,
            &covered_before,
            &covered_after,
            rewards::RewardEffect::Payout {
                recipient: m.bond(70).recipient,
                amount: 90
            }
        )
        .required(),
        ValueAction::Pay {
            recipient: m.bond(70).recipient,
            amount: 90
        }
    );
    let replay = producers::compose(&m.bytes, &envelope, actor(20), START + 112).required();
    assert!(matches!(replay.routed, Routed::Unchanged { .. }));
    assert!(replay.state.is_empty());
    m.refused_call(
        dispatch::RefundBond,
        actor(20),
        &refund,
        START + 113,
        F11_BOND_LOCKED,
    );
}

#[test]
fn the_bond_of_a_bid_named_by_two_deals_is_refunded_once() {
    for first_settled in [62, 63] {
        let (mut m, refund) = shared_bond_market();
        let other = if first_settled == 62 { 63 } else { 62 };
        settle_shared_bond_deals(&mut m, first_settled, other, &refund);
        // Each deal's missed countersignature slashes the same bond once.
        for deal in [62, 63] {
            let batch = settlement_from_state(&m.bytes, digest(deal))
                .required()
                .required()
                .batch;
            let slash = SlashBondRequest {
                deal: digest(deal),
                bid: digest(70),
                lap: 0,
                evidence: batch.batch_digest,
            }
            .encode()
            .required();
            m.call(dispatch::SlashBond, owner(), &slash, START + 96);
            m.refused_call(dispatch::SlashBond, owner(), &slash, START + 96, CONFLICT);
        }
        assert_eq!(m.bond(70).slashed, 20);
        assert_eq!(m.bond(70).remaining().required(), 80);
        let batch = settlement_from_state(&m.bytes, digest(other))
            .required()
            .required()
            .batch;
        m.refused_call(
            dispatch::CloseRoom,
            owner(),
            digest(60).as_bytes(),
            START + 96,
            F11_BOND_LOCKED,
        );
        m.call(
            dispatch::SplitPayout,
            owner(),
            &SplitPayoutRequest {
                deal: digest(other),
                batch: batch.batch_digest,
                amount: 100,
            }
            .encode()
            .required(),
            START + 96,
        );
        m.refused_call(
            dispatch::RefundBond,
            actor(20),
            &refund,
            START + 99,
            F11_BOND_LOCKED,
        );
        m.close(START + 100);
        let shared = decode_shared_state(&m.bytes).required();
        let book = RoomBook::decode(shared.room_market().required().1).required();
        assert_eq!(
            book.room(digest(60)).required().required().state,
            RoomState::Closed
        );
        for deal in [62, 63] {
            assert_eq!(
                book.deal(digest(deal)).required().required().state,
                DealState::Settled
            );
        }
        m.refused_call(
            dispatch::RefundBond,
            actor(20),
            &refund,
            START + 115,
            F11_BOND_LOCKED,
        );
        verify_shared_bond_refund(&mut m, &refund);
    }
}
fn shared_bond_market() -> (Market, [u8; 64]) {
    let mut m = Market::new();
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
    m.call(
        dispatch::RecordDeal,
        owner(),
        &encoded(1400, |out| {
            RecordDealRequest {
                room_id: digest(60),
                deal_id: digest(63),
                ask_id: digest(61),
                agreed_price_per_thousand_tokens: 1000,
                roster_digest: m.roster,
                layers,
            }
            .encode(out)
        }),
        AT,
    );
    assert_eq!(m.bond(70).amount, 100);
    assert_eq!(bonds::held_from_state(&m.bytes).required(), 200);
    let refund = Market::refund_request().encode().required();
    m.refused_call(
        dispatch::RefundBond,
        actor(20),
        &refund,
        AT,
        F11_BOND_LOCKED,
    );
    m.refused_call(
        dispatch::CloseRoom,
        owner(),
        digest(60).as_bytes(),
        AT,
        F11_BOND_LOCKED,
    );
    (m, refund)
}
fn settle_shared_bond_deals(m: &mut Market, first_settled: u8, other: u8, refund: &[u8]) {
    for (index, deal) in [first_settled, other].into_iter().enumerate() {
        let batch = producers::lap_batch(digest(deal), 0, 1, 2).required();
        let manifest = m.manifest(&batch);
        m.seal(&manifest, 2 + u8::try_from(index).required());
        m.call(
            dispatch::SettleLaps,
            owner(),
            &Market::settle_payload(batch, &manifest),
            AT,
        );
        let settled = settlement_from_state(&m.bytes, digest(deal))
            .required()
            .required();
        assert_eq!(settled.batch, batch);
        assert_eq!(settled.settled_amount_owed, 100);
        if index == 0 {
            m.call(
                dispatch::SplitPayout,
                owner(),
                &SplitPayoutRequest {
                    deal: digest(deal),
                    batch: batch.batch_digest,
                    amount: 100,
                }
                .encode()
                .required(),
                AT,
            );
            assert!(settlement_from_state(&m.bytes, digest(other))
                .required()
                .is_none());
            m.refused_call(dispatch::RefundBond, actor(20), refund, AT, F11_BOND_LOCKED);
            m.refused_call(
                dispatch::CloseRoom,
                owner(),
                digest(60).as_bytes(),
                AT,
                F11_BOND_LOCKED,
            );
        }
    }
}
fn verify_shared_bond_refund(m: &mut Market, refund: &[u8]) {
    let before = m.reward_state().ledger().required();
    let held_before = bonds::held_from_state(&m.bytes).required();
    let honest = m.balance(m.peer(21).worker);
    let envelope = m.call(dispatch::RefundBond, actor(20), refund, START + 116);
    let after = m.reward_state().ledger().required();
    let held_after = bonds::held_from_state(&m.bytes).required();
    assert!(m.bond(70).refunded);
    assert_eq!(m.bond(70).remaining().required(), 0);
    assert_eq!(m.bond(70).slashed, 20);
    assert_eq!(held_before - held_after, 80);
    assert_eq!(held_after, 100);
    assert_eq!(after.total_claimed - before.total_claimed, 80);
    assert_eq!(after.tracked_deposits - before.tracked_deposits, 80);
    assert_eq!(after.liability, before.liability);
    assert_eq!(m.balance(m.peer(21).worker), honest);
    let bound = RewardsAccount::for_ledger(ProgramId::new([11; 32]).required(), &before).required();
    let mut covered_before = before;
    covered_before.tracked_deposits += held_before;
    covered_before.free += held_before;
    let mut covered_after = after;
    covered_after.tracked_deposits += held_after;
    covered_after.free += held_after;
    assert_eq!(
        value_adapter::plan(
            &bound,
            &covered_before,
            &covered_after,
            rewards::RewardEffect::Payout {
                recipient: m.bond(70).recipient,
                amount: 80
            }
        )
        .required(),
        ValueAction::Pay {
            recipient: m.bond(70).recipient,
            amount: 80
        }
    );
    let persisted = m.bytes.clone();
    let replay = producers::compose(&persisted, &envelope, actor(20), START + 116).required();
    assert!(matches!(replay.routed, Routed::Unchanged { .. }));
    assert!(replay.state.is_empty());
    assert!(replay.event.is_empty());
    m.refused_call(
        dispatch::RefundBond,
        actor(20),
        refund,
        START + 117,
        F11_BOND_LOCKED,
    );
    assert_eq!(m.bytes, persisted);
}
