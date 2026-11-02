//! Proves: Executor/AbiEffects -> AbiEffects::canonical_program_event_envelope/AbiEffects::program_event_logs -> transfer::parse_event_envelope/ProgramEventLog::write_encoded.
//!
//! Repairs empty-topic acceptance in `AbiEffects`' event serializers and
//! `transfer.rs::parse_event_envelope`, matching the real emitter's refusal.
//! The native log exporter now uses
//! the same safe `ProgramEventLog::write_encoded` consumer exercised here.
//!
//! Decision 2324 assigns `ffi_call.rs::read_native_receipt_authority` to
//! `tests/proof/lxp_proof_lxp_receipt_h.c`, which links the real `call.c` host.
//! This proof covers the pure Rust paths without calling an extern C function.
//! Event transport has no persistent state: each consumer is called repeatedly
//! with freshly produced results, and execution leaves storage unchanged.

use layerx_programs_runtime::abi::{
    AbiEffects, AbiError, UnavailableReceiptOracle, MAX_EVENTS_PER_ACTIVITY,
};
use layerx_programs_runtime::test_support::{
    code_section, func_body, function_section, import_section, module, raw_section, type_section,
    unsigned_leb, OP_CALL, OP_DROP, OP_END, OP_I32_CONST, TYPE_I32,
};
use layerx_programs_runtime::transfer::{parse_event_envelope, TransferLawError};
use layerx_programs_runtime::{
    AuthorizationContext, AuthorizedExecutionRequest, Capability, CapabilitySet,
    CompositionContext, Executor, PrincipalId, ProgramId, Storage, WasmEngine, ABI_MODULE,
};
use sha3::{Digest, Keccak256};

const EVENT_DOMAIN: &[u8] = b"LayerX/programs/events/v1\0";

fn program() -> ProgramId {
    ProgramId::new([0x11; 32]).unwrap_or_else(|error| panic!("program: {error}"))
}

fn principal() -> PrincipalId {
    PrincipalId::new([0x22; 32]).unwrap_or_else(|error| panic!("principal: {error}"))
}

fn event_guest(count: usize, payload: &[u8]) -> Vec<u8> {
    let topic = b"runtime-proof";
    let mut exports = unsigned_leb(2);
    for (name, kind, index) in [("run", 0, 1), ("memory", 2, 0)] {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.extend([kind, index]);
    }
    let mut data = vec![1, 0, OP_I32_CONST, 0, OP_END];
    data.extend(unsigned_leb((topic.len() + payload.len()) as u64));
    data.extend_from_slice(topic);
    data.extend_from_slice(payload);
    let mut instructions = Vec::new();
    for _ in 0..count {
        // Small positive constants fit in one signed LEB byte.
        instructions.extend([
            OP_I32_CONST,
            0,
            OP_I32_CONST,
            13,
            OP_I32_CONST,
            13,
            OP_I32_CONST,
            u8::try_from(payload.len()).unwrap_or_else(|error| panic!("payload: {error}")),
            OP_CALL,
            0,
            OP_DROP,
        ]);
    }
    instructions.extend([OP_I32_CONST, 0, OP_END]);
    module(&[
        type_section(&[(&[TYPE_I32; 4], &[TYPE_I32]), (&[TYPE_I32; 2], &[TYPE_I32])]),
        import_section(&[(ABI_MODULE, "event_emit", 0)]),
        function_section(&[1]),
        raw_section(5, &[1, 1, 1, 1]),
        raw_section(7, &exports),
        code_section(&[func_body(&[], &instructions)]),
        raw_section(11, &data),
    ])
}

fn produce_events(count: usize, payload: &[u8]) -> AbiEffects {
    let engine = WasmEngine::declared().unwrap_or_else(|error| panic!("engine: {error}"));
    let validated = engine
        .validate(&event_guest(count, payload))
        .unwrap_or_else(|error| panic!("guest: {error}"));
    let mut storage = Storage::new();
    let before = storage.clone();
    let record = Executor::declared()
        .execute_authorized(
            &mut storage,
            AuthorizedExecutionRequest {
                module: &validated,
                program: program(),
                authorization: AuthorizationContext::new(
                    principal(),
                    CapabilitySet::new([Capability::EmitEvent])
                        .unwrap_or_else(|error| panic!("grant: {error}")),
                ),
                receipts: &UnavailableReceiptOracle,
                entrypoint: "run",
                calldata: &[],
                composition: CompositionContext::isolated(),
                response_capacity: 0,
            },
        )
        .unwrap_or_else(|error| panic!("execution: {error}"));
    assert_eq!(storage, before);
    assert_eq!(record.effects.events.len(), count);
    for event in &record.effects.events {
        assert_eq!(event.program, program());
        assert_eq!(event.principal, principal());
        assert_eq!(event.topic, b"runtime-proof");
        assert_eq!(event.data, payload);
    }
    record.effects
}

