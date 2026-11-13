use crate::{
    chain_proof::{
        validators::{TrustedValidator, ValidatorSet},
        ChainProofVerifier,
    },
    ProtocolDeploymentVerifier, ProtocolEvidenceError,
};

impl ProtocolDeploymentVerifier {
    ///
    /// # Errors
    ///
    /// Returns an error when the supplied chain evidence cannot be decoded or verified.
    pub fn from_chain_trust(
        chain_id: impl Into<String>,
        validators: Vec<TrustedValidator>,
        max_age_ms: u64,
    ) -> Result<Self, ProtocolEvidenceError> {
        if validators.is_empty() || max_age_ms == 0 {
            return Err(ProtocolEvidenceError::InvalidTrustAnchor);
        }
        let validators =
            ValidatorSet::new(validators).map_err(|_| ProtocolEvidenceError::InvalidTrustAnchor)?;
        if validators.total_power() == 0 {
            return Err(ProtocolEvidenceError::InvalidTrustAnchor);
        }
        let chain_verifier = ChainProofVerifier {
            chain_id: chain_id.into(),
            validators,
        };
        Ok(Self {
            staleness_limit_ms: max_age_ms,
            chain_verifier: Some(chain_verifier),
        })
    }
}
