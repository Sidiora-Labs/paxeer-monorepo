#[path = "support/settlement_fixture.rs"]
mod fixture;

type TestResult = Result<(), Box<dyn std::error::Error + Send + Sync>>;

#[test]
fn legacy_plain_listener_inputs() -> TestResult {
    fixture::minimal_node_receipt_pin_startup()
}

#[test]
fn legacy_sequencer_start_inputs() -> TestResult {
    fixture::minimal_node_receipt_pin_startup()
}

#[test]
fn unavailable_node_refuses_minimal_startup() -> TestResult {
    fixture::unavailable_node_refuses_minimal_startup()
}

#[test]
fn private_network_route_coverage() -> TestResult {
    fixture::private_network_payment_recovers_through_mounted_route()
}
