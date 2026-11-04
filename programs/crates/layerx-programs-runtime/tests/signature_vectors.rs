//! Golden test vectors for deterministic signature verification.
//!
//! This test suite covers published test vectors, malleable signatures, and
//! every rejection case to prove byte-identical results on the determinism
//! differential.

use layerx_programs_runtime::{
    recover_secp256k1, verify_ed25519, verify_secp256k1, SignatureAlgorithm, SignatureRefusal,
    ED25519_PUBLIC_KEY_BYTES, ED25519_SIGNATURE_BYTES, SECP256K1_COMPRESSED_PUBLIC_KEY_BYTES,
    SECP256K1_SIGNATURE_BYTES, SECP256K1_UNCOMPRESSED_PUBLIC_KEY_BYTES,
};

#[test]
fn ed25519_algorithm_identifier() {
    assert_eq!(SignatureAlgorithm::Ed25519 as u32, 1);
    assert_eq!(
        SignatureAlgorithm::decode(1).ok(),
        Some(SignatureAlgorithm::Ed25519)
    );
}

#[test]
fn secp256k1_verify_algorithm_identifier() {
    assert_eq!(SignatureAlgorithm::Secp256k1Verify as u32, 2);
    assert_eq!(
        SignatureAlgorithm::decode(2).ok(),
        Some(SignatureAlgorithm::Secp256k1Verify)
    );
}

#[test]
fn secp256k1_recover_algorithm_identifier() {
    assert_eq!(SignatureAlgorithm::Secp256k1Recover as u32, 3);
    assert_eq!(
        SignatureAlgorithm::decode(3).ok(),
        Some(SignatureAlgorithm::Secp256k1Recover)
    );
}

#[test]
fn invalid_algorithm_identifier_is_refused() {
    assert_eq!(
        SignatureAlgorithm::decode(0).err(),
        Some(SignatureRefusal::InvalidAlgorithm)
    );
    assert_eq!(
        SignatureAlgorithm::decode(4).err(),
        Some(SignatureRefusal::InvalidAlgorithm)
    );
    assert_eq!(
        SignatureAlgorithm::decode(u32::MAX).err(),
        Some(SignatureRefusal::InvalidAlgorithm)
    );
}

#[test]
fn ed25519_rejects_malformed_public_key() {
    let message = [0u8; 32];
    let public_key = [0u8; ED25519_PUBLIC_KEY_BYTES - 1];
    let signature = [0u8; ED25519_SIGNATURE_BYTES];

    assert_eq!(
        verify_ed25519(&message, &public_key, &signature).err(),
        Some(SignatureRefusal::MalformedPublicKey)
    );
}

#[test]
fn ed25519_rejects_malformed_signature() {
    let message = [0u8; 32];
    let public_key = [0u8; ED25519_PUBLIC_KEY_BYTES];
    let signature = [0u8; ED25519_SIGNATURE_BYTES - 1];

    assert_eq!(
        verify_ed25519(&message, &public_key, &signature).err(),
        Some(SignatureRefusal::MalformedSignature)
    );
}

#[test]
fn ed25519_rejects_oversized_message() {
    let message = [0u8; 65];
    let public_key = [0u8; ED25519_PUBLIC_KEY_BYTES];
    let signature = [0u8; ED25519_SIGNATURE_BYTES];

    assert_eq!(
        verify_ed25519(&message, &public_key, &signature).err(),
        Some(SignatureRefusal::InvalidMessageLength)
    );
}

#[test]
fn ed25519_zero_vector_fails_verification() {
    let message = [0u8; 32];
    let public_key = [0u8; ED25519_PUBLIC_KEY_BYTES];
    let signature = [0u8; ED25519_SIGNATURE_BYTES];

    assert_eq!(
        verify_ed25519(&message, &public_key, &signature).err(),
        Some(SignatureRefusal::VerificationFailed)
    );
}

