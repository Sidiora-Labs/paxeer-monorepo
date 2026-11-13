use layerx_ai_market_test_values::RequiredValue as _;
#[allow(
    dead_code,
    reason = "Shared room producers expose the operations used by each independent suite."
)]
#[path = "support/f11_rooms.rs"]
mod rooms_support;
use rooms_support::*;

use layerx_programs_ai_market::{epoch, queries};
#[path = "repair_22_33_260.rs"]
mod producers;

fn read_market_header(h: &Harness) -> codec::ReadHeader {
    let ctx = h.context(HOST);
    queries::read_header(&h.current, ctx.chain, ctx.program).required()
}

fn install_settlement(h: &mut Harness, settlement: &[u8]) {
    let state = h.state();
    let next = state
        .replace_section(Section::SettlementClaims, settlement)
        .required();
    let mut out = vec![0; MAX_STATE_BYTES];
    let mut control = vec![0; Section::Control.payload_cap()];
    let n = state::encode_shared_state(&next, &mut out, &mut control).required();
    h.current = out[..n].to_vec();
}

fn retain_admissions_before_first_epoch(h: &mut Harness) {
    let mut table = vec![0; TABLE_MAX_BYTES];
    let n = h.admission().encode(&mut table).required();
    table.truncate(n);
    assert_eq!(table[2], 1);
    table.drain(3..11);
    table[2] = 0;
    let state = h.state();
    let (region, _) =
        split_joint(state.section(Section::ReputationAdmission).required()).required();
    let mut joint = vec![0; Section::ReputationAdmission.payload_cap()];
    let n = if let Some(region) = region {
        layerx_programs_ai_market::reputation_transition::encode_joint(&region, &table, &mut joint)
            .required()
    } else {
        joint[..table.len()].copy_from_slice(&table);
        table.len()
    };
    let next = state
        .replace_section(Section::ReputationAdmission, &joint[..n])
        .required();
    let mut out = vec![0; MAX_STATE_BYTES];
    let mut control = vec![0; Section::Control.payload_cap()];
    let n = state::encode_shared_state(&next, &mut out, &mut control).required();
    h.current = out[..n].to_vec();
    h.height = ORIGIN + 8;
}

fn open_room_before_epoch(h: &mut Harness) {
    let bytes = payload(OPEN_ROOM_REQUEST_BYTES, |out| {
        open_request(digest(60)).encode(out)
    });
    h.request += 1;
    let ctx = h.context(HOST);
    let mut request = [1; 32];
    request[..8].copy_from_slice(&h.request.to_be_bytes());
    let envelope = payload(MAX_ENVELOPE_BYTES, |out| {
        codec::encode_envelope(
            &Envelope {
                operation: dispatch::OpenRoom,
                chain: ctx.chain,
                program: ctx.program,
                market: codec::derive_market(ctx.chain, ctx.program)?,
                actor: principal(HOST),
                epoch: (h.height - ORIGIN) / 128,
                config: 1,
                roster: Presence::Present(RosterDigest::new([4; 32])?),
                sequence: h.sequence(HOST),
                expiry: ORIGIN + 1000,
                request: RequestId::new(request)?,
                payload: &bytes,
                authentication: Authentication::Native,
            },
            out,
        )
    });
    let (routed, result, _) = h.execute(HOST, &envelope);
    assert!(
        matches!(routed, Routed::Applied { .. }),
        "{:?}",
        codec::decode_result(&result).required()
    );
}

fn open_committed_epoch(h: &mut Harness) {
    h.current.clear();
    h.height = ORIGIN;
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
    let mut owner_sequence = h.sequence(HOST);
    producers::enroll(
        &mut h.current,
        &mut owner_sequence,
        principal(PEER_A),
        PEER_A,
        &SigningKey::from_bytes(&[0x90 + PEER_A; 32]),
        MetadataDigest::new([0x44; 32]).required(),
        ORIGIN + 2,
    )
    .required();
    for principal_byte in [40, 41, 42] {
        let key = SigningKey::from_bytes(&[principal_byte; 32]);
        producers::evaluator(
            &mut h.current,
            &mut owner_sequence,
            principal(principal_byte),
            principal_byte,
            &key,
            ORIGIN + 2,
        )
        .required();
    }
    let fund = payload(57, |out| {
        FundRequest {
            amount: 500,
            refund_recipient: AccountId::new([14; 32])?,
            policy_version: FUNDING_POLICY_VERSION,
            consent: true,
        }
        .encode(out)
    });
    producers::call(
        &mut h.current,
        dispatch::FUND,
        principal(HOST),
        &fund,
        ORIGIN + 8,
    )
    .required();
    producers::open(&mut h.current, ORIGIN + 256).required();
    h.height = ORIGIN + 257;
    h.request = 1000;
    assert_eq!(
        epoch::captured_roster(&h.state())
            .required()
            .required()
            .binding
            .epoch,
        2
    );
}

