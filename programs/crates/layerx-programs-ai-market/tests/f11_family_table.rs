use layerx_programs_ai_market::{
    codec::{self, Envelope},
    dispatch::{self, AuthorityClass, CallBoundary, Operation, SequencePolicy, OPERATIONS},
    errors::{self, ApplicationError, CodecResult, APPLICATION_ALIASES, APPLICATION_ERRORS},
    evidence, laps,
    policy::{
        MarketPolicy, PolicyCommitments, RoomProfile, TaskPolicyV1, MARKET_POLICY_BYTES,
        MARKET_POLICY_VERSION, ROOM_PROFILE_BYTES, TASK_POLICY_BYTES,
    },
    queries::{feature_availability, Availability},
    registry::{derive_rewards_account, F01_SECTION_CAP},
    registry_ops,
    rooms::{self, OpenRoomRequest, RoomBook, OPEN_ROOM_REQUEST_BYTES},
    state::{self, Control, ReplayTable, Section, SharedState},
    types::{
        AccountId, AssetId, Authentication, ChainDomain, Digest32, Presence, PrincipalId,
        ProgramId, RequestId, RosterDigest, RubricDigest,
    },
    F11_ASK_UNFUNDED, F11_BATCH_UNSEALED, F11_BID_BELOW_FLOOR, F11_BOND_INSUFFICIENT,
    F11_BOND_LOCKED, F11_CHALLENGE_WINDOW_CLOSED, F11_ROOM_CLOSED, F11_ROOM_UNKNOWN,
    F11_ROSTER_DIGEST_MISMATCH, F11_SPLIT_EXCEEDS_SETTLED, MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES,
    MAX_RESULT_BYTES, MAX_STATE_BYTES,
};

