use std::collections::BTreeSet;
use std::fs;
use std::path::PathBuf;

use layerx_crypto::payments::{Payment, TransferLeg};
use layerx_types::intent::ProgramId;
use layerx_types::json::{decode_hex, parse};
use layerx_types::limits::MAX_PAYLOAD_BYTES;
use layerx_types::payload::{ActivityType, ModuleId, Payload};
use layerx_types::program_call::{NativeProgramCall, Resources};
use layerx_types::program_lifecycle::{
    NativeProgramAccountProfile2, NativeProgramWindDown, ProgramWindDownOperation,
};
use layerx_wire::{activity, encode::Encoder};

#[path = "../fuzz_targets/payload_codec.rs"]
mod payload_codec;
#[path = "../fuzz_targets/support.rs"]
mod support;

fn corpus_directory() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("fuzz_targets/payload_corpus")
}

fn seed(name: &str) -> Vec<u8> {
    fs::read(corpus_directory().join(name)).expect("committed payload corpus seed")
}

fn produce(kind: ActivityType, bytes: &[u8]) -> Vec<u8> {
    let payload =
        Payload::new(support::registry(), kind, bytes).expect("registered producer payload");
    let mut encoder = Encoder::new(support::MAX_FUZZ_INPUT);
    encoder
        .u32(payload.activity_type().value())
        .expect("activity tag");
    encoder
        .bytes(payload.as_bytes(), MAX_PAYLOAD_BYTES)
        .expect("bounded payload");
    encoder.finish()
}

fn producer_envelopes() -> Vec<(&'static str, Vec<u8>)> {
    include_str!("../fuzz_targets/payload_producers.lxv")
        .lines()
        .map(|line| {
            let (name, hex) = line
                .split_once('|')
                .expect("recorded codec producer transcript");
            (
                name,
                decode_hex(&format!("0x{hex}")).expect("canonical producer hex"),
            )
        })
        .collect()
}

#[test]
fn fuzz_module_registry_matches_current_production_registrations() {
    let registry = support::registry();
    let expected: BTreeSet<_> = producer_envelopes()
        .into_iter()
        .map(|(_, bytes)| {
            activity::peek_signed_activity_type(&bytes).expect("producer activity tag")
        })
        .collect();
    let registered: BTreeSet<_> = registry
        .registrations()
        .iter()
        .flat_map(|r| r.activity_types().iter().copied())
        .collect();
    assert_eq!(
        registered, expected,
        "frozen corpus must cover every production registration exactly"
    );
    assert_eq!(registry.registrations().len(), 9);
    assert_eq!(expected.len(), 78);
    for module in ModuleId::ALL {
        for ordinal in 1..=u16::MAX {
            let kind = ActivityType::new(module, ordinal).expect("nonzero ordinal");
            assert_eq!(
                registry.declares(kind),
                expected.contains(&kind),
                "{module:?}/{ordinal}"
            );
        }
    }
    for ordinal in [6, 7, 8, 9] {
        assert!(!registry.declares(ActivityType::new(ModuleId::Asset, ordinal).unwrap()));
    }
    // Programs has a reserved gap at 8; genesis currently selects ABI 4,
    // rather than every activity constant that happens to exist in its header.
    for ordinal in [8, 11] {
        assert!(!registry.declares(ActivityType::new(ModuleId::Programs, ordinal).unwrap()));
    }
    for (module, ordinal) in [
        (ModuleId::Asset, 10),
        (ModuleId::Asset, 11),
        (ModuleId::Budget, 10),
        (ModuleId::Programs, 10),
        (ModuleId::Spot, 5),
    ] {
        assert!(registry.declares(ActivityType::new(module, ordinal).unwrap()));
    }
}

