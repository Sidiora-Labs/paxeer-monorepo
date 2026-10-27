use super::{
    ics23::{verify_existence, ExistenceProof, InnerOp, InnerSpec, ProofSpec},
    proto::{ProtoReader, ProtoValue},
    ChainProofError,
};

const MAX_PROOF_BYTES: usize = 1 << 20;

#[derive(Clone, Debug)]
pub struct NonExistenceProof {
    pub key: Vec<u8>,
    pub left: Option<ExistenceProof>,
    pub right: Option<ExistenceProof>,
}

impl NonExistenceProof {
    pub fn decode(bytes: &[u8]) -> Result<Self, ChainProofError> {
        if bytes.len() > MAX_PROOF_BYTES {
            return Err(ChainProofError::NonCanonical);
        }
        let mut reader = ProtoReader::new(bytes);
        let mut proof = Self {
            key: Vec::new(),
            left: None,
            right: None,
        };
        let mut seen = 0_u8;
        while let Some(field) = reader.next_field()? {
            if !(1..=3).contains(&field.number) {
                return Err(ChainProofError::UnknownField);
            }
            let bit = 1 << field.number;
            if seen & bit != 0 {
                return Err(ChainProofError::NonCanonical);
            }
            seen |= bit;
            let ProtoValue::Bytes(value) = field.value else {
                return Err(ChainProofError::NonCanonical);
            };
            match field.number {
                1 => proof.key = value.to_vec(),
                2 => proof.left = Some(ExistenceProof::decode(value)?),
                3 => proof.right = Some(ExistenceProof::decode(value)?),
                _ => return Err(ChainProofError::UnknownField),
            }
        }
        reader.finish()?;
        Ok(proof)
    }
}

pub fn verify_non_existence(
    proof: &NonExistenceProof,
    spec: &ProofSpec,
    root: &[u8],
    key: &[u8],
) -> Result<(), ChainProofError> {
    if proof.key != key {
        return Err(ChainProofError::Mismatch);
    }
    if let Some(left) = &proof.left {
        verify_existence(left, spec, root, &left.key, &left.value)?;
        if left.key.as_slice() >= key {
            return Err(ChainProofError::Mismatch);
        }
    }
    if let Some(right) = &proof.right {
        verify_existence(right, spec, root, &right.key, &right.value)?;
        if key >= right.key.as_slice() {
            return Err(ChainProofError::Mismatch);
        }
    }
    let ordered = match (&proof.left, &proof.right) {
        (Some(left), Some(right)) => adjacent(&left.path, &right.path, &spec.inner_spec),
        (None, Some(right)) => is_extreme(&right.path, 0, &spec.inner_spec),
        (Some(left), None) => is_extreme(&left.path, 1, &spec.inner_spec),
        (None, None) => false,
    };
    if !ordered {
        return Err(ChainProofError::Mismatch);
    }
    Ok(())
}

fn child_position(op: &InnerOp, spec: &InnerSpec) -> Option<u8> {
    if op.hash != spec.hash {
        return None;
    }
    let (position, prefix_length) = if op.suffix.len() == spec.child_size {
        (0, op.prefix.len())
    } else if op.suffix.is_empty() {
        (1, op.prefix.len().checked_sub(spec.child_size)?)
    } else {
        return None;
    };
    if (spec.min_prefix_length..=spec.max_prefix_length).contains(&prefix_length) {
        Some(position)
    } else {
        None
    }
}

fn is_extreme(path: &[InnerOp], position: u8, spec: &InnerSpec) -> bool {
    path.iter()
        .all(|op| child_position(op, spec) == Some(position))
}

fn adjacent(left: &[InnerOp], right: &[InnerOp], spec: &InnerSpec) -> bool {
    let (mut left_end, mut right_end) = (left.len(), right.len());
    while left_end > 0 && right_end > 0 {
        let l = &left[left_end - 1];
        let r = &right[right_end - 1];
        if l.hash != r.hash || l.prefix != r.prefix || l.suffix != r.suffix {
            break;
        }
        left_end -= 1;
        right_end -= 1;
    }
    let (Some((left_split, left_below)), Some((right_split, right_below))) = (
        left[..left_end].split_last(),
        right[..right_end].split_last(),
    ) else {
        return false;
    };
    child_position(left_split, spec) == Some(0)
        && child_position(right_split, spec) == Some(1)
        && is_extreme(left_below, 1, spec)
        && is_extreme(right_below, 0, spec)
}
