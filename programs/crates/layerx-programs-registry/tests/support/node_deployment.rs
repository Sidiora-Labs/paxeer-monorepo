use layerx_programs::{
    DeploymentExecutionProof, DeploymentProof, ProtocolDeploymentVerifier, ProtocolEvidenceError,
    VerifiedDeploymentEvidence,
};

/// An exported admission authenticated by the production verifier. Unlike the
/// primitive constructors, this fixture never builds transactions, results,
/// state records, Merkle paths or signatures from expected deployment values.
#[derive(Debug)]
pub struct NodeDeploymentFixture {
    admission: Vec<u8>,
    execution: DeploymentExecutionProof,
    evidence: VerifiedDeploymentEvidence,
}

/// Consume the producer's canonical `DeploymentProof` transport unchanged.
/// Trust and observation time belong to the consumer, not to the export.
pub fn node_deployment_fixture(
    admission: &[u8],
    verifier: &ProtocolDeploymentVerifier,
    now_ms: u64,
) -> Result<NodeDeploymentFixture, ProtocolEvidenceError> {
    let proof = DeploymentProof::decode(admission)?;
    let execution = DeploymentExecutionProof::decode(&proof.state.transaction)?;
    let evidence = verifier.verify_deployment(&proof, now_ms)?;
    Ok(NodeDeploymentFixture {
        admission: admission.to_vec(),
        execution,
        evidence,
    })
}

impl NodeDeploymentFixture {
    pub fn admission(&self) -> &[u8] {
        &self.admission
    }

    pub fn proof(&self) -> &DeploymentProof {
        self.evidence.proof()
    }

    pub fn execution(&self) -> &DeploymentExecutionProof {
        &self.execution
    }

    pub fn evidence(&self) -> &VerifiedDeploymentEvidence {
        &self.evidence
    }
}
