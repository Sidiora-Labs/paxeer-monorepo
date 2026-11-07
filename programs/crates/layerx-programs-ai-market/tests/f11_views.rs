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
            protocol_share_bps: 100,
            settlement_account: AccountId::new([80; 32]).unwrap(),
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
        if op.metadata().feature != 11 {
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
                generated_tokens: 22,
                cumulative_tokens: (first + i as u64 + 1) * 32,
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

use layerx_programs_ai_market::queries::{self, DealView, PeerView, ReceiptBatchView, RoomView};

impl Market {
    fn read_envelope(&self, operation: dispatch::Operation, payload: &[u8]) -> Vec<u8> {
        let header = producers::header(&self.bytes).unwrap();
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
            .unwrap()
            .request_digest()
            .unwrap();
        let before = self.bytes.clone();
        let reply = producers::compose(&self.bytes, &input, actor(99), AT).unwrap();
        assert!(matches!(reply.routed, Routed::Unchanged { .. }));
        assert!(reply.state.is_empty());
        assert!(reply.event.is_empty());
        assert_eq!(self.bytes, before);
        let result = codec::decode_result(&reply.result).unwrap();
        assert_eq!(result.status, codec::ResultStatus::Ok);
        assert_eq!(result.error, None);
        assert_eq!(result.request, Presence::Present(request));
        assert_eq!(
            result.revision,
            decode_shared_state(&self.bytes).unwrap().revision
        );
        result.payload.to_vec()
    }
}

#[test]
fn room_view_reads_persisted_room_and_only_open_orders() {
    let mut m = Market::new();
    let before = m.read(dispatch::RoomView, digest(60).as_bytes());
    assert_eq!(before.len(), queries::ROOM_VIEW_BYTES);
    let view = RoomView::decode(&before).unwrap();
    let state = decode_shared_state(&m.bytes).unwrap();
    let book = RoomBook::decode(state.room_market().unwrap().1).unwrap();
    assert_eq!(view.room, book.room(digest(60)).unwrap().unwrap());
    assert!(view.open_asks.iter().all(Option::is_none));
    assert!(view.open_bids.iter().all(Option::is_none));
    drop(state);
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
    let state = decode_shared_state(&m.bytes).unwrap();
    let workers = layerx_programs_ai_market::workers::WorkerTable::decode(
        layerx_programs_ai_market::evaluators::authority::split_identity_section(
            state.section(Section::IdentityRoster).unwrap(),
        )
        .unwrap()
        .0,
    )
    .unwrap();
    let metadata = workers
        .iter()
        .find(|w| w.owner == actor(20))
        .unwrap()
        .metadata;
    drop(state);
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
    let view = RoomView::decode(&bytes).unwrap();
    assert_eq!(view.open_asks[0], Some(digest(63)));
    assert!(view.open_asks[1..].iter().all(Option::is_none));
    assert_eq!(view.open_bids[0], Some(digest(72)));
    assert!(view.open_bids[1..].iter().all(Option::is_none));
    assert_eq!(view.room, RoomView::decode(&before).unwrap().room);
    assert!(RoomView::decode(&bytes[..bytes.len() - 1]).is_err());
    let mut trailing = bytes.clone();
    trailing.push(0);
    assert!(RoomView::decode(&trailing).is_err());
    let mut gap = bytes;
    gap[ROOM_RECORD_BYTES..ROOM_RECORD_BYTES + 32].fill(0);
    gap[ROOM_RECORD_BYTES + 32..ROOM_RECORD_BYTES + 64].copy_from_slice(digest(63).as_bytes());
    assert!(RoomView::decode(&gap).is_err());
}

#[test]
fn deal_and_receipt_batch_views_follow_real_sealed_settlements_after_reopen() {
    let mut m = Market::new();
    let bytes = m.read(dispatch::DealView, digest(62).as_bytes());
    assert_eq!(bytes.len(), queries::DEAL_VIEW_BYTES);
    let initial = DealView::decode(&bytes).unwrap();
    let state = decode_shared_state(&m.bytes).unwrap();
    assert_eq!(
        initial.deal,
        RoomBook::decode(state.room_market().unwrap().1)
            .unwrap()
            .deal(digest(62))
            .unwrap()
            .unwrap()
    );
    assert_eq!(initial.settled_tokens, 0);
    assert_eq!(initial.settled_amount_owed, 0);
    drop(state);
    for (first, count, voter) in [(0, 4, 2), (4, 3, 3)] {
        let batch = batch(first, count, 3);
        let manifest = m.manifest(&batch);
        m.seal(&manifest, voter);
        m.call(
            dispatch::SettleLaps,
            owner(),
            &m.settle_payload(batch, &manifest),
            AT,
        );
        let reopened = Market {
            bytes: m.bytes.clone(),
            roster: m.roster,
        };
        let bytes = reopened.read(dispatch::DealView, digest(62).as_bytes());
        let view = DealView::decode(&bytes).unwrap();
        let settled = settlement_from_state(&reopened.bytes, digest(62))
            .unwrap()
            .unwrap();
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
        let view = ReceiptBatchView::decode(&bytes).unwrap();
        assert_eq!(view.batch, settled.batch);
        assert_eq!(view.batch, batch);
        assert_eq!(view.batch.countersign_quorum, 2);
        assert!(ReceiptBatchView::decode(&bytes[..bytes.len() - 1]).is_err());
        let mut trailing = bytes;
        trailing.push(0);
        assert!(ReceiptBatchView::decode(&trailing).is_err());
    }
}

#[test]
fn peer_view_reads_enrollment_manifest_binding_and_explicit_unavailable_sources() {
    let m = Market::new();
    for key in [actor(20), actor(21)] {
        let bytes = m.read(dispatch::PeerView, key.as_bytes());
        assert_eq!(bytes.len(), queries::PEER_VIEW_BYTES);
        let view = PeerView::decode(&bytes).unwrap();
        let state = decode_shared_state(&m.bytes).unwrap();
        let workers = layerx_programs_ai_market::workers::WorkerTable::decode(
            layerx_programs_ai_market::evaluators::authority::split_identity_section(
                state.section(Section::IdentityRoster).unwrap(),
            )
            .unwrap()
            .0,
        )
        .unwrap();
        let worker = workers.iter().find(|w| w.owner == key).unwrap();
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

#[test]
fn unknown_ids_and_batch_digest_mismatch_answer_room_unknown_without_effects() {
    let m = Market::new();
    for (operation, key) in [
        (dispatch::RoomView, digest(99).bytes().to_vec()),
        (dispatch::DealView, digest(99).bytes().to_vec()),
        (dispatch::PeerView, digest(99).bytes().to_vec()),
        (
            dispatch::ReceiptBatchView,
            [digest(99).bytes(), digest(98).bytes()].concat(),
        ),
        (
            dispatch::ReceiptBatchView,
            [digest(62).bytes(), digest(98).bytes()].concat(),
        ),
    ] {
        let input = m.read_envelope(operation, &key);
        let reply = producers::compose(&m.bytes, &input, actor(99), AT).unwrap();
        assert!(matches!(reply.routed, Routed::Refused { .. }));
        assert_eq!(
            codec::decode_result(&reply.result).unwrap().error,
            Some(F11_ROOM_UNKNOWN)
        );
        assert!(reply.state.is_empty());
        assert!(reply.event.is_empty());
    }
}

#[test]
fn view_envelopes_enforce_public_read_binding_and_canonical_payloads() {
    let m = Market::new();
    let input = m.read_envelope(dispatch::RoomView, digest(60).as_bytes());
    let parsed = queries::decode_view_envelope(&input).unwrap();
    assert!(matches!(
        producers::compose(&m.bytes, &input, actor(98), AT)
            .unwrap()
            .routed,
        Routed::Refused { .. }
    ));
    assert!(matches!(
        producers::compose(&m.bytes, &input, actor(99), AT + 11)
            .unwrap()
            .routed,
        Routed::Refused { .. }
    ));
    let mut envelope = parsed.envelope;
    envelope.config = 1;
    assert_eq!(
        queries::encode_view_envelope(&envelope, &mut vec![0; MAX_ENVELOPE_BYTES]),
        Err(NON_CANONICAL)
    );
    envelope.config = 0;
    envelope.payload = &[0; 32];
    assert!(queries::encode_view_envelope(&envelope, &mut vec![0; MAX_ENVELOPE_BYTES]).is_err());
    assert!(queries::decode_view_envelope(&input[..input.len() - 1]).is_err());
    let mut trailing = input;
    trailing.push(0);
    assert!(queries::decode_view_envelope(&trailing).is_err());
    assert_eq!(
        queries::answer_room_view(&m.bytes, digest(60).as_bytes(), &mut [0; 1]),
        Err(CAPACITY)
    );
}
