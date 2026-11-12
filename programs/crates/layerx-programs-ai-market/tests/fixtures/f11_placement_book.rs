use ed25519_dalek::{Signer, SigningKey};
use layerx_ai_market_test_values::RequiredValue as _;
use layerx_programs_ai_market::{
    admission::{
        Admission, AdmissionContext, AdmissionTable, ApprovalTerms, Participant, TABLE_MAX_BYTES,
    },
    codec::{self, Envelope, ResultStatus, Writer},
    dispatch::{self, Buffers, CallBoundary, Operation, Routed, SequencePolicy},
    errors::*,
    evaluators::authority::split_identity_section,
    placement::{self, BookView},
    policy::{PolicyCommitments, RoomProfile, TaskPolicyV1, TASK_POLICY_BYTES},
    registry::{derive_rewards_account, MarketHeader},
    registry_ops::{CallContext, PolicySection},
    reputation_transition::split_joint,
    rewards::{FundRequest, FUNDING_POLICY_VERSION},
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
    Digest32::new([n; 32]).required()
}
fn principal(n: u8) -> PrincipalId {
    PrincipalId::new([n; 32]).required()
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
        settlement_account: AccountId::new([80; 32]).required(),
        max_generated_tokens: 4096,
        queue_depth: layerx_programs_ai_market::policy::DEFAULT_QUEUE_DEPTH,
        liveness_timeout_ms: 30_000,
        acknowledgement_window_ms: 5_000,
        canary_relative_tolerance_bps: 100,
    }
}
fn payload(size: usize, encode: impl FnOnce(&mut [u8]) -> CodecResult<usize>) -> Vec<u8> {
    let mut out = vec![0; size];
    let n = encode(&mut out).required();
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
            chain: ChainDomain::new(MERGED_CHAIN_DOMAIN).required(),
            program: ProgramId::new([11; 32]).required(),
            principal: principal(actor),
            height: self.height,
        }
    }
    fn state(&self) -> state::SharedState<'_> {
        state::decode_shared_state(&self.current).required()
    }
    fn header(&self) -> MarketHeader {
        PolicySection::decode(self.state().section(Section::PolicyLifecycle).required())
            .required()
            .header
    }
    fn workers(&self) -> WorkerTable {
        WorkerTable::decode(
            split_identity_section(self.state().section(Section::IdentityRoster).required())
                .required()
                .0,
        )
        .required()
    }
    fn worker(&self, peer: u8) -> WorkerCurrent {
        *self
            .workers()
            .iter()
            .find(|w| w.owner == principal(peer))
            .required()
    }
    fn admission(&self) -> AdmissionTable {
        AdmissionTable::decode(
            split_joint(
                self.state()
                    .section(Section::ReputationAdmission)
                    .required(),
            )
            .required()
            .1,
        )
        .required()
    }
    fn roster(&self) -> RosterDigest {
        RosterDigest::new(
            roster::roster_digest(
                &self.admission(),
                self.admission().current_epoch().required(),
            )
            .required()
            .bytes(),
        )
        .required()
    }
    fn book(&self) -> RoomBook<'_> {
        RoomBook::decode(self.state().room_market().required().1).required()
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
                    .actor(ActorSlot::from_index(i).required())
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
        let read = op.metadata().boundary == CallBoundary::ProgramRead;
        let roster = if read {
            Presence::Absent
        } else if op.metadata().feature == 11 && roster == Presence::Absent {
            Presence::Present(self.roster())
        } else {
            roster
        };
        payload(MAX_ENVELOPE_BYTES, |out| {
            let encode = if op == dispatch::BookView {
                placement::encode_book_envelope
            } else {
                codec::encode_envelope
            };
            encode(
                &Envelope {
                    operation: op,
                    chain: ctx.chain,
                    program: ctx.program,
                    market: codec::derive_market(ctx.chain, ctx.program)?,
                    actor: principal(actor),
                    epoch: if read {
                        0
                    } else {
                        (self.height - ORIGIN) / 128
                    },
                    config: u64::from(!read),
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
        .required();
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
                let framed = codec::decode_result(&result[..result_len]).required();
                assert_eq!(framed.status, ResultStatus::Ok);
                assert_eq!(framed.revision, revision);
                let mut topic = [0; 64];
                let n = codec::event_topic(operation, &mut topic).required();
                let (named, common, suffix) =
                    codec::decode_event_frame(&topic[..n], &event[..event_len]).required();
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
            let room = r.digest32().required();
            let bid = r.digest32().required();
            r.take(48).required();
            let amount = r.u128().required();
            if layerx_programs_ai_market::bonds::bond_from_state(&self.current, bid)
                .required()
                .is_none()
            {
                self.apply(
                    dispatch::LockBond,
                    actor,
                    &layerx_programs_ai_market::bonds::LockBondRequest { room, bid, amount }
                        .encode()
                        .required(),
                    roster,
                );
            }
        }
        let envelope = self.envelope(op, actor, bytes, roster);
        let (routed, result, _) = self.execute(actor, &envelope);
        assert!(
            matches!(routed, Routed::Applied { .. }),
            "{op:?}: {:?}",
            codec::decode_result(&result).required()
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
        assert_eq!(codec::decode_result(&result).required().error, Some(error));
        assert_eq!(self.current, before);
    }
    fn install_admission(&mut self, table: &AdmissionTable) {
        let mut bytes = vec![0; TABLE_MAX_BYTES];
        let n = table.encode(&mut bytes).required();
        let mut out = vec![0; MAX_STATE_BYTES];
        let mut control = vec![0; Section::Control.payload_cap()];
        let state = self.state();
        let next = state
            .replace_section(Section::ReputationAdmission, &bytes[..n])
            .required();
        let n = state::encode_shared_state(&next, &mut out, &mut control).required();
        self.current = out[..n].to_vec();
    }
    fn new() -> Self {
        let mut h = Self {
            current: Vec::new(),
            height: ORIGIN,
            request: 0,
        };
        h.create_market();
        h.enroll_peers();
        h.refresh_metadata();
        h.activate_workers();
        h.height = ORIGIN + 129;
        h
    }
    fn create_market(&mut self) {
        let h = self;
        let ctx = h.context(HOST);
        let policy = TaskPolicyV1::bounded_default(
            1,
            1,
            PolicyCommitments {
                model_artifact: digest(50),
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
        let mut create = principal(HOST).as_bytes().to_vec();
        create.extend_from_slice(&[13; 32]);
        create.extend_from_slice(
            derive_rewards_account(ctx.program, AssetId::new([13; 32]).required())
                .required()
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
    }

    fn enroll_peers(&mut self) {
        let h = self;
        for peer in [PEER_A, PEER_B] {
            let key = SigningKey::from_bytes(&[peer; 32]);
            let public = PublicKey32(key.verifying_key().to_bytes());
            let nonce = [peer + 40; 32];
            let market = h.header();
            let worker = codec::derive_worker(market.market_id, principal(peer), nonce).required();
            let initial_manifest =
                manifest(market.market_id, worker, peer, 1, ORIGIN, ORIGIN + 128);
            let metadata = workers::decode_manifest(&initial_manifest)
                .required()
                .digest;
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
            .required();
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
    }

    fn refresh_metadata(&mut self) {
        let h = self;
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
            let metadata = workers::decode_manifest(&manifest).required().digest;
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
            let mut envelope = codec::decode_envelope(&native).required().envelope;
            envelope.authentication = Authentication::Delegate {
                key: PublicKey32(key.verifying_key().to_bytes()),
                signature: Signature64([0; 64]),
            };
            let unsigned = payload(MAX_ENVELOPE_BYTES, |out| {
                codec::encode_envelope(&envelope, out)
            });
            let hash = codec::decode_envelope(&unsigned)
                .required()
                .request_digest()
                .required();
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
                codec::decode_result(&result).required()
            );
        }
    }

    fn activate_workers(&mut self) {
        let h = self;
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
                identity_commitment: worker.proposal_digest(&market).required(),
                effective_epoch: 0,
                config_version: 1,
                request: RequestId::new([peer; 32]).required(),
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
                .required();
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
                .required();
        }
        roster::open_epoch(&mut table, 1, |_| false).required();
        h.install_admission(&table);
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
            capability_digest: Digest32::new(self.worker(peer).metadata.bytes()).required(),
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

impl Harness {
    fn priced_ask(&mut self, room: u8, id: u8, price: u128, stream: u128) -> Vec<u8> {
        let mut request = ask_request(digest(room), digest(id), stream);
        request.model_digest = self
            .book()
            .room(digest(room))
            .required()
            .required()
            .model_digest;
        request.maximum_price_per_thousand_tokens = price;
        self.apply(
            dispatch::PostAsk,
            ASKER,
            &payload(128, |out| request.encode(out)),
            Presence::Absent,
        )
    }

    fn priced_bid(&mut self, room: u8, id: u8, price: u128) -> Vec<u8> {
        let mut request = self.bid_request(digest(room), digest(id), PEER_A);
        request.minimum_price_per_thousand_tokens = price;
        self.apply(
            dispatch::PostBid,
            PEER_A,
            &payload(130, |out| request.encode(out)),
            Presence::Absent,
        )
    }

    fn view(&mut self, model: u8) -> Vec<u8> {
        let before = self.current.clone();
        let revision = self.state().revision;
        let envelope = self.envelope(
            dispatch::BookView,
            ASKER,
            digest(model).as_bytes(),
            Presence::Absent,
        );
        let (routed, result, event) = self.execute(ASKER, &envelope);
        assert!(matches!(routed, Routed::Unchanged { .. }));
        let framed = codec::decode_result(&result).required();
        assert_eq!(framed.status, ResultStatus::Ok);
        assert_eq!(framed.revision, revision);
        assert!(event.is_empty());
        assert_eq!(self.current, before);
        let view = BookView::decode(framed.payload).required();
        assert_eq!(view.model_digest, digest(model));
        assert!(result.len() <= MAX_RESULT_BYTES);
        framed.payload.to_vec()
    }
}

pub fn recorded_journey() {
    let mut h = Harness::new();
    h.open(digest(60));
    h.view(50);
}
