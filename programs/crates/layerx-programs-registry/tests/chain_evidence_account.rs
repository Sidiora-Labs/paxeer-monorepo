#[allow(dead_code)]
#[path = "support/chain_deployment_support.rs"]
mod repair;

#[test]
fn chain_account_snapshot_verifies_primary_leaf() {
    repair::account_proof_fixture();
}
