type DecodeCheck = fn(&[u8]) -> CodecResult<()>;
use layerx_ai_market_test_values::RequiredValue as _;
use layerx_programs_ai_market::placement::SIDE_CAPACITY;
#[allow(
    dead_code,
    reason = "Shared room producers expose the operations used by each independent suite."
)]
#[path = "support/f11_rooms.rs"]
mod rooms_support;
use rooms_support::*;

impl Harness {
    fn further_deal_request(&self, deal: Digest32) -> RecordDealRequest {
        let mut request = self.deal_request(digest(60), digest(61), deal);
        request.layers[0] = Some(LayerSplit {
            bid_id: digest(72),
            first_layer: 0,
            end_layer: 12,
        });
        request.layers[1] = Some(LayerSplit {
            bid_id: digest(73),
            first_layer: 12,
            end_layer: 20,
        });
        request
    }
}

#[test]
fn a_matched_ask_accepts_a_further_deal_of_the_room_host() {
    use layerx_programs_ai_market::queries::{self, DealView};

    let mut h = Harness::ready(123);
    let first = h.deal_request(digest(60), digest(61), digest(62));
    h.deal(&first);
    let earlier = h.book().deal(first.deal_id).required().required();
    let ask = h.book().ask(first.ask_id).required().required();
    assert_eq!(ask.state, AskState::Matched);
    h.bid(digest(60), digest(72), PEER_A);
    h.bid(digest(60), digest(73), PEER_B);
    let further = h.further_deal_request(digest(63));
    assert_ne!(further.layers, first.layers);
    let sequence = h.sequence(HOST);
    let encoded = h.deal(&further);
    assert_eq!(h.sequence(HOST), sequence + 1);
    let persisted = h.current.clone();
    let mut restarted = Harness {
        current: persisted.clone(),
        height: h.height,
        request: h.request,
    };
    assert_eq!(restarted.book().counts(), [1, 1, 4, 2]);
    assert_eq!(
        restarted.book().ask(first.ask_id).required().required(),
        ask
    );
    assert_eq!(
        restarted.book().deal(first.deal_id).required().required(),
        earlier
    );
    for request in [&first, &further] {
        let mut bytes = vec![0; queries::DEAL_VIEW_BYTES];
        let n =
            queries::answer_deal_view(&restarted.current, request.deal_id.as_bytes(), &mut bytes)
                .required();
        let view = DealView::decode(&bytes[..n]).required();
        assert_eq!(
            view.deal,
            restarted.book().deal(request.deal_id).required().required()
        );
        assert_eq!(view.deal.ask_id, first.ask_id);
        assert_eq!(view.deal.room_id, request.room_id);
        assert_eq!(view.deal.deal_id, request.deal_id);
        assert_eq!(view.deal.layers, request.layers);
        assert_eq!(view.deal.roster_digest, request.roster_digest);
        assert_eq!(
            view.deal.agreed_price_per_thousand_tokens,
            request.agreed_price_per_thousand_tokens
        );
        assert_eq!(view.deal.state, DealState::Open);
        assert_eq!(view.settled_tokens, 0);
        assert_eq!(view.settled_amount_owed, 0);
    }
    let (routed, result, _) = restarted.execute(HOST, &encoded);
    assert!(matches!(routed, Routed::Unchanged { .. }));
    assert_eq!(
        codec::decode_result(&result).required().status,
        ResultStatus::AlreadyApplied
    );
    assert_eq!(restarted.current, persisted);
}

#[test]
fn a_further_deal_passes_the_checks_of_a_first_deal() {
    let mut h = Harness::ready(123);
    let first = h.deal_request(digest(60), digest(61), digest(62));
    h.deal(&first);
    h.ask(digest(60), digest(64), 124);
    h.bid(digest(60), digest(72), PEER_A);
    h.bid(digest(60), digest(73), PEER_B);
    h.bid(digest(60), digest(74), PEER_B);
    let stale_roster = h.roster();
    let mut admission = h.admission();
    roster::open_epoch(&mut admission, 2, |_| false).required();
    h.install_admission(&admission);
    h.height = ORIGIN + 258;
    assert_ne!(h.roster(), stale_roster);
    let sequence = h.sequence(HOST);
    let before = h.current.clone();
    for case in 0..4 {
        let mut request = h.further_deal_request(digest(63));
        let error = match case {
            0 => {
                request.roster_digest = stale_roster;
                F11_ROSTER_DIGEST_MISMATCH
            }
            1 => {
                request.agreed_price_per_thousand_tokens = 21;
                CONFLICT
            }
            2 => {
                request.layers[0].as_mut().required().bid_id = digest(74);
                CONFLICT
            }
            3 => {
                request.deal_id = first.deal_id;
                CONFLICT
            }
            _ => unreachable!(),
        };
        for ask_id in [digest(64), first.ask_id] {
            request.ask_id = ask_id;
            h.refuse(
                dispatch::RecordDeal,
                HOST,
                &payload(1297, |out| request.encode(out)),
                Presence::Present(request.roster_digest),
                error,
            );
            assert_eq!(h.sequence(HOST), sequence);
            assert_eq!(h.current, before);
        }
    }
    assert_eq!(h.book().counts(), [1, 2, 5, 1]);
    assert_eq!(
        h.book().ask(first.ask_id).required().required().state,
        AskState::Matched
    );
    assert_eq!(
        h.book().ask(digest(64)).required().required().state,
        AskState::Open
    );
    assert!(h.book().deal(digest(63)).required().is_none());
    h.deal(&h.further_deal_request(digest(63)));
    assert_eq!(h.book().counts(), [1, 2, 5, 2]);
}

