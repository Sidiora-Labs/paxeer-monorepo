use super::observe;
use ed25519_dalek::{Signer, SigningKey};
use layerx_programs_ai_market::{
    admission::{
        Admission, AdmissionContext, AdmissionTable, ApprovalTerms, Participant, TABLE_MAX_BYTES,
    },
    codec::{self, Envelope, ResultStatus, Writer},
    dispatch::{self, Buffers, Operation, Routed, SequencePolicy},
    errors::*,
    evaluators::authority::split_identity_section,
    placement::SIDE_CAPACITY,
    policy::{PolicyCommitments, RoomProfile, TaskPolicyV1, TASK_POLICY_BYTES},
    registry::{derive_rewards_account, MarketHeader},
    registry_ops::{CallContext, PolicySection},
    reputation_transition::split_joint,
    rewards::{FundRequest, FUNDING_POLICY_VERSION, REWARD_STATE_BYTES},
    rooms::*,
    roster,
    state::{self, ActorSlot, Section},
    types::*,
    workers::{self, CapabilityFields, ManifestV2, WorkerCurrent, WorkerTable},
    MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES, MERGED_CHAIN_DOMAIN,
};

const ORIGIN: u64 = 1000;
const HOST: u8 = 12;
const PEER_A: u8 = 20;
const PEER_B: u8 = 21;
const ASKER: u8 = 30;

fn digest(n: u8) -> Digest32 {
    Digest32::new([n; 32]).unwrap()
}
fn principal(n: u8) -> PrincipalId {
    PrincipalId::new([n; 32]).unwrap()
}
fn profile() -> RoomProfile {
    RoomProfile {
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
    }
}
fn payload(size: usize, encode: impl FnOnce(&mut [u8]) -> CodecResult<usize>) -> Vec<u8> {
    let mut out = vec![0; size];
    let n = encode(&mut out).unwrap_or_else(|error| {
        let name = APPLICATION_ERRORS
            .iter()
            .find(|entry| entry.code == error.code())
            .unwrap();
        panic!(
            "fixture payload capacity={size}: {} (0x{:04x})",
            name.name,
            error.code()
        );
    });
    out.truncate(n);
    out
}
fn open_request(room_id: Digest32) -> OpenRoomRequest {
    OpenRoomRequest {
        room_id,
        model_digest: digest(50),
        runtime_digest: digest(51),
        profile: profile(),
    }
}
fn ask_request(room_id: Digest32, ask_id: Digest32, stream_id: u128) -> PostAskRequest {
    PostAskRequest {
        room_id,
        ask_id,
        model_digest: digest(50),
        maximum_price_per_thousand_tokens: 20,
        stream_id,
    }
}

