use ed25519_dalek::{Signer as _, SigningKey};
use layerx_programs_runtime::terminal::{
    decode_terminal_payload, FailureTerminal, TerminalDetail, EMPTY_CALL_GRAPH, PRE_RUNTIME_FAILURE,
};
use layerx_proof::program::{
    verify_program_execution, ProgramExecutionCheck, ProgramExecutionExpectation,
};
use layerx_types::intent::{ProgramCallFailure, ProgramCallOutcome};
use layerx_wire::hash::receipt_digest;
use layerx_wire::receipt::{decode, encode_unsigned};
use sha2::{Digest as _, Sha256};
use std::path::PathBuf;

const ACTIVITY_ID: [u8; 32] = [
    0xb6, 0x40, 0x04, 0xd5, 0x40, 0x4d, 0x5b, 0x13, 0xcf, 0x99, 0x3b, 0xbc, 0xc8, 0xa1, 0xc0, 0xcc,
    0xa3, 0x41, 0xca, 0xe7, 0xfd, 0x56, 0xf1, 0xf2, 0x98, 0xfe, 0xe1, 0x2f, 0xdc, 0x0c, 0x99, 0x20,
];
const PAYLOAD_HASH: [u8; 32] = [
    0x0d, 0xbb, 0xb3, 0xba, 0xeb, 0xbc, 0xe6, 0x12, 0x9e, 0x45, 0xf4, 0x0f, 0xfa, 0x4f, 0xcf, 0x02,
    0x64, 0x64, 0x68, 0xc5, 0xc1, 0x1d, 0x68, 0x11, 0x57, 0xc5, 0xbc, 0xbb, 0x46, 0x61, 0xc8, 0xbb,
];
const CALL_PAYLOAD: &[u8] = &[
    0x8b, 0xf8, 0xe2, 0xd8, 0x26, 0x19, 0x0f, 0x4e, 0x61, 0x5e, 0xb9, 0x77, 0x8a, 0x94, 0xbc, 0x2b,
    0xdb, 0x26, 0x96, 0x5f, 0xa3, 0x7d, 0x76, 0xd3, 0xfe, 0x60, 0x86, 0xfe, 0x29, 0x87, 0xcd, 0x4b,
    0x00, 0x02, 0x00, 0x0b, 0x00, 0x00, 0x00, 0xe4, 0x00, 0x56, 0x00, 0x00, 0x00, 0x27, 0x00, 0x00,
    0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05, 0xf5, 0xe1, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x78,
    0x5f, 0x63, 0x61, 0x6c, 0x6c, 0x01, 0x01, 0x00, 0x10, 0x7f, 0x6c, 0x51, 0xbc, 0x76, 0x47, 0x74,
    0x63, 0x4a, 0x25, 0x9a, 0xde, 0xc7, 0xe9, 0xf3, 0x28, 0x41, 0x87, 0x43, 0xb4, 0x94, 0x9d, 0x2b,
    0x19, 0xe6, 0xea, 0xd8, 0xb8, 0x11, 0x72, 0x31, 0xb0, 0x6d, 0xca, 0xab, 0xf0, 0x7e, 0x26, 0xfb,
    0x22, 0xab, 0x8c, 0x95, 0xe2, 0xcd, 0x2e, 0xd1, 0x71, 0x50, 0x96, 0xb5, 0xd4, 0xc6, 0x43, 0x7b,
    0x9f, 0xe8, 0x59, 0x87, 0x77, 0x3a, 0xcf, 0xd4, 0x76, 0x6f, 0x3d, 0xaa, 0x14, 0xdd, 0x28, 0x75,
    0xf5, 0x27, 0x21, 0x40, 0x0e, 0x1a, 0xef, 0xdb, 0x8e, 0x41, 0x87, 0x43, 0xb4, 0x94, 0x9d, 0x2b,
    0x19, 0xe6, 0xea, 0xd8, 0xb8, 0x11, 0x72, 0x31, 0xb0, 0x6d, 0xca, 0xab, 0xf0, 0x7e, 0x26, 0xfb,
    0x22, 0xab, 0x8c, 0x95, 0xe2, 0xcd, 0x2e, 0xd1, 0x71, 0x41, 0x87, 0x43, 0xb4, 0x94, 0x9d, 0x2b,
    0x19, 0xe6, 0xea, 0xd8, 0xb8, 0x11, 0x72, 0x31, 0xb0, 0x6d, 0xca, 0xab, 0xf0, 0x7e, 0x26, 0xfb,
    0x22, 0xab, 0x8c, 0x95, 0xe2, 0xcd, 0x2e, 0xd1, 0x71, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0xe2, 0x85, 0x25, 0x69, 0xbb, 0x8a, 0x5e,
    0xd2, 0xa6, 0xad, 0x49, 0xd3, 0x4c, 0xcb, 0x0d, 0xbf, 0x05, 0x6b, 0x09, 0x03, 0x8b, 0x33, 0x5c,
    0x18, 0x09, 0xac, 0x4f, 0x85, 0x21, 0x44, 0xf6, 0xed, 0x5b, 0xea, 0xe4, 0x33, 0xa7, 0x6c, 0xe8,
    0x9e, 0xab, 0xd8, 0x13, 0x9d, 0x76, 0x1b, 0xcb, 0x74, 0x49, 0x66, 0x25, 0xca, 0xf1, 0x94, 0x96,
    0x83, 0xd1, 0xc2, 0xfc, 0x23, 0x1b, 0x0c, 0x86, 0x5a, 0x00, 0x04, 0x03, 0x05, 0x50, 0x96, 0xb5,
    0xd4, 0xc6, 0x43, 0x7b, 0x9f, 0xe8, 0x59, 0x87, 0x77, 0x3a, 0xcf, 0xd4, 0x76, 0x6f, 0x3d, 0xaa,
    0x14, 0xdd, 0x28, 0x75, 0xf5, 0x27, 0x21, 0x40, 0x0e, 0x1a, 0xef, 0xdb, 0x8e, 0x41, 0x87, 0x43,
    0xb4, 0x94, 0x9d, 0x2b, 0x19, 0xe6, 0xea, 0xd8, 0xb8, 0x11, 0x72, 0x31, 0xb0, 0x6d, 0xca, 0xab,
    0xf0, 0x7e, 0x26, 0xfb, 0x22, 0xab, 0x8c, 0x95, 0xe2, 0xcd, 0x2e, 0xd1, 0x71, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x07, 0x08, 0x4c,
    0x61, 0x79, 0x65, 0x72, 0x58, 0x2f, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x61, 0x6d, 0x73, 0x2f, 0x61,
    0x63, 0x63, 0x65, 0x73, 0x73, 0x2d, 0x64, 0x65, 0x63, 0x6c, 0x61, 0x72, 0x61, 0x74, 0x69, 0x6f,
    0x6e, 0x2f, 0x76, 0x31, 0x00, 0x00,
];
const NETWORK_ID: u32 = 7332;

