//! PAXAI schema-1 common contracts and the direct native Programs entrypoint. No codec
//! value establishes host authority or finality.
#![cfg_attr(target_arch = "wasm32", no_std)]

// File modules are mirrored in gateway/tests/repair_22_33_1439.rs and checked by
// tests/ci/test_ai_market_module_mirror.py, including conditional private modules.
pub mod admission;
pub mod aggregation;
pub mod aggregation_codec;
pub mod canary;
pub mod closure;
pub mod codec;
pub mod commit_reveal;
pub mod dispatch;
pub mod epoch;
pub mod errors;
pub mod evidence;
pub mod evaluators {
    pub mod admission;
    pub mod authority;
    pub mod codec;
    pub mod model;
}
#[cfg(target_arch = "wasm32")]
mod guest_cell;
pub mod host_adapter;
pub mod laps;
pub mod policy;
pub mod queries;
pub use queries::placement;
pub mod registry;
pub mod registry_ops;
pub mod reputation;
pub mod reputation_codec;
pub mod reputation_transition;
pub mod reward_math;
pub mod rewards;
pub use rewards::bonds;
pub mod rooms;
pub mod roster;
pub mod state;
pub mod tasks;
pub mod types;
pub mod value_adapter;
pub mod workers;

pub fn merged_chain_domain(chain_id: u64) -> [u8; 32] {
    let mut hash = codec::Sha256State::new();
    hash.update(b"PAXAI/chain-domain/v1");
    hash.update(&chain_id.to_be_bytes());
    hash.finalize()
}

/// Activation identity of the merged chain. Packaging overrides must match this domain.
pub const MERGED_CHAIN_ID: u64 = 125;
pub const MERGED_CHAIN_DOMAIN: [u8; 32] = [
    0x62, 0xb6, 0xb1, 0x47, 0x34, 0xfa, 0x76, 0x9c, 0x3e, 0x0f, 0xbc, 0x94, 0xde, 0x49, 0x5a, 0x3d,
    0x03, 0x8b, 0xe4, 0x9a, 0x25, 0x24, 0xb1, 0xc0, 0x71, 0xd7, 0x5d, 0x0a, 0x4e, 0xed, 0x71, 0xf6,
];

pub fn deployment_chain_domain() -> CodecResult<ChainDomain> {
    deployment_chain_domain_from_setting(option_env!("PAXAI_CHAIN_DOMAIN"))
}

/// Validates a packaging domain against Paxeer chain 125, using its domain when unset.
///
/// # Errors
/// Returns `WRONG_DOMAIN` for a malformed setting or any foreign chain domain.
pub fn deployment_chain_domain_from_setting(setting: Option<&str>) -> CodecResult<ChainDomain> {
    deployment::chain_domain(setting)
}

mod deployment {
    use crate::{errors::WRONG_DOMAIN, ChainDomain, CodecResult};

    const fn nibble(byte: u8) -> Option<u8> {
        match byte {
            b'0'..=b'9' => Some(byte - b'0'),
            b'a'..=b'f' => Some(byte - b'a' + 10),
            _ => None,
        }
    }

    const fn parse_domain(hex: &[u8]) -> Option<[u8; 32]> {
        if hex.len() != 64 {
            return None;
        }
        let mut out = [0; 32];
        let mut i = 0;
        let mut nonzero = false;
        while i < 32 {
            let (Some(high), Some(low)) = (nibble(hex[2 * i]), nibble(hex[2 * i + 1])) else {
                return None;
            };
            out[i] = (high << 4) | low;
            nonzero = nonzero || out[i] != 0;
            i += 1;
        }
        if nonzero {
            Some(out)
        } else {
            None
        }
    }

    fn checked_domain(bytes: Option<[u8; 32]>) -> CodecResult<ChainDomain> {
        ChainDomain::new(bytes.ok_or(WRONG_DOMAIN)?).map_err(|_| WRONG_DOMAIN)
    }

    pub(super) fn chain_domain(setting: Option<&str>) -> CodecResult<ChainDomain> {
        let domain = checked_domain(match setting {
            Some(hex) => parse_domain(hex.as_bytes()),
            None => Some(crate::MERGED_CHAIN_DOMAIN),
        })?;
        if domain.bytes() != crate::MERGED_CHAIN_DOMAIN {
            return Err(WRONG_DOMAIN);
        }
        Ok(domain)
    }

