use crate::{
    chain_evidence::{
        deployment::{ChainDeploymentProof, VerifiedDeploymentEvidence},
        freshness::chain_freshness,
    },
    chain_proof::{header::ChainHeader, validators::ValidatorSet, Commit, VerifiedChainHead},
    ProtocolDeploymentVerifier, ProtocolEvidenceError,
};

pub(crate) fn deployment_at(
    verifier: &ProtocolDeploymentVerifier,
    proof: &ChainDeploymentProof,
    next: &VerifiedChainHead,
    now_ms: u64,
) -> Result<VerifiedDeploymentEvidence, ProtocolEvidenceError> {
    let trust = verifier
        .chain_trust()
        .ok_or(ProtocolEvidenceError::InvalidTrustAnchor)?;
    if next.chain_id() != trust.chain_id {
        return Err(ProtocolEvidenceError::ProtocolDomain);
    }
    chain_freshness(next, now_ms, verifier.staleness_limit_ms())?;
    proof.verify(next)
}

pub(crate) fn historical_deployment_at(
    verifier: &ProtocolDeploymentVerifier,
    proof: &ChainDeploymentProof,
    next: &VerifiedChainHead,
    current: &VerifiedChainHead,
    trust_chain: &[(ChainHeader, Commit, ValidatorSet)],
    now_ms: u64,
) -> Result<VerifiedDeploymentEvidence, ProtocolEvidenceError> {
    let mut trust = verifier
        .chain_trust()
        .ok_or(ProtocolEvidenceError::InvalidTrustAnchor)?
        .clone();
    if next.chain_id() != trust.chain_id || current.chain_id() != trust.chain_id {
        return Err(ProtocolEvidenceError::ProtocolDomain);
    }
    chain_freshness(current, now_ms, verifier.staleness_limit_ms())?;
    let ((header, commit, validators), remaining) = trust_chain
        .split_first()
        .ok_or(ProtocolEvidenceError::HistoricalTrustAnchor)?;
    if validators != &trust.validators {
        return Err(ProtocolEvidenceError::InvalidTrustAnchor);
    }
    let mut head = trust
        .verify_head(header, commit)
        .map_err(|_| ProtocolEvidenceError::HistoricalTrustAnchor)?;
    if &head != next {
        return Err(ProtocolEvidenceError::HistoricalTrustAnchor);
    }
    for (header, commit, validators) in remaining {
        trust = trust
            .advance(&head, header, commit, validators.clone())
            .map_err(|_| ProtocolEvidenceError::HistoricalTrustAnchor)?;
        head = trust
            .verify_head(header, commit)
            .map_err(|_| ProtocolEvidenceError::HistoricalTrustAnchor)?;
    }
    if &head != current {
        return Err(ProtocolEvidenceError::HistoricalTrustAnchor);
    }
    proof.verify(next)
}
