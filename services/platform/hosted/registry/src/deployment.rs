use layerx_programs::VerifiedDeploymentEvidence;

pub(crate) fn deployment_header_hash(evidence: &VerifiedDeploymentEvidence) -> [u8; 32] {
    layerx_programs::chain_proof::header::ChainHeader::decode(
        &evidence.proof().state.commitment.signed_header,
    )
    .and_then(|header| header.hash())
    .expect("verified deployment carries a verified chain header")
}
