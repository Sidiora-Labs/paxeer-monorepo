use layerx_programs::chain_proof::{proto, ChainProofError};
use sha2::{Digest, Sha256};

#[path = "../src/chain_proof/ics23.rs"]
mod ics23;

use ics23::{
    decode_existence, verify_existence, ExistenceProof, InnerOp, InnerSpec, LeafOp, ProofSpec,
    NO_HASH, SHA256,
};

// Go-generated IAVL vectors from programs/tests/vectors/ics23-iavl-existence.json.
const GO_VECTORS: &[(&str, &str, &str, &str)] = &[
    ("01000970726f6772616d001111111111111111111111111111111111111111111111111111111111111111", "70726f6772616d2d7265636f72642d7631", "d56e85913a20e1854de2a2114caf883e5040874faebe77a9b726de42309f4e98", "0a2b01000970726f6772616d001111111111111111111111111111111111111111111111111111111111111111121170726f6772616d2d7265636f72642d76311a0c0801180120012a0400028002222c0801120502048002201a21204ff1c8b149d4b2ae9a601ea54a7e5b4f173ac21e1d7a82e152a466779d0c8913222a08011226040880022006343d304383e318b9cbca5b8c5dcd6bf0e0694c3c6fc992541ef661bf5b81a120"),
    ("01000970726f676d65742f6163746976652f7631", "636174616c6f6775652d7265636f72642d7631", "d56e85913a20e1854de2a2114caf883e5040874faebe77a9b726de42309f4e98", "0a1401000970726f676d65742f6163746976652f76311213636174616c6f6775652d7265636f72642d76311a0c0801180120012a0400028002222a080112260204800220da3d877f9322e4ba6e4833359807a1e66785fce9937f5d882de0da917a15846d20222c0801120504088002201a212047bdcaadfb2c1349efbee8c0fab64deb192b3d7f1e88a11caf90fa6e892ed24f"),
    ("0100096e616d6573706163652f0001ff", "000102ff0007", "d56e85913a20e1854de2a2114caf883e5040874faebe77a9b726de42309f4e98", "0a100100096e616d6573706163652f0001ff1206000102ff00071a0c0801180120012a0400028002222c0801120502048002201a21207c40595e5e3e1fe85304510e244164e9cb6e0bd5d90ff800aa71517bf95135d3222c0801120504088002201a212047bdcaadfb2c1349efbee8c0fab64deb192b3d7f1e88a11caf90fa6e892ed24f"),
    ("01000aa5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5", "5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a", "d56e85913a20e1854de2a2114caf883e5040874faebe77a9b726de42309f4e98", "0a800101000aa5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a51280025a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a1a0c0801180120012a0400028002222a0801122602048002208c9ed8579c555c773f2def61663f0a641da413f88e91a38dfa524eda711d9e9a20222a08011226040880022006343d304383e318b9cbca5b8c5dcd6bf0e0694c3c6fc992541ef661bf5b81a120"),
];

fn hex(text: &str) -> Vec<u8> {
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).expect("hex byte"))
        .collect()
}

fn varint(mut value: usize) -> Vec<u8> {
    let mut result = Vec::new();
    while value >= 128 {
        result.push((value as u8 & 127) | 128);
        value >>= 7;
    }
    result.push(value as u8);
    result
}

fn reference_root(proof: &ExistenceProof) -> Vec<u8> {
    let mut preimage = proof.leaf.prefix.clone();
    for (op, data) in [
        (proof.leaf.prehash_key, &proof.key),
        (proof.leaf.prehash_value, &proof.value),
    ] {
        let data = if op == NO_HASH {
            data.clone()
        } else {
            Sha256::digest(data).to_vec()
        };
        if proof.leaf.length != 0 {
            preimage.extend(varint(data.len()));
        }
        preimage.extend(data);
    }
    let mut child = Sha256::digest(&preimage).to_vec();
    for op in &proof.path {
        let mut preimage = op.prefix.clone();
        preimage.extend(child);
        preimage.extend_from_slice(&op.suffix);
        child = Sha256::digest(preimage).to_vec();
    }
    child
}

fn put_bytes(out: &mut Vec<u8>, tag: u8, data: &[u8]) {
    out.push(tag);
    out.extend(varint(data.len()));
    out.extend_from_slice(data);
}

