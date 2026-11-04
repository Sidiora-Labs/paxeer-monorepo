#[path = "support/repair_22_33_221.rs"]
mod repair;

#[test]
fn chain_upgrade_restart_test() {
    repair::restart_recovery(true);
}

#[test]
fn account_proof_fixture() {
    repair::account_proof_fixture();
}

#[test]
fn deployment_interface_fixture() {
    repair::deployment_interface_fixture();
}

#[test]
fn header_bootstrap_test() {
    repair::header_bootstrap_test();
}

#[test]
fn verifier_interface_coverage() {
    repair::verifier_interface_coverage();
}

#[test]
fn native_interface_transport_test() {
    repair::native_interface_transport();
}

#[test]
fn registry_proof_restart_coverage() {
    repair::restart_recovery(false);
}
