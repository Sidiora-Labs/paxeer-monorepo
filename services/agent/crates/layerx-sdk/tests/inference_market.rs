use layerx_sdk::inference_market::{self as sdk, program, *};
use program::{
    codec::{self, Envelope},
    dispatch::{self, Operation, Routed},
    errors::CodecResult,
    evidence::{self, ArtifactContext, ArtifactKind, ArtifactManifest, Items, Privacy},
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    registry::derive_rewards_account,
    registry_ops::CallContext,
    rooms::{RoomBook, RoomState},
    state::decode_shared_state,
    types::*,
};
use serde_json::json;

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}

fn digest(value: u8) -> Digest32 {
    must(Digest32::new([value; 32]))
}

fn principal(value: u8) -> PrincipalId {
    must(PrincipalId::new([value; 32]))
}

fn encoded(capacity: usize, write: impl FnOnce(&mut [u8]) -> CodecResult<usize>) -> Vec<u8> {
    let mut bytes = vec![0; capacity];
    let length = must(write(&mut bytes));
    bytes.truncate(length);
    bytes
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
        settlement_account: must(AccountId::new([80; 32])),
        max_generated_tokens: 4096,
        queue_depth: program::policy::DEFAULT_QUEUE_DEPTH,
        liveness_timeout_ms: 30_000,
        acknowledgement_window_ms: 5_000,
        canary_relative_tolerance_bps: 100,
    }
}

fn room_request() -> OpenRoomRequest {
    OpenRoomRequest {
        room_id: digest(60),
        model_digest: digest(50),
        runtime_digest: digest(51),
        profile: profile(),
    }
}

fn ask_request() -> PostAskRequest {
    PostAskRequest {
        room_id: digest(60),
        ask_id: digest(61),
        model_digest: digest(50),
        maximum_price_per_thousand_tokens: 20,
        stream_id: 123,
    }
}

fn assert_fixture(operation: MarketOperation, producer: Operation, expected: &[u8]) {
    assert_eq!(operation.operation(), producer);
    assert_eq!(operation.payload(), expected);
    let request = operation.request(OperationContext {
        actor: principal(12),
        roster: None,
        sequence: 7,
        expiry: 2000,
        request: must(RequestId::new([9; 32])),
        required_rank: 4,
    });
    assert_eq!(request.operation, producer);
    assert_eq!(request.payload, expected);
    assert_eq!(request.actor, principal(12));
    assert_eq!(request.sequence, 7);
    assert_eq!(request.expiry, 2000);
    assert_eq!(request.request.bytes(), [9; 32]);
    assert_eq!(request.required_rank, 4);
}

#[test]
fn room_and_deal_encoders_reproduce_program_fixtures() {
    let open = room_request();
    assert_fixture(
        must(open_room(&open)),
        dispatch::OpenRoom,
        &encoded(program::rooms::OPEN_ROOM_REQUEST_BYTES, |out| {
            open.encode(out)
        }),
    );
    let ask = ask_request();
    assert_fixture(
        must(post_ask(&ask)),
        dispatch::PostAsk,
        &encoded(128, |out| ask.encode(out)),
    );
    let bid = PostBidRequest {
        room_id: digest(60),
        bid_id: digest(70),
        capability_digest: digest(20),
        minimum_price_per_thousand_tokens: 5,
        bond_amount: 100,
        layer_capacity: 16,
    };
    assert_fixture(
        must(post_bid(&bid)),
        dispatch::PostBid,
        &encoded(130, |out| bid.encode(out)),
    );
    let mut layers = [None; program::rooms::MAX_DEAL_PEERS];
    layers[0] = Some(LayerSplit {
        bid_id: digest(70),
        first_layer: 0,
        end_layer: 16,
    });
    layers[1] = Some(LayerSplit {
        bid_id: digest(71),
        first_layer: 16,
        end_layer: 32,
    });
    let deal = RecordDealRequest {
        room_id: digest(60),
        deal_id: digest(62),
        ask_id: digest(61),
        agreed_price_per_thousand_tokens: 10,
        roster_digest: must(RosterDigest::new([40; 32])),
        layers,
    };
    assert_fixture(
        must(record_deal(&deal)),
        dispatch::RecordDeal,
        &encoded(1297, |out| deal.encode(out)),
    );
    assert_fixture(
        must(close_room(digest(60))),
        dispatch::CloseRoom,
        digest(60).as_bytes(),
    );
    let mut invalid_ask = ask;
    invalid_ask.maximum_price_per_thousand_tokens = 0;
    assert!(post_ask(&invalid_ask).is_err());
    let mut invalid_bid = bid;
    invalid_bid.layer_capacity = 0;
    assert!(post_bid(&invalid_bid).is_err());
    let mut invalid_deal = deal;
    invalid_deal.layers[1] = Some(LayerSplit {
        bid_id: digest(71),
        first_layer: 17,
        end_layer: 32,
    });
    assert!(record_deal(&invalid_deal).is_err());
    let mut invalid_room = open;
    invalid_room.profile.host_share_bps = 10000;
    assert!(open_room(&invalid_room).is_err());
}

