//! Task 22.33.1536 repair record (audit 22.30.95, requirement 22998.1).
//!
//! Finding 25: approval::ledger_preparation_projects_real_call_and_refuses_semantic_changes,
//! approval::approval_is_bound_to_the_disclosure_from_prepared_bytes,
//! approval::concurrent_approvals_have_exactly_one_winner and
//! approval::replay_refuses_a_hold_whose_call_bytes_or_sender_changed.
//! Finding 26: approval_ops::approval_service_refuses_changed_call_and_changed_disclosure,
//! retaining the exact-preparation, decision, tenant and expiry cases.
//! Finding 28: approval_semantics::changed_disclosure_cannot_reuse_a_durable_approval_decision,
//! retaining conflicting-decision, decision-restart and downtime-expiry cases.
//! Finding 21: evm_signing::self_signed_transaction_recovers_to_the_session_address
//! provisions and reloads the decoded authorization grant, checks exact scope,
//! reopens the encrypted registry and refuses a durably revoked signer.
//! Finding 34: export::state_export_uses_ics23_store_and_app_witness_and_rejects_tampering
//! transports the committed node ICS23 vector through the real storage decoder,
//! store/app verifier and daemon export consumer, including its success assertion.
//! Finding 33: finality_rungs::committed_requires_more_than_two_thirds_voting_power
//! and finality_rungs::empty_or_unrelated_block_cannot_promote_a_receipt bind a real
//! signed transaction to the quorum-authenticated Comet header data root, after
//! reconstructing its full transaction through the production node block decoder.
//! Finding 12: handshake_gate::startup_is_not_ready_until_real_node_reports_receipt_keys
//! and handshake_gate::empty_receipt_keys_do_not_open_the_write_gate use the
//! ReceiptKeysResult object envelope and numeric windows over real NodeRpc HTTP.
//! Finding 18: human_node_budgets::budget_pipeline_submits_raw_transaction_and_reads_get_budget
//! and human_node_budgets::receipt_payment_denomination_is_required_and_bound_to_signed_asset
//! preserve the full receipt field set and bind paymentDenom to the signed asset.
//!
//! Inherited evidence (attempt 7, revision 85a8c333e18816a00a1fafcb7ea77d26f77c54b2):
//! 44/47 cases passed; the oversized header-bound quorum case and two state export
//! cases failed. No unchanged baseline gate is repeated in this continuation.
//! Attempt 8 integrates 22.31.561's export witness encoder/decoder and verifier:
//! export.rs::Fixture::new obtains the committed ICS23 vector over eth_getProof,
//! calls encode_storage_witnesses, and binds it to the header app hash and account.
//! The local JSON row encoder is removed; tampered witnesses refuse with Contract.
//! finality_rungs.rs retains both u64::MAX cases, expecting InvalidCommit under
//! verify_with_header via checkpoint.rs's i64::MAX / 8 total voting power bound.
//! The public VerifiedBlock::verify still proves u128 quorum acceptance/refusal.
//! Wiring request (domain 9): services/agent/crates/layerx-agentd/src/finality.rs::observe_block
//! cannot satisfy the ruling's no-header success expectation: VerifiedBlock::binds
//! requires a header while VerifiedBlock::verify sets header to None. Both real
//! paths are exercised; no-header promotion remains refused and authenticated
//! normal-weight quorum cases still reach Committed.
//! Qualification evidence: the runner's private gate ledger /out/run/gates.jsonl
//! records this candidate's exact revision, commands, exits and private log paths;
//! hf-build and hf-done are the only gates and no file is amended after they pass.

#[path = "approval.rs"]
mod approval;
#[path = "approval_ops.rs"]
mod approval_ops;
#[path = "approval_semantics.rs"]
mod approval_semantics;
#[path = "evm_signing.rs"]
mod evm_signing;
#[path = "export.rs"]
mod export;
#[path = "finality_rungs.rs"]
mod finality_rungs;
#[path = "handshake_gate.rs"]
mod handshake_gate;
#[path = "human_node_budgets.rs"]
mod human_node_budgets;
