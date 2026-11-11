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
    PrincipalId::new([12; 32]).unwrap()
}
fn actor(n: u8) -> PrincipalId {
    PrincipalId::new([n; 32]).unwrap()
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
            protocol_share_bps: 1000,
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
        if op.metadata().feature != 11
            || matches!(op, dispatch::ChallengeLap | dispatch::ResolveChallenge)
        {
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
    fn manifest(&self, batch: &ReceiptBatch) -> Vec<u8> {
        let state = decode_shared_state(&self.bytes).unwrap();
        let section =
            PolicySection::decode(state.section(Section::PolicyLifecycle).unwrap()).unwrap();
        let binding = epoch::captured_roster(&state).unwrap().unwrap().binding;
        let parents = [ParentRef {
            purpose: ParentPurpose::Execution,
            root: batch.summary_digest().unwrap().bytes(),
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
                        policy: section.current.digest().unwrap(),
                    },
                    epoch: binding.epoch,
                    publisher: owner(),
                    subject: producers::sealed_task_set(&self.bytes, 6).unwrap().bytes(),
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
        let state = decode_shared_state(&self.bytes).unwrap();
        let policy = PolicySection::decode(state.section(Section::PolicyLifecycle).unwrap())
            .unwrap()
            .current;
        let mut payload = 1u16.to_be_bytes().to_vec();
        payload.extend_from_slice(&manifest_root(manifest).unwrap().bytes());
        payload.extend_from_slice(policy.digest().unwrap().as_bytes());
        payload.extend_from_slice(
            producers::sealed_task_set(&self.bytes, 6)
                .unwrap()
                .as_bytes(),
        );
        payload.extend_from_slice(policy.commitments.rubric.as_bytes());
        payload.push(policy.assessment_mode);
        self.call(dispatch::SealEvidence, actor(voter), &payload, AT);
    }
    fn settle_payload(&self, batch: ReceiptBatch, manifest: &[u8]) -> Vec<u8> {
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
                key.sign(receipt.signing_digest().unwrap().as_bytes())
                    .to_bytes(),
            );
            receipt
        })
        .collect();
    ReceiptBatch::from_receipts(&receipts).unwrap()
}

use layerx_programs_ai_market::{
    canary::{
        self, CanaryReveal, CanaryRoomProfile, CanaryTask, ChallengeLapPayload,
        ResolveChallengePayload,
    },
    commit_reveal,
    reward_math::split_settled_amount,
    rewards::{self, ClaimRequest, SplitPayoutRequest},
    value_adapter::{self, RewardsAccount, ValueAction},
};

