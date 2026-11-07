use layerx_programs_arbiter::{activity_codec::signed_activity_bytes, ReplayError};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::activity::{
    decode_signed, decode_unsigned, encode_signed, encode_unsigned, Activity,
};

const MAX_BYTES: usize = 1_048_576;
const MAX_PAYLOAD_BYTES: usize = 524_288;

const AUTHORITY_FROM_PRESTATE_ACTIVITIES: [&str; 4] = [
    concat!(
        "000310010c01000302000000070300090003040000001f6469643a6c78703a6d61696e74656e616e63652d7075626c69",
        "636174696f6e05000000208461a06f7c3c0cf1111ad70da871ba9b00ffb601073e7b5705dfabcfad043cd50600000000",
        "000000030700000000000000010000000000000064080000002000000000000000040000000000000000000000000000",
        "00000000000000000001090000000000000000000000003b9aca000a00000020756f6318ffc4ff902faaa0d809b60079",
        "f8684d937b548ab16bb7eeb7906596bf0b0000013f000000000000000000000000000000000000000000000000000000",
        "000000000000004c58502f70726f6772616d2d7265706c61792d70726f66696c652f7631000001000000800010000000",
        "0000f131000000000000000000000000000000000000000000000000000000000000000002000b000000000055000000",
        "270000001000000000000f42400000000001000000000000000010000000000000001000000000000000000040000000",
        "000010000000000000000010006c61796572785f63616c6c000302030509090909090909090909090909090909090909",
        "090909090909090909090909090a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a000000",
        "000000000000000000000000014c61796572582f70726f6772616d732f6163636573732d6465636c61726174696f6e2f",
        "763100000c000000402b6801f3c2a4f4e0de4483e857514b46c1462379755e91f0080f3c1e2e14c73de69df5dd803df6",
        "469f73e2d94c082700fab5dfe61b1b3b8f98fe6cfc5949800c",
    ),
    concat!(
        "000310010c01000302000000070300090003040000001f6469643a6c78703a6d61696e74656e616e63652d7075626c69",
        "636174696f6e05000000208461a06f7c3c0cf1111ad70da871ba9b00ffb601073e7b5705dfabcfad043cd50600000000",
        "000000040700000000000000010000000000000064080000002000000000000000050000000000000000000000000000",
        "00000000000000000001090000000000000000000000003b9aca000a00000020756f6318ffc4ff902faaa0d809b60079",
        "f8684d937b548ab16bb7eeb7906596bf0b0000013f000000000000000000000000000000000000000000000000000000",
        "000000000000004c58502f70726f6772616d2d7265706c61792d70726f66696c652f7631000001000000800010000000",
        "0000f131000000000000000000000000000000000000000000000000000000000000000002000b000000000055000000",
        "270000001000000000000f42400000000001000000000000000010000000000000001000000000000000000040000000",
        "000010000000000000000010006c61796572785f63616c6c000302030509090909090909090909090909090909090909",
        "090909090909090909090909090a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a000000",
        "000000000000000000000000014c61796572582f70726f6772616d732f6163636573732d6465636c61726174696f6e2f",
        "763100000c00000040ca324f9c4270db8cdffd48206e1c59a4e0c410df45bc631ae353e333411dca05c5c1e5b5507916",
        "c19f344563f4bd15072e6ae33a40928eb2770e1c00bc332f0a",
    ),
    concat!(
        "000310010c01000302000000070300090003040000001f6469643a6c78703a6d61696e74656e616e63652d7075626c69",
        "636174696f6e05000000208461a06f7c3c0cf1111ad70da871ba9b00ffb601073e7b5705dfabcfad043cd50600000000",
        "000000050700000000000000010000000000000064080000002000000000000000060000000000000000000000000000",
        "00000000000000000001090000000000000000000000003b9aca000a00000020756f6318ffc4ff902faaa0d809b60079",
        "f8684d937b548ab16bb7eeb7906596bf0b0000013f000000000000000000000000000000000000000000000000000000",
        "000000000000004c58502f70726f6772616d2d7265706c61792d70726f66696c652f7631000001000000800010000000",
        "0000f131000000000000000000000000000000000000000000000000000000000000000002000b000000000055000000",
        "270000001000000000000f42400000000001000000000000000010000000000000001000000000000000000040000000",
        "000010000000000000000010006c61796572785f63616c6c000302030509090909090909090909090909090909090909",
        "090909090909090909090909090a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a000000",
        "000000000000000000000000014c61796572582f70726f6772616d732f6163636573732d6465636c61726174696f6e2f",
        "763100000c00000040a4035b85b234349b971d34a6d5cf3c77dce2b06441f7fa7115a1f27fc3313ad989ebae2fc6cdd4",
        "eef6d5f8baa3b2b54a52c6c99f171866f2ba45ba9e1e8c7d08",
    ),
    concat!(
        "000310010c01000302000000070300090003040000001f6469643a6c78703a6d61696e74656e616e63652d7075626c69",
        "636174696f6e05000000208461a06f7c3c0cf1111ad70da871ba9b00ffb601073e7b5705dfabcfad043cd50600000000",
        "000000070700000000000000010000000000000064080000002000000000000000080000000000000000000000000000",
        "00000000000000000001090000000000000000000000003b9aca000a0000002062d45ff71087fe0b8a64d20045a625bc",
        "18b7cd392387331d69b75eb1dbd391020b0000013f000000000000000000000000000000000000000000000000000000",
        "000000000000004c58502f70726f6772616d2d7265706c61792d70726f66696c652f7631000001000000800010000000",
        "0000f132000000000000000000000000000000000000000000000000000000000000000002000b000000000055000000",
        "270000001000000000000f42400000000001000000000000000010000000000000001000000000000000000040000000",
        "000010000000000000000010006c61796572785f63616c6c000302030509090909090909090909090909090909090909",
        "090909090909090909090909090a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a000000",
        "000000000000000000000000014c61796572582f70726f6772616d732f6163636573732d6465636c61726174696f6e2f",
        "763100000c0000004013ce6e6441236b8db15060838eef511eb80ed1680b8665cf1ef750605ca8bf77f90c9cc4a38cbe",
        "5518e8b8deaf676b612ac9132162a35957bfbe8bea02260002",
    ),
 ];