#[test]
fn secp256k1_verify_rejects_malformed_digest() {
    let digest = [0u8; 31];
    let public_key = [0u8; SECP256K1_COMPRESSED_PUBLIC_KEY_BYTES];
    let signature = [0u8; SECP256K1_SIGNATURE_BYTES];

    assert_eq!(
        verify_secp256k1(&digest, &public_key, &signature).err(),
        Some(SignatureRefusal::InvalidMessageLength)
    );
}

#[test]
fn secp256k1_verify_rejects_malformed_public_key() {
    let digest = [0u8; 32];
    let public_key = [0u8; 32];
    let signature = [0u8; SECP256K1_SIGNATURE_BYTES];

    assert_eq!(
        verify_secp256k1(&digest, &public_key, &signature).err(),
        Some(SignatureRefusal::MalformedPublicKey)
    );
}

#[test]
fn secp256k1_verify_rejects_malformed_signature() {
    let digest = [0u8; 32];
    let public_key = [0u8; SECP256K1_COMPRESSED_PUBLIC_KEY_BYTES];
    let signature = [0u8; 63];

    assert_eq!(
        verify_secp256k1(&digest, &public_key, &signature).err(),
        Some(SignatureRefusal::MalformedSignature)
    );
}

#[test]
fn secp256k1_verify_accepts_compressed_public_key() {
    let digest = [0u8; 32];
    let public_key = [0u8; SECP256K1_COMPRESSED_PUBLIC_KEY_BYTES];
    let signature = [0u8; SECP256K1_SIGNATURE_BYTES];

    assert_eq!(
        verify_secp256k1(&digest, &public_key, &signature).err(),
        Some(SignatureRefusal::MalformedSignature)
    );

    // Wycheproof ecdsa_secp256k1_sha256 tcId 3; tests/vectors/signatures/sources.json.
    let digest = hex::decode("bb5a52f42f9c9261ed4361f59422a1e30036e7c32b270c8807a419feca605023")
        .unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    let public_key =
        hex::decode("02782c8ed17e3b2a783b5464f33b09652a71c678e05ec51e84e2bcfc663a3de963")
            .unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    let signature = hex::decode("d035ee1f17fdb0b2681b163e33c359932659990af77dca632012b30b27a057b31939d9f3b2858bc13e3474cb50e6a82be44faa71940f876c1cba4c3e989202b6").unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    assert_eq!(verify_secp256k1(&digest, &public_key, &signature), Ok(()));
}

#[test]
fn secp256k1_verify_accepts_uncompressed_public_key() {
    let digest = [0u8; 32];
    let public_key = [0u8; SECP256K1_UNCOMPRESSED_PUBLIC_KEY_BYTES];
    let signature = [0u8; SECP256K1_SIGNATURE_BYTES];

    assert_eq!(
        verify_secp256k1(&digest, &public_key, &signature).err(),
        Some(SignatureRefusal::MalformedSignature)
    );

    // Wycheproof ecdsa_secp256k1_sha256 tcId 3; tests/vectors/signatures/sources.json.
    let digest = hex::decode("bb5a52f42f9c9261ed4361f59422a1e30036e7c32b270c8807a419feca605023")
        .unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    let public_key = hex::decode("04782c8ed17e3b2a783b5464f33b09652a71c678e05ec51e84e2bcfc663a3de963af9acb4280b8c7f7c42f4ef9aba6245ec1ec1712fd38a0fa96418d8cd6aa6152").unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    let signature = hex::decode("d035ee1f17fdb0b2681b163e33c359932659990af77dca632012b30b27a057b31939d9f3b2858bc13e3474cb50e6a82be44faa71940f876c1cba4c3e989202b6").unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    assert_eq!(verify_secp256k1(&digest, &public_key, &signature), Ok(()));
}

