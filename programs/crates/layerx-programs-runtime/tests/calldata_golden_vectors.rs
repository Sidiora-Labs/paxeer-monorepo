//! Golden vector tests for calldata encoding.
//!
//! This test suite validates the frozen `LayerX` canonical calldata encoding
//! against comprehensive golden vectors covering every type, boundary case,
//! rejection scenario, and the empty call.

use layerx_programs_runtime::abi::codec::{Calldata, CodecError};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::Path;

#[derive(Debug, Deserialize, Serialize)]
struct TestVector {
    description: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    hex: Option<String>,
    expected: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    note: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    hex_generator: Option<String>,
}

fn hex_decode(hex: &str) -> Vec<u8> {
    let cleaned = hex.replace(' ', "");
    (0..cleaned.len())
        .step_by(2)
        .map(|i| {
            u8::from_str_radix(&cleaned[i..i + 2], 16)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "invalid hex"))
        })
        .collect()
}

fn load_vectors(path: &Path) -> Vec<TestVector> {
    let content = fs::read_to_string(path)
        .unwrap_or_else(|error| panic!("failed to read {}: {}", path.display(), error));
    serde_json::from_str(&content)
        .unwrap_or_else(|error| panic!("failed to parse {}: {}", path.display(), error))
}

fn generate_hex(generator: &str) -> Vec<u8> {
    match generator {
        "layerx_convention + bytes_tag + length(4096) + repeat(0x42, 4096)" => {
            let mut bytes = vec![0x01, 0x20, 0x00, 0x00, 0x10, 0x00];
            bytes.extend_from_slice(&[0x42; 4096]);
            bytes
        }
        _ => panic!("Unknown calldata hex_generator syntax: {generator}"),
    }
}

fn run_vector(vector: &TestVector) -> bool {
    let bytes = match (&vector.hex, &vector.hex_generator) {
        (Some(hex), None) => hex_decode(hex),
        (None, Some(generator)) => generate_hex(generator),
        _ => panic!(
            "Vector '{}' must have exactly one literal or generator",
            vector.description
        ),
    };
    let result = Calldata::from_bytes(&bytes);

    match vector.expected.as_str() {
        "pass" => {
            if let Err(error) = result {
                panic!(
                    "Vector '{}' expected to pass but failed: {:?}\nBytes: {:02x?}",
                    vector.description, error, bytes
                );
            }
        }
        "reject" => {
            assert!(
                result.is_err(),
                "Vector '{}' expected to reject but passed\nBytes: {:02x?}",
                vector.description,
                bytes
            );
            if let Some(expected_error) = &vector.error {
                let actual_error = format!(
                    "{:?}",
                    result.err().unwrap_or_else(|| panic!("expected error"))
                );
                assert!(actual_error.contains(expected_error),
                        "Vector '{}' rejected with wrong error\nExpected: {}\nActual: {}\nBytes: {:02x?}",
                        vector.description, expected_error, actual_error, bytes
                    );
            }
        }
        other => panic!("Invalid expected value: {other}"),
    }

    true
}

fn test_vector_directory(dir: &str, expected_executed: usize, expected_skipped: usize) {
    let base = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../tests/vectors/calldata")
        .join(dir);
    assert!(
        base.exists(),
        "Vector directory does not exist: {}",
        base.display()
    );

    let mut executed = 0;
    let mut skipped = 0;
    for entry in
        fs::read_dir(&base).unwrap_or_else(|error| panic!("{}: {error:?}", "read directory"))
    {
        let entry = entry.unwrap_or_else(|error| panic!("{}: {error:?}", "directory entry"));
        let path = entry.path();
        if path.extension().and_then(|s| s.to_str()) == Some("json") {
            let vectors = load_vectors(&path);
            for vector in &vectors {
                if run_vector(vector) {
                    executed += 1;
                } else {
                    skipped += 1;
                }
            }
        }
    }

    assert!(executed > 0, "No vectors executed in {}", base.display());
    assert_eq!(
        executed,
        expected_executed,
        "Executed vector count mismatch in {} ({} skipped)",
        base.display(),
        skipped
    );
    assert_eq!(
        skipped,
        expected_skipped,
        "Skipped vector count mismatch in {} ({} executed)",
        base.display(),
        executed
    );
}

#[test]
fn valid_primitives() {
    test_vector_directory("valid", 43, 0);
}

