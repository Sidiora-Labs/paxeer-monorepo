use layerx_ai_market_test_values::RequiredValue as _;
use layerx_programs_ai_market::placement::SIDE_CAPACITY;
#[allow(
    dead_code,
    reason = "Shared room producers expose the operations used by each independent suite."
)]
#[path = "support/f11_rooms.rs"]
mod rooms_support;
use rooms_support::*;

#[test]
fn funded_reward_state_and_all_room_book_capacities_fit_together() {
    use layerx_programs_ai_market::{bonds, rewards};

    let mut h = Harness::new();
    let funded = h.state().room_market().required().0.to_vec();
    // Funding emits the fixed dictionary, all padded epoch rows and refund cursor.
    // Occupying rows or enabling payouts changes their contents, never their size.
    assert_eq!(funded.len(), rewards::REWARD_STATE_BYTES);
    assert_eq!(
        rewards::decode_reward_state(&funded)
            .required()
            .ledger()
            .required()
            .free,
        1000
    );

    fill_rooms_and_asks(&mut h);
    fill_bid_capacity(&mut h);
    fill_deal_capacity(&mut h);
    let persisted = h.current.clone();
    let restarted = Harness {
        current: persisted,
        height: h.height,
        request: h.request,
    };
    assert_eq!(
        restarted.book().counts(),
        [ROOM_CAPACITY, ASK_CAPACITY, BID_CAPACITY, DEAL_CAPACITY]
    );
    for i in 0..BID_CAPACITY {
        assert!(
            bonds::bond_from_state(&restarted.current, digest(1 + u8::try_from(i).required()))
                .required()
                .is_some()
        );
    }
    let (prefix, book) = restarted.state().room_market().required();
    assert_eq!(prefix.len(), REWARD_STATE_BYTES);
    rewards::decode_reward_state(prefix).required();
    assert!(book.len() <= BOOK_MAX_BYTES);
    assert_eq!(
        restarted
            .state()
            .section(Section::SettlementClaims)
            .required()
            .len(),
        prefix.len() + book.len() + state::ROOM_MARKET_FRAMING_BYTES
    );
    assert!(restarted.current.len() <= MAX_STATE_BYTES);
}
use layerx_programs_ai_market::bonds;
fn fill_rooms_and_asks(h: &mut Harness) {
    for i in 0..ROOM_CAPACITY {
        let request = OpenRoomRequest {
            model_digest: if i < ROOM_CAPACITY / 2 {
                digest(50)
            } else {
                digest(52)
            },
            ..open_request(digest(100 + u8::try_from(i).required()))
        };
        h.apply(
            dispatch::OpenRoom,
            HOST,
            &payload(OPEN_ROOM_REQUEST_BYTES, |out| request.encode(out)),
            Presence::Absent,
        );
    }
    h.refuse(
        dispatch::OpenRoom,
        HOST,
        &payload(OPEN_ROOM_REQUEST_BYTES, |out| {
            open_request(digest(116)).encode(out)
        }),
        Presence::Absent,
        CAPACITY,
    );

    for i in 0..ASK_CAPACITY {
        h.ask(
            digest(100),
            digest(1 + u8::try_from(i).required()),
            1 + i as u128,
        );
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
}
fn fill_bid_capacity(h: &mut Harness) {
    for i in 0..BID_CAPACITY {
        let room = if i < SIDE_CAPACITY {
            digest(100)
        } else {
            digest(108)
        };
        let peer = if i % 2 == 0 { PEER_A } else { PEER_B };
        // Harness::bid first applies LockBond, then PostBid through dispatch::route.
        h.bid(room, digest(1 + u8::try_from(i).required()), peer);
        let bond = bonds::bond_from_state(&h.current, digest(1 + u8::try_from(i).required()))
            .required()
            .required();
        assert_eq!(bond.room, room);
        assert_eq!(bond.owner, principal(peer));
        assert_eq!(bond.remaining().required(), profile().minimum_bond);
        assert!(!bond.refunded);
        assert_eq!(
            h.state().room_market().required().0.len(),
            REWARD_STATE_BYTES
        );
        assert!(
            h.state()
                .section(Section::SettlementClaims)
                .required()
                .len()
                <= Section::SettlementClaims.payload_cap()
        );
    }
    for room in [digest(100), digest(108)] {
        let next = h.bid_request(room, digest(129), PEER_A);
        h.refuse(
            dispatch::PostBid,
            PEER_A,
            &payload(130, |out| next.encode(out)),
            Presence::Absent,
            CAPACITY,
        );
    }
    h.refuse(
        dispatch::LockBond,
        PEER_A,
        &bonds::LockBondRequest {
            room: digest(100),
            bid: digest(129),
            amount: 100,
        }
        .encode()
        .required(),
        Presence::Present(h.roster()),
        CAPACITY,
    );
}
fn fill_deal_capacity(h: &mut Harness) {
    for i in 0..DEAL_CAPACITY {
        let ask_id = digest(1 + u8::try_from(i).required());
        let deal_id = digest(150 + u8::try_from(i).required());
        let mut layers = [None; MAX_DEAL_PEERS];
        layers[0] = Some(LayerSplit {
            bid_id: digest(1 + 2 * u8::try_from(i).required()),
            first_layer: 0,
            end_layer: 8,
        });
        layers[1] = Some(LayerSplit {
            bid_id: digest(2 + 2 * u8::try_from(i).required()),
            first_layer: 8,
            end_layer: 20,
        });
        let request = RecordDealRequest {
            room_id: digest(100),
            deal_id,
            ask_id,
            agreed_price_per_thousand_tokens: 10,
            roster_digest: h.roster(),
            layers,
        };
        let bytes = payload(1297, |out| request.encode(out));
        let envelope = h.envelope(
            dispatch::RecordDeal,
            HOST,
            &bytes,
            Presence::Present(request.roster_digest),
        );
        let (routed, result, _) = h.execute(HOST, &envelope);
        assert!(
            matches!(routed, Routed::Applied { .. }),
            "deal index {i}, ask {ask_id:?}, layers {layers:?}: {:?}",
            codec::decode_result(&result).required()
        );
        assert_eq!(
            h.book().ask(ask_id).required().required().state,
            AskState::Matched
        );
        assert_eq!(h.book().deal(deal_id).required().required().layers, layers);
    }
    assert_eq!(
        h.book().counts(),
        [ROOM_CAPACITY, ASK_CAPACITY, BID_CAPACITY, DEAL_CAPACITY]
    );
    let mut overflow = h.book().deal(digest(150)).required().required();
    overflow.deal_id = digest(166);
    overflow.layers[0].as_mut().required().bid_id = digest(33);
    overflow.layers[1].as_mut().required().bid_id = digest(34);
    let next = RecordDealRequest {
        room_id: overflow.room_id,
        deal_id: overflow.deal_id,
        ask_id: digest(17),
        agreed_price_per_thousand_tokens: overflow.agreed_price_per_thousand_tokens,
        roster_digest: overflow.roster_digest,
        layers: overflow.layers,
    };
    h.refuse(
        dispatch::RecordDeal,
        HOST,
        &payload(1297, |out| next.encode(out)),
        Presence::Present(h.roster()),
        CAPACITY,
    );
}

#[test]
fn settlement_maxima_and_protocol_resource_bounds_are_explicit() {
    use layerx_programs_ai_market::{aggregation_codec, rewards};
    use layerx_programs_runtime::{storage::MAX_STORAGE_VALUE_BYTES, DeclaredBudget};

    assert_eq!(
        rewards::REWARD_STATE_BYTES,
        206 + 256 * 67 + 33 * (174 + 32 * 19) + 97
    );
    assert_eq!(
        rewards::PAYOUT_REGION_MAX_BYTES,
        9 + 64 * 99 + DEAL_CAPACITY * 104
    );
    const {
        assert!(rewards::PAYOUT_REGION_MAX_BYTES <= rewards::EPOCH_ROWS_BYTES);
    }
    assert_eq!(
        aggregation_codec::F05_MAX_BYTES,
        aggregation_codec::CURRENT_MAX_BYTES + aggregation_codec::HISTORY_MAX_BYTES
    );
    assert_eq!(BOOK_MAX_BYTES, 79_460);
    assert_eq!(SETTLEMENT_MAX_BYTES, 128_520);
    let growth = (SETTLEMENT_MAX_BYTES - 81_912).div_ceil(4096) * 4096;
    assert_eq!(growth, 49_152);
    assert_eq!(Section::SettlementClaims.payload_cap(), 81_912 + growth);
    assert_eq!(MAX_STATE_BYTES, 196_608 + growth);
    assert_eq!(
        codec::STATE_SECTION_CAPS.iter().sum::<usize>(),
        MAX_STATE_BYTES
    );
    assert!(SETTLEMENT_MAX_BYTES <= Section::SettlementClaims.payload_cap());
    const {
        assert!(MAX_STATE_BYTES <= MAX_STORAGE_VALUE_BYTES);
    }
    assert_eq!(
        layerx_programs_ai_market::reputation::COMMON_CAP,
        MAX_STATE_BYTES
    );
    let guest_buffers =
        3 * MAX_STATE_BYTES + dispatch::SCRATCH_BYTES + MAX_EVENT_BYTES + MAX_RESULT_BYTES;
    let memory = DeclaredBudget::protocol_maximum().memory_bytes();
    assert_eq!(memory, 16 * 1024 * 1024);
    assert!(guest_buffers as u64 <= memory);
    println!("settlement_max={SETTLEMENT_MAX_BYTES} state_max={MAX_STATE_BYTES} guest_buffers={guest_buffers} activity_memory_max={memory}");
}

#[test]
fn rebuilt_guest_instantiates_within_the_activity_memory_declaration() {
    use layerx_programs_runtime::{FuelSchedule, WasmEngine, ABI_V5_VERSION};
    use std::path::PathBuf;

    let path = std::env::var_os("PAXAI_HOST_BOUNDARY_MANIFEST").map_or_else(
        || {
            PathBuf::from(env!("CARGO_MANIFEST_DIR"))
                .join("../../../build/paxai-host-boundary/manifest")
        },
        PathBuf::from,
    );
    let manifest = std::fs::read_to_string(&path).required();
    let lines: Vec<_> = manifest.lines().collect();
    assert_eq!(lines.len(), 3);
    let wasm_path = PathBuf::from(lines[0]);
    let wasm_path = if wasm_path.is_absolute() {
        wasm_path
    } else {
        path.parent().required().join(wasm_path)
    };
    let wasm = std::fs::read(wasm_path).required();
    assert!(wasm.len() <= 524_184);
    let engine = WasmEngine::declared().required();
    let module = engine
        .validate_versioned_metered(ABI_V5_VERSION, &wasm, FuelSchedule::WASMI_0_31_2)
        .required();
    module.instantiate_for_qualification().required();
    println!(
        "packaged_guest_bytes={} digest={:?}",
        wasm.len(),
        codec::sha256([wasm.as_slice()]).required()
    );
}
