use crate::{
    chain_evidence::programs_state_key,
    chain_proof::{
        decode::ProofOp,
        state::{verify_absent, verify_state},
        VerifiedChainHead,
    },
    ProgramId, ProtocolEvidenceError,
};

const STATUS_KEY_PREFIX: &[u8] = b"wind-down\0s";
const STATUS_RECORD_BYTES: usize = 54;
const STORE_NAME: &str = "layerx";

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ChainWindDownProof {
    pub program: ProgramId,
    pub value: Option<Vec<u8>>,
    pub ops: Vec<ProofOp>,
}

impl ChainWindDownProof {
    pub fn verify(
        &self,
        next: &VerifiedChainHead,
    ) -> Result<Option<[u8; STATUS_RECORD_BYTES]>, ProtocolEvidenceError> {
        let mut kernel_key = Vec::with_capacity(STATUS_KEY_PREFIX.len() + 32);
        kernel_key.extend_from_slice(STATUS_KEY_PREFIX);
        kernel_key.extend_from_slice(&self.program.bytes());
        let key = programs_state_key(&kernel_key);

        match &self.value {
            Some(value) => {
                let status = value
                    .as_slice()
                    .try_into()
                    .map_err(|_| ProtocolEvidenceError::Encoding)?;
                verify_state(next, STORE_NAME, &key, value, &self.ops)
                    .map_err(|_| ProtocolEvidenceError::StateProof)?;
                Ok(Some(status))
            }
            None => {
                verify_absent(next, STORE_NAME, &key, &self.ops)
                    .map_err(|_| ProtocolEvidenceError::StateProof)?;
                Ok(None)
            }
        }
    }
}
