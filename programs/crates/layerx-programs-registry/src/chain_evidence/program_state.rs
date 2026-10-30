use crate::chain_evidence::programs_state_key;
use crate::chain_proof::{
    decode::ProofOp,
    ics23::ExistenceProof,
    proto::{ProtoReader, ProtoValue},
    state::verify_state,
    ChainProofError, VerifiedChainHead,
};
use crate::protocol_evidence::{
    decode_program_record, ProgramRecord, ProtocolEvidenceError, StateLeafWitness,
    PROGRAM_KEY_PREFIX,
};
use crate::{ProgramId, StateProof, UpgradePolicy};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ChainProgramStateProof {
    pub program: ProgramId,
    pub ops: Vec<ProofOp>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct VerifiedProgramRecord {
    record: ProgramRecord,
    state_height: i64,
    state_root: [u8; 32],
}

impl VerifiedProgramRecord {
    #[must_use]
    pub const fn program(&self) -> ProgramId {
        self.record.program
    }

    #[must_use]
    pub const fn version(&self) -> u32 {
        self.record.version
    }

    #[must_use]
    pub const fn code_hash(&self) -> [u8; 32] {
        self.record.code_hash
    }

    #[must_use]
    pub const fn abi_version(&self) -> u16 {
        self.record.abi_version
    }

    #[must_use]
    pub const fn upgrade_policy(&self) -> UpgradePolicy {
        self.record.policy
    }

    #[must_use]
    pub const fn state_height(&self) -> i64 {
        self.state_height
    }

    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.state_root
    }
}

impl ChainProgramStateProof {
    pub fn verify(
        &self,
        next: &VerifiedChainHead,
    ) -> Result<VerifiedProgramRecord, ProtocolEvidenceError> {
        let existence = self
            .existence()
            .map_err(|_| ProtocolEvidenceError::StateProof)?;
        let module_prefix = programs_state_key(&[]);
        let kernel_key = existence
            .key
            .strip_prefix(module_prefix.as_slice())
            .ok_or(ProtocolEvidenceError::StateProof)?;
        let record = decode_program_record(&StateLeafWitness {
            key: kernel_key.to_vec(),
            value: existence.value.clone(),
            proof: StateProof {
                leaf_index: 0,
                leaf_count: 1,
                siblings: Vec::new(),
            },
        })?;
        if record.program != self.program {
            return Err(ProtocolEvidenceError::ProgramRecord);
        }
        let mut kernel_key = PROGRAM_KEY_PREFIX.to_vec();
        kernel_key.extend_from_slice(&self.program.bytes());
        let key = programs_state_key(&kernel_key);
        let state_height = verify_state(next, "layerx", &key, &existence.value, &self.ops)
            .map_err(|_| ProtocolEvidenceError::StateProof)?;
        Ok(VerifiedProgramRecord {
            record,
            state_height,
            state_root: next.app_hash(),
        })
    }

    fn existence(&self) -> Result<ExistenceProof, ChainProofError> {
        let op = self.ops.first().ok_or(ChainProofError::Mismatch)?;
        let mut reader = ProtoReader::new(&op.data);
        let field = reader.next_field()?.ok_or(ChainProofError::Mismatch)?;
        let proof = match (field.number, field.value) {
            (1, ProtoValue::Bytes(proof)) => ExistenceProof::decode(proof)?,
            _ => return Err(ChainProofError::Mismatch),
        };
        reader.finish()?;
        Ok(proof)
    }
}
