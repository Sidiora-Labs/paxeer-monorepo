use std::time::Duration;

use x_websearch::payment::{PaymentResolution, PAYMENT_RESOLUTION_INTERVAL};

#[test]
fn payment_start_unresolved_state_carries_its_reason() {
    let status = PaymentResolution {
        payable_assets: Vec::new(),
        withheld_assets: Vec::new(),
        state: "unresolved",
        reason: Some("receiver identity read failed".to_owned()),
    };
    assert_eq!(status.state, "unresolved");
    assert_eq!(
        status.reason.as_deref(),
        Some("receiver identity read failed")
    );
}

#[test]
fn payment_start_resolution_interval_is_thirty_seconds() {
    assert_eq!(PAYMENT_RESOLUTION_INTERVAL, Duration::from_secs(30));
}

#[test]
fn payment_start_ready_state_has_no_reason() {
    let status = PaymentResolution {
        payable_assets: Vec::new(),
        withheld_assets: Vec::new(),
        state: "ready",
        reason: None,
    };
    assert_eq!(status.state, "ready");
    assert!(status.reason.is_none());
}
