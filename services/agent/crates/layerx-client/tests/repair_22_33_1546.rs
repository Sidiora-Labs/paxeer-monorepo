//! Task 22.33.1546: producer transcripts cross real HTTP and the public Rust APIs.
//! Finding/test mapping, baseline limitation and wiring requests: `repair_22_33_1546.kvx`.
#[path = "support/gateway_22_33_1546.rs"]
mod gateway;
#[path = "support/market_22_33_1546.rs"]
mod market;
#[path = "support/receipt_22_33_1546.rs"]
mod receipt;
#[path = "support/witness_22_33_1546.rs"]
mod witness;

#[test]
fn f39_signed_sequence_seven_execution_height_150_recovers_and_requires_covering_commit() {
    market::receipt_sequence_and_height_survive_reopen();
}
#[test]
fn f37_producer_payment_denom_is_authenticated_and_journal_recovers() {
    receipt::producer_receipts_survive_reopen_and_refuse_missing_payment_denom();
}
#[test]
fn f36_proof_bundle_authenticates_every_requested_witness_and_landing_block() {
    witness::f03_proof_bundle_retains_every_requested_slot_identity_and_value();
    witness::f03_proof_bundle_refuses_absent_mismatched_and_uncommitted_material();
}
#[test]
fn f44_gateway_history_keeps_page_coordinates_readiness_and_refusals() {
    gateway::history();
}
#[test]
fn f35_public_module_state_authenticates_derived_slots_values_and_both_roots() {
    witness::f02_module_state_returns_only_the_derived_slot_witness();
    witness::f02_module_state_refuses_slot_value_store_root_and_app_hash_substitution();
}
#[test]
fn f42_gateway_flat_asset_metadata_survives_list_and_lookup() {
    gateway::assets();
}
#[test]
fn f40_gateway_repeated_reads_and_verified_market_finality_survive_consumers() {
    gateway::repeated_reads();
    for _ in 0..2 {
        market::market_finality_requires_same_root_node_evidence();
    }
    market::receipt_sequence_and_height_survive_reopen();
}
#[test]
fn f41_gateway_balances_preserve_uint256_and_flat_asset_metadata() {
    gateway::balances();
    gateway::assets();
}
