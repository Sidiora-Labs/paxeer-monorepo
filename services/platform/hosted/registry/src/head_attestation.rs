use layerx_programs::{
    chain_proof::{
        decode::{decode_commitment_proof, CommitmentProof},
        merkle::SimpleProof,
    },
    hex,
};
use serde_json::Value;

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct ChainHeadEvidence {
    pub(crate) commitment: CommitmentProof,
    pub(crate) transaction: Vec<u8>,
    pub(crate) transaction_proof: SimpleProof,
}

pub(crate) fn chain_head_evidence(value: &Value) -> Result<ChainHeadEvidence, String> {
    let value = &value["chain_evidence"];
    let commitment = decode_commitment_proof(
        &hex::decode(field(value, "commitment_hex")?).map_err(|error| error.to_string())?,
    )
    .map_err(|error| format!("chain commitment refused: {error:?}"))?;
    let transaction =
        hex::decode(field(value, "transaction_hex")?).map_err(|error| error.to_string())?;
    let proof = &value["transaction_proof"];
    let total = proof["total"]
        .as_i64()
        .filter(|total| *total > 0)
        .ok_or("invalid transaction proof total")?;
    let index = proof["index"]
        .as_i64()
        .filter(|index| *index >= 0 && *index < total)
        .ok_or("invalid transaction proof index")?;
    let leaf_hash =
        hex::decode_digest(field(proof, "leaf_hash")?).map_err(|error| error.to_string())?;
    let aunts = proof["aunts"]
        .as_array()
        .filter(|aunts| aunts.len() <= 64)
        .ok_or("invalid transaction proof aunts")?
        .iter()
        .map(|aunt| {
            hex::decode_digest(aunt.as_str().ok_or("invalid transaction proof aunt")?)
                .map_err(|error| error.to_string())
        })
        .collect::<Result<Vec<_>, String>>()?;
    Ok(ChainHeadEvidence {
        commitment,
        transaction,
        transaction_proof: SimpleProof {
            total,
            index,
            leaf_hash,
            aunts,
        },
    })
}

fn field<'a>(value: &'a Value, name: &str) -> Result<&'a str, String> {
    value[name]
        .as_str()
        .ok_or_else(|| format!("chain evidence omitted {name}"))
}
