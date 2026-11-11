//! Task 22.33.1518 executes both repaired integration suites through their real
//! daemon/client/crypto paths. Finding-to-test mapping and baseline limitations
//! are recorded in `repair_22_33_1518.kvx` beside this target.
//!
//! Receipt peers replay committed node conformance transcripts over loopback HTTP;
//! preparation peers use production `ledger_payment_call` and actual signatures.
//! Store, outbox, idempotency and approval recovery reopen real durable stores.

#[path = "human_node_receipts.rs"]
mod human_node_receipts;
#[path = "proof_agentd.rs"]
mod proof_agentd;
