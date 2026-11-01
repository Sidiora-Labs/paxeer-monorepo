use std::fs;
use std::path::Path;

use layerx_client::ai_market::{decode_snapshot_binding, AiClientError};
use layerx_programs_ai_market::errors::{BAD_VERSION, NON_CANONICAL};
use layerx_programs_ai_market::queries::{QueryError, BINDING_MAX_BYTES};

#[test]
fn snapshot_binding_v2_decodes_vector() {
    let bad_version = AiClientError::Query(QueryError::Application(BAD_VERSION));
    let noncanonical = AiClientError::Query(QueryError::Application(NON_CANONICAL));
    assert_eq!(decode_snapshot_binding(&[0, 2]), Err(bad_version.clone()));

    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../programs/tests/vectors/snapshot-binding-v2.hex");
    let encoded = fs::read_to_string(&path)
        .unwrap_or_else(|error| panic!("cannot read shared vector {}: {error}", path.display()));
    let hex = encoded.trim_end_matches('\n');
    assert!(!hex.is_empty());
    assert_eq!(hex.len() % 2, 0);
    assert!(hex
        .bytes()
        .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte)));
    let bytes: Vec<u8> = hex
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let digit = |byte: u8| match byte {
                b'0'..=b'9' => byte - b'0',
                b'a'..=b'f' => byte - b'a' + 10,
                _ => panic!("invalid shared vector hex"),
            };
            (digit(pair[0]) << 4) | digit(pair[1])
        })
        .collect();
    assert_eq!(bytes.len(), BINDING_MAX_BYTES);
    assert_eq!(&bytes[..2], &[0, 1]);
    let binding = decode_snapshot_binding(&bytes)
        .unwrap_or_else(|error| panic!("shared v2 binding refused: {error:?}"));
    assert_eq!(binding.block_height, 0x5152_5354_5556_5758);
    assert_eq!(binding.block_hash.as_bytes(), &[0xb2; 32]);
    assert_eq!(binding.app_hash.as_bytes(), &[0xc2; 32]);
    assert_eq!(binding.store_proof_digest.as_bytes(), &[0xd2; 32]);

    // The v2 commit section is the fixed 121-byte suffix, after the content prefix.
    let commit = &bytes[bytes.len() - 121..];
    assert_eq!(binding.block_height.to_be_bytes(), commit[..8]);
    assert_eq!(binding.block_hash.as_bytes(), &commit[8..40]);
    assert_eq!(binding.app_hash.as_bytes(), &commit[40..72]);
    assert_eq!(binding.store_proof_digest.as_bytes(), &commit[72..104]);
    assert_eq!(binding.rank, commit[104]);
    assert_eq!(binding.publication_time_ms.to_be_bytes(), commit[105..113]);
    assert_eq!(&commit[113..], &[0; 8]);

    let mut v1 = bytes[..bytes.len() - 121].to_vec();
    v1.extend_from_slice(&commit[104..]);
    assert_eq!(&v1[..2], &[0, 1]);
    assert_eq!(decode_snapshot_binding(&v1), Err(noncanonical.clone()));
    for version in [0_u16, 2, u16::MAX] {
        let mut unsupported = bytes.clone();
        unsupported[..2].copy_from_slice(&version.to_be_bytes());
        assert_eq!(
            decode_snapshot_binding(&unsupported),
            Err(bad_version.clone())
        );
    }
    for length in 0..bytes.len() {
        assert_eq!(
            decode_snapshot_binding(&bytes[..length]),
            Err(noncanonical.clone())
        );
    }
    let mut trailing = bytes.clone();
    trailing.push(0);
    assert_eq!(
        decode_snapshot_binding(&trailing),
        Err(noncanonical.clone())
    );
    let mut reserved = bytes.clone();
    *reserved
        .last_mut()
        .unwrap_or_else(|| panic!("empty vector")) = 1;
    assert_eq!(
        decode_snapshot_binding(&reserved),
        Err(noncanonical.clone())
    );
    let mut invalid_rank = bytes.clone();
    let rank_offset = invalid_rank.len() - 17;
    invalid_rank[rank_offset] = 5;
    assert_eq!(decode_snapshot_binding(&invalid_rank), Err(noncanonical));
}
