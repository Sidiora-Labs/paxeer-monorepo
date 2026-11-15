#[path = "proof_arbiter_crate.rs"]
#[allow(dead_code)]
mod producer;

#[test]
fn abi_v5_authority_crosses_admission_and_replay() {
    producer::repair_v5_authority();
}

#[test]
fn abi_v5_authority_retains_unknown_version_refusals() {
    producer::repair_unknown_abi_refusal();
}

#[test]
fn executor_payer_replay_accepts_genuine_adjacent_steps() {
    producer::repair_executor_payer_replay();
}

#[test]
fn authenticated_executor_refuses_mismatched_payer_activity_and_context() {
    producer::repair_authenticated_execution_refusals();
}
