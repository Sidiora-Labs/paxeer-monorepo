use super::ChainProofError;

pub const MAX_FIELD_BYTES: usize = 524_288;
pub const MAX_PROOF_OPS: usize = 2;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProofOp {
    pub op_type: String,
    pub key: Vec<u8>,
    pub data: Vec<u8>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommitmentProof {
    pub signed_header: Vec<u8>,
    pub commit: Vec<u8>,
    pub validator_set: Vec<u8>,
    pub store_name: String,
    pub key: Vec<u8>,
    pub value: Option<Vec<u8>>,
    pub proof_ops: Vec<ProofOp>,
}

impl CommitmentProof {
    pub fn encode(&self) -> Result<Vec<u8>, ChainProofError> {
        if self.proof_ops.len() > MAX_PROOF_OPS {
            return Err(ChainProofError::NonCanonical);
        }
        let mut bytes = b"LXCP\x01".to_vec();
        for field in [
            self.signed_header.as_slice(),
            self.commit.as_slice(),
            self.validator_set.as_slice(),
            self.store_name.as_bytes(),
            self.key.as_slice(),
        ] {
            put_field(&mut bytes, field)?;
        }
        match &self.value {
            None => bytes.push(0),
            Some(value) => {
                bytes.push(1);
                put_field(&mut bytes, value)?;
            }
        }
        let count =
            u32::try_from(self.proof_ops.len()).map_err(|_| ChainProofError::NonCanonical)?;
        bytes.extend_from_slice(&count.to_be_bytes());
        for op in &self.proof_ops {
            put_field(&mut bytes, op.op_type.as_bytes())?;
            put_field(&mut bytes, &op.key)?;
            put_field(&mut bytes, &op.data)?;
        }
        Ok(bytes)
    }
}

pub fn decode_commitment_proof(bytes: &[u8]) -> Result<CommitmentProof, ChainProofError> {
    let mut reader = BundleReader { bytes, cursor: 0 };
    if reader.take(4)? != b"LXCP" || reader.take(1)? != [1] {
        return Err(ChainProofError::NonCanonical);
    }
    let signed_header = reader.field()?.to_vec();
    let commit = reader.field()?.to_vec();
    let validator_set = reader.field()?.to_vec();
    let store_name = reader.text()?;
    let key = reader.field()?.to_vec();
    let value = match reader.take(1)?[0] {
        0 => None,
        1 => Some(reader.field()?.to_vec()),
        _ => return Err(ChainProofError::NonCanonical),
    };
    let count = usize::try_from(reader.u32()?).map_err(|_| ChainProofError::NonCanonical)?;
    if count > MAX_PROOF_OPS {
        return Err(ChainProofError::NonCanonical);
    }
    let mut proof_ops = Vec::new();
    for _ in 0..count {
        proof_ops.push(ProofOp {
            op_type: reader.text()?,
            key: reader.field()?.to_vec(),
            data: reader.field()?.to_vec(),
        });
    }
    if reader.cursor != bytes.len() {
        return Err(ChainProofError::NonCanonical);
    }
    Ok(CommitmentProof {
        signed_header,
        commit,
        validator_set,
        store_name,
        key,
        value,
        proof_ops,
    })
}

fn put_field(bytes: &mut Vec<u8>, field: &[u8]) -> Result<(), ChainProofError> {
    if field.len() > MAX_FIELD_BYTES {
        return Err(ChainProofError::NonCanonical);
    }
    let length = u32::try_from(field.len()).map_err(|_| ChainProofError::NonCanonical)?;
    bytes.extend_from_slice(&length.to_be_bytes());
    bytes.extend_from_slice(field);
    Ok(())
}

struct BundleReader<'a> {
    bytes: &'a [u8],
    cursor: usize,
}

impl<'a> BundleReader<'a> {
    fn take(&mut self, length: usize) -> Result<&'a [u8], ChainProofError> {
        let end = self
            .cursor
            .checked_add(length)
            .ok_or(ChainProofError::Truncated)?;
        let field = self
            .bytes
            .get(self.cursor..end)
            .ok_or(ChainProofError::Truncated)?;
        self.cursor = end;
        Ok(field)
    }

    fn u32(&mut self) -> Result<u32, ChainProofError> {
        let mut bytes = [0; 4];
        bytes.copy_from_slice(self.take(4)?);
        Ok(u32::from_be_bytes(bytes))
    }

    fn field(&mut self) -> Result<&'a [u8], ChainProofError> {
        let length = usize::try_from(self.u32()?).map_err(|_| ChainProofError::NonCanonical)?;
        if length > MAX_FIELD_BYTES {
            return Err(ChainProofError::NonCanonical);
        }
        self.take(length)
    }

    fn text(&mut self) -> Result<String, ChainProofError> {
        let text = std::str::from_utf8(self.field()?).map_err(|_| ChainProofError::NonCanonical)?;
        Ok(text.to_owned())
    }
}
