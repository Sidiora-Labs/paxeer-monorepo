use layerx_programs::chain_proof::ChainProofError;

#[path = "../src/chain_proof/decode.rs"]
mod decode;

use decode::{decode_commitment_proof, CommitmentProof, ProofOp, MAX_FIELD_BYTES, MAX_PROOF_OPS};

fn commitment_proof() -> CommitmentProof {
    CommitmentProof {
        signed_header: vec![0x0a, 0x07, 0x12, 0x03, b'l', b'x', b'p', 0x18, 0x01],
        commit: vec![0x08, 0x01, 0x22, 0x02, 0x08, 0x01],
        validator_set: vec![0x0a, 0x02, 0x18, 0x0a, 0x18, 0x0a],
        store_name: "lxphost".to_owned(),
        key: vec![0, 0xff, b'k'],
        value: Some(vec![0xff, 0, b'v']),
        proof_ops: vec![
            ProofOp {
                op_type: "ics23:iavl".to_owned(),
                key: vec![0, 0xff, b'k'],
                data: vec![0x0a, 0x02, 0x0a, 0x00],
            },
            ProofOp {
                op_type: "ics23:simple".to_owned(),
                key: b"lxphost".to_vec(),
                data: vec![0x0a, 0x02, 0x12, 0x00],
            },
        ],
    }
}

fn canonical_bytes(proof: &CommitmentProof) -> (Vec<u8>, Vec<usize>, usize, usize) {
    let mut bytes = b"LXCP\x01".to_vec();
    let mut prefixes = Vec::new();
    let mut append = |bytes: &mut Vec<u8>, field: &[u8]| {
        prefixes.push(bytes.len());
        bytes.extend_from_slice(&(field.len() as u32).to_be_bytes());
        bytes.extend_from_slice(field);
    };
    for field in [
        proof.signed_header.as_slice(),
        proof.commit.as_slice(),
        proof.validator_set.as_slice(),
        proof.store_name.as_bytes(),
        proof.key.as_slice(),
    ] {
        append(&mut bytes, field);
    }
    let presence = bytes.len();
    bytes.push(u8::from(proof.value.is_some()));
    if let Some(value) = &proof.value {
        append(&mut bytes, value);
    }
    let count = bytes.len();
    bytes.extend_from_slice(&(proof.proof_ops.len() as u32).to_be_bytes());
    for op in &proof.proof_ops {
        for field in [op.op_type.as_bytes(), op.key.as_slice(), op.data.as_slice()] {
            append(&mut bytes, field);
        }
    }
    (bytes, prefixes, presence, count)
}

#[test]
fn chain_commitment_proof_round_trips() -> Result<(), ChainProofError> {
    for value in [Some(vec![0xff, 0, b'v']), Some(Vec::new()), None] {
        for count in 0..=MAX_PROOF_OPS {
            let mut proof = commitment_proof();
            proof.value = value.clone();
            proof.proof_ops.truncate(count);
            let (bytes, _, _, _) = canonical_bytes(&proof);
            let decoded = decode_commitment_proof(&bytes)?;
            assert_eq!(decoded, proof);
            assert_eq!(decoded.encode()?, bytes);
        }
    }

    let mut proof = commitment_proof();
    proof.value = Some(vec![0xa5; MAX_FIELD_BYTES]);
    let bytes = proof.encode()?;
    assert_eq!(decode_commitment_proof(&bytes)?, proof);
    assert_eq!(decode_commitment_proof(&bytes)?.encode()?, bytes);
    Ok(())
}

#[test]
fn chain_commitment_proof_refuses_oversize_and_trailing_bytes() -> Result<(), ChainProofError> {
    let proof = commitment_proof();
    let (bytes, prefixes, presence, count) = canonical_bytes(&proof);
    for prefix in &prefixes {
        for length in [524_289_u32, u32::MAX] {
            let mut invalid = bytes.clone();
            invalid[*prefix..*prefix + 4].copy_from_slice(&length.to_be_bytes());
            assert_eq!(
                decode_commitment_proof(&invalid),
                Err(ChainProofError::NonCanonical)
            );
        }
        let mut boundary = bytes[..*prefix + 4].to_vec();
        boundary[*prefix..*prefix + 4].copy_from_slice(&524_288_u32.to_be_bytes());
        assert_eq!(
            decode_commitment_proof(&boundary),
            Err(ChainProofError::Truncated)
        );
    }
    let mut third_op = bytes.clone();
    third_op[count..count + 4].copy_from_slice(&3_u32.to_be_bytes());
    third_op.extend_from_slice(&bytes[count + 4..]);
    assert_eq!(
        decode_commitment_proof(&third_op),
        Err(ChainProofError::NonCanonical)
    );
    let mut huge_count = bytes.clone();
    huge_count[count..count + 4].copy_from_slice(&u32::MAX.to_be_bytes());
    assert_eq!(
        decode_commitment_proof(&huge_count),
        Err(ChainProofError::NonCanonical)
    );

    let mut trailing = bytes.clone();
    trailing.push(0);
    assert_eq!(
        decode_commitment_proof(&trailing),
        Err(ChainProofError::NonCanonical)
    );
    for end in 0..bytes.len() {
        assert_eq!(
            decode_commitment_proof(&bytes[..end]),
            Err(ChainProofError::Truncated),
            "cut at {end}"
        );
    }
    for version in [0, 2, u8::MAX] {
        let mut invalid = bytes.clone();
        invalid[4] = version;
        assert_eq!(
            decode_commitment_proof(&invalid),
            Err(ChainProofError::NonCanonical)
        );
    }
    let mut bad_magic = bytes.clone();
    bad_magic[0] = b'Y';
    assert_eq!(
        decode_commitment_proof(&bad_magic),
        Err(ChainProofError::NonCanonical)
    );
    for flag in [2, u8::MAX] {
        let mut invalid = bytes.clone();
        invalid[presence] = flag;
        assert_eq!(
            decode_commitment_proof(&invalid),
            Err(ChainProofError::NonCanonical)
        );
    }
    for prefix in [prefixes[3], prefixes[6]] {
        let mut invalid = bytes.clone();
        invalid[prefix + 4] = 0xff;
        assert_eq!(
            decode_commitment_proof(&invalid),
            Err(ChainProofError::NonCanonical)
        );
    }

    let mut oversize = proof.clone();
    oversize.key = vec![0; MAX_FIELD_BYTES + 1];
    assert_eq!(oversize.encode(), Err(ChainProofError::NonCanonical));
    let mut too_many = proof;
    too_many.proof_ops.push(too_many.proof_ops[0].clone());
    assert_eq!(too_many.encode(), Err(ChainProofError::NonCanonical));
    Ok(())
}
