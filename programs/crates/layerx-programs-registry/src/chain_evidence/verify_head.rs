use sha2::{Digest, Sha256};

use crate::{
    chain_evidence::{freshness::chain_freshness, protocol_head::ChainProtocolHeadProof},
    chain_proof::{
        header::ChainHeader, validators::ValidatorSet, ChainProofVerifier, Commit,
        VerifiedChainHead as VerifiedHeader,
    },
    protocol_evidence::{ProgramHeadKind, ReceiptEvidenceDigest},
    ProtocolEvidenceError, ReadFreshness,
};

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct VerifiedProtocolHead {
    header: VerifiedHeader,
    receipt_digest: [u8; 32],
    freshness: ReadFreshness,
}

impl VerifiedProtocolHead {
    pub(crate) const fn state_root(&self) -> [u8; 32] {
        self.header.app_hash()
    }

    pub(crate) const fn header_hash(&self) -> [u8; 32] {
        self.header.header_hash()
    }

    pub(crate) const fn header_height(&self) -> i64 {
        self.header.height()
    }

    pub(crate) const fn receipt_digest(&self) -> [u8; 32] {
        self.receipt_digest
    }

    pub(crate) const fn receipt_evidence_digest(&self) -> ReceiptEvidenceDigest {
        ReceiptEvidenceDigest::OrdinaryUnsignedReceipt(self.receipt_digest)
    }

    pub(crate) const fn head_kind(&self) -> ProgramHeadKind {
        ProgramHeadKind::Ordinary
    }

    pub(crate) const fn freshness(&self) -> ReadFreshness {
        self.freshness
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct VerifiedChainHead {
    protocol: VerifiedProtocolHead,
    evidence_digest: ReceiptEvidenceDigest,
}

impl VerifiedChainHead {
    pub(crate) const fn state_root(&self) -> [u8; 32] {
        self.protocol.state_root()
    }

    pub(crate) const fn header_hash(&self) -> [u8; 32] {
        self.protocol.header_hash()
    }

    pub(crate) const fn header_height(&self) -> i64 {
        self.protocol.header_height()
    }

    pub(crate) const fn head_kind(&self) -> ProgramHeadKind {
        self.evidence_digest.head_kind()
    }

    pub(crate) const fn receipt_evidence_digest(&self) -> ReceiptEvidenceDigest {
        self.evidence_digest
    }

    pub(crate) const fn receipt_digest(&self) -> [u8; 32] {
        self.evidence_digest.bytes()
    }

    pub(crate) const fn freshness(&self) -> ReadFreshness {
        self.protocol.freshness()
    }
}

pub(crate) struct ChainTrustStep<'a> {
    pub header: &'a ChainHeader,
    pub commit: &'a Commit,
    pub validators: &'a ValidatorSet,
}

pub(crate) fn protocol_head_at(
    verifier: &ChainProofVerifier,
    proof: &ChainProtocolHeadProof<'_>,
    now_ms: u64,
    max_age_ms: u64,
) -> Result<VerifiedProtocolHead, ProtocolEvidenceError> {
    if proof.verifier.chain_id != verifier.chain_id
        || proof.verifier.validators != verifier.validators
    {
        return Err(ProtocolEvidenceError::InvalidTrustAnchor);
    }
    let head = verifier
        .verify_head(proof.header, proof.commit)
        .map_err(|_| ProtocolEvidenceError::StateRoot)?;
    proof.verify(&head)?;
    let freshness = chain_freshness(&head, now_ms, max_age_ms)?;
    Ok(protocol_head(head, freshness))
}

pub(crate) fn chain_head_at(
    verifier: &ChainProofVerifier,
    kind: ProgramHeadKind,
    header: &ChainHeader,
    commit: &Commit,
    now_ms: u64,
    max_age_ms: u64,
) -> Result<VerifiedChainHead, ProtocolEvidenceError> {
    let head = verifier
        .verify_head(header, commit)
        .map_err(|_| ProtocolEvidenceError::StateRoot)?;
    let freshness = chain_freshness(&head, now_ms, max_age_ms)?;
    let protocol = protocol_head(head, freshness);
    let evidence_digest = match kind {
        ProgramHeadKind::Ordinary => {
            ReceiptEvidenceDigest::OrdinaryUnsignedReceipt(protocol.receipt_digest())
        }
        ProgramHeadKind::Maintenance => {
            ReceiptEvidenceDigest::MaintenanceReceiptSha256(protocol.receipt_digest())
        }
    };
    Ok(VerifiedChainHead {
        protocol,
        evidence_digest,
    })
}

pub(crate) fn historical_protocol_head_at(
    verifier: &ChainProofVerifier,
    trusted_header: &ChainHeader,
    trusted_commit: &Commit,
    history: &[ChainTrustStep<'_>],
    proof: &ChainProtocolHeadProof<'_>,
) -> Result<VerifiedProtocolHead, ProtocolEvidenceError> {
    let mut verifier = verifier.clone();
    let mut trusted = verifier
        .verify_head(trusted_header, trusted_commit)
        .map_err(|_| ProtocolEvidenceError::InvalidTrustAnchor)?;
    for step in history {
        verifier = verifier
            .advance(&trusted, step.header, step.commit, step.validators.clone())
            .map_err(|_| ProtocolEvidenceError::InvalidTrustAnchor)?;
        trusted = verifier
            .verify_head(step.header, step.commit)
            .map_err(|_| ProtocolEvidenceError::InvalidTrustAnchor)?;
    }
    let head = if proof
        .header
        .hash()
        .map_err(|_| ProtocolEvidenceError::StateRoot)?
        == trusted.header_hash()
    {
        verifier
            .verify_head(proof.header, proof.commit)
            .map_err(|_| ProtocolEvidenceError::InvalidTrustAnchor)?
    } else {
        verifier = verifier
            .advance(
                &trusted,
                proof.header,
                proof.commit,
                proof.verifier.validators.clone(),
            )
            .map_err(|_| ProtocolEvidenceError::InvalidTrustAnchor)?;
        verifier
            .verify_head(proof.header, proof.commit)
            .map_err(|_| ProtocolEvidenceError::InvalidTrustAnchor)?
    };
    if verifier.chain_id != proof.verifier.chain_id
        || verifier.validators != proof.verifier.validators
    {
        return Err(ProtocolEvidenceError::InvalidTrustAnchor);
    }
    proof.verify(&head)?;
    let time_ms = u64::try_from(head.time_ms()).map_err(|_| ProtocolEvidenceError::Stale)?;
    let freshness = chain_freshness(&head, time_ms, 0)?;
    Ok(protocol_head(head, freshness))
}

fn protocol_head(header: VerifiedHeader, freshness: ReadFreshness) -> VerifiedProtocolHead {
    let mut digest = Sha256::new();
    digest.update(header.header_hash());
    digest.update(header.app_hash());
    VerifiedProtocolHead {
        header,
        receipt_digest: digest.finalize().into(),
        freshness,
    }
}