#[test]
fn canonical_program_event_envelope_reaches_transfer_decoder_twice() {
    let mut previous = None;
    for payload in [b"first".as_slice(), b"second".as_slice()] {
        let effects = produce_events(2, payload);
        let envelope = effects
            .canonical_program_event_envelope()
            .unwrap_or_else(|error| panic!("envelope: {error}"));
        for _ in 0..2 {
            assert_eq!(parse_event_envelope(&envelope), Ok(envelope.clone()));
        }
        if let Some(previous) = previous {
            assert_ne!(envelope, previous);
        }
        previous = Some(envelope.clone());
        // Corrupt actual producer bytes only for refusal cases.
        let mut malformed = envelope.clone();
        malformed[0] ^= 1;
        assert_eq!(
            parse_event_envelope(&malformed),
            Err(TransferLawError::InvalidTransferSet)
        );
        assert_eq!(
            parse_event_envelope(&envelope[..envelope.len() - 1]),
            Err(TransferLawError::InvalidTransferSet)
        );
        let mut trailing = envelope.clone();
        trailing.push(0);
        assert_eq!(
            parse_event_envelope(&trailing),
            Err(TransferLawError::InvalidTransferSet)
        );
        assert_eq!(parse_event_envelope(&envelope), Ok(envelope.clone()));
    }
}

#[test]
fn transfer_decoder_enforces_the_producers_exact_event_count_limit() {
    let effects = produce_events(MAX_EVENTS_PER_ACTIVITY, b"boundary");
    let envelope = effects
        .canonical_program_event_envelope()
        .unwrap_or_else(|error| panic!("boundary envelope: {error}"));
    assert_eq!(parse_event_envelope(&envelope), Ok(envelope.clone()));
    // Append one real event record, so the excessive count is the only defect.
    let first_record = produce_events(1, b"boundary")
        .canonical_program_event_envelope()
        .unwrap_or_else(|error| panic!("single envelope: {error}"));
    let mut oversized = envelope.clone();
    oversized[EVENT_DOMAIN.len()..EVENT_DOMAIN.len() + 4].copy_from_slice(
        &u32::try_from(MAX_EVENTS_PER_ACTIVITY + 1)
            .unwrap_or_else(|error| panic!("count: {error}"))
            .to_be_bytes(),
    );
    oversized.extend_from_slice(&first_record[EVENT_DOMAIN.len() + 4..]);
    assert_eq!(
        parse_event_envelope(&oversized),
        Err(TransferLawError::InvalidTransferSet)
    );
    assert_eq!(parse_event_envelope(&envelope), Ok(envelope.clone()));
}

#[test]
fn empty_topics_are_refused_at_the_producer_and_transfer_decoder() {
    let mut effects = produce_events(1, b"payload");
    let mut envelope = effects
        .canonical_program_event_envelope()
        .unwrap_or_else(|error| panic!("envelope: {error}"));
    // Remove the real producer's topic while preserving every other field.
    let topic_length_offset = EVENT_DOMAIN.len() + 4 + 32 + 32 + 8 + 1;
    let topic_offset = topic_length_offset + 4;
    envelope.drain(topic_offset..topic_offset + effects.events[0].topic.len());
    envelope[topic_length_offset..topic_offset].copy_from_slice(&0_u32.to_be_bytes());
    assert_eq!(
        parse_event_envelope(&envelope),
        Err(TransferLawError::InvalidTransferSet)
    );
    effects.events[0].topic.clear();
    assert_eq!(
        effects.canonical_program_event_envelope(),
        Err(AbiError::EventBounds)
    );
    assert_eq!(
        effects.program_event_logs(program()),
        Err(AbiError::EventBounds)
    );
}