fn hex(text: &str) -> Vec<u8> {
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).expect("literal fixture hex"))
        .collect()
}

fn registry() -> ModuleRegistry {
    let call = ActivityType::new(ModuleId::Programs, 3).expect("program call");
    let programs = ModuleRegistration::new(ModuleId::Programs, &[call]).expect("program module");
    ModuleRegistry::new(&[programs]).expect("program registry")
}

#[test]
fn signed_activity_bytes_match_wire_encoder_golden() {
    let registry = registry();
    for (golden, sequence) in AUTHORITY_FROM_PRESTATE_ACTIVITIES.iter().zip([3, 4, 5, 7]) {
        let bytes = hex(golden);
        let activity = decode_signed(&bytes, &registry).expect("signed replay fixture");
        assert_eq!(activity.account_sequence(), sequence);
        assert_eq!(activity.actor_did(), b"did:lxp:maintenance-publication");
        assert_eq!(activity.protocol_version(), 3);
        assert_eq!(activity.network_id(), 7);
        assert_eq!(
            layerx_wire::hash::payload_hash(&activity).expect("payload hash"),
            activity.payload_hash()
        );
        let key = activity.authority().try_into().expect("Ed25519 public key");
        let signature = activity
            .signature()
            .expect("signature")
            .try_into()
            .expect("Ed25519 signature");
        let preimage = layerx_wire::sign::preimage(&activity).expect("signing preimage");
        layerx_crypto::ed25519::verify_digest(&key, &signature, preimage.as_bytes())
            .expect("genuine fixture signature");
        assert_eq!(encode_signed(&activity).expect("wire golden"), bytes);
        assert_eq!(signed_activity_bytes(&activity), Ok(bytes));
        let unsigned = decode_unsigned(
            &encode_unsigned(&activity).expect("unsigned bytes"),
            &registry,
        )
        .expect("unsigned activity");
        assert_eq!(
            signed_activity_bytes(&unsigned),
            Err(ReplayError::Authority)
        );
    }
}

fn replace_vectors(bytes: &[u8], activity: &Activity, authority: &[u8], payload: &[u8]) -> Vec<u8> {
    let mut bytes = bytes.to_vec();
    let payload_offset = bytes.len()
        - activity.signature().expect("signature").len()
        - 5
        - activity.payload().len()
        - 4;
    let payload_length = u32::try_from(payload.len())
        .expect("payload length")
        .to_be_bytes();
    bytes.splice(
        payload_offset..payload_offset + 4 + activity.payload().len(),
        payload_length.into_iter().chain(payload.iter().copied()),
    );
    let authority_offset = 24 + activity.actor_did().len();
    let authority_length = u32::try_from(authority.len())
        .expect("authority length")
        .to_be_bytes();
    bytes.splice(
        authority_offset..authority_offset + 4 + activity.authority().len(),
        authority_length
            .into_iter()
            .chain(authority.iter().copied()),
    );
    bytes
}

#[test]
fn signed_activity_bytes_refuse_oversize_payload() {
    let registry = registry();
    let bytes = hex(AUTHORITY_FROM_PRESTATE_ACTIVITIES[0]);
    let activity = decode_signed(&bytes, &registry).expect("signed replay fixture");
    let payload = vec![0x5a; MAX_PAYLOAD_BYTES];
    let authority_length = MAX_BYTES
        - 155
        - activity.actor_did().len()
        - activity.signature().expect("signature").len()
        - payload.len();
    let authority = vec![0xa5; authority_length + 1];
    let boundary_bytes =
        replace_vectors(&bytes, &activity, &authority[..authority_length], &payload);
    assert_eq!(boundary_bytes.len(), MAX_BYTES);
    let boundary = decode_signed(&boundary_bytes, &registry).expect("maximum-sized activity");
    assert_eq!(
        encode_signed(&boundary).expect("wire boundary"),
        boundary_bytes
    );
    assert_eq!(signed_activity_bytes(&boundary), Ok(boundary_bytes));
    let oversized_bytes = replace_vectors(&bytes, &activity, &authority, &payload);
    assert_eq!(oversized_bytes.len(), MAX_BYTES + 1);
    let oversized =
        decode_signed(&oversized_bytes, &registry).expect("bounded fields with oversized encoding");
    assert!(encode_signed(&oversized).is_err());
    assert_eq!(
        signed_activity_bytes(&oversized),
        Err(ReplayError::Authority)
    );
    for length in [MAX_PAYLOAD_BYTES + 1, MAX_BYTES + 1] {
        let payload = vec![0x5a; length];
        let encoded = replace_vectors(&bytes, &activity, activity.authority(), &payload);
        assert!(
            decode_signed(&encoded, &registry).is_err(),
            "Activity must refuse payload length {length}"
        );
    }
}
