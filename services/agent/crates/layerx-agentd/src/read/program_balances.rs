//! Program custody reads from layerxprograms at a committed block height.

use super::program_node::ProgramReadError;
use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use layerx_programs::{ProgramId, ProgramLifecycle, VerifiedProgramBalanceRead};

pub use super::program_node::read_committed_program_balances as program_balances_from_protocol;

/// A program balance reader bound to an authenticated sequencer endpoint.
pub struct ProtocolProgramBalanceReader {
    endpoint: SequencerEndpoint,
}

impl ProtocolProgramBalanceReader {
    /// Binds the reader to the sequencer used for committed block lookups and calls.
    #[must_use]
    pub fn bind(endpoint: SequencerEndpoint) -> Self {
        Self { endpoint }
    }

    /// Reads the requested accounts from layerxprograms at one committed height.
    ///
    /// # Errors
    /// Refuses unavailable blocks, malformed ABI results, account substitution
    /// and a block that changes while the read is in progress.
    pub fn read(
        &self,
        program: ProgramId,
        accounts: &[[u8; 32]],
        height: u64,
    ) -> Result<ProgramBalanceRead<CommittedProgramBalanceFreshness>, SequencerError> {
        super::program_node::LayerxdProgramBalanceReader::read_at(
            &self.endpoint,
            program,
            accounts,
            height,
        )
    }
}

/// Coordinates of the committed state used by every call in a balance read.
/// An eth_call answer carries no kernel receipt or state-inclusion proof.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct CommittedProgramBalanceFreshness {
    pub height: u64,
    pub block_hash: [u8; 32],
    pub timestamp: u64,
}

/// One real derived account rendered for an agent without losing its asset or
/// frozen-state semantics.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramValueBalance {
    pub account: [u8; 32],
    pub asset: [u8; 32],
    pub amount: u128,
    pub frozen: Option<bool>,
}

/// Current-head evidence attached to the complete program account list.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ProgramBalanceFreshness {
    pub observed_sequence: u64,
    pub observed_at: u64,
    pub receipt_digest: [u8; 32],
    pub state_root: [u8; 32],
    pub valid_through: u64,
}

/// Agent-facing balances with the evidence coordinates of their read path.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramBalanceRead<F = ProgramBalanceFreshness> {
    pub program: [u8; 32],
    pub lifecycle: ProgramLifecycle,
    pub accounts: Vec<ProgramValueBalance>,
    pub freshness: F,
}

pub(super) fn program_balances(
    read: &VerifiedProgramBalanceRead,
    staleness_limit: u64,
) -> Result<ProgramBalanceRead, ProgramReadError> {
    let valid_through = read
        .freshness()
        .observed_at
        .checked_add(staleness_limit)
        .ok_or(ProgramReadError::NonCanonicalView)?;
    Ok(ProgramBalanceRead {
        program: read.program().bytes(),
        lifecycle: read.lifecycle(),
        accounts: read
            .value_accounts()
            .iter()
            .map(|account| ProgramValueBalance {
                account: account.account_id,
                asset: account.asset_id,
                amount: account.balance,
                frozen: Some(account.frozen),
            })
            .collect(),
        freshness: ProgramBalanceFreshness {
            observed_sequence: read.freshness().observed_sequence,
            observed_at: read.freshness().observed_at,
            receipt_digest: read.receipt_digest(),
            state_root: read.state_root(),
            valid_through,
        },
    })
}
