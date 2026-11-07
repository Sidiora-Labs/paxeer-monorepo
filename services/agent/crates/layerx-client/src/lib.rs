//! Clients for the merged `paxd` BFT chain (EVM chain id 125).
//!
//! [`sequencer`] and [`rpc`] use the node's root JSON-RPC endpoint for chain
//! reads, signed transactions and committed-block evidence. [`px`] joins account,
//! asset and history data at the hosted gateway; its labels do not prove finality.
//! [`frame`] provides a generic bounded codec and transport, without a separate
//! kernel protocol boundary.

pub mod account_binding;
pub mod budget;
pub mod chains;
pub mod fee;
pub mod frame;
pub mod grants;
pub mod paxeer_binding;
pub mod px;
pub mod rpc;
#[cfg(target_os = "linux")]
pub mod runtime_clock;
pub mod sequencer;
pub mod storage_witness;
pub mod subscription;

pub mod ai_market;
