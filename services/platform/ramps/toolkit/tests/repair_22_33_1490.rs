//! Recovery and payment tests exercise retained receipt and signer evidence.

#[path = "support/repair_22_33_1490.rs"]
pub mod support;
pub use support::*;

#[test]
fn v2_recovery_engine_writes_and_production_recovery_reads_after_restart() {
    support::engine_restart();
}

#[test]
fn receipt_evidence_comes_from_recorded_node_transport_and_rejects_mutations() {
    support::recorded_receipt();
}

#[test]
fn signer_compact_contract_refuses_third_shape_and_wrong_key() {
    support::signer_contract();
}

#[test]
fn maintained_customer_payment_binds_authority_and_refuses_retired_calldata() {
    support::customer_authority();
}

#[test]
fn recovery_node_namespace_and_terminal_custody_contradictions() {
    support::custody_restart();
}
