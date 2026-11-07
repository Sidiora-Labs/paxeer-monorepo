use super::program_state::{ChainProgramStateProof, VerifiedProgramRecord};
use super::{programs_state_key, HOST_STORE_NAME};
use crate::{
    chain_proof::VerifiedChainHead,
    chain_proof::{
        decode::{decode_commitment_proof, CommitmentProof},
        header::ChainHeader,
        state::{verify_absent, verify_state},
    },
    hash::sha256,
    ProgramId, ProgramInterface, ProtocolEvidenceError, ReadFreshness, UpgradePolicy,
};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ChainDeploymentProof {
    pub program_state: ChainProgramStateProof,
    pub code: Vec<u8>,
    /// Canonical `CommitmentProof` for the interface record, including proven
    /// absence. Bare descriptors and `None` lack state authority and are refused.
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
    /// Carries the host's interface witness unchanged through deployment and
    /// journal codecs. Encoding grants no trust; `verify` checks both records
    /// against the same authenticated head.
    pub fn from_state_proofs(
        program_state: ChainProgramStateProof,
        code: Vec<u8>,
        interface_state: &CommitmentProof,
    ) -> Result<Self, ProtocolEvidenceError> {
        Ok(Self {
            program_state,
            code,
            interface: Some(
                interface_state
                    .encode()
                    .map_err(|_| ProtocolEvidenceError::Encoding)?,
            ),
        })
    }

    pub fn verify(
        &self,
        next: &VerifiedChainHead,
    ) -> Result<VerifiedDeploymentEvidence, ProtocolEvidenceError> {
        let record = self.program_state.verify(next)?;
        if sha256(&self.code) != record.code_hash() {
            return Err(ProtocolEvidenceError::DeploymentMismatch);
        }
        let witness = decode_commitment_proof(
            self.interface
                .as_deref()
                .ok_or(ProtocolEvidenceError::StateProof)?,
        )
        .map_err(|_| ProtocolEvidenceError::StateProof)?;
        let key = programs_state_key(&crate::interface_state_key(record.program()));
        let header = ChainHeader::decode(&witness.signed_header)
            .map_err(|_| ProtocolEvidenceError::StateProof)?;
        if witness.store_name != HOST_STORE_NAME
            || witness.key != key
            || header
                .hash()
                .map_err(|_| ProtocolEvidenceError::StateProof)?
                != next.header_hash()
        {
            return Err(ProtocolEvidenceError::StateProof);
        }
        let interface = match witness.value.as_deref() {
            Some(value) => {
                verify_state(next, HOST_STORE_NAME, &key, value, &witness.proof_ops)
                    .map_err(|_| ProtocolEvidenceError::StateProof)?;
                // The host record frames program, version, digest and descriptor.
                // Only bytes authenticated above may reach the descriptor decoder.
                let bytes = value
                    .get(72..)
                    .ok_or(ProtocolEvidenceError::DeploymentMismatch)?;
                let interface = ProgramInterface::decode(bytes)
                    .map_err(|_| ProtocolEvidenceError::DeploymentMismatch)?;
                if crate::interface_state_value(record.program(), record.version(), &interface)
                    .map_err(|_| ProtocolEvidenceError::DeploymentMismatch)?
                    != value
                    || interface.code_hash() != record.code_hash()
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
                Some(interface)
            }
            None => {
                verify_absent(next, HOST_STORE_NAME, &key, &witness.proof_ops)
                    .map_err(|_| ProtocolEvidenceError::StateProof)?;
                None
            }
        };
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
