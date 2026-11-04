use layerx_types::payload::{ActivityType, Payload};
use layerx_wire::decode::Decoder;
use layerx_wire::encode::Encoder;

use crate::support;

pub fn payload_decode_case(data: &[u8]) -> Option<Payload> {
    let materialized = support::input_bytes(data);
    let mut decoder = Decoder::new(&materialized, support::MAX_FUZZ_INPUT);
    let raw_type = decoder.u32().ok()?;
    let bytes = decoder.bytes_owned(support::MAX_FUZZ_INPUT / 2).ok()?;
    decoder.finish().ok()?;
    let activity_type = ActivityType::from_u32(raw_type).ok()?;
    let payload = Payload::new(support::registry(), activity_type, &bytes).ok()?;
    let mut encoder = Encoder::new(support::MAX_FUZZ_INPUT);
    encoder
        .u32(payload.activity_type().value())
        .expect("accepted activity fits the encoder");
    encoder
        .bytes(payload.as_bytes(), support::MAX_FUZZ_INPUT / 2)
        .expect("accepted payload fits the encoder");
    assert_eq!(encoder.finish(), materialized.as_ref());
    let _ = typed_payload_round_trip(&payload);
    Some(payload)
}

pub fn typed_payload_round_trip(payload: &Payload) -> bool {
    use layerx_crypto::payments::Payment;
    use layerx_types::payload::{ModuleId, PerpsPayload, SpotPayload};
    use layerx_types::program_call::NativeProgramCall;
    use layerx_types::program_lifecycle::{
        NativeProgramAccountProfile2, NativeProgramDeploy, NativeProgramUpgrade,
        NativeProgramWindDown,
    };

    let kind = payload.activity_type();
    let bytes = payload.as_bytes();
    let encoded = match (kind.module(), kind.ordinal()) {
        (ModuleId::Asset, 1 | 2 | 3 | 4 | 10 | 11) | (ModuleId::Programs, 5) => {
            Payment::decode(kind.module(), kind.ordinal(), bytes, b"did:lx:fuzz")
                .ok()
                .map(|value| {
                    value
                        .encode(b"did:lx:fuzz")
                        .expect("decoded payment encodes")
                })
        }
        (ModuleId::Programs, 6) if bytes.get(32..37) == Some(b"LXPA2".as_slice()) => {
            NativeProgramAccountProfile2::decode(bytes)
                .ok()
                .map(|value| value.encode().expect("decoded program account encodes"))
        }
        (ModuleId::Programs, 6) => {
            Payment::decode(kind.module(), kind.ordinal(), bytes, b"did:lx:fuzz")
                .ok()
                .map(|value| {
                    value
                        .encode(b"did:lx:fuzz")
                        .expect("decoded program account encodes")
                })
        }
        (ModuleId::Programs, 1) => NativeProgramDeploy::decode(bytes)
            .ok()
            .map(|value| value.encode().expect("decoded deploy encodes")),
        (ModuleId::Programs, 2) => NativeProgramUpgrade::decode(bytes)
            .ok()
            .map(|value| value.encode().expect("decoded upgrade encodes")),
        (ModuleId::Programs, 3) => NativeProgramCall::decode(bytes)
            .ok()
            .map(|value| value.encode().expect("decoded call encodes")),
        (ModuleId::Programs, 7) => NativeProgramWindDown::decode(bytes)
            .ok()
            .map(|value| value.encode().expect("decoded wind-down encodes")),
        (ModuleId::Perps, _) => PerpsPayload::decode(kind, bytes)
            .ok()
            .map(|value| value.encode().expect("decoded perps payload encodes")),
        (ModuleId::Spot, _) => SpotPayload::decode(kind.value(), bytes)
            .ok()
            .map(|value| value.encode().expect("decoded spot payload encodes")),
        _ => None,
    };
    match encoded {
        Some(encoded) => {
            assert_eq!(encoded, bytes);
            true
        }
        None => false,
    }
}
