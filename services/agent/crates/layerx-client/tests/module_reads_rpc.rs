#[path = "support/witness_22_33_1546.rs"]
mod repair;

#[test]
fn module_state_reads_the_storage_slot_proof_at_the_committed_height() {
    repair::f02_module_state_returns_only_the_derived_slot_witness();
}

#[test]
fn module_state_refuses_a_storage_proof_for_another_slot() {
    repair::f02_module_state_refuses_slot_value_store_root_and_app_hash_substitution();
}