fn receipt_batch() -> ReceiptBatch {
    use ed25519_dalek::{Signer as _, SigningKey};
    let ranges = [digest(80), digest(81)];
    let key = SigningKey::from_bytes(&[12; 32]);
    let receipts: Vec<_> = (0..4)
        .map(|index| {
            let mut receipt = LapReceipt {
                deal_id: digest(62),
                lap_index: index,
                prompt_tokens: 10,
                generated_tokens: 22,
                cumulative_tokens: (index + 1) * 32,
                layer_range_digests: &ranges,
                host_signature: Signature64([1; 64]),
                peer_countersignatures: 3,
            };
            receipt.host_signature = Signature64(
                key.sign(must(receipt.signing_digest()).as_bytes())
                    .to_bytes(),
            );
            receipt
        })
        .collect();
    must(ReceiptBatch::from_receipts(&receipts))
}

#[test]
fn lap_and_challenge_encoders_reproduce_program_fixtures() {
    let batch = receipt_batch();
    let mut manifest = vec![0; evidence::MAX_MANIFEST_BYTES];
    let length = must(evidence::encode_manifest(
        &ArtifactManifest {
            kind: ArtifactKind::EvaluationEvidence,
            privacy: Privacy::Public,
            context: ArtifactContext {
                chain: must(ChainDomain::new(program::MERGED_CHAIN_DOMAIN)),
                program: must(ProgramId::new([11; 32])),
                market: must(MarketId::new([15; 32])),
                policy: must(PolicyDigest::new([16; 32])),
            },
            epoch: 6,
            publisher: principal(12),
            subject: [17; 32],
            byte_length: 1,
            chunk_count: 1,
            content_root: evidence::content_root(1, 1, &batch.batch_digest.bytes()),
            parents: Items::Typed(&[]),
            declaration_root: [0; 32],
            reproduction_root: [0; 32],
            access_policy_root: [0; 32],
            not_after_height: u64::MAX,
        },
        &mut manifest,
    ));
    manifest.truncate(length);
    let settle = SettleLapsRequest {
        batch,
        artifact_manifest: &manifest,
    };
    assert_fixture(
        must(settle_laps(&settle)),
        dispatch::SettleLaps,
        &encoded(dispatch::SettleLaps.metadata().payload_max, |out| {
            settle.encode(out)
        }),
    );
    let challenge = ChallengeLapPayload {
        deal: digest(62),
        lap: 2,
        commitment: digest(90),
        deposit: 100,
    };
    assert_fixture(
        must(challenge_lap(&challenge)),
        dispatch::ChallengeLap,
        &must(challenge.encode()),
    );
    let resolve = ResolveChallengePayload {
        deal: digest(62),
        lap: 2,
        commitment: digest(90),
        answer: [-65536, 0, 1, 2, 3, 4, 5, 65536],
    };
    assert_fixture(
        must(resolve_challenge(&resolve)),
        dispatch::ResolveChallenge,
        &must(resolve.encode()),
    );
    let encoded = must(batch.encode());
    assert_eq!(must(decode_receipt_batch_view(&encoded)).batch, batch);
    assert!(decode_receipt_batch_view(&encoded[..encoded.len() - 1]).is_err());
    let mut trailing = encoded.to_vec();
    trailing.push(0);
    assert!(decode_receipt_batch_view(&trailing).is_err());
    let oversized = vec![1; evidence::MAX_MANIFEST_BYTES + 1];
    assert!(settle_laps(&SettleLapsRequest {
        batch,
        artifact_manifest: &oversized
    })
    .is_err());
}

