use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};
use layerx_proof::sequencer_logs::SequencerLog;
use sha2::{Digest as _, Sha256};

use crate::error::GatewayError;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SettlementRung {
    Submitted,
    Receipted,
    Committed {
        block_number: u64,
        block_hash: [u8; 32],
    },
}

impl SettlementRung {
    #[must_use]
    pub const fn label(self) -> &'static str {
        match self {
            Self::Submitted => "submitted",
            Self::Receipted => "receipted",
            Self::Committed { .. } => "committed",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SequencerEvidence {
    pub transaction_hash: [u8; 32],
    pub signed_receipt: Vec<u8>,
    pub logs: Vec<SequencerLog>,
    pub rung: SettlementRung,
}

pub fn canonical_receipt_digest(signed_receipt: &[u8]) -> Result<[u8; 32], GatewayError> {
    let missing_signature = GatewayError::ReceiptRejected(VerificationFailure {
        check: ReceiptCheck::MissingSignature,
    });
    let unsigned_end = signed_receipt
        .len()
        .checked_sub(69)
        .ok_or(missing_signature)?;
    let (unsigned_prefix, signature) = signed_receipt.split_at(unsigned_end);
    if !signature.starts_with(&[1, 0, 0, 0, 64]) {
        return Err(missing_signature);
    }
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(unsigned_prefix);
    digest.update([0]);
    Ok(digest.finalize().into())
}
