//! Retirement disposition for the former emulator settlement authority.
//!
//! This process has no merged-network block-commit authority. A retained
//! authorized batch cannot establish either portable v2 evidence or finality.
//! Sellers verify node-produced receipts with `ReceiptPayloadAuthority` and
//! require independently verified block-commit evidence for the committed rung.

pub const RETIRED_CODE: &str = "settlement_authority_retired";
pub const RETIRED_DETAIL: &str = "POST /v1/settle is retired; use the seller ReceiptPayloadAuthority with pinned pax_sequencerReceipt keys and block-commit evidence for committed payments";
