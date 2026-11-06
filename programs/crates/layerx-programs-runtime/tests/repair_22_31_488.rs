//! The native sandbox producer's committed execution and LXUR transport transcript
//! supply the real canonical body and digest. Re-publish them through the same
//! frame writer used by host_ffi::publish_settlement, then read the terminal.
//! The bounded gate runner records the verified revision and private evidence log.

use layerx_programs_runtime::{
    hash_bytes,
    terminal::{
        attach_sandbox_settlement, decode_terminal_payload, SandboxReceiptBinding,
        SandboxSettlement, SANDBOX_SETTLEMENT_DOMAIN,
    },
    HashAlgorithm,
};

const NON_CANONICAL: i32 = -3;
const NATIVE_RESULT: &str = include_str!(
    "../../layerx-programs-sandbox/src/repair_22_31_262_recordings/sandboxExecute.result.hex"
);
const NATIVE_USAGE_FRAMES: &str =
    include_str!("../../layerx-programs-sandbox/src/repair_22_31_262_recordings/usage.frames.hex");

struct NativeSettlement {
    execution: Vec<u8>,
    frame: Vec<u8>,
    transfer_root: [u8; 32],
}

fn sized(input: &mut &[u8]) -> Vec<u8> {
    let length = u32::from_be_bytes(input[..4].try_into().unwrap()) as usize;
    let value = input[4..4 + length].to_vec();
    *input = &input[4 + length..];
    value
}

fn native_settlement() -> NativeSettlement {
    let result = hex::decode(NATIVE_RESULT.trim()).unwrap();
    let offset = result
        .windows(SANDBOX_SETTLEMENT_DOMAIN.len())
        .position(|bytes| bytes == SANDBOX_SETTLEMENT_DOMAIN)
        .unwrap();
    let mut input = &result[offset + SANDBOX_SETTLEMENT_DOMAIN.len()..];
    let execution = sized(&mut input);
    let published_frame = sized(&mut input);
    let transfer_root = input[..32].try_into().unwrap();

    // Reassemble the independently published native LXUR transport and prove it
    // carries exactly the receipt passed to the terminal by the real producer.
    let transport = hex::decode(NATIVE_USAGE_FRAMES.trim()).unwrap();
    let mut input = transport.as_slice();
    let mut transported_frame = Vec::new();
    let mut index = 0u16;
    while !input.is_empty() {
        let chunk = sized(&mut input);
        assert_eq!(&chunk[..4], b"LXUR");
        assert_eq!(u16::from_be_bytes(chunk[44..46].try_into().unwrap()), index);
        assert_eq!(
            u32::from_be_bytes(chunk[48..52].try_into().unwrap()) as usize,
            published_frame.len()
        );
        transported_frame.extend_from_slice(&chunk[52..]);
        index += 1;
        if input.is_empty() {
            assert_eq!(u16::from_be_bytes(chunk[46..48].try_into().unwrap()), index);
        }
    }
    assert_eq!(transported_frame, published_frame);

    let body_length = published_frame.len() - 32;
    let mut frame = published_frame[..body_length].to_vec();
    let digest = published_frame[body_length..].try_into().unwrap();
    assert_eq!(hash_bytes(HashAlgorithm::Sha256, &frame).unwrap(), digest);
    SandboxSettlement::append_receipt_digest(&mut frame, digest);
    assert_eq!(frame, transported_frame);
    NativeSettlement {
        execution,
        frame,
        transfer_root,
    }
}

#[test]
fn native_producer_frame_is_accepted_by_the_terminal_reader() {
    let NativeSettlement {
        mut execution,
        frame,
        transfer_root,
    } = native_settlement();
    let original = execution.clone();
    // The pre-repair reader passes this entire frame to the canonical parser,
    // whose end-of-input check refuses the digest trailer.
    attach_sandbox_settlement(&mut execution, &frame, transfer_root).unwrap();
    let terminal = decode_terminal_payload(1, 3, &execution).unwrap();
    let settlement = terminal.sandbox_settlement().unwrap();
    let canonical = &frame[..frame.len() - 32];
    assert_eq!(settlement.receipt, canonical);
    assert_eq!(settlement.canonical_usage_receipt().unwrap(), canonical);
    assert_eq!(settlement.receipt_digest, frame[frame.len() - 32..]);
    assert_eq!(settlement.transfer_root, transfer_root);
    assert_eq!(settlement.sequence, 1);
    assert!(settlement.charged > 0);
    settlement
        .authenticate(SandboxReceiptBinding {
            activity_id: settlement.activity_id,
            program: settlement.program,
            kind: 1,
            fee_schedule_version: settlement.fees.version(),
            usage: settlement.usage,
            usage_receipt_digest: frame[frame.len() - 32..].try_into().unwrap(),
            transfer_root,
        })
        .unwrap();
    assert_eq!(
        terminal.detail,
        decode_terminal_payload(1, 3, &original).unwrap().detail
    );
}

#[test]
fn flipped_digest_byte_is_non_canonical() {
    let mut native = native_settlement();
    let last = native.frame.len() - 1;
    native.frame[last] ^= 1;
    let original = native.execution.clone();
    let refusal =
        attach_sandbox_settlement(&mut native.execution, &native.frame, native.transfer_root)
            .unwrap_err();
    assert_eq!(refusal.result_code(), NON_CANONICAL);
    assert_eq!(refusal.to_string(), "sandbox usage receipt digest mismatch");
    assert_eq!(native.execution, original);
}

#[test]
fn flipped_canonical_byte_is_non_canonical() {
    let mut native = native_settlement();
    // Changing the lease keeps the body structurally valid; only digest
    // authentication can detect that this is not the producer's receipt.
    native.frame[b"LayerX/programs/sandbox/usage-receipt/v4\0".len()] ^= 1;
    let original = native.execution.clone();
    let refusal =
        attach_sandbox_settlement(&mut native.execution, &native.frame, native.transfer_root)
            .unwrap_err();
    assert_eq!(refusal.result_code(), NON_CANONICAL);
    assert_eq!(refusal.to_string(), "sandbox usage receipt digest mismatch");
    assert_eq!(native.execution, original);
}

#[test]
fn every_short_frame_is_non_canonical_without_mutating_execution() {
    let native = native_settlement();
    for length in 0..32 {
        let mut execution = native.execution.clone();
        let refusal = attach_sandbox_settlement(
            &mut execution,
            &native.frame[..length],
            native.transfer_root,
        )
        .unwrap_err();
        assert_eq!(refusal.result_code(), NON_CANONICAL, "prefix {length}");
        assert_eq!(refusal.to_string(), "short sandbox usage receipt frame");
        assert_eq!(execution, native.execution);
    }
}

#[test]
fn authenticated_malformed_body_has_a_distinct_non_canonical_message() {
    let mut native = native_settlement();
    native.frame.truncate(native.frame.len() - 32);
    native.frame[0] ^= 1;
    let digest = hash_bytes(HashAlgorithm::Sha256, &native.frame).unwrap();
    SandboxSettlement::append_receipt_digest(&mut native.frame, digest);
    let original = native.execution.clone();
    let refusal =
        attach_sandbox_settlement(&mut native.execution, &native.frame, native.transfer_root)
            .unwrap_err();
    assert_eq!(refusal.result_code(), NON_CANONICAL);
    assert_eq!(
        refusal.to_string(),
        "malformed canonical sandbox usage receipt body"
    );
    assert_eq!(native.execution, original);
}
