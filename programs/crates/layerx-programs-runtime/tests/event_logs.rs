use layerx_programs_runtime::abi::{
    AbiEffects, AbiError, CallFrameId, ProgramEvent, ProgramEventLog, MAX_EVENT_DATA_BYTES,
    MAX_EVENT_TOPIC_BYTES,
};
use layerx_programs_runtime::{PrincipalId, ProgramId};
use sha3::{Digest, Keccak256};

const ENVELOPE_HEX: &str = concat!(
    "4c61796572582f70726f6772616d732f6576656e74732f7631000000000211111111111111111111",
    "11111111111111111111111111111111111111111111222222222222222222222222222222222222",
    "2222222222222222222222222222000000000000000000000000036162630000000300ff80111111",
    "11111111111111111111111111111111111111111111111111111111113333333333333333333333",
    "3333333333333333333333333333333333333333330700000000000000010000000568656c6c6f00",
    "000028000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f2021222324",
    "252627",
);

fn program() -> ProgramId {
    ProgramId::new([0x11; 32]).unwrap_or_else(|error| panic!("program: {error}"))
}

fn effects() -> AbiEffects {
    AbiEffects {
        events: vec![
            ProgramEvent {
                program: program(),
                principal: PrincipalId::new([0x22; 32])
                    .unwrap_or_else(|error| panic!("principal: {error}")),
                frame: CallFrameId::root(),
                topic: b"abc".to_vec(),
                data: vec![0, 0xff, 0x80],
            },
            ProgramEvent {
                program: program(),
                principal: PrincipalId::new([0x33; 32])
                    .unwrap_or_else(|error| panic!("principal: {error}")),
                frame: CallFrameId::root()
                    .child(7)
                    .unwrap_or_else(|error| panic!("frame: {error}")),
                topic: b"hello".to_vec(),
                data: (0..40).collect(),
            },
        ],
        ..AbiEffects::default()
    }
}

#[test]
fn program_event_logs_encode_topics_and_abi_data() {
    let effects = effects();
    let logs: Vec<ProgramEventLog> = effects
        .program_event_logs(program())
        .unwrap_or_else(|error| panic!("logs: {error}"));
    let signature: [u8; 32] = Keccak256::digest(b"ProgramGuestEvent(bytes32,bytes32,bytes)").into();
    assert_eq!(logs.len(), 2);
    assert_eq!(logs[0].data.len(), 128);
    assert_eq!(logs[1].data.len(), 160);
    let kinds = [
        "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45",
        "1c8aff950685c2ed4bc3174f3472287b56d9517b9c948127319a09a7a36deac8",
    ];
    for ((log, event), kind) in logs.iter().zip(&effects.events).zip(kinds) {
        assert_eq!(log.topics, [signature, program().bytes()]);
        assert_eq!(hex::encode(&log.data[..32]), kind);
        let mut offset = [0; 32];
        offset[31] = 64;
        assert_eq!(log.data[32..64], offset);
        let mut length = [0; 32];
        length[24..].copy_from_slice(
            &u64::try_from(event.data.len())
                .unwrap_or_else(|error| panic!("payload length: {error}"))
                .to_be_bytes(),
        );
        assert_eq!(log.data[64..96], length);
        assert_eq!(&log.data[96..96 + event.data.len()], event.data);
        assert!(log.data[96 + event.data.len()..]
            .iter()
            .all(|byte| *byte == 0));
    }

    for length in [0, 1, 31, 32, 33, 63, 64, 65, MAX_EVENT_DATA_BYTES] {
        let mut boundary = effects.clone();
        boundary.events.truncate(1);
        boundary.events[0].data = vec![0xa5; length];
        let logs = boundary
            .program_event_logs(program())
            .unwrap_or_else(|error| panic!("payload boundary {length}: {error}"));
        assert_eq!(logs[0].data.len(), 96 + length.div_ceil(32) * 32);
        assert_eq!(&logs[0].data[96..96 + length], vec![0xa5; length]);
        assert!(logs[0].data[96 + length..].iter().all(|byte| *byte == 0));
        assert_eq!(
            &logs[0].data[88..96],
            u64::try_from(length)
                .unwrap_or_else(|error| panic!("boundary length: {error}"))
                .to_be_bytes()
        );
    }
    let mut maximum_topic = effects.clone();
    maximum_topic.events[0].topic = vec![0xa5; MAX_EVENT_TOPIC_BYTES];
    assert!(maximum_topic.program_event_logs(program()).is_ok());
    for invalid_topic in [false, true] {
        let mut invalid = effects.clone();
        if invalid_topic {
            invalid.events[1].topic = vec![0; MAX_EVENT_TOPIC_BYTES + 1];
        } else {
            invalid.events[1].data = vec![0; MAX_EVENT_DATA_BYTES + 1];
        }
        assert_eq!(
            invalid.program_event_logs(program()),
            Err(AbiError::EventBounds)
        );
    }
    assert_eq!(
        AbiEffects::default().program_event_logs(program()),
        Ok(vec![])
    );
}

#[test]
fn program_event_logs_keep_envelope_commitment() {
    let effects = effects();
    let frozen =
        hex::decode(ENVELOPE_HEX).unwrap_or_else(|error| panic!("frozen envelope: {error}"));
    let before = effects
        .canonical_program_event_envelope()
        .unwrap_or_else(|error| panic!("envelope before logs: {error}"));
    assert_eq!(before, frozen);
    let original = effects.clone();
    assert_eq!(
        effects
            .program_event_logs(program())
            .unwrap_or_else(|error| panic!("logs: {error}"))
            .len(),
        2
    );
    assert_eq!(effects, original);
    assert_eq!(effects.canonical_program_event_envelope(), Ok(frozen));
    let mut reversed = effects.clone();
    reversed.events.reverse();
    assert_ne!(reversed.canonical_program_event_envelope(), Ok(before));

    let mut boundary = AbiEffects {
        events: vec![effects.events[0].clone(); 512],
        ..AbiEffects::default()
    };
    assert_eq!(
        boundary
            .program_event_logs(program())
            .unwrap_or_else(|error| panic!("512 logs: {error}"))
            .len(),
        512
    );
    assert!(boundary.canonical_program_event_envelope().is_ok());
    boundary.events.push(effects.events[0].clone());
    assert_eq!(
        boundary.program_event_logs(program()),
        Err(AbiError::EffectLimit)
    );
    assert_eq!(
        boundary.canonical_program_event_envelope(),
        Err(AbiError::EffectLimit)
    );
    let empty_envelope =
        hex::decode("4c61796572582f70726f6772616d732f6576656e74732f76310000000000")
            .unwrap_or_else(|error| panic!("empty envelope: {error}"));
    assert_eq!(
        AbiEffects::default().canonical_program_event_envelope(),
        Ok(empty_envelope)
    );
}
