//! RFC 6962 trees matching consensus/crypto/merkle/{tree,proof}.go.

use sha2::{Digest, Sha256};

use crate::ProofError;

/// Hash ordered items as `HashFromByteSlices` does, including SHA-256 of the
/// empty byte string for an empty tree.
#[must_use]
pub fn root(items: &[&[u8]]) -> [u8; 32] {
    match items.len() {
        0 => Sha256::digest([]).into(),
        1 => leaf_hash(items[0]),
        length => {
            let split = 1 << (usize::BITS - 1 - (length - 1).leading_zeros());
            inner_hash(root(&items[..split]), root(&items[split..]))
        }
    }
}

/// Verify an inclusion proof whose aunts are ordered from the leaf toward the
/// root, as returned by `ProofsFromByteSlices`.
///
/// # Errors
/// Returns [`ProofError::InvalidProof`] for an invalid position or an inexact
/// aunt count, and [`ProofError::RootMismatch`] for a different commitment.
pub fn verify_proof(
    root: [u8; 32],
    total: u64,
    index: u64,
    leaf: &[u8],
    aunts: &[[u8; 32]],
) -> Result<(), ProofError> {
    if total == 0 || index >= total {
        return Err(ProofError::InvalidProof);
    }
    let computed = compute_root(total, index, leaf_hash(leaf), aunts)?;
    if computed != root {
        return Err(ProofError::RootMismatch);
    }
    Ok(())
}

fn compute_root(
    total: u64,
    index: u64,
    leaf_hash: [u8; 32],
    aunts: &[[u8; 32]],
) -> Result<[u8; 32], ProofError> {
    if total == 1 {
        return if aunts.is_empty() {
            Ok(leaf_hash)
        } else {
            Err(ProofError::InvalidProof)
        };
    }
    let (aunt, remaining) = aunts.split_last().ok_or(ProofError::InvalidProof)?;
    // Largest power of two strictly below total; no next-power overflow even
    // for u64::MAX. Each recursive call consumes one aunt and reduces total.
    let split = 1 << (u64::BITS - 1 - (total - 1).leading_zeros());
    if index < split {
        Ok(inner_hash(
            compute_root(split, index, leaf_hash, remaining)?,
            *aunt,
        ))
    } else {
        Ok(inner_hash(
            *aunt,
            compute_root(total - split, index - split, leaf_hash, remaining)?,
        ))
    }
}

fn leaf_hash(leaf: &[u8]) -> [u8; 32] {
    let mut hash = Sha256::new();
    hash.update([0x00]);
    hash.update(leaf);
    hash.finalize().into()
}

fn inner_hash(left: [u8; 32], right: [u8; 32]) -> [u8; 32] {
    let mut hash = Sha256::new();
    hash.update([0x01]);
    hash.update(left);
    hash.update(right);
    hash.finalize().into()
}
