use std::fs;
use std::path::{Path, PathBuf};
#[path = "../../../tests/support/rustc.rs"]
mod rustc;

use layerx_types::vectors::checkpoint::{load_checkpoint_vectors, CHECKPOINT_VECTOR_CASES};
use layerx_wire::hash::{checkpoint_attestation_digest, checkpoint_id, Domain};
use layerx_wire::receipt::{decode_batch_header, encode_batch_header};

const ALL_DOMAINS: [Domain; 20] = [
    Domain::ActivityId,
    Domain::PayloadHash,
    Domain::SignaturePreimage,
    Domain::AuthorityHash,
    Domain::ContextHash,
    Domain::MerkleLeaf,
    Domain::MerkleInternal,
    Domain::BatchHeader,
    Domain::Receipt,
    Domain::CheckpointCertificate,
    Domain::AccountId,
    Domain::DidId,
    Domain::EvmPayoutBinding,
    Domain::StateLeaf,
    Domain::StateNode,
    Domain::StateRootChain,
    Domain::Snapshot,
    Domain::DaChunk,
    Domain::DaChallenge,
    Domain::GuarantorAttestation,
];

fn repository_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..")
}

#[test]
fn raw_text_and_debug_values_cannot_enter_consensus_hashing() {
    let dependency_dir = std::env::current_exe()
        .ok()
        .and_then(|path| path.parent().map(Path::to_path_buf));
    let Some(dependency_dir) = dependency_dir else {
        panic!("dependency directory unavailable");
    };
    let rlib = fs::read_dir(&dependency_dir).ok().and_then(|entries| {
        entries
            .filter_map(Result::ok)
            .map(|entry| entry.path())
            .filter(|path| {
                path.file_name().is_some_and(|name| {
                    name.to_string_lossy().starts_with("liblayerx_wire-")
                        && path
                            .extension()
                            .is_some_and(|extension| extension == "rlib")
                })
            })
            .max_by_key(|path| {
                fs::metadata(path)
                    .and_then(|metadata| metadata.modified())
                    .ok()
            })
    });
    let Some(rlib) = rlib else {
        panic!("layerx-wire rlib unavailable");
    };
    let source = std::env::temp_dir().join(format!("layerx-hash-gate-{}.rs", std::process::id()));
    let binary = source.with_extension("bin");
    assert!(fs::write(
        &source,
        "extern crate layerx_wire;\nuse layerx_wire::hash::{domain, Domain};\nfn main() { let _ = domain(Domain::ActivityId, b\"debug text\"); }\n",
    )
    .is_ok());
    let output = rustc::command(&dependency_dir)
        .arg("--edition=2021")
        .arg(&source)
        .arg("--extern")
        .arg(format!("layerx_wire={}", rlib.display()))
        .arg("-L")
        .arg(format!("dependency={}", dependency_dir.display()))
        .arg("-o")
        .arg(&binary)
        .output();
    let _ = fs::remove_file(source);
    let _ = fs::remove_file(binary);
    let Ok(output) = output else {
        panic!("rustc unavailable for hash input gate");
    };
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("expected `&CanonicalBytes`"));
}

#[test]
fn checkpoint_domain_tags_are_the_declared_version_two_tags() {
    assert_eq!(
        Domain::CheckpointCertificate.tag(),
        b"LXP/v2/checkpoint-certificate\0"
    );
    assert_eq!(
        Domain::GuarantorAttestation.tag(),
        b"LXP/v2/guarantor-attestation\0"
    );
    for purpose in ALL_DOMAINS {
        let tag = purpose.tag();
        assert!(
            !tag.starts_with(b"LXP/v1/checkpoint-certificate")
                && !tag.starts_with(b"LXP/v1/guarantor-attestation"),
            "{purpose:?} still carries a version 1 checkpoint tag"
        );
    }
}

#[test]
fn checkpoint_vectors_pin_header_encoding_and_identities() {
    let vectors = load_checkpoint_vectors(&repository_root())
        .unwrap_or_else(|error| panic!("checkpoint vectors failed to load: {error:?}"));
    assert_eq!(vectors.len(), CHECKPOINT_VECTOR_CASES.len());
    for vector in &vectors {
        let name = vector.case_name.as_str();
        assert_eq!(vector.header.bytes.len(), 354, "{name}");
        assert!(
            vector
                .header
                .bytes
                .starts_with(&[0x00, 0x02, 0x17, 0x01, 0x0f]),
            "{name}: header prefix"
        );
        let header = decode_batch_header(&vector.header.bytes)
            .unwrap_or_else(|error| panic!("{name}: header decode failed: {error:?}"));
        assert_eq!(header.protocol_version(), vector.header.protocol_version);
        assert_eq!(header.protocol_version(), 2);
        assert_eq!(header.network_id(), vector.header.network_id);
        assert_eq!(header.epoch(), vector.header.epoch);
        assert_eq!(header.batch_number(), vector.header.batch_number);
        assert_eq!(header.first_sequence(), vector.header.first_sequence);
        assert_eq!(header.last_sequence(), vector.header.last_sequence);
        assert_eq!(
            header.previous_state_root(),
            vector.header.previous_state_root
        );
        assert_eq!(
            header.resulting_state_root(),
            vector.header.resulting_state_root
        );
        assert_eq!(
            header.activity_merkle_root(),
            vector.header.activity_merkle_root
        );
        assert_eq!(
            header.receipt_merkle_root(),
            vector.header.receipt_merkle_root
        );
        assert_eq!(header.event_merkle_root(), vector.header.event_merkle_root);
        assert_eq!(
            header.data_availability_root(),
            vector.header.data_availability_root
        );
        assert_eq!(header.oracle_root(), vector.header.oracle_root);
        assert_eq!(header.timestamp_ms(), vector.header.timestamp_ms);
        assert_eq!(header.sequencer_id(), vector.header.sequencer_id);
        assert_eq!(
            encode_batch_header(&header),
            Ok(vector.header.bytes.clone()),
            "{name}"
        );
        assert_eq!(
            checkpoint_id(&vector.header.bytes, &vector.validity_proof),
            Ok(vector.expected_digest),
            "{name}"
        );
        for attestation in &vector.attestations {
            assert_eq!(
                &attestation.message[42..74],
                &vector.expected_digest,
                "{name}"
            );
            assert_eq!(
                &attestation.message[74..106],
                &vector.expected_digest,
                "{name}"
            );
            assert_eq!(
                &attestation.message[106..138],
                &attestation.guarantor_id,
                "{name}"
            );
            assert_eq!(
                attestation.message[181..],
                attestation.attested_at_ms.to_be_bytes(),
                "{name}"
            );
            assert_eq!(
                checkpoint_attestation_digest(&attestation.message),
                Ok(attestation.digest),
                "{name}"
            );
        }
    }
}

