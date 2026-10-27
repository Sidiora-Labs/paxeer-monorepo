use super::{
    decode::ProofOp,
    ics23::{verify_existence, ExistenceProof, ProofSpec},
    ics23_absent::{verify_non_existence, NonExistenceProof},
    multistore::verify_store_root,
    proto::{ProtoReader, ProtoValue},
    ChainProofError, VerifiedChainHead,
};

pub type StateHeight = i64;

pub fn verify_state(
    next: &VerifiedChainHead,
    store_name: &str,
    key: &[u8],
    value: &[u8],
    ops: &[ProofOp],
) -> Result<StateHeight, ChainProofError> {
    let (height, root) = state_root(next, store_name, key, ops)?;
    let proof = ExistenceProof::decode(commitment(&ops[0].data, 1)?)?;
    verify_existence(&proof, &ProofSpec::iavl(), &root, key, value)?;
    verify_store_root(store_name, root, &ops[1].data, next.app_hash())?;
    Ok(height)
}

pub fn verify_absent(
    next: &VerifiedChainHead,
    store_name: &str,
    key: &[u8],
    ops: &[ProofOp],
) -> Result<StateHeight, ChainProofError> {
    let (height, root) = state_root(next, store_name, key, ops)?;
    let proof = NonExistenceProof::decode(commitment(&ops[0].data, 2)?)?;
    verify_non_existence(&proof, &ProofSpec::iavl(), &root, key)?;
    verify_store_root(store_name, root, &ops[1].data, next.app_hash())?;
    Ok(height)
}

fn state_root(
    next: &VerifiedChainHead,
    store_name: &str,
    key: &[u8],
    ops: &[ProofOp],
) -> Result<(StateHeight, [u8; 32]), ChainProofError> {
    if next.height() <= 1
        || ops.len() != 2
        || store_name.is_empty()
        || ops[0].op_type != "ics23:iavl"
        || ops[0].key != key
        || ops[1].op_type != "ics23:simple"
        || ops[1].key != store_name.as_bytes()
    {
        return Err(ChainProofError::Mismatch);
    }
    let store = ExistenceProof::decode(commitment(&ops[1].data, 1)?)?;
    if store.key != store_name.as_bytes() {
        return Err(ChainProofError::Mismatch);
    }
    let root = store
        .value
        .as_slice()
        .try_into()
        .map_err(|_| ChainProofError::Mismatch)?;
    Ok((next.height() - 1, root))
}

fn commitment(data: &[u8], variant: u32) -> Result<&[u8], ChainProofError> {
    let mut reader = ProtoReader::new(data);
    let field = reader.next_field()?.ok_or(ChainProofError::Mismatch)?;
    let proof = match (field.number, field.value) {
        (number, ProtoValue::Bytes(proof)) if number == variant => proof,
        _ => return Err(ChainProofError::Mismatch),
    };
    reader.finish()?;
    Ok(proof)
}