fn fixture(name: &str) -> Vec<u8> {
    let path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../tests/fixtures/programs/pre-runtime-refusal")
        .join(name);
    let text = std::fs::read_to_string(path).unwrap_or_else(|error| panic!("{error}"));
    let text = text.trim();
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(
                core::str::from_utf8(pair).unwrap_or_else(|error| panic!("{error}")),
                16,
            )
            .unwrap_or_else(|error| panic!("{error}"))
        })
        .collect()
}

fn expected() -> ProgramExecutionExpectation {
    assert_eq!(NETWORK_ID, 7332);
    let native = layerx_types::program_call::NativeProgramCall::decode(CALL_PAYLOAD)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(native.resources.0[0], 100_000_000);
    let receipt = decode(&fixture("receipt.hex")).unwrap_or_else(|error| panic!("{error:?}"));
    ProgramExecutionExpectation {
        sequencer_public_key: fixture("sequencer-public.hex")
            .try_into()
            .unwrap_or_else(|_| panic!("public key")),
        previous_state_root: receipt
            .protocol()
            .unwrap_or_else(|| panic!("protocol"))
            .previous_state_root(),
        activity_id: ACTIVITY_ID,
        payload_hash: PAYLOAD_HASH,
        program_id: native.callee().bytes(),
        guest_abi_version: native.guest_abi,
    }
}

#[test]
fn actual_native_admission_refusal_authenticates_as_refused() {
    let terminal = fixture("terminal.hex");
    let graph = fixture("call-graph.hex");
    assert_eq!(terminal.len(), 165);
    assert_eq!(graph, EMPTY_CALL_GRAPH);
    let verified = verify_program_execution(&fixture("receipt.hex"), &terminal, &graph, expected())
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(verified.result_code(), -3);
    assert!(matches!(
        verified.outcome(),
        ProgramCallOutcome::Refused(ProgramCallFailure::GuestRefused { code: -3 })
    ));
    let TerminalDetail::Failure(FailureTerminal::PreRuntime(failure)) = &verified.terminal().detail
    else {
        panic!("pre-runtime refusal");
    };
    assert_eq!(failure.payload_hash, expected().payload_hash);
    assert_eq!(failure.module_version, 4);
    assert_eq!(failure.parameter_version, 1);
    assert_eq!(
        failure.applied_legs_digest,
        <[u8; 32]>::from(Sha256::digest([]))
    );
    assert!(verified.terminal().attachments.is_empty());
    let mut wrong_payload = expected();
    wrong_payload.payload_hash[0] ^= 1;
    assert_eq!(
        verify_program_execution(&fixture("receipt.hex"), &terminal, &graph, wrong_payload)
            .err()
            .map(|error| error.check),
        Some(ProgramExecutionCheck::Terminal)
    );
    let mut wrong_activity = expected();
    wrong_activity.activity_id[0] ^= 1;
    assert_eq!(
        verify_program_execution(&fixture("receipt.hex"), &terminal, &graph, wrong_activity)
            .err()
            .map(|error| error.check),
        Some(ProgramExecutionCheck::Activity)
    );
}