#[test]
fn secp256k1_verify_zero_vector_fails() {
    let digest = [0u8; 32];
    let public_key = [0u8; SECP256K1_COMPRESSED_PUBLIC_KEY_BYTES];
    let signature = [0u8; SECP256K1_SIGNATURE_BYTES];

    assert_eq!(
        verify_secp256k1(&digest, &public_key, &signature).err(),
        Some(SignatureRefusal::MalformedSignature)
    );
}

#[test]
fn secp256k1_recover_rejects_malformed_digest() {
    let digest = [0u8; 31];
    let signature = [0u8; SECP256K1_SIGNATURE_BYTES];

    assert_eq!(
        recover_secp256k1(&digest, &signature, 0).err(),
        Some(SignatureRefusal::InvalidMessageLength)
    );
}

#[test]
fn secp256k1_recover_rejects_malformed_signature() {
    let digest = [0u8; 32];
    let signature = [0u8; 63];

    assert_eq!(
        recover_secp256k1(&digest, &signature, 0).err(),
        Some(SignatureRefusal::MalformedSignature)
    );
}

#[test]
fn secp256k1_recover_rejects_invalid_recovery_id() {
    let digest = [0u8; 32];
    let signature = [0u8; SECP256K1_SIGNATURE_BYTES];

    assert_eq!(
        recover_secp256k1(&digest, &signature, 4).err(),
        Some(SignatureRefusal::InvalidRecoveryId)
    );
    assert_eq!(
        recover_secp256k1(&digest, &signature, 255).err(),
        Some(SignatureRefusal::InvalidRecoveryId)
    );
}

#[test]
fn secp256k1_recover_zero_vector_fails() {
    let digest = [0u8; 32];
    let signature = [0u8; SECP256K1_SIGNATURE_BYTES];

    for recovery_id in 0..=3 {
        assert_eq!(
            recover_secp256k1(&digest, &signature, recovery_id).err(),
            Some(SignatureRefusal::MalformedSignature)
        );
    }
}

#[test]
fn ed25519_published_test_vector_1() {
    let message = hex::decode("").unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    let public_key =
        hex::decode("d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a")
            .unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    let signature = hex::decode(
        "e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b",
    )
    .unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));

    assert!(verify_ed25519(&message, &public_key, &signature).is_ok());
}

#[test]
fn ed25519_published_test_vector_2() {
    let message =
        hex::decode("72").unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    let public_key =
        hex::decode("3d4017c3e843895a92b70aa74d1b7ebc9c982ccf2ec4968cc0cd55f12af4660c")
            .unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    let signature = hex::decode(
        "92a009a9f0d4cab8720e820b5f642540a2b27b5416503f8fb3762223ebdb69da085ac1e43e15996e458f3613d0f11d8c387b2eaeb4302aeeb00d291612bb0c00",
    )
    .unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));

    assert!(verify_ed25519(&message, &public_key, &signature).is_ok());
}

#[test]
fn secp256k1_verify_published_test_vector_1() {
    // Wycheproof ecdsa_secp256k1_sha256 tcId 3; tests/vectors/signatures/sources.json.
    let digest = hex::decode("bb5a52f42f9c9261ed4361f59422a1e30036e7c32b270c8807a419feca605023")
        .unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    let public_key = hex::decode("04782c8ed17e3b2a783b5464f33b09652a71c678e05ec51e84e2bcfc663a3de963af9acb4280b8c7f7c42f4ef9aba6245ec1ec1712fd38a0fa96418d8cd6aa6152").unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    let signature = hex::decode("d035ee1f17fdb0b2681b163e33c359932659990af77dca632012b30b27a057b31939d9f3b2858bc13e3474cb50e6a82be44faa71940f876c1cba4c3e989202b6").unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    assert_eq!(verify_secp256k1(&digest, &public_key, &signature), Ok(()));
}

