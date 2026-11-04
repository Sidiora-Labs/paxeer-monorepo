use layerx_ap2::PortableLayerXEvidence;
use layerx_proof::inclusion::SequencerAuthorization;
use serde::{Deserialize, Serialize};

pub fn validate_startup_arguments(arguments: &[String]) -> Result<(), String> {
    if arguments.is_empty() {
        Ok(())
    } else {
        Err("invalid interoperability gateway arguments: configure startup through the environment; verify PortableReceiptV2 evidence with verify_receipted_json".to_owned())
    }
}

/// Operational consumers use the same string enum as the readiness producer.
#[derive(Clone, Copy, Debug, Deserialize, Serialize, Eq, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum ComponentStatus {
    Ready,
    Unavailable,
}

impl ComponentStatus {
    pub const fn from_ready(ready: bool) -> Self {
        if ready {
            Self::Ready
        } else {
            Self::Unavailable
        }
    }
}

#[derive(Debug, Deserialize)]
pub struct ReadinessComponents {
    pub durable_gateway_store: ComponentStatus,
    pub sequencer: ComponentStatus,
}

#[derive(Debug, Deserialize)]
pub struct ReadinessDocument {
    pub readiness_version: u16,
    pub service: String,
    pub status: String,
    pub components: ReadinessComponents,
}

impl ReadinessDocument {
    pub fn from_json(bytes: &[u8]) -> Result<Self, String> {
        if bytes.len() > 512 * 1024 {
            return Err("readiness response exceeds its bound".to_owned());
        }
        let document: Self = serde_json::from_slice(bytes).map_err(|error| error.to_string())?;
        if document.readiness_version != 1
            || document.service != "layerx-interop-gateway"
            || !matches!(document.status.as_str(), "ready" | "degraded")
        {
            return Err("unsupported interoperability readiness response".to_owned());
        }
        Ok(document)
    }

    pub fn is_ready(&self) -> bool {
        self.status == "ready"
            && self.components.sequencer == ComponentStatus::Ready
            && self.components.durable_gateway_store == ComponentStatus::Ready
    }
}

/// Verifies concrete payment evidence offline; /readyz is operational status,
/// and supplies no receipt or consensus inclusion proof.
pub fn verify_receipted_json(
    bytes: &[u8],
    receipt_keys: &[SequencerAuthorization],
) -> Result<[u8; 32], String> {
    PortableLayerXEvidence::verify_json(bytes, receipt_keys)
        .map(|verified| verified.receipt_digest())
        .map_err(|error| error.to_string())
}
