mod publication {
    use layerx_gas_station::journal::Journal;
    use layerx_gas_station::quote::{Address, Word};
    use layerx_gas_station::rate::RateRefusal;
    use layerx_gas_station::rpc::{bytes, hex, ConfiguredRpc, RpcFault};
    use layerx_wire::receipt::encode;
    use serde_json::{json, Value};
    type TestResult = Result<(), Box<dyn std::error::Error>>;
    include!("support/rate_repairs.rs");
}

mod http {
    include!("support/service_harness.rs");
    include!("support/service_repairs.rs");
}

#[test]
fn rate_fixture_effects_success_and_commitment() -> Result<(), Box<dyn std::error::Error>> {
    publication::publication_success_commit_and_effects()
}

#[test]
fn rate_fixture_effects_pending_publication_restart() -> Result<(), Box<dyn std::error::Error>> {
    publication::publication_pending_restart_and_commit()
}

#[test]
fn rate_fixture_effects_reverted_publication() -> Result<(), Box<dyn std::error::Error>> {
    publication::publication_revert_and_effects()
}

#[test]
fn rate_fixture_effects_view_only_tampering() -> Result<(), Box<dyn std::error::Error>> {
    publication::publication_view_only_tampering_is_refused()
}

#[test]
fn revert_fixture_shape_matches_signed_node_recording() -> Result<(), Box<dyn std::error::Error>> {
    http::producer_revert_retains_signed_effects_and_malformed_checks()
}

#[test]
fn revert_fixture_shape_http_refusal_status_and_restart() -> Result<(), Box<dyn std::error::Error>>
{
    http::reverted_receipt_survives_http_status_and_restart()
}

#[test]
fn registered_budget_test_gap_admissions_and_restart() -> Result<(), Box<dyn std::error::Error>> {
    http::registered_budgets_cross_caps_and_survive_restart()
}