impl Market {
    fn settle_batch(&mut self, batch: ReceiptBatch) {
        let manifest = self.manifest(&batch);
        self.seal(&manifest, if batch.first_lap_index == 0 { 2 } else { 3 });
        self.call(
            dispatch::SettleLaps,
            owner(),
            &self.settle_payload(batch, &manifest),
            AT,
        );
    }
    fn split_request(&self, amount: u128) -> SplitPayoutRequest {
        let settled = settlement_from_state(&self.bytes, digest(62))
            .unwrap()
            .unwrap();
        SplitPayoutRequest {
            deal: digest(62),
            batch: settled.batch.batch_digest,
            amount,
        }
    }
    fn reward_state(&self) -> rewards::RewardState<'_> {
        layerx_programs_ai_market::state::settlement_rewards(
            &decode_shared_state(&self.bytes).unwrap(),
            None,
        )
        .unwrap()
    }
    fn peer(&self, n: u8) -> WorkerRosterEntry {
        let state = decode_shared_state(&self.bytes).unwrap();
        *epoch::captured_roster(&state)
            .unwrap()
            .unwrap()
            .workers()
            .iter()
            .find(|w| w.owner == actor(n))
            .unwrap()
    }
    fn balance(&self, worker: WorkerId) -> u128 {
        self.reward_state().payout_balance(6, worker).unwrap()
    }
    fn split(&mut self, amount: u128, who: PrincipalId, height: u64) -> Vec<u8> {
        self.call(
            dispatch::SplitPayout,
            who,
            &self.split_request(amount).encode().unwrap(),
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
        let reply = producers::compose(&bytes, &e, who, height).unwrap();
        assert!(
            matches!(reply.routed, Routed::Refused { .. }),
            "{:?}",
            codec::decode_result(&reply.result).unwrap()
        );
        assert_eq!(
            codec::decode_result(&reply.result).unwrap().error,
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
        let sequence = producers::next_sequence(&self.bytes, owner()).unwrap();
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
            request: RequestId::new(request).unwrap(),
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
        let reply = producers::compose(&self.bytes, &envelope, owner(), height).unwrap();
        assert!(
            matches!(reply.routed, Routed::Applied { .. }),
            "{:?}",
            codec::decode_result(&reply.result).unwrap()
        );
        self.bytes = reply.state;
    }
    fn canary(&mut self, lap: u64) -> (CanaryTask, Digest32) {
        let binding = epoch::captured_roster(&decode_shared_state(&self.bytes).unwrap())
            .unwrap()
            .unwrap()
            .binding;
        let room = RoomBook::decode(
            decode_shared_state(&self.bytes)
                .unwrap()
                .room_market()
                .unwrap()
                .1,
        )
        .unwrap()
        .room(digest(60))
        .unwrap()
        .unwrap();
        let answer = [200_000; 8];
        let mut task = CanaryTask {
            deal: digest(62),
            lap,
            peer: self.peer(20).worker,
            prompt_digest: digest(90),
            expected_logits_digest: canary::logits_digest(&[100_000; 8]).unwrap(),
            answer_digest: canary::logits_digest(&answer).unwrap(),
            profile: CanaryRoomProfile { room: room.profile },
            peer_signature: Signature64([0; 64]),
        };
        task.peer_signature = Signature64(
            SigningKey::from_bytes(&[0x90 + 20; 32])
                .sign(task.answer_attestation(&binding).unwrap().as_bytes())
                .to_bytes(),
        );
        let id = task.commitment(&binding, owner()).unwrap();
        let payload = task.encode().unwrap();
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
            .unwrap(),
            AT + 1,
        );
        (task, id)
    }
    fn resolve_fault(&mut self, task: CanaryTask, id: Digest32) {
        let binding = epoch::captured_roster(&decode_shared_state(&self.bytes).unwrap())
            .unwrap()
            .unwrap()
            .binding;
        let payload = CanaryReveal {
            commitment: id,
            prompt_digest: task.prompt_digest,
            logits: [100_000; 8],
        }
        .encode()
        .unwrap();
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
            .unwrap(),
            START + 81,
        );
        assert!(canary::commitment_from_state(&self.bytes, id)
            .unwrap()
            .peer_at_fault());
    }
}

#[test]
fn split_in_deal_proportions_conserves_settled_amount_and_replays_after_restart() {
    let mut m = Market::new();
    m.settle_batch(batch(0, 4, 3));
    let before = m.reward_state().ledger().unwrap();
    let rows_before = m
        .reward_state()
        .rows()
        .records()
        .collect::<CodecResult<Vec<_>>>()
        .unwrap();
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
            .unwrap()
            .bytes(),
    )
    .unwrap();
    let protocol = WorkerId::new(
        codec::domain_hash("PAXAI/payout-protocol/v1", actor(15).as_bytes())
            .unwrap()
            .bytes(),
    )
    .unwrap();
    assert_eq!(m.balance(host), 40);
    assert_eq!(m.balance(protocol), 40);
    assert_eq!(
        m.balance(peer_a.worker) + m.balance(peer_b.worker) + m.balance(host) + m.balance(protocol),
        400
    );
    let after = m.reward_state().ledger().unwrap();
    assert_eq!(
        m.reward_state()
            .rows()
            .records()
            .collect::<CodecResult<Vec<_>>>()
            .unwrap(),
        rows_before
    );
    assert_eq!(after.tracked_deposits - before.tracked_deposits, 400);
    assert_eq!(after.liability - before.liability, 400);
    assert_eq!(after.free, before.free);
    assert_eq!(after.reserved, before.reserved);
    let before_replay = m.bytes.clone();
    let reply = producers::compose(&m.bytes, &e, owner(), AT + 2).unwrap();
    assert!(matches!(reply.routed, Routed::Unchanged { .. }));
    assert_eq!(m.bytes, before_replay);
}

#[test]
fn exceeding_settled_amount_and_mismatched_batch_refuse_without_credits() {
    let mut m = Market::new();
    m.settle_batch(batch(0, 4, 3));
    m.refused_call(
        dispatch::SplitPayout,
        owner(),
        &m.split_request(401).encode().unwrap(),
        AT + 1,
        F06_INVALID_AMOUNT,
    );
    m.refused_call(
        dispatch::SplitPayout,
        owner(),
        &m.split_request(0).encode().unwrap(),
        AT + 1,
        F06_INVALID_AMOUNT,
    );
    m.refused_call(
        dispatch::SplitPayout,
        owner(),
        &m.split_request(399).encode().unwrap(),
        AT + 1,
        F06_WRONG_CLAIM_AMOUNT,
    );
    let mut request = m.split_request(400);
    request.batch = digest(99);
    m.refused_call(
        dispatch::SplitPayout,
        owner(),
        &request.encode().unwrap(),
        AT + 1,
        CONFLICT,
    );
    assert_eq!(m.balance(m.peer(20).worker), 0);
}