#[test]
fn header_answers_after_a_room_opens_before_the_first_epoch() {
    let mut h = Harness::new();
    retain_admissions_before_first_epoch(&mut h);
    install_settlement(&mut h, &[]);
    open_room_before_epoch(&mut h);
    assert!(h.state().room_market().required().0.is_empty());
    assert!(!h.state().room_market().required().1.is_empty());
    let header = read_market_header(&h);
    assert_eq!(header.revision, h.state().revision);
    assert_eq!(header.market, h.header().market_id);
    assert_eq!(header.epoch, Presence::Absent);
    assert_eq!(header.roster, Presence::Absent);
    assert!(epoch::captured_roster(&h.state()).required().is_none());
    let availability = queries::feature_availability(&h.current).required();
    assert_eq!(availability[5], queries::Availability::NotYetProduced);
    assert_eq!(availability[10], queries::Availability::Available);
}

#[test]
fn header_answers_after_a_room_opens_on_a_market_with_a_reward_state() {
    let mut h = Harness::new();
    open_committed_epoch(&mut h);
    let rewards = h.state().room_market().required().0.to_vec();
    assert_eq!(rewards.len(), REWARD_STATE_BYTES);
    h.open(digest(60));
    assert_eq!(h.state().room_market().required().0, rewards);
    let header = read_market_header(&h);
    assert_eq!(header.revision, h.state().revision);
    assert_eq!(header.market, h.header().market_id);
    assert_eq!(header.epoch, Presence::Present(2));
    assert!(matches!(header.roster, Presence::Present(_)));
    assert_eq!(
        state::settlement_rewards(&h.state(), None)
            .required()
            .bytes(),
        rewards
    );
}

#[test]
fn header_answers_after_the_room_closes() {
    let mut h = Harness::new();
    open_committed_epoch(&mut h);
    h.open(digest(60));
    h.apply(
        dispatch::CloseRoom,
        HOST,
        digest(60).as_bytes(),
        Presence::Absent,
    );
    assert_eq!(
        h.book().room(digest(60)).required().required().state,
        RoomState::Closed
    );
    let header = read_market_header(&h);
    assert_eq!(header.revision, h.state().revision);
    assert_eq!(header.market, h.header().market_id);
    assert_eq!(header.epoch, Presence::Present(2));
}

#[test]
fn reward_and_room_readers_use_their_own_section_parts() {
    let mut h = Harness::new();
    let reward = h.state().room_market().required().0.to_vec();
    h.open(digest(60));
    let state = h.state();
    assert_eq!(state.room_market().required().0, reward);
    assert!(!state.room_market().required().1.is_empty());
    assert_eq!(
        state::settlement_rewards(&state, None).required().bytes(),
        reward
    );
    assert_eq!(
        h.book().room(digest(60)).required().required().state,
        RoomState::Open
    );
    let availability = queries::feature_availability(&h.current).required();
    assert_eq!(availability[5], queries::Availability::Available);
    assert_eq!(availability[10], queries::Availability::Available);
}

#[test]
fn funding_preserves_the_room_market_tail() {
    let mut h = Harness::new();
    retain_admissions_before_first_epoch(&mut h);
    open_room_before_epoch(&mut h);
    let room_before = h.state().room_market().required().1.to_vec();
    let before = state::settlement_rewards(&h.state(), None)
        .required()
        .ledger()
        .required();
    let fund = payload(57, |out| {
        FundRequest {
            amount: 25,
            refund_recipient: AccountId::new([14; 32])?,
            policy_version: FUNDING_POLICY_VERSION,
            consent: true,
        }
        .encode(out)
    });
    h.apply(dispatch::FUND, HOST, &fund, Presence::Absent);
    assert_eq!(h.state().room_market().required().1, room_before);
    assert_eq!(
        h.book().room(digest(60)).required().required().state,
        RoomState::Open
    );
    let after = state::settlement_rewards(&h.state(), None)
        .required()
        .ledger()
        .required();
    assert_eq!(after.tracked_deposits, before.tracked_deposits + 25);
}
