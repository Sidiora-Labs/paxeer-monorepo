//! Audit 22.30.107 regression paths; finding and evidence record: repair_22_33_63.kvx.

#[path = "support/repair_22_33_63_bank.rs"]
pub mod bank;
#[path = "native_send_nonces.rs"]
mod native;
#[path = "support/repair_22_33_63_rotation.rs"]
pub mod rotation;

#[test]
fn journey_faults_producer_not_reopened() {
    bank::producer_outbox_and_consumer_reopen();
}

#[test]
fn move_test_substituted_transaction() {
    bank::exact_custody_bank_transfer();
}

#[test]
fn native_approval_clock_units() {
    native::consent_expiry_round_trip("repair-22-33-63-approval-expiry");
}

#[test]
fn native_producer_clock_units() {
    native::consent_expiry_round_trip("repair-22-33-63-producer-expiry");
}

#[test]
fn native_producer_assumes_static_head() {
    native::progressing_head_preserves_exact_action();
}

#[test]
fn session_key_rotation_synthetic_transaction() {
    rotation::current_rotation_reaches_custody("repair-22-33-63-session-rotation");
}

#[test]
fn session_rung_rotation_synthetic_transaction() {
    rotation::current_rotation_reaches_custody("repair-22-33-63-rung-rotation");
}
