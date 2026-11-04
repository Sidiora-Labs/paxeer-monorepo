//! Original prerequisite proofs share the audit protocol peers with the repair gate.
#[allow(dead_code)]
#[path = "support/audit_22_30_99.rs"]
mod audit;

#[test]
fn wallet_native_token_and_node_summary_round_trip_after_reopen() {
    audit::wallet_native_token_and_node_summary_round_trip_after_reopen();
}

#[test]
fn committed_wait_uses_live_block_authority_after_reopen() {
    audit::committed_wait_uses_live_block_authority_after_reopen();
}

#[test]
fn daemon_not_ready_goldens_preserve_value_and_rung_order() {
    audit::daemon_not_ready_goldens_preserve_value_and_rung_order();
}

#[test]
fn program_constructors_share_live_abi_submit_retry_and_recovery() {
    audit::program_constructors_share_live_abi_submit_retry_and_recovery();
}

#[test]
fn native_owner_preview_prepare_disclose_external_sign_and_submit() {
    audit::native_owner_preview_prepare_disclose_external_sign_and_submit();
}

#[test]
fn abi5_reward_authority_receipt_recovers_after_node_reopen() {
    audit::abi5_reward_authority_receipt_recovers_after_node_reopen();
}

#[test]
fn daemon_not_ready_lookup_crosses_authenticated_transport() {
    audit::daemon_not_ready_lookup_crosses_authenticated_transport();
}

#[test]
fn program_abi_constructor_retains_c_producer_transaction() {
    audit::program_abi_constructor_retains_c_producer_transaction();
}
