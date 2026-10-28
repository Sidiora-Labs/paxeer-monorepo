use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    proto,
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    ChainProofError, ChainProofVerifier, Commit, VerifiedChainHead,
};
use sha2::{Digest, Sha256};

#[path = "../src/chain_proof/decode.rs"]
mod decode;
#[path = "../src/chain_proof/ics23.rs"]
mod ics23;
#[path = "../src/chain_proof/ics23_absent.rs"]
mod ics23_absent;
#[path = "../src/chain_proof/multistore.rs"]
mod multistore;
#[path = "../src/chain_proof/state.rs"]
mod state;

use decode::ProofOp;
use ics23::{ExistenceProof, InnerOp, LeafOp};
use state::{verify_absent, verify_state};

fn varint(mut value: usize, output: &mut Vec<u8>) {
    while value >= 128 {
        output.push((value as u8 & 127) | 128);
        value >>= 7;
    }
    output.push(value as u8);
}

fn bytes_field(number: u8, value: &[u8], output: &mut Vec<u8>) {
    output.push(number << 3 | 2);
    varint(value.len(), output);
    output.extend_from_slice(value);
}

fn leaf_hash(prefix: &[u8], key: &[u8], value: &[u8]) -> [u8; 32] {
    let mut bytes = prefix.to_vec();
    varint(key.len(), &mut bytes);
    bytes.extend_from_slice(key);
    varint(32, &mut bytes);
    bytes.extend_from_slice(&Sha256::digest(value));
    Sha256::digest(bytes).into()
}

fn existence_bytes(proof: &ExistenceProof) -> Vec<u8> {
    let mut bytes = Vec::new();
    bytes_field(1, &proof.key, &mut bytes);
    bytes_field(2, &proof.value, &mut bytes);
    let mut leaf = vec![8, 1, 24, 1, 32, 1];
    bytes_field(5, &proof.leaf.prefix, &mut leaf);
    bytes_field(3, &leaf, &mut bytes);
    for inner in &proof.path {
        let mut op = vec![8, 1];
        bytes_field(2, &inner.prefix, &mut op);
        if !inner.suffix.is_empty() {
            bytes_field(3, &inner.suffix, &mut op);
        }
        bytes_field(4, &op, &mut bytes);
    }
    bytes
}

fn commitment(variant: u8, proof: &[u8]) -> Vec<u8> {
    let mut bytes = Vec::new();
    bytes_field(variant, proof, &mut bytes);
    bytes
}

struct Store {
    app_hash: [u8; 32],
    left: ExistenceProof,
    right: ExistenceProof,
    multistore: ProofOp,
}

fn store() -> Store {
    let leaf = LeafOp {
        hash: 1,
        prehash_key: 0,
        prehash_value: 1,
        length: 1,
        prefix: vec![0, 2, 2],
    };
    let mut left = ExistenceProof {
        key: b"b".to_vec(),
        value: b"balance:10".to_vec(),
        leaf: leaf.clone(),
        path: Vec::new(),
    };
    let mut right = ExistenceProof {
        key: b"d".to_vec(),
        value: b"balance:20".to_vec(),
        leaf,
        path: Vec::new(),
    };
    let left_hash = leaf_hash(&left.leaf.prefix, &left.key, &left.value);
    let right_hash = leaf_hash(&right.leaf.prefix, &right.key, &right.value);
    let mut branch = vec![2, 4, 2, 32];
    branch.extend_from_slice(&left_hash);
    branch.push(32);
    branch.extend_from_slice(&right_hash);
    let root: [u8; 32] = Sha256::digest(&branch).into();
    let mut suffix = vec![32];
    suffix.extend_from_slice(&right_hash);
    left.path.push(InnerOp {
        hash: 1,
        prefix: vec![2, 4, 2, 32],
        suffix,
    });
    right.path.push(InnerOp {
        hash: 1,
        prefix: branch[..37].to_vec(),
        suffix: Vec::new(),
    });

    let other_root = leaf_hash(&[0, 2, 2], b"supply", b"30");
    let other_hash = leaf_hash(&[0], b"auth", &other_root);
    let bank_hash = leaf_hash(&[0], b"bank", &root);
    let mut app = vec![1];
    app.extend_from_slice(&other_hash);
    app.extend_from_slice(&bank_hash);
    let mut prefix = vec![1];
    prefix.extend_from_slice(&other_hash);
    let proof = ExistenceProof {
        key: b"bank".to_vec(),
        value: root.to_vec(),
        leaf: LeafOp {
            prefix: vec![0],
            ..left.leaf.clone()
        },
        path: vec![InnerOp {
            hash: 1,
            prefix,
            suffix: Vec::new(),
        }],
    };
    Store {
        app_hash: Sha256::digest(app).into(),
        left,
        right,
        multistore: ProofOp {
            op_type: "ics23:simple".to_owned(),
            key: b"bank".to_vec(),
            data: commitment(1, &existence_bytes(&proof)),
        },
    }
}