struct Harness {
    current: Vec<u8>,
    height: u64,
    request: u64,
}
impl Harness {
    fn context(&self, actor: u8) -> CallContext {
        CallContext {
            chain: ChainDomain::new(MERGED_CHAIN_DOMAIN).unwrap(),
            program: ProgramId::new([11; 32]).unwrap(),
            principal: principal(actor),
            height: self.height,
        }
    }
    fn state(&self) -> state::SharedState<'_> {
        state::decode_shared_state(&self.current).unwrap()
    }
    fn header(&self) -> MarketHeader {
        PolicySection::decode(self.state().section(Section::PolicyLifecycle).unwrap())
            .unwrap()
            .header
    }
    fn workers(&self) -> WorkerTable {
        WorkerTable::decode(
            split_identity_section(self.state().section(Section::IdentityRoster).unwrap())
                .unwrap()
                .0,
        )
        .unwrap()
    }
    fn worker(&self, peer: u8) -> WorkerCurrent {
        *self
            .workers()
            .iter()
            .find(|w| w.owner == principal(peer))
            .unwrap()
    }
    fn admission(&self) -> AdmissionTable {
        AdmissionTable::decode(
            split_joint(self.state().section(Section::ReputationAdmission).unwrap())
                .unwrap()
                .1,
        )
        .unwrap()
    }
    fn roster(&self) -> RosterDigest {
        RosterDigest::new(
            roster::roster_digest(&self.admission(), self.admission().current_epoch().unwrap())
                .unwrap()
                .bytes(),
        )
        .unwrap()
    }
    fn book(&self) -> RoomBook<'_> {
        RoomBook::decode(self.state().room_market().unwrap().1).unwrap()
    }
    fn sequence(&self, actor: u8) -> u64 {
        if self.current.is_empty() {
            return 1;
        }
        let state = self.state();
        (0..43)
            .filter_map(|i| {
                state
                    .control
                    .replay
                    .actor(ActorSlot::from_index(i).unwrap())
            })
            .find(|a| a.principal == principal(actor))
            .map_or(1, |a| a.last.map_or(1, |last| last.sequence + 1))
    }
    fn envelope(
        &mut self,
        op: Operation,
        actor: u8,
        bytes: &[u8],
        roster: Presence<RosterDigest>,
    ) -> Vec<u8> {
        self.request += 1;
        let mut request = [1; 32];
        request[..8].copy_from_slice(&self.request.to_be_bytes());
        let ctx = self.context(actor);
        let roster = if op.metadata().feature == 11 && roster == Presence::Absent {
            Presence::Present(self.roster())
        } else {
            roster
        };
        payload(MAX_ENVELOPE_BYTES, |out| {
            codec::encode_envelope(
                &Envelope {
                    operation: op,
                    chain: ctx.chain,
                    program: ctx.program,
                    market: codec::derive_market(ctx.chain, ctx.program)?,
                    actor: principal(actor),
                    epoch: (self.height - ORIGIN) / 128,
                    config: 1,
                    roster,
                    sequence: if op.metadata().sequence == SequencePolicy::ObjectLocal {
                        0
                    } else {
                        self.sequence(actor)
                    },
                    expiry: ORIGIN + 1000,
                    request: RequestId::new(request)?,
                    payload: bytes,
                    authentication: Authentication::Native,
                },
                out,
            )
        })
    }
    fn execute(&mut self, actor: u8, envelope: &[u8]) -> (Routed, Vec<u8>, Vec<u8>) {
        let before = self.current.clone();
        let mut next = vec![0; MAX_STATE_BYTES];
        let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
        let mut event = vec![0; MAX_EVENT_BYTES];
        let mut result = vec![0; MAX_RESULT_BYTES];
        let routed = dispatch::route(
            &self.context(actor),
            envelope,
            if self.current.is_empty() {
                None
            } else {
                Some(&self.current)
            },
            Buffers {
                next: &mut next,
                scratch: &mut scratch,
                event: &mut event,
                result: &mut result,
            },
        )
        .unwrap();
        super::observe(
            &self.context(actor),
            &before,
            envelope,
            match routed {
                Routed::Applied { state_len, .. } => &next[..state_len],
                _ => &before,
            },
            match routed {
                Routed::Applied { event_len, .. } => &event[..event_len],
                _ => &[],
            },
            match routed {
                Routed::Applied { result_len, .. }
                | Routed::Unchanged { result_len }
                | Routed::Refused { result_len } => &result[..result_len],
            },
            routed,
        );
        let result_len = match routed {
            Routed::Applied {
                state_len,
                event_len,
                result_len,
                revision,
                operation,
            } => {
                let framed = codec::decode_result(&result[..result_len]).unwrap();
                assert_eq!(framed.status, ResultStatus::Ok);
                assert_eq!(framed.revision, revision);
                let mut topic = [0; 64];
                let n = codec::event_topic(operation, &mut topic).unwrap();
                let (named, common, suffix) =
                    codec::decode_event_frame(&topic[..n], &event[..event_len]).unwrap();
                assert_eq!(named, operation);
                assert_eq!(common.revision, revision);
                assert_eq!(common.result, framed.digest);
                if operation.metadata().feature == 11 {
                    assert_eq!(suffix, framed.payload);
                    assert_eq!(suffix.len(), 32);
                }
                self.current = next[..state_len].to_vec();
                event.truncate(event_len);
                assert!(state_len <= MAX_STATE_BYTES);
                assert_eq!(self.state().revision, revision);
                assert_eq!(self.header().state_revision, revision);
                result_len
            }
            Routed::Unchanged { result_len } | Routed::Refused { result_len } => {
                assert_eq!(self.current, before);
                event.clear();
                result_len
            }
        };
        result.truncate(result_len);
        (routed, result, event)
    }
    fn apply(
        &mut self,
        op: Operation,
        actor: u8,
        bytes: &[u8],
        roster: Presence<RosterDigest>,
    ) -> Vec<u8> {
        if op == dispatch::PostBid {
            let mut r = codec::Reader::new(bytes);
            let room = r.digest32().unwrap();
            let bid = r.digest32().unwrap();
            r.take(48).unwrap();
            let amount = r.u128().unwrap();
            if layerx_programs_ai_market::bonds::bond_from_state(&self.current, bid)
                .unwrap()
                .is_none()
            {
                self.apply(
                    dispatch::LockBond,
                    actor,
                    &layerx_programs_ai_market::bonds::LockBondRequest { room, bid, amount }
                        .encode()
                        .unwrap(),
                    roster,
                );
            }
        }
        let envelope = self.envelope(op, actor, bytes, roster);
        let (routed, result, _) = self.execute(actor, &envelope);
        assert!(
            matches!(routed, Routed::Applied { .. }),
            "{op:?}: {:?}",
            codec::decode_result(&result).unwrap()
        );
        envelope
    }
    fn refuse(
        &mut self,
        op: Operation,
        actor: u8,
        bytes: &[u8],
        roster: Presence<RosterDigest>,
        error: ApplicationError,
    ) {
        let before = self.current.clone();
        let envelope = self.envelope(op, actor, bytes, roster);
        let (routed, result, _) = self.execute(actor, &envelope);
        assert!(matches!(routed, Routed::Refused { .. }), "{routed:?}");
        assert_eq!(codec::decode_result(&result).unwrap().error, Some(error));
        assert_eq!(self.current, before);
    }
    fn install_admission(&mut self, table: &AdmissionTable) {
        let mut bytes = vec![0; TABLE_MAX_BYTES];
        let n = table.encode(&mut bytes).unwrap();
        let mut out = vec![0; MAX_STATE_BYTES];
        let mut control = vec![0; Section::Control.payload_cap()];
        let state = self.state();
        let next = state
            .replace_section(Section::ReputationAdmission, &bytes[..n])
            .unwrap();
        let n = state::encode_shared_state(&next, &mut out, &mut control).unwrap();
        self.current = out[..n].to_vec();
    }
    fn new() -> Self {
        let mut h = Self {
            current: Vec::new(),
            height: ORIGIN,
            request: 0,
        };
        let ctx = h.context(HOST);
        let policy = TaskPolicyV1::bounded_default(
            1,
            1,
            PolicyCommitments {
                model_artifact: digest(50),
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
        let mut create = principal(HOST).as_bytes().to_vec();
        create.extend_from_slice(&[13; 32]);
        create.extend_from_slice(
            derive_rewards_account(ctx.program, AssetId::new([13; 32]).unwrap())
                .unwrap()
                .as_bytes(),
        );
        create.extend_from_slice(&[14; 32]);
        create.push(0);
        create.extend_from_slice(&payload(TASK_POLICY_BYTES, |out| policy.encode(out)));
        create.extend_from_slice(&[16; 32]);
        h.apply(dispatch::CREATE, HOST, &create, Presence::Absent);
        let fund = payload(57, |out| {
            FundRequest {
                amount: 1000,
                refund_recipient: AccountId::new([14; 32])?,
                policy_version: FUNDING_POLICY_VERSION,
                consent: true,
            }
            .encode(out)
        });
        h.apply(dispatch::FUND, HOST, &fund, Presence::Absent);
        for peer in [PEER_A, PEER_B] {
            let key = SigningKey::from_bytes(&[peer; 32]);
            let public = PublicKey32(key.verifying_key().to_bytes());
            let nonce = [peer + 40; 32];
            let market = h.header();
            let worker = codec::derive_worker(market.market_id, principal(peer), nonce).unwrap();
            let initial_manifest =
                manifest(market.market_id, worker, peer, 1, ORIGIN, ORIGIN + 128);
            let metadata = workers::decode_manifest(&initial_manifest).unwrap().digest;
            let enroll = payload(136, |out| {
                let mut w = Writer::new(out);
                w.put(principal(peer).as_bytes())?;
                w.put(&nonce)?;
                w.put(&public.0)?;
                w.put(metadata.as_bytes())?;
                w.u64(ORIGIN + 128)?;
                Ok(w.len())
            });
            h.apply(dispatch::EnrollWorker, HOST, &enroll, Presence::Absent);
            let consent = workers::consent_digest(
                &h.header(),
                worker,
                principal(peer),
                public,
                1,
                1,
                metadata,
                ORIGIN + 128,
            )
            .unwrap();
            let accept = payload(144, |out| {
                let mut w = Writer::new(out);
                w.put(worker.as_bytes())?;
                w.u64(1)?;
                w.u64(1)?;
                w.put(metadata.as_bytes())?;
                w.put(&key.sign(consent.as_bytes()).to_bytes())?;
                Ok(w.len())
            });
            h.apply(dispatch::AcceptEnrollment, peer, &accept, Presence::Absent);
        }
        h.height = ORIGIN + 8;
        for peer in [PEER_A, PEER_B] {
            let key = SigningKey::from_bytes(&[peer; 32]);
            let record = h.worker(peer);
            let manifest = manifest(
                h.header().market_id,
                record.worker,
                peer,
                2,
                ORIGIN + 8,
                ORIGIN + 264,
            );
            let metadata = workers::decode_manifest(&manifest).unwrap().digest;
            let publish = payload(100 + manifest.len(), |out| {
                let mut w = Writer::new(out);
                w.put(record.worker.as_bytes())?;
                w.u64(1)?;
                w.u64(2)?;
                w.put(metadata.as_bytes())?;
                w.u64(ORIGIN + 8)?;
                w.u64(ORIGIN + 264)?;
                w.bytes(&manifest, workers::MAX_MANIFEST_BYTES)?;
                Ok(w.len())
            });
            let native = h.envelope(dispatch::PublishMetadata, peer, &publish, Presence::Absent);
            let mut envelope = codec::decode_envelope(&native).unwrap().envelope;
            envelope.authentication = Authentication::Delegate {
                key: PublicKey32(key.verifying_key().to_bytes()),
                signature: Signature64([0; 64]),
            };
            let unsigned = payload(MAX_ENVELOPE_BYTES, |out| {
                codec::encode_envelope(&envelope, out)
            });
            let hash = codec::decode_envelope(&unsigned)
                .unwrap()
                .request_digest()
                .unwrap();
            envelope.authentication = Authentication::Delegate {
                key: PublicKey32(key.verifying_key().to_bytes()),
                signature: Signature64(key.sign(hash.as_bytes()).to_bytes()),
            };
            let signed = payload(MAX_ENVELOPE_BYTES, |out| {
                codec::encode_envelope(&envelope, out)
            });
            let (routed, result, _) = h.execute(peer, &signed);
            assert!(
                matches!(routed, Routed::Applied { .. }),
                "{:?}",
                codec::decode_result(&result).unwrap()
            );
        }
        let market = h.header();
        let mut table = AdmissionTable::new();
        for peer in [PEER_A, PEER_B] {
            let worker = h.worker(peer);
            let terms = ApprovalTerms {
                participant: Participant::Worker(worker.worker),
                owner: principal(peer),
                enrollment_nonce_commitment: digest(peer + 40),
                delegate: worker.delegate,
                delegate_generation: 1,
                identity_commitment: worker.proposal_digest(&market).unwrap(),
                effective_epoch: 0,
                config_version: 1,
                request: RequestId::new([peer; 32]).unwrap(),
                expiry_height: ORIGIN + 64,
            };
            let approval = table
                .approve(
                    &AdmissionContext {
                        market: &market,
                        invoking_principal: principal(HOST),
                        height: h.height,
                    },
                    &terms,
                )
                .unwrap();
            table
                .admit(
                    &AdmissionContext {
                        market: &market,
                        invoking_principal: principal(peer),
                        height: h.height,
                    },
                    &Admission {
                        participant: terms.participant,
                        delegate_generation: 1,
                        effective_epoch: 0,
                        config_version: 1,
                        approval_digest: approval,
                    },
                )
                .unwrap();
        }
        roster::open_epoch(&mut table, 1, |_| false).unwrap();
        h.install_admission(&table);
        h.height = ORIGIN + 129;
        h
    }
    fn open(&mut self, room: Digest32) -> Vec<u8> {
        self.apply(
            dispatch::OpenRoom,
            HOST,
            &payload(
                layerx_programs_ai_market::rooms::OPEN_ROOM_REQUEST_BYTES,
                |out| open_request(room).encode(out),
            ),
            Presence::Absent,
        )
    }
    fn ask(&mut self, room: Digest32, ask: Digest32, stream: u128) -> Vec<u8> {
        self.apply(
            dispatch::PostAsk,
            ASKER,
            &payload(128, |out| ask_request(room, ask, stream).encode(out)),
            Presence::Absent,
        )
    }
    fn bid_request(&self, room: Digest32, bid: Digest32, peer: u8) -> PostBidRequest {
        PostBidRequest {
            room_id: room,
            bid_id: bid,
            capability_digest: Digest32::new(self.worker(peer).metadata.bytes()).unwrap(),
            minimum_price_per_thousand_tokens: 5,
            bond_amount: 100,
            layer_capacity: 16,
        }
    }
    fn bid(&mut self, room: Digest32, bid: Digest32, peer: u8) -> Vec<u8> {
        let request = self.bid_request(room, bid, peer);
        self.apply(
            dispatch::PostBid,
            peer,
            &payload(130, |out| request.encode(out)),
            Presence::Absent,
        )
    }
    fn deal_request(&self, room: Digest32, ask: Digest32, deal: Digest32) -> RecordDealRequest {
        let mut layers = [None; MAX_DEAL_PEERS];
        layers[0] = Some(LayerSplit {
            bid_id: digest(71),
            first_layer: 0,
            end_layer: 8,
        });
        layers[1] = Some(LayerSplit {
            bid_id: digest(70),
            first_layer: 8,
            end_layer: 20,
        });
        RecordDealRequest {
            room_id: room,
            deal_id: deal,
            ask_id: ask,
            agreed_price_per_thousand_tokens: 10,
            roster_digest: self.roster(),
            layers,
        }
    }
    fn ready(stream: u128) -> Self {
        let mut h = Self::new();
        h.open(digest(60));
        h.ask(digest(60), digest(61), stream);
        h.bid(digest(60), digest(70), PEER_A);
        h.bid(digest(60), digest(71), PEER_B);
        h
    }
    fn deal(&mut self, request: &RecordDealRequest) -> Vec<u8> {
        self.apply(
            dispatch::RecordDeal,
            HOST,
            &payload(1297, |out| request.encode(out)),
            Presence::Present(request.roster_digest),
        )
    }
}

fn manifest(
    market: MarketId,
    worker: WorkerId,
    peer: u8,
    revision: u64,
    from: u64,
    expiry: u64,
) -> Vec<u8> {
    let v1 = payload(workers::MAX_MANIFEST_BYTES, |out| {
        let mut w = Writer::new(out);
        w.u16(1)?;
        w.put(market.as_bytes())?;
        w.put(worker.as_bytes())?;
        w.put(principal(peer).as_bytes())?;
        for n in [1, 1, revision, from, expiry] {
            w.u64(n)?;
        }
        w.put(&[51; 32])?;
        w.u16(1)?;
        w.u8(1)?;
        w.u8(1)?;
        for n in [50, 52, 53, 54, 55] {
            w.put(&[n; 32])?;
        }
        for n in [1024, 2048, 4096, 8192] {
            w.u32(n)?;
        }
        w.u8(1)?;
        w.u32(60000)?;
        w.u16(4)?;
        w.u8(1)?;
        w.u16(1)?;
        w.u8(1)?;
        w.bytes(b"https://peer.example/paxai/v1", workers::MAX_URI_BYTES)?;
        w.put(&[56; 32])?;
        w.u8(1)?;
        w.u16(1)?;
        w.u16(1)?;
        w.put(&[57; 32])?;
        w.put(&[58; 32])?;
        w.u32(0)?;
        Ok(w.len())
    });
    payload(workers::MAX_MANIFEST_BYTES, |out| {
        codec::encode_manifest_v2(
            &ManifestV2 {
                manifest_v1: &v1,
                capabilities: CapabilityFields {
                    accelerator_memory_mib: 24576,
                    system_memory_mib: 65536,
                    bandwidth_class: 4,
                    region_code: *b"US",
                    attestation_family: 255,
                    runtime_kind: 2,
                },
            },
            out,
        )
    })
}

fn open_ask_bid_deal_persist_with_ordered_layers_and_roster_binding() {
    let mut h = Harness::ready(123);
    let before = h.state();
    let legacy = before.room_market().unwrap().0.to_vec();
    let identity = before.section(Section::IdentityRoster).unwrap().to_vec();
    let admission = before
        .section(Section::ReputationAdmission)
        .unwrap()
        .to_vec();
    assert_eq!(legacy.len(), REWARD_STATE_BYTES);
    let request = h.deal_request(digest(60), digest(61), digest(62));
    let encoded = h.deal(&request);
    let persisted = h.current.clone();
    let mut restarted = Harness {
        current: persisted.clone(),
        height: h.height,
        request: h.request,
    };
    assert_eq!(restarted.current, persisted);
    let room = restarted.book().room(digest(60)).unwrap().unwrap();
    assert_eq!(room.host_key, principal(HOST));
    assert_eq!(room.model_digest, digest(50));
    assert_eq!(room.runtime_digest, digest(51));
    assert_eq!(room.profile, profile());
    assert_eq!(room.state, RoomState::Open);
    assert_eq!(room.opened_at_epoch, 1);
    let ask = restarted.book().ask(digest(61)).unwrap().unwrap();
    assert_eq!(ask.asker_key, principal(ASKER));
    assert_eq!(ask.stream_id, 123);
    assert_eq!(ask.state, AskState::Matched);
    let bid = restarted.book().bid(digest(70)).unwrap().unwrap();
    assert_eq!(bid.peer_key, principal(PEER_A));
    assert_eq!(
        bid.capability_digest.bytes(),
        restarted.worker(PEER_A).metadata.bytes()
    );
    assert_eq!(bid.minimum_price_per_thousand_tokens, 5);
    assert_eq!(bid.bond_amount, 100);
    assert_eq!(bid.layer_capacity, 16);
    let deal = restarted.book().deal(digest(62)).unwrap().unwrap();
    assert_eq!(deal.ask_id, digest(61));
    assert_eq!(deal.layers, request.layers);
    assert_eq!(deal.roster_digest, restarted.roster());
    assert_eq!(deal.agreed_price_per_thousand_tokens, 10);
    assert_eq!(deal.state, DealState::Open);
    let (routed, result, _) = restarted.execute(HOST, &encoded);
    assert!(matches!(routed, Routed::Unchanged { .. }));
    assert_eq!(
        codec::decode_result(&result).unwrap().status,
        ResultStatus::AlreadyApplied
    );
    assert_eq!(restarted.current, persisted);
    restarted.ask(digest(60), digest(63), 124);
    assert_eq!(restarted.book().counts(), [1, 2, 2, 1]);
    let state = restarted.state();
    assert_eq!(state.room_market().unwrap().0, legacy);
    assert_eq!(state.section(Section::IdentityRoster).unwrap(), identity);
    assert_eq!(
        state.section(Section::ReputationAdmission).unwrap(),
        admission
    );
}

fn host_can_close_an_idle_room_and_closed_rooms_refuse_new_work() {
    let mut h = Harness::new();
    h.open(digest(60));
    let encoded = h.apply(
        dispatch::CloseRoom,
        HOST,
        digest(60).as_bytes(),
        Presence::Absent,
    );
    assert_eq!(
        h.book().room(digest(60)).unwrap().unwrap().state,
        RoomState::Closed
    );
    let (routed, _, _) = h.execute(HOST, &encoded);
    assert!(matches!(routed, Routed::Unchanged { .. }));
    h.refuse(
        dispatch::PostAsk,
        ASKER,
        &payload(128, |out| {
            ask_request(digest(60), digest(61), 1).encode(out)
        }),
        Presence::Absent,
        F11_ROOM_CLOSED,
    );
    let bid = h.bid_request(digest(60), digest(70), PEER_A);
    h.refuse(
        dispatch::PostBid,
        PEER_A,
        &payload(130, |out| bid.encode(out)),
        Presence::Absent,
        F11_ROOM_CLOSED,
    );
}

pub fn recorded_journey() {
    open_ask_bid_deal_persist_with_ordered_layers_and_roster_binding();
    host_can_close_an_idle_room_and_closed_rooms_refuse_new_work();
}

pub fn room_profile_wire_round_trip() {
    let request = open_request(digest(60));
    request.profile.validate().unwrap();
    let mut profile_bytes = [0; layerx_programs_ai_market::policy::ROOM_PROFILE_BYTES];
    assert_eq!(
        request.profile.encode(&mut profile_bytes).unwrap(),
        profile_bytes.len()
    );
    assert_eq!(
        RoomProfile::decode(&profile_bytes).unwrap(),
        request.profile
    );
    let mut bytes = vec![0; OPEN_ROOM_REQUEST_BYTES];
    assert_eq!(request.encode(&mut bytes).unwrap(), OPEN_ROOM_REQUEST_BYTES);
    assert_eq!(&bytes[96..], profile_bytes);
    let mut short = vec![0; OPEN_ROOM_REQUEST_BYTES - 1];
    assert_eq!(request.encode(&mut short), Err(CAPACITY));
}