#[test]
fn program_event_logs_reach_the_native_log_encoder_without_cached_bytes() {
    let mut previous = None;
    for payload in [b"first".as_slice(), b"second".as_slice()] {
        let effects = produce_events(2, payload);
        let original = effects.clone();
        for _ in 0..2 {
            let logs = effects
                .program_event_logs(program())
                .unwrap_or_else(|error| panic!("logs: {error}"));
            assert_eq!(logs.len(), 2);
            for log in logs {
                let length = 64 + log.data.len();
                let mut output = vec![0xa5; length + 2];
                assert_eq!(log.write_encoded(&mut output[1..]), Ok(length));
                assert_eq!(output[0], 0xa5);
                assert_eq!(output[length + 1], 0xa5);
                let encoded = &output[1..=length];
                assert_eq!(
                    &encoded[..32],
                    Keccak256::digest(b"ProgramGuestEvent(bytes32,bytes32,bytes)").as_slice()
                );
                assert_eq!(&encoded[32..64], &program().bytes());
                assert_eq!(
                    &encoded[64..96],
                    Keccak256::digest(b"runtime-proof").as_slice()
                );
                let mut dynamic_offset = [0; 32];
                dynamic_offset[31] = 64;
                assert_eq!(&encoded[96..128], &dynamic_offset);
                assert_eq!(&encoded[128..152], &[0; 24]);
                assert_eq!(&encoded[152..160], &(payload.len() as u64).to_be_bytes());
                assert_eq!(&encoded[160..160 + payload.len()], payload);
                assert!(encoded[160 + payload.len()..].iter().all(|byte| *byte == 0));
                let mut short = vec![0xa5; length - 1];
                let untouched = short.clone();
                assert_eq!(log.write_encoded(&mut short), Err(AbiError::EventBounds));
                assert_eq!(short, untouched);
            }
        }
        let encoded = effects
            .program_event_logs(program())
            .unwrap_or_else(|error| panic!("logs: {error}"));
        if let Some(previous) = previous {
            assert_ne!(encoded, previous);
        }
        previous = Some(encoded);
        assert_eq!(effects, original);
    }
}

#[test]
fn event_consumers_refuse_oversized_producer_fields_and_recover() {
    use layerx_programs_runtime::abi::{MAX_EVENT_DATA_BYTES, MAX_EVENT_TOPIC_BYTES};

    let original = produce_events(1, b"payload");
    let canonical = original
        .canonical_program_event_envelope()
        .unwrap_or_else(|error| panic!("envelope: {error}"));
    let topic_length_offset = EVENT_DOMAIN.len() + 4 + 32 + 32 + 8 + 1;
    let data_length_offset = topic_length_offset + 4 + original.events[0].topic.len();
    for (length_offset, maximum) in [
        (topic_length_offset, MAX_EVENT_TOPIC_BYTES),
        (data_length_offset, MAX_EVENT_DATA_BYTES),
    ] {
        let mut malformed = canonical.clone();
        malformed[length_offset..length_offset + 4].copy_from_slice(
            &u32::try_from(maximum + 1)
                .unwrap_or_else(|error| panic!("length: {error}"))
                .to_be_bytes(),
        );
        for _ in 0..2 {
            assert_eq!(
                parse_event_envelope(&malformed),
                Err(TransferLawError::InvalidTransferSet)
            );
            assert_eq!(parse_event_envelope(&canonical), Ok(canonical.clone()));
        }
    }

    for oversized_topic in [true, false] {
        let mut effects = original.clone();
        if oversized_topic {
            effects.events[0].topic.resize(MAX_EVENT_TOPIC_BYTES + 1, 1);
        } else {
            effects.events[0].data.resize(MAX_EVENT_DATA_BYTES + 1, 1);
        }
        for _ in 0..2 {
            assert_eq!(
                effects.canonical_program_event_envelope(),
                Err(AbiError::EventBounds)
            );
            assert_eq!(
                effects.program_event_logs(program()),
                Err(AbiError::EventBounds)
            );
        }
    }
    assert_eq!(original.canonical_program_event_envelope(), Ok(canonical));
    assert_eq!(
        original
            .program_event_logs(program())
            .unwrap_or_else(|error| panic!("logs after refusal: {error}"))
            .len(),
        1
    );
}

#[test]
fn transfer_decoder_refuses_malformed_identifiers() {
    let effects = produce_events(1, b"identifiers");
    let envelope = effects
        .canonical_program_event_envelope()
        .unwrap_or_else(|error| panic!("identifier envelope: {error}"));
    let program_offset = EVENT_DOMAIN.len() + 4;
    let principal_offset = program_offset + 32;
    // Mutate only the actual producer's program or principal identifier.
    for offset in [program_offset, principal_offset] {
        let mut malformed = envelope.clone();
        malformed[offset..offset + 32].fill(0);
        for _ in 0..2 {
            assert_eq!(
                parse_event_envelope(&malformed),
                Err(TransferLawError::InvalidTransferSet)
            );
            assert_eq!(parse_event_envelope(&envelope), Ok(envelope.clone()));
        }
    }
}
