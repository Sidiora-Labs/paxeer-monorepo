use std::collections::BTreeMap;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    decode::ProofOp,
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    proto::{ProtoReader, ProtoValue},
    state::{verify_absent, verify_state},
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    ChainProofError, ChainProofVerifier, Commit, VerifiedChainHead,
};
use sha2::{Digest, Sha256};

const STORE_VECTOR: &[u8] = include_bytes!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../sdk/store/rootmulti/testdata/chain_proof_v1.hex"
));
const STORE_VECTOR_SHA256: &str =
    "c6c868d420a886b648f147626cf75e8db612451b1c4529f3abb33acc461ac52d";

fn hex(text: &str) -> Vec<u8> {
    assert_eq!(text.len() % 2, 0, "vector hex must contain whole bytes");
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let text = std::str::from_utf8(pair).expect("vector hex must be ASCII");
            u8::from_str_radix(text, 16).expect("vector must contain valid hex")
        })
        .collect()
}

fn proof_ops(bytes: &[u8]) -> Result<Vec<ProofOp>, ChainProofError> {
    let mut reader = ProtoReader::new(bytes);
    let mut ops = Vec::new();
    while let Some(field) = reader.next_field()? {
        let encoded = match (field.number, field.value) {
            (1, ProtoValue::Bytes(encoded)) => encoded,
            _ => return Err(ChainProofError::NonCanonical),
        };
        let mut op = ProtoReader::new(encoded);
        let mut fields = Vec::new();
        for number in 1..=3 {
            let field = op.next_field()?.ok_or(ChainProofError::Truncated)?;
            match (field.number, field.value) {
                (actual, ProtoValue::Bytes(value)) if actual == number => fields.push(value),
                _ => return Err(ChainProofError::NonCanonical),
            }
        }
        op.finish()?;
        ops.push(ProofOp {
            op_type: std::str::from_utf8(fields[0])
                .map_err(|_| ChainProofError::NonCanonical)?
                .to_owned(),
            key: fields[1].to_vec(),
            data: fields[2].to_vec(),
        });
    }
    reader.finish()?;
    Ok(ops)
}

fn verified_head(height: i64, app_hash: [u8; 32]) -> Result<VerifiedChainHead, ChainProofError> {
    let key = SigningKey::from_bytes(&[17; 32]);
    let validator = TrustedValidator::new(key.verifying_key().to_bytes(), 10);
    let address = validator.address;
    let validators = ValidatorSet::new(vec![validator])?;
    let header = ChainHeader {
        version: HeaderVersion { block: 11, app: 1 },
        chain_id: "paxeer-store-vector-test".to_owned(),
        height,
        time: HeaderTime {
            seconds: 1_700_000_000,
            nanos: 0,
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
        consensus_hash: vec![5; 32],
        app_hash: app_hash.to_vec(),
        last_results_hash: vec![6; 32],
        evidence_hash: vec![7; 32],
        proposer_address: address.to_vec(),
    };
    let mut commit = Commit {
        height,
        round: 0,
        block_id: BlockId {
            hash: header.hash()?.to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![8; 32],
            },
        },
        signatures: Vec::new(),
    };
    let timestamp = Timestamp {
        seconds: header.time.seconds,
        nanos: 0,
    };
    let bytes = vote_sign_bytes(
        &header.chain_id,
        height,
        commit.round,
        Some(&commit.block_id),
        timestamp,
    )?;
    commit.signatures.push(CommitSig {
        flag: BlockIdFlag::Commit,
        validator_address: address.to_vec(),
        timestamp,
        signature: key.sign(&bytes).to_bytes().to_vec(),
    });
    ChainProofVerifier {
        chain_id: header.chain_id.clone(),
        validators,
    }
    .verify_head(&header, &commit)
}

#[test]
fn chain_ics23_matches_store_vector() -> Result<(), ChainProofError> {
    assert_eq!(
        &Sha256::digest(STORE_VECTOR)[..],
        hex(STORE_VECTOR_SHA256),
        "store proof vector changed; review the regenerated vector before updating its SHA-256 pin"
    );
    let mut fields = BTreeMap::new();
    for line in std::str::from_utf8(STORE_VECTOR)
        .expect("store vector must be UTF-8")
        .lines()
    {
        let (name, value) = line
            .split_once('=')
            .expect("vector must contain name=value");
        assert!(
            fields.insert(name, value).is_none(),
            "duplicate field {name}"
        );
    }
    assert_eq!(fields.len(), 10);
    assert_eq!(fields["version"], "1");
    let height: i64 = fields["height"].parse().expect("vector height must be i64");
    assert!(height > 0);
    let module =
        String::from_utf8(hex(fields["module_key"])).expect("vector module key must be UTF-8");
    let app_hash: [u8; 32] = hex(fields["app_hash"])
        .try_into()
        .expect("vector app hash must be 32 bytes");
    let next_height = height.checked_add(1).expect("next height must fit i64");
    let next = verified_head(next_height, app_hash)?;
    assert_eq!(next.app_hash(), app_hash);
    assert_eq!(next.height(), next_height);
    let mut wrong_hash = app_hash;
    wrong_hash[0] ^= 1;
    let wrong_head = verified_head(next_height, wrong_hash)?;

    for (name, present) in [("present", true), ("absent", false)] {
        let key = hex(fields[format!("{name}_key").as_str()]);
        let value = hex(fields[format!("{name}_value").as_str()]);
        let ops = proof_ops(&hex(fields[format!("{name}_proof_ops").as_str()]))?;
        assert_eq!(ops.len(), 2, "{name}: expected IAVL and multistore ops");
        assert_eq!(ops[0].op_type, "ics23:iavl");
        assert_eq!(ops[0].key, key);
        assert_eq!(ops[1].op_type, "ics23:simple");
        assert_eq!(ops[1].key, module.as_bytes());
        if present {
            assert!(!value.is_empty());
            assert_eq!(verify_state(&next, &module, &key, &value, &ops), Ok(height));
            assert_eq!(
                verify_absent(&next, &module, &key, &ops),
                Err(ChainProofError::Mismatch)
            );
            assert_eq!(
                verify_state(&wrong_head, &module, &key, &value, &ops),
                Err(ChainProofError::Mismatch)
            );
        } else {
            assert!(value.is_empty());
            assert_eq!(verify_absent(&next, &module, &key, &ops), Ok(height));
            assert_eq!(
                verify_state(&next, &module, &key, &value, &ops),
                Err(ChainProofError::Mismatch)
            );
            assert_eq!(
                verify_absent(&wrong_head, &module, &key, &ops),
                Err(ChainProofError::Mismatch)
            );
        }
    }
    Ok(())
}
