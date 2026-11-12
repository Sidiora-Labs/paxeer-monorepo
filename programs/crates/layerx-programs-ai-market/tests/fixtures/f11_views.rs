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
            protocol_share_bps: 100,
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
                &layerx_programs_ai_market::bonds::LockBondRequest {
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
            end_layer: 8,
        });
        layers[1] = Some(LayerSplit {
            bid_id: digest(71),
            first_layer: 8,
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
                    agreed_price_per_thousand_tokens: 1001,
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
        if op.metadata().feature != 11 {
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
    let ranges = [digest(80), digest(81)];
    let key = SigningKey::from_bytes(&[12; 32]);
    let receipts: Vec<_> = (0..count)
        .map(|i| {
            let mut receipt = LapReceipt {
                deal_id: digest(62),
                lap_index: first + i as u64,
                prompt_tokens: 10,
                generated_tokens: 22,
                cumulative_tokens: (first + i as u64 + 1) * 32,
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

use layerx_programs_ai_market::queries::{self, DealView, PeerView, ReceiptBatchView, RoomView};

impl Market {
    fn read_envelope(&self, operation: dispatch::Operation, payload: &[u8]) -> Vec<u8> {
        let header = producers::header(&self.bytes).required();
        encoded(MAX_ENVELOPE_BYTES, |out| {
            queries::encode_view_envelope(
                &Envelope {
                    operation,
                    chain: header.deployment_chain_domain,
                    program: header.program_id,
                    market: header.market_id,
                    actor: actor(99),
                    epoch: 0,
                    config: 0,
                    roster: Presence::Absent,
                    sequence: 0,
                    expiry: AT + 10,
                    request: RequestId::new([99; 32])?,
                    payload,
                    authentication: Authentication::Native,
                },
                out,
            )
        })
    }
    fn read(&self, operation: dispatch::Operation, payload: &[u8]) -> Vec<u8> {
        let input = self.read_envelope(operation, payload);
        let request = queries::decode_view_envelope(&input)
            .required()
            .request_digest()
            .required();
        let before = self.bytes.clone();
        let reply = producers::compose(&self.bytes, &input, actor(99), AT).required();
        assert!(matches!(reply.routed, Routed::Unchanged { .. }));
        assert!(reply.state.is_empty());
        assert!(reply.event.is_empty());
        assert_eq!(self.bytes, before);
        let result = codec::decode_result(&reply.result).required();
        assert_eq!(result.status, codec::ResultStatus::Ok);
        assert_eq!(result.error, None);
        assert_eq!(result.request, Presence::Present(request));
        assert_eq!(
            result.revision,
            decode_shared_state(&self.bytes).required().revision
        );
        result.payload.to_vec()
    }
}

fn room_view_reads_persisted_room_and_only_open_orders() {
    let mut m = Market::new();
    let before = m.read(dispatch::RoomView, digest(60).as_bytes());
    assert_eq!(before.len(), queries::ROOM_VIEW_BYTES);
    let view = RoomView::decode(&before).required();
    let state = decode_shared_state(&m.bytes).required();
    let book = RoomBook::decode(state.room_market().required().1).required();
    assert_eq!(view.room, book.room(digest(60)).required().required());
    assert!(view.open_asks.iter().all(Option::is_none));
    assert!(view.open_bids.iter().all(Option::is_none));

    m.call(
        dispatch::PostAsk,
        actor(30),
        &encoded(128, |out| {
            PostAskRequest {
                room_id: digest(60),
                ask_id: digest(63),
                model_digest: digest(1),
                maximum_price_per_thousand_tokens: 2500,
                stream_id: 124,
            }
            .encode(out)
        }),
        AT,
    );
    let state = decode_shared_state(&m.bytes).required();
    let workers = layerx_programs_ai_market::workers::WorkerTable::decode(
        layerx_programs_ai_market::evaluators::authority::split_identity_section(
            state.section(Section::IdentityRoster).required(),
        )
        .required()
        .0,
    )
    .required();
    let metadata = workers
        .iter()
        .find(|w| w.owner == actor(20))
        .required()
        .metadata;

    m.call(
        dispatch::LockBond,
        actor(20),
        &layerx_programs_ai_market::bonds::LockBondRequest {
            room: digest(60),
            bid: digest(72),
            amount: 100,
        }
        .encode()
        .required(),
        AT,
    );
    m.call(
        dispatch::PostBid,
        actor(20),
        &encoded(130, |out| {
            PostBidRequest {
                room_id: digest(60),
                bid_id: digest(72),
                capability_digest: Digest32::new(metadata.bytes())?,
                minimum_price_per_thousand_tokens: 6,
                bond_amount: 100,
                layer_capacity: 16,
            }
            .encode(out)
        }),
        AT,
    );
    let bytes = m.read(dispatch::RoomView, digest(60).as_bytes());
    assert_eq!(bytes.len(), before.len());
    let view = RoomView::decode(&bytes).required();
    assert_eq!(view.open_asks[0], Some(digest(63)));
    assert!(view.open_asks[1..].iter().all(Option::is_none));
    assert_eq!(view.open_bids[0], Some(digest(72)));
    assert!(view.open_bids[1..].iter().all(Option::is_none));
    assert_eq!(view.room, RoomView::decode(&before).required().room);
    assert!(RoomView::decode(&bytes[..bytes.len() - 1]).is_err());
    let mut trailing = bytes.clone();
    trailing.push(0);
    assert!(RoomView::decode(&trailing).is_err());
    let mut gap = bytes;
    gap[ROOM_RECORD_BYTES..ROOM_RECORD_BYTES + 32].fill(0);
    gap[ROOM_RECORD_BYTES + 32..ROOM_RECORD_BYTES + 64].copy_from_slice(digest(63).as_bytes());
    assert!(RoomView::decode(&gap).is_err());
}

fn deal_and_receipt_batch_views_follow_real_sealed_settlements_after_reopen() {
    let mut m = Market::new();
    let bytes = m.read(dispatch::DealView, digest(62).as_bytes());
    assert_eq!(bytes.len(), queries::DEAL_VIEW_BYTES);
    let initial = DealView::decode(&bytes).required();
    let state = decode_shared_state(&m.bytes).required();
    assert_eq!(
        initial.deal,
        RoomBook::decode(state.room_market().required().1)
            .required()
            .deal(digest(62))
            .required()
            .required()
    );
    assert_eq!(initial.settled_tokens, 0);
    assert_eq!(initial.settled_amount_owed, 0);

    for (first, count, voter) in [(0, 4, 2), (4, 3, 3)] {
        let batch = batch(first, count, 3);
        let manifest = m.manifest(&batch);
        m.seal(&manifest, voter);
        m.call(
            dispatch::SettleLaps,
            owner(),
            &Market::settle_payload(batch, &manifest),
            AT,
        );
        let reopened = Market {
            bytes: m.bytes.clone(),
            roster: m.roster,
        };
        let bytes = reopened.read(dispatch::DealView, digest(62).as_bytes());
        let view = DealView::decode(&bytes).required();
        let settled = settlement_from_state(&reopened.bytes, digest(62))
            .required()
            .required();
        assert_eq!(view.deal, initial.deal);
        assert_eq!(view.settled_tokens, settled.meter_reading());
        assert_eq!(view.settled_amount_owed, settled.settled_amount_owed);
        assert_eq!(view.settled_tokens, (first + count as u64) * 32);
        assert!(view.settled_amount_owed > 0);
        assert!(DealView::decode(&bytes[..bytes.len() - 1]).is_err());
        let mut key = digest(62).bytes().to_vec();
        key.extend_from_slice(batch.batch_digest.as_bytes());
        let bytes = reopened.read(dispatch::ReceiptBatchView, &key);
        assert_eq!(bytes.len(), queries::RECEIPT_BATCH_VIEW_BYTES);
        let view = ReceiptBatchView::decode(&bytes).required();
        assert_eq!(view.batch, settled.batch);
        assert_eq!(view.batch, batch);
        assert_eq!(view.batch.countersign_quorum, 2);
        assert!(ReceiptBatchView::decode(&bytes[..bytes.len() - 1]).is_err());
        let mut trailing = bytes;
        trailing.push(0);
        assert!(ReceiptBatchView::decode(&trailing).is_err());
    }
}

fn peer_view_reads_enrollment_manifest_binding_and_explicit_unavailable_sources() {
    let m = Market::new();
    for key in [actor(20), actor(21)] {
        let bytes = m.read(dispatch::PeerView, key.as_bytes());
        assert_eq!(bytes.len(), queries::PEER_VIEW_BYTES);
        let view = PeerView::decode(&bytes).required();
        let state = decode_shared_state(&m.bytes).required();
        let workers = layerx_programs_ai_market::workers::WorkerTable::decode(
            layerx_programs_ai_market::evaluators::authority::split_identity_section(
                state.section(Section::IdentityRoster).required(),
            )
            .required()
            .0,
        )
        .required();
        let worker = workers.iter().find(|w| w.owner == key).required();
        assert_eq!(view.peer_key, key);
        assert_eq!(view.worker, worker.worker);
        assert_eq!(view.delegate, worker.delegate);
        assert_eq!(view.metadata, worker.metadata);
        assert_eq!(view.generation, worker.generation);
        assert_eq!(view.key_version, worker.key_version);
        assert_eq!(view.metadata_revision, worker.metadata_revision);
        assert_eq!(view.valid_from, worker.valid_from);
        assert_eq!(view.expiry, worker.expiry);
        assert_eq!(view.state, worker.state);
        assert!(view.enrolled);
        assert_eq!(view.capabilities, None);
        assert_eq!(view.bond_status, queries::Availability::NotEnabled);
        assert_eq!(view.fault_count, 0);
        assert!(PeerView::decode(&bytes[..bytes.len() - 1]).is_err());
        let mut trailing = bytes;
        trailing.push(0);
        assert!(PeerView::decode(&trailing).is_err());
    }
}

pub fn recorded_journey() {
    room_view_reads_persisted_room_and_only_open_orders();
    deal_and_receipt_batch_views_follow_real_sealed_settlements_after_reopen();
    peer_view_reads_enrollment_manifest_binding_and_explicit_unavailable_sources();
}