#[test]
fn fixed_payload_encoders_preserve_the_published_operation_widths() {
    for (operation, producer, bytes) in [
        (
            must(split_payout([8; 80])),
            dispatch::SplitPayout,
            vec![8; 80],
        ),
        (must(lock_bond([9; 80])), dispatch::LockBond, vec![9; 80]),
        (
            must(slash_bond([10; 104])),
            dispatch::SlashBond,
            vec![10; 104],
        ),
        (
            must(refund_bond([11; 64])),
            dispatch::RefundBond,
            vec![11; 64],
        ),
    ] {
        assert_eq!(bytes.len(), producer.metadata().payload_min);
        assert_eq!(bytes.len(), producer.metadata().payload_max);
        assert_fixture(operation, producer, &bytes);
    }
}

struct Producer {
    state: Vec<u8>,
    request: u8,
}

impl Producer {
    fn new() -> Self {
        let mut producer = Self {
            state: Vec::new(),
            request: 0,
        };
        let policy = must(TaskPolicyV1::bounded_default(
            1,
            1,
            PolicyCommitments {
                model_artifact: digest(50),
                dataset_artifact: [2; 32],
                benchmark_suite: digest(3),
                rubric: must(RubricDigest::new([4; 32])),
                task_schema: digest(5),
                result_schema: digest(6),
                service_terms: digest(7),
            },
            1000,
            1,
        ));
        let program = must(ProgramId::new([11; 32]));
        let mut create = principal(12).bytes().to_vec();
        create.extend_from_slice(&[13; 32]);
        create.extend_from_slice(
            must(derive_rewards_account(
                program,
                must(AssetId::new([13; 32])),
            ))
            .as_bytes(),
        );
        create.extend_from_slice(&[14; 32]);
        create.push(0);
        create.extend_from_slice(&encoded(TASK_POLICY_BYTES, |out| policy.encode(out)));
        create.extend_from_slice(&[16; 32]);
        producer.apply(dispatch::CREATE, &create, 1);
        producer
    }

    fn apply(&mut self, operation: Operation, payload: &[u8], sequence: u64) {
        self.request += 1;
        let chain = must(ChainDomain::new(program::MERGED_CHAIN_DOMAIN));
        let program = must(ProgramId::new([11; 32]));
        let envelope = encoded(program::MAX_ENVELOPE_BYTES, |out| {
            codec::encode_envelope(
                &Envelope {
                    operation,
                    chain,
                    program,
                    market: codec::derive_market(chain, program)?,
                    actor: principal(12),
                    epoch: 0,
                    config: 1,
                    roster: if operation.metadata().feature == 11 {
                        Presence::Present(RosterDigest::new([40; 32])?)
                    } else {
                        Presence::Absent
                    },
                    sequence,
                    expiry: 2000,
                    request: RequestId::new([self.request; 32])?,
                    payload,
                    authentication: Authentication::Native,
                },
                out,
            )
        });
        let mut next = vec![0; program::MAX_STATE_BYTES];
        let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
        let mut event = vec![0; program::MAX_EVENT_BYTES];
        let mut result = vec![0; program::MAX_RESULT_BYTES];
        let routed = must(dispatch::route(
            &CallContext {
                chain,
                program,
                principal: principal(12),
                height: 1000,
            },
            &envelope,
            (!self.state.is_empty()).then_some(self.state.as_slice()),
            dispatch::Buffers {
                next: &mut next,
                scratch: &mut scratch,
                event: &mut event,
                result: &mut result,
            },
        ));
        let Routed::Applied { state_len, .. } = routed else {
            panic!("producer refused: {routed:?}");
        };
        next.truncate(state_len);
        self.state = next;
    }
}

#[test]
fn view_decoders_consume_real_program_answers_after_state_reopen() {
    let mut producer = Producer::new();
    let open = must(open_room(&room_request()));
    producer.apply(open.operation(), open.payload(), 2);
    let ask = must(post_ask(&ask_request()));
    producer.apply(ask.operation(), ask.payload(), 0);
    let reopened = Producer {
        state: producer.state.clone(),
        request: producer.request,
    };
    let bytes = encoded(program::queries::ROOM_VIEW_BYTES, |out| {
        program::queries::answer_room_view(&reopened.state, digest(60).as_bytes(), out)
    });
    let view = must(decode_room_view(&bytes));
    let shared = must(decode_shared_state(&reopened.state));
    let book = must(RoomBook::decode(must(shared.room_market()).1));
    assert_eq!(
        view.room,
        must(book.room(digest(60))).unwrap_or_else(|| panic!("room"))
    );
    assert_eq!(view.room.state, RoomState::Open);
    assert_eq!(view.open_asks[0], Some(digest(61)));
    assert!(view.open_asks[1..].iter().all(Option::is_none));
    assert!(view.open_bids.iter().all(Option::is_none));
    assert!(decode_room_view(&bytes[..bytes.len() - 1]).is_err());
    let mut trailing = bytes.clone();
    trailing.push(0);
    assert!(decode_room_view(&trailing).is_err());
    let placement = encoded(program::placement::BOOK_VIEW_MAX_BYTES, |out| {
        program::queries::answer_book_view(&reopened.state, digest(50).as_bytes(), out)
    });
    let placement = must(decode_book_view(&placement));
    assert_eq!(placement.model_digest, digest(50));
    assert_eq!(placement.counts(), [1, 0]);
    let row = must(placement.ask(0));
    assert_eq!(row.order_id, digest(61));
    assert!(decode_deal_view(&[0; program::queries::DEAL_VIEW_BYTES]).is_err());
    assert!(decode_peer_view(&[0; program::queries::PEER_VIEW_BYTES]).is_err());
    assert!(decode_book_view(&[]).is_err());
}

