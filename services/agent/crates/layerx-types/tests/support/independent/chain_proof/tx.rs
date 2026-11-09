// Source-derived native proof rules; see ../PROVENANCE.md.
use super::super::Sha256;

use super::{merkle::SimpleProof, ChainProofError, VerifiedChainHead};

pub fn verify_tx(
    head: &VerifiedChainHead,
    tx: &[u8],
    proof: &SimpleProof,
) -> Result<[u8; 32], ChainProofError> {
    let hash: [u8; 32] = Sha256::digest(tx);
    proof.verify(&head.data_hash(), &hash)?;
    Ok(hash)
}
