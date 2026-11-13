//! Comet's deterministic ABCI result bytes and simple Merkle inclusion proofs.

use super::{CheckpointReadError, CommitValidator, ServedCheckpoint};
use layerx_programs::chain_proof::merkle::SimpleProof;

const PREFIX: &[u8; 5] = b"LXAR\x01";

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AbciResultProof {
    pub result: Vec<u8>,
    pub proof: SimpleProof,
}

impl AbciResultProof {
    /// The first field carries the version, total, index and exact protobuf
    /// leaf; remaining fields are the producer's bottom-up SHA-256 aunts.
    /// # Errors
    /// Returns an error if node acquisition fails or the committed checkpoint, target, result
    /// root or proof cannot be verified.
    pub fn encode(&self) -> Result<Vec<Vec<u8>>, CheckpointReadError> {
        if self.proof.total <= 0
            || self.proof.index < 0
            || self.proof.index >= self.proof.total
            || self.proof.aunts.len() > 64
            || self.result.len() > 262_144
        {
            return Err(CheckpointReadError::InvalidProof);
        }
        let mut leaf = PREFIX.to_vec();
        leaf.extend_from_slice(&self.proof.total.to_be_bytes());
        leaf.extend_from_slice(&self.proof.index.to_be_bytes());
        leaf.extend_from_slice(&self.result);
        let mut fields = vec![leaf];
        fields.extend(self.proof.aunts.iter().map(|aunt| aunt.to_vec()));
        Ok(fields)
    }

    /// # Errors
    /// Returns an error if node acquisition fails or the committed checkpoint, target, result
    /// root or proof cannot be verified.
    pub fn decode(fields: &[Vec<u8>]) -> Result<Self, CheckpointReadError> {
        use sha2::{Digest, Sha256};
        let bad = || CheckpointReadError::InvalidProof;
        let leaf = fields.first().ok_or_else(bad)?;
        if leaf.len() < 21
            || !leaf.starts_with(PREFIX)
            || fields.len() > 65
            || fields.iter().map(Vec::len).sum::<usize>() > 262_144
        {
            return Err(bad());
        }
        let total = i64::from_be_bytes(leaf[5..13].try_into().map_err(|_| bad())?);
        let index = i64::from_be_bytes(leaf[13..21].try_into().map_err(|_| bad())?);
        if total <= 0 || index < 0 || index >= total {
            return Err(bad());
        }
        let result = leaf[21..].to_vec();
        let mut hash = Sha256::new();
        hash.update([0]);
        hash.update(&result);
        let aunts = fields[1..]
            .iter()
            .map(|field| field.as_slice().try_into().map_err(|_| bad()))
            .collect::<Result<Vec<_>, _>>()?;
        Ok(Self {
            result,
            proof: SimpleProof {
                total,
                index,
                leaf_hash: hash.finalize().into(),
                aunts,
            },
        })
    }

    /// # Errors
    /// Returns an error if node acquisition fails or the committed checkpoint, target, result
    /// root or proof cannot be verified.
    pub fn verify(&self, root: [u8; 32], index: u64) -> Result<(), CheckpointReadError> {
        if u64::try_from(self.proof.index).ok() != Some(index) || self.proof.aunts.len() > 64 {
            return Err(CheckpointReadError::TargetMismatch);
        }
        self.proof
            .verify(&root, &self.result)
            .map_err(|_| CheckpointReadError::RootMismatch)
    }

    /// # Errors
    /// Returns an error if node acquisition fails or the committed checkpoint, target, result
    /// root or proof cannot be verified.
    pub fn verify_committed(
        &self,
        result_height: u64,
        index: u64,
        checkpoint: &ServedCheckpoint,
        chain_id: &str,
        trusted_validators: &[CommitValidator],
    ) -> Result<(), CheckpointReadError> {
        if result_height.checked_add(1) != Some(checkpoint.height)
            || checkpoint.validators != trusted_validators
        {
            return Err(CheckpointReadError::TargetMismatch);
        }
        let verified = super::checkpoint(
            checkpoint.height,
            chain_id,
            &checkpoint.header,
            &checkpoint.commit,
            trusted_validators,
        )?;
        if verified != *checkpoint {
            return Err(CheckpointReadError::CheckpointBlockMismatch);
        }
        self.verify(
            checkpoint
                .header
                .last_results_hash
                .ok_or(CheckpointReadError::RootMismatch)?,
            index,
        )
    }
}