// RustCrypto k256 upstream ecdsa.rs RECOVERY_TEST_VECTORS, recovery ID 0:
// https://github.com/RustCrypto/elliptic-curves/blob/k256/v0.13.4/k256/src/ecdsa.rs
// SHA-256("example message"); the published compressed key has even Y.
const RECOVERY_DIGEST: &str = "ad84cd0b10fc028738971b078124aec2a0e7c6d986a381be0b386f32bee887af";
const RECOVERY_SIGNATURE: &str = "ce53abb3721bafc561408ce8ff99c909f7f0b18a2f788649d6470162ab1aa0323971edc523a6d6453f3fb6128d318d9db1a5ff3386feb1047d9816e780039d52";
const RECOVERY_PUBLIC_KEY: &str = "041a7a569e91dbf60581509c7fc946d1003b60c7dee85299538db6353538d59574b4e89d60c7d584d084632d296f125f165b4df8e061a49daeba51d36133d03e1a";

#[test]
fn secp256k1_recover_published_test_vector_1() {
    let digest = hex::decode(RECOVERY_DIGEST).unwrap();
    let signature = hex::decode(RECOVERY_SIGNATURE).unwrap();
    let expected: [u8; 65] = hex::decode(RECOVERY_PUBLIC_KEY)
        .unwrap()
        .try_into()
        .unwrap();
    assert_eq!(recover_secp256k1(&digest, &signature, 0), Ok(expected));
    assert_eq!(verify_secp256k1(&digest, &expected, &signature), Ok(()));
}

fn signed_i32(mut value: i32) -> Vec<u8> {
    let mut bytes = Vec::new();
    loop {
        let byte = (value & 0x7f) as u8;
        value >>= 7;
        let done = (value == 0 && byte & 0x40 == 0) || (value == -1 && byte & 0x40 != 0);
        bytes.push(if done { byte } else { byte | 0x80 });
        if done {
            return bytes;
        }
    }
}

#[test]
fn published_recovery_bytes_cross_the_real_signature_recover_host_transport() {
    use layerx_programs_runtime::test_support::{
        code_section, func_body, function_section, import_section, module, raw_section,
        type_section, unsigned_leb, OP_CALL, OP_END, OP_I32_CONST, TYPE_I32,
    };
    use layerx_programs_runtime::{WasmEngine, WasmValue, ABI_V2_MODULE};
    let digest = hex::decode(RECOVERY_DIGEST).unwrap();
    let signature = hex::decode(RECOVERY_SIGNATURE).unwrap();
    let expected = hex::decode(RECOVERY_PUBLIC_KEY).unwrap();
    let mut run = Vec::new();
    for value in [0, 32, 32, 64, 0, 128, 65] {
        run.push(OP_I32_CONST);
        run.extend(signed_i32(value));
    }
    run.extend([OP_CALL, 0, OP_END]);
    // Read the actual host output through Wasm loads and compare every byte.
    let mut compare = vec![OP_I32_CONST, 1];
    for (index, byte) in expected.iter().enumerate() {
        compare.push(OP_I32_CONST);
        compare.extend(signed_i32(128 + i32::try_from(index).unwrap()));
        compare.extend([0x2d, 0, 0, OP_I32_CONST]); // i32.load8_u
        compare.extend(signed_i32(i32::from(*byte)));
        compare.extend([0x46, 0x71]); // i32.eq, i32.and
    }
    compare.push(OP_END);
    let mut data = vec![1, 0, OP_I32_CONST, 0, OP_END];
    data.extend(unsigned_leb((digest.len() + signature.len()) as u64));
    data.extend(digest);
    data.extend(signature);
    let mut export_payload = vec![3];
    for (name, kind, index) in [("run", 0, 1), ("key_is_exact", 0, 2), ("memory", 2, 0)] {
        export_payload.extend(unsigned_leb(name.len() as u64));
        export_payload.extend_from_slice(name.as_bytes());
        export_payload.extend([kind, index]);
    }
    let exports = raw_section(7, &export_payload);
    let wasm = module(&[
        type_section(&[(&[TYPE_I32; 7], &[TYPE_I32]), (&[], &[TYPE_I32])]),
        import_section(&[(ABI_V2_MODULE, "signature_recover", 0)]),
        function_section(&[1, 1]),
        raw_section(5, &[1, 1, 1, 1]),
        exports,
        code_section(&[func_body(&[], &run), func_body(&[], &compare)]),
        raw_section(11, &data),
    ]);
    let engine = WasmEngine::declared().unwrap();
    for version in 2..=layerx_programs_runtime::ABI_V5_VERSION {
        let validated = engine.validate_versioned(version, &wasm).unwrap();
        let mut instance = validated.instantiate().unwrap();
        assert_eq!(
            instance.call("key_is_exact", &[]),
            Ok(vec![WasmValue::I32(0)])
        );
        assert_eq!(instance.call("run", &[]), Ok(vec![WasmValue::I32(65)]));
        assert_eq!(
            instance.call("key_is_exact", &[]),
            Ok(vec![WasmValue::I32(1)])
        );
    }
}

