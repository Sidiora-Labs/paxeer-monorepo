#[path = "support/repair_22_33_221.rs"]
mod repair;

#[test]
fn chain_account_snapshot_verifies_primary_leaf() {
    repair::account_proof_fixture();
}
