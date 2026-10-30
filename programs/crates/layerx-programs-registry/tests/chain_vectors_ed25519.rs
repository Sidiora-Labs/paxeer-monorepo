#[path = "../src/chain_proof/ed25519.rs"]
mod ed25519;

use std::collections::BTreeSet;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    ChainProofError, ChainProofVerifier, Commit,
};
use sha2::{Digest, Sha256};

const CONSENSUS_VECTOR: &str =
    include_str!("../../../../consensus/crypto/ed25519/testdata/zip215_v1.json");
const CONSENSUS_VECTOR_SHA256: &str =
    "10d7d62cdb0a86d925011df908415a30b9577ca20e554f16a37ec1e07e6adcc8";

fn vector_field<'a>(lines: &mut impl Iterator<Item = &'a str>, name: &str) -> &'a str {
    let line = lines.next().expect("vector field");
    let (key, value) = line.split_once(": ").expect("vector key and value");
    assert_eq!(key, format!("\"{name}\""));
    value.strip_suffix(',').unwrap_or(value)
}

fn vector_string<'a>(lines: &mut impl Iterator<Item = &'a str>, name: &str) -> &'a str {
    vector_field(lines, name)
        .strip_prefix('"')
        .and_then(|value| value.strip_suffix('"'))
        .expect("quoted vector string")
}

fn hex_bytes(value: &str) -> Vec<u8> {
    assert_eq!(value.len() % 2, 0, "whole hex bytes");
    value
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("ASCII hex"), 16)
                .expect("hex vector byte")
        })
        .collect()
}

fn one_validator_commit(
    public_key: [u8; 32],
    signature: [u8; 64],
) -> Result<(ChainProofVerifier, ChainHeader, Commit), ChainProofError> {
    let validator = TrustedValidator::new(public_key, 1);
    let address = validator.address.to_vec();
    let validators = ValidatorSet::new(vec![validator])?;
    let header = ChainHeader {
        version: HeaderVersion { block: 11, app: 1 },
        chain_id: "paxeer-zip215-vector".to_owned(),
        height: 37,
        time: HeaderTime {
            seconds: 1_700_000_000,
            nanos: 123_456_789,
        },
        last_block_id: HeaderBlockId {
            hash: vec![1; 32],
            part_set_total: 1,
            part_set_hash: vec![2; 32],
        },
        last_commit_hash: vec![3; 32],
        data_hash: vec![4; 32],
        validators_hash: validators.hash().to_vec(),
        next_validators_hash: validators.hash().to_vec(),
        consensus_hash: vec![6; 32],
        app_hash: vec![7; 32],
        last_results_hash: vec![8; 32],
        evidence_hash: vec![9; 32],
        proposer_address: address.clone(),
    };
    let commit = Commit {
        height: header.height,
        round: 2,
        block_id: BlockId {
            hash: header.hash()?.to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![10; 32],
            },
        },
        signatures: vec![CommitSig {
            flag: BlockIdFlag::Commit,
            validator_address: address,
            timestamp: Timestamp {
                seconds: 1_700_000_001,
                nanos: 987_654_321,
            },
            signature: signature.to_vec(),
        }],
    };
    Ok((
        ChainProofVerifier {
            chain_id: header.chain_id.clone(),
            validators,
        },
        header,
        commit,
    ))
}

#[test]
fn chain_zip215_matches_consensus_vector() -> Result<(), ChainProofError> {
    assert_eq!(
        format!("{:x}", Sha256::digest(CONSENSUS_VECTOR.as_bytes())),
        CONSENSUS_VECTOR_SHA256,
        "consensus ZIP-215 vector changed"
    );
    let mut lines = CONSENSUS_VECTOR.lines().map(str::trim);
    assert_eq!(lines.next(), Some("{"));
    assert_eq!(vector_field(&mut lines, "version"), "1");
    assert_eq!(vector_string(&mut lines, "bytes_encoding"), "hex");
    assert_eq!(vector_string(&mut lines, "verification"), "ZIP-215");
    assert_eq!(lines.next(), Some("\"cases\": ["));

    let ordinary_key = SigningKey::from_bytes(&Sha256::digest(b"zip215-vector-v1").into());
    let mut names = BTreeSet::new();
    let mut accepted = 0;
    let mut refused = 0;
    loop {
        assert_eq!(lines.next(), Some("{"));
        let name = vector_string(&mut lines, "name");
        assert!(names.insert(name), "duplicate vector case: {name}");
        let public_key: [u8; 32] = hex_bytes(vector_string(&mut lines, "public_key"))
            .try_into()
            .expect("32-byte public key");
        let message = hex_bytes(vector_string(&mut lines, "message"));
        let signature: [u8; 64] = hex_bytes(vector_string(&mut lines, "signature"))
            .try_into()
            .expect("64-byte signature");
        let expected = match vector_field(&mut lines, "verify_signature") {
            "true" => true,
            "false" => false,
            other => panic!("invalid acceptance result for {name}: {other}"),
        };
        assert_eq!(
            ed25519::verify_zip215(&public_key, &message, &signature),
            expected,
            "consensus message: {name}"
        );

        let (verifier, header, mut commit) = one_validator_commit(public_key, signature)?;
        assert_eq!(verifier.validators.validators().len(), 1);
        assert_eq!(verifier.validators.total_power(), 1);
        if matches!(name, "ordinary_signature" | "ordinary_changed_message") {
            assert_eq!(public_key, ordinary_key.verifying_key().to_bytes());
            assert_eq!(
                signature,
                ordinary_key.sign(b"ZIP-215 vector v1").to_bytes(),
                "reproduce the consensus generator's ordinary signature"
            );
            let original_vote = vote_sign_bytes(
                &header.chain_id,
                commit.height,
                commit.round,
                Some(&commit.block_id),
                commit.signatures[0].timestamp,
            )?;
            commit.signatures[0].signature = ordinary_key.sign(&original_vote).to_bytes().to_vec();
            assert!(verifier.verify_head(&header, &commit).is_ok(), "{name}");
            if name == "ordinary_changed_message" {
                assert_eq!(message, b"ZIP-215 vector v2");
                commit.signatures[0].timestamp.nanos += 1;
            } else {
                assert_eq!(message, b"ZIP-215 vector v1");
            }
        }

        let result = verifier.verify_head(&header, &commit);
        assert_eq!(result.is_ok(), expected, "one-validator commit: {name}");
        if expected {
            let head = result.expect("accepted vector commit");
            assert_eq!(head.chain_id(), header.chain_id);
            assert_eq!(head.height(), header.height);
            assert_eq!(head.time_ms(), 1_700_000_000_123);
            assert_eq!(head.app_hash().as_slice(), header.app_hash);
            assert_eq!(head.data_hash().as_slice(), header.data_hash);
            assert_eq!(
                head.last_results_hash().as_slice(),
                header.last_results_hash
            );
            assert_eq!(
                head.next_validators_hash().as_slice(),
                header.next_validators_hash
            );
            accepted += 1;
        } else {
            assert_eq!(result, Err(ChainProofError::Mismatch), "{name}");
            refused += 1;
        }
        match lines.next() {
            Some("},") => continue,
            Some("}") => break,
            other => panic!("invalid vector case terminator: {other:?}"),
        }
    }
    assert_eq!(lines.next(), Some("]"));
    assert_eq!(lines.next(), Some("}"));
    assert_eq!(lines.next(), None);
    assert_eq!(names.len(), 13);
    assert_eq!((accepted, refused), (9, 4));
    Ok(())
}
