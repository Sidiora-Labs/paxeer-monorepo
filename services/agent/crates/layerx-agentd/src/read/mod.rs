//! Proof-gated daemon read surfaces.

#[path = "availability.rs"]
mod available;
#[path = "balance.rs"]
mod balances;
#[path = "checkpoint.rs"]
mod checkpoint_impl;
#[path = "history.rs"]
mod historical;
mod native;
#[path = "program_balances.rs"]
mod program_balances_impl;
#[path = "program_node.rs"]
mod program_node;
#[path = "proof_bundle.rs"]
pub(crate) mod proof_bundle_owner;
pub use native::{NativeReadError, NativeReadRoute};

pub use available::{
    availability, AvailabilityAudit, AvailabilityFailure, AvailabilityRead, AvailabilityRequest,
    ReplayFraming,
};
pub use balances::{balance, BalanceRead, Freshness};
pub use checkpoint_impl::{
    proof_bundle, validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CheckpointReadError,
    CommitTimestamp, CommitValidator, CommitVote, ProofBundleKind, ProofBundleRequest,
    ServedCheckpoint, ServedProofBundle,
};
pub use program_balances_impl::{
    program_balances_from_protocol, ProgramBalanceFreshness, ProgramBalanceRead,
    ProgramValueBalance, ProtocolProgramBalanceReader,
};
pub use program_node::{LayerxdProgramBalanceReader, ProgramAuthority, ProgramBalanceReadRoute};

/// Verifies the block header and its Comet precommit quorum at the requested height.
/// The caller supplies the trusted validator set for this chain and height in consensus order.
///
/// # Errors
///
/// Returns `Commit` for a mismatched block, invalid signature or voting power at or below two thirds.
pub fn checkpoint(
    requested_height: u64,
    expected_chain_id: &str,
    header: &CheckpointHeader,
    commit: &BlockCommit,
    validators: &[CommitValidator],
) -> Result<ServedCheckpoint, CheckpointReadError> {
    checkpoint_impl::serve_checkpoint(
        requested_height,
        expected_chain_id,
        header,
        commit,
        validators,
    )
}
pub use historical::{history, Cursor, HistoryLimits, HistoryPage, HistoryReadError};
