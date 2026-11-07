use super::{
    proto::{ProtoReader, ProtoValue},
    ChainProofError,
};
use sha2::{Digest, Sha256};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct TendermintSpec {
    leaf_ops: [u64; 4],
    leaf_prefix: &'static [u8],
    inner_hash: u64,
    inner_prefix: u8,
    child_size: usize,
}

pub const TENDERMINT_SPEC: TendermintSpec = TendermintSpec {
    leaf_ops: [1, 0, 1, 1],
    leaf_prefix: &[0],
    inner_hash: 1,
    inner_prefix: 1,
    child_size: 32,
};

pub fn verify_store_root(
    store_name: &str,
    store_root: [u8; 32],
    proof: &[u8],
    app_hash: [u8; 32],
) -> Result<(), ChainProofError> {
    if store_name.is_empty() {
        return Err(ChainProofError::Mismatch);
    }
    let mut commitment = ProtoReader::new(proof);
    let existence = required_bytes(&mut commitment, 1)?;
    commitment.finish()?;

    let mut existence = ProtoReader::new(existence);
    let key = required_bytes(&mut existence, 1)?;
    let value = required_bytes(&mut existence, 2)?;
    if key != store_name.as_bytes() || value != store_root {
        return Err(ChainProofError::Mismatch);
    }
    verify_leaf_op(required_bytes(&mut existence, 3)?)?;

    let mut leaf = Sha256::new();
    leaf.update(TENDERMINT_SPEC.leaf_prefix);
    hash_length(&mut leaf, key.len());
    leaf.update(key);
    hash_length(&mut leaf, TENDERMINT_SPEC.child_size);
    leaf.update(Sha256::digest(value));
    let mut root: [u8; 32] = leaf.finalize().into();

    while let Some(field) = existence.next_field()? {
        match (field.number, field.value) {
            (4, ProtoValue::Bytes(inner)) => root = apply_inner_op(inner, root)?,
            (1..=3, _) => return Err(ChainProofError::NonCanonical),
            _ => return Err(ChainProofError::UnknownField),
        }
    }
    existence.finish()?;
    if root != app_hash {
        return Err(ChainProofError::Mismatch);
    }
    Ok(())
}

fn required_bytes<'a>(
    reader: &mut ProtoReader<'a>,
    number: u32,
) -> Result<&'a [u8], ChainProofError> {
    match reader.next_field()? {
        Some(field) if field.number == number => match field.value {
            ProtoValue::Bytes(bytes) => Ok(bytes),
            _ => Err(ChainProofError::Mismatch),
        },
        _ => Err(ChainProofError::Mismatch),
    }
}

fn verify_leaf_op(bytes: &[u8]) -> Result<(), ChainProofError> {
    let mut reader = ProtoReader::new(bytes);
    let mut ops = [0; 4];
    let mut prefix = None;
    let mut seen = 0_u32;
    while let Some(field) = reader.next_field()? {
        if !(1..=5).contains(&field.number) {
            return Err(ChainProofError::UnknownField);
        }
        let bit = 1 << field.number;
        if seen & bit != 0 {
            return Err(ChainProofError::NonCanonical);
        }
        seen |= bit;
        match (field.number, field.value) {
            (1..=4, ProtoValue::Varint(value)) => ops[(field.number - 1) as usize] = value,
            (5, ProtoValue::Bytes(value)) => prefix = Some(value),
            _ => return Err(ChainProofError::Mismatch),
        }
    }
    reader.finish()?;
    if ops != TENDERMINT_SPEC.leaf_ops || prefix != Some(TENDERMINT_SPEC.leaf_prefix) {
        return Err(ChainProofError::Mismatch);
    }
    Ok(())
}

fn apply_inner_op(bytes: &[u8], child: [u8; 32]) -> Result<[u8; 32], ChainProofError> {
    let mut reader = ProtoReader::new(bytes);
    let hash = reader.next_field()?.ok_or(ChainProofError::Mismatch)?;
    if hash.number != 1 || hash.value != ProtoValue::Varint(TENDERMINT_SPEC.inner_hash) {
        return Err(ChainProofError::Mismatch);
    }
    let prefix = required_bytes(&mut reader, 2)?;
    let suffix = match reader.next_field()? {
        Some(field) if field.number == 3 => match field.value {
            ProtoValue::Bytes(bytes) => bytes,
            _ => return Err(ChainProofError::Mismatch),
        },
        None => &[],
        _ => return Err(ChainProofError::UnknownField),
    };
    reader.finish()?;
    if prefix.first() != Some(&TENDERMINT_SPEC.inner_prefix)
        || !((prefix.len() == 1 && suffix.len() == TENDERMINT_SPEC.child_size)
            || (prefix.len() == 1 + TENDERMINT_SPEC.child_size && suffix.is_empty()))
    {
        return Err(ChainProofError::Mismatch);
    }
    let mut inner = Sha256::new();
    inner.update(prefix);
    inner.update(child);
    inner.update(suffix);
    Ok(inner.finalize().into())
}

fn hash_length(hash: &mut Sha256, mut length: usize) {
    while length >= 0x80 {
        hash.update([(length as u8 & 0x7f) | 0x80]);
        length >>= 7;
    }
    hash.update([length as u8]);
}
