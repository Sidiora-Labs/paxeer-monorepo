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
    }
}
fn payload(size: usize, encode: impl FnOnce(&mut [u8]) -> CodecResult<usize>) -> Vec<u8> {
    let mut out = vec![0; size];
    let n = encode(&mut out).unwrap();
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
            &payload(168, |out| open_request(room).encode(out)),
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

#[test]
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

#[test]
fn roster_digest_mismatch_refuses_without_consuming_the_host_sequence() {
    let mut h = Harness::ready(123);
    let sequence = h.sequence(HOST);
    let mut request = h.deal_request(digest(60), digest(61), digest(62));
    request.roster_digest = RosterDigest::new([99; 32]).unwrap();
    let bytes = payload(1297, |out| request.encode(out));
    h.refuse(
        dispatch::RecordDeal,
        HOST,
        &bytes,
        Presence::Present(request.roster_digest),
        F11_ROSTER_DIGEST_MISMATCH,
    );
    assert_eq!(h.sequence(HOST), sequence);
    assert_eq!(h.book().counts(), [1, 1, 2, 0]);
    request.roster_digest = h.roster();
    h.deal(&request);
}

#[test]
fn a_deal_cannot_reuse_a_digest_from_an_earlier_admitted_roster() {
    let mut h = Harness::ready(123);
    let request = h.deal_request(digest(60), digest(61), digest(62));
    let mut table = h.admission();
    roster::open_epoch(&mut table, 2, |_| false).unwrap();
    h.install_admission(&table);
    h.height = ORIGIN + 258;
    assert_ne!(h.roster(), request.roster_digest);
    h.refuse(
        dispatch::RecordDeal,
        HOST,
        &payload(1297, |out| request.encode(out)),
        Presence::Present(request.roster_digest),
        F11_ROSTER_DIGEST_MISMATCH,
    );
}

#[test]
fn unfunded_ask_is_not_matched_and_no_deal_is_recorded() {
    let mut h = Harness::ready(0);
    let request = h.deal_request(digest(60), digest(61), digest(62));
    h.refuse(
        dispatch::RecordDeal,
        HOST,
        &payload(1297, |out| request.encode(out)),
        Presence::Present(request.roster_digest),
        F11_ASK_UNFUNDED,
    );
    assert_eq!(
        h.book().ask(digest(61)).unwrap().unwrap().state,
        AskState::Open
    );
    assert!(h.book().deal(digest(62)).unwrap().is_none());
}

#[test]
fn deal_price_respects_every_bid_floor_and_the_ask_ceiling() {
    let mut h = Harness::ready(123);
    let mut request = h.deal_request(digest(60), digest(61), digest(62));
    request.agreed_price_per_thousand_tokens = 4;
    h.refuse(
        dispatch::RecordDeal,
        HOST,
        &payload(1297, |out| request.encode(out)),
        Presence::Present(request.roster_digest),
        F11_BID_BELOW_FLOOR,
    );
    request.agreed_price_per_thousand_tokens = 21;
    h.refuse(
        dispatch::RecordDeal,
        HOST,
        &payload(1297, |out| request.encode(out)),
        Presence::Present(request.roster_digest),
        CONFLICT,
    );
    request.agreed_price_per_thousand_tokens = 5;
    h.deal(&request);
}

#[test]
fn close_with_open_deals_refuses_and_retains_the_room() {
    let mut h = Harness::ready(123);
    let request = h.deal_request(digest(60), digest(61), digest(62));
    h.deal(&request);
    h.refuse(
        dispatch::CloseRoom,
        HOST,
        digest(60).as_bytes(),
        Presence::Absent,
        WRONG_PHASE,
    );
    assert_eq!(
        h.book().room(digest(60)).unwrap().unwrap().state,
        RoomState::Open
    );
    assert_eq!(
        h.book().deal(digest(62)).unwrap().unwrap().state,
        DealState::Open
    );
}

#[test]
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

#[test]
fn room_host_authority_and_native_identity_are_enforced() {
    let mut h = Harness::ready(123);
    let request = h.deal_request(digest(60), digest(61), digest(62));
    h.refuse(
        dispatch::CloseRoom,
        PEER_A,
        digest(60).as_bytes(),
        Presence::Absent,
        UNAUTHORIZED,
    );
    h.refuse(
        dispatch::RecordDeal,
        PEER_A,
        &payload(1297, |out| request.encode(out)),
        Presence::Present(request.roster_digest),
        UNAUTHORIZED,
    );
    let bytes = payload(168, |out| open_request(digest(65)).encode(out));
    let envelope = h.envelope(dispatch::OpenRoom, HOST, &bytes, Presence::Absent);
    let (routed, result, _) = h.execute(PEER_A, &envelope);
    assert!(matches!(routed, Routed::Refused { .. }));
    assert_eq!(
        codec::decode_result(&result).unwrap().error,
        Some(UNAUTHORIZED)
    );
    assert!(h.book().room(digest(65)).unwrap().is_none());
    h.apply(dispatch::OpenRoom, PEER_A, &bytes, Presence::Absent);
    assert_eq!(
        h.book().room(digest(65)).unwrap().unwrap().host_key,
        principal(PEER_A)
    );
    h.refuse(
        dispatch::CloseRoom,
        HOST,
        digest(65).as_bytes(),
        Presence::Absent,
        UNAUTHORIZED,
    );
    h.apply(
        dispatch::CloseRoom,
        PEER_A,
        digest(65).as_bytes(),
        Presence::Absent,
    );
}

#[test]
fn bids_require_enrollment_matching_manifest_and_minimum_bond() {
    let mut h = Harness::new();
    h.open(digest(60));
    let mut request = h.bid_request(digest(60), digest(70), PEER_A);
    h.refuse(
        dispatch::PostBid,
        ASKER,
        &payload(130, |out| request.encode(out)),
        Presence::Absent,
        F02_CAPABILITY_MISMATCH,
    );
    request.capability_digest = digest(99);
    h.refuse(
        dispatch::PostBid,
        PEER_A,
        &payload(130, |out| request.encode(out)),
        Presence::Absent,
        F02_CAPABILITY_MISMATCH,
    );
    request.capability_digest = Digest32::new(h.worker(PEER_A).metadata.bytes()).unwrap();
    request.bond_amount = 99;
    h.refuse(
        dispatch::PostBid,
        PEER_A,
        &payload(130, |out| request.encode(out)),
        Presence::Absent,
        F11_BOND_INSUFFICIENT,
    );
    request.bond_amount = 100;
    h.apply(
        dispatch::PostBid,
        PEER_A,
        &payload(130, |out| request.encode(out)),
        Presence::Absent,
    );
    h.height = ORIGIN + 264;
    let request = h.bid_request(digest(60), digest(71), PEER_A);
    h.refuse(
        dispatch::PostBid,
        PEER_A,
        &payload(130, |out| request.encode(out)),
        Presence::Absent,
        F02_METADATA_EXPIRED,
    );
}

#[test]
fn split_capacity_cross_room_and_duplicate_peer_are_refused() {
    let mut h = Harness::ready(123);
    let mut request = h.deal_request(digest(60), digest(61), digest(62));
    request.layers[1].as_mut().unwrap().end_layer = 25;
    h.refuse(
        dispatch::RecordDeal,
        HOST,
        &payload(1297, |out| request.encode(out)),
        Presence::Present(request.roster_digest),
        CAPACITY,
    );
    h.open(digest(65));
    h.bid(digest(65), digest(72), PEER_A);
    request.layers[1] = Some(LayerSplit {
        bid_id: digest(72),
        first_layer: 8,
        end_layer: 20,
    });
    h.refuse(
        dispatch::RecordDeal,
        HOST,
        &payload(1297, |out| request.encode(out)),
        Presence::Present(request.roster_digest),
        CONFLICT,
    );
    h.bid(digest(60), digest(73), PEER_B);
    request.layers[1].as_mut().unwrap().bid_id = digest(73);
    h.refuse(
        dispatch::RecordDeal,
        HOST,
        &payload(1297, |out| request.encode(out)),
        Presence::Present(request.roster_digest),
        CONFLICT,
    );
    request.layers[1].as_mut().unwrap().bid_id = digest(70);
    request.layers[1].as_mut().unwrap().first_layer = 9;
    assert_eq!(request.encode(&mut [0; 1297]), Err(NON_CANONICAL));
    request.layers[1].as_mut().unwrap().first_layer = 8;
    request.layers[1].as_mut().unwrap().bid_id = digest(71);
    assert_eq!(request.encode(&mut [0; 1297]), Err(NON_CANONICAL));
}

#[test]
fn asks_from_any_account_are_idempotent_and_bound_to_the_room_model() {
    let mut h = Harness::new();
    h.open(digest(60));
    let encoded = h.ask(digest(60), digest(61), 123);
    let before = h.current.clone();
    let (routed, result, _) = h.execute(ASKER, &encoded);
    assert!(matches!(routed, Routed::Unchanged { .. }));
    assert_eq!(
        codec::decode_result(&result).unwrap().status,
        ResultStatus::AlreadyApplied
    );
    assert_eq!(h.current, before);
    h.refuse(
        dispatch::PostAsk,
        PEER_A,
        &payload(128, |out| {
            ask_request(digest(60), digest(61), 123).encode(out)
        }),
        Presence::Absent,
        CONFLICT,
    );
    let mut request = ask_request(digest(60), digest(62), 123);
    request.model_digest = digest(99);
    h.refuse(
        dispatch::PostAsk,
        ASKER,
        &payload(128, |out| request.encode(out)),
        Presence::Absent,
        CONFLICT,
    );
    request.room_id = digest(99);
    h.refuse(
        dispatch::PostAsk,
        ASKER,
        &payload(128, |out| request.encode(out)),
        Presence::Absent,
        F11_ROOM_UNKNOWN,
    );
}

#[test]
fn every_record_codec_is_fixed_width_and_rejects_truncation_trailing_and_invalid_states() {
    let mut h = Harness::ready(123);
    let request = h.deal_request(digest(60), digest(61), digest(62));
    h.deal(&request);
    let room = h.book().room(digest(60)).unwrap().unwrap();
    let ask = h.book().ask(digest(61)).unwrap().unwrap();
    let bid = h.book().bid(digest(70)).unwrap().unwrap();
    let deal = h.book().deal(digest(62)).unwrap().unwrap();
    let room_bytes = payload(ROOM_RECORD_BYTES, |out| room.encode(out));
    let ask_bytes = payload(ASK_RECORD_BYTES, |out| ask.encode(out));
    let bid_bytes = payload(BID_RECORD_BYTES, |out| bid.encode(out));
    let deal_bytes = payload(DEAL_RECORD_BYTES, |out| deal.encode(out));
    assert_eq!(RoomRecord::decode(&room_bytes).unwrap(), room);
    assert_eq!(AskRecord::decode(&ask_bytes).unwrap(), ask);
    assert_eq!(BidRecord::decode(&bid_bytes).unwrap(), bid);
    assert_eq!(DealRecord::decode(&deal_bytes).unwrap(), deal);
    let decoders: [fn(&[u8]) -> CodecResult<()>; 4] = [
        |b| RoomRecord::decode(b).map(|_| ()),
        |b| AskRecord::decode(b).map(|_| ()),
        |b| BidRecord::decode(b).map(|_| ()),
        |b| DealRecord::decode(b).map(|_| ()),
    ];
    for (bytes, decode) in [&room_bytes, &ask_bytes, &bid_bytes, &deal_bytes]
        .into_iter()
        .zip(decoders)
    {
        for len in 0..bytes.len() {
            assert!(decode(&bytes[..len]).is_err());
        }
        let mut trailing = bytes.clone();
        trailing.push(0);
        assert_eq!(decode(&trailing), Err(NON_CANONICAL));
    }
    let mut invalid = room_bytes;
    invalid[200] = 2;
    assert_eq!(RoomRecord::decode(&invalid), Err(NON_CANONICAL));
    let mut invalid = ask_bytes;
    invalid[160] = 3;
    assert_eq!(AskRecord::decode(&invalid), Err(NON_CANONICAL));
    let mut invalid = bid_bytes;
    invalid[160..162].fill(0);
    assert_eq!(BidRecord::decode(&invalid), Err(NON_CANONICAL));
    let mut invalid = deal_bytes.clone();
    invalid[145] = 2;
    assert_eq!(DealRecord::decode(&invalid), Err(NON_CANONICAL));
    let mut invalid = deal_bytes;
    invalid[146 + 72] = 1;
    assert_eq!(DealRecord::decode(&invalid), Err(NON_CANONICAL));
    let mut too_short = [0; ROOM_RECORD_BYTES - 1];
    assert_eq!(room.encode(&mut too_short), Err(CAPACITY));
}

#[test]
fn all_room_ask_and_bid_capacities_fit_together_in_the_shared_state_budget() {
    let mut h = Harness::new();
    let legacy = h
        .state()
        .section(Section::SettlementClaims)
        .unwrap()
        .to_vec();
    for i in (0..ROOM_CAPACITY).rev() {
        let request = OpenRoomRequest {
            model_digest: if i < ROOM_CAPACITY / 2 {
                digest(50)
            } else {
                digest(52)
            },
            ..open_request(digest(100 + i as u8))
        };
        h.apply(
            dispatch::OpenRoom,
            HOST,
            &payload(168, |out| request.encode(out)),
            Presence::Absent,
        );
    }
    h.refuse(
        dispatch::OpenRoom,
        HOST,
        &payload(168, |out| open_request(digest(116)).encode(out)),
        Presence::Absent,
        CAPACITY,
    );
    for i in (0..ASK_CAPACITY).rev() {
        h.ask(digest(100), digest(1 + i as u8), 1 + i as u128);
    }
    h.refuse(
        dispatch::PostAsk,
        ASKER,
        &payload(128, |out| {
            ask_request(digest(100), digest(65), 65).encode(out)
        }),
        Presence::Absent,
        CAPACITY,
    );
    for i in (0..SIDE_CAPACITY).rev() {
        h.bid(digest(100), digest(1 + i as u8), PEER_A);
    }
    assert_eq!(h.book().counts(), [16, 64, 64, 0]);
    assert!(h.book().counts()[2] < BID_CAPACITY);
    let bid = h.bid_request(digest(100), digest(65), PEER_A);
    h.refuse(
        dispatch::PostBid,
        PEER_A,
        &payload(130, |out| bid.encode(out)),
        Presence::Absent,
        CAPACITY,
    );
    for i in (SIDE_CAPACITY..BID_CAPACITY).rev() {
        h.bid(digest(108), digest(1 + i as u8), PEER_A);
    }
    for room in [digest(100), digest(108)] {
        let bid = h.bid_request(room, digest(129), PEER_A);
        h.refuse(
            dispatch::PostBid,
            PEER_A,
            &payload(130, |out| bid.encode(out)),
            Presence::Absent,
            CAPACITY,
        );
    }
    assert_eq!(h.book().counts(), [16, 64, 128, 0]);
    assert!(h.current.len() <= 196608);
    assert_eq!(h.state().room_market().unwrap().0, legacy);
    assert_eq!(
        h.book().room(digest(100)).unwrap().unwrap().room_id,
        digest(100)
    );
    assert_eq!(
        h.book().ask(digest(64)).unwrap().unwrap().ask_id,
        digest(64)
    );
    assert_eq!(
        h.book().bid(digest(128)).unwrap().unwrap().bid_id,
        digest(128)
    );
    let mut malformed = h.state().room_market().unwrap().1.to_vec();
    malformed[10..12].copy_from_slice(&129u16.to_be_bytes());
    assert!(matches!(RoomBook::decode(&malformed), Err(CAPACITY)));
}
