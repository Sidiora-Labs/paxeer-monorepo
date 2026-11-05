use sha2::{Digest, Sha256};

use super::ChainProofError;

#[must_use]
pub fn hash_from_byte_slices(items: &[&[u8]]) -> [u8; 32] {
    match items.len() {
        0 => Sha256::digest([]).into(),
        1 => leaf_hash(items[0]),
        length => {
            let split = 1_usize << (length - 1).ilog2();
            inner_hash(
                &hash_from_byte_slices(&items[..split]),
                &hash_from_byte_slices(&items[split..]),
            )
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SimpleProof {
    pub total: i64,
    pub index: i64,
    pub leaf_hash: [u8; 32],
    pub aunts: Vec<[u8; 32]>,
}

impl SimpleProof {
    pub fn verify(&self, root: &[u8; 32], leaf: &[u8]) -> Result<(), ChainProofError> {
        if self.total <= 0 || self.index < 0 || self.index >= self.total {
            return Err(ChainProofError::Mismatch);
        }
        if self.leaf_hash != leaf_hash(leaf) {
            return Err(ChainProofError::Mismatch);
        }
        let computed = root_from_aunts(self.total, self.index, &self.leaf_hash, &self.aunts)?;
        if computed != *root {
            return Err(ChainProofError::Mismatch);
        }
        Ok(())
    }
}

fn leaf_hash(leaf: &[u8]) -> [u8; 32] {
    let mut hash = Sha256::new();
    hash.update([0x00]);
    hash.update(leaf);
    hash.finalize().into()
}

fn inner_hash(left: &[u8; 32], right: &[u8; 32]) -> [u8; 32] {
    let mut hash = Sha256::new();
    hash.update([0x01]);
    hash.update(left);
    hash.update(right);
    hash.finalize().into()
}

fn root_from_aunts(
    total: i64,
    index: i64,
    leaf: &[u8; 32],
    aunts: &[[u8; 32]],
) -> Result<[u8; 32], ChainProofError> {
    if total == 1 {
        return if aunts.is_empty() {
            Ok(*leaf)
        } else {
            Err(ChainProofError::Mismatch)
        };
    }
    let (aunt, remaining) = aunts.split_last().ok_or(ChainProofError::Mismatch)?;
    let split = 1_i64 << (total - 1).ilog2();
    if index < split {
        Ok(inner_hash(
            &root_from_aunts(split, index, leaf, remaining)?,
            aunt,
        ))
    } else {
        Ok(inner_hash(
            aunt,
            &root_from_aunts(total - split, index - split, leaf, remaining)?,
        ))
    }
}