#[test]
fn payload_fuzz_coverage_canonical_producer_corpus() {
    let mut observed = BTreeSet::new();
    for (name, bytes) in producer_envelopes() {
        let activity = activity::decode_signed(&bytes, support::registry())
            .expect("real wire consumer admits producer envelope");
        assert_eq!(activity::encode_signed(&activity).unwrap(), bytes);
        let input = produce(activity.activity_type(), activity.payload());
        assert_eq!(support::input_bytes(&seed(name)).as_ref(), input);
        let decoded = payload_codec::payload_decode_case(&input)
            .expect("producer payload reaches successful fuzz decoding");
        assert_eq!(decoded.activity_type(), activity.activity_type());
        assert_eq!(decoded.as_bytes(), activity.payload());
        let module = match decoded {
            Payload::Asset(..) => ModuleId::Asset,
            Payload::Escrow(..) => ModuleId::Escrow,
            Payload::Budget(..) => ModuleId::Budget,
            Payload::Stream(..) => ModuleId::Stream,
            Payload::Service(..) => ModuleId::Service,
            Payload::Perps(..) => ModuleId::Perps,
            Payload::Governance(..) => ModuleId::Governance,
            Payload::Programs(..) => ModuleId::Programs,
            Payload::Spot(..) => ModuleId::Spot,
            Payload::Bridge(..) | Payload::Web(..) => panic!("unregistered module was admitted"),
        };
        observed.insert(module);
    }
    assert_eq!(
        observed,
        support::registry()
            .registrations()
            .iter()
            .map(|r| r.module())
            .collect()
    );
    let mut paths: Vec<_> = fs::read_dir(corpus_directory())
        .unwrap()
        .map(|p| p.unwrap().path())
        .collect();
    paths.sort();
    assert_eq!(paths.len(), 106, "all recorded seeds must execute");
    for path in paths {
        let input = fs::read(&path).unwrap();
        let decoded = payload_codec::payload_decode_case(&input)
            .unwrap_or_else(|| panic!("{} rejected", path.display()));
        let raw = support::input_bytes(&input);
        assert_eq!(
            produce(decoded.activity_type(), decoded.as_bytes()),
            raw.as_ref()
        );
        assert_eq!(
            payload_codec::payload_decode_case(raw.as_ref()),
            Some(decoded)
        );
    }
}

#[test]
fn fuzz_module_registry_later_asset_and_programs_producers() {
    let cases = [
        (
            "asset-mint",
            Payment::Mint {
                asset: [3; 32],
                to: [2; 32],
                amount: 25,
            },
        ),
        (
            "asset-burn",
            Payment::Burn {
                asset: [3; 32],
                from: [1; 32],
                amount: 25,
            },
        ),
        (
            "program-transfer",
            Payment::ProgramTransfer {
                program: [9; 32],
                legs: vec![
                    TransferLeg {
                        from: [1; 32],
                        asset: [3; 32],
                        to: [2; 32],
                        amount: 25,
                    },
                    TransferLeg {
                        from: [2; 32],
                        asset: [4; 32],
                        to: [1; 32],
                        amount: 30,
                    },
                ],
            },
        ),
        (
            "program-account",
            Payment::ProgramAccount {
                program: [9; 32],
                asset: [3; 32],
                seed: b"seed".to_vec(),
            },
        ),
    ];
    for (name, expected) in cases {
        let (module, ordinal) = expected.activity_type();
        let kind = ActivityType::new(module, ordinal).unwrap();
        let bytes = expected
            .encode(b"did:lx:fuzz")
            .expect("real payment producer");
        let input = produce(kind, &bytes);
        assert_eq!(support::input_bytes(&seed(name)).as_ref(), input);
        let decoded = payload_codec::payload_decode_case(&input)
            .expect("modern operation reaches payload consumer");
        assert!(payload_codec::typed_payload_round_trip(&decoded));
        assert_eq!(
            Payment::decode(module, ordinal, decoded.as_bytes(), b"did:lx:fuzz").unwrap(),
            expected
        );
        for length in 0..input.len() {
            assert!(payload_codec::payload_decode_case(&input[..length]).is_none());
        }
    }
}

#[test]
fn payload_fuzz_coverage_real_trading_codecs() {
    let vectors = parse(include_str!(
        "../../../../tests/fixtures/trading-payloads/vectors.json"
    ))
    .unwrap();
    let mut seen = BTreeSet::new();
    for vector in vectors.array_at("vectors").unwrap() {
        let name = vector.str_at("name").unwrap();
        let kind =
            ActivityType::from_u32(u32::try_from(vector.u64_at("activity_type").unwrap()).unwrap())
                .unwrap();
        let bytes = decode_hex(&format!("0x{}", vector.str_at("bytes").unwrap())).unwrap();
        let input = produce(kind, &bytes);
        assert_eq!(support::input_bytes(&seed(name)).as_ref(), input);
        let decoded = payload_codec::payload_decode_case(&input)
            .expect("kernel codec fixture reaches fuzz payload codec");
        assert!(
            payload_codec::typed_payload_round_trip(&decoded),
            "{name} must execute its real module decoder"
        );
        seen.insert(kind);
    }
    assert_eq!(seen.len(), 16, "every perps and spot activity is exercised");
}