#[test]
fn f11_selectors_route_to_their_operation_metadata() -> CodecResult<()> {
    use AuthorityClass::*;
    let expected = [
        (
            Operation::OpenRoom,
            dispatch::OpenRoom,
            0x0B01,
            "OpenRoom",
            layerx_programs_ai_market::rooms::OPEN_ROOM_REQUEST_BYTES,
            layerx_programs_ai_market::rooms::OPEN_ROOM_REQUEST_BYTES,
            Host,
        ),
        (
            Operation::PostAsk,
            dispatch::PostAsk,
            0x0B02,
            "PostAsk",
            128,
            128,
            AnyAccount,
        ),
        (
            Operation::PostBid,
            dispatch::PostBid,
            0x0B03,
            "PostBid",
            130,
            130,
            EnrolledPeer,
        ),
        (
            Operation::RecordDeal,
            dispatch::RecordDeal,
            0x0B04,
            "RecordDeal",
            181,
            1297,
            Host,
        ),
        (
            Operation::SettleLaps,
            dispatch::SettleLaps,
            0x0B05,
            "SettleLaps",
            laps::SETTLE_FIXED_BYTES + evidence::MANIFEST_FIXED_BYTES,
            laps::SETTLE_FIXED_BYTES + evidence::MAX_MANIFEST_BYTES,
            Host,
        ),
        (
            Operation::ChallengeLap,
            dispatch::ChallengeLap,
            0x0B06,
            "ChallengeLap",
            88,
            88,
            AnyAccount,
        ),
        (
            Operation::ResolveChallenge,
            dispatch::ResolveChallenge,
            0x0B07,
            "ResolveChallenge",
            104,
            104,
            AnyAccount,
        ),
        (
            Operation::SplitPayout,
            dispatch::SplitPayout,
            0x0B08,
            "SplitPayout",
            80,
            80,
            HostOrAfterChallengeWindow,
        ),
        (
            Operation::LockBond,
            dispatch::LockBond,
            0x0B09,
            "LockBond",
            80,
            80,
            EnrolledPeer,
        ),
        (
            Operation::SlashBond,
            dispatch::SlashBond,
            0x0B0A,
            "SlashBond",
            104,
            104,
            ResolvedFault,
        ),
        (
            Operation::RefundBond,
            dispatch::RefundBond,
            0x0B0B,
            "RefundBond",
            64,
            64,
            BondOwner,
        ),
        (
            Operation::CloseRoom,
            dispatch::CloseRoom,
            0x0B0C,
            "CloseRoom",
            32,
            32,
            Host,
        ),
        (
            Operation::RoomView,
            dispatch::RoomView,
            0x0B10,
            "RoomView",
            32,
            32,
            PublicRead,
        ),
        (
            Operation::DealView,
            dispatch::DealView,
            0x0B11,
            "DealView",
            32,
            32,
            PublicRead,
        ),
        (
            Operation::PeerView,
            dispatch::PeerView,
            0x0B12,
            "PeerView",
            32,
            32,
            PublicRead,
        ),
        (
            Operation::ReceiptBatchView,
            dispatch::ReceiptBatchView,
            0x0B13,
            "ReceiptBatchView",
            64,
            64,
            PublicRead,
        ),
        (
            Operation::BookView,
            dispatch::BookView,
            0x0B14,
            "BookView",
            32,
            32,
            PublicRead,
        ),
    ];
    assert_eq!(OPERATIONS.iter().filter(|m| m.feature == 11).count(), 17);
    for (associated, exported, selector, name, min, max, authority) in expected {
        let operation = Operation::decode(selector)?;
        assert_eq!(operation, associated);
        assert_eq!(operation, exported);
        assert_eq!(operation.selector(), selector);
        let metadata = operation.metadata();
        assert_eq!(metadata.operation, operation);
        assert_eq!(metadata.name, name);
        assert_eq!(metadata.feature, 11);
        assert_eq!(metadata.authority, authority);
        assert_eq!((metadata.payload_min, metadata.payload_max), (min, max));
        assert!(!metadata.delegate_allowed);
        let read = selector >= 0x0B10;
        assert_eq!(
            metadata.boundary,
            if read {
                CallBoundary::ProgramRead
            } else {
                CallBoundary::Mutation
            }
        );
        assert_eq!(
            metadata.sequence,
            if read
                || matches!(
                    authority,
                    AnyAccount | HostOrAfterChallengeWindow | ResolvedFault
                )
            {
                SequencePolicy::ObjectLocal
            } else {
                SequencePolicy::Role
            }
        );
        use Section::*;
        let sections: &[Section] = match selector {
            0x0B01 | 0x0B02 | 0x0B04 => &[PolicyLifecycle, IdentityRoster, Control],
            0x0B03 | 0x0B05 | 0x0B09 => {
                &[PolicyLifecycle, IdentityRoster, SettlementClaims, Control]
            }
            0x0B06 => &[PolicyLifecycle, IdentityRoster, CurrentReports, Control],
            0x0B07 | 0x0B08 | 0x0B0A | 0x0B0B | 0x0B0C => &[
                PolicyLifecycle,
                IdentityRoster,
                CurrentReports,
                SettlementClaims,
                Control,
            ],
            0x0B10 => &[PolicyLifecycle, IdentityRoster],
            0x0B11 | 0x0B12 | 0x0B13 => &[IdentityRoster, SettlementClaims],
            0x0B14 => &[PolicyLifecycle, SettlementClaims],
            _ => unreachable!(),
        };
        assert_eq!(metadata.sections, sections);
        assert_eq!(
            metadata.sections.contains(&Section::IdentityRoster),
            selector != 0x0B14
        );
        assert_eq!(metadata.sections.contains(&Section::Control), !read);
        for (i, section) in metadata.sections.iter().enumerate() {
            assert!(!metadata.sections[..i].contains(section));
        }
        assert_eq!(operation.validate_payload_length(min), Ok(()));
        assert_eq!(operation.validate_payload_length(max), Ok(()));
        assert_eq!(
            operation.validate_payload_length(min - 1),
            Err(errors::NON_CANONICAL)
        );
        assert_eq!(
            operation.validate_payload_length(max + 1),
            Err(errors::NON_CANONICAL)
        );
    }
    for left in OPERATIONS {
        assert_eq!(
            OPERATIONS
                .iter()
                .filter(|right| left.operation == right.operation)
                .count(),
            1
        );
    }
    for selector in [0x0B00, 0x0B0D, 0x0B0E, 0x0B0F, 0x0B15, 0x0BFF] {
        assert_eq!(Operation::decode(selector), Err(errors::UNKNOWN_OPERATION));
    }
    for count in 1..=32 {
        assert_eq!(
            dispatch::RecordDeal.validate_payload_length(145 + 36 * count),
            Ok(())
        );
        assert_eq!(
            dispatch::RecordDeal.validate_payload_length(146 + 36 * count),
            Err(errors::NON_CANONICAL)
        );
    }
    Ok(())
}