#[test]
fn px_reads_name_the_node_root_methods_and_canonical_parameters() {
    let node = include_str!("../../../../../rpc/px_inference.go");
    let rooms = InferenceMarket::get_inference_rooms();
    assert_eq!(rooms.method(), "px_getInferenceRooms");
    assert_eq!(rooms.params(), &json!([]));
    for (read, name) in [
        (rooms, "px_getInferenceRooms"),
        (
            InferenceMarket::get_inference_room(digest(60)),
            "px_getInferenceRoom",
        ),
        (
            InferenceMarket::get_inference_deal(digest(60)),
            "px_getInferenceDeal",
        ),
        (
            InferenceMarket::get_inference_peer(principal(60)),
            "px_getInferencePeer",
        ),
        (
            InferenceMarket::get_inference_receipts(digest(60)),
            "px_getInferenceReceipts",
        ),
    ] {
        assert_eq!(read.method(), name);
        assert!(
            node.contains(&format!("a.observe(ctx, \"{name}\"")),
            "producer exports {name}"
        );
        if name != "px_getInferenceRooms" {
            assert_eq!(read.params(), &json!(["3c".repeat(32)]));
        }
    }
    assert!(Digest32::new([0; 32]).is_err());
    assert!(PrincipalId::new([0; 32]).is_err());
}

#[test]
fn ask_funding_encodes_the_native_precompile_abi() {
    let request = AskFunding {
        ask: digest(0xa5),
        settlement_account: [0x12; 20],
        rate: 3,
        rate_unit: 2,
        total_cap: 100,
        host_authority_key: [0x34; 32],
    };
    let abi: serde_json::Value = must(serde_json::from_str(include_str!(
        "../../../../../precompiles/layerxpay/abi.json"
    )));
    let method = abi
        .as_array()
        .unwrap_or_else(|| panic!("ABI array"))
        .iter()
        .find(|row| row["name"] == "streamOpenForAsk")
        .unwrap_or_else(|| panic!("producer method"));
    let types: Vec<_> = method["inputs"]
        .as_array()
        .unwrap_or_else(|| panic!("ABI inputs"))
        .iter()
        .map(|input| input["type"].as_str().unwrap_or_else(|| panic!("ABI type")))
        .collect();
    let signature = format!("streamOpenForAsk({})", types.join(","));
    let data = must(sdk::fund_ask(&request));
    assert_eq!(data.len(), 4 + 6 * 32);
    assert_eq!(
        data[..4],
        layerx_crypto::disclosure::call::ledger_selector(&signature)
    );
    assert_eq!(&data[4..36], request.ask.as_bytes());
    assert_eq!(&data[36..48], &[0; 12]);
    assert_eq!(&data[48..68], &request.settlement_account);
    assert_eq!(&data[68..84], &[0; 16]);
    assert_eq!(&data[84..100], &3_u128.to_be_bytes());
    assert_eq!(&data[100..131], &[0; 31]);
    assert_eq!(data[131], 2);
    assert_eq!(&data[132..148], &[0; 16]);
    assert_eq!(&data[148..164], &100_u128.to_be_bytes());
    assert_eq!(&data[164..196], &request.host_authority_key);
    for invalid in [
        AskFunding { rate: 0, ..request },
        AskFunding {
            rate_unit: 0,
            ..request
        },
        AskFunding {
            total_cap: 0,
            ..request
        },
        AskFunding {
            settlement_account: [0; 20],
            ..request
        },
        AskFunding {
            host_authority_key: [0; 32],
            ..request
        },
    ] {
        assert!(fund_ask(&invalid).is_err());
    }
}
