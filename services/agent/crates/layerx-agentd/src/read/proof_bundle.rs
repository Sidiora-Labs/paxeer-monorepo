use layerx_agent_api::error::{ApiError, RequestId};
use layerx_agent_api::proof::{ProofBundle, ProofBundleTarget};
use layerx_agent_api::read::VerifiedRead;
use layerx_agent_api::verify::Rung;
use layerx_client::Client;
use layerx_types::payload::ModuleRegistry;

pub(crate) struct ProofBundleScope<'a> {
    pub account: [u8; 32],
    pub asset: [u8; 32],
    pub retained_activity: &'a [u8],
    pub served_receipt: &'a [u8],
    pub correlation: u64,
}

pub(crate) fn acquire(
    _node: &mut Client,
    _registry: &ModuleRegistry,
    _target: ProofBundleTarget,
    _requested: Rung,
    scope: ProofBundleScope<'_>,
) -> Result<VerifiedRead<ProofBundle>, ApiError> {
    Err(
        ApiError::unavailable_capability(RequestId(scope.correlation), "read.proof_bundle")
            .expect("static operation name"),
    )
}