    #[cfg(test)]
    mod tests {
        use super::{chain_domain, checked_domain, parse_domain};
        use crate::{
            codec::{decode_envelope, derive_market, encode_envelope, Envelope},
            dispatch::BeginAggregation,
            errors::{CAPACITY, WRONG_DOMAIN},
            Authentication, ChainDomain, CodecResult, Presence, PrincipalId, ProgramId, RequestId,
            RosterDigest, MAX_ENVELOPE_BYTES, MAX_STATE_BYTES,
        };

        #[test]
        fn deployment_domain_preserves_all_bytes() -> CodecResult<()> {
            const BYTES: [u8; 32] = [
                0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab,
                0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67,
                0x89, 0xab, 0xcd, 0xef,
            ];
            const DOMAIN: Option<[u8; 32]> =
                parse_domain(b"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef");
            assert_eq!(DOMAIN, Some(BYTES));
            assert_eq!(checked_domain(DOMAIN)?.bytes(), BYTES);
            let setting = option_env!("PAXAI_CHAIN_DOMAIN");
            let configured = checked_domain(match setting {
                Some(hex) => parse_domain(hex.as_bytes()),
                None => Some(crate::MERGED_CHAIN_DOMAIN),
            })?;
            if configured.bytes() == crate::merged_chain_domain(crate::MERGED_CHAIN_ID) {
                assert_eq!(chain_domain(setting)?, configured);
            } else {
                assert_eq!(chain_domain(setting), Err(WRONG_DOMAIN));
            }
            Ok(())
        }

        #[test]
        fn deployment_domain_refuses_invalid_build_settings() {
            for len in 0..=65 {
                if len != 64 {
                    assert_eq!(
                        checked_domain(parse_domain(&[b'a'; 65][..len])),
                        Err(WRONG_DOMAIN)
                    );
                }
            }
            for byte in [b'A', b'F', b'g', b' ', 0, 0xff] {
                for position in 0..64 {
                    let mut setting = [b'a'; 64];
                    setting[position] = byte;
                    assert_eq!(checked_domain(parse_domain(&setting)), Err(WRONG_DOMAIN));
                }
            }
            assert_eq!(checked_domain(None), Err(WRONG_DOMAIN));
            assert_eq!(checked_domain(Some([0; 32])), Err(WRONG_DOMAIN));
            assert_eq!(checked_domain(parse_domain(&[b'0'; 64])), Err(WRONG_DOMAIN));
        }

        #[test]
        fn deployment_domain_binds_envelope_bytes_and_refusals() -> CodecResult<()> {
            let chain = checked_domain(parse_domain(&[b'a'; 64]))?;
            let program = ProgramId::new([2; 32])?;
            let market = derive_market(chain, program)?;
            let envelope = Envelope {
                operation: BeginAggregation,
                chain,
                program,
                market,
                actor: PrincipalId::new([3; 32])?,
                epoch: 1,
                config: 1,
                roster: Presence::Present(RosterDigest::new([4; 32])?),
                sequence: 0,
                expiry: 1,
                request: RequestId::new([5; 32])?,
                payload: &[],
                authentication: Authentication::Native,
            };
            let mut bytes = [0; 512];
            let len = encode_envelope(&envelope, &mut bytes)?;
            assert_eq!(&bytes[10..42], chain.as_bytes());
            let admitted = decode_envelope(&bytes[..len])?;
            assert_eq!(
                admitted.envelope.check_domain(chain, program, market),
                Ok(())
            );
            let request = admitted.request_digest()?;
            let other = ChainDomain::new([0xbb; 32])?;
            bytes[10..42].copy_from_slice(other.as_bytes());
            let foreign = decode_envelope(&bytes[..len])?;
            assert_eq!(
                foreign.envelope.check_domain(chain, program, market),
                Err(WRONG_DOMAIN)
            );
            assert_ne!(foreign.request_digest()?, request);
            assert_eq!(MAX_ENVELOPE_BYTES, 16_384);
            assert_eq!(MAX_STATE_BYTES, 245_760);
            let oversized = vec![0; MAX_ENVELOPE_BYTES + 1];
            assert_eq!(decode_envelope(&oversized).err(), Some(CAPACITY));
            Ok(())
        }
    }
}