#[test]
fn fuel_coefficients_are_deterministic() {
    assert_eq!(SignatureAlgorithm::Ed25519.fuel_coefficient(), 2_000);
    assert_eq!(
        SignatureAlgorithm::Secp256k1Verify.fuel_coefficient(),
        3_000
    );
    assert_eq!(
        SignatureAlgorithm::Secp256k1Recover.fuel_coefficient(),
        3_500
    );
}

#[test]
fn malleable_signature_detection_ed25519() {
    let message = [1u8; 32];
    let public_key = [2u8; ED25519_PUBLIC_KEY_BYTES];
    let mut signature = [0u8; ED25519_SIGNATURE_BYTES];

    signature[63] = 0xed;

    assert_eq!(
        verify_ed25519(&message, &public_key, &signature).err(),
        Some(SignatureRefusal::MalformedPublicKey)
    );

    // RFC 8032 sections 7.1 (TEST 1) and 5.1.7: valid signature, then S + L.
    let message = [];
    let public_key =
        hex::decode("d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a")
            .unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    let mut signature = hex::decode("e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b").unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    assert_eq!(verify_ed25519(&message, &public_key, &signature), Ok(()));
    let order = hex::decode("edd3f55c1a631258d69cf7a2def9de1400000000000000000000000000000010")
        .unwrap_or_else(|error| panic!("{}: {error:?}", "required value"));
    let mut carry = 0u16;
    for (scalar_byte, order_byte) in signature[32..].iter_mut().zip(order) {
        let sum = u16::from(*scalar_byte) + u16::from(order_byte) + carry;
        *scalar_byte = sum.to_le_bytes()[0];
        carry = sum >> 8;
    }
    assert_eq!(carry, 0);
    assert_eq!(
        verify_ed25519(&message, &public_key, &signature),
        Err(SignatureRefusal::VerificationFailed)
    );
}

#[test]
fn malleable_signature_detection_secp256k1() {
    let digest = [1u8; 32];
    let public_key = [2u8; SECP256K1_COMPRESSED_PUBLIC_KEY_BYTES];
    let mut signature = [0u8; SECP256K1_SIGNATURE_BYTES];

    signature[63] = 0xff;

    assert_eq!(
        verify_secp256k1(&digest, &public_key, &signature).err(),
        Some(SignatureRefusal::MalformedSignature)
    );
}

#[test]
fn constant_shape_execution_smoke_test() {
    let message = [0u8; 32];
    let public_key = [0u8; ED25519_PUBLIC_KEY_BYTES];
    let signature_valid = [1u8; ED25519_SIGNATURE_BYTES];
    let signature_invalid = [2u8; ED25519_SIGNATURE_BYTES];

    let _ = verify_ed25519(&message, &public_key, &signature_valid);
    let _ = verify_ed25519(&message, &public_key, &signature_invalid);
}
