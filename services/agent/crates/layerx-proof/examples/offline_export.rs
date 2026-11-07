//! Minimal offline-verifier entry point.

use layerx_proof::export::{
    verify, verify_complete, CompleteExportError, CompleteVerificationReport,
    IndependentOfflineTrust, OfflineExport,
};
use layerx_proof::export_codec::CompleteOfflineArtifact;
use layerx_types::rung::Rung;

fn verify_without_network(
    artifact: &OfflineExport,
) -> Result<layerx_proof::export::VerificationReport, layerx_proof::export::ExportVerificationError>
{
    verify(artifact)
}

fn verify_complete_without_network(
    artifact: &CompleteOfflineArtifact,
    trust: &IndependentOfflineTrust,
    requested: Rung,
) -> Result<CompleteVerificationReport, CompleteExportError> {
    verify_complete(artifact, trust, requested)
}

fn main() {
    let verifier: fn(
        &OfflineExport,
    ) -> Result<
        layerx_proof::export::VerificationReport,
        layerx_proof::export::ExportVerificationError,
    > = verify_without_network;
    let complete: fn(
        &CompleteOfflineArtifact,
        &IndependentOfflineTrust,
        Rung,
    ) -> Result<CompleteVerificationReport, CompleteExportError> = verify_complete_without_network;
    let _ = (verifier, complete);
}
