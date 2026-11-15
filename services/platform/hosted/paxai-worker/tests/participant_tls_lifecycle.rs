//! Audit 22.30.84 participant / worker-live-path-test-gap regression evidence.
#[path = "support/participant.rs"]
mod participant;

#[test]
fn participant_tls_same_store_chain_commit_and_restart() {
    participant::participant_tls_same_store_chain_commit_and_restart();
}

#[test]
fn participant_tls_cancel_survives_store_reopen() {
    participant::participant_tls_cancel_survives_store_reopen();
}