#[test]
fn payload_fuzz_coverage_native_program_producers() {
    let program_id = ProgramId::new([9; 32]);
    let call = NativeProgramCall {
        program_id,
        guest_abi: layerx_types::guest_abi::MAX_VERSION,
        entrypoint: b"run",
        calldata: b"fixture",
        capabilities: &[],
        access_declaration: &[],
        response_capacity: 64,
        resources: Resources([1; 7]),
    };
    let account = NativeProgramAccountProfile2 {
        program_id,
        asset: [3; 32],
        seed: b"fixture",
    };
    let wind_down = NativeProgramWindDown {
        program_id,
        operation: ProgramWindDownOperation::Tombstone,
    };
    for (ordinal, bytes) in [
        (3, call.encode().unwrap()),
        (6, account.encode().unwrap()),
        (7, wind_down.encode().unwrap()),
    ] {
        let input = produce(
            ActivityType::new(ModuleId::Programs, ordinal).unwrap(),
            &bytes,
        );
        let decoded = payload_codec::payload_decode_case(&input)
            .expect("native Programs producer reaches fuzz consumer");
        assert_eq!(decoded.as_bytes(), bytes);
        assert!(payload_codec::typed_payload_round_trip(&decoded));
    }
}

#[test]
fn payload_fuzz_coverage_refuses_invalid_tags_lengths_and_trailing_bytes() {
    let input = support::input_bytes(&seed("asset-mint")).into_owned();
    for raw in [
        0,
        0x0001_0000,
        0x0001_0006,
        0x0001_0009,
        0x0008_0001,
        0x0009_0008,
        0x0009_000b,
        0x000b_0001,
        0x000c_0001,
        u32::MAX,
    ] {
        let mut changed = input.clone();
        changed[..4].copy_from_slice(&raw.to_be_bytes());
        assert!(
            payload_codec::payload_decode_case(&changed).is_none(),
            "invalid activity {raw:#x}"
        );
    }
    let mut trailing = input.clone();
    trailing.push(0);
    assert!(payload_codec::payload_decode_case(&trailing).is_none());
    let kind = ActivityType::new(ModuleId::Asset, 10).unwrap();
    let at_limit = produce(kind, &vec![0x5a; MAX_PAYLOAD_BYTES]);
    assert_eq!(
        payload_codec::payload_decode_case(&at_limit)
            .unwrap()
            .as_bytes()
            .len(),
        MAX_PAYLOAD_BYTES
    );
    let mut too_long = Encoder::new(support::MAX_FUZZ_INPUT);
    too_long.u32(kind.value()).unwrap();
    too_long
        .bytes(&vec![0x5a; MAX_PAYLOAD_BYTES + 1], MAX_PAYLOAD_BYTES + 1)
        .unwrap();
    assert!(payload_codec::payload_decode_case(&too_long.finish()).is_none());
    let mut forged_length = input;
    forged_length[4..8].copy_from_slice(&u32::MAX.to_be_bytes());
    assert!(payload_codec::payload_decode_case(&forged_length).is_none());
    for malformed_hex in [
        b"hex:0".as_slice(),
        b"hex:gg",
        b"hex:0001000a00000000\nextra",
    ] {
        assert!(payload_codec::payload_decode_case(malformed_hex).is_none());
    }
}

#[test]
fn payload_fuzz_coverage_native_kernel_codec_fixtures() {
    for name in [
        "deploy",
        "upgrade",
        "call",
        "wind-down-route",
        "wind-down-deprecate",
        "wind-down-tombstone",
        "wind-down-exit",
    ] {
        let path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("../../../sdk/platform/conformance/fixtures")
            .join(format!("native-program-{name}-v3.json"));
        let document =
            parse(&fs::read_to_string(path).expect("committed native kernel codec fixture"))
                .unwrap();
        let bytes = decode_hex(&format!("0x{}", document.str_at("payload_hex").unwrap())).unwrap();
        let ordinal = u16::try_from(document.u64_at("ordinal").unwrap()).unwrap();
        let kind = ActivityType::new(ModuleId::Programs, ordinal).unwrap();
        let signed = decode_hex(&format!(
            "0x{}",
            document.str_at("signed_activity_hex").unwrap()
        ))
        .unwrap();
        let envelope = activity::decode_signed(&signed, support::registry())
            .expect("real wire consumer reads kernel producer envelope");
        assert_eq!(envelope.activity_type(), kind);
        assert_eq!(envelope.payload(), bytes);
        assert_eq!(activity::encode_signed(&envelope).unwrap(), signed);
        let input = produce(kind, envelope.payload());
        assert_eq!(
            support::input_bytes(&seed(&format!("native-program-{name}-v3"))).as_ref(),
            input
        );
        let decoded = payload_codec::payload_decode_case(&input)
            .expect("kernel codec payload reaches fuzz consumer");
        assert!(
            payload_codec::typed_payload_round_trip(&decoded),
            "{name} must run its production decoder"
        );
    }
}