#[test]
fn f11_mutation_selectors_survive_the_real_envelope_codec() -> CodecResult<()> {
    let chain = ChainDomain::new([1; 32])?;
    let program = ProgramId::new([2; 32])?;
    for selector in 0x0B01..=0x0B0C {
        let operation = Operation::decode(selector)?;
        let payload = vec![0x2a; operation.metadata().payload_min];
        let envelope = Envelope {
            operation,
            chain,
            program,
            market: codec::derive_market(chain, program)?,
            actor: PrincipalId::new([3; 32])?,
            epoch: 1,
            config: 1,
            roster: Presence::Present(RosterDigest::new([4; 32])?),
            sequence: u64::from(operation.metadata().sequence == SequencePolicy::Role),
            expiry: 100,
            request: RequestId::new([5; 32])?,
            payload: &payload,
            authentication: Authentication::Native,
        };
        let mut bytes = vec![0; MAX_ENVELOPE_BYTES];
        let n = codec::encode_envelope(&envelope, &mut bytes)?;
        let decoded = codec::decode_envelope(&bytes[..n])?;
        assert_eq!(decoded.envelope, envelope);
        assert_eq!(decoded.envelope.operation.selector(), selector);
    }
    Ok(())
}

#[test]
fn f11_availability_has_eleven_entries_from_encoded_shared_state() -> CodecResult<()> {
    let state = SharedState {
        revision: 1,
        feature_sections: [&[]; 5],
        control: Control {
            replay: ReplayTable::new(),
            feature_bytes: &[],
        },
    };
    let mut bytes = vec![0; state.encoded_len()?];
    let mut scratch = vec![0; Section::Control.payload_cap()];
    let n = state::encode_shared_state(&state, &mut bytes, &mut scratch)?;
    let availability: [Availability; 11] = feature_availability(&bytes[..n])?;
    assert_eq!(availability.len(), 11);
    assert_eq!(availability, [Availability::NotYetProduced; 11]);
    assert!(feature_availability(&bytes[..n - 1]).is_err());
    Ok(())
}

#[test]
fn f11_availability_is_not_enabled_for_a_created_market_without_a_room_profile() -> CodecResult<()>
{
    let bytes = created_market()?;
    let availability = feature_availability(&bytes)?;
    assert_eq!(availability.len(), 11);
    assert_eq!(availability[0], Availability::Available);
    assert_eq!(availability[10], Availability::NotEnabled);
    Ok(())
}

fn created_market() -> CodecResult<Vec<u8>> {
    let chain = ChainDomain::new([1; 32])?;
    let program = ProgramId::new([2; 32])?;
    let owner = PrincipalId::new([3; 32])?;
    let asset = AssetId::new([4; 32])?;
    let rewards = derive_rewards_account(program, asset)?;
    let mut payload = owner.as_bytes().to_vec();
    payload.extend_from_slice(asset.as_bytes());
    payload.extend_from_slice(rewards.as_bytes());
    payload.extend_from_slice(&[5; 32]);
    payload.push(0);
    let mut policy = [0; TASK_POLICY_BYTES];
    market_policy()?.task_policy.encode(&mut policy)?;
    payload.extend_from_slice(&policy);
    payload.extend_from_slice(&[6; 32]);
    let envelope = Envelope {
        operation: dispatch::CREATE,
        chain,
        program,
        market: codec::derive_market(chain, program)?,
        actor: owner,
        epoch: 0,
        config: 1,
        roster: Presence::Absent,
        sequence: 1,
        expiry: 100,
        request: RequestId::new([7; 32])?,
        payload: &payload,
        authentication: Authentication::Native,
    };
    let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
    let n = codec::encode_envelope(&envelope, &mut encoded)?;
    let validated = codec::decode_envelope(&encoded[..n])?;
    let ctx = registry_ops::CallContext {
        chain,
        program,
        principal: owner,
        height: 1,
    };
    let mut section = vec![0; F01_SECTION_CAP];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let registry_ops::Outcome::Applied { state, .. } =
        registry_ops::apply(&ctx, None, &validated, &mut section, &mut event)?
    else {
        panic!("a fresh create must apply");
    };
    let mut bytes = vec![0; state.encoded_len()?];
    let mut scratch = vec![0; Section::Control.payload_cap()];
    let n = state::encode_shared_state(&state, &mut bytes, &mut scratch)?;
    bytes.truncate(n);
    Ok(bytes)
}