#[test]
fn invalid_malformed() {
    test_vector_directory("invalid", 22, 0);
}

#[test]
fn boundaries_depth_and_size() {
    test_vector_directory("boundaries", 11, 0);
}

#[test]
fn evm_head_only_layout() {
    test_vector_directory("evm", 10, 0);
}

#[test]
fn empty_calldata_golden() {
    let empty = Calldata::from_bytes(&[]).unwrap_or_else(|error| panic!("{}: {error:?}", "empty"));
    assert!(empty.payload().is_empty());
    assert_eq!(empty.as_bytes(), vec![0x01]);
}

#[test]
fn layerx_convention_frozen() {
    let mut calldata = Calldata::new();
    calldata
        .encode_u32(42)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "encode"));
    let bytes = calldata.as_bytes();
    assert_eq!(bytes, vec![0x01, 0x12, 0x00, 0x00, 0x00, 0x2a]);
    let decoded =
        Calldata::from_bytes(&bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "decode"));
    assert_eq!(decoded.payload(), &[0x12, 0x00, 0x00, 0x00, 0x2a]);
}

#[test]
fn evm_convention_frozen() {
    let evm = Calldata::with_convention(
        layerx_programs_runtime::abi::codec::EncodingConvention::EvmHeadOnly,
    );
    let bytes = evm.as_bytes();
    assert_eq!(bytes, vec![0x02]);
}

#[test]
fn non_canonical_encoding_is_rejected() {
    let invalid_option = vec![0x01, 0x40, 0x03];
    assert_eq!(
        Calldata::from_bytes(&invalid_option)
            .err()
            .unwrap_or_else(|| panic!("expected error")),
        CodecError::InvalidOption
    );
}

#[test]
fn nesting_depth_limit_is_enforced() {
    let mut deep = vec![0x01];
    for _ in 0..17 {
        deep.extend_from_slice(&[0x30, 0x00, 0x00, 0x00, 0x01]);
    }
    deep.extend_from_slice(&[0x10, 0xff]);
    assert_eq!(
        Calldata::from_bytes(&deep)
            .err()
            .unwrap_or_else(|| panic!("expected error")),
        CodecError::NestingTooDeep
    );
}

#[test]
fn input_size_limit_is_enforced() {
    let oversized = vec![0u8; layerx_programs_runtime::abi::codec::MAX_CALLDATA_BYTES + 1];
    assert_eq!(
        Calldata::from_bytes(&oversized)
            .err()
            .unwrap_or_else(|| panic!("expected error")),
        CodecError::InputTooLarge
    );
}

#[test]
fn truncated_integer_is_rejected() {
    let truncated_u32 = vec![0x01, 0x12, 0x00, 0x00];
    assert_eq!(
        Calldata::from_bytes(&truncated_u32)
            .err()
            .unwrap_or_else(|| panic!("expected error")),
        CodecError::Truncated
    );
}

#[test]
fn invalid_type_tag_is_rejected() {
    let invalid = vec![0x01, 0xff];
    assert_eq!(
        Calldata::from_bytes(&invalid)
            .err()
            .unwrap_or_else(|| panic!("expected error")),
        CodecError::InvalidType
    );
}

#[test]
fn invalid_convention_tag_is_rejected() {
    let invalid = vec![0xff, 0x10, 0x42];
    assert_eq!(
        Calldata::from_bytes(&invalid)
            .err()
            .unwrap_or_else(|| panic!("expected error")),
        CodecError::InvalidConvention
    );
}

#[test]
fn evm_misalignment_is_rejected() {
    let misaligned = vec![0x02, 0x42];
    assert_eq!(
        Calldata::from_bytes(&misaligned)
            .err()
            .unwrap_or_else(|| panic!("expected error")),
        CodecError::NonCanonical
    );
}

#[test]
fn canonical_u8_roundtrip() {
    let mut calldata = Calldata::new();
    calldata
        .encode_u8(255)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "encode"));
    let bytes = calldata.as_bytes();
    let decoded =
        Calldata::from_bytes(&bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "decode"));
    assert_eq!(decoded.payload(), &[0x10, 0xff]);
}

#[test]
fn canonical_bytes_roundtrip() {
    let mut calldata = Calldata::new();
    calldata
        .encode_bytes(b"test")
        .unwrap_or_else(|error| panic!("{}: {error:?}", "encode"));
    let bytes = calldata.as_bytes();
    let decoded =
        Calldata::from_bytes(&bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "decode"));
    assert_eq!(
        decoded.payload(),
        &[0x20, 0x00, 0x00, 0x00, 0x04, b't', b'e', b's', b't']
    );
}

