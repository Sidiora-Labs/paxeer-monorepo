use crate::{
    chain_evidence::programs_state_key,
    chain_proof::{
        decode::ProofOp, header::ChainHeader, state::verify_state, ChainProofVerifier, Commit,
        VerifiedChainHead,
    },
    ProtocolEvidenceError,
};

pub struct ChainProtocolHeadProof<'a> {
    pub ops: &'a [ProofOp],
    pub kernel_key: &'a [u8],
    pub value: &'a [u8],
    pub store_name: &'a str,
    pub verifier: &'a ChainProofVerifier,
    pub header: &'a ChainHeader,
    pub commit: &'a Commit,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct VerifiedProtocolHead {
    state_root: [u8; 32],
    header_hash: [u8; 32],
    header_height: i64,
    height: i64,
}

impl VerifiedProtocolHead {
    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.state_root
    }

    #[must_use]
    pub const fn header_hash(&self) -> [u8; 32] {
        self.header_hash
    }

    #[must_use]
    pub const fn header_height(&self) -> i64 {
        self.header_height
    }

    #[must_use]
    pub const fn height(&self) -> i64 {
        self.height
    }
}

impl ChainProtocolHeadProof<'_> {
    pub fn verify(
        &self,
        next: &VerifiedChainHead,
    ) -> Result<VerifiedProtocolHead, ProtocolEvidenceError> {
        if next.chain_id() != self.verifier.chain_id
            || self.header.chain_id != self.verifier.chain_id
        {
            return Err(ProtocolEvidenceError::ProtocolDomain);
        }
        let verified = self
            .verifier
            .verify_head(self.header, self.commit)
            .map_err(|_| ProtocolEvidenceError::StateRoot)?;
        if &verified != next {
            return Err(ProtocolEvidenceError::StateRoot);
        }
        let height = verify_state(
            next,
            self.store_name,
            &programs_state_key(self.kernel_key),
            self.value,
            self.ops,
        )
        .map_err(|_| ProtocolEvidenceError::StateProof)?;
        let header_hash = self
            .header
            .hash()
            .map_err(|_| ProtocolEvidenceError::StateRoot)?;
        Ok(VerifiedProtocolHead {
            state_root: next.app_hash(),
            header_hash,
            header_height: next.height(),
            height,
        })
    }
}