#[test]
fn protocol_three_account_ids_match_native_ledger_vectors() {
    use layerx_types::account::AccountId;
    use layerx_wire::hash::{account_id, account_id_for_protocol};
    for (name, expected) in [
        (
            "agent:did:key:alice:main",
            "efc9802f76722dfc48ebfed35bfd8b20dbc2775fe2f027d6cbd595aff1307454",
        ),
        (
            "agent:did:key:alice:budget:daily",
            "6f27c8a878c055eeb4056e5bf32864cbf113748af82e6b72e2060c39b4a18829",
        ),
        (
            "system:fees",
            "dbf940aa4c1f587b73f3b65da0dec92760e0bd0187f1f33c659ea043caebdcdf",
        ),
        (
            "system:treasury",
            "6bf40bc6e0f5ac9535582c71001d6460e2bae3da2aa5f2c92cdeaf23f3579cdb",
        ),
    ] {
        let account = AccountId::parse(name).unwrap_or_else(|error| panic!("{error:?}"));
        let actual =
            account_id_for_protocol(&account, 3).unwrap_or_else(|error| panic!("{error:?}"));
        let mut encoded = String::new();
        for byte in actual {
            use std::fmt::Write as _;
            write!(&mut encoded, "{byte:02x}").unwrap_or_else(|error| panic!("{error}"));
        }
        assert_eq!(encoded, expected);
        for version in [1, 2] {
            assert_eq!(
                account_id_for_protocol(&account, version),
                account_id(&account)
            );
        }
        assert_ne!(account_id_for_protocol(&account, 3), account_id(&account));
        assert!(account_id_for_protocol(&account, 4).is_err());
    }
    for name in ["agent:budget:x:budget:y", "agent:did:budget:x:escrow:y"] {
        let account = AccountId::parse(name).unwrap_or_else(|error| panic!("{error:?}"));
        assert!(account_id_for_protocol(&account, 3).is_ok());
    }
    for name in [
        "agent:ALICE:main",
        "agent:did::alice:main",
        "agent:did:alice:budget:x:y",
        "agent:did:alice:budget:x:budget:y",
        "system:liquidity:x:y",
    ] {
        let account = AccountId::parse(name).unwrap_or_else(|error| panic!("{error:?}"));
        assert!(account_id_for_protocol(&account, 3).is_err());
        assert!(account_id_for_protocol(&account, 2).is_ok());
    }
}

#[test]
fn native_program_batch_identity_binds_tree_and_entire_sequence_range() {
    use layerx_wire::hash::{execution_batch_id, program_execution_batch_id};
    let expected = [
        0x68, 0xc8, 0x78, 0x01, 0x57, 0x45, 0xe7, 0xbf, 0x88, 0xd2, 0xaf, 0x83, 0x42, 0xfd, 0xca,
        0x4d, 0x0c, 0x53, 0x6f, 0x37, 0xe8, 0xab, 0xcb, 0x12, 0xbe, 0x89, 0x88, 0x43, 0xc7, 0x04,
        0xa5, 0xf5,
    ];
    assert_eq!(
        program_execution_batch_id([1; 32], [2; 32], 7, 9, 11),
        Ok(expected)
    );
    for actual in [
        program_execution_batch_id([3; 32], [2; 32], 7, 9, 11),
        program_execution_batch_id([1; 32], [3; 32], 7, 9, 11),
        program_execution_batch_id([1; 32], [2; 32], 8, 9, 11),
        program_execution_batch_id([1; 32], [2; 32], 7, 10, 11),
        program_execution_batch_id([1; 32], [2; 32], 7, 9, 12),
        execution_batch_id([1; 32], [2; 32], 7, 11),
    ] {
        assert!(actual.is_ok());
        assert_ne!(actual, Ok(expected));
    }
}

#[path = "repair_22_33_1523.rs"]
mod native_submission;

#[test]
fn native_transaction_identity_comes_from_the_signed_producer() {
    native_submission::wire_hash_tests_native_identity_and_scope();
}