fn room_market_capture() -> CodecResult<Vec<u8>> {
    let current = created_market()?;
    let shared = state::decode_shared_state(&current)?;
    let header =
        registry_ops::PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?.header;
    let admission = layerx_programs_ai_market::admission::AdmissionTable::decode(
        layerx_programs_ai_market::reputation_transition::split_joint(
            shared.section(Section::ReputationAdmission)?,
        )?
        .1,
    )?;
    let roster = RosterDigest::new(
        layerx_programs_ai_market::roster::roster_digest(&admission, 0)?.bytes(),
    )?;
    let request = OpenRoomRequest {
        room_id: Digest32::new([9; 32])?,
        model_digest: market_policy()?.task_policy.commitments.model_artifact,
        runtime_digest: Digest32::new([10; 32])?,
        profile: market_policy()?.room_profile,
    };
    let mut payload = vec![0; OPEN_ROOM_REQUEST_BYTES];
    let n = request.encode(&mut payload)?;
    payload.truncate(n);
    let envelope = Envelope {
        operation: dispatch::OpenRoom,
        chain: header.deployment_chain_domain,
        program: header.program_id,
        market: header.market_id,
        actor: header.owner_principal,
        epoch: 0,
        config: 1,
        roster: Presence::Present(roster),
        sequence: 2,
        expiry: 100,
        request: RequestId::new([11; 32])?,
        payload: &payload,
        authentication: Authentication::Native,
    };
    let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
    let n = codec::encode_envelope(&envelope, &mut encoded)?;
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    let routed = dispatch::route(
        &registry_ops::CallContext {
            chain: envelope.chain,
            program: envelope.program,
            principal: envelope.actor,
            height: header.origin_height,
        },
        &encoded[..n],
        Some(&current),
        dispatch::Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?;
    let dispatch::Routed::Applied { state_len, .. } = routed else {
        panic!("OpenRoom must apply: {routed:?}");
    };
    next.truncate(state_len);
    let shared = state::decode_shared_state(&next)?;
    assert_eq!(
        RoomBook::decode(shared.room_market()?.1)?
            .room(request.room_id)?
            .unwrap()
            .profile,
        request.profile
    );
    Ok(next)
}

#[test]
fn f11_availability_is_available_for_a_captured_room_profile_and_placement_book() -> CodecResult<()>
{
    let bytes = room_market_capture()?;
    let shared = state::decode_shared_state(&bytes)?;
    let book = shared.room_market()?.1;
    assert_eq!(RoomBook::decode(book)?.counts(), [1, 0, 0, 0]);
    assert_eq!(
        layerx_programs_ai_market::queries::placement::PlacementBook::decode(
            book,
            market_policy()?.task_policy.commitments.model_artifact,
        )?
        .counts(),
        [0, 0]
    );
    let availability = feature_availability(&bytes)?;
    assert_eq!(availability[0], Availability::Available);
    assert_eq!(availability[10], Availability::Available);
    Ok(())
}

#[test]
fn f11_availability_is_content_unavailable_for_an_undecodable_room_book() -> CodecResult<()> {
    let bytes = room_market_capture()?;
    let shared = state::decode_shared_state(&bytes)?;
    let book = shared.room_market()?.1;
    let mut malformed_books = Vec::new();
    let mut bad_magic = book.to_vec();
    bad_magic[0] ^= 1;
    malformed_books.push(bad_magic);
    let mut bad_version = book.to_vec();
    bad_version[4..6].copy_from_slice(&(rooms::ROOM_BOOK_VERSION + 1).to_be_bytes());
    malformed_books.push(bad_version);
    let mut bad_profile = book.to_vec();
    let bond_start = 14 + 4 * 32 + 2;
    bad_profile[bond_start..bond_start + 16].fill(0);
    malformed_books.push(bad_profile);
    malformed_books.push(book[..book.len() - 1].to_vec());
    for malformed in malformed_books {
        assert!(RoomBook::decode(&malformed).is_err());
        let mut settlement = vec![0; Section::SettlementClaims.payload_cap()];
        let n = shared.encode_room_market(&malformed, &mut settlement)?;
        let candidate = shared.replace_section(Section::SettlementClaims, &settlement[..n])?;
        let mut encoded = vec![0; MAX_STATE_BYTES];
        let mut scratch = vec![0; Section::Control.payload_cap()];
        let n = state::encode_shared_state(&candidate, &mut encoded, &mut scratch)?;
        let availability = feature_availability(&encoded[..n])?;
        assert_eq!(availability[0], Availability::Available);
        assert_eq!(availability[10], Availability::ContentUnavailable);
    }
    Ok(())
}

#[test]
fn f11_errors_are_registered_exported_and_distinct_from_f01_through_f10() -> CodecResult<()> {
    let expected = [
        (F11_ROOM_UNKNOWN, "RoomUnknown"),
        (F11_ROOM_CLOSED, "RoomClosed"),
        (F11_ASK_UNFUNDED, "AskUnfunded"),
        (F11_BID_BELOW_FLOOR, "BidBelowFloor"),
        (F11_ROSTER_DIGEST_MISMATCH, "RosterDigestMismatch"),
        (F11_BATCH_UNSEALED, "BatchUnsealed"),
        (F11_CHALLENGE_WINDOW_CLOSED, "ChallengeWindowClosed"),
        (F11_BOND_LOCKED, "BondLocked"),
        (F11_BOND_INSUFFICIENT, "BondInsufficient"),
        (F11_SPLIT_EXCEEDS_SETTLED, "SplitExceedsSettled"),
    ];
    assert_eq!(
        APPLICATION_ERRORS
            .iter()
            .filter(|e| e.feature == "F11")
            .count(),
        10
    );
    for (index, (error, name)) in expected.iter().enumerate() {
        assert_eq!(error.code(), 0x0B01 + index as u16);
        assert_eq!(ApplicationError::from_code(error.code())?, *error);
        assert_eq!(ApplicationError::named("F11", name)?, *error);
        assert!(!expected[..index]
            .iter()
            .any(|(prior, _)| prior.code() == error.code()));
        for prior in APPLICATION_ERRORS.iter().chain(APPLICATION_ALIASES) {
            if prior.feature != "F11" {
                assert_ne!(prior.code, error.code(), "{} {}", prior.feature, prior.name);
            }
        }
        let result = codec::ApplicationResult::failure(*error, Presence::Absent, 0)?;
        let mut bytes = [0; 256];
        let n = codec::encode_result(&result, &mut bytes)?;
        assert_eq!(codec::decode_result(&bytes[..n])?.error, Some(*error));
    }
    assert_eq!(
        ApplicationError::from_code(0x0B00),
        Err(errors::NON_CANONICAL)
    );
    assert_eq!(
        ApplicationError::from_code(0x0B0B),
        Err(errors::NON_CANONICAL)
    );
    Ok(())
}

fn market_policy() -> CodecResult<MarketPolicy> {
    Ok(MarketPolicy {
        task_policy: TaskPolicyV1::bounded_default(
            1,
            1,
            PolicyCommitments {
                model_artifact: Digest32::new([1; 32])?,
                dataset_artifact: [2; 32],
                benchmark_suite: Digest32::new([3; 32])?,
                rubric: RubricDigest::new([4; 32])?,
                task_schema: Digest32::new([5; 32])?,
                result_schema: Digest32::new([6; 32])?,
                service_terms: Digest32::new([7; 32])?,
            },
            1_000_000,
            100,
        )?,
        room_profile: RoomProfile {
            minimum_bond: (1u128 << 80) + 123,
            canary_ratio_bps: 250,
            lap_size_tokens: (1u64 << 40) + 256,
            challenge_window_blocks: 120,
            slash_fraction_bps: 2_500,
            host_share_bps: 1_000,
            protocol_share_bps: 500,
            settlement_account: AccountId::new([8; 32])?,
            max_generated_tokens: (1u64 << 40) + 4096,
            queue_depth: layerx_programs_ai_market::policy::DEFAULT_QUEUE_DEPTH,
            liveness_timeout_ms: 30_000,
            acknowledgement_window_ms: 5_000,
            canary_relative_tolerance_bps: 100,
        },
    })
}

#[test]
fn f11_market_policy_round_trips_every_room_profile_field() -> CodecResult<()> {
    let policy = market_policy()?;
    assert_eq!(policy.room_profile_ref(), &policy.room_profile);
    let mut encoded = vec![0xa5; MARKET_POLICY_BYTES + 1];
    let n = policy.encode(&mut encoded)?;
    assert_eq!(n, MARKET_POLICY_BYTES);
    assert_eq!(n, 411);
    assert_eq!(encoded[n], 0xa5);
    assert_eq!(&encoded[..2], &MARKET_POLICY_VERSION.to_be_bytes());
    assert_eq!(
        TaskPolicyV1::decode(&encoded[2..2 + TASK_POLICY_BYTES])?,
        policy.task_policy
    );
    let decoded = MarketPolicy::decode(&encoded[..n])?;
    assert_eq!(decoded, policy);
    assert_eq!(decoded.digest()?, policy.digest()?);
    let room = &encoded[2 + TASK_POLICY_BYTES..n];
    assert_eq!(room.len(), ROOM_PROFILE_BYTES);
    assert_eq!(
        &room[..2],
        &layerx_programs_ai_market::policy::ROOM_PROFILE_VERSION.to_be_bytes()
    );
    assert_eq!(
        &room[2..18],
        &policy.room_profile.minimum_bond.to_be_bytes()
    );
    assert_eq!(&room[18..20], &250u16.to_be_bytes());
    assert_eq!(
        &room[20..28],
        &policy.room_profile.lap_size_tokens.to_be_bytes()
    );
    assert_eq!(&room[28..36], &120u64.to_be_bytes());
    assert_eq!(&room[36..38], &2_500u16.to_be_bytes());
    assert_eq!(&room[38..40], &1_000u16.to_be_bytes());
    assert_eq!(&room[40..42], &500u16.to_be_bytes());
    assert_eq!(&room[42..74], &[8; 32]);
    assert_eq!(
        &room[74..82],
        &policy.room_profile.max_generated_tokens.to_be_bytes()
    );
    assert_eq!(
        &room[82..84],
        &policy.room_profile.queue_depth.to_be_bytes()
    );
    assert_eq!(
        &room[84..92],
        &policy.room_profile.liveness_timeout_ms.to_be_bytes()
    );
    assert_eq!(
        &room[92..100],
        &policy.room_profile.acknowledgement_window_ms.to_be_bytes()
    );
    assert_eq!(
        &room[100..102],
        &policy
            .room_profile
            .canary_relative_tolerance_bps
            .to_be_bytes()
    );
    assert_eq!(
        decoded.room_profile.max_generated_tokens,
        policy.room_profile.max_generated_tokens
    );
    assert_eq!(decoded.room_profile.queue_depth, 32);
    assert_eq!(decoded.room_profile.liveness_timeout_ms, 30_000);
    assert_eq!(decoded.room_profile.acknowledgement_window_ms, 5_000);
    assert_eq!(decoded.room_profile.canary_relative_tolerance_bps, 100);
    for field in 0..5 {
        let mut changed = policy;
        match field {
            0 => changed.room_profile.max_generated_tokens += 1,
            1 => changed.room_profile.queue_depth += 1,
            2 => changed.room_profile.liveness_timeout_ms += 1,
            3 => changed.room_profile.acknowledgement_window_ms += 1,
            4 => changed.room_profile.canary_relative_tolerance_bps += 1,
            _ => unreachable!(),
        }
        assert_ne!(changed.digest()?, policy.digest()?, "profile field {field}");
    }
    for end in 0..room.len() {
        assert!(
            RoomProfile::decode(&room[..end]).is_err(),
            "profile prefix {end}"
        );
    }
    let mut trailing = room.to_vec();
    trailing.push(0);
    assert_eq!(RoomProfile::decode(&trailing), Err(errors::NON_CANONICAL));
    for version in [0u16, 1, 3, u16::MAX] {
        let mut unsupported = room.to_vec();
        unsupported[..2].copy_from_slice(&version.to_be_bytes());
        assert_eq!(RoomProfile::decode(&unsupported), Err(errors::BAD_VERSION));
    }
    // The old unversioned profile and old policy cannot acquire default fields.
    assert!(RoomProfile::decode(&room[2..74]).is_err());
    let mut old_policy = encoded[..n].to_vec();
    old_policy[..2].copy_from_slice(&2u16.to_be_bytes());
    assert_eq!(MarketPolicy::decode(&old_policy), Err(errors::BAD_VERSION));
    assert_eq!(RoomProfile::decode(room)?, policy.room_profile);
    let mut changed = policy;
    changed.room_profile.protocol_share_bps += 1;
    assert_ne!(changed.digest()?, policy.digest()?);
    let mut v1 = [0; TASK_POLICY_BYTES];
    policy.task_policy.encode(&mut v1)?;
    assert_eq!(TaskPolicyV1::decode(&v1)?, policy.task_policy);
    assert_ne!(policy.digest()?, policy.task_policy.digest()?);
    for end in 0..n {
        assert!(
            MarketPolicy::decode(&encoded[..end]).is_err(),
            "prefix {end}"
        );
    }
    assert_eq!(MarketPolicy::decode(&encoded), Err(errors::NON_CANONICAL));
    encoded[1] = MARKET_POLICY_VERSION as u8 + 1;
    assert_eq!(
        MarketPolicy::decode(&encoded[..n]),
        Err(errors::BAD_VERSION)
    );
    for size in 0..MARKET_POLICY_BYTES {
        let mut short = vec![0xa5; size];
        assert_eq!(policy.encode(&mut short), Err(errors::CAPACITY));
        assert!(short.iter().all(|b| *b == 0xa5));
    }
    Ok(())
}

#[test]
fn f11_room_profile_refuses_invalid_bounds_and_zero_settlement_account() -> CodecResult<()> {
    let policy = market_policy()?;
    let profile = policy.room_profile;
    for invalid in [
        RoomProfile {
            max_generated_tokens: 0,
            ..profile
        },
        RoomProfile {
            queue_depth: 0,
            ..profile
        },
        RoomProfile {
            liveness_timeout_ms: 0,
            ..profile
        },
        RoomProfile {
            acknowledgement_window_ms: 0,
            ..profile
        },
        RoomProfile {
            canary_relative_tolerance_bps: 10_001,
            ..profile
        },
        RoomProfile {
            minimum_bond: 0,
            ..profile
        },
        RoomProfile {
            canary_ratio_bps: 10_001,
            ..profile
        },
        RoomProfile {
            lap_size_tokens: 0,
            ..profile
        },
        RoomProfile {
            challenge_window_blocks: 0,
            ..profile
        },
        RoomProfile {
            slash_fraction_bps: 10_001,
            ..profile
        },
        RoomProfile {
            host_share_bps: 10_001,
            ..profile
        },
        RoomProfile {
            protocol_share_bps: 10_001,
            ..profile
        },
        RoomProfile {
            host_share_bps: 9_501,
            protocol_share_bps: 500,
            ..profile
        },
        RoomProfile {
            host_share_bps: u16::MAX,
            protocol_share_bps: u16::MAX,
            ..profile
        },
    ] {
        assert_eq!(invalid.validate(), Err(errors::F01_INVALID_POLICY));
        let mut bytes = [0xa5; MARKET_POLICY_BYTES];
        assert_eq!(
            MarketPolicy {
                room_profile: invalid,
                ..policy
            }
            .encode(&mut bytes),
            Err(errors::F01_INVALID_POLICY)
        );
        assert!(bytes.iter().all(|b| *b == 0xa5));
    }
    for ratio in [0, 10_000] {
        let boundary = RoomProfile {
            canary_relative_tolerance_bps: ratio,
            canary_ratio_bps: ratio,
            slash_fraction_bps: ratio,
            host_share_bps: ratio,
            protocol_share_bps: 10_000 - ratio,
            ..profile
        };
        let mut bytes = [0; ROOM_PROFILE_BYTES];
        boundary.encode(&mut bytes)?;
        assert_eq!(RoomProfile::decode(&bytes)?, boundary);
    }
    let mut bytes = [0; ROOM_PROFILE_BYTES];
    profile.encode(&mut bytes)?;
    // Validation must also run on hostile bytes, not just typed producers.
    for (start, end) in [(74, 82), (82, 84), (84, 92), (92, 100)] {
        let mut invalid = bytes;
        invalid[start..end].fill(0);
        assert_eq!(
            RoomProfile::decode(&invalid),
            Err(errors::F01_INVALID_POLICY)
        );
        let mut market_bytes = [0; MARKET_POLICY_BYTES];
        policy.encode(&mut market_bytes)?;
        market_bytes[2 + TASK_POLICY_BYTES..].copy_from_slice(&invalid);
        assert_eq!(
            MarketPolicy::decode(&market_bytes),
            Err(errors::F01_INVALID_POLICY)
        );
    }
    let mut invalid = bytes;
    invalid[100..102].copy_from_slice(&10_001u16.to_be_bytes());
    assert_eq!(
        RoomProfile::decode(&invalid),
        Err(errors::F01_INVALID_POLICY)
    );
    bytes[42..74].fill(0);
    assert_eq!(RoomProfile::decode(&bytes), Err(errors::F01_INVALID_POLICY));
    Ok(())
}