#[cfg(target_arch = "wasm32")]
mod guest {
    // Deployment qualification must build with release opt-level z and measure the
    // packaged Wasm plus its 104-byte envelope against the 524288-byte ceiling.
    use crate::dispatch::{self, Buffers, Routed, SCRATCH_BYTES};
    use crate::host_adapter::{AdapterError, ApplicationEvent, DirectCall};
    use crate::registry_ops::CallContext;
    use crate::types::{GuestIndex as _, Presence};
    use crate::{ApplicationError, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES};
    use layerx_program_sdk::{
        CallResult, EntryResponse, ProgramRefusal, RefusalClass, RefusalReason,
    };

    layerx_program_sdk::trap_on_panic!();
    layerx_program_sdk::failure_entrypoint!(handle);

    struct Arena {
        current: [u8; MAX_STATE_BYTES],
        next: [u8; MAX_STATE_BYTES],
        scratch: [u8; SCRATCH_BYTES],
        event: [u8; MAX_EVENT_BYTES],
        result: [u8; MAX_RESULT_BYTES],
    }

    static ARENA: crate::guest_cell::GuestCell<Arena> = crate::guest_cell::GuestCell::new(Arena {
        current: [0; MAX_STATE_BYTES],
        next: [0; MAX_STATE_BYTES],
        scratch: [0; SCRATCH_BYTES],
        event: [0; MAX_EVENT_BYTES],
        result: [0; MAX_RESULT_BYTES],
    });

    const EMPTY_REFUSAL: ProgramRefusal<'static> = match RefusalReason::new(&[]) {
        Ok(reason) => match ProgramRefusal::new(RefusalClass::Rejected, reason) {
            Ok(refusal) => refusal,
            Err(_) => panic!(),
        },
        Err(_) => panic!(),
    };

    fn rejected(reason: &'static [u8]) -> ProgramRefusal<'static> {
        RefusalReason::new(reason)
            .and_then(|r| ProgramRefusal::new(RefusalClass::Rejected, r))
            .unwrap_or(EMPTY_REFUSAL)
    }

    fn failure(
        error: ApplicationError,
        result: &'static mut [u8; MAX_RESULT_BYTES],
    ) -> ProgramRefusal<'static> {
        match dispatch::failure_frame(error, Presence::Absent, 0, result) {
            Ok(n) => rejected(result.guest_index(..n)),
            Err(_) => EMPTY_REFUSAL,
        }
    }

    fn handle(input: &[u8]) -> Result<EntryResponse<'static>, ProgramRefusal<'static>> {
        let arena = ARENA.try_borrow_mut().ok_or(EMPTY_REFUSAL)?.leak();
        let Arena {
            current,
            next,
            scratch,
            event,
            result,
        } = arena;
        let call = match DirectCall::read_context() {
            Ok(call) => call,
            Err(AdapterError::Application(error)) => return Err(failure(error, result)),
            Err(AdapterError::Host(_)) => return Err(EMPTY_REFUSAL),
        };
        let chain = match crate::deployment_chain_domain() {
            Ok(chain) => chain,
            Err(error) => return Err(failure(error, result)),
        };
        let ctx = CallContext {
            chain,
            program: call.program(),
            principal: call.principal(),
            height: call.height(),
        };
        let current_len = match call.read_state(current) {
            Ok(Some(state)) => match state.encoded_len() {
                Ok(n) if n <= current.len() => Some(n),
                Ok(_) => return Err(failure(crate::errors::NON_CANONICAL, result)),
                Err(error) => return Err(failure(error, result)),
            },
            Ok(None) => None,
            Err(AdapterError::Application(error)) => return Err(failure(error, result)),
            Err(AdapterError::Host(_)) => return Err(EMPTY_REFUSAL),
        };
        let routed = dispatch::route(
            &ctx,
            input,
            current_len.map(|n| current.guest_index(..n)),
            Buffers {
                next,
                scratch,
                event,
                result,
            },
        );
        match routed {
            Ok(Routed::Applied {
                operation,
                state_len,
                event_len,
                result_len,
                ..
            }) => {
                if state_len > next.len() || event_len > event.len() || result_len > result.len() {
                    return Err(failure(crate::errors::NON_CANONICAL, result));
                }
                if matches!(
                    operation,
                    dispatch::ChallengeLap | dispatch::ResolveChallenge
                ) {
                    let Some(n) = current_len else {
                        return Err(EMPTY_REFUSAL);
                    };
                    match crate::canary::stage_value(
                        operation,
                        current.guest_index(..n),
                        next.guest_index(..state_len),
                    ) {
                        Ok(()) => {}
                        Err(AdapterError::Application(error)) => {
                            return Err(failure(error, result))
                        }
                        Err(AdapterError::Host(_)) => return Err(EMPTY_REFUSAL),
                    }
                }
                let events = [ApplicationEvent {
                    operation,
                    body: event.guest_index(..event_len),
                }];
                match call.stage_state_and_events(next.guest_index(..state_len), &events) {
                    Ok(()) => Ok(EntryResponse::new(
                        CallResult::OK,
                        result.guest_index(..result_len),
                    )),
                    Err(AdapterError::Application(error)) => Err(failure(error, result)),
                    Err(AdapterError::Host(_)) => Err(EMPTY_REFUSAL),
                }
            }
            Ok(Routed::Unchanged { result_len }) => {
                if result_len > result.len() {
                    return Err(failure(crate::errors::NON_CANONICAL, result));
                }
                Ok(EntryResponse::new(
                    CallResult::OK,
                    result.guest_index(..result_len),
                ))
            }
            Ok(Routed::Refused { result_len }) => {
                if result_len > result.len() {
                    return Err(failure(crate::errors::NON_CANONICAL, result));
                }
                Err(rejected(result.guest_index(..result_len)))
            }
            Err(error) => Err(failure(error, result)),
        }
    }
}

