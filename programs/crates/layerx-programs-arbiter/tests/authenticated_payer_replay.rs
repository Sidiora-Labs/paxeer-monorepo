#[path = "proof_arbiter_crate.rs"]
#[allow(dead_code)]
mod producer;

#[test]
fn arbiter_proof_pins_payer_and_verifies_every_genuine_adjacent_pair() {
    producer::repair_executor_payer_replay();
}

#[test]
fn arbiter_proof_rejects_unauthorized_payer_activity_and_context() {
    producer::repair_authenticated_execution_refusals();
}