fn encode(proof: &ExistenceProof) -> Vec<u8> {
    let mut out = Vec::new();
    put_bytes(&mut out, 0x0a, &proof.key);
    put_bytes(&mut out, 0x12, &proof.value);
    let mut leaf = Vec::new();
    for (tag, value) in [
        (8, proof.leaf.hash),
        (16, proof.leaf.prehash_key),
        (24, proof.leaf.prehash_value),
        (32, proof.leaf.length),
    ] {
        if value != 0 {
            leaf.push(tag);
            leaf.extend(varint(value as usize));
        }
    }
    put_bytes(&mut leaf, 0x2a, &proof.leaf.prefix);
    put_bytes(&mut out, 0x1a, &leaf);
    for op in &proof.path {
        let mut inner = vec![8];
        inner.extend(varint(op.hash as usize));
        put_bytes(&mut inner, 0x12, &op.prefix);
        if !op.suffix.is_empty() {
            put_bytes(&mut inner, 0x1a, &op.suffix);
        }
        put_bytes(&mut out, 0x22, &inner);
    }
    out
}

#[test]
fn chain_ics23_existence_verifies_iavl_leaf() -> Result<(), ChainProofError> {
    let spec = ProofSpec::iavl();
    assert_eq!(
        spec.leaf_spec,
        LeafOp {
            hash: SHA256,
            prehash_key: NO_HASH,
            prehash_value: SHA256,
            length: 1,
            prefix: vec![0]
        }
    );
    assert_eq!(
        spec.inner_spec,
        InnerSpec {
            hash: SHA256,
            child_order: vec![0, 1],
            child_size: 33,
            min_prefix_length: 4,
            max_prefix_length: 12
        }
    );
    let (mut left, mut right, mut long_key) = (false, false, false);
    for &(key, value, root, encoded) in GO_VECTORS {
        let (key, value, root, encoded) = (hex(key), hex(value), hex(root), hex(encoded));
        let proof = ExistenceProof::decode(&encoded)?;
        assert_eq!(proof.key, key);
        assert_eq!(proof.value, value);
        assert_eq!(encode(&proof), encoded);
        assert_eq!(proof.path.len(), 2);
        assert_eq!(reference_root(&proof), root);
        verify_existence(&proof, &spec, &root, &key, &value)?;
        long_key |= key.len() >= 128;
        for op in &proof.path {
            left |= op.suffix.len() == 33;
            right |= op.suffix.is_empty();
        }
    }
    assert!(left && right && long_key);

    let mut proof = decode_existence(&hex(GO_VECTORS[0].3))?;
    proof.path.clear();
    proof.key = vec![0xa5; 128];
    proof.value = vec![0x5a; 256];
    verify_existence(
        &proof,
        &spec,
        &reference_root(&proof),
        &proof.key,
        &proof.value,
    )?;
    for prefix_len in [4, 12] {
        let mut prefix = vec![2, 4];
        prefix.extend(vec![0x80; prefix_len - 4]);
        prefix.extend([2, 32]);
        let mut sibling = vec![32];
        sibling.extend(Sha256::digest(b"sibling"));
        for right in [false, true] {
            let (mut prefix, mut suffix) = (prefix.clone(), sibling.clone());
            if right {
                prefix.extend_from_slice(&sibling[1..]);
                prefix.push(32);
                suffix.clear();
            }
            proof.path = vec![InnerOp {
                hash: SHA256,
                prefix,
                suffix,
            }];
            let decoded = decode_existence(&encode(&proof))?;
            assert_eq!(decoded, proof);
            verify_existence(
                &decoded,
                &spec,
                &reference_root(&proof),
                &proof.key,
                &proof.value,
            )?;
        }
    }
    Ok(())
}