pub use errors::{
    ApplicationError, CodecResult, F11_ASK_UNFUNDED, F11_BATCH_UNSEALED, F11_BID_BELOW_FLOOR,
    F11_BOND_INSUFFICIENT, F11_BOND_LOCKED, F11_CHALLENGE_WINDOW_CLOSED, F11_ROOM_CLOSED,
    F11_ROOM_UNKNOWN, F11_ROSTER_DIGEST_MISMATCH, F11_SPLIT_EXCEEDS_SETTLED,
};
pub use policy::{MarketPolicy, RoomProfile};
pub use types::*;

pub const SCHEMA_VERSION: u16 = 1;
pub const SHARED_STATE_KEY: &[u8] = b"paxai/state/v1";
pub const REWARDS_ACCOUNT_SEED: &[u8] = b"paxai/rewards/v1";
pub const MAX_ENVELOPE_BYTES: usize = 16_384;
pub const MAX_PAYLOAD_BYTES: usize = 15_965;
pub const MAX_RESULT_BYTES: usize = 16_384;
pub const RESULT_HEADER_BYTES: usize = 82;
pub const MAX_RESULT_PAYLOAD_BYTES: usize = 16_302;
pub const MAX_STATE_BYTES: usize = 245_760;
const _: () = assert!(MAX_STATE_BYTES <= layerx_program_sdk::MAX_STORAGE_VALUE_BYTES);
pub const MAX_EVENT_BYTES: usize = 2_048;
pub const MAX_EVENTS: usize = 2;
pub const MAX_WORKERS: usize = 32;
pub const MAX_EVALUATORS: usize = 8;
pub const MAX_TASKS: usize = 64;
pub const MAX_RETAINED_EPOCHS: usize = 32;
pub const MAX_PAYOUT_IDENTITIES: usize = 256;
pub const MAX_CHUNK_BYTES: usize = 8_192;

#[cfg(test)]
extern crate self as layerx_programs_ai_market;
#[cfg(test)]
#[path = "../tests/support/repair_478_fixture.rs"]
mod repair_478_fixture;