#[test]
fn host_or_any_account_after_challenge_window() {
    let mut m = Market::new();
    m.settle_batch(batch(0, 4, 3));
    let payload = m.split_request(400).encode().unwrap();
    m.refused_call(
        dispatch::SplitPayout,
        actor(120),
        &payload,
        START + 111,
        UNAUTHORIZED,
    );
    m.split(400, actor(120), START + 112);
    assert_eq!(m.balance(m.peer(20).worker), 80);
}

#[test]
fn open_challenge_blocks_host_and_permissionless_split() {
    let mut m = Market::new();
    m.settle_batch(batch(0, 4, 3));
    m.canary(1);
    let payload = m.split_request(400).encode().unwrap();
    m.refused_call(dispatch::SplitPayout, owner(), &payload, AT + 2, CONFLICT);
    m.refused_call(
        dispatch::SplitPayout,
        actor(120),
        &payload,
        START + 112,
        CONFLICT,
    );
    assert_eq!(m.balance(m.peer(20).worker), 0);
}

#[test]
fn faulty_peer_loses_only_the_faulty_lap_share_to_other_peers() {
    let mut m = Market::new();
    m.settle_batch(batch(0, 4, 3));
    let (task, id) = m.canary(1);
    m.resolve_fault(task, id);
    m.split(400, owner(), START + 82);
    assert_eq!(m.balance(m.peer(20).worker), 60);
    assert_eq!(m.balance(m.peer(21).worker), 260);
    assert_eq!(
        m.balance(m.peer(20).worker) + m.balance(m.peer(21).worker),
        320
    );
}

#[test]
fn claim_pays_frozen_recipient_once_and_later_batch_credits_only_the_delta() {
    let mut m = Market::new();
    m.settle_batch(batch(0, 4, 3));
    m.split(400, owner(), AT + 1);
    let peer = m.peer(20);
    let mut claim = [0; rewards::CLAIM_PAYLOAD_BYTES];
    ClaimRequest {
        worker: peer.worker,
        recipient: peer.recipient,
        amount: 80,
    }
    .encode(&mut claim)
    .unwrap();
    for (recipient, amount, error) in [
        (
            AccountId::new([99; 32]).unwrap(),
            80,
            F06_WRONG_CLAIM_RECIPIENT,
        ),
        (peer.recipient, 81, F06_WRONG_CLAIM_AMOUNT),
    ] {
        let mut invalid = [0; rewards::CLAIM_PAYLOAD_BYTES];
        ClaimRequest {
            worker: peer.worker,
            recipient,
            amount,
        }
        .encode(&mut invalid)
        .unwrap();
        m.refused_call(dispatch::CLAIM, actor(123), &invalid, AT + 2, error);
    }
    let e = producers::envelope(&m.bytes, dispatch::CLAIM, actor(123), &claim, 0).unwrap();
    let before = m.reward_state().ledger().unwrap();
    let reply = producers::compose(&m.bytes, &e, actor(123), AT + 2).unwrap();
    assert!(
        matches!(reply.routed, Routed::Applied { .. }),
        "{:?}",
        codec::decode_result(&reply.result).unwrap()
    );
    m.bytes = reply.state;
    let after = m.reward_state().ledger().unwrap();
    assert_eq!(after.total_claimed - before.total_claimed, 80);
    assert_eq!(before.liability - after.liability, 80);
    let bound =
        RewardsAccount::for_ledger(producers::header(&m.bytes).unwrap().program_id, &before)
            .unwrap();
    let action = value_adapter::plan(
        &bound,
        &before,
        &after,
        rewards::RewardEffect::Payout {
            recipient: peer.recipient,
            amount: 80,
        },
    )
    .unwrap();
    assert_eq!(
        action,
        ValueAction::Pay {
            recipient: peer.recipient,
            amount: 80
        }
    );
    assert_eq!(m.balance(peer.worker), 0);
    let repeated = producers::compose(&m.bytes, &e, actor(123), AT + 3).unwrap();
    assert!(matches!(repeated.routed, Routed::Unchanged { .. }));
    m.settle_batch(batch(4, 4, 3));
    m.split(400, owner(), AT + 4);
    assert_eq!(m.balance(peer.worker), 80);
    let claim_e = producers::envelope(&m.bytes, dispatch::CLAIM, actor(123), &claim, 0).unwrap();
    let result = producers::compose(&m.bytes, &claim_e, actor(123), AT + 5).unwrap();
    assert!(matches!(result.routed, Routed::Applied { .. }));
    m.bytes = result.state;
    assert_eq!(m.balance(peer.worker), 0);
    assert_eq!(
        m.reward_state().ledger().unwrap().total_claimed - before.total_claimed,
        160
    );
}

