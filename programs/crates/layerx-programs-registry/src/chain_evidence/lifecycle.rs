use crate::chain_evidence::programs_state_key;
use crate::chain_proof::{
    decode::ProofOp,
    state::{verify_absent, verify_state},
    VerifiedChainHead,
};
use crate::protocol_evidence::{
    decode_program_record, StateLeafWitness, PROGRAM_KEY_PREFIX, STATUS_KEY_PREFIX,
};
use crate::{ProgramId, ProgramLifecycle, ProtocolEvidenceError, StateProof};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ChainLifecycleProof {
    pub program: ProgramId,
    pub record: Option<Vec<u8>>,
    pub record_ops: Vec<ProofOp>,
    pub status: Option<Vec<u8>>,
    pub status_ops: Vec<ProofOp>,
}

impl ChainLifecycleProof {
    pub fn verify(
        &self,
        next: &VerifiedChainHead,
    ) -> Result<ProgramLifecycle, ProtocolEvidenceError> {
        let mut record_key = PROGRAM_KEY_PREFIX.to_vec();
        record_key.extend_from_slice(&self.program.bytes());
        let key = programs_state_key(&record_key);
        let Some(value) = &self.record else {
            verify_absent(next, "layerx", &key, &self.record_ops)
                .map_err(|_| ProtocolEvidenceError::StateProof)?;
            return Err(ProtocolEvidenceError::LifecycleProof);
        };
        verify_state(next, "layerx", &key, value, &self.record_ops)
            .map_err(|_| ProtocolEvidenceError::StateProof)?;
        let record = decode_program_record(&StateLeafWitness {
            key: record_key,
            value: value.clone(),
            proof: StateProof {
                leaf_index: 0,
                leaf_count: 1,
                siblings: Vec::new(),
            },
        })?;
        if record.program != self.program {
            return Err(ProtocolEvidenceError::LifecycleProof);
        }

        let mut status_key = STATUS_KEY_PREFIX.to_vec();
        status_key.extend_from_slice(&self.program.bytes());
        let key = programs_state_key(&status_key);
        match &self.status {
            None => {
                verify_absent(next, "layerx", &key, &self.status_ops)
                    .map_err(|_| ProtocolEvidenceError::StateProof)?;
                Ok(ProgramLifecycle::Active)
            }
            Some(value) => {
                verify_state(next, "layerx", &key, value, &self.status_ops)
                    .map_err(|_| ProtocolEvidenceError::StateProof)?;
                if value.len() != 54
                    || value[0] != 1
                    || value[2..34] != self.program.bytes()
                    || value[34..42] == [0; 8]
                    || value[42..50] == [0; 8]
                {
                    return Err(ProtocolEvidenceError::LifecycleProof);
                }
                match value[1] {
                    2 => Ok(ProgramLifecycle::Deprecated),
                    3 => Ok(ProgramLifecycle::Tombstoned),
                    _ => Err(ProtocolEvidenceError::LifecycleProof),
                }
            }
        }
    }
}
