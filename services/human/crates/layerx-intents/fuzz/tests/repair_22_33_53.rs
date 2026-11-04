//! Task 22.33.53, audit 22.30.106 finding 15: proved by
//! `finding_15_retired_fixtures_pass_the_real_fuzz_oracle`, with malformed and
//! live-path regression cases below. No out-of-path wiring is required.
//! Baseline: this test target did not exist; no pre-change gate was run because
//! the ordered qualification contract requires implementation before gates.

#[path = "../fuzz_targets/intent_harness.rs"]
mod intent_harness;

use layerx_intents::{
    compile, inspect_intent, Intent, IntentHeader, IntentKind, IntentKindTag, LxpSend, RejectReason,
};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, NetworkId, ProtocolVersion, PublicKey, SendAuthorization,
    SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload};
use layerx_types::result::KnownResult;
use layerx_wire::encode::Encoder;
use layerx_wire::WireError;

fn assert_rejection(input: &[u8], reason: RejectReason) {
    let rejected = inspect_intent(input).expect_err("invalid envelope accepted");
    assert_eq!(rejected.reason(), reason);
    assert_eq!(rejected.input(), input);
    assert_eq!(rejected.input().as_ptr(), input.as_ptr());
    intent_harness::fuzz_intent(input);
}

#[test]
fn finding_15_retired_fixtures_pass_the_real_fuzz_oracle() {
    let fixtures: [(u16, u16, &[u8]); 8] = [
        (
            1,
            4,
            include_bytes!("../fuzz_targets/fixtures/retired-v1-kind-4"),
        ),
        (
            1,
            11,
            include_bytes!("../fuzz_targets/fixtures/retired-v1-kind-11"),
        ),
        (
            1,
            12,
            include_bytes!("../fuzz_targets/fixtures/retired-v1-kind-12"),
        ),
        (
            1,
            16,
            include_bytes!("../fuzz_targets/fixtures/retired-v1-kind-16"),
        ),
        (
            2,
            4,
            include_bytes!("../fuzz_targets/fixtures/retired-v2-kind-4"),
        ),
        (
            2,
            11,
            include_bytes!("../fuzz_targets/fixtures/retired-v2-kind-11"),
        ),
        (
            2,
            12,
            include_bytes!("../fuzz_targets/fixtures/retired-v2-kind-12"),
        ),
        (
            2,
            16,
            include_bytes!("../fuzz_targets/fixtures/retired-v2-kind-16"),
        ),
    ];
    for (version, kind, fixture) in fixtures {
        let mut encoder = Encoder::new(8);
        encoder.u16(version).expect("encode version");
        encoder.u16(kind).expect("encode kind");
        encoder
            .fixed(&[0xde, 0xad, 0, 0xff])
            .expect("encode opaque tail");
        let encoded = encoder.finish();
        assert_eq!(fixture, &encoded[..4]);
        for input in [fixture, encoded.as_slice()] {
            assert_rejection(input, RejectReason::RetiredShape);
        }
    }
}

#[test]
fn finding_15_malformed_version_and_kind_checks_remain_strict() {
    for version in [1_u16, 2] {
        let mut encoder = Encoder::new(4);
        encoder.u16(version).expect("encode version");
        encoder.u16(4).expect("encode retired kind");
        let bytes = encoder.finish();
        for end in 0..4 {
            assert_rejection(
                &bytes[..end],
                RejectReason::Malformed(WireError {
                    result: KnownResult::Truncated.into(),
                    offset: if end < 2 { 0 } else { 2 },
                }),
            );
        }
    }
    for version in [0, 2, 3, u16::MAX] {
        let mut encoder = Encoder::new(6);
        encoder.u16(version).expect("encode version");
        encoder
            .u16(IntentKindTag::LxpSend as u16)
            .expect("encode live kind");
        encoder.fixed(&[0xaa, 0xbb]).expect("encode tail");
        assert_rejection(&encoder.finish(), RejectReason::UnknownVersion(version));
    }
    for version in [1, 2] {
        for kind in [0, 18, 255, u16::MAX] {
            let mut encoder = Encoder::new(5);
            encoder.u16(version).expect("encode version");
            encoder.u16(kind).expect("encode kind");
            encoder.u8(0xcc).expect("encode tail");
            assert_rejection(&encoder.finish(), RejectReason::UnknownKind(kind));
        }
    }
}

#[test]
fn finding_15_live_headers_and_compilation_reach_the_real_consumer() {
    for kind in [
        IntentKindTag::DidRegistration,
        IntentKindTag::KeyRotation,
        IntentKindTag::RecoveryRegistration,
        IntentKindTag::LxpSend,
        IntentKindTag::LxpReceive,
        IntentKindTag::PayerGrantRegistration,
        IntentKindTag::BudgetCreate,
        IntentKindTag::BudgetFund,
        IntentKindTag::BudgetDefund,
        IntentKindTag::SessionGrant,
        IntentKindTag::SessionRevoke,
        IntentKindTag::AuthorityGrant,
        IntentKindTag::NativeReceive,
    ] {
        let header = kind.header_bytes();
        assert_eq!(
            inspect_intent(&header),
            Ok(IntentHeader { version: 1, kind })
        );
        intent_harness::fuzz_intent(&header);
    }

    let activity = ActivityType::new(ModuleId::Asset, 5).expect("asset send activity");
    let registration = ModuleRegistration::new(ModuleId::Asset, &[activity]).expect("registration");
    let registry = ModuleRegistry::new(&[registration]).expect("registry");
    for selector in [0_u8, 1, 127, 255] {
        let send = LxpSend::new(
            AccountId::parse("agent:did:layerx:fuzz:main").expect("sender"),
            AccountId::parse("agent:did:layerx:fuzz-recipient:main").expect("recipient"),
            AssetId::new([2; 32]),
            Amount::from_u128(u128::from(selector) + 1),
            Sequence::from_u64(u64::from(selector)),
            IdempotencyKey::new([selector; 32]),
            TimestampSeconds::from_u64(1),
            ContextHash::new([3; 32]),
            SendAuthorization::new(
                SendAuthorizationKind::Owner,
                PublicKey::new([4; 32]),
                AuthorizationSignature::new([5; 64]),
            ),
            NetworkId::new(1).expect("network"),
            ProtocolVersion::new(layerx_wire::limits::PROTOCOL_VERSION).expect("protocol"),
        )
        .expect("valid live send");
        let intent = Intent::v1(IntentKind::LxpSend(send));
        let compiled = compile(&intent, &registry).expect("compile live send");
        let transport = compiled.payload().as_bytes().to_vec();
        let received = Payload::new(&registry, activity, &transport).expect("received payload");
        let disclosure = compiled
            .verify_received_payload(&intent, &received)
            .expect("decode live send");
        assert_eq!(disclosure.canonical_payload(), transport);
        assert_eq!(disclosure.activity_type(), activity);
        assert_eq!(disclosure.payload_hash(), compiled.payload_hash());
        intent_harness::fuzz_intent(&transport);
        intent_harness::fuzz_intent(&[selector; 32]);

        for end in [0, 1, transport.len() - 1] {
            let truncated =
                Payload::new(&registry, activity, &transport[..end]).expect("bounded bytes");
            assert!(compiled
                .verify_received_payload(&intent, &truncated)
                .is_err());
        }
        let mut corrupted = transport;
        corrupted[0] ^= 1;
        let corrupted = Payload::new(&registry, activity, &corrupted).expect("bounded bytes");
        assert!(compiled
            .verify_received_payload(&intent, &corrupted)
            .is_err());
    }
}
