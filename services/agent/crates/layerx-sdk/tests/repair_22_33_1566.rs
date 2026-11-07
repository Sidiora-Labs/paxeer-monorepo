//! Audit 22.30.99: real SDK codecs, HTTP/TLS transports and binary actions.
#[allow(dead_code)]
#[path = "support/audit_22_30_99.rs"]
mod audit;

#[test]
fn signed_prepare_fixture() {
    audit::native_owner_preview_prepare_disclose_external_sign_and_submit();
}

#[test]
fn bootstrap_probe_credentials() {
    audit::authenticated_registration_and_session_reach_exact_owner_results();
}

#[test]
fn program_constructor_test_gap() {
    audit::program_abi_constructor_retains_c_producer_transaction();
    audit::program_constructors_share_live_abi_submit_retry_and_recovery();
}

#[test]
fn bridge_proof_positive_gap() {
    audit::bridge_balances_and_summary_receipt_survive_reopen();
}

#[test]
fn proof_sdk_positive_gap() {
    audit::wallet_native_token_and_node_summary_round_trip_after_reopen();
}

#[test]
fn rpc_route_fixture() {
    audit::root_rpc_request_and_method_response();
}

#[test]
fn wallet_route_fixture() {
    audit::root_wallet_request_and_method_response();
}

#[test]
fn binary_route_fixture() {
    audit::root_binary_request_and_method_response();
}

#[test]
fn route_refusals_preserve_endpoint_framing_and_deadline_bounds() {
    audit::root_routes_preserve_endpoint_framing_and_timeout_refusals();
}
