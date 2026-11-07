//! Task 22.33.1571, audit 22.30.99, exactly-once-effect-gap.
//! Proof: limits::node_recovery::lost_response_recovers_receipt_balances_and_budget_once
//! and limits::node_recovery::persisted_receipt_recovers_unsettled_budget_once.
//! Baseline: the declared repair target did not exist on aa5e0820b3652890f3812a220f4b91f4f8a45387;
//! no baseline test failure is claimed. The task's implementation-first gate order precludes
//! the audit template's request for an additional unchanged-tree verification.
//! Continued baseline: attempt 2 failed both node_recovery cases with NotQueued;
//! the two existing limits cases passed. Recovery now uses production resolve_unknown:
//! a found receipt forbids resend; an absent receipt permits one exact-byte resend
//! at the retained sender nonce, followed by authenticated receipt resolution.
//! A retry during resolver backoff performs no transport exchange or additional resend;
//! receipt resolution keeps the budget held until durable settlement.
//! Qualification repair: release the test's shared store handle before reopening the writer.
//! Wiring request: rpc/sequencer_receipt_vectors_test.go (node domain) must record duplicate raw sends
//! across a node restart with ledger balance reads; the committed receipt transcript below
//! proves daemon recovery and authenticated recorded effects, not execution of a node restart.
//! Qualification revision and commands are recorded by hf-build/hf-done in private evidence.

#[path = "limits/exactly_once.rs"]
mod limits;