#[test]
fn a_further_deal_is_refused_from_another_account() {
    let mut h = Harness::ready(123);
    let first = h.deal_request(digest(60), digest(61), digest(62));
    h.deal(&first);
    h.bid(digest(60), digest(72), PEER_A);
    h.bid(digest(60), digest(73), PEER_B);
    let further = h.further_deal_request(digest(63));
    let sequence = h.sequence(PEER_A);
    h.refuse(
        dispatch::RecordDeal,
        PEER_A,
        &payload(1297, |out| further.encode(out)),
        Presence::Present(further.roster_digest),
        UNAUTHORIZED,
    );
    assert_eq!(h.sequence(PEER_A), sequence);
    assert_eq!(h.book().counts(), [1, 1, 4, 1]);
    assert_eq!(
        h.book().ask(first.ask_id).required().required().state,
        AskState::Matched
    );
    assert!(h.book().deal(further.deal_id).required().is_none());
    h.deal(&further);
    assert_eq!(h.book().counts(), [1, 1, 4, 2]);
}

#[test]
fn open_ask_bid_deal_persist_with_ordered_layers_and_roster_binding() {
    let mut h = Harness::ready(123);
    let before = h.state();
    let legacy = before.room_market().required().0.to_vec();
    let identity = before.section(Section::IdentityRoster).required().to_vec();
    let admission = before
        .section(Section::ReputationAdmission)
        .required()
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
    let room = restarted.book().room(digest(60)).required().required();
    assert_eq!(room.host_key, principal(HOST));
    assert_eq!(room.model_digest, digest(50));
    assert_eq!(room.runtime_digest, digest(51));
    assert_eq!(room.profile, profile());
    assert_eq!(room.state, RoomState::Open);
    assert_eq!(room.opened_at_epoch, 1);
    let ask = restarted.book().ask(digest(61)).required().required();
    assert_eq!(ask.asker_key, principal(ASKER));
    assert_eq!(ask.stream_id, 123);
    assert_eq!(ask.state, AskState::Matched);
    let bid = restarted.book().bid(digest(70)).required().required();
    assert_eq!(bid.peer_key, principal(PEER_A));
    assert_eq!(
        bid.capability_digest.bytes(),
        restarted.worker(PEER_A).metadata.bytes()
    );
    assert_eq!(bid.minimum_price_per_thousand_tokens, 5);
    assert_eq!(bid.bond_amount, 100);
    assert_eq!(bid.layer_capacity, 16);
    let deal = restarted.book().deal(digest(62)).required().required();
    assert_eq!(deal.ask_id, digest(61));
    assert_eq!(deal.layers, request.layers);
    assert_eq!(deal.roster_digest, restarted.roster());
    assert_eq!(deal.agreed_price_per_thousand_tokens, 10);
    assert_eq!(deal.state, DealState::Open);
    let (routed, result, _) = restarted.execute(HOST, &encoded);
    assert!(matches!(routed, Routed::Unchanged { .. }));
    assert_eq!(
        codec::decode_result(&result).required().status,
        ResultStatus::AlreadyApplied
    );
    assert_eq!(restarted.current, persisted);
    restarted.ask(digest(60), digest(63), 124);
    assert_eq!(restarted.book().counts(), [1, 2, 2, 1]);
    let state = restarted.state();
    assert_eq!(state.room_market().required().0, legacy);
    assert_eq!(state.section(Section::IdentityRoster).required(), identity);
    assert_eq!(
        state.section(Section::ReputationAdmission).required(),
        admission
    );
}

