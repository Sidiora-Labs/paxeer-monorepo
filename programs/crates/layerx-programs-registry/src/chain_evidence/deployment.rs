use super::program_state::{ChainProgramStateProof, VerifiedProgramRecord};
use crate::{
    chain_proof::VerifiedChainHead, hash::sha256, ProgramId, ProgramInterface,
    ProtocolEvidenceError, ReadFreshness, UpgradePolicy,
};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ChainDeploymentProof {
    pub program_state: ChainProgramStateProof,
    pub code: Vec<u8>,
    pub interface: Option<Vec<u8>>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedDeploymentEvidence {
    record: VerifiedProgramRecord,
    code: Vec<u8>,
    interface: Option<ProgramInterface>,
    freshness: ReadFreshness,
}

impl ChainDeploymentProof {
    pub fn verify(
        &self,
        next: &VerifiedChainHead,
    ) -> Result<VerifiedDeploymentEvidence, ProtocolEvidenceError> {
        let record = self.program_state.verify(next)?;
        if sha256(&self.code) != record.code_hash() {
            return Err(ProtocolEvidenceError::DeploymentMismatch);
        }
        let interface = self
            .interface
            .as_ref()
            .map(|bytes| {
                let interface = ProgramInterface::decode(bytes)
                    .map_err(|_| ProtocolEvidenceError::DeploymentMismatch)?;
                if interface.code_hash() != record.code_hash()
                    || interface.abi_version() != record.abi_version()
                {
                    return Err(ProtocolEvidenceError::DeploymentMismatch);
                }
                let rebound = ProgramInterface::bind(
                    &self.code,
                    record.abi_version(),
                    interface.entries().to_vec(),
                )
                .map_err(|_| ProtocolEvidenceError::DeploymentMismatch)?;
                if rebound.canonical_encoding() != bytes {
                    return Err(ProtocolEvidenceError::DeploymentMismatch);
                }
                Ok(interface)
            })
            .transpose()?;
        let freshness = ReadFreshness {
            observed_sequence: u64::try_from(next.height())
                .map_err(|_| ProtocolEvidenceError::Encoding)?,
            observed_at: u64::try_from(next.time_ms()).map_err(|_| ProtocolEvidenceError::Stale)?,
        };
        Ok(VerifiedDeploymentEvidence {
            record,
            code: self.code.clone(),
            interface,
            freshness,
        })
    }
}

impl VerifiedDeploymentEvidence {
    #[must_use]
    pub const fn program(&self) -> ProgramId {
        self.record.program()
    }

    #[must_use]
    pub const fn version(&self) -> u32 {
        self.record.version()
    }

    #[must_use]
    pub const fn code_hash(&self) -> [u8; 32] {
        self.record.code_hash()
    }

    #[must_use]
    pub const fn abi_version(&self) -> u16 {
        self.record.abi_version()
    }

    #[must_use]
    pub const fn upgrade_policy(&self) -> UpgradePolicy {
        self.record.upgrade_policy()
    }

    #[must_use]
    pub const fn state_height(&self) -> i64 {
        self.record.state_height()
    }

    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.record.state_root()
    }

    #[must_use]
    pub const fn freshness(&self) -> ReadFreshness {
        self.freshness
    }

    #[must_use]
    pub fn module(&self) -> &[u8] {
        &self.code
    }

    #[must_use]
    pub const fn interface(&self) -> Option<&ProgramInterface> {
        self.interface.as_ref()
    }

    #[must_use]
    pub const fn interface_present(&self) -> bool {
        self.interface.is_some()
    }
}