#[test]
fn chain_ics23_existence_refuses_spec_violations() -> Result<(), ChainProofError> {
    let spec = ProofSpec::iavl();
    let proof = decode_existence(&hex(GO_VECTORS[0].3))?;
    let root = hex(GO_VECTORS[0].2);
    assert_eq!(
        verify_existence(&proof, &spec, &root, &proof.key, b"changed value"),
        Err(ChainProofError::Mismatch)
    );
    assert_eq!(
        verify_existence(&proof, &spec, &root, b"changed key", &proof.value),
        Err(ChainProofError::Mismatch)
    );
    let mut changed = proof.clone();
    changed.value[0] ^= 1;
    assert_eq!(
        verify_existence(&changed, &spec, &root, &changed.key, &changed.value),
        Err(ChainProofError::Mismatch)
    );
    let mut changed_root = root.clone();
    changed_root[0] ^= 1;
    assert_eq!(
        verify_existence(&proof, &spec, &changed_root, &proof.key, &proof.value),
        Err(ChainProofError::Mismatch)
    );
    assert_eq!(
        verify_existence(&proof, &spec, &root[..31], &proof.key, &proof.value),
        Err(ChainProofError::Mismatch)
    );

    for violation in 0..18 {
        let mut bad = proof.clone();
        match violation {
            0 => bad.leaf.prehash_value = NO_HASH,
            1 => bad.path[0]
                .prefix
                .truncate(spec.inner_spec.min_prefix_length - 1),
            2 => bad.leaf.hash = NO_HASH,
            3 => bad.leaf.prehash_key = SHA256,
            4 => bad.leaf.length = 0,
            5 => bad.leaf.prefix[0] = 2,
            6 => bad.path[0].hash = 2,
            7 => bad.path[0].prefix = vec![2; spec.inner_spec.max_prefix_length + 1],
            8 => bad.path[0].suffix.pop().map(|_| ()).unwrap(),
            9 => bad.path[0].suffix.extend_from_slice(&[32; 33]),
            10 => bad.path[0].prefix[0] = 0,
            11 => bad.path[0].suffix[0] = 31,
            12 => bad.leaf.prefix.push(0),
            13 => bad.leaf.prefix[1] = 1,
            14 => bad.path[0].prefix[0] = 1,
            15 => bad.key.clear(),
            16 => bad.value.clear(),
            17 => bad.path = vec![bad.path[0].clone(); 129],
            _ => unreachable!(),
        }
        // A matching hash must never rescue an operation that violates the spec.
        assert_eq!(
            verify_existence(&bad, &spec, &reference_root(&bad), &bad.key, &bad.value),
            Err(ChainProofError::Mismatch),
            "violation {violation}"
        );
    }
    for violation in 0..5 {
        let mut bad = spec.clone();
        match violation {
            0 => bad.min_depth = proof.path.len() + 1,
            1 => bad.max_depth = proof.path.len() - 1,
            2 => bad.inner_spec.child_size = 0,
            3 => bad.inner_spec.child_order.reverse(),
            4 => bad.inner_spec.min_prefix_length = bad.inner_spec.max_prefix_length + 1,
            _ => unreachable!(),
        }
        assert_eq!(
            verify_existence(&proof, &bad, &root, &proof.key, &proof.value),
            Err(ChainProofError::Mismatch)
        );
    }

    for (encoded, error) in [
        (vec![], ChainProofError::NonCanonical),
        (vec![0x0a, 0x80], ChainProofError::Truncated),
        (vec![0x0a, 0x80, 0], ChainProofError::NonCanonical),
        (vec![0x0a, 0], ChainProofError::NonCanonical),
        (vec![8, 1], ChainProofError::NonCanonical),
        (vec![0x0a, 0, 0x0a, 0], ChainProofError::NonCanonical),
        (vec![0x12, 0, 0x0a, 0], ChainProofError::NonCanonical),
        (vec![0x2a, 0], ChainProofError::UnknownField),
    ] {
        assert_eq!(decode_existence(&encoded), Err(error), "{encoded:x?}");
    }
    assert_eq!(
        LeafOp::decode(&[8, 1, 8, 1]),
        Err(ChainProofError::NonCanonical)
    );
    assert_eq!(
        LeafOp::decode(&[8, 0x81, 0]),
        Err(ChainProofError::NonCanonical)
    );
    assert_eq!(
        LeafOp::decode(&[0x0a, 0]),
        Err(ChainProofError::NonCanonical)
    );
    assert_eq!(
        LeafOp::decode(&[0x30, 1]),
        Err(ChainProofError::UnknownField)
    );
    assert_eq!(
        InnerOp::decode(&[8, 1, 8, 1]),
        Err(ChainProofError::NonCanonical)
    );
    assert_eq!(
        InnerOp::decode(&[0x20, 1]),
        Err(ChainProofError::UnknownField)
    );
    assert_eq!(
        InnerOp::decode(&[0x12, 2, 0]),
        Err(ChainProofError::Truncated)
    );
    let mut excessive = proof.clone();
    excessive.path = vec![proof.path[0].clone(); 129];
    assert_eq!(
        decode_existence(&encode(&excessive)),
        Err(ChainProofError::NonCanonical)
    );
    assert_eq!(
        decode_existence(&vec![0; (1 << 20) + 1]),
        Err(ChainProofError::NonCanonical)
    );
    Ok(())
}
