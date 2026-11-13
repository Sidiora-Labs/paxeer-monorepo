use super::producers;
use layerx_ai_market_test_values::RequiredValue as _;

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
        if op == dispatch::PostBid {
            let mut r = codec::Reader::new(payload);
            let room = r.digest32().required();
            let bid = r.digest32().required();
            r.take(48).required();
            let amount = r.u128().required();
            self.call(
                dispatch::LockBond,
                who,
                &layerx_programs_ai_market::bonds::LockBondRequest { room, bid, amount }
                    .encode()
                    .required(),
                at,
            );
        }
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
    let ranges = [digest(80), digest(81)];
    let key = SigningKey::from_bytes(&[12; 32]);
    let receipts: Vec<_> = (0..count)
        .map(|i| {
            let mut receipt = LapReceipt {
                deal_id: digest(62),
                lap_index: first + i as u64,
                prompt_tokens: 10,
                generated_tokens: 90,
                cumulative_tokens: (first + i as u64 + 1) * 100,
                layer_range_digests: &ranges,
                host_signature: Signature64([1; 64]),
                peer_countersignatures: bitmap,
            };
            receipt.host_signature = Signature64(
                key.sign(receipt.signing_digest().required().as_bytes())
                    .to_bytes(),
            );
            receipt
        })
        .collect();
    ReceiptBatch::from_receipts(&receipts).required()
}

use layerx_programs_ai_market::{
    canary::{
        self, CanaryReveal, CanaryRoomProfile, CanaryTask, ChallengeLapPayload,
        ResolveChallengePayload,
    },
    commit_reveal,
    rewards::{self, SplitPayoutRequest},
};

impl Market {
    fn settle_batch(&mut self, batch: ReceiptBatch) {
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

fn split_in_deal_proportions_conserves_settled_amount_and_replays_after_restart() {
    let mut m = Market::new();
    m.settle_batch(batch(0, 4, 3));
    let before = m.reward_state().ledger().required();
    let rows_before = m
        .reward_state()
        .rows()
        .records()
        .collect::<CodecResult<Vec<_>>>()
        .required();
    let peer_a = m.peer(20);
    let peer_b = m.peer(21);
    let e = m.split(400, owner(), AT + 1);
    let saved = m.bytes.clone();
    let roster = m.roster;
    drop(m);
    let m = Market {
        bytes: saved,
        roster,
    };
    assert_eq!(m.balance(peer_a.worker), 80);
    assert_eq!(m.balance(peer_b.worker), 240);
    let host = WorkerId::new(
        codec::domain_hash("PAXAI/payout-host/v1", owner().as_bytes())
            .required()
            .bytes(),
    )
    .required();
    let protocol = WorkerId::new(
        codec::domain_hash("PAXAI/payout-protocol/v1", actor(15).as_bytes())
            .required()
            .bytes(),
    )
    .required();
    assert_eq!(m.balance(host), 40);
    assert_eq!(m.balance(protocol), 40);
    assert_eq!(
        m.balance(peer_a.worker) + m.balance(peer_b.worker) + m.balance(host) + m.balance(protocol),
        400
    );
    let after = m.reward_state().ledger().required();
    assert_eq!(
        m.reward_state()
            .rows()
            .records()
            .collect::<CodecResult<Vec<_>>>()
            .required(),
        rows_before
    );
    assert_eq!(after.tracked_deposits - before.tracked_deposits, 400);
    assert_eq!(after.liability - before.liability, 400);
    assert_eq!(after.free, before.free);
    assert_eq!(after.reserved, before.reserved);
    let before_replay = m.bytes.clone();
    let reply = producers::compose(&m.bytes, &e, owner(), AT + 2).required();
    assert!(matches!(reply.routed, Routed::Unchanged { .. }));
    assert_eq!(m.bytes, before_replay);
}

pub fn recorded_journey() {
    split_in_deal_proportions_conserves_settled_amount_and_replays_after_restart();
}
