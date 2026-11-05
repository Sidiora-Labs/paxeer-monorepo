use super::{
    proto::{ProtoReader, ProtoValue},
    ChainProofError,
};
use sha2::{Digest, Sha256};

pub const NO_HASH: u32 = 0;
pub const SHA256: u32 = 1;
pub const VAR_PROTO: u32 = 1;
const MAX_PROOF_BYTES: usize = 1 << 20;
const MAX_PATH_DEPTH: usize = 128;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ExistenceProof {
    pub key: Vec<u8>,
    pub value: Vec<u8>,
    pub leaf: LeafOp,
    pub path: Vec<InnerOp>,
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct LeafOp {
    pub hash: u32,
    pub prehash_key: u32,
    pub prehash_value: u32,
    pub length: u32,
    pub prefix: Vec<u8>,
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct InnerOp {
    pub hash: u32,
    pub prefix: Vec<u8>,
    pub suffix: Vec<u8>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct InnerSpec {
    pub child_order: Vec<usize>,
    pub child_size: usize,
    pub min_prefix_length: usize,
    pub max_prefix_length: usize,
    pub hash: u32,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProofSpec {
    pub leaf_spec: LeafOp,
    pub inner_spec: InnerSpec,
    pub min_depth: usize,
    pub max_depth: usize,
}

impl ProofSpec {
    #[must_use]
    pub fn iavl() -> Self {
        Self {
            leaf_spec: LeafOp {
                hash: SHA256,
                prehash_key: NO_HASH,
                prehash_value: SHA256,
                length: VAR_PROTO,
                prefix: vec![0],
            },
            inner_spec: InnerSpec {
                child_order: vec![0, 1],
                child_size: 33,
                min_prefix_length: 4,
                max_prefix_length: 12,
                hash: SHA256,
            },
            min_depth: 0,
            max_depth: 0,
        }
    }
}

fn bytes(value: ProtoValue<'_>) -> Result<&[u8], ChainProofError> {
    match value {
        ProtoValue::Bytes(value) => Ok(value),
        _ => Err(ChainProofError::NonCanonical),
    }
}

fn enum_value(value: ProtoValue<'_>) -> Result<u32, ChainProofError> {
    match value {
        ProtoValue::Varint(value) if value <= i32::MAX as u64 => Ok(value as u32),
        _ => Err(ChainProofError::NonCanonical),
    }
}

fn once(seen: &mut u8, field: u32) -> Result<(), ChainProofError> {
    let bit = 1 << field;
    if *seen & bit != 0 {
        return Err(ChainProofError::NonCanonical);
    }
    *seen |= bit;
    Ok(())
}

impl LeafOp {
    pub fn decode(input: &[u8]) -> Result<Self, ChainProofError> {
        let mut reader = ProtoReader::new(input);
        let mut op = Self::default();
        let mut seen = 0;
        while let Some(field) = reader.next_field()? {
            if !(1..=5).contains(&field.number) {
                return Err(ChainProofError::UnknownField);
            }
            once(&mut seen, field.number)?;
            match field.number {
                1 => op.hash = enum_value(field.value)?,
                2 => op.prehash_key = enum_value(field.value)?,
                3 => op.prehash_value = enum_value(field.value)?,
                4 => op.length = enum_value(field.value)?,
                5 => op.prefix = bytes(field.value)?.to_vec(),
                _ => unreachable!(),
            }
        }
        reader.finish()?;
        Ok(op)
    }
}

impl InnerOp {
    pub fn decode(input: &[u8]) -> Result<Self, ChainProofError> {
        let mut reader = ProtoReader::new(input);
        let mut op = Self::default();
        let mut seen = 0;
        while let Some(field) = reader.next_field()? {
            if !(1..=3).contains(&field.number) {
                return Err(ChainProofError::UnknownField);
            }
            once(&mut seen, field.number)?;
            match field.number {
                1 => op.hash = enum_value(field.value)?,
                2 => op.prefix = bytes(field.value)?.to_vec(),
                3 => op.suffix = bytes(field.value)?.to_vec(),
                _ => unreachable!(),
            }
        }
        reader.finish()?;
        Ok(op)
    }
}

impl ExistenceProof {
    pub fn decode(input: &[u8]) -> Result<Self, ChainProofError> {
        decode_existence(input)
    }
}

pub fn decode_existence(input: &[u8]) -> Result<ExistenceProof, ChainProofError> {
    if input.len() > MAX_PROOF_BYTES {
        return Err(ChainProofError::NonCanonical);
    }
    let mut reader = ProtoReader::new(input);
    let (mut key, mut value, mut leaf) = (None, None, None);
    let mut path = Vec::new();
    let mut seen = 0;
    while let Some(field) = reader.next_field()? {
        if !(1..=4).contains(&field.number) {
            return Err(ChainProofError::UnknownField);
        }
        if field.number != 4 {
            once(&mut seen, field.number)?;
        }
        let data = bytes(field.value)?;
        match field.number {
            1 => key = Some(data.to_vec()),
            2 => value = Some(data.to_vec()),
            3 => leaf = Some(LeafOp::decode(data)?),
            4 => {
                if path.len() == MAX_PATH_DEPTH {
                    return Err(ChainProofError::NonCanonical);
                }
                path.push(InnerOp::decode(data)?);
            }
            _ => unreachable!(),
        }
    }
    reader.finish()?;
    Ok(ExistenceProof {
        key: key.ok_or(ChainProofError::NonCanonical)?,
        value: value.ok_or(ChainProofError::NonCanonical)?,
        leaf: leaf.ok_or(ChainProofError::NonCanonical)?,
        path,
    })
}

fn metadata_value(input: &mut &[u8]) -> Result<u64, ChainProofError> {
    let mut value = 0;
    for shift in (0..70).step_by(7) {
        let (&byte, rest) = input.split_first().ok_or(ChainProofError::Mismatch)?;
        *input = rest;
        if shift == 63 && byte > 1 {
            return Err(ChainProofError::Mismatch);
        }
        value |= u64::from(byte & 0x7f) << shift;
        if byte & 0x80 == 0 {
            if (shift != 0 && byte == 0) || value & 1 != 0 {
                return Err(ChainProofError::Mismatch);
            }
            return Ok(value >> 1);
        }
    }
    Err(ChainProofError::Mismatch)
}

fn metadata(input: &mut &[u8]) -> Result<u64, ChainProofError> {
    let height = metadata_value(input)?;
    metadata_value(input)?;
    metadata_value(input)?;
    Ok(height)
}

fn hash_length(hash: &mut Sha256, mut length: usize) {
    while length >= 128 {
        hash.update([(length as u8 & 0x7f) | 0x80]);
        length >>= 7;
    }
    hash.update([length as u8]);
}

pub fn verify_existence(
    proof: &ExistenceProof,
    spec: &ProofSpec,
    root: &[u8],
    key: &[u8],
    value: &[u8],
) -> Result<(), ChainProofError> {
    let leaf = &proof.leaf;
    let expected = &spec.leaf_spec;
    let inner = &spec.inner_spec;
    if proof.key != key
        || proof.value != value
        || key.is_empty()
        || value.is_empty()
        || root.len() != 32
        || proof.path.len() > MAX_PATH_DEPTH
        || proof.path.len() < spec.min_depth
        || (spec.max_depth != 0 && proof.path.len() > spec.max_depth)
        || leaf.hash != expected.hash
        || leaf.prehash_key != expected.prehash_key
        || leaf.prehash_value != expected.prehash_value
        || leaf.length != expected.length
        || !leaf.prefix.starts_with(&expected.prefix)
        || leaf.hash != SHA256
        || leaf.prehash_key != NO_HASH
        || leaf.prehash_value != SHA256
        || leaf.length != VAR_PROTO
        || expected.prefix.first() != Some(&0)
        || inner.hash != SHA256
        || inner.child_size != 33
        || inner.child_order != [0, 1]
        || inner.min_prefix_length > inner.max_prefix_length
    {
        return Err(ChainProofError::Mismatch);
    }
    let mut remaining = leaf.prefix.as_slice();
    if metadata(&mut remaining)? != 0 || !remaining.is_empty() {
        return Err(ChainProofError::Mismatch);
    }
    let mut hash = Sha256::new();
    hash.update(&leaf.prefix);
    hash_length(&mut hash, key.len());
    hash.update(key);
    hash_length(&mut hash, 32);
    hash.update(Sha256::digest(value));
    let mut child: [u8; 32] = hash.finalize().into();
    for (depth, op) in proof.path.iter().enumerate() {
        if op.hash != inner.hash || op.prefix.starts_with(&expected.prefix) {
            return Err(ChainProofError::Mismatch);
        }
        let left_children = match op.suffix.len() {
            33 => 0,
            0 => 1,
            _ => return Err(ChainProofError::Mismatch),
        };
        let prefix_length = op
            .prefix
            .len()
            .checked_sub(left_children * inner.child_size)
            .ok_or(ChainProofError::Mismatch)?;
        if !(inner.min_prefix_length..=inner.max_prefix_length).contains(&prefix_length) {
            return Err(ChainProofError::Mismatch);
        }
        let mut remaining = op.prefix.as_slice();
        if metadata(&mut remaining)? < (depth + 1) as u64 {
            return Err(ChainProofError::Mismatch);
        }
        match (left_children, remaining) {
            (0, [32]) if op.suffix[0] == 32 => {}
            (1, [32, ..]) if remaining.len() == 34 && remaining[33] == 32 => {}
            _ => return Err(ChainProofError::Mismatch),
        }
        let mut hash = Sha256::new();
        hash.update(&op.prefix);
        hash.update(child);
        hash.update(&op.suffix);
        child = hash.finalize().into();
    }
    if child.as_slice() != root {
        return Err(ChainProofError::Mismatch);
    }
    Ok(())
}