#[test]
fn canonical_option_none_roundtrip() {
    let mut calldata = Calldata::new();
    calldata
        .encode_option_none()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "encode"));
    let bytes = calldata.as_bytes();
    let decoded =
        Calldata::from_bytes(&bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "decode"));
    assert_eq!(decoded.payload(), &[0x40, 0x00]);
}

#[test]
fn canonical_option_some_roundtrip() {
    let mut calldata = Calldata::new();
    calldata
        .begin_option_some()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "begin"));
    calldata
        .encode_u8(42)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "encode"));
    let bytes = calldata.as_bytes();
    let decoded =
        Calldata::from_bytes(&bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "decode"));
    assert_eq!(decoded.payload(), &[0x40, 0x01, 0x10, 0x2a]);
}

#[test]
fn canonical_union_roundtrip() {
    let mut calldata = Calldata::new();
    calldata
        .begin_union(7)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "begin"));
    calldata
        .encode_u32(999)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "encode"));
    let bytes = calldata.as_bytes();
    let decoded =
        Calldata::from_bytes(&bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "decode"));
    assert_eq!(
        decoded.payload(),
        &[0x50, 0x00, 0x00, 0x00, 0x07, 0x12, 0x00, 0x00, 0x03, 0xe7]
    );
}

#[test]
fn canonical_fixed_array_roundtrip() {
    let mut calldata = Calldata::new();
    calldata
        .begin_fixed_array(2)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "begin"));
    calldata
        .encode_u8(1)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "first"));
    calldata
        .encode_u8(2)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "second"));
    let bytes = calldata.as_bytes();
    let decoded =
        Calldata::from_bytes(&bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "decode"));
    assert_eq!(
        decoded.payload(),
        &[0x30, 0x00, 0x00, 0x00, 0x02, 0x10, 0x01, 0x10, 0x02]
    );
}

#[test]
fn canonical_variable_array_roundtrip() {
    let mut calldata = Calldata::new();
    calldata
        .begin_variable_array(1)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "begin"));
    calldata
        .encode_bytes(b"x")
        .unwrap_or_else(|error| panic!("{}: {error:?}", "element"));
    let bytes = calldata.as_bytes();
    let decoded =
        Calldata::from_bytes(&bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "decode"));
    assert_eq!(
        decoded.payload(),
        &[0x31, 0x00, 0x00, 0x00, 0x01, 0x20, 0x00, 0x00, 0x00, 0x01, b'x']
    );
}

#[test]
fn generated_boundary_decodes_exact_published_bytes() {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../tests/vectors/calldata/boundaries/depth_and_size.json");
    let vectors = load_vectors(&path);
    let generated: Vec<_> = vectors
        .iter()
        .filter(|vector| vector.hex_generator.is_some())
        .collect();
    assert_eq!(generated.len(), 1);
    let vector = generated[0];
    assert_eq!(vector.expected, "pass");
    assert!(run_vector(vector));
    let bytes = generate_hex(vector.hex_generator.as_deref().unwrap());
    assert_eq!(bytes.len(), 4102);
    assert_eq!(&bytes[..6], &[0x01, 0x20, 0, 0, 0x10, 0]);
    assert_eq!(&bytes[6..], &[0x42; 4096]);
    let decoded = Calldata::from_bytes(&bytes).unwrap();
    assert_eq!(decoded.payload(), &bytes[1..]);
    assert_eq!(decoded.as_bytes(), bytes);
    let mut producer = Calldata::new();
    producer.encode_bytes(&[0x42; 4096]).unwrap();
    assert_eq!(producer.as_bytes(), bytes);
}

#[test]
fn unknown_generator_syntax_fails_explicitly() {
    for syntax in [
        "unknown",
        "layerx_convention + bytes_tag + length(4096) + repeat(0x42, 4095)",
        "layerx_convention + bytes_tag + length(4294967295) + repeat(0x42, 4294967295)",
    ] {
        let failure = std::panic::catch_unwind(|| generate_hex(syntax)).unwrap_err();
        let message = failure.downcast_ref::<String>().unwrap();
        assert!(message.contains("Unknown calldata hex_generator syntax:"));
        assert!(message.contains(syntax));
    }
}
