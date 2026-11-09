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
    SHARED_STATE_KEY,
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
    fn refuse(
        &self,
        batch: ReceiptBatch,
        manifest: &[u8],
        who: PrincipalId,
        error: ApplicationError,
    ) {
        let payload = self.settle_payload(batch, manifest);
        let e = self.envelope(dispatch::SettleLaps, who, &payload);
        let before = self.bytes.clone();
        let reply = producers::compose(&self.bytes, &e, who, AT).unwrap();
        assert!(matches!(reply.routed, Routed::Refused { .. }));
        assert_eq!(
            codec::decode_result(&reply.result).unwrap().error,
            Some(error)
        );
        assert!(reply.state.is_empty());
        assert!(reply.event.is_empty());
        assert_eq!(self.bytes, before);
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

#[test]
fn settle_after_seal_persists_meter_and_amount_and_replay_survives_restart() {
    let mut m = Market::new();
    let b = batch(0, 4, 3);
    let manifest = m.manifest(&b);
    m.seal(&manifest, 2);
    let before = m.bytes.clone();
    let payload = m.settle_payload(b, &manifest);
    let e = m.call(dispatch::SettleLaps, owner(), &payload, AT);
    let reopened = Market {
        bytes: m.bytes.clone(),
        roster: m.roster,
    };
    let settled = settlement_from_state(&reopened.bytes, digest(62))
        .unwrap()
        .unwrap();
    assert_eq!(settled.batch, b);
    assert_eq!(settled.meter_reading(), 128);
    assert_eq!(settled.settled_amount_owed, 128);
    let old = decode_shared_state(&before).unwrap();
    let new = decode_shared_state(&reopened.bytes).unwrap();
    assert_eq!(old.room_market().unwrap(), new.room_market().unwrap());
    assert_eq!(
        SealRegion::decode(old.control.feature_bytes)
            .unwrap()
            .seals(6)
            .collect::<Vec<_>>(),
        SealRegion::decode(new.control.feature_bytes)
            .unwrap()
            .seals(6)
            .collect::<Vec<_>>()
    );
    let reply = producers::compose(&reopened.bytes, &e, owner(), AT).unwrap();
    assert!(matches!(reply.routed, Routed::Unchanged { .. }));
    assert_eq!(
        codec::decode_result(&reply.result).unwrap().status,
        codec::ResultStatus::AlreadyApplied
    );
    let next = batch(4, 3, 3);
    let manifest = m.manifest(&next);
    m.seal(&manifest, 3);
    m.call(
        dispatch::SettleLaps,
        owner(),
        &m.settle_payload(next, &manifest),
        AT,
    );
    let settled = settlement_from_state(&m.bytes, digest(62))
        .unwrap()
        .unwrap();
    assert_eq!(settled.meter_reading(), 224);
    assert_eq!(settled.settled_amount_owed, 224);
}
#[test]
fn unsealed_digest_and_tampered_sealed_summary_refuse() {
    let mut m = Market::new();
    let b = batch(0, 4, 3);
    let manifest = m.manifest(&b);
    m.refuse(b, &manifest, owner(), F11_BATCH_UNSEALED);
    m.seal(&manifest, 2);
    let mut tampered = b;
    tampered.cumulative_tokens += 1000;
    m.refuse(tampered, &manifest, owner(), F11_BATCH_UNSEALED);
    let mut tampered = b;
    tampered.batch_digest = digest(99);
    m.refuse(tampered, &manifest, owner(), F11_BATCH_UNSEALED);
}
#[test]
fn regressing_lap_index_and_token_meter_refuse() {
    let mut m = Market::new();
    let b = batch(4, 4, 3);
    let manifest = m.manifest(&b);
    m.seal(&manifest, 2);
    m.call(
        dispatch::SettleLaps,
        owner(),
        &m.settle_payload(b, &manifest),
        AT,
    );
    let earlier = batch(0, 4, 3);
    let manifest = m.manifest(&earlier);
    m.seal(&manifest, 3);
    m.refuse(earlier, &manifest, owner(), CONFLICT);
    let mut lower = batch(8, 4, 3);
    lower.cumulative_tokens = 1;
    let manifest = m.manifest(&lower);
    m.seal(&manifest, 4);
    m.refuse(lower, &manifest, owner(), CONFLICT);
}
#[test]
fn short_quorum_and_non_host_refuse() {
    let mut m = Market::new();
    let short = batch(0, 4, 0);
    let manifest = m.manifest(&short);
    m.seal(&manifest, 2);
    m.refuse(short, &manifest, owner(), UNAUTHORIZED);
    let b = batch(0, 4, 1);
    let manifest = m.manifest(&b);
    m.seal(&manifest, 3);
    m.refuse(b, &manifest, actor(20), UNAUTHORIZED);
    m.call(
        dispatch::SettleLaps,
        owner(),
        &m.settle_payload(b, &manifest),
        AT,
    );
    assert_eq!(
        settlement_from_state(&m.bytes, digest(62))
            .unwrap()
            .unwrap()
            .batch
            .countersign_quorum,
        1
    );
}

#[test]
fn settle_laps_64_fits_one_million_cpu_fuel() {
    use layerx_programs_runtime::{
        self as rt, abi::UnavailableReceiptOracle, ActivityBudgetBinding, AuthorizationContext,
        AuthorizedExecutionRequest, BudgetedAuthorizedExecutionRequest, Capability, CapabilitySet,
        CompositionContext, DeclaredBudget, Executor, FuelSchedule, Storage, StorageNamespace,
        V2ActivityOutcome, WasmEngine, ABI_V5_VERSION, CALL_ENTRY_EXPORT,
    };
    let mut m = Market::new();
    let b = batch(0, LAP_BATCH_MAX, 3);
    assert_eq!(b.last_lap_index, 63);
    assert_eq!(b.cumulative_tokens, 2048);
    let manifest = m.manifest(&b);
    m.seal(&manifest, 2);
    let payload = m.settle_payload(b, &manifest);
    let envelope = m.envelope(dispatch::SettleLaps, owner(), &payload);
    let expected = producers::compose(&m.bytes, &envelope, owner(), AT).unwrap();
    assert!(matches!(expected.routed, Routed::Applied { .. }));
    let target = std::path::Path::new(env!("CARGO_TARGET_TMPDIR")).join("f11-lap-guest");
    let programs = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("../..");
    let status = std::process::Command::new(env!("CARGO"))
        .current_dir(programs)
        .env("CARGO_TARGET_DIR", &target)
        .args([
            "build",
            "--locked",
            "--release",
            "--target",
            "wasm32-unknown-unknown",
            "-p",
            "layerx-programs-ai-market",
        ])
        .status()
        .expect("build the production guest");
    assert!(status.success(), "production guest build failed");
    let wasm =
        std::fs::read(target.join("wasm32-unknown-unknown/release/layerx_programs_ai_market.wasm"))
            .unwrap();
    let module = WasmEngine::declared()
        .unwrap()
        .validate_versioned_metered(ABI_V5_VERSION, &wasm, FuelSchedule::WASMI_0_31_2)
        .unwrap();
    let program = rt::ProgramId::new([11; 32]).unwrap();
    let principal = rt::PrincipalId::new(owner().bytes()).unwrap();
    let namespace = StorageNamespace::shared(program);
    let mut storage = Storage::new();
    let mut transaction = storage.transaction(namespace);
    transaction.write(SHARED_STATE_KEY, &m.bytes).unwrap();
    transaction.commit();
    let binding = ActivityBudgetBinding::new(
        codec::decode_envelope(&envelope)
            .unwrap()
            .request_digest()
            .unwrap()
            .bytes(),
    )
    .unwrap();
    let executor = Executor::declared();
    let budget = executor
        .admit_activity_budget_for_qualification(
            DeclaredBudget::protocol_maximum(),
            principal,
            binding,
            u128::MAX,
        )
        .unwrap();
    let capabilities = CapabilitySet::new([
        Capability::SharedStorageRead,
        Capability::SharedStorageWrite,
        Capability::EmitEvent,
    ])
    .unwrap();
    let record = executor
        .execute_authorized_v5_budgeted_for_qualification(
            &mut storage,
            BudgetedAuthorizedExecutionRequest::new(
                AuthorizedExecutionRequest {
                    module: &module,
                    program,
                    authorization: AuthorizationContext::new(principal, capabilities),
                    receipts: &UnavailableReceiptOracle,
                    entrypoint: CALL_ENTRY_EXPORT,
                    calldata: &envelope,
                    composition: CompositionContext::isolated(),
                    response_capacity: MAX_RESULT_BYTES,
                },
                budget,
                principal,
                binding,
            ),
            1,
            AT,
        )
        .unwrap();
    let fuel = record.execution().usage().cpu_fuel;
    assert!(fuel < 1_000_000, "64-lap settlement consumed {fuel} fuel");
    match record.outcome() {
        V2ActivityOutcome::Success { response, effects } => {
            assert_eq!(response.code, 0);
            assert_eq!(response.bytes, expected.result);
            assert_eq!(effects.events.len(), 1);
            assert_eq!(effects.events[0].data, expected.event);
        }
        other => panic!("64-lap settlement did not succeed: {other:?}; fuel={fuel}"),
    }
    let stored = storage
        .transaction(namespace)
        .read(SHARED_STATE_KEY)
        .unwrap()
        .unwrap();
    assert_eq!(stored, expected.state);
    assert_eq!(
        settlement_from_state(&stored, digest(62))
            .unwrap()
            .unwrap()
            .meter_reading(),
        2048
    );
    eprintln!("SettleLaps laps=64 cpu_fuel={fuel}");
}
