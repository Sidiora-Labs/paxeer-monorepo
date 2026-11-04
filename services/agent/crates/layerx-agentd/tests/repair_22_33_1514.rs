//! subject_registration_test_gap: current registration producer -> durable Outbox
//! -> managed subject read/write resolution, before/after commitment and restart.

use layerx_agentd as agentd;

#[path = "../src/human_runtime/subject/registration_tests.rs"]
mod registration_tests;

#[test]
fn subject_registration_test_gap_resolves_reads_and_writes_across_commitment_and_restart() {
    registration_tests::prove_registration_resolution(|_, alice, bob, _| {
        (alice.clone(), bob.clone())
    });
}