#[test]
fn native_refusal_codec_rejects_trailing_bytes_and_incomplete_fields() {
    let terminal = fixture("terminal.hex");
    for length in 0..terminal.len() {
        if length == terminal.len() - 33 {
            let decoded = decode_terminal_payload(2, 2, &terminal[..length])
                .unwrap_or_else(|error| panic!("historical v3 layout: {error:?}"));
            assert!(
                matches!(decoded.detail, TerminalDetail::Failure(FailureTerminal::PreRuntime(ref failure)) if failure.encoding_version == 3)
            );
        } else {
            assert!(
                decode_terminal_payload(2, 2, &terminal[..length]).is_err(),
                "truncated at {length}"
            );
        }
    }
    let mut trailing = terminal.clone();
    trailing.push(0);
    assert!(decode_terminal_payload(2, 2, &trailing).is_err());
    for kind in [0, 1, 3, 4] {
        assert!(decode_terminal_payload(kind, 2, &terminal).is_err());
    }
}

fn resign_terminal(terminal: &[u8], key: &SigningKey) -> Vec<u8> {
    let mut receipt = fixture("receipt.hex");
    let old: [u8; 32] = Sha256::digest(fixture("terminal.hex")).into();
    let new: [u8; 32] = Sha256::digest(terminal).into();
    let offsets = receipt
        .windows(32)
        .enumerate()
        .filter_map(|(i, value)| (value == old).then_some(i))
        .collect::<Vec<_>>();
    assert_eq!(offsets.len(), 1);
    receipt[offsets[0]..offsets[0] + 32].copy_from_slice(&new);
    let decoded = decode(&receipt).unwrap_or_else(|error| panic!("{error:?}"));
    let unsigned = encode_unsigned(&decoded).unwrap_or_else(|error| panic!("{error:?}"));
    let digest = receipt_digest(&unsigned).unwrap_or_else(|error| panic!("{error:?}"));
    let offset = receipt.len() - 64;
    receipt[offset..].copy_from_slice(&key.sign(&digest).to_bytes());
    assert!(layerx_proof::receipt::verify_sequencer_signature(
        &receipt,
        key.verifying_key().to_bytes()
    )
    .is_ok());
    receipt
}

#[test]
fn resigned_codec_mutations_cannot_rebind_native_refusal() {
    let key = SigningKey::from_bytes(&[3; 32]);
    let mut expected = expected();
    expected.sequencer_public_key = key.verifying_key().to_bytes();
    let graph = fixture("call-graph.hex");
    let terminal = fixture("terminal.hex");
    let receipt = resign_terminal(&terminal, &key);
    assert!(verify_program_execution(&receipt, &terminal, &graph, expected).is_ok());
    let base = PRE_RUNTIME_FAILURE.len();
    for offset in [
        0,
        base,
        base + 32,
        base + 67,
        base + 71,
        base + 75,
        base + 76,
        base + 77,
    ] {
        let mut changed = terminal.clone();
        changed[offset] ^= 1;
        let receipt = resign_terminal(&changed, &key);
        assert!(
            verify_program_execution(&receipt, &changed, &graph, expected).is_err(),
            "re-signed changed field {offset}"
        );
    }
    let historical = &terminal[..terminal.len() - 33];
    let receipt = resign_terminal(historical, &key);
    assert_eq!(
        verify_program_execution(&receipt, historical, &graph, expected)
            .err()
            .map(|error| error.check),
        Some(ProgramExecutionCheck::Terminal)
    );
}

#[test]
fn resigned_redundant_applied_wrapper_is_not_a_native_refusal() -> Result<(), layerx_wire::WireError>
{
    let terminal = fixture("terminal.hex");
    let mut encoder = layerx_wire::encode::Encoder::new(1024);
    encoder.fixed(b"LXP/programs/terminal-applied-legs/v1\0")?;
    encoder.bytes(&terminal, 1024)?;
    encoder.bytes(&[], 1024)?;
    let wrapped = encoder.finish();
    assert_eq!(
        layerx_wire::receipt::decode_applied_terminal(&wrapped)?,
        (terminal.as_slice(), &[][..])
    );
    let key = SigningKey::from_bytes(&[3; 32]);
    let receipt = resign_terminal(&wrapped, &key);
    let mut expected = expected();
    expected.sequencer_public_key = key.verifying_key().to_bytes();
    assert_eq!(
        verify_program_execution(&receipt, &wrapped, &fixture("call-graph.hex"), expected)
            .err()
            .map(|error| error.check),
        Some(ProgramExecutionCheck::Terminal)
    );
    Ok(())
}
