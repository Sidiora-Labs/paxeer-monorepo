use super::{funding, required};

#[path = "../../funded_inputs.rs"]
mod provisioned;

pub use provisioned::{
    hex_encode, now_ms, random32, sha256, write, Cluster, HttpAnswer, ProvisionedInputs, NETWORK_ID,
};

pub const PROTOCOL_VERSION: u16 = layerx_wire::limits::PROTOCOL_VERSION;

pub fn read_commit_coverage(
    root: &std::path::Path,
    activity: [u8; 32],
) -> Result<layerx_sdk::rpc_verification::node_commit::NodeCommitCoverage, layerx_sdk::rpc::RpcError>
{
    use layerx_sdk::{
        rpc::RpcError,
        rpc_verification::node_commit::{NodeCommitCoverage, NodeTransactionProof},
    };
    let document =
        std::fs::read(root.join(format!("commit-coverage-{}.json", hex_encode(&activity))))
            .map_err(|_| RpcError::MissingFinalityTrust)?;
    let value: serde_json::Value =
        serde_json::from_slice(&document).map_err(|_| RpcError::InvalidResponse)?;
    fn bytes(value: &serde_json::Value) -> Result<Vec<u8>, RpcError> {
        let text = value.as_str().ok_or(RpcError::InvalidResponse)?;
        if text.len() % 2 != 0 || !text.is_ascii() {
            return Err(RpcError::InvalidResponse);
        }
        text.as_bytes()
            .chunks_exact(2)
            .map(|pair| {
                u8::from_str_radix(
                    std::str::from_utf8(pair).map_err(|_| RpcError::InvalidResponse)?,
                    16,
                )
                .map_err(|_| RpcError::InvalidResponse)
            })
            .collect()
    }
    fn digest(value: &serde_json::Value) -> Result<[u8; 32], RpcError> {
        bytes(value)?
            .try_into()
            .map_err(|_| RpcError::InvalidResponse)
    }
    let coverage = NodeCommitCoverage {
        covering_height: value["covering_height"]
            .as_u64()
            .ok_or(RpcError::InvalidResponse)?,
        batch_transaction: bytes(&value["batch_transaction"])?,
        transaction_proof: NodeTransactionProof {
            total: value["transaction_proof"]["total"]
                .as_u64()
                .ok_or(RpcError::InvalidResponse)?,
            index: value["transaction_proof"]["index"]
                .as_u64()
                .ok_or(RpcError::InvalidResponse)?,
            aunts: value["transaction_proof"]["aunts"]
                .as_array()
                .ok_or(RpcError::InvalidResponse)?
                .iter()
                .map(digest)
                .collect::<Result<_, _>>()?,
        },
        activity_ids: value["activity_ids"]
            .as_array()
            .ok_or(RpcError::InvalidResponse)?
            .iter()
            .map(digest)
            .collect::<Result<_, _>>()?,
    };
    if !coverage.activity_ids.contains(&activity) {
        return Err(RpcError::Verification);
    }
    Ok(coverage)
}
