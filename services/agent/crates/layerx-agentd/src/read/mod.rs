//! Proof-gated daemon read surfaces.

pub(crate) mod abci_result;
#[path = "availability.rs"]
mod available;
#[path = "balance.rs"]
mod balances;
#[path = "checkpoint.rs"]
mod checkpoint_impl;
pub mod committed;
#[path = "history.rs"]
mod history_impl;
mod native;
mod result_source;
pub use abci_result::AbciResultProof;
pub use committed::{CommittedCaps, CommittedReadError, CommittedReads};
pub use result_source::NativeResultSource;
#[path = "program_balances.rs"]
mod program_balances_impl;
#[path = "program_node.rs"]
mod program_node;
#[path = "proof_bundle.rs"]
pub(crate) mod proof_bundle_owner;
pub use native::{NativeReadError, NativeReadRoute};
pub use proof_bundle_owner::{acquire_native_receipt, NativeProofRequest};

pub use available::{availability, AvailabilityRead, AvailabilityReadError, AvailabilityRequest};
pub use balances::{balance, BalanceRead, Freshness};
pub use checkpoint_impl::{
    proof_bundle, validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CheckpointReadError,
    CommitTimestamp, CommitValidator, CommitVote, ProofBundleKind, ProofBundleRequest,
    ServedCheckpoint, ServedProofBundle,
};
pub use program_balances_impl::{
    program_balances_from_protocol, CommittedProgramBalanceFreshness, ProgramBalanceFreshness,
    ProgramBalanceRead, ProgramValueBalance, ProtocolProgramBalanceReader,
};
pub use program_node::{
    ChainHeadError, LayerxdProgramBalanceReader, ProgramAuthority, ProgramBalanceReadRoute,
    ProgramReadError,
};

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
pub use history_impl::{history, Cursor, HistoryLimits, HistoryPage, HistoryReadError};

pub mod checkpoint_request;
pub use checkpoint_request::CheckpointRequest;
