use layerx_programs_registry::chain_proof::{
    ics23::{verify_existence, ExistenceProof, ProofSpec},
    multistore::verify_store_root,
    proto::{ProtoReader, ProtoValue},
    ChainProofError,
};

use crate::sequencer::StorageWitness;

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum StorageWitnessError {
    Operations,
    Key,
    Value,
    StoreRoot,
    AppHash,
    Encoding(ChainProofError),
}

/// # Errors
///
/// Returns an encoding error for malformed proofs, or a mismatch error when operations, keys, values, store roots or the app hash fail verification.
pub fn verify_storage_witness(
    witness: &StorageWitness,
    app_hash: [u8; 32],
) -> Result<(), StorageWitnessError> {
    if witness.ops.len() != 2
        || witness.ops[0].op_type != "ics23:iavl"
        || witness.ops[1].op_type != "ics23:simple"
        || witness.ops[1].key != b"evm"
    {
        return Err(StorageWitnessError::Operations);
    }
    let key = [&[3][..], &witness.address, &witness.slot].concat();
    if witness.ops[0].key != key {
        return Err(StorageWitnessError::Key);
    }
    let proof = existence(&witness.ops[0].data)?;
    if proof.key != key {
        return Err(StorageWitnessError::Key);
    }
    if proof.value != witness.value {
        return Err(StorageWitnessError::Value);
    }
    let outer = existence(&witness.ops[1].data)?;
    if outer.key != b"evm" {
        return Err(StorageWitnessError::StoreRoot);
    }
    let root: [u8; 32] = outer
        .value
        .as_slice()
        .try_into()
        .map_err(|_| StorageWitnessError::StoreRoot)?;
    verify_existence(&proof, &ProofSpec::iavl(), &root, &key, &witness.value)
        .map_err(|_| StorageWitnessError::StoreRoot)?;
    verify_store_root("evm", root, &witness.ops[1].data, app_hash)
        .map_err(|_| StorageWitnessError::AppHash)
}

pub(crate) fn existence(data: &[u8]) -> Result<ExistenceProof, StorageWitnessError> {
    let mut reader = ProtoReader::new(data);
    let field = reader
        .next_field()
        .map_err(StorageWitnessError::Encoding)?
        .ok_or(StorageWitnessError::Encoding(ChainProofError::Mismatch))?;
    let (1, ProtoValue::Bytes(bytes)) = (field.number, field.value) else {
        return Err(StorageWitnessError::Encoding(ChainProofError::Mismatch));
    };
    reader.finish().map_err(StorageWitnessError::Encoding)?;
    ExistenceProof::decode(bytes).map_err(StorageWitnessError::Encoding)
}