#[test]
fn roster_digest_mismatch_refuses_without_consuming_the_host_sequence() {
    let mut h = Harness::ready(123);
    let sequence = h.sequence(HOST);
    let mut request = h.deal_request(digest(60), digest(61), digest(62));
    request.roster_digest = RosterDigest::new([99; 32]).required();
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
    roster::open_epoch(&mut table, 2, |_| false).required();
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
        h.book().ask(digest(61)).required().required().state,
        AskState::Open
    );
    assert!(h.book().deal(digest(62)).required().is_none());
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
        h.book().room(digest(60)).required().required().state,
        RoomState::Open
    );
    assert_eq!(
        h.book().deal(digest(62)).required().required().state,
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
        h.book().room(digest(60)).required().required().state,
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
    let bytes = payload(OPEN_ROOM_REQUEST_BYTES, |out| {
        open_request(digest(65)).encode(out)
    });
    let envelope = h.envelope(dispatch::OpenRoom, HOST, &bytes, Presence::Absent);
    let (routed, result, _) = h.execute(PEER_A, &envelope);
    assert!(matches!(routed, Routed::Refused { .. }));
    assert_eq!(
        codec::decode_result(&result).required().error,
        Some(UNAUTHORIZED)
    );
    assert!(h.book().room(digest(65)).required().is_none());
    h.apply(dispatch::OpenRoom, PEER_A, &bytes, Presence::Absent);
    assert_eq!(
        h.book().room(digest(65)).required().required().host_key,
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
    request.capability_digest = Digest32::new(h.worker(PEER_A).metadata.bytes()).required();
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
    request.layers[1].as_mut().required().end_layer = 25;
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
    request.layers[1].as_mut().required().bid_id = digest(73);
    h.refuse(
        dispatch::RecordDeal,
        HOST,
        &payload(1297, |out| request.encode(out)),
        Presence::Present(request.roster_digest),
        CONFLICT,
    );
    request.layers[1].as_mut().required().bid_id = digest(70);
    request.layers[1].as_mut().required().first_layer = 9;
    assert_eq!(request.encode(&mut [0; 1297]), Err(NON_CANONICAL));
    request.layers[1].as_mut().required().first_layer = 8;
    request.layers[1].as_mut().required().bid_id = digest(71);
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
        codec::decode_result(&result).required().status,
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
    let room = h.book().room(digest(60)).required().required();
    let ask = h.book().ask(digest(61)).required().required();
    let bid = h.book().bid(digest(70)).required().required();
    let deal = h.book().deal(digest(62)).required().required();
    let room_bytes = payload(ROOM_RECORD_BYTES, |out| room.encode(out));
    let ask_bytes = payload(ASK_RECORD_BYTES, |out| ask.encode(out));
    let bid_bytes = payload(BID_RECORD_BYTES, |out| bid.encode(out));
    let deal_bytes = payload(DEAL_RECORD_BYTES, |out| deal.encode(out));
    assert_eq!(RoomRecord::decode(&room_bytes).required(), room);
    assert_eq!(AskRecord::decode(&ask_bytes).required(), ask);
    assert_eq!(BidRecord::decode(&bid_bytes).required(), bid);
    assert_eq!(DealRecord::decode(&deal_bytes).required(), deal);
    let decoders: [DecodeCheck; 4] = [
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
    invalid[4 * 32 + layerx_programs_ai_market::policy::ROOM_PROFILE_BYTES] = 2;
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
        .required()
        .to_vec();
    for i in (0..ROOM_CAPACITY).rev() {
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
    for i in (0..ASK_CAPACITY).rev() {
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
    for i in (0..SIDE_CAPACITY).rev() {
        h.bid(digest(100), digest(1 + u8::try_from(i).required()), PEER_A);
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
        h.bid(digest(108), digest(1 + u8::try_from(i).required()), PEER_A);
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
    assert!(h.current.len() <= MAX_STATE_BYTES);
    assert_eq!(h.state().room_market().required().0, legacy);
    assert_eq!(
        h.book().room(digest(100)).required().required().room_id,
        digest(100)
    );
    assert_eq!(
        h.book().ask(digest(64)).required().required().ask_id,
        digest(64)
    );
    assert_eq!(
        h.book().bid(digest(128)).required().required().bid_id,
        digest(128)
    );
    let mut malformed = h.state().room_market().required().1.to_vec();
    malformed[10..12].copy_from_slice(&129u16.to_be_bytes());
    assert!(matches!(RoomBook::decode(&malformed), Err(CAPACITY)));
}

#[test]
fn f11_room_profile_survives_room_view_restart() -> CodecResult<()> {
    use layerx_programs_ai_market::{
        policy::{MarketPolicy, MARKET_POLICY_BYTES},
        queries::{self, RoomView},
    };

    let mut h = Harness::new();
    let policy = MarketPolicy {
        task_policy: PolicySection::decode(h.state().section(Section::PolicyLifecycle)?)?.current,
        room_profile: RoomProfile {
            max_generated_tokens: (1u64 << 40) + 19,
            queue_depth: 47,
            liveness_timeout_ms: (1u64 << 34) + 123,
            acknowledgement_window_ms: 12_345,
            canary_relative_tolerance_bps: 321,
            ..profile()
        },
    };
    let mut policy_bytes = [0; MARKET_POLICY_BYTES];
    policy.encode(&mut policy_bytes)?;
    let decoded = MarketPolicy::decode(&policy_bytes)?;
    let request = OpenRoomRequest {
        profile: decoded.room_profile,
        ..open_request(digest(60))
    };
    let bytes = payload(OPEN_ROOM_REQUEST_BYTES, |out| request.encode(out));
    assert_eq!(bytes.len(), OPEN_ROOM_REQUEST_BYTES);
    let mut unsupported = bytes.clone();
    unsupported[96..98].copy_from_slice(&1u16.to_be_bytes());
    h.refuse(
        dispatch::OpenRoom,
        HOST,
        &unsupported,
        Presence::Absent,
        BAD_VERSION,
    );
    h.apply(dispatch::OpenRoom, HOST, &bytes, Presence::Absent);

    let mut persisted = vec![0; MAX_STATE_BYTES];
    let mut control = vec![0; Section::Control.payload_cap()];
    let n = state::encode_shared_state(&h.state(), &mut persisted, &mut control)?;
    persisted.truncate(n);
    assert_eq!(persisted, h.current);
    let mut restarted = Harness {
        current: persisted.clone(),
        height: h.height,
        request: h.request,
    };
    let header = restarted.header();
    let mut input = vec![0; MAX_ENVELOPE_BYTES];
    let n = queries::encode_view_envelope(
        &Envelope {
            operation: dispatch::RoomView,
            chain: header.deployment_chain_domain,
            program: header.program_id,
            market: header.market_id,
            actor: principal(ASKER),
            epoch: 0,
            config: 0,
            roster: Presence::Absent,
            sequence: 0,
            expiry: restarted.height + 10,
            request: RequestId::new([99; 32])?,
            payload: request.room_id.as_bytes(),
            authentication: Authentication::Native,
        },
        &mut input,
    )?;
    let (routed, bytes, event) = restarted.execute(ASKER, &input[..n]);
    assert!(matches!(routed, Routed::Unchanged { .. }));
    assert!(event.is_empty());
    assert_eq!(restarted.current, persisted);
    let result = codec::decode_result(&bytes)?;
    assert_eq!(result.status, ResultStatus::Ok);
    assert_eq!(result.error, None);
    let view = RoomView::decode(result.payload)?;
    assert_eq!(result.payload.len(), queries::ROOM_VIEW_BYTES);
    assert_eq!(view.room.room_id, request.room_id);
    assert_eq!(view.room.profile, policy.room_profile);
    assert_eq!(view.room.profile.max_generated_tokens, (1u64 << 40) + 19);
    assert_eq!(view.room.profile.queue_depth, 47);
    assert_eq!(view.room.profile.liveness_timeout_ms, (1u64 << 34) + 123);
    assert_eq!(view.room.profile.acknowledgement_window_ms, 12_345);
    assert_eq!(view.room.profile.canary_relative_tolerance_bps, 321);

    // Both standalone room records and the book refuse prior profile formats.
    let mut room_bytes = [0; ROOM_RECORD_BYTES];
    view.room.encode(&mut room_bytes)?;
    room_bytes[128..130].copy_from_slice(&1u16.to_be_bytes());
    assert_eq!(RoomRecord::decode(&room_bytes), Err(BAD_VERSION));
    let mut book_bytes = restarted.state().room_market()?.1.to_vec();
    book_bytes[4..6].copy_from_slice(&1u16.to_be_bytes());
    assert!(matches!(RoomBook::decode(&book_bytes), Err(BAD_VERSION)));
    Ok(())
}

#[test]
fn a_further_deal_names_the_bids_of_the_earlier_deal_of_its_ask() {
    use layerx_programs_ai_market::{
        placement::PlacementBook,
        queries::{self, DealView, RoomView},
    };

    for further_id in [59, 63] {
        let mut h = Harness::ready(123);
        let first = h.deal_request(digest(60), digest(61), digest(62));
        h.deal(&first);
        let earlier = h.book().deal(first.deal_id).required().required();
        let ask = h.book().ask(first.ask_id).required().required();
        let mut further = h.deal_request(first.room_id, first.ask_id, digest(further_id));
        further.layers[0].as_mut().required().end_layer = 12;
        further.layers[1].as_mut().required().first_layer = 12;
        assert_ne!(first.layers, further.layers);
        let sequence = h.sequence(HOST);
        let encoded = h.deal(&further);
        assert_eq!(h.sequence(HOST), sequence + 1);
        let persisted = h.current.clone();
        let mut restarted = Harness {
            current: persisted.clone(),
            height: h.height,
            request: h.request,
        };
        assert_eq!(restarted.book().counts(), [1, 1, 2, 2]);
        assert_eq!(
            restarted.book().ask(first.ask_id).required().required(),
            ask
        );
        assert_eq!(
            restarted.book().deal(first.deal_id).required().required(),
            earlier
        );
        assert_eq!(
            PlacementBook::decode(restarted.state().room_market().required().1, digest(50))
                .required()
                .counts(),
            [0, 0]
        );
        for request in [&first, &further] {
            let mut bytes = vec![0; queries::DEAL_VIEW_BYTES];
            let n = queries::answer_deal_view(
                &restarted.current,
                request.deal_id.as_bytes(),
                &mut bytes,
            )
            .required();
            let view = DealView::decode(&bytes[..n]).required();
            assert_eq!(
                view.deal,
                restarted.book().deal(request.deal_id).required().required()
            );
            assert_eq!(view.deal.ask_id, request.ask_id);
            assert_eq!(view.deal.room_id, request.room_id);
            assert_eq!(view.deal.deal_id, request.deal_id);
            assert_eq!(view.deal.layers, request.layers);
            assert_eq!(view.deal.roster_digest, request.roster_digest);
            assert_eq!(
                view.deal.agreed_price_per_thousand_tokens,
                request.agreed_price_per_thousand_tokens
            );
            assert_eq!(view.deal.state, DealState::Open);
            assert_eq!(view.settled_tokens, 0);
            assert_eq!(view.settled_amount_owed, 0);
        }
        let mut bytes = vec![0; queries::ROOM_VIEW_BYTES];
        let n = queries::answer_room_view(&restarted.current, first.room_id.as_bytes(), &mut bytes)
            .required();
        let room = RoomView::decode(&bytes[..n]).required();
        assert!(room.open_asks.iter().all(Option::is_none));
        assert!(room.open_bids.iter().all(Option::is_none));
        let (routed, result, event) = restarted.execute(HOST, &encoded);
        assert!(matches!(routed, Routed::Unchanged { .. }));
        assert_eq!(
            codec::decode_result(&result).required().status,
            ResultStatus::AlreadyApplied
        );
        assert!(event.is_empty());
        assert_eq!(restarted.current, persisted);
        assert_eq!(restarted.sequence(HOST), sequence + 1);
    }
}

#[test]
fn a_deal_of_another_ask_that_names_a_bid_of_a_deal_is_refused() {
    let mut h = Harness::ready(123);
    let first = h.deal_request(digest(60), digest(61), digest(62));
    h.deal(&first);
    // A third deal must also agree with every existing use of the shared bid.
    let mut same_ask = h.deal_request(first.room_id, first.ask_id, digest(59));
    same_ask.layers[0].as_mut().required().end_layer = 12;
    same_ask.layers[1].as_mut().required().first_layer = 12;
    h.deal(&same_ask);
    h.ask(first.room_id, digest(64), 124);
    let before = h.current.clone();
    let sequence = h.sequence(HOST);
    for bid in [70, 71] {
        let mut other = h.deal_request(first.room_id, digest(64), digest(63));
        other.layers = [None; MAX_DEAL_PEERS];
        other.layers[0] = Some(LayerSplit {
            bid_id: digest(bid),
            first_layer: 0,
            end_layer: 8,
        });
        h.refuse(
            dispatch::RecordDeal,
            HOST,
            &payload(1297, |out| other.encode(out)),
            Presence::Present(other.roster_digest),
            CONFLICT,
        );
        assert_eq!(h.current, before);
        assert_eq!(h.sequence(HOST), sequence);
        assert!(h.book().deal(other.deal_id).required().is_none());
        assert_eq!(
            h.book().ask(other.ask_id).required().required().state,
            AskState::Open
        );
    }
}