#[test]
fn integer_split_conserves_extreme_amounts_and_refuses_all_peers_at_fault() {
    let m = Market::new();
    let room = RoomBook::decode(
        decode_shared_state(&m.bytes)
            .unwrap()
            .room_market()
            .unwrap()
            .1,
    )
    .unwrap()
    .room(digest(60))
    .unwrap()
    .unwrap();
    let mut peers = [(m.peer(20).worker, 1), (m.peer(21).worker, 3)];
    peers.sort_by_key(|p| p.0);
    for amount in [1, 7, 400, u128::MAX] {
        let split: rewards::PayoutShares =
            split_settled_amount(amount, &room.profile, &peers, &[0; 4]).unwrap();
        assert_eq!(split.peer_count, peers.len());
        assert_eq!(
            split.amounts[..2]
                .iter()
                .try_fold(split.host + split.protocol, |sum, a| sum.checked_add(*a)),
            Some(amount)
        );
        let fault = split_settled_amount(amount, &room.profile, &peers, &[1, 0, 0, 0]).unwrap();
        assert_eq!(
            fault.amounts[..2]
                .iter()
                .try_fold(fault.host + fault.protocol, |sum, a| sum.checked_add(*a)),
            Some(amount)
        );
    }
    assert_eq!(
        split_settled_amount(400, &room.profile, &peers, &[3]).unwrap_err(),
        F06_INVALID_AMOUNT
    );
}

#[test]
fn host_and_protocol_shares_use_the_same_claim_operation() {
    let mut m = Market::new();
    m.settle_batch(batch(0, 4, 3));
    m.split(400, owner(), AT + 1);
    let before = m.reward_state().ledger().unwrap();
    for (domain, principal) in [
        ("PAXAI/payout-host/v1", owner()),
        ("PAXAI/payout-protocol/v1", actor(15)),
    ] {
        let worker = WorkerId::new(
            codec::domain_hash(domain, principal.as_bytes())
                .unwrap()
                .bytes(),
        )
        .unwrap();
        let mut payload = [0; rewards::CLAIM_PAYLOAD_BYTES];
        ClaimRequest {
            worker,
            recipient: AccountId::new(principal.bytes()).unwrap(),
            amount: 40,
        }
        .encode(&mut payload)
        .unwrap();
        m.call(dispatch::CLAIM, actor(123), &payload, AT + 2);
        assert_eq!(m.balance(worker), 0);
    }
    let after = m.reward_state().ledger().unwrap();
    assert_eq!(after.total_claimed - before.total_claimed, 80);
    assert_eq!(before.liability - after.liability, 80);
    assert_eq!(m.balance(m.peer(20).worker), 80);
    assert_eq!(m.balance(m.peer(21).worker), 240);
}

#[test]
fn expired_payouts_release_only_unclaimed_credits_through_existing_expiry() {
    let mut m = Market::new();
    m.settle_batch(batch(0, 4, 3));
    m.split(400, owner(), AT + 1);
    let peer = m.peer(20);
    let mut payload = [0; rewards::CLAIM_PAYLOAD_BYTES];
    ClaimRequest {
        worker: peer.worker,
        recipient: peer.recipient,
        amount: 80,
    }
    .encode(&mut payload)
    .unwrap();
    m.call(dispatch::CLAIM, actor(123), &payload, AT + 2);
    let expiry = AT + 1 + layerx_programs_ai_market::reward_math::CLAIM_EXPIRY_HEIGHTS;
    let before = m.reward_state().ledger().unwrap();
    m.refused_call(
        dispatch::EXPIRE_EPOCH_CLAIMS,
        actor(123),
        &[],
        expiry - 1,
        WRONG_PHASE,
    );
    m.call(dispatch::EXPIRE_EPOCH_CLAIMS, actor(123), &[], expiry);
    let after = m.reward_state().ledger().unwrap();
    assert_eq!(before.liability - after.liability, 320);
    assert_eq!(after.free - before.free, 320);
    assert_eq!(after.total_claimed, before.total_claimed);
    assert_eq!(after.reserved, before.reserved);
    assert_eq!(m.balance(m.peer(21).worker), 0);
    m.refused_call(
        dispatch::CLAIM,
        actor(123),
        &payload,
        expiry,
        F06_CLAIM_EXPIRED,
    );
}
