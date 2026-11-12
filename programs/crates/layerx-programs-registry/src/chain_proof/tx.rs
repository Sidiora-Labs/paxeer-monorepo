use sha2::{Digest, Sha256};

use super::{merkle::SimpleProof, ChainProofError, VerifiedChainHead};

///
/// # Errors
///
/// Returns an error when the supplied chain evidence cannot be decoded or verified.
pub fn verify_tx(
    head: &VerifiedChainHead,
    tx: &[u8],
    proof: &SimpleProof,
) -> Result<[u8; 32], ChainProofError> {
    let hash: [u8; 32] = Sha256::digest(tx).into();
    proof.verify(&head.data_hash(), &hash)?;
    Ok(hash)
}
