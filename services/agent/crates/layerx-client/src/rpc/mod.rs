//! LayerX JSON-RPC client modules.

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Version {
    pub major: u16,
    pub minor: u16,
}

/// Freshness coordinates attached to every head-sensitive result.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Head {
    pub chain_sequence: u64,
    pub sealed_batch: u64,
    pub finalised_checkpoint: [u8; 32],
}

pub mod account;
pub mod batch;
pub mod commit;
pub mod evidence;
pub mod history;
pub mod module_state;
pub mod payments;
pub mod receipt;
pub mod receipt_commit;
pub mod stream;
pub mod submit;
pub mod withdrawal;