#[derive(Clone, Copy)]
pub(crate) enum ProtoValue<'a> {
    Integer(u64),
    Bytes(&'a [u8]),
}

pub(crate) fn fields(bytes: &[u8]) -> Result<Vec<(u32, ProtoValue<'_>)>, CheckpointReadError> {
    fn varint(bytes: &[u8], cursor: &mut usize) -> Result<u64, CheckpointReadError> {
        let mut value = 0;
        for shift in (0..70).step_by(7) {
            let byte = *bytes
                .get(*cursor)
                .ok_or(CheckpointReadError::InvalidProof)?;
            *cursor += 1;
            if shift == 63 && byte > 1 {
                return Err(CheckpointReadError::InvalidProof);
            }
            value |= u64::from(byte & 127) << shift;
            if byte & 128 == 0 {
                if shift != 0 && byte == 0 {
                    return Err(CheckpointReadError::InvalidProof);
                }
                return Ok(value);
            }
        }
        Err(CheckpointReadError::InvalidProof)
    }
    let mut cursor = 0;
    let mut output = Vec::new();
    while cursor < bytes.len() {
        let tag = varint(bytes, &mut cursor)?;
        let number = u32::try_from(tag >> 3).map_err(|_| CheckpointReadError::InvalidProof)?;
        if number == 0 || number > 0x1fff_ffff {
            return Err(CheckpointReadError::InvalidProof);
        }
        let value = match tag & 7 {
            0 => ProtoValue::Integer(varint(bytes, &mut cursor)?),
            2 => {
                let length = usize::try_from(varint(bytes, &mut cursor)?)
                    .map_err(|_| CheckpointReadError::InvalidProof)?;
                let end = cursor
                    .checked_add(length)
                    .ok_or(CheckpointReadError::InvalidProof)?;
                let value = bytes
                    .get(cursor..end)
                    .ok_or(CheckpointReadError::InvalidProof)?;
                cursor = end;
                ProtoValue::Bytes(value)
            }
            _ => return Err(CheckpointReadError::InvalidProof),
        };
        output.push((number, value));
    }
    Ok(output)
}

pub(crate) fn evm_response(result: &[u8]) -> Result<&[u8], CheckpointReadError> {
    let mut data = None;
    let mut previous = 0;
    for (number, value) in fields(result)? {
        if number <= previous {
            return Err(CheckpointReadError::InvalidProof);
        }
        previous = number;
        match (number, value) {
            (2, ProtoValue::Bytes(bytes)) => data = Some(bytes),
            (1, ProtoValue::Integer(0)) | (5 | 6, ProtoValue::Integer(_)) => {}
            _ => return Err(CheckpointReadError::InvalidProof),
        }
    }
    let messages = fields(data.ok_or(CheckpointReadError::InvalidProof)?)?;
    let [(1, ProtoValue::Bytes(message))] = messages.as_slice() else {
        return Err(CheckpointReadError::InvalidProof);
    };
    let message = fields(message)?;
    let [(1, ProtoValue::Bytes(kind)), (2, ProtoValue::Bytes(response))] = message.as_slice()
    else {
        return Err(CheckpointReadError::InvalidProof);
    };
    if *kind != b"/paxprotocol.paxchain.evm.MsgEVMTransaction" {
        return Err(CheckpointReadError::InvalidProof);
    }
    Ok(response)
}

