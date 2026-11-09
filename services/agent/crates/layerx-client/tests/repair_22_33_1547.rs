//! Audit 22.30.96, f46/f38/f45/f43: the inventory targets execute through their
//! real public clients and loopback producer transcripts. See `repair_22_33_1547.kvx`.
#[path = "receipt_rpc.rs"]
mod receipt;
#[path = "sequencer.rs"]
mod sequencer;
#[path = "sequencer_reads.rs"]
mod sequencer_reads;
#[path = "withdrawal_rpc.rs"]
mod withdrawal;