impl Store {
    fn membership(&self, proof: &ExistenceProof) -> Vec<ProofOp> {
        vec![
            ProofOp {
                op_type: "ics23:iavl".to_owned(),
                key: proof.key.clone(),
                data: commitment(1, &existence_bytes(proof)),
            },
            self.multistore.clone(),
        ]
    }

    fn absence(&self, key: &[u8], left: bool, right: bool) -> Vec<ProofOp> {
        let mut proof = Vec::new();
        bytes_field(1, key, &mut proof);
        if left {
            bytes_field(2, &existence_bytes(&self.left), &mut proof);
        }
        if right {
            bytes_field(3, &existence_bytes(&self.right), &mut proof);
        }
        vec![
            ProofOp {
                op_type: "ics23:iavl".to_owned(),
                key: key.to_vec(),
                data: commitment(2, &proof),
            },
            self.multistore.clone(),
        ]
    }
}

fn verified_head(height: i64, app_hash: [u8; 32]) -> Result<VerifiedChainHead, ChainProofError> {
    let key = SigningKey::from_bytes(&[17; 32]);
    let validator = TrustedValidator::new(key.verifying_key().to_bytes(), 10);
    let address = validator.address;
    let validators = ValidatorSet::new(vec![validator])?;
    let header = ChainHeader {
        version: HeaderVersion { block: 11, app: 1 },
        chain_id: "paxeer-state-test".to_owned(),
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
fn chain_state_membership_verifies_at_next_app_hash() -> Result<(), ChainProofError> {
    let store = store();
    let next = verified_head(38, store.app_hash)?;
    for proof in [&store.left, &store.right] {
        let ops = store.membership(proof);
        assert_eq!(
            verify_state(&next, "bank", &proof.key, &proof.value, &ops),
            Ok(37)
        );
    }
    let ops = store.membership(&store.left);
    let check =
        |head: &VerifiedChainHead, name: &str, key: &[u8], value: &[u8], ops: &[ProofOp]| {
            assert_eq!(
                verify_state(head, name, key, value, ops),
                Err(ChainProofError::Mismatch)
            );
        };
    check(
        &verified_head(1, store.app_hash)?,
        "bank",
        b"b",
        b"balance:10",
        &ops,
    );
    check(
        &verified_head(2, store.app_hash)?,
        "bank",
        b"b",
        b"wrong",
        &ops,
    );
    assert_eq!(
        verify_state(
            &verified_head(2, store.app_hash)?,
            "bank",
            b"b",
            b"balance:10",
            &ops
        ),
        Ok(1)
    );
    let mut wrong_hash = store.app_hash;
    wrong_hash[0] ^= 1;
    check(
        &verified_head(38, wrong_hash)?,
        "bank",
        b"b",
        b"balance:10",
        &ops,
    );
    check(&next, "auth", b"b", b"balance:10", &ops);
    check(&next, "", b"b", b"balance:10", &ops);
    check(&next, "bank", b"c", b"balance:10", &ops);
    for count in 0..2 {
        check(&next, "bank", b"b", b"balance:10", &ops[..count]);
    }
    let mut extra = ops.clone();
    extra.push(ops[1].clone());
    check(&next, "bank", b"b", b"balance:10", &extra);
    let mut reversed = ops.clone();
    reversed.reverse();
    check(&next, "bank", b"b", b"balance:10", &reversed);
    for index in 0..2 {
        let mut changed = ops.clone();
        changed[index].op_type.push_str(":unknown");
        check(&next, "bank", b"b", b"balance:10", &changed);
        let mut changed = ops.clone();
        changed[index].key.push(0);
        check(&next, "bank", b"b", b"balance:10", &changed);
        let mut changed = ops.clone();
        changed[index].data.pop();
        assert!(verify_state(&next, "bank", b"b", b"balance:10", &changed).is_err());
        let mut changed = ops.clone();
        changed[index].data.extend_from_slice(&[10, 0]);
        assert!(verify_state(&next, "bank", b"b", b"balance:10", &changed).is_err());
    }
    let mut forged = store.left.clone();
    forged.value = b"balance:999".to_vec();
    check(
        &next,
        "bank",
        &forged.key,
        &forged.value,
        &store.membership(&forged),
    );
    Ok(())
}

#[test]
fn chain_state_absence_verifies_and_refuses_present_key() -> Result<(), ChainProofError> {
    let store = store();
    let next = verified_head(38, store.app_hash)?;
    for (key, left, right) in [
        (b"a".as_slice(), false, true),
        (b"c", true, true),
        (b"e", true, false),
    ] {
        let ops = if key == b"a" {
            let mut proof = Vec::new();
            bytes_field(1, key, &mut proof);
            bytes_field(3, &existence_bytes(&store.left), &mut proof);
            let mut ops = store.absence(key, left, right);
            ops[0].data = commitment(2, &proof);
            ops
        } else if key == b"e" {
            let mut proof = Vec::new();
            bytes_field(1, key, &mut proof);
            bytes_field(2, &existence_bytes(&store.right), &mut proof);
            let mut ops = store.absence(key, left, right);
            ops[0].data = commitment(2, &proof);
            ops
        } else {
            store.absence(key, left, right)
        };
        assert_eq!(verify_absent(&next, "bank", key, &ops), Ok(37));
    }
    for present in [&store.left, &store.right] {
        let ops = store.membership(present);
        assert_eq!(
            verify_state(&next, "bank", &present.key, &present.value, &ops),
            Ok(37)
        );
        assert_eq!(
            verify_absent(&next, "bank", &present.key, &ops),
            Err(ChainProofError::Mismatch)
        );
        assert_eq!(
            verify_absent(
                &next,
                "bank",
                &present.key,
                &store.absence(&present.key, true, true)
            ),
            Err(ChainProofError::Mismatch)
        );
    }
    let ops = store.absence(b"c", true, true);
    assert_eq!(
        verify_absent(&verified_head(1, store.app_hash)?, "bank", b"c", &ops),
        Err(ChainProofError::Mismatch)
    );
    assert_eq!(
        verify_absent(&verified_head(2, store.app_hash)?, "bank", b"c", &ops),
        Ok(1)
    );
    assert_eq!(
        verify_absent(&next, "auth", b"c", &ops),
        Err(ChainProofError::Mismatch)
    );
    assert_eq!(
        verify_absent(&next, "bank", b"b", &ops),
        Err(ChainProofError::Mismatch)
    );
    assert_eq!(
        verify_absent(&next, "bank", b"c", &store.absence(b"c", false, false)),
        Err(ChainProofError::Mismatch)
    );
    let mut wrong_hash = store.app_hash;
    wrong_hash[0] ^= 1;
    assert_eq!(
        verify_absent(&verified_head(38, wrong_hash)?, "bank", b"c", &ops),
        Err(ChainProofError::Mismatch)
    );
    for count in 0..2 {
        assert_eq!(
            verify_absent(&next, "bank", b"c", &ops[..count]),
            Err(ChainProofError::Mismatch)
        );
    }
    let mut extra = ops.clone();
    extra.push(ops[1].clone());
    assert_eq!(
        verify_absent(&next, "bank", b"c", &extra),
        Err(ChainProofError::Mismatch)
    );
    let mut forged = store.absence(b"c", true, true);
    let mut neighbour = store.left.clone();
    neighbour.value[0] ^= 1;
    let mut proof = Vec::new();
    bytes_field(1, b"c", &mut proof);
    bytes_field(2, &existence_bytes(&neighbour), &mut proof);
    bytes_field(3, &existence_bytes(&store.right), &mut proof);
    forged[0].data = commitment(2, &proof);
    assert_eq!(
        verify_absent(&next, "bank", b"c", &forged),
        Err(ChainProofError::Mismatch)
    );
    Ok(())
}