pub(crate) fn verify_native_receipt(
    result: &[u8],
    receipt: &layerx_client::rpc::receipt::AuthenticatedReceipt,
) -> Result<(), CheckpointReadError> {
    let response = evm_response(result)?;
    let execution = receipt.execution();
    let mut gas = 0;
    let mut vm_error = &[][..];
    let mut returned = &[][..];
    let mut hash = None;
    let mut logs = Vec::new();
    let mut previous = 0;
    for (number, value) in fields(response)? {
        if number < previous || number == previous && number != 5 {
            return Err(CheckpointReadError::InvalidProof);
        }
        previous = number;
        match (number, value) {
            (1, ProtoValue::Integer(value)) => gas = value,
            (2, ProtoValue::Bytes(value)) => vm_error = value,
            (3, ProtoValue::Bytes(value)) => returned = value,
            (4, ProtoValue::Bytes(value)) => hash = Some(value),
            (5, ProtoValue::Bytes(value)) => logs.push(value),
            _ => return Err(CheckpointReadError::InvalidProof),
        }
    }
    let text = std::str::from_utf8(hash.ok_or(CheckpointReadError::InvalidProof)?)
        .map_err(|_| CheckpointReadError::InvalidProof)?;
    let hash = layerx_programs::hex::decode_digest(
        text.strip_prefix("0x")
            .ok_or(CheckpointReadError::InvalidProof)?,
    )
    .map_err(|_| CheckpointReadError::InvalidProof)?;
    if hash != execution.tx_hash
        || gas != execution.gas_used
        || returned != execution.return_data
        || vm_error != execution.vm_error.as_bytes()
        || vm_error.is_empty() != execution.success
        || logs.len() != execution.logs.len()
    {
        return Err(CheckpointReadError::TargetMismatch);
    }
    for (wire, expected) in logs.into_iter().zip(&execution.logs) {
        let mut address = None;
        let mut topics = Vec::new();
        let mut data = &[][..];
        let mut index = 0;
        let mut previous = 0;
        for (number, value) in fields(wire)? {
            if number < previous || number == previous && number != 2 {
                return Err(CheckpointReadError::InvalidProof);
            }
            previous = number;
            let hex = |bytes| -> Result<Vec<u8>, CheckpointReadError> {
                let text =
                    std::str::from_utf8(bytes).map_err(|_| CheckpointReadError::InvalidProof)?;
                layerx_programs::hex::decode(text.strip_prefix("0x").unwrap_or(text))
                    .map_err(|_| CheckpointReadError::InvalidProof)
            };
            match (number, value) {
                (1, ProtoValue::Bytes(bytes)) => address = Some(hex(bytes)?),
                (2, ProtoValue::Bytes(bytes)) => topics.push(hex(bytes)?),
                (3, ProtoValue::Bytes(bytes)) => data = bytes,
                (4, ProtoValue::Integer(value)) => index = value,
                (5, ProtoValue::Integer(0 | 1)) => {}
                _ => return Err(CheckpointReadError::InvalidProof),
            }
        }
        if address.as_deref() != Some(expected.address.as_slice())
            || data != expected.data
            || index != expected.log_index
            || topics.len() != expected.topics.len()
            || topics
                .iter()
                .zip(&expected.topics)
                .any(|(a, b)| a.as_slice() != b.as_slice())
        {
            return Err(CheckpointReadError::TargetMismatch);
        }
    }
    Ok(())
}

/// Reads the transaction identity covered by a deterministic EVM result. Empty
/// native results do not name an EVM outcome; malformed nonempty data refuses.
pub(crate) fn transaction_hash(result: &[u8]) -> Result<Option<[u8; 32]>, CheckpointReadError> {
    let outer = fields(result)?;
    let data = outer
        .iter()
        .find_map(|(number, value)| match (number, value) {
            (2, ProtoValue::Bytes(data)) => Some(*data),
            _ => None,
        });
    let Some(data) = data.filter(|data| !data.is_empty()) else {
        return Ok(None);
    };
    let messages = fields(data)?;
    let [(1, ProtoValue::Bytes(message))] = messages.as_slice() else {
        return Err(CheckpointReadError::InvalidProof);
    };
    let message = fields(message)?;
    let [(1, ProtoValue::Bytes(kind)), (2, ProtoValue::Bytes(_))] = message.as_slice() else {
        return Err(CheckpointReadError::InvalidProof);
    };
    if *kind != b"/paxprotocol.paxchain.evm.MsgEVMTransaction" {
        return Ok(None);
    }
    let response = evm_response(result)?;
    let mut hash = None;
    for (number, value) in fields(response)? {
        if let (4, ProtoValue::Bytes(bytes)) = (number, value) {
            if hash.is_some() {
                return Err(CheckpointReadError::InvalidProof);
            }
            let text = std::str::from_utf8(bytes).map_err(|_| CheckpointReadError::InvalidProof)?;
            hash = Some(
                layerx_programs::hex::decode_digest(
                    text.strip_prefix("0x")
                        .ok_or(CheckpointReadError::InvalidProof)?,
                )
                .map_err(|_| CheckpointReadError::InvalidProof)?,
            );
        }
    }
    hash.map(Some).ok_or(CheckpointReadError::InvalidProof)
}
