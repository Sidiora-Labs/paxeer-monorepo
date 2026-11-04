//! Historical checkpointed receipt shape, with no current finality privileges.

use super::checkpoint::{
    ActivityInclusionProof, CheckpointCertificate, PaxeerSettlementReference, StateInclusionProof,
};
use crate::receipt::LxpReceipt;

/// Historical archive receipt. Its guarantor certificate has no current finality authority.
#[deprecated(
    note = "historical archive only; current receipts require verified covering-block evidence"
)]
#[allow(dead_code)]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CheckpointedLxpReceipt {
    receipt: LxpReceipt,
    canonical_activity: Box<[u8]>,
    state_leaf: Box<[u8]>,
    activity_inclusion_proof: ActivityInclusionProof,
    state_inclusion_proof: StateInclusionProof,
    checkpoint_id: [u8; 32],
    guarantor_certificate: CheckpointCertificate,
    paxeer_settlement_reference: PaxeerSettlementReference,
}
