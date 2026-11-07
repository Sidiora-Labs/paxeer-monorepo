//! Availability of committed sequencer blocks.

use layerx_client::sequencer::{SequencerEndpoint, SequencerError};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct AvailabilityRequest {
    pub height: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum AvailabilityRead {
    Available {
        height: u64,
        transactions_root: [u8; 32],
        receipts_root: [u8; 32],
        transaction_count: u64,
    },
    Pruned {
        height: u64,
    },
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum AvailabilityReadError {
    Ahead { requested: u64, head: u64 },
    Sequencer(SequencerError),
    Arithmetic,
}

impl From<SequencerError> for AvailabilityReadError {
    fn from(error: SequencerError) -> Self {
        Self::Sequencer(error)
    }
}

/// # Errors
/// Refuses heights above the node's head, propagates RPC failures, and refuses
/// a transaction count that cannot be represented as a `u64`.
pub fn availability(
    endpoint: &SequencerEndpoint,
    request: &AvailabilityRequest,
) -> Result<AvailabilityRead, AvailabilityReadError> {
    let head = endpoint.block_number()?;
    if request.height > head {
        return Err(AvailabilityReadError::Ahead {
            requested: request.height,
            head,
        });
    }
    let Some(block) = endpoint.block_by_number(request.height)? else {
        return Ok(AvailabilityRead::Pruned {
            height: request.height,
        });
    };
    Ok(AvailabilityRead::Available {
        height: block.number,
        transactions_root: block.transactions_root,
        receipts_root: block.receipts_root,
        transaction_count: u64::try_from(block.transactions.len())
            .map_err(|_| AvailabilityReadError::Arithmetic)?,
    })
}