#[cfg(test)]
mod repair478_fund_tests {
    use crate::{
        codec::{self, Envelope},
        errors::{WRONG_EPOCH, WRONG_ROSTER},
        registry_ops::CallContext,
        rewards::*,
        state::{decode_shared_state, encode_shared_state, ActorSlot, Section},
        types::*,
        MAX_STATE_BYTES,
    };
    #[test]
    fn repair478_fund_head_carries_populated_regions_and_epoch_binding() {
        use super::*;
        use crate::{
            admission::AdmissionTable,
            aggregation_codec::{EpochAggregation, QualityStatus, WorkerAggregate},
            registry::market_clock,
            registry_ops::PolicySection,
            reputation_transition,
            reward_math::allocate,
            types::{PublicKey32, RequestId},
        };
        let f = crate::repair_478_fixture::fixture([12; 32], [13; 32]).unwrap();
        let mut state = decode_shared_state(&f.next).unwrap();
        let source = state.feature_sections[Section::SettlementClaims.index()];
        let rewards = decode_reward_state(source).unwrap();
        let mut refunded = vec![0; REWARD_STATE_BYTES];
        rewards
            .refund_free(
                FundingPhase::Closing,
                &RefundRequest {
                    expected_refunded: 0,
                    amount: 10,
                    recipient: rewards.ledger().unwrap().refund_recipient,
                },
                RequestDigest::new([21; 32]).unwrap(),
                ResultDigest::new([22; 32]).unwrap(),
                &mut refunded,
            )
            .unwrap();
        let worker = WorkerRosterEntry {
            worker: WorkerId::new([23; 32]).unwrap(),
            owner: PrincipalId::new([12; 32]).unwrap(),
            recipient: AccountId::new([24; 32]).unwrap(),
            generation: Version::new(1).unwrap(),
            key_version: Version::new(1).unwrap(),
            public_key: PublicKey32([25; 32]),
            metadata: crate::types::MetadataDigest::new([26; 32]).unwrap(),
        };
        let roster = RosterDigest::new([27; 32]).unwrap();
        let header =
            PolicySection::decode(state.feature_sections[Section::PolicyLifecycle.index()])
                .unwrap()
                .header;
        let epoch = 7;
        let opening_height = header.origin_height + epoch * 128;
        let height = opening_height + 127;
        assert_eq!(
            market_clock(header.origin_height, height).unwrap().epoch,
            epoch
        );
        let mut reserved = vec![0; REWARD_STATE_BYTES];
        decode_reward_state(&refunded)
            .unwrap()
            .reserve_epoch(epoch, 40, roster, &[worker], opening_height, &mut reserved)
            .unwrap();
        let binding = FrozenBinding {
            chain: header.deployment_chain_domain,
            program: header.program_id,
            market: header.market_id,
            epoch,
            config: Version::new(header.active_config_version).unwrap(),
            roster,
        };
        let output = WorkerAggregate::new(
            worker.worker,
            worker.generation,
            3,
            QualityStatus::ScoredPositive,
            1,
            1,
        )
        .unwrap();
        let outputs = [output];
        let aggregation = EpochAggregation::structural(
            binding,
            Digest32::new([29; 32]).unwrap(),
            &[worker],
            &outputs,
        )
        .unwrap();
        let allocation = allocate(40, &[output]).unwrap();
        let mut settled = vec![0; REWARD_STATE_BYTES];
        decode_reward_state(&reserved)
            .unwrap()
            .terminalize(
                &binding,
                aggregation.root(),
                &allocation,
                &[worker],
                height,
                &mut settled,
            )
            .unwrap();
        // The real F06 producers retain a populated dictionary, terminal row
        // and refund cursor; the strict decoder proves all of their bindings.
        let strict = decode_reward_state(&settled).unwrap();
        assert_eq!(strict.ledger().unwrap().retained_epochs, 1);
        assert!(!strict.ledger().unwrap().active_reserve);
        assert_eq!(strict.row(epoch).unwrap().status, EpochStatus::Terminal);
        assert_eq!(strict.dictionary().occupied(), 1);
        assert_eq!(strict.row(7).unwrap().roster, roster);
        assert!(strict.last_refund().unwrap().is_some());
        let mut table = AdmissionTable::new();
        table.begin_epoch(epoch);
        let mut table_bytes = vec![0; 1024];
        let n = table.encode(&mut table_bytes).unwrap();
        let (region, _) = reputation_transition::split_joint(
            state.feature_sections[Section::ReputationAdmission.index()],
        )
        .unwrap();
        let mut joint = vec![0; Section::ReputationAdmission.payload_cap()];
        let n =
            reputation_transition::encode_joint(&region.unwrap(), &table_bytes[..n], &mut joint)
                .unwrap();
        state.feature_sections[Section::ReputationAdmission.index()] = &joint[..n];
        state.feature_sections[Section::SettlementClaims.index()] = &settled;
        let mut current = vec![0; MAX_STATE_BYTES];
        let mut control = vec![0; Section::Control.payload_cap()];
        let n = encode_shared_state(&state, &mut current, &mut control).unwrap();
        current.truncate(n);
        let committed = decode_shared_state(&current).unwrap();
        let (_, admission) = reputation_transition::split_joint(
            committed.feature_sections[Section::ReputationAdmission.index()],
        )
        .unwrap();
        assert_eq!(
            AdmissionTable::decode(admission).unwrap().current_epoch(),
            Some(epoch)
        );
        let decoded = codec::decode_envelope(&f.fund).unwrap();
        let mut payload = vec![0; FUND_PAYLOAD_BYTES];
        FundRequest {
            amount: 25,
            refund_recipient: strict.ledger().unwrap().refund_recipient,
            policy_version: FUNDING_POLICY_VERSION,
            consent: true,
        }
        .encode(&mut payload)
        .unwrap();
        let envelope = Envelope {
            epoch,
            expiry: height + 1000,
            roster: Presence::Present(roster),
            sequence: 3,
            request: RequestId::new([28; 32]).unwrap(),
            payload: &payload,
            ..decoded.envelope
        };
        let ctx = CallContext {
            chain: envelope.chain,
            program: envelope.program,
            principal: envelope.actor,
            height,
        };
        let mut call = vec![0; crate::MAX_ENVELOPE_BYTES];
        let n = codec::encode_envelope(&envelope, &mut call).unwrap();
        call.truncate(n);
        let mut next = vec![0xa5; MAX_STATE_BYTES];
        let mut scratch = vec![0; SCRATCH_BYTES];
        let mut event = vec![0; crate::MAX_EVENT_BYTES];
        let Outcome::Applied {
            state_len,
            before,
            after,
            ..
        } = apply(
            &ctx,
            &codec::decode_envelope(&call).unwrap(),
            &current,
            &mut next,
            &mut scratch,
            &mut event,
        )
        .unwrap()
        else {
            panic!("populated Fund must apply");
        };
        assert_eq!(after, before.deposit(25).unwrap());
        let produced = decode_shared_state(&next[..state_len]).unwrap();
        let funded =
            decode_reward_state(produced.feature_sections[Section::SettlementClaims.index()])
                .unwrap();
        assert_eq!(funded.ledger().unwrap(), after);
        assert_eq!(&funded.bytes()[LEDGER_BYTES..], &settled[LEDGER_BYTES..]);
        for section in [
            Section::IdentityRoster,
            Section::CurrentReports,
            Section::ReputationAdmission,
        ] {
            assert_eq!(
                produced.feature_sections[section.index()],
                state.feature_sections[section.index()]
            );
        }
        assert_eq!(produced.revision, state.revision + 1);
        assert_eq!(
            produced
                .control
                .replay
                .actor(ActorSlot::OWNER)
                .unwrap()
                .last
                .unwrap()
                .sequence,
            3
        );
        // The binding checks still reject the wrong epoch or roster before any write.
        for (epoch, binding, error) in [
            (6, Presence::Present(roster), WRONG_EPOCH),
            (
                7,
                Presence::Present(RosterDigest::new([30; 32]).unwrap()),
                WRONG_ROSTER,
            ),
        ] {
            let bad = Envelope {
                epoch,
                roster: binding,
                ..envelope
            };
            let n = codec::encode_envelope(&bad, &mut call).unwrap();
            next.fill(0xa5);
            assert_eq!(
                apply(
                    &ctx,
                    &codec::decode_envelope(&call[..n]).unwrap(),
                    &current,
                    &mut next,
                    &mut scratch,
                    &mut event
                ),
                Err(error)
            );
            assert!(next.iter().all(|byte| *byte == 0xa5));
        }
        let missing_roster = Envelope {
            roster: Presence::Absent,
            ..envelope
        };
        assert_eq!(
            codec::encode_envelope(&missing_roster, &mut call),
            Err(WRONG_ROSTER)
        );
    }
}
