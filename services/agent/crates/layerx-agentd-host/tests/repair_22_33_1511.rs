//! Finding/test mapping and fixture provenance: `repair_22_33_1511.kvx`.

#[path = "support/payer_audit.rs"]
pub mod payer_audit;

#[test]
fn payer_equal_denom_fixture_accepts_independent_receipt_and_rejects_either_change() {
    payer_audit::payer_equal_denom_fixture();
}

#[test]
fn payer_binding_bypass_publishes_loads_and_checks_exact_and_draw_receipts() {
    payer_audit::payer_binding_bypass();
}
